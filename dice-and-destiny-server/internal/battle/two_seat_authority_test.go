package battle

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestAuthorityStartsTwoExternalBladeWardenSeatsWithPrivatePlanning(t *testing.T) {
	authority := newTwoSeatTestAuthority(t)
	started := startTwoSeatBattle(t, authority, "private-planning", 41)
	if started.Snapshot.Actors["seat-a"].Controller != state.ControllerExternal || started.Snapshot.Actors["seat-b"].Controller != state.ControllerExternal {
		t.Fatalf("controllers = %#v", started.Snapshot.Actors)
	}
	if len(started.LegalActions) == 0 || started.Snapshot.PriorityActorID != "" {
		t.Fatalf("initial external planning result = %#v", started)
	}
	checkpoint, err := authority.repo.Load("private-planning")
	if err != nil {
		t.Fatal(err)
	}
	for actorID, runtime := range checkpoint.Battle.Settled.Actors {
		if runtime.AID100 != 0 || runtime.AISimulatedRolls != 0 {
			t.Fatalf("external seat %s used D100 planning: %#v", actorID, runtime)
		}
	}
	seatBBefore := openTwoSeatBattle(t, authority, "private-planning", "seat-b")
	beforeOpponent := seatBBefore.Snapshot.Actors["seat-a"]

	roll := requireLegalAction(t, started, command.TypePlanningRoll)
	afterRoll := sendTwoSeatCommand(t, authority, roll)
	if len(afterRoll.Snapshot.Actors["seat-a"].RollHistory) == 0 {
		t.Fatal("seat A cannot see its own private roll")
	}
	if stale := sendTwoSeatCommandAllowReject(t, authority, roll); stale.Accepted || !strings.Contains(stale.Error, "stale") {
		t.Fatalf("stale legal action = %#v", stale)
	}
	for _, action := range afterRoll.LegalActions {
		if action.Type != command.TypePlanningCards {
			continue
		}
		var payload command.PlanningCardsPayload
		if err := json.Unmarshal(action.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		payload.CardIDs = []string{"fabricated-card"}
		action.Payload, _ = json.Marshal(payload)
		if fabricated := sendTwoSeatCommandAllowReject(t, authority, action); fabricated.Accepted || !strings.Contains(fabricated.Error, "does not exist") {
			t.Fatalf("fabricated command = %#v", fabricated)
		}
		break
	}
	seatBAfterRoll := openTwoSeatBattle(t, authority, "private-planning", "seat-b")
	if len(seatBAfterRoll.Snapshot.Actors["seat-a"].RollHistory) != 0 || seatBAfterRoll.Snapshot.Actors["seat-a"].SelectedAbility != "" {
		t.Fatalf("seat A planning leaked to seat B: %#v", seatBAfterRoll.Snapshot.Actors["seat-a"])
	}

	if focus, ok := findPlanningCardAction(afterRoll, afterRoll.Snapshot, "seat-a", "battle_focus"); ok {
		afterFocus := sendTwoSeatCommand(t, authority, focus)
		seatBAfterFocus := openTwoSeatBattle(t, authority, "private-planning", "seat-b")
		opponent := seatBAfterFocus.Snapshot.Actors["seat-a"]
		if opponent.HandCount != beforeOpponent.HandCount || opponent.DeckCount != beforeOpponent.DeckCount || opponent.DiscardCount != beforeOpponent.DiscardCount || opponent.EnergyPoints != beforeOpponent.EnergyPoints {
			t.Fatalf("hidden planning side effects leaked: before=%#v after=%#v", beforeOpponent, opponent)
		}
		if len(afterFocus.LegalActions) == 0 {
			t.Fatal("seat A lost its planning input after playing a card")
		}
	}
}

func TestSettledReactionPriorityRotatesByRoundAndRejectsWrongSeat(t *testing.T) {
	authority := newTwoSeatTestAuthority(t)
	result := startTwoSeatBattle(t, authority, "reaction-priority", 7)
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))
	seatB := openTwoSeatBattle(t, authority, "reaction-priority", "seat-b")
	seatB = sendTwoSeatCommand(t, authority, requireLegalAction(t, seatB, command.TypePlanningPass))
	if seatB.Snapshot.Stage != stageOffensiveReactForTest || seatB.Snapshot.PriorityActorID != "seat-a" || strings.Join(seatB.Snapshot.ReactionPriority, ",") != "seat-a,seat-b" {
		t.Fatalf("round 1 priority = %#v", seatB.Snapshot)
	}
	seatA := openTwoSeatBattle(t, authority, "reaction-priority", "seat-a")
	wrong := requireLegalAction(t, seatA, command.TypePass)
	wrong.ActorID = "seat-b"
	if rejected := sendTwoSeatCommandAllowReject(t, authority, wrong); rejected.Accepted || !strings.Contains(rejected.Error, "does not own") {
		t.Fatalf("wrong-seat reaction = %#v", rejected)
	}
	seatA = sendTwoSeatCommand(t, authority, requireLegalAction(t, seatA, command.TypePass))
	if seatA.Snapshot.PriorityActorID != "seat-b" {
		t.Fatalf("priority after seat A pass = %#v", seatA.Snapshot)
	}
	seatB = openTwoSeatBattle(t, authority, "reaction-priority", "seat-b")
	seatB = sendTwoSeatCommand(t, authority, requireLegalAction(t, seatB, command.TypePass))
	if seatB.Snapshot.Round != 2 || seatB.Snapshot.Stage != "planning" {
		t.Fatalf("round 1 did not complete = %#v", seatB.Snapshot)
	}
	seatA = openTwoSeatBattle(t, authority, "reaction-priority", "seat-a")
	seatA = sendTwoSeatCommand(t, authority, requireLegalAction(t, seatA, command.TypePlanningPass))
	seatB = openTwoSeatBattle(t, authority, "reaction-priority", "seat-b")
	seatB = sendTwoSeatCommand(t, authority, requireLegalAction(t, seatB, command.TypePlanningPass))
	if seatB.Snapshot.PriorityActorID != "seat-b" || strings.Join(seatB.Snapshot.ReactionPriority, ",") != "seat-b,seat-a" {
		t.Fatalf("round 2 priority = %#v", seatB.Snapshot)
	}
}

