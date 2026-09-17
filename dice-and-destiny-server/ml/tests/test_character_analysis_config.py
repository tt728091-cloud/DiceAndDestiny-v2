import json
from pathlib import Path


def barbarian_config() -> dict:
    path = Path(__file__).parents[1] / "configs/dice-policy-barbarian.json"
    return json.loads(path.read_text())


def monk_config() -> dict:
    path = Path(__file__).parents[1] / "configs/dice-policy-monk.json"
    return json.loads(path.read_text())


def test_barbarian_config_contains_complete_plug_and_play_metadata() -> None:
    config = barbarian_config()
    analysis = config["character_analysis"]
    target_ids = {target["id"] for target in config["targets"]}
    report_ids = {report["id"] for report in config["balance_reports"]}

    assert config["character"] == {
        "id": "barbarian",
        "name": "Barbarian",
        "source_character": "Barbarian",
    }
    assert analysis["reports"]["primary"] in report_ids
    assert analysis["reports"]["teacher"] in report_ids
    assert analysis["abilities"]
    assert all(ability["target"] in target_ids for ability in analysis["abilities"])
    assert all(
        tier["target"] in target_ids
        for ability in analysis["abilities"]
        for tier in ability.get("tiers", [])
    )
    assert all(item["target"] in target_ids for item in analysis["tier_targets"])


def test_barbarian_adjustments_reference_teacher_report_outcomes() -> None:
    config = barbarian_config()
    analysis = config["character_analysis"]
    teacher_report_id = analysis["reports"]["teacher"]
    teacher_report = next(
        report
        for report in config["balance_reports"]
        if report["id"] == teacher_report_id
    )
    outcome_ids = {outcome["target"] for outcome in teacher_report["outcomes"]}

    assert all(
        adjustment["outcome"] in outcome_ids
        for adjustment in analysis.get("adjustments", [])
    )


def test_monk_config_contains_complete_plug_and_play_metadata() -> None:
    config = monk_config()
    analysis = config["character_analysis"]
    target_ids = {target["id"] for target in config["targets"]}
    report_ids = {report["id"] for report in config["balance_reports"]}

    assert config["character"] == {
        "id": "monk",
        "name": "Monk",
        "source_character": "Monk",
    }
    assert analysis["reports"]["primary"] in report_ids
    assert analysis["reports"]["teacher"] in report_ids
    assert analysis["abilities"]
    assert all(ability["target"] in target_ids for ability in analysis["abilities"])
    assert all(
        tier["target"] in target_ids
        for ability in analysis["abilities"]
        for tier in ability.get("tiers", [])
    )
    assert all(item["target"] in target_ids for item in analysis["tier_targets"])


def test_monk_teacher_report_includes_zero_value_meditate() -> None:
    config = monk_config()
    teacher_report_id = config["character_analysis"]["reports"]["teacher"]
    teacher_report = next(
        report
        for report in config["balance_reports"]
        if report["id"] == teacher_report_id
    )

    meditate = next(
        outcome for outcome in teacher_report["outcomes"]
        if outcome["target"] == "meditate"
    )
    assert meditate["damage"] == 0
