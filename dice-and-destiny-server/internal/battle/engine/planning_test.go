package engine_test

import (
	"encoding/json"
	"reflect"
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

func TestSharedPlanningOffensiveDefensiveReactionReentryAndPersistence(t *testing.T) {
	battle := sharedPlanningBattle(t)
	controller := &planningAIController{}
	offensive, err := engine.NewOffensiveFlow(controller)
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	damageWait := &waitingFlow{id: segment.DamageResolution}
	eng, err := engine.NewEngineWithFlows(offensive, engine.DefensiveFlow{}, damageWait)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	started, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if started.Status != engine.ProgressWaitingForInput || controller.calls != 2 {
		t.Fatalf("start = %#v, AI calls = %d", started, controller.calls)
	}
	assertPlanningWindow(t, battle, segment.Offensive, 1)
	if battle.Flow.Actors["enemy"].Status != state.ActorLockedIn ||
		battle.Flow.Actors["spectator"].Status != state.ActorLockedIn ||
		battle.Flow.Actors["player"].Status != state.ActorNeedsInput {
		t.Fatalf("initial actor progress = %#v", battle.Flow.Actors)
	}
	assertNoPlanningSecret(t, battle, started.Events, "player", "Enemy Strike")

	repo := repository.NewInMemory()
	if err := repo.Create(repository.Checkpoint{Battle: battle, Events: started.Events}); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	battle = loadBattle(t, repo, battle.ID)

	applyPlanning(t, eng, &battle, command.TypePlanningRoll, nil)
	applyPlanning(t, eng, &battle, command.TypePlanningKeep, []int{0})
	applyPlanning(t, eng, &battle, command.TypePlanningReroll, []int{1})
	applyPlanning(t, eng, &battle, command.TypePlanningCards, []string{"Card A", "Card B"})
	applyPlanning(t, eng, &battle, command.TypePlanningAbility, "Hero Strike")
	applyPlanning(t, eng, &battle, command.TypePlanningTargets, []string{"enemy"})
	resolutionWithCosts := battle.Resolutions[battle.ActiveResolutionID]
	planWithCosts := resolutionWithCosts.Planning.Actors["player"]
	planWithCosts.PaidCostIDs = []string{"cost-1"}
	planWithCosts.ResolvedCardIDs = []string{"Card A"}
	resolutionWithCosts.Planning.Actors["player"] = planWithCosts
	battle.Resolutions[battle.ActiveResolutionID] = resolutionWithCosts

	beforeLock := battle.Clone()
	lockResult := applyPlanning(t, eng, &battle, command.TypePlanningLockIn, nil)
	if lockResult.Status != engine.ProgressWaitingForInput {
		t.Fatalf("lock result status = %q", lockResult.Status)
	}
	reaction := activeInteractionWindow(t, battle)
	if reaction.Purpose != state.InteractionPurposeReaction {
		t.Fatalf("active window = %#v, want reaction", reaction)
	}
	assertPlanningReveal(t, lockResult.Events, "player", "enemy")
	if len(beforeLock.Resolutions[beforeLock.ActiveResolutionID].Planning.Actors["player"].FinalDice) == 0 {
		t.Fatal("player dice were not retained before lock")
	}

	saveBattle(t, repo, battle, lockResult.Events)
	battle = loadBattle(t, repo, battle.ID)
	reactionPending := battle.Flow.PendingInput["player"]
	reacted, err := eng.ApplyBattleCommand(
		&battle,
		interactionCommitCommand(
			battle.ID,
			"player",
			reactionPending,
			command.InteractionCommitmentData{
				PlanningAdjustments: []command.PlanningAdjustment{
					{
						Type:    string(state.PlanningAdjustmentIncreaseMaxRolls),
						ActorID: "player",
						Amount:  1,
					},
				},
			},
		),
	)
	if err != nil {
		t.Fatalf("reaction commitment returned error: %v", err)
	}
	if activeInteractionWindow(t, battle).ReactionRound != 2 {
		t.Fatal("non-pass reaction did not open the next round")
	}
	saveBattle(t, repo, battle, reacted.Events)
	battle = loadBattle(t, repo, battle.ID)

	reactionPending = battle.Flow.PendingInput["player"]
	reentered, err := eng.ApplyBattleCommand(
		&battle,
		interactionPassCommand(battle.ID, "player", reactionPending),
	)
	if err != nil {
		t.Fatalf("reaction pass returned error: %v", err)
	}
	assertPlanningWindow(t, battle, segment.Offensive, 2)
	resolution := battle.Resolutions[battle.ActiveResolutionID]
	playerPlan := resolution.Planning.Actors["player"]
	enemyPlan := resolution.Planning.Actors["enemy"]
	if playerPlan.MaxRolls != 4 || playerPlan.RollsUsed != 2 ||
		!reflect.DeepEqual(playerPlan.KeptIndices, []int{0}) ||
		!reflect.DeepEqual(playerPlan.CommittedCards, []string{"Card A", "Card B"}) ||
		playerPlan.SelectedAbility != "Hero Strike" ||
		!reflect.DeepEqual(playerPlan.SelectedTargets, []string{"enemy"}) ||
		!reflect.DeepEqual(playerPlan.PaidCostIDs, []string{"cost-1"}) ||
		!reflect.DeepEqual(playerPlan.ResolvedCardIDs, []string{"Card A"}) {
		t.Fatalf("reopened player plan lost state: %#v", playerPlan)
	}
	if enemyPlan.LockedIn != true || battle.Flow.Actors["enemy"].Status != state.ActorLockedIn {
		t.Fatalf("unaffected enemy was reopened: plan %#v flow %#v", enemyPlan, battle.Flow.Actors["enemy"])
	}
	if !commandAllowedForTest(battle.Flow.PendingInput["player"], command.TypePlanningReroll) {
		t.Fatalf("newly granted roll is not allowed: %#v", battle.Flow.PendingInput["player"])
	}
	if containsRevealForActor(reentered.Events, "enemy") {
		t.Fatalf("re-entry unexpectedly revealed unchanged enemy commitment: %#v", reentered.Events)
	}

	saveBattle(t, repo, battle, reentered.Events)
	battle = loadBattle(t, repo, battle.ID)
	applyPlanning(t, eng, &battle, command.TypePlanningReroll, []int{1})
	pendingAtDefaultRollCount := battle.Flow.PendingInput["player"]
	if !commandAllowedForTest(pendingAtDefaultRollCount, command.TypePlanningReroll) ||
		!commandAllowedForTest(pendingAtDefaultRollCount, command.TypePlanningLockIn) ||
		!commandAllowedForTest(pendingAtDefaultRollCount, command.TypePlanningPass) {
		t.Fatalf("legal decisions at the original roll count = %#v", pendingAtDefaultRollCount)
	}
	applyPlanning(t, eng, &battle, command.TypePlanningReroll, []int{1})
	pendingAfterGrantedRoll := battle.Flow.PendingInput["player"]
	if commandAllowedForTest(pendingAfterGrantedRoll, command.TypePlanningReroll) ||
		!commandAllowedForTest(pendingAfterGrantedRoll, command.TypePlanningLockIn) {
		t.Fatalf("decisions after granted roll = %#v", pendingAfterGrantedRoll)
	}
	secondReveal := applyPlanning(t, eng, &battle, command.TypePlanningLockIn, nil)
	if !containsRevealForActor(secondReveal.Events, "player") ||
		containsRevealForActor(secondReveal.Events, "enemy") {
		t.Fatalf("replacement reveal was not delta-only: %#v", secondReveal.Events)
	}

	reactionPending = battle.Flow.PendingInput["player"]
	offenseFinalized, err := eng.ApplyBattleCommand(
		&battle,
		interactionPassCommand(battle.ID, "player", reactionPending),
	)
	if err != nil {
		t.Fatalf("final offensive reaction pass returned error: %v", err)
	}
	if offenseFinalized.Status != engine.ProgressWaitingForInput {
		t.Fatalf("offensive finalization status = %q", offenseFinalized.Status)
	}
	if len(battle.OffensiveProposals) != 3 {
		t.Fatalf("offensive proposals = %#v", battle.OffensiveProposals)
	}
	playerProposal := planningProposalForActor(t, battle.OffensiveProposals, "player")
	if playerProposal.Commitment.RollsUsed != 4 ||
		playerProposal.Commitment.MaxRolls != 4 ||
		!reflect.DeepEqual(playerProposal.Commitment.CommittedCards, []string{"Card A", "Card B"}) ||
		!reflect.DeepEqual(playerProposal.Commitment.SelectedTargets, []string{"enemy"}) {
		t.Fatalf("final offensive proposal = %#v", playerProposal)
	}
	if battle.Segment.Current != segment.Defensive {
		t.Fatalf("segment = %q, want defensive", battle.Segment.Current)
	}
	if battle.Flow.Actors["spectator"].Status != state.ActorNotParticipating {
		t.Fatalf("spectator defensive status = %#v", battle.Flow.Actors["spectator"])
	}
	assertPlanningWindow(t, battle, segment.Defensive, 1)

	defensiveResolution := battle.Resolutions[battle.ActiveResolutionID]
	defender := defensiveResolution.Planning.Actors["player"]
	if len(defender.EligibleTargetIDs) != 1 {
		t.Fatalf("defensive targets = %#v", defender.EligibleTargetIDs)
	}
	applyPlanning(t, eng, &battle, command.TypePlanningRoll, nil)
	applyPlanning(t, eng, &battle, command.TypePlanningCards, []string{"Card A"})
	applyPlanning(t, eng, &battle, command.TypePlanningAbility, "Hero Guard")
	applyPlanning(t, eng, &battle, command.TypePlanningTargets, defender.EligibleTargetIDs)
	applyPlanning(t, eng, &battle, command.TypePlanningLockIn, nil)

	reactionPending = battle.Flow.PendingInput["player"]
	defenseFinalized, err := eng.ApplyBattleCommand(
		&battle,
		interactionPassCommand(battle.ID, "player", reactionPending),
	)
	if err != nil {
		t.Fatalf("defensive reaction pass returned error: %v", err)
	}
	if len(battle.DefensiveProposals) != 2 {
		t.Fatalf("defensive proposals = %#v", battle.DefensiveProposals)
	}
	defensiveProposal := planningProposalForActor(t, battle.DefensiveProposals, "player")
	if defensiveProposal.Commitment.SelectedAbility != "Hero Guard" ||
		!reflect.DeepEqual(defensiveProposal.Commitment.CommittedCards, []string{"Card A"}) ||
		!reflect.DeepEqual(defensiveProposal.Commitment.SelectedTargets, defender.EligibleTargetIDs) {
		t.Fatalf("final defensive proposal = %#v", defensiveProposal)
	}
	if battle.Segment.Current != segment.DamageResolution ||
		defenseFinalized.Status != engine.ProgressWaitingForInput {
		t.Fatalf("defensive completion = %#v segment %q", defenseFinalized, battle.Segment.Current)
	}
	if len(battle.Actors["player"].Cards.Removed) != 0 ||
		len(battle.Actors["enemy"].Cards.Removed) != 0 {
		t.Fatal("planning permanently removed cards")
	}
}

func TestPlanningSupportsMultipleTargetsAndDefensiveSkipsWithoutIncomingProposals(t *testing.T) {
	battle := sharedPlanningBattle(t)
	offensive, err := engine.NewOffensiveFlow(&planningAIController{})
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	eng, err := engine.NewEngineWithFlows(offensive)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	applyPlanning(t, eng, &battle, command.TypePlanningAbility, "Hero Strike")
	applyPlanning(t, eng, &battle, command.TypePlanningTargets, []string{"enemy", "spectator"})
	plan := battle.Resolutions[battle.ActiveResolutionID].Planning.Actors["player"]
	if !reflect.DeepEqual(plan.SelectedTargets, []string{"enemy", "spectator"}) {
		t.Fatalf("selected targets = %#v", plan.SelectedTargets)
	}

	skipBattle := sharedPlanningBattle(t)
	skipBattle.Segment = segment.State{Current: segment.Defensive, Round: 1}
	skipBattle.Flow = state.NewSegmentFlowState(skipBattle.Segment)
	damageWait := &waitingFlow{id: segment.DamageResolution}
	skipEngine, err := engine.NewEngineWithFlows(engine.DefensiveFlow{}, damageWait)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	result, err := skipEngine.ProgressUntilInput(&skipBattle)
	if err != nil {
		t.Fatalf("defensive skip returned error: %v", err)
	}
	if result.Status != engine.ProgressWaitingForInput ||
		skipBattle.Segment.Current != segment.DamageResolution ||
		skipBattle.ActiveResolutionID != "" ||
		len(skipBattle.DefensiveProposals) != 0 {
		t.Fatalf("defensive skip state = result %#v battle %#v", result, skipBattle)
	}
}

func TestDefensiveFlowRunsActualPlanningRevealReactionAndFinalization(t *testing.T) {
	battle := sharedPlanningBattle(t)
	battle.Segment = segment.State{Current: segment.Defensive, Round: 1}
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	battle.OffensiveProposals = []state.PlanningProposal{
		{
			ID:         "incoming-enemy-attack",
			ActorID:    "enemy",
			Segment:    segment.Offensive,
			Defensible: true,
			Commitment: state.PlanningCommitmentData{
				Segment:         segment.Offensive,
				SelectedAbility: "Enemy Strike",
				SelectedTargets: []string{"player"},
				LockedIn:        true,
			},
		},
	}

	damageWait := &waitingFlow{id: segment.DamageResolution}
	eng, err := engine.NewEngineWithFlows(engine.DefensiveFlow{}, damageWait)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	started, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if started.Status != engine.ProgressWaitingForInput {
		t.Fatalf("status = %q, want waiting_for_input", started.Status)
	}
	assertPlanningWindow(t, battle, segment.Defensive, 1)
	if battle.Flow.Actors["player"].Status != state.ActorNeedsInput ||
		battle.Flow.Actors["enemy"].Status != state.ActorNotParticipating ||
		battle.Flow.Actors["spectator"].Status != state.ActorNotParticipating {
		t.Fatalf("defensive participation = %#v", battle.Flow.Actors)
	}
	pending := battle.Flow.PendingInput["player"]
	for _, commandType := range []command.Type{
		command.TypePlanningRoll,
		command.TypePlanningCards,
		command.TypePlanningAbility,
		command.TypePlanningTargets,
		command.TypePlanningPass,
	} {
		if !commandAllowedForTest(pending, commandType) {
			t.Fatalf("defensive pending input does not allow %q: %#v", commandType, pending)
		}
	}

	applyPlanning(t, eng, &battle, command.TypePlanningRoll, nil)
	applyPlanning(t, eng, &battle, command.TypePlanningCards, []string{"Card A", "Card B"})
	applyPlanning(t, eng, &battle, command.TypePlanningAbility, "Hero Guard")
	applyPlanning(t, eng, &battle, command.TypePlanningTargets, []string{"incoming-enemy-attack"})
	revealed := applyPlanning(t, eng, &battle, command.TypePlanningLockIn, nil)
	if !containsRevealForActor(revealed.Events, "player") ||
		activeInteractionWindow(t, battle).Purpose != state.InteractionPurposeReaction {
		t.Fatalf("defensive reveal/reaction state = events %#v window %#v", revealed.Events, activeInteractionWindow(t, battle))
	}

	reactionPending := battle.Flow.PendingInput["player"]
	finalized, err := eng.ApplyBattleCommand(
		&battle,
		interactionPassCommand(battle.ID, "player", reactionPending),
	)
	if err != nil {
		t.Fatalf("defensive reaction pass returned error: %v", err)
	}
	if finalized.Status != engine.ProgressWaitingForInput ||
		battle.Segment.Current != segment.DamageResolution {
		t.Fatalf("defensive completion = %#v segment %q", finalized, battle.Segment.Current)
	}
	if len(battle.DefensiveProposals) != 1 {
		t.Fatalf("defensive proposals = %#v", battle.DefensiveProposals)
	}
	proposal := battle.DefensiveProposals[0]
	if proposal.Segment != segment.Defensive ||
		proposal.Commitment.SelectedAbility != "Hero Guard" ||
		!reflect.DeepEqual(proposal.Commitment.CommittedCards, []string{"Card A", "Card B"}) ||
		!reflect.DeepEqual(proposal.Commitment.SelectedTargets, []string{"incoming-enemy-attack"}) ||
		proposal.Commitment.RollsUsed != 1 {
		t.Fatalf("final defensive proposal = %#v", proposal)
	}
}

func TestPlanningSupportsMultipleHumanAndAIActors(t *testing.T) {
	battle := sharedPlanningBattle(t)
	spectator := battle.Actors["spectator"]
	spectator.Controller = state.ControllerHuman
	spectator.AbilityIDs = []string{"Spectator Strike"}
	battle.Actors["spectator"] = spectator
	offensive, err := engine.NewOffensiveFlow(&planningAIController{})
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	eng, err := engine.NewEngineWithFlows(offensive)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if len(battle.Flow.PendingInput) != 2 ||
		battle.Flow.Actors["enemy"].Status != state.ActorLockedIn {
		t.Fatalf("multi-actor planning state = %#v pending %#v", battle.Flow.Actors, battle.Flow.PendingInput)
	}

	playerPending := battle.Flow.PendingInput["player"]
	playerPass := planningCommand(t, battle.ID, "player", playerPending, command.TypePlanningPass, nil)
	if _, err := eng.ApplyBattleCommand(&battle, playerPass); err != nil {
		t.Fatalf("player pass returned error: %v", err)
	}
	playerPending = battle.Flow.PendingInput["player"]
	playerLock := planningCommand(t, battle.ID, "player", playerPending, command.TypePlanningLockIn, nil)
	if _, err := eng.ApplyBattleCommand(&battle, playerLock); err != nil {
		t.Fatalf("player lock returned error: %v", err)
	}
	resolution := battle.Resolutions[battle.ActiveResolutionID]
	if resolution.Windows[resolution.ActiveWindowID].RevealStatus != state.RevealStatusCollecting {
		t.Fatal("planning revealed before every required human locked")
	}

	spectatorPending := battle.Flow.PendingInput["spectator"]
	spectatorPass := planningCommand(t, battle.ID, "spectator", spectatorPending, command.TypePlanningPass, nil)
	if _, err := eng.ApplyBattleCommand(&battle, spectatorPass); err != nil {
		t.Fatalf("spectator pass returned error: %v", err)
	}
	spectatorPending = battle.Flow.PendingInput["spectator"]
	spectatorLock := planningCommand(t, battle.ID, "spectator", spectatorPending, command.TypePlanningLockIn, nil)
	result, err := eng.ApplyBattleCommand(&battle, spectatorLock)
	if err != nil {
		t.Fatalf("spectator lock returned error: %v", err)
	}
	if !containsRevealForActor(result.Events, "player") ||
		!containsRevealForActor(result.Events, "spectator") ||
		!containsRevealForActor(result.Events, "enemy") {
		t.Fatalf("simultaneous reveal = %#v", result.Events)
	}
}

func TestPlanningCommandsRejectStaleDuplicateAndLockedActors(t *testing.T) {
	battle := sharedPlanningBattle(t)
	offensive, err := engine.NewOffensiveFlow(&planningAIController{})
	if err != nil {
		t.Fatalf("NewOffensiveFlow() returned error: %v", err)
	}
	eng, err := engine.NewEngineWithFlows(offensive)
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}

	pending := battle.Flow.PendingInput["player"]
	cmd := planningCommand(t, battle.ID, "player", pending, command.TypePlanningPass, nil)
	if _, err := eng.ApplyBattleCommand(&battle, cmd); err != nil {
		t.Fatalf("planning pass returned error: %v", err)
	}
	before := battle.Clone()
	if _, err := eng.ApplyBattleCommand(&battle, cmd); err == nil ||
		(!strings.Contains(err.Error(), "stale") && !strings.Contains(err.Error(), "pending")) {
		t.Fatalf("duplicate command error = %v", err)
	}
	if !reflect.DeepEqual(battle, before) {
		t.Fatal("duplicate command mutated battle")
	}

	applyPlanning(t, eng, &battle, command.TypePlanningLockIn, nil)
	before = battle.Clone()
	if _, err := eng.ApplyBattleCommand(
		&battle,
		planningCommand(t, battle.ID, "player", pending, command.TypePlanningRoll, nil),
	); err == nil {
		t.Fatal("locked actor planning command was accepted")
	}
	if !reflect.DeepEqual(battle, before) {
		t.Fatal("locked actor rejection mutated battle")
	}
}

