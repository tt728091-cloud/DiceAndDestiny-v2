package battle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

const twoRoundBattleID = "two-round-multi-enemy"

func TestAuthorityRunsPlayerAndMultipleEnemiesThroughTwoCompleteRounds(t *testing.T) {
	root := participantTestServerRoot(t)
	assembler := NewFileParticipantAssembler(
		filepath.Join(root, "content"),
		filepath.Join(root, "save", "run_players"),
	)
	initialSetup, err := assembler.AssembleParticipants(twoRoundParticipants())
	if err != nil {
		t.Fatalf("AssembleParticipants() baseline returned error: %v", err)
	}
	initialActors := actorSetupByID(t, initialSetup)

	repo := repository.NewInMemory()
	authority := NewAuthority(engine.NewEngine(), repo, assembler)
	var previousEvents []event.Event
	var previousPendingID string

	started := sendAuthorityJSON(t, authority, map[string]any{
		"battle_id": twoRoundBattleID,
		"actor_id":  "player",
		"type":      command.TypeStartBattle,
		"payload": map[string]any{
			"player": map[string]string{
				"instance_id":   "player",
				"definition_id": "current_run_player",
			},
			"enemies": []map[string]string{
				{"instance_id": "goblin-1", "definition_id": "mock_goblin"},
				{"instance_id": "goblin-2", "definition_id": "mock_goblin"},
			},
		},
	})
	checkpoint := assertPersistedWait(
		t, repo, started, 1, segment.Offensive, state.InteractionPurposePlanning,
		&previousEvents, previousPendingID,
	)
	assertParticipantsAtCheckpoint(t, checkpoint, initialActors)
	assertInitialPlanningWait(t, checkpoint, started, 1)
	assertHiddenEnemyCommitmentsRemainAuthoritative(t, checkpoint.Events, started.Events, 1)
	previousPendingID = checkpoint.Battle.Flow.PendingInput["player"].ID

	for round := 1; round <= 2; round++ {
		passed := sendPlanningCommand(
			t, authority, checkpoint.Battle.Flow.PendingInput["player"], command.TypePlanningPass,
		)
		checkpoint = assertPersistedWait(
			t, repo, passed, round, segment.Offensive, state.InteractionPurposePlanning,
			&previousEvents, previousPendingID,
		)
		assertParticipantsAtCheckpoint(t, checkpoint, initialActors)
		assertPlanningPassWait(t, checkpoint, passed, round, previousPendingID)
		previousPendingID = checkpoint.Battle.Flow.PendingInput["player"].ID

		locked := sendPlanningCommand(
			t, authority, checkpoint.Battle.Flow.PendingInput["player"], command.TypePlanningLockIn,
		)
		checkpoint = assertPersistedWait(
			t, repo, locked, round, segment.Offensive, state.InteractionPurposeReaction,
			&previousEvents, previousPendingID,
		)
		assertParticipantsAtCheckpoint(t, checkpoint, initialActors)
		assertOffensiveRevealAndReactionWait(t, checkpoint, locked, round)
		previousPendingID = checkpoint.Battle.Flow.PendingInput["player"].ID

		reactionPassed := sendReactionPass(
			t, authority, checkpoint.Battle.Flow.PendingInput["player"],
		)
		checkpoint = assertPersistedWait(
			t, repo, reactionPassed, round+1, segment.Offensive, state.InteractionPurposePlanning,
			&previousEvents, previousPendingID,
		)
		assertParticipantsAtCheckpoint(t, checkpoint, initialActors)
		assertRoundCompleted(t, checkpoint, reactionPassed, round)
		previousPendingID = checkpoint.Battle.Flow.PendingInput["player"].ID
	}

	assertFinalIncomeState(t, checkpoint, initialActors)
	assertCompletedRoundPlanning(t, checkpoint.Battle, 1)
	assertCompletedRoundPlanning(t, checkpoint.Battle, 2)
	assertRoundPlanningIDsDiffer(t, checkpoint.Battle)
	assertAuthoritativeBattleRoute(t, checkpoint.Events)
	assertNoDamageOrDefensiveWork(t, checkpoint)
}

