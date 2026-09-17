from __future__ import annotations

from collections import OrderedDict
from dataclasses import dataclass
from pathlib import Path
from random import Random

from .champions import ChampionRegistry
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
        category_weights: dict[str, float] | None = None,
    ) -> None:
        if historical_cache_size < 1:
            raise ValueError("historical cache size must be positive")
        self.specifications = tuple(specifications)
        self.device = device
        self.deterministic = deterministic
        self.historical_cache_size = historical_cache_size
        self.category_weights = category_weights or {
            "checkpoint": 0.50,
            "global": 0.40,
            "hall": 0.10,
        }
        if any(value < 0 for value in self.category_weights.values()) or not any(
            self.category_weights.values()
        ):
            raise ValueError("champion category weights must be nonnegative with a positive sum")
        self._static_cache: dict[str, Policy] = {}
        self._historical_cache: OrderedDict[str, Policy] = OrderedDict()
        self._historical_manifest: frozenset[str] = frozenset()
        self._champion_weights: dict[str, float] = {}
        self._explicit_categories: dict[str, str] = {}
        self._registry_cache: dict[Path, tuple[int, ChampionRegistry]] = {}

    def flat_manifest(self) -> tuple[str, ...]:
        """Publish one immutable pool snapshot for an episode's selection draw."""

        available: list[str] = []
        historical: set[str] = set()
        champion_weights: dict[str, float] = {}
        explicit_categories: dict[str, str] = {}
        for specification in self.specifications:
            if specification.startswith("pool:"):
                directory = Path(specification.removeprefix("pool:"))
                entries = tuple(f"model:{path}" for path in sorted(directory.glob("*.zip")))
                available.extend(entries)
                historical.update(entries)
            elif specification.startswith("registry:"):
                registry_path = Path(specification.removeprefix("registry:")).resolve()
                unverified = ChampionRegistry.load(registry_path, verify=False)
                cached = self._registry_cache.get(registry_path)
                if cached is not None and cached[0] == unverified.revision:
                    registry = cached[1]
                else:
                    registry = ChampionRegistry.load(registry_path, verify=True)
                    self._registry_cache[registry_path] = (registry.revision, registry)
                categories = {
                    "checkpoint": [registry.checkpoint_champion],
                    "global": [registry.global_champion],
                    "hall": [
                        champion
                        for champion in registry.hall
                        if champion.checkpoint_sha256
                        not in {
                            registry.checkpoint_champion.checkpoint_sha256,
                            registry.global_champion.checkpoint_sha256,
                        }
                    ],
                }
                for category, champions in categories.items():
                    unique = {champion.checkpoint_sha256: champion for champion in champions}
                    if not unique:
                        continue
                    per_entry = self.category_weights.get(category, 0.0) / len(unique)
                    for champion in unique.values():
                        entry = f"model:{champion.checkpoint_path}"
                        if entry not in available:
                            available.append(entry)
                        historical.add(entry)
                        champion_weights[entry] = champion_weights.get(entry, 0.0) + per_entry
                        prior = explicit_categories.get(entry, "")
                        names = prior.split("+") if prior else []
                        names.append(category)
                        explicit_categories[entry] = "+".join(dict.fromkeys(names))
            else:
                available.append(specification)
        self._historical_manifest = frozenset(historical)
        self._champion_weights = champion_weights
        self._explicit_categories = explicit_categories
        return tuple(available or ("random",))

    def select(self, rng: Random, *, mode: str) -> OpponentSelection:
        flat = self.flat_manifest()
        if mode == "legacy-flat":
            specification = rng.choice(flat)
            return OpponentSelection(specification, self._category(specification))
        if mode == "champion-registry":
            if not self._champion_weights:
                raise ValueError("champion-registry mode requires a registry: specification")
            entries = sorted(self._champion_weights)
            weights = [self._champion_weights[entry] for entry in entries]
            if not any(weights):
                raise ValueError("champion registry resolved to zero sampling weight")
            specification = rng.choices(entries, weights=weights, k=1)[0]
            return OpponentSelection(
                specification,
                self._explicit_categories.get(specification, "champion"),
            )
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
        available_categories = [category for category, entries in categories.items() if entries]
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
        if specification in self._explicit_categories:
            return self._explicit_categories[specification]
        if specification in self._historical_manifest:
            return "historical"
        if specification == "random":
            return "random"
        if specification in {"mechanics", "mechanics-v2", "heuristic", "heuristic-v1"}:
            return "mechanics"
        if specification.startswith("model:"):
            return "accepted"
        return "other"
