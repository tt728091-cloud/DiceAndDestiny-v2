"""Generate exact 5d6 reroll policy tables from data-defined targets.

The solver uses sorted rolls as states and backward induction for zero, one,
and two rerolls remaining. It intentionally uses only the Python standard
library so policy tables can be regenerated without the ML runtime.
"""

from __future__ import annotations

import argparse
import json
import re
from collections import Counter
from dataclasses import dataclass
from fractions import Fraction
from functools import cache
from itertools import combinations_with_replacement
from math import factorial
from pathlib import Path
from typing import Any

DICE_COUNT = 5
SIDES = 6
MAX_REROLLS = 2
SCHEMA = "dice-and-destiny-reroll-policy-v1"
BALANCE_SCHEMA = "dice-and-destiny-balance-report-v2"

Roll = tuple[int, ...]
FaceCounts = tuple[int, ...]
_ID_PATTERN = re.compile(r"^[a-z][a-z0-9_]*$")


@dataclass(frozen=True)
class SymbolRequirement:
    symbol: str
    faces: Roll
    at_least: int

    def completed(self, counts: FaceCounts) -> bool:
        return sum(counts[face - 1] for face in self.faces) >= self.at_least


@dataclass(frozen=True)
class Target:
    id: str
    name: str
    patterns: tuple[FaceCounts, ...] = ()
    symbol: str | None = None
    symbol_faces: Roll = ()
    at_least: int = 0
    requirements: tuple[SymbolRequirement, ...] = ()

    def completed(self, counts: FaceCounts) -> bool:
        if self.patterns:
            return any(
                all(actual >= required for actual, required in zip(counts, pattern, strict=True))
                for pattern in self.patterns
            )
        if self.requirements:
            return all(requirement.completed(counts) for requirement in self.requirements)
        return sum(counts[face - 1] for face in self.symbol_faces) >= self.at_least


@dataclass(frozen=True)
class Objective:
    id: str
    name: str
    targets: tuple[Target, ...]
    rewards: tuple[Fraction, ...] = ()

    def completed(self, counts: FaceCounts) -> bool:
        return any(target.completed(counts) for target in self.targets)

    @property
    def ranked(self) -> bool:
        return bool(self.rewards)

    def terminal_value(self, counts: FaceCounts) -> Fraction:
        if not self.ranked:
            return Fraction(int(self.completed(counts)))
        return max(
            (
                reward
                for target, reward in zip(self.targets, self.rewards, strict=True)
                if target.completed(counts)
            ),
            default=Fraction(0),
        )

    def terminal_tier_index(self, counts: FaceCounts) -> int:
        """Return zero for no tier, otherwise the one-based highest completed tier."""
        completed = [
            index
            for index, target in enumerate(self.targets, start=1)
            if target.completed(counts)
        ]
        return max(completed, default=0)


@dataclass(frozen=True)
class BalanceReport:
    id: str
    name: str
    objective: Objective
    teacher_objective: Objective


def face_counts(values: Roll) -> FaceCounts:
    counts = Counter(values)
    return tuple(counts[face] for face in range(1, SIDES + 1))


@cache
def all_states() -> tuple[Roll, ...]:
    return tuple(combinations_with_replacement(range(1, SIDES + 1), DICE_COUNT))


@cache
def unique_keeps(state: Roll) -> tuple[Roll, ...]:
    counts = Counter(state)
    faces = tuple(sorted(counts))
    keeps: list[Roll] = []

    def visit(index: int, current: list[int]) -> None:
        if index == len(faces):
            keeps.append(tuple(current))
            return
        face = faces[index]
        for amount in range(counts[face] + 1):
            visit(index + 1, [*current, *([face] * amount)])

    visit(0, [])
    return tuple(keeps)


@cache
def outcome_distribution(dice_count: int) -> tuple[tuple[Roll, Fraction], ...]:
    if not 0 <= dice_count <= DICE_COUNT:
        raise ValueError(f"dice_count must be between 0 and {DICE_COUNT}")
    if dice_count == 0:
        return (((), Fraction(1)),)

    outcomes: list[tuple[Roll, Fraction]] = []
    for outcome in combinations_with_replacement(range(1, SIDES + 1), dice_count):
        permutations = factorial(dice_count)
        for amount in Counter(outcome).values():
            permutations //= factorial(amount)
        outcomes.append((outcome, Fraction(permutations, SIDES**dice_count)))

    if sum(probability for _, probability in outcomes) != 1:
        raise AssertionError("outcome probabilities do not sum to one")
    return tuple(outcomes)


