from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

import numpy as np
import torch
from sb3_contrib import MaskablePPO

from .bridge import AuthorityBridge
from .policies import MechanicsPolicyV2
from .schema import SchemaEncoder
from .schema_v2 import OBSERVATION_SCHEMA_V2, OBSERVATION_SIZE_V2, SchemaEncoderV2

OWNER_BATTLE_ID = "learned-1785691059-2718787-15088"
OWNER_SEED = 1785691061228807


def run_owner_diagnostic(
    *,
    binary: Path,
    server_root: Path,
    trace_file: Path,
    accepted_checkpoint: Path,
    output_file: Path,
) -> dict[str, Any]:
    commands = []
    for line in trace_file.read_text().splitlines():
        record = json.loads(line)
        if record.get("battle_id") != OWNER_BATTLE_ID:
            continue
        if record.get("kind") in {"human_decision", "model_decision"}:
            commands.append(record["payload"]["command"])
    if len(commands) < 11:
        raise RuntimeError("owner trace does not contain the required first 11 decisions")

    with AuthorityBridge(
        binary,
        server_root,
        observation_schema=OBSERVATION_SCHEMA_V2,
        transport_mode="full",
        authority_mode="ephemeral",
        telemetry_mode="full",
        session_id="phase2-owner-diagnostic",
    ) as bridge:
        transition = bridge.reset(
            OWNER_SEED,
            {"seat-a": "human", "seat-b": "accepted-v1"},
            battle_id=OWNER_BATTLE_ID,
        )
        replayed = []
        for sequence, expected in enumerate(commands[:11], 1):
            matches = [
                index
                for index, candidate in enumerate(transition["result"]["legal_actions"])
                if candidate == expected
            ]
            if len(matches) != 1:
                raise RuntimeError(f"diagnostic decision {sequence} matched {len(matches)} candidates")
            replayed.append({"sequence": sequence, "candidate_index": matches[0], "command": expected})
            transition = bridge.step(matches[0])

    snapshot = transition["result"]["snapshot"]
    actor = snapshot["actors"][transition["actor_id"]]
    actions = transition["result"]["legal_actions"]
    dice_faces = [die["face"] for die in actor["dice"]["dice"]]
    if dice_faces != [6, 1, 3, 4, 2] or len(actions) != 68:
        raise RuntimeError(f"owner diagnostic drift: dice={dice_faces}, candidates={len(actions)}")

    v1_decision = SchemaEncoder().encode(transition)
    model = MaskablePPO.load(accepted_checkpoint, device="cpu")
    with torch.no_grad():
        distribution = model.policy.get_distribution(
            torch.as_tensor(v1_decision.observation[None, :]),
            action_masks=torch.as_tensor(v1_decision.action_mask[None, :]),
        )
        probabilities = distribution.distribution.probs.detach().cpu().numpy()[0]
    ranked = np.argsort(probabilities)[::-1]
    v1_top = [
        {
            "candidate_index": int(index),
            "probability": float(probabilities[index]),
            "command": actions[int(index)],
        }
        for index in ranked[:10]
    ]

    teacher = MechanicsPolicyV2()
    teacher.reset(OWNER_SEED, transition["actor_id"])
    teacher_index = teacher.select(transition, SchemaEncoderV2().encode(transition))
    teacher_command = actions[teacher_index]
    if teacher_command["type"] != "planning_reroll" or teacher_command["payload"].get(
        "reroll_indices"
    ) != [0, 3]:
        raise RuntimeError(f"mechanics teacher diagnostic regression: {teacher_command}")

    result = {
        "diagnostic": "owner-heldout-immediate-qualified-ability-bias-v1",
        "battle_id": OWNER_BATTLE_ID,
        "seed": OWNER_SEED,
        "trace_file": str(trace_file.resolve()),
        "trace_sha256": hashlib.sha256(trace_file.read_bytes()).hexdigest(),
        "accepted_checkpoint": str(accepted_checkpoint.resolve()),
        "accepted_checkpoint_sha256": hashlib.sha256(accepted_checkpoint.read_bytes()).hexdigest(),
        "replayed_commands": replayed,
        "decision": {
            "actor_id": transition["actor_id"],
            "dice_faces": dice_faces,
            "rolls_used": actor["dice"]["rolls_used"],
            "rolls_remaining": actor["dice"]["rolls_remaining"],
            "qualified_abilities": actor["qualified_abilities"],
            "candidate_count": len(actions),
        },
        "accepted_v1_top_probabilities": v1_top,
        "mechanics_v2_teacher": {
            "candidate_index": teacher_index,
            "command": teacher_command,
            "uses_future_authority_rng": False,
            "lookahead": "enumerated public die faces only",
        },
    }
    output_file.parent.mkdir(parents=True, exist_ok=True)
    output_file.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    return result


def score_checkpoint_on_transition(
    checkpoint: Path, transition: dict[str, Any]
) -> dict[str, Any]:
    """Return deterministic choice and legal probabilities for either family."""
    model = MaskablePPO.load(checkpoint, device="cpu")
    if tuple(model.observation_space.shape or ()) == (OBSERVATION_SIZE_V2,):
        decision = SchemaEncoderV2().encode(transition)
        family = "v2"
    else:
        decision = SchemaEncoder().encode(transition)
        family = "v1"
    with torch.no_grad():
        distribution = model.policy.get_distribution(
            torch.as_tensor(decision.observation[None, :]),
            action_masks=torch.as_tensor(decision.action_mask[None, :]),
        )
        probabilities = distribution.distribution.probs.detach().cpu().numpy()[0]
    actions = transition["result"]["legal_actions"]
    ranked = np.argsort(probabilities)[::-1]
    return {
        "checkpoint": str(checkpoint.resolve()),
        "checkpoint_sha256": hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        "family": family,
        "selected_index": int(ranked[0]),
        "selected_command": actions[int(ranked[0])],
        "top_probabilities": [
            {
                "candidate_index": int(index),
                "probability": float(probabilities[index]),
                "command": actions[int(index)],
            }
            for index in ranked[:10]
        ],
    }
