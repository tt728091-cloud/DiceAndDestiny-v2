from __future__ import annotations

import os
import subprocess
import sys
import threading
import time
from dataclasses import asdict, dataclass

from .evaluation_statistics import percentile


@dataclass(frozen=True)
class ResourceBudget:
    profile: str
    workers: int
    torch_threads: int
    blas_threads: int
    inference_concurrency: int
    io_concurrency: int
    logical_cpus: int

    def as_dict(self) -> dict[str, int | str]:
        return asdict(self)


def resolve_resource_budget(
    profile: str,
    *,
    workers: int = 0,
    torch_threads: int = 0,
    logical_cpus: int | None = None,
) -> ResourceBudget:
    logical = logical_cpus or os.cpu_count() or 1
    # Leave the efficiency-core-sized tail available to the desktop at max.
    maximum_workers = max(1, logical - max(1, logical // 4))
    defaults = {
        "max": maximum_workers,
        "balanced-80": max(1, round(maximum_workers * 0.8)),
        "light-50": max(1, round(maximum_workers * 0.5)),
    }
    if profile == "custom":
        resolved_workers = workers or 1
        resolved_threads = torch_threads or 1
    elif profile in defaults:
        if workers or torch_threads:
            raise ValueError("explicit worker/thread overrides require --profile custom")
        resolved_workers = defaults[profile]
        resolved_threads = 1
    else:
        raise ValueError(f"unknown resource profile {profile!r}")
    if resolved_workers < 1 or resolved_threads < 1:
        raise ValueError("worker and thread budgets must be positive")
    if resolved_workers > logical or resolved_workers * resolved_threads > logical:
        raise ValueError(
            f"worker/thread budget {resolved_workers}x{resolved_threads} exceeds "
            f"the machine's {logical} logical CPUs"
        )
    return ResourceBudget(
        profile=profile,
        workers=resolved_workers,
        torch_threads=resolved_threads,
        blas_threads=resolved_threads,
        inference_concurrency=resolved_workers,
        io_concurrency=min(2, resolved_workers),
        logical_cpus=logical,
    )


class ForegroundResponsivenessProbe:
    """Measure scheduler wake-up delay without modifying rollout pacing."""

    def __init__(self, interval_seconds: float = 0.05) -> None:
        self.interval_seconds = interval_seconds
        self._stop = threading.Event()
        self._delays_ms: list[float] = []
        self._thread = threading.Thread(target=self._run, name="responsiveness-probe", daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> dict[str, float | int]:
        self._stop.set()
        self._thread.join(timeout=1)
        return {
            "samples": len(self._delays_ms),
            "p50_wakeup_delay_ms": percentile(self._delays_ms, 0.50),
            "p95_wakeup_delay_ms": percentile(self._delays_ms, 0.95),
            "max_wakeup_delay_ms": max(self._delays_ms, default=0.0),
        }

    def _run(self) -> None:
        deadline = time.perf_counter() + self.interval_seconds
        while not self._stop.wait(max(0.0, deadline - time.perf_counter())):
            observed = time.perf_counter()
            self._delays_ms.append(max(0.0, observed - deadline) * 1000)
            deadline += self.interval_seconds


def capture_host_state() -> dict[str, str]:
    commands = {
        "swap": ["sysctl", "-n", "vm.swapusage"],
        "memory_pressure": ["memory_pressure", "-Q"],
        "thermal": ["pmset", "-g", "therm"],
    }
    result: dict[str, str] = {"platform": sys.platform}
    for name, command in commands.items():
        try:
            completed = subprocess.run(
                command,
                check=False,
                capture_output=True,
                text=True,
                timeout=5,
            )
            value = (completed.stdout or completed.stderr).strip()
            result[name] = value if value else f"unavailable (exit {completed.returncode})"
        except (OSError, subprocess.TimeoutExpired) as error:
            result[name] = f"unavailable ({error})"
    return result
