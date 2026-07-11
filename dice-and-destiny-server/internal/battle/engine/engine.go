package engine

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/event"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const DefaultMaxAutomaticSteps = 1000
const DefaultMaxReactionChainDepth = 16
const DefaultMaxReactionRounds = 32

var (
	ErrMissingSegmentFlow      = errors.New("missing segment flow")
	ErrInvalidSegmentState     = errors.New("invalid segment state")
	ErrInvalidProgressStatus   = errors.New("invalid progress status")
	ErrAutomaticStepLimit      = errors.New("automatic progression step limit exceeded")
	ErrReactionChainDepthLimit = errors.New("reaction chain depth limit exceeded")
	ErrReactionRoundLimit      = errors.New("reaction round limit exceeded")
)

type Config struct {
	MaxAutomaticSteps     int
	MaxReactionChainDepth int
	MaxReactionRounds     int
	ProposalRules         []ProposalRule
	InteractionAI         InteractionAI
	DiceRandom            battlerandom.Source
	DamageRandom          battlerandom.Source
	NamedRandom           battlerandom.NamedSource
}

type Engine struct {
	manager               segment.Manager
	flows                 map[segment.Segment]SegmentFlow
	maxAutomaticSteps     int
	maxReactionChainDepth int
	maxReactionRounds     int
	proposalRules         map[state.ProposalOperation]ProposalRule
	interactionAI         InteractionAI
	diceRandom            battlerandom.Source
	damageRandom          battlerandom.Source
	namedRandom           battlerandom.NamedSource
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
	maxDepth := config.MaxReactionChainDepth
	if maxDepth == 0 {
		maxDepth = DefaultMaxReactionChainDepth
	}
	if maxDepth < 1 {
		return Engine{}, errors.New("max reaction chain depth must be positive")
	}
	maxRounds := config.MaxReactionRounds
	if maxRounds == 0 {
		maxRounds = DefaultMaxReactionRounds
	}
	if maxRounds < 1 {
		return Engine{}, errors.New("max reaction rounds must be positive")
	}
	rules := make(map[state.ProposalOperation]ProposalRule, len(config.ProposalRules))
	for _, rule := range config.ProposalRules {
		if rule == nil {
			return Engine{}, errors.New("proposal rule is required")
		}
		operation := rule.Operation()
		if operation == "" {
			return Engine{}, errors.New("proposal rule operation is required")
		}
		if _, exists := rules[operation]; exists {
			return Engine{}, fmt.Errorf("duplicate proposal rule %q", operation)
		}
		rules[operation] = rule
	}
	interactionAI := config.InteractionAI
	if interactionAI == nil {
		interactionAI = DefaultInteractionAI{}
	}

	return Engine{
		manager:               segment.NewManager(),
		flows:                 registered,
		maxAutomaticSteps:     maxSteps,
		maxReactionChainDepth: maxDepth,
		maxReactionRounds:     maxRounds,
		proposalRules:         rules,
		interactionAI:         interactionAI,
		diceRandom:            config.DiceRandom,
		damageRandom:          config.DamageRandom,
		namedRandom:           config.NamedRandom,
	}, nil
}

func (e Engine) diceRandomSource(battle *state.Battle) battlerandom.Source {
	return battlerandom.BattleSource{Battle: battle, Fallback: e.diceRandom}
}

