"""Train the policy-v2 network: which move to make, learned from the search.

``balance-lab -label`` records real decisions with every option's measured
value (many futures per option, the same futures for every option), and
``cmd/decision-data`` turns them into arrays: each decision's value-v2 state
inputs and each option's inputs (what the move does, never IDs). This module
trains a network that scores options, and judges it the way the search is
judged: by the value a decision loses when it plays the network's pick
instead of the best option.

The network has a state trunk, an option scorer (the trunk's output and the
option's inputs give the option's value relative to the greedy player's
pick) and two state heads (the decision's value and the battle's result).
It is exported as ``dice-and-destiny-policy-v2`` JSON for the Go player
(``internal/battle/valuenet``), with the input scaling folded into the first
layers and check rows the Go loader must reproduce.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
from dataclasses import asdict, dataclass
from pathlib import Path

import numpy as np
import torch
from torch import nn

MODEL_FORMAT = "dice-and-destiny-policy-v2"
DATA_FORMAT = "dice-and-destiny-decisions-v1"


@dataclass
class Data:
    """Decisions and their options; options are contiguous per decision."""

    header: dict
    states: np.ndarray  # (decisions, state width)
    options: np.ndarray  # (options, option width)
    targets: np.ndarray  # (options, 5): value vs best, SE, futures, mean, greedy pick
    owner: np.ndarray  # (options,) decision row
    start: np.ndarray  # (decisions,) first option row
    count: np.ndarray  # (decisions,) options
    anchor: np.ndarray  # (decisions,) greedy's option (local index)
    meta: list[dict]

    @property
    def decisions(self) -> int:
        return len(self.states)


def load(path: Path) -> Data:
    header = json.loads((path / "header.json").read_text())
    if header.get("format") != DATA_FORMAT:
        raise ValueError(f"{path}: not a {DATA_FORMAT} directory")
    ws, wo = len(header["state_features"]), len(header["option_features"])
    states = np.fromfile(path / "states.f32", dtype="<f4").reshape(-1, ws)
    options = np.fromfile(path / "options.f32", dtype="<f4").reshape(-1, wo)
    targets = np.fromfile(path / "targets.f32", dtype="<f4").reshape(-1, 5)
    owner = np.fromfile(path / "option_decision.i32", dtype="<i4")
    meta = header["decisions"]
    count = np.array([m["options"] for m in meta], dtype=np.int64)
    start = np.concatenate([[0], np.cumsum(count)[:-1]]).astype(np.int64)
    anchor = np.array([m["anchor"] for m in meta], dtype=np.int64)
    if len(states) != len(meta) or len(options) != count.sum() or len(owner) != len(options):
        raise ValueError(f"{path}: arrays disagree with the header")
    return Data(header, states, options, targets, owner, start, count, anchor, meta)


def concat(parts: list[Data]) -> Data:
    first = parts[0]
    for p in parts[1:]:
        if p.header["state_schema_hash"] != first.header["state_schema_hash"] or p.header["option_schema_hash"] != first.header["option_schema_hash"]:
            raise ValueError("datasets were encoded with different schemas")
    offset, owners = 0, []
    for p in parts:
        owners.append(p.owner + offset)
        offset += p.decisions
    count = np.concatenate([p.count for p in parts])
    return Data(
        header=first.header,
        states=np.concatenate([p.states for p in parts]),
        options=np.concatenate([p.options for p in parts]),
        targets=np.concatenate([p.targets for p in parts]),
        owner=np.concatenate(owners),
        start=np.concatenate([[0], np.cumsum(count)[:-1]]).astype(np.int64),
        count=count,
        anchor=np.concatenate([p.anchor for p in parts]),
        meta=[m for p in parts for m in p.meta],
    )


def subset(data: Data, rows: np.ndarray) -> Data:
    rows = np.asarray(rows, dtype=np.int64)
    option_rows = np.concatenate([np.arange(data.start[d], data.start[d] + data.count[d]) for d in rows]) if len(rows) else np.zeros(0, dtype=np.int64)
    count = data.count[rows]
    owner = np.repeat(np.arange(len(rows)), count)
    return Data(
        header=data.header,
        states=data.states[rows],
        options=data.options[option_rows],
        targets=data.targets[option_rows],
        owner=owner,
        start=np.concatenate([[0], np.cumsum(count)[:-1]]).astype(np.int64),
        count=count,
        anchor=data.anchor[rows],
        meta=[data.meta[d] for d in rows],
    )


def battle_key(m: dict) -> str:
    return f"{m['hero']}|{m['opponent']}|{m['seed']}|{m['seat']}"


def split(data: Data, validation: float, seed: int, held_out: set[str]) -> tuple[Data, Data, Data]:
    """Train and validation split by battle; decisions of held-out characters apart."""
    rng = np.random.default_rng(seed)
    keys = sorted({battle_key(m) for m in data.meta})
    rng.shuffle(keys)
    valid_keys = set(keys[: int(len(keys) * validation)])
    train, valid, out = [], [], []
    for d, m in enumerate(data.meta):
        if m["hero"] in held_out or m["opponent"] in held_out:
            out.append(d)
        elif battle_key(m) in valid_keys:
            valid.append(d)
        else:
            train.append(d)
    return subset(data, np.array(train)), subset(data, np.array(valid)), subset(data, np.array(out))


class PolicyNet(nn.Module):
    """State trunk, option scorer, and the value and result heads."""

    def __init__(self, state_width: int, option_width: int, trunk: int, scorer: list[int]):
        super().__init__()
        self.trunk = nn.ModuleList([nn.Linear(state_width, trunk), nn.Linear(trunk, trunk)])
        layers, width = [], trunk + option_width
        for size in scorer:
            layers.append(nn.Linear(width, size))
            width = size
        self.scorer = nn.ModuleList(layers)
        self.score = nn.Linear(width, 1)
        self.value = nn.Linear(trunk, 1)
        self.result = nn.Linear(trunk, 1)

    def state(self, s: torch.Tensor) -> torch.Tensor:
        for layer in self.trunk:
            s = nn.functional.gelu(layer(s))
        return s

    def options(self, h: torch.Tensor, o: torch.Tensor) -> torch.Tensor:
        x = torch.cat([h, o], dim=-1)
        for layer in self.scorer:
            x = nn.functional.gelu(layer(x))
        return self.score(x).squeeze(-1)

    def heads(self, h: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return torch.tanh(self.value(h).squeeze(-1)) * 1.2, self.result(h).squeeze(-1)


@dataclass
class Settings:
    trunk: int = 128
    scorer: tuple[int, ...] = (128, 64)
    learning_rate: float = 1e-3
    weight_decay: float = 1e-5
    batch: int = 256
    epochs: int = 60
    patience: int = 8
    validation: float = 0.15
    noise_floor: float = 0.02
    value_weight: float = 0.5
    result_weight: float = 0.2
    seed: int = 7


class Scaler:
    def __init__(self, x: np.ndarray):
        self.mean = x.mean(axis=0).astype(np.float32)
        std = x.std(axis=0).astype(np.float32)
        self.std = np.where(std < 1e-3, 1.0, std).astype(np.float32)

    def __call__(self, x: np.ndarray) -> torch.Tensor:
        return torch.from_numpy(((x - self.mean) / self.std).astype(np.float32))


def tensors(data: Data, ss: Scaler, os_: Scaler) -> dict:
    t = data.targets
    anchor_row = data.start + data.anchor
    best_mean = np.array([t[data.start[d]: data.start[d] + data.count[d], 3][np.argmax(t[data.start[d]: data.start[d] + data.count[d], 0])] for d in range(data.decisions)], dtype=np.float32)
    result = np.array([m["result"] for m in data.meta], dtype=np.float32)
    return {
        "s": ss(data.states),
        "o": os_(data.options),
        "y": torch.from_numpy(t[:, 0].copy()),
        "se": torch.from_numpy(t[:, 1].copy()),
        "owner": torch.from_numpy(data.owner.astype(np.int64)),
        "anchor_row": torch.from_numpy(anchor_row.astype(np.int64)),
        "best_mean": torch.from_numpy(best_mean),
        "result": torch.from_numpy(result),
    }


def scores(model: PolicyNet, data: Data, ss: Scaler, os_: Scaler) -> np.ndarray:
    """The network's score for every option."""
    model.eval()
    with torch.no_grad():
        h = model.state(ss(data.states))
        q = model.options(h[torch.from_numpy(data.owner.astype(np.int64))], os_(data.options))
    return q.numpy()


