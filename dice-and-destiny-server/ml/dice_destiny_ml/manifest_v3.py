from __future__ import annotations

import hashlib
import json
import math
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import yaml

from .champions import atomic_json_write, canonical_hash

MANIFEST_SCHEMA_V3 = "dice-and-destiny-observation-v3-manifest-v1"
OBSERVATION_SCHEMA_V3 = "dice-and-destiny-observation-v3"
ACTION_SCHEMA_V3 = "dice-and-destiny-action-candidates-v3"
ENVIRONMENT_SCHEMA_V3 = "dice-and-destiny-ml-env-v3"

CONTEXT_FEATURES_V3 = 64
ACTOR_FEATURES_V3 = 32
DIE_FEATURES_V3 = 24
ABILITY_FEATURES_V3 = 64
STATUS_FEATURES_V3 = 64
CARD_FEATURES_V3 = 48
ACTION_FEATURES_V3 = 96

COMMAND_TYPES_V3 = (
    "planning_roll",
    "planning_keep",
    "planning_reroll",
    "planning_commit_cards",
    "planning_select_ability",
    "planning_select_targets",
    "planning_pass",
    "roll_dice",
    "commit_interaction",
    "pass",
)


@dataclass(frozen=True)
class EntityRange:
    offset: int
    rows: int
    features: int

    @property
    def size(self) -> int:
        return self.rows * self.features

    @property
    def end(self) -> int:
        return self.offset + self.size


@dataclass(frozen=True)
class ObservationLayoutV3:
    context: EntityRange
    actors: EntityRange
    dice: EntityRange
    abilities: EntityRange
    statuses: EntityRange
    cards: EntityRange
    candidates: EntityRange
    observation_size: int

    def validate(self) -> None:
        ranges = (
            self.context,
            self.actors,
            self.dice,
            self.abilities,
            self.statuses,
            self.cards,
            self.candidates,
        )
        expected = 0
        for value in ranges:
            if value.offset != expected:
                raise RuntimeError(
                    f"observation v3 layout gap/overlap at {value.offset}; expected {expected}"
                )
            if value.rows < 1 or value.features < 1:
                raise RuntimeError("observation v3 ranges must have positive dimensions")
            expected = value.end
        if expected != self.observation_size:
            raise RuntimeError(
                f"observation v3 size mismatch: ranges end at {expected}, declared {self.observation_size}"
            )


@dataclass(frozen=True)
class ObservationManifestV3:
    schema: str
    observation_schema: str
    action_schema: str
    environment_schema: str
    content_sha256: str
    eligible_combatants: tuple[str, ...]
    combatant_vocabulary: tuple[str, ...]
    form_vocabulary: tuple[str, ...]
    symbol_vocabulary: tuple[str, ...]
    die_vocabulary: tuple[str, ...]
    ability_vocabulary: tuple[str, ...]
    status_vocabulary: tuple[str, ...]
    card_vocabulary: tuple[str, ...]
    operation_vocabulary: tuple[str, ...]
    command_vocabulary: tuple[str, ...]
    maximum_dice_per_actor: int
    maximum_active_abilities_per_actor: int
    maximum_passives_per_actor: int
    maximum_statuses_per_actor: int
    maximum_tokens_per_actor: int
    maximum_visible_hand_cards: int
    maximum_tiers_per_ability: int
    maximum_requirements_per_tier: int
    maximum_targets: int
    maximum_legal_candidates: int
    normalization: dict[str, float]
    layout: ObservationLayoutV3
    capacity_evidence: dict[str, Any]
    manifest_sha256: str = ""

    @classmethod
    def load(cls, path: Path) -> ObservationManifestV3:
        raw = json.loads(path.read_text())
        if raw.get("schema") != MANIFEST_SCHEMA_V3:
            raise RuntimeError(f"unsupported observation manifest {raw.get('schema')!r}")
        layout = ObservationLayoutV3(
            **{
                key: EntityRange(**raw["layout"][key])
                for key in (
                    "context",
                    "actors",
                    "dice",
                    "abilities",
                    "statuses",
                    "cards",
                    "candidates",
                )
            },
            observation_size=int(raw["layout"]["observation_size"]),
        )
        tuple_fields = (
            "eligible_combatants",
            "combatant_vocabulary",
            "form_vocabulary",
            "symbol_vocabulary",
            "die_vocabulary",
            "ability_vocabulary",
            "status_vocabulary",
            "card_vocabulary",
            "operation_vocabulary",
            "command_vocabulary",
        )
        for key in tuple_fields:
            raw[key] = tuple(raw[key])
        raw["layout"] = layout
        manifest = cls(**raw)
        manifest.validate()
        return manifest

    def save(self, path: Path) -> None:
        self.validate()
        atomic_json_write(path, asdict(self))

    def validate(self) -> None:
        if self.schema != MANIFEST_SCHEMA_V3:
            raise RuntimeError(f"unsupported observation manifest {self.schema!r}")
        self.layout.validate()
        if self.layout.candidates.rows != self.maximum_legal_candidates:
            raise RuntimeError("candidate layout does not match manifest capacity")
        for field in (
            "eligible_combatants",
            "combatant_vocabulary",
            "symbol_vocabulary",
            "die_vocabulary",
            "ability_vocabulary",
            "status_vocabulary",
            "card_vocabulary",
            "operation_vocabulary",
            "command_vocabulary",
        ):
            values = getattr(self, field)
            if tuple(sorted(set(values))) != values:
                raise RuntimeError(f"manifest vocabulary {field} is not sorted and unique")
        if tuple(self.command_vocabulary) != tuple(sorted(COMMAND_TYPES_V3)):
            raise RuntimeError("manifest command vocabulary is not the supported v3 set")
        value = asdict(self)
        declared = value.pop("manifest_sha256")
        actual = canonical_hash(value)
        if declared != actual:
            raise RuntimeError(f"observation manifest hash mismatch: {actual}, expected {declared}")


