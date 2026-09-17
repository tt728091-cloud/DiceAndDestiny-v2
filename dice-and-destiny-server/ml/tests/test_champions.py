from __future__ import annotations

import json
from pathlib import Path
from types import SimpleNamespace

import pytest

from dice_destiny_ml.campaign import (
    CHECKPOINT_400_HISTORY_INTERVAL,
    CHECKPOINT_400_RECIPE,
    CURRENT_GLOBAL_CHAMPION_ID,
    CURRENT_GLOBAL_LEARNER_STEPS,
    EXPECTED_GLOBAL_SHA256,
    Recipe,
    _family_evaluation_due,
    _next_family_after_plateau,
    _require_deadline_reached,
    _run_global_progression_gate,
    _updated_high_watermark_miss_streak,
    _validate_campaign_power,
    write_campaign_seed_banks,
)
from dice_destiny_ml.champions import (
    CampaignTimer,
    Champion,
    ChampionRegistry,
    ExperimentLedger,
    canonical_hash,
    global_confirmation_block,
    global_progression_block,
    write_seed_bank,
)
from dice_destiny_ml.training import ArtifactCallback


def _champion(path: Path, identifier: str, value: bytes) -> Champion:
    path.write_bytes(value)
    return Champion.from_checkpoint(
        champion_id=identifier,
        checkpoint=path,
        observation_schema="observation",
        action_schema="actions",
        model_family="family",
        learner_steps=1,
        promoted_at="now",
    )


def test_global_progression_uses_strict_family_relative_improvement() -> None:
    first = global_progression_block(wins=55, draws=0, losses=945, baseline_score=None)
    assert first.decision == "pass"
    assert first.adjusted_score == pytest.approx(0.055)
    assert (
        global_progression_block(wins=62, draws=0, losses=938, baseline_score=first.adjusted_score).decision
        == "pass"
    )
    assert (
        global_progression_block(wins=55, draws=0, losses=945, baseline_score=first.adjusted_score).decision
        == "fail"
    )
    assert (
        global_progression_block(wins=40, draws=0, losses=960, baseline_score=first.adjusted_score).decision
        == "fail"
    )
    assert (
        global_progression_block(
            wins=900,
            draws=0,
            losses=100,
            baseline_score=None,
            safety={"authority_rejects": 1},
        ).decision
        == "fail"
    )
    with pytest.raises(ValueError):
        global_progression_block(wins=519, draws=0, losses=480, baseline_score=0.5)
    with pytest.raises(ValueError):
        global_progression_block(wins=500, draws=0, losses=500, baseline_score=1.01)


def test_global_confirmation_requires_fresh_3k_majority_and_safety() -> None:
    assert global_confirmation_block(wins=1501, draws=0, losses=1499).decision == "pass"
    assert global_confirmation_block(wins=1500, draws=0, losses=1500).decision == "fail"
    assert global_confirmation_block(wins=1490, draws=40, losses=1470).decision == "pass"
    assert (
        global_confirmation_block(
            wins=2000,
            draws=0,
            losses=1000,
            safety={"replay_mismatches": 1},
        ).decision
        == "fail"
    )
    with pytest.raises(ValueError):
        global_confirmation_block(wins=501, draws=0, losses=499)


