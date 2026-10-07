package engine

import "diceanddestiny/server/internal/battle/state"

func hasDividendStatus(b *state.Battle, actor string, reward state.DividendState) bool {
	for _, status := range b.Actors[actor].Statuses {
		if status.DefinitionID == dividendStatus(reward) && status.InstanceID == reward.StatusInstance && status.Stacks > 0 {
			return true
		}
	}
	return false
}

func rewardBlackDividend(b *state.Battle, actor string, die state.RolledDie, stream string) {
	c := curseRuntime(b)
	reward, ok := c.Dividends[actor]
	if !ok {
		return
	}
	if !hasDividendStatus(b, actor, reward) {
		delete(c.Dividends, actor)
		return
	}
	if reward.LastRewardRound == b.Segment.Round || reward.Rewards >= dividendLimit(reward) {
		return
	}
	before := b.Actors[reward.Source].Resources.EnergyPoints
	gainEnergy(b, reward.Source, dividendEnergy(reward))
	reward.LastRewardRound = b.Segment.Round
	reward.Rewards++
	c.Dividends[actor] = reward
	consumed := reward.Rewards == dividendLimit(reward)
	if consumed {
		removeStatus(b, actor, dividendStatus(reward), 0)
		delete(c.Dividends, actor)
	}
	data := map[string]any{"kind": "dividend_trigger", "source_card_id": rememberedMechanicID(b, actor, "black_dividend"), "source_actor_id": reward.Source, "die": die, "energy_before": before, "energy_after": b.Actors[reward.Source].Resources.EnergyPoints, "rewards": reward.Rewards, "limit": dividendLimit(reward), "consumed": consumed}
	// A reward must not reveal a private offensive face before joint reveal.
	if stream == "combat_dice" && state.SettledPlanningPrivate(*b) {
		c.PendingDividend = append(c.PendingDividend, state.CurseLog{Actor: actor, Round: b.Segment.Round, Segment: string(b.Segment.Current), Data: data})
	} else {
		curseLog(b, actor, "dividend_trigger", data)
	}
}

func expireBlackDividends(b *state.Battle) {
	c := curseRuntime(b)
	for _, actor := range sortedSettledActorIDs(b) {
		reward, ok := c.Dividends[actor]
		if !ok {
			continue
		}
		if !hasDividendStatus(b, actor, reward) {
			delete(c.Dividends, actor)
			continue
		}
		if configuredExpiration(b, actor, "black_dividend") || reward.ExpiresEffects > b.Segment.Round {
			continue
		}
		removeStatus(b, actor, dividendStatus(reward), 0)
		delete(c.Dividends, actor)
		curseLog(b, actor, "dividend_expired", map[string]any{"source_actor_id": reward.Source, "rewards": reward.Rewards})
	}
}

func dividendStatus(r state.DividendState) string {
	if r.StatusID != "" {
		return r.StatusID
	}
	return "black_dividend"
}
func dividendLimit(r state.DividendState) int {
	if r.Limit > 0 {
		return r.Limit
	}
	return 2
}
func dividendEnergy(r state.DividendState) int {
	if r.Energy > 0 {
		return r.Energy
	}
	return 1
}
