package engine_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/income"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestProgressUntilInputRunsMinimumRealPathInOrderAndExactlyOnce(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	eng := storyEngine(t, &secretAIController{})

	got, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if got.Status != engine.ProgressWaitingForInput {
		t.Fatalf("status = %q, want waiting_for_input", got.Status)
	}

	wantTypes := []event.Type{
		event.TypeSegmentEntered,
		event.TypeSegmentAdvanced,
		event.TypeSegmentEntered,
		event.TypeCardsDrawn,
		event.TypeEnergyPointsGained,
		event.TypeSegmentAdvanced,
		event.TypeSegmentEntered,
		event.TypeRollRequested,
	}
	if gotTypes := eventTypes(got.Events); !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("event types = %#v, want %#v", gotTypes, wantTypes)
	}
	if got.Events[0].Segment != segment.OngoingEffects {
		t.Fatalf("first event = %#v, want ongoing_effects entered", got.Events[0])
	}
	if got.Events[1].From != segment.OngoingEffects || got.Events[1].To != segment.Income {
		t.Fatalf("first transition = %#v, want ongoing_effects -> income", got.Events[1])
	}
	if got.Events[5].From != segment.Income || got.Events[5].To != segment.Offensive {
		t.Fatalf("second transition = %#v, want income -> offensive", got.Events[5])
	}

	if battle.Segment.Current != segment.Offensive || battle.Flow.Stage != "planning" || !battle.Flow.Entered {
		t.Fatalf("flow state = %#v, want entered offensive planning", battle.Flow)
	}
	if battle.Actors["player"].EnergyPoints != 2 {
		t.Fatalf("player energy = %d, want 2", battle.Actors["player"].EnergyPoints)
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Hand, []string{"strike"}) {
		t.Fatalf("player hand = %#v, want one income draw", battle.Actors["player"].Cards.Hand)
	}

	second, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("second ProgressUntilInput() returned error: %v", err)
	}
	if second.Status != engine.ProgressWaitingForInput || len(second.Events) != 0 {
		t.Fatalf("second result = %#v, want event-free wait", second)
	}
	if battle.Actors["player"].EnergyPoints != 2 || len(battle.Actors["player"].Cards.Hand) != 1 {
		t.Fatalf("automatic income rewards repeated: actor = %#v", battle.Actors["player"])
	}
}

func TestOffensiveAIAutomaticallyLocksWhileHumanStillNeedsInput(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	controller := &secretAIController{}
	eng := storyEngine(t, controller)

	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}

	if controller.calls != 1 {
		t.Fatalf("AI controller calls = %d, want 1", controller.calls)
	}
	if got := battle.Flow.Actors["enemy"].Status; got != state.ActorLockedIn {
		t.Fatalf("enemy status = %q, want locked_in", got)
	}
	if got := battle.Flow.Actors["player"].Status; got != state.ActorNeedsInput {
		t.Fatalf("player status = %q, want needs_input", got)
	}
	if battle.Flow.Stage != "planning" {
		t.Fatalf("shared stage = %q, want planning", battle.Flow.Stage)
	}
	if len(battle.Flow.PendingInput) != 1 || battle.Flow.PendingInput["player"].ID == "" {
		t.Fatalf("pending input = %#v, want player input", battle.Flow.PendingInput)
	}
	if battle.Segment.Current != segment.Offensive {
		t.Fatalf("segment = %q, want offensive; one locked actor must not advance", battle.Segment.Current)
	}
}

func TestAdvanceUntilInputReturnsStatusPendingInputAndFinalSnapshot(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	eng := storyEngine(t, &secretAIController{})

	got := eng.AdvanceUntilInput(&battle, "player")
	if !got.Accepted || got.Status != engine.ProgressWaitingForInput {
		t.Fatalf("AdvanceUntilInput() = %#v, want accepted waiting result", got)
	}
	if got.PendingInput["player"].ID == "" {
		t.Fatalf("pending input = %#v, want player input", got.PendingInput)
	}
	if got.Snapshot == nil || got.Snapshot.Segment != segment.Offensive ||
		got.Snapshot.Flow == nil || got.Snapshot.Flow.Stage != "planning" {
		t.Fatalf("snapshot = %#v, want final offensive planning state", got.Snapshot)
	}
	if got.Snapshot.Actors["enemy"].Dice != nil {
		t.Fatalf("snapshot leaked enemy dice: %#v", got.Snapshot.Actors["enemy"].Dice)
	}
}