def build_observation_manifest_v3(
    content_root: Path,
    *,
    eligible_combatants: list[str] | None = None,
    observed_candidate_maximum: int = 0,
    generated_scenarios: int = 0,
) -> ObservationManifestV3:
    root = content_root.resolve()
    documents = _load_documents(root)
    combatants = documents["combatants"]
    eligible = tuple(sorted(eligible_combatants or combatants))
    missing = sorted(set(eligible) - set(combatants))
    if missing:
        raise ValueError(f"eligible combatants are missing from content: {missing}")
    selected = [combatants[identifier] for identifier in eligible]
    symbols = tuple(sorted(documents["symbols"]))
    dice = tuple(sorted(documents["dice"]))
    abilities = tuple(sorted(documents["abilities"]))
    statuses = tuple(sorted(documents["statuses"]))
    cards = tuple(sorted(documents["cards"]))
    forms = tuple(
        sorted(
            {
                str(form.get("id"))
                for combatant in selected
                for form in combatant.get("forms") or []
                if form.get("id")
            }
        )
    )
    max_dice = max(
        sum(int(entry.get("count", 0)) for entry in combatant.get("dice_loadout") or [])
        for combatant in selected
    )
    max_active_abilities = max(_simultaneous_ability_count(combatant) for combatant in selected)
    max_passives = max((len(combatant.get("passives") or []) for combatant in selected), default=0)
    max_hand_limit = max(
        int((combatant.get("resources") or {}).get("hand_limit", 0)) for combatant in selected
    )
    # A hand-limit checkpoint is opened only after cards have entered the hand,
    # so the authored hand limit is not an encoding capacity.  Reserve enough
    # rows for the largest complete eligible deck: this is a content-derived,
    # fail-loud upper bound even if a future effect moves several cards before
    # the checkpoint is resolved.
    max_hand = max(
        sum(int(entry.get("count", 0)) for entry in combatant.get("decklist") or []) for combatant in selected
    )
    max_statuses = len(statuses)
    max_tokens = max(
        max((len(combatant.get("starting_tokens") or []) for combatant in selected), default=0),
        1,
    )
    all_abilities = documents["abilities"]
    modifier_tiers = _ability_modifier_bonus_tiers(documents["cards"])
    tiers = [
        tier
        for definition in all_abilities.values()
        for tier in [
            *((definition.get("qualification") or {}).get("activation_tiers") or []),
            *((definition.get("qualification") or {}).get("conditional_bonuses") or []),
        ]
    ] + list(modifier_tiers.values())
    modifier_bonus_ids = set(modifier_tiers)
    max_tiers = max(
        (
            len((definition.get("qualification") or {}).get("activation_tiers") or [])
            + len((definition.get("qualification") or {}).get("conditional_bonuses") or [])
            + len(modifier_bonus_ids)
            for definition in all_abilities.values()
        ),
        default=1,
    )
    max_requirements = max(
        (len((tier.get("requirements") or {}).get("all") or []) for tier in tiers),
        default=1,
    )
    max_targets = max(
        (
            int((definition.get("targeting") or {}).get("maximum", 0))
            for group in (documents["abilities"], documents["cards"])
            for definition in group.values()
        ),
        default=1,
    )
    derived_candidate_bound = _candidate_upper_bound(
        max_dice=max_dice,
        max_hand=max_hand,
        max_active_abilities=max_active_abilities,
        max_statuses=max_statuses,
        max_targets=max_targets,
    )
    candidate_capacity = _next_power_of_two(max(derived_candidate_bound, observed_candidate_maximum, 1))
    if candidate_capacity > 4096:
        raise RuntimeError(
            f"derived legal-candidate capacity {candidate_capacity} is implausibly large; "
            "review content before training"
        )
    operation_vocabulary = tuple(sorted(_operation_types(documents)))
    if not operation_vocabulary:
        raise RuntimeError("content manifest found no supported operations")
    max_health = max(
        sum(int(entry.get("count", 0)) for entry in combatant.get("decklist") or []) for combatant in selected
    )
    max_energy = max(
        max_health * int((combatant.get("income") or {}).get("energy", 0))
        + int((combatant.get("resources") or {}).get("starting_energy", 0))
        for combatant in selected
    )
    max_stack = max(
        int((definition.get("stacking") or {}).get("stack_limit", 1))
        for definition in documents["statuses"].values()
    )
    max_face = max(
        int(face.get("number", 0))
        for definition in documents["dice"].values()
        for face in definition.get("faces") or []
    )
    layout = _build_layout(
        max_dice=max_dice,
        max_abilities=max_active_abilities,
        max_statuses=max_statuses,
        max_hand=max_hand,
        max_candidates=candidate_capacity,
    )
    content_digest = hashlib.sha256()
    for path in sorted(root.rglob("*.yaml")):
        content_digest.update(path.relative_to(root).as_posix().encode())
        content_digest.update(path.read_bytes())
    raw: dict[str, Any] = {
        "schema": MANIFEST_SCHEMA_V3,
        "observation_schema": OBSERVATION_SCHEMA_V3,
        "action_schema": ACTION_SCHEMA_V3,
        "environment_schema": ENVIRONMENT_SCHEMA_V3,
        "content_sha256": content_digest.hexdigest(),
        "eligible_combatants": eligible,
        "combatant_vocabulary": tuple(sorted(combatants)),
        "form_vocabulary": forms,
        "symbol_vocabulary": symbols,
        "die_vocabulary": dice,
        "ability_vocabulary": abilities,
        "status_vocabulary": statuses,
        "card_vocabulary": cards,
        "operation_vocabulary": operation_vocabulary,
        "command_vocabulary": tuple(sorted(COMMAND_TYPES_V3)),
        "maximum_dice_per_actor": max_dice,
        "maximum_active_abilities_per_actor": max_active_abilities,
        "maximum_passives_per_actor": max_passives,
        "maximum_statuses_per_actor": max_statuses,
        "maximum_tokens_per_actor": max_tokens,
        "maximum_visible_hand_cards": max_hand,
        "maximum_tiers_per_ability": max_tiers,
        "maximum_requirements_per_tier": max_requirements,
        "maximum_targets": max(max_targets, 1),
        "maximum_legal_candidates": candidate_capacity,
        "normalization": {
            "health": float(max(max_health, 1)),
            "energy": float(max(max_energy, 1)),
            "hand": float(max(max_hand, 1)),
            "deck": float(max(max_health, 1)),
            "dice": float(max(max_dice, 1)),
            "abilities": float(max(max_active_abilities, 1)),
            "statuses": float(max(max_statuses, 1)),
            "stacks": float(max(max_stack, 1)),
            "face": float(max(max_face, 1)),
            "targets": float(max(max_targets, 1)),
            "round": 50.0,
            "rolls": 3.0,
        },
        "layout": layout,
        "capacity_evidence": {
            "method": "content combinatorial upper bound plus generated/adversarial observation",
            "derived_candidate_upper_bound": derived_candidate_bound,
            "observed_candidate_maximum": int(observed_candidate_maximum),
            "generated_scenarios": int(generated_scenarios),
            "reserved_candidate_capacity": candidate_capacity,
            "maximum_authored_hand_limit": max_hand_limit,
            "maximum_complete_deck_cards": max_hand,
            "runtime_ability_modifier_bonus_ids": sorted(modifier_bonus_ids),
            "visible_hand_rule": "reserve the complete eligible deck, not the pre-checkpoint hand limit",
            "exclusive_form_rule": "maximum active form board, never sum mutually exclusive boards",
        },
    }
    raw["manifest_sha256"] = canonical_hash(
        {key: asdict(value) if key == "layout" else value for key, value in raw.items()}
    )
    manifest = ObservationManifestV3(**raw)
    manifest.validate()
    return manifest