func twoRoundParticipants() []Participant {
	return []Participant{
		{InstanceID: "player", DefinitionID: "current_run_player", Controller: state.ControllerHuman},
		{InstanceID: "goblin-1", DefinitionID: "mock_goblin", Controller: state.ControllerAI},
		{InstanceID: "goblin-2", DefinitionID: "mock_goblin", Controller: state.ControllerAI},
	}
}

func actorSetupByID(t *testing.T, setup state.BattleSetup) map[string]state.ActorSetup {
	t.Helper()
	actors := make(map[string]state.ActorSetup, len(setup.Actors))
	for _, actor := range setup.Actors {
		if _, exists := actors[actor.ID]; exists {
			t.Fatalf("baseline setup contains duplicate actor %q", actor.ID)
		}
		actors[actor.ID] = actor
	}
	if len(actors) != 3 {
		t.Fatalf("baseline actor count = %d, want 3", len(actors))
	}
	return actors
}

func sendAuthorityJSON(t *testing.T, authority *Authority, value any) engine.Result {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() command returned error: %v", err)
	}
	result := decodeAuthorityResult(t, authority.HandleCommandJSON(string(encoded)))
	if !result.Accepted || result.Status != engine.ProgressWaitingForInput {
		t.Fatalf("authority result = %#v, want accepted waiting_for_input", result)
	}
	return result
}

func sendPlanningCommand(
	t *testing.T,
	authority *Authority,
	pending state.PendingInput,
	commandType command.Type,
) engine.Result {
	t.Helper()
	checkpoint := command.PlanningCheckpoint{
		WindowID:      pending.WindowID,
		Segment:       string(pending.Segment),
		Stage:         pending.Stage,
		Iteration:     pending.Iteration,
		PlanningCycle: pending.PlanningCycle,
	}
	var payload any
	switch commandType {
	case command.TypePlanningPass:
		payload = command.PlanningPassPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
		}
	case command.TypePlanningLockIn:
		payload = command.PlanningLockInPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
		}
	default:
		t.Fatalf("unsupported integration planning command %q", commandType)
	}
	return sendAuthorityJSON(t, authority, map[string]any{
		"battle_id": twoRoundBattleID,
		"actor_id":  "player",
		"type":      commandType,
		"payload":   payload,
	})
}

func sendReactionPass(
	t *testing.T,
	authority *Authority,
	pending state.PendingInput,
) engine.Result {
	t.Helper()
	return sendAuthorityJSON(t, authority, map[string]any{
		"battle_id": twoRoundBattleID,
		"actor_id":  "player",
		"type":      command.TypePass,
		"payload": command.PassPayload{
			PendingInputID: pending.ID,
			Checkpoint: command.InteractionCheckpoint{
				WindowID:      pending.WindowID,
				Stage:         pending.Stage,
				Iteration:     pending.Iteration,
				ReactionRound: pending.ReactionRound,
			},
		},
	})
}

func assertPersistedWait(
	t *testing.T,
	repo repository.Repository,
	result engine.Result,
	wantRound int,
	wantSegment segment.Segment,
	wantPurpose state.InteractionPurpose,
	previousEvents *[]event.Event,
	previousPendingID string,
) repository.Checkpoint {
	t.Helper()
	checkpoint, err := repo.Load(twoRoundBattleID)
	if err != nil {
		t.Fatalf("Load() after command returned error: %v", err)
	}
	if result.Snapshot == nil {
		t.Fatal("authority result has no snapshot")
	}
	battle := checkpoint.Battle
	if battle.ID != result.Snapshot.BattleID ||
		battle.Segment.Current != result.Snapshot.Segment ||
		battle.Segment.Round != result.Snapshot.Round ||
		len(battle.Actors) != len(result.Snapshot.Actors) {
		t.Fatalf("stored battle and returned snapshot disagree: battle=%#v snapshot=%#v", battle.Segment, result.Snapshot)
	}
	if battle.Segment.Current != wantSegment || battle.Segment.Round != wantRound {
		t.Fatalf("stored checkpoint = %s round %d, want %s round %d",
			battle.Segment.Current, battle.Segment.Round, wantSegment, wantRound)
	}
	storedPending, ok := battle.Flow.PendingInput["player"]
	if !ok || len(battle.Flow.PendingInput) != 1 {
		t.Fatalf("stored pending input = %#v, want exactly player", battle.Flow.PendingInput)
	}
	returnedPending, ok := result.PendingInput["player"]
	if !ok || len(result.PendingInput) != 1 {
		t.Fatalf("returned pending input = %#v, want exactly player", result.PendingInput)
	}
	assertPendingInputMatches(t, storedPending, returnedPending)
	if storedPending.InputType != string(wantPurpose) {
		t.Fatalf("pending purpose = %q, want %q", storedPending.InputType, wantPurpose)
	}
	if previousPendingID != "" && storedPending.ID == previousPendingID {
		t.Fatalf("pending input %q was not rotated after accepted command", storedPending.ID)
	}
	previousEventCount := len(*previousEvents)
	if len(checkpoint.Events) < previousEventCount {
		t.Fatalf("authoritative event history length = %d, want at least %d",
			len(checkpoint.Events), previousEventCount)
	}
	if previousEventCount > 0 &&
		!reflect.DeepEqual(checkpoint.Events[:previousEventCount], *previousEvents) {
		t.Fatal("authoritative event history did not retain its prior prefix")
	}
	if got := len(checkpoint.Events) - previousEventCount; got != len(result.Events) {
		t.Fatalf("stored event growth = %d, returned viewer events = %d", got, len(result.Events))
	}
	*previousEvents = append([]event.Event(nil), checkpoint.Events...)
	return checkpoint
}

