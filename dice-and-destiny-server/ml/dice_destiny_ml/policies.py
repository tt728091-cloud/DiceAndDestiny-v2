from __future__ import annotations

import itertools
import random
import time
from pathlib import Path
from typing import Protocol

import numpy as np

from .manifest_v3 import OBSERVATION_SCHEMA_V3
from .manifest_v4 import OBSERVATION_SCHEMA_V4
from .manifest_v5 import OBSERVATION_SCHEMA_V5
from .schema import OBSERVATION_SIZE, EncodedDecision, SchemaEncoder, parse_payload
from .schema_v2 import (
    OBSERVATION_SCHEMA_V2,
    OBSERVATION_SIZE_V2,
    EncodedDecisionV2,
    SchemaEncoderV2,
    enumerate_reroll_outcomes,
    parse_payload_v2,
)
from .schema_v3 import EncodedDecisionV3, SchemaEncoderV3
from .schema_v4 import EncodedDecisionV4, SchemaEncoderV4
from .schema_v5 import EncodedDecisionV5, SchemaEncoderV5


class Policy(Protocol):
    name: str
    inference_seconds: list[float]

    def reset(self, seed: int, seat_id: str) -> None: ...

    def select(
        self,
        transition: dict,
        decision: (
            EncodedDecision | EncodedDecisionV2 | EncodedDecisionV3 | EncodedDecisionV4 | EncodedDecisionV5
        ),
    ) -> int: ...


class RandomLegalPolicy:
    name = "random"

    def __init__(self) -> None:
        self._rng = random.Random()
        self.inference_seconds: list[float] = []

    def reset(self, seed: int, seat_id: str) -> None:
        self._rng.seed(seed ^ (0xA5A5 if seat_id == "seat-a" else 0x5A5A))
        self.inference_seconds.clear()

    def select(self, transition: dict, decision: EncodedDecision) -> int:
        started = time.perf_counter()
        legal = np.flatnonzero(decision.action_mask)
        actions = transition.get("result", {}).get("legal_actions") or []
        candidate_types = (transition.get("encoded_decision") or {}).get("candidate_types") or []
        progressing = [
            index
            for index in legal
            if (actions[int(index)].get("type") if actions else candidate_types[int(index)])
            != "planning_keep"
        ]
        # planning_keep only changes a provisional subset and can be repeated
        # indefinitely. The complete list remains visible, but the random
        # liveness baseline samples it only when no progress-capable command
        # exists (a state the current authority does not normally produce).
        candidates = progressing or legal.tolist()
        selected = int(self._rng.choice(candidates))
        self.inference_seconds.append(time.perf_counter() - started)
        return selected


class HeuristicPolicy:
    """Transparent terminal-seeking Blade Warden baseline.

    It rolls before committing, selects the highest-damage qualified ability,
    rerolls as many dice as possible when no attack is qualified, uses the basic
    defense, and otherwise prefers helpful reaction cards over passing.
    """

    name = "heuristic-v1"

    def __init__(self) -> None:
        self.inference_seconds: list[float] = []

    def reset(self, seed: int, seat_id: str) -> None:
        del seed, seat_id
        self.inference_seconds.clear()

    def select(self, transition: dict, decision: EncodedDecision) -> int:
        started = time.perf_counter()
        if transition.get("encoded_decision"):
            raise RuntimeError("heuristic policy requires the full viewer-safe transport")
        actions = transition["result"].get("legal_actions") or []
        snapshot = transition["result"].get("snapshot") or {}
        viewer = snapshot.get("viewer_actor_id", transition.get("actor_id", ""))
        actor = (snapshot.get("actors") or {}).get(viewer) or {}
        instances = actor.get("card_instances") or {}
        scores: list[tuple[float, int]] = []
        for index, action in enumerate(actions):
            kind = action.get("type", "")
            payload = parse_payload(action.get("payload"))
            score = self._score(kind, payload, instances)
            scores.append((score, index))
        if not scores:
            raise RuntimeError("heuristic received no legal action")
        selected = max(scores)[1]
        self.inference_seconds.append(time.perf_counter() - started)
        return selected

    def _score(self, kind: str, payload: dict, instances: dict) -> float:
        if kind == "planning_roll":
            return 100.0
        if kind == "planning_select_ability":
            ability = payload.get("ability_id", "")
            return 95.0 + {"sword_cut": 3.0, "basic_defense": 4.0}.get(ability, 0.0)
        if kind == "planning_reroll":
            return 80.0 + len(payload.get("reroll_indices") or [])
        if kind == "roll_dice":
            return 90.0 - len(payload.get("reroll_indices") or []) * 0.01
        if kind == "planning_keep":
            # Keeping alone does not commit planning and can be repeated, so
            # reroll or pass instead of cycling on equivalent keep subsets.
            return 10.0 + len(payload.get("kept_indices") or []) * 0.01
        if kind == "planning_commit_cards":
            return 15.0 + self._card_score(payload.get("card_ids") or [], instances)
        if kind == "commit_interaction":
            cards = (payload.get("commitment") or {}).get("card_ids") or []
            return 65.0 + self._card_score(cards, instances)
        if kind == "planning_select_targets":
            return 94.0
        if kind == "planning_pass":
            return 30.0
        if kind == "pass":
            return 20.0
        return 0.0

    @staticmethod
    def _card_score(card_ids: list[str], instances: dict) -> float:
        definitions = [(instances.get(card_id) or {}).get("definition_id", "") for card_id in card_ids]
        values = {
            "tip_it": 4.0,
            "battle_focus": 3.0,
            "double_up": 2.5,
            "antidote": 3.5,
            "second_wind": 4.0,
        }
        return sum(values.get(definition, 1.0) for definition in definitions)


