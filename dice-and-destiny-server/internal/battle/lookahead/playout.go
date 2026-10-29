package lookahead

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

// errDeadline reports a playout stopped by its deadline; its value is
// unknown and must be dropped, not scored.
var errDeadline = errors.New("playout deadline passed")

// Playout plays a forked battle forward with the given policies until it
// ends or the horizon is reached, and returns the result from the hero's
// side: +1 win, -1 loss, 0 draw, or Evaluate's estimate at the horizon.
type Playout struct {
	Sim      *Simulator
	Hero     string
	Policies map[string]Policy
	// Rounds stops the playout once the battle reaches this many rounds past
	// its starting round; zero plays to the end.
	Rounds   int
	MaxSteps int
	Evaluate func(b *state.Battle, hero string) float64
	// Judge, when set, scores the horizon instead of Evaluate, at the first
	// offensive-planning start once Rounds have passed: the positions a
	// value network learns from.
	Judge Evaluator
	// Deadline, when set, abandons the playout once it passes (errDeadline).
	Deadline time.Time
}

// Evaluator judges a position from the hero's side: a value on the scale of
// a finished battle (+1 win, -1 loss) and the expected final health margin
// (the hero's health share minus the strongest opponent's). acting is the
// seat the authority is waiting on.
type Evaluator interface {
	Evaluate(b *state.Battle, hero, acting string) (value, margin float64)
}

// PlanningStart reports the first moment of a round's offensive planning:
// nobody has rolled yet.
func PlanningStart(b *state.Battle) bool {
	if b.Settled == nil || b.Settled.Window == nil || b.Segment.Current != "offensive" || b.Settled.Window.Stage != "planning" {
		return false
	}
	for _, rt := range b.Settled.Actors {
		if len(rt.FinalDice) > 0 || rt.RollsUsed > 0 {
			return false
		}
	}
	return true
}

// Run applies first (if any) to b and plays on. b is consumed.
func (p Playout) Run(b *state.Battle, first *command.Command, rng *rand.Rand) (float64, error) {
	value, _, _, err := p.Play(b, first, rng)
	return value, err
}

// Play is Run that also reports whether Judge scored the horizon, and if so
// the margin it expects.
func (p Playout) Play(b *state.Battle, first *command.Command, rng *rand.Rand) (value float64, judged bool, margin float64, err error) {
	if first != nil {
		if err := p.Sim.Apply(b, *first); err != nil {
			return 0, false, 0, err
		}
	}
	start := b.Segment.Round
	limit := p.MaxSteps
	if limit == 0 {
		limit = 2000
	}
	for steps := 0; ; steps++ {
		if Terminal(b) {
			return Outcome(b, p.Hero), false, 0, nil
		}
		past := p.Rounds > 0 && b.Segment.Round >= start+p.Rounds
		if past && p.Judge == nil {
			return p.Evaluate(b, p.Hero), false, 0, nil
		}
		if steps >= limit {
			return 0, false, 0, fmt.Errorf("playout exceeded %d steps", limit)
		}
		if !p.Deadline.IsZero() && time.Now().After(p.Deadline) {
			return 0, false, 0, errDeadline
		}
		seat, actions := p.Sim.Next(b)
		if len(actions) == 0 {
			return 0, false, 0, fmt.Errorf("playout stalled at round %d", b.Segment.Round)
		}
		if past && PlanningStart(b) {
			value, margin := p.Judge.Evaluate(b, p.Hero, seat)
			return value, true, margin, nil
		}
		policy := p.Policies[seat]
		if policy == nil {
			return 0, false, 0, fmt.Errorf("no playout policy for %s", seat)
		}
		d := &Decision{Sim: p.Sim, Battle: b, Seat: seat, Actions: actions, Rand: rng}
		index, err := policy.Choose(d)
		if err != nil {
			return 0, false, 0, err
		}
		if err := p.Sim.Apply(b, actions[index]); err != nil {
			return 0, false, 0, err
		}
	}
}

// Outcome scores a finished battle for the hero.
func Outcome(b *state.Battle, hero string) float64 {
	switch b.WinnerActorID {
	case hero:
		return 1
	case "":
		return 0
	}
	return -1
}
