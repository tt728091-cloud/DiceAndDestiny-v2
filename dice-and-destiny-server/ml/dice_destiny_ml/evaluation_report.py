from __future__ import annotations

import html
import json
from pathlib import Path
from typing import Any

CORE_METRICS = (
    ("Adjusted score", "adjusted_score", "percent"),
    ("Raw win rate", "win_rate", "percent"),
    ("Ability selected before reaction", "offensive_pre_reaction_ability_rate", "percent"),
    ("Ability selected after reaction", "offensive_post_reaction_ability_rate", "percent"),
    ("Selected on first roll", "offensive_selection_roll_1_rate", "percent"),
    ("Selected on second roll", "offensive_selection_roll_2_rate", "percent"),
    ("Selected on third roll", "offensive_selection_roll_3_rate", "percent"),
    ("Mean ability-selection roll", "mean_rolls_used_at_ability_selection", "number"),
    ("Qualified reroll rate", "qualified_reroll_rate", "percent"),
    ("Raw damage dealt / round", "average_raw_outgoing_total_per_round", "number"),
    ("Actual damage dealt / round", "average_actual_outgoing_total_per_round", "number"),
    ("Raw damage taken / round", "average_raw_incoming_total_per_round", "number"),
    ("Actual damage taken / round", "average_actual_incoming_total_per_round", "number"),
    ("Actual damage advantage / round", "average_actual_damage_advantage_per_round", "number"),
    ("Status stacks / offensive segment", "average_status_stacks_per_offensive_segment", "number"),
    ("Pass rate", "pass_rate", "percent"),
)


def write_evaluation_statistics_html(
    summary: dict[str, Any],
    path: Path,
    *,
    title: str,
    focus_policy: str | None = None,
    adjusted_scores: dict[str, float] | None = None,
) -> None:
    policies = sorted((summary.get("behavior_by_policy") or {}).keys())
    focus = focus_policy or (policies[0] if policies else "")
    views = {
        policy: _policy_view(summary, policy, (adjusted_scores or {}).get(policy)) for policy in policies
    }
    body = [
        f"<h1>{html.escape(title)}</h1>",
        '<p class="lede">Authority-derived evaluation behavior. Counts distinguish opportunities '
        "from selections; damage comes from committed authority resolutions, while status stacks are "
        "measured from state changes after commands.</p>",
        _metric_cards(summary, views, focus),
        _core_table(views, focus),
    ]
    for policy in policies:
        body.append(_policy_sections(policy, views[policy], focus=policy == focus))
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(_document(title, "".join(body)))


