"""Train and check the value-v1 network offline.

The balance lab records positions (``balance-lab -record``): the value-v1
inputs of each seat at the start of each round's offensive planning, labelled
with that seat's battle result and final health margin. This module trains a
small MLP on them, checks it against two baselines on a held-out hero and
opponent, and exports it as ``dice-and-destiny-value-v1`` JSON for the Go
search (``internal/battle/valuenet``).

Rows never leave their battle: train and validation are split by battle, and
every battle that involves the held-out hero or opponent (in either seat) is
kept out of both.
"""

from __future__ import annotations

import argparse
import glob
import hashlib
import json
import math
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path

import numpy as np
import torch
from torch import nn

MODEL_FORMAT = "dice-and-destiny-value-v1"
DATASET_FORMAT = "dice-and-destiny-value-dataset-v1"
SCHEMA = "value-v1"
EPSILON = 1e-6


@dataclass
class Dataset:
    name: str
    path: Path
    header: dict
    header_sha256: str
    features: np.ndarray  # (rows, inputs) float32
    meta: np.ndarray  # (rows, meta columns) float32

    def column(self, name: str) -> np.ndarray:
        return self.meta[:, self.header["meta_columns"].index(name)]

    def involves(self, names: set[str]) -> bool:
        return self.header["hero"] in names or self.header["opponent"] in names


def load_dataset(path: Path) -> Dataset:
    """Read one recorded dataset, checking its format and shard hashes."""
    raw = (path / "header.json").read_bytes()
    header = json.loads(raw)
    if header.get("format") != DATASET_FORMAT or header.get("schema") != SCHEMA:
        raise ValueError(f"{path}: not a {SCHEMA} dataset")
    width = len(header["features"]) + len(header["meta_columns"])
    parts = []
    for shard in header["shards"]:
        data = (path / shard["file"]).read_bytes()
        if hashlib.sha256(data).hexdigest() != shard["sha256"]:
            raise ValueError(f"{path / shard['file']}: SHA-256 mismatch")
        parts.append(np.frombuffer(data, dtype="<f4").reshape(-1, width))
    rows = np.concatenate(parts) if parts else np.zeros((0, width), dtype=np.float32)
    if len(rows) != header["rows"]:
        raise ValueError(f"{path}: {len(rows)} rows, header says {header['rows']}")
    inputs = len(header["features"])
    return Dataset(
        name=path.name,
        path=path,
        header=header,
        header_sha256=hashlib.sha256(raw).hexdigest(),
        features=np.ascontiguousarray(rows[:, :inputs], dtype=np.float32),
        meta=np.ascontiguousarray(rows[:, inputs:], dtype=np.float32),
    )


@dataclass
class Split:
    x: np.ndarray
    result: np.ndarray
    margin: np.ndarray
    battles: np.ndarray  # one id per row, unique across datasets
    datasets: list[str] = field(default_factory=list)


def join(datasets: list[Dataset], select=None) -> Split:
    """Stack datasets (optionally only some battles), numbering battles uniquely."""
    xs, results, margins, battles, names = [], [], [], [], []
    offset = 0
    for d in datasets:
        battle = d.column("battle").astype(np.int64)
        keep = np.ones(len(battle), dtype=bool) if select is None else select(d, battle)
        xs.append(d.features[keep])
        results.append(d.column("result")[keep])
        margins.append(d.column("margin")[keep])
        battles.append(battle[keep] + offset)
        offset += int(battle.max()) + 1 if len(battle) else 0
        names.append(d.name)
    empty = np.zeros((0,), dtype=np.float32)
    return Split(
        x=np.concatenate(xs) if xs else np.zeros((0, 0), dtype=np.float32),
        result=np.concatenate(results) if results else empty,
        margin=np.concatenate(margins) if margins else empty,
        battles=np.concatenate(battles) if battles else empty.astype(np.int64),
        datasets=names,
    )


def battle_split(datasets: list[Dataset], validation: float, seed: int) -> tuple[Split, Split]:
    """Split by battle: a battle's rows all go to train or all to validation."""
    rng = np.random.default_rng(seed)
    held: dict[str, np.ndarray] = {}
    for d in datasets:
        battles = np.unique(d.column("battle").astype(np.int64))
        held[d.name] = battles[rng.random(len(battles)) < validation]
    train = join(datasets, lambda d, b: ~np.isin(b, held[d.name]))
    valid = join(datasets, lambda d, b: np.isin(b, held[d.name]))
    return train, valid


