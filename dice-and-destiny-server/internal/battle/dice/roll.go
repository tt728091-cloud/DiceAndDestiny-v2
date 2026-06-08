package dice

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

var (
	ErrInvalidRoll       = errors.New("invalid dice roll")
	ErrMissingRollState  = errors.New("missing roll state")
	ErrMissingDiceState  = errors.New("missing actor dice state")
	ErrMissingDefinition = errors.New("missing dice definition")
)

type RandomSource interface {
	Intn(maxExclusive int) (int, error)
}

type RollOption func(*rollOptions)

type rollOptions struct {
	randomSource RandomSource
}

func WithRandomSource(source RandomSource) RollOption {
	return func(options *rollOptions) {
		options.randomSource = source
	}
}

type CryptoRandomSource struct{}

func (CryptoRandomSource) Intn(maxExclusive int) (int, error) {
	if maxExclusive <= 0 {
		return 0, fmt.Errorf("%w: random max must be positive", ErrInvalidRoll)
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxExclusive)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

type SequenceRandomSource struct {
	values []int
	next   int
}

func NewSequenceRandomSource(values ...int) *SequenceRandomSource {
	return &SequenceRandomSource{values: append([]int(nil), values...)}
}

func (source *SequenceRandomSource) Intn(maxExclusive int) (int, error) {
	if maxExclusive <= 0 {
		return 0, fmt.Errorf("%w: random max must be positive", ErrInvalidRoll)
	}
	if len(source.values) == 0 {
		return 0, fmt.Errorf("%w: deterministic random source is empty", ErrInvalidRoll)
	}

	value := source.values[source.next%len(source.values)]
	source.next++
	if value < 0 {
		value = -value
	}
	return value % maxExclusive, nil
}

func Roll(battle *state.Battle, requestID string, actorID string, rerollIndices []int, opts ...RollOption) ([]event.Event, error) {
	if battle == nil {
		return nil, fmt.Errorf("%w: battle is nil", ErrInvalidRoll)
	}
	if actorID == "" {
		return nil, fmt.Errorf("%w: actor id is required", ErrInvalidRoll)
	}

	request, err := activeRequestForActor(battle, requestID, actorID)
	if err != nil {
		return nil, err
	}
	if request.Complete {
		return nil, fmt.Errorf("%w: roll request %q is complete", ErrInvalidRoll, request.ID)
	}
	if request.Segment != battle.Segment.Current {
		return nil, fmt.Errorf("%w: roll request %q is for segment %q, current segment is %q", ErrInvalidRoll, request.ID, request.Segment, battle.Segment.Current)
	}

	actor, ok := battle.Actors[actorID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingDiceState, actorID)
	}

	rollState, err := rollStateForRequest(actor.Dice.CurrentRoll, request)
	if err != nil {
		return nil, err
	}
	if rollState.Complete {
		return nil, fmt.Errorf("%w: roll request %q is complete", ErrInvalidRoll, request.ID)
	}
	if rollState.RollsUsed >= rollState.MaxRolls {
		return nil, fmt.Errorf("%w: max rolls exhausted for request %q", ErrInvalidRoll, request.ID)
	}

	indices, err := indicesToRoll(rollState, rerollIndices)
	if err != nil {
		return nil, err
	}

	options := rollOptions{randomSource: CryptoRandomSource{}}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&options)
	}
	if options.randomSource == nil {
		return nil, fmt.Errorf("%w: random source is required", ErrInvalidRoll)
	}

	for _, index := range indices {
		rolled, err := rollDie(battle.DiceDefinitions, rollState.Dice[index].DieID, index, options.randomSource)
		if err != nil {
			return nil, err
		}
		rollState.Dice[index] = rolled
	}

	rollState.RollsUsed++
	rollState.SymbolCounts = SymbolCounts(rollState.Dice)
	rollState.Combinations = Combinations(rollState.Dice)

	actor.Dice.CurrentRoll = &rollState
	battle.Actors[actorID] = actor

	return []event.Event{
		event.NewDiceRolled(
			actorID,
			rollState.Segment,
			rollState.RequestID,
			rollState.Pool,
			rollState.SourceType,
			rollState.SourceID,
			rollState.Dice,
			indices,
			rollState.RollsUsed,
			rollState.MaxRolls,
			rollState.Combinations,
			rollState.SymbolCounts,
		),
	}, nil
}

