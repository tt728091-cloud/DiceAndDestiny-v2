from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import time
from collections import Counter
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import numpy as np
import torch
from stable_baselines3.common.utils import get_schedule_fn
from stable_baselines3.common.vec_env import DummyVecEnv, SubprocVecEnv

from .champions import (
    SAFETY_FIELDS,
    CampaignTimer,
    Champion,
    ChampionRegistry,
    ExperimentLedger,
    GateResult,
    atomic_json_write,
    canonical_hash,
    global_block,
    global_confirmation_block,
    global_progression_block,
    read_seed_bank,
    sha256_file,
    write_seed_bank,
)
from .evaluation import evaluate
from .gym_env import AuthorityGymEnv
from .manifest_v3 import ACTION_SCHEMA_V3, OBSERVATION_SCHEMA_V3, ObservationManifestV3
from .profiled_ppo import ProfiledMaskablePPO
from .resources import (
    BLAS_THREAD_ENVIRONMENT,
    ResourceBudget,
    capture_host_state,
    configure_thread_runtime,
)
from .schema_v2 import ACTION_SCHEMA_V2, OBSERVATION_SCHEMA_V2
from .training import ArtifactCallback, atomic_model_save, parameter_hash

CURRENT_GLOBAL_CHAMPION_ID = (
    "v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only-interval-193"
)
CURRENT_GLOBAL_LEARNER_STEPS = 3_603_456
EXPECTED_GLOBAL_SHA256 = "804fff1caaf5ddf15e4c8714d9c2d0b2563681ddb6f61477e832e5afde93c010"


@dataclass(frozen=True)
class CampaignConfig:
    campaign_id: str
    seed: int
    artifact_root: Path
    manifest_path: Path
    registry_path: Path
    initial_checkpoint: Path
    seed_bank_root: Path
    budget: ResourceBudget
    interval_steps: int = 50_000
    time_budget_seconds: float = 10_800.0
    seat_a_definition: str = "blade_warden"
    seat_b_definition: str = "blade_warden"
    warm_start: str = "mechanics-v2"
    warm_start_evidence: str = ""
    maximum_attempts: int = 512
    family_plan_path: Path | None = None
    require_full_window: bool = False
    allow_battery_power: bool = False
    initial_selection_score: float = 0.0
    baseline_intervals: int = 5
    family_failure_streak: int = 3
    require_fresh_families: bool = True


@dataclass(frozen=True)
class ModelFamily:
    family_id: str
    seed: int
    initial_checkpoint: Path
    checkpoint_sha256: str
    optimizer_state_sha256: str
    entity_width: int
    entity_depth: int
    activation: str
    warm_start_evidence: str
    change_from_previous: dict[str, list[Any]]
    seed_source: str = "system-random"

    def architecture(self) -> dict[str, Any]:
        return {
            "entity_width": self.entity_width,
            "entity_depth": self.entity_depth,
            "activation": self.activation,
        }

    def model_family(self, manifest: ObservationManifestV3) -> str:
        identity = canonical_hash(
            {
                "manifest_sha256": manifest.manifest_sha256,
                "architecture": self.architecture(),
            }
        )
        return f"v3-{identity[:12]}"


@dataclass(frozen=True)
class Recipe:
    learning_rate: float = 3e-4
    epochs: int = 8
    clip_range: float = 0.2
    target_kl: float | None = None
    entropy_coefficient: float = 0.01
    batch_size: int = 256
    rollout_steps: int = 256
    gamma: float = 0.995
    gae_lambda: float = 0.95
    value_loss_coefficient: float = 0.5


CHECKPOINT_400_RECIPE = Recipe()
CHECKPOINT_400_IMITATION_DECISIONS = 5_000
CHECKPOINT_400_IMITATION_EPOCHS = 5
CHECKPOINT_400_IMITATION_TEACHER = "mechanics-v2"
CHECKPOINT_400_HISTORY_INTERVAL = 5_000
FIXED_RECIPE_HYPOTHESIS = "Unchanged checkpoint-400 settings improve the global-relative high watermark."


def load_model_family_plan(config: CampaignConfig) -> list[ModelFamily]:
    if config.family_plan_path is None:
        model = ProfiledMaskablePPO.load(config.initial_checkpoint.resolve(), device="cpu")
        return [
            ModelFamily(
                family_id="legacy-single-family",
                seed=config.seed,
                initial_checkpoint=config.initial_checkpoint.resolve(),
                checkpoint_sha256=sha256_file(config.initial_checkpoint.resolve()),
                optimizer_state_sha256=optimizer_hash(model),
                entity_width=int(model.policy.entity_width),
                entity_depth=int(model.policy.entity_depth),
                activation=str(model.policy.entity_activation),
                warm_start_evidence=config.warm_start_evidence,
                change_from_previous={},
                seed_source="legacy-configured",
            )
        ]
    path = config.family_plan_path.resolve()
    value = json.loads(path.read_text())
    if value.get("schema") != "dice-and-destiny-model-family-plan-v2":
        raise RuntimeError(f"unsupported model-family plan: {path}")
    declared = str(value.get("sha256", ""))
    actual = canonical_hash({key: item for key, item in value.items() if key != "sha256"})
    if declared != actual:
        raise RuntimeError("model-family plan hash mismatch")
    families = [
        ModelFamily(
            family_id=str(item["family_id"]),
            seed=int(item["seed"]),
            initial_checkpoint=Path(item["initial_checkpoint"]).resolve(),
            checkpoint_sha256=str(item["checkpoint_sha256"]),
            optimizer_state_sha256=str(item["optimizer_state_sha256"]),
            entity_width=int(item["entity_width"]),
            entity_depth=int(item["entity_depth"]),
            activation=str(item["activation"]),
            warm_start_evidence=str(item["warm_start_evidence"]),
            change_from_previous={
                str(key): list(change) for key, change in item.get("change_from_previous", {}).items()
            },
            seed_source=str(item.get("seed_source", "")),
        )
        for item in value.get("families") or []
    ]
    if not families:
        raise RuntimeError("model-family plan is empty")
    if len({family.family_id for family in families}) != len(families):
        raise RuntimeError("model-family IDs must be unique")
    if len({family.seed for family in families}) != len(families):
        raise RuntimeError("every model family must use a distinct training seed")
    if any(family.seed_source != "system-random" for family in families):
        raise RuntimeError("every model family must declare a system-random seed source")
    for index, family in enumerate(families):
        if family.activation not in {"tanh", "relu", "gelu"}:
            raise RuntimeError(f"unsupported family activation: {family.activation}")
        if family.entity_width < 1 or family.entity_depth < 1:
            raise RuntimeError("model-family width and depth must be positive")
        if index == 0:
            if family.change_from_previous:
                raise RuntimeError("the first model family cannot declare a predecessor change")
            continue
        previous = families[index - 1]
        expected = {
            key: [previous.architecture()[key], family.architecture()[key]]
            for key in previous.architecture()
            if previous.architecture()[key] != family.architecture()[key]
        }
        if family.change_from_previous != expected or len(expected) != 1:
            raise RuntimeError(
                f"family {family.family_id} must change exactly one new-run architecture control"
            )
    return families


