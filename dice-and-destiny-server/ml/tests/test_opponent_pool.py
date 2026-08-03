from __future__ import annotations

from pathlib import Path
from random import Random

from dice_destiny_ml import opponent_pool
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


def test_flat_manifest_preserves_legacy_expansion_and_cache_classes(
    tmp_path: Path, monkeypatch
) -> None:
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
    categories = [
        manager.select(balanced_rng, mode="category-balanced").category for _ in range(400)
    ]
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
