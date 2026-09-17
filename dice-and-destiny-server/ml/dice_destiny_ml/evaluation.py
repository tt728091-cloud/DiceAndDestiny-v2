from __future__ import annotations

import json
import math
import multiprocessing
import os
import resource
import statistics
import sys
import time
from collections import Counter
from concurrent.futures import ProcessPoolExecutor
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

from .bridge import AuthorityBridge
from .evaluation_report import write_evaluation_statistics_html
from .evaluation_statistics import percentile
from .manifest_v3 import OBSERVATION_SCHEMA_V3, ObservationManifestV3
from .manifest_v4 import OBSERVATION_SCHEMA_V4, ObservationManifestV4
from .manifest_v5 import OBSERVATION_SCHEMA_V5, ObservationManifestV5
from .policies import Policy, build_policy, select_with_policy
from .resources import ForegroundResponsivenessProbe, capture_host_state
from .schema import SchemaEncoder
from .schema_v2 import OBSERVATION_SCHEMA_V2, SchemaEncoderV2
from .schema_v3 import SchemaEncoderV3
from .schema_v4 import SchemaEncoderV4
from .schema_v5 import SchemaEncoderV5


@dataclass
class MatchRecord:
    seed: int
    battle_id: str
    seat_a_policy: str
    seat_b_policy: str
    seat_a_definition: str
    seat_b_definition: str
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
    random_cursor: int
    duration_ms: float
    maximum_candidates: int = 0
    replay_path: str = ""
    behavior: dict[str, float | int] | None = None
    behavior_by_policy: dict[str, dict[str, float | int]] | None = None
    damage_by_policy: dict[str, dict[str, float | int]] | None = None


@dataclass(frozen=True)
class MatchTask:
    index: int
    pairing_index: int
    seed: int
    seat_a_spec: str
    seat_b_spec: str


