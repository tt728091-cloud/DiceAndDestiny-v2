from __future__ import annotations

import hashlib
import json
from pathlib import Path

from dice_destiny_ml.export_policy import ACTOR_TENSORS


def test_phase3_export_is_pinned_and_complete() -> None:
    server_root = Path(__file__).resolve().parents[2]
    model_path = (
        server_root.parent
        / "dice-and-destiny-client"
        / "models"
        / "learned"
        / "blade-warden-seed-11-final-v1.json"
    )
    payload = json.loads(model_path.read_text())
    assert hashlib.sha256(model_path.read_bytes()).hexdigest() == (
        "dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937"
    )
    assert payload["format"] == "dice-and-destiny-candidate-policy-v1"
    assert payload["model_id"] == "blade-warden-maskable-ppo-seed-11-final-v1"
    assert payload["source_checkpoint_sha256"] == (
        "e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8"
    )
    assert payload["source_parameter_sha256"] == (
        "b53c6633893bbaca6dd429edb6f1aeb94585e6c24cf05c2dbc1451e2fd758f7a"
    )
    assert payload["source_revision"] == "fff39759360ce89041940cec99410082af0671dd"
    assert payload["training_engine_revision"] == "5d81e8f6de350a3bdcc3f4ccf42a3447443f83fe"
    assert payload["observation_schema"] == "dice-and-destiny-observation-v1"
    assert payload["action_schema"] == "dice-and-destiny-action-candidates-v1"
    assert payload["observation_size"] == 4224
    assert payload["maximum_actions"] == 128
    assert set(payload["tensors"]) == set(ACTOR_TENSORS)
    for tensor in payload["tensors"].values():
        size = 1
        for dimension in tensor["shape"]:
            size *= dimension
        assert len(tensor["values"]) == size