func TestAuthorityTipItEnumeratesAndAcceptsOnlyRevealedFaceSixDice(t *testing.T) {
	authority := newTwoSeatTestAuthority(t)
	result := startTwoSeatBattle(t, authority, "tip-it-targets", 61)
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))
	result = openTwoSeatBattle(t, authority, "tip-it-targets", "seat-b")
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))

	checkpoint, err := authority.repo.Load("tip-it-targets")
	if err != nil {
		t.Fatal(err)
	}
	putDefinitionCardInHand(t, &checkpoint.Battle, "seat-a", "tip_it")
	setTestEnergy(&checkpoint.Battle, "seat-a", 5)
	setTestFinalDice(&checkpoint.Battle, "seat-a", 6, 4, 3)
	setTestFinalDice(&checkpoint.Battle, "seat-b", 2, 6, 5)
	if err := authority.repo.Save(checkpoint); err != nil {
		t.Fatal(err)
	}

	result = openTwoSeatBattle(t, authority, "tip-it-targets", "seat-a")
	tipActions := legalCardActionsByDefinition(t, result, "seat-a", "tip_it")
	if len(tipActions) != 2 {
		t.Fatalf("Tip It actions = %d, want the two revealed face-6 dice: %#v", len(tipActions), tipActions)
	}
	targets := map[string]bool{}
	for _, action := range tipActions {
		adjustment := singleReactionDieAdjustment(t, action)
		if adjustment.Face != 5 {
			t.Fatalf("Tip It adjustment = %#v, want canonical face 5", adjustment)
		}
		targets[adjustment.ActorID+":"+string(rune('0'+adjustment.DieIndex))] = true
	}
	if !targets["seat-a:0"] || !targets["seat-b:1"] || len(targets) != 2 {
		t.Fatalf("Tip It targets = %#v, want only seat-a:0 and seat-b:1", targets)
	}

	fabricated := tipActions[0]
	var payload command.CommitInteractionPayload
	if err := json.Unmarshal(fabricated.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload.Commitment.PlanningAdjustments[0].ActorID = "seat-a"
	payload.Commitment.PlanningAdjustments[0].DieIndex = 1
	fabricated.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if rejected := sendTwoSeatCommandAllowReject(t, authority, fabricated); rejected.Accepted || !strings.Contains(rejected.Error, "face 6") {
		t.Fatalf("fabricated non-face-6 Tip It command = %#v", rejected)
	}

	accepted := sendTwoSeatCommand(t, authority, tipActions[0])
	if !accepted.Accepted {
		t.Fatalf("face-6 Tip It command was rejected: %#v", accepted)
	}
	checkpoint, err = authority.repo.Load("tip-it-targets")
	if err != nil {
		t.Fatal(err)
	}
	adjustment := singleReactionDieAdjustment(t, tipActions[0])
	if got := checkpoint.Battle.Settled.Actors[adjustment.ActorID].FinalDice[adjustment.DieIndex].Face; got != 5 {
		t.Fatalf("accepted Tip It target face = %d, want 5", got)
	}
}