@dataclass
class WorkerOutput:
    records: list[tuple[int, MatchRecord]]
    inference_samples: list[float]
    replay_candidates: list[tuple[int, bool, dict[str, Any]]]
    cpu_user_seconds: float
    cpu_system_seconds: float
    max_rss_bytes: int
    step_samples: list[float]
    artifact_io_seconds: float


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
    authority_mode: str = "normal",
    workers: int = 1,
    torch_threads: int = 1,
    profile: str = "custom",
    telemetry_mode: str = "full",
    transport_mode: str = "full",
    observation_schema: str = "dice-and-destiny-observation-v1",
    observation_manifest: Path | None = None,
    seat_a_definition: str = "blade_warden",
    seat_b_definition: str = "blade_warden",
) -> dict[str, Any]:
    if workers < 1:
        raise ValueError("workers must be positive")
    if torch_threads < 1:
        raise ValueError("torch_threads must be positive")
    output_dir.mkdir(parents=True, exist_ok=True)
    replay_dir = output_dir / "replays"
    replay_dir.mkdir(exist_ok=True)
    pairings = [(seat_a_spec, seat_b_spec)]
    if swap:
        pairings.append((seat_b_spec, seat_a_spec))
    tasks = [
        MatchTask(index, pairing_index, seed, a_spec, b_spec)
        for index, (pairing_index, (a_spec, b_spec), seed) in enumerate(
            (pairing_index, pairing, seed) for pairing_index, pairing in enumerate(pairings) for seed in seeds
        )
    ]
    worker_count = min(workers, len(tasks)) if tasks else 1
    shards = [tasks[index::worker_count] for index in range(worker_count)]
    # Spawned workers import NumPy/Torch after inheriting these caps. Torch is
    # also configured explicitly inside each worker before model construction.
    for variable in ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS", "VECLIB_MAXIMUM_THREADS"):
        os.environ[variable] = str(torch_threads)
    host_before = capture_host_state()
    responsiveness = ForegroundResponsivenessProbe()
    responsiveness.start()
    started = time.perf_counter()
    arguments = [
        (
            worker,
            str(binary),
            str(server_root),
            shard,
            device,
            deterministic,
            authority_mode,
            torch_threads,
            save_replays,
            str(replay_dir),
            telemetry_mode,
            transport_mode,
            observation_schema,
            str(observation_manifest) if observation_manifest else "",
            seat_a_definition,
            seat_b_definition,
        )
        for worker, shard in enumerate(shards)
    ]
    if worker_count == 1:
        outputs = [_evaluate_shard(*arguments[0])]
    else:
        context = multiprocessing.get_context("spawn")
        with ProcessPoolExecutor(max_workers=worker_count, mp_context=context) as executor:
            outputs = list(executor.map(_evaluate_shard_from_tuple, arguments))
    elapsed = time.perf_counter() - started
    responsiveness_result = responsiveness.stop()
    host_after = capture_host_state()
    indexed_records = [item for output in outputs for item in output.records]
    records = [record for _, record in sorted(indexed_records)]
    inference_samples = [sample for output in outputs for sample in output.inference_samples]
    step_samples = [sample for output in outputs for sample in output.step_samples]
    artifact_io_seconds = sum(output.artifact_io_seconds for output in outputs)
    if save_replays == "representative":
        replay_io_started = time.perf_counter()
        _save_representative_replays(records, outputs, replay_dir)
        artifact_io_seconds += time.perf_counter() - replay_io_started
    summary = summarize(records, elapsed, inference_samples)
    summary["runtime"] = {
        "profile": profile,
        "workers": worker_count,
        "requested_workers": workers,
        "torch_threads_per_worker": torch_threads,
        "blas_threads_per_worker": torch_threads,
        "authority_mode": authority_mode,
        "telemetry_mode": telemetry_mode,
        "transport_mode": transport_mode,
        "observation_schema": observation_schema,
        "observation_manifest": str(observation_manifest) if observation_manifest else "",
        "seat_definitions": {
            "seat-a": seat_a_definition,
            "seat-b": seat_b_definition,
        },
        "inference_concurrency": worker_count,
        "io_concurrency": min(2, worker_count),
        "logical_cpus": os.cpu_count() or 1,
    }
    summary["timing"] = {
        "collection_seconds": elapsed,
        "aggregate_inference_seconds": sum(inference_samples),
        "aggregate_step_transport_seconds": sum(step_samples),
        "step_latency_ms": {
            "p50": percentile(step_samples, 0.50) * 1000,
            "p95": percentile(step_samples, 0.95) * 1000,
        },
    }
    cpu_seconds = sum(output.cpu_user_seconds + output.cpu_system_seconds for output in outputs)
    logical_cpus = os.cpu_count() or 1
    summary["resources"] = {
        "aggregate_cpu_seconds": cpu_seconds,
        "average_logical_cores": cpu_seconds / elapsed if elapsed else 0.0,
        "normalized_whole_machine_cpu_percent": (
            100.0 * cpu_seconds / (elapsed * logical_cpus) if elapsed else 0.0
        ),
        "summed_worker_peak_rss_bytes": sum(output.max_rss_bytes for output in outputs),
        "host_before": host_before,
        "host_after": host_after,
        "foreground_responsiveness": responsiveness_result,
    }
    io_started = time.perf_counter()
    (output_dir / "episodes.jsonl").write_text(
        "".join(json.dumps(asdict(record), sort_keys=True) + "\n" for record in records)
    )
    artifact_io_seconds += time.perf_counter() - io_started
    summary["timing"]["artifact_io_seconds"] = artifact_io_seconds
    (output_dir / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
    write_evaluation_statistics_html(
        summary,
        output_dir / "statistics.html",
        title=f"Evaluation statistics · {seat_a_spec} vs {seat_b_spec}",
        focus_policy=seat_a_spec,
    )
    return summary


def _evaluate_shard_from_tuple(arguments: tuple[Any, ...]) -> WorkerOutput:
    return _evaluate_shard(*arguments)


def _evaluate_shard(
    worker: int,
    binary: str,
    server_root: str,
    tasks: list[MatchTask],
    device: str,
    deterministic: bool,
    authority_mode: str,
    torch_threads: int,
    save_replays: str,
    replay_dir: str,
    telemetry_mode: str,
    transport_mode: str,
    observation_schema: str,
    observation_manifest: str,
    seat_a_definition: str,
    seat_b_definition: str,
) -> WorkerOutput:
    usage_self_before = resource.getrusage(resource.RUSAGE_SELF)
    usage_children_before = resource.getrusage(resource.RUSAGE_CHILDREN)
    import torch

    torch.set_num_threads(torch_threads)
    try:
        torch.set_num_interop_threads(torch_threads)
    except RuntimeError:
        pass
    records: list[tuple[int, MatchRecord]] = []
    inference_samples: list[float] = []
    replay_candidates: list[tuple[int, bool, dict[str, Any]]] = []
    artifact_io_seconds = 0.0
    policies: dict[tuple[str, str], Policy] = {}
    if observation_schema == OBSERVATION_SCHEMA_V5:
        if not observation_manifest:
            raise ValueError("observation v5 evaluation requires a frozen manifest")
        encoder = SchemaEncoderV5(ObservationManifestV5.load(Path(observation_manifest)))
    elif observation_schema == OBSERVATION_SCHEMA_V4:
        if not observation_manifest:
            raise ValueError("observation v4 evaluation requires a frozen manifest")
        encoder = SchemaEncoderV4(ObservationManifestV4.load(Path(observation_manifest)))
    elif observation_schema == OBSERVATION_SCHEMA_V3:
        if not observation_manifest:
            raise ValueError("observation v3 evaluation requires a frozen manifest")
        encoder: SchemaEncoder | SchemaEncoderV2 | SchemaEncoderV3 = SchemaEncoderV3(
            ObservationManifestV3.load(Path(observation_manifest))
        )
    elif observation_schema == OBSERVATION_SCHEMA_V2:
        encoder = SchemaEncoderV2()
    else:
        encoder = SchemaEncoder()
    with AuthorityBridge(
        Path(binary),
        Path(server_root),
        session_id=f"evaluation-{worker}",
        authority_mode=authority_mode,
        telemetry_mode=telemetry_mode,
        transport_mode=transport_mode,
        observation_schema=OBSERVATION_SCHEMA_V2
        if observation_schema in {OBSERVATION_SCHEMA_V4, OBSERVATION_SCHEMA_V5}
        else observation_schema,
        observation_manifest=None
        if observation_schema in {OBSERVATION_SCHEMA_V4, OBSERVATION_SCHEMA_V5}
        else Path(observation_manifest)
        if observation_manifest
        else None,
    ) as bridge:
        for local_index, task in enumerate(tasks):
            for seat_id, specification in (("seat-a", task.seat_a_spec), ("seat-b", task.seat_b_spec)):
                key = (seat_id, specification)
                if key not in policies:
                    policies[key] = build_policy(specification, device=device, deterministic=deterministic)
            seat_policies = {
                "seat-a": policies[("seat-a", task.seat_a_spec)],
                "seat-b": policies[("seat-b", task.seat_b_spec)],
            }
            for seat_id, policy in seat_policies.items():
                policy.reset(task.seed, seat_id)
            battle_id = f"eval-{task.pairing_index}-{task.seed}"
            transition = bridge.reset(
                task.seed,
                {"seat-a": seat_policies["seat-a"].name, "seat-b": seat_policies["seat-b"].name},
                battle_id=battle_id,
                seat_definitions={
                    "seat-a": seat_a_definition,
                    "seat-b": seat_b_definition,
                },
            )
            behavior: dict[str, int | float] = {
                "qualified_roll_decisions": 0,
                "qualified_rerolls": 0,
                "qualified_immediate_abilities": 0,
                "ability_selections": 0,
                "rolls_used_at_selection_total": 0,
            }
            maximum_candidates = 0
            behavior_by_policy: dict[str, dict[str, int | float]] = {}
            offensive_segments_by_policy: dict[str, set[tuple[Any, ...]]] = {}
            offensive_plans: dict[tuple[int, str], dict[str, Any]] = {}
            while not transition.get("terminal") and not transition.get("truncation_reason"):
                maximum_candidates = max(
                    maximum_candidates,
                    len((transition.get("result") or {}).get("legal_actions") or []),
                )
                actor_id = transition.get("actor_id")
                if actor_id not in seat_policies:
                    raise RuntimeError(f"authority requested unknown actor {actor_id!r}")
                if transport_mode == "parity":
                    encoder.assert_transport_parity(transition)
                selected = select_with_policy(seat_policies[actor_id], transition, encoder)
                _record_behavior(behavior, transition, selected)
                policy_spec = task.seat_a_spec if actor_id == "seat-a" else task.seat_b_spec
                policy_behavior = behavior_by_policy.setdefault(policy_spec, {})
                _record_behavior(
                    policy_behavior,
                    transition,
                    selected,
                    detailed=True,
                    offensive_segments=offensive_segments_by_policy.setdefault(policy_spec, set()),
                )
                plan = _offensive_plan(transition, selected)
                if plan is not None:
                    offensive_plans[(int(plan["round"]), str(actor_id))] = plan
                previous = transition
                transition = bridge.step(selected)
                _record_transition_outcome(policy_behavior, previous, transition, str(actor_id))
                _record_completed_offensive_reaction(
                    behavior_by_policy,
                    offensive_plans,
                    previous,
                    transition,
                    {
                        "seat-a": task.seat_a_spec,
                        "seat-b": task.seat_b_spec,
                    },
                )
            metrics = transition.get("metrics") or {}
            damage_by_policy = _damage_by_policy(
                metrics.get("damage_by_seat") or {},
                {
                    "seat-a": task.seat_a_spec,
                    "seat-b": task.seat_b_spec,
                },
                int(metrics.get("rounds", 0)),
            )
            surprising = bool(transition.get("truncation_reason")) or metrics.get("status") == "draw"
            replay_path = ""
            replay = transition.get("replay")
            if save_replays == "all" and replay:
                path = Path(replay_dir) / f"{battle_id}.json"
                io_started = time.perf_counter()
                path.write_text(json.dumps(replay, indent=2, sort_keys=True) + "\n")
                artifact_io_seconds += time.perf_counter() - io_started
                replay_path = str(path)
            elif save_replays == "representative" and replay and (local_index == 0 or surprising):
                replay_candidates.append((task.index, surprising, replay))
            records.append(
                (
                    task.index,
                    MatchRecord(
                        seed=task.seed,
                        battle_id=metrics.get("battle_id", battle_id),
                        seat_a_policy=task.seat_a_spec,
                        seat_b_policy=task.seat_b_spec,
                        seat_a_definition=seat_a_definition,
                        seat_b_definition=seat_b_definition,
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
                        random_cursor=int(metrics.get("random_cursor", 0)),
                        duration_ms=float(metrics.get("duration_ms", 0.0)),
                        maximum_candidates=maximum_candidates,
                        replay_path=replay_path,
                        behavior=behavior,
                        behavior_by_policy=behavior_by_policy,
                        damage_by_policy=damage_by_policy,
                    ),
                )
            )
            for policy in seat_policies.values():
                inference_samples.extend(policy.inference_seconds)
    usage_self_after = resource.getrusage(resource.RUSAGE_SELF)
    usage_children_after = resource.getrusage(resource.RUSAGE_CHILDREN)
    cpu_user_seconds = (
        usage_self_after.ru_utime
        - usage_self_before.ru_utime
        + usage_children_after.ru_utime
        - usage_children_before.ru_utime
    )
    cpu_system_seconds = (
        usage_self_after.ru_stime
        - usage_self_before.ru_stime
        + usage_children_after.ru_stime
        - usage_children_before.ru_stime
    )
    rss_scale = 1 if sys.platform == "darwin" else 1024
    max_rss_bytes = int((usage_self_after.ru_maxrss + usage_children_after.ru_maxrss) * rss_scale)
    return WorkerOutput(
        records,
        inference_samples,
        replay_candidates,
        cpu_user_seconds,
        cpu_system_seconds,
        max_rss_bytes,
        list(bridge.operation_seconds.get("step", [])),
        artifact_io_seconds,
    )


def _save_representative_replays(
    records: list[MatchRecord], outputs: list[WorkerOutput], replay_dir: Path
) -> None:
    candidates = sorted(candidate for output in outputs for candidate in output.replay_candidates)
    if not candidates:
        return
    selected = [candidates[0]]
    surprising = next((candidate for candidate in candidates if candidate[1]), None)
    if surprising is not None and surprising[0] != selected[0][0]:
        selected.append(surprising)
    for index, _, replay in selected:
        path = replay_dir / f"{records[index].battle_id}.json"
        path.write_text(json.dumps(replay, indent=2, sort_keys=True) + "\n")
        records[index].replay_path = str(path)


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
    game_latencies = [record.duration_ms for record in records]
    health = [value for record in records for value in record.remaining_health.values()]
    behavior_totals: Counter[str] = Counter()
    behavior_by_policy: dict[str, Counter[str]] = {}
    damage_by_policy: dict[str, Counter[str]] = {}
    for record in records:
        behavior_totals.update(record.behavior or {})
        for policy, values in (record.behavior_by_policy or {}).items():
            behavior_by_policy.setdefault(policy, Counter()).update(values)
        for policy, values in (record.damage_by_policy or {}).items():
            damage_by_policy.setdefault(policy, Counter()).update(values)
    selections = behavior_totals["ability_selections"]
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
        "maximum_candidates": max((record.maximum_candidates for record in records), default=0),
        "median_actions": statistics.median(durations) if durations else 0.0,
        "p95_actions": percentile(durations, 0.95),
        "game_latency_ms": {
            "p50": percentile(game_latencies, 0.50),
            "p95": percentile(game_latencies, 0.95),
        },
        "mean_remaining_health": statistics.fmean(health) if health else 0.0,
        "inference_latency_ms": {
            "mean": statistics.fmean(inference_samples) * 1000 if inference_samples else 0.0,
            "p95": percentile(inference_samples, 0.95) * 1000,
        },
        "action_frequency": dict(sorted(action_frequency.items())),
        "policies": policies,
        "behavior": {
            **dict(sorted(behavior_totals.items())),
            "mean_rolls_used_at_ability_selection": (
                behavior_totals["rolls_used_at_selection_total"] / selections if selections else 0.0
            ),
        },
        "behavior_by_policy": {
            policy: _summarize_behavior(values) for policy, values in sorted(behavior_by_policy.items())
        },
        "damage_by_policy": {
            policy: _summarize_damage(values) for policy, values in sorted(damage_by_policy.items())
        },
    }