func assertPendingInputMatches(
	t *testing.T,
	stored state.PendingInput,
	returned snapshot.PendingInput,
) {
	t.Helper()
	allowed := make([]string, len(stored.AllowedCommands))
	for i, commandType := range stored.AllowedCommands {
		allowed[i] = string(commandType)
	}
	if stored.ID != returned.ID ||
		stored.ActorID != returned.ActorID ||
		stored.Segment != returned.Segment ||
		stored.Phase != returned.Phase ||
		stored.Stage != returned.Stage ||
		stored.Iteration != returned.Iteration ||
		stored.WindowID != returned.WindowID ||
		stored.ReactionRound != returned.ReactionRound ||
		stored.PlanningCycle != returned.PlanningCycle ||
		stored.InputType != returned.InputType ||
		stored.SourceType != returned.SourceType ||
		stored.SourceID != returned.SourceID ||
		!reflect.DeepEqual(allowed, returned.AllowedCommands) {
		t.Fatalf("stored pending input and returned pending input disagree:\nstored=%#v\nreturned=%#v",
			stored, returned)
	}
}

func assertParticipantsAtCheckpoint(
	t *testing.T,
	checkpoint repository.Checkpoint,
	initial map[string]state.ActorSetup,
) {
	t.Helper()
	battle := checkpoint.Battle
	wantIDs := []string{"goblin-1", "goblin-2", "player"}
	gotIDs := make([]string, 0, len(battle.Actors))
	for actorID := range battle.Actors {
		gotIDs = append(gotIDs, actorID)
	}
	sort.Strings(gotIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("actor IDs = %#v, want %#v", gotIDs, wantIDs)
	}
	for _, actorID := range wantIDs {
		actor := battle.Actors[actorID]
		baseline := initial[actorID]
		wantDefinition := "mock_goblin"
		wantController := state.ControllerAI
		if actorID == "player" {
			wantDefinition = "current_run_player"
			wantController = state.ControllerHuman
		}
		if actor.DefinitionID != wantDefinition || actor.Controller != wantController {
			t.Fatalf("actor %q identity = definition %q controller %q, want %q %q",
				actorID, actor.DefinitionID, actor.Controller, wantDefinition, wantController)
		}
		if currentHealth(actor) != setupHealth(baseline) {
			t.Fatalf("actor %q health-card count = %d, want %d",
				actorID, currentHealth(actor), setupHealth(baseline))
		}
		if len(actor.Cards.Removed) != len(baseline.Removed) {
			t.Fatalf("actor %q removed cards = %d, want %d",
				actorID, len(actor.Cards.Removed), len(baseline.Removed))
		}
	}
	goblin1 := battle.Actors["goblin-1"]
	goblin2 := battle.Actors["goblin-2"]
	if goblin1.DefinitionID != "mock_goblin" || goblin2.DefinitionID != "mock_goblin" {
		t.Fatalf("goblin definitions = %q, %q", goblin1.DefinitionID, goblin2.DefinitionID)
	}
	statusIDs := make(map[string]string)
	for _, actorID := range wantIDs {
		for _, status := range battle.Actors[actorID].Statuses {
			if owner, exists := statusIDs[status.InstanceID]; exists {
				t.Fatalf("status instance %q is shared by actors %q and %q", status.InstanceID, owner, actorID)
			}
			statusIDs[status.InstanceID] = actorID
		}
	}
	if len(goblin1.Statuses) != 1 || len(goblin2.Statuses) != 1 ||
		goblin1.Statuses[0].InstanceID == goblin2.Statuses[0].InstanceID {
		t.Fatalf("goblin statuses are not independently scoped: %#v %#v", goblin1.Statuses, goblin2.Statuses)
	}
}