class PolicySolver:
    """Exact value and action-value solver for one objective."""

    def __init__(self, objective: Objective) -> None:
        self.objective = objective
        self._values: dict[tuple[Roll, int], Fraction] = {}
        self._action_values: dict[tuple[Roll, int], tuple[tuple[Roll, Fraction], ...]] = {}
        self._tier_distributions: dict[tuple[Roll, int], tuple[Fraction, ...]] = {}
        self._action_tier_distributions: dict[
            tuple[Roll, int, Roll], tuple[Fraction, ...]
        ] = {}

    def value(self, state: Roll, rerolls_remaining: int) -> Fraction:
        self._validate_state(state, rerolls_remaining)
        key = (state, rerolls_remaining)
        cached = self._values.get(key)
        if cached is not None:
            return cached
        if rerolls_remaining == 0:
            result = self.objective.terminal_value(face_counts(state))
        else:
            result = max(value for _, value in self.action_values(state, rerolls_remaining))
        self._values[key] = result
        return result

    def action_values(
        self, state: Roll, rerolls_remaining: int
    ) -> tuple[tuple[Roll, Fraction], ...]:
        self._validate_state(state, rerolls_remaining)
        key = (state, rerolls_remaining)
        cached = self._action_values.get(key)
        if cached is not None:
            return cached
        if rerolls_remaining == 0:
            return ()

        values: list[tuple[Roll, Fraction]] = []
        for keep in unique_keeps(state):
            rerolled_dice = DICE_COUNT - len(keep)
            eventual = sum(
                probability
                * self.value(tuple(sorted((*keep, *outcome))), rerolls_remaining - 1)
                for outcome, probability in outcome_distribution(rerolled_dice)
            )
            values.append((keep, eventual))
        result = tuple(values)
        self._action_values[key] = result
        return result

    def best_keeps(self, state: Roll, rerolls_remaining: int) -> tuple[Roll, ...]:
        if rerolls_remaining == 0:
            return ()
        best = self.value(state, rerolls_remaining)
        candidates = tuple(
            keep
            for keep, action_value in self.action_values(state, rerolls_remaining)
            if action_value == best
        )
        if not self.objective.ranked or len(candidates) < 2:
            return candidates

        distributions = {
            keep: self.action_tier_distribution(state, rerolls_remaining, keep)
            for keep in candidates
        }
        best_distribution = max(
            tuple(reversed(distribution[1:])) for distribution in distributions.values()
        )
        return tuple(
            keep
            for keep in candidates
            if tuple(reversed(distributions[keep][1:])) == best_distribution
        )

    def before_first_roll(self) -> Fraction:
        return sum(
            probability * self.value(state, MAX_REROLLS)
            for state, probability in outcome_distribution(DICE_COUNT)
        )

    def tier_distribution(self, state: Roll, rerolls_remaining: int) -> tuple[Fraction, ...]:
        if not self.objective.ranked:
            raise ValueError("tier distributions require a ranked objective")
        self._validate_state(state, rerolls_remaining)
        key = (state, rerolls_remaining)
        cached = self._tier_distributions.get(key)
        if cached is not None:
            return cached

        if rerolls_remaining == 0:
            tier_index = self.objective.terminal_tier_index(face_counts(state))
            result = tuple(
                Fraction(int(index == tier_index))
                for index in range(len(self.objective.targets) + 1)
            )
        else:
            keep = self.best_keeps(state, rerolls_remaining)[0]
            result = self.action_tier_distribution(state, rerolls_remaining, keep)
        self._tier_distributions[key] = result
        return result

    def action_tier_distribution(
        self, state: Roll, rerolls_remaining: int, keep: Roll
    ) -> tuple[Fraction, ...]:
        if not self.objective.ranked:
            raise ValueError("tier distributions require a ranked objective")
        self._validate_state(state, rerolls_remaining)
        if rerolls_remaining == 0:
            raise ValueError("no actions exist when zero rerolls remain")
        if keep not in unique_keeps(state):
            raise ValueError("keep must be a submultiset of state")
        key = (state, rerolls_remaining, keep)
        cached = self._action_tier_distributions.get(key)
        if cached is not None:
            return cached

        result = [Fraction(0) for _ in range(len(self.objective.targets) + 1)]
        for outcome, probability in outcome_distribution(DICE_COUNT - len(keep)):
            next_state = tuple(sorted((*keep, *outcome)))
            for index, tier_probability in enumerate(
                self.tier_distribution(next_state, rerolls_remaining - 1)
            ):
                result[index] += probability * tier_probability
        serialized = tuple(result)
        self._action_tier_distributions[key] = serialized
        return serialized

    def before_first_tier_distribution(self) -> tuple[Fraction, ...]:
        if not self.objective.ranked:
            raise ValueError("tier distributions require a ranked objective")
        result = [Fraction(0) for _ in range(len(self.objective.targets) + 1)]
        for state, probability in outcome_distribution(DICE_COUNT):
            for index, tier_probability in enumerate(self.tier_distribution(state, MAX_REROLLS)):
                result[index] += probability * tier_probability
        return tuple(result)

    @staticmethod
    def _validate_state(state: Roll, rerolls_remaining: int) -> None:
        if len(state) != DICE_COUNT or tuple(sorted(state)) != state:
            raise ValueError(f"state must be a sorted {DICE_COUNT}d{SIDES} roll")
        if any(face < 1 or face > SIDES for face in state):
            raise ValueError(f"state faces must be between 1 and {SIDES}")
        if not 0 <= rerolls_remaining <= MAX_REROLLS:
            raise ValueError(f"rerolls_remaining must be between 0 and {MAX_REROLLS}")


