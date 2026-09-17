from fractions import Fraction

from dice_destiny_ml.dice_policy_table import (
    ConstrainedPolicySolver,
    PolicySolver,
    all_states,
    face_counts,
    parse_balance_reports,
    parse_target_config,
)


def example_config() -> dict:
    return {
        "targets": [
            {"id": "three_ones", "name": "three ones", "patterns": [[1, 1, 1]]},
            {"id": "three_twos", "name": "three twos", "patterns": [[2, 2, 2]]},
            {
                "id": "small_straight",
                "name": "a small straight",
                "patterns": [[1, 2, 3, 4], [2, 3, 4, 5], [3, 4, 5, 6]],
            },
        ]
    }


def symbol_config() -> dict:
    return {
        "symbols": {"sword": [1, 2, 3], "shield": [4, 5], "heart": [6]},
        "targets": [
            {
                "id": "three_shields",
                "name": "at least three shields",
                "symbol": "shield",
                "at_least": 3,
            },
            {
                "id": "four_shields",
                "name": "at least four shields",
                "symbol": "shield",
                "at_least": 4,
            },
            {
                "id": "five_shields",
                "name": "five shields",
                "symbol": "shield",
                "at_least": 5,
            },
            {
                "id": "small_straight",
                "name": "a small straight",
                "patterns": [[1, 2, 3, 4], [2, 3, 4, 5], [3, 4, 5, 6]],
            },
        ],
        "ranked_objectives": [
            {
                "id": "shield_tiers",
                "name": "Prefer the highest shield tier",
                "tiers": [
                    {"target": "three_shields", "reward": 1},
                    {"target": "four_shields", "reward": 2},
                    {"target": "five_shields", "reward": 3},
                ],
            }
        ],
        "balance_reports": [
            {
                "id": "offensive_cycle_damage",
                "name": "Shield tiers and small straight damage",
                "teacher_objective": "any_of_all",
                "outcomes": [
                    {"target": "three_shields", "damage": 5},
                    {"target": "four_shields", "damage": 6},
                    {"target": "five_shields", "damage": 7},
                    {"target": "small_straight", "damage": 9},
                ],
            }
        ],
    }


def test_config_creates_individual_and_any_of_all_objectives() -> None:
    targets, objectives = parse_target_config(example_config())

    assert [target.id for target in targets] == ["three_ones", "three_twos", "small_straight"]
    assert [objective.id for objective in objectives] == [
        "three_ones",
        "three_twos",
        "small_straight",
        "any_of_all",
    ]


def test_all_sorted_5d6_states_are_present() -> None:
    states = all_states()

    assert len(states) == 252
    assert states[0] == (1, 1, 1, 1, 1)
    assert states[-1] == (6, 6, 6, 6, 6)
    assert len({face_counts(state) for state in states}) == 252


def test_known_two_reroll_values_and_optimal_keeps() -> None:
    _, objectives = parse_target_config(example_config())
    solvers = {objective.id: PolicySolver(objective) for objective in objectives}
    state = (1, 6, 6, 6, 6)

    assert solvers["three_ones"].value(state, 2) == Fraction(200497, 559872)
    assert solvers["three_ones"].best_keeps(state, 2) == ((1,),)

    assert solvers["three_twos"].value(state, 2) == Fraction(1718321, 10077696)
    assert solvers["three_twos"].best_keeps(state, 2) == ((),)

    assert solvers["small_straight"].value(state, 2) == Fraction(259367, 629856)
    assert solvers["small_straight"].best_keeps(state, 2) == ((),)

    assert solvers["any_of_all"].value(state, 2) == Fraction(13085, 23328)
    assert solvers["any_of_all"].best_keeps(state, 2) == ((1,),)


def test_before_first_roll_values_include_optimal_two_reroll_strategy() -> None:
    _, objectives = parse_target_config(example_config())
    solvers = {objective.id: PolicySolver(objective) for objective in objectives}

    expected_fixed_triple = Fraction(27807523471, 78364164096)
    assert solvers["three_ones"].before_first_roll() == expected_fixed_triple
    assert solvers["three_twos"].before_first_roll() == expected_fixed_triple
    assert solvers["small_straight"].before_first_roll() == Fraction(753572225, 1224440064)
    assert solvers["any_of_all"].before_first_roll() == Fraction(457043135, 612220032)