func assertInitialPlanningWait(
	t *testing.T,
	checkpoint repository.Checkpoint,
	result engine.Result,
	round int,
) {
	t.Helper()
	resolution, window := activeResolutionWindow(t, checkpoint.Battle)
	if resolution.ID != fmt.Sprintf("planning-offensive-round-%d", round) ||
		resolution.Planning == nil ||
		resolution.Planning.Segment != segment.Offensive ||
		resolution.Planning.Cycle != 1 ||
		window.Purpose != state.InteractionPurposePlanning ||
		window.RevealStatus != state.RevealStatusCollecting ||
		!window.HiddenCommitments {
		t.Fatalf("round %d planning resolution/window = %#v %#v", round, resolution, window)
	}
	assertActorSet(t, window.RequiredActors, "player", "goblin-1", "goblin-2")
	for _, enemyID := range []string{"goblin-1", "goblin-2"} {
		plan := resolution.Planning.Actors[enemyID]
		if !plan.Passed || !plan.LockedIn || plan.SelectedAbility != "" ||
			len(plan.SelectedTargets) != 0 || len(plan.CommittedCards) != 0 {
			t.Fatalf("automatic enemy plan %q = %#v, want locked pass", enemyID, plan)
		}
		if window.ActorProgress[enemyID] != state.InteractionActorCommitted {
			t.Fatalf("enemy %q planning progress = %q, want committed", enemyID, window.ActorProgress[enemyID])
		}
	}
	if result.Snapshot.Resolution == nil ||
		result.Snapshot.Resolution.ActiveWindow == nil ||
		len(result.Snapshot.Resolution.ActiveWindow.Commitments) != 0 {
		t.Fatalf("viewer snapshot exposed hidden enemy planning commitments: %#v", result.Snapshot.Resolution)
	}
}

func assertHiddenEnemyCommitmentsRemainAuthoritative(
	t *testing.T,
	stored []event.Event,
	viewer []event.Event,
	round int,
) {
	t.Helper()
	resolutionID := fmt.Sprintf("planning-offensive-round-%d", round)
	for _, enemyID := range []string{"goblin-1", "goblin-2"} {
		storedCommit := findCommittedEvent(t, stored, resolutionID, enemyID, state.InteractionPurposePlanning)
		if storedCommit.Commitment == nil || storedCommit.Commitment.Data.Planning == nil ||
			!storedCommit.Commitment.Data.Planning.Passed {
			t.Fatalf("stored enemy commitment %q is not an authoritative pass: %#v", enemyID, storedCommit)
		}
		viewerCommit := findCommittedEvent(t, viewer, resolutionID, enemyID, state.InteractionPurposePlanning)
		if viewerCommit.Commitment != nil {
			t.Fatalf("viewer event exposed hidden enemy commitment %q: %#v", enemyID, viewerCommit)
		}
	}
}

func assertPlanningPassWait(
	t *testing.T,
	checkpoint repository.Checkpoint,
	result engine.Result,
	round int,
	oldPendingID string,
) {
	t.Helper()
	resolution, window := activeResolutionWindow(t, checkpoint.Battle)
	player := resolution.Planning.Actors["player"]
	if !player.Passed || player.LockedIn {
		t.Fatalf("round %d player plan after pass = %#v", round, player)
	}
	if window.RevealStatus != state.RevealStatusCollecting ||
		len(window.Commitments) != 2 ||
		window.ActorProgress["player"] != state.InteractionActorAwaiting {
		t.Fatalf("round %d planning window revealed or advanced early: %#v", round, window)
	}
	pending := checkpoint.Battle.Flow.PendingInput["player"]
	if pending.ID == oldPendingID ||
		!containsCommand(pending.AllowedCommands, command.TypePlanningLockIn) {
		t.Fatalf("round %d pending input after pass = %#v", round, pending)
	}
	if result.Snapshot.Resolution == nil ||
		len(result.Snapshot.Resolution.ActiveWindow.Commitments) != 0 {
		t.Fatalf("round %d viewer saw hidden commitments before lock: %#v", round, result.Snapshot.Resolution)
	}
}

