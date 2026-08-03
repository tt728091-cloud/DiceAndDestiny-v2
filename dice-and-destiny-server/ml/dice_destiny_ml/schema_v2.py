from __future__ import annotations

import base64
import copy
import itertools
import json
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Any

import numpy as np

MAX_ACTIONS_V2 = 128
MAX_DICE_V2 = 10
MAX_SYMBOLS_V2 = 8
MAX_ABILITIES_V2 = 12
MAX_TIERS_V2 = 6
MAX_REQUIREMENTS_V2 = 2
BASE_FEATURES_V2 = 2560
ACTION_FEATURES_V2 = 128
OBSERVATION_SIZE_V2 = BASE_FEATURES_V2 + MAX_ACTIONS_V2 * ACTION_FEATURES_V2

OBSERVATION_SCHEMA_V2 = "dice-and-destiny-observation-v2"
ACTION_SCHEMA_V2 = "dice-and-destiny-action-candidates-v2"
ENVIRONMENT_SCHEMA_V2 = "dice-and-destiny-ml-env-v2"

COMMAND_TYPES = (
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
SEGMENTS = ("income", "offensive", "defensive", "status")


@dataclass(frozen=True)
class EncodedDecisionV2:
    observation: np.ndarray
    action_mask: np.ndarray


class SchemaEncoderV2:
    """Mechanics-first, viewer-safe candidate encoding.

    IDs determine only stable slot ordering and candidate-to-board references.
    The learned meaning comes from dice faces/symbols, authored requirements,
    tier progress and qualification, costs, targets, and generic operations.
    Capacity overflow is rejected instead of silently truncated.
    """

    def encode(self, transition: dict[str, Any]) -> EncodedDecisionV2:
        transported = transition.get("encoded_decision")
        if transported and transported.get("observation_schema") == OBSERVATION_SCHEMA_V2:
            return self._decode_transport(transported)
        result = transition.get("result") or {}
        snapshot = result.get("snapshot") or {}
        catalog = snapshot.get("content_catalog") or {}
        if not catalog:
            raise RuntimeError("observation v2 requires the viewer-safe public content catalog")
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        if viewer not in ("seat-a", "seat-b"):
            raise ValueError(f"invalid viewer actor {viewer!r}")
        opponent = "seat-b" if viewer == "seat-a" else "seat-a"
        actors = snapshot.get("actors") or {}
        own = actors.get(viewer) or {}
        other = actors.get(opponent) or {}
        symbols = sorted((catalog.get("symbols") or {}).keys())
        board = list(own.get("offensive_abilities") or []) + list(own.get("defensive_abilities") or [])
        self._audit_capacity(symbols, board, own)

        base = np.zeros(BASE_FEATURES_V2, dtype=np.float32)
        base[0] = viewer == "seat-a"
        base[1] = viewer == "seat-b"
        base[2] = min(float(snapshot.get("round", 0)) / 50.0, 1.0)
        base[3] = min(float(snapshot.get("completed_rounds", 0)) / 50.0, 1.0)
        self._one_hot(base, 4, SEGMENTS, snapshot.get("segment", ""))
        priority = snapshot.get("priority_actor_id", "")
        base[8] = priority == viewer
        base[9] = priority == opponent
        base[10] = not priority
        self._actor_scalars(base, 16, own)
        self._actor_scalars(base, 32, other)
        dice_state = own.get("dice") or {}
        base[48] = float(dice_state.get("rolls_used", 0)) / 3.0
        base[49] = float(dice_state.get("max_rolls", 0)) / 3.0
        base[50] = float(dice_state.get("rolls_remaining", 0)) / 3.0
        base[51] = bool(dice_state.get("complete"))
        dice = dice_state.get("dice") or []
        kept = {int(index) for index in dice_state.get("kept_indices") or []}
        base[52] = len(dice) / MAX_DICE_V2
        base[53] = len(kept) / MAX_DICE_V2
        base[54] = len(own.get("qualified_abilities") or []) / MAX_ABILITIES_V2
        base[55] = len(board) / MAX_ABILITIES_V2
        base[56] = len(result.get("legal_actions") or []) / MAX_ACTIONS_V2
        symbol_counts = dice_state.get("symbol_counts") or self._symbol_counts(dice)
        for index, symbol in enumerate(symbols):
            base[64 + index] = float(symbol_counts.get(symbol, 0)) / MAX_DICE_V2
        self._encode_dice(base, 128, dice, kept, symbols)
        for index, ability_id in enumerate(board):
            definition = self.effective_ability(ability_id, own, catalog)
            if not definition:
                raise RuntimeError(f"ability board references missing definition {ability_id!r}")
            self._encode_ability(
                base[256 + index * 184 : 256 + (index + 1) * 184],
                definition,
                ability_id in set(own.get("qualified_abilities") or []),
                ability_id == own.get("selected_ability"),
                dice,
                symbols,
            )

        actions = result.get("legal_actions") or []
        if len(actions) > MAX_ACTIONS_V2:
            raise RuntimeError(
                f"authority produced {len(actions)} candidates; v2 capacity is {MAX_ACTIONS_V2}"
            )
        candidates = np.zeros((MAX_ACTIONS_V2, ACTION_FEATURES_V2), dtype=np.float32)
        mask = np.zeros(MAX_ACTIONS_V2, dtype=bool)
        for index, action in enumerate(actions):
            self._encode_action(
                candidates[index], action, viewer, opponent, own, catalog, board, dice, symbols
            )
            mask[index] = self._is_progressing_action(action, kept, own, catalog)
        if not mask.any() and not transition.get("terminal") and not transition.get("truncation_reason"):
            raise RuntimeError("nonterminal v2 decision has no legal candidates")
        observation = np.concatenate((base, candidates.reshape(-1))).astype(np.float32, copy=False)
        return EncodedDecisionV2(observation, mask)

    def assert_transport_parity(self, transition: dict[str, Any]) -> None:
        if not transition.get("encoded_decision") or not (transition.get("result") or {}).get("snapshot"):
            raise RuntimeError("v2 transport parity requires raw and Go-encoded decisions")
        encoded = self.encode(transition)
        raw_transition = dict(transition)
        raw_transition.pop("encoded_decision", None)
        raw = self.encode(raw_transition)
        np.testing.assert_array_equal(encoded.action_mask, raw.action_mask)
        if not np.array_equal(encoded.observation, raw.observation):
            indices = np.flatnonzero(encoded.observation != raw.observation)
            first = int(indices[0]) if len(indices) else -1
            raise RuntimeError(
                f"Go/Python v2 observation differs at {first}: "
                f"{encoded.observation[first]} != {raw.observation[first]}"
            )

    @staticmethod
    def _decode_transport(value: dict[str, Any]) -> EncodedDecisionV2:
        observation = np.frombuffer(base64.b64decode(value["observation_f32_le_base64"]), dtype="<f4").copy()
        if observation.shape != (OBSERVATION_SIZE_V2,):
            raise RuntimeError(f"encoded v2 observation has shape {observation.shape}")
        bits = np.frombuffer(base64.b64decode(value["action_mask_bits_base64"]), dtype=np.uint8)
        mask = np.unpackbits(bits, bitorder="little")[:MAX_ACTIONS_V2].astype(bool, copy=False)
        candidate_count = int(value["candidate_count"])
        if candidate_count > MAX_ACTIONS_V2 or int(mask.sum()) > candidate_count:
            raise RuntimeError("encoded v2 progress mask is incompatible with candidate count")
        return EncodedDecisionV2(observation, mask)

    @staticmethod
    def _is_progressing_action(
        action: dict[str, Any], kept: set[int], actor: dict[str, Any], catalog: dict[str, Any]
    ) -> bool:
        kind = action.get("type")
        payload = action.get("payload") or {}
        if isinstance(payload, str):
            payload = json.loads(payload)
        if kind == "planning_keep":
            return False
        if kind == "planning_select_ability":
            return not (
                payload.get("ability_id") == actor.get("selected_ability")
                and set(payload.get("target_ids") or []) == set(actor.get("selected_targets") or [])
            )
        if kind == "planning_select_targets":
            return set(payload.get("target_ids") or []) != set(actor.get("selected_targets") or [])
        if kind == "planning_commit_cards" and int(actor.get("deck_count", 0)) == 0:
            instances = actor.get("card_instances") or {}
            cards = catalog.get("cards") or {}
            for instance_id in payload.get("card_ids") or []:
                definition_id = (instances.get(instance_id) or {}).get("definition_id")
                operations = (cards.get(definition_id) or {}).get("operations") or []
                if any(operation.get("type") == "draw_cards" for operation in operations):
                    return False
        return True

    @staticmethod
    def _audit_capacity(symbols: list[str], board: list[str], actor: dict[str, Any]) -> None:
        if len(symbols) > MAX_SYMBOLS_V2:
            raise RuntimeError(f"content has {len(symbols)} symbols; v2 capacity is {MAX_SYMBOLS_V2}")
        if len(board) > MAX_ABILITIES_V2:
            raise RuntimeError(
                f"actor has {len(board)} authored abilities; v2 capacity is {MAX_ABILITIES_V2}"
            )
        dice = (actor.get("dice") or {}).get("dice") or []
        if len(dice) > MAX_DICE_V2:
            raise RuntimeError(f"actor has {len(dice)} current dice; v2 capacity is {MAX_DICE_V2}")

    @staticmethod
    def _actor_scalars(output: np.ndarray, offset: int, actor: dict[str, Any]) -> None:
        max_health = max(float(actor.get("max_health", 20)), 1.0)
        max_energy = max(float(actor.get("max_energy_points", 10)), 1.0)
        values = (
            float(actor.get("current_health", 0)) / max_health,
            float(actor.get("max_health", 0)) / 30.0,
            float(actor.get("energy_points", 0)) / max_energy,
            float(actor.get("hand_count", 0)) / 20.0,
            float(actor.get("deck_count", 0)) / 30.0,
            float(actor.get("discard_count", 0)) / 30.0,
            float(actor.get("removed_count", 0)) / 30.0,
            float(actor.get("dice_count", 0)) / MAX_DICE_V2,
            float(actor.get("ability_count", 0)) / MAX_ABILITIES_V2,
            float(len(actor.get("statuses") or [])) / 10.0,
            float(len(actor.get("tokens") or [])) / 10.0,
            float(actor.get("defeat_state") == "defeated"),
        )
        output[offset : offset + len(values)] = values

    @staticmethod
    def _encode_dice(
        output: np.ndarray, offset: int, dice: list[dict[str, Any]], kept: set[int], symbols: list[str]
    ) -> None:
        for slot, die in enumerate(dice):
            start = offset + slot * 14
            die_index = int(die.get("index", slot))
            output[start] = 1
            output[start + 1] = die_index / MAX_DICE_V2
            output[start + 2] = float(die.get("face", 0)) / 20.0
            output[start + 3] = float(die.get("value", 0)) / 20.0
            output[start + 4] = die_index in kept
            output[start + 5] = die_index not in kept
            for symbol in die.get("symbols") or []:
                if symbol not in symbols:
                    raise RuntimeError(f"rolled die references unknown symbol {symbol!r}")
                output[start + 6 + symbols.index(symbol)] = 1

    def _encode_ability(
        self,
        output: np.ndarray,
        ability: dict[str, Any],
        qualified: bool,
        selected: bool,
        dice: list[dict[str, Any]],
        symbols: list[str],
    ) -> None:
        qualification = ability.get("qualification") or {}
        activation = qualification.get("activation_tiers") or []
        bonuses = qualification.get("conditional_bonuses") or []
        tiers = [(tier, False) for tier in activation] + [(tier, True) for tier in bonuses]
        if len(tiers) > MAX_TIERS_V2:
            raise RuntimeError(
                f"ability {ability.get('id')!r} has {len(tiers)} tiers; "
                f"v2 capacity is {MAX_TIERS_V2}"
            )
        output[0] = 1
        output[1] = ability.get("type") == "offensive"
        output[2] = ability.get("type") == "defensive"
        output[3] = qualified
        output[4] = selected
        output[5] = float((ability.get("cost") or {}).get("energy", 0)) / 10.0
        output[6] = float((ability.get("usage") or {}).get("maximum_per_segment", 0)) / 10.0
        targeting = ability.get("targeting") or {}
        output[7] = float(targeting.get("minimum", 0)) / 5.0
        output[8] = float(targeting.get("maximum", 0)) / 5.0
        selector = targeting.get("selector", "")
        output[9] = selector == "self"
        output[10] = "enemy" in selector
        output[11] = bool(selector) and not output[9] and not output[10]
        output[12] = len(activation) / MAX_TIERS_V2
        output[13] = len(bonuses) / MAX_TIERS_V2
        selection = ability.get("selection") or {}
        output[14] = bool(selection.get("requires_incoming_proposal"))
        output[15] = float(selection.get("target_count", 0)) / 5.0
        for index, (tier, conditional) in enumerate(tiers):
            self._encode_tier(
                output[16 + index * 28 : 16 + (index + 1) * 28],
                tier,
                conditional,
                dice,
                symbols,
            )

    def _encode_tier(
        self,
        output: np.ndarray,
        tier: dict[str, Any],
        conditional: bool,
        dice: list[dict[str, Any]],
        symbols: list[str],
    ) -> None:
        requirements = ((tier.get("requirements") or {}).get("all") or [])
        if len(requirements) > MAX_REQUIREMENTS_V2:
            raise RuntimeError(
                f"tier {tier.get('id')!r} has {len(requirements)} requirements; "
                f"v2 capacity is {MAX_REQUIREMENTS_V2}"
            )
        progress = [self.requirement_progress(requirement, dice) for requirement in requirements]
        output[0] = 1
        output[1] = conditional
        output[2] = bool(requirements) and all(
            self.requirement_met(requirement, dice) for requirement in requirements
        )
        output[3] = min(progress, default=0.0)
        output[4] = len(requirements) / MAX_REQUIREMENTS_V2
        for index, requirement in enumerate(requirements):
            start = 5 + index * 8
            kind = requirement.get("type", "")
            output[start] = 1
            output[start + 1] = kind == "symbol_count"
            output[start + 2] = kind == "exact_faces"
            output[start + 3] = kind == "number_pattern"
            symbol = requirement.get("symbol_id", "")
            output[start + 4] = (symbols.index(symbol) + 1) / MAX_SYMBOLS_V2 if symbol in symbols else 0
            current, target = self.requirement_current_target(requirement, dice)
            output[start + 5] = current / MAX_DICE_V2
            output[start + 6] = target / MAX_DICE_V2
            output[start + 7] = self.requirement_progress(requirement, dice)
        effect = self.operation_summary(tier.get("operations") or [])
        output[21:28] = effect

    def _encode_action(
        self,
        output: np.ndarray,
        action: dict[str, Any],
        viewer: str,
        opponent: str,
        own: dict[str, Any],
        catalog: dict[str, Any],
        board: list[str],
        dice: list[dict[str, Any]],
        symbols: list[str],
    ) -> None:
        kind = action.get("type", "")
        self._one_hot(output, 0, COMMAND_TYPES, kind)
        payload = parse_payload_v2(action.get("payload"))
        commitment = payload.get("commitment") or {}
        targets = payload.get("target_ids") or commitment.get("target_ids") or []
        indices = payload.get("reroll_indices")
        if indices is None:
            indices = payload.get("kept_indices")
        if indices is None:
            indices = commitment.get("die_indices") or []
        cards = payload.get("card_ids") or commitment.get("card_ids") or []
        choice_id = commitment.get("choice_id") or ""
        ability_id = payload.get("ability_id") or choice_id
        if ability_id not in (catalog.get("abilities") or {}):
            ability_id = ""
        output[10] = kind in {"planning_pass", "pass"}
        output[11] = kind in {"planning_roll", "roll_dice"}
        output[12] = kind in {"planning_keep", "planning_reroll"}
        output[13] = viewer in targets
        output[14] = opponent in targets
        output[15] = min(len(targets) / 5.0, 1.0)
        output[16] = min(len(indices) / MAX_DICE_V2, 1.0)
        output[17] = min(len(cards) / 10.0, 1.0)
        output[18] = bool(ability_id)
        output[19] = bool(cards)
        dice_state = own.get("dice") or {}
        output[21] = float(dice_state.get("rolls_remaining", 0)) / 3.0
        for die_index in indices:
            die_index = int(die_index)
            if not 0 <= die_index < MAX_DICE_V2:
                raise RuntimeError(f"candidate die index {die_index} exceeds v2 capacity")
            output[22 + die_index] = 1

        linked = output[32:]
        if ability_id:
            if ability_id not in board:
                raise RuntimeError(f"candidate references ability outside viewer board: {ability_id!r}")
            linked[board.index(ability_id)] = 1
            ability = self.effective_ability(ability_id, own, catalog)
            linked[12] = 1
            linked[13] = ability_id in set(own.get("qualified_abilities") or [])
            linked[14] = ability_id == own.get("selected_ability")
            linked[15] = float((ability.get("cost") or {}).get("energy", 0)) / 10.0
            linked[16] = ability.get("type") == "offensive"
            linked[17] = ability.get("type") == "defensive"
            tiers = self.ability_tiers(ability)
            linked[18] = len(tiers) / MAX_TIERS_V2
            tier_progress = [self.tier_progress(tier, dice) for tier, _ in tiers]
            linked[19] = max(tier_progress, default=0.0)
            effects = [self.operation_summary(tier.get("operations") or []) for tier, _ in tiers]
            if effects:
                linked[20:27] = np.max(np.stack(effects), axis=0)
            targeting = ability.get("targeting") or {}
            linked[27] = float(targeting.get("minimum", 0)) / 5.0
            linked[28] = float(targeting.get("maximum", 0)) / 5.0
            for tier_index, (tier, conditional) in enumerate(tiers):
                if tier_index >= 4:
                    continue
                compact = linked[32 + tier_index * 16 : 32 + (tier_index + 1) * 16]
                reqs = ((tier.get("requirements") or {}).get("all") or [])
                compact[0] = 1
                compact[1] = conditional
                compact[2] = self.tier_progress(tier, dice)
                compact[3] = all(self.requirement_met(req, dice) for req in reqs)
                compact[4] = len(reqs) / MAX_REQUIREMENTS_V2
                compact[5:12] = self.operation_summary(tier.get("operations") or [])
        if cards:
            linked[29] = 1
            summaries = []
            energy = 0.0
            instances = own.get("card_instances") or {}
            for instance_id in cards:
                definition_id = (instances.get(instance_id) or {}).get("definition_id", "")
                definition = (catalog.get("cards") or {}).get(definition_id)
                if not definition:
                    raise RuntimeError(
                        f"candidate references missing viewer card definition {definition_id!r}"
                    )
                energy += float((definition.get("cost") or {}).get("energy", 0))
                summaries.append(self.operation_summary(definition.get("operations") or []))
            output[20] = energy / 10.0
            if summaries:
                linked[20:27] += np.sum(np.stack(summaries), axis=0)
        status = (catalog.get("statuses") or {}).get(choice_id)
        if status:
            linked[30] = 1
            operations = list(status.get("operations") or [])
            for trigger in status.get("triggers") or []:
                operations.extend(trigger.get("operations") or [])
            linked[20:27] += self.operation_summary(operations)

    @staticmethod
    def ability_tiers(ability: dict[str, Any]) -> list[tuple[dict[str, Any], bool]]:
        qualification = ability.get("qualification") or {}
        return [(tier, False) for tier in qualification.get("activation_tiers") or []] + [
            (tier, True) for tier in qualification.get("conditional_bonuses") or []
        ]

    @staticmethod
    def effective_ability(
        ability_id: str, actor: dict[str, Any], catalog: dict[str, Any]
    ) -> dict[str, Any]:
        base = (catalog.get("abilities") or {}).get(ability_id)
        if not base:
            return {}
        modifiers = [
            modifier
            for modifier in actor.get("ability_modifiers") or []
            if modifier.get("ability_id") == ability_id
        ]
        if not modifiers:
            return base
        result = copy.deepcopy(base)
        qualification = result.setdefault("qualification", {})
        bonuses = qualification.get("conditional_bonuses") or []
        qualification["conditional_bonuses"] = bonuses
        instances = actor.get("card_instances") or {}
        cards = catalog.get("cards") or {}
        resolved: dict[str, dict[str, Any]] = {}
        for modifier in modifiers:
            source = instances.get(modifier.get("source_card_instance_id")) or {}
            card = cards.get(source.get("definition_id")) or {}
            for operation in card.get("operations") or []:
                tier = ((operation.get("modifier") or {}).get("add_conditional_bonus"))
                if tier and tier.get("id") == modifier.get("bonus_id"):
                    identifier = tier.get("id", "")
                    if identifier not in resolved:
                        resolved[identifier] = copy.deepcopy(tier)
                    else:
                        resolved[identifier].setdefault("operations", []).extend(
                            copy.deepcopy(tier.get("operations") or [])
                        )
        bonuses.extend(resolved.values())
        return result

    def tier_progress(self, tier: dict[str, Any], dice: list[dict[str, Any]]) -> float:
        requirements = ((tier.get("requirements") or {}).get("all") or [])
        return min((self.requirement_progress(req, dice) for req in requirements), default=0.0)

    def tier_met(self, tier: dict[str, Any], dice: list[dict[str, Any]]) -> bool:
        requirements = ((tier.get("requirements") or {}).get("all") or [])
        return bool(requirements) and all(self.requirement_met(req, dice) for req in requirements)

    @staticmethod
    def requirement_met(requirement: dict[str, Any], dice: list[dict[str, Any]]) -> bool:
        kind = requirement.get("type")
        if kind == "symbol_count":
            count = sum(requirement.get("symbol_id") in (die.get("symbols") or []) for die in dice)
            exact = requirement.get("exact")
            if exact is not None and count != int(exact):
                return False
            if int(requirement.get("minimum", 0)) > 0 and count < int(requirement["minimum"]):
                return False
            if int(requirement.get("maximum", 0)) > 0 and count > int(requirement["maximum"]):
                return False
            return True
        if kind == "exact_faces":
            return sorted(int(die.get("face", 0)) for die in dice) == sorted(
                int(face) for face in requirement.get("faces") or []
            )
        if kind == "number_pattern":
            counts: dict[int, int] = {}
            for die in dice:
                face = int(die.get("face", 0))
                counts[face] = counts.get(face, 0) + 1
            pattern = requirement.get("pattern")
            if pattern == "three_of_a_kind":
                return any(count >= 3 for count in counts.values())
            if pattern == "exact_pair":
                return any(count == 2 for count in counts.values())
            if pattern == "pair_or_better":
                return any(count >= 2 for count in counts.values())
        return False

    @staticmethod
    def requirement_current_target(
        requirement: dict[str, Any], dice: list[dict[str, Any]]
    ) -> tuple[float, float]:
        kind = requirement.get("type")
        if kind == "symbol_count":
            current = sum(requirement.get("symbol_id") in (die.get("symbols") or []) for die in dice)
            target = requirement.get("exact")
            if target is None:
                target = requirement.get("minimum") or requirement.get("maximum") or 0
            return float(current), float(target)
        if kind == "exact_faces":
            wanted = list(requirement.get("faces") or [])
            remaining = list(wanted)
            matched = 0
            for die in dice:
                face = int(die.get("face", 0))
                if face in remaining:
                    remaining.remove(face)
                    matched += 1
            return float(matched), float(len(wanted))
        if kind == "number_pattern":
            counts: dict[int, int] = {}
            for die in dice:
                face = int(die.get("face", 0))
                counts[face] = counts.get(face, 0) + 1
            current = max(counts.values(), default=0)
            target = 3 if requirement.get("pattern") == "three_of_a_kind" else 2
            return float(current), float(target)
        return 0.0, 1.0

    def requirement_progress(self, requirement: dict[str, Any], dice: list[dict[str, Any]]) -> float:
        current, target = self.requirement_current_target(requirement, dice)
        if target <= 0:
            return 0.0
        kind = requirement.get("type")
        if kind == "symbol_count":
            exact = requirement.get("exact")
            maximum = requirement.get("maximum")
            if exact is not None and current > float(exact):
                return max(0.0, 1.0 - (current - float(exact)) / MAX_DICE_V2)
            if maximum and current > float(maximum):
                return max(0.0, 1.0 - (current - float(maximum)) / MAX_DICE_V2)
        return min(current / target, 1.0)

    @classmethod
    def operation_summary(cls, operations: Iterable[dict[str, Any]]) -> np.ndarray:
        """[damage, prevention, status, resource, roll, modifier, magnitude]."""
        result = np.zeros(7, dtype=np.float32)
        for operation in operations:
            kind = operation.get("type", "")
            amount = operation.get("amount", 0)
            numeric = float(amount) if isinstance(amount, (int, float)) else 0.0
            result[0] += numeric / 20.0 if kind == "deal_damage" else 0
            result[1] += numeric / 20.0 if kind in {"prevent_damage", "scale_damage"} else 0
            result[2] += (
                float(operation.get("stack_count", 0)) / 10.0
                if kind in {"apply_status", "remove_status"}
                else 0
            )
            result[3] += numeric / 10.0 if kind in {"gain_resource", "spend_resource"} else 0
            result[4] += float(operation.get("dice_count", 0)) / MAX_DICE_V2 if kind == "roll_dice" else 0
            result[5] += 1 if kind in {"apply_ability_modifier", "modify_die"} else 0
            result[6] += numeric / 20.0
            for outcome in operation.get("outcomes") or []:
                possible_faces = {
                    face
                    for item in operation.get("outcomes") or []
                    for face in item.get("faces") or []
                }
                probability = len(outcome.get("faces") or []) / max(
                    1.0, float(len(possible_faces))
                )
                result += cls.operation_summary(outcome.get("operations") or []) * probability
        return result

    @staticmethod
    def _symbol_counts(dice: list[dict[str, Any]]) -> dict[str, int]:
        counts: dict[str, int] = {}
        for die in dice:
            for symbol in die.get("symbols") or []:
                counts[symbol] = counts.get(symbol, 0) + 1
        return counts

    @staticmethod
    def _one_hot(output: np.ndarray, offset: int, values: tuple[str, ...], selected: str) -> None:
        if selected in values:
            output[offset + values.index(selected)] = 1


def parse_payload_v2(payload: object) -> dict[str, Any]:
    if isinstance(payload, dict):
        return payload
    if isinstance(payload, str) and payload:
        value = json.loads(payload)
        return value if isinstance(value, dict) else {}
    return {}


def enumerate_reroll_outcomes(
    dice: list[dict[str, Any]], indices: list[int], catalog: dict[str, Any]
) -> Iterable[list[dict[str, Any]]]:
    """Yield public counterfactual outcomes without reading authority RNG state."""
    choices: list[list[dict[str, Any]]] = []
    dice_catalog = catalog.get("dice") or {}
    by_index = {int(die.get("index", slot)): die for slot, die in enumerate(dice)}
    for index in indices:
        die = by_index[int(index)]
        definition = dice_catalog.get(die.get("die_id")) or {}
        faces = definition.get("faces") or []
        if not faces:
            raise RuntimeError(f"die {die.get('die_id')!r} has no public faces")
        choices.append(
            [
                {
                    **die,
                    "face": face.get("number", 0),
                    "value": face.get("number", 0),
                    "symbols": [face.get("symbol")],
                }
                for face in faces
            ]
        )
    for replacements in itertools.product(*choices):
        outcome = [dict(die) for die in dice]
        slots = {int(die.get("index", slot)): slot for slot, die in enumerate(outcome)}
        for index, replacement in zip(indices, replacements, strict=True):
            outcome[slots[int(index)]] = replacement
        yield outcome