func sharedPlanningBattle(t *testing.T) state.Battle {
	t.Helper()
	battle, err := state.NewBattleFromSetup("planning-battle", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "player",
				ControllerType: state.ControllerHuman,
				Hand:           []string{"Card A", "Card B", "Card A"},
				DiceLoadout:    []state.DiceLoadoutEntry{{DiceID: "Symbol D6", Count: 2}},
				AbilityIDs:     []string{"Hero Strike", "Hero Guard"},
			},
			{
				ID:             "enemy",
				ControllerType: state.ControllerAI,
				Hand:           []string{"Enemy Card"},
				DiceLoadout:    []state.DiceLoadoutEntry{{DiceID: "Symbol D6", Count: 2}},
				AbilityIDs:     []string{"Enemy Strike", "Enemy Guard"},
			},
			{
				ID:             "spectator",
				ControllerType: state.ControllerAI,
			},
		},
		DiceDefinitions: []state.DiceDefinition{symbolD6()},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	battle.Segment = segment.State{Current: segment.Offensive, Round: 1}
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	return battle
}

func symbolD6() state.DiceDefinition {
	faces := make([]state.DiceFace, 6)
	for i := range faces {
		faces[i] = state.DiceFace{
			Face:    i + 1,
			Value:   i + 1,
			Symbols: []string{[]string{"Blank", "Shield", "Bow", "Sword", "Star", "Crown"}[i]},
		}
	}
	return state.DiceDefinition{ID: "Symbol D6", Name: "Symbol D6", DieType: "d6", SideCount: 6, Faces: faces}
}