class MechanicsPolicyV2:
    """Generic public-mechanics teacher with counterfactual one-roll lookahead.

    It never branches on an ability, card, character, die, or symbol ID. IDs
    are used only to join a legal command to its public authored definition.
    Reroll outcomes enumerate public die faces and never inspect authority RNG.
    """

    name = "mechanics-v2"

    def __init__(self) -> None:
        self.inference_seconds: list[float] = []
        self._outcome_tables: dict[tuple, tuple[np.ndarray, list[dict[int, int]]]] = {}

    def reset(self, seed: int, seat_id: str) -> None:
        del seed, seat_id
        self.inference_seconds.clear()

    def select(self, transition: dict, decision: EncodedDecisionV2) -> int:
        started = time.perf_counter()
        result = transition.get("result") or {}
        snapshot = result.get("snapshot") or {}
        catalog = snapshot.get("content_catalog") or {}
        viewer = snapshot.get("viewer_actor_id") or transition.get("actor_id")
        actor = (snapshot.get("actors") or {}).get(viewer) or {}
        actions = result.get("legal_actions") or []
        if not catalog or not actions:
            raise RuntimeError("mechanics-v2 requires full viewer-safe v2 transport")
        selectable = np.flatnonzero(decision.action_mask)
        authority_indices = getattr(decision, "authority_indices", None)
        selected = max(
            (
                self._score(
                    actions[int(authority_indices[index]) if authority_indices is not None else int(index)],
                    actor,
                    catalog,
                ),
                int(index),
            )
            for index in selectable
        )[1]
        self.inference_seconds.append(time.perf_counter() - started)
        return selected

    def _score(self, action: dict, actor: dict, catalog: dict) -> float:
        kind = action.get("type", "")
        payload = parse_payload_v2(action.get("payload"))
        commitment = payload.get("commitment") or {}
        ability_id = payload.get("ability_id") or commitment.get("choice_id") or ""
        cards = payload.get("card_ids") or commitment.get("card_ids") or []
        card_utility = self._card_utility(cards, actor, catalog)
        if kind == "planning_roll":
            return 10_000.0
        if kind == "planning_reroll":
            indices = [int(index) for index in payload.get("reroll_indices") or []]
            return 1_000.0 + 100.0 * self._expected_best_ability(actor, catalog, indices)
        if kind == "planning_select_ability":
            return 1_000.0 + 100.0 * self._qualified_ability_utility(ability_id, actor, catalog)
        if kind == "planning_select_targets":
            return 900.0
        if kind == "planning_commit_cards":
            ability_utility = self._qualified_ability_utility(ability_id, actor, catalog)
            return 700.0 + 100.0 * (ability_utility + card_utility)
        if kind == "roll_dice":
            return 800.0
        if kind == "commit_interaction":
            return 600.0 + 100.0 * card_utility
        if kind == "planning_pass":
            return 300.0
        if kind == "pass":
            return 200.0
        if kind == "planning_keep":
            return 100.0
        return 0.0

    def _expected_best_ability(self, actor: dict, catalog: dict, indices: list[int]) -> float:
        dice = (actor.get("dice") or {}).get("dice") or []
        if not indices:
            return self._best_ability_utility(actor, catalog, dice)
        table = self._outcome_table(actor, catalog, dice)
        if table is not None:
            utilities, face_indices = table
            rerolled = set(indices)
            selection = tuple(
                slice(None)
                if int(die.get("index", slot)) in rerolled
                else face_indices[slot][int(die.get("face", 0))]
                for slot, die in enumerate(dice)
            )
            return float(np.mean(utilities[selection]))
        total = 0.0
        count = 0
        for outcome in enumerate_reroll_outcomes(dice, indices, catalog):
            total += self._best_ability_utility(actor, catalog, outcome)
            count += 1
        return total / max(count, 1)

    def _outcome_table(
        self, actor: dict, catalog: dict, dice: list[dict]
    ) -> tuple[np.ndarray, list[dict[int, int]]] | None:
        dice_catalog = catalog.get("dice") or {}
        face_options = []
        for die in dice:
            faces = (dice_catalog.get(die.get("die_id")) or {}).get("faces") or []
            if not faces:
                return None
            face_options.append(faces)
        combinations = int(np.prod([len(faces) for faces in face_options], dtype=np.int64))
        if combinations > 100_000:
            return None
        board = tuple(actor.get("offensive_abilities") or [])
        key = (
            board,
            tuple(
                sorted(
                    (
                        modifier.get("ability_id", ""),
                        modifier.get("bonus_id", ""),
                        modifier.get("source_card_instance_id", ""),
                    )
                    for modifier in actor.get("ability_modifiers") or []
                )
            ),
            tuple(die.get("die_id") for die in dice),
            tuple(
                tuple((int(face.get("number", 0)), face.get("symbol", "")) for face in faces)
                for faces in face_options
            ),
        )
        cached = self._outcome_tables.get(key)
        if cached is not None:
            return cached
        shape = tuple(len(faces) for faces in face_options)
        utilities = np.empty(shape, dtype=np.float32)
        for coordinates in itertools.product(*(range(size) for size in shape)):
            outcome = []
            for slot, coordinate in enumerate(coordinates):
                face = face_options[slot][coordinate]
                outcome.append(
                    {
                        **dice[slot],
                        "face": face.get("number", 0),
                        "value": face.get("number", 0),
                        "symbols": [face.get("symbol")],
                    }
                )
            utilities[coordinates] = self._best_ability_utility(actor, catalog, outcome)
        face_indices = [
            {int(face.get("number", 0)): index for index, face in enumerate(faces)} for faces in face_options
        ]
        result = (utilities, face_indices)
        self._outcome_tables[key] = result
        return result

    def _qualified_ability_utility(self, ability_id: str, actor: dict, catalog: dict) -> float:
        if not ability_id:
            return 0.0
        ability = SchemaEncoderV2.effective_ability(ability_id, actor, catalog)
        dice = (actor.get("dice") or {}).get("dice") or []
        return self._ability_utility(ability, dice)

    def _best_ability_utility(self, actor: dict, catalog: dict, dice: list[dict]) -> float:
        board = list(actor.get("offensive_abilities") or [])
        return max(
            (
                self._ability_utility(SchemaEncoderV2.effective_ability(identifier, actor, catalog), dice)
                for identifier in board
            ),
            default=0.0,
        )

    @staticmethod
    def _ability_utility(ability: dict, dice: list[dict]) -> float:
        encoder = SchemaEncoderV2()
        tiers = encoder.ability_tiers(ability)
        activation = [
            (tier, conditional)
            for tier, conditional in tiers
            if not conditional and encoder.tier_met(tier, dice)
        ]
        if not activation:
            return 0.0
        best = max(MechanicsPolicyV2._effect_utility(tier.get("operations") or []) for tier, _ in activation)
        bonuses = sum(
            MechanicsPolicyV2._effect_utility(tier.get("operations") or [])
            for tier, conditional in tiers
            if conditional and encoder.tier_met(tier, dice)
        )
        return best + bonuses

    @staticmethod
    def _effect_utility(operations: list[dict]) -> float:
        summary = SchemaEncoderV2.operation_summary(operations)
        # All weights are generic mechanics dimensions. Terminal rollouts remain
        # the evaluation objective; this score is only a transparent teacher.
        return float(
            summary[0] * 20.0
            + summary[1] * 20.0
            + summary[2] * 12.0
            + summary[3] * 10.0
            + summary[4] * 4.0
            + summary[5] * 2.0
        )

    def _card_utility(self, instance_ids: list[str], actor: dict, catalog: dict) -> float:
        instances = actor.get("card_instances") or {}
        cards = catalog.get("cards") or {}
        result = 0.0
        for instance_id in instance_ids:
            definition_id = (instances.get(instance_id) or {}).get("definition_id", "")
            definition = cards.get(definition_id) or {}
            result += self._effect_utility(definition.get("operations") or [])
            result -= float((definition.get("cost") or {}).get("energy", 0))
        return result


