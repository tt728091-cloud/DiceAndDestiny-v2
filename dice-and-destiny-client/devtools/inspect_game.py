#!/usr/bin/env python3
"""Small standard-library client for the debug-only Godot game inspector."""

import argparse
import base64
import json
import os
import pathlib
import time
import urllib.error
import urllib.request


CLIENT_ROOT = pathlib.Path(__file__).resolve().parents[1]
DISCOVERY_ROOT = CLIENT_ROOT / ".godot" / "runtime"


def discover_inspector():
    records = list(DISCOVERY_ROOT.glob("**/inspectors/*.json"))
    records.sort(key=lambda path: path.stat().st_mtime_ns if path.exists() else 0, reverse=True)
    live = []
    for path in records:
        try:
            record = json.loads(path.read_text())
            pid = int(record["pid"])
            port = int(record["port"])
            project_root = pathlib.Path(record["project_root"]).resolve()
            if project_root != CLIENT_ROOT or port < 1 or port > 65535:
                continue
            os.kill(pid, 0)
            base = f"http://127.0.0.1:{port}"
            request(base, "", "GET", "/health")
            live.append((base, str(record["token"])))
        except (KeyError, ValueError, OSError, json.JSONDecodeError, urllib.error.URLError):
            try:
                path.unlink()
            except OSError:
                pass
    if live:
        return live[0]
    raise SystemExit(
        "No running inspector was found for this workspace. "
        "Start the game with DICE_AND_DESTINY_INSPECTOR=1 scripts/godot.sh."
    )


def request(base, token, method, path, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    headers = {"X-Inspector-Token": token} if token else {}
    if data is not None:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=65) as response:
            return json.loads(response.read())
    except urllib.error.HTTPError as error:
        body = error.read().decode(errors="replace")
        raise SystemExit(f"HTTP {error.code}: {body}") from error


def play_battle(base, token, max_actions, stop_presentation_type=None, stop_stage=None):
    ability_order = ("perfect_form", "golden_edge", "shield_bash", "sword_cut", "protect", "basic_defense")
    trace = []
    for _ in range(max_actions):
        state = request(base, token, "GET", "/state")
        controls = request(base, token, "GET", "/controls").get("controls", [])
        enabled = {item["id"]: item for item in controls if item.get("visible") and item.get("enabled")}
        if stop_presentation_type and state.get("presentation_type") == stop_presentation_type:
            return {"ok": True, "stopped_at": stop_presentation_type, "round": state.get("round"), "stage": state.get("stage"), "actions": trace}
        if stop_stage and state.get("stage") == stop_stage and not state.get("presentation_active"):
            return {"ok": True, "stopped_at": stop_stage, "round": state.get("round"), "stage": state.get("stage"), "actions": trace}
        if state.get("battle_result") and not state.get("presentation_active"):
            return {"ok": True, "result": state["battle_result"], "round": state.get("round"), "actions": trace, "completion_control": "battle.complete.play_again" in enabled}
        control_id = ""
        if state.get("presentation_active"):
            control_id = "battle.presentation.continue"
        elif state.get("stage") == "discard_to_hand_limit":
            commit = enabled.get("battle.hand_limit.commit")
            if commit:
                control_id = commit["id"]
            else:
                unselected = [item["id"] for item in enabled.values() if item["id"].startswith("battle.card.") and not item.get("pressed")]
                control_id = unselected[0] if unselected else ""
        elif state.get("stage") == "defense_selection" and not state.get("selected_source"):
            sources = sorted(item for item in enabled if item.startswith("battle.source."))
            control_id = sources[0] if sources else ""
        elif "battle.command.planning_roll" in enabled:
            control_id = "battle.command.planning_roll"
        else:
            abilities = [item for item in enabled if item.startswith("battle.ability.blade.")]
            for ability in ability_order:
                candidate = f"battle.ability.blade.{ability}"
                if candidate in abilities:
                    control_id = candidate
                    break
            if not control_id and "battle.command.roll_dice" in enabled:
                control_id = "battle.command.roll_dice"
            if not control_id and "battle.command.planning_reroll" in enabled:
                control_id = "battle.command.planning_reroll"
            if not control_id and "battle.command.planning_pass" in enabled:
                control_id = "battle.command.planning_pass"
            if not control_id and "battle.command.pass" in enabled:
                control_id = "battle.command.pass"
        if not control_id:
            return {"ok": False, "error": "no playable enabled control", "state": state, "controls": list(enabled), "actions": trace}
        result = request(base, token, "POST", f"/controls/{control_id}/activate", {})
        trace.append({"round": state.get("round"), "segment": state.get("segment"), "stage": state.get("stage"), "control": control_id})
        if not result.get("ok"):
            return {"ok": False, "error": "activation failed", "activation": result, "actions": trace}
        time.sleep(0.02)
    return {"ok": False, "error": "action limit exceeded", "actions": trace, "state": request(base, token, "GET", "/state")}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["health", "state", "controls", "errors", "screenshot", "activate", "wait", "play"])
    parser.add_argument("value", nargs="?")
    parser.add_argument("--base")
    parser.add_argument("--token")
    parser.add_argument("--out")
    parser.add_argument("--segment")
    parser.add_argument("--stage")
    parser.add_argument("--control-id")
    parser.add_argument("--enabled", action="store_true")
    parser.add_argument("--timeout-ms", type=int, default=10000)
    parser.add_argument("--max-actions", type=int, default=500)
    parser.add_argument("--stop-presentation-type")
    parser.add_argument("--stop-stage")
    args = parser.parse_args()

    base = args.base
    token = args.token
    if not base:
        base, discovered_token = discover_inspector()
        if not token:
            token = discovered_token
    if not token:
        token = os.environ.get("DICE_AND_DESTINY_INSPECTOR_TOKEN", "dev-inspector")

    if args.command in {"health", "state", "controls", "errors"}:
        result = request(base, token, "GET", "/" + args.command)
    elif args.command == "screenshot":
        result = request(base, token, "GET", "/screenshot")
        if args.out:
            path = pathlib.Path(args.out).resolve()
        else:
            output_root = DISCOVERY_ROOT / "workspace" / "screenshots"
            output_root.mkdir(parents=True, exist_ok=True)
            path = output_root / f"game-inspector-{time.time_ns()}.png"
        path.write_bytes(base64.b64decode(result.pop("png_base64")))
        result["saved_to"] = str(path)
    elif args.command == "activate":
        if not args.value:
            parser.error("activate requires a semantic control ID")
        result = request(base, token, "POST", f"/controls/{args.value}/activate", {})
    elif args.command == "wait":
        condition = {"timeout_ms": args.timeout_ms}
        for key in ("segment", "stage", "control_id"):
            value = getattr(args, key)
            if value:
                condition[key] = value
        if args.enabled:
            condition["enabled"] = True
        result = request(base, token, "POST", "/wait", condition)
    else:
        result = play_battle(base, token, args.max_actions, args.stop_presentation_type, args.stop_stage)
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
