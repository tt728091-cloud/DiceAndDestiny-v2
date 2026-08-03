from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .ablation import run_phase2_matrix
from .bridge import AuthorityBridge
from .diagnostics import run_owner_diagnostic
from .evaluation import evaluate
from .export_policy import export_candidate_policy_v2, export_phase3_policy
from .imitation import corrective_clone
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


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    server_root = Path(__file__).resolve().parents[2]
    binary = Path(args.binary) if args.binary else server_root / "build" / "battle-ml-sim"
    if args.command != "export-policy" and not binary.exists():
        parser.error(f"simulator binary not found: {binary}; run scripts/ml.sh so it is built first")
    if args.command == "export-policy":
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
    export_parser.add_argument("--policy-family", choices=("v1", "v2"), default="v1")

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
    train_parser.add_argument("--checkpoint-interval", type=int, default=5_000)
    train_parser.add_argument("--imitation-decisions", type=int, default=5_000)
    train_parser.add_argument("--imitation-epochs", type=int, default=5)
    train_parser.add_argument("--device", default="cpu")
    train_parser.add_argument(
        "--observation-schema",
        choices=("dice-and-destiny-observation-v1", OBSERVATION_SCHEMA_V2),
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
        choices=("category-balanced", "legacy-flat"),
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
        choices=("dice-and-destiny-observation-v1", OBSERVATION_SCHEMA_V2),
        default="dice-and-destiny-observation-v1",
    )
    parser.add_argument("--profile", choices=("max", "balanced-80", "light-50", "custom"), default="custom")
    parser.add_argument("--workers", type=int, default=0, help="custom rollout-process budget")
    parser.add_argument("--torch-threads", type=int, default=0, help="custom Torch/BLAS threads per worker")
    parser.add_argument("--output", default="runs/evaluation")


def add_seed_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--episodes", type=int, default=100, help="seeds per seat assignment")
    parser.add_argument("--seed-start", type=int, default=9_000_000)
    parser.add_argument("--seeds-file", default="")


def load_seeds(args: argparse.Namespace) -> list[int]:
    if args.seeds_file:
        value = json.loads(Path(args.seeds_file).read_text())
        if not isinstance(value, list):
            raise ValueError("seeds file must contain a JSON array")
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
