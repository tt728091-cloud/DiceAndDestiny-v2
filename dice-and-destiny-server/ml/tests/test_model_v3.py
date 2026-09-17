from __future__ import annotations

from pathlib import Path

import gymnasium as gym
import numpy as np
import torch
from gymnasium import spaces
from sb3_contrib import MaskablePPO

from dice_destiny_ml.campaign import optimizer_hash
from dice_destiny_ml.export_policy import export_candidate_policy_v3
from dice_destiny_ml.manifest_v3 import build_observation_manifest_v3
from dice_destiny_ml.model import SparseEntityCandidateMaskablePolicyV3
from dice_destiny_ml.profiled_ppo import ProfiledMaskablePPO

SERVER_ROOT = Path(__file__).resolve().parents[2]
CONTENT_ROOT = SERVER_ROOT / "content" / "battle_v1"


class TinyV3Env(gym.Env[np.ndarray, int]):
    def __init__(self, manifest_path: Path) -> None:
        self.manifest = build_observation_manifest_v3(CONTENT_ROOT)
        self.observation_space = spaces.Box(
            -10.0,
            10.0,
            (self.manifest.layout.observation_size,),
            dtype=np.float32,
        )
        self.action_space = spaces.Discrete(self.manifest.maximum_legal_candidates)

    def _observation(self) -> np.ndarray:
        value = np.zeros(self.manifest.layout.observation_size, dtype=np.float32)
        context = self.manifest.layout.context
        value[context.offset] = 1
        candidates = self.manifest.layout.candidates
        value[candidates.offset] = 1
        value[candidates.offset + candidates.features] = 1
        return value

    def action_masks(self) -> np.ndarray:
        value = np.zeros(self.manifest.maximum_legal_candidates, dtype=bool)
        value[:2] = True
        return value

    def reset(self, *, seed=None, options=None):
        super().reset(seed=seed)
        return self._observation(), {}

    def step(self, action: int):
        assert self.action_masks()[action]
        return self._observation(), 0.0, True, False, {}


def _manifest_path(tmp_path: Path) -> Path:
    path = tmp_path / "manifest.json"
    build_observation_manifest_v3(CONTENT_ROOT).save(path)
    return path


def test_sparse_v3_actor_matches_dense_valid_logits_and_skips_padding(tmp_path: Path) -> None:
    manifest_path = _manifest_path(tmp_path)
    manifest = build_observation_manifest_v3(CONTENT_ROOT)
    policy = SparseEntityCandidateMaskablePolicyV3(
        spaces.Box(-10.0, 10.0, (manifest.layout.observation_size,), dtype=np.float32),
        spaces.Discrete(manifest.maximum_legal_candidates),
        lambda _: 3e-4,
        manifest_path=str(manifest_path),
        entity_width=32,
        entity_depth=1,
    )
    observations = torch.zeros((2, manifest.layout.observation_size), dtype=torch.float32)
    observations[:, manifest.layout.context.offset] = 1
    candidate_range = manifest.layout.candidates
    observations[:, candidate_range.offset] = 1
    observations[:, candidate_range.offset + candidate_range.features] = 1
    masks = torch.zeros((2, manifest.maximum_legal_candidates), dtype=torch.bool)
    masks[:, :2] = True
    with torch.no_grad():
        dense = policy.action_net(observations)
        sparse = policy.action_net.forward_sparse(observations, masks)
    torch.testing.assert_close(dense[:, :2], sparse[:, :2])
    assert torch.count_nonzero(sparse[:, 2:]) == 0