class ConstrainedPolicySolver(PolicySolver):
    """Optimize one value without leaving another solver's optimal action set."""

    def __init__(self, objective: Objective, constraint: PolicySolver) -> None:
        super().__init__(objective)
        self.constraint = constraint

    def action_values(
        self, state: Roll, rerolls_remaining: int
    ) -> tuple[tuple[Roll, Fraction], ...]:
        self._validate_state(state, rerolls_remaining)
        key = (state, rerolls_remaining)
        cached = self._action_values.get(key)
        if cached is not None:
            return cached
        if rerolls_remaining == 0:
            return ()

        values: list[tuple[Roll, Fraction]] = []
        for keep in self.constraint.best_keeps(state, rerolls_remaining):
            rerolled_dice = DICE_COUNT - len(keep)
            eventual = sum(
                probability
                * self.value(tuple(sorted((*keep, *outcome))), rerolls_remaining - 1)
                for outcome, probability in outcome_distribution(rerolled_dice)
            )
            values.append((keep, eventual))
        result = tuple(values)
        self._action_values[key] = result
        return result


def parse_target_config(data: dict[str, Any]) -> tuple[tuple[Target, ...], tuple[Objective, ...]]:
    symbols = _parse_symbols(data.get("symbols", {}))
    raw_targets = data.get("targets")
    if not isinstance(raw_targets, list) or not raw_targets:
        raise ValueError("target config requires a non-empty 'targets' list")

    targets: list[Target] = []
    seen_ids: set[str] = set()
    for raw_target in raw_targets:
        if not isinstance(raw_target, dict):
            raise ValueError("each target must be an object")
        target_id = _validated_id(raw_target.get("id"), "target")
        if target_id in seen_ids:
            raise ValueError(f"duplicate target id: {target_id}")
        seen_ids.add(target_id)
        name = raw_target.get("name", target_id.replace("_", " ").title())
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"target {target_id!r} requires a non-empty name")
        has_patterns = "patterns" in raw_target
        has_symbol = "symbol" in raw_target or "at_least" in raw_target
        has_requirements = "requirements" in raw_target
        if sum((has_patterns, has_symbol, has_requirements)) != 1:
            raise ValueError(
                f"target {target_id!r} must define exactly one of 'patterns', "
                "'symbol' with 'at_least', or 'requirements'"
            )
        if has_patterns:
            patterns = _parse_patterns(target_id, raw_target.get("patterns"))
            targets.append(Target(target_id, name.strip(), patterns=patterns))
        elif has_symbol:
            symbol = raw_target.get("symbol")
            if not isinstance(symbol, str) or symbol not in symbols:
                raise ValueError(f"target {target_id!r} references unknown symbol {symbol!r}")
            at_least = raw_target.get("at_least")
            if not isinstance(at_least, int) or isinstance(at_least, bool):
                raise ValueError(f"target {target_id!r} requires an integer 'at_least'")
            if not 1 <= at_least <= DICE_COUNT:
                raise ValueError(f"target {target_id!r} 'at_least' must be 1 to {DICE_COUNT}")
            targets.append(
                Target(
                    target_id,
                    name.strip(),
                    symbol=symbol,
                    symbol_faces=symbols[symbol],
                    at_least=at_least,
                )
            )
        else:
            raw_requirements = raw_target.get("requirements")
            if not isinstance(raw_requirements, list) or not raw_requirements:
                raise ValueError(f"target {target_id!r} requires a non-empty requirements list")
            requirements = tuple(
                _parse_symbol_requirement(target_id, raw_requirement, symbols)
                for raw_requirement in raw_requirements
            )
            if len({requirement.symbol for requirement in requirements}) != len(requirements):
                raise ValueError(f"target {target_id!r} repeats a symbol requirement")
            if sum(requirement.at_least for requirement in requirements) > DICE_COUNT:
                raise ValueError(
                    f"target {target_id!r} requirements need more than {DICE_COUNT} dice"
                )
            targets.append(
                Target(target_id, name.strip(), requirements=requirements)
            )

    objectives = [
        Objective(target.id, f"Maximize probability of {target.name}", (target,))
        for target in targets
    ]
    objective_ids = {objective.id for objective in objectives}

    if data.get("include_any_of_all", True) and len(targets) > 1:
        combined_id = "any_of_all"
        if combined_id in objective_ids:
            raise ValueError(f"target id {combined_id!r} conflicts with generated combined objective")
        target_names = ", ".join(target.name for target in targets)
        objectives.append(
            Objective(combined_id, f"Maximize probability of any target: {target_names}", tuple(targets))
        )
        objective_ids.add(combined_id)

    targets_by_id = {target.id: target for target in targets}
    raw_combined = data.get("combined_objectives", [])
    if not isinstance(raw_combined, list):
        raise ValueError("'combined_objectives' must be a list when present")
    for raw_objective in raw_combined:
        if not isinstance(raw_objective, dict):
            raise ValueError("each combined objective must be an object")
        objective_id = _validated_id(raw_objective.get("id"), "combined objective")
        if objective_id in objective_ids:
            raise ValueError(f"duplicate objective id: {objective_id}")
        target_ids = raw_objective.get("targets")
        if not isinstance(target_ids, list) or not target_ids:
            raise ValueError(f"combined objective {objective_id!r} requires target ids")
        if len(set(target_ids)) != len(target_ids):
            raise ValueError(f"combined objective {objective_id!r} repeats a target id")
        unknown = [target_id for target_id in target_ids if target_id not in targets_by_id]
        if unknown:
            raise ValueError(f"combined objective {objective_id!r} has unknown targets: {unknown}")
        name = raw_objective.get("name", objective_id.replace("_", " ").title())
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"combined objective {objective_id!r} requires a non-empty name")
        objectives.append(
            Objective(
                objective_id,
                name.strip(),
                tuple(targets_by_id[target_id] for target_id in target_ids),
            )
        )
        objective_ids.add(objective_id)

    raw_ranked = data.get("ranked_objectives", [])
    if not isinstance(raw_ranked, list):
        raise ValueError("'ranked_objectives' must be a list when present")
    for raw_objective in raw_ranked:
        if not isinstance(raw_objective, dict):
            raise ValueError("each ranked objective must be an object")
        objective_id = _validated_id(raw_objective.get("id"), "ranked objective")
        if objective_id in objective_ids:
            raise ValueError(f"duplicate objective id: {objective_id}")
        raw_tiers = raw_objective.get("tiers")
        if not isinstance(raw_tiers, list) or not raw_tiers:
            raise ValueError(f"ranked objective {objective_id!r} requires a non-empty tiers list")

        tier_targets: list[Target] = []
        rewards: list[Fraction] = []
        for index, raw_tier in enumerate(raw_tiers, start=1):
            if not isinstance(raw_tier, dict):
                raise ValueError(f"ranked objective {objective_id!r} tiers must be objects")
            target_id = raw_tier.get("target")
            if target_id not in targets_by_id:
                raise ValueError(
                    f"ranked objective {objective_id!r} references unknown target {target_id!r}"
                )
            reward = _parse_reward(raw_tier.get("reward", index), objective_id)
            tier_targets.append(targets_by_id[target_id])
            rewards.append(reward)
        if len({target.id for target in tier_targets}) != len(tier_targets):
            raise ValueError(f"ranked objective {objective_id!r} repeats a target")
        if any(
            later <= earlier for earlier, later in zip(rewards, rewards[1:], strict=False)
        ):
            raise ValueError(f"ranked objective {objective_id!r} rewards must strictly increase")

        name = raw_objective.get("name", objective_id.replace("_", " ").title())
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"ranked objective {objective_id!r} requires a non-empty name")
        objectives.append(
            Objective(
                objective_id,
                name.strip(),
                tuple(tier_targets),
                tuple(rewards),
            )
        )
        objective_ids.add(objective_id)

    return tuple(targets), tuple(objectives)


