from __future__ import annotations

import hashlib
import importlib.metadata
import json
import os
import platform
import resource
import subprocess
import time
import uuid
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import torch
from sb3_contrib import MaskablePPO
from stable_baselines3.common.callbacks import BaseCallback
from stable_baselines3.common.vec_env import DummyVecEnv, SubprocVecEnv

from . import ACTION_SCHEMA_VERSION, ENVIRONMENT_SCHEMA_VERSION, OBSERVATION_SCHEMA_VERSION
from .gym_env import AuthorityGymEnv
from .imitation import behavior_clone, collect_demonstrations
from .model import (
    CandidateMaskablePolicy,
    CandidateMaskablePolicyV2,
    SparseBaseCriticCandidateMaskablePolicyV2,
    SparseCandidateMaskablePolicyV2,
)
from .profiled_ppo import ProfiledMaskablePPO
from .profiling import ProfileCollector, merge_profile_summaries, process_snapshot
from .resources import BLAS_THREAD_ENVIRONMENT, capture_host_state, configure_thread_runtime
from .schema_v2 import ACTION_SCHEMA_V2, ENVIRONMENT_SCHEMA_V2, OBSERVATION_SCHEMA_V2


@dataclass(frozen=True)
class TrainingConfig:
    seed: int
    total_timesteps: int = 20_000
    workers: int = 4
    rollout_steps: int = 256
    batch_size: int = 256
    epochs: int = 8
    learning_rate: float = 3e-4
    gamma: float = 0.995
    gae_lambda: float = 0.95
    entropy_coefficient: float = 0.01
    checkpoint_interval: int = 5_000
    imitation_decisions: int = 5_000
    imitation_epochs: int = 5
    max_episode_actions: int = 1200
    device: str = "cpu"
    resource_profile: str = "custom"
    torch_threads: int = 1
    learner_torch_interop_threads: int = 1
    learner_blas_threads: int = 1
    worker_torch_threads: int = 1
    worker_torch_interop_threads: int = 1
    worker_blas_threads: int = 1
    authority_mode: str = "ephemeral"
    telemetry_mode: str = "training"
    opponent_specs: tuple[str, ...] = ("random", "random", "heuristic", "heuristic", "historical")
    reward: str = "+1 victory, -1 defeat, 0 draw; no shaping"
    observation_schema: str = OBSERVATION_SCHEMA_VERSION
    imitation_teacher: str = "heuristic-v1"
    transport_mode: str = "full"
    ablation_condition: str = "current-recipe"
    instrumentation: bool = False
    sparse_actor: bool = True
    opponent_selection_mode: str = "legacy-flat"
    critic_architecture: str = "dense"
    verbose: int = 1


class ArtifactCallback(BaseCallback):
    def __init__(self, run_dir: Path, checkpoint_interval: int, *, instrumentation: bool) -> None:
        super().__init__(verbose=0)
        self.run_dir = run_dir
        self.checkpoint_interval = checkpoint_interval
        self.last_checkpoint = 0
        self.episode_file = run_dir / "training_episodes.jsonl"
        self.collection_seconds = 0.0
        self.update_seconds = 0.0
        self.artifact_io_seconds = 0.0
        self.completed_episodes = 0
        self._cycle_started = 0.0
        self._update_started = 0.0
        self.instrumentation = instrumentation
        self.worker_profiles: list[dict[str, Any]] = []

    def _on_training_start(self) -> None:
        self._cycle_started = time.perf_counter()

    def _on_rollout_start(self) -> None:
        now = time.perf_counter()
        if self._update_started:
            self.update_seconds += now - self._update_started
            self._update_started = 0.0
        self._cycle_started = now

    def _on_rollout_end(self) -> None:
        now = time.perf_counter()
        self.collection_seconds += now - self._cycle_started
        self._update_started = now

    def _on_training_end(self) -> None:
        if self._update_started:
            self.update_seconds += time.perf_counter() - self._update_started
            self._update_started = 0.0

    def _on_step(self) -> bool:
        for info in self.locals.get("infos", []):
            if "episode_metrics" not in info:
                continue
            record = {
                "timesteps": self.num_timesteps,
                "optimizer_updates": int(getattr(self.model, "_n_updates", 0)),
                "learner_seat": info.get("learner_seat"),
                "opponent": info.get("opponent"),
                "episode": info.get("episode"),
                "metrics": info.get("episode_metrics"),
            }
            io_started = time.perf_counter()
            with self.episode_file.open("a") as handle:
                handle.write(json.dumps(record, sort_keys=True) + "\n")
            self.artifact_io_seconds += time.perf_counter() - io_started
            self.completed_episodes += 1
            if self.instrumentation and info.get("throughput_profile"):
                self.worker_profiles.append(info["throughput_profile"])
            replay = info.get("replay")
            if replay and not (self.run_dir / "training_representative_replay.json").exists():
                io_started = time.perf_counter()
                (self.run_dir / "training_representative_replay.json").write_text(
                    json.dumps(replay, indent=2, sort_keys=True) + "\n"
                )
                self.artifact_io_seconds += time.perf_counter() - io_started
        if self.num_timesteps - self.last_checkpoint >= self.checkpoint_interval:
            io_started = time.perf_counter()
            atomic_model_save(
                self.model,
                self.run_dir / "checkpoints" / f"step-{self.num_timesteps:09d}.zip",
            )
            self.artifact_io_seconds += time.perf_counter() - io_started
            self.last_checkpoint = self.num_timesteps
        return True


