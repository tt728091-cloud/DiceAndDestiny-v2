from __future__ import annotations

import base64
import json
from dataclasses import dataclass
from typing import Any

import numpy as np

from .manifest_v3 import OBSERVATION_SCHEMA_V3, ObservationManifestV3
from .schema_v2 import SchemaEncoderV2, parse_payload_v2

SEGMENTS_V3 = ("income", "offensive", "defensive", "ongoing_effects")


@dataclass(frozen=True)
class EncodedDecisionV3:
    observation: np.ndarray
    action_mask: np.ndarray


class SchemaEncoderV3:
    """Frozen-manifest, viewer-safe entity encoding for Observation V3."""

    def __init__(self, manifest: ObservationManifestV3) -> None:
        manifest.validate()
        self.manifest = manifest

    def encode(self, transition: dict[str, Any]) -> EncodedDecisionV3:
        transported = transition.get("encoded_decision")
        if transported and transported.get("observation_schema") == OBSERVATION_SCHEMA_V3:
            return self._decode_transport(transported)
        result = transition.get("result") or {}
        snapshot = result.get("snapshot") or {}
        catalog = snapshot.get("content_catalog") or {}
        if not catalog:
            raise RuntimeError("observation v3 requires the viewer-safe public content catalog")
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        if viewer not in ("seat-a", "seat-b"):
            raise ValueError(f"invalid viewer actor {viewer!r}")
        opponent = "seat-b" if viewer == "seat-a" else "seat-a"
        actors = snapshot.get("actors") or {}
        own = actors.get(viewer) or {}
        other = actors.get(opponent) or {}
        self._audit_catalog(catalog)
        observation = np.zeros(self.manifest.layout.observation_size, dtype=np.float32)
        self._encode_context(observation, transition, snapshot, result, viewer, opponent, own, other)
        self._encode_actor_row(self._row(observation, "actors", 0), own, own=True)
        self._encode_actor_row(self._row(observation, "actors", 1), other, own=False)
        self._encode_actor_dice(observation, actor=own, owner_slot=0, own=True)
        self._encode_actor_dice(observation, actor=other, owner_slot=1, own=False)
        self._encode_actor_abilities(observation, actor=own, owner_slot=0, own=True, catalog=catalog)
        self._encode_actor_abilities(observation, actor=other, owner_slot=1, own=False, catalog=catalog)
        self._encode_actor_statuses(
            observation,
            actor=own,
            owner_slot=0,
            own=True,
            catalog=catalog,
            snapshot=snapshot,
        )
        self._encode_actor_statuses(
            observation,
            actor=other,
            owner_slot=1,
            own=False,
            catalog=catalog,
            snapshot=snapshot,
        )
        self._encode_visible_cards(observation, own, catalog)
        actions = result.get("legal_actions") or []
        if len(actions) > self.manifest.maximum_legal_candidates:
            raise RuntimeError(
                f"authority produced {len(actions)} candidates; frozen v3 capacity is "
                f"{self.manifest.maximum_legal_candidates}"
            )
        mask = np.zeros(self.manifest.maximum_legal_candidates, dtype=bool)
        kept = {int(index) for index in (own.get("dice") or {}).get("kept_indices") or []}
        board = list(own.get("offensive_abilities") or []) + list(own.get("defensive_abilities") or [])
        for index, action in enumerate(actions):
            self._encode_candidate(
                self._row(observation, "candidates", index),
                action,
                viewer=viewer,
                opponent=opponent,
                actor=own,
                board=board,
                catalog=catalog,
            )
            mask[index] = SchemaEncoderV2._is_progressing_action(action, kept, own, catalog)
        if not mask.any() and not transition.get("terminal") and not transition.get("truncation_reason"):
            raise RuntimeError("nonterminal v3 decision has no progressing legal candidates")
        return EncodedDecisionV3(observation, mask)

    def assert_transport_parity(self, transition: dict[str, Any]) -> None:
        if not transition.get("encoded_decision") or not (transition.get("result") or {}).get("snapshot"):
            raise RuntimeError("v3 transport parity requires raw and Go-encoded decisions")
        transported = self.encode(transition)
        raw_transition = dict(transition)
        raw_transition.pop("encoded_decision", None)
        raw = self.encode(raw_transition)
        np.testing.assert_array_equal(transported.action_mask, raw.action_mask)
        np.testing.assert_allclose(
            transported.observation,
            raw.observation,
            rtol=0.0,
            atol=1e-7,
        )

    def _decode_transport(self, value: dict[str, Any]) -> EncodedDecisionV3:
        if value.get("manifest_sha256") != self.manifest.manifest_sha256:
            raise RuntimeError("encoded v3 decision uses a different frozen manifest")
        observation = np.frombuffer(base64.b64decode(value["observation_f32_le_base64"]), dtype="<f4").copy()
        if observation.shape != (self.manifest.layout.observation_size,):
            raise RuntimeError(f"encoded v3 observation has shape {observation.shape}")
        bits = np.frombuffer(base64.b64decode(value["action_mask_bits_base64"]), dtype=np.uint8)
        mask = np.unpackbits(bits, bitorder="little")[: self.manifest.maximum_legal_candidates]
        mask = mask.astype(bool, copy=False)
        candidate_count = int(value["candidate_count"])
        if candidate_count > self.manifest.maximum_legal_candidates:
            raise RuntimeError("encoded v3 candidate count exceeds manifest")
        if np.any(mask[candidate_count:]):
            raise RuntimeError("encoded v3 mask enables a padded candidate")
        return EncodedDecisionV3(observation, mask)

    def _encode_context(
        self,
        observation: np.ndarray,
        transition: dict[str, Any],
        snapshot: dict[str, Any],
        result: dict[str, Any],
        viewer: str,
        opponent: str,
        own: dict[str, Any],
        other: dict[str, Any],
    ) -> None:
        row = self._row(observation, "context", 0)
        row[0] = 1
        row[1] = viewer == "seat-a"
        row[2] = viewer == "seat-b"
        row[3] = self._scaled(snapshot.get("round", 0), "round")
        row[4] = self._scaled(snapshot.get("completed_rounds", 0), "round")
        self._one_hot(row, 5, SEGMENTS_V3, str(snapshot.get("segment", "")))
        priority = snapshot.get("priority_actor_id", "")
        row[9] = priority == viewer
        row[10] = priority == opponent
        row[11] = not priority
        row[12] = bool(transition.get("terminal"))
        row[13] = transition.get("winner") == viewer
        row[14] = transition.get("winner") == opponent
        row[15] = (snapshot.get("status") == "draw") or (
            transition.get("terminal") and not transition.get("winner")
        )
        row[16] = len(result.get("legal_actions") or []) / max(self.manifest.maximum_legal_candidates, 1)
        row[17] = self._vocabulary_index(self._form_id(own), self.manifest.form_vocabulary, allow_empty=True)
        row[18] = self._vocabulary_index(
            self._form_id(other), self.manifest.form_vocabulary, allow_empty=True
        )
        row[19] = bool(own.get("dice"))
        row[20] = bool(other.get("dice"))
        row[21] = str(snapshot.get("stage", "")) == "planning"
        row[22] = str(snapshot.get("segment", "")) == "defensive"
        row[23] = str(snapshot.get("segment", "")) == "ongoing_effects"
        flow = snapshot.get("flow") or {}
        row[24] = float(flow.get("iteration", 0)) / 20.0
        row[25] = bool((snapshot.get("resolution") or {}).get("active_window"))
        row[26] = bool(snapshot.get("damage"))
        row[27] = bool(snapshot.get("settled_damage"))

    def _encode_actor_row(self, row: np.ndarray, actor: dict[str, Any], *, own: bool) -> None:
        if not actor:
            return
        row[0] = 1
        row[1] = own
        row[2] = not own
        row[3] = self._vocabulary_index(
            str(actor.get("definition_id", "")), self.manifest.combatant_vocabulary
        )
        current_health = float(actor.get("current_health", 0))
        max_health = float(actor.get("max_health", 0))
        row[4] = self._scaled(current_health, "health")
        row[5] = self._scaled(max_health, "health")
        row[6] = current_health / max(max_health, 1.0)
        row[7] = self._scaled(actor.get("energy_points", 0), "energy")
        row[8] = self._scaled(actor.get("max_energy_points", 0), "energy")
        row[9] = self._scaled(actor.get("hand_count", 0), "hand")
        row[10] = self._scaled(actor.get("deck_count", 0), "deck")
        row[11] = self._scaled(actor.get("discard_count", 0), "deck")
        row[12] = self._scaled(actor.get("removed_count", 0), "deck")
        row[13] = self._scaled(actor.get("dice_count", 0), "dice")
        row[14] = self._scaled(actor.get("ability_count", 0), "abilities")
        row[15] = self._scaled(len(actor.get("statuses") or []), "statuses")
        row[16] = float(len(actor.get("tokens") or [])) / max(self.manifest.maximum_tokens_per_actor, 1)
        row[17] = actor.get("defeat_state") == "defeated"
        row[18] = self._scaled(actor.get("max_hand_size", 0), "hand")
        row[19] = own and bool(actor.get("hand"))
        row[20] = self._vocabulary_index(
            self._form_id(actor), self.manifest.form_vocabulary, allow_empty=True
        )
        row[21] = bool(self._form_id(actor))

    def _encode_actor_dice(
        self, observation: np.ndarray, *, actor: dict[str, Any], owner_slot: int, own: bool
    ) -> None:
        state = actor.get("dice") or {}
        dice = state.get("dice") or []
        if len(dice) > self.manifest.maximum_dice_per_actor:
            raise RuntimeError(
                f"actor has {len(dice)} visible dice; frozen v3 capacity is "
                f"{self.manifest.maximum_dice_per_actor}"
            )
        kept = {int(value) for value in state.get("kept_indices") or []}
        base = owner_slot * self.manifest.maximum_dice_per_actor
        for slot, die in enumerate(dice):
            row = self._row(observation, "dice", base + slot)
            row[0] = 1
            row[1] = own
            row[2] = not own
            row[3] = 1
            row[4] = self._vocabulary_index(str(die.get("die_id", "")), self.manifest.die_vocabulary)
            die_index = int(die.get("index", slot))
            row[5] = die_index / max(self.manifest.maximum_dice_per_actor, 1)
            row[6] = self._scaled(die.get("face", 0), "face")
            row[7] = self._scaled(die.get("value", 0), "face")
            row[8] = die_index in kept
            row[9] = die_index not in kept
            row[10] = bool(state.get("complete"))
            row[11] = self._scaled(state.get("rolls_remaining", 0), "rolls")
            for symbol in die.get("symbols") or []:
                if symbol not in self.manifest.symbol_vocabulary:
                    raise RuntimeError(f"visible die references unknown symbol {symbol!r}")
                feature = 12 + self.manifest.symbol_vocabulary.index(symbol)
                if feature >= len(row):
                    raise RuntimeError("symbol vocabulary exceeds die feature width")
                row[feature] += 1

    def _encode_actor_abilities(
        self,
        observation: np.ndarray,
        *,
        actor: dict[str, Any],
        owner_slot: int,
        own: bool,
        catalog: dict[str, Any],
    ) -> None:
        offensive = list(actor.get("offensive_abilities") or [])
        defensive = list(actor.get("defensive_abilities") or [])
        passive = list(actor.get("passive_abilities") or actor.get("passives") or [])
        board = [(value, "offensive") for value in offensive]
        board.extend((value, "defensive") for value in defensive)
        board.extend((value, "passive") for value in passive)
        if len(board) > self.manifest.maximum_active_abilities_per_actor:
            raise RuntimeError(
                f"active ability board has {len(board)} rows; frozen v3 capacity is "
                f"{self.manifest.maximum_active_abilities_per_actor}"
            )
        qualified = set(actor.get("qualified_abilities") or [])
        base = owner_slot * self.manifest.maximum_active_abilities_per_actor
        dice = (actor.get("dice") or {}).get("dice") or []
        for slot, (identifier, role) in enumerate(board):
            if identifier not in self.manifest.ability_vocabulary:
                raise RuntimeError(f"active board references unknown ability {identifier!r}")
            definition = SchemaEncoderV2.effective_ability(identifier, actor, catalog)
            if not definition:
                raise RuntimeError(f"catalog omits active ability {identifier!r}")
            row = self._row(observation, "abilities", base + slot)
            row[0] = 1
            row[1] = own
            row[2] = not own
            row[3] = self._vocabulary_index(identifier, self.manifest.ability_vocabulary)
            row[4] = role == "offensive"
            row[5] = role == "defensive"
            row[6] = role == "passive"
            row[7] = identifier in qualified
            row[8] = identifier == actor.get("selected_ability")
            row[9] = self._scaled((definition.get("cost") or {}).get("energy", 0), "energy")
            row[10] = float((definition.get("usage") or {}).get("maximum_per_segment", 0)) / 10.0
            targeting = definition.get("targeting") or {}
            row[11] = self._scaled(targeting.get("minimum", 0), "targets")
            row[12] = self._scaled(targeting.get("maximum", 0), "targets")
            selector = str(targeting.get("selector", ""))
            row[13] = selector == "self"
            row[14] = "enemy" in selector
            row[15] = "status" in selector
            row[16] = bool(self._form_id(actor))
            tiers = SchemaEncoderV2.ability_tiers(definition)
            if len(tiers) > self.manifest.maximum_tiers_per_ability:
                raise RuntimeError(f"ability {identifier!r} exceeds frozen tier capacity")
            row[17] = len(tiers) / max(self.manifest.maximum_tiers_per_ability, 1)
            progresses = [SchemaEncoderV2().tier_progress(tier, dice) for tier, _ in tiers]
            row[18] = max(progresses, default=0.0)
            row[19] = sum(
                SchemaEncoderV2().tier_met(tier, dice) and not conditional for tier, conditional in tiers
            ) / max(self.manifest.maximum_tiers_per_ability, 1)
            requirements = [
                requirement
                for tier, _ in tiers
                for requirement in (tier.get("requirements") or {}).get("all") or []
            ]
            row[20] = len(requirements) / max(
                self.manifest.maximum_tiers_per_ability * self.manifest.maximum_requirements_per_tier,
                1,
            )
            operations = [operation for tier, _ in tiers for operation in tier.get("operations") or []]
            row[24:31] = SchemaEncoderV2.operation_summary(operations)
            self._encode_operation_types(row, 32, operations)

    def _encode_actor_statuses(
        self,
        observation: np.ndarray,
        *,
        actor: dict[str, Any],
        owner_slot: int,
        own: bool,
        catalog: dict[str, Any],
        snapshot: dict[str, Any],
    ) -> None:
        statuses = list(actor.get("statuses") or [])
        if len(statuses) > self.manifest.maximum_statuses_per_actor:
            raise RuntimeError(
                f"actor has {len(statuses)} active statuses; frozen v3 capacity is "
                f"{self.manifest.maximum_statuses_per_actor}"
            )
        statuses.sort(
            key=lambda value: (
                str(value.get("definition_id", "")),
                str(value.get("instance_id", "")),
            )
        )
        base = owner_slot * self.manifest.maximum_statuses_per_actor
        for slot, status in enumerate(statuses):
            identifier = str(status.get("definition_id", ""))
            if identifier not in self.manifest.status_vocabulary:
                raise RuntimeError(f"active status references unknown definition {identifier!r}")
            definition = (catalog.get("statuses") or {}).get(identifier) or {}
            if not definition:
                raise RuntimeError(f"catalog omits active status {identifier!r}")
            row = self._row(observation, "statuses", base + slot)
            row[0] = 1
            row[1] = own
            row[2] = not own
            row[3] = self._vocabulary_index(identifier, self.manifest.status_vocabulary)
            row[4] = self._scaled(status.get("stacks", 0), "stacks")
            row[5] = self._scaled((definition.get("stacking") or {}).get("stack_limit", 1), "stacks")
            polarity = str(definition.get("polarity", ""))
            row[6] = polarity == "positive"
            row[7] = polarity == "negative"
            row[8] = polarity not in {"positive", "negative"}
            mode = str(definition.get("activation_mode", ""))
            row[9] = mode == "automatic"
            row[10] = mode == "optional"
            row[11] = bool(mode) and mode not in {"automatic", "optional"}
            lifecycle = definition.get("lifecycle") or {}
            for offset, key in enumerate(
                (
                    "persistent",
                    "consume_on_trigger_checkpoint",
                    "consume_on_play",
                    "remove_after_resolution",
                    "remove_on_duration_zero",
                ),
                start=12,
            ):
                row[offset] = bool(lifecycle.get(key))
            triggers = definition.get("triggers") or []
            operations = list(definition.get("operations") or [])
            operations.extend(
                operation for trigger in triggers for operation in trigger.get("operations") or []
            )
            row[17] = len(triggers) / 10.0
            row[18] = snapshot.get("segment") == "ongoing_effects"
            row[19] = snapshot.get("priority_actor_id") in {
                "seat-a" if own else "seat-b",
                "seat-b" if own else "seat-a",
            }
            row[20:27] = SchemaEncoderV2.operation_summary(operations)
            self._encode_operation_types(row, 28, operations)

    def _encode_visible_cards(
        self, observation: np.ndarray, actor: dict[str, Any], catalog: dict[str, Any]
    ) -> None:
        hand = list(actor.get("hand") or [])
        if len(hand) > self.manifest.maximum_visible_hand_cards:
            raise RuntimeError(
                f"viewer has {len(hand)} visible hand cards; frozen v3 capacity is "
                f"{self.manifest.maximum_visible_hand_cards}"
            )
        instances = actor.get("card_instances") or {}
        cards = catalog.get("cards") or {}
        for slot, instance_id in enumerate(hand):
            identifier = str((instances.get(instance_id) or {}).get("definition_id", ""))
            if identifier not in self.manifest.card_vocabulary:
                raise RuntimeError(f"visible card references unknown definition {identifier!r}")
            definition = cards.get(identifier) or {}
            if not definition:
                raise RuntimeError(f"catalog omits visible card {identifier!r}")
            row = self._row(observation, "cards", slot)
            row[0] = 1
            row[1] = 1
            row[2] = self._vocabulary_index(identifier, self.manifest.card_vocabulary)
            row[3] = self._scaled((definition.get("cost") or {}).get("energy", 0), "energy")
            kind = str(definition.get("type", ""))
            row[4] = kind == "reaction"
            row[5] = kind == "status_response"
            row[6] = bool(kind) and kind not in {"reaction", "status_response"}
            targeting = definition.get("targeting") or {}
            row[7] = self._scaled(targeting.get("minimum", 0), "targets")
            row[8] = self._scaled(targeting.get("maximum", 0), "targets")
            selector = str(targeting.get("selector", ""))
            row[9] = selector == "self"
            row[10] = "enemy" in selector
            row[11] = "status" in selector
            operations = definition.get("operations") or []
            row[12:19] = SchemaEncoderV2.operation_summary(operations)
            self._encode_operation_types(row, 20, operations)

    def _encode_candidate(
        self,
        row: np.ndarray,
        action: dict[str, Any],
        *,
        viewer: str,
        opponent: str,
        actor: dict[str, Any],
        board: list[str],
        catalog: dict[str, Any],
    ) -> None:
        kind = str(action.get("type", ""))
        if kind not in self.manifest.command_vocabulary:
            raise RuntimeError(f"authority emitted unsupported v3 command type {kind!r}")
        row[0] = 1
        self._one_hot(row, 1, self.manifest.command_vocabulary, kind)
        payload = parse_payload_v2(action.get("payload"))
        commitment = payload.get("commitment") or {}
        targets = payload.get("target_ids") or commitment.get("target_ids") or []
        indices = payload.get("reroll_indices")
        if indices is None:
            indices = payload.get("kept_indices")
        if indices is None:
            indices = commitment.get("die_indices") or []
        cards = payload.get("card_ids") or commitment.get("card_ids") or []
        choice_id = str(commitment.get("choice_id") or "")
        ability_id = str(payload.get("ability_id") or choice_id)
        if ability_id not in (catalog.get("abilities") or {}):
            ability_id = ""
        row[12] = kind in {"planning_pass", "pass"}
        row[13] = kind in {"planning_roll", "roll_dice"}
        row[14] = kind in {"planning_keep", "planning_reroll"}
        row[15] = viewer in targets
        row[16] = opponent in targets
        row[17] = self._scaled(len(targets), "targets")
        row[18] = self._scaled(len(indices), "dice")
        row[19] = self._scaled(len(cards), "hand")
        row[20] = bool(ability_id)
        row[21] = bool(cards)
        for die_index in indices:
            die_index = int(die_index)
            if not 0 <= die_index < self.manifest.maximum_dice_per_actor:
                raise RuntimeError(f"candidate die index {die_index} exceeds frozen v3 capacity")
            row[24 + die_index] = 1
        operations: list[dict[str, Any]] = []
        if ability_id:
            if ability_id not in board:
                raise RuntimeError(f"candidate references ability outside active board: {ability_id!r}")
            row[32] = self._vocabulary_index(ability_id, self.manifest.ability_vocabulary)
            ability = SchemaEncoderV2.effective_ability(ability_id, actor, catalog)
            row[33] = ability_id in set(actor.get("qualified_abilities") or [])
            row[34] = ability_id == actor.get("selected_ability")
            row[35] = self._scaled((ability.get("cost") or {}).get("energy", 0), "energy")
            operations.extend(
                operation
                for tier, _ in SchemaEncoderV2.ability_tiers(ability)
                for operation in tier.get("operations") or []
            )
        instances = actor.get("card_instances") or {}
        for instance_id in cards:
            definition_id = str((instances.get(instance_id) or {}).get("definition_id", ""))
            definition = (catalog.get("cards") or {}).get(definition_id)
            if not definition or definition_id not in self.manifest.card_vocabulary:
                raise RuntimeError(f"candidate references missing viewer card definition {definition_id!r}")
            operations.extend(definition.get("operations") or [])
        if cards:
            row[36] = 1
        status = (catalog.get("statuses") or {}).get(choice_id)
        if status:
            if choice_id not in self.manifest.status_vocabulary:
                raise RuntimeError(f"candidate references unknown status {choice_id!r}")
            row[37] = self._vocabulary_index(choice_id, self.manifest.status_vocabulary)
            operations.extend(status.get("operations") or [])
            operations.extend(
                operation
                for trigger in status.get("triggers") or []
                for operation in trigger.get("operations") or []
            )
        row[40:47] = SchemaEncoderV2.operation_summary(operations)
        self._encode_operation_types(row, 48, operations)

    def _audit_catalog(self, catalog: dict[str, Any]) -> None:
        groups = (
            ("symbols", self.manifest.symbol_vocabulary),
            ("dice", self.manifest.die_vocabulary),
            ("abilities", self.manifest.ability_vocabulary),
            ("statuses", self.manifest.status_vocabulary),
            ("cards", self.manifest.card_vocabulary),
        )
        for group, vocabulary in groups:
            unknown = set(catalog.get(group) or {}) - set(vocabulary)
            if unknown:
                raise RuntimeError(
                    f"runtime catalog has content outside frozen v3 {group} vocabulary: {sorted(unknown)}"
                )

    def _encode_operation_types(self, row: np.ndarray, offset: int, operations: list[dict[str, Any]]) -> None:
        def visit(values: list[dict[str, Any]]) -> None:
            for operation in values:
                kind = str(operation.get("type", ""))
                if kind:
                    if kind not in self.manifest.operation_vocabulary:
                        raise RuntimeError(f"unsupported operation in frozen v3 content: {kind!r}")
                    feature = offset + self.manifest.operation_vocabulary.index(kind)
                    if feature >= len(row):
                        raise RuntimeError("operation vocabulary exceeds entity feature width")
                    row[feature] += 1
                for outcome in operation.get("outcomes") or []:
                    visit(outcome.get("operations") or [])

        visit(operations)

    def _row(self, observation: np.ndarray, name: str, index: int) -> np.ndarray:
        entity_range = getattr(self.manifest.layout, name)
        if not 0 <= index < entity_range.rows:
            raise RuntimeError(f"v3 {name} row {index} exceeds frozen capacity {entity_range.rows}")
        start = entity_range.offset + index * entity_range.features
        return observation[start : start + entity_range.features]

    def _scaled(self, value: Any, name: str) -> float:
        return float(value or 0) / max(float(self.manifest.normalization[name]), 1.0)

    @staticmethod
    def _form_id(actor: dict[str, Any]) -> str:
        form = actor.get("current_form") or actor.get("form_id") or ""
        if isinstance(form, dict):
            return str(form.get("id", ""))
        return str(form)

    @staticmethod
    def _one_hot(output: np.ndarray, offset: int, vocabulary: tuple[str, ...], selected: str) -> None:
        if selected in vocabulary:
            output[offset + vocabulary.index(selected)] = 1

    @staticmethod
    def _vocabulary_index(
        identifier: str, vocabulary: tuple[str, ...], *, allow_empty: bool = False
    ) -> float:
        if not identifier and allow_empty:
            return 0.0
        if identifier not in vocabulary:
            raise RuntimeError(f"identifier {identifier!r} is outside frozen v3 vocabulary")
        return (vocabulary.index(identifier) + 1) / max(len(vocabulary), 1)


def parse_v3_manifest_payload(payload: object) -> dict[str, Any]:
    if isinstance(payload, dict):
        return payload
    if isinstance(payload, str) and payload:
        value = json.loads(payload)
        return value if isinstance(value, dict) else {}
    return {}
