from __future__ import annotations

# ruff: noqa: E501 - long lines below are literal self-contained HTML/CSS/JavaScript.
import hashlib
import html
import json
from collections import Counter
from pathlib import Path
from typing import Any

import numpy as np

from .bridge import AuthorityBridge
from .policies import ModelPolicy, build_policy
from .schema_v2 import (
    ACTION_FEATURES_V2,
    BASE_FEATURES_V2,
    COMMAND_TYPES,
    MAX_ABILITIES_V2,
    MAX_ACTIONS_V2,
    MAX_DICE_V2,
    MAX_REQUIREMENTS_V2,
    MAX_SYMBOLS_V2,
    MAX_TIERS_V2,
    OBSERVATION_SCHEMA_V2,
    OBSERVATION_SIZE_V2,
    SEGMENTS,
    SchemaEncoderV2,
    parse_payload_v2,
)

ACTOR_SCALARS = (
    "current health / maximum health",
    "maximum health / 30",
    "energy / maximum energy",
    "hand count / 20",
    "deck count / 30",
    "discard count / 30",
    "removed count / 30",
    "dice count / 10",
    "ability count / 12",
    "active status-instance count / 10",
    "token count / 10",
    "defeated flag",
)

EFFECT_DIMENSIONS = (
    "damage / 20",
    "prevention or scaling / 20",
    "status stacks / 10",
    "resource amount / 10",
    "rolled dice / 10",
    "modifier flag/count",
    "generic numeric magnitude / 20",
)


def generate_observation_walkthrough(
    *,
    binary: Path,
    server_root: Path,
    replay_file: Path,
    output_file: Path,
    device: str = "cpu",
) -> dict[str, Any]:
    """Reconstruct one recorded v2 match and explain every model decision."""

    replay_file = replay_file.resolve()
    replay = json.loads(replay_file.read_text())
    if replay.get("observation_schema") != OBSERVATION_SCHEMA_V2:
        raise RuntimeError("the observation walkthrough currently supports observation-v2 replays")
    actions = replay.get("actions") or []
    if not actions:
        raise RuntimeError("replay contains no actions")

    ml_root = server_root / "ml"
    original_specs = replay.get("seat_models") or {}
    if set(original_specs) != {"seat-a", "seat-b"}:
        raise RuntimeError("replay must identify models for seat-a and seat-b")
    resolved_specs = {
        seat: _resolve_model_spec(specification, ml_root) for seat, specification in original_specs.items()
    }
    policies = {
        seat: build_policy(specification, device=device, deterministic=True)
        for seat, specification in resolved_specs.items()
    }
    if not all(isinstance(policy, ModelPolicy) for policy in policies.values()):
        raise RuntimeError("walkthrough replay must contain two neural-network model policies")
    for seat, policy in policies.items():
        policy.reset(int(replay["seed"]), seat)

    encoder = SchemaEncoderV2()
    decisions: list[dict[str, Any]] = []
    first_catalog: dict[str, Any] = {}
    with AuthorityBridge(
        binary,
        server_root,
        session_id="observation-walkthrough",
        authority_mode="normal",
        telemetry_mode="full",
        transport_mode="full",
        observation_schema=OBSERVATION_SCHEMA_V2,
    ) as bridge:
        transition = bridge.reset(
            int(replay["seed"]),
            original_specs,
            battle_id=replay.get("battle_id", "observation-walkthrough"),
        )
        for sequence, recorded_action in enumerate(actions, start=1):
            if transition.get("terminal") or transition.get("truncation_reason"):
                raise RuntimeError(f"replay terminated before recorded action {sequence}")
            actor = transition.get("actor_id")
            if actor != recorded_action.get("actor_id"):
                raise RuntimeError(
                    f"actor mismatch at action {sequence}: {actor!r} != {recorded_action.get('actor_id')!r}"
                )
            result = transition.get("result") or {}
            snapshot = result.get("snapshot") or {}
            if not first_catalog:
                first_catalog = snapshot.get("content_catalog") or {}
            decision = encoder.encode(transition)
            selected = int(recorded_action["action_index"])
            legal_actions = result.get("legal_actions") or []
            if not 0 <= selected < len(legal_actions):
                raise RuntimeError(f"recorded action index {selected} is invalid at action {sequence}")
            if not decision.action_mask[selected]:
                raise RuntimeError(f"recorded action {sequence} selected a masked candidate")
            if _canonical(legal_actions[selected]) != _canonical(recorded_action.get("command") or {}):
                raise RuntimeError(f"authority command mismatch at action {sequence}")

            policy = policies[actor]
            assert isinstance(policy, ModelPolicy)
            observation_tensor, _ = policy.model.policy.obs_to_tensor(decision.observation)
            distribution = policy.model.policy.get_distribution(
                observation_tensor,
                action_masks=decision.action_mask,
            )
            probabilities = distribution.distribution.probs.detach().cpu().numpy()[0]
            deterministic_action = int(probabilities.argmax())
            if deterministic_action != selected:
                raise RuntimeError(
                    f"model argmax mismatch at action {sequence}: {deterministic_action} != {selected}"
                )
            critic_value = float(
                policy.model.policy.predict_values(observation_tensor).detach().cpu().numpy()[0, 0]
            )
            decisions.append(
                _decision_record(
                    sequence=sequence,
                    actor=actor,
                    policy_spec=original_specs[actor],
                    snapshot=snapshot,
                    legal_actions=legal_actions,
                    observation=decision.observation,
                    action_mask=decision.action_mask,
                    probabilities=probabilities,
                    critic_value=critic_value,
                    selected=selected,
                )
            )
            transition = bridge.step(selected)

        if not transition.get("terminal"):
            raise RuntimeError("replay actions did not reach a terminal battle state")
        regenerated_replay = transition.get("replay") or {}
        if _canonical(regenerated_replay.get("actions") or []) != _canonical(actions):
            raise RuntimeError("regenerated action trace differs from the preserved replay")
        metrics = transition.get("metrics") or {}
        winner = metrics.get("winner") or transition.get("winner") or ""
        status = metrics.get("status") or regenerated_replay.get("status") or ""
        if winner != replay.get("winner") or status != replay.get("status"):
            raise RuntimeError("regenerated terminal result differs from the preserved replay")

    model_metadata = {
        seat: _model_metadata(original_specs[seat], resolved_specs[seat], policies[seat])
        for seat in ("seat-a", "seat-b")
    }
    summary = {
        "battle_id": replay.get("battle_id", ""),
        "seed": int(replay["seed"]),
        "winner": replay.get("winner", ""),
        "status": replay.get("status", ""),
        "decisions": len(decisions),
        "decisions_with_active_status": sum(
            any(record["explicit_status_gap"].values()) for record in decisions
        ),
        "unencoded_segment_counts": dict(
            Counter(
                record["segment"]
                for record in decisions
                if record["segment"] not in SEGMENTS
            )
        ),
        "seat_models": model_metadata,
        "action_counts": dict(Counter(record["selected_type"] for record in decisions)),
        "observation_schema": OBSERVATION_SCHEMA_V2,
        "observation_size": OBSERVATION_SIZE_V2,
        "base_features": BASE_FEATURES_V2,
        "candidate_slots": MAX_ACTIONS_V2,
        "candidate_features": ACTION_FEATURES_V2,
        "replay_sha256": hashlib.sha256(replay_file.read_bytes()).hexdigest(),
    }
    document = _render_html(summary, decisions, first_catalog, replay_file)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(document)
    summary["output"] = str(output_file.resolve())
    summary["output_sha256"] = hashlib.sha256(output_file.read_bytes()).hexdigest()
    summary["output_bytes"] = output_file.stat().st_size
    return summary


