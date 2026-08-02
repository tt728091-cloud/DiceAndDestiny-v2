from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .bridge import AuthorityBridge
from .evaluation import evaluate
from .export_policy import export_phase3_policy
from .policies import HeuristicPolicy, select_with_policy
from .reporting import generate_report_artifacts
from .schema import SchemaEncoder
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
        result = export_phase3_policy(
            Path(args.checkpoint),
            Path(args.output),
            model_id=args.model_id,
            content_version=args.content_version,
            source_revision=args.source_revision,
            training_engine_revision=args.training_engine_revision,
        )
    elif args.command == "smoke":
        result = smoke(binary, server_root, args.seed)
    elif args.command in {"evaluate", "benchmark", "acceptance"}:
        seeds = load_seeds(args)
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
        summaries = []
        for seed in args.seeds:
            set_reproducible_runtime(seed)
            config = TrainingConfig(
                seed=seed,
                total_timesteps=args.timesteps,
                workers=args.workers,
                rollout_steps=args.rollout_steps,
                batch_size=args.batch_size,
                epochs=args.epochs,
                learning_rate=args.learning_rate,
                checkpoint_interval=args.checkpoint_interval,
                imitation_decisions=args.imitation_decisions,
                imitation_epochs=args.imitation_epochs,
                device=args.device,
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
    train_parser.add_argument("--workers", type=int, default=4)
    train_parser.add_argument("--rollout-steps", type=int, default=256)
    train_parser.add_argument("--batch-size", type=int, default=256)
    train_parser.add_argument("--epochs", type=int, default=8)
    train_parser.add_argument("--learning-rate", type=float, default=3e-4)
    train_parser.add_argument("--checkpoint-interval", type=int, default=5_000)
    train_parser.add_argument("--imitation-decisions", type=int, default=5_000)
    train_parser.add_argument("--imitation-epochs", type=int, default=5)
    train_parser.add_argument("--device", default="cpu")
    train_parser.add_argument("--output", default="runs/training")

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