def parse_balance_reports(
    data: dict[str, Any], targets: tuple[Target, ...], objectives: tuple[Objective, ...]
) -> tuple[BalanceReport, ...]:
    raw_reports = data.get("balance_reports", [])
    if not isinstance(raw_reports, list):
        raise ValueError("'balance_reports' must be a list when present")

    targets_by_id = {target.id: target for target in targets}
    objectives_by_id = {objective.id: objective for objective in objectives}
    reports: list[BalanceReport] = []
    seen_ids: set[str] = set()
    for raw_report in raw_reports:
        if not isinstance(raw_report, dict):
            raise ValueError("each balance report must be an object")
        report_id = _validated_id(raw_report.get("id"), "balance report")
        if report_id in seen_ids:
            raise ValueError(f"duplicate balance report id: {report_id}")
        seen_ids.add(report_id)
        name = raw_report.get("name", report_id.replace("_", " ").title())
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"balance report {report_id!r} requires a non-empty name")
        teacher_objective_id = raw_report.get("teacher_objective")
        if teacher_objective_id not in objectives_by_id:
            raise ValueError(
                f"balance report {report_id!r} references unknown teacher objective "
                f"{teacher_objective_id!r}"
            )
        teacher_objective = objectives_by_id[teacher_objective_id]
        if teacher_objective.ranked:
            raise ValueError(
                f"balance report {report_id!r} teacher objective must maximize success probability"
            )

        raw_outcomes = raw_report.get("outcomes")
        if not isinstance(raw_outcomes, list) or not raw_outcomes:
            raise ValueError(f"balance report {report_id!r} requires a non-empty outcomes list")
        outcome_targets: list[Target] = []
        damage_values: list[Fraction] = []
        for raw_outcome in raw_outcomes:
            if not isinstance(raw_outcome, dict):
                raise ValueError(f"balance report {report_id!r} outcomes must be objects")
            target_id = raw_outcome.get("target")
            if target_id not in targets_by_id:
                raise ValueError(
                    f"balance report {report_id!r} references unknown target {target_id!r}"
                )
            damage = _parse_reward(raw_outcome.get("damage"), report_id)
            outcome_targets.append(targets_by_id[target_id])
            damage_values.append(damage)

        if len({target.id for target in outcome_targets}) != len(outcome_targets):
            raise ValueError(f"balance report {report_id!r} repeats a target")
        if any(
            later < earlier
            for earlier, later in zip(damage_values, damage_values[1:], strict=False)
        ):
            raise ValueError(
                f"balance report {report_id!r} outcomes must be ordered by nondecreasing damage"
            )
        objective = Objective(
            f"{report_id}_expected_damage",
            f"Maximize expected damage for {name.strip()}",
            tuple(outcome_targets),
            tuple(damage_values),
        )
        reports.append(BalanceReport(report_id, name.strip(), objective, teacher_objective))
    return tuple(reports)


