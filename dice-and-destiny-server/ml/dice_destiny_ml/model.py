from __future__ import annotations

from typing import Any

import numpy as np
import torch
from sb3_contrib.common.maskable.policies import MaskableActorCriticPolicy
from torch import nn

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
        candidates = features[:, BASE_FEATURES_V2:].reshape(
            -1, MAX_ACTIONS_V2, ACTION_FEATURES_V2
        )
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
        candidates = features[:, BASE_FEATURES_V2:].reshape(
            -1, MAX_ACTIONS_V2, ACTION_FEATURES_V2
        )
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
        self.optimizer = self.optimizer_class(
            self.parameters(), lr=lr_schedule(1), **self.optimizer_kwargs
        )


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