def evaluate(data: Data, q: np.ndarray, margin: float = 0.0) -> dict:
    """Value lost per decision by the network's pick (deviating from greedy
    only when its score beats greedy's by `margin`), by greedy's pick and
    by the best option, overall and by kind."""
    out: dict[str, dict] = {}
    for d in range(data.decisions):
        lo, n = data.start[d], data.count[d]
        y = data.targets[lo: lo + n, 0]
        scores_ = q[lo: lo + n]
        a = data.anchor[d]
        pick = int(np.argmax(scores_))
        if scores_[pick] - scores_[a] <= margin:
            pick = a
        for kind in ("all", data.meta[d]["kind"]):
            row = out.setdefault(kind, {"decisions": 0, "greedy": 0.0, "network": 0.0, "changed": 0, "better": 0, "worse": 0})
            row["decisions"] += 1
            row["greedy"] += float(-y[a])
            row["network"] += float(-y[pick])
            if pick != a:
                row["changed"] += 1
                if y[pick] > y[a]:
                    row["better"] += 1
                elif y[pick] < y[a]:
                    row["worse"] += 1
    for row in out.values():
        row["greedy"] /= row["decisions"]
        row["network"] /= row["decisions"]
    return out


def train(train_data: Data, valid_data: Data, settings: Settings, log=print):
    torch.manual_seed(settings.seed)
    ss, os_ = Scaler(train_data.states), Scaler(train_data.options)
    model = PolicyNet(train_data.states.shape[1], train_data.options.shape[1], settings.trunk, list(settings.scorer))
    optimizer = torch.optim.Adam(model.parameters(), lr=settings.learning_rate, weight_decay=settings.weight_decay)
    tr = tensors(train_data, ss, os_)
    generator = torch.Generator().manual_seed(settings.seed)
    best, best_state, stale, history = math.inf, None, 0, []
    starts = torch.from_numpy(train_data.start)
    counts = torch.from_numpy(train_data.count)
    anchors = torch.from_numpy(train_data.anchor)
    for epoch in range(settings.epochs):
        model.train()
        order = torch.randperm(train_data.decisions, generator=generator)
        total = 0.0
        for b0 in range(0, train_data.decisions, settings.batch):
            dec = order[b0: b0 + settings.batch]
            rows = torch.cat([torch.arange(starts[d], starts[d] + counts[d]) for d in dec.tolist()])
            local = torch.repeat_interleave(torch.arange(len(dec)), counts[dec])
            h = model.state(tr["s"][dec])
            q = model.options(h[local], tr["o"][rows])
            # Each option's score relative to greedy's pick in its decision,
            # against the measured difference, weighted by its precision.
            cnt = counts[dec]
            anchor_local = torch.cumsum(cnt, 0) - cnt + anchors[dec]
            qa = q[anchor_local][local]
            ya = tr["y"][tr["anchor_row"][dec]][local]
            se = tr["se"][rows]
            w = 1.0 / (se * se + settings.noise_floor ** 2)
            w = w / w.mean()
            policy = torch.mean(w * ((q - qa) - (tr["y"][rows] - ya)) ** 2)
            v, r = model.heads(h)
            value = torch.mean((v - tr["best_mean"][dec]) ** 2)
            result = nn.functional.binary_cross_entropy_with_logits(r, tr["result"][dec])
            loss = policy + settings.value_weight * value + settings.result_weight * result
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            total += float(policy.detach()) * len(dec)
        q = scores(model, valid_data, ss, os_)
        ev = evaluate(valid_data, q)["all"]
        history.append({"epoch": epoch + 1, "train_policy_loss": total / train_data.decisions, "valid_network_lost": ev["network"], "valid_greedy_lost": ev["greedy"]})
        log(f"epoch {epoch + 1}: train {total / train_data.decisions:.5f}, validation lost per decision: network {ev['network']:.4f}, greedy {ev['greedy']:.4f} (changed {ev['changed']}, better {ev['better']}, worse {ev['worse']})")
        if ev["network"] < best - 1e-6:
            best, stale = ev["network"], 0
            best_state = {k: v.detach().clone() for k, v in model.state_dict().items()}
        else:
            stale += 1
            if stale >= settings.patience:
                break
    if best_state is not None:
        model.load_state_dict(best_state)
    return model, ss, os_, history