def train_seed(
    *,
    config: TrainingConfig,
    binary: Path,
    server_root: Path,
    output_root: Path,
) -> dict[str, Any]:
    process_started = time.perf_counter()
    host_before = capture_host_state() if config.instrumentation else {}
    run_profile = ProfileCollector(config.instrumentation)
    run_dir = output_root / f"seed-{config.seed}"
    checkpoint_dir = run_dir / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    metadata = experiment_metadata(config, server_root)
    (run_dir / "config.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")

    opponents = [
        f"pool:{checkpoint_dir}" if specification == "historical" else specification
        for specification in config.opponent_specs
    ]
    environments = []
    for worker in range(config.workers):
        worker_seed = config.seed * 100 + worker
        environments.append(
            lambda worker_seed=worker_seed, worker=worker: AuthorityGymEnv(
                binary=binary,
                server_root=server_root,
                training_seed=worker_seed,
                opponent_specs=opponents,
                learner_seat="alternate",
                max_episode_actions=config.max_episode_actions,
                device=config.device,
                session_id=f"training-{config.seed}-{worker}",
                authority_mode=config.authority_mode,
                telemetry_mode=config.telemetry_mode,
                transport_mode=config.transport_mode,
                observation_schema=config.observation_schema,
                instrumentation=config.instrumentation,
                torch_threads=config.worker_torch_threads,
                torch_interop_threads=config.worker_torch_interop_threads,
                blas_threads=config.worker_blas_threads,
                opponent_selection_mode=config.opponent_selection_mode,
            )
        )
    learner_blas_environment = {
        variable: os.environ.get(variable) for variable in BLAS_THREAD_ENVIRONMENT
    }
    try:
        for variable in BLAS_THREAD_ENVIRONMENT:
            os.environ[variable] = str(config.worker_blas_threads)
        if config.workers == 1:
            vec_env = DummyVecEnv(environments)
        else:
            vec_env = SubprocVecEnv(environments, start_method="spawn")
    finally:
        for variable, value in learner_blas_environment.items():
            if value is None:
                os.environ.pop(variable, None)
            else:
                os.environ[variable] = value
    configure_thread_runtime(
        torch_threads=config.torch_threads,
        torch_interop_threads=config.learner_torch_interop_threads,
        blas_threads=config.learner_blas_threads,
    )
    try:
        if config.observation_schema == OBSERVATION_SCHEMA_V2:
            if config.critic_architecture == "base":
                if not config.sparse_actor:
                    raise ValueError("base critic ablation requires the sparse v2 actor")
                policy_class = SparseBaseCriticCandidateMaskablePolicyV2
            else:
                policy_class = (
                    SparseCandidateMaskablePolicyV2
                    if config.sparse_actor
                    else CandidateMaskablePolicyV2
                )
        else:
            policy_class = CandidateMaskablePolicy
        algorithm_class = ProfiledMaskablePPO if config.instrumentation else MaskablePPO
        model = algorithm_class(
            policy_class,
            vec_env,
            learning_rate=config.learning_rate,
            n_steps=config.rollout_steps,
            batch_size=config.batch_size,
            n_epochs=config.epochs,
            gamma=config.gamma,
            gae_lambda=config.gae_lambda,
            ent_coef=config.entropy_coefficient,
            verbose=config.verbose,
            seed=config.seed,
            device=config.device,
        )
        if config.instrumentation:
            _instrument_rollout_path(model, vec_env, run_profile)
        initial_hash = parameter_hash(model)
        with run_profile.span("artifact.initial_checkpoint"):
            atomic_model_save(model, checkpoint_dir / "initial.zip")
        if config.imitation_decisions > 0 and config.imitation_epochs > 0:
            with run_profile.span("imitation.demonstration_collection"):
                demonstrations = collect_demonstrations(
                    binary=binary,
                    server_root=server_root,
                    seed=config.seed,
                    decisions=config.imitation_decisions,
                    observation_schema=config.observation_schema,
                    teacher=config.imitation_teacher,
                )
            with run_profile.span("imitation.behavior_clone"):
                imitation_summary = behavior_clone(
                    model,
                    demonstrations,
                    epochs=config.imitation_epochs,
                    batch_size=config.batch_size,
                    seed=config.seed,
                )
        else:
            imitation_summary = {
                "demonstrations": 0,
                "epochs": 0,
                "teacher": config.imitation_teacher,
                "skipped": True,
            }
        with run_profile.span("artifact.behavior_cloned_checkpoint"):
            atomic_model_save(model, checkpoint_dir / "behavior-cloned.zip")
        (run_dir / "imitation_summary.json").write_text(
            json.dumps(imitation_summary, indent=2, sort_keys=True) + "\n"
        )
        callback = ArtifactCallback(
            run_dir,
            config.checkpoint_interval,
            instrumentation=config.instrumentation,
        )
        started = time.perf_counter()
        model.learn(
            total_timesteps=config.total_timesteps,
            callback=callback,
            progress_bar=False,
            log_interval=1,
        )
        elapsed = time.perf_counter() - started
        with run_profile.span("artifact.final_checkpoint"):
            atomic_model_save(model, checkpoint_dir / "final.zip")
        final_hash = parameter_hash(model)
        summary = {
            "seed": config.seed,
            "requested_timesteps": config.total_timesteps,
            "completed_timesteps": int(model.num_timesteps),
            "optimizer_updates": int(getattr(model, "_n_updates", 0)),
            "elapsed_seconds": elapsed,
            "steps_per_second": model.num_timesteps / elapsed if elapsed else 0.0,
            "completed_episodes": callback.completed_episodes,
            "complete_games_per_second": callback.completed_episodes / elapsed if elapsed else 0.0,
            "timing": {
                "collection_seconds": callback.collection_seconds,
                "ppo_update_seconds": callback.update_seconds,
                "artifact_io_seconds": callback.artifact_io_seconds,
            },
            "initial_parameter_hash": initial_hash,
            "final_parameter_hash": final_hash,
            "parameters_changed": initial_hash != final_hash,
            "imitation": imitation_summary,
            "checkpoints": [str(path) for path in sorted(checkpoint_dir.glob("*.zip"))],
        }
        if config.instrumentation:
            update_profile = getattr(model, "update_profile", ProfileCollector()).summary()
            policy_profile = getattr(model.policy, "profile", ProfileCollector()).summary()
            summary["instrumentation"] = {
                "main": run_profile.summary(),
                "ppo_update": update_profile,
                "policy": policy_profile,
                "workers": merge_profile_summaries(callback.worker_profiles),
                "process": process_snapshot(include_torch=True),
                "resource_usage": _resource_usage(),
                "host_before": host_before,
                "host_after": capture_host_state(),
                "process_start_to_final_checkpoint_seconds": time.perf_counter() - process_started,
            }
        (run_dir / "training_summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
        return summary
    finally:
        vec_env.close()


def parameter_hash(model: MaskablePPO) -> str:
    digest = hashlib.sha256()
    state = model.policy.state_dict()
    for name in sorted(state):
        digest.update(name.encode("utf-8"))
        digest.update(state[name].detach().cpu().numpy().tobytes())
    return digest.hexdigest()


def atomic_model_save(model: MaskablePPO, target: Path) -> None:
    """Publish a complete checkpoint in one rename for concurrent pool readers."""

    if target.suffix != ".zip":
        raise ValueError("atomic model targets must use the .zip suffix")
    staging = target.parent / ".staging"
    staging.mkdir(parents=True, exist_ok=True)
    temporary = staging / f"{target.stem}.{uuid.uuid4().hex}.tmp.zip"
    try:
        model.save(temporary)
        os.replace(temporary, target)
    finally:
        temporary.unlink(missing_ok=True)


def experiment_metadata(config: TrainingConfig, server_root: Path) -> dict[str, Any]:
    revision = _command_output(["git", "rev-parse", "HEAD"], server_root)
    dirty = bool(_command_output(["git", "status", "--porcelain"], server_root.parent))
    packages = {}
    for name in ("gymnasium", "numpy", "sb3-contrib", "stable-baselines3", "torch"):
        packages[name] = importlib.metadata.version(name)
    content_hash = hashlib.sha256()
    for path in sorted((server_root / "content" / "battle_v1").rglob("*.yaml")):
        content_hash.update(path.relative_to(server_root).as_posix().encode("utf-8"))
        content_hash.update(path.read_bytes())
    is_v2 = config.observation_schema == OBSERVATION_SCHEMA_V2
    return {
		"environment_schema": ENVIRONMENT_SCHEMA_V2 if is_v2 else ENVIRONMENT_SCHEMA_VERSION,
		"observation_schema": config.observation_schema,
		"action_schema": ACTION_SCHEMA_V2 if is_v2 else ACTION_SCHEMA_VERSION,
        "source_revision": revision,
        "source_dirty": dirty,
        "content_version": content_hash.hexdigest(),
        "model_architecture": (
            "MaskablePPO v2 mechanics sparse candidate scorer [96,96], base-only critic"
            if is_v2 and config.sparse_actor and config.critic_architecture == "base"
            else "MaskablePPO v2 mechanics sparse candidate scorer [96,96]"
            if is_v2 and config.sparse_actor
            else "MaskablePPO v2 mechanics dense candidate scorer [96,96]"
            if is_v2
            else "MaskablePPO shared candidate scorer: context MLP [64,64], "
            "candidate MLP [64,64], score MLP [64,1], value MLP [128,128]"
        ),
        "training": asdict(config),
        "packages": packages,
        "runtime": {
            "python": platform.python_version(),
            "platform": platform.platform(),
            "machine": platform.machine(),
            "processor": platform.processor(),
            "cpu_count": os.cpu_count(),
            "torch_threads": torch.get_num_threads(),
            "torch_interop_threads": torch.get_num_interop_threads(),
            "blas_environment": {
                variable: os.environ.get(variable, "")
                for variable in BLAS_THREAD_ENVIRONMENT
            },
            "device": config.device,
        },
        "seeds": {
            "training_seed": config.seed,
            "worker_seed_rule": "training_seed * 100 + worker",
            "episode_seed_rule": "worker_seed * 1,000,000 + episode_index",
        },
        "opponent_pool": list(config.opponent_specs),
        "reward": config.reward,
    }


def set_reproducible_runtime(
    seed: int,
    torch_threads: int = 1,
    torch_interop_threads: int = 1,
    blas_threads: int | None = None,
) -> None:
    import random

    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    configure_thread_runtime(
        torch_threads=torch_threads,
        torch_interop_threads=torch_interop_threads,
        blas_threads=blas_threads or torch_threads,
    )
    torch.use_deterministic_algorithms(True, warn_only=True)


def _instrument_rollout_path(
    model: MaskablePPO,
    vec_env: DummyVecEnv | SubprocVecEnv,
    profile: ProfileCollector,
) -> None:
    original_step_wait = vec_env.step_wait

    def step_wait() -> Any:
        with profile.span("rollout.vec_env_step_wait"):
            return original_step_wait()

    vec_env.step_wait = step_wait  # type: ignore[method-assign]
    original_add = model.rollout_buffer.add

    def add(*args: Any, **kwargs: Any) -> None:
        with profile.span("rollout.buffer_add"):
            original_add(*args, **kwargs)

    model.rollout_buffer.add = add  # type: ignore[method-assign]
    original_gae = model.rollout_buffer.compute_returns_and_advantage

    def compute_returns_and_advantage(*args: Any, **kwargs: Any) -> None:
        with profile.span("rollout.gae"):
            original_gae(*args, **kwargs)

    model.rollout_buffer.compute_returns_and_advantage = (  # type: ignore[method-assign]
        compute_returns_and_advantage
    )


def _resource_usage() -> dict[str, float | int]:
    usage = resource.getrusage(resource.RUSAGE_SELF)
    children = resource.getrusage(resource.RUSAGE_CHILDREN)
    rss_scale = 1 if platform.system() == "Darwin" else 1024
    return {
        "self_user_seconds": usage.ru_utime,
        "self_system_seconds": usage.ru_stime,
        "children_user_seconds": children.ru_utime,
        "children_system_seconds": children.ru_stime,
        "self_peak_rss_bytes": int(usage.ru_maxrss * rss_scale),
        "children_peak_rss_bytes": int(children.ru_maxrss * rss_scale),
    }


def _command_output(command: list[str], cwd: Path) -> str:
    completed = subprocess.run(command, cwd=cwd, check=True, capture_output=True, text=True)  # noqa: S603
    return completed.stdout.strip()
