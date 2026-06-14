package engine

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const DefaultMaxAutomaticSteps = 1000

var (
	ErrMissingSegmentFlow    = errors.New("missing segment flow")
	ErrInvalidSegmentState   = errors.New("invalid segment state")
	ErrInvalidProgressStatus = errors.New("invalid progress status")
	ErrAutomaticStepLimit    = errors.New("automatic progression step limit exceeded")
)

type Config struct {
	MaxAutomaticSteps int
}

type Engine struct {
	manager           segment.Manager
	flows             map[segment.Segment]SegmentFlow
	maxAutomaticSteps int
}

type ProgressionResult struct {
	Status ProgressStatus
	Events []event.Event
}

func NewEngine() Engine {
	eng, err := NewEngineWithConfig(Config{}, DefaultFlows()...)
	if err != nil {
		panic(err)
	}
	return eng
}

func NewEngineWithFlows(flows ...SegmentFlow) (Engine, error) {
	return NewEngineWithConfig(Config{}, flows...)
}

func NewEngineWithConfig(config Config, flows ...SegmentFlow) (Engine, error) {
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

	maxSteps := config.MaxAutomaticSteps
	if maxSteps == 0 {
		maxSteps = DefaultMaxAutomaticSteps
	}
	if maxSteps < 1 {
		return Engine{}, errors.New("max automatic steps must be positive")
	}

	return Engine{
		manager:           segment.NewManager(),
		flows:             registered,
		maxAutomaticSteps: maxSteps,
	}, nil
}

func (e Engine) FlowFor(id segment.Segment) (SegmentFlow, error) {
	flow, ok := e.flows[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingSegmentFlow, id)
	}
	return flow, nil
}

// ProgressUntilInput is the only authority for flow completion decisions.
// It accumulates automatic events across stages and segments, returning only
// when a human actor needs input or progression fails.
func (e Engine) ProgressUntilInput(battle *state.Battle) (ProgressionResult, error) {
	if battle == nil {
		return ProgressionResult{}, fmt.Errorf("%w: battle is nil", ErrInvalidSegmentState)
	}
	if err := validateBattleFlowState(battle); err != nil {
		return ProgressionResult{}, err
	}

	var events []event.Event
	steps := 0
	for {
		if state.IsTerminalBattleStatus(battle.Status) {
			return ProgressionResult{
				Status: ProgressBattleComplete,
				Events: events,
			}, nil
		}

		flow, err := e.FlowFor(battle.Segment.Current)
		if err != nil {
			return ProgressionResult{}, err
		}

		if !battle.Flow.Entered {
			entryEvents, err := enterFlow(flow, battle)
			if err != nil {
				return ProgressionResult{}, fmt.Errorf("enter %q flow: %w", battle.Segment.Current, err)
			}
			events = append(events, event.NewSegmentEntered(battle.Segment))
			events = append(events, entryEvents...)
		}

		if err := e.takeAutomaticStep(&steps, battle); err != nil {
			return ProgressionResult{}, err
		}

		result, err := progressFlow(flow, battle)
		if err != nil {
			return ProgressionResult{}, fmt.Errorf("progress %q flow: %w", battle.Segment.Current, err)
		}
		events = append(events, result.Events...)

		switch result.Status {
		case ProgressContinue:
			continue
		case ProgressWaitingForInput:
			if err := validateWaitingState(battle); err != nil {
				return ProgressionResult{}, err
			}
			return ProgressionResult{
				Status: ProgressWaitingForInput,
				Events: events,
			}, nil
		case ProgressSegmentComplete:
			if err := e.takeAutomaticStep(&steps, battle); err != nil {
				return ProgressionResult{}, err
			}
			transitionEvents, err := e.completeAndAdvance(flow, battle)
			if err != nil {
				return ProgressionResult{}, err
			}
			events = append(events, transitionEvents...)
		default:
			return ProgressionResult{}, fmt.Errorf(
				"%w %q in segment %q stage %q",
				ErrInvalidProgressStatus,
				result.Status,
				battle.Segment.Current,
				battle.Flow.Stage,
			)
		}
	}
}

func enterFlow(flow SegmentFlow, battle *state.Battle) ([]event.Event, error) {
	working := battle.Clone()
	events, err := flow.OnEnter(&Context{Battle: &working})
	if err != nil {
		return nil, err
	}
	working.Flow.Entered = true
	if err := validateBattleFlowState(&working); err != nil {
		return nil, err
	}
	if err := validateActorProgress(&working); err != nil {
		return nil, err
	}
	*battle = working
	return events, nil
}

func progressFlow(flow SegmentFlow, battle *state.Battle) (ProgressResult, error) {
	working := battle.Clone()
	result, err := flow.Progress(&Context{Battle: &working})
	if err != nil {
		return ProgressResult{}, err
	}
	if !isValidProgressStatus(result.Status) {
		return ProgressResult{}, fmt.Errorf(
			"%w %q in segment %q stage %q",
			ErrInvalidProgressStatus,
			result.Status,
			working.Segment.Current,
			working.Flow.Stage,
		)
	}
	if err := validateBattleFlowState(&working); err != nil {
		return ProgressResult{}, err
	}
	if err := validateActorProgress(&working); err != nil {
		return ProgressResult{}, err
	}
	if result.Status == ProgressWaitingForInput {
		if err := validateWaitingState(&working); err != nil {
			return ProgressResult{}, err
		}
	}
	*battle = working
	return result, nil
}