def write_campaign_statistics_html(root: Path, state: dict[str, Any]) -> None:
    evaluated = [item for item in state.get("attempts", []) if item.get("evaluation")]
    if not evaluated:
        return
    global_checkpoint = state["fixed_global_champion"]
    global_spec = f"model:{global_checkpoint['checkpoint_path']}"
    rows: list[str] = []
    gameplay_rows: list[str] = []
    metric_records: list[dict[str, Any]] = []
    attempt_views = {
        int(item["interval"]): _attempt_view(item) or {}
        for item in evaluated
    }
    best_score = -1.0
    previous_score: float | None = None
    for item in evaluated:
        evaluation = item["evaluation"]
        score = float(evaluation["adjusted_score"])
        before_best = best_score if best_score >= 0 else None
        delta_previous = None if previous_score is None else score - previous_score
        delta_best = None if before_best is None else score - before_best
        incumbent = item.get("incumbent_evaluation") or {}
        stability = ((item.get("training") or {}).get("telemetry") or {}).get("stability") or {}
        training_seconds = float((item.get("training") or {}).get("elapsed_seconds", 0))
        evaluation_seconds = float(evaluation.get("elapsed_seconds", 0))
        elapsed_title = html.escape(
            f"training {_duration(training_seconds)}; evaluation {_duration(evaluation_seconds)}",
            quote=True,
        )
        rows.append(
            "<tr>"
            f'<td><a href="intervals/interval-{int(item["interval"]):03d}/checkpoint-statistics.html">'
            f"CP{int(item['interval'])}</a></td>"
            f"<td>{int(item['challenger']['learner_steps']):,}</td>"
            f"<td>{evaluation['wins']}/{evaluation['draws']}/{evaluation['losses']}</td>"
            f"<td>{_percent(score)}</td>"
            f"<td>{_percent(float(incumbent['adjusted_score'])) if incumbent else '—'}</td>"
            f"<td>{_signed_pp((item.get('gate') or {}).get('score_delta'))}</td>"
            f"<td>{_signed_pp(delta_previous)}</td>"
            f"<td>{_signed_pp(delta_best)}</td>"
            f"<td>{_number(stability.get('mean_approx_kl_all_minibatches', 0))}</td>"
            f"<td>{_percent(float(stability.get('mean_policy_ratio_clip_fraction_all_minibatches', 0)))}</td>"
            f"<td>{_percent(float(stability.get('gradient_clip_event_rate', 0)))}</td>"
            f"<td>{_percent(float(stability.get('relative_parameter_l2_movement', 0)))}</td>"
            f"<td>{int(stability.get('target_kl_early_stops', 0))}/"
            f"{int(stability.get('multiworker_rollouts', 0))}</td>"
            f'<td title="{elapsed_title}">{_duration(training_seconds + evaluation_seconds)}</td>'
            f"<td>{html.escape(str(item['outcome']))}</td>"
            "</tr>"
        )
        gameplay_rows.append(
            _gameplay_overview_row(
                item,
                attempt_views[int(item["interval"])],
            )
        )
        metric_records.append(
            _checkpoint_metric_record(
                item,
                attempt_views[int(item["interval"])],
                stability,
                training_seconds,
                evaluation_seconds,
            )
        )
        previous_score = score
        best_score = max(best_score, score)

    gameplay_overview = (
        "<h2>Gameplay progression against the global champion</h2>"
        '<p class="lede">Each row describes the checkpoint learner. Ability/pass percentages are '
        "captured before and after the offensive reaction window. Roll percentages use only rounds "
        "where the learner selected an offensive ability. Damage is averaged across every authority "
        "round in the evaluation.</p>"
        '<div class="table"><table><thead><tr><th>Checkpoint</th><th>W/D/L</th><th>Score</th>'
        "<th>Before reaction</th><th>After reaction</th><th>Selected roll 1 / 2 / 3</th>"
        "<th>Reaction changes</th><th>Raw dealt / round</th><th>Actual dealt / round</th>"
        "<th>Raw taken / round</th><th>Actual taken / round</th><th>Net actual / round</th>"
        "<th>Evaluated rounds</th></tr></thead><tbody>"
        + "".join(gameplay_rows)
        + "</tbody></table></div>"
    )
    overview = (
        f"<h1>{html.escape(str(state.get('campaign_id', 'Training campaign')))} statistics</h1>"
        '<p class="lede">Checkpoint progression against the frozen global champion. Each checkpoint page '
        "contains learner/global behavior and differences from the preceding and family-best checkpoints.</p>"
        '<p><a href="gameplay-statistics.html">Open the standalone gameplay-statistics sheet →</a></p>'
        '<div class="table"><table><thead><tr><th>Checkpoint</th><th>PPO steps</th><th>W/D/L</th>'
        "<th>Score</th><th>Paired incumbent</th><th>Paired delta</th><th>vs previous</th>"
        "<th>vs prior best</th><th>Mean KL</th><th>Ratio clipped</th><th>Grad clipped</th>"
        "<th>Parameter move</th><th>KL stops</th><th>Train + eval</th><th>Outcome</th>"
        "</tr></thead><tbody>"
        + "".join(rows)
        + "</tbody></table></div>"
        + gameplay_overview
    )
    (root / "statistics.html").write_text(
        _document(f"{state.get('campaign_id', 'Campaign')} statistics", overview)
    )
    standalone_gameplay = (
        '<p><a href="statistics.html">← Campaign overview</a></p>'
        f"<h1>{html.escape(str(state.get('campaign_id', 'Training campaign')))} gameplay statistics</h1>"
        + gameplay_overview
    )
    (root / "gameplay-statistics.html").write_text(
        _document(
            f"{state.get('campaign_id', 'Campaign')} gameplay statistics",
            standalone_gameplay,
        )
    )
    (root / "checkpoint-metrics.json").write_text(
        json.dumps(
            {
                "schema": "dice-and-destiny-checkpoint-metrics-v1",
                "campaign_id": state.get("campaign_id", ""),
                "global_champion": global_checkpoint,
                "checkpoints": metric_records,
            },
            indent=2,
            sort_keys=True,
        )
        + "\n"
    )
    (root / "metrics-artifacts.json").write_text(
        json.dumps(
            {
                "schema": "dice-and-destiny-training-metrics-artifacts-v1",
                "checkpoint_count": len(metric_records),
                "artifacts": {
                    "training_and_clipping_sheet": "statistics.html",
                    "gameplay_sheet": "gameplay-statistics.html",
                    "structured_checkpoint_metrics": "checkpoint-metrics.json",
                },
            },
            indent=2,
            sort_keys=True,
        )
        + "\n"
    )

    for index, item in enumerate(evaluated):
        summary_path = Path(item["evaluation"]["summary"])
        if not summary_path.exists():
            continue
        summary = json.loads(summary_path.read_text())
        challenger_spec = f"model:{item['challenger']['checkpoint_path']}"
        current = attempt_views[int(item["interval"])]
        global_view = _policy_view(summary, global_spec, 1.0 - float(item["evaluation"]["adjusted_score"]))
        previous_item = evaluated[index - 1] if index else None
        previous = attempt_views.get(int(previous_item["interval"])) if previous_item else None
        prior_items = evaluated[:index]
        family_best_item = (
            max(prior_items, key=lambda value: float(value["evaluation"]["adjusted_score"]))
            if prior_items
            else None
        )
        family_best = (
            attempt_views.get(int(family_best_item["interval"])) if family_best_item else None
        )
        comparison = _comparison_table(
            current,
            global_view,
            previous,
            family_best,
            current_label=f"CP{item['interval']}",
            previous_label=(
                f"Previous · CP{previous_item['interval']}" if previous_item else "Previous"
            ),
            best_label=(
                f"Family best · CP{family_best_item['interval']}"
                if family_best_item
                else "Family best"
            ),
        )
        body = (
            f'<p><a href="../../statistics.html">← Campaign overview</a></p>'
            f"<h1>Checkpoint {item['interval']} behavior</h1>"
            f'<p class="lede">{item["evaluation"]["wins"]}/{item["evaluation"]["draws"]}/'
            f"{item['evaluation']['losses']} · "
            f"{_percent(float(item['evaluation']['adjusted_score']))} adjusted · "
            f"{html.escape(str(item['outcome']))}</p>"
            + comparison
            + _training_stability_table(item)
            + _policy_sections(challenger_spec, current, focus=True)
            + _policy_sections(global_spec, global_view, focus=False)
        )
        target = root / "intervals" / f"interval-{int(item['interval']):03d}" / "checkpoint-statistics.html"
        target.write_text(_document(f"Checkpoint {item['interval']} statistics", body))