func TestAuthorityBlindTipItUsesOneCanonicalPendingDieTarget(t *testing.T) {
	authority := newTwoSeatTestAuthority(t)
	result := startTwoSeatBattle(t, authority, "blind-tip-it", 67)
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))
	result = openTwoSeatBattle(t, authority, "blind-tip-it", "seat-b")
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))

	checkpoint, err := authority.repo.Load("blind-tip-it")
	if err != nil {
		t.Fatal(err)
	}
	putDefinitionCardInHand(t, &checkpoint.Battle, "seat-a", "tip_it")
	setTestEnergy(&checkpoint.Battle, "seat-a", 5)
	setTestFinalDice(&checkpoint.Battle, "seat-a", 6, 6, 6, 6, 6)
	checkpoint.Battle.Settled.PendingBlind = &state.SettledBlindResolution{ActorID: "seat-b", StatusID: "blind", DieID: "standard_d6", Face: 6}
	checkpoint.Battle.Settled.Stage = "blind_reaction"
	checkpoint.Battle.Settled.Window.Stage = "blind_reaction"
	checkpoint.Battle.Flow.Stage = "blind_reaction"
	pending := checkpoint.Battle.Flow.PendingInput["seat-a"]
	pending.Stage = "blind_reaction"
	checkpoint.Battle.Flow.PendingInput["seat-a"] = pending
	if err := authority.repo.Save(checkpoint); err != nil {
		t.Fatal(err)
	}

	result = openTwoSeatBattle(t, authority, "blind-tip-it", "seat-a")
	tipActions := legalCardActionsByDefinition(t, result, "seat-a", "tip_it")
	if len(tipActions) != 1 {
		t.Fatalf("Blind Tip It actions = %d, want one canonical pending-die action: %#v", len(tipActions), tipActions)
	}
	adjustment := singleReactionDieAdjustment(t, tipActions[0])
	if adjustment.ActorID != "seat-b" || adjustment.DieIndex != 0 || adjustment.Face != 5 {
		t.Fatalf("Blind Tip It adjustment = %#v, want seat-b canonical pending die", adjustment)
	}
	sendTwoSeatCommand(t, authority, tipActions[0])
	checkpoint, err = authority.repo.Load("blind-tip-it")
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Battle.Settled.PendingBlind == nil || checkpoint.Battle.Settled.PendingBlind.Face != 5 {
		t.Fatalf("Blind pending die after Tip It = %#v, want face 5", checkpoint.Battle.Settled.PendingBlind)
	}
}