func assertOffensiveRevealAndReactionWait(
	t *testing.T,
	checkpoint repository.Checkpoint,
	result engine.Result,
	round int,
) {
	t.Helper()
	resolution, window := activeResolutionWindow(t, checkpoint.Battle)
	if resolution.ID != fmt.Sprintf("planning-offensive-round-%d", round) ||
		window.Purpose != state.InteractionPurposeReaction ||
		window.ReactionRound != 1 ||
		window.RevealStatus != state.RevealStatusCollecting {
		t.Fatalf("round %d reaction wait = resolution %#v window %#v", round, resolution, window)
	}
	assertActorSet(t, window.RequiredActors, "player", "goblin-1", "goblin-2")
	if window.ActorProgress["player"] != state.InteractionActorAwaiting ||
		window.ActorProgress["goblin-1"] != state.InteractionActorPassed ||
		window.ActorProgress["goblin-2"] != state.InteractionActorPassed {
		t.Fatalf("round %d reaction progress = %#v", round, window.ActorProgress)
	}
	for _, enemyID := range []string{"goblin-1", "goblin-2"} {
		commitment := window.Commitments[enemyID]
		if !commitment.Passed || commitment.Command != command.TypePass {
			t.Fatalf("round %d enemy reaction %q = %#v, want pass", round, enemyID, commitment)
		}
	}
	reveal := findRevealEvent(t, checkpoint.Events, resolution.ID, state.InteractionPurposePlanning)
	assertAllPassPlanningReveal(t, reveal, round)
	responseReveal := findRevealEvent(t, result.Events, resolution.ID, state.InteractionPurposePlanning)
	assertAllPassPlanningReveal(t, responseReveal, round)
}

func assertRoundCompleted(
	t *testing.T,
	checkpoint repository.Checkpoint,
	result engine.Result,
	completedRound int,
) {
	t.Helper()
	if checkpoint.Battle.Segment.Round != completedRound+1 ||
		checkpoint.Battle.Segment.Current != segment.Offensive {
		t.Fatalf("after round %d state = %s round %d", completedRound,
			checkpoint.Battle.Segment.Current, checkpoint.Battle.Segment.Round)
	}
	completed := checkpoint.Battle.Resolutions[fmt.Sprintf("planning-offensive-round-%d", completedRound)]
	if completed.Planning == nil || !completed.Planning.Finalized ||
		completed.Stage != state.ResolutionComplete ||
		completed.ActiveWindowID != "" ||
		len(completed.Windows) != 2 {
		t.Fatalf("completed round %d planning resolution = %#v", completedRound, completed)
	}
	reactionReveal := findRevealEvent(t, checkpoint.Events, completed.ID, state.InteractionPurposeReaction)
	if reactionReveal.ReactionRound != 1 || len(reactionReveal.Commitments) != 3 {
		t.Fatalf("round %d reaction reveal = %#v", completedRound, reactionReveal)
	}
	for _, commitment := range reactionReveal.Commitments {
		if !commitment.Passed {
			t.Fatalf("round %d reaction commitment did not pass: %#v", completedRound, commitment)
		}
	}
	for _, battleEvent := range result.Events {
		if battleEvent.Type == event.TypeInteractionWindowOpened &&
			battleEvent.ResolutionID == completed.ID &&
			battleEvent.Purpose == state.InteractionPurposeReaction &&
			battleEvent.ReactionRound > 1 {
			t.Fatalf("round %d opened unnecessary reaction round: %#v", completedRound, battleEvent)
		}
	}
}

