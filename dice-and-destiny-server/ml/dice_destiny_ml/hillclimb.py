from __future__ import annotations

import json
import os
import time
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

from stable_baselines3.common.vec_env import DummyVecEnv, SubprocVecEnv

from .campaign import (
    CHECKPOINT_400_RECIPE,
    _apply_recipe,
    _parameter_snapshot,
    _recipe_from_model,
    _training_telemetry,
    optimizer_hash,
)
from .champions import (
    SAFETY_FIELDS,
    CampaignTimer,
    Champion,
    ChampionRegistry,
    atomic_json_write,
    canonical_hash,
    global_block,
    read_seed_bank,
    sha256_file,
    write_seed_bank,
)
from .evaluation import evaluate
from .gym_env import AuthorityGymEnv
from .profiled_ppo import ProfiledMaskablePPO
from .resources import BLAS_THREAD_ENVIRONMENT, ResourceBudget, capture_host_state
from .schema_v2 import ACTION_SCHEMA_V2, OBSERVATION_SCHEMA_V2
from .training import (
    ArtifactCallback,
    atomic_model_save,
    parameter_hash,
    set_reproducible_runtime,
)

HILLCLIMB_BANK_SCHEMA = "dice-and-destiny-global-hillclimb-seed-banks-v1"
HILLCLIMB_STATE_SCHEMA = "dice-and-destiny-global-hillclimb-state-v1"
HILLCLIMB_CONTROLS_SCHEMA = "dice-and-destiny-global-hillclimb-controls-v1"
SOURCE_CONFIG_SHA256 = "7e7095010cd91a9b0d3017d09bc951c89bdba6678ea5a154ec73c411ad83d35c"
ACCEPTED_V1_SHA256 = "e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8"
DEFAULT_INTERVAL_STEPS = 50_000

EXPECTED_SOURCE_TRAINING = {
    "ablation_condition": "current-recipe",
    "authority_mode": "ephemeral",
    "batch_size": 256,
    "checkpoint_interval": 5_000,
    "critic_architecture": "dense",
    "device": "cpu",
    "entropy_coefficient": 0.01,
    "epochs": 8,
    "gae_lambda": 0.95,
    "gamma": 0.995,
    "imitation_decisions": 5_000,
    "imitation_epochs": 5,
    "imitation_teacher": "mechanics-v2",
    "instrumentation": False,
    "learning_rate": 3e-4,
    "max_episode_actions": 1_200,
    "observation_schema": OBSERVATION_SCHEMA_V2,
    "opponent_selection_mode": "legacy-flat",
    "opponent_specs": [
        "random",
        "mechanics-v2",
        "model:runs/phase2-training/seed-11/checkpoints/final.zip",
        "historical",
    ],
    "reward": "+1 victory, -1 defeat, 0 draw; no shaping",
    "rollout_steps": 256,
    "sparse_actor": True,
    "telemetry_mode": "training",
    "transport_mode": "full",
    "worker_blas_threads": 1,
    "worker_torch_interop_threads": 1,
    "worker_torch_threads": 1,
    "workers": 12,
}


@dataclass(frozen=True)
class HillClimbConfig:
    campaign_id: str
    artifact_root: Path
    registry_path: Path
    seed_bank_root: Path
    source_config_path: Path
    historical_checkpoint_root: Path
    accepted_v1_checkpoint: Path
    binary: Path
    server_root: Path
    budget: ResourceBudget
    interval_steps: int = DEFAULT_INTERVAL_STEPS
    time_budget_seconds: float = 5_400.0
    maximum_attempts: int = 128
    require_full_window: bool = True
    seat_a_definition: str = "blade_warden"
    seat_b_definition: str = "blade_warden"


