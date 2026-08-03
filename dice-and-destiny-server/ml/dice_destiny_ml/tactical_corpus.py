from __future__ import annotations

import hashlib
import json
from collections import Counter
from pathlib import Path
from typing import Any

from .bridge import AuthorityBridge
from .evaluation_statistics import paired_boolean_comparison
from .policies import MechanicsPolicyV2, ModelPolicy, select_with_policy
from .schema_v2 import OBSERVATION_SCHEMA_V2, SchemaEncoderV2


def build_tactical_corpus(
    *,
    binary: Path,
    server_root: Path,
    seed_start: int,
    cases: int,
    output_file: Path,
) -> dict[str, Any]:
    """Capture real-authority first-roll decisions and generic teacher labels."""
    encoder = SchemaEncoderV2()
    teachers = {"seat-a": MechanicsPolicyV2(), "seat-b": MechanicsPolicyV2()}
    records = []
    with AuthorityBridge(
        binary,
        server_root,
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
        session_id="phase2-tactical-corpus",
    ) as bridge:
        for offset in range(cases):
            seed = seed_start + offset
            transition = bridge.reset(seed, {"seat-a": "corpus", "seat-b": "corpus"})
            actor = transition["actor_id"]
            roll_indices = [
                index
                for index, action in enumerate(transition["result"]["legal_actions"])
                if action["type"] == "planning_roll"
            ]
            if len(roll_indices) != 1:
                raise RuntimeError(f"seed {seed} has {len(roll_indices)} initial roll candidates")
            transition = bridge.step(roll_indices[0])
            actor = transition["actor_id"]
            teacher = teachers[actor]
            teacher.reset(seed, actor)
            decision = encoder.encode(transition)
            selected = teacher.select(transition, decision)
            action = transition["result"]["legal_actions"][selected]
            snapshot = transition["result"]["snapshot"]
            own = snapshot["actors"][actor]
            qualified = own.get("qualified_abilities") or []
            category = (
                "qualified_reroll"
                if qualified and action["type"] == "planning_reroll"
                else "qualified_immediate"
                if qualified and action["type"] == "planning_select_ability"
                else "unqualified_reroll"
                if action["type"] == "planning_reroll"
                else "other"
            )
            records.append(
                {
                    "seed": seed,
                    "actor_id": actor,
                    "dice": own["dice"]["dice"],
                    "rolls_used": own["dice"]["rolls_used"],
                    "rolls_remaining": own["dice"]["rolls_remaining"],
                    "qualified_count": len(qualified),
                    "candidate_count": len(transition["result"]["legal_actions"]),
                    "teacher_index": selected,
                    "teacher_command": action,
                    "category": category,
                }
            )
    categories = Counter(record["category"] for record in records)
    result = {
        "schema": "dice-and-destiny-tactical-corpus-v2",
        "seed_start": seed_start,
        "cases": len(records),
        "categories": dict(sorted(categories.items())),
        "teacher": {
            "name": "mechanics-v2",
            "authored_id_special_cases": 0,
            "future_authority_rng_reads": 0,
            "counterfactual_source": "public authored die faces",
        },
        "records": records,
    }
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    result["sha256"] = hashlib.sha256(output_file.read_bytes()).hexdigest()
    return result


def evaluate_tactical_checkpoint(
    *,
    binary: Path,
    server_root: Path,
    corpus_file: Path,
    checkpoint: Path,
    output_file: Path,
) -> dict[str, Any]:
    corpus = json.loads(corpus_file.read_text())
    encoder = SchemaEncoderV2()
    policy = ModelPolicy(checkpoint, deterministic=True, device="cpu")
    records = []
    with AuthorityBridge(
        binary,
        server_root,
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
        session_id="phase2-tactical-evaluation",
    ) as bridge:
        for expected in corpus["records"]:
            seed = int(expected["seed"])
            transition = bridge.reset(seed, {"seat-a": policy.name, "seat-b": policy.name})
            roll = next(
                index
                for index, action in enumerate(transition["result"]["legal_actions"])
                if action["type"] == "planning_roll"
            )
            transition = bridge.step(roll)
            actor = transition["actor_id"]
            policy.reset(seed, actor)
            selected = select_with_policy(policy, transition, encoder)
            action = transition["result"]["legal_actions"][selected]
            teacher_action = expected["teacher_command"]
            agreement = _semantic_action(action) == _semantic_action(teacher_action)
            records.append(
                {
                    "seed": seed,
                    "teacher_category": expected["category"],
                    "teacher_command": teacher_action,
                    "model_index": selected,
                    "model_command": action,
                    "agreement": agreement,
                    "immediate_bias_error": (
                        expected["category"] == "qualified_reroll"
                        and action["type"] == "planning_select_ability"
                    ),
                }
            )
    reroll_cases = [record for record in records if record["teacher_category"] == "qualified_reroll"]
    immediate_cases = [
        record for record in records if record["teacher_category"] == "qualified_immediate"
    ]
    result = {
        "schema": "dice-and-destiny-tactical-evaluation-v2",
        "checkpoint": str(checkpoint.resolve()),
        "checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "model_family": policy.observation_schema,
        "corpus": str(corpus_file.resolve()),
        "corpus_sha256": hashlib.sha256(corpus_file.read_bytes()).hexdigest(),
        "cases": len(records),
        "teacher_agreement": sum(record["agreement"] for record in records) / len(records),
        "qualified_reroll_agreement": (
            sum(record["agreement"] for record in reroll_cases) / len(reroll_cases)
            if reroll_cases
            else 0.0
        ),
        "qualified_immediate_agreement": (
            sum(record["agreement"] for record in immediate_cases) / len(immediate_cases)
            if immediate_cases
            else 0.0
        ),
        "immediate_bias_errors": sum(record["immediate_bias_error"] for record in records),
        "records": records,
    }
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


