from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from dice_destiny_ml.evaluation import evaluate
from dice_destiny_ml.manifest_v5 import OBSERVATION_SCHEMA_V5
from dice_destiny_ml.schema_v2 import OBSERVATION_SCHEMA_V2


@dataclass(frozen=True)
class Audit:
    label: str
    campaign: str
    interval: int
    opponent_campaign: str
    opponent_interval: int
    observation_schema: str
    expected_wdl: tuple[int, int, int]


AUDITS = (
    Audit(
        "Last-night V5 eight-hour · CP7",
        "v5-relational-cp6-continuation-cp193-20260808-8h",
        7,
        "v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only",
        193,
        OBSERVATION_SCHEMA_V5,
        (326, 64, 610),
    ),
    Audit(
        "Recent looser-clipping · CP27",
        "v5-relational-cp12-looser-clipping-cp193-20260809-2h",
        27,
        "v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only",
        193,
        OBSERVATION_SCHEMA_V5,
        (347, 90, 563),
    ),
    Audit(
        "Global-champion lineage · CP193",
        "v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only",
        193,
        "v2-winner-health-fresh-20260805-1h",
        38,
        OBSERVATION_SCHEMA_V2,
        (590, 80, 330),
    ),
)


def _checkpoint(ml_root: Path, campaign: str, interval: int) -> Path:
    return (
        ml_root
        / "runs"
        / campaign
        / "intervals"
        / f"interval-{interval:03d}"
        / "raw-ppo-challenger.zip"
    ).resolve()


def _seed_bank(ml_root: Path, campaign: str, interval: int) -> Path:
    return (
        ml_root
        / "runs"
        / campaign
        / "seed-banks"
        / f"interval-{interval:03d}-global-1k.json"
    ).resolve()


def _policy_result(episodes_path: Path, focus_policy: str) -> dict[str, Any]:
    totals: dict[str, int] = {}
    wins = draws = losses = games = 0
    for line in episodes_path.read_text().splitlines():
        episode = json.loads(line)
        damage = (episode.get("damage_by_policy") or {}).get(focus_policy)
        if damage is None:
            raise RuntimeError(f"focus policy is absent from {episode['battle_id']}")
        for key, value in damage.items():
            totals[key] = totals.get(key, 0) + int(value)
        games += 1
        focus_seat = (
            "seat-a" if episode["seat_a_policy"] == focus_policy else "seat-b"
        )
        if episode["status"] == "draw":
            draws += 1
        elif episode["winner"] == focus_seat:
            wins += 1
        else:
            losses += 1
    rounds = totals["rounds"]
    average_fields = {
        key: value / rounds
        for key, value in totals.items()
        if key.startswith(("raw_", "resolved_", "actual_"))
    }
    return {
        "games": games,
        "wins": wins,
        "draws": draws,
        "losses": losses,
        "rounds": rounds,
        "average_rounds_per_game": rounds / games,
        "totals": totals,
        "per_round": average_fields,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--start-evaluation", type=int, default=1)
    args = parser.parse_args()
    ml_root = Path(__file__).resolve().parents[1]
    server_root = ml_root.parent
    output_root = ml_root / "runs/checkpoint-damage-comparison-20260809"
    output_root.mkdir(parents=True, exist_ok=True)
    manifest = (
        ml_root
        / "runs/v5-relational-fresh-cp193-20260808-1h/observation-manifest-v5.json"
    ).resolve()
    results = []
    for index, audit in enumerate(AUDITS, start=1):
        focus_checkpoint = _checkpoint(
            ml_root, audit.campaign, audit.interval
        )
        opponent_checkpoint = _checkpoint(
            ml_root, audit.opponent_campaign, audit.opponent_interval
        )
        focus_policy = f"model:{focus_checkpoint}"
        opponent_policy = f"model:{opponent_checkpoint}"
        bank_path = _seed_bank(ml_root, audit.campaign, audit.interval)
        bank = json.loads(bank_path.read_text())
        output_dir = output_root / f"evaluation-{index}"
        if index >= args.start_evaluation:
            print(
                json.dumps(
                    {
                        "damage_audit": "started",
                        "evaluation": index,
                        "label": audit.label,
                        "games": int(bank["seat_swapped_games"]),
                    },
                    sort_keys=True,
                ),
                flush=True,
            )
            evaluate(
                binary=server_root / "build/battle-ml-sim",
                server_root=server_root,
                seat_a_spec=focus_policy,
                seat_b_spec=opponent_policy,
                seeds=[int(seed) for seed in bank["seeds"]],
                output_dir=output_dir,
                swap=True,
                deterministic=True,
                save_replays="none",
                authority_mode="ephemeral",
                workers=12,
                torch_threads=1,
                profile="max",
                telemetry_mode="full",
                transport_mode="full",
                observation_schema=audit.observation_schema,
                observation_manifest=(
                    manifest if audit.observation_schema == OBSERVATION_SCHEMA_V5 else None
                ),
            )
        result = _policy_result(output_dir / "episodes.jsonl", focus_policy)
        observed_wdl = (result["wins"], result["draws"], result["losses"])
        result.update(
            {
                "label": audit.label,
                "focus_checkpoint": str(focus_checkpoint),
                "opponent_checkpoint": str(opponent_checkpoint),
                "seed_bank": str(bank_path),
                "seed_bank_sha256": bank["sha256"],
                "observation_schema": audit.observation_schema,
                "evaluation_output": str(output_dir.resolve()),
                "preserved_wdl": list(audit.expected_wdl),
                "matches_preserved_wdl": observed_wdl == audit.expected_wdl,
            }
        )
        results.append(result)
        print(
            json.dumps(
                {
                    "damage_audit": "completed",
                    "evaluation": index,
                    "label": audit.label,
                    "wdl": observed_wdl,
                    "rounds": result["rounds"],
                },
                sort_keys=True,
            ),
            flush=True,
        )
    report = {
        "schema": "dice-and-destiny-three-campaign-damage-audit-v1",
        "definitions": {
            "raw": (
                "sum of authority damage-source base amounts, including attacks, "
                "bleed, and poison, before prevention, scaling, reactions, and overkill"
            ),
            "resolved": (
                "sum of authority damage-source final amounts after prevention, "
                "scaling, and reactions but before overkill"
            ),
            "actual": (
                "accepted health-card removals; final health loss after all defenses, "
                "reactions, and overkill"
            ),
            "round": "one authority battle round; every game-round is weighted equally",
        },
        "evaluations": results,
    }
    report_path = output_root / "damage-comparison.json"
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    print(report_path.resolve(), flush=True)


if __name__ == "__main__":
    main()
