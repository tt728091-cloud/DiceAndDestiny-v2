package lookahead

import (
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/content"
)

// Rates are the exchange rates the AI uses to turn riders into damage. One
// value feeds the greedy player, its dice solver and the ability tier values,
// so they can never disagree.
//
// Board pricing has its own rates (Protect 1 per stack, energy 0.2 per point,
// agreed 2026-10-15). Once the board work's riders.yaml merges, these move
// there so the AI and pricing read the same place.
type Rates struct {
	// Status is what one stack of a status an ability gives its own user is
	// worth; StatusByID overrides it per status definition (Protect).
	Status     float64
	StatusByID map[string]float64
	// EnergyGain is what one energy point gained is worth, from an ability
	// tier or a card; EnergySpend is what one energy point spent on a card
	// costs.
	EnergyGain  float64
	EnergySpend float64
}

// DefaultRates are the rates the version-2 players were tuned with.
func DefaultRates() Rates {
	return Rates{Status: 0.5, EnergyGain: 0.1, EnergySpend: 0.1}
}

// StatusValue is what one stack of the status is worth.
func (r Rates) StatusValue(statusID string) float64 {
	if v, ok := r.StatusByID[statusID]; ok {
		return v
	}
	return r.Status
}

// WithStatus returns a copy with one status's value overridden.
func (r Rates) WithStatus(statusID string, value float64) Rates {
	byID := map[string]float64{}
	for k, v := range r.StatusByID {
		byID[k] = v
	}
	byID[statusID] = value
	r.StatusByID = byID
	return r
}

// TierValue scores an ability tier's operations: damage counts in full, a
// status the actor gives itself its status rate per stack, a resource gained
// the energy-gain rate per point.
func (r Rates) TierValue(ops []content.BattleOperation) float64 {
	value := 0.0
	for _, op := range ops {
		switch op.Type {
		case "deal_damage":
			value += float64(amount(op.Amount))
		case "apply_status":
			if op.Target == "self" {
				value += r.StatusValue(op.StatusID) * float64(max(op.StackCount, 1))
			}
		case "gain_resource":
			value += r.EnergyGain * float64(amount(op.Amount))
		}
	}
	return value
}

// String renders the rates for reports and solver table keys.
func (r Rates) String() string {
	text := fmt.Sprintf("status=%g energy-gain=%g energy-spend=%g", r.Status, r.EnergyGain, r.EnergySpend)
	var ids []string
	for id := range r.StatusByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var parts []string
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%s=%g", id, r.StatusByID[id]))
	}
	if len(parts) > 0 {
		text += " " + strings.Join(parts, " ")
	}
	return text
}
