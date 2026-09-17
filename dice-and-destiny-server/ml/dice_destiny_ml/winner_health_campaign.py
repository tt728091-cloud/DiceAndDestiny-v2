from __future__ import annotations

import json
import os
import secrets
import time
from collections import Counter
from dataclasses import asdict, dataclass, replace
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

from stable_baselines3.common.vec_env import DummyVecEnv, SubprocVecEnv

from .campaign import (
    CHECKPOINT_400_RECIPE,
    EXPECTED_GLOBAL_SHA256,
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
    global_confirmation_block,
    global_progression_block,
    read_seed_bank,
    sha256_file,
    write_seed_bank,
)
from .evaluation import evaluate
from .evaluation_report import write_campaign_statistics_html
from .gym_env import WINNER_HEALTH_REWARD, AuthorityGymEnv
from .hillclimb import EXPECTED_SOURCE_TRAINING, SOURCE_CONFIG_SHA256, _validate_frozen_model
from .imitation import behavior_clone, collect_demonstrations
from .manifest_v4 import ACTION_SCHEMA_V4, OBSERVATION_SCHEMA_V4, ObservationManifestV4
from .manifest_v5 import ACTION_SCHEMA_V5, OBSERVATION_SCHEMA_V5, ObservationManifestV5
from .model import (
    SparseCandidateMaskablePolicyV2,
    SparseEntityCandidateMaskablePolicyV4,
    SparseRelationalCandidateMaskablePolicyV5,
)
from .profiled_ppo import ProfiledMaskablePPO
from .resources import BLAS_THREAD_ENVIRONMENT, ResourceBudget, capture_host_state
from .schema_v2 import ACTION_SCHEMA_V2, OBSERVATION_SCHEMA_V2
from .training import ArtifactCallback, atomic_model_save, parameter_hash, set_reproducible_runtime

CAMPAIGN_SCHEMA = "dice-and-destiny-winner-health-campaign-v1"
SEED_SCHEMA = "dice-and-destiny-winner-health-seeds-v1"
CONTROLS_SCHEMA = "dice-and-destiny-winner-health-controls-v1"
CP480_SHA256 = "38412a8821419f98a922149506de4a9c00feed617c37d9778b22e83f1c4df6c6"
INTERVAL_REQUESTED_STEPS = 50_000
INTERVAL_COMPLETED_STEPS = 52_224
WARMUP_INTERVALS = 5
CHECKPOINT_EVALUATION_GAMES = 1_000


@dataclass(frozen=True)
class WinnerHealthCampaignConfig:
    campaign_id: str
    artifact_root: Path
    registry_path: Path
    source_config_path: Path
    binary: Path
    server_root: Path
    budget: ResourceBudget
    time_budget_seconds: float = 3_600.0
    maximum_intervals: int = 128
    seat_a_definition: str = "blade_warden"
    seat_b_definition: str = "blade_warden"
    require_full_window: bool = True
    expected_global_sha256: str = EXPECTED_GLOBAL_SHA256
    failure_streak_limit: int = 3
    warmup_intervals: int = WARMUP_INTERVALS
    global_confirmation_games: int = 3_000
    source_campaign_root: Path | None = None
    source_interval: int | None = None
    observation_schema: str = OBSERVATION_SCHEMA_V2
    observation_manifest: Path | None = None
    entity_width: int = 96
    decision_width: int = 432
    entity_depth: int = 2
    decision_depth: int = 2
    activation: str = "tanh"
    interval_requested_steps: int = INTERVAL_REQUESTED_STEPS
    ppo_learning_rate: float = CHECKPOINT_400_RECIPE.learning_rate
    ppo_epochs: int = CHECKPOINT_400_RECIPE.epochs
    ppo_clip_range: float = CHECKPOINT_400_RECIPE.clip_range
    ppo_target_kl: float | None = CHECKPOINT_400_RECIPE.target_kl
    ppo_batch_size: int = CHECKPOINT_400_RECIPE.batch_size
    ppo_max_grad_norm: float = 0.5
    deterministic_training_opponent: bool = True
    allow_continuation_recipe_change: bool = False


def _campaign_recipe(config: WinnerHealthCampaignConfig):
    return replace(
        CHECKPOINT_400_RECIPE,
        learning_rate=config.ppo_learning_rate,
        epochs=config.ppo_epochs,
        clip_range=config.ppo_clip_range,
        target_kl=config.ppo_target_kl,
        batch_size=config.ppo_batch_size,
    )


def _interval_expected_steps(config: WinnerHealthCampaignConfig) -> int:
    rollout_batch = _campaign_recipe(config).rollout_steps * config.budget.workers
    if config.interval_requested_steps <= 0:
        raise ValueError("checkpoint interval must request positive PPO steps")
    if config.interval_requested_steps < rollout_batch:
        raise ValueError("checkpoint interval must contain at least one multi-worker rollout")
    return (
        (config.interval_requested_steps + rollout_batch - 1) // rollout_batch
    ) * rollout_batch