def write_model_family_plan(path: Path, entries: list[dict[str, Any]]) -> dict[str, Any]:
    if not entries:
        raise ValueError("model-family plan requires at least one entry")
    families: list[dict[str, Any]] = []
    previous_architecture: dict[str, Any] | None = None
    for entry in entries:
        checkpoint = Path(entry["initial_checkpoint"]).resolve()
        evidence = Path(entry["warm_start_evidence"]).resolve()
        if not checkpoint.is_file() or not evidence.exists():
            raise FileNotFoundError(f"family checkpoint/evidence is missing: {entry['family_id']}")
        model = ProfiledMaskablePPO.load(checkpoint, device="cpu")
        architecture = {
            "entity_width": int(model.policy.entity_width),
            "entity_depth": int(model.policy.entity_depth),
            "activation": str(model.policy.entity_activation),
        }
        expected = {
            "entity_width": int(entry["entity_width"]),
            "entity_depth": int(entry["entity_depth"]),
            "activation": str(entry["activation"]),
        }
        if architecture != expected:
            raise RuntimeError(f"declared architecture differs from checkpoint: {entry['family_id']}")
        change = (
            {}
            if previous_architecture is None
            else {
                key: [previous_architecture[key], architecture[key]]
                for key in architecture
                if previous_architecture[key] != architecture[key]
            }
        )
        if previous_architecture is not None and len(change) != 1:
            raise RuntimeError(f"family {entry['family_id']} must change exactly one architecture control")
        families.append(
            {
                "family_id": str(entry["family_id"]),
                "seed": int(entry["seed"]),
                "initial_checkpoint": str(checkpoint),
                "checkpoint_sha256": sha256_file(checkpoint),
                "optimizer_state_sha256": optimizer_hash(model),
                **architecture,
                "warm_start_evidence": str(evidence),
                "change_from_previous": change,
                "seed_source": str(entry.get("seed_source", "system-random")),
            }
        )
        previous_architecture = architecture
    value: dict[str, Any] = {
        "schema": "dice-and-destiny-model-family-plan-v2",
        "generated_at": datetime.now(UTC).isoformat(),
        "families": families,
    }
    value["sha256"] = canonical_hash(value)
    atomic_json_write(path, value)
    return value


def _recipe_from_model(model: ProfiledMaskablePPO) -> Recipe:
    return Recipe(
        learning_rate=float(model.learning_rate),
        epochs=int(model.n_epochs),
        clip_range=float(model.clip_range(1.0)),
        target_kl=model.target_kl,
        entropy_coefficient=float(model.ent_coef),
        batch_size=int(model.batch_size),
        rollout_steps=int(model.n_steps),
        gamma=float(model.gamma),
        gae_lambda=float(model.gae_lambda),
        value_loss_coefficient=float(model.vf_coef),
    )


def _next_family_after_plateau(
    current: int,
    total: int,
    *,
    may_start_interval: bool,
    require_full_window: bool,
) -> int | None:
    if not may_start_interval:
        return None
    if current + 1 >= total:
        if require_full_window:
            raise RuntimeError("model-family plan exhausted before the full window deadline")
        return None
    return current + 1


def _require_deadline_reached(*, required: bool, deadline_crossed: bool, reason: str) -> None:
    if required and not deadline_crossed:
        raise RuntimeError(f"required full-window campaign stopped early: {reason}")


def _family_evaluation_due(family_attempt: int, baseline_intervals: int = 5) -> bool:
    if family_attempt < 1 or baseline_intervals < 1:
        raise ValueError("family attempts and baseline interval counts must be positive")
    return family_attempt >= baseline_intervals


def _updated_high_watermark_miss_streak(current: int, decision: str) -> int:
    if current < 0:
        raise ValueError("high-watermark miss streak cannot be negative")
    if decision == "pass":
        return 0
    if decision == "fail":
        return current + 1
    raise ValueError(f"unknown checkpoint decision: {decision}")


def _validate_campaign_power(
    power: str,
    *,
    require_full_window: bool,
    allow_battery_power: bool,
) -> None:
    if require_full_window and "AC Power" not in power and not allow_battery_power:
        raise RuntimeError("required full-window campaign must start on AC power")


def initialize_campaign_registry(
    path: Path,
    *,
    global_checkpoint: Path,
    expected_global_sha256: str,
    checkpoint_champion: Path,
    manifest: ObservationManifestV3,
    baseline_evidence: str,
) -> ChampionRegistry:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite champion registry: {path}")
    if expected_global_sha256 != EXPECTED_GLOBAL_SHA256:
        raise RuntimeError("requested global champion is not the registered global anchor")
    if sha256_file(global_checkpoint) != EXPECTED_GLOBAL_SHA256:
        raise RuntimeError("current global champion SHA-256 mismatch")
    global_model = ProfiledMaskablePPO.load(global_checkpoint, device="cpu")
    checkpoint_model = ProfiledMaskablePPO.load(checkpoint_champion, device="cpu")
    now = datetime.now(UTC).isoformat()
    registry = ChampionRegistry(
        global_champion=Champion.from_checkpoint(
            champion_id=CURRENT_GLOBAL_CHAMPION_ID,
            checkpoint=global_checkpoint,
            observation_schema=OBSERVATION_SCHEMA_V2,
            action_schema=ACTION_SCHEMA_V2,
            model_family="v2-winner-health-v2-fresh",
            learner_steps=CURRENT_GLOBAL_LEARNER_STEPS,
            promoted_at=now,
            parent_experiment_id="v2-winner-health-fresh-20260805-1h-interval-038",
            optimizer_state_sha256=optimizer_hash(global_model),
        ),
        checkpoint_champion=Champion.from_checkpoint(
            champion_id="v3-warmstart-base",
            checkpoint=checkpoint_champion,
            observation_schema=OBSERVATION_SCHEMA_V3,
            action_schema=ACTION_SCHEMA_V3,
            model_family=f"v3-{manifest.manifest_sha256[:12]}",
            learner_steps=0,
            promoted_at=now,
            parent_experiment_id=baseline_evidence,
            optimizer_state_sha256=optimizer_hash(checkpoint_model),
        ),
    )
    registry.save(path)
    return registry


def write_campaign_seed_banks(
    root: Path,
    *,
    attempts: int = 512,
    seed_offset: int = 0,
) -> dict[str, Any]:
    """Predeclare mutually disjoint promotion banks before any result is seen."""

    root.mkdir(parents=True, exist_ok=True)
    used: set[int] = set()
    files: list[dict[str, Any]] = []

    def bank(name: str, count: int, start: int, purpose: str) -> None:
        seeds = list(range(start, start + count))
        if used.intersection(seeds):
            raise RuntimeError(f"predeclared seed ranges overlap at {name}")
        used.update(seeds)
        value = write_seed_bank(root / f"{name}.json", bank_id=name, seeds=seeds, purpose=purpose)
        files.append(
            {
                "path": str((root / f"{name}.json").resolve()),
                "bank_id": name,
                "hash": value["sha256"],
                "games": value["seat_swapped_games"],
            }
        )

    for attempt in range(1, attempts + 1):
        base = seed_offset + 210_000_000 + attempt * 10_000
        bank(
            f"attempt-{attempt:03d}-global-1k",
            500,
            base,
            "global-relative checkpoint progression",
        )
        bank(
            f"attempt-{attempt:03d}-global-confirmation-3k",
            1_500,
            base + 1_000,
            "global promotion confirmation",
        )
    bank(
        "final-global-comparison",
        500,
        seed_offset + 910_000_000,
        "final development comparison",
    )
    bank(
        "development-global-anchor",
        500,
        seed_offset + 810_000_000,
        "stable global trend anchor",
    )
    manifest = {
        "schema": "dice-and-destiny-campaign-seed-banks-v2",
        "generated_at": datetime.now(UTC).isoformat(),
        "attempts": attempts,
        "seed_offset": seed_offset,
        "distinct_seeds": len(used),
        "files": files,
    }
    manifest["sha256"] = canonical_hash(manifest)
    atomic_json_write(root / "manifest.json", manifest)
    return manifest