def _gameplay_overview_row(item: dict[str, Any], view: dict[str, Any]) -> str:
    evaluation = item["evaluation"]
    interval = int(item["interval"])
    before = _ability_pass_pair(
        view.get("offensive_pre_reaction_ability_rate"),
        view.get("offensive_pre_reaction_pass_rate"),
    )
    after = _ability_pass_pair(
        view.get("offensive_post_reaction_ability_rate"),
        view.get("offensive_post_reaction_pass_rate"),
    )
    rolls = " / ".join(
        _optional_percent(view.get(f"offensive_selection_roll_{roll}_rate"))
        for roll in range(1, 4)
    )
    changes = _reaction_changes(view)
    raw_outgoing = _damage_overview_cell(view, "outgoing")
    raw_incoming = _damage_overview_cell(view, "incoming")
    return (
        "<tr>"
        f'<td><a href="intervals/interval-{interval:03d}/checkpoint-statistics.html">'
        f"CP{interval}</a></td>"
        f"<td>{int(evaluation['wins'])}/{int(evaluation['draws'])}/{int(evaluation['losses'])}</td>"
        f"<td>{_percent(float(evaluation['adjusted_score']))}</td>"
        f"<td>{before}</td><td>{after}</td><td>{rolls}</td>"
        f"<td>{changes}</td><td>{raw_outgoing}</td>"
        f"<td>{_optional_number(view.get('average_actual_outgoing_total_per_round'))}</td>"
        f"<td>{raw_incoming}</td>"
        f"<td>{_optional_number(view.get('average_actual_incoming_total_per_round'))}</td>"
        f"<td>{_optional_signed_number(view.get('average_actual_damage_advantage_per_round'))}</td>"
        f"<td>{_optional_integer(view.get('rounds'))}</td>"
        "</tr>"
    )


