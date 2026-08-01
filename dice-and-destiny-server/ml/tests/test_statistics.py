from dice_destiny_ml.evaluation import percentile, wilson_interval


def test_wilson_interval_excludes_half_for_clear_win_rate() -> None:
    lower, upper = wilson_interval(600, 1000)
    assert lower > 0.5
    assert upper < 0.65


def test_percentile_interpolates() -> None:
    assert percentile([1, 2, 3, 4, 5], 0.5) == 3
    assert percentile([], 0.95) == 0