func (e Engine) damageRandomSource(battle *state.Battle) battlerandom.Source {
	return battlerandom.BattleSource{Battle: battle, Fallback: e.damageRandom}
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
	if battle.Settled != nil {
		return e.progressSettled(battle)
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
			entryEvents, err := e.enterFlow(flow, battle)
			if err != nil {
				return ProgressionResult{}, fmt.Errorf("enter %q flow: %w", battle.Segment.Current, err)
			}
			events = append(events, event.NewSegmentEntered(battle.Segment))
			events = append(events, entryEvents...)
		}

		if err := e.takeAutomaticStep(&steps, battle); err != nil {
			return ProgressionResult{}, err
		}

		var result ProgressResult
		if battle.ActiveResolutionID != "" {
			result, err = e.progressResolution(battle)
		} else {
			result, err = e.progressFlow(flow, battle)
		}
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

func (e Engine) enterFlow(flow SegmentFlow, battle *state.Battle) ([]event.Event, error) {
	working := battle.Clone()
	events, err := flow.OnEnter(e.context(&working, state.FlowPhaseOnEnter))
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

func (e Engine) progressFlow(flow SegmentFlow, battle *state.Battle) (ProgressResult, error) {
	working := battle.Clone()
	result, err := flow.Progress(e.context(&working, state.FlowPhaseInProgress))
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
	var exitEvents []event.Event
	var err error
	if !battle.Flow.ExitStarted {
		working := battle.Clone()
		exitEvents, err = flow.OnExit(e.context(&working, state.FlowPhaseOnExit))
		if err != nil {
			return nil, fmt.Errorf("exit %q flow: %w", battle.Segment.Current, err)
		}
		working.Flow.ExitStarted = true
		if err := validateBattleFlowState(&working); err != nil {
			return nil, err
		}
		if working.ActiveResolutionID != "" {
			*battle = working
			return exitEvents, nil
		}
		*battle = working
	}
	working := battle.Clone()
	completionEvents, err := evaluateBattleCompletion(&working)
	if err != nil {
		return nil, err
	}
	*battle = working
	events := append([]event.Event(nil), exitEvents...)
	events = append(events, completionEvents...)
	if state.IsTerminalBattleStatus(battle.Status) {
		return events, nil
	}

	next, advance, err := e.manager.Advance(battle.Segment)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSegmentState, err)
	}
	if _, err := e.FlowFor(next.Current); err != nil {
		return nil, err
	}
	working = battle.Clone()
	working.Segment = next
	working.Flow = state.NewSegmentFlowState(next)
	*battle = working

	events = append(events, event.NewSegmentAdvanced(advance))
	return events, nil
}

func (e Engine) context(battle *state.Battle, phase state.FlowPhase) *Context {
	return &Context{
		Battle:       battle,
		Phase:        phase,
		DiceRandom:   e.diceRandomSource(battle),
		DamageRandom: e.damageRandomSource(battle),
	}
}

func evaluateBattleCompletion(battle *state.Battle) ([]event.Event, error) {
	if battle == nil {
		return nil, errors.New("battle is required")
	}
	if battle.ActiveResolutionID != "" {
		return nil, errors.New("cannot evaluate battle completion with an active resolution")
	}
	if state.IsTerminalBattleStatus(battle.Status) {
		return nil, nil
	}

	humanCount := 0
	playerDefeated := false
	enemyCount := 0
	enemiesDefeated := 0
	for actorID, actor := range battle.Actors {
		if actor.DefeatState == state.ActorPendingDefeat {
			actor.DefeatState = state.ActorDefeated
			battle.Actors[actorID] = actor
		}
		switch actor.Controller {
		case state.ControllerHuman:
			humanCount++
			if actor.DefeatState == state.ActorDefeated {
				playerDefeated = true
			}
		case state.ControllerAI:
			enemyCount++
			if actor.DefeatState == state.ActorDefeated {
				enemiesDefeated++
			}
		}
	}
	if battle.EscapeRequested {
		battle.Status = state.BattleEscaped
		return []event.Event{event.NewBattleCompleted(battle.Status)}, nil
	}
	if humanCount != 1 {
		return nil, fmt.Errorf("battle completion requires exactly one human player, got %d", humanCount)
	}

	allEnemiesDefeated := enemyCount > 0 && enemiesDefeated == enemyCount
	switch {
	case playerDefeated && allEnemiesDefeated:
		battle.Status = state.BattleDraw
	case playerDefeated && enemiesDefeated < enemyCount:
		battle.Status = state.BattleDefeat
	case !playerDefeated && allEnemiesDefeated:
		battle.Status = state.BattleVictory
	default:
		return nil, nil
	}
	return []event.Event{event.NewBattleCompleted(battle.Status)}, nil
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
	if battle.Resolutions == nil {
		battle.Resolutions = make(map[string]state.ResolutionState)
	}
	if err := validateActiveResolution(battle); err != nil {
		return err
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
		if pending.WindowID != "" {
			resolution, window, err := activeWindow(battle)
			if err != nil {
				return err
			}
			if pending.WindowID != window.ID ||
				pending.Phase != resolution.Origin.Phase ||
				pending.ReactionRound != window.ReactionRound {
				return fmt.Errorf("pending input %q does not match current interaction window", pending.ID)
			}
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