func TestTwoExternalBasicDefenseRollsResolveSequentially(t *testing.T) {
	authority := newTwoSeatTestAuthority(t)
	result := startTwoSeatBattle(t, authority, "two-seat-defense-rolls", 71)
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))
	result = openTwoSeatBattle(t, authority, "two-seat-defense-rolls", "seat-b")
	result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePlanningPass))

	checkpoint, err := authority.repo.Load("two-seat-defense-rolls")
	if err != nil {
		t.Fatal(err)
	}
	for actorID, targetID := range map[string]string{"seat-a": "seat-b", "seat-b": "seat-a"} {
		setTestFinalDice(&checkpoint.Battle, actorID, 1, 2, 3, 4, 5)
		runtime := checkpoint.Battle.Settled.Actors[actorID]
		runtime.QualifiedAbilityIDs = []string{"sword_cut"}
		runtime.SelectedAbilityID = "sword_cut"
		runtime.SelectedTierID = "three_swords"
		runtime.SelectedTargetIDs = []string{targetID}
		checkpoint.Battle.Settled.Actors[actorID] = runtime
	}
	if err := authority.repo.Save(checkpoint); err != nil {
		t.Fatal(err)
	}

	for _, actorID := range []string{"seat-a", "seat-b"} {
		result = openTwoSeatBattle(t, authority, "two-seat-defense-rolls", actorID)
		result = sendTwoSeatCommand(t, authority, requireLegalAction(t, result, command.TypePass))
	}
	if result.Snapshot.Stage != "defense_selection" || result.Snapshot.PriorityActorID == "" || len(result.Snapshot.SettledSources) != 2 {
		t.Fatalf("mutual attacks did not reach defense selection: %#v", result.Snapshot)
	}

	firstDefender := result.Snapshot.PriorityActorID
	secondDefender := otherTwoSeatActor(firstDefender)
	result = openTwoSeatBattle(t, authority, "two-seat-defense-rolls", firstDefender)
	result = sendTwoSeatCommand(t, authority, requireAbilityAction(t, result, "basic_defense"))
	if result.Snapshot.PriorityActorID != secondDefender {
		t.Fatalf("defense selection priority after first defender = %q, want %q", result.Snapshot.PriorityActorID, secondDefender)
	}
	result = openTwoSeatBattle(t, authority, "two-seat-defense-rolls", secondDefender)
	result = sendTwoSeatCommand(t, authority, requireAbilityAction(t, result, "basic_defense"))
	if result.Snapshot.Stage != "defense_roll" || result.Snapshot.PriorityActorID != firstDefender {
		t.Fatalf("first defense roll owner = %#v, want %s", result.Snapshot, firstDefender)
	}

	secondView := openTwoSeatBattle(t, authority, "two-seat-defense-rolls", secondDefender)
	if len(secondView.LegalActions) != 0 {
		t.Fatalf("non-priority defender has legal roll actions: %#v", secondView.LegalActions)
	}
	if _, leaked := secondView.Snapshot.SettledDefenses[firstDefender]; leaked {
		t.Fatalf("unresolved opponent defense leaked to %s: %#v", secondDefender, secondView.Snapshot.SettledDefenses)
	}

	firstView := openTwoSeatBattle(t, authority, "two-seat-defense-rolls", firstDefender)
	firstRoll := requireLegalAction(t, firstView, command.TypeRollDice)
	firstPayload := requireCompleteRollPayload(t, firstRoll)
	wrongSeat := firstRoll
	wrongSeat.ActorID = secondDefender
	if rejected := sendTwoSeatCommandAllowReject(t, authority, wrongSeat); rejected.Accepted || !strings.Contains(rejected.Error, "does not own") {
		t.Fatalf("wrong-seat defense roll = %#v", rejected)
	}

	firstRolled := sendTwoSeatCommand(t, authority, firstRoll)
	if firstRolled.Snapshot.Stage != "defense_roll" || firstRolled.Snapshot.PriorityActorID != secondDefender {
		t.Fatalf("priority after first defense roll = %#v, want %s", firstRolled.Snapshot, secondDefender)
	}
	if firstRolled.Snapshot.SettledDefenses[firstDefender].RolledFace == 0 {
		t.Fatalf("first defense roll did not resolve: %#v", firstRolled.Snapshot.SettledDefenses)
	}
	if _, leaked := firstRolled.Snapshot.SettledDefenses[secondDefender]; leaked {
		t.Fatalf("unresolved second defense roll leaked to %s: %#v", firstDefender, firstRolled.Snapshot.SettledDefenses)
	}

	secondView = openTwoSeatBattle(t, authority, "two-seat-defense-rolls", secondDefender)
	secondRoll := requireLegalAction(t, secondView, command.TypeRollDice)
	secondPayload := requireCompleteRollPayload(t, secondRoll)
	if secondPayload.PendingInputID == firstPayload.PendingInputID {
		t.Fatalf("sequential defenders share pending input ID %q", secondPayload.PendingInputID)
	}
	stale := secondRoll
	secondPayload.PendingInputID = firstPayload.PendingInputID
	stale.Payload, err = json.Marshal(secondPayload)
	if err != nil {
		t.Fatal(err)
	}
	if rejected := sendTwoSeatCommandAllowReject(t, authority, stale); rejected.Accepted || !strings.Contains(rejected.Error, "stale") {
		t.Fatalf("stale defense roll = %#v", rejected)
	}

	finishedRolls := sendTwoSeatCommand(t, authority, secondRoll)
	if finishedRolls.Snapshot.Stage != "defense_reaction" || finishedRolls.Snapshot.PriorityActorID == "" {
		t.Fatalf("automatic progression after both defense rolls = %#v", finishedRolls.Snapshot)
	}
	if len(finishedRolls.Snapshot.SettledDefenses) != 2 || finishedRolls.Snapshot.SettledDefenses[firstDefender].RolledFace == 0 || finishedRolls.Snapshot.SettledDefenses[secondDefender].RolledFace == 0 {
		t.Fatalf("revealed sequential defense rolls = %#v, want both resolved", finishedRolls.Snapshot.SettledDefenses)
	}
}