class ModelPolicy:
    def __init__(self, checkpoint: Path, *, deterministic: bool = True, device: str = "cpu") -> None:
        from sb3_contrib import MaskablePPO

        self.checkpoint = checkpoint
        self.name = f"model:{checkpoint}"
        self.deterministic = deterministic
        self.model = MaskablePPO.load(checkpoint, device=device)
        shape = tuple(self.model.observation_space.shape or ())
        if shape == (OBSERVATION_SIZE,):
            self.observation_schema = "dice-and-destiny-observation-v1"
        elif shape == (OBSERVATION_SIZE_V2,):
            self.observation_schema = OBSERVATION_SCHEMA_V2
        elif hasattr(self.model.policy, "manifest"):
            manifest = self.model.policy.manifest
            if shape != (manifest.layout.observation_size,):
                raise RuntimeError("entity checkpoint shape does not match its frozen manifest")
            self.observation_schema = manifest.observation_schema
            self.observation_manifest = manifest
        else:
            raise RuntimeError(f"checkpoint has unsupported observation shape {shape}")
        self.inference_seconds: list[float] = []
        self._episode_start = np.array([True], dtype=bool)
        import torch

        self._sampling_generator = torch.Generator(device="cpu")

    def reset(self, seed: int, seat_id: str) -> None:
        seat_salt = 0xA5A5 if seat_id == "seat-a" else 0x5A5A
        self._sampling_generator.manual_seed((int(seed) ^ seat_salt) & ((1 << 63) - 1))
        self._episode_start[...] = True
        self.inference_seconds.clear()

    def select(
        self,
        transition: dict,
        decision: (
            EncodedDecision | EncodedDecisionV2 | EncodedDecisionV3 | EncodedDecisionV4 | EncodedDecisionV5
        ),
    ) -> int:
        del transition
        started = time.perf_counter()
        observation, _ = self.model.policy.obs_to_tensor(decision.observation)
        distribution = self.model.policy.get_distribution(
            observation,
            action_masks=decision.action_mask,
        )
        probabilities = distribution.distribution.probs
        if self.deterministic:
            action = probabilities.argmax(dim=1)
        else:
            import torch

            action = torch.multinomial(
                probabilities.detach().cpu(),
                1,
                generator=self._sampling_generator,
            ).flatten()
        self._episode_start[...] = False
        self.inference_seconds.append(time.perf_counter() - started)
        return int(action.item())


