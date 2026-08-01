from __future__ import annotations

import json
from itertools import combinations_with_replacement
from pathlib import Path
from typing import Any

from .evaluation import evaluate


def run_tournament(
    *,
    binary: Path,
    server_root: Path,
    checkpoints: list[Path],
    seeds: list[int],
    output_dir: Path,
    device: str = "cpu",
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    names = [path.stem for path in checkpoints]
    matrix: dict[str, dict[str, float]] = {name: {} for name in names}
    ratings = {name: 1500.0 for name in names}
    match_summaries: dict[str, Any] = {}
    for left_index, right_index in combinations_with_replacement(range(len(checkpoints)), 2):
        left = checkpoints[left_index]
        right = checkpoints[right_index]
        pair_name = f"{left.stem}__vs__{right.stem}"
        summary = evaluate(
            binary=binary,
            server_root=server_root,
            seat_a_spec=f"model:{left}",
            seat_b_spec=f"model:{right}",
            seeds=seeds,
            output_dir=output_dir / "matches" / pair_name,
            swap=True,
            device=device,
            save_replays="none",
        )
        match_summaries[pair_name] = summary
        left_policy = summary["policies"][f"model:{left}"]
        right_policy = summary["policies"][f"model:{right}"]
        matrix[left.stem][right.stem] = left_policy["adjudicated_score"]
        matrix[right.stem][left.stem] = right_policy["adjudicated_score"]
        if left != right:
            _update_elo(
                ratings,
                left.stem,
                right.stem,
                left_policy["adjudicated_score"],
                left_policy["games"],
            )
    result = {
        "checkpoints": [str(path) for path in checkpoints],
        "seeds": seeds,
        "adjudicated_score_matrix": matrix,
        "elo": dict(sorted(ratings.items(), key=lambda item: item[1], reverse=True)),
        "matches": match_summaries,
    }
    (output_dir / "tournament.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


def _update_elo(ratings: dict[str, float], left: str, right: str, score: float, games: int) -> None:
    expected = 1.0 / (1.0 + 10.0 ** ((ratings[right] - ratings[left]) / 400.0))
    change = 32.0 * max(games, 1) ** 0.5 * (score - expected)
    ratings[left] += change
    ratings[right] -= change
