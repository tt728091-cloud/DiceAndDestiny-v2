from __future__ import annotations

import json
import math
import statistics
import time
from collections import Counter
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

from .bridge import AuthorityBridge
from .policies import Policy, build_policy, select_with_policy
from .schema import SchemaEncoder


@dataclass
class MatchRecord:
    seed: int
    battle_id: str
    seat_a_policy: str
    seat_b_policy: str
    winner: str
    status: str
    truncation_reason: str
    truncation_actor: str
    actions: int
    rounds: int
    remaining_health: dict[str, int]
    action_frequency: dict[str, int]
    authority_rejects: int
    invalid_action_indices: int
    stale_action_submissions: int
    wrong_seat_submissions: int
    duration_ms: float
    replay_path: str = ""


def evaluate(
    *,
    binary: Path,
    server_root: Path,
    seat_a_spec: str,
    seat_b_spec: str,
    seeds: list[int],
    output_dir: Path,
    swap: bool = False,
    device: str = "cpu",
    deterministic: bool = True,
    save_replays: str = "representative",
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    replay_dir = output_dir / "replays"
    replay_dir.mkdir(exist_ok=True)
    pairings = [(seat_a_spec, seat_b_spec)]
    if swap:
        pairings.append((seat_b_spec, seat_a_spec))
    records: list[MatchRecord] = []
    inference_samples: list[float] = []
    saved_representative = False
    saved_surprising = False
    started = time.perf_counter()
    with AuthorityBridge(binary, server_root, session_id="evaluation") as bridge:
        encoder = SchemaEncoder()
        policies: dict[tuple[str, str], Policy] = {}
        for pairing_index, (a_spec, b_spec) in enumerate(pairings):
            # The tuple key deliberately includes the seat: identical checkpoint
            # mirror play still constructs two independent inference instances.
            policies[("seat-a", a_spec)] = build_policy(a_spec, device=device, deterministic=deterministic)
            policies[("seat-b", b_spec)] = build_policy(b_spec, device=device, deterministic=deterministic)
            for seed in seeds:
                battle_id = f"eval-{pairing_index}-{seed}"
                seat_policies = {
                    "seat-a": policies[("seat-a", a_spec)],
                    "seat-b": policies[("seat-b", b_spec)],
                }
                for seat_id, policy in seat_policies.items():
                    policy.reset(seed, seat_id)
                transition = bridge.reset(
                    seed,
                    {"seat-a": seat_policies["seat-a"].name, "seat-b": seat_policies["seat-b"].name},
                    battle_id=battle_id,
                )
                while not transition.get("terminal") and not transition.get("truncation_reason"):
                    actor_id = transition.get("actor_id")
                    if actor_id not in seat_policies:
                        raise RuntimeError(f"authority requested unknown actor {actor_id!r}")
                    selected = select_with_policy(seat_policies[actor_id], transition, encoder)
                    transition = bridge.step(selected)
                metrics = transition.get("metrics") or {}
                replay_path = ""
                surprising = bool(transition.get("truncation_reason")) or metrics.get("status") == "draw"
                should_save = save_replays == "all" or (
                    save_replays == "representative"
                    and (not saved_representative or (surprising and not saved_surprising))
                )
                if should_save and transition.get("replay"):
                    path = replay_dir / f"{battle_id}.json"
                    path.write_text(json.dumps(transition["replay"], indent=2, sort_keys=True) + "\n")
                    replay_path = str(path)
                    saved_representative = True
                    saved_surprising = saved_surprising or surprising
                records.append(
                    MatchRecord(
                        seed=seed,
                        battle_id=metrics.get("battle_id", battle_id),
                        seat_a_policy=a_spec,
                        seat_b_policy=b_spec,
                        winner=metrics.get("winner", ""),
                        status=metrics.get("status", ""),
                        truncation_reason=metrics.get("truncation_reason", ""),
                        truncation_actor=metrics.get("truncation_actor", ""),
                        actions=int(metrics.get("actions", 0)),
                        rounds=int(metrics.get("rounds", 0)),
                        remaining_health=metrics.get("remaining_health") or {},
                        action_frequency=metrics.get("action_frequency") or {},
                        authority_rejects=int(metrics.get("authority_rejects", 0)),
                        invalid_action_indices=int(metrics.get("invalid_action_indices", 0)),
                        stale_action_submissions=int(metrics.get("stale_action_submissions", 0)),
                        wrong_seat_submissions=int(metrics.get("wrong_seat_submissions", 0)),
                        duration_ms=float(metrics.get("duration_ms", 0.0)),
                        replay_path=replay_path,
                    )
                )
                for policy in seat_policies.values():
                    inference_samples.extend(policy.inference_seconds)
    elapsed = time.perf_counter() - started
    summary = summarize(records, elapsed, inference_samples)
    (output_dir / "episodes.jsonl").write_text(
        "".join(json.dumps(asdict(record), sort_keys=True) + "\n" for record in records)
    )
    (output_dir / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
    return summary


def summarize(records: list[MatchRecord], elapsed: float, inference_samples: list[float]) -> dict[str, Any]:
    total = len(records)
    seat_wins = Counter(record.winner for record in records if record.winner)
    draws = sum(record.status == "draw" for record in records)
    truncations = sum(bool(record.truncation_reason) for record in records)
    policy_games: Counter[str] = Counter()
    policy_wins: Counter[str] = Counter()
    adjudicated_points: Counter[str] = Counter()
    action_frequency: Counter[str] = Counter()
    for record in records:
        policy_games[record.seat_a_policy] += 1
        policy_games[record.seat_b_policy] += 1
        if record.winner == "seat-a":
            policy_wins[record.seat_a_policy] += 1
            adjudicated_points[record.seat_a_policy] += 1
        elif record.winner == "seat-b":
            policy_wins[record.seat_b_policy] += 1
            adjudicated_points[record.seat_b_policy] += 1
        elif record.truncation_actor == "seat-a":
            adjudicated_points[record.seat_b_policy] += 1
        elif record.truncation_actor == "seat-b":
            adjudicated_points[record.seat_a_policy] += 1
        elif record.status == "draw":
            adjudicated_points[record.seat_a_policy] += 0.5
            adjudicated_points[record.seat_b_policy] += 0.5
        action_frequency.update(record.action_frequency)
    policies: dict[str, Any] = {}
    for policy, games in policy_games.items():
        wins = policy_wins[policy]
        lower, upper = wilson_interval(wins, games)
        policies[policy] = {
            "games": games,
            "wins": wins,
            "win_rate": wins / games if games else 0.0,
            "wilson_95": [lower, upper],
            "adjudicated_score": adjudicated_points[policy] / games if games else 0.0,
        }
    durations = [record.actions for record in records]
    health = [value for record in records for value in record.remaining_health.values()]
    return {
        "games": total,
        "seat_a_wins": seat_wins["seat-a"],
        "seat_b_wins": seat_wins["seat-b"],
        "seat_a_win_rate": seat_wins["seat-a"] / total if total else 0.0,
        "seat_b_win_rate": seat_wins["seat-b"] / total if total else 0.0,
        "draws": draws,
        "truncations": truncations,
        "authority_rejects": sum(record.authority_rejects for record in records),
        "invalid_action_indices": sum(record.invalid_action_indices for record in records),
        "stale_action_submissions": sum(record.stale_action_submissions for record in records),
        "wrong_seat_submissions": sum(record.wrong_seat_submissions for record in records),
        "games_per_second": total / elapsed if elapsed else 0.0,
        "elapsed_seconds": elapsed,
        "mean_actions": statistics.fmean(durations) if durations else 0.0,
        "median_actions": statistics.median(durations) if durations else 0.0,
        "p95_actions": percentile(durations, 0.95),
        "mean_remaining_health": statistics.fmean(health) if health else 0.0,
        "inference_latency_ms": {
            "mean": statistics.fmean(inference_samples) * 1000 if inference_samples else 0.0,
            "p95": percentile(inference_samples, 0.95) * 1000,
        },
        "action_frequency": dict(sorted(action_frequency.items())),
        "policies": policies,
    }


def wilson_interval(wins: int, games: int, z: float = 1.959963984540054) -> tuple[float, float]:
    if games == 0:
        return 0.0, 1.0
    proportion = wins / games
    denominator = 1.0 + z * z / games
    center = (proportion + z * z / (2.0 * games)) / denominator
    variance = proportion * (1.0 - proportion) / games + z * z / (4.0 * games * games)
    radius = z * math.sqrt(variance) / denominator
    return max(0.0, center - radius), min(1.0, center + radius)


def percentile(values: list[float] | list[int], quantile: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    position = (len(ordered) - 1) * quantile
    lower = math.floor(position)
    upper = math.ceil(position)
    if lower == upper:
        return float(ordered[lower])
    weight = position - lower
    return float(ordered[lower] * (1.0 - weight) + ordered[upper] * weight)
