package engine_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestHandleCommandAdvanceSegmentReturnsEventAndSnapshot(t *testing.T) {
	eng := engine.NewEngine()

	got := eng.HandleCommand(command.Command{
		BattleID: "battle-1",
		ActorID:  "system",
		Type:     command.TypeAdvanceSegment,
		Payload:  json.RawMessage(`{}`),
	})

	want := engine.Result{
		Accepted: true,
		Events: []event.Event{
			{
				Type:  event.TypeSegmentAdvanced,
				From:  segment.OngoingEffects,
				To:    segment.Income,
				Round: 1,
			},
			event.NewCardsDrawn("player", []string{"card-1"}, false),
		},
		Snapshot: &snapshot.Battle{
			BattleID: "battle-1",
			Segment:  segment.Income,
			Round:    1,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() = %#v, want %#v", got, want)
	}
}

func TestHandleCommandRejectsUnsupportedCommandType(t *testing.T) {
	eng := engine.NewEngine()

	got := eng.HandleCommand(command.Command{
		BattleID: "battle-1",
		ActorID:  "system",
		Type:     command.Type("mystery_command"),
		Payload:  json.RawMessage(`{}`),
	})

	want := engine.Result{
		Accepted: false,
		Error:    "unsupported command type",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() = %#v, want %#v", got, want)
	}
}

func TestAdvanceSegmentRunsFlowHooksAndUpdatesBattleSegment(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	var calls []string
	eng, err := engine.NewEngineWithFlows(
		recordingFlow{id: segment.OngoingEffects, calls: &calls},
		recordingFlow{id: segment.Income, calls: &calls},
	)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantCalls := []string{
		"exit:ongoing_effects:ongoing_effects:1",
		"enter:income:income:1",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("flow calls = %#v, want %#v", calls, wantCalls)
	}

	wantSegment := segment.State{Current: segment.Income, Round: 1}
	if battle.Segment != wantSegment {
		t.Fatalf("battle segment = %#v, want %#v", battle.Segment, wantSegment)
	}

	wantAdvance := segment.Advance{
		From:          segment.OngoingEffects,
		To:            segment.Income,
		Round:         1,
		CompletedTurn: false,
	}
	if got.Advance != wantAdvance {
		t.Fatalf("advance = %#v, want %#v", got.Advance, wantAdvance)
	}

	if got.Exit.Decision != engine.ReadyToAdvance {
		t.Fatalf("exit decision = %q, want %q", got.Exit.Decision, engine.ReadyToAdvance)
	}

	if got.Enter.Decision != engine.ReadyToAdvance {
		t.Fatalf("enter decision = %q, want %q", got.Enter.Decision, engine.ReadyToAdvance)
	}
}

func TestDefaultEngineResolvesIncomeFlow(t *testing.T) {
	eng := engine.NewEngine()

	flow, err := eng.FlowFor(segment.Income)
	if err != nil {
		t.Fatalf("FlowFor(income) returned error: %v", err)
	}

	if _, ok := flow.(engine.IncomeFlow); !ok {
		t.Fatalf("FlowFor(income) = %T, want engine.IncomeFlow", flow)
	}
}

func TestDefaultEngineDrawsCardWhenEnteringIncome(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	eng := engine.NewEngine()

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"card-1"}, false),
	}
	if !reflect.DeepEqual(got.Enter.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Enter.Events, wantEvents)
	}

	wantZones := state.CardZones{
		Deck: []string{"card-2", "card-3"},
		Hand: []string{"card-1"},
	}
	if !reflect.DeepEqual(battle.Cards["player"], wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Cards["player"], wantZones)
	}
}

func TestAdvanceSegmentUsesSegmentManagerForRoundWrap(t *testing.T) {
	battle := state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.DamageResolution,
			Round:   1,
		},
		Cards: map[string]state.CardZones{
			"player": {},
		},
	}

	eng := engine.NewEngine()

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantSegment := segment.State{Current: segment.OngoingEffects, Round: 2}
	if battle.Segment != wantSegment {
		t.Fatalf("battle segment = %#v, want %#v", battle.Segment, wantSegment)
	}

	if !got.Advance.CompletedTurn {
		t.Fatalf("CompletedTurn = false, want true")
	}

	if got.Advance.From != segment.DamageResolution || got.Advance.To != segment.OngoingEffects || got.Advance.Round != 2 {
		t.Fatalf("advance = %#v, want damage_resolution -> ongoing_effects in round 2", got.Advance)
	}
}

func TestAdvanceSegmentReturnsErrorWhenNextFlowIsMissing(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	eng, err := engine.NewEngineWithFlows(recordingFlow{id: segment.OngoingEffects})
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	_, err = eng.AdvanceSegment(&battle)
	if err == nil {
		t.Fatal("AdvanceSegment() succeeded with missing income flow")
	}

	if !errors.Is(err, engine.ErrMissingSegmentFlow) {
		t.Fatalf("AdvanceSegment() error = %v, want ErrMissingSegmentFlow", err)
	}

	if !strings.Contains(err.Error(), `income`) {
		t.Fatalf("AdvanceSegment() error = %q, want missing segment name", err.Error())
	}
}

func TestAdvanceSegmentRejectsInvalidSegmentState(t *testing.T) {
	tests := []struct {
		name   string
		battle state.Battle
	}{
		{
			name: "unknown segment",
			battle: state.Battle{
				ID: "battle-1",
				Segment: segment.State{
					Current: segment.Segment("planning"),
					Round:   1,
				},
			},
		},
		{
			name: "zero round",
			battle: state.Battle{
				ID: "battle-1",
				Segment: segment.State{
					Current: segment.OngoingEffects,
					Round:   0,
				},
			},
		},
	}

	eng := engine.NewEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eng.AdvanceSegment(&tt.battle)
			if err == nil {
				t.Fatal("AdvanceSegment() succeeded with invalid segment state")
			}

			if !errors.Is(err, engine.ErrInvalidSegmentState) {
				t.Fatalf("AdvanceSegment() error = %v, want ErrInvalidSegmentState", err)
			}
		})
	}
}

type recordingFlow struct {
	id    segment.Segment
	calls *[]string
}

func (f recordingFlow) ID() segment.Segment {
	return f.id
}

func (f recordingFlow) OnEnter(ctx *engine.Context) (engine.FlowResult, error) {
	f.record("enter", ctx)
	return engine.FlowResult{Decision: engine.ReadyToAdvance}, nil
}

func (f recordingFlow) CanAdvance(ctx *engine.Context) (engine.FlowDecision, error) {
	return engine.ReadyToAdvance, nil
}

func (f recordingFlow) OnExit(ctx *engine.Context) (engine.FlowResult, error) {
	f.record("exit", ctx)
	return engine.FlowResult{Decision: engine.ReadyToAdvance}, nil
}

func (f recordingFlow) record(hook string, ctx *engine.Context) {
	if f.calls == nil {
		return
	}

	*f.calls = append(*f.calls, hook+":"+f.id.String()+":"+ctx.Battle.Segment.Current.String()+":"+strconv.Itoa(ctx.Battle.Segment.Round))
}
