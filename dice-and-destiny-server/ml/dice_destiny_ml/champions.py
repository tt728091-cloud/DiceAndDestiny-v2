from __future__ import annotations

import hashlib
import json
import math
import os
import time
import uuid
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Literal

REGISTRY_SCHEMA = "dice-and-destiny-champion-registry-v1"
LEDGER_SCHEMA = "dice-and-destiny-experiment-ledger-v1"
SEED_BANK_SCHEMA = "dice-and-destiny-seat-swapped-seed-bank-v1"
SAFETY_FIELDS = (
    "truncations",
    "authority_rejects",
    "invalid_action_indices",
    "stale_action_submissions",
    "wrong_seat_submissions",
    "hidden_state_leaks",
    "state_leaks",
    "replay_mismatches",
)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def canonical_hash(value: Any) -> str:
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def atomic_json_write(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.parent / f".{path.name}.{uuid.uuid4().hex}.tmp"
    try:
        with temporary.open("w") as handle:
            json.dump(value, handle, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


@dataclass(frozen=True)
class Champion:
    champion_id: str
    checkpoint_path: str
    checkpoint_sha256: str
    observation_schema: str
    action_schema: str
    model_family: str
    learner_steps: int
    promoted_at: str
    parent_experiment_id: str = ""
    optimizer_state_sha256: str = ""

    @classmethod
    def from_checkpoint(
        cls,
        *,
        champion_id: str,
        checkpoint: Path,
        observation_schema: str,
        action_schema: str,
        model_family: str,
        learner_steps: int,
        promoted_at: str,
        parent_experiment_id: str = "",
        optimizer_state_sha256: str = "",
    ) -> Champion:
        resolved = checkpoint.resolve()
        if not resolved.is_file():
            raise FileNotFoundError(resolved)
        return cls(
            champion_id=champion_id,
            checkpoint_path=str(resolved),
            checkpoint_sha256=sha256_file(resolved),
            observation_schema=observation_schema,
            action_schema=action_schema,
            model_family=model_family,
            learner_steps=learner_steps,
            promoted_at=promoted_at,
            parent_experiment_id=parent_experiment_id,
            optimizer_state_sha256=optimizer_state_sha256,
        )

    def verify(self) -> None:
        path = Path(self.checkpoint_path)
        if not path.is_file():
            raise FileNotFoundError(path)
        actual = sha256_file(path)
        if actual != self.checkpoint_sha256:
            raise RuntimeError(
                f"champion {self.champion_id} hash mismatch: {actual}, expected {self.checkpoint_sha256}"
            )


@dataclass
class ChampionRegistry:
    global_champion: Champion
    checkpoint_champion: Champion
    hall: list[Champion] = field(default_factory=list)
    schema: str = REGISTRY_SCHEMA
    revision: int = 1

    @classmethod
    def load(cls, path: Path, *, verify: bool = True) -> ChampionRegistry:
        raw = json.loads(path.read_text())
        if raw.get("schema") != REGISTRY_SCHEMA:
            raise RuntimeError(f"unsupported champion registry {raw.get('schema')!r}")
        registry = cls(
            global_champion=Champion(**raw["global_champion"]),
            checkpoint_champion=Champion(**raw["checkpoint_champion"]),
            hall=[Champion(**value) for value in raw.get("hall", [])],
            revision=int(raw.get("revision", 1)),
        )
        if verify:
            registry.verify()
        return registry

    def save(self, path: Path) -> None:
        self.verify()
        atomic_json_write(path, asdict(self))

    def verify(self) -> None:
        if self.schema != REGISTRY_SCHEMA:
            raise RuntimeError(f"unsupported champion registry {self.schema!r}")
        seen: dict[str, str] = {}
        for champion in [self.global_champion, self.checkpoint_champion, *self.hall]:
            champion.verify()
            prior = seen.get(champion.checkpoint_sha256)
            if prior and prior != champion.checkpoint_path:
                raise RuntimeError(
                    "identical champion hash registered at different paths: "
                    f"{prior}, {champion.checkpoint_path}"
                )
            seen[champion.checkpoint_sha256] = champion.checkpoint_path

    def opponent_entries(self) -> tuple[Champion, ...]:
        """Return immutable, hash-deduplicated champions; file count is irrelevant."""

        result: list[Champion] = []
        seen: set[str] = set()
        for champion in [self.checkpoint_champion, self.global_champion, *self.hall]:
            if champion.checkpoint_sha256 not in seen:
                result.append(champion)
                seen.add(champion.checkpoint_sha256)
        return tuple(result)

    def promote_checkpoint(self, champion: Champion) -> None:
        if self.checkpoint_champion.checkpoint_sha256 != champion.checkpoint_sha256:
            retired = self.checkpoint_champion
            self.checkpoint_champion = champion
            if retired.checkpoint_sha256 != self.global_champion.checkpoint_sha256:
                self._retain(retired)
            self.revision += 1

    def promote_global(self, champion: Champion) -> None:
        if self.global_champion.checkpoint_sha256 != champion.checkpoint_sha256:
            retired = self.global_champion
            self.global_champion = champion
            self.checkpoint_champion = champion
            self._retain(retired)
            self.revision += 1

    def _retain(self, champion: Champion) -> None:
        hashes = {entry.checkpoint_sha256 for entry in self.hall}
        if (
            champion.checkpoint_sha256 not in hashes
            and champion.checkpoint_sha256 != self.checkpoint_champion.checkpoint_sha256
        ):
            self.hall.append(champion)


@dataclass(frozen=True)
class GateResult:
    wins: int
    draws: int
    losses: int
    adjusted_score: float
    interval_95: tuple[float, float]
    decision: Literal["pass", "fail"]
    threshold_rule: str
    safety: dict[str, int]

    @property
    def games(self) -> int:
        return self.wins + self.draws + self.losses


def adjusted_score(wins: int, draws: int, losses: int) -> float:
    games = wins + draws + losses
    if games <= 0:
        raise ValueError("a gate result requires at least one game")
    return (wins + 0.5 * draws) / games


def adjusted_wilson_interval(
    wins: int, draws: int, losses: int, z: float = 1.959963984540054
) -> tuple[float, float]:
    """Wilson interval over draw-adjusted fractional Bernoulli observations."""

    games = wins + draws + losses
    if games <= 0:
        return 0.0, 1.0
    proportion = adjusted_score(wins, draws, losses)
    denominator = 1.0 + z * z / games
    center = (proportion + z * z / (2.0 * games)) / denominator
    half_width = (
        z * math.sqrt(proportion * (1.0 - proportion) / games + z * z / (4.0 * games * games)) / denominator
    )
    return max(0.0, center - half_width), min(1.0, center + half_width)


def global_progression_block(
    *,
    wins: int,
    draws: int,
    losses: int,
    baseline_score: float | None,
    safety: dict[str, int] | None = None,
) -> GateResult:
    games = wins + draws + losses
    if games != 1_000:
        raise ValueError(f"global progression blocks require exactly 1,000 games, got {games}")
    if baseline_score is not None and not 0.0 <= baseline_score <= 1.0:
        raise ValueError("global progression baseline must be between zero and one")
    counts = normalized_safety(safety)
    score = adjusted_score(wins, draws, losses)
    if any(counts.values()):
        decision: Literal["pass", "fail"] = "fail"
        rule = "safety counts must all be zero"
    elif baseline_score is None:
        decision, rule = "pass", "first safe checkpoint establishes the model-family baseline"
    elif score > baseline_score:
        decision, rule = "pass", "adjusted score strictly improves the accepted family baseline"
    else:
        decision, rule = "fail", "adjusted score does not strictly improve the accepted family baseline"
    return GateResult(
        wins,
        draws,
        losses,
        score,
        adjusted_wilson_interval(wins, draws, losses),
        decision,
        rule,
        counts,
    )


def global_block(*, wins: int, draws: int, losses: int, safety: dict[str, int] | None = None) -> GateResult:
    games = wins + draws + losses
    if games != 1_000:
        raise ValueError(f"global comparison blocks require exactly 1,000 games, got {games}")
    counts = normalized_safety(safety)
    score = adjusted_score(wins, draws, losses)
    decision: Literal["pass", "fail"] = "pass" if score > 0.50 and not any(counts.values()) else "fail"
    return GateResult(
        wins,
        draws,
        losses,
        score,
        adjusted_wilson_interval(wins, draws, losses),
        decision,
        "adjusted score > 50% and zero safety counts",
        counts,
    )


def global_confirmation_block(
    *, wins: int, draws: int, losses: int, safety: dict[str, int] | None = None
) -> GateResult:
    games = wins + draws + losses
    if games != 3_000:
        raise ValueError(f"global confirmation blocks require exactly 3,000 games, got {games}")
    counts = normalized_safety(safety)
    score = adjusted_score(wins, draws, losses)
    decision: Literal["pass", "fail"] = "pass" if score > 0.50 and not any(counts.values()) else "fail"
    return GateResult(
        wins,
        draws,
        losses,
        score,
        adjusted_wilson_interval(wins, draws, losses),
        decision,
        "fresh 3,000-game adjusted score > 50% and zero safety counts",
        counts,
    )


def normalized_safety(value: dict[str, int] | None) -> dict[str, int]:
    source = value or {}
    result = {field: int(source.get(field, 0)) for field in SAFETY_FIELDS}
    unknown = set(source) - set(SAFETY_FIELDS)
    if unknown:
        raise ValueError(f"unknown safety fields: {sorted(unknown)}")
    if any(count < 0 for count in result.values()):
        raise ValueError("safety counts cannot be negative")
    return result


def write_seed_bank(path: Path, *, bank_id: str, seeds: list[int], purpose: str) -> dict[str, Any]:
    if len(seeds) != len(set(seeds)):
        raise ValueError("seed bank contains duplicates")
    value = {
        "schema": SEED_BANK_SCHEMA,
        "bank_id": bank_id,
        "purpose": purpose,
        "seeds": [int(seed) for seed in seeds],
        "seat_swapped_games": 2 * len(seeds),
    }
    value["sha256"] = canonical_hash(value)
    atomic_json_write(path, value)
    return value


def read_seed_bank(path: Path, *, expected_games: int) -> list[int]:
    value = json.loads(path.read_text())
    if value.get("schema") != SEED_BANK_SCHEMA:
        raise RuntimeError(f"unsupported seed bank {value.get('schema')!r}: {path}")
    declared = value.get("sha256", "")
    actual = canonical_hash({key: item for key, item in value.items() if key != "sha256"})
    if declared != actual:
        raise RuntimeError(f"seed bank hash mismatch: {path}")
    seeds = [int(seed) for seed in value.get("seeds") or []]
    if len(seeds) != len(set(seeds)) or 2 * len(seeds) != expected_games:
        raise RuntimeError(f"seed bank {path} does not provide {expected_games} distinct seat-swapped games")
    return seeds


class ExperimentLedger:
    def __init__(self, path: Path, summary_path: Path | None = None) -> None:
        self.path = path
        self.summary_path = summary_path or path.with_suffix(".md")
        self.path.parent.mkdir(parents=True, exist_ok=True)

    def append(self, event: dict[str, Any]) -> dict[str, Any]:
        previous = self._last_hash()
        record = {
            "schema": LEDGER_SCHEMA,
            "sequence": self._count() + 1,
            "recorded_at_unix_ns": time.time_ns(),
            "previous_record_sha256": previous,
            **event,
        }
        required = {
            "event",
            "experiment_id",
            "parent_experiment_id",
            "checkpoint_champion_sha256",
            "global_champion_sha256",
            "source_revision",
            "content_sha256",
            "observation_schema_sha256",
            "action_schema_sha256",
            "architecture_sha256",
            "reward_sha256",
            "teacher_setup_sha256",
            "opponent_curriculum_sha256",
            "ppo_recipe_sha256",
            "config",
            "config_diff",
            "hypothesis",
        }
        missing = sorted(required - set(record))
        if missing:
            raise ValueError(f"ledger record missing required fields: {missing}")
        hashable = dict(record)
        record["record_sha256"] = canonical_hash(hashable)
        encoded = json.dumps(record, sort_keys=True) + "\n"
        with self.path.open("a") as handle:
            handle.write(encoded)
            handle.flush()
            os.fsync(handle.fileno())
        self.render_summary()
        return record

    def records(self) -> list[dict[str, Any]]:
        if not self.path.exists():
            return []
        result = [json.loads(line) for line in self.path.read_text().splitlines() if line]
        previous = ""
        for index, record in enumerate(result, start=1):
            if record.get("sequence") != index or record.get("previous_record_sha256") != previous:
                raise RuntimeError(f"ledger chain broken at record {index}")
            expected = canonical_hash({key: value for key, value in record.items() if key != "record_sha256"})
            if record.get("record_sha256") != expected:
                raise RuntimeError(f"ledger hash mismatch at record {index}")
            previous = expected
        return result

    def comparable(self, record: dict[str, Any]) -> list[dict[str, Any]]:
        keys = (
            "content_sha256",
            "observation_schema_sha256",
            "action_schema_sha256",
            "architecture_sha256",
            "reward_sha256",
            "teacher_setup_sha256",
            "opponent_curriculum_sha256",
            "ppo_recipe_sha256",
        )
        return [
            candidate
            for candidate in self.records()
            if all(candidate.get(key) == record.get(key) for key in keys)
        ]

    def render_summary(self) -> None:
        rows = []
        for record in self.records():
            result = record.get("result") or {}
            rows.append(
                "| {sequence} | {experiment} | {event} | {parent} | {steps} | {score} | {decision} |".format(
                    sequence=record["sequence"],
                    experiment=record["experiment_id"],
                    event=record["event"],
                    parent=record.get("parent_experiment_id", ""),
                    steps=result.get("learner_steps", ""),
                    score=(
                        f"{100 * float(result['adjusted_score']):.2f}%" if "adjusted_score" in result else ""
                    ),
                    decision=result.get("decision", ""),
                )
            )
        body = [
            "# Champion campaign history",
            "",
            "Append-only ledger rendering. The JSONL hash chain is authoritative.",
            "",
            "| # | Experiment | Event | Parent | Steps | Adjusted score | Decision |",
            "| ---: | --- | --- | --- | ---: | ---: | --- |",
            *rows,
            "",
        ]
        atomic_text_write(self.summary_path, "\n".join(body))

    def _count(self) -> int:
        return len(self.records())

    def _last_hash(self) -> str:
        records = self.records()
        return records[-1]["record_sha256"] if records else ""


def atomic_text_write(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.parent / f".{path.name}.{uuid.uuid4().hex}.tmp"
    try:
        with temporary.open("w") as handle:
            handle.write(value)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


@dataclass
class CampaignTimer:
    budget_seconds: float = 10_800.0
    started_monotonic: float | None = None

    def start(self, now: float | None = None) -> None:
        if self.started_monotonic is not None:
            raise RuntimeError("campaign timer is already started")
        self.started_monotonic = time.monotonic() if now is None else float(now)

    @property
    def deadline_monotonic(self) -> float:
        if self.started_monotonic is None:
            raise RuntimeError("campaign timer has not started")
        return self.started_monotonic + self.budget_seconds

    def may_start_interval(self, now: float | None = None) -> bool:
        current = time.monotonic() if now is None else float(now)
        return self.started_monotonic is not None and current < self.deadline_monotonic

    def deadline_crossed(self, now: float | None = None) -> bool:
        current = time.monotonic() if now is None else float(now)
        return self.started_monotonic is not None and current >= self.deadline_monotonic

    def snapshot(self, now: float | None = None) -> dict[str, float | bool | None]:
        current = time.monotonic() if now is None else float(now)
        if self.started_monotonic is None:
            return {
                "started_monotonic": None,
                "deadline_monotonic": None,
                "elapsed": 0.0,
                "remaining": self.budget_seconds,
                "deadline_crossed": False,
            }
        elapsed = current - self.started_monotonic
        return {
            "started_monotonic": self.started_monotonic,
            "deadline_monotonic": self.deadline_monotonic,
            "elapsed": elapsed,
            "remaining": max(self.budget_seconds - elapsed, 0.0),
            "deadline_crossed": current >= self.deadline_monotonic,
        }