def _resolve_model_spec(specification: str, ml_root: Path) -> str:
    if not specification.startswith("model:"):
        return specification
    checkpoint = Path(specification.removeprefix("model:"))
    if not checkpoint.is_absolute():
        checkpoint = ml_root / checkpoint
    if not checkpoint.is_file():
        raise RuntimeError(f"model checkpoint is missing: {checkpoint}")
    return f"model:{checkpoint.resolve()}"


def _model_metadata(
    original_spec: str,
    resolved_spec: str,
    policy: Any,
) -> dict[str, Any]:
    if not isinstance(policy, ModelPolicy):
        return {"specification": original_spec}
    checkpoint = Path(resolved_spec.removeprefix("model:"))
    return {
        "specification": original_spec,
        "checkpoint": str(checkpoint),
        "checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "parameter_count": int(sum(parameter.numel() for parameter in policy.model.policy.parameters())),
    }


def _decision_record(
    *,
    sequence: int,
    actor: str,
    policy_spec: str,
    snapshot: dict[str, Any],
    legal_actions: list[dict[str, Any]],
    observation: np.ndarray,
    action_mask: np.ndarray,
    probabilities: np.ndarray,
    critic_value: float,
    selected: int,
) -> dict[str, Any]:
    catalog = snapshot.get("content_catalog") or {}
    symbols = sorted((catalog.get("symbols") or {}).keys())
    actors = snapshot.get("actors") or {}
    own = actors.get(actor) or {}
    opponent = "seat-b" if actor == "seat-a" else "seat-a"
    board = list(own.get("offensive_abilities") or []) + list(own.get("defensive_abilities") or [])
    base = observation[:BASE_FEATURES_V2]
    candidates = observation[BASE_FEATURES_V2:].reshape(MAX_ACTIONS_V2, ACTION_FEATURES_V2)
    candidate_records = []
    for index, action in enumerate(legal_actions):
        vector = candidates[index]
        candidate_records.append(
            {
                "index": index,
                "type": action.get("type", ""),
                "payload": action.get("payload") or {},
                "allowed_by_model_mask": bool(action_mask[index]),
                "probability": float(probabilities[index]),
                "selected": index == selected,
                "authority_overlay": _authority_candidate_label(
                    action, own, catalog, actor, opponent
                ),
                "model_visible_summary": _model_candidate_summary(vector),
                "features": _nonzero_features(vector, _candidate_labels(board)),
                "raw_vector": [float(value) for value in vector],
            }
        )
    snapshot_without_catalog = dict(snapshot)
    snapshot_without_catalog.pop("content_catalog", None)
    return {
        "sequence": sequence,
        "actor": actor,
        "opponent": opponent,
        "policy": policy_spec,
        "round": int(snapshot.get("round", 0)),
        "completed_rounds": int(snapshot.get("completed_rounds", 0)),
        "segment": snapshot.get("segment", ""),
        "stage": snapshot.get("stage", ""),
        "priority": snapshot.get("priority_actor_id", ""),
        "critic_value": critic_value,
        "selected_index": selected,
        "selected_type": legal_actions[selected].get("type", ""),
        "selected_command": legal_actions[selected],
        "own_summary": _actor_summary(own),
        "opponent_summary": _actor_summary(actors.get(opponent) or {}),
        "explicit_status_gap": {
            actor: _status_summary(own, catalog),
            opponent: _status_summary(actors.get(opponent) or {}, catalog),
        },
        "own_hand": _hand_summary(own, catalog, legal_actions),
        "own_dice_review": _dice_review(own, catalog, symbols),
        "ability_board_review": _ability_board_review(own, catalog),
        "base_nonzero": _nonzero_features(base, _base_labels(symbols, board)),
        "base_vector": [float(value) for value in base],
        "observation_sha256": hashlib.sha256(observation.astype("<f4").tobytes()).hexdigest(),
        "observation_nonzero": int(np.count_nonzero(observation)),
        "action_mask": [bool(value) for value in action_mask],
        "candidates": candidate_records,
        "viewer_safe_snapshot": snapshot_without_catalog,
    }


def _actor_summary(actor: dict[str, Any]) -> dict[str, Any]:
    dice_state = actor.get("dice") or {}
    return {
        "definition_id": actor.get("definition_id", ""),
        "current_health": actor.get("current_health", 0),
        "max_health": actor.get("max_health", 0),
        "energy_points": actor.get("energy_points", 0),
        "max_energy_points": actor.get("max_energy_points", 0),
        "hand_count": actor.get("hand_count", 0),
        "deck_count": actor.get("deck_count", 0),
        "discard_count": actor.get("discard_count", 0),
        "removed_count": actor.get("removed_count", 0),
        "status_instance_count": len(actor.get("statuses") or []),
        "token_count": len(actor.get("tokens") or []),
        "rolls_used": dice_state.get("rolls_used", 0),
        "rolls_remaining": dice_state.get("rolls_remaining", 0),
        "max_rolls": dice_state.get("max_rolls", 0),
        "dice": dice_state.get("dice") or [],
        "kept_indices": dice_state.get("kept_indices") or [],
        "qualified_abilities": actor.get("qualified_abilities") or [],
        "selected_ability": actor.get("selected_ability", ""),
        "selected_targets": actor.get("selected_targets") or [],
        "defeat_state": actor.get("defeat_state", ""),
    }


def _status_summary(actor: dict[str, Any], catalog: dict[str, Any]) -> list[dict[str, Any]]:
    definitions = catalog.get("statuses") or {}
    result = []
    for status in actor.get("statuses") or []:
        identifier = status.get("definition_id") or status.get("id") or ""
        definition = definitions.get(identifier) or {}
        result.append(
            {
                "definition_id": identifier,
                "name": definition.get("name", identifier),
                "instance_id": status.get("instance_id", ""),
                "stacks": status.get("stacks", status.get("stack_count", 0)),
                "polarity": definition.get("polarity", ""),
                "note": "identity and stacks are not explicitly encoded; only status-instance count is",
            }
        )
    return result


