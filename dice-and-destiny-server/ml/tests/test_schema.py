from __future__ import annotations

import json

import numpy as np

from dice_destiny_ml.policies import HeuristicPolicy, RandomLegalPolicy
from dice_destiny_ml.schema import MAX_ACTIONS, OBSERVATION_SIZE, SchemaEncoder


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
