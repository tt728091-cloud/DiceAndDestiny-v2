from __future__ import annotations

import copy

import numpy as np
import torch
from gymnasium import spaces
from test_schema_v3 import CONTENT_ROOT, _transition

from dice_destiny_ml.manifest_v5 import build_observation_manifest_v5
from dice_destiny_ml.model import SparseRelationalCandidateMaskablePolicyV5
from dice_destiny_ml.schema_v5 import SchemaEncoderV5


def _manifest_path(tmp_path):
    path = tmp_path / "manifest-v5.json"
    build_observation_manifest_v5(CONTENT_ROOT).save(path)
    return path


def _policy(
    tmp_path,
    *,
    width: int = 32,
    decision_width: int = 96,
    depth: int = 1,
    decision_depth: int = 1,
):
    manifest_path = _manifest_path(tmp_path)
    manifest = build_observation_manifest_v5(CONTENT_ROOT)
    return SparseRelationalCandidateMaskablePolicyV5(
        spaces.Box(-10.0, 10.0, (manifest.layout.observation_size,), dtype=np.float32),
        spaces.Discrete(manifest.maximum_legal_candidates),
        lambda _: 3e-4,
        manifest_path=str(manifest_path),
        entity_width=width,
        decision_width=decision_width,
        entity_depth=depth,
        decision_depth=decision_depth,
    )


def test_v5_candidate_detail_mutation_does_not_contaminate_other_logits(tmp_path) -> None:
    manifest = build_observation_manifest_v5(CONTENT_ROOT)
    encoder = SchemaEncoderV5(manifest)
    transition = _transition()
    transition["result"]["legal_actions"] = [
        {
            "type": "commit_interaction",
            "payload": {
                "card_ids": ["own-card-2"],
                "planning_adjustments": [{"die_index": 0, "face": 6}],
            },
        },
        {
            "type": "commit_interaction",
            "payload": {
                "card_ids": ["own-card-2"],
                "planning_adjustments": [{"die_index": 1, "face": 5}],
            },
        },
    ]
    original = encoder.encode(transition)
    mutated_transition = copy.deepcopy(transition)
    mutated_transition["result"]["legal_actions"][1]["payload"]["planning_adjustments"][0]["face"] = 4
    mutated = encoder.encode(mutated_transition)
    policy = _policy(tmp_path)
    observations = torch.as_tensor(np.stack((original.observation, mutated.observation)))
    masks = torch.as_tensor(np.stack((original.action_mask, mutated.action_mask)))
    with torch.no_grad():
        logits = policy.action_net.forward_sparse(observations, masks)
    torch.testing.assert_close(logits[0, 0], logits[1, 0], rtol=0, atol=1e-7)
    assert not torch.isclose(logits[0, 1], logits[1, 1], rtol=0, atol=1e-7)


def test_v5_details_have_learned_parameters_and_receive_gradients(tmp_path) -> None:
    policy = _policy(tmp_path)
    manifest = policy.manifest
    observation = torch.zeros((1, manifest.layout.observation_size), dtype=torch.float32)
    candidate = manifest.layout.candidates
    observation[0, candidate.offset] = 1
    details = manifest.layout.details
    row = observation[0, details.offset : details.offset + details.features]
    row[0] = 1
    row[7] = 1  # candidate scope
    row[11] = 1
    row[82] = 1 / manifest.maximum_legal_candidates
    row[84] = 1
    mask = torch.zeros((1, manifest.maximum_legal_candidates), dtype=torch.bool)
    mask[0, 0] = True
    policy.action_net.forward_sparse(observation, mask)[0, 0].backward()
    detail_gradients = [
        parameter.grad
        for name, parameter in policy.action_net.named_parameters()
        if "detail" in name and parameter.requires_grad
    ]
    assert any(value is not None and torch.count_nonzero(value) for value in detail_gradients)


def test_v5_campaign_size_is_comparable_to_cp193(tmp_path) -> None:
    policy = _policy(tmp_path, width=96, decision_width=432, depth=1, decision_depth=2)
    parameters = sum(value.numel() for value in policy.parameters())
    assert 2_500_000 <= parameters <= 5_000_000