def prepare_winner_health_campaign(config: WinnerHealthCampaignConfig) -> dict[str, Any]:
    root = config.artifact_root.resolve()
    state_path = root / "campaign-state.json"
    if state_path.exists():
        raise FileExistsError(f"refusing to overwrite campaign state: {state_path}")
    root.mkdir(parents=True, exist_ok=True)
    registry = ChampionRegistry.load(config.registry_path.resolve())
    global_champion = registry.global_champion
    recipe = _campaign_recipe(config)
    if config.failure_streak_limit < 1:
        raise ValueError("failure streak limit must be positive")
    if config.warmup_intervals < 0:
        raise ValueError("warmup interval count cannot be negative")
    rollout_batch = recipe.rollout_steps * config.budget.workers
    if config.ppo_batch_size < 1 or rollout_batch % config.ppo_batch_size:
        raise ValueError("PPO batch size must evenly divide one multi-worker rollout")
    _interval_expected_steps(config)
    if config.ppo_learning_rate <= 0:
        raise ValueError("PPO learning rate must be positive")
    if config.ppo_epochs < 1:
        raise ValueError("PPO epochs must be positive")
    if not 0 < config.ppo_clip_range < 1:
        raise ValueError("PPO clip range must be between zero and one")
    if config.ppo_target_kl is not None and config.ppo_target_kl <= 0:
        raise ValueError("PPO target KL must be positive when enabled")
    if config.ppo_max_grad_norm <= 0:
        raise ValueError("PPO maximum gradient norm must be positive")
    if config.global_confirmation_games not in {0, 3_000}:
        raise ValueError("global confirmation games must be 0 or 3000")
    if (config.source_campaign_root is None) != (config.source_interval is None):
        raise ValueError("source campaign and source interval must be supplied together")
    if config.allow_continuation_recipe_change and config.source_campaign_root is None:
        raise ValueError("continuation recipe changes require a continuation checkpoint")
    if global_champion.checkpoint_sha256 != config.expected_global_sha256:
        raise RuntimeError("winner-health campaign global champion hash mismatch")
    if registry.checkpoint_champion.checkpoint_sha256 != config.expected_global_sha256:
        raise RuntimeError("checkpoint champion must match the frozen global champion")
    if sha256_file(config.source_config_path.resolve()) != SOURCE_CONFIG_SHA256:
        raise RuntimeError("checkpoint-480 source configuration hash mismatch")
    source = json.loads(config.source_config_path.read_text())
    source_training = source.get("training") or {}
    for key, expected in EXPECTED_SOURCE_TRAINING.items():
        if source_training.get(key) != expected:
            raise RuntimeError(f"checkpoint-480 source control changed: {key}")
    source_model = ProfiledMaskablePPO.load(global_champion.checkpoint_path, device="cpu")
    _validate_frozen_model(source_model, global_champion)
    manifest_v4 = None
    manifest_v5 = None
    if config.observation_schema == OBSERVATION_SCHEMA_V5:
        if config.observation_manifest is None:
            raise ValueError("fresh observation v5 campaign requires a frozen manifest")
        manifest_v5 = ObservationManifestV5.load(config.observation_manifest.resolve())
    elif config.observation_schema == OBSERVATION_SCHEMA_V4:
        if config.observation_manifest is None:
            raise ValueError("fresh observation v4 campaign requires a frozen manifest")
        manifest_v4 = ObservationManifestV4.load(config.observation_manifest.resolve())
    elif config.observation_schema != OBSERVATION_SCHEMA_V2:
        raise ValueError(f"unsupported winner-health observation schema {config.observation_schema!r}")

    seed_manifest = _write_seed_manifest(
        root / "seed-banks",
        config.maximum_intervals,
        global_confirmation_games=config.global_confirmation_games,
    )
    initial_seed = int(seed_manifest["initial_seed"])
    continuation = (
        _load_continuation_source(config, global_champion)
        if config.source_campaign_root is not None
        else None
    )
    controls = _frozen_controls(
        config,
        global_champion,
        source,
        seed_manifest,
        continuation=continuation,
    )
    atomic_json_write(root / "frozen-controls.json", controls)
    if continuation is not None:
        setup = {
            "schema": CAMPAIGN_SCHEMA,
            "status": "prepared",
            "mode": "checkpoint-continuation",
            "prepared_at": datetime.now(UTC).isoformat(),
            "initial_seed": initial_seed,
            "starting_checkpoint": continuation["starting_checkpoint"],
            "starting_baseline_score": continuation["starting_baseline_score"],
            "source_campaign": continuation["source_campaign"],
            "source_campaign_state_sha256": continuation["source_campaign_state_sha256"],
            "source_controls_sha256": continuation["source_controls_sha256"],
            "source_interval": continuation["source_interval"],
            "controls_sha256": controls["sha256"],
            "seed_manifest_sha256": seed_manifest["sha256"],
            "global_champion": asdict(global_champion),
        }
        atomic_json_write(root / "setup.json", setup)
        _progress(
            "continuation_setup_completed",
            source_interval=continuation["source_interval"],
            starting_checkpoint_sha256=continuation["starting_checkpoint"]["checkpoint_sha256"],
            starting_optimizer_sha256=continuation["starting_checkpoint"]["optimizer_state_sha256"],
            starting_baseline_score=continuation["starting_baseline_score"],
            global_sha256=global_champion.checkpoint_sha256,
            controls_sha256=controls["sha256"],
            seed_manifest_sha256=seed_manifest["sha256"],
        )
        return setup
    _progress(
        "fresh_setup_started",
        initial_seed=initial_seed,
        global_sha256=global_champion.checkpoint_sha256,
        reward=WINNER_HEALTH_REWARD,
    )
    set_reproducible_runtime(
        initial_seed,
        config.budget.learner_torch_threads,
        config.budget.learner_torch_interop_threads,
        config.budget.learner_blas_threads,
    )
    setup_dir = root / "fresh-setup"
    setup_dir.mkdir(parents=True, exist_ok=True)
    vec_env = _build_vec_env(config, global_champion, initial_seed, "fresh-setup")
    try:
        policy = (
            SparseRelationalCandidateMaskablePolicyV5
            if manifest_v5
            else SparseEntityCandidateMaskablePolicyV4
            if manifest_v4
            else SparseCandidateMaskablePolicyV2
        )
        model = ProfiledMaskablePPO(
            policy,
            vec_env,
            learning_rate=recipe.learning_rate,
            n_steps=recipe.rollout_steps,
            batch_size=recipe.batch_size,
            n_epochs=recipe.epochs,
            gamma=recipe.gamma,
            gae_lambda=recipe.gae_lambda,
            ent_coef=recipe.entropy_coefficient,
            clip_range=recipe.clip_range,
            target_kl=recipe.target_kl,
            vf_coef=recipe.value_loss_coefficient,
            max_grad_norm=config.ppo_max_grad_norm,
            verbose=0,
            seed=initial_seed,
            device="cpu",
            policy_kwargs=(
                {
                    "manifest_path": str(config.observation_manifest.resolve()),
                    "entity_width": config.entity_width,
                    "decision_width": config.decision_width,
                    "entity_depth": config.entity_depth,
                    "decision_depth": config.decision_depth,
                    "activation": config.activation,
                }
                if manifest_v4 or manifest_v5
                else None
            ),
        )
        initial_path = setup_dir / "initial.zip"
        atomic_model_save(model, initial_path)
        demonstrations = collect_demonstrations(
            binary=config.binary,
            server_root=config.server_root,
            seed=initial_seed,
            decisions=5_000,
            observation_schema=config.observation_schema,
            teacher="mechanics-v2",
            observation_manifest=config.observation_manifest,
            seat_definitions={
                "seat-a": config.seat_a_definition,
                "seat-b": config.seat_b_definition,
            },
        )
        imitation = behavior_clone(
            model,
            demonstrations,
            epochs=5,
            batch_size=CHECKPOINT_400_RECIPE.batch_size,
            seed=initial_seed,
        )
        behavior_cloned_path = setup_dir / "behavior-cloned.zip"
        atomic_model_save(model, behavior_cloned_path)
        parent = Champion.from_checkpoint(
            champion_id=f"{config.campaign_id}-fresh-teacher-warmstart",
            checkpoint=behavior_cloned_path,
            observation_schema=config.observation_schema,
            action_schema=(
                ACTION_SCHEMA_V5 if manifest_v5 else ACTION_SCHEMA_V4 if manifest_v4 else ACTION_SCHEMA_V2
            ),
            model_family="v5-relational-complete-view-winner-health-v2-fresh"
            if manifest_v5
            else "v4-complete-view-winner-health-v2-fresh"
            if manifest_v4
            else "v2-winner-health-v2-fresh",
            learner_steps=0,
            promoted_at=datetime.now(UTC).isoformat(),
            parent_experiment_id="fresh-random-initialization-mechanics-v2-teacher",
            optimizer_state_sha256=optimizer_hash(model),
        )
        if parent.checkpoint_sha256 in {entry.checkpoint_sha256 for entry in registry.opponent_entries()}:
            raise RuntimeError("fresh setup unexpectedly duplicated a preserved checkpoint")
        if _recipe_from_model(model) != recipe:
            raise RuntimeError("fresh model does not use the frozen campaign PPO recipe")
        setup = {
            "schema": CAMPAIGN_SCHEMA,
            "status": "prepared",
            "prepared_at": datetime.now(UTC).isoformat(),
            "initial_seed": initial_seed,
            "initial_checkpoint": str(initial_path.resolve()),
            "initial_checkpoint_sha256": sha256_file(initial_path),
            "behavior_cloned_checkpoint": asdict(parent),
            "imitation": imitation,
            "controls_sha256": controls["sha256"],
            "seed_manifest_sha256": seed_manifest["sha256"],
            "global_champion": asdict(global_champion),
        }
        atomic_json_write(root / "setup.json", setup)
        _progress(
            "fresh_setup_completed",
            initial_seed=initial_seed,
            initial_sha256=setup["initial_checkpoint_sha256"],
            behavior_cloned_sha256=parent.checkpoint_sha256,
            imitation=imitation,
        )
        return setup
    finally:
        vec_env.close()


def _load_continuation_source(
    config: WinnerHealthCampaignConfig,
    global_champion: Champion,
) -> dict[str, Any]:
    assert config.source_campaign_root is not None
    assert config.source_interval is not None
    source_root = config.source_campaign_root.resolve()
    source_state_path = source_root / "campaign-state.json"
    source_controls_path = source_root / "frozen-controls.json"
    source_state = json.loads(source_state_path.read_text())
    if source_state.get("status") != "completed":
        raise RuntimeError("continuation source campaign is not complete")
    matching = [
        item for item in source_state.get("attempts", []) if int(item["interval"]) == config.source_interval
    ]
    if len(matching) != 1:
        raise RuntimeError(f"source campaign does not contain exactly one interval {config.source_interval}")
    source_attempt = matching[0]
    source_gate = source_attempt.get("gate") or {}
    source_evaluation = source_attempt.get("evaluation") or {}
    if source_gate.get("decision") != "pass" or int(source_gate.get("games", 0)) != 1_000:
        raise RuntimeError("continuation must start from an accepted 1,000-game checkpoint")
    baseline_score = float(source_evaluation["adjusted_score"])
    if baseline_score != float(source_gate["adjusted_score"]):
        raise RuntimeError("continuation source evaluation and gate scores disagree")
    source_global = Champion(**source_state["fixed_global_champion"])
    source_global.verify()
    if source_global.checkpoint_sha256 != global_champion.checkpoint_sha256:
        raise RuntimeError("continuation source used a different frozen global champion")
    source_controls = json.loads(source_controls_path.read_text())
    declared_controls = source_controls.get("sha256", "")
    actual_controls = canonical_hash(
        {key: value for key, value in source_controls.items() if key != "sha256"}
    )
    if declared_controls != actual_controls:
        raise RuntimeError("continuation source controls hash mismatch")
    source_recipe = source_controls.get("ppo_recipe") or {}
    source_max_grad_norm = float(source_controls.get("ppo_max_grad_norm", 0.5))
    target_recipe = asdict(_campaign_recipe(config))
    if not config.allow_continuation_recipe_change and source_recipe != target_recipe:
        raise RuntimeError("continuation source PPO recipe changed")
    if source_controls.get("observation_schema") != config.observation_schema:
        raise RuntimeError("continuation source observation schema changed")
    expected_action_schema = (
        ACTION_SCHEMA_V5
        if config.observation_schema == OBSERVATION_SCHEMA_V5
        else ACTION_SCHEMA_V4
        if config.observation_schema == OBSERVATION_SCHEMA_V4
        else ACTION_SCHEMA_V2
    )
    if source_controls.get("action_schema") != expected_action_schema:
        raise RuntimeError("continuation source action schema changed")
    expected_manifest_sha256 = (
        sha256_file(config.observation_manifest.resolve()) if config.observation_manifest else ""
    )
    if source_controls.get("observation_manifest_sha256", "") != expected_manifest_sha256:
        raise RuntimeError("continuation source observation manifest changed")
    if source_controls.get("teacher_setup", {}).get("post_ppo_correction") is not False:
        raise RuntimeError("continuation source enabled post-PPO correction")
    reward_change = source_controls.get("owner_directed_differences", {}).get("reward", [])
    if not reward_change or reward_change[-1] != WINNER_HEALTH_REWARD:
        raise RuntimeError("continuation source used a different PPO reward")
    expected_opponent = f"model:{global_champion.checkpoint_path}"
    opponent_change = source_controls.get("owner_directed_differences", {}).get("ppo_opponents", [None, None])
    if opponent_change[-1] != [expected_opponent]:
        raise RuntimeError("continuation source did not use the frozen global as sole PPO opponent")
    parent = Champion(**source_attempt["challenger"])
    parent.verify()
    model = ProfiledMaskablePPO.load(parent.checkpoint_path, device="cpu")
    _validate_campaign_model(model, parent, config, validate_recipe=False)
    if asdict(_recipe_from_model(model)) != source_recipe:
        raise RuntimeError("continuation checkpoint does not use its source campaign PPO recipe")
    if float(model.max_grad_norm) != source_max_grad_norm:
        raise RuntimeError("continuation checkpoint maximum gradient norm differs from source controls")
    if optimizer_hash(model) != parent.optimizer_state_sha256:
        raise RuntimeError("continuation checkpoint optimizer hash mismatch")
    return {
        "starting_checkpoint": asdict(parent),
        "starting_baseline_score": baseline_score,
        "source_campaign": str(source_root),
        "source_campaign_state_sha256": sha256_file(source_state_path),
        "source_controls_sha256": sha256_file(source_controls_path),
        "source_interval": config.source_interval,
        "source_ppo_recipe": source_recipe,
        "target_ppo_recipe": target_recipe,
        "ppo_recipe_changes": {
            key: [source_recipe.get(key), value]
            for key, value in target_recipe.items()
            if source_recipe.get(key) != value
        },
        "source_ppo_max_grad_norm": source_max_grad_norm,
        "target_ppo_max_grad_norm": config.ppo_max_grad_norm,
    }