def _checkpoint_metric_record(
    item: dict[str, Any],
    view: dict[str, Any],
    stability: dict[str, Any],
    training_seconds: float,
    evaluation_seconds: float,
) -> dict[str, Any]:
    evaluation = item["evaluation"]
    keys = (
        "offensive_pre_reaction_outcomes",
        "offensive_pre_reaction_ability_selected",
        "offensive_pre_reaction_passed",
        "offensive_pre_reaction_ability_rate",
        "offensive_pre_reaction_pass_rate",
        "offensive_post_reaction_outcomes",
        "offensive_post_reaction_ability_selected",
        "offensive_post_reaction_passed",
        "offensive_post_reaction_ability_rate",
        "offensive_post_reaction_pass_rate",
        "offensive_selection_roll_1_rate",
        "offensive_selection_roll_2_rate",
        "offensive_selection_roll_3_rate",
        "offensive_reaction_ability_preserved",
        "offensive_reaction_ability_lost",
        "offensive_reaction_ability_gained",
        "offensive_reaction_ability_switched",
        "offensive_reaction_pass_preserved",
        "average_raw_outgoing_attack_per_round",
        "average_raw_outgoing_bleed_per_round",
        "average_raw_outgoing_poison_per_round",
        "average_raw_outgoing_total_per_round",
        "average_resolved_outgoing_total_per_round",
        "average_actual_outgoing_total_per_round",
        "average_raw_incoming_attack_per_round",
        "average_raw_incoming_bleed_per_round",
        "average_raw_incoming_poison_per_round",
        "average_raw_incoming_total_per_round",
        "average_resolved_incoming_total_per_round",
        "average_actual_incoming_total_per_round",
        "average_actual_damage_advantage_per_round",
        "rounds",
    )
    gameplay = {key: view[key] if key in view else None for key in keys}
    return {
        "checkpoint": int(item["interval"]),
        "ppo_steps": int(item["challenger"]["learner_steps"]),
        "wins": int(evaluation["wins"]),
        "draws": int(evaluation["draws"]),
        "losses": int(evaluation["losses"]),
        "adjusted_score": float(evaluation["adjusted_score"]),
        "outcome": str(item["outcome"]),
        "timing": {
            "training_seconds": training_seconds,
            "evaluation_seconds": evaluation_seconds,
            "training_plus_evaluation_seconds": training_seconds + evaluation_seconds,
        },
        "ppo_update_pressure": {
            "mean_approx_kl_all_minibatches": stability.get(
                "mean_approx_kl_all_minibatches"
            ),
            "mean_policy_ratio_clip_fraction_all_minibatches": stability.get(
                "mean_policy_ratio_clip_fraction_all_minibatches"
            ),
            "gradient_clip_event_rate": stability.get("gradient_clip_event_rate"),
            "relative_parameter_l2_movement": stability.get(
                "relative_parameter_l2_movement"
            ),
            "target_kl_early_stops": stability.get("target_kl_early_stops"),
            "multiworker_rollouts": stability.get("multiworker_rollouts"),
        },
        "gameplay": gameplay,
        "evaluation_summary": evaluation.get("summary"),
    }


def _ability_pass_pair(ability: Any, passed: Any) -> str:
    return f"ability {_optional_percent(ability)} · pass {_optional_percent(passed)}"


def _reaction_changes(view: dict[str, Any]) -> str:
    if "offensive_post_reaction_outcomes" not in view:
        return "—"
    return " · ".join(
        (
            f"kept {int(view.get('offensive_reaction_ability_preserved', 0)):,}",
            f"lost {int(view.get('offensive_reaction_ability_lost', 0)):,}",
            f"gained {int(view.get('offensive_reaction_ability_gained', 0)):,}",
            f"switched {int(view.get('offensive_reaction_ability_switched', 0)):,}",
            f"pass kept {int(view.get('offensive_reaction_pass_preserved', 0)):,}",
        )
    )


def _damage_overview_cell(view: dict[str, Any], direction: str) -> str:
    total = _optional_number(view.get(f"average_raw_{direction}_total_per_round"))
    components = "/".join(
        _optional_number(view.get(f"average_raw_{direction}_{category}_per_round"))
        for category in ("attack", "bleed", "poison")
    )
    return f"{total}<small>attack/bleed/poison {components}</small>"


