from __future__ import annotations

import argparse
from pathlib import Path

from dice_destiny_ml.gym_env import WINNER_HEALTH_REWARD
from dice_destiny_ml.manifest_v5 import OBSERVATION_SCHEMA_V5, build_observation_manifest_v5
from dice_destiny_ml.resources import ResourceBudget
from dice_destiny_ml.training import TrainingConfig, train_seed
from dice_destiny_ml.winner_health_campaign import (
    WinnerHealthCampaignConfig,
    run_winner_health_campaign,
)

CAMPAIGN_ID = "v5-relational-fresh-cp193-20260808-1h"
ENTITY_WIDTH = 96
DECISION_WIDTH = 432
ENTITY_DEPTH = 1
DECISION_DEPTH = 2
ACTIVATION = "tanh"


def repository_paths() -> tuple[Path, Path]:
    ml_root = Path(__file__).resolve().parents[1]
    return ml_root, ml_root.parent


def ensure_manifest(target: Path, server_root: Path) -> Path:
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        build_observation_manifest_v5(
            server_root / "content" / "battle_v1",
            eligible_combatants=["blade_warden"],
            observed_candidate_maximum=114,
            generated_scenarios=10,
        ).save(target)
    return target.resolve()


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


def run_preflight(manifest: Path, ml_root: Path, server_root: Path) -> None:
    champion = (
        ml_root
        / "runs/v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only"
        / "intervals/interval-193/raw-ppo-challenger.zip"
    ).resolve()
    config = TrainingConfig(
        seed=850801,
        total_timesteps=3_072,
        workers=12,
        rollout_steps=256,
        batch_size=512,
        epochs=8,
        learning_rate=3e-4,
        gamma=0.995,
        gae_lambda=0.95,
        entropy_coefficient=0.01,
        checkpoint_interval=100_000,
        imitation_decisions=256,
        imitation_epochs=1,
        max_episode_actions=1_200,
        device="cpu",
        resource_profile="max-preflight",
        torch_threads=4,
        learner_torch_interop_threads=1,
        learner_blas_threads=4,
        worker_torch_threads=1,
        worker_torch_interop_threads=1,
        worker_blas_threads=1,
        authority_mode="ephemeral",
        telemetry_mode="training",
        opponent_specs=(f"model:{champion}",),
        reward=WINNER_HEALTH_REWARD,
        observation_schema=OBSERVATION_SCHEMA_V5,
        imitation_teacher="mechanics-v2",
        transport_mode="full",
        instrumentation=True,
        sparse_actor=True,
        verbose=0,
        observation_manifest=str(manifest),
        entity_width=ENTITY_WIDTH,
        decision_width=DECISION_WIDTH,
        entity_depth=ENTITY_DEPTH,
        decision_depth=DECISION_DEPTH,
        activation=ACTIVATION,
        post_ppo_correction=False,
        clip_range=0.2,
        target_kl=None,
        value_loss_coefficient=0.5,
    )
    summary = train_seed(
        config=config,
        binary=server_root / "build" / "battle-ml-sim",
        server_root=server_root,
        output_root=ml_root / "runs/preflight-v5-relational-entitydepth1-20260808",
    )
    print(
        {
            key: summary[key]
            for key in (
                "completed_timesteps",
                "elapsed_seconds",
                "completed_episodes",
                "parameters_changed",
                "optimizer_updates",
                "steps_per_second",
            )
        },
        flush=True,
    )


def run_campaign(manifest: Path, ml_root: Path, server_root: Path) -> None:
    config = WinnerHealthCampaignConfig(
        campaign_id=CAMPAIGN_ID,
        artifact_root=ml_root / "runs" / CAMPAIGN_ID,
        registry_path=ml_root / "champions/current-global.json",
        source_config_path=ml_root / "runs/v3-seed22-optimized-5m-20260803-m3max/seed-22/config.json",
        binary=server_root / "build/battle-ml-sim",
        server_root=server_root,
        budget=budget(),
        time_budget_seconds=3_600.0,
        maximum_intervals=128,
        require_full_window=True,
        failure_streak_limit=7,
        warmup_intervals=5,
        global_confirmation_games=0,
        observation_schema=OBSERVATION_SCHEMA_V5,
        observation_manifest=manifest,
        entity_width=ENTITY_WIDTH,
        decision_width=DECISION_WIDTH,
        entity_depth=ENTITY_DEPTH,
        decision_depth=DECISION_DEPTH,
        activation=ACTIVATION,
        ppo_batch_size=512,
    )
    run_winner_health_campaign(config)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("preflight", "campaign"))
    args = parser.parse_args()
    ml_root, server_root = repository_paths()
    root = ml_root / "runs" / CAMPAIGN_ID
    manifest = ensure_manifest(root / "observation-manifest-v5.json", server_root)
    if args.mode == "preflight":
        run_preflight(manifest, ml_root, server_root)
    else:
        run_campaign(manifest, ml_root, server_root)


if __name__ == "__main__":
    main()