def _record_behavior(
    counters: dict[str, int | float],
    transition: dict[str, Any],
    selected: int,
    *,
    detailed: bool = False,
    offensive_segments: set[tuple[Any, ...]] | None = None,
) -> None:
    result = transition.get("result") or {}
    actions = result.get("legal_actions") or []
    if not actions or selected >= len(actions):
        return
    action = actions[selected]
    snapshot = result.get("snapshot") or {}
    actor_id = snapshot.get("viewer_actor_id") or transition.get("actor_id")
    actor = (snapshot.get("actors") or {}).get(actor_id) or {}
    dice = actor.get("dice") or {}
    qualified = actor.get("qualified_abilities") or []
    kind = action.get("type", "")
    payload = action.get("payload") or {}
    if isinstance(payload, str):
        payload = json.loads(payload)
    if detailed:
        counters["decisions"] = counters.get("decisions", 0) + 1
        counters[f"action_{kind}"] = counters.get(f"action_{kind}", 0) + 1
        segment = str(snapshot.get("segment", "unknown"))
        counters[f"segment_action::{segment}"] = counters.get(f"segment_action::{segment}", 0) + 1
        legal_kinds = {str(candidate.get("type", "")) for candidate in actions}
        for legal_kind in legal_kinds:
            counters[f"action_offered::{legal_kind}"] = counters.get(f"action_offered::{legal_kind}", 0) + 1
        _record_available_content(counters, actions, actor, snapshot.get("content_catalog") or {})
        selected_cards = _card_definition_ids(payload, actor)
        for definition_id in selected_cards:
            counters[f"card_selected::{definition_id}"] = (
                counters.get(f"card_selected::{definition_id}", 0) + 1
            )
        selected_ability = str(payload.get("ability_id", ""))
        if selected_ability:
            counters[f"ability_selected::{selected_ability}"] = (
                counters.get(f"ability_selected::{selected_ability}", 0) + 1
            )
        if kind in {"pass", "planning_pass"}:
            counters["passes"] = counters.get("passes", 0) + 1
            productive = legal_kinds - {"pass", "planning_pass", "planning_keep"}
            if productive:
                counters["passes_with_productive_option"] = (
                    counters.get("passes_with_productive_option", 0) + 1
                )
            else:
                counters["passes_without_productive_option"] = (
                    counters.get("passes_without_productive_option", 0) + 1
                )
        if segment == "offensive" and offensive_segments is not None:
            flow = snapshot.get("flow") or {}
            key = (
                actor_id,
                snapshot.get("round"),
                flow.get("iteration"),
                flow.get("planning_cycle"),
            )
            if key not in offensive_segments:
                offensive_segments.add(key)
                counters["offensive_segments"] = counters.get("offensive_segments", 0) + 1
    if qualified and float(dice.get("rolls_remaining", 0)) > 0:
        counters["qualified_roll_decisions"] = counters.get("qualified_roll_decisions", 0) + 1
        if kind == "planning_reroll":
            counters["qualified_rerolls"] = counters.get("qualified_rerolls", 0) + 1
            if detailed:
                counters["qualified_attacks_declined"] = counters.get("qualified_attacks_declined", 0) + 1
        elif kind == "planning_select_ability":
            counters["qualified_immediate_abilities"] = counters.get("qualified_immediate_abilities", 0) + 1
    if detailed and kind == "planning_reroll":
        subset_size = len(payload.get("reroll_indices") or [])
        key = f"reroll_subset_size_{subset_size}"
        counters[key] = counters.get(key, 0) + 1
    if kind == "planning_select_ability":
        rolls_used = int(dice.get("rolls_used", 0))
        counters["ability_selections"] = counters.get("ability_selections", 0) + 1
        counters["rolls_used_at_selection_total"] = (
            counters.get("rolls_used_at_selection_total", 0) + rolls_used
        )
        if detailed and segment == "offensive":
            counters["offensive_pre_reaction_outcomes"] = (
                counters.get("offensive_pre_reaction_outcomes", 0) + 1
            )
            counters["offensive_pre_reaction_ability_selected"] = (
                counters.get("offensive_pre_reaction_ability_selected", 0) + 1
            )
            counters[f"offensive_pre_reaction_selected_roll_{rolls_used}"] = (
                counters.get(f"offensive_pre_reaction_selected_roll_{rolls_used}", 0) + 1
            )
            counters[f"ability_selected_roll_{rolls_used}"] = (
                counters.get(f"ability_selected_roll_{rolls_used}", 0) + 1
            )
            _record_ability_effects(counters, payload, actor, snapshot.get("content_catalog") or {})
    elif detailed and segment == "offensive" and kind == "planning_pass":
        counters["offensive_pre_reaction_outcomes"] = (
            counters.get("offensive_pre_reaction_outcomes", 0) + 1
        )
        counters["offensive_pre_reaction_passed"] = (
            counters.get("offensive_pre_reaction_passed", 0) + 1
        )


