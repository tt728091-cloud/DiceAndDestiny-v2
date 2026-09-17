from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

from .champions import atomic_json_write, canonical_hash
from .manifest_v3 import EntityRange, ObservationLayoutV3, ObservationManifestV3
from .manifest_v4 import (
    DETAIL_FEATURES_V4,
    ObservationLayoutV4,
    build_observation_manifest_v4,
)

MANIFEST_SCHEMA_V5 = "dice-and-destiny-observation-v5-manifest-v1"
OBSERVATION_SCHEMA_V5 = "dice-and-destiny-observation-v5"
ACTION_SCHEMA_V5 = "dice-and-destiny-action-candidates-v5"
ENVIRONMENT_SCHEMA_V5 = "dice-and-destiny-ml-env-v5"
SEMANTIC_CANDIDATE_SCHEMA_V5 = "dice-and-destiny-semantic-candidate-v1"


@dataclass(frozen=True)
class ObservationManifestV5:
    """Frozen V5 contract.

    V5 deliberately retains V4's proven, fail-loud public layout while changing
    the meaning of candidate fingerprints and the policy that consumes detail
    rows.  A V4 checkpoint can therefore never be silently loaded as V5.
    """

    schema: str
    observation_schema: str
    action_schema: str
    environment_schema: str
    semantic_candidate_schema: str
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
    def load(cls, path: Path) -> ObservationManifestV5:
        raw = json.loads(path.read_text())
        if raw.get("schema") != MANIFEST_SCHEMA_V5:
            raise RuntimeError(f"unsupported observation manifest {raw.get('schema')!r}")
        base_raw = raw["base"]
        base_layout_raw = base_raw["layout"]
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
        if (
            self.schema != MANIFEST_SCHEMA_V5
            or self.observation_schema != OBSERVATION_SCHEMA_V5
            or self.action_schema != ACTION_SCHEMA_V5
            or self.environment_schema != ENVIRONMENT_SCHEMA_V5
            or self.semantic_candidate_schema != SEMANTIC_CANDIDATE_SCHEMA_V5
        ):
            raise RuntimeError("unsupported observation v5 manifest")
        self.base.validate()
        self.layout.validate()
        if self.layout.details.rows != self.detail_rows:
            raise RuntimeError("v5 detail layout does not match declared capacity")
        if self.layout.details.features != DETAIL_FEATURES_V4:
            raise RuntimeError("v5 detail feature width changed without a new layout")
        for name, table in (("path", self.content_path_hashes), ("value", self.content_value_hashes)):
            if len(table) != len(set(table.values())):
                raise RuntimeError(f"v5 {name} hash collision in frozen content")
        raw = asdict(self)
        declared = raw.pop("manifest_sha256")
        if canonical_hash(raw) != declared:
            raise RuntimeError("observation v5 manifest hash mismatch")


def build_observation_manifest_v5(
    content_root: Path,
    *,
    eligible_combatants: list[str] | None = None,
    observed_candidate_maximum: int = 0,
    generated_scenarios: int = 0,
) -> ObservationManifestV5:
    v4 = build_observation_manifest_v4(
        content_root,
        eligible_combatants=eligible_combatants,
        observed_candidate_maximum=observed_candidate_maximum,
        generated_scenarios=generated_scenarios,
    )
    raw: dict[str, Any] = {
        "schema": MANIFEST_SCHEMA_V5,
        "observation_schema": OBSERVATION_SCHEMA_V5,
        "action_schema": ACTION_SCHEMA_V5,
        "environment_schema": ENVIRONMENT_SCHEMA_V5,
        "semantic_candidate_schema": SEMANTIC_CANDIDATE_SCHEMA_V5,
        "base": v4.base,
        "layout": v4.layout,
        "detail_rows": v4.detail_rows,
        "hash_bits": v4.hash_bits,
        "content_path_hashes": v4.content_path_hashes,
        "content_value_hashes": v4.content_value_hashes,
        "capacity_evidence": {
            **v4.capacity_evidence,
            "candidate_identity": "canonical semantic action; runtime identifiers removed or resolved",
            "policy_consumption": "candidate-isolated learned detail encoding and typed mean/max pooling",
        },
    }
    raw["manifest_sha256"] = canonical_hash(
        {
            key: asdict(value) if hasattr(value, "__dataclass_fields__") else value
            for key, value in raw.items()
        }
    )
    result = ObservationManifestV5(**raw)
    result.validate()
    return result
