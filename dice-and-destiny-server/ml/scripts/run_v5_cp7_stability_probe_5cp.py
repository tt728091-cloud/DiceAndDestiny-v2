from __future__ import annotations

import argparse
import json
from pathlib import Path

from dice_destiny_ml.champions import Champion, atomic_json_write
from dice_destiny_ml.manifest_v5 import OBSERVATION_SCHEMA_V5
from dice_destiny_ml.resources import ResourceBudget
from dice_destiny_ml.winner_health_campaign import (
    WinnerHealthCampaignConfig,
    _evaluate,
    prepare_winner_health_campaign,
    run_winner_health_campaign,
)

CAMPAIGN_ID = "v5-relational-cp7-stability-probe-cp193-20260809-5cp"
SOURCE_CAMPAIGN_ID = "v5-relational-cp6-continuation-cp193-20260808-8h"
MANIFEST_CAMPAIGN_ID = "v5-relational-fresh-cp193-20260808-1h"
SOURCE_INTERVAL = 7
FINAL_INTERVAL = 12


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
    source_root = (ml_root / "runs" / SOURCE_CAMPAIGN_ID).resolve()
    manifest_root = (ml_root / "runs" / MANIFEST_CAMPAIGN_ID).resolve()
    return WinnerHealthCampaignConfig(
        campaign_id=CAMPAIGN_ID,
        artifact_root=ml_root / "runs" / CAMPAIGN_ID,
        registry_path=ml_root / "champions/current-global.json",
        source_config_path=ml_root / "runs/v3-seed22-optimized-5m-20260803-m3max/seed-22/config.json",
        binary=server_root / "build/battle-ml-sim",
        server_root=server_root,
        budget=budget(),
        time_budget_seconds=2 * 60 * 60,
        maximum_intervals=FINAL_INTERVAL,
        require_full_window=False,
        failure_streak_limit=1,
        warmup_intervals=0,
        global_confirmation_games=0,
        source_campaign_root=source_root,
        source_interval=SOURCE_INTERVAL,
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
        ppo_clip_range=0.15,
        ppo_target_kl=0.015,
        ppo_batch_size=512,
        ppo_max_grad_norm=0.75,
        deterministic_training_opponent=True,
        allow_continuation_recipe_change=True,
    )


def run_final_audit(config: WinnerHealthCampaignConfig) -> None:
    root = config.artifact_root.resolve()
    state = json.loads((root / "campaign-state.json").read_text())
    if state.get("status") != "completed" or len(state.get("attempts", [])) != 5:
        raise RuntimeError("five-checkpoint campaign must complete before final audit")
    start = Champion(**state["starting_checkpoint"])
    fixed = Champion(**state["fixed_global_champion"])
    final = state["attempts"][-1]
    bank = Path(final["evaluation"]["seed_bank"])
    start_evaluation = _evaluate(
        config,
        start,
        fixed,
        bank,
        1_000,
        root / "final-audit" / "starting-cp7-global-1k",
    )
    audit = {
        "schema": "dice-and-destiny-cp7-stability-final-audit-v1",
        "bank": str(bank.resolve()),
        "starting_checkpoint": state["starting_checkpoint"],
        "starting_evaluation": start_evaluation,
        "final_checkpoint": final["challenger"],
        "final_evaluation": final["evaluation"],
        "adjusted_score_delta": (
            float(final["evaluation"]["adjusted_score"])
            - float(start_evaluation["adjusted_score"])
        ),
    }
    atomic_json_write(root / "final-audit" / "cp12-vs-starting-cp7-same-bank.json", audit)
    print(json.dumps(audit, sort_keys=True), flush=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("prepare", "campaign", "audit"))
    args = parser.parse_args()
    ml_root, server_root = repository_paths()
    config = campaign_config(ml_root, server_root)
    if args.mode == "prepare":
        prepare_winner_health_campaign(config)
    elif args.mode == "audit":
        run_final_audit(config)
    else:
        run_winner_health_campaign(config)


if __name__ == "__main__":
    main()
