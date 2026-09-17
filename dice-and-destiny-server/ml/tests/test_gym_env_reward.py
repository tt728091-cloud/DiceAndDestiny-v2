from __future__ import annotations

import pytest

from dice_destiny_ml.gym_env import (
    OUTCOME_ONLY_REWARD,
    WINNER_HEALTH_REWARD,
    WINNER_HEALTH_V1_REWARD,
    AuthorityGymEnv,
)


def reward(
    *,
    learner: str,
    winner: str,
    winner_health: int,
    winner_max_health: int = 20,
    definition: str = WINNER_HEALTH_REWARD,
) -> float:
    environment = object.__new__(AuthorityGymEnv)
    environment.learner_seat = learner
    environment.reward_definition = definition
    environment.transition = {
        "winner": winner,
        "metrics": {"remaining_health": {winner: winner_health}},
        "result": {
            "snapshot": {
                "actors": {
                    winner: {
                        "current_health": winner_health,
                        "max_health": winner_max_health,
                    }
                }
            }
        },
    }
    return environment._terminal_reward()


def test_winner_health_reward_uses_learner_health_on_win() -> None:
    assert reward(learner="seat-a", winner="seat-a", winner_health=10) == pytest.approx(1.0)


def test_winner_health_reward_uses_opponent_health_on_loss() -> None:
    assert reward(learner="seat-a", winner="seat-b", winner_health=16) == pytest.approx(-1.3)


def test_winner_health_reward_is_zero_sum_for_the_same_terminal_state() -> None:
    winner_reward = reward(learner="seat-b", winner="seat-b", winner_health=7)
    loser_reward = reward(learner="seat-a", winner="seat-b", winner_health=7)
    assert winner_reward == pytest.approx(-loser_reward)


def test_draw_reward_is_zero_without_health_data() -> None:
    environment = object.__new__(AuthorityGymEnv)
    environment.learner_seat = "seat-a"
    environment.reward_definition = WINNER_HEALTH_REWARD
    environment.transition = {"winner": "", "metrics": {}, "result": {}}
    assert environment._terminal_reward() == 0.0


def test_winner_health_fraction_is_clamped_to_reward_range() -> None:
    assert reward(learner="seat-a", winner="seat-a", winner_health=25) == pytest.approx(1.5)
    assert reward(learner="seat-a", winner="seat-b", winner_health=0) == pytest.approx(-0.5)


def test_winner_health_v1_remains_available_for_cp38_reproducibility() -> None:
    assert reward(
        learner="seat-a",
        winner="seat-a",
        winner_health=10,
        definition=WINNER_HEALTH_V1_REWARD,
    ) == pytest.approx(1.125)
    assert reward(
        learner="seat-a",
        winner="seat-b",
        winner_health=16,
        definition=WINNER_HEALTH_V1_REWARD,
    ) == pytest.approx(-1.35)


def test_outcome_only_reward_remains_available_for_historical_recipes() -> None:
    assert reward(
        learner="seat-a",
        winner="seat-a",
        winner_health=20,
        definition=OUTCOME_ONLY_REWARD,
    ) == 1.0
    assert reward(
        learner="seat-a",
        winner="seat-b",
        winner_health=20,
        definition=OUTCOME_ONLY_REWARD,
    ) == -1.0


def test_winner_health_reward_rejects_missing_max_health() -> None:
    environment = object.__new__(AuthorityGymEnv)
    environment.learner_seat = "seat-a"
    environment.reward_definition = WINNER_HEALTH_REWARD
    environment.transition = {
        "winner": "seat-b",
        "metrics": {"remaining_health": {"seat-b": 10}},
        "result": {"snapshot": {"actors": {"seat-b": {"current_health": 10}}}},
    }
    with pytest.raises(RuntimeError, match="winner max health"):
        environment._terminal_reward()