def preflight_campaign(config: CampaignConfig) -> dict[str, Any]:
    root = config.artifact_root.resolve()
    if (root / "campaign-state.json").exists():
        raise RuntimeError("timed campaign state already exists")
    manifest = ObservationManifestV3.load(config.manifest_path.resolve())
    registry = ChampionRegistry.load(config.registry_path.resolve())
    families = load_model_family_plan(config)
    if registry.global_champion.checkpoint_sha256 != EXPECTED_GLOBAL_SHA256:
        raise RuntimeError("registry global champion does not match the current registered anchor")
    if registry.checkpoint_champion.checkpoint_sha256 != families[0].checkpoint_sha256:
        raise RuntimeError("initial checkpoint does not match registry")
    if families[0].initial_checkpoint != config.initial_checkpoint.resolve():
        raise RuntimeError("the first family must use --initial-checkpoint")
    if config.require_full_window and len(families) < 32:
        raise RuntimeError("a required full-window campaign needs at least 32 predeclared families")
    if config.require_full_window and config.maximum_attempts < len(families) * 8:
        raise RuntimeError(
            "maximum attempts cannot cover five baseline intervals plus three misses per family"
        )
    if config.baseline_intervals != 5:
        raise RuntimeError("campaign families must establish their baseline after exactly five intervals")
    if config.family_failure_streak != 3:
        raise RuntimeError("campaign families must fail after exactly three consecutive misses")
    accepted_v1 = root.parents[1] / "runs" / "phase2-training" / "seed-11" / "checkpoints" / "final.zip"
    if not accepted_v1.is_file():
        raise RuntimeError("checkpoint-400 accepted-v1 training opponent is missing")
    seed_manifest_path = config.seed_bank_root.resolve() / "manifest.json"
    seed_manifest = json.loads(seed_manifest_path.read_text())
    declared = seed_manifest.pop("sha256", "")
    if declared != canonical_hash(seed_manifest):
        raise RuntimeError("campaign seed-bank manifest hash mismatch")
    if seed_manifest.get("schema") != "dice-and-destiny-campaign-seed-banks-v2":
        raise RuntimeError("campaign seed banks do not use the global-relative gate layout")
    if int(seed_manifest.get("attempts", 0)) < config.maximum_attempts:
        raise RuntimeError("campaign does not have enough predeclared attempt seed banks")
    required = {
        f"attempt-{attempt:03d}-{suffix}.json": games
        for attempt in range(1, config.maximum_attempts + 1)
        for suffix, games in (
            ("global-1k", 1_000),
            ("global-confirmation-3k", 3_000),
        )
    }
    required["final-global-comparison.json"] = 1_000
    required["development-global-anchor.json"] = 1_000
    for name, games in required.items():
        read_seed_bank(config.seed_bank_root.resolve() / name, expected_games=games)
    for family in families:
        if sha256_file(family.initial_checkpoint) != family.checkpoint_sha256:
            raise RuntimeError(f"family checkpoint hash mismatch: {family.family_id}")
        model = ProfiledMaskablePPO.load(family.initial_checkpoint, device="cpu")
        recipe = _recipe_from_model(model)
        if recipe != CHECKPOINT_400_RECIPE:
            raise RuntimeError(f"family does not use checkpoint-400 settings: {family.family_id}")
        if (
            model.policy.manifest.manifest_sha256 != manifest.manifest_sha256
            or int(model.policy.entity_width) != family.entity_width
            or int(model.policy.entity_depth) != family.entity_depth
            or str(model.policy.entity_activation) != family.activation
        ):
            raise RuntimeError(f"family architecture/manifest mismatch: {family.family_id}")
        if optimizer_hash(model) != family.optimizer_state_sha256:
            raise RuntimeError(f"family optimizer hash mismatch: {family.family_id}")
        if not Path(family.warm_start_evidence).resolve().exists():
            raise RuntimeError(f"family warm-start evidence is missing: {family.family_id}")
        if config.require_fresh_families:
            family_root = root / "preflight" / "families"
            if not family.initial_checkpoint.is_relative_to(family_root):
                raise RuntimeError(f"family checkpoint is not a fresh campaign artifact: {family.family_id}")
            if int(model.num_timesteps) != 0:
                raise RuntimeError(f"family checkpoint contains prior PPO steps: {family.family_id}")
            training_config_path = family.initial_checkpoint.parents[1] / "config.json"
            if not training_config_path.is_file():
                raise RuntimeError(f"family training config is missing: {family.family_id}")
            training = json.loads(training_config_path.read_text()).get("training") or {}
            expected_warm_start = {
                "seed": family.seed,
                "imitation_decisions": CHECKPOINT_400_IMITATION_DECISIONS,
                "imitation_epochs": CHECKPOINT_400_IMITATION_EPOCHS,
                "imitation_teacher": CHECKPOINT_400_IMITATION_TEACHER,
                "total_timesteps": 0,
                "post_ppo_correction": False,
            }
            if {key: training.get(key) for key in expected_warm_start} != expected_warm_start:
                raise RuntimeError(f"family warm-start provenance mismatch: {family.family_id}")
    if families[0].optimizer_state_sha256 != registry.checkpoint_champion.optimizer_state_sha256:
        raise RuntimeError("initial family optimizer hash does not match registry")
    if (
        config.budget.workers != 12
        or (
            config.budget.worker_torch_threads,
            config.budget.worker_torch_interop_threads,
            config.budget.worker_blas_threads,
        )
        != (1, 1, 1)
        or (
            config.budget.learner_torch_threads,
            config.budget.learner_torch_interop_threads,
            config.budget.learner_blas_threads,
        )
        != (4, 1, 4)
    ):
        raise RuntimeError(
            "resolved max resource profile differs from the declared 12/1/1/1 + 4/1/4 contract"
        )
    repository_root = root.parents[3]
    disk = shutil.disk_usage(root.parent)
    power = _git_output(["pmset", "-g", "batt"], repository_root)
    _validate_campaign_power(
        power,
        require_full_window=config.require_full_window,
        allow_battery_power=config.allow_battery_power,
    )
    result = {
        "passed": True,
        "campaign_id": config.campaign_id,
        "source_head": _git_output(["git", "rev-parse", "HEAD"], repository_root),
        "source_tree_sha256": _source_tree_hash(repository_root),
        "manifest_sha256": manifest.manifest_sha256,
        "content_sha256": manifest.content_sha256,
        "initial_checkpoint_sha256": registry.checkpoint_champion.checkpoint_sha256,
        "initial_optimizer_sha256": registry.checkpoint_champion.optimizer_state_sha256,
        "global_checkpoint_sha256": registry.global_champion.checkpoint_sha256,
        "seed_bank_manifest_sha256": declared,
        "predeclared_attempts": seed_manifest["attempts"],
        "model_family_plan": str(config.family_plan_path.resolve())
        if config.family_plan_path is not None
        else "legacy-single-family",
        "predeclared_model_families": len(families),
        "family_seeds": [family.seed for family in families],
        "require_full_window": config.require_full_window,
        "allow_battery_power": config.allow_battery_power,
        "resource_budget": config.budget.as_dict(),
        "disk": {"total": disk.total, "used": disk.used, "free": disk.free},
        "host": capture_host_state(),
        "power": power,
        "post_ppo_correction": False,
        "time_budget_seconds": config.time_budget_seconds,
        "checkpoint_evaluation": "1,000 games against global; strict family-score improvement",
        "global_promotion": "fresh 3,000 games against global with adjusted score > 50%",
        "baseline_intervals": config.baseline_intervals,
        "family_failure_streak": config.family_failure_streak,
        "fresh_random_family_seeds": config.require_fresh_families,
        "checkpoint_400_recipe": asdict(CHECKPOINT_400_RECIPE),
        "checkpoint_400_training_opponents": [
            "random",
            "mechanics-v2",
            f"model:{accepted_v1.resolve()}",
            "current-family-history",
        ],
    }
    output = root / "preflight" / "campaign-preflight.json"
    atomic_json_write(output, result)
    result["output"] = str(output)
    return result


