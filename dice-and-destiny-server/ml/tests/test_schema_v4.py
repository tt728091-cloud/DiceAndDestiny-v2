from __future__ import annotations

import copy

import numpy as np
from test_schema_v3 import CONTENT_ROOT, _transition

from dice_destiny_ml.manifest_v4 import ObservationManifestV4, build_observation_manifest_v4
from dice_destiny_ml.schema_v4 import SchemaEncoderV4


def test_v4_manifest_round_trip_and_nonoverlapping_detail_capacity(tmp_path) -> None:
    manifest = build_observation_manifest_v4(CONTENT_ROOT, eligible_combatants=["blade_warden"])
    manifest.layout.validate()
    assert manifest.layout.details.offset == manifest.base.layout.observation_size
    assert manifest.layout.details.rows == 2048
    assert len(manifest.content_path_hashes) == len(set(manifest.content_path_hashes.values()))
    path = tmp_path / "manifest-v4.json"
    manifest.save(path)
    assert ObservationManifestV4.load(path) == manifest


def test_v4_compacts_dice_subsets_without_losing_reachable_keep_choices() -> None:
    manifest = build_observation_manifest_v4(CONTENT_ROOT)
    encoder = SchemaEncoderV4(manifest)
    transition = _transition()
    actor = transition["result"]["snapshot"]["actors"]["seat-a"]
    actor["dice"]["dice"].append(
        {"index": 2, "die_id": "standard_d6", "face": 2, "value": 2, "symbols": ["sword"]}
    )
    actions = [
        {"type": "planning_keep", "payload": {"kept_indices": [0]}},
        {"type": "planning_keep", "payload": {"kept_indices": [0, 1]}},
        {"type": "planning_keep", "payload": {"kept_indices": [0, 2]}},
        {"type": "planning_keep", "payload": {"kept_indices": [0, 1, 2]}},
        {"type": "planning_reroll", "payload": {"reroll_indices": [1]}},
        {"type": "planning_reroll", "payload": {"reroll_indices": [1, 2]}},
        {"type": "planning_pass", "payload": {}},
    ]
    transition["result"]["legal_actions"] = actions
    decision = encoder.encode(transition)
    assert decision.authority_indices.tolist() == [1, 2, 5, 6]
    assert decision.action_mask[:4].all()
    assert not decision.action_mask[4:].any()


def test_v4_exact_payload_fingerprint_and_detail_rows_change_for_nested_action() -> None:
    manifest = build_observation_manifest_v4(CONTENT_ROOT)
    encoder = SchemaEncoderV4(manifest)
    transition = _transition()
    transition["result"]["legal_actions"] = [
        {
            "type": "commit_interaction",
            "payload": {"card_ids": ["own-card-2"], "planning_adjustments": [{"die_index": 0, "face": 6}]},
        },
        {
            "type": "commit_interaction",
            "payload": {"card_ids": ["own-card-2"], "planning_adjustments": [{"die_index": 1, "face": 5}]},
        },
    ]
    original = encoder.encode(transition)
    candidate = manifest.layout.candidates
    rows = original.observation[candidate.offset : candidate.end].reshape(candidate.rows, candidate.features)
    assert not np.array_equal(rows[0, 64:96], rows[1, 64:96])
    mutated = copy.deepcopy(transition)
    mutated["result"]["legal_actions"][1]["payload"]["planning_adjustments"][0]["face"] = 4
    changed = encoder.encode(mutated)
    details = manifest.layout.details
    assert not np.array_equal(
        original.observation[details.offset : details.end],
        changed.observation[details.offset : details.end],
    )


def test_v4_preserves_exact_card_and_ability_mechanics_in_detail_bank() -> None:
    manifest = build_observation_manifest_v4(CONTENT_ROOT)
    encoder = SchemaEncoderV4(manifest)
    transition = _transition()
    original = encoder.encode(transition).observation
    mutated = copy.deepcopy(transition)
    mutated["result"]["snapshot"]["content_catalog"]["abilities"]["sword_cut"]["qualification"][
        "activation_tiers"
    ][0]["operations"][0]["amount"] += 1
    changed = encoder.encode(mutated).observation
    details = manifest.layout.details
    assert not np.array_equal(original[details.offset :], changed[details.offset :])
