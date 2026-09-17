from __future__ import annotations

import base64
import inspect
import json

import numpy as np

from dice_destiny_ml.policies import HeuristicPolicy, MechanicsPolicyV2, RandomLegalPolicy
from dice_destiny_ml.schema import MAX_ACTIONS, OBSERVATION_SIZE, SchemaEncoder
from dice_destiny_ml.schema_v2 import (
    ACTION_FEATURES_V2,
    BASE_FEATURES_V2,
    MAX_ACTIONS_V2,
    OBSERVATION_SIZE_V2,
    SchemaEncoderV2,
)


def test_schema_has_fixed_shape_and_masks_unused_candidates() -> None:
    transition = decision_transition(
        [
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_roll", "payload": "{}"},
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_pass", "payload": "{}"},
        ]
    )
    encoded = SchemaEncoder().encode(transition)
    assert encoded.observation.shape == (OBSERVATION_SIZE,)
    assert encoded.observation.dtype == np.float32
    assert encoded.action_mask.shape == (MAX_ACTIONS,)
    assert encoded.action_mask.tolist()[:3] == [True, True, False]


def test_encoded_transport_round_trips_and_matches_raw_schema() -> None:
    transition = decision_transition(
        [
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_roll", "payload": "{}"},
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_pass", "payload": "{}"},
        ]
    )
    encoder = SchemaEncoder()
    raw = encoder.encode(transition)
    transition["encoded_decision"] = {
        "observation_f32_le_base64": base64.b64encode(raw.observation.astype("<f4").tobytes()).decode(),
        "action_mask_bits_base64": base64.b64encode(
            np.packbits(raw.action_mask, bitorder="little").tobytes()
        ).decode(),
        "candidate_count": int(raw.action_mask.sum()),
        "candidate_types": ["planning_roll", "planning_pass"],
    }
    encoder.assert_transport_parity(transition)
    stripped = dict(transition)
    stripped["result"] = {}
    transported = encoder.encode(stripped)
    np.testing.assert_array_equal(transported.observation, raw.observation)
    np.testing.assert_array_equal(transported.action_mask, raw.action_mask)


