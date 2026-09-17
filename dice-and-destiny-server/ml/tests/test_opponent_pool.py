from __future__ import annotations

from collections import Counter
from pathlib import Path
from random import Random

from dice_destiny_ml import opponent_pool
from dice_destiny_ml.champions import Champion, ChampionRegistry
from dice_destiny_ml.opponent_pool import OpponentManager
from dice_destiny_ml.training import atomic_model_save


class FakePolicy:
    def __init__(self, name: str) -> None:
        self.name = name
        self.inference_seconds: list[float] = []

    def reset(self, seed: int, seat_id: str) -> None:
        del seed, seat_id

    def select(self, transition: dict, decision: object) -> int:
        del transition, decision
        return 0


def test_flat_manifest_preserves_legacy_expansion_and_cache_classes(tmp_path: Path, monkeypatch) -> None:
    first = tmp_path / "a.zip"
    second = tmp_path / "b.zip"
    first.touch()
    second.touch()
    builds: list[str] = []

    def fake_build(specification: str, **kwargs) -> FakePolicy:
        del kwargs
        builds.append(specification)
        return FakePolicy(specification)

    monkeypatch.setattr(opponent_pool, "build_policy", fake_build)
    manager = OpponentManager(
        ["random", f"pool:{tmp_path}", "mechanics-v2"],
        device="cpu",
        deterministic=False,
        historical_cache_size=1,
    )
    assert manager.flat_manifest() == (
        "random",
        f"model:{first}",
        f"model:{second}",
        "mechanics-v2",
    )
    legacy_rng = Random(22)
    assert manager.select(legacy_rng, mode="legacy-flat").specification == Random(22).choice(
        manager.flat_manifest()
    )
    balanced_rng = Random(33)
    categories = [manager.select(balanced_rng, mode="category-balanced").category for _ in range(400)]
    assert set(categories) == {"random", "mechanics", "historical"}
    assert max(categories.count(category) for category in set(categories)) < 170
    assert manager.acquire("random").cache_status == "static_miss"
    assert manager.acquire("random").cache_status == "static_hit"
    assert manager.acquire(f"model:{first}").cache_status == "historical_miss"
    assert manager.acquire(f"model:{first}").cache_status == "historical_hit"
    assert manager.acquire(f"model:{second}").cache_status == "historical_miss"
    assert manager.acquire(f"model:{first}").cache_status == "historical_miss"
    assert builds == ["random", f"model:{first}", f"model:{second}", f"model:{first}"]
    assert len(manager._historical_cache) == 1


def test_atomic_model_save_never_publishes_a_partial_zip(tmp_path: Path) -> None:
    checkpoints = tmp_path / "checkpoints"
    checkpoints.mkdir()
    target = checkpoints / "final.zip"

    class FakeModel:
        def save(self, path: Path) -> None:
            assert list(checkpoints.glob("*.zip")) == []
            assert path.parent.name == ".staging"
            path.write_bytes(b"complete")

    atomic_model_save(FakeModel(), target)  # type: ignore[arg-type]
    assert target.read_bytes() == b"complete"
    assert list((checkpoints / ".staging").glob("*.zip")) == []


def _champion(path: Path, identifier: str) -> Champion:
    path.write_bytes(identifier.encode())
    return Champion.from_checkpoint(
        champion_id=identifier,
        checkpoint=path,
        observation_schema="observation",
        action_schema="action",
        model_family="family",
        learner_steps=1,
        promoted_at="now",
    )


def test_champion_registry_sampling_is_explicit_deduplicated_and_file_count_independent(
    tmp_path: Path,
) -> None:
    global_champion = _champion(tmp_path / "global.zip", "global")
    checkpoint = _champion(tmp_path / "checkpoint.zip", "checkpoint")
    hall = _champion(tmp_path / "hall.zip", "hall")
    registry_path = tmp_path / "registry.json"
    ChampionRegistry(global_champion, checkpoint, [hall, global_champion]).save(registry_path)
    manager = OpponentManager(
        [f"registry:{registry_path}"],
        device="cpu",
        deterministic=True,
        category_weights={"checkpoint": 0.5, "global": 0.4, "hall": 0.1},
    )
    before_rng = Random(44)
    before = [manager.select(before_rng, mode="champion-registry") for _ in range(1_000)]
    for index in range(50):
        (tmp_path / f"ordinary-{index}.zip").write_bytes(b"ordinary")
    after_rng = Random(44)
    after = [manager.select(after_rng, mode="champion-registry") for _ in range(1_000)]
    assert before == after
    counts = Counter(selection.category for selection in before)
    assert set(counts) == {"checkpoint", "global", "hall"}
    assert abs(counts["checkpoint"] - 500) < 80
    assert abs(counts["global"] - 400) < 80
    assert abs(counts["hall"] - 100) < 60
