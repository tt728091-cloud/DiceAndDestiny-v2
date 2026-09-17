from __future__ import annotations

from collections import Counter

from dice_destiny_ml.evaluation import _damage_by_policy, _summarize_damage


def test_damage_ledger_separates_raw_resolved_and_actual_damage() -> None:
    damage_by_seat = {
        "seat-a": {
            "raw_attack": 0,
            "raw_bleed": 0,
            "raw_poison": 1,
            "raw_total": 1,
            "resolved_total": 1,
            "actual_total": 1,
        },
        "seat-b": {
            "raw_attack": 6,
            "raw_bleed": 1,
            "raw_poison": 0,
            "raw_total": 7,
            "resolved_total": 3,
            "actual_total": 3,
        },
    }

    result = _damage_by_policy(
        damage_by_seat,
        {"seat-a": "learner", "seat-b": "opponent"},
        rounds=4,
    )

    assert result["learner"] == {
        "games": 1,
        "rounds": 4,
        "raw_outgoing_attack": 6,
        "raw_outgoing_bleed": 1,
        "raw_outgoing_poison": 0,
        "raw_outgoing_total": 7,
        "resolved_outgoing_total": 3,
        "actual_outgoing_total": 3,
        "raw_incoming_attack": 0,
        "raw_incoming_bleed": 0,
        "raw_incoming_poison": 1,
        "raw_incoming_total": 1,
        "resolved_incoming_total": 1,
        "actual_incoming_total": 1,
    }
    assert result["opponent"]["raw_outgoing_poison"] == 1
    assert result["opponent"]["actual_incoming_total"] == 3


def test_damage_summary_reports_each_resolution_stage_per_round() -> None:
    summary = _summarize_damage(
        Counter(
            rounds=4,
            raw_outgoing_attack=12,
            raw_outgoing_bleed=4,
            raw_outgoing_poison=2,
            raw_outgoing_total=18,
            resolved_outgoing_total=10,
            actual_outgoing_total=8,
            raw_incoming_attack=8,
            raw_incoming_bleed=2,
            raw_incoming_poison=2,
            raw_incoming_total=12,
            resolved_incoming_total=7,
            actual_incoming_total=6,
        )
    )

    assert summary["average_raw_outgoing_total_per_round"] == 4.5
    assert summary["average_resolved_outgoing_total_per_round"] == 2.5
    assert summary["average_actual_outgoing_total_per_round"] == 2
    assert summary["average_raw_incoming_poison_per_round"] == 0.5
    assert summary["average_actual_damage_advantage_per_round"] == 0.5
