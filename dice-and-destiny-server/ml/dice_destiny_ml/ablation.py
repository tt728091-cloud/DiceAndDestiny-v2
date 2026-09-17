from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from .evaluation import evaluate
from .resources import capture_host_state
from .training import TrainingConfig, set_reproducible_runtime, train_seed


def run_phase2_matrix(*, binary: Path, server_root: Path, config_file: Path) -> dict[str, Any]:
    config = json.loads(config_file.read_text())
    artifact_root = server_root / "ml" / config["artifact_root"]
    artifact_root.mkdir(parents=True, exist_ok=True)
    accepted_v1 = server_root / "ml" / config["accepted_v1_checkpoint"]
    runs = []
    heldout = config["heldout"]
    heldout_seeds = list(
        range(
            int(heldout["seed_start"]),
            int(heldout["seed_start"]) + int(heldout["episodes_per_seat_assignment"]),
        )
    )
    host_before = capture_host_state()
    for condition in config["conditions"]:
        for budget in config["budgets"]:
            run_root = artifact_root / "matrix" / condition["id"] / f"budget-{budget}"
            for seed in config["seeds"]:
                seed_dir = run_root / f"seed-{seed}"
                final_checkpoint = seed_dir / "checkpoints" / "final.zip"
                if not (seed_dir / "training_summary.json").exists():
                    opponents = tuple(
                        f"model:{accepted_v1}" if value == "accepted-v1" else value
                        for value in condition["opponents"]
                    )
                    set_reproducible_runtime(int(seed), int(config["training"]["torch_threads"]))
                    train_seed(
                        config=TrainingConfig(
                            seed=int(seed),
                            total_timesteps=int(budget),
                            workers=int(config["training"]["workers"]),
                            rollout_steps=int(config["training"]["rollout_steps"]),
                            batch_size=int(config["training"]["batch_size"]),
                            epochs=int(config["training"]["epochs"]),
                            learning_rate=float(config["training"]["learning_rate"]),
                            checkpoint_interval=int(config["training"]["checkpoint_interval"]),
                            imitation_decisions=int(condition["imitation_decisions"]),
                            imitation_epochs=int(condition["imitation_epochs"]),
                            device=config["training"]["device"],
                            resource_profile=config["profile"],
                            torch_threads=int(config["training"]["torch_threads"]),
                            opponent_specs=opponents,
                            observation_schema=condition["observation_schema"],
                            imitation_teacher=condition["imitation_teacher"],
                            transport_mode=config["training"]["transport_mode"],
                            ablation_condition=condition["id"],
                            reward=str(config["reward"]),
                            verbose=0,
                        ),
                        binary=binary,
                        server_root=server_root,
                        output_root=run_root,
                    )
                evaluation_dir = seed_dir / "heldout-vs-accepted-v1"
                if not (evaluation_dir / "summary.json").exists():
                    evaluate(
                        binary=binary,
                        server_root=server_root,
                        seat_a_spec=f"model:{final_checkpoint}",
                        seat_b_spec=f"model:{accepted_v1}",
                        seeds=heldout_seeds,
                        output_dir=evaluation_dir,
                        swap=bool(heldout["swap"]),
                        device=config["training"]["device"],
                        deterministic=True,
                        save_replays="representative",
                        authority_mode="ephemeral",
                        workers=int(config["training"]["workers"]),
                        torch_threads=int(config["training"]["torch_threads"]),
                        profile=config["profile"],
                        telemetry_mode="training",
                        transport_mode="full",
                        observation_schema=condition["observation_schema"],
                    )
                runs.append(
                    {
                        "condition": condition["id"],
                        "budget": budget,
                        "seed": seed,
                        "checkpoint": str(final_checkpoint),
                        "training_summary": str(seed_dir / "training_summary.json"),
                        "heldout_summary": str(evaluation_dir / "summary.json"),
                    }
                )
    result = {
        "config": str(config_file.resolve()),
        "expected_training_runs": len(config["conditions"]) * len(config["budgets"]) * len(config["seeds"]),
        "completed_runs": len(runs),
        "runs": runs,
        "host_before": host_before,
        "host_after": capture_host_state(),
    }
    (artifact_root / "matrix-index.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result