func TestTwoExternalBladeWardensCompleteDeterministicallyWithFreshLegalActions(t *testing.T) {
	firstStatus, firstWinner, firstRounds, firstActions := runLegalMirrorBattle(t, "mirror-e2e-a", 20260801)
	secondStatus, secondWinner, secondRounds, secondActions := runLegalMirrorBattle(t, "mirror-e2e-b", 20260801)
	if firstStatus != secondStatus || firstWinner != secondWinner || firstRounds != secondRounds || firstActions != secondActions {
		t.Fatalf("same-seed mirror mismatch: first=%s/%s/r%d/%d second=%s/%s/r%d/%d", firstStatus, firstWinner, firstRounds, firstActions, secondStatus, secondWinner, secondRounds, secondActions)
	}
	if !state.IsTerminalBattleStatus(firstStatus) || firstRounds < 1 || firstActions < 1 {
		t.Fatalf("mirror did not reach a valid terminal result: %s/%s/r%d/%d", firstStatus, firstWinner, firstRounds, firstActions)
	}
}

func runLegalMirrorBattle(t *testing.T, battleID string, seed uint64) (state.BattleStatus, string, int, int) {
	t.Helper()
	authority := newTwoSeatTestAuthority(t)
	result := startTwoSeatBattle(t, authority, battleID, seed)
	for actionCount := 0; actionCount < 1200; actionCount++ {
		if result.Status == engine.ProgressBattleComplete {
			return result.Snapshot.Status, result.Snapshot.WinnerActorID, result.Snapshot.CompletedRounds, actionCount
		}
		checkpoint, err := authority.repo.Load(battleID)
		if err != nil {
			t.Fatal(err)
		}
		var actorID string
		for _, candidate := range []string{"seat-a", "seat-b"} {
			if _, ok := checkpoint.Battle.Flow.PendingInput[candidate]; ok {
				actorID = candidate
				break
			}
		}
		if actorID == "" {
			t.Fatalf("no external input at nonterminal checkpoint: %#v", checkpoint.Battle)
		}
		result = openTwoSeatBattle(t, authority, battleID, actorID)
		if len(result.LegalActions) == 0 {
			t.Fatalf("%s has pending input without legal actions at %s r%d %s", actorID, result.Snapshot.Segment, result.Snapshot.Round, result.Snapshot.Stage)
		}
		assertFreshLegalActionsAccepted(t, checkpoint.Battle, result.LegalActions)
		result = sendTwoSeatCommand(t, authority, chooseMirrorAction(result.LegalActions))
	}
	t.Fatal("mirror battle exceeded action safety limit")
	return "", "", 0, 0
}