def generate_policy_table(data: dict[str, Any]) -> dict[str, Any]:
    targets, objectives = parse_target_config(data)
    symbols = _parse_symbols(data.get("symbols", {}))
    states = all_states()
    serialized_objectives = []

    for objective in objectives:
        solver = PolicySolver(objective)
        stages: dict[str, Any] = {}
        for rerolls_remaining in range(MAX_REROLLS + 1):
            entries = {
                _state_key(state): _serialize_state(solver, state, rerolls_remaining)
                for state in states
            }
            stages[str(rerolls_remaining)] = {
                "state_count": len(entries),
                "states": entries,
            }

        serialized_objective = {
            "id": objective.id,
            "name": objective.name,
            "target_ids": [target.id for target in objective.targets],
            "metric": "expected_reward" if objective.ranked else "success_probability",
            "before_first_roll": _serialize_objective_value(
                objective, solver.before_first_roll()
            ),
            "stages": stages,
        }
        if objective.ranked:
            serialized_objective["tiers"] = [
                {
                    "target_id": target.id,
                    "reward": _serialize_number(reward),
                }
                for target, reward in zip(objective.targets, objective.rewards, strict=True)
            ]
            serialized_objective["before_first_roll"]["tier_probabilities"] = (
                _serialize_tier_distribution(objective, solver.before_first_tier_distribution())
            )
        serialized_objectives.append(serialized_objective)

    return {
        "schema": SCHEMA,
        "rules": {
            "dice_count": DICE_COUNT,
            "sides": SIDES,
            "maximum_rerolls": MAX_REROLLS,
            "state_count": len(states),
            "state_entries_per_objective": len(states) * (MAX_REROLLS + 1),
            "values_per_objective_including_before_first_roll": (
                len(states) * (MAX_REROLLS + 1) + 1
            ),
            "target_match": "pattern containment or minimum named-symbol count",
        },
        "symbols": {symbol: list(faces) for symbol, faces in symbols.items()},
        "targets": [
            _serialize_target(target)
            for target in targets
        ],
        "objectives": serialized_objectives,
    }


