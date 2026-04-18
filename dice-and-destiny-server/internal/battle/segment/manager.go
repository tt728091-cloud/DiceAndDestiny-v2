package segment

import "fmt"

type Segment string

const (
	OngoingEffects   Segment = "ongoing_effects"
	Income           Segment = "income"
	Offensive        Segment = "offensive"
	Defensive        Segment = "defensive"
	DamageResolution Segment = "damage_resolution"
)

var orderedSegments = []Segment{
	OngoingEffects,
	Income,
	Offensive,
	Defensive,
	DamageResolution,
}

type State struct {
	Current Segment
	Round   int
}

type Advance struct {
	From          Segment
	To            Segment
	Round         int
	CompletedTurn bool
}

type Manager struct{}

func NewManager() Manager {
	return Manager{}
}

func Order() []Segment {
	order := make([]Segment, len(orderedSegments))
	copy(order, orderedSegments)
	return order
}

func (m Manager) InitialState() State {
	return State{
		Current: OngoingEffects,
		Round:   1,
	}
}

func (m Manager) Advance(state State) (State, Advance, error) {
	if state.Round < 1 {
		return State{}, Advance{}, fmt.Errorf("invalid round %d", state.Round)
	}

	index := segmentIndex(state.Current)
	if index == -1 {
		return State{}, Advance{}, fmt.Errorf("unknown segment %q", state.Current)
	}

	nextIndex := index + 1
	nextRound := state.Round
	completedTurn := false
	if nextIndex == len(orderedSegments) {
		nextIndex = 0
		nextRound++
		completedTurn = true
	}

	next := State{
		Current: orderedSegments[nextIndex],
		Round:   nextRound,
	}

	return next, Advance{
		From:          state.Current,
		To:            next.Current,
		Round:         next.Round,
		CompletedTurn: completedTurn,
	}, nil
}

func (s Segment) String() string {
	return string(s)
}

func IsValid(s Segment) bool {
	return segmentIndex(s) != -1
}

func segmentIndex(segment Segment) int {
	for i, candidate := range orderedSegments {
		if candidate == segment {
			return i
		}
	}
	return -1
}
