from __future__ import annotations

import os
import subprocess
import sys
import threading
import time
from dataclasses import asdict, dataclass

from .evaluation_statistics import percentile

BLAS_THREAD_ENVIRONMENT = (
    "OMP_NUM_THREADS",
    "OPENBLAS_NUM_THREADS",
    "MKL_NUM_THREADS",
    "VECLIB_MAXIMUM_THREADS",
)


@dataclass(frozen=True)
class ResourceBudget:
    profile: str
    workers: int
    # Kept as the evaluation-worker setting for CLI compatibility.
    torch_threads: int
    blas_threads: int
    learner_torch_threads: int
    learner_torch_interop_threads: int
    learner_blas_threads: int
    worker_torch_threads: int
    worker_torch_interop_threads: int
    worker_blas_threads: int
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
    learner_torch_threads: int = 0,
    learner_torch_interop_threads: int = 0,
    learner_blas_threads: int = 0,
    worker_torch_threads: int = 0,
    worker_torch_interop_threads: int = 0,
    worker_blas_threads: int = 0,
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
    learner_defaults = {
        "max": max(1, logical // 4),
        "balanced-80": max(1, logical // 8),
        "light-50": 1,
    }
    explicit_training_threads = any(
        (
            learner_torch_threads,
            learner_torch_interop_threads,
            learner_blas_threads,
            worker_torch_threads,
            worker_torch_interop_threads,
            worker_blas_threads,
        )
    )
    if profile == "custom":
        resolved_workers = workers or 1
        resolved_threads = torch_threads or 1
        resolved_learner_threads = learner_torch_threads or resolved_threads
        resolved_learner_interop = learner_torch_interop_threads or 1
        resolved_learner_blas = learner_blas_threads or resolved_learner_threads
        resolved_worker_threads = worker_torch_threads or resolved_threads
        resolved_worker_interop = worker_torch_interop_threads or 1
        resolved_worker_blas = worker_blas_threads or resolved_worker_threads
    elif profile in defaults:
        if workers or torch_threads or explicit_training_threads:
            raise ValueError("explicit worker/thread overrides require --profile custom")
        resolved_workers = defaults[profile]
        resolved_threads = 1
        resolved_learner_threads = learner_defaults[profile]
        resolved_learner_interop = 1
        resolved_learner_blas = resolved_learner_threads
        resolved_worker_threads = 1
        resolved_worker_interop = 1
        resolved_worker_blas = 1
    else:
        raise ValueError(f"unknown resource profile {profile!r}")
    thread_budgets = (
        resolved_threads,
        resolved_learner_threads,
        resolved_learner_interop,
        resolved_learner_blas,
        resolved_worker_threads,
        resolved_worker_interop,
        resolved_worker_blas,
    )
    if resolved_workers < 1 or any(value < 1 for value in thread_budgets):
        raise ValueError("worker and thread budgets must be positive")
    if resolved_workers > logical or resolved_workers * resolved_worker_threads > logical:
        raise ValueError(
            f"worker/thread budget {resolved_workers}x{resolved_worker_threads} exceeds "
            f"the machine's {logical} logical CPUs"
        )
    return ResourceBudget(
        profile=profile,
        workers=resolved_workers,
        torch_threads=resolved_threads,
        blas_threads=resolved_threads,
        learner_torch_threads=resolved_learner_threads,
        learner_torch_interop_threads=resolved_learner_interop,
        learner_blas_threads=resolved_learner_blas,
        worker_torch_threads=resolved_worker_threads,
        worker_torch_interop_threads=resolved_worker_interop,
        worker_blas_threads=resolved_worker_blas,
        inference_concurrency=resolved_workers,
        io_concurrency=min(2, resolved_workers),
        logical_cpus=logical,
    )


def configure_thread_runtime(
    *, torch_threads: int, torch_interop_threads: int, blas_threads: int
) -> None:
    """Apply an explicit intra/inter-op and BLAS contract in the current process."""

    if min(torch_threads, torch_interop_threads, blas_threads) < 1:
        raise ValueError("thread budgets must be positive")
    for variable in BLAS_THREAD_ENVIRONMENT:
        os.environ[variable] = str(blas_threads)
    import torch

    torch.set_num_threads(torch_threads)
    if torch.get_num_interop_threads() != torch_interop_threads:
        torch.set_num_interop_threads(torch_interop_threads)


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
