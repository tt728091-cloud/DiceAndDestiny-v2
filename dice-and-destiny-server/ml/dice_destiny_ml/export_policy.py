from __future__ import annotations

import hashlib
import json
from dataclasses import asdict
from pathlib import Path
from typing import Any

from sb3_contrib import MaskablePPO

from . import ACTION_SCHEMA_VERSION, ENVIRONMENT_SCHEMA_VERSION, OBSERVATION_SCHEMA_VERSION
from .manifest_v3 import (
    ACTION_SCHEMA_V3,
    ENVIRONMENT_SCHEMA_V3,
    OBSERVATION_SCHEMA_V3,
    ObservationManifestV3,
)
from .schema_v2 import (
    ACTION_FEATURES_V2,
    ACTION_SCHEMA_V2,
    BASE_FEATURES_V2,
    ENVIRONMENT_SCHEMA_V2,
    MAX_ACTIONS_V2,
    OBSERVATION_SCHEMA_V2,
    OBSERVATION_SIZE_V2,
)
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
            "shared candidate scorer: context MLP [64,64], candidate MLP [64,64], score MLP [64,1]"
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


def export_candidate_policy_v2(
    checkpoint: Path,
    output: Path,
    *,
    model_id: str,
    content_version: str,
    source_revision: str,
    training_engine_revision: str,
) -> dict[str, Any]:
    """Export a mechanics-aware candidate without changing accepted v1."""

    checkpoint = checkpoint.resolve()
    model = MaskablePPO.load(checkpoint, device="cpu")
    if tuple(model.observation_space.shape or ()) != (OBSERVATION_SIZE_V2,):
        raise RuntimeError("checkpoint is not an observation-v2 model")
    state = model.policy.state_dict()
    missing = [name for name in ACTOR_TENSORS if name not in state]
    if missing:
        raise RuntimeError(f"checkpoint is missing v2 actor tensors: {missing}")
    tensors: dict[str, Any] = {}
    for name in ACTOR_TENSORS:
        value = state[name].detach().cpu().numpy()
        tensors[name] = {"shape": list(value.shape), "values": value.reshape(-1).tolist()}
    payload = {
        "format": "dice-and-destiny-candidate-policy-v2",
        "model_id": model_id,
        "algorithm": "MaskablePPO deterministic masked argmax",
        "architecture": "v2 mechanics candidate scorer: context/candidate/score width 96",
        "source_checkpoint": checkpoint.name,
        "source_checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "source_parameter_sha256": parameter_hash(model),
        "source_revision": source_revision,
        "training_engine_revision": training_engine_revision,
        "content_version": content_version,
        "environment_schema": ENVIRONMENT_SCHEMA_V2,
        "observation_schema": OBSERVATION_SCHEMA_V2,
        "action_schema": ACTION_SCHEMA_V2,
        "observation_size": OBSERVATION_SIZE_V2,
        "maximum_actions": MAX_ACTIONS_V2,
        "base_features": BASE_FEATURES_V2,
        "action_features": ACTION_FEATURES_V2,
        "tensors": tensors,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(payload, separators=(",", ":"), sort_keys=True) + "\n")
    return {
        **{
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
        },
        "policy_export_sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
    }


def export_candidate_policy_v3(
    checkpoint: Path,
    output: Path,
    *,
    manifest_path: Path,
    model_id: str,
    source_revision: str,
    training_engine_revision: str,
) -> dict[str, Any]:
    """Export a frozen-manifest V3 sparse entity actor for Go inference."""

    checkpoint = checkpoint.resolve()
    manifest = ObservationManifestV3.load(manifest_path.resolve())
    model = MaskablePPO.load(checkpoint, device="cpu")
    if not hasattr(model.policy, "manifest"):
        raise RuntimeError("checkpoint is not an observation-v3 model")
    if model.policy.manifest.manifest_sha256 != manifest.manifest_sha256:
        raise RuntimeError("checkpoint and supplied frozen manifest differ")
    state = model.policy.state_dict()
    names = sorted(name for name in state if name.startswith("action_net."))
    if not names:
        raise RuntimeError("checkpoint has no V3 actor tensors")
    tensors: dict[str, Any] = {}
    for name in names:
        value = state[name].detach().cpu().numpy()
        tensors[name] = {"shape": list(value.shape), "values": value.reshape(-1).tolist()}
    payload = {
        "format": "dice-and-destiny-candidate-policy-v3",
        "model_id": model_id,
        "algorithm": "MaskablePPO deterministic masked argmax",
        "architecture": "v3 sparse pooled entity candidate scorer",
        "architecture_config": {
            "entity_width": int(model.policy.entity_width),
            "entity_depth": int(model.policy.entity_depth),
            "activation": str(model.policy.entity_activation),
        },
        "source_checkpoint": checkpoint.name,
        "source_checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "source_parameter_sha256": parameter_hash(model),
        "source_revision": source_revision,
        "training_engine_revision": training_engine_revision,
        "content_version": manifest.content_sha256,
        "environment_schema": ENVIRONMENT_SCHEMA_V3,
        "observation_schema": OBSERVATION_SCHEMA_V3,
        "action_schema": ACTION_SCHEMA_V3,
        "observation_size": manifest.layout.observation_size,
        "maximum_actions": manifest.maximum_legal_candidates,
        "action_features": manifest.layout.candidates.features,
        "observation_manifest_sha256": manifest.manifest_sha256,
        "observation_manifest": asdict(manifest),
        "tensors": tensors,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(payload, separators=(",", ":"), sort_keys=True) + "\n")
    return {
        **{
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
                "observation_manifest_sha256",
            )
        },
        "policy_export_sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
    }
