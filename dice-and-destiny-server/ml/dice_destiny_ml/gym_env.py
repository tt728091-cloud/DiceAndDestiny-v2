from __future__ import annotations

import random
from pathlib import Path
from typing import Any

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from .bridge import AuthorityBridge
from .policies import Policy, build_policy, select_with_policy
from .schema import MAX_ACTIONS, OBSERVATION_SIZE, EncodedDecision, SchemaEncoder
from .schema_v2 import (
    MAX_ACTIONS_V2,
    OBSERVATION_SCHEMA_V2,
    OBSERVATION_SIZE_V2,
    EncodedDecisionV2,
    SchemaEncoderV2,
)


class AuthorityGymEnv(gym.Env[np.ndarray, int]):
    """Single-learner Gym view over a two-policy authority battle.

    One environment step is one learner action followed by as many independent
    opponent actions as needed to return control to the learner. This prevents
    PPO returns from alternating between opposing reward perspectives.
    """

    metadata = {"render_modes": []}

    def __init__(
        self,
        *,
        binary: Path,
        server_root: Path,
        training_seed: int,
        opponent_specs: list[str] | None = None,
        learner_seat: str = "alternate",
        max_episode_actions: int = 1200,
        device: str = "cpu",
        session_id: str = "train",
        authority_mode: str = "ephemeral",
        telemetry_mode: str = "training",
        transport_mode: str = "full",
        observation_schema: str = "dice-and-destiny-observation-v1",
    ) -> None:
        super().__init__()
        self.observation_schema = observation_schema
        if observation_schema == OBSERVATION_SCHEMA_V2:
            self.maximum_actions = MAX_ACTIONS_V2
            self.observation_size = OBSERVATION_SIZE_V2
            self.encoder: SchemaEncoder | SchemaEncoderV2 = SchemaEncoderV2()
        else:
            self.maximum_actions = MAX_ACTIONS
            self.observation_size = OBSERVATION_SIZE
            self.encoder = SchemaEncoder()
        self.action_space = spaces.Discrete(self.maximum_actions)
        self.observation_space = spaces.Box(
            -10.0, 10.0, (self.observation_size,), dtype=np.float32
        )
        self.bridge = AuthorityBridge(
            binary,
            server_root,
            max_episode_actions=max_episode_actions,
            session_id=session_id,
            authority_mode=authority_mode,
            telemetry_mode=telemetry_mode,
            transport_mode=transport_mode,
            observation_schema=observation_schema,
        )
        self.training_seed = training_seed
        self.opponent_specs = opponent_specs or ["random", "heuristic"]
        self.learner_seat_mode = learner_seat
        self.device = device
        self.episode_index = 0
        self.transition: dict[str, Any] | None = None
        self.decision: EncodedDecision | EncodedDecisionV2 | None = None
        self.learner_seat = "seat-a"
        self.opponent_seat = "seat-b"
        self.opponent: Policy | None = None
        self.episode_return = 0.0
        self.learner_steps = 0
        self._selection_rng = random.Random(training_seed)

    def reset(
        self,
        *,
        seed: int | None = None,
        options: dict[str, Any] | None = None,
    ) -> tuple[np.ndarray, dict[str, Any]]:
        super().reset(seed=seed)
        del options
        episode_seed = self.training_seed * 1_000_000 + self.episode_index
        self.learner_seat = self._learner_seat_for_episode()
        self.opponent_seat = "seat-b" if self.learner_seat == "seat-a" else "seat-a"
        opponent_spec = self._selection_rng.choice(self._available_opponents())
        self.opponent = build_policy(opponent_spec, device=self.device, deterministic=False)
        self.opponent.reset(episode_seed, self.opponent_seat)
        seat_models = {
            self.learner_seat: "current-learner",
            self.opponent_seat: self.opponent.name,
        }
        self.transition = self.bridge.reset(episode_seed, seat_models)
        self.episode_index += 1
        self.episode_return = 0.0
        self.learner_steps = 0
        self._drive_opponent()
        if self.transition.get("terminal") or self.transition.get("truncation_reason"):
            raise RuntimeError("episode ended before learner received a decision")
        self.decision = self.encoder.encode(self.transition)
        return self.decision.observation, self._base_info(training=True)

    def step(self, action: int) -> tuple[np.ndarray, float, bool, bool, dict[str, Any]]:
        if self.transition is None or self.decision is None:
            raise RuntimeError("reset must be called before step")
        selected = int(action)
        if selected < 0 or selected >= self.maximum_actions or not self.decision.action_mask[selected]:
            raise RuntimeError(f"learner selected masked action {selected}")
        self.transition = self.bridge.step(selected)
        self.learner_steps += 1
        self._drive_opponent()
        terminated = bool(self.transition.get("terminal"))
        truncated = bool(self.transition.get("truncation_reason"))
        reward = self._terminal_reward() if terminated else 0.0
        self.episode_return += reward
        info = self._base_info(training=True)
        if terminated or truncated:
            info.update(
                {
                    "episode": {"r": self.episode_return, "l": self.learner_steps},
                    "episode_metrics": self.transition.get("metrics") or {},
                    "replay": self.transition.get("replay"),
                    "learner_seat": self.learner_seat,
                    "opponent": self.opponent.name if self.opponent else "",
                }
            )
            observation = np.zeros(self.observation_size, dtype=np.float32)
            self.decision = type(self.decision)(
                observation, np.zeros(self.maximum_actions, dtype=bool)
            )
            return observation, reward, terminated, truncated, info
        self.decision = self.encoder.encode(self.transition)
        return self.decision.observation, reward, terminated, truncated, info

    def action_masks(self) -> np.ndarray:
        if self.decision is None:
            return np.zeros(self.maximum_actions, dtype=bool)
        return self.decision.action_mask.copy()

    def close(self) -> None:
        self.bridge.close()
        super().close()

    def _drive_opponent(self) -> None:
        assert self.transition is not None
        assert self.opponent is not None
        while (
            not self.transition.get("terminal")
            and not self.transition.get("truncation_reason")
            and self.transition.get("actor_id") != self.learner_seat
        ):
            if self.transition.get("actor_id") != self.opponent_seat:
                raise RuntimeError(f"wrong-seat transition: {self.transition.get('actor_id')!r}")
            selected = select_with_policy(self.opponent, self.transition, self.encoder)
            self.transition = self.bridge.step(selected)

    def _terminal_reward(self) -> float:
        winner = self.transition.get("winner") if self.transition else ""
        if not winner:
            return 0.0
        return 1.0 if winner == self.learner_seat else -1.0

    def _learner_seat_for_episode(self) -> str:
        if self.learner_seat_mode == "alternate":
            return "seat-a" if self.episode_index % 2 == 0 else "seat-b"
        if self.learner_seat_mode in {"seat-a", "seat-b"}:
            return self.learner_seat_mode
        raise ValueError(f"unknown learner seat mode {self.learner_seat_mode!r}")

    def _available_opponents(self) -> list[str]:
        available: list[str] = []
        for specification in self.opponent_specs:
            if specification.startswith("pool:"):
                directory = Path(specification.removeprefix("pool:"))
                available.extend(f"model:{path}" for path in sorted(directory.glob("*.zip")))
            else:
                available.append(specification)
        return available or ["random"]

    def _base_info(self, *, training: bool) -> dict[str, Any]:
        return {
            "training": training,
            "battle_id": (self.transition.get("metrics") or {}).get("battle_id") if self.transition else "",
            "seed": (self.transition.get("metrics") or {}).get("seed") if self.transition else None,
        }
