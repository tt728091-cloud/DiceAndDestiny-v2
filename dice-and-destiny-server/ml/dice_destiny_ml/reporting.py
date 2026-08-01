from __future__ import annotations

import json
from collections import Counter
from pathlib import Path
from typing import Any

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402
import numpy as np  # noqa: E402


def generate_report_artifacts(
    *,
    training_root: Path,
    acceptance_root: Path,
    tournament_file: Path,
    evaluation_dirs: list[Path],
    output_dir: Path,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    training = {
        seed_dir.name: read_jsonl(seed_dir / "training_episodes.jsonl")
        for seed_dir in sorted(training_root.glob("seed-*"))
    }
    acceptance = read_jsonl(acceptance_root / "episodes.jsonl")
    tournament = json.loads(tournament_file.read_text())
    evaluations = {
        directory.name: json.loads((directory / "summary.json").read_text()) for directory in evaluation_dirs
    }
    plot_training_curves(training, output_dir / "training-curves.png")
    plot_checkpoint_ratings(tournament["elo"], output_dir / "checkpoint-ratings.png")
    plot_baseline_win_rates(evaluations, output_dir / "baseline-win-rates.png")
    plot_distributions(acceptance, output_dir / "battle-distributions.png")
    plot_action_frequency(acceptance, output_dir / "action-frequency.png")
    result = {
        "training_episodes": {seed: len(records) for seed, records in training.items()},
        "acceptance_episodes": len(acceptance),
        "plots": [
            str(output_dir / "training-curves.png"),
            str(output_dir / "checkpoint-ratings.png"),
            str(output_dir / "baseline-win-rates.png"),
            str(output_dir / "battle-distributions.png"),
            str(output_dir / "action-frequency.png"),
        ],
        "evaluations": evaluations,
        "tournament_elo": tournament["elo"],
    }
    (output_dir / "artifact-index.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


def plot_training_curves(training: dict[str, list[dict]], output: Path) -> None:
    figure, axis = plt.subplots(figsize=(9, 5))
    for seed, records in training.items():
        timesteps = np.array([record["timesteps"] for record in records])
        rewards = np.array([record["episode"]["r"] for record in records], dtype=float)
        window = min(25, len(rewards))
        smoothed = np.convolve(rewards, np.ones(window) / window, mode="valid")
        axis.plot(timesteps[window - 1 :], smoothed, label=seed)
    axis.axhline(0, color="#555555", linewidth=0.8)
    axis.set(
        title="Training reward by seed (25-episode rolling mean)",
        xlabel="PPO timesteps",
        ylabel="Terminal reward",
    )
    axis.legend()
    axis.grid(alpha=0.2)
    save_figure(figure, output)


def plot_checkpoint_ratings(ratings: dict[str, float], output: Path) -> None:
    labels = ["initial", "behavior-cloned", "step-000005000", "final"]
    values = [ratings[label] for label in labels]
    figure, axis = plt.subplots(figsize=(8, 4.5))
    axis.bar(labels, values, color=["#9aa0a6", "#6aaed6", "#3572b0", "#173f6b"])
    axis.set(title="Seed 11 checkpoint tournament rating", ylabel="Elo")
    axis.tick_params(axis="x", rotation=15)
    axis.grid(axis="y", alpha=0.2)
    save_figure(figure, output)


def plot_baseline_win_rates(evaluations: dict[str, dict], output: Path) -> None:
    labels: list[str] = []
    rates: list[float] = []
    lowers: list[float] = []
    uppers: list[float] = []
    for label, summary in evaluations.items():
        learned = next(
            (value for key, value in summary["policies"].items() if key.startswith("model:")), None
        )
        if learned is None:
            continue
        labels.append(label.replace("-final-v-random", ""))
        rates.append(learned["win_rate"])
        lowers.append(learned["wilson_95"][0])
        uppers.append(learned["wilson_95"][1])
    figure, axis = plt.subplots(figsize=(8, 4.5))
    errors = np.array([np.array(rates) - np.array(lowers), np.array(uppers) - np.array(rates)])
    axis.bar(labels, rates, yerr=errors, capsize=5, color="#2f76b7")
    axis.axhline(0.5, color="#b5483a", linestyle="--", label="50%")
    axis.set(title="Held-out final-policy win rate versus random", ylabel="Win rate", ylim=(0, 1.05))
    axis.legend()
    axis.grid(axis="y", alpha=0.2)
    save_figure(figure, output)


def plot_distributions(records: list[dict], output: Path) -> None:
    actions = [record["actions"] for record in records]
    health = [health for record in records for health in record["remaining_health"].values()]
    figure, axes = plt.subplots(1, 2, figsize=(10, 4.5))
    axes[0].hist(actions, bins=25, color="#2f76b7")
    axes[0].set(title="Acceptance battle duration", xlabel="Authority actions", ylabel="Battles")
    axes[1].hist(health, bins=range(0, max(health) + 2), color="#e69d32", align="left")
    axes[1].set(title="Acceptance remaining health", xlabel="Health", ylabel="Seat outcomes")
    save_figure(figure, output)


def plot_action_frequency(records: list[dict], output: Path) -> None:
    counts: Counter[str] = Counter()
    for record in records:
        counts.update(record["action_frequency"])
    labels, values = zip(*counts.most_common(), strict=True)
    figure, axis = plt.subplots(figsize=(9, 5))
    axis.barh(labels[::-1], values[::-1], color="#3572b0")
    axis.set(title="Action frequency in 1,000-game acceptance soak", xlabel="Selections")
    axis.grid(axis="x", alpha=0.2)
    save_figure(figure, output)


def read_jsonl(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text().splitlines() if line]


def save_figure(figure: plt.Figure, output: Path) -> None:
    figure.tight_layout()
    figure.savefig(output, dpi=160)
    plt.close(figure)