class ValueNet(nn.Module):
    """Two hidden layers and two heads: win probability and final margin."""

    def __init__(self, inputs: int, hidden: int, depth: int, activation: str):
        super().__init__()
        self.activation = activation
        self.hidden = nn.ModuleList()
        width = inputs
        for _ in range(depth):
            self.hidden.append(nn.Linear(width, hidden))
            width = hidden
        self.win = nn.Linear(width, 1)
        self.margin = nn.Linear(width, 1)

    def act(self, x: torch.Tensor) -> torch.Tensor:
        if self.activation == "relu":
            return torch.relu(x)
        return torch.nn.functional.gelu(x)

    def forward(self, x: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        for layer in self.hidden:
            x = self.act(layer(x))
        return self.win(x).squeeze(-1), torch.tanh(self.margin(x).squeeze(-1))


@dataclass
class Settings:
    hidden: int = 128
    depth: int = 2
    activation: str = "gelu"
    margin_weight: float = 1.0
    learning_rate: float = 1e-3
    weight_decay: float = 1e-5
    batch: int = 1024
    epochs: int = 40
    patience: int = 5
    validation: float = 0.1
    seed: int = 11


def log_loss(p: np.ndarray, y: np.ndarray) -> float:
    p = np.clip(p, EPSILON, 1 - EPSILON)
    return float(np.mean(-(y * np.log(p) + (1 - y) * np.log(1 - p))))


def brier(p: np.ndarray, y: np.ndarray) -> float:
    return float(np.mean((p - y) ** 2))


def calibration(p: np.ndarray, y: np.ndarray, bins: int = 10) -> list[dict]:
    """Rows per predicted-probability decile: count, mean prediction, mean outcome."""
    table = []
    edges = np.linspace(0, 1, bins + 1)
    for i in range(bins):
        upper = p <= edges[i + 1] if i == bins - 1 else p < edges[i + 1]
        mask = (p >= edges[i]) & upper
        n = int(mask.sum())
        table.append(
            {
                "bin": f"{edges[i]:.1f}-{edges[i + 1]:.1f}",
                "count": n,
                "mean_predicted": float(p[mask].mean()) if n else None,
                "mean_outcome": float(y[mask].mean()) if n else None,
            }
        )
    return table


def scores(p: np.ndarray, y: np.ndarray) -> dict:
    return {
        "rows": int(len(y)),
        "log_loss": log_loss(p, y),
        "brier": brier(p, y),
        "calibration": calibration(p, y),
    }


def health_margin(split: Split, features: list[str]) -> np.ndarray:
    """The health race today's evaluator scores: own health share minus the foe's."""
    return split.x[:, features.index("own.health")] - split.x[:, features.index("foe.health")]


def fit_logistic(m: np.ndarray, y: np.ndarray, iterations: int = 50) -> tuple[float, float]:
    """Newton's method for p = sigmoid(a*m + b) on soft labels."""
    a, b = 0.0, 0.0
    for _ in range(iterations):
        p = 1 / (1 + np.exp(-(a * m + b)))
        w = np.maximum(p * (1 - p), 1e-9)
        g = np.array([np.sum((p - y) * m), np.sum(p - y)])
        h = np.array([[np.sum(w * m * m), np.sum(w * m)], [np.sum(w * m), np.sum(w)]])
        step = np.linalg.solve(h + 1e-9 * np.eye(2), g)
        a, b = a - step[0], b - step[1]
        if np.max(np.abs(step)) < 1e-10:
            break
    return float(a), float(b)


def predict(
    model: ValueNet, x: np.ndarray, mean: np.ndarray, std: np.ndarray
) -> tuple[np.ndarray, np.ndarray]:
    model.eval()
    with torch.no_grad():
        logits, margin = model(torch.from_numpy((x - mean) / std))
    return torch.sigmoid(logits).numpy().astype(np.float64), margin.numpy().astype(np.float64)


def train(train_split: Split, valid_split: Split, settings: Settings, log=print):
    """Fit the network; returns it with the input mean and scale and its history."""
    torch.manual_seed(settings.seed)
    np.random.seed(settings.seed)
    mean = train_split.x.mean(axis=0).astype(np.float32)
    std = train_split.x.std(axis=0).astype(np.float32)
    std = np.where(std < 1e-3, 1.0, std).astype(np.float32)
    model = ValueNet(train_split.x.shape[1], settings.hidden, settings.depth, settings.activation)
    optimizer = torch.optim.Adam(
        model.parameters(), lr=settings.learning_rate, weight_decay=settings.weight_decay
    )
    x = torch.from_numpy((train_split.x - mean) / std)
    result = torch.from_numpy(train_split.result.astype(np.float32))
    margin = torch.from_numpy(train_split.margin.astype(np.float32))
    bce = nn.BCEWithLogitsLoss()
    best, best_state, stale, history = math.inf, None, 0, []
    generator = torch.Generator().manual_seed(settings.seed)
    for epoch in range(settings.epochs):
        model.train()
        order = torch.randperm(len(x), generator=generator)
        total = 0.0
        for start in range(0, len(x), settings.batch):
            index = order[start : start + settings.batch]
            logits, predicted = model(x[index])
            loss = bce(logits, result[index]) + settings.margin_weight * torch.mean(
                (predicted - margin[index]) ** 2
            )
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            total += float(loss.detach()) * len(index)
        p, m = predict(model, valid_split.x, mean, std)
        valid = log_loss(p, valid_split.result)
        history.append(
            {
                "epoch": epoch + 1,
                "train_loss": total / len(x),
                "valid_log_loss": valid,
                "valid_margin_mse": float(np.mean((m - valid_split.margin) ** 2)),
            }
        )
        log(f"epoch {epoch + 1}: train loss {total / len(x):.4f}, validation log loss {valid:.4f}")
        if valid < best - 1e-5:
            best, stale = valid, 0
            best_state = {k: v.detach().clone() for k, v in model.state_dict().items()}
        else:
            stale += 1
            if stale >= settings.patience:
                break
    if best_state is not None:
        model.load_state_dict(best_state)
    return model, mean, std, history


def float32_list(values) -> list[float]:
    """float32 values in their shortest round-trip form (smaller JSON, same float32)."""
    return [float(str(np.float32(v))) for v in values]


def tensor(value: np.ndarray) -> dict:
    return {"shape": list(value.shape), "values": float32_list(value.reshape(-1))}


def export_tensors(model: ValueNet, mean: np.ndarray, std: np.ndarray) -> dict:
    """The network's tensors with input standardization folded into layer 0."""
    tensors = {}
    for i, layer in enumerate(model.hidden):
        w = layer.weight.detach().numpy().astype(np.float64)
        b = layer.bias.detach().numpy().astype(np.float64)
        if i == 0:
            w = w / std.astype(np.float64)
            b = b - w @ mean.astype(np.float64)
        tensors[f"hidden.{i}.weight"] = tensor(w.astype(np.float32))
        tensors[f"hidden.{i}.bias"] = tensor(b.astype(np.float32))
    for head in ("win", "margin"):
        layer = getattr(model, head)
        tensors[f"{head}.weight"] = tensor(layer.weight.detach().numpy())
        tensors[f"{head}.bias"] = tensor(layer.bias.detach().numpy())
    return tensors


def mix(datasets: list[Dataset], held_out: set[str]) -> str:
    """A markdown table of positions and battles per dataset."""
    lines = ["| Dataset | Players | Battles | Positions | Held out |", "|---|---|---|---|---|"]
    totals = {False: [0, 0], True: [0, 0]}
    for d in datasets:
        out = d.involves(held_out)
        totals[out][0] += d.header["battles"]
        totals[out][1] += d.header["rows"]
        players = ", ".join(d.header.get("players") or [])
        name = f"{d.header['hero']} vs {d.header['opponent']}"
        lines.append(
            f"| {name} | {players} | {d.header['battles']} | {d.header['rows']} | {'yes' if out else ''} |"
        )
    for out, label in ((False, "training"), (True, "held out")):
        lines.append(f"| **{label}** | | {totals[out][0]} | {totals[out][1]} | |")
    return "\n".join(lines)


def run(args: argparse.Namespace) -> dict:
    settings = Settings(
        hidden=args.hidden,
        depth=args.depth,
        activation=args.activation,
        margin_weight=args.margin_weight,
        learning_rate=args.learning_rate,
        epochs=args.epochs,
        batch=args.batch,
        seed=args.seed,
    )
    paths = sorted(
        {Path(p) for pattern in args.data for p in glob.glob(pattern) if (Path(p) / "header.json").exists()}
    )
    if not paths:
        raise SystemExit(f"no datasets match {args.data}")
    datasets = [load_dataset(p) for p in paths]
    features = datasets[0].header["features"]
    for d in datasets:
        if d.header["schema_hash"] != datasets[0].header["schema_hash"] or d.header["features"] != features:
            raise SystemExit(f"{d.path}: schema differs from {datasets[0].path}")
    held_out = {args.held_out_hero, args.held_out_opponent}
    if args.mix_only:
        print(mix(datasets, held_out))
        return {}
    training = [d for d in datasets if not d.involves(held_out)]
    testing = [d for d in datasets if d.involves(held_out)]
    if not args.output:
        raise SystemExit("--output is required")
    if not training or not testing:
        raise SystemExit("need datasets both with and without the held-out hero/opponent")
    train_split, valid_split = battle_split(training, settings.validation, settings.seed)
    test = join(testing)
    matchup = [
        d
        for d in testing
        if d.header["hero"] == args.held_out_hero and d.header["opponent"] == args.held_out_opponent
    ]
    rows = f"{len(train_split.result)} train rows, {len(valid_split.result)} validation rows"
    print(f"{len(training)} training datasets: {rows}")
    print(f"{len(testing)} held-out datasets: {len(test.result)} rows")
    started = time.time()
    model, mean, std, history = train(train_split, valid_split, settings)

    def evaluate(split: Split) -> dict:
        p, m = predict(model, split.x, mean, std)
        out = scores(p, split.result)
        out["margin_mse"] = float(np.mean((m - split.margin) ** 2))
        return out

    a, b = fit_logistic(health_margin(train_split, features), train_split.result)

    def baselines(split: Split) -> dict:
        m = health_margin(split, features)
        return {
            "logistic_health_margin": scores(1 / (1 + np.exp(-(a * m + b))), split.result),
            "tanh_health_margin": scores((1 + np.tanh(2 * m)) / 2, split.result),
        }

    metrics = {
        "validation": {"network": evaluate(valid_split), **baselines(valid_split)},
        "held_out": {"network": evaluate(test), **baselines(test)},
        "logistic_fit": {"slope": a, "intercept": b},
    }
    if matchup:
        split = join(matchup)
        metrics["held_out_matchup"] = {"network": evaluate(split), **baselines(split)}
    held = metrics["held_out"]
    passed = held["network"]["log_loss"] < min(
        held["logistic_health_margin"]["log_loss"], held["tanh_health_margin"]["log_loss"]
    )
    metrics["beats_both_baselines_on_held_out_log_loss"] = bool(passed)
    hashes = sorted({h for d in datasets for h in d.header.get("content_hashes", [])})
    tensors = export_tensors(model, mean, std)
    # Check rows let the Go loader confirm it computes what torch computed.
    check_rows = test.x[: min(8, len(test.x))]
    p, m = predict(model, check_rows, mean, std)
    export = {
        "format": MODEL_FORMAT,
        "schema": SCHEMA,
        "schema_hash": datasets[0].header["schema_hash"],
        "feature_count": len(features),
        "features": features,
        "architecture": {
            "hidden": [settings.hidden] * settings.depth,
            "activation": settings.activation,
            "heads": {"win": "sigmoid probability", "margin": "tanh final health margin"},
        },
        "tensors": tensors,
        "check": [
            {"input": float32_list(row), "win": float(pw), "margin": float(pm)}
            for row, pw, pm in zip(check_rows, p, m, strict=True)
        ],
        "datasets": [
            {
                "name": d.name,
                "sha256": d.header_sha256,
                "rows": int(d.header["rows"]),
                "hero": d.header["hero"],
                "opponent": d.header["opponent"],
                "held_out": d.involves(held_out),
            }
            for d in datasets
        ],
        "held_out": {"hero": args.held_out_hero, "opponent": args.held_out_opponent},
        "training_content_hashes": hashes,
        "training_content_hash": hashlib.sha256("\n".join(hashes).encode()).hexdigest(),
        "training": {**asdict(settings), "seconds": round(time.time() - started, 1), "history": history},
        "metrics": metrics,
        "created_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(export, separators=(",", ":")))
    print(report(metrics))
    digest = hashlib.sha256(output.read_bytes()).hexdigest()
    print(f"wrote {output} ({output.stat().st_size} bytes, sha256 {digest})")
    return export


def report(metrics: dict) -> str:
    lines = []
    for part in ("validation", "held_out", "held_out_matchup"):
        if part not in metrics:
            continue
        lines.append(f"{part}:")
        for name, s in metrics[part].items():
            extra = f", margin MSE {s['margin_mse']:.4f}" if "margin_mse" in s else ""
            loss = f"log loss {s['log_loss']:.4f}  Brier {s['brier']:.4f}"
            lines.append(f"  {name:24s} {loss}  ({s['rows']} rows{extra})")
    lines.append(
        f"beats both baselines on held-out log loss: {metrics['beats_both_baselines_on_held_out_log_loss']}"
    )
    return "\n".join(lines)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "--data", nargs="+", required=True, help="dataset directories or globs (ml/runs/value-v1/*)"
    )
    parser.add_argument("--held-out-hero", required=True)
    parser.add_argument("--held-out-opponent", required=True)
    parser.add_argument("--output", default="", help="model JSON path")
    parser.add_argument("--mix-only", action="store_true", help="print the datasets' mix and stop")
    parser.add_argument("--hidden", type=int, default=128)
    parser.add_argument("--depth", type=int, default=2)
    parser.add_argument("--activation", choices=("gelu", "relu"), default="gelu")
    parser.add_argument("--margin-weight", type=float, default=1.0)
    parser.add_argument("--learning-rate", type=float, default=1e-3)
    parser.add_argument("--epochs", type=int, default=40)
    parser.add_argument("--batch", type=int, default=1024)
    parser.add_argument("--seed", type=int, default=11)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="value-train", description=__doc__.splitlines()[0])
    add_arguments(parser)
    run(parser.parse_args(argv))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