func assertCompletedRoundPlanning(t *testing.T, battle state.Battle, round int) {
	t.Helper()
	resolutionID := fmt.Sprintf("planning-offensive-round-%d", round)
	resolution, ok := battle.Resolutions[resolutionID]
	if !ok || resolution.Planning == nil || !resolution.Planning.Finalized {
		t.Fatalf("completed planning resolution %q is missing or incomplete", resolutionID)
	}
	if len(resolution.Planning.Actors) != 3 {
		t.Fatalf("%s actor count = %d, want 3", resolutionID, len(resolution.Planning.Actors))
	}
	for _, actorID := range []string{"player", "goblin-1", "goblin-2"} {
		plan := resolution.Planning.Actors[actorID]
		if !plan.Passed || !plan.LockedIn || plan.SelectedAbility != "" ||
			len(plan.SelectedTargets) != 0 || len(plan.CommittedCards) != 0 {
			t.Fatalf("%s actor %q plan = %#v, want finalized pass", resolutionID, actorID, plan)
		}
	}
	if len(resolution.Batch.Proposals) != 3 || !resolution.Batch.Revealed || !resolution.Batch.Committed {
		t.Fatalf("%s proposal batch = %#v", resolutionID, resolution.Batch)
	}
	for _, proposal := range resolution.Batch.Proposals {
		if proposal.Data.Planning == nil || !proposal.Data.Planning.Passed ||
			proposal.Data.Planning.SelectedAbility != "" ||
			len(proposal.Data.Planning.SelectedTargets) != 0 {
			t.Fatalf("%s proposal = %#v, want pass without target or ability", resolutionID, proposal)
		}
	}
}

func assertRoundPlanningIDsDiffer(t *testing.T, battle state.Battle) {
	t.Helper()
	round1 := battle.Resolutions["planning-offensive-round-1"]
	round2 := battle.Resolutions["planning-offensive-round-2"]
	if round1.ID == round2.ID {
		t.Fatalf("round planning resolution IDs collide: %q", round1.ID)
	}
	if planningWindowID(round1) == planningWindowID(round2) {
		t.Fatalf("round planning window IDs collide: %q", planningWindowID(round1))
	}
}

func planningWindowID(resolution state.ResolutionState) string {
	for windowID, window := range resolution.Windows {
		if window.Purpose == state.InteractionPurposePlanning {
			return windowID
		}
	}
	return ""
}

func assertFinalIncomeState(
	t *testing.T,
	checkpoint repository.Checkpoint,
	initial map[string]state.ActorSetup,
) {
	t.Helper()
	player := checkpoint.Battle.Actors["player"]
	baseline := initial["player"]
	wantDraws := minInt(3, baseline.Resources.MaxHandSize-len(baseline.Hand))
	if len(player.Cards.Hand)-len(baseline.Hand) != wantDraws ||
		len(baseline.Deck)-len(player.Cards.Deck) != wantDraws {
		t.Fatalf("player card movement = hand %+d deck %+d, want hand +%d deck -%d",
			len(player.Cards.Hand)-len(baseline.Hand),
			len(player.Cards.Deck)-len(baseline.Deck),
			wantDraws,
			wantDraws,
		)
	}
	if currentHealth(player) != setupHealth(baseline) {
		t.Fatalf("player health-card count = %d, want unchanged %d",
			currentHealth(player), setupHealth(baseline))
	}
	drawEvents := eventsOfType(checkpoint.Events, event.TypeCardsDrawn)
	if len(drawEvents) != 3 {
		t.Fatalf("cards_drawn event count = %d, want 3", len(drawEvents))
	}
	for _, draw := range drawEvents {
		if draw.ActorID != "player" || len(draw.Cards) != 1 {
			t.Fatalf("income draw event = %#v, want one player card", draw)
		}
	}
	for _, enemyID := range []string{"goblin-1", "goblin-2"} {
		enemy := checkpoint.Battle.Actors[enemyID]
		enemyBaseline := initial[enemyID]
		if !reflect.DeepEqual(enemy.Cards.Deck, enemyBaseline.Deck) ||
			!reflect.DeepEqual(enemy.Cards.Hand, enemyBaseline.Hand) ||
			!reflect.DeepEqual(enemy.Cards.Discard, enemyBaseline.Discard) {
			t.Fatalf("enemy %q received income or changed card zones", enemyID)
		}
	}
}