def write_hillclimb_seed_banks(
    root: Path,
    *,
    attempts: int,
    evaluation_seed_offset: int,
    training_seed_offset: int,
) -> dict[str, Any]:
    if attempts < 1:
        raise ValueError("hill-climb seed banks require at least one attempt")
    if root.exists() and any(root.iterdir()):
        raise FileExistsError(f"refusing to overwrite hill-climb seed banks: {root}")
    root.mkdir(parents=True, exist_ok=True)
    files: list[dict[str, Any]] = []
    used: set[int] = set()
    for attempt in range(1, attempts + 1):
        start = evaluation_seed_offset + (attempt - 1) * 500
        seeds = list(range(start, start + 500))
        if used.intersection(seeds):
            raise RuntimeError("hill-climb evaluation seed banks overlap")
        used.update(seeds)
        path = root / f"attempt-{attempt:03d}-global-1k.json"
        bank = write_seed_bank(
            path,
            bank_id=f"global-hillclimb-attempt-{attempt:03d}-1k",
            seeds=seeds,
            purpose="Fresh 1,000-game seat-swapped challenger-versus-current-global gate",
        )
        files.append(
            {
                "attempt": attempt,
                "games": 1_000,
                "path": str(path.resolve()),
                "sha256": bank["sha256"],
                "training_seed": training_seed_offset + attempt,
            }
        )
    manifest: dict[str, Any] = {
        "schema": HILLCLIMB_BANK_SCHEMA,
        "generated_at": datetime.now(UTC).isoformat(),
        "attempts": attempts,
        "evaluation_seed_offset": evaluation_seed_offset,
        "training_seed_offset": training_seed_offset,
        "distinct_evaluation_seeds": len(used),
        "files": files,
    }
    manifest["sha256"] = canonical_hash(manifest)
    atomic_json_write(root / "manifest.json", manifest)
    return manifest


def frozen_controls(config: HillClimbConfig, champion: Champion) -> dict[str, Any]:
    source = json.loads(config.source_config_path.read_text())
    training = source.get("training") or {}
    selected_training = {key: training.get(key) for key in EXPECTED_SOURCE_TRAINING}
    controls: dict[str, Any] = {
        "schema": HILLCLIMB_CONTROLS_SCHEMA,
        "source_config_path": str(config.source_config_path.resolve()),
        "source_config_sha256": sha256_file(config.source_config_path),
        "initial_global_sha256": champion.checkpoint_sha256,
        "initial_global_learner_steps": champion.learner_steps,
        "model_family_controls": {
            "environment_schema": source.get("environment_schema"),
            "observation_schema": source.get("observation_schema"),
            "action_schema": source.get("action_schema"),
            "model_architecture": source.get("model_architecture"),
            "critic_architecture": training.get("critic_architecture"),
            "sparse_actor": training.get("sparse_actor"),
            "reward": source.get("reward"),
            "post_ppo_correction": False,
        },
        "resume_safe_controls": {
            **selected_training,
            "clip_range": CHECKPOINT_400_RECIPE.clip_range,
            "target_kl": CHECKPOINT_400_RECIPE.target_kl,
            "value_loss_coefficient": CHECKPOINT_400_RECIPE.value_loss_coefficient,
            "interval_requested_steps": config.interval_steps,
            "evaluation_games": 1_000,
            "evaluation_swap": True,
            "promotion_rule": "adjusted score strictly greater than 50% and zero safety counts",
            "failure_rule": "discard challenger and reload the unchanged current global champion",
        },
        "resolved_opponents": [
            "random",
            "mechanics-v2",
            f"model:{config.accepted_v1_checkpoint.resolve()}",
            f"pool:{config.historical_checkpoint_root.resolve()}",
        ],
        "resource_budget": config.budget.as_dict(),
        "allowed_per_attempt_variation": {
            "training_random_stream": "predeclared unique seed",
            "evaluation_random_stream": "predeclared fresh disjoint 500-seed bank",
        },
    }
    controls["sha256"] = canonical_hash(controls)
    return controls


