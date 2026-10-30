package tactics

import "fmt"

// BudgetInput is what a movement plan commits.
type BudgetInput struct {
	// BaseSpeed is the creature's free Move per round.
	BaseSpeed int
	// Cards is how many cards are discarded for Move.
	Cards int
	// Energy is the energy spent for Move (a multiple of energy_per_move).
	Energy int
	// MapAmounts are the printed amounts of the map Move cards played.
	MapAmounts []int
	// Sprint is set when the creature skipped its offense this round.
	Sprint bool
	// ChargeRange is a Charge's built-in reach (0: not charging).
	ChargeRange int
}

// MoveBudget is a plan's Move this round and where it came from.
type MoveBudget struct {
	Base    int  `json:"base"`
	Cards   int  `json:"cards"`
	Energy  int  `json:"energy"`
	Map     int  `json:"map"`
	Sprint  int  `json:"sprint_bonus"`
	Charge  int  `json:"charge"`
	Total   int  `json:"total"`
	Paid    int  `json:"paid"`
	Running bool `json:"running"`
}

// ComputeBudget applies the rules: base speed free (× sprint_base_multiplier
// when sprinting), +card_move per card, +1 per energy_per_move energy, each
// map card's amount × map_card_move_multiplier, and +sprint_card_bonus per
// committed card while sprinting. A Charge moves exactly its reach, plus
// cards and energy only when charge_allows_extra_move. A plan runs when its
// Move exceeds base speed; a Sprint always runs; a Charge runs only when
// charge_counts_as_running.
func ComputeBudget(r Rules, in BudgetInput) (MoveBudget, error) {
	if in.Energy < 0 || in.Energy%r.EnergyPerMove != 0 {
		return MoveBudget{}, fmt.Errorf("energy for Move comes in steps of %d", r.EnergyPerMove)
	}
	committed := in.Cards + len(in.MapAmounts)
	if in.ChargeRange > 0 {
		if in.Sprint {
			return MoveBudget{}, fmt.Errorf("a sprint cannot charge")
		}
		if !r.ChargeAllowsExtraMove && (committed > 0 || in.Energy > 0) {
			return MoveBudget{}, fmt.Errorf("a charge cannot add cards or energy")
		}
	}
	b := MoveBudget{Base: in.BaseSpeed}
	if in.Sprint {
		b.Base = in.BaseSpeed * r.SprintBaseMultiplier
		b.Sprint = committed * r.SprintCardBonus
	}
	if in.ChargeRange > 0 {
		b.Base, b.Charge = 0, in.ChargeRange
	}
	b.Cards = in.Cards * r.CardMove
	b.Energy = in.Energy / r.EnergyPerMove
	for _, amount := range in.MapAmounts {
		if amount < 0 {
			return MoveBudget{}, fmt.Errorf("a map card's Move is never negative")
		}
		b.Map += amount * r.MapCardMoveMultiplier
	}
	b.Total = b.Base + b.Charge + b.Cards + b.Energy + b.Map + b.Sprint
	b.Paid = b.Cards + b.Energy + b.Map + b.Sprint
	switch {
	case in.ChargeRange > 0:
		b.Running = r.ChargeCountsAsRunning
	case in.Sprint:
		b.Running = true
	default:
		b.Running = b.Total > in.BaseSpeed
	}
	return b, nil
}