def _hand_summary(
    actor: dict[str, Any],
    catalog: dict[str, Any],
    legal_actions: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    instances = actor.get("card_instances") or {}
    definitions = catalog.get("cards") or {}
    legal_indices: dict[str, list[int]] = {}
    for candidate_index, action in enumerate(legal_actions):
        payload = parse_payload_v2(action.get("payload"))
        commitment = payload.get("commitment") or {}
        card_ids = payload.get("card_ids") or commitment.get("card_ids") or []
        for instance_id in card_ids:
            legal_indices.setdefault(str(instance_id), []).append(candidate_index)
    result = []
    for instance_id in actor.get("hand") or []:
        definition_id = (instances.get(instance_id) or {}).get("definition_id", "")
        definition = definitions.get(definition_id) or {}
        indices = legal_indices.get(instance_id, [])
        result.append(
            {
                "instance_id": instance_id,
                "definition_id": definition_id,
                "name": definition.get("name", definition_id),
                "energy_cost": (definition.get("cost") or {}).get("energy", 0),
                "rules_text": (definition.get("presentation") or {}).get("rules_text", ""),
                "playable_during": (definition.get("play") or {}).get("playable_during") or [],
                "legal_candidate_indices": indices,
                "model_visibility": (
                    "Identity is absent from the base vector; compressed mechanics appear in "
                    f"candidate rows {indices}."
                    if indices
                    else "Only the total hand count is encoded; this card has no candidate row now."
                ),
            }
        )
    return result


def _dice_review(
    actor: dict[str, Any],
    catalog: dict[str, Any],
    symbols: list[str],
) -> list[dict[str, Any]]:
    dice = ((actor.get("dice") or {}).get("dice") or [])
    kept = {int(index) for index in (actor.get("dice") or {}).get("kept_indices") or []}
    definitions = catalog.get("dice") or {}
    result = []
    for slot, die in enumerate(dice):
        die_index = int(die.get("index", slot))
        definition_id = die.get("die_id", "")
        definition = definitions.get(definition_id) or {}
        result.append(
            {
                "slot": slot,
                "definition_id": definition_id,
                "name": definition.get("name", definition_id),
                "face": die.get("face", 0),
                "value": die.get("value", 0),
                "symbols": die.get("symbols") or [],
                "kept": die_index in kept,
                "model_values": {
                    "occupied": 1,
                    "die_index_div_10": die_index / MAX_DICE_V2,
                    "face_div_20": float(die.get("face", 0)) / 20.0,
                    "value_div_20": float(die.get("value", 0)) / 20.0,
                    "kept": die_index in kept,
                    "not_kept": die_index not in kept,
                    "symbol_bits": {
                        symbol: int(symbol in (die.get("symbols") or [])) for symbol in symbols
                    },
                },
                "not_encoded": "die definition and its face distribution",
            }
        )
    return result


def _ability_board_review(
    actor: dict[str, Any],
    catalog: dict[str, Any],
) -> list[dict[str, Any]]:
    board = list(actor.get("offensive_abilities") or []) + list(
        actor.get("defensive_abilities") or []
    )
    qualified = set(actor.get("qualified_abilities") or [])
    result = []
    for slot, ability_id in enumerate(board):
        ability = SchemaEncoderV2.effective_ability(ability_id, actor, catalog)
        qualification = ability.get("qualification") or {}
        tiers = list(qualification.get("activation_tiers") or []) + list(
            qualification.get("conditional_bonuses") or []
        )
        result.append(
            {
                "slot": slot,
                "base_feature_range": [256 + slot * 184, 256 + (slot + 1) * 184 - 1],
                "definition_id": ability_id,
                "name": ability.get("name", ability_id),
                "type": ability.get("type", ""),
                "energy_cost": (ability.get("cost") or {}).get("energy", 0),
                "qualified": ability_id in qualified,
                "selected": ability_id == actor.get("selected_ability"),
                "rules_text": (ability.get("presentation") or {}).get("rules_text", ""),
                "targeting": ability.get("targeting") or {},
                "tier_count": len(tiers),
                "model_visibility": (
                    "Dedicated board slot with type, qualification, selection, cost, targeting, "
                    "tier requirements/progress, and seven-number tier effect summaries. The "
                    "ability name and rules text are not model input."
                ),
            }
        )
    return result


def _authority_candidate_label(
    action: dict[str, Any],
    actor: dict[str, Any],
    catalog: dict[str, Any],
    viewer: str,
    opponent: str,
) -> str:
    kind = action.get("type", "")
    payload = parse_payload_v2(action.get("payload"))
    commitment = payload.get("commitment") or {}
    instances = actor.get("card_instances") or {}
    card_definitions = catalog.get("cards") or {}
    ability_definitions = catalog.get("abilities") or {}
    card_ids = payload.get("card_ids") or commitment.get("card_ids") or []
    card_names = [
        (card_definitions.get((instances.get(instance_id) or {}).get("definition_id", "")) or {}).get(
            "name", str(instance_id)
        )
        for instance_id in card_ids
    ]
    ability_id = payload.get("ability_id") or commitment.get("choice_id") or ""
    ability_name = (ability_definitions.get(ability_id) or {}).get("name", ability_id)
    targets = payload.get("target_ids") or commitment.get("target_ids") or []
    target_names = [
        "self" if target == viewer else "opponent" if target == opponent else str(target)
        for target in targets
    ]
    if card_names:
        target = ability_name or ", ".join(target_names)
        return f"Play {', '.join(card_names)}" + (f" → {target}" if target else "")
    if kind == "planning_roll":
        return "Begin planning roll"
    if kind == "planning_reroll":
        return f"Reroll dice {payload.get('reroll_indices') or []}"
    if kind == "planning_keep":
        return f"Keep dice {payload.get('kept_indices') or []}"
    if kind == "planning_select_ability":
        return f"Select {ability_name}" + (f" → {', '.join(target_names)}" if target_names else "")
    if kind == "planning_select_targets":
        return f"Select targets {', '.join(target_names)}"
    if kind == "roll_dice":
        indices = payload.get("reroll_indices")
        return "Roll dice" + (f" {indices}" if indices is not None else "")
    if kind == "planning_pass":
        return "Pass planning"
    if kind == "pass":
        return "Pass"
    return kind


def _model_candidate_summary(vector: np.ndarray) -> str:
    command = next(
        (COMMAND_TYPES[index] for index in range(len(COMMAND_TYPES)) if vector[index]),
        "unencoded command",
    )
    parts = [f"type={command}"]
    if vector[17]:
        parts.append(f"cards={vector[17] * 10:g}")
        parts.append(f"total_cost={vector[20] * 10:g}")
    if vector[13]:
        parts.append("targets_viewer=1")
    if vector[14]:
        parts.append("targets_opponent=1")
    if vector[15]:
        parts.append(f"target_count={vector[15] * 5:g}")
    ability_slots = [index for index in range(MAX_ABILITIES_V2) if vector[32 + index]]
    if ability_slots:
        parts.append(f"ability_slot={ability_slots[0]}")
    die_indices = [index for index in range(MAX_DICE_V2) if vector[22 + index]]
    if die_indices:
        parts.append(f"die_indices={die_indices}")
    effects = vector[52:59]
    effect_names = ("damage", "prevention", "status", "resource", "roll", "modifier", "magnitude")
    populated = [f"{name}={value:g}" for name, value in zip(effect_names, effects, strict=True) if value]
    if populated:
        parts.append("effects[" + ", ".join(populated) + "]")
    return "; ".join(parts)


def _base_labels(symbols: list[str], board: list[str]) -> dict[int, str]:
    labels: dict[int, str] = {
        0: "viewer is seat-a",
        1: "viewer is seat-b",
        2: "round / 50 (clamped to 1)",
        3: "completed rounds / 50 (clamped to 1)",
        8: "priority belongs to viewer",
        9: "priority belongs to opponent",
        10: "no priority actor",
        48: "rolls used / 3",
        49: "maximum rolls / 3",
        50: "rolls remaining / 3",
        51: "dice phase complete",
        52: "current dice count / 10",
        53: "kept dice count / 10",
        54: "qualified ability count / 12",
        55: "ability board count / 12",
        56: "authority legal candidate count / 128",
    }
    for offset, segment in enumerate(SEGMENTS, start=4):
        labels[offset] = f"segment is {segment}"
    for start, role in ((16, "viewer"), (32, "opponent")):
        for index, label in enumerate(ACTOR_SCALARS):
            labels[start + index] = f"{role}: {label}"
    for index in range(MAX_SYMBOLS_V2):
        symbol = symbols[index] if index < len(symbols) else f"unused symbol slot {index}"
        labels[64 + index] = f"rolled symbol count for {symbol} / 10"
    for slot in range(MAX_DICE_V2):
        start = 128 + slot * 14
        dice_labels = (
            "slot occupied",
            "die index / 10",
            "face / 20",
            "value / 20",
            "kept",
            "not kept",
        )
        for index, label in enumerate(dice_labels):
            labels[start + index] = f"die slot {slot}: {label}"
        for symbol_index in range(MAX_SYMBOLS_V2):
            symbol = symbols[symbol_index] if symbol_index < len(symbols) else f"unused-{symbol_index}"
            labels[start + 6 + symbol_index] = f"die slot {slot}: symbol {symbol}"
    for slot in range(MAX_ABILITIES_V2):
        start = 256 + slot * 184
        ability = board[slot] if slot < len(board) else f"unused ability slot {slot}"
        ability_labels = (
            "slot occupied",
            "offensive",
            "defensive",
            "currently qualified",
            "currently selected",
            "energy cost / 10",
            "maximum uses per segment / 10",
            "minimum targets / 5",
            "maximum targets / 5",
            "targets self",
            "targets enemy",
            "other target selector",
            "activation-tier count / 6",
            "conditional-tier count / 6",
            "requires incoming proposal",
            "target count / 5",
        )
        for index, label in enumerate(ability_labels):
            labels[start + index] = f"ability {ability}: {label}"
        for tier in range(MAX_TIERS_V2):
            tier_start = start + 16 + tier * 28
            tier_labels = (
                "slot occupied",
                "conditional bonus",
                "requirements met",
                "minimum requirement progress",
                "requirement count / 2",
            )
            for index, label in enumerate(tier_labels):
                labels[tier_start + index] = f"ability {ability}, tier {tier}: {label}"
            for requirement in range(MAX_REQUIREMENTS_V2):
                req_start = tier_start + 5 + requirement * 8
                req_labels = (
                    "slot occupied",
                    "symbol-count type",
                    "exact-faces type",
                    "number-pattern type",
                    "symbol index / 8",
                    "current / 10",
                    "target / 10",
                    "progress",
                )
                for index, label in enumerate(req_labels):
                    labels[req_start + index] = (
                        f"ability {ability}, tier {tier}, requirement {requirement}: {label}"
                    )
            for index, label in enumerate(EFFECT_DIMENSIONS):
                labels[tier_start + 21 + index] = f"ability {ability}, tier {tier}, effect: {label}"
    return labels


def _candidate_labels(board: list[str]) -> dict[int, str]:
    labels: dict[int, str] = {}
    for index, command in enumerate(COMMAND_TYPES):
        labels[index] = f"command type is {command}"
    labels.update(
        {
            10: "pass-family action",
            11: "roll-family action",
            12: "keep/reroll-family action",
            13: "targets viewer",
            14: "targets opponent",
            15: "target count / 5",
            16: "selected dice count / 10",
            17: "selected card count / 10",
            18: "references an ability",
            19: "contains cards",
            20: "total card energy cost / 10",
            21: "rolls remaining / 3",
            44: "linked ability exists",
            45: "linked ability is qualified",
            46: "linked ability is selected",
            47: "linked ability energy cost / 10",
            48: "linked ability is offensive",
            49: "linked ability is defensive",
            50: "linked ability tier count / 6",
            51: "maximum linked tier progress",
            59: "linked ability minimum targets / 5",
            60: "linked ability maximum targets / 5",
            61: "candidate contains card mechanics",
            62: "candidate references status mechanics",
        }
    )
    for index in range(MAX_DICE_V2):
        labels[22 + index] = f"includes die index {index}"
    for index in range(MAX_ABILITIES_V2):
        ability = board[index] if index < len(board) else f"unused ability slot {index}"
        labels[32 + index] = f"links board ability {ability}"
    for index, label in enumerate(EFFECT_DIMENSIONS):
        labels[52 + index] = f"combined candidate effect: {label}"
    for tier in range(4):
        start = 64 + tier * 16
        compact = (
            "slot occupied",
            "conditional bonus",
            "progress",
            "requirements met",
            "requirement count / 2",
        )
        for index, label in enumerate(compact):
            labels[start + index] = f"linked tier {tier}: {label}"
        for index, label in enumerate(EFFECT_DIMENSIONS):
            labels[start + 5 + index] = f"linked tier {tier} effect: {label}"
    return labels


def _nonzero_features(vector: np.ndarray, labels: dict[int, str]) -> list[dict[str, Any]]:
    return [
        {
            "index": int(index),
            "value": float(vector[index]),
            "meaning": labels.get(int(index), "unassigned/reserved feature"),
        }
        for index in np.flatnonzero(vector)
    ]


def _canonical(value: Any) -> Any:
    return json.loads(json.dumps(value, sort_keys=True))


def _json(value: Any, *, compact: bool = False) -> str:
    rendered = json.dumps(
        value,
        separators=(",", ":") if compact else None,
        indent=None if compact else 2,
        sort_keys=True,
    )
    return html.escape(rendered)


def _render_html(
    summary: dict[str, Any],
    decisions: list[dict[str, Any]],
    catalog: dict[str, Any],
    replay_file: Path,
) -> str:
    rows = "\n".join(_render_decision(record, index == 0) for index, record in enumerate(decisions))
    unencoded_segment_count = sum(summary["unencoded_segment_counts"].values())
    unencoded_segments = ", ".join(
        f"{name}: {count}" for name, count in sorted(summary["unencoded_segment_counts"].items())
    ) or "none"
    model_cards = "\n".join(
        f"""
        <article class="model-card">
          <div class="eyebrow">{html.escape(seat)}</div>
          <strong>{html.escape(metadata["specification"])}</strong>
          <dl>
            <dt>Checkpoint SHA-256</dt><dd><code>{metadata["checkpoint_sha256"]}</code></dd>
            <dt>Parameters</dt><dd>{metadata["parameter_count"]:,}</dd>
          </dl>
        </article>
        """.strip()
        for seat, metadata in summary["seat_models"].items()
    )
    catalog_summary = {
        "symbols": sorted((catalog.get("symbols") or {}).keys()),
        "dice": sorted((catalog.get("dice") or {}).keys()),
        "cards": sorted((catalog.get("cards") or {}).keys()),
        "abilities": sorted((catalog.get("abilities") or {}).keys()),
        "statuses": sorted((catalog.get("statuses") or {}).keys()),
    }
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Dice &amp; Destiny · One-game model observation walkthrough</title>
  <style>
    :root {{ color-scheme: dark; --bg:#08111d; --panel:#111d2b; --panel2:#172638;
      --line:#2b4055; --text:#e8eef5; --muted:#9fb1c3; --gold:#f2c35d;
      --cyan:#67d7e8; --green:#69d69b; --red:#ff8a89; --purple:#b9a1ff; }}
    * {{ box-sizing:border-box; }}
    body {{ margin:0; background:radial-gradient(circle at 10% 0,#152945 0,#08111d 32rem);
      color:var(--text); font:15px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,sans-serif; }}
    main {{ width:min(1500px,calc(100% - 32px)); margin:0 auto; padding:48px 0 90px; }}
    h1 {{ max-width:1000px; font-size:clamp(2.2rem,5vw,4.8rem); line-height:.98; margin:.3rem 0 1.2rem; }}
    h2 {{ margin:0 0 1rem; font-size:1.45rem; }} h3 {{ margin:.2rem 0 .8rem; }}
    p {{ color:var(--muted); }} a {{ color:var(--cyan); }} code {{ overflow-wrap:anywhere; }}
    .eyebrow {{ color:var(--gold); font-size:.76rem; font-weight:800; letter-spacing:.15em; text-transform:uppercase; }}
    .hero {{ border-bottom:1px solid var(--line); padding-bottom:32px; }}
    .lede {{ max-width:900px; font-size:1.1rem; }}
    .cards {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(210px,1fr)); gap:12px; margin:22px 0; }}
    .card,.model-card,.callout {{ background:linear-gradient(145deg,var(--panel2),var(--panel)); border:1px solid var(--line); border-radius:16px; padding:18px; }}
    .metric {{ font-size:1.9rem; font-weight:800; margin-top:4px; }}
    .model-card strong {{ display:block; margin:.35rem 0 1rem; overflow-wrap:anywhere; }}
    dl {{ display:grid; grid-template-columns:max-content 1fr; gap:5px 12px; margin:0; }} dt {{ color:var(--muted); }} dd {{ margin:0; min-width:0; }}
    .callout {{ margin:18px 0; }} .warning {{ border-color:#775e32; background:#241d14; }}
    .danger {{ border-color:#713b46; background:#25171d; }}
    .toolbar {{ position:sticky; top:0; z-index:5; display:flex; gap:10px; align-items:center;
      margin:30px 0 14px; padding:12px; background:rgba(8,17,29,.93); backdrop-filter:blur(12px); border:1px solid var(--line); border-radius:14px; }}
    input,select {{ background:#0c1724; color:var(--text); border:1px solid var(--line); border-radius:9px; padding:9px 11px; }}
    input {{ flex:1; min-width:140px; }}
    details.decision {{ border:1px solid var(--line); border-radius:14px; background:var(--panel); margin:10px 0; overflow:hidden; }}
    details.decision > summary {{ cursor:pointer; list-style:none; display:grid; grid-template-columns:80px 90px 110px 1fr 110px; gap:12px; align-items:center; padding:15px 18px; }}
    details.decision > summary::-webkit-details-marker {{ display:none; }}
    details.decision[open] > summary {{ border-bottom:1px solid var(--line); background:var(--panel2); }}
    .seat-a {{ color:var(--cyan); }} .seat-b {{ color:var(--purple); }} .selected {{ color:var(--green); font-weight:750; }}
    .decision-body {{ padding:18px; }}
    .grid2 {{ display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:14px; }}
    .mini {{ border:1px solid var(--line); border-radius:12px; padding:14px; background:#0c1724; }}
    table {{ width:100%; border-collapse:collapse; font-size:.88rem; }} th,td {{ border-bottom:1px solid var(--line); padding:7px 8px; text-align:left; vertical-align:top; }} th {{ color:var(--muted); position:sticky; top:0; background:#0c1724; }}
    .table-wrap {{ max-height:440px; overflow:auto; border:1px solid var(--line); border-radius:10px; }}
    pre {{ margin:0; padding:14px; background:#07101a; border:1px solid var(--line); border-radius:10px; overflow:auto; max-height:480px; font:12px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace; }}
    details.raw {{ margin:10px 0; }} details.raw > summary {{ color:var(--cyan); cursor:pointer; }}
    .pill {{ display:inline-block; padding:2px 8px; border:1px solid var(--line); border-radius:99px; color:var(--muted); font-size:.78rem; }}
    .prob {{ font-variant-numeric:tabular-nums; }}
    .omitted {{ color:var(--red); }}
    .todo-list {{ display:grid; gap:10px; padding:0; list-style:none; }}
    .todo-list li {{ padding:12px 14px; background:#0c1724; border:1px solid var(--line); border-radius:10px; }}
    .todo-list strong {{ color:var(--text); }}
    .todo-status {{ display:inline-block; min-width:7.5rem; margin-right:8px; color:var(--gold); font-size:.76rem; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }}
    footer {{ margin-top:36px; color:var(--muted); border-top:1px solid var(--line); padding-top:20px; }}
    @media (max-width:850px) {{ .grid2 {{ grid-template-columns:1fr; }} details.decision > summary {{ grid-template-columns:60px 70px 1fr; }} .wide {{ display:none; }} }}
  </style>
</head>
<body><main>
  <header class="hero">
    <div class="eyebrow">Deterministic authority reconstruction · Observation V2</div>
    <h1>What the models saw during one complete game</h1>
    <p class="lede">This document replays one preserved Observation V2 model-versus-model battle command-for-command. It separates the viewer-safe authority snapshot, the mechanics encoder, and the final 18,944-number neural-network input. Every selected action was recomputed from the original checkpoint and matched the preserved replay.</p>
    <div class="cards">
      <article class="card"><div class="eyebrow">Seed</div><div class="metric">{summary["seed"]}</div></article>
      <article class="card"><div class="eyebrow">Decisions</div><div class="metric">{summary["decisions"]}</div></article>
      <article class="card"><div class="eyebrow">Winner</div><div class="metric">{html.escape(summary["winner"])}</div></article>
      <article class="card"><div class="eyebrow">Result</div><div class="metric">{html.escape(summary["status"])}</div></article>
      <article class="card"><div class="eyebrow">Input</div><div class="metric">18,944</div><div>float32 values per decision</div></article>
      <article class="card"><div class="eyebrow">Status information gap</div><div class="metric">{summary["decisions_with_active_status"]}</div><div>decisions had active status identity/stacks omitted</div></article>
      <article class="card"><div class="eyebrow">Unencoded segment</div><div class="metric">{unencoded_segment_count}</div><div>decisions had no matching segment bit</div></article>
    </div>
    <div class="cards">{model_cards}</div>
    <div class="callout warning"><strong>Three different information layers:</strong> the authority owns the real state; each actor receives a viewer-safe snapshot; the encoder converts that snapshot and legal commands into 2,560 base values plus 128 candidate slots × 128 values. The neural network receives only those numbers and the action mask—not the names or JSON shown for human readability.</div>
    <div class="callout danger"><strong>Confirmed status gap:</strong> the authority snapshot contains active status identity and stacks. Observation V2 gives the network only each actor’s number of status instances. This walkthrough marks every status state where that information was discarded.</div>
    <div class="callout danger"><strong>Confirmed segment/stage gap:</strong> the base vector has segment bits only for <code>income</code>, <code>offensive</code>, <code>defensive</code>, and <code>status</code>. This replay has {unencoded_segment_count} decisions whose actual segment has no matching bit ({html.escape(unencoded_segments)}). The exact stage, flow iteration, planning cycle, reaction round, window, and pending-input context shown below are also absent from the neural-network input.</div>
    <div class="callout danger"><strong>Confirmed response-context gap:</strong> the viewer-safe snapshot contains public offensive and defensive proposals, resolution/window state, settled attack sources, defense selections, pending damage, and revealed status-effect rolls. Observation V2 encodes none of those structures. A defense or reaction candidate describes the learner’s possible response, but proposal/source IDs and the incoming attack, damage amount, or revealed effect-roll result are not represented directly.</div>
    <div class="callout warning"><strong>Confirmed dice/card compression:</strong> current dice faces, values, symbols, and selected reroll indices are encoded, but die definitions and their face distributions are not, so reroll odds are absent. The viewer’s hand identities are absent from the base vector; currently legal card candidates carry only card count, total cost, and a seven-number aggregate operation summary. Different cards or combinations can therefore become indistinguishable when those aggregates match.</div>
  </header>

  <section class="cards">
    <article class="card"><div class="eyebrow">Replay SHA-256</div><code>{summary["replay_sha256"]}</code></article>
    <article class="card"><div class="eyebrow">Battle</div><code>{html.escape(summary["battle_id"])}</code></article>
    <article class="card"><div class="eyebrow">Replay source</div><code>{html.escape(str(replay_file))}</code></article>
  </section>

  <section class="callout">
    <div class="eyebrow">Living implementation backlog</div>
    <h2>What the model is given · improvements to make after the audit</h2>
    <p>This checklist is intentionally stored in the generated walkthrough so discoveries are not lost while the game is reviewed. “Confirmed” means the current encoder was inspected and the information is absent or collapses distinct authority choices. “Investigate” is a design question that still needs evidence before implementation.</p>
    <h3>Confirmed model-input defects and omissions</h3>
    <ul class="todo-list">
      <li><span class="todo-status">Confirmed · P0</span><strong>Preserve exact action targets and parameters.</strong> Observation V2 drops scalar fields such as Loaded Die’s <code>die_index</code>. Distinct legal commands can therefore produce identical candidate vectors. Encode selected dice, actors, abilities, statuses, faces, modification types, and other command parameters generically.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Represent the actual result of a card or modifier choice.</strong> Loaded Die candidates do not say that the selected die becomes face 6, do not relate that result to its current face, and cannot expose that changing an existing face 6 to face 6 has no die-state benefit.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Preserve exact card mechanics instead of only seven aggregate totals.</strong> Candidate rows currently reduce cards to card count, total energy cost, and aggregated damage/prevention/status/resource/dice/modifier/magnitude numbers. Preserve individual operations, operation order, conditional requirements, status identity, timing, target selector, and multi-card composition.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Keep operation types and direction distinct.</strong> Observation V2 merges <code>prevent_damage</code> with <code>scale_damage</code>, <code>apply_status</code> with <code>remove_status</code>, <code>gain_resource</code> with <code>spend_resource</code>, and <code>apply_ability_modifier</code> with <code>modify_die</code>. Different mechanics can therefore produce the same seven-number summary. Encode the operation type, direction, operands, target, and nested outcomes without these semantic collisions.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Encode complete ability resolution mechanics.</strong> Observation V2 summarizes operations attached to qualification tiers but ignores top-level <code>resolution.operations</code>. In Round 1, Basic Defense’s “roll 1D6 and prevent the result” and Protect’s “halve incoming damage” mechanics are absent from both their board rows and candidate effect summaries. Encode resolution operations, required rolls, outcome tables, reaction-window behavior, allowed proposal types, and rounding/scaling rules generically.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Represent private hand contents.</strong> The base state contains only hand count. The learner cannot distinguish the identities, costs, timing windows, or future strategic uses of cards that are in hand but not currently playable.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Represent known card zones, deck composition, and hand constraints.</strong> The learner knows only hand/deck/discard/removed counts. It does not receive its authored decklist, identities of known discarded or removed cards, or maximum hand size, so it cannot reason accurately about future draws, recycling, depletion, or a draw that may hit the hand limit. Preserve viewer-known zone contents and constraints without exposing hidden opponent cards or deck order.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Represent response and resolution context.</strong> Encode the incoming attack/source, proposals, defense selections, pending damage, revealed effect rolls, and the exact event/window to which a reaction candidate responds.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Represent the learner’s complete active commitment and outcome.</strong> Observation V2 marks the selected ability but omits selected target identities, selected tier, and the evaluated offensive outcome. During Round 1 reactions and defense selection, the snapshot already contains these values, but the learner cannot see the exact damage, resource gains, or status applications it and its opponent committed.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Represent visible opponent mechanics, not only scalar counts.</strong> Observation V2 gives the learner only twelve opponent summary values: health, maximum health, energy, hand/deck/discard/removed counts, dice/ability/status/token counts, and defeat state. It omits visible opponent dice and roll state, ability board, requirements, effects, qualification and selection, ability modifiers, and detailed statuses. Add public opponent entities and mechanics while continuing to exclude private hand identities and other hidden information.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Encode status identity and state.</strong> The current actor vector contains only a count of status instances. Preserve status definition, stacks, duration, source/owner, polarity, and mechanically relevant parameters for every visible status.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Encode complete phase and decision context.</strong> Add every segment plus stage, flow iteration, planning cycle, reaction round, window purpose, and other pending-input context needed to distinguish otherwise similar decisions.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Represent die definitions and face distributions.</strong> Current faces, values, and recognized symbols are present, but the die definition and its possible faces are absent. The learner therefore cannot infer exact reroll odds from its input when different dice exist.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Eliminate the latent dice/ability layout overlap.</strong> Observation V2 reserves ten 14-value die rows starting at base index 128, which requires indices 128–267, but the first ability row starts at index 256. A tenth die would overwrite or be overwritten by the first twelve ability values. The current five-die battle does not trigger it, but expanded dice counts can. Use a manifest-validated, non-overlapping entity layout and test every boundary.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Represent combatant identity, current form, and authored loadouts.</strong> The viewer-safe state identifies the combatant and its ability, dice, and card loadouts, but Observation V2 does not encode the combatant definition or form directly. This is harmless in the current one-combatant mirror match but becomes ambiguous when characters can share some mechanics while differing in passives, forms, or loadouts.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Make content representation data-driven and extensible.</strong> New combatants, forms, cards, dice, symbols, abilities, statuses, resources, tokens, targets, and operation types must receive usable representations without content-specific hardcoding or silent collisions. Capacity and unknown-content behavior must be explicit.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Audit action-candidate completeness and quality.</strong> Verify that every strategically meaningful legal choice—such as each Sharpen Blade ability attachment—is generated exactly once, with its exact parameters, and identify redundant or mechanically ineffective candidates.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Replace combinatorial dice-subset candidate expansion.</strong> Round 1 produces as many as 72 authority candidates with only five dice, including 31 provisional keep subsets that Observation V2 masks out. Keep/reroll subset enumeration grows exponentially; six dice plus cards and abilities can exceed the fixed 128-candidate capacity. Use a scalable structured or factorized dice-selection action while preserving every legal reroll choice and deterministic masking.</li>
      <li><span class="todo-status">Confirmed · P0</span><strong>Add representation-collision tests.</strong> Automatically fail when two materially different authority commands encode to the same candidate row unless the equivalence is explicitly approved. Include cards, targets, dice, statuses, face choices, and combined effects.</li>
      <li><span class="todo-status">Confirmed · P1</span><strong>Add content and capacity coverage tests.</strong> Exercise every content definition and legal command shape, prove that each field reaches the intended entity/action representation, and fail loudly on truncation, vocabulary overflow, unsupported symbols, or excessive candidates.</li>
    </ul>
    <h3>Investigation queue · not yet established as defects</h3>
    <ul class="todo-list">
      <li><span class="todo-status">Deferred · later</span><strong>Derived roll-probability features.</strong> Keep this idea recorded, but do not include it in the initial observation redesign. Exact reroll-to-ability calculations become substantially more complex as dice counts, die types, symbols, modifiers, and ability requirements expand. Revisit only after the model has complete, data-driven die definitions and ability requirements.</li>
      <li><span class="todo-status">Investigate</span><strong>Projected after-state features.</strong> Test whether candidate-specific before/after deltas improve learning beyond a complete symbolic description of the command and current state.</li>
      <li><span class="todo-status">Investigate</span><strong>Decision history and memory.</strong> Determine which strategically relevant past events cannot be reconstructed from current authority state before adding recurrence or a bounded event history.</li>
      <li><span class="todo-status">Investigate</span><strong>Seat symmetry and simultaneous-play context.</strong> Verify that equivalent states and choices encode equivalently for both seats while retaining any information genuinely needed for simultaneous commitments.</li>
    </ul>
  </section>

  <section class="callout">
    <h2>Frozen public content catalog</h2>
    <p>These are the definitions available to the encoder in this battle. The network does not receive this list directly.</p>
    <pre>{_json(catalog_summary)}</pre>
    <details class="raw"><summary>Show the complete public catalog JSON</summary><pre>{_json(catalog)}</pre></details>
  </section>

  <section>
    <div class="toolbar">
      <input id="search" placeholder="Filter decisions: poison, reroll, Golden Edge, round 4…">
      <select id="seat"><option value="">Both seats</option><option>seat-a</option><option>seat-b</option></select>
      <button onclick="toggleAll(true)">Open shown</button><button onclick="toggleAll(false)">Close all</button>
    </div>
    <div id="decisions">{rows}</div>
  </section>

  <section class="callout">
    <h2>How to read the exact vectors</h2>
    <p>Each decision includes the complete 2,560-value base vector and every authority candidate’s complete 128-value vector. Candidate slots after the authority candidate count are zero-filled, and their mask values are false. Concatenating the base vector with 128 candidate vectors reconstructs the exact 18,944-float observation. The SHA-256 is calculated over its little-endian float32 bytes.</p>
    <p>“Critic value” is PPO’s learned estimate of the acting model’s discounted terminal return from that state. It is an estimate, not a rule-engine score or a guaranteed win probability.</p>
  </section>
  <footer>Generated from real Dice &amp; Destiny authority state and preserved model checkpoints. No game rules, commands, seeds, or policy choices were approximated.</footer>
</main>
<script>
  const search = document.getElementById('search'); const seat = document.getElementById('seat');
  function filter() {{ const q=search.value.toLowerCase(); const s=seat.value;
    document.querySelectorAll('.decision').forEach(el => {{ const ok=(!q||el.dataset.search.includes(q))&&(!s||el.dataset.seat===s); el.hidden=!ok; }}); }}
  search.addEventListener('input',filter); seat.addEventListener('change',filter);
  function toggleAll(open) {{ document.querySelectorAll('.decision:not([hidden])').forEach(el => el.open=open); }}
</script>
</body></html>
"""


def _render_decision(record: dict[str, Any], opened: bool) -> str:
    candidates = "".join(
        f"""<tr class="{"selected" if candidate["selected"] else ""}">
          <td>{candidate["index"]}</td>
          <td><strong>{html.escape(candidate["authority_overlay"])}</strong><br><span class="omitted">Human audit overlay; not model input</span></td>
          <td>{html.escape(candidate["model_visible_summary"])}</td>
          <td>{"yes" if candidate["allowed_by_model_mask"] else "no"}</td>
          <td class="prob">{candidate["probability"]:.6f}</td>
          <td><code>{_json(candidate["payload"], compact=True)}</code></td></tr>"""
        for candidate in record["candidates"]
    )
    hand_rows = "".join(
        f"""<tr>
          <td><strong>{html.escape(card["name"])}</strong><br><code>{html.escape(card["instance_id"])}</code></td>
          <td>{card["energy_cost"]}</td>
          <td>{html.escape(card["rules_text"])}</td>
          <td>{html.escape(str(card["legal_candidate_indices"]))}</td>
          <td>{html.escape(card["model_visibility"])}</td>
        </tr>"""
        for card in record["own_hand"]
    )
    dice_rows = "".join(
        f"""<tr>
          <td>{die["slot"]}</td>
          <td><strong>{html.escape(die["name"])}</strong><br><code>{html.escape(die["definition_id"])}</code></td>
          <td>{die["face"]}</td><td>{die["value"]}</td>
          <td>{html.escape(", ".join(die["symbols"]))}</td><td>{"yes" if die["kept"] else "no"}</td>
          <td><code>{_json(die["model_values"], compact=True)}</code><br><span class="omitted">Not encoded: {html.escape(die["not_encoded"])}</span></td>
        </tr>"""
        for die in record["own_dice_review"]
    ) or """<tr><td colspan="7">No dice are currently rolled. All current-die rows are zero; rolls-used and rolls-remaining scalars still reach the model.</td></tr>"""
    ability_rows = "".join(
        f"""<tr>
          <td>{ability["slot"]}</td>
          <td><strong>{html.escape(ability["name"])}</strong><br><code>{html.escape(ability["definition_id"])}</code></td>
          <td>{html.escape(ability["type"])}</td><td>{ability["energy_cost"]}</td>
          <td>{"yes" if ability["qualified"] else "no"}</td><td>{"yes" if ability["selected"] else "no"}</td>
          <td>{html.escape(ability["rules_text"])}</td>
          <td><code>{ability["base_feature_range"][0]}–{ability["base_feature_range"][1]}</code><br>{html.escape(ability["model_visibility"])}</td>
        </tr>"""
        for ability in record["ability_board_review"]
    )
    feature_rows = "".join(
        f"<tr><td>{feature['index']}</td><td>{feature['value']:.7g}</td><td>{html.escape(feature['meaning'])}</td></tr>"
        for feature in record["base_nonzero"]
    )
    status_payload = record["explicit_status_gap"]
    status_count = sum(len(statuses) for statuses in status_payload.values())
    search_value = html.escape(
        json.dumps(
            {
                "round": record["round"],
                "segment": record["segment"],
                "stage": record["stage"],
                "actor": record["actor"],
                "selected": record["selected_type"],
                "statuses": status_payload,
                "hand": record["own_hand"],
                "command": record["selected_command"],
            },
            sort_keys=True,
        ).lower(),
        quote=True,
    )
    return f"""
    <details class="decision" {"open" if opened else ""} data-seat="{record["actor"]}" data-search="{search_value}">
      <summary>
        <strong>#{record["sequence"]}</strong>
        <span class="{record["actor"]}">{record["actor"]}</span>
        <span>Round {record["round"]}</span>
        <span class="selected">{html.escape(record["selected_type"])} · candidate {record["selected_index"]}</span>
        <span class="wide">value {record["critic_value"]:+.3f}</span>
      </summary>
      <div class="decision-body">
        <section class="callout warning">
          <h3>How this decision is labeled</h3>
          <p><strong>Authority overlay</strong> supplies names, rules text, IDs, and exact commands only so a human can audit the game. <strong>Model-visible</strong> means the numeric information is actually present in the observation or mask. <strong>Probability and critic value</strong> are model outputs.</p>
        </section>
        <div class="grid2">
          <section class="mini"><div class="eyebrow">Decision context</div><dl>
            <dt>Segment</dt><dd>{html.escape(record["segment"])}</dd>
            <dt>Stage</dt><dd>{html.escape(record["stage"])}</dd>
            <dt>Priority</dt><dd>{html.escape(record["priority"])}</dd>
            <dt>Policy</dt><dd><code>{html.escape(record["policy"])}</code></dd>
            <dt>Critic estimate</dt><dd>{record["critic_value"]:+.6f}</dd>
            <dt>Observation nonzero</dt><dd>{record["observation_nonzero"]:,} / {OBSERVATION_SIZE_V2:,}</dd>
            <dt>Observation SHA</dt><dd><code>{record["observation_sha256"]}</code></dd>
          </dl></section>
          <section class="mini"><div class="eyebrow">Selected authority command · not model input</div><pre>{_json(record["selected_command"])}</pre></section>
          <section class="mini"><div class="eyebrow">Acting player · authority overlay</div><pre>{_json(record["own_summary"])}</pre></section>
          <section class="mini"><div class="eyebrow">Visible opponent · authority overlay</div><pre>{_json(record["opponent_summary"])}</pre></section>
        </div>

        <section class="callout">
          <h3>Starting state of the acting player</h3>
          <p>Review the human-readable authority truth together with the explicitly stated model representation before judging the available actions.</p>
          <h4>Cards in hand</h4>
          <div class="table-wrap"><table><thead><tr><th>Authority card · not model input</th><th>Energy</th><th>Authority rules · not model input</th><th>Legal candidate rows now</th><th>What reaches the model</th></tr></thead><tbody>{hand_rows}</tbody></table></div>
          <h4>Current dice</h4>
          <div class="table-wrap"><table><thead><tr><th>Slot</th><th>Authority die · not model input</th><th>Face</th><th>Value</th><th>Symbols</th><th>Kept</th><th>Exact model representation</th></tr></thead><tbody>{dice_rows}</tbody></table></div>
          <h4>Ability board</h4>
          <div class="table-wrap"><table><thead><tr><th>Model slot</th><th>Authority ability · name not model input</th><th>Type</th><th>Energy</th><th>Qualified</th><th>Selected</th><th>Authority rules · not model input</th><th>What reaches the model</th></tr></thead><tbody>{ability_rows}</tbody></table></div>
        </section>

        <section class="callout {"danger" if status_count else ""}">
          <h3>Active statuses: authority truth versus encoded input</h3>
          <p class="{"omitted" if status_count else ""}">{"The identities and stacks below are omitted from the explicit base features." if status_count else "Neither actor has an active status in this state."}</p>
          <pre>{_json(status_payload)}</pre>
        </section>

        <h3>All authority candidates and model probabilities</h3>
        <p>Authority labels and payloads identify the real command for human review. Only the model-visible summary, mask, and numeric observation were available to the policy before it produced the probability.</p>
        <div class="table-wrap"><table><thead><tr><th>Index</th><th>Authority meaning · not model input</th><th>Model-visible candidate summary</th><th>Model mask</th><th>Probability · model output</th><th>Authority payload · not model input</th></tr></thead><tbody>{candidates}</tbody></table></div>

        <h3>Populated named base features</h3>
        <p>Zero-valued and reserved features are omitted from this readable table but retained in the exact raw vector below.</p>
        <div class="table-wrap"><table><thead><tr><th>Index</th><th>Value</th><th>Meaning</th></tr></thead><tbody>{feature_rows}</tbody></table></div>

        <details class="raw"><summary>Exact 2,560-value base vector</summary><pre>{_json(record["base_vector"], compact=True)}</pre></details>
        <details class="raw"><summary>Exact 128-value action mask</summary><pre>{_json(record["action_mask"], compact=True)}</pre></details>
        <details class="raw"><summary>Exact vectors and named populated features for every authority candidate</summary><pre>{_json(record["candidates"])}</pre></details>
        <details class="raw"><summary>Complete viewer-safe authority snapshot, excluding the repeated public catalog</summary><pre>{_json(record["viewer_safe_snapshot"])}</pre></details>
      </div>
    </details>
    """.strip()
