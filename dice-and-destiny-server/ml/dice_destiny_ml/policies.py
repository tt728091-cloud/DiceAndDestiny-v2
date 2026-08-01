from __future__ import annotations

import random
import time
from pathlib import Path
from typing import Protocol

import numpy as np

from .schema import EncodedDecision, SchemaEncoder, parse_payload


class Policy(Protocol):
    name: str
    inference_seconds: list[float]

    def reset(self, seed: int, seat_id: str) -> None: ...

    def select(self, transition: dict, decision: EncodedDecision) -> int: ...


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
        actions = transition["result"].get("legal_actions") or []
        progressing = [index for index in legal if actions[int(index)].get("type") != "planning_keep"]
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


class ModelPolicy:
    def __init__(self, checkpoint: Path, *, deterministic: bool = True, device: str = "cpu") -> None:
        from sb3_contrib import MaskablePPO

        self.checkpoint = checkpoint
        self.name = f"model:{checkpoint}"
        self.deterministic = deterministic
        self.model = MaskablePPO.load(checkpoint, device=device)
        self.inference_seconds: list[float] = []
        self._episode_start = np.array([True], dtype=bool)

    def reset(self, seed: int, seat_id: str) -> None:
        del seed, seat_id
        self._episode_start[...] = True
        self.inference_seconds.clear()

    def select(self, transition: dict, decision: EncodedDecision) -> int:
        del transition
        started = time.perf_counter()
        action, _ = self.model.predict(
            decision.observation,
            episode_start=self._episode_start,
            deterministic=self.deterministic,
            action_masks=decision.action_mask,
        )
        self._episode_start[...] = False
        self.inference_seconds.append(time.perf_counter() - started)
        return int(action)


def build_policy(specification: str, *, device: str = "cpu", deterministic: bool = True) -> Policy:
    if specification == "random":
        return RandomLegalPolicy()
    if specification in {"heuristic", "heuristic-v1"}:
        return HeuristicPolicy()
    if specification.startswith("model:"):
        return ModelPolicy(
            Path(specification.removeprefix("model:")),
            deterministic=deterministic,
            device=device,
        )
    raise ValueError(f"unknown policy specification {specification!r}")


def select_with_policy(policy: Policy, transition: dict, encoder: SchemaEncoder) -> int:
    decision = encoder.encode(transition)
    selected = policy.select(transition, decision)
    if selected < 0 or selected >= len(decision.action_mask) or not decision.action_mask[selected]:
        raise RuntimeError(f"policy {policy.name} selected masked action {selected}")
    return selected
