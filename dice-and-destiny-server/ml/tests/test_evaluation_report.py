from __future__ import annotations

from collections import Counter

from dice_destiny_ml.evaluation import _record_completed_offensive_reaction, _summarize_behavior
from dice_destiny_ml.evaluation_report import (
    write_campaign_statistics_html,
    write_evaluation_statistics_html,
)


def test_damage_per_offensive_segment_uses_settled_damage() -> None:
    behavior = _summarize_behavior(
        Counter(actual_damage_dealt=15, offensive_actual_damage=0, offensive_segments=3)
    )
    assert behavior["average_damage_per_offensive_segment"] == 5


def _summary(challenger: str, global_spec: str) -> dict:
    return {
        "games": 1000,
        "mean_actions": 150.5,
        "maximum_candidates": 94,
        "policies": {
            challenger: {"games": 1000, "wins": 200, "win_rate": 0.2, "adjudicated_score": 0.22},
            global_spec: {"games": 1000, "wins": 760, "win_rate": 0.76, "adjudicated_score": 0.78},
        },
        "behavior_by_policy": {
            challenger: {
                "ability_selected::sword_cut": 120,
                "ability_offered::sword_cut": 300,
                "card_selected::tip_it": 22,
                "card_offered::tip_it": 90,
                "average_damage_per_offensive_segment": 2.5,
                "average_status_stacks_per_offensive_segment": 0.4,
                "mean_rolls_used_at_ability_selection": 2.1,
                "qualified_reroll_rate": 0.25,
                "pass_rate": 0.1,
                "offensive_pre_reaction_outcomes": 100,
                "offensive_pre_reaction_ability_selected": 75,
                "offensive_pre_reaction_passed": 25,
                "offensive_pre_reaction_ability_rate": 0.75,
                "offensive_pre_reaction_pass_rate": 0.25,
                "offensive_post_reaction_outcomes": 100,
                "offensive_post_reaction_ability_selected": 70,
                "offensive_post_reaction_passed": 30,
                "offensive_post_reaction_ability_rate": 0.7,
                "offensive_post_reaction_pass_rate": 0.3,
                "offensive_pre_reaction_selected_roll_1": 15,
                "offensive_pre_reaction_selected_roll_2": 25,
                "offensive_pre_reaction_selected_roll_3": 35,
                "offensive_selection_roll_1_rate": 0.2,
                "offensive_selection_roll_2_rate": 1 / 3,
                "offensive_selection_roll_3_rate": 7 / 15,
            },
            global_spec: {
                "ability_selected::sword_cut": 180,
                "average_damage_per_offensive_segment": 3.7,
                "average_status_stacks_per_offensive_segment": 0.8,
                "mean_rolls_used_at_ability_selection": 2.6,
                "qualified_reroll_rate": 0.5,
                "pass_rate": 0.04,
            },
        },
        "damage_by_policy": {
            challenger: {
                "rounds": 5000,
                "average_raw_outgoing_attack_per_round": 4.0,
                "average_raw_outgoing_bleed_per_round": 0.8,
                "average_raw_outgoing_poison_per_round": 0.4,
                "average_raw_outgoing_total_per_round": 5.2,
                "average_resolved_outgoing_total_per_round": 2.6,
                "average_actual_outgoing_total_per_round": 2.4,
                "average_raw_incoming_attack_per_round": 4.5,
                "average_raw_incoming_bleed_per_round": 1.0,
                "average_raw_incoming_poison_per_round": 0.2,
                "average_raw_incoming_total_per_round": 5.7,
                "average_resolved_incoming_total_per_round": 2.8,
                "average_actual_incoming_total_per_round": 2.5,
                "average_actual_damage_advantage_per_round": -0.1,
            }
        },
    }