func activeRequestForActor(battle *state.Battle, requestID string, actorID string) (state.RollRequest, error) {
	if requestID != "" {
		request, ok := battle.RollRequests[requestID]
		if !ok {
			return state.RollRequest{}, fmt.Errorf("%w: active roll request %q was not found", ErrMissingRollState, requestID)
		}
		if request.ActorID != actorID {
			return state.RollRequest{}, fmt.Errorf("%w: roll request %q belongs to actor %q", ErrInvalidRoll, requestID, request.ActorID)
		}
		return request, nil
	}

	var matches []state.RollRequest
	for _, request := range battle.RollRequests {
		if request.ActorID == actorID && !request.Complete {
			matches = append(matches, request)
		}
	}
	switch len(matches) {
	case 0:
		return state.RollRequest{}, fmt.Errorf("%w: actor %q has no active roll request", ErrMissingRollState, actorID)
	case 1:
		return matches[0], nil
	default:
		return state.RollRequest{}, fmt.Errorf("%w: actor %q has multiple active roll requests", ErrInvalidRoll, actorID)
	}
}

func rollStateForRequest(current *state.RollState, request state.RollRequest) (state.RollState, error) {
	if request.ID == "" {
		return state.RollState{}, fmt.Errorf("%w: request id is required", ErrInvalidRoll)
	}
	if request.ActorID == "" {
		return state.RollState{}, fmt.Errorf("%w: request actor id is required", ErrInvalidRoll)
	}
	if request.MaxRolls <= 0 {
		return state.RollState{}, fmt.Errorf("%w: request %q max rolls must be positive", ErrInvalidRoll, request.ID)
	}

	if current != nil && current.RequestID == request.ID {
		copied := *current
		copied.Dice = copyRolledDice(current.Dice)
		copied.KeptIndices = append([]int(nil), current.KeptIndices...)
		copied.Combinations = append([]string(nil), current.Combinations...)
		copied.SymbolCounts = copySymbolCounts(current.SymbolCounts)
		return copied, nil
	}

	dice, err := expandDiceLoadout(request.DiceLoadout)
	if err != nil {
		return state.RollState{}, err
	}

	return state.RollState{
		RequestID:    request.ID,
		ActorID:      request.ActorID,
		Segment:      request.Segment,
		Pool:         request.Pool,
		SourceType:   request.SourceType,
		SourceID:     request.SourceID,
		Dice:         dice,
		RollsUsed:    0,
		MaxRolls:     request.MaxRolls,
		Combinations: nil,
		SymbolCounts: map[string]int{},
	}, nil
}

func expandDiceLoadout(loadout []state.DiceLoadoutEntry) ([]state.RolledDie, error) {
	if len(loadout) == 0 {
		return nil, fmt.Errorf("%w: dice loadout is required", ErrInvalidRoll)
	}

	var dice []state.RolledDie
	for _, entry := range loadout {
		switch {
		case entry.DiceID == "":
			return nil, fmt.Errorf("%w: dice id is required", ErrInvalidRoll)
		case entry.Count <= 0:
			return nil, fmt.Errorf("%w: dice count for %q must be positive", ErrInvalidRoll, entry.DiceID)
		}

		for i := 0; i < entry.Count; i++ {
			dice = append(dice, state.RolledDie{
				Index: len(dice),
				DieID: entry.DiceID,
			})
		}
	}
	return dice, nil
}