def _offensive_plan(transition: dict[str, Any], selected: int) -> dict[str, Any] | None:
    result = transition.get("result") or {}
    actions = result.get("legal_actions") or []
    if selected < 0 or selected >= len(actions):
        return None
    action = actions[selected]
    kind = str(action.get("type", ""))
    if kind not in {"planning_select_ability", "planning_pass"}:
        return None
    payload = action.get("payload") or {}
    if isinstance(payload, str):
        payload = json.loads(payload)
    snapshot = result.get("snapshot") or {}
    if str(snapshot.get("segment", "")) != "offensive":
        return None
    actor_id = str(snapshot.get("viewer_actor_id") or transition.get("actor_id") or "")
    actor = (snapshot.get("actors") or {}).get(actor_id) or {}
    return {
        "round": int(snapshot.get("round", 0)),
        "actor_id": actor_id,
        "selected_ability": str(payload.get("ability_id", "")) if kind == "planning_select_ability" else "",
        "rolls_used": int((actor.get("dice") or {}).get("rolls_used", 0)),
    }


def _record_completed_offensive_reaction(
    behavior_by_policy: dict[str, dict[str, int | float]],
    offensive_plans: dict[tuple[int, str], dict[str, Any]],
    before: dict[str, Any],
    after: dict[str, Any],
    seat_policies: dict[str, str],
) -> None:
    before_snapshot = (before.get("result") or {}).get("snapshot") or {}
    after_snapshot = (after.get("result") or {}).get("snapshot") or {}
    if str(before_snapshot.get("stage", "")) != "offensive_reaction":
        return
    if (
        str(after_snapshot.get("stage", "")) == "offensive_reaction"
        and str(after_snapshot.get("segment", "")) == "offensive"
    ):
        return

    round_number = int(before_snapshot.get("round", 0))
    actors = before_snapshot.get("actors") or {}
    for actor_id, policy in seat_policies.items():
        plan = offensive_plans.pop((round_number, actor_id), None)
        if plan is None:
            continue
        counters = behavior_by_policy.setdefault(policy, {})
        final_ability = str((actors.get(actor_id) or {}).get("selected_ability", ""))
        initial_ability = str(plan.get("selected_ability", ""))
        counters["offensive_post_reaction_outcomes"] = (
            counters.get("offensive_post_reaction_outcomes", 0) + 1
        )
        outcome = "ability_selected" if final_ability else "passed"
        counters[f"offensive_post_reaction_{outcome}"] = (
            counters.get(f"offensive_post_reaction_{outcome}", 0) + 1
        )
        if initial_ability and final_ability == initial_ability:
            change = "ability_preserved"
        elif initial_ability and not final_ability:
            change = "ability_lost"
        elif not initial_ability and final_ability:
            change = "ability_gained"
        elif initial_ability and final_ability != initial_ability:
            change = "ability_switched"
        else:
            change = "pass_preserved"
        counters[f"offensive_reaction_{change}"] = (
            counters.get(f"offensive_reaction_{change}", 0) + 1
        )