def run_winner_health_campaign(config: WinnerHealthCampaignConfig) -> dict[str, Any]:
    setup_path = config.artifact_root.resolve() / "setup.json"
    setup = (
        json.loads(setup_path.read_text()) if setup_path.exists() else prepare_winner_health_campaign(config)
    )
    state_path = config.artifact_root.resolve() / "campaign-state.json"
    controls = json.loads((config.artifact_root / "frozen-controls.json").read_text())
    declared_controls = controls.get("sha256", "")
    if canonical_hash({key: value for key, value in controls.items() if key != "sha256"}) != (
        declared_controls
    ):
        raise RuntimeError("winner-health frozen controls hash mismatch")
    if int(controls.get("failure_streak_limit", 0)) != config.failure_streak_limit:
        raise RuntimeError("winner-health failure streak control changed after preparation")
    if int(controls.get("warmup_intervals_without_evaluation", -1)) != config.warmup_intervals:
        raise RuntimeError("winner-health warmup control changed after preparation")
    if int(controls.get("global_confirmation_games", 3_000)) != (config.global_confirmation_games):
        raise RuntimeError("winner-health confirmation control changed after preparation")
    if controls.get("ppo_recipe") != asdict(_campaign_recipe(config)):
        raise RuntimeError("winner-health PPO recipe changed after preparation")
    if float(controls.get("ppo_max_grad_norm", 0.5)) != config.ppo_max_grad_norm:
        raise RuntimeError("winner-health maximum gradient norm changed after preparation")
    if int(controls.get("interval_expected_steps", 0)) != _interval_expected_steps(config):
        raise RuntimeError("winner-health checkpoint interval changed after preparation")
    if bool(controls.get("paired_incumbent_gate", False)):
        raise RuntimeError("paired incumbent gates are historical-only; use the cached 1k baseline")
    if bool(
        controls.get("owner_directed_differences", {}).get(
            "deterministic_training_opponent", False
        )
    ) != config.deterministic_training_opponent:
        raise RuntimeError("winner-health training opponent determinism changed after preparation")
    seeds = _load_seed_manifest(
        config.artifact_root / "seed-banks",
        config.maximum_intervals,
        global_confirmation_games=config.global_confirmation_games,
    )
    registry = ChampionRegistry.load(config.registry_path.resolve())
    if registry.global_champion.checkpoint_sha256 != config.expected_global_sha256:
        raise RuntimeError("winner-health global champion changed after preparation")
    if registry.checkpoint_champion.checkpoint_sha256 != config.expected_global_sha256:
        raise RuntimeError("winner-health checkpoint champion changed after preparation")
    if controls["global_champion"]["checkpoint_sha256"] != config.expected_global_sha256:
        raise RuntimeError("winner-health prepared global champion hash mismatch")
    if state_path.exists():
        state = json.loads(state_path.read_text())
        if (
            state.get("status") != "completed"
            or not state.get("objective_achieved")
            or state.get("window_fully_used")
        ):
            raise FileExistsError(
                f"campaign state is not eligible for full-window continuation: {state_path}"
            )
        deadline_wall = datetime.fromisoformat(str(state["deadline_at"]))
        remaining_seconds = max((deadline_wall - datetime.now(UTC)).total_seconds(), 0.0)
        timer = CampaignTimer(remaining_seconds)
        timer.start()
        fixed_global = Champion(**state["fixed_global_champion"])
        fixed_global.verify()
        parent = Champion(**state["current_parent"])
        parent.verify()
        passed = [item for item in state["attempts"] if item["outcome"] in {"baseline-established", "passed"}]
        if not passed:
            raise RuntimeError("continued campaign has no accepted checkpoint")
        last_passed = Champion(**passed[-1]["challenger"])
        baseline_score = float(state["baseline_score"])
        failure_streak = int(state["failure_streak"])
        objective_achieved = True
        first_interval = len(state["attempts"]) + 1
        state["status"] = "running"
        state.setdefault("resume_events", []).append(
            {
                "resumed_at": datetime.now(UTC).isoformat(),
                "reason": "continue through the owner-required original one-hour deadline",
                "remaining_seconds": remaining_seconds,
                "first_interval": first_interval,
            }
        )
        state["timer"] = timer.snapshot()
        atomic_json_write(state_path, state)
        _progress(
            "winner_health_campaign_resumed",
            first_interval=first_interval,
            parent_sha256=parent.checkpoint_sha256,
            baseline_score=baseline_score,
            original_deadline_at=state["deadline_at"],
            remaining_seconds=remaining_seconds,
            timer=timer.snapshot(),
        )
    else:
        fixed_global = registry.global_champion
        continuation_mode = setup.get("mode") == "checkpoint-continuation"
        if continuation_mode:
            parent = Champion(**setup["starting_checkpoint"])
            last_passed: Champion | None = parent
            baseline_score: float | None = float(setup["starting_baseline_score"])
            first_interval = int(setup["source_interval"]) + 1
        else:
            parent = Champion(**setup["behavior_cloned_checkpoint"])
            last_passed = None
            baseline_score = None
            first_interval = 1
        failure_streak = 0
        started_wall = datetime.now(UTC)
        timer = CampaignTimer(config.time_budget_seconds)
        timer.start()
        objective_achieved = False
        state = {
            "schema": CAMPAIGN_SCHEMA,
            "status": "running",
            "campaign_id": config.campaign_id,
            "started_at": started_wall.isoformat(),
            "deadline_at": (started_wall + timedelta(seconds=config.time_budget_seconds)).isoformat(),
            "time_budget_seconds": config.time_budget_seconds,
            "fixed_global_champion": asdict(fixed_global),
            "campaign_setup": setup,
            "fresh_setup": None if continuation_mode else setup,
            "source_campaign": setup.get("source_campaign"),
            "source_interval": setup.get("source_interval"),
            "starting_checkpoint": asdict(parent),
            "starting_baseline_score": baseline_score,
            "controls_sha256": controls["sha256"],
            "seed_manifest_sha256": seeds["sha256"],
            "attempts": [],
            "passes": 0,
            "failures": 0,
            "rollbacks": 0,
            "deadline_grace": [],
            "host_before": capture_host_state(),
            "timer": timer.snapshot(),
        }
        atomic_json_write(state_path, state)
        _progress(
            (
                "winner_health_continuation_campaign_started"
                if continuation_mode
                else "winner_health_campaign_started"
            ),
            campaign_id=config.campaign_id,
            started_at=state["started_at"],
            deadline_at=state["deadline_at"],
            campaign_seed=setup["initial_seed"],
            source_interval=setup.get("source_interval"),
            starting_parent_sha256=parent.checkpoint_sha256,
            starting_parent_steps=parent.learner_steps,
            starting_baseline_score=baseline_score,
            fixed_global_sha256=fixed_global.checkpoint_sha256,
            timer=timer.snapshot(),
        )
    if first_interval > config.maximum_intervals:
        raise RuntimeError("winner-health maximum interval does not allow another checkpoint")
    try:
        for interval in range(first_interval, config.maximum_intervals + 1):
            if not timer.may_start_interval():
                break
            entry = seeds["intervals"][interval - 1]
            interval_dir = config.artifact_root / "intervals" / f"interval-{interval:03d}"
            interval_dir.mkdir(parents=True, exist_ok=True)
            training_seed = int(entry["training_seed"])
            _progress(
                "interval_started",
                interval=interval,
                parent_sha256=parent.checkpoint_sha256,
                parent_steps=parent.learner_steps,
                training_seed=training_seed,
                failure_streak=failure_streak,
                timer=timer.snapshot(),
            )
            training = _train_interval(config, fixed_global, parent, training_seed, interval_dir)
            challenger = training.pop("challenger")
            crossed_after_training = timer.deadline_crossed()
            _progress(
                "checkpoint_saved",
                interval=interval,
                learner_steps=challenger.learner_steps,
                checkpoint_sha256=challenger.checkpoint_sha256,
                training_seconds=training["elapsed_seconds"],
                timer=timer.snapshot(),
            )
            evaluation: dict[str, Any] | None = None
            incumbent_evaluation: dict[str, Any] | None = None
            gate: dict[str, Any] | None = None
            outcome = "warmup-no-evaluation"
            next_parent = challenger
            evaluation_deferred = len(state["attempts"]) < config.warmup_intervals
            if not evaluation_deferred:
                evaluation = _evaluate(
                    config,
                    challenger,
                    fixed_global,
                    Path(entry["global_1k"]),
                    CHECKPOINT_EVALUATION_GAMES,
                    interval_dir / "global-1k",
                )
                comparison_baseline = baseline_score
                decision = global_progression_block(
                    wins=evaluation["wins"],
                    draws=evaluation["draws"],
                    losses=evaluation["losses"],
                    baseline_score=comparison_baseline,
                    safety=evaluation["safety"],
                )
                gate = _gate_json(decision)
                gate["baseline_score_before"] = comparison_baseline
                gate["historical_best_score_before"] = baseline_score
                gate["comparison_mode"] = "cached-incumbent-baseline"
                gate["incumbent"] = asdict(last_passed) if last_passed is not None else None
                gate["incumbent_evaluation"] = None
                gate["score_delta"] = (
                    None
                    if comparison_baseline is None
                    else decision.adjusted_score - comparison_baseline
                )
                confirmation: dict[str, Any] | None = None
                if (
                    config.global_confirmation_games == 3_000
                    and not objective_achieved
                    and decision.adjusted_score > 0.50
                    and not any(decision.safety.values())
                ):
                    confirmation_result = _evaluate(
                        config,
                        challenger,
                        fixed_global,
                        Path(entry["global_3k"]),
                        3_000,
                        interval_dir / "global-3k-confirmation",
                    )
                    confirmation_gate = global_confirmation_block(
                        wins=confirmation_result["wins"],
                        draws=confirmation_result["draws"],
                        losses=confirmation_result["losses"],
                        safety=confirmation_result["safety"],
                    )
                    confirmation = {
                        "evaluation": confirmation_result,
                        "gate": _gate_json(confirmation_gate),
                    }
                    if confirmation_gate.decision == "pass":
                        promoted = Champion(
                            **{
                                **asdict(challenger),
                                "promoted_at": datetime.now(UTC).isoformat(),
                                "parent_experiment_id": fixed_global.champion_id,
                            }
                        )
                        registry.promote_global(promoted)
                        registry.save(config.registry_path.resolve())
                        objective_achieved = True
                gate["global_confirmation"] = confirmation
                if decision.decision == "pass":
                    baseline_score = decision.adjusted_score
                    last_passed = challenger
                    failure_streak = 0
                    state["passes"] += 1
                    outcome = "baseline-established" if gate["baseline_score_before"] is None else "passed"
                else:
                    failure_streak += 1
                    state["failures"] += 1
                    outcome = "failed-continued"
                    if failure_streak >= config.failure_streak_limit:
                        if last_passed is None:
                            raise RuntimeError("failure limit was reached before a safe baseline existed")
                        next_parent = last_passed
                        failure_streak = 0
                        state["rollbacks"] += 1
                        outcome = f"failed-{config.failure_streak_limit}-rollback"
                gate["baseline_score_after"] = baseline_score
                gate["failure_streak_after"] = failure_streak
                _progress(
                    "global_1k_completed",
                    interval=interval,
                    wins=decision.wins,
                    draws=decision.draws,
                    losses=decision.losses,
                    adjusted_score=decision.adjusted_score,
                    score_delta=gate["score_delta"],
                    incumbent_wins=(incumbent_evaluation or {}).get("wins"),
                    incumbent_draws=(incumbent_evaluation or {}).get("draws"),
                    incumbent_losses=(incumbent_evaluation or {}).get("losses"),
                    incumbent_score=(incumbent_evaluation or {}).get("adjusted_score"),
                    decision=decision.decision,
                    outcome=outcome,
                    failure_streak=failure_streak,
                    rollback_parent_sha256=(
                        next_parent.checkpoint_sha256 if outcome.endswith("-rollback") else None
                    ),
                    confirmation=confirmation,
                    timer=timer.snapshot(),
                )
            result = {
                "interval": interval,
                "training_seed": training_seed,
                "started_from": asdict(parent),
                "challenger": asdict(challenger),
                "training": training,
                "evaluation_deferred": evaluation_deferred,
                "evaluation": evaluation,
                "incumbent_evaluation": incumbent_evaluation,
                "gate": gate,
                "outcome": outcome,
                "next_parent": asdict(next_parent),
                "timer": timer.snapshot(),
            }
            atomic_json_write(interval_dir / "interval-result.json", result)
            _append_history(config.artifact_root / "campaign-history.jsonl", result)
            state["attempts"].append(result)
            state["baseline_score"] = baseline_score
            state["failure_streak"] = failure_streak
            state["current_parent"] = asdict(next_parent)
            state["timer"] = timer.snapshot()
            atomic_json_write(state_path, state)
            write_campaign_statistics_html(config.artifact_root, state)
            parent = next_parent
            if timer.deadline_crossed():
                state["deadline_grace"].append(
                    {
                        "interval": interval,
                        "phase": "training" if crossed_after_training else "required_evaluation",
                        "observed_at": datetime.now(UTC).isoformat(),
                    }
                )
                break
        if config.require_full_window and not timer.deadline_crossed():
            raise RuntimeError("campaign stopped before the required configured deadline")
        finished_wall = datetime.now(UTC)
        state.update(
            {
                "status": "completed",
                "finished_at": finished_wall.isoformat(),
                "timer": timer.snapshot(),
                "wall_elapsed_seconds": (
                    finished_wall - datetime.fromisoformat(str(state["started_at"]))
                ).total_seconds(),
                "window_fully_used": finished_wall >= datetime.fromisoformat(str(state["deadline_at"])),
                "objective_achieved": objective_achieved,
                "final_global_champion": asdict(
                    ChampionRegistry.load(config.registry_path.resolve()).global_champion
                ),
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _write_report(config.artifact_root / "final-report.md", state)
        write_campaign_statistics_html(config.artifact_root, state)
        _progress(
            "winner_health_campaign_completed",
            intervals=len(state["attempts"]),
            passes=state["passes"],
            failures=state["failures"],
            rollbacks=state["rollbacks"],
            objective_achieved=objective_achieved,
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
        _progress("winner_health_campaign_failed", failure=state["failure"], timer=timer.snapshot())
        raise


def run_winner_health_plateau_continuation(
    config: WinnerHealthCampaignConfig,
    source_campaign_root: Path,
    source_interval: int = 38,
) -> dict[str, Any]:
    """Continue an accepted winner-health checkpoint until its first three-loss plateau."""
    root = config.artifact_root.resolve()
    state_path = root / "campaign-state.json"
    if state_path.exists():
        raise FileExistsError(f"refusing to overwrite plateau campaign state: {state_path}")
    root.mkdir(parents=True, exist_ok=True)
    source_root = source_campaign_root.resolve()
    source_state_path = source_root / "campaign-state.json"
    source_state = json.loads(source_state_path.read_text())
    if source_state.get("status") != "completed":
        raise RuntimeError("source winner-health campaign is not complete")
    matching = [item for item in source_state.get("attempts", []) if int(item["interval"]) == source_interval]
    if len(matching) != 1:
        raise RuntimeError(f"source campaign does not contain exactly one interval {source_interval}")
    source_attempt = matching[0]
    source_gate = source_attempt.get("gate") or {}
    source_evaluation = source_attempt.get("evaluation") or {}
    if source_gate.get("decision") != "pass":
        raise RuntimeError("plateau continuation must start from an accepted checkpoint")
    baseline_score = float(source_evaluation["adjusted_score"])
    parent = Champion(**source_attempt["challenger"])
    parent.verify()
    strongest = parent
    fixed_global = Champion(**source_state["fixed_global_champion"])
    fixed_global.verify()
    if fixed_global.checkpoint_sha256 != CP480_SHA256:
        raise RuntimeError("plateau continuation requires frozen raw checkpoint 480")
    if sha256_file(config.source_config_path.resolve()) != SOURCE_CONFIG_SHA256:
        raise RuntimeError("checkpoint-480 source configuration hash mismatch")
    source_controls_path = source_root / "frozen-controls.json"
    source_controls = json.loads(source_controls_path.read_text())
    if source_controls.get("ppo_recipe") != asdict(CHECKPOINT_400_RECIPE):
        raise RuntimeError("source campaign PPO recipe changed")
    if source_controls.get("teacher_setup", {}).get("post_ppo_correction") is not False:
        raise RuntimeError("source campaign enabled post-PPO correction")
    expected_opponent = f"model:{fixed_global.checkpoint_path}"
    if source_controls.get("owner_directed_differences", {}).get("ppo_opponents", [None, None])[1] != [
        expected_opponent
    ]:
        raise RuntimeError("source campaign did not freeze CP480 as its sole PPO opponent")
    model = ProfiledMaskablePPO.load(parent.checkpoint_path, device="cpu")
    _validate_frozen_model(model, parent)
    if _recipe_from_model(model) != CHECKPOINT_400_RECIPE:
        raise RuntimeError("source checkpoint does not use the checkpoint-480 PPO recipe")
    controls = {
        "schema": "dice-and-destiny-winner-health-plateau-controls-v1",
        "campaign_id": config.campaign_id,
        "source_campaign": str(source_root),
        "source_campaign_state_sha256": sha256_file(source_state_path),
        "source_controls_sha256": sha256_file(source_controls_path),
        "source_interval": source_interval,
        "starting_checkpoint": asdict(parent),
        "starting_adjusted_score": baseline_score,
        "fixed_evaluation_and_ppo_opponent": asdict(fixed_global),
        "ppo_recipe": asdict(CHECKPOINT_400_RECIPE),
        "reward": WINNER_HEALTH_REWARD,
        "interval_requested_steps": INTERVAL_REQUESTED_STEPS,
        "interval_expected_steps": INTERVAL_COMPLETED_STEPS,
        "evaluation_games": 1_000,
        "seat_swap": True,
        "pass_rule": "safe adjusted score strictly improves the strongest accepted score",
        "stop_rule": "stop after the first three consecutive failed intervals; do not rollback",
        "post_ppo_correction": False,
        "resource_budget": config.budget.as_dict(),
    }
    controls["sha256"] = canonical_hash(controls)
    atomic_json_write(root / "frozen-controls.json", controls)
    started = datetime.now(UTC)
    state: dict[str, Any] = {
        "schema": "dice-and-destiny-winner-health-plateau-campaign-v1",
        "status": "running",
        "campaign_id": config.campaign_id,
        "started_at": started.isoformat(),
        "source_campaign": str(source_root),
        "source_interval": source_interval,
        "fixed_global_champion": asdict(fixed_global),
        "starting_checkpoint": asdict(parent),
        "starting_score": baseline_score,
        "baseline_score": baseline_score,
        "strongest_checkpoint": asdict(strongest),
        "current_parent": asdict(parent),
        "failure_streak": 0,
        "passes": 0,
        "failures": 0,
        "attempts": [],
        "controls_sha256": controls["sha256"],
        "host_before": capture_host_state(),
    }
    atomic_json_write(state_path, state)
    _progress(
        "winner_health_plateau_started",
        campaign_id=config.campaign_id,
        source_interval=source_interval,
        parent_sha256=parent.checkpoint_sha256,
        parent_steps=parent.learner_steps,
        baseline_score=baseline_score,
        fixed_global_sha256=fixed_global.checkpoint_sha256,
    )
    failure_streak = 0
    try:
        while failure_streak < 3:
            interval = source_interval + len(state["attempts"]) + 1
            entry = _append_plateau_seed_entry(root / "seed-banks", interval)
            interval_dir = root / "intervals" / f"interval-{interval:03d}"
            interval_dir.mkdir(parents=True, exist_ok=True)
            training_seed = int(entry["training_seed"])
            _progress(
                "plateau_interval_started",
                interval=interval,
                parent_sha256=parent.checkpoint_sha256,
                parent_steps=parent.learner_steps,
                strongest_score=baseline_score,
                failure_streak=failure_streak,
                training_seed=training_seed,
            )
            training = _train_interval(config, fixed_global, parent, training_seed, interval_dir)
            challenger = training.pop("challenger")
            _progress(
                "plateau_checkpoint_saved",
                interval=interval,
                learner_steps=challenger.learner_steps,
                checkpoint_sha256=challenger.checkpoint_sha256,
                optimizer_sha256=challenger.optimizer_state_sha256,
                training_seconds=training["elapsed_seconds"],
            )
            evaluation = _evaluate(
                config,
                challenger,
                fixed_global,
                Path(entry["global_1k"]),
                1_000,
                interval_dir / "global-1k",
            )
            decision, next_score, next_streak, should_stop = _plateau_transition(
                baseline_score=baseline_score,
                wins=evaluation["wins"],
                draws=evaluation["draws"],
                losses=evaluation["losses"],
                safety=evaluation["safety"],
                failure_streak=failure_streak,
            )
            score_delta = decision.adjusted_score - baseline_score
            outcome = "passed" if decision.decision == "pass" else "failed-continued"
            if decision.decision == "pass":
                strongest = challenger
                state["passes"] += 1
            else:
                state["failures"] += 1
                if should_stop:
                    outcome = "failed-third-stop"
            result = {
                "interval": interval,
                "training_seed": training_seed,
                "started_from": asdict(parent),
                "challenger": asdict(challenger),
                "training": training,
                "evaluation": evaluation,
                "gate": {
                    **_gate_json(decision),
                    "baseline_score_before": baseline_score,
                    "baseline_score_after": next_score,
                    "score_delta": score_delta,
                    "failure_streak_after": next_streak,
                    "stop": should_stop,
                },
                "outcome": outcome,
            }
            atomic_json_write(interval_dir / "interval-result.json", result)
            _append_history(root / "campaign-history.jsonl", result)
            state["attempts"].append(result)
            baseline_score = next_score
            failure_streak = next_streak
            parent = challenger
            state.update(
                {
                    "baseline_score": baseline_score,
                    "failure_streak": failure_streak,
                    "strongest_checkpoint": asdict(strongest),
                    "current_parent": asdict(parent),
                }
            )
            atomic_json_write(state_path, state)
            write_campaign_statistics_html(root, state)
            _progress(
                "plateau_global_1k_completed",
                interval=interval,
                wins=decision.wins,
                draws=decision.draws,
                losses=decision.losses,
                adjusted_score=decision.adjusted_score,
                score_delta=score_delta,
                decision=decision.decision,
                outcome=outcome,
                strongest_score=baseline_score,
                failure_streak=failure_streak,
                stop=should_stop,
            )
        finished = datetime.now(UTC)
        state.update(
            {
                "status": "completed",
                "finished_at": finished.isoformat(),
                "wall_elapsed_seconds": (finished - started).total_seconds(),
                "stop_reason": "first three consecutive failed intervals",
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _write_plateau_report(root / "final-report.md", state)
        write_campaign_statistics_html(root, state)
        _progress(
            "winner_health_plateau_completed",
            intervals=len(state["attempts"]),
            passes=state["passes"],
            failures=state["failures"],
            strongest=state["strongest_checkpoint"],
            strongest_score=state["baseline_score"],
            wall_elapsed_seconds=state["wall_elapsed_seconds"],
        )
        return state
    except Exception as error:
        state.update(
            {
                "status": "failed",
                "finished_at": datetime.now(UTC).isoformat(),
                "failure": {"type": type(error).__name__, "message": str(error)},
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        if state["attempts"]:
            write_campaign_statistics_html(root, state)
        _progress("winner_health_plateau_failed", failure=state["failure"])
        raise


def _plateau_transition(
    *,
    baseline_score: float,
    wins: int,
    draws: int,
    losses: int,
    safety: dict[str, int] | None,
    failure_streak: int,
) -> tuple[Any, float, int, bool]:
    decision = global_progression_block(
        wins=wins,
        draws=draws,
        losses=losses,
        baseline_score=baseline_score,
        safety=safety,
    )
    if decision.decision == "pass":
        return decision, decision.adjusted_score, 0, False
    next_streak = failure_streak + 1
    return decision, baseline_score, next_streak, next_streak >= 3


def _append_plateau_seed_entry(root: Path, interval: int) -> dict[str, Any]:
    root.mkdir(parents=True, exist_ok=True)
    manifest_path = root / "manifest.json"
    if manifest_path.exists():
        manifest = json.loads(manifest_path.read_text())
        declared = manifest.pop("sha256", "")
        if manifest.get("schema") != "dice-and-destiny-winner-health-plateau-seeds-v1":
            raise RuntimeError("plateau seed manifest has the wrong schema")
        if canonical_hash(manifest) != declared:
            raise RuntimeError("plateau seed manifest hash mismatch")
    else:
        manifest = {
            "schema": "dice-and-destiny-winner-health-plateau-seeds-v1",
            "generated_at": datetime.now(UTC).isoformat(),
            "intervals": [],
        }
    if any(int(item["interval"]) == interval for item in manifest["intervals"]):
        raise RuntimeError(f"plateau interval {interval} already has declared seeds")
    rng = secrets.SystemRandom()
    used_training = {int(item["training_seed"]) for item in manifest["intervals"]}
    training_seed = rng.randrange(100_000, 2_000_000_000)
    while training_seed in used_training or training_seed == 22:
        training_seed = rng.randrange(100_000, 2_000_000_000)
    used_ranges = [
        (int(item["evaluation_seed_start"]), int(item["evaluation_seed_start"]) + 500)
        for item in manifest["intervals"]
    ]
    evaluation_start = rng.randrange(20_000_000_000, 80_000_000_000)
    while any(start <= evaluation_start + 499 and evaluation_start <= end - 1 for start, end in used_ranges):
        evaluation_start = rng.randrange(20_000_000_000, 80_000_000_000)
    bank = root / f"interval-{interval:03d}-global-1k.json"
    bank_value = write_seed_bank(
        bank,
        bank_id=f"winner-health-plateau-interval-{interval:03d}-global-1k",
        seeds=list(range(evaluation_start, evaluation_start + 500)),
        purpose="frozen checkpoint-480 continuation progression gate",
    )
    entry = {
        "interval": interval,
        "training_seed": training_seed,
        "evaluation_seed_start": evaluation_start,
        "global_1k": str(bank.resolve()),
        "global_1k_sha256": bank_value["sha256"],
    }
    manifest["intervals"].append(entry)
    manifest["sha256"] = canonical_hash(manifest)
    atomic_json_write(manifest_path, manifest)
    return entry


def _write_plateau_report(path: Path, state: dict[str, Any]) -> None:
    rows = []
    for item in state["attempts"]:
        gate = item["gate"]
        rows.append(
            f"| {item['interval']} | {item['challenger']['learner_steps']} | "
            f"{gate['wins']}/{gate['draws']}/{gate['losses']} | "
            f"{100 * gate['adjusted_score']:.2f}% | {100 * gate['score_delta']:+.2f} pp | "
            f"{item['outcome']} |"
        )
    strongest = state["strongest_checkpoint"]
    lines = [
        "# Winner-health CP38 plateau continuation",
        "",
        f"- Status: {state['status']}",
        f"- Started: {state['started_at']}",
        f"- Finished: {state['finished_at']}",
        f"- Wall seconds: {state['wall_elapsed_seconds']:.2f}",
        f"- Source checkpoint: interval {state['source_interval']}",
        f"- Starting score: {100 * state['starting_score']:.2f}%",
        f"- Attempts/passes/failures: {len(state['attempts'])}/{state['passes']}/{state['failures']}",
        f"- Stop reason: {state['stop_reason']}",
        f"- Strongest checkpoint: {strongest['champion_id']}",
        f"- Strongest PPO steps: {strongest['learner_steps']}",
        f"- Strongest score: {100 * state['baseline_score']:.2f}%",
        f"- Strongest checkpoint SHA-256: `{strongest['checkpoint_sha256']}`",
        f"- Strongest optimizer SHA-256: `{strongest['optimizer_state_sha256']}`",
        "- PPO/clipping metrics: [statistics.html](statistics.html)",
        "- Gameplay metrics: [gameplay-statistics.html](gameplay-statistics.html)",
        "- Structured checkpoint metrics: [checkpoint-metrics.json](checkpoint-metrics.json)",
        "",
        "| Interval | PPO steps | W/D/L vs CP480 | Score | Difference | Outcome |",
        "| ---: | ---: | ---: | ---: | ---: | --- |",
        *rows,
        "",
    ]
    path.write_text("\n".join(lines))


def _write_seed_manifest(
    root: Path,
    maximum_intervals: int,
    *,
    global_confirmation_games: int = 3_000,
) -> dict[str, Any]:
    if root.exists() and any(root.iterdir()):
        return _load_seed_manifest(
            root,
            maximum_intervals,
            global_confirmation_games=global_confirmation_games,
        )
    if global_confirmation_games not in {0, 3_000}:
        raise ValueError("global confirmation games must be 0 or 3000")
    root.mkdir(parents=True, exist_ok=True)
    rng = secrets.SystemRandom()
    initial_seed = rng.randrange(100_000, 2_000_000_000)
    while initial_seed == 22:
        initial_seed = rng.randrange(100_000, 2_000_000_000)
    training_seeds = rng.sample(range(100_000, 2_000_000_000), maximum_intervals)
    base = rng.randrange(10_000_000_000, 20_000_000_000 - maximum_intervals * 10_000)
    intervals: list[dict[str, Any]] = []
    for interval in range(1, maximum_intervals + 1):
        interval_base = base + interval * 10_000
        block_1 = root / f"interval-{interval:03d}-global-1k.json"
        one = write_seed_bank(
            block_1,
            bank_id=f"winner-health-interval-{interval:03d}-global-1k",
            seeds=list(range(interval_base, interval_base + 500)),
            purpose="fixed global-champion progression gate",
        )
        entry = {
            "interval": interval,
            "training_seed": training_seeds[interval - 1],
            "global_1k": str(block_1.resolve()),
            "global_1k_sha256": one["sha256"],
        }
        if global_confirmation_games == 3_000:
            block_2 = root / f"interval-{interval:03d}-global-3k.json"
            three = write_seed_bank(
                block_2,
                bank_id=f"winner-health-interval-{interval:03d}-global-3k",
                seeds=list(range(interval_base + 1_000, interval_base + 2_500)),
                purpose="fixed global-champion promotion confirmation",
            )
            entry.update(
                {
                    "global_3k": str(block_2.resolve()),
                    "global_3k_sha256": three["sha256"],
                }
            )
        intervals.append(entry)
    manifest = {
        "schema": SEED_SCHEMA,
        "generated_at": datetime.now(UTC).isoformat(),
        "initial_seed": initial_seed,
        "global_confirmation_games": global_confirmation_games,
        "intervals": intervals,
    }
    manifest["sha256"] = canonical_hash(manifest)
    atomic_json_write(root / "manifest.json", manifest)
    return manifest


def _load_seed_manifest(
    root: Path,
    maximum_intervals: int,
    *,
    global_confirmation_games: int = 3_000,
) -> dict[str, Any]:
    value = json.loads((root.resolve() / "manifest.json").read_text())
    declared = value.get("sha256", "")
    actual = canonical_hash({key: item for key, item in value.items() if key != "sha256"})
    if value.get("schema") != SEED_SCHEMA or declared != actual:
        raise RuntimeError("winner-health seed manifest is invalid")
    declared_confirmation_games = int(value.get("global_confirmation_games", 3_000))
    if declared_confirmation_games != global_confirmation_games:
        raise RuntimeError("winner-health seed manifest confirmation mode changed")
    intervals = value.get("intervals") or []
    if len(intervals) < maximum_intervals:
        raise RuntimeError("winner-health seed manifest is too short")
    if int(value["initial_seed"]) == 22:
        raise RuntimeError("winner-health family must not reuse checkpoint-480 seed 22")
    training = [int(item["training_seed"]) for item in intervals[:maximum_intervals]]
    if len(training) != len(set(training)):
        raise RuntimeError("winner-health training seeds are not unique")
    seen: set[int] = set()
    banks = [("global_1k", 1_000)]
    if global_confirmation_games == 3_000:
        banks.append(("global_3k", 3_000))
    for item in intervals[:maximum_intervals]:
        for key, games in banks:
            bank = Path(item[key])
            bank_seeds = read_seed_bank(bank, expected_games=games)
            if seen.intersection(bank_seeds):
                raise RuntimeError("winner-health evaluation banks overlap")
            seen.update(bank_seeds)
    return value


def _frozen_controls(
    config: WinnerHealthCampaignConfig,
    global_champion: Champion,
    source: dict[str, Any],
    seeds: dict[str, Any],
    *,
    continuation: dict[str, Any] | None = None,
) -> dict[str, Any]:
    controls = {
        "schema": CONTROLS_SCHEMA,
        "campaign_id": config.campaign_id,
        "checkpoint_480_source_config": str(config.source_config_path.resolve()),
        "checkpoint_480_source_config_sha256": sha256_file(config.source_config_path.resolve()),
        "global_champion": asdict(global_champion),
        "source_training": source["training"],
        "ppo_recipe": asdict(_campaign_recipe(config)),
        "ppo_max_grad_norm": config.ppo_max_grad_norm,
        "model_architecture": (
            f"V5 candidate-isolated relational scorer entity_width={config.entity_width} "
            f"decision_width={config.decision_width} "
            f"entity_depth={config.entity_depth} decision_depth={config.decision_depth} "
            f"activation={config.activation}"
            if config.observation_schema == OBSERVATION_SCHEMA_V5
            else "checkpoint-480 width/depth recipe with V4 typed detail-bank input"
            if config.observation_schema == OBSERVATION_SCHEMA_V4
            else source["model_architecture"]
        ),
        "observation_schema": config.observation_schema,
        "action_schema": ACTION_SCHEMA_V5
        if config.observation_schema == OBSERVATION_SCHEMA_V5
        else ACTION_SCHEMA_V4
        if config.observation_schema == OBSERVATION_SCHEMA_V4
        else ACTION_SCHEMA_V2,
        "observation_manifest": (
            str(config.observation_manifest.resolve()) if config.observation_manifest else ""
        ),
        "observation_manifest_sha256": (
            sha256_file(config.observation_manifest.resolve()) if config.observation_manifest else ""
        ),
        "teacher_setup": {
            "teacher": "mechanics-v2",
            "decisions": 5_000,
            "epochs": 5,
            "post_ppo_correction": False,
            "performed_this_campaign": continuation is None,
        },
        "owner_directed_differences": {
            "family_seed": [22, int(seeds["initial_seed"])],
            "reward": [source["reward"], WINNER_HEALTH_REWARD],
            "ppo_opponents": [source["opponent_pool"], [f"model:{global_champion.checkpoint_path}"]],
            "deterministic_training_opponent": config.deterministic_training_opponent,
            "continuation_recipe_changes": (
                (continuation or {}).get("ppo_recipe_changes", {})
            ),
            "continuation_max_grad_norm_change": (
                [
                    (continuation or {}).get("source_ppo_max_grad_norm"),
                    (continuation or {}).get("target_ppo_max_grad_norm"),
                ]
                if continuation
                and (continuation or {}).get("source_ppo_max_grad_norm")
                != (continuation or {}).get("target_ppo_max_grad_norm")
                else []
            ),
            "representation_corrections": (
                [
                    "candidate-isolated learned detail encoding",
                    "typed learned mean/max detail pooling",
                    "semantic candidate identity without transient runtime identifiers",
                    f"relational policy capacity entity_width={config.entity_width} "
                    f"decision_width={config.decision_width} entity_depth={config.entity_depth} "
                    f"decision_depth={config.decision_depth}",
                ]
                if config.observation_schema == OBSERVATION_SCHEMA_V5
                else [
                    "complete viewer-safe state and linked content mechanics",
                    "exact candidate payload mapping",
                    "linear keep-one-die action interface",
                    "V4 fail-loud content/capacity manifest",
                ]
                if config.observation_schema == OBSERVATION_SCHEMA_V4
                else []
            ),
        },
        "interval_requested_steps": config.interval_requested_steps,
        "interval_expected_steps": _interval_expected_steps(config),
        "starting_mode": "checkpoint-continuation" if continuation else "fresh-teacher-warmstart",
        "continuation": continuation,
        "warmup_intervals_without_evaluation": config.warmup_intervals,
        "first_evaluated_checkpoint": (
            int(continuation["source_interval"]) + 1 if continuation else config.warmup_intervals + 1
        ),
        "progression_rule": (
            "one fresh deterministic seat-swapped 1k for the challenger; compare its safe score "
            "against the accepted incumbent's cached score"
        ),
        "paired_incumbent_gate": False,
        "rollback_rule": (
            f"after {config.failure_streak_limit} consecutive misses, reload last passed weights "
            "and optimizer"
        ),
        "failure_streak_limit": config.failure_streak_limit,
        "global_confirmation_games": config.global_confirmation_games,
        "promotion_rule": (
            "disabled; 1,000-game results are progression evidence only"
            if config.global_confirmation_games == 0
            else "1k score above 50% triggers fresh 3k; fresh 3k above 50% promotes"
        ),
        "resource_budget": config.budget.as_dict(),
    }
    controls["sha256"] = canonical_hash(controls)
    return controls


def _build_vec_env(
    config: WinnerHealthCampaignConfig,
    global_champion: Champion,
    training_seed: int,
    label: str,
) -> DummyVecEnv | SubprocVecEnv:
    opponent = f"model:{Path(global_champion.checkpoint_path).resolve()}"
    environments = []
    for worker in range(config.budget.workers):
        worker_seed = training_seed * 100 + worker
        environments.append(
            lambda worker_seed=worker_seed, worker=worker: AuthorityGymEnv(
                binary=config.binary,
                server_root=config.server_root,
                training_seed=worker_seed,
                opponent_specs=[opponent],
                learner_seat="alternate",
                max_episode_actions=1_200,
                device="cpu",
                session_id=f"winner-health-{config.campaign_id}-{label}-{worker}",
                authority_mode="ephemeral",
                telemetry_mode="training",
                transport_mode="full",
                observation_schema=config.observation_schema,
                observation_manifest=config.observation_manifest,
                instrumentation=True,
                torch_threads=config.budget.worker_torch_threads,
                torch_interop_threads=config.budget.worker_torch_interop_threads,
                blas_threads=config.budget.worker_blas_threads,
                opponent_selection_mode="legacy-flat",
                seat_definitions={
                    "seat-a": config.seat_a_definition,
                    "seat-b": config.seat_b_definition,
                },
                reward_definition=WINNER_HEALTH_REWARD,
                opponent_deterministic=config.deterministic_training_opponent,
            )
        )
    prior = {name: os.environ.get(name) for name in BLAS_THREAD_ENVIRONMENT}
    try:
        for name in BLAS_THREAD_ENVIRONMENT:
            os.environ[name] = str(config.budget.worker_blas_threads)
        return (
            DummyVecEnv(environments)
            if config.budget.workers == 1
            else SubprocVecEnv(environments, start_method="spawn")
        )
    finally:
        for name, value in prior.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value


def _validate_campaign_model(
    model: ProfiledMaskablePPO,
    champion: Champion,
    config: WinnerHealthCampaignConfig,
    *,
    validate_recipe: bool = True,
) -> None:
    if int(model.num_timesteps) != champion.learner_steps:
        raise RuntimeError("loaded campaign model learner steps do not match checkpoint metadata")
    if validate_recipe and _recipe_from_model(model) != _campaign_recipe(config):
        raise RuntimeError("campaign model changed the frozen campaign PPO recipe")
    expected_policy = (
        "SparseRelationalCandidateMaskablePolicyV5"
        if config.observation_schema == OBSERVATION_SCHEMA_V5
        else "SparseEntityCandidateMaskablePolicyV4"
        if config.observation_schema == OBSERVATION_SCHEMA_V4
        else "SparseCandidateMaskablePolicyV2"
    )
    if type(model.policy).__name__ != expected_policy:
        raise RuntimeError(
            f"campaign model policy is {type(model.policy).__name__}, expected {expected_policy}"
        )
    if validate_recipe and float(model.max_grad_norm) != config.ppo_max_grad_norm:
        raise RuntimeError("campaign model changed the frozen maximum gradient norm")
    if config.observation_schema == OBSERVATION_SCHEMA_V5:
        if config.observation_manifest is None:
            raise RuntimeError("campaign model has no V5 manifest")
        manifest_v5 = ObservationManifestV5.load(config.observation_manifest.resolve())
        if tuple(model.observation_space.shape or ()) != (manifest_v5.layout.observation_size,):
            raise RuntimeError("campaign model observation shape differs from frozen V5 manifest")
        if int(model.action_space.n) != manifest_v5.maximum_legal_candidates:
            raise RuntimeError("campaign model action capacity differs from frozen V5 manifest")
        if int(model.policy.entity_width) != config.entity_width:
            raise RuntimeError("campaign model V5 width changed")
        if int(model.policy.decision_width) != config.decision_width:
            raise RuntimeError("campaign model V5 decision width changed")
        if int(model.policy.entity_depth) != config.entity_depth:
            raise RuntimeError("campaign model V5 depth changed")
        if int(model.policy.decision_depth) != config.decision_depth:
            raise RuntimeError("campaign model V5 decision depth changed")
        if str(model.policy.entity_activation) != config.activation:
            raise RuntimeError("campaign model V5 activation changed")
    elif config.observation_schema == OBSERVATION_SCHEMA_V4:
        if config.observation_manifest is None:
            raise RuntimeError("campaign model has no V4 manifest")
        manifest = ObservationManifestV4.load(config.observation_manifest.resolve())
        if tuple(model.observation_space.shape or ()) != (manifest.layout.observation_size,):
            raise RuntimeError("campaign model observation shape differs from frozen V4 manifest")
        if int(model.action_space.n) != manifest.maximum_legal_candidates:
            raise RuntimeError("campaign model action capacity differs from frozen V4 manifest")
    else:
        _validate_frozen_model(model, champion)


def _train_interval(
    config: WinnerHealthCampaignConfig,
    global_champion: Champion,
    parent: Champion,
    training_seed: int,
    output: Path,
) -> dict[str, Any]:
    set_reproducible_runtime(
        training_seed,
        config.budget.learner_torch_threads,
        config.budget.learner_torch_interop_threads,
        config.budget.learner_blas_threads,
    )
    vec_env = _build_vec_env(config, global_champion, training_seed, output.name)
    try:
        model = ProfiledMaskablePPO.load(parent.checkpoint_path, env=vec_env, device="cpu")
        model.set_random_seed(training_seed)
        model.policy.enable_instrumentation(True)
        _validate_campaign_model(model, parent, config, validate_recipe=False)
        recipe_before = _recipe_from_model(model)
        recipe = _campaign_recipe(config)
        if recipe_before != recipe and not config.allow_continuation_recipe_change:
            raise RuntimeError("campaign parent does not use the frozen campaign PPO recipe")
        _apply_recipe(model, recipe)
        model.max_grad_norm = config.ppo_max_grad_norm
        _validate_campaign_model(model, parent, config)
        before_steps = int(model.num_timesteps)
        before_parameters = _parameter_snapshot(model)
        before_optimizer = optimizer_hash(model)
        before_parameter_hash = parameter_hash(model)
        callback = ArtifactCallback(output, checkpoint_interval=10**12, instrumentation=True)
        model.update_profile.clear()
        model.policy.profile.clear()
        started = time.perf_counter()
        model.learn(
            total_timesteps=config.interval_requested_steps,
            callback=callback,
            reset_num_timesteps=False,
            progress_bar=False,
            log_interval=1,
        )
        elapsed = time.perf_counter() - started
        checkpoint = output / "raw-ppo-challenger.zip"
        atomic_model_save(model, checkpoint)
        completed = int(model.num_timesteps) - before_steps
        expected_steps = _interval_expected_steps(config)
        if completed != expected_steps:
            raise RuntimeError(
                f"checkpoint completed {completed} PPO steps, expected {expected_steps}"
            )
        telemetry = _training_telemetry(
            model,
            callback,
            before_parameters,
            before_optimizer,
            elapsed,
            before_steps,
        )
        telemetry["recipe_before"] = asdict(recipe_before)
        telemetry["recipe_applied"] = asdict(recipe)
        telemetry["stability"] = _stability_telemetry(config, telemetry)
        opponents = Counter(telemetry["opponent_distribution"])
        if len(opponents) != 1 or sum(opponents.values()) != telemetry["completed_episodes"]:
            raise RuntimeError("training telemetry did not contain exactly one global opponent")
        challenger = Champion.from_checkpoint(
            champion_id=f"{config.campaign_id}-{output.name}",
            checkpoint=checkpoint,
            observation_schema=config.observation_schema,
            action_schema=ACTION_SCHEMA_V5
            if config.observation_schema == OBSERVATION_SCHEMA_V5
            else ACTION_SCHEMA_V4
            if config.observation_schema == OBSERVATION_SCHEMA_V4
            else ACTION_SCHEMA_V2,
            model_family="v5-relational-complete-view-winner-health-v2-fresh"
            if config.observation_schema == OBSERVATION_SCHEMA_V5
            else "v4-complete-view-winner-health-v2-fresh"
            if config.observation_schema == OBSERVATION_SCHEMA_V4
            else "v2-winner-health-v2-fresh",
            learner_steps=int(model.num_timesteps),
            promoted_at=datetime.now(UTC).isoformat(),
            parent_experiment_id=parent.champion_id,
            optimizer_state_sha256=optimizer_hash(model),
        )
        return {
            "challenger": challenger,
            "requested_steps": config.interval_requested_steps,
            "completed_steps": completed,
            "parent_steps": before_steps,
            "challenger_steps": challenger.learner_steps,
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


def _stability_telemetry(
    config: WinnerHealthCampaignConfig,
    telemetry: dict[str, Any],
) -> dict[str, Any]:
    recipe = _campaign_recipe(config)
    counters = ((telemetry.get("profile") or {}).get("counters") or {})

    def count(name: str) -> float:
        return float(counters.get(name, 0.0))

    rollout_size = recipe.rollout_steps * config.budget.workers
    rollouts = _interval_expected_steps(config) // rollout_size
    minibatches_per_epoch = rollout_size // recipe.batch_size
    planned_optimizer_updates = rollouts * recipe.epochs * minibatches_per_epoch
    optimizer_updates = int(count("ppo.optimizer_updates"))
    kl_samples = count("ppo.approx_kl_samples")
    gradient_samples = count("ppo.gradient_norm_samples")
    train_calls = int(count("ppo.train_calls"))
    early_stops = int(count("ppo.target_kl_early_stops"))
    return {
        "multiworker_rollouts": rollouts,
        "rollout_size": rollout_size,
        "planned_optimizer_updates": planned_optimizer_updates,
        "actual_optimizer_updates": optimizer_updates,
        "optimizer_update_utilization": (
            optimizer_updates / planned_optimizer_updates if planned_optimizer_updates else 0.0
        ),
        "target_kl": recipe.target_kl,
        "target_kl_early_stops": early_stops,
        "target_kl_early_stop_rate": early_stops / train_calls if train_calls else 0.0,
        "mean_approx_kl_all_minibatches": (
            count("ppo.approx_kl_sum") / kl_samples if kl_samples else 0.0
        ),
        "policy_ratio_clip_range": recipe.clip_range,
        "mean_policy_ratio_clip_fraction_all_minibatches": (
            count("ppo.clip_fraction_sum") / count("ppo.clip_fraction_samples")
            if count("ppo.clip_fraction_samples")
            else 0.0
        ),
        "gradient_norm_limit": config.ppo_max_grad_norm,
        "mean_preclip_gradient_norm_all_updates": (
            count("ppo.gradient_norm_sum") / gradient_samples if gradient_samples else 0.0
        ),
        "gradient_clip_events": int(count("ppo.gradient_clip_events")),
        "gradient_clip_event_rate": (
            count("ppo.gradient_clip_events") / gradient_samples if gradient_samples else 0.0
        ),
        "relative_parameter_l2_movement": float(
            telemetry.get("relative_parameter_l2_movement", 0.0)
        ),
        "final_rollout_entropy_loss": float((telemetry.get("ppo") or {}).get("entropy_loss", 0.0)),
        "final_rollout_explained_variance": float(
            (telemetry.get("ppo") or {}).get("explained_variance", 0.0)
        ),
    }


def _evaluate(
    config: WinnerHealthCampaignConfig,
    challenger: Champion,
    global_champion: Champion,
    bank: Path,
    games: int,
    output: Path,
) -> dict[str, Any]:
    summary = evaluate(
        binary=config.binary,
        server_root=config.server_root,
        seat_a_spec=f"model:{challenger.checkpoint_path}",
        seat_b_spec=f"model:{global_champion.checkpoint_path}",
        seeds=read_seed_bank(bank, expected_games=games),
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
    losses = games - wins - draws
    return {
        "wins": wins,
        "draws": draws,
        "losses": losses,
        "adjusted_score": (wins + 0.5 * draws) / games,
        "seed_bank": str(bank.resolve()),
        "seed_bank_sha256": sha256_file(bank),
        "summary": str((output / "summary.json").resolve()),
        "summary_sha256": sha256_file(output / "summary.json"),
        "episodes_sha256": sha256_file(output / "episodes.jsonl"),
        "elapsed_seconds": float(summary["elapsed_seconds"]),
        "games_per_second": float(summary["games_per_second"]),
        "safety": {field: int(summary.get(field, 0)) for field in SAFETY_FIELDS},
        "behavior": summary.get("behavior_by_policy", {}).get(spec, {}),
        "damage": summary.get("damage_by_policy", {}).get(spec, {}),
    }


def _gate_json(gate: Any) -> dict[str, Any]:
    return {**asdict(gate), "games": gate.games, "interval_95": list(gate.interval_95)}


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
        "schema": "dice-and-destiny-winner-health-history-v1",
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


def _write_report(path: Path, state: dict[str, Any]) -> None:
    rows = []
    for item in state["attempts"]:
        gate = item.get("gate")
        if gate is None:
            result = "deferred"
        else:
            result = (
                f"{gate['wins']}/{gate['draws']}/{gate['losses']} "
                f"({100 * gate['adjusted_score']:.2f}%, {item['outcome']})"
            )
            incumbent = item.get("incumbent_evaluation")
            if incumbent is not None:
                result += (
                    f" vs incumbent {incumbent['wins']}/{incumbent['draws']}/{incumbent['losses']} "
                    f"({100 * incumbent['adjusted_score']:.2f}%; "
                    f"delta {100 * gate['score_delta']:+.2f} pp)"
                )
        confirmation = (gate or {}).get("global_confirmation") or {}
        confirmation_gate = confirmation.get("gate")
        if confirmation_gate is None:
            confirmation = "—"
        else:
            confirmation = (
                f"{confirmation_gate['wins']}/{confirmation_gate['draws']}/"
                f"{confirmation_gate['losses']} "
                f"({100 * confirmation_gate['adjusted_score']:.4f}%, "
                f"{confirmation_gate['decision']})"
            )
        rows.append(
            f"| {item['interval']} | {item['challenger']['learner_steps']} | "
            f"`{item['challenger']['checkpoint_sha256']}` | "
            f"{item['training']['elapsed_seconds']:.2f} | {result} | {confirmation} |"
        )
    training_seconds = sum(float(item["training"]["elapsed_seconds"]) for item in state["attempts"])
    evaluation_seconds = sum(
        float((item.get("evaluation") or {}).get("elapsed_seconds", 0.0))
        + float((item.get("incumbent_evaluation") or {}).get("elapsed_seconds", 0.0))
        for item in state["attempts"]
    )
    confirmation_seconds = sum(
        float(
            (((item.get("gate") or {}).get("global_confirmation") or {}).get("evaluation") or {}).get(
                "elapsed_seconds", 0.0
            )
        )
        for item in state["attempts"]
    )
    global_label = state["fixed_global_champion"]["champion_id"]
    setup = state.get("campaign_setup") or state.get("fresh_setup") or {}
    if setup.get("mode") == "checkpoint-continuation":
        setup_line = (
            f"- Continued from checkpoint {setup['source_interval']}: "
            f"`{setup['starting_checkpoint']['checkpoint_sha256']}` at "
            f"{100 * setup['starting_baseline_score']:.2f}%"
        )
    else:
        setup_line = f"- Fresh seed: {setup.get('initial_seed')}"
    lines = [
        "# Winner-health timed campaign",
        "",
        f"- Status: {state['status']}",
        f"- Started: {state['started_at']}",
        f"- Finished: {state['finished_at']}",
        f"- Time budget seconds: {state['time_budget_seconds']:.2f}",
        setup_line,
        f"- Intervals: {len(state['attempts'])}",
        f"- Passes/failures/rollbacks: {state['passes']}/{state['failures']}/{state['rollbacks']}",
        f"- Training seconds: {training_seconds:.2f}",
        f"- 1,000-game evaluation seconds: {evaluation_seconds:.2f}",
        f"- 3,000-game confirmation seconds: {confirmation_seconds:.2f}",
        f"- Fixed global opponent: {global_label}",
        f"- Objective achieved: {state['objective_achieved']}",
        "- PPO/clipping metrics: [statistics.html](statistics.html)",
        "- Gameplay metrics: [gameplay-statistics.html](gameplay-statistics.html)",
        "- Structured checkpoint metrics: [checkpoint-metrics.json](checkpoint-metrics.json)",
        "",
        "| Interval | PPO steps | Checkpoint SHA-256 | Training seconds | "
        "1,000-game global result | 3,000-game confirmation |",
        "| ---: | ---: | --- | ---: | --- | --- |",
        *rows,
        "",
    ]
    path.write_text("\n".join(lines))


def _progress(event: str, **values: Any) -> None:
    print(json.dumps({"campaign_progress": event, **values}, sort_keys=True), flush=True)
