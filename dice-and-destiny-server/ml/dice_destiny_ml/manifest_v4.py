from __future__ import annotations

import hashlib
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import yaml

from .champions import atomic_json_write, canonical_hash
from .manifest_v3 import EntityRange, ObservationManifestV3, build_observation_manifest_v3

MANIFEST_SCHEMA_V4 = "dice-and-destiny-observation-v4-manifest-v1"
OBSERVATION_SCHEMA_V4 = "dice-and-destiny-observation-v4"
ACTION_SCHEMA_V4 = "dice-and-destiny-action-candidates-v4"
ENVIRONMENT_SCHEMA_V4 = "dice-and-destiny-ml-env-v4"
DETAIL_FEATURES_V4 = 96
DETAIL_ROWS_V4 = 2048


@dataclass(frozen=True)
class ObservationLayoutV4:
    context: EntityRange
    actors: EntityRange
    dice: EntityRange
    abilities: EntityRange
    statuses: EntityRange
    cards: EntityRange
    candidates: EntityRange
    details: EntityRange
    observation_size: int

    def validate(self) -> None:
        expected = 0
        for value in (
            self.context,
            self.actors,
            self.dice,
            self.abilities,
            self.statuses,
            self.cards,
            self.candidates,
            self.details,
        ):
            if value.offset != expected:
                raise RuntimeError(
                    f"observation v4 layout gap/overlap at {value.offset}; expected {expected}"
                )
            expected = value.end
        if expected != self.observation_size:
            raise RuntimeError(f"observation v4 ranges end at {expected}, declared {self.observation_size}")


@dataclass(frozen=True)
class ObservationManifestV4:
    schema: str
    observation_schema: str
    action_schema: str
    environment_schema: str
    base: ObservationManifestV3
    layout: ObservationLayoutV4
    detail_rows: int
    hash_bits: int
    content_path_hashes: dict[str, str]
    content_value_hashes: dict[str, str]
    capacity_evidence: dict[str, Any]
    manifest_sha256: str = ""

    @property
    def maximum_legal_candidates(self) -> int:
        return self.base.maximum_legal_candidates

    @classmethod
    def load(cls, path: Path) -> ObservationManifestV4:
        raw = json.loads(path.read_text())
        if raw.get("schema") != MANIFEST_SCHEMA_V4:
            raise RuntimeError(f"unsupported observation manifest {raw.get('schema')!r}")
        base_raw = raw["base"]
        base_layout_raw = base_raw["layout"]
        from .manifest_v3 import ObservationLayoutV3

        base_raw["layout"] = ObservationLayoutV3(
            **{
                key: EntityRange(**base_layout_raw[key])
                for key in ("context", "actors", "dice", "abilities", "statuses", "cards", "candidates")
            },
            observation_size=int(base_layout_raw["observation_size"]),
        )
        for key in (
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
        ):
            base_raw[key] = tuple(base_raw[key])
        raw["base"] = ObservationManifestV3(**base_raw)
        layout_raw = raw["layout"]
        raw["layout"] = ObservationLayoutV4(
            **{
                key: EntityRange(**layout_raw[key])
                for key in (
                    "context",
                    "actors",
                    "dice",
                    "abilities",
                    "statuses",
                    "cards",
                    "candidates",
                    "details",
                )
            },
            observation_size=int(layout_raw["observation_size"]),
        )
        value = cls(**raw)
        value.validate()
        return value

    def save(self, path: Path) -> None:
        self.validate()
        atomic_json_write(path, asdict(self))

    def validate(self) -> None:
        if self.schema != MANIFEST_SCHEMA_V4 or self.observation_schema != OBSERVATION_SCHEMA_V4:
            raise RuntimeError("unsupported observation v4 manifest")
        self.base.validate()
        self.layout.validate()
        if self.layout.details.rows != self.detail_rows or self.layout.details.features != DETAIL_FEATURES_V4:
            raise RuntimeError("v4 detail layout does not match declared capacity")
        for name, table in (("path", self.content_path_hashes), ("value", self.content_value_hashes)):
            if len(table) != len(set(table.values())):
                raise RuntimeError(f"v4 {name} hash collision in frozen content")
        raw = asdict(self)
        declared = raw.pop("manifest_sha256")
        if canonical_hash(raw) != declared:
            raise RuntimeError("observation v4 manifest hash mismatch")