def float32_list(values) -> list[float]:
    return [float(str(np.float32(v))) for v in values]


def tensor(value: np.ndarray) -> dict:
    return {"shape": list(value.shape), "values": float32_list(value.reshape(-1))}


def export(model: PolicyNet, ss: Scaler, os_: Scaler, header: dict, check: Data, extra: dict) -> dict:
    """Tensors with input scaling folded into the first layers."""
    tensors_ = {}
    for i, layer in enumerate(model.trunk):
        w = layer.weight.detach().numpy().astype(np.float64)
        b = layer.bias.detach().numpy().astype(np.float64)
        if i == 0:
            w = w / ss.std.astype(np.float64)
            b = b - w @ ss.mean.astype(np.float64)
        tensors_[f"trunk.{i}.weight"] = tensor(w.astype(np.float32))
        tensors_[f"trunk.{i}.bias"] = tensor(b.astype(np.float32))
    trunk_width = model.trunk[-1].out_features
    for i, layer in enumerate(model.scorer):
        w = layer.weight.detach().numpy().astype(np.float64)
        b = layer.bias.detach().numpy().astype(np.float64)
        if i == 0:
            wo = w[:, trunk_width:] / os_.std.astype(np.float64)
            b = b - wo @ os_.mean.astype(np.float64)
            w = np.concatenate([w[:, :trunk_width], wo], axis=1)
        tensors_[f"scorer.{i}.weight"] = tensor(w.astype(np.float32))
        tensors_[f"scorer.{i}.bias"] = tensor(b.astype(np.float32))
    for head in ("score", "value", "result"):
        layer = getattr(model, head)
        tensors_[f"{head}.weight"] = tensor(layer.weight.detach().numpy())
        tensors_[f"{head}.bias"] = tensor(layer.bias.detach().numpy())
    # Check rows: a few decisions' inputs and the scores the Go loader must
    # reproduce.
    rows = []
    q = scores(model, check, ss, os_)
    model.eval()
    with torch.no_grad():
        v, r = model.heads(model.state(ss(check.states)))
    for d in range(min(4, check.decisions)):
        lo, n = check.start[d], check.count[d]
        rows.append({
            "state": float32_list(check.states[d]),
            "options": [float32_list(o) for o in check.options[lo: lo + n]],
            "scores": [float(x) for x in q[lo: lo + n]],
            "value": float(v[d]),
            "result": float(torch.sigmoid(r[d])),
        })
    return {
        "format": MODEL_FORMAT,
        "state_schema": header["state_schema"],
        "state_schema_hash": header["state_schema_hash"],
        "state_width": len(header["state_features"]),
        "option_schema": header["option_schema"],
        "option_schema_hash": header["option_schema_hash"],
        "option_width": len(header["option_features"]),
        "architecture": {"trunk": [model.trunk[0].out_features, model.trunk[1].out_features], "scorer": [l.out_features for l in model.scorer], "activation": "gelu", "value_scale": 1.2},
        "tensors": tensors_,
        "check": rows,
        **extra,
    }