def generate_balance_table(data: dict[str, Any]) -> dict[str, Any]:
    targets, objectives = parse_target_config(data)
    reports = parse_balance_reports(data, targets, objectives)
    if not reports:
        raise ValueError("target config does not define any balance_reports")

    states = all_states()
    serialized_reports = []
    for report in reports:
        teacher_primary_solver = PolicySolver(report.teacher_objective)
        teacher_damage_solver = ConstrainedPolicySolver(
            report.objective, teacher_primary_solver
        )
        damage_solver = PolicySolver(report.objective)
        teacher_policy = _serialize_balance_policy(
            report,
            teacher_damage_solver,
            states,
            policy_id="teacher_success_policy",
            optimization=(
                f"maximize {report.teacher_objective.id} success; "
                "break exact success ties by expected damage"
            ),
        )
        teacher_policy["primary_success_probability"] = _serialize_probability(
            teacher_primary_solver.before_first_roll()
        )
        damage_policy = _serialize_balance_policy(
            report,
            damage_solver,
            states,
            policy_id="damage_optimized_policy",
            optimization="maximize expected damage",
        )
        teacher_before = teacher_policy["before_first_roll"]
        damage_before = damage_policy["before_first_roll"]
        serialized_reports.append(
            {
                "id": report.id,
                "name": report.name,
                "teacher_objective_id": report.teacher_objective.id,
                "policies": {
                    "teacher_success_policy": teacher_policy,
                    "damage_optimized_policy": damage_policy,
                },
                "comparison": {
                    "average_damage_gain_from_damage_optimization": _serialize_number(
                        _fraction_from_serialized(damage_before["average_damage"])
                        - _fraction_from_serialized(teacher_before["average_damage"])
                    ),
                    "hit_probability_change_from_damage_optimization": _serialize_probability(
                        _fraction_from_serialized(damage_before["hit_probability"])
                        - _fraction_from_serialized(teacher_before["hit_probability"])
                    ),
                },
            }
        )

    return {
        "schema": BALANCE_SCHEMA,
        "rules": {
            "dice_count": DICE_COUNT,
            "sides": SIDES,
            "maximum_rerolls": MAX_REROLLS,
            "state_count": len(states),
            "overlap_resolution": "highest_damage_completed_outcome",
        },
        "reports": serialized_reports,
    }


def _serialize_balance_policy(
    report: BalanceReport,
    solver: PolicySolver,
    states: tuple[Roll, ...],
    *,
    policy_id: str,
    optimization: str,
) -> dict[str, Any]:
    stages: dict[str, Any] = {}
    for rerolls_remaining in range(MAX_REROLLS + 1):
        entries = {
            _state_key(state): _serialize_balance_state(
                report, solver, state, rerolls_remaining
            )
            for state in states
        }
        stages[str(rerolls_remaining)] = {
            "state_count": len(entries),
            "states": entries,
        }
    return {
        "id": policy_id,
        "optimization": optimization,
        "before_first_roll": _serialize_damage_breakdown(
            report,
            solver.before_first_tier_distribution(),
            solver.before_first_roll(),
        ),
        "stages": stages,
    }


def _serialize_state(
    solver: PolicySolver, state: Roll, rerolls_remaining: int
) -> dict[str, Any]:
    value = solver.value(state, rerolls_remaining)
    entry: dict[str, Any] = {
        "roll": list(state),
        "counts": list(face_counts(state)),
        "completed": solver.objective.completed(face_counts(state)),
        "value": _serialize_objective_value(solver.objective, value),
    }
    if solver.objective.ranked:
        entry["value"]["tier_probabilities"] = _serialize_tier_distribution(
            solver.objective,
            solver.tier_distribution(state, rerolls_remaining),
        )
    if rerolls_remaining == 0:
        entry["optimal_keeps"] = []
        entry["actions"] = []
        return entry

    best_keeps = solver.best_keeps(state, rerolls_remaining)
    entry["optimal_keeps"] = [list(keep) for keep in best_keeps]
    actions = []
    for keep, action_value in solver.action_values(state, rerolls_remaining):
        action = {
            "keep": list(keep),
            "keep_counts": list(face_counts(keep)),
            "reroll_count": DICE_COUNT - len(keep),
            "value": _serialize_objective_value(solver.objective, action_value),
            "optimal": keep in best_keeps,
        }
        if solver.objective.ranked:
            action["value"]["tier_probabilities"] = _serialize_tier_distribution(
                solver.objective,
                solver.action_tier_distribution(state, rerolls_remaining, keep),
            )
        actions.append(action)
    entry["actions"] = actions
    return entry


