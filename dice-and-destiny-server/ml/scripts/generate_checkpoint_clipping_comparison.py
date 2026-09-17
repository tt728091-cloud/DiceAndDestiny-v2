# ruff: noqa: E501 - embedded HTML/CSS is intentionally kept readable as a page template
from __future__ import annotations

import csv
import html
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class CampaignSource:
    label: str
    root: Path
    stop_interval: int | None
    telemetry_kind: str


ML_ROOT = Path(__file__).resolve().parents[1]
REPOSITORY_ROOT = ML_ROOT.parents[1]
OUTPUT_ROOT = REPOSITORY_ROOT / "docs/machine-learning-battle-simulation"
HTML_PATH = OUTPUT_ROOT / "checkpoint-progression-clipping-three-campaign-comparison.html"
CSV_PATH = OUTPUT_ROOT / "checkpoint-progression-clipping-three-campaign-comparison.csv"

SOURCES = (
    CampaignSource(
        "Current · looser clipping",
        ML_ROOT / "runs/v5-relational-cp12-looser-clipping-cp193-20260809-2h",
        None,
        "interval-wide",
    ),
    CampaignSource(
        "Last night · V5 eight-hour",
        ML_ROOT / "runs/v5-relational-cp6-continuation-cp193-20260808-8h",
        None,
        "legacy-final-rollout",
    ),
    CampaignSource(
        "Global champion lineage · through CP193",
        ML_ROOT / "runs/v2-winner-health-v2-cp35-continuation-cp38-20260806-8h-1k-only",
        193,
        "legacy-final-rollout",
    ),
)


def _attempts(source: CampaignSource) -> list[dict[str, Any]]:
    state = json.loads((source.root / "campaign-state.json").read_text())
    return [
        attempt
        for attempt in state.get("attempts", [])
        if source.stop_interval is None or int(attempt["interval"]) <= source.stop_interval
    ]


def _view(source: CampaignSource, attempt: dict[str, Any]) -> dict[str, Any]:
    telemetry = attempt["training"]["telemetry"]
    stability = telemetry.get("stability") or {}
    ppo = telemetry.get("ppo") or {}
    evaluation = attempt["evaluation"]
    if source.telemetry_kind == "interval-wide":
        ratio_clip = float(stability["mean_policy_ratio_clip_fraction_all_minibatches"])
        gradient_clip = float(stability["gradient_clip_event_rate"])
        kl = float(stability["mean_approx_kl_all_minibatches"])
        kl_stops = f"{int(stability['target_kl_early_stops'])}/{int(stability['multiworker_rollouts'])}"
    else:
        ratio_clip = float(ppo["clip_fraction"])
        gradient_clip = None
        kl = float(ppo["approx_kl"])
        kl_stops = "disabled"
    parameter_movement = telemetry.get("relative_parameter_l2_movement")
    if parameter_movement is None:
        parameter_movement = stability.get("relative_parameter_l2_movement")
    training_seconds = float(attempt["training"]["elapsed_seconds"])
    evaluation_seconds = float(evaluation["elapsed_seconds"])
    return {
        "cp": int(attempt["interval"]),
        "score": float(evaluation["adjusted_score"]),
        "ratio_clip": ratio_clip,
        "gradient_clip": gradient_clip,
        "parameter_movement": (
            None if parameter_movement is None else float(parameter_movement)
        ),
        "kl": kl,
        "kl_stops": kl_stops,
        "ppo_steps": int(attempt["challenger"]["learner_steps"]),
        "ppo_delta": int(attempt["training"]["completed_steps"]),
        "training_seconds": training_seconds,
        "evaluation_seconds": evaluation_seconds,
        "total_seconds": training_seconds + evaluation_seconds,
        "outcome": str(attempt["outcome"]),
        "wins": int(evaluation["wins"]),
        "draws": int(evaluation["draws"]),
        "losses": int(evaluation["losses"]),
    }


def _duration(seconds: float) -> str:
    minutes, remainder = divmod(seconds, 60)
    return f"{int(minutes)}m {remainder:04.1f}s"


def _cell(view: dict[str, Any] | None, key: str) -> str:
    if view is None:
        return '<td class="missing">—</td>'
    if key == "cp":
        passed = view["outcome"] in {"passed", "baseline-established"}
        label = f"CP{view['cp']}"
        return f'<td class="cp {"pass" if passed else "fail"}">{label}</td>'
    if key == "score":
        title = f"{view['wins']}/{view['draws']}/{view['losses']}"
        return f'<td title="{title}">{100 * view[key]:.2f}%</td>'
    if key in {"ratio_clip", "gradient_clip", "parameter_movement"}:
        value = view[key]
        return '<td class="missing">not recorded</td>' if value is None else f"<td>{100 * value:.2f}%</td>"
    if key == "kl":
        return f"<td>{view[key]:.5f}</td>"
    if key == "kl_stops":
        return f"<td>{html.escape(view[key])}</td>"
    if key == "ppo_steps":
        return f"<td>{view[key]:,}<small>+{view['ppo_delta']:,}</small></td>"
    if key == "total_seconds":
        title = (
            f"training {_duration(view['training_seconds'])}; "
            f"evaluation {_duration(view['evaluation_seconds'])}"
        )
        return f'<td title="{title}">{_duration(view[key])}</td>'
    raise KeyError(key)


