package engine_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestBattleCompletionOutcomesAtSegmentExit(t *testing.T) {
	tests := []struct {
		name        string
		playerState state.ActorDefeatState
		enemyStates []state.ActorDefeatState
		escape      bool
		want        state.BattleStatus
	}{
		{name: "active", enemyStates: []state.ActorDefeatState{"", ""}, want: state.BattleActive},
		{name: "victory", enemyStates: []state.ActorDefeatState{state.ActorPendingDefeat, state.ActorDefeated}, want: state.BattleVictory},
		{name: "defeat", playerState: state.ActorPendingDefeat, enemyStates: []state.ActorDefeatState{""}, want: state.BattleDefeat},
		{name: "draw", playerState: state.ActorPendingDefeat, enemyStates: []state.ActorDefeatState{state.ActorPendingDefeat}, want: state.BattleDraw},
		{name: "escaped", enemyStates: []state.ActorDefeatState{""}, escape: true, want: state.BattleEscaped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			battle := completionBattle(t, tt.playerState, tt.enemyStates...)
			battle.EscapeRequested = tt.escape
			complete := &immediateCompleteFlow{id: segment.OngoingEffects}
			next := &completionWaitFlow{id: segment.Income}
			eng, err := engine.NewEngineWithFlows(complete, next)
			if err != nil {
				t.Fatalf("NewEngineWithFlows() returned error: %v", err)
			}

			result, err := eng.ProgressUntilInput(&battle)
			if err != nil {
				t.Fatalf("ProgressUntilInput() returned error: %v", err)
			}
			if battle.Status != tt.want {
				t.Fatalf("battle status = %q, want %q", battle.Status, tt.want)
			}
			completed := countCompletionEvents(result.Events)
			if tt.want == state.BattleActive {
				if result.Status != engine.ProgressWaitingForInput || completed != 0 ||
					battle.Segment.Current != segment.Income {
					t.Fatalf("active result = %#v battle=%#v", result, battle.Segment)
				}
				return
			}
			if result.Status != engine.ProgressBattleComplete || completed != 1 {
				t.Fatalf("terminal result = %#v, completion events = %d", result, completed)
			}
			for _, actor := range battle.Actors {
				if actor.DefeatState == state.ActorPendingDefeat {
					t.Fatalf("pending defeat survived completion evaluation: %#v", battle.Actors)
				}
			}
			second, err := eng.ProgressUntilInput(&battle)
			if err != nil {
				t.Fatalf("second ProgressUntilInput() returned error: %v", err)
			}
			if second.Status != engine.ProgressBattleComplete || len(second.Events) != 0 {
				t.Fatalf("second terminal progress = %#v, want no duplicate event", second)
			}
		})
	}
}

func TestCompletionWaitsForOnExitNestedResolution(t *testing.T) {
	battle := completionBattle(t, "", state.ActorPendingDefeat)
	exitFlow := &completionExitResolutionFlow{}
	eng, err := engine.NewEngineWithConfig(
		engine.Config{ProposalRules: []engine.ProposalRule{adjustTokenRule{}}},
		exitFlow,
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfig() returned error: %v", err)
	}

	started, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if started.Status != engine.ProgressWaitingForInput ||
		battle.Status != state.BattleActive ||
		battle.ActiveResolutionID == "" ||
		exitFlow.exitCalls != 1 ||
		countCompletionEvents(started.Events) != 0 {
		t.Fatalf("nested exit wait = %#v battle=%#v exits=%d", started, battle, exitFlow.exitCalls)
	}

	pending := battle.Flow.PendingInput["player"]
	progressed, err := eng.ApplyBattleCommand(
		&battle,
		interactionCommitCommand(
			battle.ID,
			"player",
			pending,
			command.InteractionCommitmentData{ChoiceID: "finish-exit"},
		),
	)
	if err != nil {
		t.Fatalf("ApplyBattleCommand() returned error: %v", err)
	}
	if progressed.Status != engine.ProgressBattleComplete ||
		battle.Status != state.BattleVictory ||
		battle.ActiveResolutionID != "" ||
		exitFlow.exitCalls != 1 ||
		countCompletionEvents(progressed.Events) != 1 {
		t.Fatalf("completed nested exit = %#v battle=%#v exits=%d", progressed, battle, exitFlow.exitCalls)
	}
}