def preflight_hillclimb(config: HillClimbConfig) -> dict[str, Any]:
    if config.interval_steps != DEFAULT_INTERVAL_STEPS:
        raise RuntimeError("hill-climb must use the frozen 50,000-step checkpoint interval")
    if config.maximum_attempts < 1:
        raise RuntimeError("hill-climb maximum attempts must be positive")
    if config.time_budget_seconds <= 0:
        raise RuntimeError("hill-climb time budget must be positive")
    if not config.binary.is_file():
        raise FileNotFoundError(config.binary)
    registry = ChampionRegistry.load(config.registry_path.resolve())
    champion = registry.global_champion
    if registry.checkpoint_champion.checkpoint_sha256 != champion.checkpoint_sha256:
        raise RuntimeError("current checkpoint champion must equal the current global champion")
    if champion.observation_schema != OBSERVATION_SCHEMA_V2 or champion.action_schema != ACTION_SCHEMA_V2:
        raise RuntimeError("hill-climb global champion must use the checkpoint-480 V2 contracts")
    if sha256_file(config.source_config_path) != SOURCE_CONFIG_SHA256:
        raise RuntimeError("checkpoint-480 source configuration hash mismatch")
    source = json.loads(config.source_config_path.read_text())
    training = source.get("training") or {}
    for key, expected in EXPECTED_SOURCE_TRAINING.items():
        if training.get(key) != expected:
            raise RuntimeError(f"checkpoint-480 source control changed: {key}")
    if sha256_file(config.accepted_v1_checkpoint) != ACCEPTED_V1_SHA256:
        raise RuntimeError("checkpoint-480 accepted-v1 opponent hash mismatch")
    history = sorted(config.historical_checkpoint_root.glob("*.zip"))
    if len(history) != 1_002:
        raise RuntimeError(f"checkpoint-480 historical pool changed: expected 1002 ZIPs, got {len(history)}")
    model = ProfiledMaskablePPO.load(Path(champion.checkpoint_path), device="cpu")
    _validate_frozen_model(model, champion)
    bank_manifest = _load_seed_manifest(config.seed_bank_root, config.maximum_attempts)
    controls = frozen_controls(config, champion)
    config.artifact_root.mkdir(parents=True, exist_ok=True)
    controls_path = config.artifact_root / "frozen-controls.json"
    if controls_path.exists():
        existing = json.loads(controls_path.read_text())
        if existing != controls:
            raise RuntimeError("frozen hill-climb controls changed after preflight")
    else:
        atomic_json_write(controls_path, controls)
    result = {
        "status": "passed",
        "campaign_id": config.campaign_id,
        "global_champion": asdict(champion),
        "controls": str(controls_path.resolve()),
        "controls_sha256": controls["sha256"],
        "seed_manifest": str((config.seed_bank_root / "manifest.json").resolve()),
        "seed_manifest_sha256": bank_manifest["sha256"],
        "historical_checkpoint_count": len(history),
        "maximum_attempts": config.maximum_attempts,
        "time_budget_seconds": config.time_budget_seconds,
    }
    atomic_json_write(config.artifact_root / "preflight.json", result)
    return result


