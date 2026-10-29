package lookahead_test

import (
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/lookahead"
)

// Ability tiers are valued at the rates: Guarded Strike is 5 damage plus 2
// Protect, Measured Strike 4 damage plus 5 energy. The defaults keep the
// version-2 values; the board-pricing rates (Protect 1, energy 0.2) raise
// both.
func TestRatesValueTiers(t *testing.T) {
	d := firstRollDecision(t)
	lib, err := engine.SettledLibrary(d.Battle)
	if err != nil {
		t.Fatal(err)
	}
	guarded := lib.Abilities["guarded_strike"].Qualification.ActivationTiers[0].Operations
	measured := lib.Abilities["measured_strike"].Qualification.ActivationTiers[0].Operations
	rates := lookahead.DefaultRates()
	if got := rates.TierValue(guarded); got != 6 {
		t.Fatalf("default Guarded Strike %v, want 6", got)
	}
	if got := rates.TierValue(measured); got != 4.5 {
		t.Fatalf("default Measured Strike %v, want 4.5", got)
	}
	pricing := rates.WithStatus("protect", 1)
	pricing.EnergyGain = 0.2
	if got := pricing.TierValue(guarded); got != 7 {
		t.Fatalf("Guarded Strike at Protect 1: %v, want 7", got)
	}
	if got := pricing.TierValue(measured); got != 5 {
		t.Fatalf("Measured Strike at energy 0.2: %v, want 5", got)
	}
	if rates.StatusByID != nil {
		t.Fatal("WithStatus changed the original rates")
	}
}
