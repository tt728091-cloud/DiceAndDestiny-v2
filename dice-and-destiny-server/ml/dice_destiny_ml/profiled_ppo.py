from __future__ import annotations

import numpy as np
import torch as th
import torch.nn.functional as F
from gymnasium import spaces
from sb3_contrib import MaskablePPO
from stable_baselines3.common.utils import explained_variance

from .profiling import ProfileCollector


class ProfiledMaskablePPO(MaskablePPO):
    """Maskable PPO with the upstream update recipe plus opt-in nested timings."""

    def __init__(self, *args: object, **kwargs: object) -> None:
        super().__init__(*args, **kwargs)
        self.update_profile = ProfileCollector(True)
        enable = getattr(getattr(self, "policy", None), "enable_instrumentation", None)
        if enable is not None:
            enable(True)

    def train(self) -> None:  # noqa: C901 - intentionally mirrors upstream MaskablePPO
        self.policy.set_training_mode(True)
        self.update_profile.increment("ppo.train_calls")
        self.update_profile.increment("ppo.planned_epochs", self.n_epochs)
        self._update_learning_rate(self.policy.optimizer)
        clip_range = self.clip_range(self._current_progress_remaining)
        if self.clip_range_vf is not None:
            clip_range_vf = self.clip_range_vf(self._current_progress_remaining)

        entropy_losses = []
        pg_losses, value_losses = [], []
        clip_fractions = []
        gradient_norms = []
        continue_training = True

        for epoch in range(self.n_epochs):
            approx_kl_divs = []
            with self.update_profile.span("ppo.epoch"):
                for rollout_data in self.rollout_buffer.get(self.batch_size):
                    self.update_profile.increment("ppo.minibatch_updates")
                    with self.update_profile.span("ppo.minibatch"):
                        actions = rollout_data.actions
                        if isinstance(self.action_space, spaces.Discrete):
                            actions = rollout_data.actions.long().flatten()

                        with self.update_profile.span("ppo.evaluate_actions"):
                            values, log_prob, entropy = self.policy.evaluate_actions(
                                rollout_data.observations,
                                actions,
                                action_masks=rollout_data.action_masks,
                            )

                        with self.update_profile.span("ppo.loss_and_distribution_math"):
                            values = values.flatten()
                            advantages = rollout_data.advantages
                            if self.normalize_advantage:
                                advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)
                            ratio = th.exp(log_prob - rollout_data.old_log_prob)
                            policy_loss_1 = advantages * ratio
                            policy_loss_2 = advantages * th.clamp(ratio, 1 - clip_range, 1 + clip_range)
                            policy_loss = -th.min(policy_loss_1, policy_loss_2).mean()
                            pg_losses.append(policy_loss.item())
                            clip_fraction = th.mean((th.abs(ratio - 1) > clip_range).float()).item()
                            clip_fractions.append(clip_fraction)
                            self.update_profile.increment("ppo.clip_fraction_sum", clip_fraction)
                            self.update_profile.increment("ppo.clip_fraction_samples")
                            if self.clip_range_vf is None:
                                values_pred = values
                            else:
                                values_pred = rollout_data.old_values + th.clamp(
                                    values - rollout_data.old_values,
                                    -clip_range_vf,
                                    clip_range_vf,
                                )
                            value_loss = F.mse_loss(rollout_data.returns, values_pred)
                            value_losses.append(value_loss.item())
                            if entropy is None:
                                entropy_loss = -th.mean(-log_prob)
                            else:
                                entropy_loss = -th.mean(entropy)
                            entropy_losses.append(entropy_loss.item())
                            loss = policy_loss + self.ent_coef * entropy_loss + self.vf_coef * value_loss
                            with th.no_grad():
                                log_ratio = log_prob - rollout_data.old_log_prob
                                approx_kl_div = th.mean((th.exp(log_ratio) - 1) - log_ratio).cpu().numpy()
                                approx_kl_divs.append(approx_kl_div)
                                self.update_profile.increment("ppo.approx_kl_sum", float(approx_kl_div))
                                self.update_profile.increment("ppo.approx_kl_samples")

                        if self.target_kl is not None and approx_kl_div > 1.5 * self.target_kl:
                            continue_training = False
                            self.update_profile.increment("ppo.target_kl_early_stops")
                            if self.verbose >= 1:
                                print(
                                    f"Early stopping at step {epoch} due to reaching "
                                    f"max kl: {approx_kl_div:.2f}"
                                )
                            break

                        self.policy.optimizer.zero_grad()
                        with self.update_profile.span("ppo.backward"):
                            loss.backward()
                        with self.update_profile.span("ppo.gradient_clip"):
                            gradient_norm = float(
                                th.nn.utils.clip_grad_norm_(self.policy.parameters(), self.max_grad_norm)
                                .detach()
                                .cpu()
                            )
                            gradient_norms.append(gradient_norm)
                            self.update_profile.increment("ppo.gradient_norm_sum", gradient_norm)
                            self.update_profile.increment("ppo.gradient_norm_samples")
                            if gradient_norm > self.max_grad_norm:
                                self.update_profile.increment("ppo.gradient_clip_events")
                        with self.update_profile.span("ppo.optimizer_step"):
                            self.policy.optimizer.step()
                        self.update_profile.increment("ppo.optimizer_updates")

            self._n_updates += 1
            self.update_profile.increment("ppo.epochs_completed")
            if not continue_training:
                break

        explained_var = explained_variance(
            self.rollout_buffer.values.flatten(), self.rollout_buffer.returns.flatten()
        )
        self.logger.record("train/entropy_loss", np.mean(entropy_losses))
        self.logger.record("train/policy_gradient_loss", np.mean(pg_losses))
        self.logger.record("train/value_loss", np.mean(value_losses))
        self.logger.record("train/approx_kl", np.mean(approx_kl_divs))
        self.logger.record("train/clip_fraction", np.mean(clip_fractions))
        self.logger.record("train/gradient_norm", np.mean(gradient_norms))
        self.logger.record("train/loss", loss.item())
        self.logger.record("train/explained_variance", explained_var)
        self.logger.record("train/n_updates", self._n_updates, exclude="tensorboard")
        self.logger.record("train/clip_range", clip_range)
        if self.clip_range_vf is not None:
            self.logger.record("train/clip_range_vf", clip_range_vf)