func (e Engine) completeAndAdvance(flow SegmentFlow, battle *state.Battle) ([]event.Event, error) {
	next, advance, err := e.manager.Advance(battle.Segment)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSegmentState, err)
	}
	if _, err := e.FlowFor(next.Current); err != nil {
		return nil, err
	}

	working := battle.Clone()
	exitEvents, err := flow.OnExit(&Context{Battle: &working})
	if err != nil {
		return nil, fmt.Errorf("exit %q flow: %w", battle.Segment.Current, err)
	}
	working.Segment = next
	working.Flow = state.NewSegmentFlowState(next)
	*battle = working

	events := append([]event.Event(nil), exitEvents...)
	events = append(events, event.NewSegmentAdvanced(advance))
	return events, nil
}

func (e Engine) takeAutomaticStep(steps *int, battle *state.Battle) error {
	if *steps >= e.maxAutomaticSteps {
		return fmt.Errorf(
			"%w: limit %d at segment %q stage %q iteration %d",
			ErrAutomaticStepLimit,
			e.maxAutomaticSteps,
			battle.Segment.Current,
			battle.Flow.Stage,
			battle.Flow.Iteration,
		)
	}
	*steps++
	return nil
}

func validateBattleFlowState(battle *state.Battle) error {
	if err := validateSegmentState(battle.Segment); err != nil {
		return err
	}

	if battle.Flow.Segment == "" && !battle.Flow.Entered {
		battle.Flow = state.NewSegmentFlowState(battle.Segment)
	}
	if battle.Flow.Segment != battle.Segment.Current || battle.Flow.Round != battle.Segment.Round {
		return fmt.Errorf(
			"%w: flow is %q round %d, segment is %q round %d",
			ErrInvalidSegmentState,
			battle.Flow.Segment,
			battle.Flow.Round,
			battle.Segment.Current,
			battle.Segment.Round,
		)
	}
	if battle.Flow.Actors == nil {
		battle.Flow.Actors = make(map[string]state.ActorFlowState)
	}
	if battle.Flow.PendingInput == nil {
		battle.Flow.PendingInput = make(map[string]state.PendingInput)
	}
	return nil
}

func validateSegmentState(current segment.State) error {
	if current.Round < 1 {
		return fmt.Errorf("%w: invalid round %d", ErrInvalidSegmentState, current.Round)
	}
	if !segment.IsValid(current.Current) {
		return fmt.Errorf("%w: unknown segment %q", ErrInvalidSegmentState, current.Current)
	}
	return nil
}

func validateActorProgress(battle *state.Battle) error {
	for actorID, progress := range battle.Flow.Actors {
		if _, ok := battle.Actors[actorID]; !ok {
			return fmt.Errorf("flow actor %q is not in battle", actorID)
		}
		if !state.IsValidActorProgressStatus(progress.Status) {
			return fmt.Errorf("invalid actor progress status %q for actor %q", progress.Status, actorID)
		}
	}
	return nil
}

func validateWaitingState(battle *state.Battle) error {
	found := false
	for actorID, progress := range battle.Flow.Actors {
		if progress.Status != state.ActorNeedsInput {
			continue
		}
		actor, ok := battle.Actors[actorID]
		if !ok || actor.Controller != state.ControllerHuman {
			return fmt.Errorf("actor %q needs input but is not human-controlled", actorID)
		}
		if _, ok := battle.Flow.PendingInput[actorID]; !ok {
			return fmt.Errorf("actor %q needs input without pending input", actorID)
		}
	}
	for actorID, pending := range battle.Flow.PendingInput {
		found = true
		actor, ok := battle.Actors[actorID]
		if !ok {
			return fmt.Errorf("pending input actor %q is not in battle", actorID)
		}
		if actor.Controller != state.ControllerHuman {
			return fmt.Errorf("pending input actor %q is not human-controlled", actorID)
		}
		progress, ok := battle.Flow.Actors[actorID]
		if !ok || progress.Status != state.ActorNeedsInput {
			return fmt.Errorf("pending input actor %q does not need input", actorID)
		}
		if pending.ID == "" || pending.ActorID != actorID {
			return fmt.Errorf("invalid pending input for actor %q", actorID)
		}
		if pending.Segment != battle.Segment.Current ||
			pending.Stage != battle.Flow.Stage ||
			pending.Iteration != battle.Flow.Iteration {
			return fmt.Errorf("pending input %q does not match current flow checkpoint", pending.ID)
		}
		if len(pending.AllowedCommands) == 0 {
			return fmt.Errorf("pending input %q has no allowed commands", pending.ID)
		}
	}
	if !found {
		return errors.New("waiting_for_input requires pending human input")
	}
	return nil
}

func isValidProgressStatus(status ProgressStatus) bool {
	switch status {
	case ProgressContinue, ProgressWaitingForInput, ProgressSegmentComplete:
		return true
	default:
		return false
	}
}