func TestAdvanceUntilInputReturnsTerminalBattleStatus(t *testing.T) {
	battle := battleWithOneHuman(t)
	battle.Status = state.BattleVictory
	eng := storyEngine(t, &secretAIController{})

	got := eng.AdvanceUntilInput(&battle, "player")
	if !got.Accepted || got.Status != engine.ProgressBattleComplete ||
		got.BattleResult != state.BattleVictory {
		t.Fatalf("AdvanceUntilInput() = %#v, want completed victory", got)
	}
	if len(got.PendingInput) != 0 {
		t.Fatalf("pending input = %#v, want none for terminal battle", got.PendingInput)
	}
}

func TestRepeatedProgressAtWaitDoesNotRerunSegmentEntry(t *testing.T) {
	battle := battleWithOneHuman(t)
	flow := &waitingFlow{id: segment.OngoingEffects}
	eng, err := engine.NewEngineWithFlows(flow)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	first, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("first ProgressUntilInput() returned error: %v", err)
	}
	second, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("second ProgressUntilInput() returned error: %v", err)
	}

	if flow.entries != 1 {
		t.Fatalf("OnEnter calls = %d, want 1", flow.entries)
	}
	if len(first.Events) != 1 || first.Events[0].Type != event.TypeSegmentEntered {
		t.Fatalf("first events = %#v, want one segment_entered", first.Events)
	}
	if len(second.Events) != 0 {
		t.Fatalf("second events = %#v, want no repeated entry events", second.Events)
	}
}

func TestPendingInputAndHiddenAIPlanningAreViewerSafe(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	eng := storyEngine(t, &secretAIController{})
	progressed, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}

	playerView := snapshot.FromBattleForViewer(battle, "player")
	if playerView.Flow == nil || len(playerView.Flow.PendingInput) != 1 {
		t.Fatalf("player flow snapshot = %#v, want own pending input", playerView.Flow)
	}
	if playerView.Actors["enemy"].Dice != nil {
		t.Fatalf("enemy dice = %#v, want hidden before reveal", playerView.Actors["enemy"].Dice)
	}
	payload, err := json.Marshal(playerView)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	for _, secret := range []string{"secret-ability", "secret-card", "secret-target", `"value":6`, `"rolls_used":2`} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("viewer snapshot leaked %q: %s", secret, payload)
		}
	}

	enemyEvent := event.NewDiceRolled(
		"enemy",
		segment.Offensive,
		"secret-request",
		state.RollPoolOffensive,
		state.RollSourceSegment,
		"secret-source",
		[]state.RolledDie{{Index: 0, DieID: "Secret D6", Face: 6, Value: 6, Symbols: []string{"secret-symbol"}}},
		[]int{0},
		2,
		3,
		[]string{"secret-combination"},
		map[string]int{"secret-symbol": 1},
	)
	filtered := event.ForViewer([]event.Event{enemyEvent}, "player")[0]
	if filtered.RequestID != "" || filtered.Dice != nil || filtered.RollsUsed != 0 ||
		filtered.RolledIndices != nil || filtered.SymbolCounts != nil {
		t.Fatalf("filtered enemy dice event leaked private planning data: %#v", filtered)
	}

	if got := event.ForViewer(progressed.Events, "player"); len(got) == 0 {
		t.Fatal("progressed events unexpectedly empty")
	}
}

func TestValidRollCommandUsesCurrentPendingInputAndReturnsToWait(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	eng := storyEngine(t, &secretAIController{})
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}

	pending := battle.Flow.PendingInput["player"]
	got := eng.HandleBattleCommand(&battle, rollCommand("battle-1", "player", pending.SourceID, pending.ID))
	if !got.Accepted {
		t.Fatalf("HandleBattleCommand() rejected valid roll: %#v", got)
	}
	if len(got.Events) != 1 || got.Events[0].Type != event.TypeDiceRolled {
		t.Fatalf("events = %#v, want one dice_rolled event", got.Events)
	}
	if got.PendingInput["player"].ID != pending.ID {
		t.Fatalf("pending input = %#v, want continued offensive decision", got.PendingInput)
	}
	if battle.Flow.Actors["enemy"].Status != state.ActorLockedIn ||
		battle.Flow.Actors["player"].Status != state.ActorNeedsInput {
		t.Fatalf("actor synchronization changed after roll: %#v", battle.Flow.Actors)
	}
}