def write_html(data: list[list[dict[str, Any]]]) -> None:
    keys = (
        "cp", "score", "ratio_clip", "gradient_clip", "parameter_movement",
        "kl", "kl_stops", "ppo_steps", "total_seconds",
    )
    labels = (
        "CP", "Score", "Ratio clipped", "Grad clipped", "Parameter movement",
        "KL", "KL stops", "PPO steps", "Train + eval",
    )
    maximum = max(map(len, data))
    header_groups = "".join(f'<th colspan="9">{html.escape(source.label)}</th>' for source in SOURCES)
    header_labels = "".join("".join(f"<th>{label}</th>" for label in labels) for _ in SOURCES)
    rows = []
    for index in range(maximum):
        groups = []
        for campaign in data:
            view = campaign[index] if index < len(campaign) else None
            groups.append("".join(_cell(view, key) for key in keys))
        rows.append(f'<tr><th class="attempt">{index + 1}</th>{"".join(groups)}</tr>')
    cards = []
    for source, campaign in zip(SOURCES, data, strict=True):
        best = max(campaign, key=lambda item: item["score"])
        cards.append(
            '<section class="card">'
            f"<h2>{html.escape(source.label)}</h2>"
            f"<p><b>{len(campaign)}</b> checkpoints · CP{campaign[0]['cp']}–CP{campaign[-1]['cp']}</p>"
            f"<p>Best: <b>CP{best['cp']} · {100 * best['score']:.2f}%</b></p>"
            f"<p>{html.escape(source.telemetry_kind.replace('-', ' '))} clipping telemetry</p>"
            "</section>"
        )
    document = f"""<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Checkpoint progression and clipping · three-campaign comparison</title>
<style>
:root{{--bg:#0b111b;--panel:#131c29;--line:#2a394c;--text:#e9eef5;--muted:#9fb0c5;--gold:#f5c95d;--green:#74d69b;--red:#f08c8c}}
*{{box-sizing:border-box}} body{{margin:0;background:var(--bg);color:var(--text);font:14px/1.45 system-ui,sans-serif}}
main{{max-width:100%;padding:28px}} h1{{color:var(--gold);margin:0 0 8px}} .lede,.note{{color:var(--muted);max-width:1100px}}
.cards{{display:grid;grid-template-columns:repeat(3,minmax(260px,1fr));gap:12px;margin:22px 0}} .card{{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:14px}} .card h2{{font-size:16px;margin:0 0 6px}} .card p{{margin:4px 0;color:var(--muted)}}
.wrap{{overflow:auto;border:1px solid var(--line);border-radius:10px;max-height:78vh}} table{{border-collapse:separate;border-spacing:0;min-width:2740px;background:var(--panel)}} th,td{{padding:7px 9px;border-right:1px solid var(--line);border-bottom:1px solid var(--line);white-space:nowrap;text-align:right}} thead th{{position:sticky;top:0;background:#1a2636;z-index:3;color:var(--gold);text-align:center}} thead tr:nth-child(2) th{{top:34px;color:var(--text)}} .attempt{{position:sticky;left:0;background:#182333;z-index:2;text-align:center}} thead .attempt{{z-index:5}} td.cp{{font-weight:700}} .pass{{color:var(--green)}} .fail{{color:var(--red)}} .missing{{color:#627286;text-align:center}} small{{display:block;color:var(--muted)}}
@media(max-width:900px){{.cards{{grid-template-columns:1fr}} main{{padding:16px}}}}
</style></head><body><main>
<h1>Checkpoint progression and clipping</h1>
<p class="lede">Aligned by attempt number: row 1 is the first checkpoint produced in each campaign. Score is wins + half draws against that campaign’s fixed global opponent. Hover score cells for W/D/L and time cells for the training/evaluation split.</p>
<p class="note"><b>Telemetry comparability:</b> the current run reports interval-wide means across every minibatch. The two older runs preserved only the final PPO rollout’s ratio-clip fraction and KL. Their gradient-clipping event rates were not recorded, and target KL was disabled. Those cells are deliberately marked rather than inferred.</p>
<div class="cards">{"".join(cards)}</div>
<div class="wrap"><table><thead><tr><th class="attempt" rowspan="2">Attempt</th>{header_groups}</tr><tr>{header_labels}</tr></thead><tbody>{"".join(rows)}</tbody></table></div>
</main></body></html>"""
    HTML_PATH.write_text(document)


def write_csv(data: list[list[dict[str, Any]]]) -> None:
    maximum = max(map(len, data))
    fields = (
        "cp", "score", "ratio_clip", "gradient_clip", "parameter_movement", "kl", "kl_stops", "ppo_steps",
        "ppo_delta", "training_seconds", "evaluation_seconds", "total_seconds", "outcome",
        "wins", "draws", "losses",
    )
    with CSV_PATH.open("w", newline="") as handle:
        writer = csv.DictWriter(
            handle,
            fieldnames=["attempt", *[f"{index + 1}_{field}" for index in range(3) for field in fields]],
        )
        writer.writeheader()
        for attempt_index in range(maximum):
            row: dict[str, Any] = {"attempt": attempt_index + 1}
            for source_index, campaign in enumerate(data, start=1):
                if attempt_index < len(campaign):
                    for field in fields:
                        row[f"{source_index}_{field}"] = campaign[attempt_index][field]
            writer.writerow(row)


def main() -> None:
    data = [[_view(source, attempt) for attempt in _attempts(source)] for source in SOURCES]
    OUTPUT_ROOT.mkdir(parents=True, exist_ok=True)
    write_html(data)
    write_csv(data)
    print(HTML_PATH)
    print(CSV_PATH)


if __name__ == "__main__":
    main()
