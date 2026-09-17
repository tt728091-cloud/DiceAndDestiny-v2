from __future__ import annotations

import os
import resource
import time
from collections import defaultdict
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from typing import Any

from .evaluation_statistics import percentile

MAXIMUM_COUNTERS = {"policy.maximum_valid_candidates"}


@dataclass
class ProfileCollector:
    """Low-overhead, opt-in timing and counter aggregation."""

    enabled: bool = False
    durations: dict[str, list[float]] = field(default_factory=lambda: defaultdict(list))
    counters: dict[str, float] = field(default_factory=lambda: defaultdict(float))

    @contextmanager
    def span(self, name: str) -> Iterator[None]:
        if not self.enabled:
            yield
            return
        started = time.perf_counter()
        try:
            yield
        finally:
            self.durations[name].append(time.perf_counter() - started)

    def observe(self, name: str, seconds: float) -> None:
        if self.enabled:
            self.durations[name].append(float(seconds))

    def increment(self, name: str, value: float = 1.0) -> None:
        if self.enabled:
            self.counters[name] += float(value)

    def clear(self) -> None:
        self.durations.clear()
        self.counters.clear()

    def summary(self) -> dict[str, Any]:
        return {
            "timings": {name: summarize_samples(samples) for name, samples in sorted(self.durations.items())},
            "counters": dict(sorted(self.counters.items())),
        }


def summarize_samples(samples: list[float]) -> dict[str, float | int]:
    milliseconds = [sample * 1000.0 for sample in samples]
    return {
        "count": len(samples),
        "total_seconds": sum(samples),
        "p50_ms": percentile(milliseconds, 0.50),
        "p95_ms": percentile(milliseconds, 0.95),
        "max_ms": max(milliseconds, default=0.0),
    }


def merge_profile_summaries(summaries: list[dict[str, Any]]) -> dict[str, Any]:
    """Merge process summaries without pretending their percentiles are raw samples."""

    timings: dict[str, dict[str, float]] = defaultdict(
        lambda: {"count": 0.0, "total_seconds": 0.0, "p50_ms_max": 0.0, "p95_ms_max": 0.0}
    )
    counters: dict[str, float] = defaultdict(float)
    processes: list[dict[str, Any]] = []
    for summary in summaries:
        for name, value in (summary.get("timings") or {}).items():
            target = timings[name]
            target["count"] += float(value.get("count", 0))
            target["total_seconds"] += float(value.get("total_seconds", 0.0))
            target["p50_ms_max"] = max(target["p50_ms_max"], float(value.get("p50_ms", 0.0)))
            target["p95_ms_max"] = max(target["p95_ms_max"], float(value.get("p95_ms", 0.0)))
        for name, value in (summary.get("counters") or {}).items():
            if name in MAXIMUM_COUNTERS:
                counters[name] = max(counters[name], float(value))
            else:
                counters[name] += float(value)
        process = summary.get("process")
        if process:
            existing = next((entry for entry in processes if entry.get("pid") == process.get("pid")), None)
            if existing is None:
                processes.append(process)
            elif process.get("rss_high_water_bytes", 0) > existing.get("rss_high_water_bytes", 0):
                processes[processes.index(existing)] = process
    return {
        "timings": {name: dict(value) for name, value in sorted(timings.items())},
        "counters": dict(sorted(counters.items())),
        "processes": processes,
    }


def process_snapshot(*, include_torch: bool = True, child_pid: int | None = None) -> dict[str, Any]:
    usage = resource.getrusage(resource.RUSAGE_SELF)
    snapshot: dict[str, Any] = {
        "pid": os.getpid(),
        "child_pid": child_pid,
        "rss_high_water_bytes": int(
            usage.ru_maxrss if os.uname().sysname == "Darwin" else usage.ru_maxrss * 1024
        ),
        "blas_environment": {
            name: os.environ.get(name, "")
            for name in (
                "OMP_NUM_THREADS",
                "OPENBLAS_NUM_THREADS",
                "MKL_NUM_THREADS",
                "VECLIB_MAXIMUM_THREADS",
            )
        },
    }
    if include_torch:
        import torch

        snapshot["torch_intra_threads"] = torch.get_num_threads()
        snapshot["torch_inter_threads"] = torch.get_num_interop_threads()
    return snapshot
