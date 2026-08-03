from __future__ import annotations

from collections import OrderedDict
from dataclasses import dataclass
from pathlib import Path
from random import Random

from .policies import Policy, build_policy


@dataclass(frozen=True)
class OpponentAcquisition:
    policy: Policy
    cache_status: str


@dataclass(frozen=True)
class OpponentSelection:
    specification: str
    category: str


class OpponentManager:
    """Legacy-equivalent flat selection with long-lived, bounded policy caches."""

    def __init__(
        self,
        specifications: list[str],
        *,
        device: str,
        deterministic: bool,
        historical_cache_size: int = 8,
    ) -> None:
        if historical_cache_size < 1:
            raise ValueError("historical cache size must be positive")
        self.specifications = tuple(specifications)
        self.device = device
        self.deterministic = deterministic
        self.historical_cache_size = historical_cache_size
        self._static_cache: dict[str, Policy] = {}
        self._historical_cache: OrderedDict[str, Policy] = OrderedDict()
        self._historical_manifest: frozenset[str] = frozenset()

    def flat_manifest(self) -> tuple[str, ...]:
        """Publish one immutable pool snapshot for an episode's selection draw."""

        available: list[str] = []
        historical: set[str] = set()
        for specification in self.specifications:
            if specification.startswith("pool:"):
                directory = Path(specification.removeprefix("pool:"))
                entries = tuple(
                    f"model:{path}" for path in sorted(directory.glob("*.zip"))
                )
                available.extend(entries)
                historical.update(entries)
            else:
                available.append(specification)
        self._historical_manifest = frozenset(historical)
        return tuple(available or ("random",))

    def select(self, rng: Random, *, mode: str) -> OpponentSelection:
        flat = self.flat_manifest()
        if mode == "legacy-flat":
            specification = rng.choice(flat)
            return OpponentSelection(specification, self._category(specification))
        if mode != "category-balanced":
            raise ValueError(f"unknown opponent selection mode {mode!r}")
        categories: dict[str, list[str]] = {
            "random": [],
            "mechanics": [],
            "accepted": [],
            "historical": [],
            "other": [],
        }
        for specification in flat:
            categories[self._category(specification)].append(specification)
        available_categories = [
            category for category, entries in categories.items() if entries
        ]
        category = rng.choice(available_categories)
        return OpponentSelection(rng.choice(categories[category]), category)

    def acquire(self, specification: str) -> OpponentAcquisition:
        if specification in self._historical_manifest:
            policy = self._historical_cache.pop(specification, None)
            if policy is not None:
                self._historical_cache[specification] = policy
                return OpponentAcquisition(policy, "historical_hit")
            policy = self._build(specification)
            self._historical_cache[specification] = policy
            while len(self._historical_cache) > self.historical_cache_size:
                self._historical_cache.popitem(last=False)
            return OpponentAcquisition(policy, "historical_miss")

        policy = self._static_cache.get(specification)
        if policy is not None:
            return OpponentAcquisition(policy, "static_hit")
        policy = self._build(specification)
        self._static_cache[specification] = policy
        return OpponentAcquisition(policy, "static_miss")

    def _build(self, specification: str) -> Policy:
        return build_policy(
            specification,
            device=self.device,
            deterministic=self.deterministic,
        )

    def _category(self, specification: str) -> str:
        if specification in self._historical_manifest:
            return "historical"
        if specification == "random":
            return "random"
        if specification in {"mechanics", "mechanics-v2", "heuristic", "heuristic-v1"}:
            return "mechanics"
        if specification.startswith("model:"):
            return "accepted"
        return "other"