def test_progression_runner_uses_only_global_and_confirms_only_an_accepted_beater(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    config = SimpleNamespace(seed_bank_root=tmp_path)
    challenger = SimpleNamespace(checkpoint_sha256="challenger")
    global_champion = SimpleNamespace(checkpoint_sha256="global")
    calls: list[tuple[object, str, int]] = []
    outcomes = iter(
        [
            {"wins": 520, "draws": 0, "losses": 480, "safety": {}},
            {"wins": 1501, "draws": 0, "losses": 1499, "safety": {}},
        ]
    )

    def fake_match(
        _config: object,
        _challenger: object,
        opponent: object,
        bank: Path,
        games: int,
        _output: Path,
    ) -> dict[str, object]:
        calls.append((opponent, bank.name, games))
        return next(outcomes)

    monkeypatch.setattr("dice_destiny_ml.campaign._run_match_block", fake_match)
    result = _run_global_progression_gate(config, 4, challenger, global_champion, 0.50, tmp_path / "gate")
    assert result["checkpoint_decision"] == "pass"
    assert result["global_confirmation_required"] is True
    assert result["global_promoted"] is True
    assert calls == [
        (global_champion, "attempt-004-global-1k.json", 1000),
        (global_champion, "attempt-004-global-confirmation-3k.json", 3000),
    ]

    calls.clear()
    outcomes = iter([{"wins": 520, "draws": 0, "losses": 480, "safety": {}}])
    result = _run_global_progression_gate(config, 5, challenger, global_champion, 0.55, tmp_path / "gate-2")
    assert result["checkpoint_decision"] == "fail"
    assert result["global_confirmation_required"] is False
    assert result["global_promoted"] is False
    assert calls == [(global_champion, "attempt-005-global-1k.json", 1000)]


def test_registry_is_atomic_hash_checked_and_deduplicates_pool(tmp_path: Path) -> None:
    global_champion = _champion(tmp_path / "global.zip", "global", b"global")
    checkpoint = _champion(tmp_path / "checkpoint.zip", "checkpoint", b"checkpoint")
    registry = ChampionRegistry(global_champion, checkpoint, hall=[global_champion])
    path = tmp_path / "registry.json"
    registry.save(path)
    loaded = ChampionRegistry.load(path)
    assert [entry.champion_id for entry in loaded.opponent_entries()] == ["checkpoint", "global"]
    (tmp_path / "ordinary.zip").write_bytes(b"ordinary")
    assert [entry.champion_id for entry in loaded.opponent_entries()] == ["checkpoint", "global"]
    Path(checkpoint.checkpoint_path).write_bytes(b"changed")
    with pytest.raises(RuntimeError, match="hash mismatch"):
        ChampionRegistry.load(path)


def test_registry_retains_replaced_checkpoint_and_global_champions(tmp_path: Path) -> None:
    global_champion = _champion(tmp_path / "global.zip", "global", b"global")
    checkpoint = _champion(tmp_path / "checkpoint.zip", "checkpoint", b"checkpoint")
    challenger = _champion(tmp_path / "challenger.zip", "challenger", b"challenger")
    registry = ChampionRegistry(global_champion, checkpoint)
    registry.promote_checkpoint(challenger)
    assert [entry.champion_id for entry in registry.hall] == ["checkpoint"]
    registry.promote_global(challenger)
    assert [entry.champion_id for entry in registry.hall] == ["checkpoint", "global"]


def test_seed_bank_is_seat_swapped_and_hashed(tmp_path: Path) -> None:
    result = write_seed_bank(
        tmp_path / "seeds.json", bank_id="gate-1", seeds=list(range(500)), purpose="checkpoint"
    )
    assert result["seat_swapped_games"] == 1000
    stored = json.loads((tmp_path / "seeds.json").read_text())
    checksum = stored.pop("sha256")
    assert checksum == canonical_hash(stored)


def test_campaign_seed_banks_are_predeclared_and_disjoint(tmp_path: Path) -> None:
    manifest = write_campaign_seed_banks(tmp_path, attempts=2)
    assert manifest["schema"] == "dice-and-destiny-campaign-seed-banks-v2"
    assert len(manifest["files"]) == 6
    assert [entry["games"] for entry in manifest["files"]] == [
        1000,
        3000,
        1000,
        3000,
        1000,
        1000,
    ]
    seen: set[int] = set()
    for entry in manifest["files"]:
        value = json.loads(Path(entry["path"]).read_text())
        assert not seen.intersection(value["seeds"])
        seen.update(value["seeds"])
    assert manifest["distinct_seeds"] == len(seen)

    second = write_campaign_seed_banks(tmp_path / "second", attempts=2, seed_offset=1_000_000_000)
    second_seen = {
        seed for entry in second["files"] for seed in json.loads(Path(entry["path"]).read_text())["seeds"]
    }
    assert not seen.intersection(second_seen)


def test_checkpoint_400_recipe_is_the_only_within_family_recipe() -> None:
    assert CHECKPOINT_400_HISTORY_INTERVAL == 5_000
    assert CHECKPOINT_400_RECIPE == Recipe(
        learning_rate=3e-4,
        epochs=8,
        clip_range=0.2,
        target_kl=None,
        entropy_coefficient=0.01,
        batch_size=256,
        rollout_steps=256,
        gamma=0.995,
        gae_lambda=0.95,
        value_loss_coefficient=0.5,
    )


def test_current_global_registry_matches_constants_and_retains_prior_champions() -> None:
    registry_path = Path(__file__).resolve().parents[1] / "champions" / "current-global.json"
    registry = ChampionRegistry.load(registry_path)
    assert registry.global_champion.champion_id == CURRENT_GLOBAL_CHAMPION_ID
    assert registry.global_champion.learner_steps == CURRENT_GLOBAL_LEARNER_STEPS
    assert registry.global_champion.checkpoint_sha256 == EXPECTED_GLOBAL_SHA256
    assert registry.checkpoint_champion.checkpoint_sha256 == EXPECTED_GLOBAL_SHA256
    hall_ids = [entry.champion_id for entry in registry.hall]
    assert hall_ids[0] == "raw-v3-step-400"
    assert "raw-v3-step-480" in hall_ids
    assert "v2-winner-health-fresh-20260805-1h-interval-015" in hall_ids
    assert "v2-winner-health-fresh-20260805-1h-interval-038" in hall_ids
    assert "v2-global-hillclimb-20260805-90m-attempt-031" in hall_ids
    assert "v2-global-hillclimb-20260805-90m-attempt-037" in hall_ids


def test_history_checkpoint_counter_can_continue_across_campaign_intervals(tmp_path: Path) -> None:
    callback = ArtifactCallback(
        tmp_path,
        CHECKPOINT_400_HISTORY_INTERVAL,
        instrumentation=False,
        checkpoint_directory=tmp_path / "history",
        initial_checkpoint_step=50_040,
    )
    assert callback.last_checkpoint == 50_040
    with pytest.raises(ValueError, match="cannot be negative"):
        ArtifactCallback(
            tmp_path,
            CHECKPOINT_400_HISTORY_INTERVAL,
            instrumentation=False,
            initial_checkpoint_step=-1,
        )


def test_family_waits_five_intervals_and_fails_after_three_consecutive_misses() -> None:
    assert [_family_evaluation_due(attempt) for attempt in range(1, 7)] == [
        False,
        False,
        False,
        False,
        True,
        True,
    ]
    streak = _updated_high_watermark_miss_streak(0, "fail")
    streak = _updated_high_watermark_miss_streak(streak, "fail")
    assert streak == 2
    assert _updated_high_watermark_miss_streak(streak, "pass") == 0
    assert _updated_high_watermark_miss_streak(2, "fail") == 3


def test_plateau_starts_next_predeclared_family_until_deadline() -> None:
    assert (
        _next_family_after_plateau(
            0,
            3,
            may_start_interval=True,
            require_full_window=True,
        )
        == 1
    )
    assert (
        _next_family_after_plateau(
            0,
            3,
            may_start_interval=False,
            require_full_window=True,
        )
        is None
    )
    with pytest.raises(RuntimeError, match="plan exhausted"):
        _next_family_after_plateau(
            2,
            3,
            may_start_interval=True,
            require_full_window=True,
        )
    assert (
        _next_family_after_plateau(
            2,
            3,
            may_start_interval=True,
            require_full_window=False,
        )
        is None
    )


def test_required_full_window_treats_every_early_stop_as_failure() -> None:
    _require_deadline_reached(required=True, deadline_crossed=True, reason="deadline")
    _require_deadline_reached(required=False, deadline_crossed=False, reason="legacy")
    with pytest.raises(RuntimeError, match="stopped early"):
        _require_deadline_reached(required=True, deadline_crossed=False, reason="test")


def test_full_window_power_requires_ac_unless_owner_override_is_explicit() -> None:
    ac = "Now drawing from 'AC Power'"
    battery = "Now drawing from 'Battery Power'"
    _validate_campaign_power(ac, require_full_window=True, allow_battery_power=False)
    _validate_campaign_power(battery, require_full_window=False, allow_battery_power=False)
    _validate_campaign_power(battery, require_full_window=True, allow_battery_power=True)
    with pytest.raises(RuntimeError, match="must start on AC power"):
        _validate_campaign_power(
            battery,
            require_full_window=True,
            allow_battery_power=False,
        )


def test_ledger_hash_chain_query_and_render(tmp_path: Path) -> None:
    ledger = ExperimentLedger(tmp_path / "ledger.jsonl")
    base = {
        "event": "experiment_started",
        "experiment_id": "child",
        "parent_experiment_id": "parent",
        "checkpoint_champion_sha256": "checkpoint",
        "global_champion_sha256": "global",
        "source_revision": "source",
        "content_sha256": "content",
        "observation_schema_sha256": "observation",
        "action_schema_sha256": "action",
        "architecture_sha256": "architecture",
        "reward_sha256": "reward",
        "teacher_setup_sha256": "teacher",
        "opponent_curriculum_sha256": "opponents",
        "ppo_recipe_sha256": "ppo",
        "config": {"lr": 3e-4},
        "config_diff": {"lr": [1e-4, 3e-4]},
        "hypothesis": "test",
    }
    first = ledger.append(base)
    second = ledger.append(
        {
            **base,
            "event": "checkpoint_rejected",
            "result": {"adjusted_score": 0.47, "decision": "fail"},
        }
    )
    assert second["previous_record_sha256"] == first["record_sha256"]
    assert len(ledger.comparable(second)) == 2
    assert "checkpoint_rejected" in ledger.summary_path.read_text()
    values = [json.loads(line) for line in ledger.path.read_text().splitlines()]
    values[0]["hypothesis"] = "tampered"
    ledger.path.write_text("\n".join(json.dumps(value) for value in values) + "\n")
    with pytest.raises(RuntimeError, match="hash mismatch"):
        ledger.records()


def test_timer_only_blocks_new_intervals_and_allows_grace_completion() -> None:
    timer = CampaignTimer(10.0)
    assert timer.may_start_interval(now=100.0) is False
    timer.start(now=100.0)
    assert timer.may_start_interval(now=109.999)
    assert not timer.may_start_interval(now=110.0)
    assert timer.deadline_crossed(now=111.0)
    # The timer carries no cancellation primitive: callers finish an interval
    # already in flight and consult may_start_interval only before the next one.
    assert timer.snapshot(now=111.0)["deadline_crossed"] is True