def _training_stability_table(item: dict[str, Any]) -> str:
    telemetry = ((item.get("training") or {}).get("telemetry") or {})
    stability = telemetry.get("stability") or {}
    if not stability:
        return ""
    rows = (
        (
            "Mean approximate KL (all minibatches)",
            _number(stability.get("mean_approx_kl_all_minibatches", 0)),
        ),
        ("Target KL", _number(stability.get("target_kl", 0))),
        (
            "Target-KL early stops",
            f"{int(stability.get('target_kl_early_stops', 0))} / "
            f"{int(stability.get('multiworker_rollouts', 0))} rollouts",
        ),
        (
            "Policy-ratio samples outside clip range",
            _percent(float(stability.get("mean_policy_ratio_clip_fraction_all_minibatches", 0))),
        ),
        (
            f"Pre-clip gradients over {float(stability.get('gradient_norm_limit', 0.5)):g} "
            "norm limit",
            _percent(float(stability.get("gradient_clip_event_rate", 0))),
        ),
        (
            "Mean pre-clip gradient norm",
            _number(stability.get("mean_preclip_gradient_norm_all_updates", 0)),
        ),
        (
            "Optimizer updates used",
            f"{int(stability.get('actual_optimizer_updates', 0))} / "
            f"{int(stability.get('planned_optimizer_updates', 0))}",
        ),
        (
            "Relative parameter L2 movement",
            _percent(float(stability.get("relative_parameter_l2_movement", 0))),
        ),
        ("Final-rollout entropy", _number(-float(stability.get("final_rollout_entropy_loss", 0)))),
        ("Final-rollout explained variance", _number(stability.get("final_rollout_explained_variance", 0))),
    )
    return (
        "<h2>PPO update pressure</h2>"
        '<p class="lede">These measurements expose clipping and target-limit pressure across the entire '
        "checkpoint interval.</p>"
        '<div class="table"><table><thead><tr><th>Measurement</th><th>Value</th></tr></thead><tbody>'
        + "".join(
            f"<tr><th>{html.escape(label)}</th><td>{html.escape(value)}</td></tr>"
            for label, value in rows
        )
        + "</tbody></table></div>"
    )


def _attempt_view(item: dict[str, Any] | None) -> dict[str, Any] | None:
    if not item:
        return None
    path = Path(item["evaluation"]["summary"])
    if not path.exists():
        return None
    summary = json.loads(path.read_text())
    spec = f"model:{item['challenger']['checkpoint_path']}"
    return _policy_view(summary, spec, float(item["evaluation"]["adjusted_score"]))


def _policy_view(summary: dict[str, Any], policy: str, adjusted_score: float | None) -> dict[str, Any]:
    behavior = dict((summary.get("behavior_by_policy") or {}).get(policy, {}))
    behavior.update((summary.get("damage_by_policy") or {}).get(policy, {}))
    outcome = dict((summary.get("policies") or {}).get(policy, {}))
    if adjusted_score is not None:
        behavior["adjusted_score"] = adjusted_score
    elif "adjudicated_score" in outcome:
        behavior["adjusted_score"] = outcome["adjudicated_score"]
    behavior["win_rate"] = outcome.get("win_rate", 0.0)
    behavior["wins"] = outcome.get("wins", 0)
    behavior["games"] = outcome.get("games", 0)
    return behavior


def _metric_cards(summary: dict[str, Any], views: dict[str, dict[str, Any]], focus: str) -> str:
    current = views.get(focus, {})
    return (
        '<div class="cards">'
        + "".join(
            (
                _card("Games", f"{int(summary.get('games', 0)):,}"),
                _card("Adjusted score", _percent(float(current.get("adjusted_score", 0.0)))),
                _card(
                    "Actual damage dealt / round",
                    _number(current.get("average_actual_outgoing_total_per_round", 0)),
                ),
                _card(
                    "Actual damage taken / round",
                    _number(current.get("average_actual_incoming_total_per_round", 0)),
                ),
                _card(
                    "Status stacks / offense",
                    _number(current.get("average_status_stacks_per_offensive_segment", 0)),
                ),
                _card("Mean actions", _number(summary.get("mean_actions", 0))),
                _card("Maximum candidates", str(summary.get("maximum_candidates", 0))),
            )
        )
        + "</div>"
    )


