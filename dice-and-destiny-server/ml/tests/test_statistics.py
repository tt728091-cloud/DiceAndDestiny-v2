import pytest

from dice_destiny_ml.evaluation import percentile, wilson_interval
from dice_destiny_ml.evaluation_statistics import exact_mcnemar_p, paired_boolean_comparison


def test_wilson_interval_excludes_half_for_clear_win_rate() -> None:
    lower, upper = wilson_interval(600, 1000)
    assert lower > 0.5
    assert upper < 0.65


def test_percentile_interpolates() -> None:
    assert percentile([1, 2, 3, 4, 5], 0.5) == 3
    assert percentile([], 0.95) == 0


def test_exact_mcnemar_reports_paired_improvement() -> None:
    comparison = paired_boolean_comparison(
        [True] * 5 + [False] * 34 + [True, False],
        [False] * 5 + [True] * 34 + [True, False],
    )
    assert comparison["baseline_only"] == 5
    assert comparison["candidate_only"] == 34
    assert comparison["exact_mcnemar_p"] == pytest.approx(2.4299079086631536e-06)
    assert exact_mcnemar_p(0, 0) == 1.0


def test_paired_boolean_comparison_rejects_unpaired_samples() -> None:
    with pytest.raises(ValueError, match="same length"):
        paired_boolean_comparison([True], [])