def _build_layout(
    *, max_dice: int, max_abilities: int, max_statuses: int, max_hand: int, max_candidates: int
) -> ObservationLayoutV3:
    offset = 0

    def allocate(rows: int, features: int) -> EntityRange:
        nonlocal offset
        value = EntityRange(offset, max(rows, 1), features)
        offset = value.end
        return value

    context = allocate(1, CONTEXT_FEATURES_V3)
    actors = allocate(2, ACTOR_FEATURES_V3)
    dice = allocate(2 * max_dice, DIE_FEATURES_V3)
    abilities = allocate(2 * max_abilities, ABILITY_FEATURES_V3)
    statuses = allocate(2 * max_statuses, STATUS_FEATURES_V3)
    cards = allocate(max_hand, CARD_FEATURES_V3)
    candidates = allocate(max_candidates, ACTION_FEATURES_V3)
    result = ObservationLayoutV3(
        context=context,
        actors=actors,
        dice=dice,
        abilities=abilities,
        statuses=statuses,
        cards=cards,
        candidates=candidates,
        observation_size=offset,
    )
    result.validate()
    return result


def _load_documents(root: Path) -> dict[str, dict[str, dict[str, Any]]]:
    result: dict[str, dict[str, dict[str, Any]]] = {
        "combatants": {},
        "dice": {},
        "cards": {},
        "abilities": {},
        "statuses": {},
        "symbols": {},
    }
    symbols_path = root / "symbols.yaml"
    symbols_document = yaml.safe_load(symbols_path.read_text())
    for value in symbols_document.get("symbols") or []:
        result["symbols"][str(value["id"])] = value
    for group in ("combatants", "dice", "cards", "abilities", "statuses"):
        for path in sorted((root / group).glob("*.yaml")):
            value = yaml.safe_load(path.read_text())
            identifier = str(value.get("id", ""))
            if not identifier:
                raise RuntimeError(f"content file has no id: {path}")
            if identifier in result[group]:
                raise RuntimeError(f"duplicate content id {identifier!r} in {group}")
            result[group][identifier] = value
    return result


