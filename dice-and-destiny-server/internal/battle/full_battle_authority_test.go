package battle

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestAuthorityRunsBladeWardenVsVenomGoblinFullBattle(t *testing.T) {
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
		// Opening hands: Blade Warden then Venom Goblin.
		{Stream: "card_draw", Bound: 20, Value: 8}, {Stream: "card_draw", Bound: 19, Value: 0}, {Stream: "card_draw", Bound: 18, Value: 8}, {Stream: "card_draw", Bound: 17, Value: 5},
		{Stream: "card_draw", Bound: 12, Value: 4}, {Stream: "card_draw", Bound: 11, Value: 6},
		// Round 1 Income and enemy plan.
		{Stream: "card_draw", Bound: 16, Value: 10}, {Stream: "card_draw", Bound: 10, Value: 2}, {Stream: "ai_d100", Bound: 100, Value: 18},
		// Round 1 player combat dice.
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 3}, {Stream: "combat_dice", Bound: 6, Value: 5}, {Stream: "combat_dice", Bound: 6, Value: 5}, {Stream: "combat_dice", Bound: 6, Value: 4},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 1}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 2},
		// Both Basic Defenses.
		{Stream: "ai_defense", Bound: 2, Value: 0}, {Stream: "defense_dice", Bound: 6, Value: 1}, {Stream: "defense_dice", Bound: 6, Value: 2},
		// Round 1 exact damage-card reveals and enemy pass.
		{Stream: "damage_selection", Bound: 15, Value: 2}, {Stream: "damage_selection", Bound: 9, Value: 0}, {Stream: "damage_selection", Bound: 8, Value: 2}, {Stream: "damage_selection", Bound: 7, Value: 3}, {Stream: "ai_damage_response", Bound: 2, Value: 0},
		// Automatic Round 2 Poison dice after Round 1 damage acknowledgment.
		{Stream: "effect_dice", Bound: 6, Value: 1}, {Stream: "effect_dice", Bound: 6, Value: 5},
		// Round 2 Ongoing child damage, Income, AI plan, and player dice.
		{Stream: "damage_selection", Bound: 14, Value: 9}, {Stream: "damage_selection", Bound: 6, Value: 1},
		{Stream: "card_draw", Bound: 13, Value: 2}, {Stream: "card_draw", Bound: 5, Value: 2}, {Stream: "ai_d100", Bound: 100, Value: 51},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 3}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 5}, {Stream: "combat_dice", Bound: 6, Value: 1},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 2}, {Stream: "combat_dice", Bound: 6, Value: 0},
		{Stream: "ai_defense", Bound: 2, Value: 1}, {Stream: "damage_selection", Bound: 4, Value: 0}, {Stream: "damage_selection", Bound: 3, Value: 1}, {Stream: "ai_damage_response", Bound: 2, Value: 1},
		// Round 3 Bleed, Income, AI plan, combat and defenses.
		{Stream: "damage_selection", Bound: 4, Value: 1}, {Stream: "damage_selection", Bound: 3, Value: 0}, {Stream: "card_draw", Bound: 12, Value: 5}, {Stream: "card_draw", Bound: 2, Value: 0}, {Stream: "ai_d100", Bound: 100, Value: 32},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 2}, {Stream: "combat_dice", Bound: 6, Value: 3}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 5},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 1}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 2},
		{Stream: "ai_defense", Bound: 2, Value: 0}, {Stream: "defense_dice", Bound: 6, Value: 2}, {Stream: "defense_dice", Bound: 6, Value: 3},
		{Stream: "damage_selection", Bound: 11, Value: 0}, {Stream: "damage_selection", Bound: 10, Value: 6}, {Stream: "damage_selection", Bound: 1, Value: 0}, {Stream: "damage_selection", Bound: 1, Value: 0}, {Stream: "damage_selection", Bound: 4, Value: 1}, {Stream: "ai_damage_response", Bound: 2, Value: 0},
		// Round 4 Bleed, Income, AI miss, player dice, enemy defense, final overage.
		{Stream: "damage_selection", Bound: 3, Value: 1}, {Stream: "damage_selection", Bound: 2, Value: 1}, {Stream: "card_draw", Bound: 9, Value: 6}, {Stream: "ai_d100", Bound: 100, Value: 89},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 3}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 5}, {Stream: "combat_dice", Bound: 6, Value: 5},
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 1}, {Stream: "combat_dice", Bound: 6, Value: 4}, {Stream: "combat_dice", Bound: 6, Value: 2},
		{Stream: "ai_defense", Bound: 2, Value: 0}, {Stream: "defense_dice", Bound: 6, Value: 0}, {Stream: "damage_selection", Bound: 1, Value: 0}, {Stream: "ai_damage_response", Bound: 2, Value: 0},
	}}
	root := participantTestServerRoot(t)
	assembler := NewFileParticipantAssembler(filepath.Join(root, "content"), filepath.Join(root, "save", "run_players"))
	repo := repository.NewInMemory()
	battleEngine, err := engine.NewEngineWithConfig(engine.Config{NamedRandom: script}, engine.DefaultFlows()...)
	if err != nil {
		t.Fatal(err)
	}
	authority := NewAuthority(battleEngine, repo, assembler)

	result := fullBattleSend(t, authority, map[string]any{"battle_id": "blade-v-goblin", "actor_id": "blade", "type": command.TypeStartBattle, "payload": map[string]any{"player": map[string]string{"instance_id": "blade", "definition_id": "blade_warden"}, "enemies": []map[string]string{{"instance_id": "goblin", "definition_id": "venom_goblin"}}}})
	assertFullBattleWait(t, result, "offensive", 1, "planning")
	assertZones(t, result, "blade", 15, 5, 0, 0, 3)
	assertZones(t, result, "goblin", 9, 3, 0, 0, 2)
	sharpen := cardInHand(t, result.Snapshot.Actors["blade"], "sharpen_blade")
	result = sendPlanningCardsFull(t, authority, result, sharpen, "sword_cut")
	assertZones(t, result, "blade", 15, 4, 1, 0, 2)
	result = sendPlanningRollFull(t, authority, result)
	assertFaces(t, result.Snapshot.Actors["blade"].RollHistory[0].Dice, []int{1, 4, 6, 6, 5})
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1})
	result = sendPlanningRerollFull(t, authority, result, []int{2, 3, 4})
	assertFaces(t, result.Snapshot.Actors["blade"].RollHistory[1].Dice, []int{1, 4, 1, 2, 5})
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1, 2, 3})
	result = sendPlanningRerollFull(t, authority, result, []int{4})
	assertFaces(t, result.Snapshot.Actors["blade"].RollHistory[2].Dice, []int{1, 4, 1, 2, 3})
	result = sendAbilityFull(t, authority, result, "sword_cut", []string{"goblin"})
	assertFullBattleWait(t, result, "offensive", 1, "offensive_reaction")
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 1, "defense_selection")
	sourceID := result.Snapshot.SettledSources[0].ID
	for _, source := range result.Snapshot.SettledSources {
		if source.TargetActorID == "blade" {
			sourceID = source.ID
		}
	}
	result = sendAbilityFull(t, authority, result, "basic_defense", []string{sourceID})
	assertFullBattleWait(t, result, "defensive", 1, "defense_roll")
	result = sendRollDiceFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 1, "defense_reaction")
	assertDefenseRollEvent(t, result, "blade", 2)
	if len(result.Snapshot.SettledDefenses) != 2 || result.Snapshot.SettledDefenses["blade"].AbilityID != "basic_defense" || result.Snapshot.SettledDefenses["goblin"].AbilityID != "basic_defense" {
		t.Fatalf("defense reveal snapshot=%#v, want both actor defenses", result.Snapshot.SettledDefenses)
	}
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "damage_resolution", 1, "damage_reaction")
	assertDamageDefinitions(t, result, []string{"loaded_die", "tip_it", "emergency_ward", "battle_focus"})
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 2, "status_roll_reaction")
	assertZones(t, result, "blade", 14, 4, 1, 1, 2)
	assertZones(t, result, "goblin", 6, 3, 0, 3, 2)

	// Round 2: Antidote responds after both Poison rolls were collected.
	antidote := cardInHand(t, result.Snapshot.Actors["blade"], "antidote")
	result = sendCommitFull(t, authority, result, []string{antidote}, nil, nil, "poison")
	assertFullBattleWait(t, result, "ongoing_effects", 2, "status_roll_reaction")
	assertZones(t, result, "blade", 14, 3, 2, 1, 1)
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 2, "status_damage_reaction")
	assertDamageDefinitions(t, result, []string{"battle_focus", "loaded_die"})
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "offensive", 2, "planning")
	assertZones(t, result, "blade", 12, 4, 2, 2, 2)
	assertZones(t, result, "goblin", 4, 4, 0, 4, 3)
	result = sendPlanningRollFull(t, authority, result)
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1, 2})
	result = sendPlanningRerollFull(t, authority, result, []int{3, 4})
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1, 2, 3})
	result = sendPlanningRerollFull(t, authority, result, []int{4})
	result = sendAbilityFull(t, authority, result, "sword_cut", []string{"goblin"})
	tip := cardInHand(t, result.Snapshot.Actors["blade"], "tip_it")
	result = sendCommitFull(t, authority, result, []string{tip}, nil, []command.PlanningAdjustment{{Type: string(state.PlanningAdjustmentSetDieFace), ActorID: "goblin", DieIndex: 3, Face: 5}}, "")
	assertFullBattleWait(t, result, "offensive", 2, "offensive_reaction")
	if result.Snapshot.Actors["goblin"].SelectedAbility != "" {
		t.Fatalf("enemy fallback = %q, want no ability", result.Snapshot.Actors["goblin"].SelectedAbility)
	}
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 2, "defense_reaction")
	if defense := result.Snapshot.SettledDefenses["goblin"]; defense.AbilityID != "protect" || defense.SourceID == "" {
		t.Fatalf("round 2 enemy defense reveal=%#v, want Protect against the player attack", defense)
	}
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "damage_resolution", 2, "damage_reaction")
	// Emergency Ward has already paid and moved to discard before this human wait.
	assertZones(t, result, "goblin", 4, 3, 1, 4, 1)
	for _, removal := range result.Snapshot.SettledDamage.Removals {
		if removal.Accepted && !removal.Released {
			t.Fatalf("prevention did not release %#v", removal)
		}
	}
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 3, "status_damage_reaction")
	// Both stacks dealt damage on Ongoing Effects entry, then Bleed's normal
	// checkpoint decay removed one stack before opening this reaction window.
	if statuses := result.Snapshot.Actors["goblin"].Statuses; len(statuses) != 1 || statuses[0].DefinitionID != "bleed" || statuses[0].Stacks != 1 {
		t.Fatalf("post-trigger Bleed=%#v, want one stack after two-stack damage and one-stack decay", statuses)
	}
	assertDamageDefinitions(t, result, []string{"emergency_ward", "tip_it"})

	// Round 3: five-Sword Sword Cut, paired bonus, two Basic Defenses, and
	// deck/discard/hand damage selection in one revealed batch.
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "offensive", 3, "planning")
	result = sendPlanningRollFull(t, authority, result)
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1})
	result = sendPlanningRerollFull(t, authority, result, []int{2, 3, 4})
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1, 2, 3})
	result = sendPlanningRerollFull(t, authority, result, []int{4})
	result = sendAbilityFull(t, authority, result, "sword_cut", []string{"goblin"})
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 3, "defense_selection")
	sourceID = incomingSourceID(t, result, "blade")
	result = sendAbilityFull(t, authority, result, "basic_defense", []string{sourceID})
	result = sendRollDiceFull(t, authority, result)
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "damage_resolution", 3, "damage_reaction")
	assertDamageDefinitions(t, result, []string{"tip_it", "battle_focus", "battle_focus", "emergency_ward", "loaded_die"})
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 4, "status_damage_reaction")
	assertZones(t, result, "blade", 9, 4, 3, 4, 2)
	assertZones(t, result, "goblin", 0, 3, 0, 9, 2)
	assertDamageDefinitions(t, result, []string{"battle_focus", "battle_focus"})

	// Round 4: the final card remains in hand through reveal, overage is four,
	// and victory is declared only after Damage Resolution exits.
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "offensive", 4, "planning")
	assertZones(t, result, "blade", 8, 5, 3, 4, 3)
	assertZones(t, result, "goblin", 0, 1, 0, 11, 3)
	result = sendPlanningRollFull(t, authority, result)
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1})
	result = sendPlanningRerollFull(t, authority, result, []int{2, 3, 4})
	result = sendPlanningKeepFull(t, authority, result, []int{0, 1, 2, 3})
	result = sendPlanningRerollFull(t, authority, result, []int{4})
	result = sendAbilityFull(t, authority, result, "sword_cut", []string{"goblin"})
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 4, "defense_reaction")
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "damage_resolution", 4, "damage_reaction")
	assertDamageDefinitions(t, result, []string{"battle_focus"})
	if got := result.Snapshot.SettledDamage.Overage["goblin"]; got != 4 {
		t.Fatalf("final overage=%d want 4", got)
	}
	if result.Snapshot.Actors["goblin"].HandCount != 1 || result.Snapshot.Actors["goblin"].RemovedCount != 11 {
		t.Fatal("final card moved before acknowledgment")
	}
	result = sendPassFull(t, authority, result)
	if result.Status != engine.ProgressBattleComplete || result.BattleResult != state.BattleVictory {
		t.Fatalf("final result=%#v", result)
	}
	assertZones(t, result, "blade", 8, 5, 3, 4, 3)
	assertZones(t, result, "goblin", 0, 0, 0, 12, 3)
	if result.Snapshot.CompletedRounds != 4 {
		t.Fatalf("completed rounds=%d", result.Snapshot.CompletedRounds)
	}
	if len(result.Snapshot.Actors["blade"].AbilityModifiers) != 0 {
		t.Fatal("battle-duration modifier was not cleared")
	}
	if len(result.Snapshot.Actors["goblin"].Statuses) != 1 || result.Snapshot.Actors["goblin"].Statuses[0].DefinitionID != "bleed" {
		t.Fatalf("final scheduled Bleed state=%#v", result.Snapshot.Actors["goblin"].Statuses)
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func fullBattleSend(t *testing.T, authority *Authority, value any) engine.Result {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result engine.Result
	if err := json.Unmarshal([]byte(authority.HandleCommandJSON(string(data))), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Accepted {
		t.Fatalf("command rejected: %s\n%s", result.Error, data)
	}
	checkpoint, err := authority.repo.Load("blade-v-goblin")
	if err != nil {
		t.Fatalf("persisted checkpoint missing after command: %v", err)
	}
	if result.Snapshot == nil || checkpoint.Battle.Segment.Current != result.Snapshot.Segment || checkpoint.Battle.Segment.Round != result.Snapshot.Round || checkpoint.Battle.Status != result.Snapshot.Status {
		t.Fatalf("persisted battle disagrees with response: stored=%#v response=%#v", checkpoint.Battle.Segment, result.Snapshot)
	}
	for i, battleEvent := range checkpoint.Events {
		want := uint64(i + 1)
		if battleEvent.Sequence != want {
			t.Fatalf("event sequence %d = %d", i, battleEvent.Sequence)
		}
	}
	return result
}
func pendingFull(t *testing.T, result engine.Result) snapshot.PendingInput {
	t.Helper()
	pending, ok := result.PendingInput["blade"]
	if !ok {
		t.Fatalf("missing blade pending input: %#v", result.PendingInput)
	}
	return pending
}
func planningCheckpointFull(p snapshot.PendingInput) command.PlanningCheckpoint {
	return command.PlanningCheckpoint{WindowID: p.WindowID, Segment: string(p.Segment), Stage: p.Stage, Iteration: p.Iteration, PlanningCycle: p.PlanningCycle}
}
func interactionCheckpointFull(p snapshot.PendingInput) command.InteractionCheckpoint {
	return command.InteractionCheckpoint{WindowID: p.WindowID, Stage: p.Stage, Iteration: p.Iteration, ReactionRound: p.ReactionRound, PlanningCycle: p.PlanningCycle}
}
func envelopeFull(result engine.Result, kind command.Type, payload any) map[string]any {
	return map[string]any{"battle_id": "blade-v-goblin", "actor_id": "blade", "type": kind, "payload": payload}
}
func sendPlanningCardsFull(t *testing.T, a *Authority, r engine.Result, cardID, abilityID string) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: p.ID, Checkpoint: planningCheckpointFull(p), CardIDs: []string{cardID}, AbilityID: abilityID}))
}
func sendPlanningRollFull(t *testing.T, a *Authority, r engine.Result) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePlanningRoll, command.PlanningRollPayload{PendingInputID: p.ID, Checkpoint: planningCheckpointFull(p)}))
}
func sendPlanningKeepFull(t *testing.T, a *Authority, r engine.Result, indices []int) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePlanningKeep, command.PlanningKeepPayload{PendingInputID: p.ID, Checkpoint: planningCheckpointFull(p), KeptIndices: indices}))
}
func sendPlanningRerollFull(t *testing.T, a *Authority, r engine.Result, indices []int) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePlanningReroll, command.PlanningRerollPayload{PendingInputID: p.ID, Checkpoint: planningCheckpointFull(p), RerollIndices: indices}))
}
func sendAbilityFull(t *testing.T, a *Authority, r engine.Result, ability string, targets []string) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: p.ID, Checkpoint: planningCheckpointFull(p), AbilityID: ability, TargetIDs: targets}))
}
func sendRollDiceFull(t *testing.T, a *Authority, r engine.Result) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypeRollDice, command.RollDicePayload{PendingInputID: p.ID, RequestID: p.SourceID}))
}
func sendPassFull(t *testing.T, a *Authority, r engine.Result) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypePass, command.PassPayload{PendingInputID: p.ID, Checkpoint: interactionCheckpointFull(p)}))
}
func sendCommitFull(t *testing.T, a *Authority, r engine.Result, cards, proposals []string, adjustments []command.PlanningAdjustment, choice string) engine.Result {
	p := pendingFull(t, r)
	return fullBattleSend(t, a, envelopeFull(r, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: p.ID, Checkpoint: interactionCheckpointFull(p), Commitment: command.InteractionCommitmentData{CardIDs: cards, ProposalIDs: proposals, ChoiceID: choice, PlanningAdjustments: adjustments}}))
}
func incomingSourceID(t *testing.T, r engine.Result, target string) string {
	t.Helper()
	for _, source := range r.Snapshot.SettledSources {
		if source.TargetActorID == target {
			return source.ID
		}
	}
	t.Fatalf("no incoming source for %s", target)
	return ""
}
func cardInHand(t *testing.T, actor snapshot.Actor, definition string) string {
	t.Helper()
	for _, id := range actor.Hand {
		if actor.CardInstances[id].DefinitionID == definition {
			return id
		}
	}
	t.Fatalf("%s is not in hand: %#v", definition, actor.Hand)
	return ""
}
func assertFullBattleWait(t *testing.T, r engine.Result, seg string, round int, stage string) {
	t.Helper()
	if r.Snapshot == nil || string(r.Snapshot.Segment) != seg || r.Snapshot.Round != round || r.Snapshot.Stage != stage {
		t.Fatalf("checkpoint = %#v, want %s r%d %s", r.Snapshot, seg, round, stage)
	}
	p := pendingFull(t, r)
	if p.Stage != stage || len(p.AllowedCommands) == 0 {
		t.Fatalf("pending = %#v", p)
	}
}
func assertZones(t *testing.T, r engine.Result, id string, deck, hand, discard, removed, energy int) {
	t.Helper()
	a := r.Snapshot.Actors[id]
	if a.DeckCount != deck || a.HandCount != hand || a.DiscardCount != discard || a.RemovedCount != removed || a.EnergyPoints != energy {
		t.Fatalf("%s zones/energy = d%d h%d x%d r%d e%d, want d%d h%d x%d r%d e%d", id, a.DeckCount, a.HandCount, a.DiscardCount, a.RemovedCount, a.EnergyPoints, deck, hand, discard, removed, energy)
	}
}
func assertFaces(t *testing.T, dice []state.RolledDie, want []int) {
	t.Helper()
	if len(dice) != len(want) {
		t.Fatalf("dice=%#v", dice)
	}
	for i, face := range want {
		if dice[i].Face != face {
			t.Fatalf("dice faces=%#v want=%v", dice, want)
		}
	}
}
func assertDefenseRollEvent(t *testing.T, result engine.Result, actorID string, face int) {
	t.Helper()
	for _, battleEvent := range result.Events {
		if battleEvent.Type == event.TypeDiceRolled && battleEvent.ActorID == actorID && battleEvent.Pool == state.RollPoolDefensive {
			if len(battleEvent.Dice) != 1 || battleEvent.Dice[0].Face != face {
				t.Fatalf("%s defensive roll event = %#v, want face %d", actorID, battleEvent, face)
			}
			return
		}
	}
	t.Fatalf("missing %s defensive roll event in %#v", actorID, result.Events)
}
func assertDamageDefinitions(t *testing.T, r engine.Result, want []string) {
	t.Helper()
	got := make([]string, len(r.Snapshot.SettledDamage.Removals))
	for i, card := range r.Snapshot.SettledDamage.Removals {
		got[i] = card.CardDefinitionID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("damage definitions=%v want=%v", got, want)
	}
}