def _serialize_balance_state(
    report: BalanceReport,
    solver: PolicySolver,
    state: Roll,
    rerolls_remaining: int,
) -> dict[str, Any]:
    distribution = solver.tier_distribution(state, rerolls_remaining)
    expected_damage = solver.value(state, rerolls_remaining)
    return {
        "roll": list(state),
        "counts": list(face_counts(state)),
        "optimal_keeps": [list(keep) for keep in solver.best_keeps(state, rerolls_remaining)],
        **_serialize_damage_breakdown(report, distribution, expected_damage),
    }


def _serialize_damage_breakdown(
    report: BalanceReport,
    distribution: tuple[Fraction, ...],
    expected_damage: Fraction,
) -> dict[str, Any]:
    labels = ("nothing", *(target.id for target in report.objective.targets))
    damages = (Fraction(0), *report.objective.rewards)
    outcomes = []
    for label, damage, probability in zip(labels, damages, distribution, strict=True):
        outcomes.append(
            {
                "outcome": label,
                "damage": _serialize_number(damage),
                "probability": _serialize_probability(probability),
                "average_damage_contribution": _serialize_number(probability * damage),
            }
        )
    return {
        "average_damage": _serialize_number(expected_damage),
        "hit_probability": _serialize_probability(1 - distribution[0]),
        "outcomes": outcomes,
    }


def _serialize_probability(value: Fraction) -> dict[str, Any]:
    return {
        "fraction": f"{value.numerator}/{value.denominator}",
        "probability": round(float(value), 12),
        "percent": round(float(value) * 100, 10),
    }


def _serialize_number(value: Fraction) -> dict[str, Any]:
    return {
        "fraction": f"{value.numerator}/{value.denominator}",
        "value": round(float(value), 12),
    }


def _fraction_from_serialized(value: dict[str, Any]) -> Fraction:
    return Fraction(value["fraction"])


def _serialize_objective_value(objective: Objective, value: Fraction) -> dict[str, Any]:
    if objective.ranked:
        return _serialize_number(value)
    return _serialize_probability(value)


def _serialize_tier_distribution(
    objective: Objective, distribution: tuple[Fraction, ...]
) -> dict[str, Any]:
    labels = ("none", *(target.id for target in objective.targets))
    return {
        label: _serialize_probability(probability)
        for label, probability in zip(labels, distribution, strict=True)
    }


def _serialize_target(target: Target) -> dict[str, Any]:
    result: dict[str, Any] = {"id": target.id, "name": target.name}
    if target.patterns:
        result["patterns"] = [_counts_to_roll(pattern) for pattern in target.patterns]
    elif target.requirements:
        result["requirements"] = [
            {
                "symbol": requirement.symbol,
                "faces": list(requirement.faces),
                "at_least": requirement.at_least,
            }
            for requirement in target.requirements
        ]
    else:
        result.update(
            {
                "symbol": target.symbol,
                "faces": list(target.symbol_faces),
                "at_least": target.at_least,
            }
        )
    return result


def _state_key(state: Roll) -> str:
    return ",".join(str(amount) for amount in face_counts(state))


def _counts_to_roll(counts: FaceCounts) -> list[int]:
    return [face for face, amount in enumerate(counts, start=1) for _ in range(amount)]


def _validated_id(value: Any, kind: str) -> str:
    if not isinstance(value, str) or not _ID_PATTERN.fullmatch(value):
        raise ValueError(f"{kind} id must match {_ID_PATTERN.pattern!r}")
    return value


def _parse_patterns(target_id: str, raw_patterns: Any) -> tuple[FaceCounts, ...]:
    if not isinstance(raw_patterns, list) or not raw_patterns:
        raise ValueError(f"target {target_id!r} requires a non-empty patterns list")
    patterns: list[FaceCounts] = []
    for raw_pattern in raw_patterns:
        if not isinstance(raw_pattern, list) or not 1 <= len(raw_pattern) <= DICE_COUNT:
            raise ValueError(
                f"each pattern for target {target_id!r} must contain 1 to {DICE_COUNT} dice"
            )
        if any(not isinstance(face, int) or isinstance(face, bool) for face in raw_pattern):
            raise ValueError(f"target {target_id!r} patterns must contain integer faces")
        if any(face < 1 or face > SIDES for face in raw_pattern):
            raise ValueError(f"target {target_id!r} faces must be between 1 and {SIDES}")
        normalized = face_counts(tuple(raw_pattern))
        if normalized not in patterns:
            patterns.append(normalized)
    return tuple(patterns)