def test_evaluation_and_campaign_html_include_behavior_comparisons(tmp_path) -> None:
    challenger_checkpoint = tmp_path / "intervals/interval-006/raw-ppo-challenger.zip"
    global_checkpoint = tmp_path / "global.zip"
    challenger = f"model:{challenger_checkpoint}"
    global_spec = f"model:{global_checkpoint}"
    summary = _summary(challenger, global_spec)
    evaluation_dir = tmp_path / "intervals/interval-006/global-1k"
    evaluation_dir.mkdir(parents=True)
    summary_path = evaluation_dir / "summary.json"
    import json

    summary_path.write_text(json.dumps(summary))
    write_evaluation_statistics_html(
        summary,
        evaluation_dir / "statistics.html",
        title="Test evaluation",
        focus_policy=challenger,
    )
    state = {
        "campaign_id": "test-campaign",
        "fixed_global_champion": {"checkpoint_path": str(global_checkpoint)},
        "attempts": [
            {
                "interval": 6,
                "challenger": {
                    "checkpoint_path": str(challenger_checkpoint),
                    "learner_steps": 313344,
                },
                "evaluation": {
                    "wins": 200,
                    "draws": 40,
                    "losses": 760,
                    "adjusted_score": 0.22,
                    "elapsed_seconds": 95.5,
                    "summary": str(summary_path),
                },
                "training": {"elapsed_seconds": 75.25},
                "outcome": "baseline-established",
            }
        ],
    }
    write_campaign_statistics_html(tmp_path, state)
    standalone = (evaluation_dir / "statistics.html").read_text()
    checkpoint = (tmp_path / "intervals/interval-006/checkpoint-statistics.html").read_text()
    overview = (tmp_path / "statistics.html").read_text()
    gameplay_overview = (tmp_path / "gameplay-statistics.html").read_text()
    metrics = json.loads((tmp_path / "checkpoint-metrics.json").read_text())
    artifact_manifest = json.loads((tmp_path / "metrics-artifacts.json").read_text())
    assert "Actual damage dealt / round" in standalone
    assert "ability selected · sword cut" in standalone
    assert "Offensive ability decisions" in standalone
    assert "Percent of ability selections" in standalone
    assert "Damage per round" in standalone
    assert "Actual health removed" in checkpoint
    assert "Global champion" in checkpoint
    assert "vs previous" in overview
    assert "Train + eval" in overview
    assert "Gameplay progression against the global champion" in overview
    assert "standalone gameplay-statistics sheet" in overview
    assert "ability 75.00% · pass 25.00%" in overview
    assert "attack/bleed/poison" in overview
    assert "-0.100" in overview
    assert "ability 75.00% · pass 25.00%" in gameplay_overview
    assert "Campaign overview" in gameplay_overview
    assert metrics["schema"] == "dice-and-destiny-checkpoint-metrics-v1"
    assert metrics["checkpoints"][0]["checkpoint"] == 6
    assert metrics["checkpoints"][0]["gameplay"]["offensive_pre_reaction_ability_rate"] == 0.75
    assert metrics["checkpoints"][0]["gameplay"]["average_actual_outgoing_total_per_round"] == 2.4
    assert artifact_manifest["checkpoint_count"] == 1
    assert artifact_manifest["artifacts"]["gameplay_sheet"] == "gameplay-statistics.html"


def test_completed_reaction_records_final_ability_and_reaction_change() -> None:
    behavior: dict[str, dict[str, int | float]] = {"learner": {}}
    plans = {
        (3, "seat-a"): {
            "round": 3,
            "actor_id": "seat-a",
            "selected_ability": "sword_cut",
            "rolls_used": 2,
        }
    }
    before = {
        "result": {
            "snapshot": {
                "round": 3,
                "segment": "offensive",
                "stage": "offensive_reaction",
                "actors": {"seat-a": {"selected_ability": ""}},
            }
        }
    }
    after = {
        "result": {
            "snapshot": {
                "round": 3,
                "segment": "defensive",
                "stage": "defense_roll",
            }
        }
    }

    _record_completed_offensive_reaction(
        behavior,
        plans,
        before,
        after,
        {"seat-a": "learner"},
    )

    assert behavior["learner"]["offensive_post_reaction_outcomes"] == 1
    assert behavior["learner"]["offensive_post_reaction_passed"] == 1
    assert behavior["learner"]["offensive_reaction_ability_lost"] == 1
    assert not plans
