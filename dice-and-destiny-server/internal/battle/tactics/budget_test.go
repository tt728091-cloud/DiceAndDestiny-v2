package tactics

import "testing"

// §11.10 (rules side): Sprint multipliers, map Move cards, energy → Move.
func TestMoveBudgets(t *testing.T) {
	r := testRules()
	cases := []struct {
		name    string
		in      BudgetInput
		total   int
		paid    int
		running bool
	}{
		{"base only", BudgetInput{BaseSpeed: 1}, 1, 0, false},
		{"two cards", BudgetInput{BaseSpeed: 1, Cards: 2}, 3, 2, true},
		{"energy", BudgetInput{BaseSpeed: 1, Energy: 10}, 3, 2, true},
		{"map card", BudgetInput{BaseSpeed: 1, MapAmounts: []int{4}}, 5, 4, true},
		{"fast base", BudgetInput{BaseSpeed: 2}, 2, 0, false},
		{"sprint, nothing committed", BudgetInput{BaseSpeed: 1, Sprint: true}, 2, 0, true},
		{"sprint with two cards", BudgetInput{BaseSpeed: 1, Cards: 2, Sprint: true}, 6, 4, true},
		{"sprint with a map card", BudgetInput{BaseSpeed: 1, MapAmounts: []int{4}, Sprint: true}, 7, 5, true},
		{"sprint, energy gets no bonus", BudgetInput{BaseSpeed: 2, Energy: 5, Sprint: true}, 5, 1, true},
		{"charge", BudgetInput{BaseSpeed: 1, ChargeRange: 4}, 4, 0, false},
	}
	for _, c := range cases {
		b, err := ComputeBudget(r, c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if b.Total != c.total || b.Paid != c.paid || b.Running != c.running {
			t.Errorf("%s: total %d paid %d running %v, want %d %d %v (%+v)", c.name, b.Total, b.Paid, b.Running, c.total, c.paid, c.running, b)
		}
	}
	if _, err := ComputeBudget(r, BudgetInput{BaseSpeed: 1, Energy: 7}); err == nil {
		t.Error("energy outside 5-steps was accepted")
	}
	if _, err := ComputeBudget(r, BudgetInput{BaseSpeed: 1, Cards: 1, ChargeRange: 4}); err == nil {
		t.Error("a charge added a card with charge_allows_extra_move off")
	}
	if _, err := ComputeBudget(r, BudgetInput{BaseSpeed: 1, Sprint: true, ChargeRange: 4}); err == nil {
		t.Error("a sprint charged")
	}
	tuned := testRules()
	tuned.ChargeAllowsExtraMove, tuned.ChargeCountsAsRunning = true, true
	tuned.SprintBaseMultiplier, tuned.SprintCardBonus, tuned.MapCardMoveMultiplier, tuned.EnergyPerMove = 3, 2, 2, 10
	if b, _ := ComputeBudget(tuned, BudgetInput{BaseSpeed: 1, Cards: 1, ChargeRange: 4}); b.Total != 5 || !b.Running {
		t.Errorf("tuned charge = %+v", b)
	}
	if b, _ := ComputeBudget(tuned, BudgetInput{BaseSpeed: 1, Cards: 1, MapAmounts: []int{4}, Energy: 20, Sprint: true}); b.Total != 3+1+8+2+4 {
		t.Errorf("tuned sprint = %+v", b)
	}
}
