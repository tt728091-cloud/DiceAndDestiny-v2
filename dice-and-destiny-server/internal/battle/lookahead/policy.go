package lookahead

import (
	"math/rand"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
)

// Decision is one choice a player must make. Battle is the full state: the
// look-ahead player only uses it through Fork, which hides future randomness
// and the opponent's hand; the baselines read the same public facts a human
// sees.
type Decision struct {
	Sim     *Simulator
	Battle  *state.Battle
	Seat    string
	Actions []command.Command
	Rand    *rand.Rand
	view    *engine.Result
}

// View is the acting seat's viewer-safe result.
func (d *Decision) View() engine.Result {
	if d.view == nil {
		view := d.Sim.View(d.Battle, d.Seat, d.Actions)
		d.view = &view
	}
	return *d.view
}

// Policy picks the index of one listed command.
type Policy interface {
	Name() string
	Choose(d *Decision) (int, error)
}

// RandomPolicy picks uniformly among the listed commands that change
// something (see Useful).
type RandomPolicy struct{}

func (RandomPolicy) Name() string { return "random" }

func (RandomPolicy) Choose(d *Decision) (int, error) {
	useful := Useful(d.Battle, d.Seat, d.Actions)
	return useful[d.Rand.Intn(len(useful))], nil
}

// TransitionPolicy is a policy written against the ML environment's
// transition, such as the content-configured scripted minions.
type TransitionPolicy interface {
	Select(mlsim.Transition) (int, time.Duration, error)
}

// Scripted adapts a TransitionPolicy to a Policy.
type Scripted struct {
	Label  string
	Policy TransitionPolicy
}

func (p Scripted) Name() string { return p.Label }

func (p Scripted) Choose(d *Decision) (int, error) {
	index, _, err := p.Policy.Select(mlsim.Transition{ActorID: d.Seat, Result: d.View()})
	return index, err
}