def _record_available_content(
    counters: dict[str, int | float],
    actions: list[dict[str, Any]],
    actor: dict[str, Any],
    catalog: dict[str, Any],
) -> None:
    del catalog
    abilities: set[str] = set()
    cards: set[str] = set()
    for action in actions:
        payload = action.get("payload") or {}
        if isinstance(payload, str):
            payload = json.loads(payload)
        ability_id = str(payload.get("ability_id", ""))
        if ability_id:
            abilities.add(ability_id)
        cards.update(_card_definition_ids(payload, actor))
    for ability_id in abilities:
        counters[f"ability_offered::{ability_id}"] = counters.get(f"ability_offered::{ability_id}", 0) + 1
    for definition_id in cards:
        counters[f"card_offered::{definition_id}"] = counters.get(f"card_offered::{definition_id}", 0) + 1


def _card_definition_ids(payload: dict[str, Any], actor: dict[str, Any]) -> set[str]:
    commitment = payload.get("commitment") or {}
    identifiers = payload.get("card_ids") or commitment.get("card_ids") or []
    instances = actor.get("card_instances") or {}
    return {
        str((instances.get(instance_id) or {}).get("definition_id", ""))
        for instance_id in identifiers
        if (instances.get(instance_id) or {}).get("definition_id")
    }


