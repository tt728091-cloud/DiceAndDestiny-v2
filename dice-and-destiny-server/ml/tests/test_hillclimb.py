from __future__ import annotations

import json
from pathlib import Path

import pytest

from dice_destiny_ml.champions import Champion, read_seed_bank
from dice_destiny_ml.hillclimb import _next_parent_after_gate, write_hillclimb_seed_banks


def _champion(path: Path, champion_id: str, payload: bytes) -> Champion:
    path.write_bytes(payload)
    return Champion.from_checkpoint(
        champion_id=champion_id,
        checkpoint=path,
        observation_schema="dice-and-destiny-observation-v2",
        action_schema="dice-and-destiny-action-candidates-v2",
        model_family="raw-v3-v2-observation",
        learner_steps=1,
        promoted_at="now",
    )


def test_hillclimb_pass_advances_and_failure_rolls_back(tmp_path: Path) -> None:
    parent = _champion(tmp_path / "parent.zip", "parent", b"parent")
    challenger = _champion(tmp_path / "challenger.zip", "challenger", b"challenger")
    assert _next_parent_after_gate(parent, challenger, "pass") is challenger
    assert _next_parent_after_gate(parent, challenger, "fail") is parent
    with pytest.raises(ValueError, match="unknown global gate decision"):
        _next_parent_after_gate(parent, challenger, "maybe")


def test_hillclimb_banks_are_fresh_disjoint_and_predeclare_training_streams(
    tmp_path: Path,
) -> None:
    manifest = write_hillclimb_seed_banks(
        tmp_path,
        attempts=3,
        evaluation_seed_offset=7_000_000_000,
        training_seed_offset=800_000,
    )
    assert manifest["distinct_evaluation_seeds"] == 1_500
    assert [entry["training_seed"] for entry in manifest["files"]] == [800_001, 800_002, 800_003]
    seen: set[int] = set()
    for entry in manifest["files"]:
        seeds = read_seed_bank(Path(entry["path"]), expected_games=1_000)
        assert not seen.intersection(seeds)
        seen.update(seeds)
    saved = json.loads((tmp_path / "manifest.json").read_text())
    assert saved["sha256"] == manifest["sha256"]


def test_hillclimb_banks_refuse_overwrite(tmp_path: Path) -> None:
    write_hillclimb_seed_banks(
        tmp_path,
        attempts=1,
        evaluation_seed_offset=7_100_000_000,
        training_seed_offset=900_000,
    )
    with pytest.raises(FileExistsError, match="refusing to overwrite"):
        write_hillclimb_seed_banks(
            tmp_path,
            attempts=1,
            evaluation_seed_offset=7_200_000_000,
            training_seed_offset=1_000_000,
        )
