package engine_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/income"
	"diceanddestiny/server/internal/battle/segment"
	battlesetup "diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestHandleCommandAdvanceSegmentRejectsWithoutPreparedBattleSetup(t *testing.T) {
	eng := engine.NewEngine()

	got := eng.HandleCommand(command.Command{
		BattleID: "battle-1",
		ActorID:  "system",
		Type:     command.TypeAdvanceSegment,
		Payload:  json.RawMessage(`{}`),
	})

	want := engine.Result{
		Accepted: false,
		Error:    `enter "income" flow: missing actor card state: "player"`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() = %#v, want %#v", got, want)
	}
}

func TestHandleCommandAdvanceSegmentDoesNotInventViewerActorState(t *testing.T) {
	eng := engine.NewEngine()

	got := eng.HandleCommand(command.Command{
		BattleID: "battle-1",
		ActorID:  "player",
		Type:     command.TypeAdvanceSegment,
		Payload:  json.RawMessage(`{}`),
	})

	want := engine.Result{
		Accepted: false,
		Error:    `enter "income" flow: missing actor card state: "player"`,
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

func TestRollDiceAcceptedForActiveOffensiveRollRequest(t *testing.T) {
	battle := battleWithEngineRollRequest(t, segment.Offensive, state.RollPoolOffensive, "offensive", 3)
	eng := engine.NewEngine()

	got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", "roll-player-test"),
	)

	if !got.Accepted {
		t.Fatalf("HandleBattleCommand() rejected active offensive request: %#v", got)
	}
	if len(got.Events) != 1 || got.Events[0].Type != event.TypeDiceRolled {
		t.Fatalf("events = %#v, want dice_rolled", got.Events)
	}
	if got.Events[0].Pool != state.RollPoolOffensive {
		t.Fatalf("pool = %q, want offensive", got.Events[0].Pool)
	}
	if got.Snapshot == nil || got.Snapshot.Actors["player"].Dice == nil {
		t.Fatalf("snapshot = %#v, want current dice roll", got.Snapshot)
	}
}

func TestRollDiceAcceptedForNonOffensivePendingRequestOutsideOffensive(t *testing.T) {
	battle := battleWithEngineRollRequest(t, segment.Defensive, state.RollPoolDefensive, "guarding-light", 1)
	eng := engine.NewEngine()

	got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", "roll-player-test"),
	)

	if !got.Accepted {
		t.Fatalf("HandleBattleCommand() rejected non-offensive pending request: %#v", got)
	}
	if got.Events[0].Segment != segment.Defensive || got.Events[0].Pool != state.RollPoolDefensive {
		t.Fatalf("event = %#v, want defensive request details", got.Events[0])
	}
}

func TestRollDiceRejectsActorWithNoActiveRollRequest(t *testing.T) {
	battle := battleWithEngineRollRequest(t, segment.Offensive, state.RollPoolOffensive, "offensive", 3)
	battle.RollRequests = map[string]state.RollRequest{}
	eng := engine.NewEngine()

	got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", ""),
	)

	if got.Accepted {
		t.Fatalf("HandleBattleCommand() accepted without active request: %#v", got)
	}
	if !strings.Contains(got.Error, "no active roll request") {
		t.Fatalf("error = %q, want no active request context", got.Error)
	}
}

func TestRollDiceRejectsRequestOwnedByAnotherActor(t *testing.T) {
	battle := battleWithEngineRollRequest(t, segment.Offensive, state.RollPoolOffensive, "offensive", 3)
	battle.RollRequests["roll-player-test"] = state.RollRequest{
		ID:          "roll-player-test",
		ActorID:     "enemy",
		Segment:     segment.Offensive,
		Pool:        state.RollPoolOffensive,
		SourceType:  state.RollSourceSegment,
		SourceID:    "offensive",
		DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 1}},
		MaxRolls:    3,
	}
	eng := engine.NewEngine()

	got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", "roll-player-test"),
	)

	if got.Accepted {
		t.Fatalf("HandleBattleCommand() accepted another actor's request: %#v", got)
	}
	if !strings.Contains(got.Error, "belongs to actor") {
		t.Fatalf("error = %q, want ownership context", got.Error)
	}
}

