package lookahead

import (
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

// CardFilter plays like Inner but never plays the cards Blocked names (by
// card definition ID; a nil Blocked blocks every card). Their plays are
// removed from its choices, so those cards only count as health, like
// blank cards. Discarding to the hand limit is never filtered, and a
// decision that lists nothing else keeps all its choices.
type CardFilter struct {
	Inner   Policy
	Label   string
	Blocked func(definitionID string) bool
	// Fallbacks counts decisions that listed only blocked plays.
	Fallbacks int
}

func (f *CardFilter) Name() string { return f.Label }

func (f *CardFilter) Choose(d *Decision) (int, error) {
	if w := d.Battle.Settled.Window; w != nil && w.Stage == "discard_to_hand_limit" {
		return f.Inner.Choose(d)
	}
	instances := d.Battle.Settled.Actors[d.Seat].CardInstances
	var keep []int
	for i, action := range d.Actions {
		if !f.blocks(instances, action) {
			keep = append(keep, i)
		}
	}
	if len(keep) == len(d.Actions) {
		return f.Inner.Choose(d)
	}
	if len(keep) == 0 {
		f.Fallbacks++
		return f.Inner.Choose(d)
	}
	filtered := &Decision{Sim: d.Sim, Battle: d.Battle, Seat: d.Seat, Rand: d.Rand}
	for _, i := range keep {
		filtered.Actions = append(filtered.Actions, d.Actions[i])
	}
	k, err := f.Inner.Choose(filtered)
	if err != nil {
		return 0, err
	}
	if k < 0 || k >= len(keep) {
		return 0, fmt.Errorf("%s chose index %d of %d", f.Inner.Name(), k, len(keep))
	}
	return keep[k], nil
}

func (f *CardFilter) blocks(instances map[string]state.CardInstance, action command.Command) bool {
	for _, id := range CardIDs(action) {
		if f.Blocked == nil || f.Blocked(instances[id].DefinitionID) {
			return true
		}
	}
	return false
}

// Counts and FutureCount report the inner player's search statistics.
func (f *CardFilter) Counts() (searched, overrides int) {
	if c, ok := f.Inner.(counter); ok {
		return c.Counts()
	}
	return 0, 0
}

func (f *CardFilter) FutureCount() int {
	if c, ok := f.Inner.(futureCounter); ok {
		return c.FutureCount()
	}
	return 0
}
