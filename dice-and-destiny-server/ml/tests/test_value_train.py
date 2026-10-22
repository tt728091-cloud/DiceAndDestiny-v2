import hashlib
import json

import numpy as np
import pytest
import torch

from dice_destiny_ml import value_train

FEATURES = ["round", "own.health", "foe.health", "own.energy"]
META = ["battle", "seed", "round", "viewer_seat_b", "viewer_is_hero", "player", "result", "margin"]


def write_dataset(path, hero, opponent, battles, seed):
    """A dataset in the recorder's layout: two seats per round, results by health lead."""
    rng = np.random.default_rng(seed)
    rows = []
    for battle in range(battles):
        lead = rng.normal()
        for round_ in range(1, 4):
            own = np.clip(0.5 + 0.2 * lead + 0.05 * rng.normal(), 0, 1)
            foe = np.clip(0.5 - 0.2 * lead + 0.05 * rng.normal(), 0, 1)
            win = 1.0 if lead > 0 else 0.0
            for viewer, (a, b, result) in enumerate(((own, foe, win), (foe, own, 1 - win))):
                features = [round_ / 12, a, b, rng.random()]
                meta = [battle, battle + 1, round_, viewer, 1 - viewer, 0, result, (a - b) / 2]
                rows.append(features + meta)
    data = np.asarray(rows, dtype="<f4").tobytes()
    path.mkdir(parents=True)
    (path / "shard-0000.f32").write_bytes(data)
    header = {
        "format": value_train.DATASET_FORMAT,
        "schema": value_train.SCHEMA,
        "schema_hash": "test-schema",
        "features": FEATURES,
        "meta_columns": META,
        "hero": hero,
        "opponent": opponent,
        "rows": len(rows),
        "content_hashes": ["content-a"],
        "shards": [{"file": "shard-0000.f32", "rows": len(rows), "sha256": hashlib.sha256(data).hexdigest()}],
    }
    (path / "header.json").write_text(json.dumps(header))


@pytest.fixture
def datasets(tmp_path):
    write_dataset(tmp_path / "starter-vs-brine", "starter", "brine", 120, 1)
    write_dataset(tmp_path / "curse-vs-starter", "curse", "starter", 120, 2)
    write_dataset(tmp_path / "venom-vs-bell", "venom", "bell", 60, 3)
    write_dataset(tmp_path / "starter-vs-bell", "starter", "bell", 60, 4)
    return tmp_path


def test_split_keeps_battles_whole(datasets):
    loaded = [
        value_train.load_dataset(datasets / "starter-vs-brine"),
        value_train.load_dataset(datasets / "curse-vs-starter"),
    ]
    train, valid = value_train.battle_split(loaded, 0.25, seed=5)
    assert len(train.result) + len(valid.result) == sum(len(d.features) for d in loaded)
    assert not set(train.battles) & set(valid.battles)
    assert len(valid.result) > 0


def test_shard_hash_is_checked(datasets):
    shard = datasets / "starter-vs-brine" / "shard-0000.f32"
    data = bytearray(shard.read_bytes())
    data[0] ^= 1
    shard.write_bytes(bytes(data))
    with pytest.raises(ValueError, match="SHA-256"):
        value_train.load_dataset(datasets / "starter-vs-brine")


def test_train_export_and_baselines(datasets, tmp_path):
    output = tmp_path / "model.json"
    args = value_train.argparse.Namespace(
        data=[str(datasets / "*")],
        held_out_hero="venom",
        held_out_opponent="bell",
        output=str(output),
        mix_only=False,
        hidden=16,
        depth=2,
        activation="gelu",
        margin_weight=1.0,
        learning_rate=3e-3,
        epochs=30,
        batch=64,
        seed=3,
    )
    export = value_train.run(args)
    saved = json.loads(output.read_text())
    assert saved["format"] == value_train.MODEL_FORMAT and saved["feature_count"] == len(FEATURES)
    held = {d["name"]: d["held_out"] for d in saved["datasets"]}
    assert held == {
        "curse-vs-starter": False,
        "starter-vs-bell": True,
        "starter-vs-brine": False,
        "venom-vs-bell": True,
    }
    metrics = saved["metrics"]
    assert metrics["held_out"]["network"]["rows"] == 2 * 3 * 2 * 60
    assert (
        sum(b["count"] for b in metrics["held_out"]["network"]["calibration"])
        == metrics["held_out"]["network"]["rows"]
    )
    # The health lead decides these battles, so every predictor beats a coin.
    for name in ("network", "logistic_health_margin", "tanh_health_margin"):
        assert metrics["held_out"][name]["log_loss"] < np.log(2)
    # The exported tensors (standardization folded in) reproduce torch.
    tensors = export["tensors"]

    def layer(name, x):
        w = np.asarray(tensors[name + ".weight"]["values"]).reshape(tensors[name + ".weight"]["shape"])
        return w @ x + np.asarray(tensors[name + ".bias"]["values"])

    for check in saved["check"]:
        h = np.asarray(check["input"], dtype=np.float64)
        for i in range(2):
            h = torch.nn.functional.gelu(torch.from_numpy(layer(f"hidden.{i}", h))).numpy()
        win = 1 / (1 + np.exp(-layer("win", h)[0]))
        assert abs(win - check["win"]) < 1e-4
        assert abs(np.tanh(layer("margin", h)[0]) - check["margin"]) < 1e-4


def test_calibration_bins_cover_everything():
    p = np.array([0.0, 0.05, 0.1, 0.55, 0.999, 1.0])
    y = np.array([0, 0, 1, 1, 1, 1], dtype=float)
    table = value_train.calibration(p, y)
    assert len(table) == 10 and sum(row["count"] for row in table) == len(p)
    assert table[-1]["count"] == 2 and table[0]["count"] == 2
