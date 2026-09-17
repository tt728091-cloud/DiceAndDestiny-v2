from __future__ import annotations

from dataclasses import replace
from pathlib import Path

import pytest

from dice_destiny_ml.champions import global_progression_block
from dice_destiny_ml.resources import resolve_resource_budget
from dice_destiny_ml.winner_health_campaign import (
    CHECKPOINT_EVALUATION_GAMES,
    CP480_SHA256,
    WARMUP_INTERVALS,
    WinnerHealthCampaignConfig,
    _campaign_recipe,
    _interval_expected_steps,
    _load_seed_manifest,
    _plateau_transition,
    _stability_telemetry,
    _write_seed_manifest,
)


def test_first_evaluation_follows_five_unevaluated_intervals() -> None:
    assert CHECKPOINT_EVALUATION_GAMES == 1_000
    assert WARMUP_INTERVALS == 5
    assert [interval > WARMUP_INTERVALS for interval in range(1, 8)] == [
        False,
        False,
        False,
        False,
        False,
        True,
        True,
    ]


def test_progression_gate_establishes_then_strictly_improves_baseline() -> None:
    baseline = global_progression_block(wins=100, draws=100, losses=800, baseline_score=None)
    assert baseline.decision == "pass"
    assert baseline.adjusted_score == 0.15
    tie = global_progression_block(wins=100, draws=100, losses=800, baseline_score=0.15)
    assert tie.decision == "fail"
    improvement = global_progression_block(wins=101, draws=100, losses=799, baseline_score=0.15)
    assert improvement.decision == "pass"


def test_seed_manifest_is_fresh_unique_and_disjoint(tmp_path: Path) -> None:
    manifest = _write_seed_manifest(tmp_path, 4)
    assert manifest["initial_seed"] != 22
    assert len({item["training_seed"] for item in manifest["intervals"]}) == 4
    loaded = _load_seed_manifest(tmp_path, 4)
    assert loaded["sha256"] == manifest["sha256"]


def test_seed_manifest_can_disable_global_confirmation_banks(tmp_path: Path) -> None:
    manifest = _write_seed_manifest(tmp_path, 4, global_confirmation_games=0)
    assert manifest["global_confirmation_games"] == 0
    assert all("global_3k" not in item for item in manifest["intervals"])
    assert not list(tmp_path.glob("*-global-3k.json"))
    loaded = _load_seed_manifest(tmp_path, 4, global_confirmation_games=0)
    assert loaded["sha256"] == manifest["sha256"]


def test_plateau_stops_at_first_three_consecutive_failures_and_pass_resets() -> None:
    baseline = 0.6185
    failure_one, score, streak, stop = _plateau_transition(
        baseline_score=baseline,
        wins=578,
        draws=80,
        losses=342,
        safety=None,
        failure_streak=0,
    )
    assert failure_one.decision == "fail"
    assert (score, streak, stop) == (baseline, 1, False)
    passed, score, streak, stop = _plateau_transition(
        baseline_score=baseline,
        wins=579,
        draws=80,
        losses=341,
        safety=None,
        failure_streak=1,
    )
    assert passed.decision == "pass"
    assert (score, streak, stop) == (0.619, 0, False)
    for expected_streak in (1, 2, 3):
        failed, next_score, streak, stop = _plateau_transition(
            baseline_score=score,
            wins=570,
            draws=80,
            losses=350,
            safety=None,
            failure_streak=expected_streak - 1,
        )
        assert failed.decision == "fail"
        assert next_score == score
        assert streak == expected_streak
        assert stop is (expected_streak == 3)


def test_campaign_controls_support_resumed_1k_only_seven_failure_run(
    tmp_path: Path,
) -> None:
    config = WinnerHealthCampaignConfig(
        campaign_id="test",
        artifact_root=tmp_path / "run",
        registry_path=tmp_path / "registry.json",
        source_config_path=tmp_path / "source.json",
        binary=tmp_path / "battle-ml-sim",
        server_root=tmp_path,
        budget=resolve_resource_budget("light-50", logical_cpus=4),
        expected_global_sha256="7" * 64,
        failure_streak_limit=7,
        warmup_intervals=0,
        global_confirmation_games=0,
        source_campaign_root=tmp_path / "source",
        source_interval=35,
    )
    assert config.expected_global_sha256 != CP480_SHA256
    assert config.failure_streak_limit == 7
    assert config.warmup_intervals == 0
    assert config.global_confirmation_games == 0
    assert config.source_interval == 35


def test_stability_probe_uses_four_exact_rollouts_and_reports_clip_pressure(
    tmp_path: Path,
) -> None:
    config = WinnerHealthCampaignConfig(
        campaign_id="stability",
        artifact_root=tmp_path / "run",
        registry_path=tmp_path / "registry.json",
        source_config_path=tmp_path / "source.json",
        binary=tmp_path / "battle-ml-sim",
        server_root=tmp_path,
        budget=resolve_resource_budget("max", logical_cpus=16),
        interval_requested_steps=12_288,
        ppo_learning_rate=1e-4,
        ppo_epochs=4,
        ppo_clip_range=0.15,
        ppo_target_kl=0.015,
        ppo_batch_size=512,
        ppo_max_grad_norm=0.75,
    )
    assert _interval_expected_steps(config) == 12_288
    assert _campaign_recipe(config).learning_rate == 1e-4
    telemetry = {
        "relative_parameter_l2_movement": 0.04,
        "ppo": {"entropy_loss": -0.5, "explained_variance": 0.75},
        "profile": {
            "counters": {
                "ppo.train_calls": 4,
                "ppo.target_kl_early_stops": 1,
                "ppo.approx_kl_sum": 0.96,
                "ppo.approx_kl_samples": 96,
                "ppo.clip_fraction_sum": 9.6,
                "ppo.clip_fraction_samples": 96,
                "ppo.gradient_norm_sum": 28.8,
                "ppo.gradient_norm_samples": 96,
                "ppo.gradient_clip_events": 24,
                "ppo.optimizer_updates": 95,
            }
        },
    }
    stability = _stability_telemetry(config, telemetry)
    assert stability["multiworker_rollouts"] == 4
    assert stability["planned_optimizer_updates"] == 96
    assert stability["actual_optimizer_updates"] == 95
    assert stability["target_kl_early_stop_rate"] == 0.25
    assert stability["mean_approx_kl_all_minibatches"] == 0.01
    assert stability["policy_ratio_clip_range"] == 0.15
    assert stability["mean_policy_ratio_clip_fraction_all_minibatches"] == pytest.approx(0.1)
    assert stability["gradient_norm_limit"] == 0.75
    assert stability["gradient_clip_event_rate"] == 0.25
    assert stability["relative_parameter_l2_movement"] == 0.04


def test_default_interval_keeps_legacy_rollout_rounding(tmp_path: Path) -> None:
    config = WinnerHealthCampaignConfig(
        campaign_id="legacy",
        artifact_root=tmp_path / "run",
        registry_path=tmp_path / "registry.json",
        source_config_path=tmp_path / "source.json",
        binary=tmp_path / "battle-ml-sim",
        server_root=tmp_path,
        budget=resolve_resource_budget("max", logical_cpus=16),
    )
    assert _interval_expected_steps(config) == 52_224
    assert _interval_expected_steps(replace(config, interval_requested_steps=12_288)) == 12_288