func assertAuthoritativeBattleRoute(t *testing.T, events []event.Event) {
	t.Helper()
	wantEntries := []segmentEntry{
		{segment.OngoingEffects, 1},
		{segment.Income, 1},
		{segment.Offensive, 1},
		{segment.Defensive, 1},
		{segment.DamageResolution, 1},
		{segment.OngoingEffects, 2},
		{segment.Income, 2},
		{segment.Offensive, 2},
		{segment.Defensive, 2},
		{segment.DamageResolution, 2},
		{segment.OngoingEffects, 3},
		{segment.Income, 3},
		{segment.Offensive, 3},
	}
	var gotEntries []segmentEntry
	entryCounts := make(map[segment.Segment]int)
	seenEntry := make(map[segmentEntry]bool)
	var advances []event.Event
	for _, battleEvent := range events {
		switch battleEvent.Type {
		case event.TypeSegmentEntered:
			entry := segmentEntry{battleEvent.Segment, battleEvent.Round}
			if seenEntry[entry] {
				t.Fatalf("segment entered twice: %#v", entry)
			}
			seenEntry[entry] = true
			gotEntries = append(gotEntries, entry)
			entryCounts[entry.segment]++
		case event.TypeSegmentAdvanced:
			advances = append(advances, battleEvent)
		}
	}
	if !reflect.DeepEqual(gotEntries, wantEntries) {
		t.Fatalf("segment entry route = %#v, want %#v", gotEntries, wantEntries)
	}
	wantCounts := map[segment.Segment]int{
		segment.OngoingEffects:   3,
		segment.Income:           3,
		segment.Offensive:        3,
		segment.Defensive:        2,
		segment.DamageResolution: 2,
	}
	if !reflect.DeepEqual(entryCounts, wantCounts) {
		t.Fatalf("segment entry counts = %#v, want %#v", entryCounts, wantCounts)
	}
	if len(advances) != len(wantEntries)-1 {
		t.Fatalf("segment advance count = %d, want %d", len(advances), len(wantEntries)-1)
	}
	for i, advance := range advances {
		from := wantEntries[i]
		to := wantEntries[i+1]
		if advance.From != from.segment || advance.To != to.segment || advance.Round != to.round {
			t.Fatalf("advance %d = %#v, want %s round %d -> %s round %d",
				i, advance, from.segment, from.round, to.segment, to.round)
		}
		wantCompletedTurn := from.segment == segment.DamageResolution
		if advance.CompletedTurn != wantCompletedTurn {
			t.Fatalf("advance %s round %d completed_turn = %v, want %v",
				advance.From, from.round, advance.CompletedTurn, wantCompletedTurn)
		}
	}
	wraps := 0
	for _, advance := range advances {
		if advance.CompletedTurn {
			wraps++
			if advance.From != segment.DamageResolution ||
				advance.To != segment.OngoingEffects {
				t.Fatalf("completed turn has invalid route: %#v", advance)
			}
		}
	}
	if wraps != 2 {
		t.Fatalf("completed turn count = %d, want 2", wraps)
	}

	for round := 1; round <= 2; round++ {
		resolutionID := fmt.Sprintf("planning-offensive-round-%d", round)
		planningReveal := eventIndex(events, func(value event.Event) bool {
			return value.Type == event.TypeInteractionRevealed &&
				value.ResolutionID == resolutionID &&
				value.Purpose == state.InteractionPurposePlanning
		})
		finalized := eventIndex(events, func(value event.Event) bool {
			return value.Type == event.TypeProposalBatchCommitted && value.ResolutionID == resolutionID
		})
		defensiveEntry := eventIndex(events, func(value event.Event) bool {
			return value.Type == event.TypeSegmentEntered &&
				value.Segment == segment.Defensive &&
				value.Round == round
		})
		if planningReveal < 0 || finalized < 0 || defensiveEntry < 0 ||
			!(planningReveal < finalized && finalized < defensiveEntry) {
			t.Fatalf("round %d event order reveal=%d finalized=%d defensive=%d",
				round, planningReveal, finalized, defensiveEntry)
		}
	}
}