func TestRollCommandValidationRejectsWrongCheckpointActorAndBattle(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*state.Battle, *command.Command)
		want   string
	}{
		{
			name: "wrong battle",
			mutate: func(_ *state.Battle, cmd *command.Command) {
				cmd.BattleID = "battle-2"
			},
			want: "does not match",
		},
		{
			name: "wrong actor",
			mutate: func(_ *state.Battle, cmd *command.Command) {
				cmd.ActorID = "enemy"
			},
			want: "not human-controlled",
		},
		{
			name: "missing pending input",
			mutate: func(_ *state.Battle, cmd *command.Command) {
				setRollPayload(cmd, "roll-player-offensive-1-1", "")
			},
			want: "pending_input_id is required",
		},
		{
			name: "stale pending input",
			mutate: func(_ *state.Battle, cmd *command.Command) {
				setRollPayload(cmd, "roll-player-offensive-1-1", "input-player-offensive-1-0")
			},
			want: "stale",
		},
		{
			name: "wrong segment checkpoint",
			mutate: func(battle *state.Battle, _ *command.Command) {
				pending := battle.Flow.PendingInput["player"]
				pending.Segment = segment.Defensive
				battle.Flow.PendingInput["player"] = pending
			},
			want: "checkpoint",
		},
		{
			name: "wrong stage checkpoint",
			mutate: func(battle *state.Battle, _ *command.Command) {
				pending := battle.Flow.PendingInput["player"]
				pending.Stage = "reveal"
				battle.Flow.PendingInput["player"] = pending
			},
			want: "checkpoint",
		},
		{
			name: "already locked actor",
			mutate: func(battle *state.Battle, _ *command.Command) {
				progress := battle.Flow.Actors["player"]
				progress.Status = state.ActorLockedIn
				battle.Flow.Actors["player"] = progress
			},
			want: "not waiting for input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			battle := battleWithHumanAndAI(t)
			eng := storyEngine(t, &secretAIController{})
			if _, err := eng.ProgressUntilInput(&battle); err != nil {
				t.Fatalf("ProgressUntilInput() returned error: %v", err)
			}

			pending := battle.Flow.PendingInput["player"]
			cmd := rollCommand("battle-1", "player", pending.SourceID, pending.ID)
			tt.mutate(&battle, &cmd)
			before := battle.Clone()

			got := eng.HandleBattleCommand(&battle, cmd)
			if got.Accepted || !strings.Contains(got.Error, tt.want) {
				t.Fatalf("HandleBattleCommand() = %#v, want rejection containing %q", got, tt.want)
			}
			if !reflect.DeepEqual(battle, before) {
				t.Fatalf("rejected command mutated battle\n got: %#v\nwant: %#v", battle, before)
			}
		})
	}
}

func TestFlowCanWaitForHumanInputInOngoingEffectsBeforeIncome(t *testing.T) {
	battle := battleWithOneHuman(t)
	flow := &waitingFlow{id: segment.OngoingEffects}
	eng, err := engine.NewEngineWithFlows(
		flow,
		mustIncomeFlow(t),
		defaultOffensiveFlow(t, &secretAIController{}),
		engine.DefensiveFlow{},
		engine.DamageResolutionFlow{},
	)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	got, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if got.Status != engine.ProgressWaitingForInput || battle.Segment.Current != segment.OngoingEffects {
		t.Fatalf("result = %#v, segment = %q; want ongoing wait", got, battle.Segment.Current)
	}
	if battle.Actors["player"].EnergyPoints != 0 || len(battle.Actors["player"].Cards.Hand) != 0 {
		t.Fatalf("income ran before ongoing input: %#v", battle.Actors["player"])
	}
}

