from __future__ import annotations

import pytest

from dice_destiny_ml.resources import resolve_resource_budget


def test_m3_max_profiles_resolve_to_bounded_single_thread_workers() -> None:
    assert resolve_resource_budget("max", logical_cpus=16).workers == 12
    assert resolve_resource_budget("balanced-80", logical_cpus=16).workers == 10
    assert resolve_resource_budget("light-50", logical_cpus=16).workers == 6
    assert resolve_resource_budget("max", logical_cpus=16).torch_threads == 1


def test_custom_profile_honors_explicit_worker_and_thread_budget() -> None:
    budget = resolve_resource_budget("custom", workers=7, torch_threads=2, logical_cpus=16)
    assert budget.workers == 7
    assert budget.torch_threads == 2
    assert budget.inference_concurrency == 7
    assert budget.io_concurrency == 2


def test_named_profiles_reject_hidden_overrides() -> None:
    with pytest.raises(ValueError, match="require --profile custom"):
        resolve_resource_budget("max", workers=4, logical_cpus=16)


def test_custom_profile_rejects_oversubscribed_thread_budget() -> None:
    with pytest.raises(ValueError, match="exceeds"):
        resolve_resource_budget("custom", workers=9, torch_threads=2, logical_cpus=16)