def _simultaneous_ability_count(combatant: dict[str, Any]) -> int:
    base = combatant.get("ability_board") or {}
    base_count = len(base.get("offensive") or []) + len(base.get("defensive") or [])
    form_counts = []
    for form in combatant.get("forms") or []:
        board = form.get("ability_board") or {}
        form_counts.append(len(board.get("offensive") or []) + len(board.get("defensive") or []))
    # Exclusive forms replace the base/other boards. Persistent passives are
    # represented separately and therefore do not inflate active ability rows.
    return max([base_count, *form_counts, 1])


def _candidate_upper_bound(
    *, max_dice: int, max_hand: int, max_active_abilities: int, max_statuses: int, max_targets: int
) -> int:
    die_subsets = 2 * (2**max_dice)
    # Planning/reaction actions expose one card instance per candidate.  Their
    # selector fans out over one of dice, abilities, statuses, or targets.  The
    # hand-limit checkpoint is settled after each bounded card inflow, rather
    # than exposing arbitrary power-set card commitments.
    card_subsets = max_hand * max(
        max_dice,
        max_active_abilities,
        max_statuses,
        max_targets,
        1,
    )
    ability_targets = max_active_abilities * max(max_targets, 1)
    status_choices = max_statuses * max(max_targets, 1)
    return len(COMMAND_TYPES_V3) + die_subsets + card_subsets + ability_targets + status_choices


def _next_power_of_two(value: int) -> int:
    return 1 << max(0, math.ceil(math.log2(max(value, 1))))


def _operation_types(documents: dict[str, dict[str, dict[str, Any]]]) -> set[str]:
    result: set[str] = set()

    def visit(value: Any, *, inside_operations: bool = False) -> None:
        if isinstance(value, list):
            for child in value:
                visit(child, inside_operations=inside_operations)
        elif isinstance(value, dict):
            if inside_operations and isinstance(value.get("type"), str):
                result.add(value["type"])
            for key, child in value.items():
                visit(child, inside_operations=inside_operations or key == "operations")

    for group in ("abilities", "cards", "statuses"):
        visit(documents[group])
    return result


def _ability_modifier_bonus_tiers(
    cards: dict[str, dict[str, Any]],
) -> dict[str, dict[str, Any]]:
    """Return unique runtime tiers that authored cards can add to an ability.

    The authority merges repeated applications of the same bonus id but keeps
    distinct ids, so every distinct authored id is a conservative per-ability
    addition to the frozen tier capacity.
    """
    result: dict[str, dict[str, Any]] = {}
    for card in cards.values():
        for operation in card.get("operations") or []:
            bonus = (operation.get("modifier") or {}).get("add_conditional_bonus") or {}
            if bonus.get("id"):
                result.setdefault(str(bonus["id"]), bonus)
    return result
