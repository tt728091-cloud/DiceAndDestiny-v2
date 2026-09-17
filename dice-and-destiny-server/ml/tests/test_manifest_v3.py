from __future__ import annotations

from dataclasses import asdict
from pathlib import Path

import pytest

from dice_destiny_ml.manifest_v3 import (
    ABILITY_FEATURES_V3,
    ACTION_FEATURES_V3,
    CARD_FEATURES_V3,
    DIE_FEATURES_V3,
    STATUS_FEATURES_V3,
    ObservationManifestV3,
    build_observation_manifest_v3,
)

SERVER_ROOT = Path(__file__).resolve().parents[2]
CONTENT_ROOT = SERVER_ROOT / "content" / "battle_v1"


def test_manifest_scans_complete_eligible_roster_and_generates_nonoverlapping_layout(
    tmp_path: Path,
) -> None:
    manifest = build_observation_manifest_v3(
        CONTENT_ROOT,
        eligible_combatants=["blade_warden", "venom_goblin"],
        observed_candidate_maximum=128,
        generated_scenarios=500,
    )
    assert manifest.eligible_combatants == ("blade_warden", "venom_goblin")
    assert manifest.symbol_vocabulary == ("gold_coin", "shield", "sword")
    assert {"bleed", "blind", "entangle", "poison", "volatile_poison"} == set(manifest.status_vocabulary)
    assert manifest.maximum_dice_per_actor == 5
    assert manifest.maximum_active_abilities_per_actor == 7
    assert manifest.maximum_visible_hand_cards == 20
    assert manifest.capacity_evidence["maximum_authored_hand_limit"] == 6
    assert manifest.maximum_statuses_per_actor == len(manifest.status_vocabulary)
    assert manifest.maximum_tiers_per_ability == 5
    assert manifest.capacity_evidence["runtime_ability_modifier_bonus_ids"] == ["sharpened_pair_bleed"]
    assert manifest.maximum_legal_candidates >= 128
    assert manifest.layout.dice.features == DIE_FEATURES_V3
    assert manifest.layout.abilities.features == ABILITY_FEATURES_V3
    assert manifest.layout.statuses.features == STATUS_FEATURES_V3
    assert manifest.layout.cards.features == CARD_FEATURES_V3
    assert manifest.layout.candidates.features == ACTION_FEATURES_V3
    manifest.layout.validate()
    path = tmp_path / "manifest.json"
    manifest.save(path)
    assert ObservationManifestV3.load(path) == manifest


def test_manifest_hash_fails_if_frozen_capacity_is_silently_changed(tmp_path: Path) -> None:
    manifest = build_observation_manifest_v3(CONTENT_ROOT)
    path = tmp_path / "manifest.json"
    manifest.save(path)
    value = asdict(manifest)
    value["maximum_visible_hand_cards"] += 1
    from dice_destiny_ml.champions import atomic_json_write

    atomic_json_write(path, value)
    with pytest.raises(RuntimeError, match="manifest hash mismatch"):
        ObservationManifestV3.load(path)


def test_unknown_eligible_combatant_fails_preflight() -> None:
    with pytest.raises(ValueError, match="missing from content"):
        build_observation_manifest_v3(CONTENT_ROOT, eligible_combatants=["not_authored"])
