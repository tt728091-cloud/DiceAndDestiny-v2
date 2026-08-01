from __future__ import annotations

import time
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import torch
from sb3_contrib import MaskablePPO

from .bridge import AuthorityBridge
from .policies import HeuristicPolicy, select_with_policy
from .schema import MAX_ACTIONS, OBSERVATION_SIZE, SchemaEncoder


@dataclass(frozen=True)
class Demonstrations:
    observations: np.ndarray
    action_masks: np.ndarray
    actions: np.ndarray
    episodes: int
    authority_rejects: int
    truncations: int


def collect_heuristic_demonstrations(
    *,
    binary: Path,
    server_root: Path,
    seed: int,
    decisions: int,
) -> Demonstrations:
    observations = np.empty((decisions, OBSERVATION_SIZE), dtype=np.float32)
    masks = np.empty((decisions, MAX_ACTIONS), dtype=bool)
    actions = np.empty(decisions, dtype=np.int64)
    encoder = SchemaEncoder()
    policies = {"seat-a": HeuristicPolicy(), "seat-b": HeuristicPolicy()}
    collected = 0
    episodes = 0
    rejects = 0
    truncations = 0
    with AuthorityBridge(binary, server_root, session_id=f"imitation-{seed}") as bridge:
        while collected < decisions:
            episode_seed = seed * 10_000_000 + episodes
            for seat_id, policy in policies.items():
                policy.reset(episode_seed, seat_id)
            transition = bridge.reset(
                episode_seed,
                {seat_id: policy.name for seat_id, policy in policies.items()},
            )
            episodes += 1
            while (
                collected < decisions
                and not transition.get("terminal")
                and not transition.get("truncation_reason")
            ):
                actor = transition["actor_id"]
                decision = encoder.encode(transition)
                selected = select_with_policy(policies[actor], transition, encoder)
                observations[collected] = decision.observation
                masks[collected] = decision.action_mask
                actions[collected] = selected
                collected += 1
                transition = bridge.step(selected)
            metrics = transition.get("metrics") or {}
            rejects += int(metrics.get("authority_rejects", 0))
            truncations += int(bool(transition.get("truncation_reason")))
    return Demonstrations(observations, masks, actions, episodes, rejects, truncations)


def behavior_clone(
    model: MaskablePPO,
    demonstrations: Demonstrations,
    *,
    epochs: int,
    batch_size: int,
    seed: int,
) -> dict[str, float | int]:
    device = model.device
    optimizer = model.policy.optimizer
    rng = np.random.default_rng(seed)
    sample_count = len(demonstrations.actions)
    started = time.perf_counter()
    final_loss = 0.0
    for _ in range(epochs):
        order = rng.permutation(sample_count)
        for start in range(0, sample_count, batch_size):
            batch = order[start : start + batch_size]
            observations = torch.as_tensor(demonstrations.observations[batch], device=device)
            masks = torch.as_tensor(demonstrations.action_masks[batch], device=device)
            actions = torch.as_tensor(demonstrations.actions[batch], device=device)
            distribution = model.policy.get_distribution(observations, action_masks=masks)
            loss = -distribution.log_prob(actions).mean()
            optimizer.zero_grad()
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.policy.parameters(), 0.5)
            optimizer.step()
            final_loss = float(loss.detach().cpu())
    accuracy = imitation_accuracy(model, demonstrations, batch_size=batch_size)
    return {
        "demonstrations": sample_count,
        "demonstration_episodes": demonstrations.episodes,
        "epochs": epochs,
        "final_loss": final_loss,
        "training_accuracy": accuracy,
        "elapsed_seconds": time.perf_counter() - started,
        "authority_rejects": demonstrations.authority_rejects,
        "truncations": demonstrations.truncations,
    }


def imitation_accuracy(
    model: MaskablePPO,
    demonstrations: Demonstrations,
    *,
    batch_size: int,
) -> float:
    correct = 0
    with torch.no_grad():
        for start in range(0, len(demonstrations.actions), batch_size):
            stop = start + batch_size
            observations = torch.as_tensor(demonstrations.observations[start:stop], device=model.device)
            masks = torch.as_tensor(demonstrations.action_masks[start:stop], device=model.device)
            distribution = model.policy.get_distribution(observations, action_masks=masks)
            predicted = distribution.get_actions(deterministic=True).detach().cpu().numpy()
            correct += int(np.sum(predicted == demonstrations.actions[start:stop]))
    return correct / len(demonstrations.actions)