func TestAllRequiredActorsLockedAllowsSharedStageToContinue(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	syncFlow := &synchronizedFlow{}
	incomeWait := &waitingFlow{id: segment.Income}
	eng, err := engine.NewEngineWithFlows(
		syncFlow,
		incomeWait,
		defaultOffensiveFlow(t, &secretAIController{}),
		engine.DefensiveFlow{},
		engine.DamageResolutionFlow{},
	)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("initial ProgressUntilInput() returned error: %v", err)
	}
	if battle.Segment.Current != segment.OngoingEffects {
		t.Fatalf("one locked actor advanced segment to %q", battle.Segment.Current)
	}

	cmd := command.Command{
		BattleID: "battle-1",
		ActorID:  "player",
		Type:     command.TypeRollDice,
		Payload:  json.RawMessage(`{}`),
	}
	got := eng.HandleBattleCommand(&battle, cmd)
	if !got.Accepted {
		t.Fatalf("HandleBattleCommand() rejected lock-in: %#v", got)
	}
	if battle.Segment.Current != segment.Income {
		t.Fatalf("segment = %q, want income after all required actors locked", battle.Segment.Current)
	}
	if !incomeWait.entered {
		t.Fatal("income flow was not entered after synchronized completion")
	}
}

func TestOnExitRunsOnlyAfterSegmentComplete(t *testing.T) {
	battle := battleWithOneHuman(t)
	flow := &waitingFlow{id: segment.OngoingEffects}
	eng, err := engine.NewEngineWithFlows(
		flow,
		mustIncomeFlow(t),
		defaultOffensiveFlow(t, &secretAIController{}),
		engine.DefensiveFlow{},
		engine.DamageResolutionFlow{},
	)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if flow.exits != 0 {
		t.Fatalf("OnExit calls = %d, want 0 while waiting", flow.exits)
	}
}

func TestInvalidProgressStatusIsRejected(t *testing.T) {
	battle := battleWithOneHuman(t)
	eng, err := engine.NewEngineWithConfig(
		engine.Config{MaxAutomaticSteps: 5},
		&invalidStatusFlow{},
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfig() returned error: %v", err)
	}

	_, err = eng.ProgressUntilInput(&battle)
	if err == nil || !errors.Is(err, engine.ErrInvalidProgressStatus) {
		t.Fatalf("ProgressUntilInput() error = %v, want ErrInvalidProgressStatus", err)
	}
}

func TestInvalidActorProgressStatusIsRejectedWithoutCommittingEntry(t *testing.T) {
	battle := battleWithOneHuman(t)
	eng, err := engine.NewEngineWithFlows(&invalidActorStatusFlow{})
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	_, err = eng.ProgressUntilInput(&battle)
	if err == nil || !strings.Contains(err.Error(), "invalid actor progress status") {
		t.Fatalf("ProgressUntilInput() error = %v, want invalid actor status", err)
	}
	if battle.Flow.Entered {
		t.Fatal("failed entry was committed")
	}
}

func TestAutomaticProgressionGuardRejectsInfiniteFlow(t *testing.T) {
	battle := battleWithOneHuman(t)
	flow := &infiniteFlow{}
	eng, err := engine.NewEngineWithConfig(
		engine.Config{MaxAutomaticSteps: 3},
		flow,
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfig() returned error: %v", err)
	}

	_, err = eng.ProgressUntilInput(&battle)
	if err == nil || !errors.Is(err, engine.ErrAutomaticStepLimit) {
		t.Fatalf("ProgressUntilInput() error = %v, want ErrAutomaticStepLimit", err)
	}
	if flow.progressCalls != 3 {
		t.Fatalf("progress calls = %d, want 3", flow.progressCalls)
	}
	if !strings.Contains(err.Error(), "ongoing_effects") || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("guard error lacks flow context: %v", err)
	}
}

func TestFailedEntryRollsBackMutationAndCanRetryWithoutDuplication(t *testing.T) {
	battle := battleWithOneHuman(t)
	flow := &failingEntryFlow{}
	eng, err := engine.NewEngineWithFlows(flow)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := eng.ProgressUntilInput(&battle); err == nil {
			t.Fatal("ProgressUntilInput() succeeded with failing entry")
		}
		if battle.Actors["player"].EnergyPoints != 0 || battle.Flow.Entered {
			t.Fatalf("failed entry committed on attempt %d: %#v", i+1, battle)
		}
	}
	if flow.entries != 2 {
		t.Fatalf("entry attempts = %d, want 2 independent retries", flow.entries)
	}
}

