from __future__ import annotations

import argparse
import csv
import json
import sys
from collections import Counter
from fractions import Fraction
from pathlib import Path
from typing import Any


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate exact 5d6 character analysis from a data-only character config."
    )
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--source-csv", type=Path, required=True)
    parser.add_argument("--source-workbook", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args()


def as_fraction(value: Any, field: str) -> Fraction:
    if isinstance(value, bool) or not isinstance(value, (int, float, str)):
        raise ValueError(f"{field} must be a number or numeric string")
    try:
        return Fraction(str(value))
    except (ValueError, ZeroDivisionError) as exc:
        raise ValueError(f"{field} must be a finite numeric value") from exc


def initial_value(solver: Any, rerolls: int, outcome_distribution: Any) -> Fraction:
    return sum(
        probability * solver.value(state, rerolls)
        for state, probability in outcome_distribution(5)
    )


def initial_tier_distribution(
    solver: Any, rerolls: int, outcome_distribution: Any
) -> tuple[Fraction, ...]:
    result = [Fraction(0) for _ in range(len(solver.objective.targets) + 1)]
    for state, state_probability in outcome_distribution(5):
        for index, outcome_probability in enumerate(
            solver.tier_distribution(state, rerolls)
        ):
            result[index] += state_probability * outcome_probability
    return tuple(result)


def difficulty(probability: float) -> str:
    if probability >= 0.85:
        return "Very reliable"
    if probability >= 0.65:
        return "Reliable"
    if probability >= 0.45:
        return "Moderate"
    if probability >= 0.25:
        return "Difficult"
    if probability >= 0.10:
        return "Long shot"
    return "Extreme long shot"


def generate(args: argparse.Namespace) -> dict[str, Any]:
    repo = args.repo.resolve()
    ml_root = repo / "dice-and-destiny-server/ml"
    sys.path.insert(0, str(ml_root))

    from dice_destiny_ml.dice_policy_table import (  # noqa: PLC0415
        MAX_REROLLS,
        ConstrainedPolicySolver,
        Objective,
        PolicySolver,
        all_states,
        face_counts,
        outcome_distribution,
        parse_balance_reports,
        parse_target_config,
        unique_keeps,
    )

    config_path = args.config.resolve()
    config = json.loads(config_path.read_text())
    character = config.get("character")
    analysis_config = config.get("character_analysis")
    if not isinstance(character, dict):
        raise ValueError("config must define a 'character' object")
    if not isinstance(analysis_config, dict):
        raise ValueError("config must define a 'character_analysis' object")
    character_name = character.get("name")
    character_id = character.get("id")
    source_character = character.get("source_character", character_name)
    if not isinstance(character_name, str) or not character_name.strip():
        raise ValueError("character.name must be a non-empty string")
    if not isinstance(character_id, str) or not character_id.strip():
        raise ValueError("character.id must be a non-empty string")

    targets, objectives = parse_target_config(config)
    targets_by_id = {target.id: target for target in targets}
    objectives_by_id = {objective.id: objective for objective in objectives}
    solvers = {
        objective.id: PolicySolver(objective)
        for objective in objectives
        if len(objective.targets) == 1 and not objective.ranked
    }
    balance_reports = {
        report.id: report
        for report in parse_balance_reports(config, targets, objectives)
    }
    report_config_by_id = {
        report["id"]: report for report in config.get("balance_reports", [])
    }

    report_ids = analysis_config.get("reports", {})
    primary_report_id = report_ids.get("primary")
    teacher_report_id = report_ids.get("teacher")
    if primary_report_id not in balance_reports:
        raise ValueError("character_analysis.reports.primary must name a balance report")
    if teacher_report_id not in balance_reports:
        raise ValueError("character_analysis.reports.teacher must name a balance report")
    primary_report = balance_reports[primary_report_id]
    teacher_report = balance_reports[teacher_report_id]

    primary_teacher_objective = PolicySolver(primary_report.teacher_objective)
    primary_teacher_value = ConstrainedPolicySolver(
        primary_report.objective, primary_teacher_objective
    )
    primary_value_first = PolicySolver(primary_report.objective)
    teacher_objective = PolicySolver(teacher_report.teacher_objective)
    teacher_value = ConstrainedPolicySolver(
        teacher_report.objective, teacher_objective
    )
    value_first = PolicySolver(teacher_report.objective)

    source_rows: list[dict[str, Any]] = []
    with args.source_csv.resolve().open(newline="", encoding="utf-8-sig") as handle:
        for row_number, row in enumerate(csv.DictReader(handle), start=2):
            if row.get("character") == source_character:
                source_rows.append({"csv_row": row_number, **row})
    source_by_name = {row["name"]: row for row in source_rows}

    symbol_labels = analysis_config.get("symbol_labels", {})
    symbol_order = list(config.get("symbols", {}).keys())
    face_symbols: dict[int, str] = {}
    for symbol, faces in config.get("symbols", {}).items():
        label = symbol_labels.get(symbol, symbol.replace("_", " ").title())
        for face in faces:
            if face in face_symbols:
                raise ValueError(f"die face {face} is assigned to more than one symbol")
            face_symbols[face] = label
    display_symbol_order = [
        symbol_labels.get(symbol, symbol.replace("_", " ").title())
        for symbol in symbol_order
    ]

    outcome_labels = {"nothing": analysis_config.get("nothing_label", "Nothing")}
    outcome_notes = {"nothing": analysis_config.get("nothing_note", "No configured ability completed")}
    for report_id, raw_report in report_config_by_id.items():
        for raw_outcome in raw_report.get("outcomes", []):
            target_id = raw_outcome["target"]
            outcome_labels.setdefault(
                target_id,
                raw_outcome.get("label", targets_by_id[target_id].name),
            )
            outcome_notes.setdefault(target_id, raw_outcome.get("note", ""))
    outcome_labels.update(analysis_config.get("outcome_labels", {}))
    outcome_notes.update(analysis_config.get("outcome_notes", {}))

    teacher_outcome_ids = (
        "nothing",
        *(target.id for target in teacher_report.objective.targets),
    )

    def subtract_roll(
        state: tuple[int, ...], keep: tuple[int, ...]
    ) -> tuple[int, ...]:
        remaining = list(state)
        for die in keep:
            remaining.remove(die)
        return tuple(remaining)

    def format_roll(roll: tuple[int, ...]) -> str:
        return ", ".join(str(face) for face in roll) if roll else "—"

    def format_symbol_counts(roll: tuple[int, ...]) -> str:
        counts = Counter(face_symbols.get(face, str(face)) for face in roll)
        ordered = [*display_symbol_order]
        ordered.extend(symbol for symbol in counts if symbol not in ordered)
        return " + ".join(
            f"{symbol} ×{counts[symbol]}" for symbol in ordered if counts[symbol]
        ) or "—"

    def state_rows(solver: Any, rerolls_remaining: int, reliability: bool) -> list[dict]:
        rows: list[dict] = []
        for state in all_states():
            keeps = solver.best_keeps(state, rerolls_remaining)
            primary_keep = keeps[0]
            reroll = subtract_roll(state, primary_keep)
            distribution = solver.tier_distribution(state, rerolls_remaining)
            indexed_routes = sorted(
                enumerate(zip(teacher_outcome_ids, distribution, strict=True)),
                key=lambda item: (item[1][1], item[0]),
                reverse=True,
            )
            routes = [route for _, route in indexed_routes if route[1] > 0]
            most_likely_id, most_likely_probability = routes[0]
            current_tier = teacher_report.objective.terminal_tier_index(
                face_counts(state)
            )
            current_id = teacher_outcome_ids[current_tier]
            alternatives = keeps[1:]
            if reroll:
                suffix = (
                    "Re-evaluate after the new dice arrive."
                    if reliability
                    else "Re-evaluate for maximum listed value after the new dice arrive."
                )
                action = (
                    f"Keep {format_roll(primary_keep)}; reroll {format_roll(reroll)}. "
                    f"{suffix}"
                )
            else:
                action = (
                    "Keep all five dice; the selected result is already optimal."
                    if reliability
                    else "Keep all five dice; the current result already maximizes listed value."
                )
            rows.append(
                {
                    "state_key": "-".join(str(face) for face in state),
                    "rolled_dice": format_roll(state),
                    "face_counts": ",".join(str(count) for count in face_counts(state)),
                    "symbols": format_symbol_counts(state),
                    "current_best_ability": outcome_labels[current_id],
                    "primary_keep": format_roll(primary_keep),
                    "keep_symbols": format_symbol_counts(primary_keep),
                    "reroll": format_roll(reroll),
                    "alternate_optimal_keeps": "; ".join(
                        format_roll(keep) for keep in alternatives
                    )
                    if alternatives
                    else "—",
                    "success_probability": float(
                        teacher_objective.value(state, rerolls_remaining)
                        if reliability
                        else 1 - distribution[0]
                    ),
                    "expected_listed_value": float(
                        solver.value(state, rerolls_remaining)
                    ),
                    "most_likely_final_ability": outcome_labels[most_likely_id],
                    "most_likely_probability": float(most_likely_probability),
                    "top_routes": " | ".join(
                        f"{outcome_labels[label]} {float(probability):.2%}"
                        for label, probability in routes[:3]
                    ),
                    "action": action,
                }
            )
        return rows

    def all_action_rows(rerolls_remaining: int) -> list[dict]:
        rows: list[dict] = []
        for state in all_states():
            success_actions = dict(
                teacher_objective.action_values(state, rerolls_remaining)
            )
            value_actions = dict(value_first.action_values(state, rerolls_remaining))
            reliability_selected = set(
                teacher_value.best_keeps(state, rerolls_remaining)
            )
            value_selected = set(value_first.best_keeps(state, rerolls_remaining))
            best_success = teacher_objective.value(state, rerolls_remaining)
            selected_reliability_value = teacher_value.value(
                state, rerolls_remaining
            )
            best_value = value_first.value(state, rerolls_remaining)
            reliability_values = {
                keep: sum(
                    probability
                    * teacher_value.value(
                        tuple(sorted((*keep, *outcome))), rerolls_remaining - 1
                    )
                    for outcome, probability in outcome_distribution(5 - len(keep))
                )
                for keep in unique_keeps(state)
            }
            for keep in unique_keeps(state):
                reroll = subtract_roll(state, keep)
                success = success_actions[keep]
                reliability_value = reliability_values[keep]
                value_first_value = value_actions[keep]
                is_reliability_selected = keep in reliability_selected
                is_value_selected = keep in value_selected
                if is_reliability_selected and is_value_selected:
                    note = "Selected by both policies"
                elif is_reliability_selected:
                    note = "Reliability-first selection"
                elif is_value_selected:
                    note = "Value-first selection"
                elif success == best_success:
                    note = "Maximum hit chance; loses reliability tie-break"
                else:
                    note = "Alternative action"
                rows.append(
                    {
                        "state_key": "-".join(str(face) for face in state),
                        "rolled_dice": format_roll(state),
                        "symbols": format_symbol_counts(state),
                        "keep": format_roll(keep),
                        "keep_symbols": format_symbol_counts(keep),
                        "reroll": format_roll(reroll),
                        "reroll_count": len(reroll),
                        "hit_probability": float(success),
                        "hit_probability_loss": float(best_success - success),
                        "reliability_continuation_value": float(reliability_value),
                        "value_change_vs_reliability_selection": float(
                            reliability_value - selected_reliability_value
                        ),
                        "reliability_selected": is_reliability_selected,
                        "value_first_continuation_value": float(value_first_value),
                        "value_loss_from_best": float(best_value - value_first_value),
                        "value_first_selected": is_value_selected,
                        "note": note,
                    }
                )
        return rows

    def state_distributions_for_policy(
        solver: Any,
    ) -> tuple[dict[tuple[int, ...], Fraction], ...]:
        opening = dict(outcome_distribution(5))

        def advance(
            current: dict[tuple[int, ...], Fraction], rerolls_remaining: int
        ) -> dict[tuple[int, ...], Fraction]:
            following = {state: Fraction(0) for state in all_states()}
            for state, state_probability in current.items():
                keep = solver.best_keeps(state, rerolls_remaining)[0]
                for outcome, probability in outcome_distribution(5 - len(keep)):
                    next_state = tuple(sorted((*keep, *outcome)))
                    following[next_state] += state_probability * probability
            if sum(following.values()) != 1:
                raise AssertionError("state probabilities do not sum to one")
            return following

        after_first = advance(opening, 2)
        after_second = advance(after_first, 1)
        return opening, after_first, after_second

    def state_frequency_rows() -> list[dict]:
        opening, reliability_first, reliability_second = (
            state_distributions_for_policy(teacher_value)
        )
        _, value_first_roll, value_second_roll = state_distributions_for_policy(
            value_first
        )
        return [
            {
                "state_key": "-".join(str(face) for face in state),
                "rolled_dice": format_roll(state),
                "face_counts": ",".join(str(count) for count in face_counts(state)),
                "symbols": format_symbol_counts(state),
                "opening_ordered_combinations": int(opening[state] * (6**5)),
                "opening_probability": float(opening[state]),
                "reliability_after_first_probability": float(reliability_first[state]),
                "reliability_after_second_probability": float(reliability_second[state]),
                "value_after_first_probability": float(value_first_roll[state]),
                "value_after_second_probability": float(value_second_roll[state]),
            }
            for state in all_states()
        ]

    def report_labels(report_id: str) -> tuple[dict[str, str], dict[str, str]]:
        raw_report = report_config_by_id[report_id]
        labels = {"nothing": outcome_labels["nothing"]}
        notes = {"nothing": outcome_notes["nothing"]}
        for raw_outcome in raw_report["outcomes"]:
            target_id = raw_outcome["target"]
            labels[target_id] = outcome_labels[target_id]
            notes[target_id] = outcome_notes.get(target_id, "")
        return labels, notes

    def overall_breakdown(solver: Any, report_id: str, rerolls: int) -> dict:
        distribution = initial_tier_distribution(
            solver, rerolls, outcome_distribution
        )
        labels = ("nothing", *(target.id for target in solver.objective.targets))
        values = (Fraction(0), *solver.objective.rewards)
        names, notes = report_labels(report_id)
        outcomes = [
            {
                "outcome": label,
                "label": names[label],
                "note": notes.get(label, ""),
                "probability": float(probability),
                "value": float(value),
                "average_value_contribution": float(probability * value),
                # Compatibility aliases for the existing workbook builder.
                "damage": float(value),
                "average_damage_contribution": float(probability * value),
            }
            for label, value, probability in zip(
                labels, values, distribution, strict=True
            )
        ]
        return {
            "hit_probability": float(1 - distribution[0]),
            "average_value": float(initial_value(solver, rerolls, outcome_distribution)),
            "average_damage": float(initial_value(solver, rerolls, outcome_distribution)),
            "probability_total": float(sum(distribution)),
            "outcomes": outcomes,
        }

    abilities_config = analysis_config.get("abilities")
    if not isinstance(abilities_config, list) or not abilities_config:
        raise ValueError("character_analysis.abilities must be a non-empty list")
    ability_rows: list[dict[str, Any]] = []
    for ability in abilities_config:
        target_id = ability.get("target")
        if target_id not in solvers:
            raise ValueError(f"ability target {target_id!r} is not an individual target")
        solver = solvers[target_id]
        probabilities = [
            initial_value(solver, rerolls, outcome_distribution)
            for rerolls in range(MAX_REROLLS + 1)
        ]
        tiers = ability.get("tiers")
        if tiers:
            tier_targets = tuple(targets_by_id[tier["target"]] for tier in tiers)
            rewards = tuple(
                as_fraction(tier["value"], f"{ability['name']} tier value")
                for tier in tiers
            )
            ranked = PolicySolver(
                Objective(
                    f"{character_id}_{target_id}_listed_value",
                    f"{ability['name']} listed value",
                    tier_targets,
                    rewards,
                )
            )
            expected_value = ranked.before_first_roll()
        else:
            fixed_value = as_fraction(
                ability.get("fixed_value", 0), f"{ability['name']} fixed_value"
            )
            expected_value = probabilities[MAX_REROLLS] * fixed_value
        source_name = ability.get("source", ability["name"])
        source = source_by_name.get(source_name)
        ability_rows.append(
            {
                "ability": ability["name"],
                "target": target_id,
                "requirement": ability.get("requirement", targets_by_id[target_id].name),
                "effect": ability.get("effect", source["rules_text"] if source else ""),
                "source": source_name,
                "category": ability.get("category", "Ability"),
                "value_kind": ability.get("value_kind", "listed value"),
                "recommendation": ability.get("recommendation", ""),
                "first_roll_probability": float(probabilities[0]),
                "one_reroll_probability": float(probabilities[1]),
                "two_reroll_probability": float(probabilities[2]),
                "difficulty": difficulty(float(probabilities[2])),
                "expected_listed_value": float(expected_value),
                "source_csv_row": source["csv_row"] if source else None,
                "source_rules_text": source["rules_text"] if source else "",
            }
        )

    tier_rows = []
    for item in analysis_config.get("tier_targets", []):
        target_id = item["target"]
        solver = solvers[target_id]
        tier_rows.append(
            {
                "target": target_id,
                "name": item.get("name", targets_by_id[target_id].name),
                "note": item.get("note", ""),
                "first_roll_probability": float(
                    initial_value(solver, 0, outcome_distribution)
                ),
                "one_reroll_probability": float(
                    initial_value(solver, 1, outcome_distribution)
                ),
                "two_reroll_probability": float(
                    initial_value(solver, 2, outcome_distribution)
                ),
            }
        )

    payload = {
        "schema": "dice-throne-character-analysis-v1",
        "character": {
            **character,
            "value_label": analysis_config.get("value_label", "listed value"),
            "value_unit": analysis_config.get("value_unit", "value"),
        },
        "presentation": {
            "analysis_note": analysis_config.get("analysis_note", ""),
            "overall_note": analysis_config.get("overall_note", ""),
            "reroll_note": analysis_config.get("reroll_note", ""),
            "teacher_policy_description": analysis_config.get(
                "teacher_policy_description", ""
            ),
            "value_policy_description": analysis_config.get(
                "value_policy_description", ""
            ),
            "adjustments": analysis_config.get("adjustments", []),
        },
        "source_csv": str(args.source_csv.resolve()),
        "source_workbook": str(args.source_workbook.resolve()),
        "config": str(config_path),
        "source_rows": source_rows,
        "abilities": ability_rows,
        "tier_probabilities": tier_rows,
        "overall_policies": {
            "report_id": primary_report_id,
            "teacher_success_policy": {
                "primary_success_probability": float(
                    primary_teacher_objective.before_first_roll()
                ),
                **overall_breakdown(
                    primary_teacher_value, primary_report_id, MAX_REROLLS
                ),
            },
            "value_optimized_policy": overall_breakdown(
                primary_value_first, primary_report_id, MAX_REROLLS
            ),
            "damage_optimized_policy": overall_breakdown(
                primary_value_first, primary_report_id, MAX_REROLLS
            ),
        },
        "overall_by_rerolls": {
            "report_id": teacher_report_id,
            "teacher_success_policy": {
                str(rerolls): {
                    "primary_success_probability": float(
                        initial_value(
                            teacher_objective, rerolls, outcome_distribution
                        )
                    ),
                    **overall_breakdown(
                        teacher_value, teacher_report_id, rerolls
                    ),
                }
                for rerolls in range(MAX_REROLLS + 1)
            },
            "value_optimized_policy": {
                str(rerolls): overall_breakdown(
                    value_first, teacher_report_id, rerolls
                )
                for rerolls in range(MAX_REROLLS + 1)
            },
            "damage_optimized_policy": {
                str(rerolls): overall_breakdown(
                    value_first, teacher_report_id, rerolls
                )
                for rerolls in range(MAX_REROLLS + 1)
            },
        },
        "teacher_state_guides": {
            "after_first_roll": {
                "rerolls_remaining": 2,
                "rows": state_rows(teacher_value, 2, True),
            },
            "after_second_roll": {
                "rerolls_remaining": 1,
                "rows": state_rows(teacher_value, 1, True),
            },
        },
        "value_first_state_guides": {
            "after_first_roll": {
                "rerolls_remaining": 2,
                "rows": state_rows(value_first, 2, False),
            },
            "after_second_roll": {
                "rerolls_remaining": 1,
                "rows": state_rows(value_first, 1, False),
            },
        },
        "state_frequencies": {"rows": state_frequency_rows()},
        "all_action_guides": {
            "two_rerolls": {
                "rerolls_remaining": 2,
                "rows": all_action_rows(2),
            },
            "one_reroll": {
                "rerolls_remaining": 1,
                "rows": all_action_rows(1),
            },
        },
        "objective_ids": sorted(objectives_by_id),
    }
    return payload


def main() -> None:
    args = parse_args()
    payload = generate(args)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2) + "\n")
    print(args.output.resolve())
    for row in sorted(
        payload["abilities"],
        key=lambda item: item["two_reroll_probability"],
        reverse=True,
    ):
        print(
            f"{row['ability']}: {row['two_reroll_probability']:.6%}; "
            f"expected listed {row['expected_listed_value']:.4f}"
        )


if __name__ == "__main__":
    main()
