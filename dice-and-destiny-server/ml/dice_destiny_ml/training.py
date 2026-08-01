from __future__ import annotations

import hashlib
import importlib.metadata
import json
import os
import platform
import subprocess
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import torch
from sb3_contrib import MaskablePPO
from stable_baselines3.common.callbacks import BaseCallback
from stable_baselines3.common.vec_env import DummyVecEnv

from . import ACTION_SCHEMA_VERSION, ENVIRONMENT_SCHEMA_VERSION, OBSERVATION_SCHEMA_VERSION
from .gym_env import AuthorityGymEnv
from .imitation import behavior_clone, collect_heuristic_demonstrations
from .model import CandidateMaskablePolicy


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
    opponent_specs: tuple[str, ...] = ("random", "random", "heuristic", "heuristic", "historical")
    reward: str = "+1 victory, -1 defeat, 0 draw; no shaping"


class ArtifactCallback(BaseCallback):
    def __init__(self, run_dir: Path, checkpoint_interval: int) -> None:
        super().__init__(verbose=0)
        self.run_dir = run_dir
        self.checkpoint_interval = checkpoint_interval
        self.last_checkpoint = 0
        self.episode_file = run_dir / "training_episodes.jsonl"

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
            with self.episode_file.open("a") as handle:
                handle.write(json.dumps(record, sort_keys=True) + "\n")
            replay = info.get("replay")
            if replay and not (self.run_dir / "training_representative_replay.json").exists():
                (self.run_dir / "training_representative_replay.json").write_text(
                    json.dumps(replay, indent=2, sort_keys=True) + "\n"
                )
        if self.num_timesteps - self.last_checkpoint >= self.checkpoint_interval:
            self.model.save(self.run_dir / "checkpoints" / f"step-{self.num_timesteps:09d}")
            self.last_checkpoint = self.num_timesteps
        return True


def train_seed(
    *,
    config: TrainingConfig,
    binary: Path,
    server_root: Path,
    output_root: Path,
) -> dict[str, Any]:
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
            )
        )
    vec_env = DummyVecEnv(environments)
    try:
        model = MaskablePPO(
            CandidateMaskablePolicy,
            vec_env,
            learning_rate=config.learning_rate,
            n_steps=config.rollout_steps,
            batch_size=config.batch_size,
            n_epochs=config.epochs,
            gamma=config.gamma,
            gae_lambda=config.gae_lambda,
            ent_coef=config.entropy_coefficient,
            verbose=1,
            seed=config.seed,
            device=config.device,
        )
        initial_hash = parameter_hash(model)
        model.save(checkpoint_dir / "initial")
        demonstrations = collect_heuristic_demonstrations(
            binary=binary,
            server_root=server_root,
            seed=config.seed,
            decisions=config.imitation_decisions,
        )
        imitation_summary = behavior_clone(
            model,
            demonstrations,
            epochs=config.imitation_epochs,
            batch_size=config.batch_size,
            seed=config.seed,
        )
        model.save(checkpoint_dir / "behavior-cloned")
        (run_dir / "imitation_summary.json").write_text(
            json.dumps(imitation_summary, indent=2, sort_keys=True) + "\n"
        )
        callback = ArtifactCallback(run_dir, config.checkpoint_interval)
        started = time.perf_counter()
        model.learn(
            total_timesteps=config.total_timesteps,
            callback=callback,
            progress_bar=False,
            log_interval=1,
        )
        elapsed = time.perf_counter() - started
        model.save(checkpoint_dir / "final")
        final_hash = parameter_hash(model)
        summary = {
            "seed": config.seed,
            "requested_timesteps": config.total_timesteps,
            "completed_timesteps": int(model.num_timesteps),
            "optimizer_updates": int(getattr(model, "_n_updates", 0)),
            "elapsed_seconds": elapsed,
            "steps_per_second": model.num_timesteps / elapsed if elapsed else 0.0,
            "initial_parameter_hash": initial_hash,
            "final_parameter_hash": final_hash,
            "parameters_changed": initial_hash != final_hash,
            "imitation": imitation_summary,
            "checkpoints": [str(path) for path in sorted(checkpoint_dir.glob("*.zip"))],
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
    return {
        "environment_schema": ENVIRONMENT_SCHEMA_VERSION,
        "observation_schema": OBSERVATION_SCHEMA_VERSION,
        "action_schema": ACTION_SCHEMA_VERSION,
        "source_revision": revision,
        "source_dirty": dirty,
        "content_version": content_hash.hexdigest(),
        "model_architecture": (
            "MaskablePPO shared candidate scorer: context MLP [64,64], "
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


def set_reproducible_runtime(seed: int) -> None:
    import random

    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    torch.use_deterministic_algorithms(True, warn_only=True)


def _command_output(command: list[str], cwd: Path) -> str:
    completed = subprocess.run(command, cwd=cwd, check=True, capture_output=True, text=True)  # noqa: S603
    return completed.stdout.strip()
