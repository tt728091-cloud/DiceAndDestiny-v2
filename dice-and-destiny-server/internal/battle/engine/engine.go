package engine

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

var (
	ErrMissingSegmentFlow  = errors.New("missing segment flow")
	ErrInvalidSegmentState = errors.New("invalid segment state")
)

type Engine struct {
	manager segment.Manager
	flows   map[segment.Segment]SegmentFlow
}

type AdvanceResult struct {
	Exit    FlowResult
	Advance segment.Advance
	Enter   FlowResult
}

func NewEngine() Engine {
	eng, err := NewEngineWithFlows(DefaultFlows()...)
	if err != nil {
		panic(err)
	}
	return eng
}

func NewEngineWithFlows(flows ...SegmentFlow) (Engine, error) {
	registered := make(map[segment.Segment]SegmentFlow, len(flows))

	for _, flow := range flows {
		if flow == nil {
			return Engine{}, errors.New("segment flow is required")
		}

		id := flow.ID()
		if !segment.IsValid(id) {
			return Engine{}, fmt.Errorf("invalid segment flow id %q", id)
		}

		if _, exists := registered[id]; exists {
			return Engine{}, fmt.Errorf("duplicate segment flow %q", id)
		}

		registered[id] = flow
	}

	return Engine{
		manager: segment.NewManager(),
		flows:   registered,
	}, nil
}

func (e Engine) FlowFor(id segment.Segment) (SegmentFlow, error) {
	flow, ok := e.flows[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingSegmentFlow, id)
	}
	return flow, nil
}

func (e Engine) AdvanceSegment(battle *state.Battle) (AdvanceResult, error) {
	if battle == nil {
		return AdvanceResult{}, fmt.Errorf("%w: battle is nil", ErrInvalidSegmentState)
	}

	if err := validateSegmentState(battle.Segment); err != nil {
		return AdvanceResult{}, err
	}

	currentFlow, err := e.FlowFor(battle.Segment.Current)
	if err != nil {
		return AdvanceResult{}, err
	}

	exit, err := currentFlow.OnExit(&Context{Battle: battle})
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("exit %q flow: %w", battle.Segment.Current, err)
	}

	// The engine owns progression timing, but the segment manager remains the
	// only code that calculates the next segment and round.
	next, advance, err := e.manager.Advance(battle.Segment)
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("%w: %v", ErrInvalidSegmentState, err)
	}

	// Update mutable battle state only after the current segment has fully exited.
	battle.Segment = next

	nextFlow, err := e.FlowFor(next.Current)
	if err != nil {
		return AdvanceResult{}, err
	}

	enter, err := nextFlow.OnEnter(&Context{Battle: battle})
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("enter %q flow: %w", next.Current, err)
	}

	return AdvanceResult{
		Exit:    exit,
		Advance: advance,
		Enter:   enter,
	}, nil
}

func validateSegmentState(state segment.State) error {
	if state.Round < 1 {
		return fmt.Errorf("%w: invalid round %d", ErrInvalidSegmentState, state.Round)
	}

	if !segment.IsValid(state.Current) {
		return fmt.Errorf("%w: unknown segment %q", ErrInvalidSegmentState, state.Current)
	}

	return nil
}