def run_campaign(config: CampaignConfig) -> dict[str, Any]:
    """Use one timer across reversible seed runs and predeclared model-family restarts."""

    preflight_campaign(config)
    root = config.artifact_root.resolve()
    root.mkdir(parents=True, exist_ok=True)
    manifest = ObservationManifestV3.load(config.manifest_path.resolve())
    registry = ChampionRegistry.load(config.registry_path.resolve())
    families = load_model_family_plan(config)
    if config.interval_steps != 50_000:
        raise ValueError("the campaign checkpoint interval must remain 50,000 learner steps")

    ledger = ExperimentLedger(root / "history" / "experiments.jsonl")
    state_path = root / "campaign-state.json"
    if state_path.exists():
        raise RuntimeError(
            "campaign-state.json already exists; refusing to restart or double-spend a timed campaign"
        )
    repository_root = root.parents[3]
    head_revision = _git_output(["git", "rev-parse", "HEAD"], repository_root)
    source_tree_sha256 = _source_tree_hash(repository_root)
    source_revision = f"{head_revision}+tree:{source_tree_sha256}"

    configure_thread_runtime(
        torch_threads=config.budget.learner_torch_threads,
        torch_interop_threads=config.budget.learner_torch_interop_threads,
        blas_threads=config.budget.learner_blas_threads,
    )
    family_index = 0
    family = families[family_index]
    family_history_pool = root / "family-history" / family.family_id
    _seed_family_history(family_history_pool, family)
    vec_env = _build_vec_env(config, family, family_history_pool)
    model = ProfiledMaskablePPO.load(family.initial_checkpoint, env=vec_env, device="cpu")
    model.policy.enable_instrumentation(True)
    recipe = _recipe_from_model(model)
    if recipe != CHECKPOINT_400_RECIPE:
        raise RuntimeError("first family does not use checkpoint-400 training settings")
    _apply_recipe(model, recipe)
    current_parent = registry.checkpoint_champion
    best_checkpoint = current_parent
    best_selection_score = float(config.initial_selection_score)

    # Initialization above is deliberately outside the authoritative timer.
    timer = CampaignTimer(config.time_budget_seconds)
    timer.start()
    started_wall = datetime.now(UTC)
    deadline_wall = started_wall + timedelta(seconds=config.time_budget_seconds)
    state: dict[str, Any] = {
        "schema": "dice-and-destiny-champion-campaign-state-v4",
        "campaign_id": config.campaign_id,
        "status": "running",
        "started_at": started_wall.isoformat(),
        "deadline_at": deadline_wall.isoformat(),
        "time_budget_seconds": config.time_budget_seconds,
        "require_full_window": config.require_full_window,
        "allow_battery_power": config.allow_battery_power,
        "source_revision": source_revision,
        "source_tree_sha256": source_tree_sha256,
        "manifest_sha256": manifest.manifest_sha256,
        "global_champion_sha256": registry.global_champion.checkpoint_sha256,
        "best_checkpoint_sha256": best_checkpoint.checkpoint_sha256,
        "best_selection_score": best_selection_score,
        "family_baseline_score": None,
        "baseline_intervals": config.baseline_intervals,
        "family_failure_streak": config.family_failure_streak,
        "checkpoint_400_recipe": asdict(CHECKPOINT_400_RECIPE),
        "attempts": [],
        "family_runs": [
            {
                "family_index": 1,
                "family_id": family.family_id,
                "seed": family.seed,
                "architecture": family.architecture(),
                "initial_checkpoint_sha256": family.checkpoint_sha256,
                "seed_source": family.seed_source,
                "baseline_intervals_completed": 0,
                "started_at": started_wall.isoformat(),
            }
        ],
        "deadline_grace": [],
    }
    atomic_json_write(state_path, state)
    _progress(
        "campaign_started",
        started_at=started_wall.isoformat(),
        deadline_at=deadline_wall.isoformat(),
        checkpoint_champion=registry.checkpoint_champion.checkpoint_sha256,
        global_champion=registry.global_champion.checkpoint_sha256,
        family_id=family.family_id,
        family_seed=family.seed,
        predeclared_families=len(families),
        require_full_window=config.require_full_window,
        allow_battery_power=config.allow_battery_power,
    )
    ledger.append(
        {
            **_ledger_common(config, manifest, registry, recipe, source_revision, family),
            "event": "campaign_started",
            "experiment_id": config.campaign_id,
            "parent_experiment_id": registry.checkpoint_champion.parent_experiment_id,
            "hypothesis": "Use the full window across predeclared architecture/seed families.",
            "config_diff": {},
            "result": {
                "started_at": started_wall.isoformat(),
                "deadline_at": deadline_wall.isoformat(),
                "timer": timer.snapshot(),
                "host": capture_host_state(),
                "family_plan": [
                    {
                        "family_id": item.family_id,
                        "seed": item.seed,
                        "architecture": item.architecture(),
                        "change_from_previous": item.change_from_previous,
                    }
                    for item in families
                ],
            },
        }
    )

    attempt_index = 0
    family_attempt_index = 0
    consecutive_high_watermark_misses = 0
    family_plateaus = 0
    family_baseline_score: float | None = None
    history_checkpoint_step = 0
    deadline_crossed_in_flight = False
    try:
        while attempt_index < config.maximum_attempts and timer.may_start_interval():
            attempt_index += 1
            family_attempt_index += 1
            experiment_id = f"{config.campaign_id}-{family.family_id}-attempt-{family_attempt_index:03d}"
            attempt_dir = root / "attempts" / f"attempt-{attempt_index:03d}"
            attempt_dir.mkdir(parents=True, exist_ok=True)
            before_parameters = _parameter_snapshot(model)
            before_optimizer = optimizer_hash(model)
            before_steps = int(model.num_timesteps)
            diff: dict[str, list[Any]] = {}
            if recipe != CHECKPOINT_400_RECIPE:
                raise RuntimeError("within-family training settings changed")
            attempt_common = _ledger_common(config, manifest, registry, recipe, source_revision, family)
            ledger.append(
                {
                    **attempt_common,
                    "event": "experiment_started",
                    "experiment_id": experiment_id,
                    "parent_experiment_id": current_parent.champion_id,
                    "hypothesis": FIXED_RECIPE_HYPOTHESIS,
                    "config_diff": diff,
                    "model_family": family.model_family(manifest),
                    "family_id": family.family_id,
                    "family_seed": family.seed,
                    "result": {"learner_steps": before_steps, "timer": timer.snapshot()},
                }
            )
            _progress(
                "interval_started",
                attempt=attempt_index,
                family_attempt=family_attempt_index,
                family_id=family.family_id,
                family_seed=family.seed,
                parent=current_parent.checkpoint_sha256,
                recipe=asdict(recipe),
                timer=timer.snapshot(),
            )
            callback = ArtifactCallback(
                attempt_dir,
                checkpoint_interval=CHECKPOINT_400_HISTORY_INTERVAL,
                instrumentation=True,
                checkpoint_directory=family_history_pool,
                initial_checkpoint_step=history_checkpoint_step,
            )
            model.update_profile.clear()
            model.policy.profile.clear()
            train_started = time.perf_counter()
            model.learn(
                total_timesteps=config.interval_steps,
                callback=callback,
                reset_num_timesteps=False,
                progress_bar=False,
                log_interval=1,
            )
            history_checkpoint_step = callback.last_checkpoint
            train_elapsed = time.perf_counter() - train_started
            challenger_path = attempt_dir / "raw-ppo-challenger.zip"
            atomic_model_save(model, challenger_path)
            deadline_crossed_in_flight = timer.deadline_crossed()
            if deadline_crossed_in_flight:
                state["deadline_grace"].append(
                    {
                        "attempt": attempt_index,
                        "phase": "checkpoint_save",
                        "observed_at": datetime.now(UTC).isoformat(),
                    }
                )
            telemetry = _training_telemetry(
                model,
                callback,
                before_parameters,
                before_optimizer,
                train_elapsed,
                before_steps,
            )
            challenger = Champion.from_checkpoint(
                champion_id=experiment_id,
                checkpoint=challenger_path,
                observation_schema=OBSERVATION_SCHEMA_V3,
                action_schema=ACTION_SCHEMA_V3,
                model_family=family.model_family(manifest),
                learner_steps=int(model.num_timesteps),
                promoted_at=datetime.now(UTC).isoformat(),
                parent_experiment_id=current_parent.champion_id,
                optimizer_state_sha256=optimizer_hash(model),
            )
            _publish_family_history(
                family_history_pool,
                challenger_path,
                f"checkpoint-{family_attempt_index:03d}",
            )
            _progress(
                "interval_saved",
                attempt=attempt_index,
                family_id=family.family_id,
                learner_steps=challenger.learner_steps,
                checkpoint_sha256=challenger.checkpoint_sha256,
                elapsed_seconds=train_elapsed,
                timer=timer.snapshot(),
            )
            evaluation_due = _family_evaluation_due(family_attempt_index, config.baseline_intervals)
            gate: dict[str, Any] | None = None
            if evaluation_due:
                gate = _run_global_progression_gate(
                    config,
                    attempt_index,
                    challenger,
                    registry.global_champion,
                    family_baseline_score,
                    attempt_dir / "global-progression-gate",
                )
                _progress(
                    "global_progression_gate_completed",
                    attempt=attempt_index,
                    family_id=family.family_id,
                    checkpoint_decision=gate["checkpoint_decision"],
                    baseline_score_before=gate["baseline_score_before"],
                    score_delta=gate["score_delta"],
                    block_1=gate["block_1"],
                    global_confirmation=gate.get("global_confirmation"),
                    timer=timer.snapshot(),
                )
                deadline_crossed_in_flight = deadline_crossed_in_flight or timer.deadline_crossed()
            else:
                _progress(
                    "baseline_interval_accumulated",
                    attempt=attempt_index,
                    family_id=family.family_id,
                    baseline_intervals_completed=family_attempt_index,
                    baseline_intervals_required=config.baseline_intervals,
                    timer=timer.snapshot(),
                )
            result: dict[str, Any] = {
                "family_id": family.family_id,
                "family_seed": family.seed,
                "family_architecture": family.architecture(),
                "learner_steps": challenger.learner_steps,
                "interval_requested_steps": config.interval_steps,
                "interval_completed_steps": challenger.learner_steps - before_steps,
                "training_elapsed_seconds": train_elapsed,
                "checkpoint": asdict(challenger),
                "evaluation_deferred": not evaluation_due,
                "baseline_intervals_required": config.baseline_intervals,
                "global_progression_gate": gate,
                "telemetry": telemetry,
                "timer": timer.snapshot(),
                "post_ppo_correction": False,
            }
            family_plateaued = False
            event = "baseline_interval_completed"
            if not evaluation_due:
                current_parent = challenger
            elif gate is not None and gate["checkpoint_decision"] == "pass":
                registry.promote_checkpoint(challenger)
                registry.save(config.registry_path)
                block_1_score = float(gate["block_1"]["adjusted_score"])
                if block_1_score > best_selection_score:
                    best_checkpoint = challenger
                    best_selection_score = block_1_score
                if gate["global_promoted"]:
                    registry.promote_global(challenger)
                    registry.save(config.registry_path)
                    best_checkpoint = challenger
                    best_selection_score = 0.50
                    family_baseline_score = 0.50
                else:
                    family_baseline_score = block_1_score
                consecutive_high_watermark_misses = _updated_high_watermark_miss_streak(
                    consecutive_high_watermark_misses, "pass"
                )
                gate["baseline_score_after"] = family_baseline_score
                gate["consecutive_high_watermark_misses"] = 0
                _progress(
                    "checkpoint_accepted",
                    attempt=attempt_index,
                    family_id=family.family_id,
                    adjusted_score=block_1_score,
                    baseline_score_after=family_baseline_score,
                    global_confirmation_required=gate["global_confirmation_required"],
                    global_promoted=gate["global_promoted"],
                    timer=timer.snapshot(),
                )
                current_parent = challenger
                event = "global_promoted" if gate["global_promoted"] else "checkpoint_high_watermark"
            elif gate is not None:
                consecutive_high_watermark_misses = _updated_high_watermark_miss_streak(
                    consecutive_high_watermark_misses, "fail"
                )
                gate["baseline_score_after"] = family_baseline_score
                gate["consecutive_high_watermark_misses"] = consecutive_high_watermark_misses
                result["continued_without_rollback"] = True
                current_parent = challenger
                event = "checkpoint_below_high_watermark"
                if consecutive_high_watermark_misses >= config.family_failure_streak:
                    family_plateaued = True
                    family_plateaus += 1
                    _progress(
                        "family_failed",
                        family_id=family.family_id,
                        family_seed=family.seed,
                        high_watermark=family_baseline_score,
                        consecutive_misses=consecutive_high_watermark_misses,
                        timer=timer.snapshot(),
                    )
            result["consecutive_high_watermark_misses"] = consecutive_high_watermark_misses
            ledger.append(
                {
                    **(_ledger_common(config, manifest, registry, recipe, source_revision, family)),
                    "event": event,
                    "experiment_id": experiment_id,
                    "parent_experiment_id": challenger.parent_experiment_id,
                    "hypothesis": FIXED_RECIPE_HYPOTHESIS,
                    "config_diff": {},
                    "family_id": family.family_id,
                    "result": result,
                }
            )

            if timer.deadline_crossed() and not deadline_crossed_in_flight:
                deadline_crossed_in_flight = True
                state["deadline_grace"].append(
                    {
                        "attempt": attempt_index,
                        "phase": "required_evaluation",
                        "observed_at": datetime.now(UTC).isoformat(),
                    }
                )
            state["attempts"].append(
                {
                    "attempt": attempt_index,
                    "family_index": family_index + 1,
                    "family_attempt": family_attempt_index,
                    "family_id": family.family_id,
                    "experiment_id": experiment_id,
                    "parent": challenger.parent_experiment_id,
                    "challenger_sha256": challenger.checkpoint_sha256,
                    "result": result,
                }
            )
            state["checkpoint_champion_sha256"] = registry.checkpoint_champion.checkpoint_sha256
            state["global_champion_sha256"] = registry.global_champion.checkpoint_sha256
            state["best_checkpoint_sha256"] = best_checkpoint.checkpoint_sha256
            state["best_selection_score"] = best_selection_score
            state["family_baseline_score"] = family_baseline_score
            state["consecutive_high_watermark_misses"] = consecutive_high_watermark_misses
            state["family_plateaus"] = family_plateaus
            state["timer"] = timer.snapshot()
            state["family_runs"][-1].update(
                {
                    "baseline_intervals_completed": min(family_attempt_index, config.baseline_intervals),
                    "high_watermark_score": family_baseline_score,
                    "consecutive_high_watermark_misses": consecutive_high_watermark_misses,
                }
            )
            atomic_json_write(state_path, state)
            if deadline_crossed_in_flight:
                break
            if not family_plateaued:
                continue
            state["family_runs"][-1].update(
                {
                    "finished_at": datetime.now(UTC).isoformat(),
                    "stop_reason": "three-consecutive-high-watermark-misses",
                    "attempts": family_attempt_index,
                }
            )
            next_family_index = _next_family_after_plateau(
                family_index,
                len(families),
                may_start_interval=timer.may_start_interval(),
                require_full_window=config.require_full_window,
            )
            if next_family_index is None:
                break

            previous_family = family
            vec_env.close()
            family_index = next_family_index
            family = families[family_index]
            family_history_pool = root / "family-history" / family.family_id
            _seed_family_history(family_history_pool, family)
            vec_env = _build_vec_env(config, family, family_history_pool)
            model = ProfiledMaskablePPO.load(family.initial_checkpoint, env=vec_env, device="cpu")
            model.policy.enable_instrumentation(True)
            recipe = _recipe_from_model(model)
            if recipe != CHECKPOINT_400_RECIPE:
                raise RuntimeError("new family does not use checkpoint-400 training settings")
            _apply_recipe(model, recipe)
            base = Champion.from_checkpoint(
                champion_id=f"{config.campaign_id}-{family.family_id}-base",
                checkpoint=family.initial_checkpoint,
                observation_schema=OBSERVATION_SCHEMA_V3,
                action_schema=ACTION_SCHEMA_V3,
                model_family=family.model_family(manifest),
                learner_steps=int(model.num_timesteps),
                promoted_at=datetime.now(UTC).isoformat(),
                parent_experiment_id=previous_family.family_id,
                optimizer_state_sha256=family.optimizer_state_sha256,
            )
            current_parent = base
            consecutive_high_watermark_misses = 0
            family_attempt_index = 0
            family_baseline_score = None
            history_checkpoint_step = 0
            state["family_baseline_score"] = None
            transition = {
                "family_index": family_index + 1,
                "family_id": family.family_id,
                "seed": family.seed,
                "architecture": family.architecture(),
                "change_from_previous": family.change_from_previous,
                "initial_checkpoint_sha256": family.checkpoint_sha256,
                "seed_source": family.seed_source,
                "baseline_intervals_completed": 0,
                "started_at": datetime.now(UTC).isoformat(),
            }
            state["family_runs"].append(transition)
            atomic_json_write(state_path, state)
            ledger.append(
                {
                    **_ledger_common(config, manifest, registry, recipe, source_revision, family),
                    "event": "model_family_started",
                    "experiment_id": f"{config.campaign_id}-{family.family_id}",
                    "parent_experiment_id": previous_family.family_id,
                    "hypothesis": "A new-run architecture toggle may escape the prior seed-run plateau.",
                    "config_diff": family.change_from_previous,
                    "family_seed": family.seed,
                    "result": {"timer": timer.snapshot(), **transition},
                }
            )
            _progress("model_family_started", timer=timer.snapshot(), **transition)

        reason = (
            "maximum attempt count reached"
            if attempt_index >= config.maximum_attempts
            else "campaign loop ended before deadline"
        )
        _require_deadline_reached(
            required=config.require_full_window,
            deadline_crossed=timer.deadline_crossed(),
            reason=reason,
        )

        final_comparison = _ensure_final_global_comparison(
            config,
            best_checkpoint,
            registry.global_champion,
            root / "final-comparison",
        )
        finished_wall = datetime.now(UTC)
        state["family_runs"][-1].update(
            {
                "finished_at": finished_wall.isoformat(),
                "stop_reason": "deadline",
                "attempts": family_attempt_index,
            }
        )
        state.update(
            {
                "status": "completed",
                "finished_at": finished_wall.isoformat(),
                "timer": timer.snapshot(),
                "window_fully_used": timer.deadline_crossed(),
                "stopped_after_deadline_grace": deadline_crossed_in_flight,
                "final_comparison": final_comparison,
                "best_checkpoint": asdict(best_checkpoint),
                "checkpoint_champion": asdict(registry.checkpoint_champion),
                "global_champion": asdict(registry.global_champion),
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _progress(
            "campaign_completed",
            finished_at=finished_wall.isoformat(),
            best_checkpoint=best_checkpoint.checkpoint_sha256,
            global_champion=registry.global_champion.checkpoint_sha256,
            window_fully_used=state["window_fully_used"],
            stopped_after_deadline_grace=deadline_crossed_in_flight,
            timer=timer.snapshot(),
        )
        ledger.append(
            {
                **_ledger_common(config, manifest, registry, recipe, source_revision, family),
                "event": "campaign_completed",
                "experiment_id": config.campaign_id,
                "parent_experiment_id": best_checkpoint.champion_id,
                "hypothesis": "Campaign stop and final outcome.",
                "config_diff": {},
                "result": {
                    "learner_steps": best_checkpoint.learner_steps,
                    "decision": "new-global"
                    if registry.global_champion.checkpoint_sha256 != EXPECTED_GLOBAL_SHA256
                    else "global-held",
                    "state": str(state_path),
                    "timer": timer.snapshot(),
                    "window_fully_used": state["window_fully_used"],
                    "final_comparison": final_comparison,
                },
            }
        )
        return state
    except Exception as error:
        finished_wall = datetime.now(UTC)
        state.update(
            {
                "status": "failed",
                "finished_at": finished_wall.isoformat(),
                "timer": timer.snapshot(),
                "window_fully_used": False,
                "failure": {"type": type(error).__name__, "message": str(error)},
                "host_after": capture_host_state(),
            }
        )
        atomic_json_write(state_path, state)
        _progress(
            "campaign_failed_early",
            finished_at=finished_wall.isoformat(),
            error_type=type(error).__name__,
            error=str(error),
            timer=timer.snapshot(),
        )
        ledger.append(
            {
                **_ledger_common(config, manifest, registry, recipe, source_revision, family),
                "event": "campaign_failed_early",
                "experiment_id": config.campaign_id,
                "parent_experiment_id": current_parent.champion_id,
                "hypothesis": "An early stop is a debug failure, not a successful campaign completion.",
                "config_diff": {},
                "result": {"failure": state["failure"], "timer": timer.snapshot()},
            }
        )
        raise
    finally:
        vec_env.close()


def _publish_family_history(pool: Path, checkpoint: Path, label: str) -> Path:
    pool.mkdir(parents=True, exist_ok=True)
    source = checkpoint.resolve()
    destination = pool / f"{label}-{sha256_file(source)[:12]}.zip"
    if not destination.exists():
        destination.symlink_to(source)
    return destination


def _seed_family_history(pool: Path, family: ModelFamily) -> None:
    """Recreate checkpoint 400's initial/teacher-cloned pool for a fresh family."""

    checkpoint_directory = family.initial_checkpoint.parent
    initial = checkpoint_directory / "initial.zip"
    behavior_cloned = checkpoint_directory / "behavior-cloned.zip"
    if initial.is_file() and behavior_cloned.is_file():
        _publish_family_history(pool, initial, "initial")
        _publish_family_history(pool, behavior_cloned, "behavior-cloned")
        return
    _publish_family_history(pool, family.initial_checkpoint, "warmstart")


def _build_vec_env(
    config: CampaignConfig,
    family: ModelFamily,
    family_history_pool: Path,
) -> DummyVecEnv | SubprocVecEnv:
    budget = config.budget
    accepted_v1 = (
        config.artifact_root.resolve().parents[1]
        / "runs"
        / "phase2-training"
        / "seed-11"
        / "checkpoints"
        / "final.zip"
    )
    opponent_specs = [
        "random",
        "mechanics-v2",
        f"model:{accepted_v1}",
        f"pool:{family_history_pool.resolve()}",
    ]
    environments = []
    for worker in range(budget.workers):
        worker_seed = family.seed * 100 + worker
        environments.append(
            lambda worker_seed=worker_seed, worker=worker: AuthorityGymEnv(
                binary=config.artifact_root.resolve().parents[2] / "build" / "battle-ml-sim",
                server_root=config.artifact_root.resolve().parents[2],
                training_seed=worker_seed,
                opponent_specs=opponent_specs,
                learner_seat="alternate",
                max_episode_actions=1200,
                device="cpu",
                session_id=f"campaign-{family.family_id}-{family.seed}-{worker}",
                authority_mode="ephemeral",
                telemetry_mode="training",
                transport_mode="full",
                observation_schema=OBSERVATION_SCHEMA_V3,
                instrumentation=True,
                torch_threads=budget.worker_torch_threads,
                torch_interop_threads=budget.worker_torch_interop_threads,
                blas_threads=budget.worker_blas_threads,
                opponent_selection_mode="legacy-flat",
                observation_manifest=config.manifest_path.resolve(),
                seat_definitions={
                    "seat-a": config.seat_a_definition,
                    "seat-b": config.seat_b_definition,
                },
            )
        )
    prior = {name: os.environ.get(name) for name in BLAS_THREAD_ENVIRONMENT}
    try:
        for name in BLAS_THREAD_ENVIRONMENT:
            os.environ[name] = str(budget.worker_blas_threads)
        return (
            DummyVecEnv(environments)
            if budget.workers == 1
            else SubprocVecEnv(environments, start_method="spawn")
        )
    finally:
        for name, value in prior.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value


def _run_global_progression_gate(
    config: CampaignConfig,
    attempt: int,
    challenger: Champion,
    global_champion: Champion,
    baseline_score: float | None,
    output: Path,
) -> dict[str, Any]:
    first = _run_match_block(
        config,
        challenger,
        global_champion,
        config.seed_bank_root / f"attempt-{attempt:03d}-global-1k.json",
        1_000,
        output / "block-1",
    )
    decision = global_progression_block(**_gate_arguments(first), baseline_score=baseline_score)
    result: dict[str, Any] = {
        "opponent_global_champion_sha256": global_champion.checkpoint_sha256,
        "baseline_score_before": baseline_score,
        "baseline_kind": "new-family" if baseline_score is None else "accepted-checkpoint",
        "score_delta": None if baseline_score is None else decision.adjusted_score - baseline_score,
        "block_1": _gate_json(decision),
        "checkpoint_decision": decision.decision,
        "global_confirmation_required": False,
        "global_promoted": False,
    }
    if decision.decision == "pass" and decision.adjusted_score > 0.50:
        second = _run_match_block(
            config,
            challenger,
            global_champion,
            config.seed_bank_root / f"attempt-{attempt:03d}-global-confirmation-3k.json",
            3_000,
            output / "global-confirmation",
        )
        confirmation = global_confirmation_block(**_gate_arguments(second))
        result["global_confirmation_required"] = True
        result["global_confirmation"] = {
            "block_2": _gate_json(confirmation),
            "decision": confirmation.decision,
        }
        result["global_promoted"] = confirmation.decision == "pass"
    return result


def _run_match_block(
    config: CampaignConfig,
    challenger: Champion,
    opponent: Champion,
    bank: Path,
    games: int,
    output: Path,
) -> dict[str, Any]:
    seeds = read_seed_bank(bank, expected_games=games)
    summary = evaluate(
        binary=config.artifact_root.resolve().parents[2] / "build" / "battle-ml-sim",
        server_root=config.artifact_root.resolve().parents[2],
        seat_a_spec=f"model:{challenger.checkpoint_path}",
        seat_b_spec=f"model:{opponent.checkpoint_path}",
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
        observation_schema=OBSERVATION_SCHEMA_V3,
        observation_manifest=config.manifest_path.resolve(),
        seat_a_definition=config.seat_a_definition,
        seat_b_definition=config.seat_b_definition,
    )
    challenger_spec = f"model:{challenger.checkpoint_path}"
    policy = summary["policies"][challenger_spec]
    wins = int(policy["wins"])
    draws = int(summary["draws"])
    result = {
        "wins": wins,
        "draws": draws,
        "losses": games - wins - draws,
        "seed_bank": str(bank.resolve()),
        "seed_bank_sha256": sha256_file(bank),
        "summary": str((output / "summary.json").resolve()),
        "elapsed_seconds": summary["elapsed_seconds"],
        "games_per_second": summary["games_per_second"],
        "behavior": summary.get("behavior_by_policy", {}).get(challenger_spec, {}),
        "safety": {field: int(summary.get(field, 0)) for field in SAFETY_FIELDS},
    }
    result["adjusted_score"] = (wins + 0.5 * draws) / games
    return result


def _ensure_final_global_comparison(
    config: CampaignConfig,
    checkpoint: Champion,
    global_champion: Champion,
    output: Path,
) -> dict[str, Any]:
    if checkpoint.checkpoint_sha256 == global_champion.checkpoint_sha256:
        return {
            "decision": "checkpoint-is-global",
            "challenger_sha256": checkpoint.checkpoint_sha256,
            "global_sha256": global_champion.checkpoint_sha256,
        }
    block = _run_match_block(
        config,
        checkpoint,
        global_champion,
        config.seed_bank_root / "final-global-comparison.json",
        1_000,
        output,
    )
    return {**block, "decision": global_block(**_gate_arguments(block)).decision}


def _gate_arguments(result: dict[str, Any]) -> dict[str, Any]:
    return {
        "wins": int(result["wins"]),
        "draws": int(result["draws"]),
        "losses": int(result["losses"]),
        "safety": result.get("safety") or {},
    }


def _gate_json(result: GateResult) -> dict[str, Any]:
    return {
        **asdict(result),
        "games": result.games,
        "interval_95": list(result.interval_95),
    }


def _apply_recipe(model: ProfiledMaskablePPO, recipe: Recipe) -> None:
    model.learning_rate = recipe.learning_rate
    model.lr_schedule = get_schedule_fn(recipe.learning_rate)
    model.n_epochs = recipe.epochs
    model.clip_range = get_schedule_fn(recipe.clip_range)
    model.target_kl = recipe.target_kl
    model.ent_coef = recipe.entropy_coefficient
    model.batch_size = recipe.batch_size
    model.gamma = recipe.gamma
    model.gae_lambda = recipe.gae_lambda
    model.vf_coef = recipe.value_loss_coefficient
    model.rollout_buffer.gamma = recipe.gamma
    model.rollout_buffer.gae_lambda = recipe.gae_lambda


def _training_telemetry(
    model: ProfiledMaskablePPO,
    callback: ArtifactCallback,
    before_parameters: dict[str, torch.Tensor],
    before_optimizer: str,
    elapsed: float,
    before_steps: int,
) -> dict[str, Any]:
    logs = {
        key.removeprefix("train/"): _json_number(value)
        for key, value in model.logger.name_to_value.items()
        if key.startswith("train/")
    }
    movement_squared = 0.0
    base_squared = 0.0
    for name, value in model.policy.state_dict().items():
        current = value.detach().cpu().float()
        prior = before_parameters[name].float()
        movement_squared += float(torch.sum((current - prior) ** 2))
        base_squared += float(torch.sum(prior**2))
    episodes = []
    if callback.episode_file.exists():
        episodes = [json.loads(line) for line in callback.episode_file.read_text().splitlines()]
    seats = Counter(str(record.get("learner_seat", "")) for record in episodes)
    opponents = Counter(str(record.get("opponent", "")) for record in episodes)
    behavior: Counter[str] = Counter()
    safety = {field: 0 for field in SAFETY_FIELDS}
    for record in episodes:
        metrics = record.get("metrics") or {}
        behavior.update(metrics.get("action_frequency") or {})
        for field in safety:
            safety[field] += int(metrics.get(field, 0))
    completed = int(model.num_timesteps) - before_steps
    return {
        "ppo": logs,
        "parameter_l2_movement": movement_squared**0.5,
        "relative_parameter_l2_movement": ((movement_squared / base_squared) ** 0.5 if base_squared else 0.0),
        "parameter_hash": parameter_hash(model),
        "optimizer_hash_before": before_optimizer,
        "optimizer_hash_after": optimizer_hash(model),
        "completed_episodes": callback.completed_episodes,
        "learner_seat_distribution": dict(seats),
        "opponent_distribution": dict(opponents),
        "action_behavior": dict(behavior),
        "safety": safety,
        "completed_steps": completed,
        "elapsed_seconds": elapsed,
        "steps_per_second": completed / elapsed if elapsed else 0.0,
        "timing": {
            "collection_seconds": callback.collection_seconds,
            "ppo_update_seconds": callback.update_seconds,
            "artifact_io_seconds": callback.artifact_io_seconds,
        },
        "profile": model.update_profile.summary(),
    }


def optimizer_hash(model: ProfiledMaskablePPO) -> str:
    digest = hashlib.sha256()

    def update(value: Any) -> None:
        if isinstance(value, torch.Tensor):
            tensor = value.detach().cpu().contiguous()
            digest.update(str(tensor.dtype).encode())
            digest.update(str(tuple(tensor.shape)).encode())
            digest.update(tensor.numpy().tobytes())
        elif isinstance(value, dict):
            for key in sorted(value, key=lambda item: str(item)):
                digest.update(str(key).encode())
                update(value[key])
        elif isinstance(value, (list, tuple)):
            digest.update(str(len(value)).encode())
            for item in value:
                update(item)
        else:
            digest.update(repr(value).encode())

    update(model.policy.optimizer.state_dict())
    return digest.hexdigest()


def _parameter_snapshot(model: ProfiledMaskablePPO) -> dict[str, torch.Tensor]:
    return {name: value.detach().cpu().clone() for name, value in model.policy.state_dict().items()}


def _ledger_common(
    config: CampaignConfig,
    manifest: ObservationManifestV3,
    registry: ChampionRegistry,
    recipe: Recipe,
    source_revision: str,
    family: ModelFamily,
) -> dict[str, Any]:
    full_config = {
        "campaign": {
            **asdict(config),
            "artifact_root": str(config.artifact_root.resolve()),
            "manifest_path": str(config.manifest_path.resolve()),
            "registry_path": str(config.registry_path.resolve()),
            "initial_checkpoint": str(config.initial_checkpoint.resolve()),
            "seed_bank_root": str(config.seed_bank_root.resolve()),
            "family_plan_path": str(config.family_plan_path.resolve())
            if config.family_plan_path is not None
            else None,
            "budget": config.budget.as_dict(),
        },
        "recipe": asdict(recipe),
        "opponent_curriculum": {
            "mode": "checkpoint-400-legacy-flat",
            "categories": ["random", "mechanics-v2", "accepted-v1", "current-family-history"],
        },
        "post_ppo_correction": False,
    }
    return {
        "checkpoint_champion_sha256": registry.checkpoint_champion.checkpoint_sha256,
        "global_champion_sha256": registry.global_champion.checkpoint_sha256,
        "source_revision": source_revision,
        "content_sha256": manifest.content_sha256,
        "observation_schema_sha256": canonical_hash(
            {"schema": OBSERVATION_SCHEMA_V3, "manifest": manifest.manifest_sha256}
        ),
        "action_schema_sha256": canonical_hash(
            {"schema": ACTION_SCHEMA_V3, "capacity": manifest.maximum_legal_candidates}
        ),
        "architecture_sha256": canonical_hash({"family": "v3-sparse-entity", **family.architecture()}),
        "reward_sha256": canonical_hash("+1 victory, -1 defeat, 0 draw; no shaping"),
        "teacher_setup_sha256": canonical_hash(
            {
                "warm_start": config.warm_start,
                "evidence": config.warm_start_evidence,
                "post_ppo": False,
            }
        ),
        "opponent_curriculum_sha256": canonical_hash(
            {
                "mode": "checkpoint-400-legacy-flat",
                "categories": [
                    "random",
                    "mechanics-v2",
                    "accepted-v1",
                    "current-family-history",
                ],
            }
        ),
        "ppo_recipe_sha256": canonical_hash(asdict(recipe)),
        "config": full_config,
        "config_diff": {},
    }


def _json_number(value: Any) -> int | float | str:
    if isinstance(value, (np.integer, int)):
        return int(value)
    if isinstance(value, (np.floating, float)):
        return float(value)
    return str(value)


def _git_output(command: list[str], cwd: Path) -> str:
    return subprocess.run(command, cwd=cwd, check=True, capture_output=True, text=True).stdout.strip()


def _source_tree_hash(repository_root: Path) -> str:
    completed = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=repository_root,
        check=True,
        capture_output=True,
    )
    digest = hashlib.sha256()
    paths = sorted(path for path in completed.stdout.split(b"\0") if path)
    for encoded in paths:
        path = repository_root / os.fsdecode(encoded)
        if not path.is_file():
            continue
        digest.update(encoded)
        digest.update(b"\0")
        with path.open("rb") as handle:
            for block in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(block)
    return digest.hexdigest()


def _progress(event: str, **values: Any) -> None:
    print(json.dumps({"campaign_progress": event, **values}, sort_keys=True), flush=True)
