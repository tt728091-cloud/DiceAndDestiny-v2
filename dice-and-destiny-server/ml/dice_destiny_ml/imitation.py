from __future__ import annotations

import hashlib
import json
import time
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import torch
from sb3_contrib import MaskablePPO

from .bridge import AuthorityBridge
from .manifest_v3 import OBSERVATION_SCHEMA_V3, ObservationManifestV3
from .manifest_v4 import OBSERVATION_SCHEMA_V4, ObservationManifestV4
from .manifest_v5 import OBSERVATION_SCHEMA_V5, ObservationManifestV5
from .policies import build_policy
from .schema import MAX_ACTIONS, OBSERVATION_SIZE, SchemaEncoder
from .schema_v2 import (
    MAX_ACTIONS_V2,
    OBSERVATION_SCHEMA_V2,
    OBSERVATION_SIZE_V2,
    SchemaEncoderV2,
)
from .schema_v3 import SchemaEncoderV3
from .schema_v4 import EncodedDecisionV4, SchemaEncoderV4
from .schema_v5 import SchemaEncoderV5


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
    return collect_demonstrations(
        binary=binary,
        server_root=server_root,
        seed=seed,
        decisions=decisions,
        observation_schema="dice-and-destiny-observation-v1",
        teacher="heuristic-v1",
    )


def collect_demonstrations(
    *,
    binary: Path,
    server_root: Path,
    seed: int,
    decisions: int,
    observation_schema: str,
    teacher: str,
    observation_manifest: Path | None = None,
    seat_definitions: dict[str, str] | None = None,
) -> Demonstrations:
    if observation_schema == OBSERVATION_SCHEMA_V5:
        if observation_manifest is None:
            raise ValueError("observation v5 demonstrations require a frozen manifest")
        manifest_v5 = ObservationManifestV5.load(observation_manifest)
        observation_size = manifest_v5.layout.observation_size
        maximum_actions = manifest_v5.maximum_legal_candidates
        encoder = SchemaEncoderV5(manifest_v5)
    elif observation_schema == OBSERVATION_SCHEMA_V4:
        if observation_manifest is None:
            raise ValueError("observation v4 demonstrations require a frozen manifest")
        manifest_v4 = ObservationManifestV4.load(observation_manifest)
        observation_size = manifest_v4.layout.observation_size
        maximum_actions = manifest_v4.maximum_legal_candidates
        encoder = SchemaEncoderV4(manifest_v4)
    elif observation_schema == OBSERVATION_SCHEMA_V3:
        if observation_manifest is None:
            raise ValueError("observation v3 demonstrations require a frozen manifest")
        manifest = ObservationManifestV3.load(observation_manifest)
        observation_size = manifest.layout.observation_size
        maximum_actions = manifest.maximum_legal_candidates
        encoder: SchemaEncoder | SchemaEncoderV2 | SchemaEncoderV3 = SchemaEncoderV3(manifest)
    elif observation_schema == OBSERVATION_SCHEMA_V2:
        observation_size, maximum_actions = OBSERVATION_SIZE_V2, MAX_ACTIONS_V2
        encoder = SchemaEncoderV2()
    else:
        observation_size, maximum_actions = OBSERVATION_SIZE, MAX_ACTIONS
        encoder = SchemaEncoder()
    observations = np.empty((decisions, observation_size), dtype=np.float32)
    masks = np.empty((decisions, maximum_actions), dtype=bool)
    actions = np.empty(decisions, dtype=np.int64)
    policies = {
        "seat-a": build_policy(teacher),
        "seat-b": build_policy(teacher),
    }
    collected = 0
    episodes = 0
    rejects = 0
    truncations = 0
    with AuthorityBridge(
        binary,
        server_root,
        session_id=f"imitation-{seed}",
        observation_schema=OBSERVATION_SCHEMA_V2
        if observation_schema in {OBSERVATION_SCHEMA_V4, OBSERVATION_SCHEMA_V5}
        else observation_schema,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
        observation_manifest=(
            None
            if observation_schema in {OBSERVATION_SCHEMA_V4, OBSERVATION_SCHEMA_V5}
            else observation_manifest
        ),
    ) as bridge:
        while collected < decisions:
            episode_seed = seed * 10_000_000 + episodes
            for seat_id, policy in policies.items():
                policy.reset(episode_seed, seat_id)
            transition = bridge.reset(
                episode_seed,
                {seat_id: policy.name for seat_id, policy in policies.items()},
                seat_definitions=seat_definitions,
            )
            episodes += 1
            while (
                collected < decisions
                and not transition.get("terminal")
                and not transition.get("truncation_reason")
            ):
                actor = transition["actor_id"]
                decision = encoder.encode(transition)
                selected = policies[actor].select(transition, decision)
                if (
                    selected < 0
                    or selected >= len(decision.action_mask)
                    or not decision.action_mask[selected]
                ):
                    raise RuntimeError(f"teacher selected masked model action {selected}")
                observations[collected] = decision.observation
                masks[collected] = decision.action_mask
                actions[collected] = selected
                collected += 1
                authority_selected = (
                    int(decision.authority_indices[selected])
                    if isinstance(decision, EncodedDecisionV4)
                    else selected
                )
                transition = bridge.step(authority_selected)
            metrics = transition.get("metrics") or {}
            rejects += int(metrics.get("authority_rejects", 0))
            truncations += int(bool(transition.get("truncation_reason")))
    return Demonstrations(observations, masks, actions, episodes, rejects, truncations)