def _record_transition_outcome(
    counters: dict[str, int | float],
    before: dict[str, Any],
    after: dict[str, Any],
    actor_id: str,
) -> None:
    before_snapshot = (before.get("result") or {}).get("snapshot") or {}
    after_snapshot = (after.get("result") or {}).get("snapshot") or {}
    before_actors = before_snapshot.get("actors") or {}
    after_actors = after_snapshot.get("actors") or {}
    opponent_id = "seat-b" if actor_id == "seat-a" else "seat-a"
    own_before = before_actors.get(actor_id) or {}
    own_after = after_actors.get(actor_id) or {}
    other_before = before_actors.get(opponent_id) or {}
    other_after = after_actors.get(opponent_id) or {}

    damage = max(
        float(other_before.get("current_health", 0)) - float(other_after.get("current_health", 0)),
        0.0,
    )
    healing = max(
        float(own_after.get("current_health", 0)) - float(own_before.get("current_health", 0)),
        0.0,
    )
    counters["actual_damage_dealt"] = counters.get("actual_damage_dealt", 0.0) + damage
    counters["actual_healing_received"] = counters.get("actual_healing_received", 0.0) + healing
    if str(before_snapshot.get("segment", "")) == "offensive":
        counters["offensive_actual_damage"] = counters.get("offensive_actual_damage", 0.0) + damage

    energy_spent = max(
        float(own_before.get("energy_points", 0)) - float(own_after.get("energy_points", 0)),
        0.0,
    )
    counters["energy_spent"] = counters.get("energy_spent", 0.0) + energy_spent
    before_statuses = _status_stacks(other_before)
    after_statuses = _status_stacks(other_after)
    applied_total = 0
    for definition_id in sorted(set(before_statuses) | set(after_statuses)):
        increase = max(after_statuses.get(definition_id, 0) - before_statuses.get(definition_id, 0), 0)
        if increase:
            applied_total += increase
            counters[f"status_applied_events::{definition_id}"] = (
                counters.get(f"status_applied_events::{definition_id}", 0) + 1
            )
            counters[f"status_applied_stacks::{definition_id}"] = (
                counters.get(f"status_applied_stacks::{definition_id}", 0) + increase
            )
    counters["status_stacks_applied"] = counters.get("status_stacks_applied", 0) + applied_total