def _core_table(views: dict[str, dict[str, Any]], focus: str) -> str:
    policies = [focus, *[policy for policy in views if policy != focus]] if focus else list(views)
    header = "".join(f"<th>{html.escape(_short_policy(policy))}</th>" for policy in policies)
    rows = []
    for label, key, kind in CORE_METRICS:
        cells = []
        for policy in policies:
            value = views[policy].get(key, 0)
            cells.append(f"<td>{_percent(float(value)) if kind == 'percent' else _number(value)}</td>")
        rows.append(f"<tr><th>{html.escape(label)}</th>{''.join(cells)}</tr>")
    return (
        f'<div class="table"><table><thead><tr><th>Metric</th>{header}</tr></thead>'
        f"<tbody>{''.join(rows)}</tbody></table></div>"
    )


def _comparison_table(
    current: dict[str, Any],
    global_view: dict[str, Any],
    previous: dict[str, Any] | None,
    family_best: dict[str, Any] | None,
    *,
    current_label: str,
    previous_label: str,
    best_label: str,
) -> str:
    headers = (current_label, previous_label, "Difference", best_label, "Difference", "Global champion")
    rows = []
    for label, key, kind in CORE_METRICS:
        value = float(current.get(key, 0))
        previous_value = float((previous or {}).get(key, 0)) if previous else None
        best_value = float((family_best or {}).get(key, 0)) if family_best else None
        formatter = _percent if kind == "percent" else _number
        rows.append(
            "<tr>"
            f"<th>{html.escape(label)}</th><td>{formatter(value)}</td>"
            f"<td>{formatter(previous_value) if previous_value is not None else '—'}</td>"
            f"<td>{_difference(value, previous_value, kind)}</td>"
            f"<td>{formatter(best_value) if best_value is not None else '—'}</td>"
            f"<td>{_difference(value, best_value, kind)}</td>"
            f"<td>{formatter(float(global_view.get(key, 0)))}</td></tr>"
        )
    return (
        '<div class="table"><table><thead><tr><th>Metric</th>'
        + "".join(f"<th>{html.escape(value)}</th>" for value in headers)
        + "</tr></thead><tbody>"
        + "".join(rows)
        + "</tbody></table></div>"
    )


def _policy_sections(policy: str, view: dict[str, Any], *, focus: bool) -> str:
    categories = {
        "Actions and passes": ("action_", "action_offered::", "passes", "segment_action::"),
        "Abilities": ("ability_", "commitment_tier_"),
        "Cards": ("card_",),
        "Dice and rerolls": ("reroll_", "qualified_", "rolls_", "mean_rolls_"),
        "Damage, healing, resources": (
            "actual_",
            "raw_",
            "resolved_",
            "offensive_",
            "average_damage",
            "average_actual_",
            "average_raw_",
            "average_resolved_",
            "energy_",
        ),
        "Statuses": ("status_", "average_status"),
    }
    sections = [
        f'<section class="policy {"focus" if focus else ""}"><h2>{html.escape(_short_policy(policy))}</h2>'
    ]
    ability_outcomes = _ability_outcome_table(view)
    if ability_outcomes:
        sections.append(ability_outcomes)
    damage_pipeline = _damage_pipeline_table(view)
    if damage_pipeline:
        sections.append(damage_pipeline)
    used: set[str] = set()
    for title, prefixes in categories.items():
        values = {
            key: value
            for key, value in view.items()
            if any(key == prefix or key.startswith(prefix) for prefix in prefixes)
        }
        used.update(values)
        if values:
            sections.append(_key_value_table(title, values))
    remaining = {
        key: value
        for key, value in view.items()
        if key not in used and key not in {"adjusted_score", "win_rate", "wins", "games"}
    }
    if remaining:
        sections.append(_key_value_table("Other behavior", remaining))
    sections.append("</section>")
    return "".join(sections)