func TestRollDiceRejectsWhenMaxRollsAreExhausted(t *testing.T) {
	battle := battleWithEngineRollRequest(t, segment.Offensive, state.RollPoolOffensive, "offensive", 1)
	eng := engine.NewEngine()

	if got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", "roll-player-test"),
	); !got.Accepted {
		t.Fatalf("first HandleBattleCommand() rejected: %#v", got)
	}

	got := eng.HandleBattleCommand(
		&battle,
		rollDiceCommand("player", "roll-player-test"),
	)
	if got.Accepted {
		t.Fatalf("second HandleBattleCommand() accepted after max rolls exhausted: %#v", got)
	}
	if !strings.Contains(got.Error, "max rolls exhausted") {
		t.Fatalf("error = %q, want max rolls context", got.Error)
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
	battle := repositoryMockPaladinBattle(t)

	eng := engine.NewEngine()

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"Mock Strike"}, false),
	}
	if !reflect.DeepEqual(got.Enter.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Enter.Events, wantEvents)
	}

	wantZones := state.CardZones{
		Deck: append(
			append(
				append([]string{}, repeatedCard("Mock Strike", 7)...),
				repeatedCard("Mock Guard", 6)...,
			),
			repeatedCard("Mock Focus", 6)...,
		),
		Hand:    []string{"Mock Strike"},
		Discard: nil,
		Removed: nil,
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDefaultEngineDrawsCardFromConfiguredSetupActorWhenEnteringIncome(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike", "guard"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	eng := engine.NewEngine()

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"strike"}, false),
	}
	if !reflect.DeepEqual(got.Enter.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Enter.Events, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    []string{"guard"},
		Hand:    []string{"strike"},
		Discard: nil,
		Removed: nil,
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDefaultEngineCreatesOffensiveRollRequestWhenEnteringOffensive(t *testing.T) {
	battle := repositoryMockPaladinBattle(t)
	battle.Segment = segment.State{Current: segment.Income, Round: 1}
	eng := engine.NewEngine()

	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	if got.Enter.Decision != engine.WaitForCommand {
		t.Fatalf("offensive enter decision = %q, want wait_for_command", got.Enter.Decision)
	}

	wantID := "roll-player-offensive-1"
	request, ok := battle.RollRequests[wantID]
	if !ok {
		t.Fatalf("roll request %q was not created; requests = %#v", wantID, battle.RollRequests)
	}
	if request.ActorID != "player" || request.Pool != state.RollPoolOffensive || request.MaxRolls != 3 {
		t.Fatalf("roll request = %#v, want default player offensive request", request)
	}
	if !reflect.DeepEqual(request.DiceLoadout, []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 5}}) {
		t.Fatalf("dice loadout = %#v, want Standard D6 x5", request.DiceLoadout)
	}
}