def run_hillclimb(config: HillClimbConfig) -> dict[str, Any]:
    preflight = preflight_hillclimb(config)
    state_path = config.artifact_root / "campaign-state.json"
    if state_path.exists():
        raise FileExistsError(f"refusing to overwrite hill-climb campaign state: {state_path}")
    registry = ChampionRegistry.load(config.registry_path.resolve())
    controls = json.loads((config.artifact_root / "frozen-controls.json").read_text())
    seed_manifest = _load_seed_manifest(config.seed_bank_root, config.maximum_attempts)
    started_wall = datetime.now(UTC)
    deadline_wall = started_wall + timedelta(seconds=config.time_budget_seconds)
    timer = CampaignTimer(config.time_budget_seconds)
    timer.start()
    state: dict[str, Any] = {
        "schema": HILLCLIMB_STATE_SCHEMA,
        "status": "running",
        "campaign_id": config.campaign_id,
        "started_at": started_wall.isoformat(),
        "deadline_at": deadline_wall.isoformat(),
        "time_budget_seconds": config.time_budget_seconds,
        "controls_sha256": controls["sha256"],
        "seed_manifest_sha256": seed_manifest["sha256"],
        "initial_global_champion": asdict(registry.global_champion),
        "current_global_champion": asdict(registry.global_champion),
        "attempts": [],
        "promotions": 0,
        "rejections": 0,
        "deadline_grace": [],
        "timer": timer.snapshot(),
        "host_before": capture_host_state(),
        "preflight": preflight,
    }
    atomic_json_write(state_path, state)
    _progress(
        "hillclimb_started",
        campaign_id=config.campaign_id,
        global_champion_sha256=registry.global_champion.checkpoint_sha256,
        controls_sha256=controls["sha256"],
        started_at=started_wall.isoformat(),
        deadline_at=deadline_wall.isoformat(),
        timer=timer.snapshot(),
    )
    try:
        for attempt in range(1, config.maximum_attempts + 1):
            if not timer.may_start_interval():
                break
            parent = registry.global_champion
            entry = seed_manifest["files"][attempt - 1]
            training_seed = int(entry["training_seed"])
            attempt_dir = config.artifact_root / "attempts" / f"attempt-{attempt:03d}"
            attempt_dir.mkdir(parents=True, exist_ok=True)
            _progress(
                "hillclimb_interval_started",
                attempt=attempt,
                parent_sha256=parent.checkpoint_sha256,
                parent_learner_steps=parent.learner_steps,
                training_seed=training_seed,
                timer=timer.snapshot(),
            )
            training = _train_one_interval(config, parent, training_seed, attempt_dir)
            challenger = training.pop("challenger")
            crossed_after_training = timer.deadline_crossed()
            if crossed_after_training:
                state["deadline_grace"].append(
                    {
                        "attempt": attempt,
                        "phase": "training_or_checkpoint_save",
                        "observed_at": datetime.now(UTC).isoformat(),
                    }
                )
            _progress(
                "hillclimb_interval_saved",
                attempt=attempt,
                challenger_sha256=challenger.checkpoint_sha256,
                learner_steps=challenger.learner_steps,
                elapsed_seconds=training["elapsed_seconds"],
                timer=timer.snapshot(),
            )
            evaluation = _evaluate_challenger(
                config,
                challenger,
                parent,
                Path(entry["path"]),
                attempt_dir / "global-gate",
            )
            gate = global_block(
                wins=evaluation["wins"],
                draws=evaluation["draws"],
                losses=evaluation["losses"],
                safety=evaluation["safety"],
            )
            next_parent = _next_parent_after_gate(parent, challenger, gate.decision)
            promoted = next_parent.checkpoint_sha256 == challenger.checkpoint_sha256
            if promoted:
                registry.promote_global(challenger)
                registry.save(config.registry_path.resolve())
                state["promotions"] += 1
                outcome = "promoted"
            else:
                reloaded = ChampionRegistry.load(config.registry_path.resolve()).global_champion
                if reloaded.checkpoint_sha256 != parent.checkpoint_sha256:
                    raise RuntimeError(
                        "failed challenger changed the global champion instead of rolling back"
                    )
                state["rejections"] += 1
                outcome = "rejected-rolled-back"
            attempt_result = {
                "attempt": attempt,
                "started_from": asdict(parent),
                "training_seed": training_seed,
                "challenger": asdict(challenger),
                "training": training,
                "evaluation": evaluation,
                "gate": {
                    **asdict(gate),
                    "games": gate.games,
                    "interval_95": list(gate.interval_95),
                },
                "outcome": outcome,
                "next_parent_sha256": registry.global_champion.checkpoint_sha256,
                "controls_sha256": controls["sha256"],
                "timer": timer.snapshot(),
            }
            atomic_json_write(attempt_dir / "attempt-result.json", attempt_result)
            _append_history(config.artifact_root / "attempt-history.jsonl", attempt_result)
            state["attempts"].append(attempt_result)
            state["current_global_champion"] = asdict(registry.global_champion)
            state["timer"] = timer.snapshot()
            atomic_json_write(state_path, state)
            _progress(
                "hillclimb_gate_completed",
                attempt=attempt,
                wins=gate.wins,
                draws=gate.draws,
                losses=gate.losses,
                adjusted_score=gate.adjusted_score,
                decision=gate.decision,
                outcome=outcome,
                next_parent_sha256=registry.global_champion.checkpoint_sha256,
                promotions=state["promotions"],
                rejections=state["rejections"],
                timer=timer.snapshot(),
            )
            if timer.deadline_crossed():
                if not crossed_after_training:
                    state["deadline_grace"].append(
                        {
                            "attempt": attempt,
                            "phase": "required_1000_game_evaluation",
                            "observed_at": datetime.now(UTC).isoformat(),
                        }
                    )
                break
        if config.require_full_window and not timer.deadline_crossed():
            raise RuntimeError("hill-climb stopped before the required 90-minute deadline")
        finished_wall = datetime.now(UTC)
        state.update(
            {
                "status": "completed",
                "finished_at": finished_wall.isoformat(),
                "timer": timer.snapshot(),
                "window_fully_used": timer.deadline_crossed(),
                "stopped_after_deadline_grace": bool(state["deadline_grace"]),
                "final_global_champion": asdict(registry.global_champion),
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _progress(
            "hillclimb_completed",
            attempts=len(state["attempts"]),
            promotions=state["promotions"],
            rejections=state["rejections"],
            final_global_sha256=registry.global_champion.checkpoint_sha256,
            timer=timer.snapshot(),
        )
        return state
    except Exception as error:
        state.update(
            {
                "status": "failed",
                "finished_at": datetime.now(UTC).isoformat(),
                "failure": {"type": type(error).__name__, "message": str(error)},
                "timer": timer.snapshot(),
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _progress("hillclimb_failed", failure=state["failure"], timer=timer.snapshot())
        raise


def _train_one_interval(
    config: HillClimbConfig,
    parent: Champion,
    training_seed: int,
    attempt_dir: Path,
) -> dict[str, Any]:
    set_reproducible_runtime(
        training_seed,
        config.budget.learner_torch_threads,
        config.budget.learner_torch_interop_threads,
        config.budget.learner_blas_threads,
    )
    vec_env = _build_vec_env(config, training_seed)
    try:
        model = ProfiledMaskablePPO.load(parent.checkpoint_path, env=vec_env, device="cpu")
        model.set_random_seed(training_seed)
        model.policy.enable_instrumentation(True)
        _validate_frozen_model(model, parent)
        _apply_recipe(model, CHECKPOINT_400_RECIPE)
        before_steps = int(model.num_timesteps)
        before_parameters = _parameter_snapshot(model)
        before_optimizer = optimizer_hash(model)
        before_parameter_hash = parameter_hash(model)
        callback = ArtifactCallback(
            attempt_dir,
            checkpoint_interval=10**12,
            instrumentation=True,
        )
        model.update_profile.clear()
        model.policy.profile.clear()
        started = time.perf_counter()
        model.learn(
            total_timesteps=config.interval_steps,
            callback=callback,
            reset_num_timesteps=False,
            progress_bar=False,
            log_interval=1,
        )
        elapsed = time.perf_counter() - started
        challenger_path = attempt_dir / "raw-ppo-challenger.zip"
        atomic_model_save(model, challenger_path)
        telemetry = _training_telemetry(
            model,
            callback,
            before_parameters,
            before_optimizer,
            elapsed,
            before_steps,
        )
        challenger = Champion.from_checkpoint(
            champion_id=f"{config.campaign_id}-attempt-{attempt_dir.name.removeprefix('attempt-')}",
            checkpoint=challenger_path,
            observation_schema=OBSERVATION_SCHEMA_V2,
            action_schema=ACTION_SCHEMA_V2,
            model_family=parent.model_family,
            learner_steps=int(model.num_timesteps),
            promoted_at=datetime.now(UTC).isoformat(),
            parent_experiment_id=parent.champion_id,
            optimizer_state_sha256=optimizer_hash(model),
        )
        if challenger.learner_steps - before_steps != 52_224:
            raise RuntimeError(
                "frozen 50,000-step interval did not complete the expected 52,224 PPO steps"
            )
        return {
            "challenger": challenger,
            "requested_steps": config.interval_steps,
            "completed_steps": challenger.learner_steps - before_steps,
            "parent_learner_steps": before_steps,
            "challenger_learner_steps": challenger.learner_steps,
            "elapsed_seconds": elapsed,
            "parameter_hash_before": before_parameter_hash,
            "parameter_hash_after": parameter_hash(model),
            "optimizer_hash_before": before_optimizer,
            "optimizer_hash_after": challenger.optimizer_state_sha256,
            "telemetry": telemetry,
            "post_ppo_correction": False,
        }
    finally:
        vec_env.close()


def _evaluate_challenger(
    config: HillClimbConfig,
    challenger: Champion,
    parent: Champion,
    bank: Path,
    output: Path,
) -> dict[str, Any]:
    seeds = read_seed_bank(bank, expected_games=1_000)
    summary = evaluate(
        binary=config.binary,
        server_root=config.server_root,
        seat_a_spec=f"model:{challenger.checkpoint_path}",
        seat_b_spec=f"model:{parent.checkpoint_path}",
        seeds=seeds,
        output_dir=output,
        swap=True,
        device="cpu",
        deterministic=True,
        save_replays="representative",
        authority_mode="normal",
        workers=config.budget.workers,
        torch_threads=config.budget.torch_threads,
        profile=config.budget.profile,
        telemetry_mode="full",
        transport_mode="full",
        observation_schema=OBSERVATION_SCHEMA_V2,
        observation_manifest=None,
        seat_a_definition=config.seat_a_definition,
        seat_b_definition=config.seat_b_definition,
    )
    spec = f"model:{challenger.checkpoint_path}"
    wins = int(summary["policies"][spec]["wins"])
    draws = int(summary["draws"])
    losses = 1_000 - wins - draws
    return {
        "wins": wins,
        "draws": draws,
        "losses": losses,
        "adjusted_score": (wins + 0.5 * draws) / 1_000,
        "seed_bank": str(bank.resolve()),
        "seed_bank_file_sha256": sha256_file(bank),
        "summary": str((output / "summary.json").resolve()),
        "summary_sha256": sha256_file(output / "summary.json"),
        "episodes_sha256": sha256_file(output / "episodes.jsonl"),
        "elapsed_seconds": float(summary["elapsed_seconds"]),
        "games_per_second": float(summary["games_per_second"]),
        "safety": {field: int(summary.get(field, 0)) for field in SAFETY_FIELDS},
        "behavior": summary.get("behavior_by_policy", {}).get(spec, {}),
    }


def _build_vec_env(config: HillClimbConfig, training_seed: int) -> DummyVecEnv | SubprocVecEnv:
    opponents = [
        "random",
        "mechanics-v2",
        f"model:{config.accepted_v1_checkpoint.resolve()}",
        f"pool:{config.historical_checkpoint_root.resolve()}",
    ]
    environments = []
    for worker in range(config.budget.workers):
        worker_seed = training_seed * 100 + worker
        environments.append(
            lambda worker_seed=worker_seed, worker=worker: AuthorityGymEnv(
                binary=config.binary,
                server_root=config.server_root,
                training_seed=worker_seed,
                opponent_specs=opponents,
                learner_seat="alternate",
                max_episode_actions=1_200,
                device="cpu",
                session_id=f"hillclimb-{config.campaign_id}-{training_seed}-{worker}",
                authority_mode="ephemeral",
                telemetry_mode="training",
                transport_mode="full",
                observation_schema=OBSERVATION_SCHEMA_V2,
                instrumentation=True,
                torch_threads=config.budget.worker_torch_threads,
                torch_interop_threads=config.budget.worker_torch_interop_threads,
                blas_threads=config.budget.worker_blas_threads,
                opponent_selection_mode="legacy-flat",
                seat_definitions={
                    "seat-a": config.seat_a_definition,
                    "seat-b": config.seat_b_definition,
                },
            )
        )
    prior = {name: os.environ.get(name) for name in BLAS_THREAD_ENVIRONMENT}
    try:
        for name in BLAS_THREAD_ENVIRONMENT:
            os.environ[name] = str(config.budget.worker_blas_threads)
        if config.budget.workers == 1:
            return DummyVecEnv(environments)
        return SubprocVecEnv(environments, start_method="spawn")
    finally:
        for name, value in prior.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value


def _validate_frozen_model(model: ProfiledMaskablePPO, champion: Champion) -> None:
    if int(model.num_timesteps) != champion.learner_steps:
        raise RuntimeError("loaded model learner steps do not match the registered champion")
    if _recipe_from_model(model) != CHECKPOINT_400_RECIPE:
        raise RuntimeError("loaded model does not use the checkpoint-480 PPO recipe")
    if type(model.policy).__name__ != "SparseCandidateMaskablePolicyV2":
        raise RuntimeError("loaded model does not use checkpoint-480's sparse V2 actor/dense critic")
    if tuple(model.observation_space.shape or ()) != (18_944,):
        raise RuntimeError("loaded model observation shape changed from checkpoint 480")
    if int(model.action_space.n) != 128:
        raise RuntimeError("loaded model action capacity changed from checkpoint 480")
    if float(model.max_grad_norm) != 0.5:
        raise RuntimeError("loaded model gradient clipping changed from checkpoint 480")


def _load_seed_manifest(root: Path, maximum_attempts: int) -> dict[str, Any]:
    path = root.resolve() / "manifest.json"
    value = json.loads(path.read_text())
    declared = value.get("sha256", "")
    actual = canonical_hash({key: item for key, item in value.items() if key != "sha256"})
    if value.get("schema") != HILLCLIMB_BANK_SCHEMA or declared != actual:
        raise RuntimeError("hill-climb seed manifest is invalid")
    files = value.get("files") or []
    if len(files) < maximum_attempts:
        raise RuntimeError("hill-climb seed manifest does not cover every possible attempt")
    training_seeds = [int(entry["training_seed"]) for entry in files[:maximum_attempts]]
    if len(training_seeds) != len(set(training_seeds)):
        raise RuntimeError("hill-climb training seeds are not unique")
    seen: set[int] = set()
    for entry in files[:maximum_attempts]:
        bank = Path(entry["path"])
        seeds = read_seed_bank(bank, expected_games=1_000)
        if seen.intersection(seeds):
            raise RuntimeError("hill-climb evaluation seed banks overlap")
        seen.update(seeds)
    return value


def _next_parent_after_gate(parent: Champion, challenger: Champion, decision: str) -> Champion:
    if decision == "pass":
        return challenger
    if decision == "fail":
        return parent
    raise ValueError(f"unknown global gate decision: {decision}")


def _append_history(path: Path, result: dict[str, Any]) -> None:
    previous = ""
    sequence = 1
    if path.exists():
        lines = path.read_text().splitlines()
        if lines:
            prior = json.loads(lines[-1])
            previous = str(prior["record_sha256"])
            sequence = int(prior["sequence"]) + 1
    record = {
        "schema": "dice-and-destiny-global-hillclimb-history-v1",
        "sequence": sequence,
        "recorded_at": datetime.now(UTC).isoformat(),
        "previous_record_sha256": previous,
        "result": result,
    }
    record["record_sha256"] = canonical_hash(record)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a") as handle:
        handle.write(json.dumps(record, sort_keys=True) + "\n")
        handle.flush()
        os.fsync(handle.fileno())


def summarize_hillclimb(state: dict[str, Any]) -> dict[str, Any]:
    attempts = state.get("attempts") or []
    training_seconds = sum(float(item["training"]["elapsed_seconds"]) for item in attempts)
    evaluation_seconds = sum(float(item["evaluation"]["elapsed_seconds"]) for item in attempts)
    total_games = sum(int(item["gate"]["games"]) for item in attempts)
    return {
        "attempts": len(attempts),
        "promotions": int(state.get("promotions", 0)),
        "rejections": int(state.get("rejections", 0)),
        "training_seconds": training_seconds,
        "evaluation_seconds": evaluation_seconds,
        "accounted_seconds": training_seconds + evaluation_seconds,
        "total_evaluation_games": total_games,
        "training_steps": sum(int(item["training"]["completed_steps"]) for item in attempts),
        "best_adjusted_score": max(
            (float(item["gate"]["adjusted_score"]) for item in attempts), default=0.0
        ),
        "final_global_champion": state.get("final_global_champion"),
    }


def _progress(event: str, **values: Any) -> None:
    print(json.dumps({"campaign_progress": event, **values}, sort_keys=True), flush=True)
