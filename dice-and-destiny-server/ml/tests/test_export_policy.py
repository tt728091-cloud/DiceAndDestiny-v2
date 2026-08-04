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


def test_phase2_v2_export_is_separate_pinned_and_complete() -> None:
    server_root = Path(__file__).resolve().parents[2]
    model_path = (
        server_root.parent
        / "dice-and-destiny-client"
        / "models"
        / "learned"
        / "blade-warden-decision-quality-seed-22-v2.json"
    )
    payload = json.loads(model_path.read_text())
    assert hashlib.sha256(model_path.read_bytes()).hexdigest() == (
        "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
    )
    assert payload["format"] == "dice-and-destiny-candidate-policy-v2"
    assert payload["model_id"] == "blade-warden-decision-quality-seed-22-v2"
    assert payload["source_checkpoint_sha256"] == (
        "e3b9fb8c393f17c6d7a70a563a55e2596e83651f16c9e0fa226096b082f772e2"
    )
    assert payload["source_parameter_sha256"] == (
        "e82001aaa98066e3a60823631de4622ba654267036c140d7647330b69edeb4a5"
    )
    assert payload["content_version"] == ("9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4")
    assert payload["environment_schema"] == "dice-and-destiny-ml-env-v2"
    assert payload["observation_schema"] == "dice-and-destiny-observation-v2"
    assert payload["action_schema"] == "dice-and-destiny-action-candidates-v2"
    assert payload["observation_size"] == 18_944
    assert payload["maximum_actions"] == 128
    assert payload["base_features"] == 2_560
    assert payload["action_features"] == 128
    assert set(payload["tensors"]) == set(ACTOR_TENSORS)
    for tensor in payload["tensors"].values():
        size = 1
        for dimension in tensor["shape"]:
            size *= dimension
        assert len(tensor["values"]) == size


def test_optimized_v3_export_is_separate_pinned_and_complete() -> None:
    server_root = Path(__file__).resolve().parents[2]
    model_path = (
        server_root.parent
        / "dice-and-destiny-client"
        / "models"
        / "learned"
        / "blade-warden-optimized-5m-seed-22-v3.json"
    )
    payload = json.loads(model_path.read_text())
    assert hashlib.sha256(model_path.read_bytes()).hexdigest() == (
        "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"
    )
    assert payload["format"] == "dice-and-destiny-candidate-policy-v2"
    assert payload["model_id"] == "blade-warden-optimized-5m-seed-22-v3"
    assert payload["source_checkpoint_sha256"] == (
        "5e2214b89cbe84f5405f5fef87f368453801598fc2125f3535c58a245c3b876d"
    )
    assert payload["source_parameter_sha256"] == (
        "8f682a5645692ba278f2082c01411b86c01afb0e5d457583a7a4ab6fb8f3431d"
    )
    assert payload["source_revision"] == "5f3de5e2be2cfdd16b66415f8279f58707506288"
    assert payload["training_engine_revision"] == "5f3de5e2be2cfdd16b66415f8279f58707506288"
    assert payload["content_version"] == ("9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4")
    assert payload["environment_schema"] == "dice-and-destiny-ml-env-v2"
    assert payload["observation_schema"] == "dice-and-destiny-observation-v2"
    assert payload["action_schema"] == "dice-and-destiny-action-candidates-v2"
    assert payload["observation_size"] == 18_944
    assert payload["maximum_actions"] == 128
    assert payload["base_features"] == 2_560
    assert payload["action_features"] == 128
    assert set(payload["tensors"]) == set(ACTOR_TENSORS)
    for tensor in payload["tensors"].values():
        size = 1
        for dimension in tensor["shape"]:
            size *= dimension
        assert len(tensor["values"]) == size