def build_observation_manifest_v4(
    content_root: Path,
    *,
    eligible_combatants: list[str] | None = None,
    observed_candidate_maximum: int = 0,
    generated_scenarios: int = 0,
) -> ObservationManifestV4:
    base = build_observation_manifest_v3(
        content_root,
        eligible_combatants=eligible_combatants,
        observed_candidate_maximum=observed_candidate_maximum,
        generated_scenarios=generated_scenarios,
    )
    paths: set[str] = set()
    values: set[str] = set()
    for path in sorted(content_root.resolve().rglob("*.yaml")):
        _collect_tokens(yaml.safe_load(path.read_text()), "content", paths, values)
    # Runtime-only authority vocabulary. Content fields are discovered, while
    # authority identifiers and values remain data driven and are hashed at encode time.
    values.update(("seat-a", "seat-b", "income", "offensive", "defensive", "ongoing_effects"))
    path_hashes = _collision_checked_hashes(paths)
    value_hashes = _collision_checked_hashes(values)
    base_layout = base.layout
    details = EntityRange(base_layout.observation_size, DETAIL_ROWS_V4, DETAIL_FEATURES_V4)
    layout = ObservationLayoutV4(
        context=base_layout.context,
        actors=base_layout.actors,
        dice=base_layout.dice,
        abilities=base_layout.abilities,
        statuses=base_layout.statuses,
        cards=base_layout.cards,
        candidates=base_layout.candidates,
        details=details,
        observation_size=details.end,
    )
    raw: dict[str, Any] = {
        "schema": MANIFEST_SCHEMA_V4,
        "observation_schema": OBSERVATION_SCHEMA_V4,
        "action_schema": ACTION_SCHEMA_V4,
        "environment_schema": ENVIRONMENT_SCHEMA_V4,
        "base": base,
        "layout": layout,
        "detail_rows": DETAIL_ROWS_V4,
        "hash_bits": 32,
        "content_path_hashes": path_hashes,
        "content_value_hashes": value_hashes,
        "capacity_evidence": {
            "method": "complete live-state and linked-content tokenization with fail-loud row capacity",
            "detail_rows": DETAIL_ROWS_V4,
            "computed_reroll_probabilities": (
                "deferred by product decision; raw die faces and requirements are encoded"
            ),
            "authority_candidates": base.maximum_legal_candidates,
        },
    }
    raw["manifest_sha256"] = canonical_hash(
        {
            key: asdict(value) if hasattr(value, "__dataclass_fields__") else value
            for key, value in raw.items()
        }
    )
    result = ObservationManifestV4(**raw)
    result.validate()
    return result


def stable_hash(value: str) -> str:
    return hashlib.sha256(value.encode()).hexdigest()[:8]


def _collision_checked_hashes(values: set[str]) -> dict[str, str]:
    result = {value: stable_hash(value) for value in sorted(values)}
    reverse: dict[str, str] = {}
    for value, digest in result.items():
        if digest in reverse and reverse[digest] != value:
            raise RuntimeError(f"frozen token hash collision: {reverse[digest]!r} and {value!r}")
        reverse[digest] = value
    return result


def _collect_tokens(value: Any, path: str, paths: set[str], values: set[str]) -> None:
    paths.add(path)
    if isinstance(value, dict):
        for key, child in value.items():
            _collect_tokens(child, f"{path}.{key}", paths, values)
    elif isinstance(value, list):
        for child in value:
            _collect_tokens(child, f"{path}[]", paths, values)
    elif isinstance(value, str):
        values.add(value)