type planningAIController struct {
	calls int
}

func (controller *planningAIController) Plan(
	ctx *engine.Context,
	actorID string,
) (state.OffensiveCommitment, error) {
	controller.calls++
	if actorID == "spectator" {
		return state.OffensiveCommitment{ID: "spectator-pass", ActorID: actorID}, nil
	}
	actor := ctx.Battle.Actors[actorID]
	actor.Dice.CurrentRoll = &state.RollState{
		RequestID:   "enemy-private-roll",
		ActorID:     actorID,
		Segment:     segment.Offensive,
		Pool:        state.RollPoolOffensive,
		SourceType:  state.RollSourceSegment,
		SourceID:    string(segment.Offensive),
		Dice:        []state.RolledDie{{Index: 0, DieID: "Symbol D6", Face: 6, Value: 6, Symbols: []string{"Crown"}}},
		KeptIndices: []int{0},
		RollsUsed:   2,
		MaxRolls:    3,
	}
	ctx.Battle.Actors[actorID] = actor
	return state.OffensiveCommitment{
		ID:              "enemy-private-plan",
		ActorID:         actorID,
		FinalDice:       actor.Dice.CurrentRoll.Dice,
		RollsUsed:       2,
		SelectedAbility: "Enemy Strike",
		SelectedCards:   []string{"Enemy Card"},
		SelectedTargets: []string{"player"},
		RollHistory: [][]state.RolledDie{
			{{Index: 0, DieID: "Symbol D6", Face: 1, Symbols: []string{"Blank"}}},
			{{Index: 0, DieID: "Symbol D6", Face: 6, Symbols: []string{"Crown"}}},
		},
	}, nil
}