def collect_tactical_demonstrations(
    *,
    binary: Path,
    server_root: Path,
    corpus_file: Path,
) -> Demonstrations:
    """Recreate fixed public tactical states and resolve their semantic teacher labels."""
    corpus = json.loads(corpus_file.read_text())
    records = corpus["records"]
    encoder = SchemaEncoderV2()
    observations = np.empty((len(records), OBSERVATION_SIZE_V2), dtype=np.float32)
    masks = np.empty((len(records), MAX_ACTIONS_V2), dtype=bool)
    actions = np.empty(len(records), dtype=np.int64)
    with AuthorityBridge(
        binary,
        server_root,
        session_id="corrective-tactical-imitation",
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="training",
    ) as bridge:
        for row, record in enumerate(records):
            seed = int(record["seed"])
            transition = bridge.reset(seed, {"seat-a": "corpus", "seat-b": "corpus"})
            roll = next(
                index
                for index, action in enumerate(transition["result"]["legal_actions"])
                if action["type"] == "planning_roll"
            )
            transition = bridge.step(roll)
            decision = encoder.encode(transition)
            expected = record["teacher_command"]
            selected = next(
                (
                    index
                    for index, action in enumerate(transition["result"]["legal_actions"])
                    if _semantic_action(action) == _semantic_action(expected)
                ),
                None,
            )
            if selected is None:
                raise RuntimeError(f"tactical teacher action missing for seed {seed}: {expected}")
            observations[row] = decision.observation
            masks[row] = decision.action_mask
            actions[row] = selected
    return Demonstrations(observations, masks, actions, len(records), 0, 0)


def combine_demonstrations(
    full_game: Demonstrations,
    tactical: Demonstrations,
    *,
    tactical_repeats: int,
) -> Demonstrations:
    if tactical_repeats < 1:
        raise ValueError("tactical_repeats must be positive")
    return Demonstrations(
        observations=np.concatenate(
            (full_game.observations, np.tile(tactical.observations, (tactical_repeats, 1)))
        ),
        action_masks=np.concatenate(
            (full_game.action_masks, np.tile(tactical.action_masks, (tactical_repeats, 1)))
        ),
        actions=np.concatenate((full_game.actions, np.tile(tactical.actions, tactical_repeats))),
        episodes=full_game.episodes + tactical.episodes * tactical_repeats,
        authority_rejects=full_game.authority_rejects + tactical.authority_rejects,
        truncations=full_game.truncations + tactical.truncations,
    )


def corrective_clone(
    *,
    binary: Path,
    server_root: Path,
    base_checkpoint: Path,
    corpus_file: Path,
    output_dir: Path,
    seed: int,
    full_decisions: int,
    tactical_repeats: int,
    epochs: int,
    batch_size: int,
    device: str,
    learning_rate: float | None = None,
) -> dict[str, object]:
    """Produce a v2 candidate with balanced mechanics and first-roll tactical imitation."""
    model = MaskablePPO.load(base_checkpoint, device=device)
    full_game = (
        collect_demonstrations(
            binary=binary,
            server_root=server_root,
            seed=seed,
            decisions=full_decisions,
            observation_schema=OBSERVATION_SCHEMA_V2,
            teacher="mechanics-v2",
        )
        if full_decisions > 0
        else None
    )
    tactical = collect_tactical_demonstrations(
        binary=binary,
        server_root=server_root,
        corpus_file=corpus_file,
    )
    if full_game is None:
        combined = Demonstrations(
            observations=np.tile(tactical.observations, (tactical_repeats, 1)),
            action_masks=np.tile(tactical.action_masks, (tactical_repeats, 1)),
            actions=np.tile(tactical.actions, tactical_repeats),
            episodes=tactical.episodes * tactical_repeats,
            authority_rejects=tactical.authority_rejects,
            truncations=tactical.truncations,
        )
    else:
        combined = combine_demonstrations(
            full_game,
            tactical,
            tactical_repeats=tactical_repeats,
        )
    if learning_rate is not None:
        if learning_rate <= 0:
            raise ValueError("learning_rate must be positive")
        for group in model.policy.optimizer.param_groups:
            group["lr"] = learning_rate
    clone_summary = behavior_clone(
        model,
        combined,
        epochs=epochs,
        batch_size=batch_size,
        seed=seed,
    )
    output_dir.mkdir(parents=True, exist_ok=True)
    checkpoint = output_dir / "corrective-cloned.zip"
    model.save(checkpoint)
    result: dict[str, object] = {
        "schema": "dice-and-destiny-corrective-imitation-v2",
        "seed": seed,
        "base_checkpoint": str(base_checkpoint.resolve()),
        "base_checkpoint_sha256": hashlib.sha256(base_checkpoint.read_bytes()).hexdigest(),
        "corpus": str(corpus_file.resolve()),
        "corpus_sha256": hashlib.sha256(corpus_file.read_bytes()).hexdigest(),
        "full_game_decisions": len(full_game.actions) if full_game is not None else 0,
        "tactical_cases": len(tactical.actions),
        "tactical_repeats": tactical_repeats,
        "combined_decisions": len(combined.actions),
        "learning_rate": learning_rate,
        "checkpoint": str(checkpoint.resolve()),
        "checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "behavior_clone": clone_summary,
    }
    (output_dir / "summary.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


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


def _semantic_action(action: dict[str, object]) -> tuple[object, ...]:
    payload = action.get("payload") or {}
    assert isinstance(payload, dict)
    return (
        action.get("type"),
        tuple(payload.get("reroll_indices") or []),
        tuple(payload.get("kept_indices") or []),
        payload.get("ability_id", ""),
        tuple(payload.get("target_ids") or []),
    )
