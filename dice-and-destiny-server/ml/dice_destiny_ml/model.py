from __future__ import annotations

from typing import Any

import torch
from sb3_contrib.common.maskable.policies import MaskableActorCriticPolicy
from torch import nn

from .schema import ACTION_FEATURES, BASE_FEATURES, MAX_ACTIONS, OBSERVATION_SIZE
from .schema_v2 import (
    ACTION_FEATURES_V2,
    BASE_FEATURES_V2,
    MAX_ACTIONS_V2,
    OBSERVATION_SIZE_V2,
)


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


class CandidateMaskablePolicy(MaskableActorCriticPolicy):
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


class CandidateMaskablePolicyV2(MaskableActorCriticPolicy):
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