def build_policy(specification: str, *, device: str = "cpu", deterministic: bool = True) -> Policy:
    if specification == "random":
        return RandomLegalPolicy()
    if specification in {"heuristic", "heuristic-v1"}:
        return HeuristicPolicy()
    if specification in {"mechanics", "mechanics-v2"}:
        return MechanicsPolicyV2()
    if specification.startswith("model:"):
        return ModelPolicy(
            Path(specification.removeprefix("model:")),
            deterministic=deterministic,
            device=device,
        )
    raise ValueError(f"unknown policy specification {specification!r}")


def select_with_policy(
    policy: Policy,
    transition: dict,
    encoder: SchemaEncoder | SchemaEncoderV2 | SchemaEncoderV3 | SchemaEncoderV4 | SchemaEncoderV5,
) -> int:
    required_schema = getattr(policy, "observation_schema", None)
    if required_schema == OBSERVATION_SCHEMA_V5:
        decision = SchemaEncoderV5(policy.observation_manifest).encode(transition)
    elif required_schema == OBSERVATION_SCHEMA_V4:
        decision = SchemaEncoderV4(policy.observation_manifest).encode(transition)
    elif required_schema == OBSERVATION_SCHEMA_V3:
        decision = SchemaEncoderV3(policy.observation_manifest).encode(transition)
    elif required_schema == OBSERVATION_SCHEMA_V2 and not isinstance(encoder, SchemaEncoderV2):
        decision = SchemaEncoderV2().encode(transition)
    elif (
        required_schema
        and required_schema
        not in {OBSERVATION_SCHEMA_V2, OBSERVATION_SCHEMA_V3, OBSERVATION_SCHEMA_V4, OBSERVATION_SCHEMA_V5}
        and isinstance(encoder, (SchemaEncoderV2, SchemaEncoderV3, SchemaEncoderV4, SchemaEncoderV5))
    ):
        decision = SchemaEncoder().encode(transition)
    else:
        decision = encoder.encode(transition)
    selected = policy.select(transition, decision)
    if selected < 0 or selected >= len(decision.action_mask) or not decision.action_mask[selected]:
        raise RuntimeError(f"policy {policy.name} selected masked action {selected}")
    if isinstance(decision, EncodedDecisionV4):
        return int(decision.authority_indices[selected])
    return selected