func applyPlanning(
	t *testing.T,
	eng engine.Engine,
	battle *state.Battle,
	commandType command.Type,
	value any,
) engine.ProgressionResult {
	t.Helper()
	pending := battle.Flow.PendingInput["player"]
	cmd := planningCommand(t, battle.ID, "player", pending, commandType, value)
	result, err := eng.ApplyBattleCommand(battle, cmd)
	if err != nil {
		t.Fatalf("%s returned error: %v", commandType, err)
	}
	return result
}

func planningCommand(
	t *testing.T,
	battleID string,
	actorID string,
	pending state.PendingInput,
	commandType command.Type,
	value any,
) command.Command {
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
	case command.TypePlanningRoll:
		payload = command.PlanningRollPayload{PendingInputID: pending.ID, Checkpoint: checkpoint}
	case command.TypePlanningKeep:
		payload = command.PlanningKeepPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
			KeptIndices:    value.([]int),
		}
	case command.TypePlanningReroll:
		payload = command.PlanningRerollPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
			RerollIndices:  value.([]int),
		}
	case command.TypePlanningCards:
		payload = command.PlanningCardsPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
			CardIDs:        value.([]string),
		}
	case command.TypePlanningAbility:
		payload = command.PlanningAbilityPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
			AbilityID:      value.(string),
		}
	case command.TypePlanningTargets:
		payload = command.PlanningTargetsPayload{
			PendingInputID: pending.ID,
			Checkpoint:     checkpoint,
			TargetIDs:      value.([]string),
		}
	case command.TypePlanningPass:
		payload = command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: checkpoint}
	case command.TypePlanningLockIn:
		payload = command.PlanningLockInPayload{PendingInputID: pending.ID, Checkpoint: checkpoint}
	default:
		t.Fatalf("unsupported planning command %q", commandType)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	return command.Command{BattleID: battleID, ActorID: actorID, Type: commandType, Payload: encoded}
}

