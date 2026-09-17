from __future__ import annotations

import argparse
import json
import sys
from dataclasses import asdict
from pathlib import Path

from .ablation import run_phase2_matrix
from .bridge import AuthorityBridge
from .campaign import (
    EXPECTED_GLOBAL_SHA256,
    CampaignConfig,
    initialize_campaign_registry,
    preflight_campaign,
    run_campaign,
    write_campaign_seed_banks,
    write_model_family_plan,
)
from .champions import read_seed_bank
from .diagnostics import run_owner_diagnostic
from .evaluation import evaluate
from .export_policy import (
    export_candidate_policy_v2,
    export_candidate_policy_v3,
    export_phase3_policy,
)
from .gym_env import REWARD_DEFINITIONS
from .hillclimb import (
    HillClimbConfig,
    preflight_hillclimb,
    run_hillclimb,
    write_hillclimb_seed_banks,
)
from .imitation import corrective_clone
from .manifest_v3 import (
    OBSERVATION_SCHEMA_V3,
    ObservationManifestV3,
    build_observation_manifest_v3,
)
from .observation_walkthrough import generate_observation_walkthrough
from .policies import HeuristicPolicy, select_with_policy
from .reporting import generate_report_artifacts
from .resources import resolve_resource_budget
from .schema import SchemaEncoder
from .schema_v2 import OBSERVATION_SCHEMA_V2
from .tactical_corpus import (
    build_decision_corpus,
    build_tactical_corpus,
    evaluate_decision_corpus,
    evaluate_tactical_matrix,
)
from .tournament import run_tournament
from .training import TrainingConfig, set_reproducible_runtime, train_seed
from .winner_health_campaign import (
    WinnerHealthCampaignConfig,
    prepare_winner_health_campaign,
    run_winner_health_campaign,
    run_winner_health_plateau_continuation,
)


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    server_root = Path(__file__).resolve().parents[2]
    binary = Path(args.binary) if args.binary else server_root / "build" / "battle-ml-sim"
    if (
        args.command
        not in {
            "export-policy",
            "manifest-v3",
            "campaign-seed-banks",
            "campaign-family-plan",
            "champion-registry-init",
            "global-hillclimb-seed-banks",
        }
        and not binary.exists()
    ):
        parser.error(f"simulator binary not found: {binary}; run scripts/ml.sh so it is built first")
    if args.command == "manifest-v3":
        manifest = build_observation_manifest_v3(
            server_root / "content" / "battle_v1",
            eligible_combatants=args.combatants,
            observed_candidate_maximum=args.observed_candidate_maximum,
            generated_scenarios=args.generated_scenarios,
        )
        manifest.save(Path(args.output))
        result = {
            "output": str(Path(args.output).resolve()),
            "manifest_sha256": manifest.manifest_sha256,
            "content_sha256": manifest.content_sha256,
            "observation_size": manifest.layout.observation_size,
            "maximum_legal_candidates": manifest.maximum_legal_candidates,
        }
    elif args.command == "campaign-seed-banks":
        result = write_campaign_seed_banks(
            Path(args.output),
            attempts=args.attempts,
            seed_offset=args.seed_offset,
        )
    elif args.command == "campaign-family-plan":
        result = write_model_family_plan(
            Path(args.output),
            [json.loads(value) for value in args.family],
        )
    elif args.command == "champion-registry-init":
        manifest = ObservationManifestV3.load(Path(args.observation_manifest))
        registry = initialize_campaign_registry(
            Path(args.output),
            global_checkpoint=Path(args.global_checkpoint),
            expected_global_sha256=args.expected_global_sha256,
            checkpoint_champion=Path(args.checkpoint_champion),
            manifest=manifest,
            baseline_evidence=args.baseline_evidence,
        )
        result = asdict(registry)
    elif args.command == "global-hillclimb-seed-banks":
        result = write_hillclimb_seed_banks(
            Path(args.output),
            attempts=args.attempts,
            evaluation_seed_offset=args.evaluation_seed_offset,
            training_seed_offset=args.training_seed_offset,
        )
    elif args.command in {"global-hillclimb-preflight", "global-hillclimb-campaign"}:
        budget = resolve_resource_budget("max")
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        hillclimb_config = HillClimbConfig(
            campaign_id=args.campaign_id,
            artifact_root=Path(args.output),
            registry_path=Path(args.registry),
            seed_bank_root=Path(args.seed_banks),
            source_config_path=Path(args.source_config),
            historical_checkpoint_root=Path(args.historical_checkpoints),
            accepted_v1_checkpoint=Path(args.accepted_v1),
            binary=binary,
            server_root=server_root,
            budget=budget,
            interval_steps=args.interval_steps,
            time_budget_seconds=args.time_budget_seconds,
            maximum_attempts=args.maximum_attempts,
            require_full_window=args.require_full_window,
            seat_a_definition=args.seat_a_definition,
            seat_b_definition=args.seat_b_definition,
        )
        result = (
            preflight_hillclimb(hillclimb_config)
            if args.command == "global-hillclimb-preflight"
            else run_hillclimb(hillclimb_config)
        )
    elif args.command in {"champion-campaign-preflight", "champion-campaign"}:
        budget = resolve_resource_budget("max")
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        campaign_config = CampaignConfig(
            campaign_id=args.campaign_id,
            seed=22,
            artifact_root=Path(args.output),
            manifest_path=Path(args.observation_manifest),
            registry_path=Path(args.registry),
            initial_checkpoint=Path(args.initial_checkpoint),
            seed_bank_root=Path(args.seed_banks),
            budget=budget,
            time_budget_seconds=args.time_budget_seconds,
            seat_a_definition=args.seat_a_definition,
            seat_b_definition=args.seat_b_definition,
            warm_start=args.warm_start,
            warm_start_evidence=args.warm_start_evidence,
            maximum_attempts=args.maximum_attempts,
            family_plan_path=Path(args.family_plan) if args.family_plan else None,
            require_full_window=args.require_full_window,
            allow_battery_power=args.allow_battery_power,
            initial_selection_score=args.initial_selection_score,
        )
        result = (
            preflight_campaign(campaign_config)
            if args.command == "champion-campaign-preflight"
            else run_campaign(campaign_config)
        )
    elif args.command in {"winner-health-campaign-prepare", "winner-health-campaign"}:
        budget = resolve_resource_budget("max")
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        winner_health_config = WinnerHealthCampaignConfig(
            campaign_id=args.campaign_id,
            artifact_root=Path(args.output),
            registry_path=Path(args.registry),
            source_config_path=Path(args.source_config),
            binary=binary,
            server_root=server_root,
            budget=budget,
            time_budget_seconds=args.time_budget_seconds,
            maximum_intervals=args.maximum_intervals,
            seat_a_definition=args.seat_a_definition,
            seat_b_definition=args.seat_b_definition,
            require_full_window=args.require_full_window,
            expected_global_sha256=args.expected_global_sha256,
            failure_streak_limit=args.failure_streak_limit,
            warmup_intervals=args.warmup_intervals,
            global_confirmation_games=args.global_confirmation_games,
            source_campaign_root=(Path(args.source_campaign) if args.source_campaign else None),
            source_interval=args.source_interval,
        )
        result = (
            prepare_winner_health_campaign(winner_health_config)
            if args.command == "winner-health-campaign-prepare"
            else run_winner_health_campaign(winner_health_config)
        )
    elif args.command == "winner-health-plateau-continuation":
        budget = resolve_resource_budget("max")
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        winner_health_config = WinnerHealthCampaignConfig(
            campaign_id=args.campaign_id,
            artifact_root=Path(args.output),
            registry_path=Path(args.registry),
            source_config_path=Path(args.source_config),
            binary=binary,
            server_root=server_root,
            budget=budget,
            seat_a_definition=args.seat_a_definition,
            seat_b_definition=args.seat_b_definition,
            require_full_window=False,
        )
        result = run_winner_health_plateau_continuation(
            winner_health_config,
            Path(args.source_campaign),
            source_interval=args.source_interval,
        )
    elif args.command == "export-policy":
        if args.policy_family == "v3":
            if not args.observation_manifest:
                parser.error("v3 export requires --observation-manifest")
            result = export_candidate_policy_v3(
                Path(args.checkpoint),
                Path(args.output),
                manifest_path=Path(args.observation_manifest),
                model_id=args.model_id,
                source_revision=args.source_revision,
                training_engine_revision=args.training_engine_revision,
            )
        else:
            exporter = export_candidate_policy_v2 if args.policy_family == "v2" else export_phase3_policy
            result = exporter(
                Path(args.checkpoint),
                Path(args.output),
                model_id=args.model_id,
                content_version=args.content_version,
                source_revision=args.source_revision,
                training_engine_revision=args.training_engine_revision,
            )
    elif args.command == "phase2-matrix":
        result = run_phase2_matrix(
            binary=binary,
            server_root=server_root,
            config_file=Path(args.config),
        )
    elif args.command == "owner-diagnostic":
        result = run_owner_diagnostic(
            binary=binary,
            server_root=server_root,
            trace_file=Path(args.trace),
            accepted_checkpoint=Path(args.checkpoint),
            output_file=Path(args.output),
        )
    elif args.command == "tactical-matrix":
        result = evaluate_tactical_matrix(
            binary=binary,
            server_root=server_root,
            corpus_file=Path(args.corpus),
            matrix_root=Path(args.matrix),
        )
    elif args.command == "tactical-build":
        result = build_tactical_corpus(
            binary=binary,
            server_root=server_root,
            seed_start=args.seed_start,
            cases=args.cases,
            output_file=Path(args.output),
        )
    elif args.command == "decision-corpus-build":
        result = build_decision_corpus(
            binary=binary,
            server_root=server_root,
            seed_start=args.seed_start,
            cases_per_category=args.cases_per_category,
            max_episodes=args.max_episodes,
            output_file=Path(args.output),
        )
    elif args.command == "decision-corpus-evaluate":
        result = evaluate_decision_corpus(
            binary=binary,
            server_root=server_root,
            corpus_file=Path(args.corpus),
            checkpoint=Path(args.checkpoint),
            output_file=Path(args.output),
            baseline_summary=Path(args.baseline) if args.baseline else None,
        )
    elif args.command == "corrective-clone":
        result = corrective_clone(
            binary=binary,
            server_root=server_root,
            base_checkpoint=Path(args.base_checkpoint),
            corpus_file=Path(args.corpus),
            output_dir=Path(args.output),
            seed=args.seed,
            full_decisions=args.full_decisions,
            tactical_repeats=args.tactical_repeats,
            epochs=args.epochs,
            batch_size=args.batch_size,
            device=args.device,
            learning_rate=args.learning_rate,
        )
    elif args.command == "observation-walkthrough":
        result = generate_observation_walkthrough(
            binary=binary,
            server_root=server_root,
            replay_file=Path(args.replay),
            output_file=Path(args.output),
            device=args.device,
        )
    elif args.command == "smoke":
        result = smoke(binary, server_root, args.seed)
    elif args.command in {"evaluate", "benchmark", "acceptance"}:
        seeds = load_seeds(args)
        try:
            budget = resolve_resource_budget(
                args.profile,
                workers=args.workers,
                torch_threads=args.torch_threads,
            )
        except ValueError as error:
            parser.error(str(error))
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        result = evaluate(
            binary=binary,
            server_root=server_root,
            seat_a_spec=args.seat_a,
            seat_b_spec=args.seat_b,
            seeds=seeds,
            output_dir=Path(args.output),
            swap=args.swap,
            device=args.device,
            deterministic=not args.stochastic,
            save_replays="none" if args.command == "benchmark" else args.save_replays,
            authority_mode=args.authority_mode,
            workers=budget.workers,
            torch_threads=budget.torch_threads,
            profile=budget.profile,
            telemetry_mode=args.telemetry_mode,
            transport_mode=args.transport_mode,
            observation_schema=args.observation_schema,
            observation_manifest=(
                Path(args.observation_manifest).resolve() if args.observation_manifest else None
            ),
            seat_a_definition=args.seat_a_definition,
            seat_b_definition=args.seat_b_definition,
        )
        if args.command == "acceptance":
            failures = []
            if result["games"] < 1000:
                failures.append("fewer than 1,000 games")
            for field in (
                "truncations",
                "authority_rejects",
                "invalid_action_indices",
                "stale_action_submissions",
                "wrong_seat_submissions",
            ):
                if result[field] != 0:
                    failures.append(f"{field}={result[field]}")
            result["acceptance_passed"] = not failures
            result["acceptance_failures"] = failures
            Path(args.output, "summary.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    elif args.command == "train":
        try:
            budget = resolve_resource_budget(
                args.profile,
                workers=args.workers or (4 if args.profile == "custom" else 0),
                torch_threads=args.torch_threads,
                learner_torch_threads=args.learner_torch_threads,
                learner_torch_interop_threads=args.learner_torch_interop_threads,
                learner_blas_threads=args.learner_blas_threads,
                worker_torch_threads=args.worker_torch_threads,
                worker_torch_interop_threads=args.worker_torch_interop_threads,
                worker_blas_threads=args.worker_blas_threads,
            )
        except ValueError as error:
            parser.error(str(error))
        print(json.dumps({"resolved_resource_budget": budget.as_dict()}, sort_keys=True), file=sys.stderr)
        summaries = []
        for seed in args.seeds:
            set_reproducible_runtime(
                seed,
                budget.learner_torch_threads,
                budget.learner_torch_interop_threads,
                budget.learner_blas_threads,
            )
            config = TrainingConfig(
                seed=seed,
                total_timesteps=args.timesteps,
                workers=budget.workers,
                rollout_steps=args.rollout_steps,
                batch_size=args.batch_size,
                epochs=args.epochs,
                learning_rate=args.learning_rate,
                checkpoint_interval=args.checkpoint_interval,
                imitation_decisions=args.imitation_decisions,
                imitation_epochs=args.imitation_epochs,
                device=args.device,
                resource_profile=budget.profile,
                torch_threads=budget.learner_torch_threads,
                learner_torch_interop_threads=budget.learner_torch_interop_threads,
                learner_blas_threads=budget.learner_blas_threads,
                worker_torch_threads=budget.worker_torch_threads,
                worker_torch_interop_threads=budget.worker_torch_interop_threads,
                worker_blas_threads=budget.worker_blas_threads,
                observation_schema=args.observation_schema,
                imitation_teacher=args.imitation_teacher,
                transport_mode=args.transport_mode,
                ablation_condition=args.ablation_condition,
                opponent_specs=tuple(args.opponents),
                instrumentation=args.instrumentation,
                sparse_actor=args.sparse_actor,
                opponent_selection_mode=args.opponent_selection_mode,
                critic_architecture=args.critic_architecture,
                verbose=args.verbose,
                observation_manifest=args.observation_manifest,
                seat_a_definition=args.seat_a_definition,
                seat_b_definition=args.seat_b_definition,
                entity_width=args.entity_width,
                entity_depth=args.entity_depth,
                activation=args.activation,
                post_ppo_correction=args.post_ppo_correction,
                clip_range=args.clip_range,
                target_kl=args.target_kl,
                value_loss_coefficient=args.value_loss_coefficient,
                reward=REWARD_DEFINITIONS[args.reward],
            )
            summaries.append(
                train_seed(
                    config=config,
                    binary=binary,
                    server_root=server_root,
                    output_root=Path(args.output),
                )
            )
        result = {"runs": summaries}
    elif args.command == "tournament":
        result = run_tournament(
            binary=binary,
            server_root=server_root,
            checkpoints=[Path(path) for path in args.checkpoints],
            seeds=load_seeds(args),
            output_dir=Path(args.output),
            device=args.device,
        )
    elif args.command == "replay":
        record = json.loads(Path(args.replay_file).read_text())
        with AuthorityBridge(binary, server_root, session_id="replay") as bridge:
            result = bridge.replay(record)
    elif args.command == "report":
        result = generate_report_artifacts(
            training_root=Path(args.training),
            acceptance_root=Path(args.acceptance),
            tournament_file=Path(args.tournament),
            evaluation_dirs=[Path(path) for path in args.evaluations],
            output_dir=Path(args.output),
        )
    else:
        parser.error(f"unsupported command {args.command}")
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Dice and Destiny battle-intelligence runners")
    parser.add_argument("--binary", default="", help="prebuilt battle-ml-sim path")
    subparsers = parser.add_subparsers(dest="command", required=True)

    manifest_parser = subparsers.add_parser(
        "manifest-v3", help="freeze and hash the Observation V3 campaign content manifest"
    )
    manifest_parser.add_argument("--combatants", nargs="+", default=None)
    manifest_parser.add_argument("--observed-candidate-maximum", type=int, default=0)
    manifest_parser.add_argument("--generated-scenarios", type=int, default=0)
    manifest_parser.add_argument("--output", required=True)

    seed_bank_parser = subparsers.add_parser(
        "campaign-seed-banks", help="predeclare disjoint global progression and confirmation banks"
    )
    seed_bank_parser.add_argument("--attempts", type=int, default=512)
    seed_bank_parser.add_argument("--seed-offset", type=int, default=0)
    seed_bank_parser.add_argument("--output", required=True)

    family_plan_parser = subparsers.add_parser(
        "campaign-family-plan",
        help="freeze and hash predeclared seed/model-family restart checkpoints",
    )
    family_plan_parser.add_argument(
        "--family",
        action="append",
        required=True,
        help="JSON object with family_id, seed, checkpoint/evidence, and architecture",
    )
    family_plan_parser.add_argument("--output", required=True)

    registry_parser = subparsers.add_parser(
        "champion-registry-init", help="create the hash-verified initial champion registry"
    )
    registry_parser.add_argument("--global-checkpoint", required=True)
    registry_parser.add_argument("--expected-global-sha256", required=True)
    registry_parser.add_argument("--checkpoint-champion", required=True)
    registry_parser.add_argument("--observation-manifest", required=True)
    registry_parser.add_argument("--baseline-evidence", required=True)
    registry_parser.add_argument("--output", required=True)

    hillclimb_banks_parser = subparsers.add_parser(
        "global-hillclimb-seed-banks",
        help="predeclare disjoint 1,000-game banks and stochastic training seeds",
    )
    hillclimb_banks_parser.add_argument("--attempts", type=int, default=128)
    hillclimb_banks_parser.add_argument("--evaluation-seed-offset", type=int, required=True)
    hillclimb_banks_parser.add_argument("--training-seed-offset", type=int, required=True)
    hillclimb_banks_parser.add_argument("--output", required=True)

    for name, help_text in (
        ("global-hillclimb-preflight", "verify the frozen checkpoint-480 rollback hill-climb"),
        ("global-hillclimb-campaign", "run the timed checkpoint-480 rollback hill-climb"),
    ):
        hillclimb_parser = subparsers.add_parser(name, help=help_text)
        hillclimb_parser.add_argument("--campaign-id", required=True)
        hillclimb_parser.add_argument("--registry", required=True)
        hillclimb_parser.add_argument("--seed-banks", required=True)
        hillclimb_parser.add_argument("--source-config", required=True)
        hillclimb_parser.add_argument("--historical-checkpoints", required=True)
        hillclimb_parser.add_argument("--accepted-v1", required=True)
        hillclimb_parser.add_argument("--output", required=True)
        hillclimb_parser.add_argument("--interval-steps", type=int, default=50_000)
        hillclimb_parser.add_argument("--time-budget-seconds", type=float, default=5_400.0)
        hillclimb_parser.add_argument("--maximum-attempts", type=int, default=128)
        hillclimb_parser.add_argument(
            "--require-full-window",
            action=argparse.BooleanOptionalAction,
            default=True,
        )
        hillclimb_parser.add_argument("--seat-a-definition", default="blade_warden")
        hillclimb_parser.add_argument("--seat-b-definition", default="blade_warden")

    campaign_preflight_parser = subparsers.add_parser(
        "champion-campaign-preflight",
        help="verify every timed campaign input without starting its clock",
    )
    add_campaign_arguments(campaign_preflight_parser)
    campaign_parser = subparsers.add_parser(
        "champion-campaign", help="run the timed reversible Observation V3 champion campaign"
    )
    add_campaign_arguments(campaign_parser)

    for name, help_text in (
        (
            "winner-health-campaign-prepare",
            "prepare a fresh frozen-global reward family using the checkpoint-480 PPO recipe",
        ),
        ("winner-health-campaign", "run the timed fresh winner-health campaign"),
    ):
        reward_campaign_parser = subparsers.add_parser(name, help=help_text)
        reward_campaign_parser.add_argument("--campaign-id", required=True)
        reward_campaign_parser.add_argument("--registry", required=True)
        reward_campaign_parser.add_argument("--source-config", required=True)
        reward_campaign_parser.add_argument("--output", required=True)
        reward_campaign_parser.add_argument("--time-budget-seconds", type=float, default=3_600.0)
        reward_campaign_parser.add_argument("--maximum-intervals", type=int, default=128)
        reward_campaign_parser.add_argument(
            "--expected-global-sha256",
            default=EXPECTED_GLOBAL_SHA256,
        )
        reward_campaign_parser.add_argument("--failure-streak-limit", type=int, default=3)
        reward_campaign_parser.add_argument("--warmup-intervals", type=int, default=5)
        reward_campaign_parser.add_argument(
            "--global-confirmation-games",
            type=int,
            choices=(0, 3_000),
            default=3_000,
        )
        reward_campaign_parser.add_argument("--source-campaign", default="")
        reward_campaign_parser.add_argument("--source-interval", type=int, default=None)
        reward_campaign_parser.add_argument(
            "--require-full-window", action=argparse.BooleanOptionalAction, default=True
        )
        reward_campaign_parser.add_argument("--seat-a-definition", default="blade_warden")
        reward_campaign_parser.add_argument("--seat-b-definition", default="blade_warden")

    plateau_parser = subparsers.add_parser(
        "winner-health-plateau-continuation",
        help="continue an accepted winner-health checkpoint until three consecutive misses",
    )
    plateau_parser.add_argument("--campaign-id", required=True)
    plateau_parser.add_argument("--registry", required=True)
    plateau_parser.add_argument("--source-config", required=True)
    plateau_parser.add_argument("--source-campaign", required=True)
    plateau_parser.add_argument("--source-interval", type=int, default=38)
    plateau_parser.add_argument("--output", required=True)
    plateau_parser.add_argument("--seat-a-definition", default="blade_warden")
    plateau_parser.add_argument("--seat-b-definition", default="blade_warden")

    smoke_parser = subparsers.add_parser("smoke", help="reset/observe/step/terminal/replay smoke test")
    smoke_parser.add_argument("--seed", type=int, default=20260801)

    matrix_parser = subparsers.add_parser("phase2-matrix", help="run the declared 4x2x3 Phase 2 ablation")
    matrix_parser.add_argument("--config", default="ml/configs/decision-quality-phase2.json")
    diagnostic_parser = subparsers.add_parser(
        "owner-diagnostic", help="replay the preserved immediate-ability-bias diagnostic"
    )
    diagnostic_parser.add_argument("--trace", required=True)
    diagnostic_parser.add_argument("--checkpoint", required=True)
    diagnostic_parser.add_argument("--output", required=True)
    tactical_parser = subparsers.add_parser(
        "tactical-matrix", help="score every final Phase 2 checkpoint on the fixed tactical corpus"
    )
    tactical_parser.add_argument("--corpus", required=True)
    tactical_parser.add_argument("--matrix", required=True)
    tactical_build_parser = subparsers.add_parser(
        "tactical-build", help="build a real-authority first-roll mechanics corpus"
    )
    tactical_build_parser.add_argument("--seed-start", type=int, required=True)
    tactical_build_parser.add_argument("--cases", type=int, required=True)
    tactical_build_parser.add_argument("--output", required=True)
    broad_build_parser = subparsers.add_parser(
        "decision-corpus-build", help="build a broad mechanics decision corpus"
    )
    broad_build_parser.add_argument("--seed-start", type=int, required=True)
    broad_build_parser.add_argument("--cases-per-category", type=int, default=20)
    broad_build_parser.add_argument("--max-episodes", type=int, default=500)
    broad_build_parser.add_argument("--output", required=True)
    broad_eval_parser = subparsers.add_parser(
        "decision-corpus-evaluate", help="score a checkpoint on a broad decision corpus"
    )
    broad_eval_parser.add_argument("--corpus", required=True)
    broad_eval_parser.add_argument("--checkpoint", required=True)
    broad_eval_parser.add_argument("--output", required=True)
    broad_eval_parser.add_argument("--baseline", default="")
    corrective_parser = subparsers.add_parser(
        "corrective-clone", help="train a balanced mechanics and tactical imitation candidate"
    )
    corrective_parser.add_argument("--base-checkpoint", required=True)
    corrective_parser.add_argument("--corpus", required=True)
    corrective_parser.add_argument("--output", required=True)
    corrective_parser.add_argument("--seed", type=int, default=11)
    corrective_parser.add_argument("--full-decisions", type=int, default=5_000)
    corrective_parser.add_argument("--tactical-repeats", type=int, default=20)
    corrective_parser.add_argument("--epochs", type=int, default=10)
    corrective_parser.add_argument("--batch-size", type=int, default=256)
    corrective_parser.add_argument("--device", default="cpu")
    corrective_parser.add_argument("--learning-rate", type=float)

    walkthrough_parser = subparsers.add_parser(
        "observation-walkthrough",
        help="reconstruct one observation-v2 replay as a readable self-contained HTML audit",
    )
    walkthrough_parser.add_argument("--replay", required=True)
    walkthrough_parser.add_argument("--output", required=True)
    walkthrough_parser.add_argument("--device", default="cpu")

    export_parser = subparsers.add_parser(
        "export-policy",
        help="export the accepted deterministic actor for the Phase 3 Go runtime",
    )
    export_parser.add_argument("--checkpoint", required=True)
    export_parser.add_argument("--output", required=True)
    export_parser.add_argument("--model-id", required=True)
    export_parser.add_argument("--content-version", required=True)
    export_parser.add_argument("--source-revision", required=True)
    export_parser.add_argument("--training-engine-revision", required=True)
    export_parser.add_argument("--policy-family", choices=("v1", "v2", "v3"), default="v1")
    export_parser.add_argument("--observation-manifest", default="")

    for name, help_text in (
        ("evaluate", "evaluate any two independently instantiated policies"),
        ("benchmark", "measure complete-game and inference throughput"),
        ("acceptance", "run and enforce the 1,000-game two-model soak"),
    ):
        match_parser = subparsers.add_parser(name, help=help_text)
        add_match_arguments(match_parser)
        if name == "acceptance":
            match_parser.set_defaults(episodes=500, swap=True, save_replays="representative")

    train_parser = subparsers.add_parser("train", help="train Maskable PPO from real authority episodes")
    train_parser.add_argument("--seeds", type=int, nargs="+", default=[11, 22, 33])
    train_parser.add_argument("--timesteps", type=int, default=20_000)
    train_parser.add_argument(
        "--profile",
        choices=("max", "balanced-80", "light-50", "custom"),
        default="custom",
    )
    train_parser.add_argument("--workers", type=int, default=0)
    train_parser.add_argument(
        "--torch-threads",
        type=int,
        default=0,
        help="legacy fallback for learner/worker intra-op and BLAS threads",
    )
    train_parser.add_argument("--learner-torch-threads", type=int, default=0)
    train_parser.add_argument("--learner-torch-interop-threads", type=int, default=0)
    train_parser.add_argument("--learner-blas-threads", type=int, default=0)
    train_parser.add_argument("--worker-torch-threads", type=int, default=0)
    train_parser.add_argument("--worker-torch-interop-threads", type=int, default=0)
    train_parser.add_argument("--worker-blas-threads", type=int, default=0)
    train_parser.add_argument("--rollout-steps", type=int, default=256)
    train_parser.add_argument("--batch-size", type=int, default=256)
    train_parser.add_argument("--epochs", type=int, default=8)
    train_parser.add_argument("--learning-rate", type=float, default=3e-4)
    train_parser.add_argument("--clip-range", type=float, default=0.2)
    train_parser.add_argument("--target-kl", type=float)
    train_parser.add_argument("--value-loss-coefficient", type=float, default=0.5)
    train_parser.add_argument(
        "--reward",
        choices=tuple(REWARD_DEFINITIONS),
        default="winner-health-v2",
        help="terminal PPO reward; evaluation remains win/draw/loss scoring",
    )
    train_parser.add_argument("--checkpoint-interval", type=int, default=5_000)
    train_parser.add_argument("--imitation-decisions", type=int, default=5_000)
    train_parser.add_argument("--imitation-epochs", type=int, default=5)
    train_parser.add_argument("--device", default="cpu")
    train_parser.add_argument(
        "--observation-schema",
        choices=("dice-and-destiny-observation-v1", OBSERVATION_SCHEMA_V2, OBSERVATION_SCHEMA_V3),
        default="dice-and-destiny-observation-v1",
    )
    train_parser.add_argument(
        "--imitation-teacher",
        choices=("heuristic-v1", "mechanics-v2"),
        default="heuristic-v1",
    )
    train_parser.add_argument("--transport-mode", choices=("full", "encoded", "parity"), default="full")
    train_parser.add_argument("--ablation-condition", default="current-recipe")
    train_parser.add_argument(
        "--instrumentation",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="record nested timing, byte, candidate-density, process, and host profiles",
    )
    train_parser.add_argument(
        "--sparse-actor",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="score only valid v2 candidates; use --no-sparse-actor for dense ablation",
    )
    train_parser.add_argument(
        "--opponent-selection-mode",
        choices=("champion-registry", "category-balanced", "legacy-flat"),
        default="legacy-flat",
        help="balance declared opponent categories or retain legacy pool-size weighting",
    )
    train_parser.add_argument(
        "--critic-architecture",
        choices=("dense", "base"),
        default="dense",
        help="dense-critic production path or base-only learning ablation",
    )
    train_parser.add_argument(
        "--opponents",
        nargs="+",
        default=["random", "random", "heuristic", "heuristic", "historical"],
    )
    train_parser.add_argument("--output", default="runs/training")
    train_parser.add_argument("--verbose", type=int, choices=(0, 1, 2), default=1)
    train_parser.add_argument("--observation-manifest", default="")
    train_parser.add_argument("--seat-a-definition", default="blade_warden")
    train_parser.add_argument("--seat-b-definition", default="blade_warden")
    train_parser.add_argument("--entity-width", type=int, default=96)
    train_parser.add_argument("--entity-depth", type=int, default=2)
    train_parser.add_argument("--activation", choices=("tanh", "relu", "gelu"), default="tanh")
    train_parser.add_argument(
        "--post-ppo-correction",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="forbidden in the main campaign; isolated corrective-clone remains separately gated",
    )

    tournament_parser = subparsers.add_parser("tournament", help="checkpoint cross-play matrix and Elo")
    tournament_parser.add_argument("--checkpoints", nargs="+", required=True)
    add_seed_arguments(tournament_parser)
    tournament_parser.add_argument("--device", default="cpu")
    tournament_parser.add_argument("--output", default="runs/tournament")

    replay_parser = subparsers.add_parser("replay", help="replay a recorded authority command trace")
    replay_parser.add_argument("replay_file")
    report_parser = subparsers.add_parser("report", help="generate plots and an artifact index")
    report_parser.add_argument("--training", default="runs/phase2-training")
    report_parser.add_argument("--acceptance", default="runs/phase2-acceptance")
    report_parser.add_argument("--tournament", default="runs/phase2-tournament/tournament.json")
    report_parser.add_argument("--evaluations", nargs="+", required=True)
    report_parser.add_argument("--output", default="runs/phase2-report")
    return parser


def add_match_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--seat-a", required=True, help="random, heuristic, or model:/path/checkpoint.zip")
    parser.add_argument("--seat-b", required=True, help="random, heuristic, or model:/path/checkpoint.zip")
    add_seed_arguments(parser)
    parser.add_argument("--swap", action=argparse.BooleanOptionalAction, default=False)
    parser.add_argument("--device", default="cpu")
    parser.add_argument("--stochastic", action="store_true", help="sample model actions instead of argmax")
    parser.add_argument("--save-replays", choices=("none", "representative", "all"), default="representative")
    parser.add_argument("--authority-mode", choices=("normal", "ephemeral"), default="normal")
    parser.add_argument("--telemetry-mode", choices=("full", "training"), default="full")
    parser.add_argument("--transport-mode", choices=("full", "encoded", "parity"), default="full")
    parser.add_argument(
        "--observation-schema",
        choices=("dice-and-destiny-observation-v1", OBSERVATION_SCHEMA_V2, OBSERVATION_SCHEMA_V3),
        default="dice-and-destiny-observation-v1",
    )
    parser.add_argument("--profile", choices=("max", "balanced-80", "light-50", "custom"), default="custom")
    parser.add_argument("--workers", type=int, default=0, help="custom rollout-process budget")
    parser.add_argument("--torch-threads", type=int, default=0, help="custom Torch/BLAS threads per worker")
    parser.add_argument("--output", default="runs/evaluation")
    parser.add_argument("--observation-manifest", default="")
    parser.add_argument("--seat-a-definition", default="blade_warden")
    parser.add_argument("--seat-b-definition", default="blade_warden")


def add_campaign_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--campaign-id", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--observation-manifest", required=True)
    parser.add_argument("--registry", required=True)
    parser.add_argument("--initial-checkpoint", required=True)
    parser.add_argument("--seed-banks", required=True)
    parser.add_argument("--time-budget-seconds", type=float, default=10_800.0)
    parser.add_argument("--seat-a-definition", default="blade_warden")
    parser.add_argument("--seat-b-definition", default="blade_warden")
    parser.add_argument("--warm-start", default="mechanics-v2")
    parser.add_argument("--warm-start-evidence", required=True)
    parser.add_argument("--maximum-attempts", type=int, default=512)
    parser.add_argument("--family-plan", default="")
    parser.add_argument("--require-full-window", action=argparse.BooleanOptionalAction, default=False)
    parser.add_argument(
        "--allow-battery-power",
        action="store_true",
        help="owner-authorized override of the AC-only full-window preflight prerequisite",
    )
    parser.add_argument("--initial-selection-score", type=float, default=0.0)


