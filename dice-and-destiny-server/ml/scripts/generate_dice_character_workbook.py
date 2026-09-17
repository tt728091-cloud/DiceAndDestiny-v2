from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def parse_args() -> argparse.Namespace:
    script = Path(__file__).resolve()
    default_repo = script.parents[3]
    parser = argparse.ArgumentParser(
        description=(
            "Generate exact 5d6 teacher/value analysis and the complete character "
            "workbook from one character JSON configuration."
        )
    )
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--repo", type=Path, default=default_repo)
    parser.add_argument("--source-csv", type=Path)
    parser.add_argument("--source-workbook", type=Path)
    parser.add_argument("--analysis-output", type=Path)
    parser.add_argument("--preview-dir", type=Path)
    parser.add_argument(
        "--node",
        default=os.environ.get("DICE_ANALYSIS_NODE", "node"),
        help="Node.js executable; defaults to DICE_ANALYSIS_NODE or node",
    )
    parser.add_argument(
        "--node-modules",
        type=Path,
        default=os.environ.get("DICE_ANALYSIS_NODE_MODULES"),
        help=(
            "node_modules directory containing @oai/artifact-tool; defaults to "
            "DICE_ANALYSIS_NODE_MODULES"
        ),
    )
    return parser.parse_args()


def run(command: list[str]) -> None:
    print("+", " ".join(command), flush=True)
    subprocess.run(command, check=True)


def main() -> None:
    args = parse_args()
    repo = args.repo.resolve()
    config = args.config.resolve()
    output = args.output.resolve()
    reference_dir = repo / "docs/game-design/dice-throne-reference"
    source_csv = (args.source_csv or reference_dir / "dice-throne-reference.csv").resolve()
    source_workbook = (
        args.source_workbook or reference_dir / "dice-throne-reference.xlsx"
    ).resolve()
    config_data = json.loads(config.read_text())
    character_id = config_data.get("character", {}).get("id", config.stem)
    analysis_output = (
        args.analysis_output
        or output.parent / "analysis-data" / f"{character_id}-analysis.json"
    ).resolve()
    preview_dir = (
        args.preview_dir or output.parent / f"{character_id}-previews"
    ).resolve()
    scripts_dir = Path(__file__).resolve().parent
    analyzer = scripts_dir / "generate_dice_character_analysis.py"
    builder = scripts_dir / "build_dice_character_workbook.mjs"

    analysis_output.parent.mkdir(parents=True, exist_ok=True)
    output.parent.mkdir(parents=True, exist_ok=True)
    preview_dir.mkdir(parents=True, exist_ok=True)

    run(
        [
            sys.executable,
            str(analyzer),
            "--repo",
            str(repo),
            "--config",
            str(config),
            "--source-csv",
            str(source_csv),
            "--source-workbook",
            str(source_workbook),
            "--output",
            str(analysis_output),
        ]
    )

    with tempfile.TemporaryDirectory(prefix="dice-character-workbook-") as temp:
        temp_dir = Path(temp)
        temp_builder = temp_dir / builder.name
        shutil.copy2(builder, temp_builder)
        if args.node_modules:
            node_modules = args.node_modules.resolve()
            if not node_modules.is_dir():
                raise SystemExit(f"node_modules directory does not exist: {node_modules}")
            (temp_dir / "node_modules").symlink_to(node_modules, target_is_directory=True)
        run(
            [
                args.node,
                str(temp_builder),
                "--analysis",
                str(analysis_output),
                "--source-xlsx",
                str(source_workbook),
                "--source-csv",
                str(source_csv),
                "--output",
                str(output),
                "--preview-dir",
                str(preview_dir),
            ]
        )
    print(output)


if __name__ == "__main__":
    main()