func chooseMirrorAction(actions []command.Command) command.Command {
	for _, kind := range []command.Type{command.TypePlanningRoll, command.TypePlanningAbility, command.TypePlanningReroll, command.TypeRollDice, command.TypePlanningPass, command.TypePass, command.TypeCommitInteraction} {
		var matches []command.Command
		for _, action := range actions {
			if action.Type == kind {
				matches = append(matches, action)
			}
		}
		if len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}
	return actions[0]
}

func assertFreshLegalActionsAccepted(t *testing.T, battleState state.Battle, actions []command.Command) {
	t.Helper()
	eng := engine.NewEngine()
	for _, action := range actions {
		clone := battleState.Clone()
		if _, err := eng.ApplyBattleCommand(&clone, action); err != nil {
			t.Fatalf("fresh legal action rejected: %s %s: %v", action.Type, action.Payload, err)
		}
	}
}

func newTwoSeatTestAuthority(t *testing.T) *Authority {
	t.Helper()
	root := participantTestServerRoot(t)
	return NewAuthority(engine.NewEngine(), repository.NewInMemory(), NewFileParticipantAssembler(filepath.Join(root, "content"), filepath.Join(root, "save", "run_players")))
}

func startTwoSeatBattle(t *testing.T, authority *Authority, battleID string, seed uint64) engine.Result {
	t.Helper()
	payload := command.StartBattlePayload{Seats: []command.ParticipantDescriptor{{InstanceID: "seat-a", DefinitionID: "blade_warden"}, {InstanceID: "seat-b", DefinitionID: "blade_warden"}}, Seed: &seed}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return sendTwoSeatCommand(t, authority, command.Command{BattleID: battleID, ActorID: "seat-a", Type: command.TypeStartBattle, Payload: encoded})
}