def _damage_by_policy(
    damage_by_seat: dict[str, dict[str, Any]],
    seat_policies: dict[str, str],
    rounds: int,
) -> dict[str, dict[str, float | int]]:
    seats = tuple(sorted(seat_policies))
    by_seat: dict[str, Counter[str]] = {seat: Counter() for seat in seats}
    for target in seats:
        incoming = damage_by_seat.get(target) or {}
        opponents = [seat for seat in seats if seat != target]
        if len(opponents) != 1:
            continue
        dealer = opponents[0]
        for category in ("attack", "bleed", "poison"):
            amount = int(incoming.get(f"raw_{category}", 0))
            by_seat[dealer][f"raw_outgoing_{category}"] += amount
            by_seat[target][f"raw_incoming_{category}"] += amount
        raw_total = int(incoming.get("raw_total", 0))
        resolved_total = int(incoming.get("resolved_total", 0))
        actual_total = int(incoming.get("actual_total", 0))
        by_seat[dealer]["raw_outgoing_total"] += raw_total
        by_seat[dealer]["resolved_outgoing_total"] += resolved_total
        by_seat[dealer]["actual_outgoing_total"] += actual_total
        by_seat[target]["raw_incoming_total"] += raw_total
        by_seat[target]["resolved_incoming_total"] += resolved_total
        by_seat[target]["actual_incoming_total"] += actual_total

    result: dict[str, Counter[str]] = {}
    for seat, policy in seat_policies.items():
        counters = result.setdefault(policy, Counter())
        counters.update(by_seat[seat])
        counters["rounds"] += rounds
        counters["games"] += 1
    fields = (
        "games",
        "rounds",
        "raw_outgoing_attack",
        "raw_outgoing_bleed",
        "raw_outgoing_poison",
        "raw_outgoing_total",
        "resolved_outgoing_total",
        "actual_outgoing_total",
        "raw_incoming_attack",
        "raw_incoming_bleed",
        "raw_incoming_poison",
        "raw_incoming_total",
        "resolved_incoming_total",
        "actual_incoming_total",
    )
    return {
        policy: {field: int(counters[field]) for field in fields}
        for policy, counters in result.items()
    }