def test_symbol_count_targets_and_ranked_tier_distribution() -> None:
    targets, objectives = parse_target_config(symbol_config())
    targets_by_id = {target.id: target for target in targets}
    solvers = {objective.id: PolicySolver(objective) for objective in objectives}

    assert targets_by_id["three_shields"].completed(face_counts((1, 4, 4, 5, 6)))
    assert not targets_by_id["four_shields"].completed(face_counts((1, 4, 4, 5, 6)))

    denominator = 27**5
    assert solvers["three_shields"].before_first_roll() == Fraction(12_078_699, denominator)
    assert solvers["four_shields"].before_first_roll() == Fraction(7_688_939, denominator)
    assert solvers["five_shields"].before_first_roll() == Fraction(2_476_099, denominator)
    assert solvers["small_straight"].before_first_roll() == Fraction(753_572_225, 1_224_440_064)
    assert solvers["any_of_all"].before_first_roll() == Fraction(40_986_899, 45_349_632)
    assert solvers["any_of_all"].best_keeps((1, 6, 6, 6, 6), 2) == ((),)
    assert solvers["any_of_all"].best_keeps((4, 6, 6, 6, 6), 2) == ((4,),)

    ranked = solvers["shield_tiers"]
    assert ranked.before_first_tier_distribution() == (
        Fraction(2_270_208, denominator),
        Fraction(4_389_760, denominator),
        Fraction(5_212_840, denominator),
        Fraction(2_476_099, denominator),
    )
    assert ranked.best_keeps((4, 6, 6, 6, 6), 2) == ((4,),)


def test_expected_damage_balance_report_is_separate_and_exclusive() -> None:
    config = symbol_config()
    targets, objectives = parse_target_config(config)
    reports = parse_balance_reports(config, targets, objectives)

    assert "offensive_cycle_damage" not in {objective.id for objective in objectives}
    assert len(reports) == 1

    solver = PolicySolver(reports[0].objective)
    assert solver.before_first_roll() == Fraction(65_028_430_753, 9_795_520_512)
    assert solver.before_first_tier_distribution() == (
        Fraction(137_622_167, 1_088_391_168),
        Fraction(4_515_021_245, 29_386_561_536),
        Fraction(1_176_098_645, 7_346_640_384),
        Fraction(472_288_283, 7_346_640_384),
        Fraction(2_427_032_345, 4_897_760_256),
    )
    assert solver.best_keeps((3, 4, 6, 6, 6), 2) == ((3, 4),)

    teacher_primary = PolicySolver(reports[0].teacher_objective)
    teacher_damage = ConstrainedPolicySolver(reports[0].objective, teacher_primary)
    assert teacher_primary.before_first_roll() == Fraction(40_986_899, 45_349_632)
    assert teacher_damage.before_first_roll() == Fraction(879_280_529, 136_048_896)
    assert teacher_damage.before_first_tier_distribution() == (
        Fraction(4_362_733, 45_349_632),
        Fraction(690_635_935, 3_673_320_192),
        Fraction(219_631_075, 918_330_048),
        Fraction(46_297_403, 459_165_024),
        Fraction(1_797_395, 4_782_969),
    )


def test_balance_report_allows_zero_value_utility_outcome() -> None:
    config = symbol_config()
    config["balance_reports"][0]["outcomes"][0]["damage"] = 0
    targets, objectives = parse_target_config(config)

    report = parse_balance_reports(config, targets, objectives)[0]

    assert report.objective.rewards[0] == 0


def test_compound_symbol_requirements_support_mixed_ability_targets() -> None:
    config = {
        "symbols": {"sword": [1, 2, 3], "life": [4, 5], "pow": [6]},
        "targets": [
            {
                "id": "sturdy_blow",
                "requirements": [
                    {"symbol": "sword", "at_least": 2},
                    {"symbol": "pow", "at_least": 2},
                ],
            }
        ],
    }
    targets, objectives = parse_target_config(config)

    assert targets[0].completed(face_counts((1, 2, 6, 6, 6)))
    assert targets[0].completed(face_counts((1, 2, 3, 6, 6)))
    assert not targets[0].completed(face_counts((1, 2, 3, 4, 6)))
    assert PolicySolver(objectives[0]).before_first_roll() > Fraction(0)
