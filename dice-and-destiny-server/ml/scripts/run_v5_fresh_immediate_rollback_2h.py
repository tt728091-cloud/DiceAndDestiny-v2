from __future__ import annotations

import argparse
from pathlib import Path

from dice_destiny_ml.manifest_v5 import OBSERVATION_SCHEMA_V5
from dice_destiny_ml.resources import ResourceBudget
from dice_destiny_ml.winner_health_campaign import (
    WinnerHealthCampaignConfig,
    prepare_winner_health_campaign,
    run_winner_health_campaign,
)

CAMPAIGN_ID = "v5-relational-fresh-immediate-rollback-cp193-20260809-2h-corrected"
MANIFEST_CAMPAIGN_ID = "v5-relational-fresh-cp193-20260808-1h"


def repository_paths() -> tuple[Path, Path]:
    ml_root = Path(__file__).resolve().parents[1]
    return ml_root, ml_root.parent


def budget() -> ResourceBudget:
    return ResourceBudget(
        profile="max",
        workers=12,
        torch_threads=1,
        blas_threads=1,
        learner_torch_threads=4,
        learner_torch_interop_threads=1,
        learner_blas_threads=4,
        worker_torch_threads=1,
        worker_torch_interop_threads=1,
        worker_blas_threads=1,
        inference_concurrency=12,
        io_concurrency=2,
        logical_cpus=16,
    )


def campaign_config(ml_root: Path, server_root: Path) -> WinnerHealthCampaignConfig:
    manifest_root = (ml_root / "runs" / MANIFEST_CAMPAIGN_ID).resolve()
    return WinnerHealthCampaignConfig(
        campaign_id=CAMPAIGN_ID,
        artifact_root=ml_root / "runs" / CAMPAIGN_ID,
        registry_path=ml_root / "champions/current-global.json",
        source_config_path=ml_root
        / "runs/v3-seed22-optimized-5m-20260803-m3max/seed-22/config.json",
        binary=server_root / "build/battle-ml-sim",
        server_root=server_root,
        budget=budget(),
        time_budget_seconds=2 * 60 * 60,
        maximum_intervals=256,
        require_full_window=True,
        # A single failed evaluation immediately restores the last accepted
        # checkpoint's weights and optimizer before the next attempt.
        failure_streak_limit=1,
        warmup_intervals=5,
        global_confirmation_games=0,
        observation_schema=OBSERVATION_SCHEMA_V5,
        observation_manifest=manifest_root / "observation-manifest-v5.json",
        entity_width=96,
        decision_width=432,
        entity_depth=1,
        decision_depth=2,
        activation="tanh",
        interval_requested_steps=12_288,
        ppo_learning_rate=1e-4,
        ppo_epochs=4,
        ppo_clip_range=0.075,
        ppo_target_kl=0.015,
        ppo_batch_size=512,
        ppo_max_grad_norm=0.75,
        deterministic_training_opponent=True,
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("prepare", "campaign"))
    args = parser.parse_args()
    ml_root, server_root = repository_paths()
    config = campaign_config(ml_root, server_root)
    if args.mode == "prepare":
        prepare_winner_health_campaign(config)
    else:
        run_winner_health_campaign(config)


if __name__ == "__main__":
    main()
