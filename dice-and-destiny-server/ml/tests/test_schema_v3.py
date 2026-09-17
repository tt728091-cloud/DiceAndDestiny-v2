from __future__ import annotations

import copy
from pathlib import Path

import numpy as np
import pytest

from dice_destiny_ml.manifest_v3 import build_observation_manifest_v3
from dice_destiny_ml.schema_v3 import SchemaEncoderV3

SERVER_ROOT = Path(__file__).resolve().parents[2]
CONTENT_ROOT = SERVER_ROOT / "content" / "battle_v1"


def _catalog() -> dict:
    import yaml

    result: dict[str, dict] = {key: {} for key in ("symbols", "dice", "cards", "abilities", "statuses")}
    symbols = yaml.safe_load((CONTENT_ROOT / "symbols.yaml").read_text())
    result["symbols"] = {value["id"]: value for value in symbols["symbols"]}
    for group in ("dice", "cards", "abilities", "statuses"):
        for path in (CONTENT_ROOT / group).glob("*.yaml"):
            value = yaml.safe_load(path.read_text())
            result[group][value["id"]] = value
    return result


def _transition() -> dict:
    catalog = _catalog()
    own_instances = {
        "own-card-1": {"instance_id": "own-card-1", "definition_id": "antidote"},
        "own-card-2": {"instance_id": "own-card-2", "definition_id": "tip_it"},
    }
    actor = {
        "definition_id": "blade_warden",
        "current_health": 11,
        "max_health": 20,
        "energy_points": 3,
        "max_energy_points": 10,
        "hand_count": 2,
        "deck_count": 10,
        "discard_count": 2,
        "removed_count": 6,
        "dice_count": 5,
        "ability_count": 7,
        "max_hand_size": 6,
        "hand": ["own-card-1", "own-card-2"],
        "card_instances": own_instances,
        "offensive_abilities": [
            "sword_cut",
            "shield_bash",
            "golden_edge",
            "perfect_form",
            "venom_strike",
        ],
        "defensive_abilities": ["basic_defense", "protect"],
        "qualified_abilities": ["sword_cut"],
        "selected_ability": "sword_cut",
        "selected_targets": ["seat-b"],
        "statuses": [{"instance_id": "status-poison", "definition_id": "poison", "stacks": 2}],
        "tokens": [],
        "dice": {
            "dice": [
                {"index": 0, "die_id": "standard_d6", "face": 1, "value": 1, "symbols": ["sword"]},
                {"index": 1, "die_id": "standard_d6", "face": 4, "value": 4, "symbols": ["shield"]},
            ],
            "kept_indices": [0],
            "rolls_remaining": 1,
            "complete": False,
        },
    }
    opponent = {
        "definition_id": "venom_goblin",
        "current_health": 9,
        "max_health": 12,
        "energy_points": 1,
        "max_energy_points": 10,
        "hand_count": 4,
        "deck_count": 3,
        "discard_count": 1,
        "removed_count": 4,
        "dice_count": 5,
        "ability_count": 6,
        "offensive_abilities": [
            "jagged_slash",
            "venom_strike",
            "crushing_advance",
            "greedy_blow",
        ],
        "defensive_abilities": ["basic_defense", "protect"],
        "statuses": [{"instance_id": "status-bleed", "definition_id": "bleed", "stacks": 1}],
        "tokens": [],
    }
    action = {
        "type": "planning_select_ability",
        "payload": {"ability_id": "sword_cut", "target_ids": ["seat-b"]},
    }
    return {
        "actor_id": "seat-a",
        "result": {
            "snapshot": {
                "viewer_actor_id": "seat-a",
                "round": 2,
                "completed_rounds": 1,
                "segment": "offensive",
                "stage": "planning",
                "priority_actor_id": "seat-a",
                "actors": {"seat-a": actor, "seat-b": opponent},
                "content_catalog": catalog,
            },
            "legal_actions": [action, {"type": "planning_pass", "payload": {}}],
        },
        "terminal": False,
    }


def _encoder() -> SchemaEncoderV3:
    return SchemaEncoderV3(build_observation_manifest_v3(CONTENT_ROOT))