def _ability_outcome_table(view: dict[str, Any]) -> str:
    pre_total = int(view.get("offensive_pre_reaction_outcomes", 0))
    post_total = int(view.get("offensive_post_reaction_outcomes", 0))
    if not pre_total and not post_total:
        return ""
    outcome_rows = (
        (
            "Ability selected",
            int(view.get("offensive_pre_reaction_ability_selected", 0)),
            float(view.get("offensive_pre_reaction_ability_rate", 0)),
            int(view.get("offensive_post_reaction_ability_selected", 0)),
            float(view.get("offensive_post_reaction_ability_rate", 0)),
        ),
        (
            "Passed",
            int(view.get("offensive_pre_reaction_passed", 0)),
            float(view.get("offensive_pre_reaction_pass_rate", 0)),
            int(view.get("offensive_post_reaction_passed", 0)),
            float(view.get("offensive_post_reaction_pass_rate", 0)),
        ),
    )
    outcome_body = "".join(
        "<tr>"
        f"<th>{html.escape(label)}</th><td>{before_count:,}</td><td>{_percent(before_rate)}</td>"
        f"<td>{after_count:,}</td><td>{_percent(after_rate)}</td></tr>"
        for label, before_count, before_rate, after_count, after_rate in outcome_rows
    )
    roll_body = "".join(
        "<tr>"
        f"<th>{label}</th>"
        f"<td>{int(view.get(f'offensive_pre_reaction_selected_roll_{roll}', 0)):,}</td>"
        f"<td>{_percent(float(view.get(f'offensive_selection_roll_{roll}_rate', 0)))}</td>"
        "</tr>"
        for roll, label in ((1, "First roll"), (2, "Second roll"), (3, "Third roll"))
    )
    changes = (
        ("Ability preserved", "offensive_reaction_ability_preserved"),
        ("Ability lost after reaction", "offensive_reaction_ability_lost"),
        ("Ability gained after initially passing", "offensive_reaction_ability_gained"),
        ("Ability switched", "offensive_reaction_ability_switched"),
        ("Pass preserved", "offensive_reaction_pass_preserved"),
    )
    change_body = "".join(
        f"<tr><th>{html.escape(label)}</th><td>{int(view.get(key, 0)):,}</td>"
        f"<td>{_percent(int(view.get(key, 0)) / post_total if post_total else 0)}</td></tr>"
        for label, key in changes
    )
    return (
        "<h3>Offensive ability decisions</h3>"
        '<p class="lede">The pre-reaction result is the creature’s locked choice after rolling. '
        "The post-reaction result records what remained after every offensive reaction resolved.</p>"
        '<div class="table"><table><thead><tr><th>Outcome</th><th>Before count</th>'
        "<th>Before rate</th><th>After count</th><th>After rate</th></tr></thead><tbody>"
        + outcome_body
        + "</tbody></table></div>"
        "<h4>Roll used when selecting an ability</h4>"
        '<div class="table"><table><thead><tr><th>Selection timing</th><th>Count</th>'
        "<th>Percent of ability selections</th></tr></thead><tbody>"
        + roll_body
        + "</tbody></table></div>"
        "<h4>Reaction changes</h4>"
        '<div class="table"><table><thead><tr><th>Result</th><th>Count</th>'
        "<th>Percent of completed reactions</th></tr></thead><tbody>"
        + change_body
        + "</tbody></table></div>"
    )


def _damage_pipeline_table(view: dict[str, Any]) -> str:
    if not int(view.get("rounds", 0)):
        return ""
    rows = (
        (
            "Attack before defense",
            "average_raw_outgoing_attack_per_round",
            "average_raw_incoming_attack_per_round",
        ),
        (
            "Bleed before defense",
            "average_raw_outgoing_bleed_per_round",
            "average_raw_incoming_bleed_per_round",
        ),
        (
            "Poison before defense",
            "average_raw_outgoing_poison_per_round",
            "average_raw_incoming_poison_per_round",
        ),
        (
            "Total before defense",
            "average_raw_outgoing_total_per_round",
            "average_raw_incoming_total_per_round",
        ),
        (
            "After prevention, scaling, and reactions",
            "average_resolved_outgoing_total_per_round",
            "average_resolved_incoming_total_per_round",
        ),
        (
            "Actual health removed",
            "average_actual_outgoing_total_per_round",
            "average_actual_incoming_total_per_round",
        ),
    )
    body = "".join(
        f"<tr><th>{html.escape(label)}</th><td>{_number(view.get(outgoing, 0))}</td>"
        f"<td>{_number(view.get(incoming, 0))}</td></tr>"
        for label, outgoing, incoming in rows
    )
    return (
        "<h3>Damage per round</h3>"
        '<p class="lede">Authority damage totals include direct attacks, bleed, and poison. Actual '
        "damage is the number of health cards removed after all defensive processing and overkill.</p>"
        '<div class="table"><table><thead><tr><th>Resolution point</th><th>Dealt</th>'
        "<th>Taken</th></tr></thead><tbody>"
        + body
        + "</tbody></table></div>"
    )


