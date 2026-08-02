from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

from sb3_contrib import MaskablePPO

from . import ACTION_SCHEMA_VERSION, ENVIRONMENT_SCHEMA_VERSION, OBSERVATION_SCHEMA_VERSION
from .training import parameter_hash

ACTOR_TENSORS = (
    "action_net.context.0.weight",
    "action_net.context.0.bias",
    "action_net.context.2.weight",
    "action_net.context.2.bias",
    "action_net.candidate.0.weight",
    "action_net.candidate.0.bias",
    "action_net.candidate.2.weight",
    "action_net.candidate.2.bias",
    "action_net.score.0.weight",
    "action_net.score.0.bias",
    "action_net.score.2.weight",
    "action_net.score.2.bias",
)


def export_phase3_policy(
    checkpoint: Path,
    output: Path,
    *,
    model_id: str,
    content_version: str,
    source_revision: str,
    training_engine_revision: str,
) -> dict[str, Any]:
    """Export the deterministic actor from an accepted Maskable PPO checkpoint.

    Phase 3 does not need the PPO optimizer or value network. The exported file
    contains the exact actor tensors used by ``model.predict(...,
    deterministic=True)`` plus compatibility metadata and hashes tying it back
    to the reviewed Phase 2 archive.
    """

    checkpoint = checkpoint.resolve()
    model = MaskablePPO.load(checkpoint, device="cpu")
    state = model.policy.state_dict()
    missing = [name for name in ACTOR_TENSORS if name not in state]
    if missing:
        raise RuntimeError(f"checkpoint is missing actor tensors: {missing}")

    tensors: dict[str, Any] = {}
    for name in ACTOR_TENSORS:
        value = state[name].detach().cpu().to(dtype=state[name].dtype).numpy()
        tensors[name] = {
            "shape": list(value.shape),
            "values": value.reshape(-1).tolist(),
        }

    payload = {
        "format": "dice-and-destiny-candidate-policy-v1",
        "model_id": model_id,
        "algorithm": "MaskablePPO deterministic masked argmax",
        "architecture": (
            "shared candidate scorer: context MLP [64,64], candidate MLP [64,64], "
            "score MLP [64,1]"
        ),
        "source_checkpoint": checkpoint.name,
        "source_checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "source_parameter_sha256": parameter_hash(model),
        "source_revision": source_revision,
        "training_engine_revision": training_engine_revision,
        "content_version": content_version,
        "environment_schema": ENVIRONMENT_SCHEMA_VERSION,
        "observation_schema": OBSERVATION_SCHEMA_VERSION,
        "action_schema": ACTION_SCHEMA_VERSION,
        "observation_size": 4224,
        "maximum_actions": 128,
        "base_features": 128,
        "action_features": 32,
        "tensors": tensors,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(payload, separators=(",", ":"), sort_keys=True) + "\n")
    return {
        key: payload[key]
        for key in (
            "format",
            "model_id",
            "source_checkpoint_sha256",
            "source_parameter_sha256",
            "source_revision",
            "training_engine_revision",
            "content_version",
            "environment_schema",
            "observation_schema",
            "action_schema",
        )
    }