func indicesToRoll(rollState state.RollState, requested []int) ([]int, error) {
	if len(rollState.Dice) == 0 {
		return nil, fmt.Errorf("%w: roll request %q has no dice", ErrInvalidRoll, rollState.RequestID)
	}
	if rollState.RollsUsed == 0 && len(requested) > 0 {
		return nil, fmt.Errorf("%w: cannot reroll specific dice before the first roll", ErrInvalidRoll)
	}

	if len(requested) == 0 {
		indices := make([]int, 0, len(rollState.Dice))
		for i := range rollState.Dice {
			if !containsIndex(rollState.KeptIndices, i) {
				indices = append(indices, i)
			}
		}
		return indices, nil
	}

	seen := map[int]bool{}
	for _, index := range requested {
		switch {
		case index < 0 || index >= len(rollState.Dice):
			return nil, fmt.Errorf("%w: reroll index %d is out of range", ErrInvalidRoll, index)
		case seen[index]:
			return nil, fmt.Errorf("%w: duplicate reroll index %d", ErrInvalidRoll, index)
		case containsIndex(rollState.KeptIndices, index):
			return nil, fmt.Errorf("%w: reroll index %d is kept", ErrInvalidRoll, index)
		}
		seen[index] = true
	}

	return append([]int(nil), requested...), nil
}

func rollDie(definitions map[string]state.DiceDefinition, dieID string, index int, source RandomSource) (state.RolledDie, error) {
	definition, ok := definitions[dieID]
	if !ok {
		return state.RolledDie{}, fmt.Errorf("%w: %q", ErrMissingDefinition, dieID)
	}
	if len(definition.Faces) == 0 {
		return state.RolledDie{}, fmt.Errorf("%w: dice %q has no faces", ErrInvalidRoll, dieID)
	}

	faceIndex, err := source.Intn(len(definition.Faces))
	if err != nil {
		return state.RolledDie{}, fmt.Errorf("%w: roll %q: %v", ErrInvalidRoll, dieID, err)
	}

	face := definition.Faces[faceIndex]
	return state.RolledDie{
		Index:   index,
		DieID:   dieID,
		Face:    face.Face,
		Value:   face.Value,
		Symbols: copyStrings(face.Symbols),
	}, nil
}

func SymbolCounts(dice []state.RolledDie) map[string]int {
	counts := map[string]int{}
	for _, die := range dice {
		for _, symbol := range die.Symbols {
			counts[symbol]++
		}
	}
	return counts
}

func Combinations(dice []state.RolledDie) []string {
	if len(dice) == 0 {
		return nil
	}

	frequency := map[int]int{}
	unique := map[int]bool{}
	for _, die := range dice {
		frequency[die.Value]++
		unique[die.Value] = true
	}

	found := map[string]bool{}
	for _, count := range frequency {
		switch {
		case count >= 5:
			found["five_of_kind"] = true
			fallthrough
		case count >= 4:
			found["four_of_kind"] = true
			fallthrough
		case count >= 3:
			found["three_of_kind"] = true
			fallthrough
		case count >= 2:
			found["pair"] = true
		}
	}
	if hasStraight(unique, 4) {
		found["small_straight"] = true
	}
	if hasStraight(unique, 5) {
		found["large_straight"] = true
	}

	combinations := make([]string, 0, len(found))
	for combination := range found {
		combinations = append(combinations, combination)
	}
	sort.Strings(combinations)
	return combinations
}

func hasStraight(values map[int]bool, length int) bool {
	for start := range values {
		for offset := 0; offset < length; offset++ {
			if !values[start+offset] {
				goto nextStart
			}
		}
		return true
	nextStart:
	}
	return false
}

func containsIndex(indices []int, target int) bool {
	for _, index := range indices {
		if index == target {
			return true
		}
	}
	return false
}

func copyRolledDice(values []state.RolledDie) []state.RolledDie {
	copied := make([]state.RolledDie, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Symbols = copyStrings(value.Symbols)
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func copySymbolCounts(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