def test_status_identity_and_stack_change_only_status_row() -> None:
    encoder = _encoder()
    original = _transition()
    base = encoder.encode(original).observation
    mutated = copy.deepcopy(original)
    mutated["result"]["snapshot"]["actors"]["seat-a"]["statuses"][0]["definition_id"] = "bleed"
    mutated["result"]["snapshot"]["actors"]["seat-a"]["statuses"][0]["stacks"] = 3
    changed = encoder.encode(mutated).observation
    indices = np.flatnonzero(base != changed)
    status_range = encoder.manifest.layout.statuses
    assert len(indices) >= 2
    assert np.all((indices >= status_range.offset) & (indices < status_range.end))


def test_hidden_opponent_hand_instances_and_roll_history_do_not_leak() -> None:
    encoder = _encoder()
    transition = _transition()
    original = encoder.encode(transition).observation
    mutated = copy.deepcopy(transition)
    opponent = mutated["result"]["snapshot"]["actors"]["seat-b"]
    opponent["hand"] = ["secret-card"]
    opponent["card_instances"] = {"secret-card": {"instance_id": "secret-card", "definition_id": "tip_it"}}
    opponent["roll_history"] = [{"dice": [{"index": 0, "face": 6, "value": 6, "symbols": ["gold_coin"]}]}]
    np.testing.assert_array_equal(original, encoder.encode(mutated).observation)


def test_viewer_visible_opponent_dice_changes_observation() -> None:
    encoder = _encoder()
    transition = _transition()
    original = encoder.encode(transition).observation
    mutated = copy.deepcopy(transition)
    mutated["result"]["snapshot"]["actors"]["seat-b"]["dice"] = {
        "dice": [{"index": 0, "die_id": "standard_d6", "face": 6, "value": 6, "symbols": ["gold_coin"]}],
        "complete": True,
    }
    assert not np.array_equal(original, encoder.encode(mutated).observation)


def test_exclusive_form_replaces_active_board_and_does_not_sum_inactive_board() -> None:
    encoder = _encoder()
    transition = _transition()
    actor = transition["result"]["snapshot"]["actors"]["seat-a"]
    actor["current_form"] = ""
    actor["offensive_abilities"] = ["sword_cut"]
    actor["defensive_abilities"] = ["protect"]
    decision = encoder.encode(transition)
    ability_range = encoder.manifest.layout.abilities
    rows = decision.observation[ability_range.offset : ability_range.end].reshape(
        ability_range.rows, ability_range.features
    )
    assert int(rows[:, 0].sum()) == 2 + 6  # two own active rows plus six public opponent rows


def test_capacity_overflow_and_unknown_command_fail_before_training() -> None:
    encoder = _encoder()
    transition = _transition()
    overflow = copy.deepcopy(transition)
    overflow["result"]["legal_actions"] = [
        {"type": "pass", "payload": {}} for _ in range(encoder.manifest.maximum_legal_candidates + 1)
    ]
    with pytest.raises(RuntimeError, match="frozen v3 capacity"):
        encoder.encode(overflow)
    unsupported = copy.deepcopy(transition)
    unsupported["result"]["legal_actions"] = [{"type": "new_rule", "payload": {}}]
    with pytest.raises(RuntimeError, match="unsupported v3 command"):
        encoder.encode(unsupported)


def test_every_entity_range_boundary_is_nonoverlapping_and_candidate_order_is_preserved() -> None:
    encoder = _encoder()
    decision = encoder.encode(_transition())
    encoder.manifest.layout.validate()
    candidates = encoder.manifest.layout.candidates
    rows = decision.observation[candidates.offset : candidates.end].reshape(
        candidates.rows, candidates.features
    )
    vocabulary = encoder.manifest.command_vocabulary
    select_index = vocabulary.index("planning_select_ability")
    pass_index = vocabulary.index("planning_pass")
    assert rows[0, 1 + select_index] == 1
    assert rows[1, 1 + pass_index] == 1
    assert decision.action_mask[:2].tolist() == [False, True]
    assert not decision.action_mask[2:].any()
