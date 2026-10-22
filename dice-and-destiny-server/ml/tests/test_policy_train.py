import json

import numpy as np
import pytest

from dice_destiny_ml import policy_train

STATE = ["round", "own.health", "foe.health"]
OPTION = ["option.greedy_pick", "option.gain", "option.noise"]


def write_decisions(path, hero, opponent, decisions, seed):
    """Decisions in decision-data's layout. The best option is the one with
    the highest hidden gain; greedy picks the first option."""
    rng = np.random.default_rng(seed)
    states, options, targets, owner, meta = [], [], [], [], []
    for d in range(decisions):
        n = int(rng.integers(2, 5))
        gains = rng.normal(size=n)
        best = int(np.argmax(gains))
        states.append([d % 5 / 5, rng.random(), rng.random()])
        for k in range(n):
            options.append([1.0 if k == 0 else 0.0, gains[k], rng.random()])
            targets.append([gains[k] - gains[best], 0.01, 128, gains[k], 1.0 if k == 0 else 0.0])
            owner.append(d)
        meta.append({"file": "x", "hero": hero, "opponent": opponent, "seed": d // 10, "seat": "seat-a", "step": d, "round": 1,
                     "kind": "offensive/planning:dice" if d % 2 else "defensive/defense_selection:ability", "result": float(d % 2),
                     "margin": 0.0, "options": n, "best": best, "anchor": 0, "played": 0})
    path.mkdir(parents=True)
    np.asarray(states, dtype="<f4").tofile(path / "states.f32")
    np.asarray(options, dtype="<f4").tofile(path / "options.f32")
    np.asarray(targets, dtype="<f4").tofile(path / "targets.f32")
    np.asarray(owner, dtype="<i4").tofile(path / "option_decision.i32")
    header = {"format": policy_train.DATA_FORMAT, "state_schema": "value-v2", "state_schema_hash": "s", "state_features": STATE,
              "option_schema": "option-v1", "option_schema_hash": "o", "option_features": OPTION,
              "targets": ["value_vs_best", "standard_error", "futures", "mean", "greedy_pick"], "decisions": meta}
    (path / "header.json").write_text(json.dumps(header))


@pytest.fixture
def data(tmp_path):
    write_decisions(tmp_path / "a", "starter", "starter", 400, 1)
    write_decisions(tmp_path / "b", "venom", "bell", 60, 2)
    return policy_train.concat([policy_train.load(tmp_path / "a"), policy_train.load(tmp_path / "b")])


def test_evaluate_counts_value_lost():
    q = np.array([0.0, 1.0, 0.5, 0.2])
    targets = np.array([[-0.3, 0, 0, 0, 1], [0.0, 0, 0, 0, 0], [-0.1, 0, 0, 0, 0], [0.0, 0, 0, 0, 1]], dtype=np.float32)
    data = policy_train.Data(header={}, states=np.zeros((2, 1), np.float32), options=np.zeros((4, 1), np.float32), targets=targets,
                             owner=np.array([0, 0, 0, 1]), start=np.array([0, 3]), count=np.array([3, 1]), anchor=np.array([0, 0]),
                             meta=[{"kind": "k"}, {"kind": "k"}])
    ev = policy_train.evaluate(data, q)["all"]
    assert ev["decisions"] == 2
    assert ev["greedy"] == pytest.approx(0.15)
    assert ev["network"] == pytest.approx(0.0)
    assert (ev["changed"], ev["better"], ev["worse"]) == (1, 1, 0)
    # A margin above the network's lead keeps greedy's pick.
    assert policy_train.evaluate(data, q, margin=2.0)["all"]["network"] == pytest.approx(0.15)


def test_training_learns_the_gain_and_exports_check_rows(data, tmp_path):
    train, valid, held = policy_train.split(data, 0.2, 3, {"venom"})
    assert held.decisions == 60 and all(m["hero"] == "venom" for m in held.meta)
    assert not {policy_train.battle_key(m) for m in train.meta} & {policy_train.battle_key(m) for m in valid.meta}
    model, ss, os_, history = policy_train.train(train, valid, policy_train.Settings(trunk=16, scorer=(16,), epochs=30, patience=30), log=lambda _: None)
    ev = policy_train.evaluate(held, policy_train.scores(model, held, ss, os_))["all"]
    assert ev["network"] < 0.5 * ev["greedy"]
    payload = policy_train.export(model, ss, os_, data.header, valid, {})
    assert payload["format"] == policy_train.MODEL_FORMAT and len(payload["check"]) == 4
    # Folding the input scaling into the first layers keeps the scores.
    check = payload["check"][0]
    w = payload["tensors"]
    def layer(name, x):
        weight = np.array(w[name + ".weight"]["values"]).reshape(w[name + ".weight"]["shape"])
        return weight @ x + np.array(w[name + ".bias"]["values"])
    gelu = lambda v: 0.5 * v * (1 + np.vectorize(__import__("math").erf)(v / np.sqrt(2)))
    h = gelu(layer("trunk.1", gelu(layer("trunk.0", np.array(check["state"])))))
    for option, score in zip(check["options"], check["scores"]):
        x = gelu(layer("scorer.0", np.concatenate([h, option])))
        assert float(layer("score", x)[0]) == pytest.approx(score, abs=1e-4)
