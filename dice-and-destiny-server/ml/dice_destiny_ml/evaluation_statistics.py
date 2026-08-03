from __future__ import annotations

import math


def percentile(values: list[float] | list[int], quantile: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    position = (len(ordered) - 1) * quantile
    lower = math.floor(position)
    upper = math.ceil(position)
    if lower == upper:
        return float(ordered[lower])
    weight = position - lower
    return float(ordered[lower] * (1.0 - weight) + ordered[upper] * weight)


def exact_mcnemar_p(baseline_only: int, candidate_only: int) -> float:
    """Two-sided exact McNemar p-value for paired binary outcomes."""
    discordant = baseline_only + candidate_only
    if discordant == 0:
        return 1.0
    smaller = min(baseline_only, candidate_only)
    tail = sum(math.comb(discordant, index) for index in range(smaller + 1))
    return min(1.0, 2.0 * tail / (2**discordant))


def paired_boolean_comparison(baseline: list[bool], candidate: list[bool]) -> dict[str, int | float]:
    if len(baseline) != len(candidate):
        raise ValueError("paired outcomes must have the same length")
    baseline_only = sum(old and not new for old, new in zip(baseline, candidate, strict=True))
    candidate_only = sum(new and not old for old, new in zip(baseline, candidate, strict=True))
    return {
        "cases": len(baseline),
        "baseline_successes": sum(baseline),
        "candidate_successes": sum(candidate),
        "baseline_only": baseline_only,
        "candidate_only": candidate_only,
        "exact_mcnemar_p": exact_mcnemar_p(baseline_only, candidate_only),
    }