def evaluate_tactical_matrix(
    *,
    binary: Path,
    server_root: Path,
    corpus_file: Path,
    matrix_root: Path,
) -> dict[str, Any]:
    """Score every valid final matrix checkpoint against the fixed tactical corpus."""
    checkpoints = sorted(matrix_root.glob("*/budget-*/seed-*/checkpoints/final.zip"))
    if not checkpoints:
        raise RuntimeError(f"no final checkpoints found beneath {matrix_root}")
    summaries = []
    for checkpoint in checkpoints:
        output_file = checkpoint.parents[1] / "tactical-summary.json"
        summaries.append(
            evaluate_tactical_checkpoint(
                binary=binary,
                server_root=server_root,
                corpus_file=corpus_file,
                checkpoint=checkpoint,
                output_file=output_file,
            )
        )
    result = {
        "schema": "dice-and-destiny-tactical-matrix-v2",
        "matrix_root": str(matrix_root.resolve()),
        "corpus": str(corpus_file.resolve()),
        "evaluated_checkpoints": len(summaries),
        "summaries": [
            {
                "checkpoint": summary["checkpoint"],
                "teacher_agreement": summary["teacher_agreement"],
                "qualified_reroll_agreement": summary["qualified_reroll_agreement"],
                "qualified_immediate_agreement": summary["qualified_immediate_agreement"],
                "immediate_bias_errors": summary["immediate_bias_errors"],
            }
            for summary in summaries
        ],
    }
    (matrix_root / "tactical-index.json").write_text(
        json.dumps(result, indent=2, sort_keys=True) + "\n"
    )
    return result


def build_decision_corpus(
    *,
    binary: Path,
    server_root: Path,
    seed_start: int,
    cases_per_category: int,
    max_episodes: int,
    output_file: Path,
) -> dict[str, Any]:
    """Capture disjoint real-authority decisions across broad action classes."""
    categories = (
        "qualified_reroll",
        "unqualified_reroll",
        "ability_commit",
        "planning_card",
        "reaction_card",
        "planning_pass",
        "reaction_pass",
        "effect_roll",
    )
    counts = Counter()
    records: list[dict[str, Any]] = []
    episodes: list[dict[str, Any]] = []
    encoder = SchemaEncoderV2()
    policies = {"seat-a": MechanicsPolicyV2(), "seat-b": MechanicsPolicyV2()}
    with AuthorityBridge(
        binary,
        server_root,
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
        session_id="phase2-broad-decision-corpus",
    ) as bridge:
        for offset in range(max_episodes):
            if all(counts[category] >= cases_per_category for category in categories):
                break
            seed = seed_start + offset
            for seat_id, policy in policies.items():
                policy.reset(seed, seat_id)
            transition = bridge.reset(seed, {seat: policy.name for seat, policy in policies.items()})
            action_indices: list[int] = []
            episode_records: list[dict[str, Any]] = []
            while not transition.get("terminal") and not transition.get("truncation_reason"):
                actor = transition["actor_id"]
                decision = encoder.encode(transition)
                selected = policies[actor].select(transition, decision)
                action = transition["result"]["legal_actions"][selected]
                own = transition["result"]["snapshot"]["actors"][actor]
                category = _decision_category(action, own)
                if category in categories and counts[category] < cases_per_category:
                    counts[category] += 1
                    episode_records.append(
                        {
                            "seed": seed,
                            "sequence": len(action_indices),
                            "actor_id": actor,
                            "category": category,
                            "candidate_count": len(transition["result"]["legal_actions"]),
                            "teacher_index": selected,
                            "teacher_command": action,
                        }
                    )
                action_indices.append(selected)
                transition = bridge.step(selected)
            if episode_records:
                episode_index = len(episodes)
                episodes.append({"seed": seed, "action_indices": action_indices})
                for record in episode_records:
                    record["episode_index"] = episode_index
                    records.append(record)
    shortfalls = {
        category: cases_per_category - counts[category]
        for category in categories
        if counts[category] < cases_per_category
    }
    result = {
        "schema": "dice-and-destiny-broad-decision-corpus-v2",
        "seed_start": seed_start,
        "cases_per_category": cases_per_category,
        "max_episodes": max_episodes,
        "categories": {category: counts[category] for category in categories},
        "shortfalls": shortfalls,
        "teacher": {
            "name": "mechanics-v2",
            "authored_id_special_cases": 0,
            "future_authority_rng_reads": 0,
        },
        "episodes": episodes,
        "records": records,
    }
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    result["sha256"] = hashlib.sha256(output_file.read_bytes()).hexdigest()
    return result