def _key_value_table(title: str, values: dict[str, Any]) -> str:
    rows = "".join(
        "<tr><td>"
        f"{html.escape(key.replace('::', ' · ').replace('_', ' '))}"
        f"</td><td>{_number(value)}</td></tr>"
        for key, value in sorted(values.items())
    )
    return f'<h3>{html.escape(title)}</h3><div class="table"><table><tbody>{rows}</tbody></table></div>'


def _document(title: str, body: str) -> str:
    return f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{html.escape(title)}</title>
<style>
:root {{
  --bg:#07111d; --panel:#101d2b; --line:#2b4258; --text:#eaf0f6;
  --muted:#9eb2c5; --gold:#f2c45e; --cyan:#69d7e8;
}}
* {{ box-sizing:border-box; }}
body {{ margin:0; background:var(--bg); color:var(--text); font:15px/1.5 system-ui,sans-serif; }}
main {{ width:min(1500px,calc(100% - 32px)); margin:auto; padding:42px 0 90px; }}
h1 {{ font-size:clamp(2rem,5vw,4.2rem); margin:.2em 0; }}
h2 {{ margin-top:2rem; }} h3 {{ color:var(--gold); }}
p,.lede {{ color:var(--muted); }} a {{ color:var(--cyan); }}
.cards {{
  display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr));
  gap:10px; margin:20px 0;
}}
.card,.policy {{
  border:1px solid var(--line); background:var(--panel); border-radius:14px; padding:18px;
}}
.card strong {{ display:block; font-size:1.5rem; }} .card span {{ color:var(--muted); }}
.policy {{ margin:18px 0; }} .policy.focus {{ border-color:var(--gold); }}
.table {{ overflow:auto; border:1px solid var(--line); border-radius:10px; margin:12px 0; }}
table {{ width:100%; border-collapse:collapse; }}
th,td {{ padding:9px 11px; border-bottom:1px solid var(--line); text-align:left; white-space:nowrap; }}
th {{ color:var(--muted); background:#0b1724; }} tr:last-child td {{ border-bottom:0; }}
small {{ display:block; color:var(--muted); }}
</style>
</head>
<body><main>{body}</main></body>
</html>"""


def _card(label: str, value: str) -> str:
    return f'<div class="card"><span>{html.escape(label)}</span><strong>{html.escape(value)}</strong></div>'


def _short_policy(policy: str) -> str:
    if not policy:
        return "Policy"
    return Path(policy.removeprefix("model:")).stem if policy.startswith("model:") else policy


def _percent(value: float | None) -> str:
    return "—" if value is None else f"{100 * value:.2f}%"


def _number(value: Any) -> str:
    if value is None:
        return "—"
    number = float(value)
    return f"{number:,.3f}" if not number.is_integer() else f"{int(number):,}"


def _optional_number(value: Any) -> str:
    return "—" if value is None else _number(value)


def _optional_signed_number(value: Any) -> str:
    return "—" if value is None else f"{float(value):+.3f}"


def _optional_integer(value: Any) -> str:
    return "—" if value is None else f"{int(value):,}"


def _optional_percent(value: Any) -> str:
    return "—" if value is None else _percent(float(value))


def _duration(seconds: float) -> str:
    total_seconds = max(float(seconds), 0.0)
    minutes, remainder = divmod(total_seconds, 60)
    hours, minutes = divmod(int(minutes), 60)
    if hours:
        return f"{hours}h {minutes:02d}m {remainder:04.1f}s"
    return f"{minutes}m {remainder:04.1f}s"


def _signed_pp(value: float | None) -> str:
    return "—" if value is None else f"{100 * value:+.2f} pp"


def _difference(value: float, baseline: float | None, kind: str) -> str:
    if baseline is None:
        return "—"
    difference = value - baseline
    return _signed_pp(difference) if kind == "percent" else f"{difference:+.3f}"
