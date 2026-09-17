from __future__ import annotations

from pathlib import Path
from typing import Any

import numpy as np
import torch
from sb3_contrib.common.maskable.policies import MaskableActorCriticPolicy
from torch import nn

from .manifest_v3 import ObservationManifestV3
from .manifest_v4 import ObservationManifestV4
from .manifest_v5 import ObservationManifestV5
from .profiling import ProfileCollector
from .schema import ACTION_FEATURES, BASE_FEATURES, MAX_ACTIONS, OBSERVATION_SIZE
from .schema_v2 import (
    ACTION_FEATURES_V2,
    BASE_FEATURES_V2,
    MAX_ACTIONS_V2,
    OBSERVATION_SIZE_V2,
)


class InstrumentedMaskablePolicyMixin:
    profile: ProfileCollector

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        self.profile = ProfileCollector(False)
        super().__init__(*args, **kwargs)

    def enable_instrumentation(self, enabled: bool = True) -> None:
        self.profile.enabled = enabled

    def forward(
        self,
        obs: torch.Tensor,
        deterministic: bool = False,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        if not self.profile.enabled:
            return super().forward(obs, deterministic, action_masks)
        with self.profile.span("policy.rollout.extract_features"):
            features = self.extract_features(obs)
        with self.profile.span("policy.rollout.actor_critic_latent"):
            latent_pi, latent_vf = self.mlp_extractor(features)
        with self.profile.span("policy.rollout.value_forward"):
            values = self.value_net(latent_vf)
        with self.profile.span("policy.rollout.action_logits_distribution"):
            distribution = self._get_action_dist_from_latent(latent_pi)
        if action_masks is not None:
            with self.profile.span("policy.rollout.mask_distribution"):
                distribution.apply_masking(action_masks)
        with self.profile.span("policy.rollout.sample_log_prob"):
            actions = distribution.get_actions(deterministic=deterministic)
            log_prob = distribution.log_prob(actions)
        actions = actions.reshape((-1, *self.action_space.shape))
        return actions, values, log_prob

    def evaluate_actions(
        self,
        obs: torch.Tensor,
        actions: torch.Tensor,
        action_masks: torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor | None]:
        if not self.profile.enabled:
            return super().evaluate_actions(obs, actions, action_masks)
        with self.profile.span("policy.update.extract_features"):
            features = self.extract_features(obs)
        with self.profile.span("policy.update.actor_critic_latent"):
            latent_pi, latent_vf = self.mlp_extractor(features)
        with self.profile.span("policy.update.action_logits_distribution"):
            distribution = self._get_action_dist_from_latent(latent_pi)
        if action_masks is not None:
            with self.profile.span("policy.update.mask_distribution"):
                distribution.apply_masking(action_masks)
        with self.profile.span("policy.update.log_prob_entropy"):
            log_prob = distribution.log_prob(actions)
            entropy = distribution.entropy()
        with self.profile.span("policy.update.value_forward"):
            values = self.value_net(latent_vf)
        return values, log_prob, entropy

    def get_distribution(
        self,
        obs: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> Any:
        if not self.profile.enabled:
            return super().get_distribution(obs, action_masks)
        with self.profile.span("policy.predict.extract_features"):
            features = super().extract_features(obs, self.pi_features_extractor)
            latent_pi = self.mlp_extractor.forward_actor(features)
        with self.profile.span("policy.predict.action_logits_distribution"):
            distribution = self._get_action_dist_from_latent(latent_pi)
        if action_masks is not None:
            with self.profile.span("policy.predict.mask_distribution"):
                distribution.apply_masking(action_masks)
        return distribution


class CandidateMlpExtractor(nn.Module):
    """Keep raw candidates for the actor and learn a compact value latent."""

    latent_dim_pi = OBSERVATION_SIZE
    latent_dim_vf = 128

    def __init__(self) -> None:
        super().__init__()
        self.value_network = nn.Sequential(
            nn.Linear(OBSERVATION_SIZE, 128),
            nn.Tanh(),
            nn.Linear(128, 128),
            nn.Tanh(),
        )

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, self.value_network(features)

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        return self.value_network(features)


class CandidateActionNet(nn.Module):
    """Score every candidate with shared weights conditioned on viewer state."""

    def __init__(self) -> None:
        super().__init__()
        self.context = nn.Sequential(
            nn.Linear(BASE_FEATURES, 64),
            nn.Tanh(),
            nn.Linear(64, 64),
            nn.Tanh(),
        )
        self.candidate = nn.Sequential(
            nn.Linear(ACTION_FEATURES, 64),
            nn.Tanh(),
            nn.Linear(64, 64),
            nn.Tanh(),
        )
        self.score = nn.Sequential(
            nn.Linear(128, 64),
            nn.Tanh(),
            nn.Linear(64, 1),
        )

    def forward(self, features: torch.Tensor) -> torch.Tensor:
        base = features[:, :BASE_FEATURES]
        candidates = features[:, BASE_FEATURES:].reshape(-1, MAX_ACTIONS, ACTION_FEATURES)
        context = self.context(base).unsqueeze(1).expand(-1, MAX_ACTIONS, -1)
        candidate = self.candidate(candidates)
        return self.score(torch.cat((context, candidate), dim=-1)).squeeze(-1)


class CandidateMaskablePolicy(InstrumentedMaskablePolicyMixin, MaskableActorCriticPolicy):
    """Maskable actor-critic policy with a position-independent action scorer."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        kwargs["ortho_init"] = False
        kwargs["net_arch"] = []
        super().__init__(*args, **kwargs)

    def _build_mlp_extractor(self) -> None:
        self.mlp_extractor = CandidateMlpExtractor().to(self.device)

    def _build(self, lr_schedule: Any) -> None:
        self._build_mlp_extractor()
        self.action_net = CandidateActionNet().to(self.device)
        self.value_net = nn.Linear(self.mlp_extractor.latent_dim_vf, 1).to(self.device)
        self.optimizer = self.optimizer_class(
            self.parameters(),
            lr=lr_schedule(1),
            **self.optimizer_kwargs,
        )


class CandidateMlpExtractorV2(nn.Module):
    latent_dim_pi = OBSERVATION_SIZE_V2
    latent_dim_vf = 128

    def __init__(self) -> None:
        super().__init__()
        self.value_network = nn.Sequential(
            nn.Linear(OBSERVATION_SIZE_V2, 128),
            nn.Tanh(),
            nn.Linear(128, 128),
            nn.Tanh(),
        )

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, self.value_network(features)

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        return self.value_network(features)


class BaseOnlyCandidateMlpExtractorV2(nn.Module):
    """Learning-architecture ablation: value function sees viewer/base features only."""

    latent_dim_pi = OBSERVATION_SIZE_V2
    latent_dim_vf = 128

    def __init__(self) -> None:
        super().__init__()
        self.value_network = nn.Sequential(
            nn.Linear(BASE_FEATURES_V2, 128),
            nn.Tanh(),
            nn.Linear(128, 128),
            nn.Tanh(),
        )

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, self.forward_critic(features)

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        return self.value_network(features[:, :BASE_FEATURES_V2])


class CandidateActionNetV2(nn.Module):
    def __init__(self) -> None:
        super().__init__()
        self.context = nn.Sequential(
            nn.Linear(BASE_FEATURES_V2, 96),
            nn.Tanh(),
            nn.Linear(96, 96),
            nn.Tanh(),
        )
        self.candidate = nn.Sequential(
            nn.Linear(ACTION_FEATURES_V2, 96),
            nn.Tanh(),
            nn.Linear(96, 96),
            nn.Tanh(),
        )
        self.score = nn.Sequential(nn.Linear(192, 96), nn.Tanh(), nn.Linear(96, 1))

    def forward(self, features: torch.Tensor) -> torch.Tensor:
        base = features[:, :BASE_FEATURES_V2]
        candidates = features[:, BASE_FEATURES_V2:].reshape(-1, MAX_ACTIONS_V2, ACTION_FEATURES_V2)
        context = self.context(base).unsqueeze(1).expand(-1, MAX_ACTIONS_V2, -1)
        candidate = self.candidate(candidates)
        return self.score(torch.cat((context, candidate), dim=-1)).squeeze(-1)

    def forward_sparse(
        self,
        features: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor,
    ) -> torch.Tensor:
        """Score only valid candidates, then scatter into the public 128-slot API."""

        masks = torch.as_tensor(action_masks, dtype=torch.bool, device=features.device)
        if masks.ndim == 1:
            masks = masks.unsqueeze(0)
        if masks.shape != (features.shape[0], MAX_ACTIONS_V2):
            raise ValueError(
                f"expected action masks {(features.shape[0], MAX_ACTIONS_V2)}, got {masks.shape}"
            )
        base = features[:, :BASE_FEATURES_V2]
        candidates = features[:, BASE_FEATURES_V2:].reshape(-1, MAX_ACTIONS_V2, ACTION_FEATURES_V2)
        rows, columns = masks.nonzero(as_tuple=True)
        if rows.numel() == 0:
            raise ValueError("every sparse actor batch must contain a valid candidate")
        context = self.context(base)
        candidate = self.candidate(candidates[rows, columns])
        scores = self.score(torch.cat((context[rows], candidate), dim=-1)).squeeze(-1)
        logits = features.new_zeros((features.shape[0], MAX_ACTIONS_V2))
        return logits.index_put((rows, columns), scores)


class CandidateMaskablePolicyV2(InstrumentedMaskablePolicyMixin, MaskableActorCriticPolicy):
    """Separate mechanics-aware model family; never reinterprets v1 weights."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        kwargs["ortho_init"] = False
        kwargs["net_arch"] = []
        super().__init__(*args, **kwargs)

    def _build_mlp_extractor(self) -> None:
        self.mlp_extractor = CandidateMlpExtractorV2().to(self.device)

    def _build(self, lr_schedule: Any) -> None:
        self._build_mlp_extractor()
        self.action_net = CandidateActionNetV2().to(self.device)
        self.value_net = nn.Linear(self.mlp_extractor.latent_dim_vf, 1).to(self.device)
        self.optimizer = self.optimizer_class(self.parameters(), lr=lr_schedule(1), **self.optimizer_kwargs)


class SparseCandidateMaskablePolicyV2(CandidateMaskablePolicyV2):
    """V2 actor that preserves the dense public action API but skips padded slots."""

    def _sparse_distribution(
        self,
        latent_pi: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None,
    ) -> Any:
        if action_masks is None:
            return self._get_action_dist_from_latent(latent_pi)
        logits = self.action_net.forward_sparse(latent_pi, action_masks)
        distribution = self.action_dist.proba_distribution(action_logits=logits)
        distribution.apply_masking(action_masks)
        return distribution

    def forward(
        self,
        obs: torch.Tensor,
        deterministic: bool = False,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        with self.profile.span("policy.rollout.extract_features"):
            features = self.extract_features(obs)
        with self.profile.span("policy.rollout.actor_critic_latent"):
            latent_pi, latent_vf = self.mlp_extractor(features)
        with self.profile.span("policy.rollout.value_forward"):
            values = self.value_net(latent_vf)
        with self.profile.span("policy.rollout.action_logits_distribution"):
            distribution = self._sparse_distribution(latent_pi, action_masks)
        with self.profile.span("policy.rollout.sample_log_prob"):
            actions = distribution.get_actions(deterministic=deterministic)
            log_prob = distribution.log_prob(actions)
        actions = actions.reshape((-1, *self.action_space.shape))
        return actions, values, log_prob

    def evaluate_actions(
        self,
        obs: torch.Tensor,
        actions: torch.Tensor,
        action_masks: torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor | None]:
        with self.profile.span("policy.update.extract_features"):
            features = self.extract_features(obs)
        with self.profile.span("policy.update.actor_critic_latent"):
            latent_pi, latent_vf = self.mlp_extractor(features)
        with self.profile.span("policy.update.action_logits_distribution"):
            distribution = self._sparse_distribution(latent_pi, action_masks)
        with self.profile.span("policy.update.log_prob_entropy"):
            log_prob = distribution.log_prob(actions)
            entropy = distribution.entropy()
        with self.profile.span("policy.update.value_forward"):
            values = self.value_net(latent_vf)
        return values, log_prob, entropy

    def get_distribution(
        self,
        obs: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> Any:
        with self.profile.span("policy.predict.extract_features"):
            features = self.extract_features(obs, self.pi_features_extractor)
            latent_pi = self.mlp_extractor.forward_actor(features)
        with self.profile.span("policy.predict.action_logits_distribution"):
            return self._sparse_distribution(latent_pi, action_masks)


class SparseBaseCriticCandidateMaskablePolicyV2(SparseCandidateMaskablePolicyV2):
    """Separate base-only critic experiment; never reinterprets dense-critic weights."""

    def _build_mlp_extractor(self) -> None:
        self.mlp_extractor = BaseOnlyCandidateMlpExtractorV2().to(self.device)


def _activation(name: str) -> type[nn.Module]:
    if name == "tanh":
        return nn.Tanh
    if name == "relu":
        return nn.ReLU
    if name == "gelu":
        return nn.GELU
    raise ValueError(f"unsupported v3 activation {name!r}")


def _mlp(
    input_size: int,
    hidden_size: int,
    output_size: int,
    *,
    depth: int,
    activation: str,
) -> nn.Sequential:
    if depth < 1:
        raise ValueError("v3 MLP depth must be positive")
    activation_class = _activation(activation)
    layers: list[nn.Module] = []
    current = input_size
    for _ in range(depth):
        layers.extend((nn.Linear(current, hidden_size), activation_class()))
        current = hidden_size
    layers.append(nn.Linear(current, output_size))
    return nn.Sequential(*layers)


class EntityMlpExtractorV3(nn.Module):
    """Actor retains entity rows; critic pools only occupied rows by entity type."""

    latent_dim_vf = 128

    def __init__(
        self,
        manifest: ObservationManifestV3,
        *,
        entity_width: int,
        depth: int,
        activation: str,
    ) -> None:
        super().__init__()
        self.manifest = manifest
        self.latent_dim_pi = manifest.layout.observation_size
        self.context = _mlp(
            manifest.layout.context.features,
            entity_width,
            entity_width,
            depth=depth,
            activation=activation,
        )
        self.entity_names = ("actors", "dice", "abilities", "statuses", "cards") + (
            ("details",) if hasattr(manifest.layout, "details") else ()
        )
        self.entity_encoders = nn.ModuleDict(
            {
                name: nn.Identity()
                if name == "details"
                else _mlp(
                    getattr(manifest.layout, name).features,
                    entity_width,
                    entity_width,
                    depth=depth,
                    activation=activation,
                )
                for name in self.entity_names
            }
        )
        pooled_size = entity_width * (1 + len(self.entity_names))
        self.value_network = _mlp(
            pooled_size,
            entity_width * 2,
            self.latent_dim_vf,
            depth=depth,
            activation=activation,
        )

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, self.forward_critic(features)

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        context_range = self.manifest.layout.context
        context = features[:, context_range.offset : context_range.end]
        pooled = [self.context(context)]
        for name in self.entity_names:
            entity_range = getattr(self.manifest.layout, name)
            rows = features[:, entity_range.offset : entity_range.end].reshape(
                -1, entity_range.rows, entity_range.features
            )
            mask = rows[:, :, 0] > 0
            encoded = self.entity_encoders[name](rows)
            denominator = mask.sum(dim=1, keepdim=True).clamp_min(1).to(encoded.dtype)
            pooled.append((encoded * mask.unsqueeze(-1)).sum(dim=1) / denominator)
        return self.value_network(torch.cat(pooled, dim=-1))


class EntityCandidateActionNetV3(nn.Module):
    def __init__(
        self,
        manifest: ObservationManifestV3,
        *,
        entity_width: int,
        depth: int,
        activation: str,
    ) -> None:
        super().__init__()
        self.manifest = manifest
        self.context = _mlp(
            manifest.layout.context.features,
            entity_width,
            entity_width,
            depth=depth,
            activation=activation,
        )
        self.state_names = ("actors", "dice", "abilities", "statuses", "cards") + (
            ("details",) if hasattr(manifest.layout, "details") else ()
        )
        self.state_encoders = nn.ModuleDict(
            {
                name: nn.Identity()
                if name == "details"
                else _mlp(
                    getattr(manifest.layout, name).features,
                    entity_width,
                    entity_width,
                    depth=depth,
                    activation=activation,
                )
                for name in self.state_names
            }
        )
        self.candidate = _mlp(
            manifest.layout.candidates.features,
            entity_width,
            entity_width,
            depth=depth,
            activation=activation,
        )
        state_width = entity_width * (1 + len(self.state_names))
        self.score = _mlp(
            state_width + entity_width,
            entity_width,
            1,
            depth=depth,
            activation=activation,
        )

    def _pooled_state(self, features: torch.Tensor) -> torch.Tensor:
        context_range = self.manifest.layout.context
        context = features[:, context_range.offset : context_range.end]
        pooled = [self.context(context)]
        for name in self.state_names:
            entity_range = getattr(self.manifest.layout, name)
            rows = features[:, entity_range.offset : entity_range.end].reshape(
                -1, entity_range.rows, entity_range.features
            )
            mask = rows[:, :, 0] > 0
            encoded = self.state_encoders[name](rows)
            denominator = mask.sum(dim=1, keepdim=True).clamp_min(1).to(encoded.dtype)
            pooled.append((encoded * mask.unsqueeze(-1)).sum(dim=1) / denominator)
        return torch.cat(pooled, dim=-1)

    def forward(self, features: torch.Tensor) -> torch.Tensor:
        candidate_range = self.manifest.layout.candidates
        candidates = features[:, candidate_range.offset : candidate_range.end].reshape(
            -1, candidate_range.rows, candidate_range.features
        )
        state = self._pooled_state(features).unsqueeze(1).expand(-1, candidate_range.rows, -1)
        encoded = self.candidate(candidates)
        return self.score(torch.cat((state, encoded), dim=-1)).squeeze(-1)

    def forward_sparse(self, features: torch.Tensor, action_masks: np.ndarray | torch.Tensor) -> torch.Tensor:
        masks = torch.as_tensor(action_masks, dtype=torch.bool, device=features.device)
        if masks.ndim == 1:
            masks = masks.unsqueeze(0)
        capacity = self.manifest.maximum_legal_candidates
        if masks.shape != (features.shape[0], capacity):
            raise ValueError(f"expected v3 action masks {(features.shape[0], capacity)}, got {masks.shape}")
        rows, columns = masks.nonzero(as_tuple=True)
        if rows.numel() == 0:
            raise ValueError("every sparse v3 actor batch must contain a valid candidate")
        candidate_range = self.manifest.layout.candidates
        candidates = features[:, candidate_range.offset : candidate_range.end].reshape(
            -1, candidate_range.rows, candidate_range.features
        )
        state = self._pooled_state(features)
        encoded = self.candidate(candidates[rows, columns])
        scores = self.score(torch.cat((state[rows], encoded), dim=-1)).squeeze(-1)
        logits = features.new_zeros((features.shape[0], capacity))
        return logits.index_put((rows, columns), scores)


class SparseEntityCandidateMaskablePolicyV3(InstrumentedMaskablePolicyMixin, MaskableActorCriticPolicy):
    """Version-separated sparse entity policy for the frozen Observation V3 manifest."""

    def __init__(
        self,
        *args: Any,
        manifest_path: str,
        entity_width: int = 96,
        entity_depth: int = 2,
        activation: str = "tanh",
        **kwargs: Any,
    ) -> None:
        self.manifest_path = str(manifest_path)
        self.manifest = ObservationManifestV3.load(Path(self.manifest_path))
        self.entity_width = int(entity_width)
        self.entity_depth = int(entity_depth)
        self.entity_activation = activation
        kwargs["ortho_init"] = False
        kwargs["net_arch"] = []
        super().__init__(*args, **kwargs)
        if tuple(self.observation_space.shape or ()) != (self.manifest.layout.observation_size,):
            raise RuntimeError("v3 policy observation space does not match frozen manifest")
        if int(self.action_space.n) != self.manifest.maximum_legal_candidates:
            raise RuntimeError("v3 policy action space does not match frozen manifest")

    def _build_mlp_extractor(self) -> None:
        self.mlp_extractor = EntityMlpExtractorV3(
            self.manifest,
            entity_width=self.entity_width,
            depth=self.entity_depth,
            activation=self.entity_activation,
        ).to(self.device)

    def _build(self, lr_schedule: Any) -> None:
        self._build_mlp_extractor()
        self.action_net = EntityCandidateActionNetV3(
            self.manifest,
            entity_width=self.entity_width,
            depth=self.entity_depth,
            activation=self.entity_activation,
        ).to(self.device)
        self.value_net = nn.Linear(self.mlp_extractor.latent_dim_vf, 1).to(self.device)
        self.optimizer = self.optimizer_class(self.parameters(), lr=lr_schedule(1), **self.optimizer_kwargs)

    def _sparse_distribution(
        self,
        latent_pi: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None,
    ) -> Any:
        if action_masks is None:
            return self._get_action_dist_from_latent(latent_pi)
        logits = self.action_net.forward_sparse(latent_pi, action_masks)
        distribution = self.action_dist.proba_distribution(action_logits=logits)
        distribution.apply_masking(action_masks)
        return distribution

    def forward(
        self,
        obs: torch.Tensor,
        deterministic: bool = False,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        features = self.extract_features(obs)
        latent_pi, latent_vf = self.mlp_extractor(features)
        values = self.value_net(latent_vf)
        distribution = self._sparse_distribution(latent_pi, action_masks)
        actions = distribution.get_actions(deterministic=deterministic)
        log_prob = distribution.log_prob(actions)
        actions = actions.reshape((-1, *self.action_space.shape))
        return actions, values, log_prob

    def evaluate_actions(
        self,
        obs: torch.Tensor,
        actions: torch.Tensor,
        action_masks: torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor | None]:
        features = self.extract_features(obs)
        latent_pi, latent_vf = self.mlp_extractor(features)
        distribution = self._sparse_distribution(latent_pi, action_masks)
        return (
            self.value_net(latent_vf),
            distribution.log_prob(actions),
            distribution.entropy(),
        )

    def get_distribution(
        self,
        obs: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> Any:
        features = self.extract_features(obs, self.pi_features_extractor)
        latent_pi = self.mlp_extractor.forward_actor(features)
        return self._sparse_distribution(latent_pi, action_masks)


class SparseEntityCandidateMaskablePolicyV4(SparseEntityCandidateMaskablePolicyV3):
    """V4 policy keeps the CP480 PPO network recipe and consumes detail rows."""

    def __init__(
        self,
        *args: Any,
        manifest_path: str,
        entity_width: int = 96,
        entity_depth: int = 2,
        activation: str = "tanh",
        **kwargs: Any,
    ) -> None:
        self.manifest_path = str(manifest_path)
        self.manifest = ObservationManifestV4.load(Path(self.manifest_path))
        self.entity_width = int(entity_width)
        self.entity_depth = int(entity_depth)
        self.entity_activation = activation
        kwargs["ortho_init"] = False
        kwargs["net_arch"] = []
        # Skip V3's manifest loader while retaining the instrumented SB3 setup.
        super(SparseEntityCandidateMaskablePolicyV3, self).__init__(*args, **kwargs)
        if tuple(self.observation_space.shape or ()) != (self.manifest.layout.observation_size,):
            raise RuntimeError("v4 policy observation space does not match frozen manifest")
        if int(self.action_space.n) != self.manifest.maximum_legal_candidates:
            raise RuntimeError("v4 policy action space does not match frozen manifest")


def _masked_mean_max(encoded: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    weights = mask.unsqueeze(-1).to(encoded.dtype)
    count = weights.sum(dim=1).clamp_min(1.0)
    mean = (encoded * weights).sum(dim=1) / count
    minimum = torch.finfo(encoded.dtype).min
    maximum = encoded.masked_fill(~mask.unsqueeze(-1), minimum).amax(dim=1)
    maximum = torch.where(mask.any(dim=1, keepdim=True), maximum, torch.zeros_like(maximum))
    return torch.cat((mean, maximum), dim=-1)


def _grouped_weighted_max(
    encoded: torch.Tensor,
    groups: torch.Tensor,
    valid: torch.Tensor,
    group_count: int,
    logits: torch.Tensor,
) -> torch.Tensor:
    """Learned set attention plus max, without mixing group identities."""

    batch, _, width = encoded.shape
    safe_groups = groups.clamp(0, group_count - 1)
    indices = safe_groups.unsqueeze(-1).expand(-1, -1, width)
    weights = torch.sigmoid(logits).squeeze(-1) * valid.to(encoded.dtype)
    sums = encoded.new_zeros((batch, group_count, width))
    sums.scatter_add_(1, indices, encoded * weights.unsqueeze(-1))
    totals = encoded.new_zeros((batch, group_count, 1))
    totals.scatter_add_(1, safe_groups.unsqueeze(-1), weights.unsqueeze(-1))
    means = sums / totals.clamp_min(1e-6)

    minimum = torch.finfo(encoded.dtype).min
    sources = encoded.masked_fill(~valid.unsqueeze(-1), minimum)
    maximum = encoded.new_full((batch, group_count, width), minimum)
    maximum.scatter_reduce_(1, indices, sources, reduce="amax", include_self=True)
    maximum = torch.where(totals > 0, maximum, torch.zeros_like(maximum))
    return torch.cat((means, maximum), dim=-1)


def _grouped_weighted_max_sparse(
    encoded: torch.Tensor,
    groups: torch.Tensor,
    group_count: int,
    logits: torch.Tensor,
) -> torch.Tensor:
    """Learned set pooling over occupied rows only."""

    width = encoded.shape[-1]
    if encoded.shape[0] == 0:
        return encoded.new_zeros((group_count, width * 2))
    weights = torch.sigmoid(logits).squeeze(-1)
    indices = groups.unsqueeze(-1).expand(-1, width)
    sums = encoded.new_zeros((group_count, width))
    sums.scatter_add_(0, indices, encoded * weights.unsqueeze(-1))
    totals = encoded.new_zeros((group_count, 1))
    totals.scatter_add_(0, groups.unsqueeze(-1), weights.unsqueeze(-1))
    means = sums / totals.clamp_min(1e-6)
    minimum = torch.finfo(encoded.dtype).min
    maximum = encoded.new_full((group_count, width), minimum)
    maximum.scatter_reduce_(0, indices, encoded, reduce="amax", include_self=True)
    maximum = torch.where(totals > 0, maximum, torch.zeros_like(maximum))
    return torch.cat((means, maximum), dim=-1)


def _encoded_mean_max_sparse_rows(raw: torch.Tensor, encoder: nn.Module) -> torch.Tensor:
    """Encode occupied rows only, then mean/max pool each batch independently."""

    batch_indices, row_indices = (raw[:, :, 0] > 0).nonzero(as_tuple=True)
    encoded = encoder(raw[batch_indices, row_indices])
    batch_size = raw.shape[0]
    width = encoded.shape[-1]
    sums = encoded.new_zeros((batch_size, width))
    sums.scatter_add_(0, batch_indices.unsqueeze(-1).expand(-1, width), encoded)
    counts = encoded.new_zeros((batch_size, 1))
    counts.scatter_add_(0, batch_indices.unsqueeze(-1), encoded.new_ones((encoded.shape[0], 1)))
    means = sums / counts.clamp_min(1)
    minimum = torch.finfo(encoded.dtype).min
    maximum = encoded.new_full((batch_size, width), minimum)
    maximum.scatter_reduce_(
        0,
        batch_indices.unsqueeze(-1).expand(-1, width),
        encoded,
        reduce="amax",
        include_self=True,
    )
    maximum = torch.where(counts > 0, maximum, torch.zeros_like(maximum))
    return torch.cat((means, maximum), dim=-1)


class RelationalEntityMlpExtractorV5(nn.Module):
    """V5 critic: learned typed detail sets, never one all-detail average."""

    latent_dim_vf = 128

    def __init__(
        self,
        manifest: ObservationManifestV5,
        *,
        entity_width: int,
        depth: int,
        activation: str,
    ) -> None:
        super().__init__()
        self.manifest = manifest
        self.latent_dim_pi = manifest.layout.observation_size
        self.context = _mlp(
            manifest.layout.context.features,
            entity_width,
            entity_width,
            depth=depth,
            activation=activation,
        )
        self.entity_names = ("actors", "dice", "abilities", "statuses", "cards")
        self.entity_encoders = nn.ModuleDict(
            {
                name: _mlp(
                    getattr(manifest.layout, name).features,
                    entity_width,
                    entity_width,
                    depth=depth,
                    activation=activation,
                )
                for name in self.entity_names
            }
        )
        self.detail_encoder = _mlp(
            manifest.layout.details.features,
            entity_width,
            entity_width,
            depth=depth,
            activation=activation,
        )
        self.detail_gate = nn.Linear(entity_width, 1)
        # context + five typed mean/max sets + seven V5 detail scopes
        state_width = entity_width * (1 + 2 * len(self.entity_names) + 2 * 7)
        self.value_network = _mlp(
            state_width,
            entity_width * 2,
            self.latent_dim_vf,
            depth=depth,
            activation=activation,
        )

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, self.forward_critic(features)

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        context_range = self.manifest.layout.context
        context = features[:, context_range.offset : context_range.end]
        pooled = [self.context(context)]
        for name in self.entity_names:
            entity_range = getattr(self.manifest.layout, name)
            rows = features[:, entity_range.offset : entity_range.end].reshape(
                -1, entity_range.rows, entity_range.features
            )
            pooled.append(_masked_mean_max(self.entity_encoders[name](rows), rows[:, :, 0] > 0))
        details_range = self.manifest.layout.details
        details = features[:, details_range.offset : details_range.end].reshape(
            -1, details_range.rows, details_range.features
        )
        encoded = self.detail_encoder(details)
        valid = details[:, :, 0] > 0
        scope = details[:, :, 1:8].argmax(dim=-1)
        pooled.append(_grouped_weighted_max(encoded, scope, valid, 7, self.detail_gate(encoded)).flatten(1))
        return self.value_network(torch.cat(pooled, dim=-1))


class RelationalCandidateActionNetV5(nn.Module):
    """Shared V5 actor/critic backbone with candidate-isolated detail attention."""

    value_latent_width = 128

    def __init__(
        self,
        manifest: ObservationManifestV5,
        *,
        entity_width: int,
        decision_width: int,
        entity_depth: int,
        decision_depth: int,
        activation: str,
    ) -> None:
        super().__init__()
        self.manifest = manifest
        self.entity_width = entity_width
        self.decision_width = decision_width
        self.context = _mlp(
            manifest.layout.context.features,
            entity_width,
            entity_width,
            depth=entity_depth,
            activation=activation,
        )
        self.entity_names = ("actors", "dice", "abilities", "statuses", "cards")
        self.entity_encoders = nn.ModuleDict(
            {
                name: _mlp(
                    getattr(manifest.layout, name).features,
                    entity_width,
                    entity_width,
                    depth=entity_depth,
                    activation=activation,
                )
                for name in self.entity_names
            }
        )
        self.detail_encoder = _mlp(
            manifest.layout.details.features,
            entity_width,
            entity_width,
            depth=entity_depth,
            activation=activation,
        )
        self.global_detail_gate = nn.Linear(entity_width, 1)
        self.candidate = _mlp(
            manifest.layout.candidates.features,
            entity_width,
            entity_width,
            depth=entity_depth,
            activation=activation,
        )
        self.detail_key = nn.Linear(entity_width, entity_width)
        self.candidate_query = nn.Linear(entity_width, entity_width)
        self.candidate_detail_gate = nn.Linear(entity_width, 1)
        # context + five typed mean/max sets + six non-candidate detail scopes
        state_width = entity_width * (1 + 2 * len(self.entity_names) + 2 * 6)
        self.score = _mlp(
            state_width + entity_width * 3,
            decision_width,
            1,
            depth=decision_depth,
            activation=activation,
        )
        # Candidate-set mean/max (2w) plus local-detail mean/max (4w).
        self.value_network = _mlp(
            state_width + entity_width * 6,
            decision_width,
            self.value_latent_width,
            depth=decision_depth,
            activation=activation,
        )

    def _encoded_candidates(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        candidate_range = self.manifest.layout.candidates
        raw = features[:, candidate_range.offset : candidate_range.end].reshape(
            -1, candidate_range.rows, candidate_range.features
        )
        batch_indices, row_indices = (raw[:, :, 0] > 0).nonzero(as_tuple=True)
        occupied = self.candidate(raw[batch_indices, row_indices])
        encoded = raw.new_zeros((raw.shape[0], raw.shape[1], self.entity_width)).index_put(
            (batch_indices, row_indices), occupied
        )
        return raw, encoded

    def _encoded_details(
        self, features: torch.Tensor
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor, torch.Tensor]:
        detail_range = self.manifest.layout.details
        details = features[:, detail_range.offset : detail_range.end].reshape(
            -1, detail_range.rows, detail_range.features
        )
        batch_indices, row_indices = (details[:, :, 0] > 0).nonzero(as_tuple=True)
        encoded = self.detail_encoder(details[batch_indices, row_indices])
        return details, batch_indices, row_indices, encoded

    def _global_state(
        self,
        features: torch.Tensor,
        details: torch.Tensor,
        batch_indices: torch.Tensor,
        row_indices: torch.Tensor,
        encoded_details: torch.Tensor,
    ) -> torch.Tensor:
        context_range = self.manifest.layout.context
        context = features[:, context_range.offset : context_range.end]
        pooled = [self.context(context)]
        for name in self.entity_names:
            entity_range = getattr(self.manifest.layout, name)
            rows = features[:, entity_range.offset : entity_range.end].reshape(
                -1, entity_range.rows, entity_range.features
            )
            pooled.append(_encoded_mean_max_sparse_rows(rows, self.entity_encoders[name]))
        occupied = details[batch_indices, row_indices]
        global_mask = occupied[:, 11] < 0.5
        global_encoded = encoded_details[global_mask]
        scope = occupied[global_mask, 1:7].argmax(dim=-1)
        groups = batch_indices[global_mask] * 6 + scope
        global_details = _grouped_weighted_max_sparse(
            global_encoded,
            groups,
            features.shape[0] * 6,
            self.global_detail_gate(global_encoded),
        ).reshape(features.shape[0], 6, -1)
        pooled.append(global_details.flatten(1))
        return torch.cat(pooled, dim=-1)

    def _candidate_details(
        self,
        details: torch.Tensor,
        batch_indices: torch.Tensor,
        row_indices: torch.Tensor,
        encoded_details: torch.Tensor,
        candidates: torch.Tensor,
    ) -> torch.Tensor:
        capacity = self.manifest.maximum_legal_candidates
        occupied = details[batch_indices, row_indices]
        candidate_indices = torch.round(occupied[:, 82] * capacity).to(torch.long) - 1
        valid = (occupied[:, 11] > 0.5) & (candidate_indices >= 0) & (candidate_indices < capacity)
        local_encoded = encoded_details[valid]
        local_batches = batch_indices[valid]
        local_candidates = candidate_indices[valid]
        candidate_for_row = candidates[local_batches, local_candidates]
        attention = self.candidate_detail_gate(
            torch.tanh(self.detail_key(local_encoded) + self.candidate_query(candidate_for_row))
        )
        groups = local_batches * capacity + local_candidates
        return _grouped_weighted_max_sparse(
            local_encoded,
            groups,
            details.shape[0] * capacity,
            attention,
        ).reshape(details.shape[0], capacity, -1)

    def _components(
        self, features: torch.Tensor
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor, torch.Tensor]:
        raw_candidates, candidates = self._encoded_candidates(features)
        details, batch_indices, row_indices, encoded_details = self._encoded_details(features)
        state = self._global_state(
            features,
            details,
            batch_indices,
            row_indices,
            encoded_details,
        )
        local = self._candidate_details(
            details,
            batch_indices,
            row_indices,
            encoded_details,
            candidates,
        )
        return state, raw_candidates, candidates, local

    def _value_latent(
        self,
        state: torch.Tensor,
        raw_candidates: torch.Tensor,
        candidates: torch.Tensor,
        local: torch.Tensor,
    ) -> torch.Tensor:
        valid = raw_candidates[:, :, 0] > 0
        return self.value_network(
            torch.cat(
                (
                    state,
                    _masked_mean_max(candidates, valid),
                    _masked_mean_max(local, valid),
                ),
                dim=-1,
            )
        )

    def forward(self, features: torch.Tensor) -> torch.Tensor:
        state, _, candidates, local = self._components(features)
        expanded = state.unsqueeze(1).expand(-1, candidates.shape[1], -1)
        return self.score(torch.cat((expanded, candidates, local), dim=-1)).squeeze(-1)

    def forward_value(self, features: torch.Tensor) -> torch.Tensor:
        state, raw_candidates, candidates, local = self._components(features)
        return self._value_latent(state, raw_candidates, candidates, local)

    def forward_sparse(
        self,
        features: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor,
    ) -> torch.Tensor:
        masks = torch.as_tensor(action_masks, dtype=torch.bool, device=features.device)
        if masks.ndim == 1:
            masks = masks.unsqueeze(0)
        rows, columns = masks.nonzero(as_tuple=True)
        state, _, candidates, local = self._components(features)
        scores = self.score(
            torch.cat((state[rows], candidates[rows, columns], local[rows, columns]), dim=-1)
        ).squeeze(-1)
        return features.new_zeros((features.shape[0], self.manifest.maximum_legal_candidates)).index_put(
            (rows, columns), scores
        )

    def forward_sparse_with_value(
        self,
        features: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        masks = torch.as_tensor(action_masks, dtype=torch.bool, device=features.device)
        if masks.ndim == 1:
            masks = masks.unsqueeze(0)
        capacity = self.manifest.maximum_legal_candidates
        if masks.shape != (features.shape[0], capacity):
            raise ValueError(f"expected v5 action masks {(features.shape[0], capacity)}, got {masks.shape}")
        rows, columns = masks.nonzero(as_tuple=True)
        if rows.numel() == 0:
            raise ValueError("every sparse v5 actor batch must contain a valid candidate")
        state, raw_candidates, candidates, local = self._components(features)
        scores = self.score(
            torch.cat((state[rows], candidates[rows, columns], local[rows, columns]), dim=-1)
        ).squeeze(-1)
        logits = features.new_zeros((features.shape[0], capacity)).index_put((rows, columns), scores)
        value_latent = self._value_latent(state, raw_candidates, candidates, local)
        return logits, value_latent


class PassthroughMlpExtractorV5(nn.Module):
    def __init__(self, observation_size: int) -> None:
        super().__init__()
        self.latent_dim_pi = observation_size
        self.latent_dim_vf = RelationalCandidateActionNetV5.value_latent_width

    def forward(self, features: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return features, features

    def forward_actor(self, features: torch.Tensor) -> torch.Tensor:
        return features

    def forward_critic(self, features: torch.Tensor) -> torch.Tensor:
        return features


class SparseRelationalCandidateMaskablePolicyV5(SparseEntityCandidateMaskablePolicyV3):
    """Version-separated, candidate-aware V5 policy."""

    def __init__(
        self,
        *args: Any,
        manifest_path: str,
        entity_width: int = 96,
        decision_width: int = 432,
        entity_depth: int = 2,
        decision_depth: int = 2,
        activation: str = "tanh",
        **kwargs: Any,
    ) -> None:
        self.manifest_path = str(manifest_path)
        self.manifest = ObservationManifestV5.load(Path(self.manifest_path))
        self.entity_width = int(entity_width)
        self.decision_width = int(decision_width)
        self.entity_depth = int(entity_depth)
        self.decision_depth = int(decision_depth)
        self.entity_activation = activation
        kwargs["ortho_init"] = False
        kwargs["net_arch"] = []
        super(SparseEntityCandidateMaskablePolicyV3, self).__init__(*args, **kwargs)
        if tuple(self.observation_space.shape or ()) != (self.manifest.layout.observation_size,):
            raise RuntimeError("v5 policy observation space does not match frozen manifest")
        if int(self.action_space.n) != self.manifest.maximum_legal_candidates:
            raise RuntimeError("v5 policy action space does not match frozen manifest")

    def _build_mlp_extractor(self) -> None:
        self.mlp_extractor = PassthroughMlpExtractorV5(self.manifest.layout.observation_size).to(self.device)

    def _build(self, lr_schedule: Any) -> None:
        self._build_mlp_extractor()
        self.action_net = RelationalCandidateActionNetV5(
            self.manifest,
            entity_width=self.entity_width,
            decision_width=self.decision_width,
            entity_depth=self.entity_depth,
            decision_depth=self.decision_depth,
            activation=self.entity_activation,
        ).to(self.device)
        self.value_net = nn.Linear(self.mlp_extractor.latent_dim_vf, 1).to(self.device)
        self.optimizer = self.optimizer_class(self.parameters(), lr=lr_schedule(1), **self.optimizer_kwargs)

    def _resolved_masks(
        self,
        features: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None,
    ) -> torch.Tensor:
        if action_masks is not None:
            masks = torch.as_tensor(action_masks, dtype=torch.bool, device=features.device)
            return masks.unsqueeze(0) if masks.ndim == 1 else masks
        candidate_range = self.manifest.layout.candidates
        candidates = features[:, candidate_range.offset : candidate_range.end].reshape(
            -1, candidate_range.rows, candidate_range.features
        )
        return candidates[:, :, 0] > 0

    def _distribution_from_logits(self, logits: torch.Tensor, masks: torch.Tensor) -> Any:
        distribution = self.action_dist.proba_distribution(action_logits=logits)
        distribution.apply_masking(masks)
        return distribution

    def forward(
        self,
        obs: torch.Tensor,
        deterministic: bool = False,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        features = self.extract_features(obs)
        masks = self._resolved_masks(features, action_masks)
        logits, latent_vf = self.action_net.forward_sparse_with_value(features, masks)
        distribution = self._distribution_from_logits(logits, masks)
        actions = distribution.get_actions(deterministic=deterministic)
        log_prob = distribution.log_prob(actions)
        return actions.reshape((-1, *self.action_space.shape)), self.value_net(latent_vf), log_prob

    def evaluate_actions(
        self,
        obs: torch.Tensor,
        actions: torch.Tensor,
        action_masks: torch.Tensor | None = None,
    ) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor | None]:
        features = self.extract_features(obs)
        masks = self._resolved_masks(features, action_masks)
        logits, latent_vf = self.action_net.forward_sparse_with_value(features, masks)
        distribution = self._distribution_from_logits(logits, masks)
        return self.value_net(latent_vf), distribution.log_prob(actions), distribution.entropy()

    def get_distribution(
        self,
        obs: torch.Tensor,
        action_masks: np.ndarray | torch.Tensor | None = None,
    ) -> Any:
        features = self.extract_features(obs, self.pi_features_extractor)
        masks = self._resolved_masks(features, action_masks)
        return self._distribution_from_logits(self.action_net.forward_sparse(features, masks), masks)

    def predict_values(self, obs: torch.Tensor) -> torch.Tensor:
        features = self.extract_features(obs, self.vf_features_extractor)
        return self.value_net(self.action_net.forward_value(features))
