from inspect import signature
from pathlib import Path

import pytest

from dice_destiny_ml.gym_env import AuthorityGymEnv
from dice_destiny_ml.training import TrainingConfig, train_seed
from dice_destiny_ml.winner_health_campaign import WinnerHealthCampaignConfig


def test_new_training_runs_default_to_deterministic_opponents() -> None:
    assert TrainingConfig(seed=22).opponent_deterministic is True
    assert (
        WinnerHealthCampaignConfig.__dataclass_fields__["deterministic_training_opponent"].default
        is True
    )
    assert signature(AuthorityGymEnv).parameters["opponent_deterministic"].default is True


def test_main_training_rejects_automatic_post_ppo_correction(tmp_path: Path) -> None:
    with pytest.raises(ValueError, match="automatic post-PPO correction is forbidden"):
        train_seed(
            config=TrainingConfig(seed=22, post_ppo_correction=True),
            binary=tmp_path / "unused-binary",
            server_root=tmp_path,
            output_root=tmp_path / "runs",
        )
    assert not (tmp_path / "runs").exists()