func assertNoDamageOrDefensiveWork(t *testing.T, checkpoint repository.Checkpoint) {
	t.Helper()
	battle := checkpoint.Battle
	if len(battle.DefensiveProposals) != 0 ||
		battle.DamageResolution != nil ||
		len(battle.PendingOperations) != 0 {
		t.Fatalf("all-pass path retained defensive/damage work: defensive=%#v damage=%#v pending=%#v",
			battle.DefensiveProposals, battle.DamageResolution, battle.PendingOperations)
	}
	for _, proposal := range battle.OffensiveProposals {
		if proposal.Defensible || len(proposal.Operations) != 0 {
			t.Fatalf("current all-pass offensive proposal produced work: %#v", proposal)
		}
	}
	for _, battleEvent := range checkpoint.Events {
		switch battleEvent.Type {
		case event.TypeDamageProposed,
			event.TypeDamageCardsRevealed,
			event.TypeDamageModified,
			event.TypeDamageCommitted,
			event.TypeCardsPermanentlyRemoved:
			t.Fatalf("all-pass path emitted damage event: %#v", battleEvent)
		}
		if (battleEvent.Type == event.TypeInteractionWindowOpened ||
			battleEvent.Type == event.TypeInteractionRevealed) &&
			strings.Contains(battleEvent.ResolutionID, "defensive") {
			t.Fatalf("all-pass path opened defensive input: %#v", battleEvent)
		}
	}
}

type segmentEntry struct {
	segment segment.Segment
	round   int
}

func activeResolutionWindow(
	t *testing.T,
	battle state.Battle,
) (state.ResolutionState, state.InteractionWindow) {
	t.Helper()
	resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
	if !ok {
		t.Fatalf("active resolution %q is missing", battle.ActiveResolutionID)
	}
	window, ok := resolution.Windows[resolution.ActiveWindowID]
	if !ok {
		t.Fatalf("active window %q is missing", resolution.ActiveWindowID)
	}
	return resolution, window
}

func findCommittedEvent(
	t *testing.T,
	events []event.Event,
	resolutionID string,
	actorID string,
	purpose state.InteractionPurpose,
) event.Event {
	t.Helper()
	for _, battleEvent := range events {
		if battleEvent.Type == event.TypeInteractionCommitted &&
			battleEvent.ResolutionID == resolutionID &&
			battleEvent.ActorID == actorID &&
			battleEvent.Purpose == purpose {
			return battleEvent
		}
	}
	t.Fatalf("missing committed event for resolution %q actor %q purpose %q", resolutionID, actorID, purpose)
	return event.Event{}
}

func findRevealEvent(
	t *testing.T,
	events []event.Event,
	resolutionID string,
	purpose state.InteractionPurpose,
) event.Event {
	t.Helper()
	for _, battleEvent := range events {
		if battleEvent.Type == event.TypeInteractionRevealed &&
			battleEvent.ResolutionID == resolutionID &&
			battleEvent.Purpose == purpose {
			return battleEvent
		}
	}
	t.Fatalf("missing reveal event for resolution %q purpose %q", resolutionID, purpose)
	return event.Event{}
}

func assertAllPassPlanningReveal(t *testing.T, reveal event.Event, round int) {
	t.Helper()
	if len(reveal.Commitments) != 3 {
		t.Fatalf("round %d planning reveal commitments = %d, want 3", round, len(reveal.Commitments))
	}
	actorIDs := make([]string, 0, len(reveal.Commitments))
	for _, commitment := range reveal.Commitments {
		actorIDs = append(actorIDs, commitment.ActorID)
		planning := commitment.Data.Planning
		if planning == nil || !planning.Passed || !planning.LockedIn ||
			planning.SelectedAbility != "" ||
			len(planning.SelectedTargets) != 0 ||
			len(planning.CommittedCards) != 0 {
			t.Fatalf("round %d revealed commitment = %#v, want locked pass", round, commitment)
		}
	}
	assertActorSet(t, actorIDs, "player", "goblin-1", "goblin-2")
}

func assertActorSet(t *testing.T, got []string, want ...string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actor set = %#v, want %#v", got, want)
	}
}

func containsCommand(values []command.Type, target command.Type) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func eventsOfType(events []event.Event, eventType event.Type) []event.Event {
	var matched []event.Event
	for _, battleEvent := range events {
		if battleEvent.Type == eventType {
			matched = append(matched, battleEvent)
		}
	}
	return matched
}

func eventIndex(events []event.Event, match func(event.Event) bool) int {
	for i, battleEvent := range events {
		if match(battleEvent) {
			return i
		}
	}
	return -1
}

func currentHealth(actor state.ActorState) int {
	return len(actor.Cards.Deck) + len(actor.Cards.Hand) + len(actor.Cards.Discard)
}

func setupHealth(actor state.ActorSetup) int {
	return len(actor.Deck) + len(actor.Hand) + len(actor.Discard)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
