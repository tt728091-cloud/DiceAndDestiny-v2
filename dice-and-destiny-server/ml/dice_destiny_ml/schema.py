from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any

import numpy as np

MAX_ACTIONS = 128
BASE_FEATURES = 128
ACTION_FEATURES = 32
OBSERVATION_SIZE = BASE_FEATURES + MAX_ACTIONS * ACTION_FEATURES

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
class EncodedDecision:
    observation: np.ndarray
    action_mask: np.ndarray


class SchemaEncoder:
    """Encode only a viewer-safe result and its listed authority commands.

    The first 128 values describe public state plus the current viewer's private
    state. Each of the 128 candidate slots then owns 32 features. Unused slots
    are zero and masked false. Stable SHA-256 buckets encode authored IDs without
    depending on transient card-instance or battle IDs.
    """

    def encode(self, transition: dict[str, Any]) -> EncodedDecision:
        result = transition["result"]
        snapshot = result.get("snapshot") or {}
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        if viewer not in ("seat-a", "seat-b"):
            raise ValueError(f"invalid viewer actor {viewer!r}")
        opponent = "seat-b" if viewer == "seat-a" else "seat-a"
        actors = snapshot.get("actors") or {}
        own = actors.get(viewer) or {}
        other = actors.get(opponent) or {}
        base = np.zeros(BASE_FEATURES, dtype=np.float32)
        base[0] = viewer == "seat-a"
        base[1] = viewer == "seat-b"
        base[2] = min(float(snapshot.get("round", 0)) / 50.0, 1.0)
        base[3] = min(float(snapshot.get("completed_rounds", 0)) / 50.0, 1.0)
        self._one_hot(base, 4, SEGMENTS, snapshot.get("segment", ""))
        self._bucket_one_hot(base, 8, 16, snapshot.get("stage", ""))
        priority = snapshot.get("priority_actor_id", "")
        base[24] = priority == viewer
        base[25] = priority == opponent
        base[26] = not priority
        self._actor_scalars(base, 27, own)
        self._actor_scalars(base, 39, other)
        self._dice_features(base, 51, own)
        self._dice_features(base, 61, other)
        self._status_buckets(base, 71, own)
        self._status_buckets(base, 79, other)
        self._hand_buckets(base, 87, own)
        self._id_buckets(base, 99, 8, own.get("qualified_abilities") or [])
        self._id_buckets(base, 107, 8, [own.get("selected_ability", "")])
        self._id_buckets(base, 115, 8, [other.get("selected_ability", "")])
        base[123] = len(result.get("legal_actions") or []) / MAX_ACTIONS
        base[124] = float(snapshot.get("status") == "victory")
        base[125] = float(snapshot.get("status") == "defeat")
        base[126] = float(own.get("defeat_state") == "defeated")
        base[127] = float(other.get("defeat_state") == "defeated")

        actions = result.get("legal_actions") or []
        if len(actions) > MAX_ACTIONS:
            raise RuntimeError(
                f"authority produced {len(actions)} candidates; schema capacity is {MAX_ACTIONS}"
            )
        candidate_matrix = np.zeros((MAX_ACTIONS, ACTION_FEATURES), dtype=np.float32)
        mask = np.zeros(MAX_ACTIONS, dtype=bool)
        for index, action in enumerate(actions):
            candidate_matrix[index] = self._action_features(action, viewer, opponent)
            mask[index] = True
        if not mask.any() and not transition.get("terminal") and not transition.get("truncation_reason"):
            raise RuntimeError("nonterminal decision has no legal candidates")
        observation = np.concatenate((base, candidate_matrix.reshape(-1))).astype(np.float32, copy=False)
        return EncodedDecision(observation=observation, action_mask=mask)

    def _actor_scalars(self, output: np.ndarray, offset: int, actor: dict[str, Any]) -> None:
        values = (
            float(actor.get("current_health", 0)) / max(float(actor.get("max_health", 20)), 1.0),
            float(actor.get("max_health", 0)) / 30.0,
            float(actor.get("energy_points", 0)) / max(float(actor.get("max_energy_points", 10)), 1.0),
            float(actor.get("hand_count", 0)) / 20.0,
            float(actor.get("deck_count", 0)) / 30.0,
            float(actor.get("discard_count", 0)) / 30.0,
            float(actor.get("removed_count", 0)) / 30.0,
            float(actor.get("dice_count", 0)) / 10.0,
            float(actor.get("ability_count", 0)) / 10.0,
            float(len(actor.get("statuses") or [])) / 10.0,
            float(len(actor.get("tokens") or [])) / 10.0,
            float(actor.get("defeat_state") == "defeated"),
        )
        output[offset : offset + len(values)] = values

    def _dice_features(self, output: np.ndarray, offset: int, actor: dict[str, Any]) -> None:
        roll_history = actor.get("roll_history") or []
        latest = roll_history[-1] if roll_history else {}
        dice = latest.get("dice") or []
        output[offset] = min(float(latest.get("number", 0)) / 3.0, 1.0)
        for index, die in enumerate(dice[:5]):
            output[offset + 1 + index] = float(die.get("face", 0)) / 6.0
        output[offset + 6] = float(len(dice)) / 5.0
        output[offset + 7] = float(len(latest.get("kept_indices") or [])) / 5.0
        output[offset + 8] = float(bool(actor.get("selected_ability")))
        output[offset + 9] = float(len(actor.get("selected_targets") or [])) / 2.0

    def _status_buckets(self, output: np.ndarray, offset: int, actor: dict[str, Any]) -> None:
        for status in actor.get("statuses") or []:
            identifier = status.get("definition_id") or status.get("id") or ""
            bucket = stable_bucket(identifier, 8)
            stacks = status.get("stack_count", status.get("stacks", 1))
            output[offset + bucket] += min(float(stacks) / 5.0, 1.0)

    def _hand_buckets(self, output: np.ndarray, offset: int, actor: dict[str, Any]) -> None:
        instances = actor.get("card_instances") or {}
        for instance_id in actor.get("hand") or []:
            definition = (instances.get(instance_id) or {}).get("definition_id", "")
            output[offset + stable_bucket(definition, 12)] += 0.2

    def _id_buckets(self, output: np.ndarray, offset: int, count: int, identifiers: list[str]) -> None:
        for identifier in identifiers:
            if identifier:
                output[offset + stable_bucket(identifier, count)] = 1.0

    def _action_features(self, action: dict[str, Any], viewer: str, opponent: str) -> np.ndarray:
        features = np.zeros(ACTION_FEATURES, dtype=np.float32)
        kind = action.get("type", "")
        self._one_hot(features, 0, COMMAND_TYPES, kind)
        payload = parse_payload(action.get("payload"))
        features[10] = kind in {"planning_pass", "pass"}
        features[11] = kind in {"planning_roll", "roll_dice"}
        features[12] = kind in {"planning_keep", "planning_reroll"}
        targets = payload.get("target_ids") or (payload.get("commitment") or {}).get("target_ids") or []
        features[13] = viewer in targets
        features[14] = opponent in targets
        features[15] = min(len(targets) / 2.0, 1.0)
        indices = (
            payload.get("reroll_indices")
            or payload.get("kept_indices")
            or (payload.get("commitment") or {}).get("die_indices")
            or []
        )
        features[16] = min(len(indices) / 5.0, 1.0)
        features[17] = sum(1 << int(index) for index in indices if 0 <= int(index) < 5) / 31.0
        cards = payload.get("card_ids") or (payload.get("commitment") or {}).get("card_ids") or []
        features[18] = min(len(cards) / 5.0, 1.0)
        for card_id in cards:
            features[19 + stable_bucket(card_id.rsplit("-", 1)[0], 4)] = 1.0
        ability = payload.get("ability_id") or (payload.get("commitment") or {}).get("choice_id") or ""
        if ability:
            features[23 + stable_bucket(ability, 4)] = 1.0
        adjustments = (payload.get("commitment") or {}).get("planning_adjustments") or []
        features[27] = bool(adjustments)
        if adjustments:
            adjustment = adjustments[0]
            features[28] = adjustment.get("actor_id") == viewer
            features[29] = adjustment.get("actor_id") == opponent
            features[30] = float(adjustment.get("face", 0)) / 6.0
        features[31] = stable_bucket(json.dumps(payload, sort_keys=True), 1024) / 1023.0
        return features

    @staticmethod
    def _one_hot(output: np.ndarray, offset: int, values: tuple[str, ...], selected: str) -> None:
        try:
            output[offset + values.index(selected)] = 1.0
        except ValueError:
            pass

    @staticmethod
    def _bucket_one_hot(output: np.ndarray, offset: int, count: int, identifier: str) -> None:
        if identifier:
            output[offset + stable_bucket(identifier, count)] = 1.0


def parse_payload(payload: object) -> dict[str, Any]:
    if isinstance(payload, dict):
        return payload
    if isinstance(payload, str) and payload:
        value = json.loads(payload)
        return value if isinstance(value, dict) else {}
    return {}


def stable_bucket(identifier: str, count: int) -> int:
    digest = hashlib.sha256(identifier.encode("utf-8")).digest()
    return int.from_bytes(digest[:8], "big") % count