def test_v3_policy_width_depth_are_versioned_toggles(tmp_path: Path) -> None:
    manifest_path = _manifest_path(tmp_path)
    manifest = build_observation_manifest_v3(CONTENT_ROOT)
    arguments = (
        spaces.Box(-10.0, 10.0, (manifest.layout.observation_size,), dtype=np.float32),
        spaces.Discrete(manifest.maximum_legal_candidates),
        lambda _: 3e-4,
    )
    small = SparseEntityCandidateMaskablePolicyV3(
        *arguments,
        manifest_path=str(manifest_path),
        entity_width=16,
        entity_depth=1,
        activation="relu",
    )
    large = SparseEntityCandidateMaskablePolicyV3(
        *arguments,
        manifest_path=str(manifest_path),
        entity_width=64,
        entity_depth=2,
        activation="gelu",
    )
    assert sum(value.numel() for value in large.parameters()) > sum(
        value.numel() for value in small.parameters()
    )


def test_v3_checkpoint_round_trip_remains_version_separated(tmp_path: Path) -> None:
    manifest_path = _manifest_path(tmp_path)
    env = TinyV3Env(manifest_path)
    model = MaskablePPO(
        SparseEntityCandidateMaskablePolicyV3,
        env,
        n_steps=2,
        batch_size=2,
        n_epochs=1,
        policy_kwargs={
            "manifest_path": str(manifest_path),
            "entity_width": 16,
            "entity_depth": 1,
        },
        seed=7,
        device="cpu",
        verbose=0,
    )
    checkpoint = tmp_path / "v3.zip"
    model.save(checkpoint)
    loaded = MaskablePPO.load(checkpoint, device="cpu")
    assert isinstance(loaded.policy, SparseEntityCandidateMaskablePolicyV3)
    assert loaded.policy.manifest.manifest_sha256 == env.manifest.manifest_sha256
    observation, _ = env.reset()
    action, _ = loaded.predict(
        observation,
        deterministic=True,
        action_masks=env.action_masks(),
    )
    assert env.action_masks()[int(action)]


def test_profiled_v3_checkpoint_restores_optimizer_exactly(tmp_path: Path) -> None:
    manifest_path = _manifest_path(tmp_path)
    env = TinyV3Env(manifest_path)
    model = ProfiledMaskablePPO(
        SparseEntityCandidateMaskablePolicyV3,
        env,
        n_steps=2,
        batch_size=2,
        n_epochs=1,
        policy_kwargs={
            "manifest_path": str(manifest_path),
            "entity_width": 16,
            "entity_depth": 1,
        },
        seed=9,
        device="cpu",
        verbose=0,
    )
    model.learn(4)
    before = optimizer_hash(model)
    checkpoint = tmp_path / "optimizer.zip"
    model.save(checkpoint)
    restored = ProfiledMaskablePPO.load(checkpoint, env=env, device="cpu")
    assert optimizer_hash(restored) == before
    restored.policy.enable_instrumentation(True)
    assert restored.policy.profile.enabled is True


def test_v3_export_embeds_the_frozen_manifest_and_all_actor_tensors(tmp_path: Path) -> None:
    manifest_path = _manifest_path(tmp_path)
    env = TinyV3Env(manifest_path)
    model = MaskablePPO(
        SparseEntityCandidateMaskablePolicyV3,
        env,
        n_steps=2,
        batch_size=2,
        n_epochs=1,
        policy_kwargs={
            "manifest_path": str(manifest_path),
            "entity_width": 16,
            "entity_depth": 1,
        },
        seed=3,
        device="cpu",
        verbose=0,
    )
    checkpoint = tmp_path / "model.zip"
    model.save(checkpoint)
    output = tmp_path / "policy.json"
    summary = export_candidate_policy_v3(
        checkpoint,
        output,
        manifest_path=manifest_path,
        model_id="test-v3",
        source_revision="source",
        training_engine_revision="engine",
    )
    import json

    payload = json.loads(output.read_text())
    assert summary["observation_manifest_sha256"] == env.manifest.manifest_sha256
    assert payload["observation_manifest"]["manifest_sha256"] == env.manifest.manifest_sha256
    assert payload["tensors"]
    assert all(name.startswith("action_net.") for name in payload["tensors"])