def report(name: str, ev: dict) -> str:
    lines = [f"{name}", "| Kind | Decisions | Greedy lost | Network lost | Changed | Better | Worse |", "|---|---|---|---|---|---|---|"]
    for kind in ["all"] + sorted(k for k in ev if k != "all"):
        r = ev[kind]
        lines.append(f"| {kind} | {r['decisions']} | {r['greedy']:.4f} | {r['network']:.4f} | {r['changed']} | {r['better']} | {r['worse']} |")
    return "\n".join(lines)


def run(args: argparse.Namespace) -> dict:
    data = concat([load(Path(p)) for p in args.data])
    held = set(args.held_out.split(",")) if args.held_out else set()
    train_data, valid_data, out_data = split(data, args.validation, args.seed, held)
    print(f"{train_data.decisions} training decisions, {valid_data.decisions} validation, {out_data.decisions} held out ({', '.join(sorted(held)) or 'none'})")
    settings = Settings(trunk=args.trunk, scorer=tuple(int(x) for x in args.scorer.split(",")), learning_rate=args.learning_rate, weight_decay=args.weight_decay,
                        epochs=args.epochs, patience=args.patience, validation=args.validation, noise_floor=args.noise_floor, seed=args.seed)
    model, ss, os_, history = train(train_data, valid_data, settings)
    results = {"settings": asdict(settings), "history": history}
    # The margin a pick must beat greedy's by is chosen on validation.
    margin, margins = 0.0, {}
    if valid_data.decisions:
        q = scores(model, valid_data, ss, os_)
        for m in (0.0, 0.0025, 0.005, 0.01, 0.02, 0.04, 0.08):
            margins[m] = evaluate(valid_data, q, m)["all"]["network"]
        margin = min(margins, key=lambda m: (margins[m], m))
        print("validation value lost by margin: " + ", ".join(f"{m:g}: {v:.4f}" for m, v in margins.items()) + f" -> margin {margin:g}")
    results["margin"] = margin
    for name, part in (("validation", valid_data), ("held out", out_data)):
        if part.decisions:
            results[name] = evaluate(part, scores(model, part, ss, os_), margin)
            print(report(f"{name} (margin {margin:g})", results[name]))
    for bench in args.benchmark or []:
        b = load(Path(bench))
        q = scores(model, b, ss, os_)
        results[f"benchmark {bench}"] = evaluate(b, q, margin)
        print(report(f"benchmark {bench} (margin {margin:g})", results[f"benchmark {bench}"]))
        if margin != 0:
            print(f"(at margin 0: network {evaluate(b, q)['all']['network']:.4f})")
    if args.output:
        payload = export(model, ss, os_, data.header, valid_data if valid_data.decisions else train_data,
                         {"margin": margin, "metrics": {k: v for k, v in results.items() if k != "history"}, "held_out": sorted(held), "data": [str(p) for p in args.data]})
        raw = json.dumps(payload).encode()
        Path(args.output).write_bytes(raw)
        print(f"wrote {args.output} (sha256 {hashlib.sha256(raw).hexdigest()}, {len(raw) // 1024} KB)")
    return results


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--data", nargs="+", required=True, help="decision-data directories")
    parser.add_argument("--benchmark", nargs="*", help="decision-data directories to score after training (never trained on)")
    parser.add_argument("--held-out", default="", help="comma-separated characters kept out of training and validation")
    parser.add_argument("--output", default="", help="model JSON path")
    parser.add_argument("--trunk", type=int, default=128)
    parser.add_argument("--scorer", default="128,64")
    parser.add_argument("--learning-rate", type=float, default=1e-3)
    parser.add_argument("--weight-decay", type=float, default=1e-5)
    parser.add_argument("--epochs", type=int, default=60)
    parser.add_argument("--patience", type=int, default=8)
    parser.add_argument("--validation", type=float, default=0.15)
    parser.add_argument("--noise-floor", type=float, default=0.02)
    parser.add_argument("--seed", type=int, default=7)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    add_arguments(parser)
    run(parser.parse_args(argv))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