func openTwoSeatBattle(t *testing.T, authority *Authority, battleID, actorID string) engine.Result {
	t.Helper()
	return sendTwoSeatCommand(t, authority, command.Command{BattleID: battleID, ActorID: actorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
}

func sendTwoSeatCommand(t *testing.T, authority *Authority, action command.Command) engine.Result {
	t.Helper()
	result := sendTwoSeatCommandAllowReject(t, authority, action)
	if !result.Accepted {
		t.Fatalf("command rejected: %s %s: %s", action.Type, action.Payload, result.Error)
	}
	return result
}

func sendTwoSeatCommandAllowReject(t *testing.T, authority *Authority, action command.Command) engine.Result {
	t.Helper()
	encoded, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	var result engine.Result
	if err := json.Unmarshal([]byte(authority.HandleCommandJSON(string(encoded))), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func requireLegalAction(t *testing.T, result engine.Result, kind command.Type) command.Command {
	t.Helper()
	for _, action := range result.LegalActions {
		if action.Type == kind {
			return action
		}
	}
	t.Fatalf("missing %s action in %#v", kind, result.LegalActions)
	return command.Command{}
}

func requireAbilityAction(t *testing.T, result engine.Result, abilityID string) command.Command {
	t.Helper()
	for _, action := range result.LegalActions {
		if action.Type != command.TypePlanningAbility {
			continue
		}
		var payload command.PlanningAbilityPayload
		if json.Unmarshal(action.Payload, &payload) == nil && payload.AbilityID == abilityID {
			return action
		}
	}
	t.Fatalf("missing %s ability action in %#v", abilityID, result.LegalActions)
	return command.Command{}
}

func requireCompleteRollPayload(t *testing.T, action command.Command) command.RollDicePayload {
	t.Helper()
	if action.BattleID == "" || action.ActorID == "" || action.Type != command.TypeRollDice {
		t.Fatalf("incomplete roll command envelope: %#v", action)
	}
	var payload command.RollDicePayload
	if err := json.Unmarshal(action.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.PendingInputID == "" {
		t.Fatalf("roll action has no pending input ID: %#v", action)
	}
	return payload
}

func legalCardActionsByDefinition(t *testing.T, result engine.Result, actorID, definitionID string) []command.Command {
	t.Helper()
	var actions []command.Command
	for _, action := range result.LegalActions {
		if action.Type != command.TypeCommitInteraction {
			continue
		}
		var payload command.CommitInteractionPayload
		if json.Unmarshal(action.Payload, &payload) != nil || len(payload.Commitment.CardIDs) != 1 {
			continue
		}
		instance, ok := result.Snapshot.Actors[actorID].CardInstances[payload.Commitment.CardIDs[0]]
		if ok && instance.DefinitionID == definitionID {
			actions = append(actions, action)
		}
	}
	return actions
}

func singleReactionDieAdjustment(t *testing.T, action command.Command) command.PlanningAdjustment {
	t.Helper()
	var payload command.CommitInteractionPayload
	if err := json.Unmarshal(action.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Commitment.PlanningAdjustments) != 1 {
		t.Fatalf("reaction adjustments = %#v, want one", payload.Commitment.PlanningAdjustments)
	}
	return payload.Commitment.PlanningAdjustments[0]
}

func putDefinitionCardInHand(t *testing.T, battleState *state.Battle, actorID, definitionID string) string {
	t.Helper()
	runtime := battleState.Settled.Actors[actorID]
	instanceID := ""
	for candidateID, instance := range runtime.CardInstances {
		if instance.DefinitionID == definitionID {
			instanceID = candidateID
			break
		}
	}
	if instanceID == "" {
		t.Fatalf("%s has no %s card instance", actorID, definitionID)
	}
	actor := battleState.Actors[actorID]
	actor.Cards.Deck = removeTestCard(actor.Cards.Deck, instanceID)
	actor.Cards.Hand = removeTestCard(actor.Cards.Hand, instanceID)
	actor.Cards.Discard = removeTestCard(actor.Cards.Discard, instanceID)
	actor.Cards.Removed = removeTestCard(actor.Cards.Removed, instanceID)
	actor.Cards.Hand = append(actor.Cards.Hand, instanceID)
	battleState.Actors[actorID] = actor
	return instanceID
}

func removeTestCard(cards []string, instanceID string) []string {
	result := cards[:0]
	for _, cardID := range cards {
		if cardID != instanceID {
			result = append(result, cardID)
		}
	}
	return result
}

func setTestEnergy(battleState *state.Battle, actorID string, energy int) {
	actor := battleState.Actors[actorID]
	actor.Resources.EnergyPoints = energy
	actor.EnergyPoints = energy
	battleState.Actors[actorID] = actor
}

func setTestFinalDice(battleState *state.Battle, actorID string, faces ...int) {
	dice := make([]state.RolledDie, len(faces))
	for index, face := range faces {
		symbol := "gold_coin"
		if face <= 3 {
			symbol = "sword"
		} else if face <= 5 {
			symbol = "shield"
		}
		dice[index] = state.RolledDie{Index: index, DieID: "standard_d6", Face: face, Value: face, Symbols: []string{symbol}}
	}
	runtime := battleState.Settled.Actors[actorID]
	runtime.FinalDice = dice
	battleState.Settled.Actors[actorID] = runtime
}

func otherTwoSeatActor(actorID string) string {
	if actorID == "seat-a" {
		return "seat-b"
	}
	return "seat-a"
}

func findPlanningCardAction(result engine.Result, snap *snapshot.Battle, actorID, definitionID string) (command.Command, bool) {
	for _, action := range result.LegalActions {
		if action.Type != command.TypePlanningCards {
			continue
		}
		var payload command.PlanningCardsPayload
		if json.Unmarshal(action.Payload, &payload) == nil && len(payload.CardIDs) == 1 && snap.Actors[actorID].CardInstances[payload.CardIDs[0]].DefinitionID == definitionID {
			return action, true
		}
	}
	return command.Command{}, false
}

const stageOffensiveReactForTest = "offensive_reaction"
