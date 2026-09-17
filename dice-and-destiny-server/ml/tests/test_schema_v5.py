from __future__ import annotations

import copy

import numpy as np
from test_schema_v3 import CONTENT_ROOT, _transition

from dice_destiny_ml.manifest_v5 import ObservationManifestV5, build_observation_manifest_v5
from dice_destiny_ml.schema_v5 import SchemaEncoderV5


def test_v5_manifest_round_trip_is_version_separated(tmp_path) -> None:
    manifest = build_observation_manifest_v5(CONTENT_ROOT, eligible_combatants=["blade_warden"])
    path = tmp_path / "manifest-v5.json"
    manifest.save(path)
    loaded = ObservationManifestV5.load(path)
    assert loaded == manifest
    assert loaded.observation_schema == "dice-and-destiny-observation-v5"
    assert loaded.semantic_candidate_schema == "dice-and-destiny-semantic-candidate-v1"


def test_v5_semantic_candidate_ignores_window_and_card_instance_identity() -> None:
    encoder = SchemaEncoderV5(build_observation_manifest_v5(CONTENT_ROOT))
    transition = _transition()
    transition["result"]["legal_actions"] = [
        {
            "type": "planning_commit_cards",
            "payload": {
                "card_ids": ["own-card-2"],
                "checkpoint": {
                    "iteration": 1,
                    "planning_cycle": 2,
                    "segment": "offensive",
                    "stage": "planning",
                    "window_id": "offensive-planning-r2-runtime-17",
                },
                "pending_input_id": "input-runtime-17",
                "target_ids": ["seat-a"],
            },
        }
    ]
    original = encoder.encode(transition)

    mutated = copy.deepcopy(transition)
    actor = mutated["result"]["snapshot"]["actors"]["seat-a"]
    actor["hand"] = ["own-card-1", "different-runtime-card"]
    actor["card_instances"] = {
        "own-card-1": {
            "instance_id": "own-card-1",
            "definition_id": "antidote",
        },
        "different-runtime-card": {
            "instance_id": "different-runtime-card",
            "definition_id": "tip_it",
        }
    }
    payload = mutated["result"]["legal_actions"][0]["payload"]
    payload["card_ids"] = ["different-runtime-card"]
    payload["checkpoint"]["window_id"] = "offensive-planning-r99-runtime-9000"
    payload["pending_input_id"] = "input-runtime-9000"
    changed = encoder.encode(mutated)
    np.testing.assert_array_equal(original.observation, changed.observation)
    np.testing.assert_array_equal(original.authority_indices, changed.authority_indices)


def test_v5_semantic_candidate_changes_for_real_game_meaning() -> None:
    encoder = SchemaEncoderV5(build_observation_manifest_v5(CONTENT_ROOT))
    transition = _transition()
    transition["result"]["legal_actions"] = [
        {
            "type": "commit_interaction",
            "payload": {
                "card_ids": ["own-card-2"],
                "planning_adjustments": [{"die_index": 0, "face": 6}],
            },
        }
    ]
    original = encoder.encode(transition).observation
    changed_transition = copy.deepcopy(transition)
    changed_transition["result"]["legal_actions"][0]["payload"]["planning_adjustments"][0]["face"] = 5
    changed = encoder.encode(changed_transition).observation
    candidate = encoder.manifest.layout.candidates
    assert not np.array_equal(
        original[candidate.offset : candidate.end], changed[candidate.offset : candidate.end]
    )


def test_v5_target_roles_are_seat_invariant() -> None:
    encoder = SchemaEncoderV5(build_observation_manifest_v5(CONTENT_ROOT))
    transition = _transition()
    action = {"type": "planning_select_targets", "payload": {"target_ids": ["seat-b"]}}
    semantic_a = encoder.semantic_action(
        action,
        snapshot=transition["result"]["snapshot"],
        viewer="seat-a",
    )
    swapped = copy.deepcopy(transition["result"]["snapshot"])
    swapped["viewer_actor_id"] = "seat-b"
    swapped["actors"] = {"seat-a": swapped["actors"]["seat-b"], "seat-b": swapped["actors"]["seat-a"]}
    semantic_b = encoder.semantic_action(
        {"type": "planning_select_targets", "payload": {"target_ids": ["seat-a"]}},
        snapshot=swapped,
        viewer="seat-b",
    )
    assert semantic_a == semantic_b


def test_v5_runtime_source_references_do_not_enter_hash_rows_or_collide() -> None:
    encoder = SchemaEncoderV5(build_observation_manifest_v5(CONTENT_ROOT))
    transition = _transition()
    source_id = "source-r7-venom_strike-273"
    pending_id = "input-damage-r7-228-seat-a-229"
    # These two real authority identifiers have the same legacy 32-bit hash.
    # They must be represented by their surrounding game meaning, never by
    # their arbitrary per-battle labels.
    snapshot = transition["result"]["snapshot"]
    snapshot["pending_damage"] = {
        "id": pending_id,
        "sources": [
            {
                "id": source_id,
                "source_actor_id": "seat-b",
                "source_content_id": "venom_strike",
                "target_actor_id": "seat-a",
                "base_amount": 4,
            }
        ],
    }
    transition["result"]["legal_actions"] = [
        {
            "type": "planning_select_ability",
            "payload": {
                "ability_id": "basic_defense",
                "pending_input_id": pending_id,
                "target_ids": [source_id],
            },
        }
    ]
    original = encoder.encode(transition)

    changed_ids = copy.deepcopy(transition)
    changed_pending = "input-damage-r99-900-seat-a-901"
    changed_source = "source-r99-venom_strike-902"
    changed_ids["result"]["snapshot"]["pending_damage"]["id"] = changed_pending
    changed_ids["result"]["snapshot"]["pending_damage"]["sources"][0]["id"] = changed_source
    payload = changed_ids["result"]["legal_actions"][0]["payload"]
    payload["pending_input_id"] = changed_pending
    payload["target_ids"] = [changed_source]
    changed = encoder.encode(changed_ids)
    np.testing.assert_array_equal(original.observation, changed.observation)

    changed_meaning = copy.deepcopy(transition)
    changed_meaning["result"]["snapshot"]["pending_damage"]["sources"][0][
        "source_content_id"
    ] = "sword_cut"
    assert not np.array_equal(original.observation, encoder.encode(changed_meaning).observation)
