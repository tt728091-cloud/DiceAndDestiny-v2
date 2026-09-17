from __future__ import annotations

import json
import subprocess
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

from . import ACTION_SCHEMA_VERSION, ENVIRONMENT_SCHEMA_VERSION, OBSERVATION_SCHEMA_VERSION
from .manifest_v3 import ACTION_SCHEMA_V3, ENVIRONMENT_SCHEMA_V3, OBSERVATION_SCHEMA_V3
from .profiling import ProfileCollector, process_snapshot
from .schema_v2 import ACTION_SCHEMA_V2, ENVIRONMENT_SCHEMA_V2, OBSERVATION_SCHEMA_V2


class SimulatorError(RuntimeError):
    """The persistent authority simulator rejected a protocol operation."""


class AuthorityBridge:
    """Own one persistent Go simulator process across actions and episodes."""

    def __init__(
        self,
        binary: Path,
        server_root: Path,
        *,
        max_episode_actions: int = 1200,
        session_id: str = "",
        authority_mode: str = "normal",
        telemetry_mode: str = "full",
        transport_mode: str = "full",
        observation_schema: str = OBSERVATION_SCHEMA_VERSION,
        observation_manifest: Path | None = None,
        instrumentation: bool = False,
    ) -> None:
        command = [
            str(binary),
            "-content-root",
            str(server_root / "content"),
            "-run-state-root",
            str(server_root / "save" / "run_players"),
            "-max-actions",
            str(max_episode_actions),
            "-authority-mode",
            authority_mode,
            "-telemetry-mode",
            telemetry_mode,
            "-transport-mode",
            transport_mode,
            "-observation-schema",
            observation_schema,
        ]
        if observation_manifest is not None:
            command.extend(["-observation-manifest", str(observation_manifest)])
        if session_id:
            command.extend(["-session-id", session_id])
        self._process = subprocess.Popen(  # noqa: S603 - local repository binary only
            command,
            cwd=server_root,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        self._closed = False
        self.profile = ProfileCollector(instrumentation)
        if observation_schema == OBSERVATION_SCHEMA_V2:
            self._expected_versions = {
                "environment_schema": ENVIRONMENT_SCHEMA_V2,
                "observation_schema": OBSERVATION_SCHEMA_V2,
                "action_schema": ACTION_SCHEMA_V2,
            }
        elif observation_schema == OBSERVATION_SCHEMA_V3:
            self._expected_versions = {
                "environment_schema": ENVIRONMENT_SCHEMA_V3,
                "observation_schema": OBSERVATION_SCHEMA_V3,
                "action_schema": ACTION_SCHEMA_V3,
            }
        elif observation_schema == OBSERVATION_SCHEMA_VERSION:
            self._expected_versions = {
                "environment_schema": ENVIRONMENT_SCHEMA_VERSION,
                "observation_schema": OBSERVATION_SCHEMA_VERSION,
                "action_schema": ACTION_SCHEMA_VERSION,
            }
        else:
            raise ValueError(f"unsupported observation schema {observation_schema!r}")
        self.operation_seconds: dict[str, list[float]] = defaultdict(list)

    def request(self, operation: str, **payload: Any) -> dict[str, Any]:
        started = time.perf_counter()
        if self._closed or self._process.poll() is not None:
            stderr = self._read_stderr()
            raise SimulatorError(f"simulator is not running{': ' + stderr if stderr else ''}")
        assert self._process.stdin is not None
        assert self._process.stdout is not None
        message = {"op": operation, **payload}
        with self.profile.span("bridge.request_encode"):
            encoded_message = json.dumps(message, separators=(",", ":")) + "\n"
        self.profile.increment("bridge.request_bytes", len(encoded_message.encode("utf-8")))
        with self.profile.span("bridge.pipe_round_trip"):
            self._process.stdin.write(encoded_message)
            self._process.stdin.flush()
            line = self._process.stdout.readline()
        if not line:
            raise SimulatorError(f"simulator exited without a response: {self._read_stderr()}")
        self.profile.increment("bridge.response_bytes", len(line.encode("utf-8")))
        with self.profile.span("bridge.response_decode"):
            response = json.loads(line)
        self._verify_versions(response)
        if not response.get("ok"):
            raise SimulatorError(response.get("error", "unknown simulator error"))
        elapsed = time.perf_counter() - started
        self.operation_seconds[operation].append(elapsed)
        self.profile.observe(f"bridge.operation.{operation}", elapsed)
        return response

    def profile_summary(self) -> dict[str, Any]:
        result = self.profile.summary()
        result["process"] = process_snapshot(
            include_torch=True,
            child_pid=self._process.pid,
        )
        return result

    def reset(
        self,
        seed: int,
        seat_models: dict[str, str],
        *,
        battle_id: str = "",
        seat_definitions: dict[str, str] | None = None,
    ) -> dict[str, Any]:
        return self.request(
            "reset",
            seed=int(seed),
            battle_id=battle_id,
            seat_models=seat_models,
            seat_definitions=seat_definitions
            or {
                "seat-a": "blade_warden",
                "seat-b": "blade_warden",
            },
        )["transition"]

    def step(self, action_index: int) -> dict[str, Any]:
        return self.request("step", action_index=int(action_index))["transition"]

    def observe(self, seat_id: str) -> dict[str, Any]:
        return self.request("observe", seat_id=seat_id)["result"]

    def legal_actions(self, seat_id: str) -> list[dict[str, Any]]:
        return self.request("legal_actions", seat_id=seat_id)["result"]

    def replay(self, record: dict[str, Any]) -> dict[str, Any]:
        return self.request("replay", replay=record)["transition"]

    def close(self) -> None:
        if self._closed:
            return
        try:
            if self._process.poll() is None:
                self.request("close")
        finally:
            self._closed = True
            if self._process.stdin is not None:
                self._process.stdin.close()
            try:
                self._process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._process.terminate()
                self._process.wait(timeout=5)

    def __enter__(self) -> AuthorityBridge:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()

    def _verify_versions(self, response: dict[str, Any]) -> None:
        for key, value in self._expected_versions.items():
            if response.get(key) != value:
                raise SimulatorError(f"{key} mismatch: {response.get(key)!r}, expected {value!r}")

    def _read_stderr(self) -> str:
        if self._process.stderr is None or self._process.poll() is None:
            return ""
        return self._process.stderr.read().strip()