func TestNewBattlePersistsUnenteredInitialFlowAndControllers(t *testing.T) {
	battle := battleWithHumanAndAI(t)
	if battle.Flow.Segment != segment.OngoingEffects || battle.Flow.Round != 1 || battle.Flow.Entered {
		t.Fatalf("initial flow = %#v, want unentered ongoing_effects round 1", battle.Flow)
	}
	if battle.Actors["player"].Controller != state.ControllerHuman ||
		battle.Actors["enemy"].Controller != state.ControllerAI {
		t.Fatalf("controllers = %#v, want human and AI", battle.Actors)
	}
}

func storyEngine(t *testing.T, controller engine.OffensiveAIController) engine.Engine {
	t.Helper()
	incomeFlow, err := engine.NewIncomeFlow(income.Reward{
		ActorID:      "player",
		DrawCards:    1,
		EnergyPoints: 2,
	})
	if err != nil {
		t.Fatalf("NewIncomeFlow() returned error: %v", err)
	}
	offensiveFlow, err := engine.NewOffensiveFlow(controller)
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	eng, err := engine.NewEngineWithFlows(
		engine.OngoingEffectsFlow{},
		incomeFlow,
		offensiveFlow,
		engine.DefensiveFlow{},
		engine.DamageResolutionFlow{},
	)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	return eng
}

func mustIncomeFlow(t *testing.T) engine.IncomeFlow {
	t.Helper()
	flow, err := engine.NewIncomeFlow()
	if err != nil {
		t.Fatalf("NewIncomeFlow() returned error: %v", err)
	}
	return flow
}

func defaultOffensiveFlow(t *testing.T, controller engine.OffensiveAIController) engine.OffensiveFlow {
	t.Helper()
	flow, err := engine.NewOffensiveFlow(controller)
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	return flow
}

func battleWithOneHuman(t *testing.T) state.Battle {
	t.Helper()
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "player",
				ControllerType: state.ControllerHuman,
				Deck:           []string{"strike", "guard"},
				DiceLoadout:    []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 1}},
			},
		},
		DiceDefinitions: []state.DiceDefinition{standardD6()},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	return battle
}

func battleWithHumanAndAI(t *testing.T) state.Battle {
	t.Helper()
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "player",
				ControllerType: state.ControllerHuman,
				Deck:           []string{"strike", "guard"},
				DiceLoadout:    []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 1}},
			},
			{
				ID:             "enemy",
				ControllerType: state.ControllerAI,
				Hand:           []string{"secret-hand-card"},
				DiceLoadout:    []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 1}},
			},
		},
		DiceDefinitions: []state.DiceDefinition{standardD6()},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	return battle
}

func standardD6() state.DiceDefinition {
	faces := make([]state.DiceFace, 6)
	for i := range faces {
		faces[i] = state.DiceFace{Face: i + 1, Value: i + 1, Symbols: []string{}}
	}
	return state.DiceDefinition{
		ID:        "Standard D6",
		Name:      "Standard D6",
		DieType:   "d6",
		SideCount: 6,
		Faces:     faces,
	}
}

func rollCommand(battleID string, actorID string, requestID string, pendingInputID string) command.Command {
	payload, err := json.Marshal(command.RollDicePayload{
		RequestID:      requestID,
		PendingInputID: pendingInputID,
	})
	if err != nil {
		panic(err)
	}
	return command.Command{
		BattleID: battleID,
		ActorID:  actorID,
		Type:     command.TypeRollDice,
		Payload:  payload,
	}
}

func setRollPayload(cmd *command.Command, requestID string, pendingInputID string) {
	payload, err := json.Marshal(command.RollDicePayload{
		RequestID:      requestID,
		PendingInputID: pendingInputID,
	})
	if err != nil {
		panic(err)
	}
	cmd.Payload = payload
}

func eventTypes(events []event.Event) []event.Type {
	types := make([]event.Type, len(events))
	for i, battleEvent := range events {
		types[i] = battleEvent.Type
	}
	return types
}

type secretAIController struct {
	calls int
}