func TestTerminalBattleCommandIsRejectedWithoutMutation(t *testing.T) {
	battle := completionBattle(t, "", state.ActorDefeated)
	battle.Status = state.BattleVictory
	before := battle.Clone()
	eng, err := engine.NewEngineWithFlows(&immediateCompleteFlow{id: segment.OngoingEffects})
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	_, err = eng.ApplyBattleCommand(&battle, command.Command{
		BattleID: battle.ID,
		ActorID:  "player",
		Type:     command.TypePass,
		Payload:  []byte(`{}`),
	})
	if err == nil || err.Error() != "battle is complete" {
		t.Fatalf("ApplyBattleCommand() error = %v, want battle is complete", err)
	}
	if !reflect.DeepEqual(battle, before) {
		t.Fatal("terminal command mutated battle")
	}
}

func completionBattle(
	t *testing.T,
	playerState state.ActorDefeatState,
	enemyStates ...state.ActorDefeatState,
) state.Battle {
	t.Helper()
	actors := []state.ActorSetup{{
		ID:             "player",
		ControllerType: state.ControllerHuman,
		Deck:           []string{"player-health"},
		Tokens:         []state.TokenState{{ID: "fake-counter"}},
	}}
	for i := range enemyStates {
		actors = append(actors, state.ActorSetup{
			ID:             "enemy-" + string(rune('1'+i)),
			ControllerType: state.ControllerAI,
			Deck:           []string{"enemy-health"},
		})
	}
	battle, err := state.NewBattleFromSetup("completion-battle", state.BattleSetup{Actors: actors})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	player := battle.Actors["player"]
	player.DefeatState = playerState
	battle.Actors["player"] = player
	for i, defeatState := range enemyStates {
		id := "enemy-" + string(rune('1'+i))
		actor := battle.Actors[id]
		actor.DefeatState = defeatState
		battle.Actors[id] = actor
	}
	return battle
}

func countCompletionEvents(events []event.Event) int {
	count := 0
	for _, battleEvent := range events {
		if battleEvent.Type == event.TypeBattleCompleted {
			count++
		}
	}
	return count
}

type immediateCompleteFlow struct {
	id segment.Segment
}

func (flow *immediateCompleteFlow) ID() segment.Segment {
	return flow.id
}

func (*immediateCompleteFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	for actorID := range ctx.Battle.Actors {
		ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolved}
	}
	return nil, nil
}

func (*immediateCompleteFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressSegmentComplete}, nil
}

func (*immediateCompleteFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (*immediateCompleteFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

type completionWaitFlow struct {
	id segment.Segment
}

func (flow *completionWaitFlow) ID() segment.Segment {
	return flow.id
}

func (*completionWaitFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "wait",
		ActorID:         "player",
		Segment:         ctx.Battle.Segment.Current,
		Phase:           state.FlowPhaseInProgress,
		AllowedCommands: []command.Type{command.TypePass},
	}
	return nil, nil
}

func (*completionWaitFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (*completionWaitFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (*completionWaitFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

type completionExitResolutionFlow struct {
	exitCalls int
}

func (*completionExitResolutionFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*completionExitResolutionFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	for actorID := range ctx.Battle.Actors {
		ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolved}
	}
	ctx.Battle.Flow.Stage = "complete-before-exit"
	return nil, nil
}

func (*completionExitResolutionFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressSegmentComplete}, nil
}

func (*completionExitResolutionFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (flow *completionExitResolutionFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	flow.exitCalls++
	return nil, engine.BeginResolution(ctx, engine.ResolutionSpec{
		ID:    "completion-exit-resolution",
		Batch: fakeProposalBatch("completion-exit-resolution"),
		InitialWindow: engine.WindowSpec{
			ID:                "completion-exit-window",
			Purpose:           state.InteractionPurposeChooseCard,
			Source:            fakeSource(),
			EligibleActors:    []string{"player"},
			RequiredActors:    []string{"player"},
			AllowedCommands:   []command.Type{command.TypeCommitInteraction},
			HiddenCommitments: true,
		},
	})
}
