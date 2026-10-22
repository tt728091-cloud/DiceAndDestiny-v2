package learned

import (
	"errors"
	"math/rand"
	"runtime"
	"time"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
)

// OpponentPolicyLookahead selects the balance lab's look-ahead player as the
// session opponent (a mirror of the human's character and deck).
const OpponentPolicyLookahead = "lookahead"

// LookaheadModelID names the look-ahead opponent in telemetry and the client.
const LookaheadModelID = "lookahead-mirror"

// LookaheadArchitecture names the look-ahead version in battle logs; logs from
// the first version say "flat-monte-carlo-greedy-anchor".
const LookaheadArchitecture = LookaheadArchitectureV2

// LookaheadArchitectureV2 is version 2's label (2026-10-15): futures played
// to the end of the battle.
const LookaheadArchitectureV2 = "flat-monte-carlo-greedy-anchor-v2"

// LookaheadVersion names the look-ahead version that logged a battle.
func LookaheadVersion(architecture string) string {
	if architecture == LookaheadArchitectureV2 {
		return "v2"
	}
	return "v1"
}

// lookaheadBudget and lookaheadRounds are the in-game search settings.
const (
	lookaheadBudget = 160
	lookaheadRounds = 0
)

// statePolicy is a session policy that needs the full battle state. It must
// only read it through lookahead.Fork, which reseeds randomness and re-deals
// the human's hidden hand.
type statePolicy interface {
	SelectWithState(mlsim.Transition, state.Battle) (int, time.Duration, error)
}

// lookaheadPolicy plays the opponent seat with lookahead.SearchPolicy: the
// greedy player's choice unless simulated futures show a clear improvement.
// The human is modelled by the greedy player.
type lookaheadPolicy struct {
	search *lookahead.SearchPolicy
	sim    *lookahead.Simulator
	rng    *rand.Rand
}

func newLookaheadPolicy() (*lookaheadPolicy, error) {
	sim, err := lookahead.NewSimulator()
	if err != nil {
		return nil, err
	}
	search := lookahead.NewSearchPolicy(func() lookahead.Policy { return lookahead.NewGreedyPolicy() }, lookaheadBudget)
	search.Rounds = lookaheadRounds
	search.Workers = max(1, runtime.NumCPU()-1)
	search.MaxThink = 5 * time.Second
	return &lookaheadPolicy{search: search, sim: sim, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}, nil
}

func (p *lookaheadPolicy) Metadata() PolicyMetadata {
	return PolicyMetadata{
		ModelID:   LookaheadModelID,
		Algorithm: "lookahead",
		// v2: the dice solver plans dice cards in hand and overrides are
		// re-tested on fresh futures. Logged battles name the version.
		Architecture:      LookaheadArchitecture,
		EnvironmentSchema: mlsim.EnvironmentSchemaVersion,
		ObservationSchema: mlsim.ObservationSchemaVersion,
		ActionSchema:      mlsim.ActionSchemaVersion,
	}
}

func (p *lookaheadPolicy) Select(mlsim.Transition) (int, time.Duration, error) {
	return 0, 0, errors.New("the look-ahead opponent needs the battle state")
}

func (p *lookaheadPolicy) SelectWithState(t mlsim.Transition, b state.Battle) (int, time.Duration, error) {
	started := time.Now()
	d := &lookahead.Decision{Sim: p.sim, Battle: &b, Seat: t.ActorID, Actions: t.Result.LegalActions, Rand: p.rng}
	index, err := p.search.Choose(d)
	return index, time.Since(started), err
}
