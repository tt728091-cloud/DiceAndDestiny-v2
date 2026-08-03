from __future__ import annotations

import numpy as np
import torch
from gymnasium import spaces

from dice_destiny_ml.model import (
    CandidateMaskablePolicyV2,
    SparseBaseCriticCandidateMaskablePolicyV2,
    SparseCandidateMaskablePolicyV2,
)
from dice_destiny_ml.profiling import ProfileCollector, merge_profile_summaries
from dice_destiny_ml.schema_v2 import MAX_ACTIONS_V2, OBSERVATION_SIZE_V2


def test_profile_collector_reports_nested_timings_and_counters() -> None:
    collector = ProfileCollector(True)
    with collector.span("outer"):
        collector.increment("bytes", 12)
    summary = collector.summary()
    assert summary["timings"]["outer"]["count"] == 1
    assert summary["timings"]["outer"]["total_seconds"] >= 0
    assert summary["counters"]["bytes"] == 12

    merged = merge_profile_summaries([summary, summary])
    assert merged["timings"]["outer"]["count"] == 2
    assert merged["counters"]["bytes"] == 24

    maximum = {"timings": {}, "counters": {"policy.maximum_valid_candidates": 17}}
    assert merge_profile_summaries([maximum, maximum])["counters"][
        "policy.maximum_valid_candidates"
    ] == 17


def test_policy_instrumentation_preserves_dense_outputs() -> None:
    torch.manual_seed(22)
    policy = CandidateMaskablePolicyV2(
        spaces.Box(-10.0, 10.0, (OBSERVATION_SIZE_V2,), dtype=np.float32),
        spaces.Discrete(MAX_ACTIONS_V2),
        lambda _: 3e-4,
    )
    observations = torch.randn(2, OBSERVATION_SIZE_V2)
    masks = torch.zeros(2, MAX_ACTIONS_V2, dtype=torch.bool)
    masks[0, :3] = True
    masks[1, :9] = True
    actions = torch.tensor([1, 4])

    expected = policy.evaluate_actions(observations, actions, masks)
    policy.enable_instrumentation(True)
    actual = policy.evaluate_actions(observations, actions, masks)

    for expected_value, actual_value in zip(expected, actual, strict=True):
        assert expected_value is not None
        assert actual_value is not None
        torch.testing.assert_close(actual_value, expected_value, rtol=0, atol=0)
    assert policy.profile.summary()["timings"]["policy.update.mask_distribution"]["count"] == 1


def test_sparse_policy_matches_dense_valid_distribution_and_gradients() -> None:
    torch.manual_seed(22)
    arguments = (
        spaces.Box(-10.0, 10.0, (OBSERVATION_SIZE_V2,), dtype=np.float32),
        spaces.Discrete(MAX_ACTIONS_V2),
        lambda _: 3e-4,
    )
    dense = CandidateMaskablePolicyV2(*arguments)
    sparse = SparseCandidateMaskablePolicyV2(*arguments)
    sparse.load_state_dict(dense.state_dict())
    observations = torch.randn(4, OBSERVATION_SIZE_V2)
    masks = torch.zeros(4, MAX_ACTIONS_V2, dtype=torch.bool)
    masks[0, :3] = True
    masks[1, [0, 7, 31, 90]] = True
    masks[2, :17] = True
    masks[3, 127] = True
    actions = torch.tensor([1, 31, 8, 127])

    dense_distribution = dense.get_distribution(observations, masks)
    sparse_distribution = sparse.get_distribution(observations, masks)
    dense_logits = dense.action_net(observations)
    sparse_logits = sparse.action_net.forward_sparse(observations, masks)
    torch.testing.assert_close(
        sparse_logits[masks], dense_logits[masks], rtol=1e-6, atol=1e-7
    )
    torch.testing.assert_close(
        sparse_distribution.distribution.probs,
        dense_distribution.distribution.probs,
        rtol=1e-6,
        atol=1e-7,
    )
    dense_values, dense_log_prob, dense_entropy = dense.evaluate_actions(
        observations, actions, masks
    )
    sparse_values, sparse_log_prob, sparse_entropy = sparse.evaluate_actions(
        observations, actions, masks
    )
    torch.testing.assert_close(sparse_values, dense_values, rtol=0, atol=0)
    torch.testing.assert_close(sparse_log_prob, dense_log_prob, rtol=1e-6, atol=1e-7)
    assert dense_entropy is not None and sparse_entropy is not None
    torch.testing.assert_close(sparse_entropy, dense_entropy, rtol=1e-6, atol=1e-7)
    dense_actions, _, _ = dense(observations, deterministic=True, action_masks=masks)
    sparse_actions, _, _ = sparse(observations, deterministic=True, action_masks=masks)
    torch.testing.assert_close(sparse_actions, dense_actions, rtol=0, atol=0)
    torch.manual_seed(101)
    dense_samples = torch.stack(
        [dense_distribution.get_actions(deterministic=False) for _ in range(16)]
    )
    torch.manual_seed(101)
    sparse_samples = torch.stack(
        [sparse_distribution.get_actions(deterministic=False) for _ in range(16)]
    )
    torch.testing.assert_close(sparse_samples, dense_samples, rtol=0, atol=0)

    dense_loss = dense_values.square().mean() - dense_log_prob.mean() - dense_entropy.mean()
    sparse_loss = sparse_values.square().mean() - sparse_log_prob.mean() - sparse_entropy.mean()
    dense_loss.backward()
    sparse_loss.backward()
    for (dense_name, dense_parameter), (sparse_name, sparse_parameter) in zip(
        dense.named_parameters(), sparse.named_parameters(), strict=True
    ):
        assert dense_name == sparse_name
        assert dense_parameter.grad is not None and sparse_parameter.grad is not None
        torch.testing.assert_close(
            sparse_parameter.grad,
            dense_parameter.grad,
            rtol=2e-5,
            atol=2e-6,
        )

    full_masks = torch.ones(4, MAX_ACTIONS_V2, dtype=torch.bool)
    full_sparse = sparse.action_net.forward_sparse(observations, full_masks)
    torch.testing.assert_close(full_sparse, dense_logits, rtol=1e-6, atol=1e-7)


def test_sparse_policy_rejects_empty_mask() -> None:
    policy = SparseCandidateMaskablePolicyV2(
        spaces.Box(-10.0, 10.0, (OBSERVATION_SIZE_V2,), dtype=np.float32),
        spaces.Discrete(MAX_ACTIONS_V2),
        lambda _: 3e-4,
    )
    observations = torch.zeros(1, OBSERVATION_SIZE_V2)
    masks = torch.zeros(1, MAX_ACTIONS_V2, dtype=torch.bool)
    with np.testing.assert_raises_regex(ValueError, "valid candidate"):
        policy.get_distribution(observations, masks)


def test_base_critic_is_versioned_and_materially_smaller() -> None:
    arguments = (
        spaces.Box(-10.0, 10.0, (OBSERVATION_SIZE_V2,), dtype=np.float32),
        spaces.Discrete(MAX_ACTIONS_V2),
        lambda _: 3e-4,
    )
    dense = SparseCandidateMaskablePolicyV2(*arguments)
    compact = SparseBaseCriticCandidateMaskablePolicyV2(*arguments)
    dense_parameters = sum(parameter.numel() for parameter in dense.parameters())
    compact_parameters = sum(parameter.numel() for parameter in compact.parameters())
    assert compact_parameters < dense_parameters * 0.25