func TestIncomeFlowConfiguredDrawRewardDrawsCards(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike", "guard"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	flow, err := engine.NewIncomeFlow(income.Reward{
		ActorID:   "player",
		DrawCards: 1,
	})
	if err != nil {
		t.Fatalf("NewIncomeFlow() returned error: %v", err)
	}

	got, err := flow.OnEnter(&engine.Context{Battle: &battle})
	if err != nil {
		t.Fatalf("OnEnter() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"strike"}, false),
	}
	if !reflect.DeepEqual(got.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Events, wantEvents)
	}

	wantZones := state.CardZones{
		Deck: []string{"guard"},
		Hand: []string{"strike"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestIncomeFlowDrawZeroRewardDoesNotDrawCards(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike", "guard"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	flow, err := engine.NewIncomeFlow(income.Reward{
		ActorID:   "player",
		DrawCards: 0,
	})
	if err != nil {
		t.Fatalf("NewIncomeFlow() returned error: %v", err)
	}

	got, err := flow.OnEnter(&engine.Context{Battle: &battle})
	if err != nil {
		t.Fatalf("OnEnter() returned error: %v", err)
	}

	if got.Decision != engine.ReadyToAdvance {
		t.Fatalf("decision = %q, want %q", got.Decision, engine.ReadyToAdvance)
	}
	if len(got.Events) != 0 {
		t.Fatalf("enter events = %#v, want none", got.Events)
	}

	wantZones := state.CardZones{
		Deck: []string{"strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestIncomeFlowEnergyOnlyRewardAddsEnergyPoints(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike", "guard"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	flow, err := engine.NewIncomeFlow(income.Reward{
		ActorID:      "player",
		EnergyPoints: 2,
	})
	if err != nil {
		t.Fatalf("NewIncomeFlow() returned error: %v", err)
	}

	got, err := flow.OnEnter(&engine.Context{Battle: &battle})
	if err != nil {
		t.Fatalf("OnEnter() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewEnergyPointsGained("player", 2),
	}
	if !reflect.DeepEqual(got.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Events, wantEvents)
	}

	if battle.Actors["player"].EnergyPoints != 2 {
		t.Fatalf("energy points = %d, want 2", battle.Actors["player"].EnergyPoints)
	}

	wantZones := state.CardZones{
		Deck: []string{"strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestNewIncomeFlowRejectsInvalidRewardConfig(t *testing.T) {
	tests := []struct {
		name   string
		reward income.Reward
		want   string
	}{
		{
			name:   "missing actor",
			reward: income.Reward{DrawCards: 1},
			want:   "actor id is required",
		},
		{
			name:   "negative draw",
			reward: income.Reward{ActorID: "player", DrawCards: -1},
			want:   "draw cards must be non-negative",
		},
		{
			name:   "negative energy",
			reward: income.Reward{ActorID: "player", EnergyPoints: -1},
			want:   "energy points must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.NewIncomeFlow(tt.reward)
			if err == nil {
				t.Fatal("NewIncomeFlow() succeeded with invalid reward")
			}

			if !errors.Is(err, income.ErrInvalidReward) {
				t.Fatalf("NewIncomeFlow() error = %v, want ErrInvalidReward", err)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewIncomeFlow() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestAdvanceSegmentUsesSegmentManagerForRoundWrap(t *testing.T) {
	battle := state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.DamageResolution,
			Round:   1,
		},
		Actors: map[string]state.ActorState{
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

func battleWithEngineRollRequest(
	t *testing.T,
	requestSegment segment.Segment,
	pool state.RollPool,
	sourceID string,
	maxRolls int,
) state.Battle {
	t.Helper()

	battle := repositoryMockPaladinBattle(t)
	battle.Segment = segment.State{Current: requestSegment, Round: 1}
	battle.RollRequests["roll-player-test"] = state.RollRequest{
		ID:          "roll-player-test",
		ActorID:     "player",
		Segment:     requestSegment,
		Pool:        pool,
		SourceType:  state.RollSourceSegment,
		SourceID:    sourceID,
		DiceLoadout: append([]state.DiceLoadoutEntry(nil), battle.Actors["player"].DiceLoadout...),
		MaxRolls:    maxRolls,
	}
	return battle
}

func repositoryMockPaladinBattle(t *testing.T) state.Battle {
	t.Helper()

	contentRoot := filepath.Join(serverRoot(t), "content")
	library, err := content.LoadContentLibrary(contentRoot)
	if err != nil {
		t.Fatalf("LoadContentLibrary() returned error: %v", err)
	}
	sheet, err := content.LoadCharacterCombatSheetWithLibrary(
		filepath.Join(contentRoot, "characters", "mock_paladin.yaml"),
		library,
	)
	if err != nil {
		t.Fatalf("LoadCharacterCombatSheetWithLibrary() returned error: %v", err)
	}
	battleSetup, err := battlesetup.BattleSetupFromCharacterCombatSheet(sheet, library)
	if err != nil {
		t.Fatalf("BattleSetupFromCharacterCombatSheet() returned error: %v", err)
	}
	battle, err := state.NewBattleFromSetup("battle-1", battleSetup)
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	return battle
}

func repeatedCard(cardID string, count int) []string {
	cards := make([]string, count)
	for i := range cards {
		cards[i] = cardID
	}
	return cards
}

func serverRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}

func rollDiceCommand(actorID string, requestID string) command.Command {
	payload, err := json.Marshal(command.RollDicePayload{RequestID: requestID})
	if err != nil {
		panic(err)
	}

	return command.Command{
		BattleID: "battle-1",
		ActorID:  actorID,
		Type:     command.TypeRollDice,
		Payload:  payload,
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
