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
	// Deadline, when set, abandons the playout once it passes (errDeadline).
	Deadline time.Time
}

// Run applies first (if any) to b and plays on. b is consumed.
func (p Playout) Run(b *state.Battle, first *command.Command, rng *rand.Rand) (float64, error) {
	if first != nil {
		if err := p.Sim.Apply(b, *first); err != nil {
			return 0, err
		}
	}
	start := b.Segment.Round
	limit := p.MaxSteps
	if limit == 0 {
		limit = 2000
	}
	for steps := 0; ; steps++ {
		if Terminal(b) {
			return Outcome(b, p.Hero), nil
		}
		if p.Rounds > 0 && b.Segment.Round >= start+p.Rounds {
			return p.Evaluate(b, p.Hero), nil
		}
		if steps >= limit {
			return 0, fmt.Errorf("playout exceeded %d steps", limit)
		}
		if !p.Deadline.IsZero() && time.Now().After(p.Deadline) {
			return 0, errDeadline
		}
		seat, actions := p.Sim.Next(b)
		if len(actions) == 0 {
			return 0, fmt.Errorf("playout stalled at round %d", b.Segment.Round)
		}
		policy := p.Policies[seat]
		if policy == nil {
			return 0, fmt.Errorf("no playout policy for %s", seat)
		}
		d := &Decision{Sim: p.Sim, Battle: b, Seat: seat, Actions: actions, Rand: rng}
		index, err := policy.Choose(d)
		if err != nil {
			return 0, err
		}
		if err := p.Sim.Apply(b, actions[index]); err != nil {
			return 0, err
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
