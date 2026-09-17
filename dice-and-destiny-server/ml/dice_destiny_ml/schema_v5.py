from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any

import numpy as np

from .champions import canonical_hash
from .manifest_v5 import ObservationManifestV5
from .schema_v2 import parse_payload_v2
from .schema_v4 import EncodedDecisionV4, SchemaEncoderV4


@dataclass(frozen=True)
class EncodedDecisionV5(EncodedDecisionV4):
    pass


_TRANSIENT_KEYS = {
    "battle_id",
    "id",
    "instance_id",
    "pending_input_id",
    "request_id",
    "window_id",
}
_REFERENCE_KEYS = {
    "choice_id",
    "proposal_id",
    "source_id",
}
_UNORDERED_LIST_KEYS = {
    "card_ids",
    "kept_indices",
    "proposal_ids",
    "reroll_indices",
    "target_ids",
}


class SchemaEncoderV5(SchemaEncoderV4):
    """V4-complete state with stable, semantic candidate identity.

    The original authority index remains the submitted command.  Only the
    neural representation is canonicalized: runtime/window identifiers are
    removed, card instances resolve to definitions, seats resolve to roles,
    and live proposal/choice references resolve to their surrounding semantic
    objects when the viewer-safe snapshot contains them.
    """

    def __init__(self, manifest: ObservationManifestV5) -> None:
        manifest.validate()
        self.manifest = manifest
        # Avoid V4's concrete manifest validator while retaining its proven V3
        # base encoder, compaction, detail layout, and collision audit.
        from .schema_v3 import SchemaEncoderV3

        self._base = SchemaEncoderV3(manifest.base)
        self._runtime_hashes: dict[tuple[str, str], str] = {}

    def encode(self, transition: dict[str, Any]) -> EncodedDecisionV5:
        raw_transition = dict(transition)
        raw_transition.pop("encoded_decision", None)
        base = self._base.encode(raw_transition)
        result = transition.get("result") or {}
        snapshot = result.get("snapshot") or {}
        actions = list(result.get("legal_actions") or [])
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        actors = snapshot.get("actors") or {}
        own = actors.get(viewer) or {}
        catalog = snapshot.get("content_catalog") or {}
        authority_indices = self._compact_authority_indices(actions, own, catalog)
        if len(authority_indices) > self.manifest.maximum_legal_candidates:
            raise RuntimeError("compacted v5 actions exceed frozen candidate capacity")

        references = self._reference_objects(snapshot)
        semantic_actions = [
            self.semantic_action(actions[index], snapshot=snapshot, viewer=str(viewer), references=references)
            for index in authority_indices
        ]
        observation = np.zeros(self.manifest.layout.observation_size, dtype=np.float32)
        candidate_offset = self.manifest.layout.candidates.offset
        observation[:candidate_offset] = base.observation[:candidate_offset]
        candidate_width = self.manifest.layout.candidates.features
        for model_index, (authority_index, semantic) in enumerate(
            zip(authority_indices, semantic_actions, strict=True)
        ):
            source = candidate_offset + authority_index * candidate_width
            target = candidate_offset + model_index * candidate_width
            observation[target : target + candidate_width] = base.observation[
                source : source + candidate_width
            ]
            fingerprint = canonical_hash(semantic)
            self._write_bits(
                observation[target : target + candidate_width],
                64,
                int(fingerprint[:8], 16),
                32,
            )

        mask = np.zeros(self.manifest.maximum_legal_candidates, dtype=bool)
        mask[: len(authority_indices)] = True
        # Candidate detail rows use the same canonical objects as the
        # fingerprint.  Global state/content rows remain viewer-safe V4 data.
        self._encode_details(
            observation,
            transition,
            snapshot,
            semantic_actions,
            list(range(len(semantic_actions))),
            viewer,
        )
        if not mask.any() and not transition.get("terminal") and not transition.get("truncation_reason"):
            raise RuntimeError("nonterminal v5 decision has no progressing model actions")
        return EncodedDecisionV5(
            observation,
            mask,
            np.asarray(authority_indices, dtype=np.int32),
        )

    def semantic_action(
        self,
        action: dict[str, Any],
        *,
        snapshot: dict[str, Any],
        viewer: str,
        references: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        catalog = snapshot.get("content_catalog") or {}
        actors = snapshot.get("actors") or {}
        actor = actors.get(viewer) or {}
        opponent = "seat-b" if viewer == "seat-a" else "seat-a"
        instances = actor.get("card_instances") or {}
        card_definitions = {
            str(instance_id): str((value or {}).get("definition_id", ""))
            for instance_id, value in instances.items()
        }
        stable_definitions = {
            str(identifier)
            for group in ("abilities", "cards", "dice", "statuses")
            for identifier in (catalog.get(group) or {})
        }
        refs = references if references is not None else self._reference_objects(snapshot)
        payload = parse_payload_v2(action.get("payload"))
        return {
            "type": str(action.get("type", "")),
            "payload": self._canonicalize(
                payload,
                key="payload",
                viewer=viewer,
                opponent=opponent,
                card_definitions=card_definitions,
                stable_definitions=stable_definitions,
                references=refs,
                resolving=frozenset(),
            ),
        }

    def _canonicalize(
        self,
        value: Any,
        *,
        key: str,
        viewer: str,
        opponent: str,
        card_definitions: dict[str, str],
        stable_definitions: set[str],
        references: dict[str, Any],
        resolving: frozenset[str],
    ) -> Any:
        if isinstance(value, dict):
            return {
                child_key: self._canonicalize(
                    child,
                    key=child_key,
                    viewer=viewer,
                    opponent=opponent,
                    card_definitions=card_definitions,
                    stable_definitions=stable_definitions,
                    references=references,
                    resolving=resolving,
                )
                for child_key, child in sorted(value.items())
                if child_key not in _TRANSIENT_KEYS
            }
        if isinstance(value, list):
            children = [
                self._canonicalize(
                    child,
                    key=key,
                    viewer=viewer,
                    opponent=opponent,
                    card_definitions=card_definitions,
                    stable_definitions=stable_definitions,
                    references=references,
                    resolving=resolving,
                )
                for child in value
            ]
            if key in _UNORDERED_LIST_KEYS:
                children.sort(key=lambda child: json.dumps(child, sort_keys=True, separators=(",", ":")))
            return children
        if not isinstance(value, str):
            return value
        if value == viewer:
            return "self"
        if value == opponent:
            return "opponent"
        if value in card_definitions and card_definitions[value]:
            return card_definitions[value]
        if value in stable_definitions:
            return value
        if value in references and value not in resolving:
            return {
                "reference": self._canonicalize(
                    references[value],
                    key="reference",
                    viewer=viewer,
                    opponent=opponent,
                    card_definitions=card_definitions,
                    stable_definitions=stable_definitions,
                    references=references,
                    resolving=resolving | {value},
                )
            }
        if key in _REFERENCE_KEYS or key.rstrip("s") in _REFERENCE_KEYS:
            # An unresolved authority identity must not become a random model
            # feature.  Its role is preserved so unsupported reference shapes
            # remain visible and testable without encoding the runtime label.
            return {"unresolved_reference_role": key}
        return value

    @staticmethod
    def _reference_objects(snapshot: dict[str, Any]) -> dict[str, Any]:
        result: dict[str, Any] = {}

        def visit(value: Any) -> None:
            if isinstance(value, dict):
                semantic = {
                    key: child
                    for key, child in value.items()
                    if key not in _TRANSIENT_KEYS and key not in _REFERENCE_KEYS
                }
                for key in (*_REFERENCE_KEYS, *_TRANSIENT_KEYS):
                    identifier = value.get(key)
                    if isinstance(identifier, str) and identifier:
                        result.setdefault(identifier, semantic)
                for child in value.values():
                    visit(child)
            elif isinstance(value, list):
                for child in value:
                    visit(child)

        visit(snapshot)
        return result

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
            if path.rsplit(".", 1)[-1] in {"card_instances", "choices", "proposals"}:
                children = sorted(
                    value.values(),
                    key=lambda child: json.dumps(child, sort_keys=True, separators=(",", ":")),
                )
                for child in children:
                    self._flatten(output, scope, candidate, f"{path}[]", child, owner)
                return
            for key in sorted(value):
                if key in _TRANSIENT_KEYS or key in _REFERENCE_KEYS:
                    continue
                self._flatten(output, scope, candidate, f"{path}.{key}", value[key], owner)
        elif isinstance(value, list):
            output.append((scope, candidate, path, float(len(value)), owner))
            if path.rsplit(".", 1)[-1] == "hand":
                # Exact visible card definitions are represented by the V3 card
                # rows and card_instances definitions; hand instance labels are
                # deliberately not model features.
                return
            for child in value:
                self._flatten(output, scope, candidate, f"{path}[]", child, owner)
        else:
            output.append((scope, candidate, path, value, owner))