def _parse_symbols(raw_symbols: Any) -> dict[str, Roll]:
    if not isinstance(raw_symbols, dict):
        raise ValueError("'symbols' must be an object when present")
    symbols: dict[str, Roll] = {}
    for raw_symbol, raw_faces in raw_symbols.items():
        symbol = _validated_id(raw_symbol, "symbol")
        if not isinstance(raw_faces, list) or not raw_faces:
            raise ValueError(f"symbol {symbol!r} requires a non-empty face list")
        if any(not isinstance(face, int) or isinstance(face, bool) for face in raw_faces):
            raise ValueError(f"symbol {symbol!r} faces must be integers")
        if any(face < 1 or face > SIDES for face in raw_faces):
            raise ValueError(f"symbol {symbol!r} faces must be between 1 and {SIDES}")
        if len(set(raw_faces)) != len(raw_faces):
            raise ValueError(f"symbol {symbol!r} repeats a face")
        symbols[symbol] = tuple(sorted(raw_faces))
    return symbols


def _parse_symbol_requirement(
    target_id: str, raw_requirement: Any, symbols: dict[str, Roll]
) -> SymbolRequirement:
    if not isinstance(raw_requirement, dict):
        raise ValueError(f"target {target_id!r} requirements must be objects")
    symbol = raw_requirement.get("symbol")
    if not isinstance(symbol, str) or symbol not in symbols:
        raise ValueError(f"target {target_id!r} references unknown symbol {symbol!r}")
    at_least = raw_requirement.get("at_least")
    if not isinstance(at_least, int) or isinstance(at_least, bool):
        raise ValueError(f"target {target_id!r} requirements need integer 'at_least' values")
    if not 1 <= at_least <= DICE_COUNT:
        raise ValueError(
            f"target {target_id!r} requirement 'at_least' must be 1 to {DICE_COUNT}"
        )
    return SymbolRequirement(symbol, symbols[symbol], at_least)


def _parse_reward(raw_reward: Any, objective_id: str) -> Fraction:
    if isinstance(raw_reward, bool) or not isinstance(raw_reward, int | float | str):
        raise ValueError(f"ranked objective {objective_id!r} rewards must be numbers")
    try:
        reward = Fraction(str(raw_reward))
    except (ValueError, ZeroDivisionError) as error:
        raise ValueError(
            f"ranked objective {objective_id!r} has invalid reward {raw_reward!r}"
        ) from error
    if reward < 0:
        raise ValueError(f"ranked objective {objective_id!r} rewards must be nonnegative")
    return reward


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--targets", type=Path, required=True, help="JSON target definition file")
    parser.add_argument("--output", type=Path, required=True, help="generated JSON table path")
    parser.add_argument(
        "--balance-output",
        type=Path,
        help="optional separate expected-damage balance report JSON path",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    with args.targets.open(encoding="utf-8") as handle:
        data = json.load(handle)
    table = generate_policy_table(data)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as handle:
        json.dump(table, handle, indent=2, sort_keys=False)
        handle.write("\n")

    summary_parts = []
    for objective in table["objectives"]:
        before = objective["before_first_roll"]
        if objective["metric"] == "success_probability":
            summary_parts.append(f"{objective['id']}={before['percent']:.6f}%")
        else:
            summary_parts.append(f"{objective['id']} expected reward={before['value']:.6f}")
    summary = ", ".join(summary_parts)
    print(f"Wrote {args.output}")
    print(f"Before first roll: {summary}")

    if args.balance_output is not None:
        balance_table = generate_balance_table(data)
        args.balance_output.parent.mkdir(parents=True, exist_ok=True)
        with args.balance_output.open("w", encoding="utf-8") as handle:
            json.dump(balance_table, handle, indent=2, sort_keys=False)
            handle.write("\n")
        print(f"Wrote {args.balance_output}")
        for report in balance_table["reports"]:
            for policy in report["policies"].values():
                before = policy["before_first_roll"]
                print(
                    f"{report['id']} {policy['id']}: "
                    f"average damage={before['average_damage']['value']:.6f}, "
                    f"hit chance={before['hit_probability']['percent']:.6f}%"
                )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