func (controller *secretAIController) Plan(ctx *engine.Context, actorID string) (state.OffensiveCommitment, error) {
	controller.calls++
	actor := ctx.Battle.Actors[actorID]
	actor.Dice.CurrentRoll = &state.RollState{
		RequestID:    "secret-request",
		ActorID:      actorID,
		Segment:      segment.Offensive,
		Pool:         state.RollPoolOffensive,
		SourceType:   state.RollSourceSegment,
		SourceID:     "secret-source",
		Dice:         []state.RolledDie{{Index: 0, DieID: "Standard D6", Face: 6, Value: 6, Symbols: []string{"secret-symbol"}}},
		RollsUsed:    2,
		MaxRolls:     3,
		Combinations: []string{"secret-combination"},
		SymbolCounts: map[string]int{"secret-symbol": 1},
	}
	ctx.Battle.Actors[actorID] = actor
	return state.OffensiveCommitment{
		ID:              "secret-commitment",
		ActorID:         actorID,
		FinalDice:       actor.Dice.CurrentRoll.Dice,
		RollsUsed:       2,
		SelectedAbility: "secret-ability",
		SelectedCards:   []string{"secret-card"},
		SelectedTargets: []string{"secret-target"},
		RollHistory: [][]state.RolledDie{
			{{Index: 0, DieID: "Standard D6", Face: 1, Value: 1}},
			{{Index: 0, DieID: "Standard D6", Face: 6, Value: 6}},
		},
	}, nil
}

type waitingFlow struct {
	id      segment.Segment
	entered bool
	entries int
	exits   int
}

func (flow *waitingFlow) ID() segment.Segment {
	return flow.id
}

func (flow *waitingFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	flow.entered = true
	flow.entries++
	ctx.Battle.Flow.Stage = "choice"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "choice-player-1",
		ActorID:         "player",
		Segment:         flow.id,
		Stage:           "choice",
		Iteration:       1,
		InputType:       string(command.TypeRollDice),
		AllowedCommands: []command.Type{command.TypeRollDice},
	}
	return nil, nil
}

func (flow *waitingFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (flow *waitingFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (flow *waitingFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	flow.exits++
	return nil, nil
}

type synchronizedFlow struct{}

func (*synchronizedFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*synchronizedFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "synchronized"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["enemy"] = state.ActorFlowState{Status: state.ActorLockedIn}
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "sync-player-1",
		ActorID:         "player",
		Segment:         segment.OngoingEffects,
		Stage:           "synchronized",
		Iteration:       1,
		InputType:       string(command.TypeRollDice),
		AllowedCommands: []command.Type{command.TypeRollDice},
	}
	return nil, nil
}

func (*synchronizedFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	if ctx.Battle.Flow.Actors["player"].Status == state.ActorNeedsInput {
		return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
	}
	return engine.ProgressResult{Status: engine.ProgressSegmentComplete}, nil
}

func (*synchronizedFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	progress := ctx.Battle.Flow.Actors[cmd.ActorID]
	progress.Status = state.ActorLockedIn
	ctx.Battle.Flow.Actors[cmd.ActorID] = progress
	delete(ctx.Battle.Flow.PendingInput, cmd.ActorID)
	return nil, nil
}

func (*synchronizedFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	return nil, nil
}

type invalidStatusFlow struct{}

func (*invalidStatusFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*invalidStatusFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "invalid"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorResolved}
	return nil, nil
}

func (*invalidStatusFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressStatus("not-a-status")}, nil
}

func (*invalidStatusFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	return nil, errors.New("unsupported")
}

func (*invalidStatusFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	return nil, nil
}

type invalidActorStatusFlow struct{}

func (*invalidActorStatusFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*invalidActorStatusFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "invalid-actor"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorProgressStatus("invalid")}
	return nil, nil
}

func (*invalidActorStatusFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (*invalidActorStatusFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	return nil, errors.New("unsupported")
}

func (*invalidActorStatusFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	return nil, nil
}

type infiniteFlow struct {
	progressCalls int
}

func (*infiniteFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*infiniteFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "loop"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorResolvingAutomatic}
	return nil, nil
}

func (flow *infiniteFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	flow.progressCalls++
	return engine.ProgressResult{Status: engine.ProgressContinue}, nil
}

func (*infiniteFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	return nil, errors.New("unsupported")
}

func (*infiniteFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	return nil, nil
}

type failingEntryFlow struct {
	entries int
}

func (*failingEntryFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (flow *failingEntryFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	flow.entries++
	actor := ctx.Battle.Actors["player"]
	actor.EnergyPoints++
	ctx.Battle.Actors["player"] = actor
	return nil, errors.New("entry failed")
}

func (*failingEntryFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{}, nil
}

func (*failingEntryFlow) HandleCommand(ctx *engine.Context, cmd command.Command) ([]event.Event, error) {
	return nil, errors.New("unsupported")
}

func (*failingEntryFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	return nil, nil
}