def add_seed_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--episodes", type=int, default=100, help="seeds per seat assignment")
    parser.add_argument("--seed-start", type=int, default=9_000_000)
    parser.add_argument("--seeds-file", default="")


def load_seeds(args: argparse.Namespace) -> list[int]:
    if args.seeds_file:
        path = Path(args.seeds_file)
        value = json.loads(path.read_text())
        if isinstance(value, dict) and isinstance(value.get("seeds"), list):
            return read_seed_bank(path, expected_games=2 * len(value["seeds"]))
        if not isinstance(value, list):
            raise ValueError("seeds file must contain a JSON array or hashed campaign seed bank")
        return [int(seed) for seed in value]
    return list(range(args.seed_start, args.seed_start + args.episodes))


def smoke(binary: Path, server_root: Path, seed: int) -> dict:
    encoder = SchemaEncoder()
    seat_policies = {"seat-a": HeuristicPolicy(), "seat-b": HeuristicPolicy()}
    with AuthorityBridge(binary, server_root, session_id="smoke") as bridge:
        transition = bridge.reset(
            seed,
            {seat: policy.name for seat, policy in seat_policies.items()},
            battle_id=f"smoke-{seed}",
        )
        seat_b_view = bridge.observe("seat-b")
        seat_a_private = seat_b_view.get("snapshot", {}).get("actors", {}).get("seat-a") or {}
        if (
            seat_a_private.get("hand")
            or seat_a_private.get("card_instances")
            or seat_a_private.get("roll_history")
        ):
            raise RuntimeError("viewer-safe smoke test found opponent private planning data")
        for seat, policy in seat_policies.items():
            policy.reset(seed, seat)
        maximum_candidates = 0
        while not transition.get("terminal") and not transition.get("truncation_reason"):
            maximum_candidates = max(maximum_candidates, len(transition["result"].get("legal_actions") or []))
            actor = transition["actor_id"]
            transition = bridge.step(select_with_policy(seat_policies[actor], transition, encoder))
        if not transition.get("terminal"):
            raise RuntimeError(f"smoke battle truncated: {transition.get('truncation_reason')}")
        replayed = bridge.replay(transition["replay"])
        if replayed.get("winner") != transition.get("winner"):
            raise RuntimeError("recorded replay winner mismatch")
        return {
            "passed": True,
            "battle_id": transition["metrics"]["battle_id"],
            "winner": transition["winner"],
            "actions": transition["metrics"]["actions"],
            "rounds": transition["metrics"]["rounds"],
            "maximum_candidates": maximum_candidates,
            "authority_rejects": transition["metrics"]["authority_rejects"],
            "invalid_action_indices": transition["metrics"]["invalid_action_indices"],
            "replay_verified": True,
        }


if __name__ == "__main__":
    sys.exit(main())
