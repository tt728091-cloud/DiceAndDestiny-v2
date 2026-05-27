package engine

import "diceanddestiny/server/internal/battle/income"

func DefaultFlows() []SegmentFlow {
	return []SegmentFlow{
		OngoingEffectsFlow{},
		mustIncomeFlow(DefaultIncomeRewards()...),
		OffensiveFlow{},
		DefensiveFlow{},
		DamageResolutionFlow{},
	}
}

func DefaultIncomeRewards() []income.Reward {
	return income.DefaultRewards()
}

func mustIncomeFlow(rewards ...income.Reward) IncomeFlow {
	flow, err := NewIncomeFlow(rewards...)
	if err != nil {
		panic(err)
	}

	return flow
}