def test_encoder_uses_viewer_private_hand_but_not_opponent_hidden_fields() -> None:
    transition = decision_transition([])
    transition["result"]["legal_actions"] = [
        {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_pass", "payload": "{}"}
    ]
    baseline = SchemaEncoder().encode(transition).observation
    hidden_mutation = json.loads(json.dumps(transition))
    hidden_mutation["result"]["snapshot"]["actors"]["seat-b"].update(
        {
            "hand": ["secret"],
            "card_instances": {"secret": {"definition_id": "future_secret_card"}},
        }
    )
    mutated = SchemaEncoder().encode(hidden_mutation).observation
    np.testing.assert_array_equal(baseline, mutated)


def test_heuristic_does_not_cycle_on_keep_when_pass_is_available() -> None:
    transition = decision_transition(
        [
            {
                "battle_id": "battle",
                "actor_id": "seat-a",
                "type": "planning_keep",
                "payload": json.dumps({"kept_indices": [0, 1, 2, 3, 4]}),
            },
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_pass", "payload": "{}"},
        ]
    )
    decision = SchemaEncoder().encode(transition)
    policy = HeuristicPolicy()
    policy.reset(1, "seat-a")
    assert policy.select(transition, decision) == 1


def test_random_baseline_does_not_cycle_on_keep_when_progress_is_available() -> None:
    transition = decision_transition(
        [
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_keep", "payload": "{}"},
            {"battle_id": "battle", "actor_id": "seat-a", "type": "planning_pass", "payload": "{}"},
        ]
    )
    decision = SchemaEncoder().encode(transition)
    policy = RandomLegalPolicy()
    policy.reset(1, "seat-a")
    assert {policy.select(transition, decision) for _ in range(20)} == {1}


def test_v2_exposes_roll_budget_dice_board_tier_progress_and_candidate_link() -> None:
    transition = decision_transition_v2()
    encoded = SchemaEncoderV2().encode(transition)
    assert encoded.observation.shape == (OBSERVATION_SIZE_V2,)
    assert encoded.action_mask.shape == (MAX_ACTIONS_V2,)
    base = encoded.observation[:BASE_FEATURES_V2]
    np.testing.assert_allclose(base[48:51], [1 / 3, 1.0, 2 / 3])
    # First die is explicit, kept, and carries the first catalog symbol.
    assert base[128] == 1
    assert base[132] == 1
    assert base[134] == 1
    # First authored ability is qualified and its first tier is fully met.
    assert base[256:261].tolist() == [1, 1, 0, 1, 0]
    assert base[256 + 16 : 256 + 20].tolist() == [1, 0, 1, 1]
    candidate = encoded.observation[BASE_FEATURES_V2 : BASE_FEATURES_V2 + ACTION_FEATURES_V2]
    assert candidate[18] == 1
    assert candidate[32] == 1  # exact board-slot reference, not an ID bucket


def test_v2_ignores_opponent_private_fields_and_rejects_capacity_loss() -> None:
    transition = decision_transition_v2()
    baseline = SchemaEncoderV2().encode(transition).observation
    hidden = json.loads(json.dumps(transition))
    hidden["result"]["snapshot"]["actors"]["seat-b"].update(
        {"hand": ["secret"], "card_instances": {"secret": {"definition_id": "secret"}}}
    )
    np.testing.assert_array_equal(baseline, SchemaEncoderV2().encode(hidden).observation)
    overflow = json.loads(json.dumps(transition))
    overflow["result"]["snapshot"]["content_catalog"]["symbols"].update(
        {f"symbol_{index}": {} for index in range(9)}
    )
    with np.testing.assert_raises_regex(RuntimeError, "symbols; v2 capacity"):
        SchemaEncoderV2().encode(overflow)


def test_v2_masks_all_provisional_keep_candidates() -> None:
    transition = decision_transition_v2()
    transition["result"]["snapshot"]["actors"]["seat-a"]["dice"]["kept_indices"] = [0]
    transition["result"]["legal_actions"] = [
        {"type": "planning_keep", "payload": {"kept_indices": [0]}},
        {"type": "planning_keep", "payload": {"kept_indices": []}},
        {"type": "planning_reroll", "payload": {"reroll_indices": [1]}},
    ]
    encoded = SchemaEncoderV2().encode(transition)
    assert encoded.action_mask[:4].tolist() == [False, False, True, False]


def test_v2_masks_empty_deck_draw_recycling_but_not_other_cards() -> None:
    transition = decision_transition_v2()
    actor = transition["result"]["snapshot"]["actors"]["seat-a"]
    catalog = transition["result"]["snapshot"]["content_catalog"]
    actor["deck_count"] = 0
    actor["card_instances"] = {
        "draw": {"definition_id": "draw_card"},
        "effect": {"definition_id": "effect_card"},
    }
    catalog["cards"] = {
        "draw_card": {"operations": [{"type": "draw_cards", "amount": 1}]},
        "effect_card": {"operations": [{"type": "gain_resource", "resource": "energy"}]},
    }
    transition["result"]["legal_actions"] = [
        {"type": "planning_commit_cards", "payload": {"card_ids": ["draw"]}},
        {"type": "planning_commit_cards", "payload": {"card_ids": ["effect"]}},
        {"type": "planning_pass", "payload": {}},
    ]
    encoded = SchemaEncoderV2().encode(transition)
    assert encoded.action_mask[:4].tolist() == [False, True, True, False]


def test_mechanics_teacher_is_content_id_neutral_and_values_roll_option() -> None:
    transition = decision_transition_v2()
    snapshot = transition["result"]["snapshot"]
    catalog = snapshot["content_catalog"]
    ability = catalog["abilities"].pop("generic_attack")
    ability["id"] = "opaque_move"
    ability["qualification"]["activation_tiers"] = [
        {
            "id": f"level_{count}",
            "requirements": {"all": [{"type": "symbol_count", "symbol_id": "blade", "exact": count}]},
            "operations": [{"type": "deal_damage", "target": "selected_targets", "amount": count + 2}],
        }
        for count in (3, 4, 5)
    ]
    catalog["abilities"]["opaque_move"] = ability
    actor = snapshot["actors"]["seat-a"]
    actor["offensive_abilities"] = ["opaque_move"]
    actor["qualified_abilities"] = ["opaque_move"]
    actor["dice"] = {
        "rolls_used": 1,
        "max_rolls": 3,
        "rolls_remaining": 2,
        "dice": [
            {"index": 0, "die_id": "generic_d6", "face": 6, "value": 6, "symbols": ["guard"]},
            {"index": 1, "die_id": "generic_d6", "face": 1, "value": 1, "symbols": ["blade"]},
            {"index": 2, "die_id": "generic_d6", "face": 2, "value": 2, "symbols": ["blade"]},
            {"index": 3, "die_id": "generic_d6", "face": 4, "value": 4, "symbols": ["guard"]},
            {"index": 4, "die_id": "generic_d6", "face": 3, "value": 3, "symbols": ["blade"]},
        ],
        "symbol_counts": {"blade": 3, "guard": 2},
    }
    transition["result"]["legal_actions"] = [
        {
            "type": "planning_select_ability",
            "payload": json.dumps({"ability_id": "opaque_move", "target_ids": ["seat-b"]}),
        },
        {"type": "planning_reroll", "payload": json.dumps({"reroll_indices": [0, 3]})},
    ]
    teacher = MechanicsPolicyV2()
    teacher.reset(1, "seat-a")
    assert teacher.select(transition, SchemaEncoderV2().encode(transition)) == 1
    source = inspect.getsource(MechanicsPolicyV2)
    for authored_id in ("sword_cut", "golden_edge", "venom_strike", "standard_d6"):
        assert authored_id not in source


def test_v2_includes_runtime_authored_modifier_tier_when_base_bonus_list_is_null() -> None:
    transition = decision_transition_v2()
    snapshot = transition["result"]["snapshot"]
    actor = snapshot["actors"]["seat-a"]
    ability = snapshot["content_catalog"]["abilities"]["generic_attack"]
    ability["qualification"]["conditional_bonuses"] = None
    actor["card_instances"]["upgrade-1"] = {
        "instance_id": "upgrade-1",
        "definition_id": "generic_upgrade",
    }
    actor["card_instances"]["upgrade-2"] = {
        "instance_id": "upgrade-2",
        "definition_id": "generic_upgrade",
    }
    actor["ability_modifiers"] = [
        {
            "source_card_instance_id": "upgrade-1",
            "ability_id": "generic_attack",
            "bonus_id": "pair_bonus",
        },
        {
            "source_card_instance_id": "upgrade-2",
            "ability_id": "generic_attack",
            "bonus_id": "pair_bonus",
        },
    ]
    snapshot["content_catalog"]["cards"]["generic_upgrade"] = {
        "id": "generic_upgrade",
        "cost": {"energy": 1},
        "operations": [
            {
                "type": "apply_ability_modifier",
                "modifier": {
                    "add_conditional_bonus": {
                        "id": "pair_bonus",
                        "requirements": {"all": [{"type": "number_pattern", "pattern": "pair_or_better"}]},
                        "operations": [
                            {"type": "apply_status", "target": "selected_targets", "stack_count": 1}
                        ],
                    }
                },
            }
        ],
    }
    base = SchemaEncoderV2().encode(transition).observation[:BASE_FEATURES_V2]
    assert base[256 + 16 + 28] == 1  # second, dynamically authored tier is present
    assert np.isclose(base[256 + 16 + 28 + 23], 0.2)  # two stacks, one distinct tier


def decision_transition(actions: list[dict]) -> dict:
    return {
        "actor_id": "seat-a",
        "terminal": False,
        "result": {
            "legal_actions": actions,
            "snapshot": {
                "viewer_actor_id": "seat-a",
                "segment": "offensive",
                "stage": "planning",
                "round": 1,
                "actors": {
                    "seat-a": {
                        "max_health": 20,
                        "current_health": 20,
                        "max_energy_points": 5,
                        "energy_points": 2,
                        "hand_count": 1,
                        "hand": ["own-card"],
                        "card_instances": {"own-card": {"definition_id": "battle_focus"}},
                    },
                    "seat-b": {
                        "max_health": 20,
                        "current_health": 20,
                        "max_energy_points": 5,
                        "energy_points": 2,
                        "hand_count": 1,
                    },
                },
            },
        },
    }


def decision_transition_v2() -> dict:
    transition = decision_transition(
        [
            {
                "battle_id": "battle",
                "actor_id": "seat-a",
                "type": "planning_select_ability",
                "payload": json.dumps({"ability_id": "generic_attack"}),
            },
            {
                "battle_id": "battle",
                "actor_id": "seat-a",
                "type": "planning_reroll",
                "payload": json.dumps({"reroll_indices": [1]}),
            },
        ]
    )
    snapshot = transition["result"]["snapshot"]
    snapshot["content_catalog"] = {
        "symbols": {"blade": {"id": "blade"}, "guard": {"id": "guard"}},
        "dice": {
            "generic_d6": {
                "id": "generic_d6",
                "faces": [
                    {"number": 1, "symbol": "blade"},
                    {"number": 2, "symbol": "blade"},
                    {"number": 3, "symbol": "blade"},
                    {"number": 4, "symbol": "guard"},
                    {"number": 5, "symbol": "guard"},
                    {"number": 6, "symbol": "guard"},
                ],
            }
        },
        "cards": {},
        "abilities": {
            "generic_attack": {
                "id": "generic_attack",
                "type": "offensive",
                "cost": {"energy": 0},
                "usage": {"maximum_per_segment": 1},
                "targeting": {"selector": "one_enemy", "minimum": 1, "maximum": 1},
                "qualification": {
                    "activation_tiers": [
                        {
                            "id": "base",
                            "requirements": {
                                "all": [{"type": "symbol_count", "symbol_id": "blade", "minimum": 2}]
                            },
                            "operations": [
                                {"type": "deal_damage", "target": "selected_targets", "amount": 4}
                            ],
                        }
                    ],
                    "conditional_bonuses": [],
                },
            }
        },
        "statuses": {},
    }
    own = snapshot["actors"]["seat-a"]
    own.update(
        {
            "offensive_abilities": ["generic_attack"],
            "defensive_abilities": [],
            "qualified_abilities": ["generic_attack"],
            "dice": {
                "rolls_used": 1,
                "max_rolls": 3,
                "rolls_remaining": 2,
                "dice": [
                    {"index": 0, "die_id": "generic_d6", "face": 1, "value": 1, "symbols": ["blade"]},
                    {"index": 1, "die_id": "generic_d6", "face": 2, "value": 2, "symbols": ["blade"]},
                ],
                "kept_indices": [0],
                "symbol_counts": {"blade": 2},
            },
        }
    )
    return transition