def _status_stacks(actor: dict[str, Any]) -> dict[str, int]:
    result: dict[str, int] = {}
    for status in actor.get("statuses") or []:
        definition_id = str(status.get("definition_id", ""))
        if definition_id:
            result[definition_id] = result.get(definition_id, 0) + int(status.get("stacks", 0))
    return result


def _record_ability_effects(
    counters: dict[str, int | float], payload: dict[str, Any], actor: dict[str, Any], catalog: dict[str, Any]
) -> None:
    ability_id = payload.get("ability_id", "")
    ability = SchemaEncoderV2.effective_ability(ability_id, actor, catalog)
    dice = (actor.get("dice") or {}).get("dice") or []
    encoder = SchemaEncoderV2()
    met = [
        tier
        for tier in (ability.get("qualification") or {}).get("activation_tiers") or []
        if encoder.tier_met(tier, dice)
    ]
    if met:
        counters[f"commitment_tier_{len(met)}"] = counters.get(f"commitment_tier_{len(met)}", 0) + 1
    operations = [operation for tier in met for operation in tier.get("operations") or []]
    summary = encoder.operation_summary(operations)
    for index, name in enumerate(("damage", "prevention", "status", "resource", "roll", "modifier")):
        if summary[index] > 0:
            counters[f"ability_effect_{name}"] = counters.get(f"ability_effect_{name}", 0) + 1


def _summarize_behavior(values: Counter[str]) -> dict[str, float | int]:
    result: dict[str, float | int] = dict(sorted(values.items()))
    selections = values["ability_selections"]
    result["mean_rolls_used_at_ability_selection"] = (
        values["rolls_used_at_selection_total"] / selections if selections else 0.0
    )
    qualified = values["qualified_roll_decisions"]
    result["qualified_reroll_rate"] = values["qualified_rerolls"] / qualified if qualified else 0.0
    offensive_segments = values["offensive_segments"]
    result["average_damage_per_offensive_segment"] = (
        values["actual_damage_dealt"] / offensive_segments if offensive_segments else 0.0
    )
    result["average_status_stacks_per_offensive_segment"] = (
        values["status_stacks_applied"] / offensive_segments if offensive_segments else 0.0
    )
    decisions = values["decisions"]
    result["pass_rate"] = values["passes"] / decisions if decisions else 0.0
    pre_outcomes = values["offensive_pre_reaction_outcomes"]
    post_outcomes = values["offensive_post_reaction_outcomes"]
    result["offensive_pre_reaction_ability_rate"] = (
        values["offensive_pre_reaction_ability_selected"] / pre_outcomes if pre_outcomes else 0.0
    )
    result["offensive_pre_reaction_pass_rate"] = (
        values["offensive_pre_reaction_passed"] / pre_outcomes if pre_outcomes else 0.0
    )
    result["offensive_post_reaction_ability_rate"] = (
        values["offensive_post_reaction_ability_selected"] / post_outcomes if post_outcomes else 0.0
    )
    result["offensive_post_reaction_pass_rate"] = (
        values["offensive_post_reaction_passed"] / post_outcomes if post_outcomes else 0.0
    )
    offensive_selections = values["offensive_pre_reaction_ability_selected"]
    for roll in range(1, 4):
        result[f"offensive_selection_roll_{roll}_rate"] = (
            values[f"offensive_pre_reaction_selected_roll_{roll}"] / offensive_selections
            if offensive_selections
            else 0.0
        )
    return result


def _summarize_damage(values: Counter[str]) -> dict[str, float | int]:
    result: dict[str, float | int] = dict(sorted(values.items()))
    rounds = values["rounds"]
    for direction in ("outgoing", "incoming"):
        for stage in ("raw", "resolved", "actual"):
            key = f"{stage}_{direction}_total"
            result[f"average_{key}_per_round"] = values[key] / rounds if rounds else 0.0
        for category in ("attack", "bleed", "poison"):
            key = f"raw_{direction}_{category}"
            result[f"average_{key}_per_round"] = values[key] / rounds if rounds else 0.0
    result["average_actual_damage_advantage_per_round"] = (
        (values["actual_outgoing_total"] - values["actual_incoming_total"]) / rounds
        if rounds
        else 0.0
    )
    return result


def wilson_interval(wins: int, games: int, z: float = 1.959963984540054) -> tuple[float, float]:
    if games == 0:
        return 0.0, 1.0
    proportion = wins / games
    denominator = 1.0 + z * z / games
    center = (proportion + z * z / (2.0 * games)) / denominator
    variance = proportion * (1.0 - proportion) / games + z * z / (4.0 * games * games)
    radius = z * math.sqrt(variance) / denominator
    return max(0.0, center - radius), min(1.0, center + radius)