func assertPlanningWindow(
	t *testing.T,
	battle state.Battle,
	planningSegment segment.Segment,
	cycle int,
) {
	t.Helper()
	resolution := battle.Resolutions[battle.ActiveResolutionID]
	window := resolution.Windows[resolution.ActiveWindowID]
	if resolution.Planning == nil ||
		resolution.Planning.Segment != planningSegment ||
		resolution.Planning.Cycle != cycle ||
		window.Purpose != state.InteractionPurposePlanning {
		t.Fatalf("planning state = resolution %#v window %#v", resolution, window)
	}
}

func assertPlanningReveal(t *testing.T, events []event.Event, actorIDs ...string) {
	t.Helper()
	for _, actorID := range actorIDs {
		if !containsRevealForActor(events, actorID) {
			t.Fatalf("reveal did not include actor %q: %#v", actorID, events)
		}
	}
	filtered := event.ForViewer(events, "player")
	for _, battleEvent := range filtered {
		for _, commitment := range battleEvent.Commitments {
			if commitment.Data.Planning == nil {
				continue
			}
			if len(commitment.Data.Planning.FinalDice) == 0 && !commitment.Data.Planning.Passed {
				t.Fatalf("reveal omitted final dice: %#v", commitment)
			}
			for _, die := range commitment.Data.Planning.FinalDice {
				if die.Face < 1 || len(die.Symbols) == 0 {
					t.Fatalf("revealed die lacks face or symbols: %#v", die)
				}
			}
			if commitment.ActorID != "player" && commitment.Data.Planning.KeptIndices != nil {
				t.Fatalf("opponent keep history leaked: %#v", commitment)
			}
		}
	}
	payload, err := json.Marshal(filtered)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	if strings.Contains(string(payload), "RollHistory") {
		t.Fatalf("intermediate roll history leaked: %s", payload)
	}
}

