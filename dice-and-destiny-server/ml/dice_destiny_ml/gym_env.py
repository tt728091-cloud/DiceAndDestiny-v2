from __future__ import annotations

import random
from pathlib import Path
from typing import Any

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from .bridge import AuthorityBridge
from .opponent_pool import OpponentManager
from .policies import Policy, select_with_policy
from .profiling import ProfileCollector, process_snapshot
from .resources import configure_thread_runtime
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
        instrumentation: bool = False,
        torch_threads: int = 1,
        torch_interop_threads: int = 1,
        blas_threads: int = 1,
        historical_cache_size: int = 8,
        opponent_selection_mode: str = "legacy-flat",
    ) -> None:
        configure_thread_runtime(
            torch_threads=torch_threads,
            torch_interop_threads=torch_interop_threads,
            blas_threads=blas_threads,
        )
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
            instrumentation=instrumentation,
        )
        self.training_seed = training_seed
        self.opponent_specs = opponent_specs or ["random", "heuristic"]
        self.learner_seat_mode = learner_seat
        self.device = device
        self.opponent_selection_mode = opponent_selection_mode
        self.episode_index = 0
        self.transition: dict[str, Any] | None = None
        self.decision: EncodedDecision | EncodedDecisionV2 | None = None
        self.learner_seat = "seat-a"
        self.opponent_seat = "seat-b"
        self.opponent: Policy | None = None
        self.episode_return = 0.0
        self.learner_steps = 0
        self._selection_rng = random.Random(training_seed)
        self._opponent_manager = OpponentManager(
            self.opponent_specs,
            device=device,
            deterministic=False,
            historical_cache_size=historical_cache_size,
        )
        self.profile = ProfileCollector(instrumentation)

    def reset(
        self,
        *,
        seed: int | None = None,
        options: dict[str, Any] | None = None,
    ) -> tuple[np.ndarray, dict[str, Any]]:
        super().reset(seed=seed)
        del options
        self.profile.clear()
        self.bridge.profile.clear()
        episode_seed = self.training_seed * 1_000_000 + self.episode_index
        self.learner_seat = self._learner_seat_for_episode()
        self.opponent_seat = "seat-b" if self.learner_seat == "seat-a" else "seat-a"
        with self.profile.span("environment.opponent_selection"):
            selection = self._opponent_manager.select(
                self._selection_rng,
                mode=self.opponent_selection_mode,
            )
            opponent_spec = selection.specification
        self.profile.increment(f"opponent_category.{selection.category}")
        with self.profile.span("environment.opponent_construction"):
            acquisition = self._opponent_manager.acquire(opponent_spec)
            self.opponent = acquisition.policy
        self.profile.increment(f"opponent_cache.{acquisition.cache_status}")
        with self.profile.span("environment.opponent_reset"):
            self.opponent.reset(episode_seed, self.opponent_seat)
        seat_models = {
            self.learner_seat: "current-learner",
            self.opponent_seat: self.opponent.name,
        }
        with self.profile.span("environment.authority_reset"):
            self.transition = self.bridge.reset(episode_seed, seat_models)
        self.episode_index += 1
        self.episode_return = 0.0
        self.learner_steps = 0
        with self.profile.span("environment.initial_opponent_drive"):
            self._drive_opponent()
        if self.transition.get("terminal") or self.transition.get("truncation_reason"):
            raise RuntimeError("episode ended before learner received a decision")
        self.decision = self._encode_transition(self.transition)
        return self.decision.observation, self._base_info(training=True)

    def step(self, action: int) -> tuple[np.ndarray, float, bool, bool, dict[str, Any]]:
        if self.transition is None or self.decision is None:
            raise RuntimeError("reset must be called before step")
        selected = int(action)
        if selected < 0 or selected >= self.maximum_actions or not self.decision.action_mask[selected]:
            raise RuntimeError(f"learner selected masked action {selected}")
        with self.profile.span("environment.learner_authority_step"):
            self.transition = self.bridge.step(selected)
        self.learner_steps += 1
        with self.profile.span("environment.opponent_drive"):
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
            if self.profile.enabled:
                info["throughput_profile"] = self._profile_summary()
            observation = np.zeros(self.observation_size, dtype=np.float32)
            self.decision = type(self.decision)(
                observation, np.zeros(self.maximum_actions, dtype=bool)
            )
            return observation, reward, terminated, truncated, info
        self.decision = self._encode_transition(self.transition)
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
            family = self._opponent_family(self.opponent.name)
            started = len(self.opponent.inference_seconds)
            with self.profile.span(f"environment.opponent_select.{family}"):
                selected = select_with_policy(self.opponent, self.transition, self.encoder)
            if len(self.opponent.inference_seconds) > started:
                self.profile.observe(
                    f"policy.opponent_inference.{family}",
                    self.opponent.inference_seconds[-1],
                )
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
        return list(self._opponent_manager.flat_manifest())

    def _base_info(self, *, training: bool) -> dict[str, Any]:
        return {
            "training": training,
            "battle_id": (self.transition.get("metrics") or {}).get("battle_id") if self.transition else "",
            "seed": (self.transition.get("metrics") or {}).get("seed") if self.transition else None,
        }

    def _encode_transition(
        self, transition: dict[str, Any]
    ) -> EncodedDecision | EncodedDecisionV2:
        with self.profile.span("environment.python_schema_encode"):
            decision = self.encoder.encode(transition)
        if self.profile.enabled:
            valid = int(np.count_nonzero(decision.action_mask))
            self.profile.increment("policy.candidate_decisions")
            self.profile.increment("policy.valid_candidates", valid)
            self.profile.increment("policy.maximum_valid_candidates", 0)
            self.profile.counters["policy.maximum_valid_candidates"] = max(
                self.profile.counters["policy.maximum_valid_candidates"], float(valid)
            )
            self.profile.increment("policy.observation_values", decision.observation.size)
            self.profile.increment(
                "policy.observation_nonzero_values", int(np.count_nonzero(decision.observation))
            )
        return decision

    def _profile_summary(self) -> dict[str, Any]:
        combined = ProfileCollector(True)
        for collector in (self.profile, self.bridge.profile):
            for name, samples in collector.durations.items():
                combined.durations[name].extend(samples)
            for name, value in collector.counters.items():
                combined.counters[name] += value
        result = combined.summary()
        result["process"] = process_snapshot(
            include_torch=True,
            child_pid=self.bridge._process.pid,
        )
        return result

    @staticmethod
    def _opponent_family(name: str) -> str:
        if name.startswith("model:"):
            return "model"
        if name == "mechanics-v2":
            return "mechanics_v2"
        if name == "random":
            return "random"
        return "other"
