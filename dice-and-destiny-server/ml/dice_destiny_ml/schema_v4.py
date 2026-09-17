from __future__ import annotations

import json
import math
from dataclasses import dataclass
from typing import Any

import numpy as np

from .champions import canonical_hash
from .manifest_v4 import ObservationManifestV4, stable_hash
from .schema_v2 import SchemaEncoderV2, parse_payload_v2
from .schema_v3 import SchemaEncoderV3


@dataclass(frozen=True)
class EncodedDecisionV4:
    observation: np.ndarray
    action_mask: np.ndarray
    authority_indices: np.ndarray


class SchemaEncoderV4:
    """Complete, viewer-safe model view layered over the frozen V3 board encoding.

    V4 preserves every live authority field and every field of content linked by
    the current state/actions as typed token rows. Exact action payloads receive
    a collision-audited fingerprint and map back to their authority indices.
    """

    def __init__(self, manifest: ObservationManifestV4) -> None:
        manifest.validate()
        self.manifest = manifest
        self._base = SchemaEncoderV3(manifest.base)
        self._runtime_hashes: dict[tuple[str, str], str] = {}

    def encode(self, transition: dict[str, Any]) -> EncodedDecisionV4:
        # Go currently transports V2 for mixed-family battles. V4 is encoded
        # from the same viewer-safe raw authority snapshot, not from V2.
        raw_transition = dict(transition)
        raw_transition.pop("encoded_decision", None)
        base = self._base.encode(raw_transition)
        result = transition.get("result") or {}
        snapshot = result.get("snapshot") or {}
        actions = list(result.get("legal_actions") or [])
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        actors = snapshot.get("actors") or {}
        own = actors.get(viewer) or {}
        authority_indices = self._compact_authority_indices(
            actions, own, snapshot.get("content_catalog") or {}
        )
        if len(authority_indices) > self.manifest.maximum_legal_candidates:
            raise RuntimeError("compacted v4 actions exceed frozen candidate capacity")

        observation = np.zeros(self.manifest.layout.observation_size, dtype=np.float32)
        candidate_offset = self.manifest.layout.candidates.offset
        observation[:candidate_offset] = base.observation[:candidate_offset]
        candidate_width = self.manifest.layout.candidates.features
        for model_index, authority_index in enumerate(authority_indices):
            source = candidate_offset + authority_index * candidate_width
            target = candidate_offset + model_index * candidate_width
            observation[target : target + candidate_width] = base.observation[
                source : source + candidate_width
            ]
            fingerprint = canonical_hash(
                {
                    "type": actions[authority_index].get("type"),
                    "payload": parse_payload_v2(actions[authority_index].get("payload")),
                }
            )
            self._write_bits(observation[target : target + candidate_width], 64, int(fingerprint[:8], 16), 32)

        mask = np.zeros(self.manifest.maximum_legal_candidates, dtype=bool)
        mask[: len(authority_indices)] = True
        self._encode_details(observation, transition, snapshot, actions, authority_indices, viewer)
        if not mask.any() and not transition.get("terminal") and not transition.get("truncation_reason"):
            raise RuntimeError("nonterminal v4 decision has no progressing model actions")
        return EncodedDecisionV4(observation, mask, np.asarray(authority_indices, dtype=np.int32))

    def authority_index(self, decision: EncodedDecisionV4, model_index: int) -> int:
        if not 0 <= model_index < len(decision.authority_indices):
            raise RuntimeError(f"v4 model action {model_index} has no authority mapping")
        return int(decision.authority_indices[model_index])

    def _compact_authority_indices(
        self, actions: list[dict[str, Any]], actor: dict[str, Any], catalog: dict[str, Any]
    ) -> list[int]:
        dice = (actor.get("dice") or {}).get("dice") or []
        all_indices = {int(die.get("index", slot)) for slot, die in enumerate(dice)}
        kept = {int(value) for value in (actor.get("dice") or {}).get("kept_indices") or []}
        complement_seen = False
        result: list[int] = []
        for index, action in enumerate(actions):
            kind = str(action.get("type", ""))
            payload = parse_payload_v2(action.get("payload"))
            if kind == "planning_keep":
                selected = {int(value) for value in payload.get("kept_indices") or []}
                if not kept.issubset(selected) or len(selected - kept) != 1:
                    continue
            elif kind == "planning_reroll":
                selected = {int(value) for value in payload.get("reroll_indices") or []}
                if selected != all_indices - kept or complement_seen:
                    continue
                complement_seen = True
            elif not SchemaEncoderV2._is_progressing_action(action, kept, actor, catalog):
                continue
            result.append(index)
        return result

    def _encode_details(
        self,
        observation: np.ndarray,
        transition: dict[str, Any],
        snapshot: dict[str, Any],
        actions: list[dict[str, Any]],
        authority_indices: list[int],
        viewer: str,
    ) -> None:
        catalog = snapshot.get("content_catalog") or {}
        actors = snapshot.get("actors") or {}
        opponent = "seat-b" if viewer == "seat-a" else "seat-a"
        rows: list[tuple[str, int, str, Any, str]] = []

        context = {key: value for key, value in snapshot.items() if key not in {"actors", "content_catalog"}}
        self._flatten(rows, "context", -1, "snapshot", context, "")
        for owner, actor_id in (("own", viewer), ("opponent", opponent)):
            actor_view = dict(actors.get(actor_id) or {})
            if owner == "opponent":
                for hidden in (
                    "hand",
                    "card_instances",
                    "decklist",
                    "deck_composition",
                    "discard_composition",
                    "removed_composition",
                    "roll_preferences",
                ):
                    actor_view.pop(hidden, None)
            self._flatten(rows, "actor", -1, "actor", actor_view, owner)

        linked: dict[tuple[str, str], dict[str, Any]] = {}
        for actor in actors.values():
            for identifier in (
                list(actor.get("offensive_abilities") or [])
                + list(actor.get("defensive_abilities") or [])
                + list(actor.get("passive_abilities") or [])
            ):
                if identifier in (catalog.get("abilities") or {}):
                    linked[("ability", str(identifier))] = (catalog.get("abilities") or {})[identifier]
            for status in actor.get("statuses") or []:
                identifier = str(status.get("definition_id", ""))
                if identifier in (catalog.get("statuses") or {}):
                    linked[("status", identifier)] = (catalog.get("statuses") or {})[identifier]
            for entry in actor.get("dice_loadout") or []:
                identifier = str(entry.get("die_id", ""))
                if identifier in (catalog.get("dice") or {}):
                    linked[("die", identifier)] = (catalog.get("dice") or {})[identifier]
            for die in (actor.get("dice") or {}).get("dice") or []:
                identifier = str(die.get("die_id", ""))
                if identifier in (catalog.get("dice") or {}):
                    linked[("die", identifier)] = (catalog.get("dice") or {})[identifier]
        own = actors.get(viewer) or {}
        instances = own.get("card_instances") or {}
        for instance_id in own.get("hand") or []:
            identifier = str((instances.get(instance_id) or {}).get("definition_id", ""))
            if identifier in (catalog.get("cards") or {}):
                linked[("card", identifier)] = (catalog.get("cards") or {})[identifier]
        for zone in ("deck_composition", "discard_composition", "removed_composition"):
            for identifier in own.get(zone) or {}:
                if identifier in (catalog.get("cards") or {}):
                    linked[("card", str(identifier))] = (catalog.get("cards") or {})[identifier]
        for (kind, identifier), definition in sorted(linked.items()):
            self._flatten(rows, kind, -1, f"content.{kind}.{identifier}", definition, "")

        for model_index, authority_index in enumerate(authority_indices):
            self._flatten(rows, "candidate", model_index, "action", actions[authority_index], "own")

        capacity = self.manifest.layout.details.rows
        if len(rows) > capacity:
            counts: dict[str, int] = {}
            for scope, *_ in rows:
                counts[scope] = counts.get(scope, 0) + 1
            raise RuntimeError(f"v4 detail rows {len(rows)} exceed frozen capacity {capacity}: {counts}")
        for row_index, (scope, candidate, path, value, owner) in enumerate(rows):
            row = self._row(observation, row_index)
            row[0] = 1
            scope_names = ("context", "actor", "die", "ability", "status", "card", "candidate")
            if scope in scope_names:
                row[1 + scope_names.index(scope)] = 1
            row[9] = owner == "own"
            row[10] = owner == "opponent"
            row[11] = candidate >= 0
            self._write_hash(row, 12, "path", path)
            if isinstance(value, str):
                self._write_hash(row, 44, "value", value)
            elif isinstance(value, bool):
                row[80] = float(value)
            elif isinstance(value, (int, float)):
                numeric = float(value)
                row[76] = 1
                row[77] = math.tanh(numeric / 20.0)
                row[78] = numeric > 0
                row[79] = numeric < 0
            elif value is None:
                row[81] = 1
            row[82] = (
                (candidate + 1) / max(self.manifest.maximum_legal_candidates, 1) if candidate >= 0 else 0
            )
            row[83] = min(path.count(".") + path.count("[]"), 20) / 20.0
            lower = path.lower()
            for offset, keyword in enumerate(
                (
                    "actor",
                    "card",
                    "ability",
                    "status",
                    "die",
                    "resource",
                    "segment",
                    "stage",
                    "operation",
                    "requirement",
                    "target",
                    "source",
                ),
                start=84,
            ):
                row[offset] = keyword in lower

    def _flatten(
        self,
        output: list[tuple[str, int, str, Any, str]],
        scope: str,
        candidate: int,
        path: str,
        value: Any,
        owner: str,
    ) -> None:
        if isinstance(value, dict):
            output.append((scope, candidate, path, float(len(value)), owner))
            for key in sorted(value):
                # Instance/window/battle IDs are arbitrary labels. Their semantic
                # relations are represented by actor role and the surrounding fields.
                if key in {"battle_id", "instance_id", "pending_input_id", "request_id", "window_id"}:
                    continue
                self._flatten(output, scope, candidate, f"{path}.{key}", value[key], owner)
        elif isinstance(value, list):
            output.append((scope, candidate, path, float(len(value)), owner))
            for child in value:
                self._flatten(output, scope, candidate, f"{path}[]", child, owner)
        else:
            output.append((scope, candidate, path, value, owner))

    def _write_hash(self, row: np.ndarray, offset: int, namespace: str, value: str) -> None:
        digest = stable_hash(value)
        key = (namespace, digest)
        previous = self._runtime_hashes.get(key)
        if previous is not None and previous != value:
            raise RuntimeError(f"runtime v4 {namespace} hash collision: {previous!r} and {value!r}")
        self._runtime_hashes[key] = value
        self._write_bits(row, offset, int(digest, 16), 32)

    @staticmethod
    def _write_bits(row: np.ndarray, offset: int, value: int, bits: int) -> None:
        for bit in range(bits):
            row[offset + bit] = bool(value & (1 << bit))

    def _row(self, observation: np.ndarray, index: int) -> np.ndarray:
        entity = self.manifest.layout.details
        start = entity.offset + index * entity.features
        return observation[start : start + entity.features]


def parse_v4_manifest_payload(payload: object) -> dict[str, Any]:
    if isinstance(payload, dict):
        return payload
    if isinstance(payload, str) and payload:
        value = json.loads(payload)
        return value if isinstance(value, dict) else {}
    return {}