func assertNoPlanningSecret(
	t *testing.T,
	battle state.Battle,
	events []event.Event,
	viewerActorID string,
	secret string,
) {
	t.Helper()
	viewPayload, err := json.Marshal(snapshot.FromBattleForViewer(battle, viewerActorID))
	if err != nil {
		t.Fatalf("Marshal(snapshot) returned error: %v", err)
	}
	eventPayload, err := json.Marshal(event.ForViewer(events, viewerActorID))
	if err != nil {
		t.Fatalf("Marshal(events) returned error: %v", err)
	}
	if strings.Contains(string(viewPayload), secret) || strings.Contains(string(eventPayload), secret) {
		t.Fatalf("hidden planning secret leaked: snapshot=%s events=%s", viewPayload, eventPayload)
	}
}

func containsRevealForActor(events []event.Event, actorID string) bool {
	for _, battleEvent := range events {
		if battleEvent.Type != event.TypeInteractionRevealed ||
			battleEvent.Purpose != state.InteractionPurposePlanning {
			continue
		}
		for _, commitment := range battleEvent.Commitments {
			if commitment.ActorID == actorID {
				return true
			}
		}
	}
	return false
}

func planningProposalForActor(
	t *testing.T,
	proposals []state.PlanningProposal,
	actorID string,
) state.PlanningProposal {
	t.Helper()
	for _, proposal := range proposals {
		if proposal.ActorID == actorID {
			return proposal
		}
	}
	t.Fatalf("planning proposal for actor %q was not found", actorID)
	return state.PlanningProposal{}
}

func commandAllowedForTest(pending state.PendingInput, target command.Type) bool {
	for _, allowed := range pending.AllowedCommands {
		if allowed == target {
			return true
		}
	}
	return false
}

func saveBattle(
	t *testing.T,
	repo repository.Repository,
	battle state.Battle,
	events []event.Event,
) {
	t.Helper()
	checkpoint, err := repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	checkpoint.Battle = battle
	checkpoint.Events = append(checkpoint.Events, events...)
	if err := repo.Save(checkpoint); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
}

func loadBattle(t *testing.T, repo repository.Repository, battleID string) state.Battle {
	t.Helper()
	checkpoint, err := repo.Load(battleID)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	return checkpoint.Battle
}