def evaluate_decision_corpus(
    *,
    binary: Path,
    server_root: Path,
    corpus_file: Path,
    checkpoint: Path,
    output_file: Path,
    baseline_summary: Path | None = None,
) -> dict[str, Any]:
    corpus = json.loads(corpus_file.read_text())
    records_by_episode: dict[int, dict[int, dict[str, Any]]] = {}
    for record in corpus["records"]:
        records_by_episode.setdefault(int(record["episode_index"]), {})[
            int(record["sequence"])
        ] = record
    policy = ModelPolicy(checkpoint, deterministic=True, device="cpu")
    encoder = SchemaEncoderV2()
    evaluated = []
    with AuthorityBridge(
        binary,
        server_root,
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
        session_id="phase2-broad-decision-evaluation",
    ) as bridge:
        for episode_index, episode in enumerate(corpus["episodes"]):
            seed = int(episode["seed"])
            policy.reset(seed, "seat-a")
            transition = bridge.reset(seed, {"seat-a": policy.name, "seat-b": policy.name})
            wanted = records_by_episode.get(episode_index, {})
            for sequence, teacher_index in enumerate(episode["action_indices"]):
                if sequence in wanted:
                    expected = wanted[sequence]
                    selected = select_with_policy(policy, transition, encoder)
                    action = transition["result"]["legal_actions"][selected]
                    teacher = expected["teacher_command"]
                    evaluated.append(
                        {
                            "seed": seed,
                            "sequence": sequence,
                            "category": expected["category"],
                            "teacher_command": teacher,
                            "model_command": action,
                            "semantic_agreement": _semantic_action(action) == _semantic_action(teacher),
                            "action_type_agreement": action.get("type") == teacher.get("type"),
                        }
                    )
                transition = bridge.step(int(teacher_index))
    by_category = {}
    for category in corpus["categories"]:
        rows = [row for row in evaluated if row["category"] == category]
        by_category[category] = {
            "cases": len(rows),
            "semantic_agreement": sum(row["semantic_agreement"] for row in rows) / len(rows)
            if rows
            else 0.0,
            "action_type_agreement": sum(row["action_type_agreement"] for row in rows) / len(rows)
            if rows
            else 0.0,
        }
    result = {
        "schema": "dice-and-destiny-broad-decision-evaluation-v2",
        "checkpoint": str(checkpoint.resolve()),
        "checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "corpus": str(corpus_file.resolve()),
        "corpus_sha256": hashlib.sha256(corpus_file.read_bytes()).hexdigest(),
        "cases": len(evaluated),
        "semantic_agreement": sum(row["semantic_agreement"] for row in evaluated) / len(evaluated),
        "action_type_agreement": sum(row["action_type_agreement"] for row in evaluated) / len(evaluated),
        "by_category": by_category,
        "records": evaluated,
    }
    if baseline_summary is not None:
        baseline = json.loads(baseline_summary.read_text())
        if baseline.get("corpus_sha256") != result["corpus_sha256"]:
            raise ValueError("paired baseline uses a different decision corpus")
        def key(row: dict[str, Any]) -> tuple[int, int, str]:
            return int(row["seed"]), int(row["sequence"]), str(row["category"])

        baseline_records = {key(row): row for row in baseline["records"]}
        candidate_records = {key(row): row for row in evaluated}
        if baseline_records.keys() != candidate_records.keys():
            raise ValueError("paired baseline and candidate decision keys differ")
        ordered = sorted(candidate_records)
        result["paired_vs_baseline"] = {
            "baseline": str(baseline_summary.resolve()),
            "baseline_checkpoint_sha256": baseline.get("checkpoint_sha256", ""),
            "semantic_agreement": paired_boolean_comparison(
                [bool(baseline_records[item]["semantic_agreement"]) for item in ordered],
                [bool(candidate_records[item]["semantic_agreement"]) for item in ordered],
            ),
            "action_type_agreement": paired_boolean_comparison(
                [bool(baseline_records[item]["action_type_agreement"]) for item in ordered],
                [bool(candidate_records[item]["action_type_agreement"]) for item in ordered],
            ),
        }
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


def _semantic_action(action: dict[str, Any]) -> tuple[Any, ...]:
    payload = action.get("payload") or {}
    return (
        action.get("type"),
        tuple(payload.get("reroll_indices") or []),
        tuple(payload.get("kept_indices") or []),
        payload.get("ability_id", ""),
        tuple(payload.get("target_ids") or []),
    )


def _decision_category(action: dict[str, Any], actor: dict[str, Any]) -> str:
    kind = action.get("type", "")
    if kind == "planning_reroll":
        return "qualified_reroll" if actor.get("qualified_abilities") else "unqualified_reroll"
    return {
        "planning_select_ability": "ability_commit",
        "planning_commit_cards": "planning_card",
        "commit_interaction": "reaction_card",
        "planning_pass": "planning_pass",
        "pass": "reaction_pass",
        "roll_dice": "effect_roll",
    }.get(kind, "other")
