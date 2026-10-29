package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// expeditionStatusFixture is the adventurer fixture with the general_v1
// statuses (Ambush, Fortified, Conqueror, Lieutenant's Favor) and the minion
// pack pinned into the catalog, a Brine Mask enemy with Salt Veil, and the
// given statuses on each side.
func expeditionStatusFixture(t *testing.T, player, enemy []state.StatusState) (state.Battle, content.BattleLibrary) {
	t.Helper()
	lib := settledTestLibrary(t)
	var err error
	for _, pack := range []string{"general_v1", "adventurer_v1", "minions_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", pack)); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	b, err := state.NewBattleFromSetup("expedition-status-test", state.BattleSetup{Actors: []state.ActorSetup{
		{ID: "player", DefinitionID: "adventurer", DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "standard_d6", Count: 5}}, ControllerType: state.ControllerHuman, Statuses: player, Resources: state.ResourceState{EnergyPoints: 10, MaxEnergyPoints: 75}},
		{ID: "enemy", DefinitionID: "drowned_oracle_brine_mask", DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "brine_d6", Count: 5}}, ControllerType: state.ControllerAI, Statuses: enemy, Resources: state.ResourceState{MaxEnergyPoints: 75}},
	}, SettledCatalog: catalog, SettledActors: map[string]state.SettledActorRuntime{"player": {MaxRolls: 3}, "enemy": {MaxRolls: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	b.Segment.Current = segment.Offensive
	b.Flow = state.NewSegmentFlowState(b.Segment)
	r := b.Settled.Actors["player"]
	r.OffensiveAbilityIDs = lib.Combatants["adventurer"].AbilityBoard.Offensive
	r.DefensiveAbilityIDs = lib.Combatants["adventurer"].AbilityBoard.Defensive
	r.UsedAbilities = map[string]int{}
	r.CardInstances = map[string]state.CardInstance{}
	b.Settled.Actors["player"] = r
	er := b.Settled.Actors["enemy"]
	er.OffensiveAbilityIDs = []string{"brine_lash"}
	er.DefensiveAbilityIDs = []string{"salt_veil"}
	er.UsedAbilities = map[string]int{}
	er.CardInstances = map[string]state.CardInstance{}
	b.Settled.Actors["enemy"] = er
	for _, id := range []string{"player", "enemy"} {
		a := b.Actors[id]
		for n := 0; n < 8; n++ {
			a.Cards.Deck = append(a.Cards.Deck, fmt.Sprintf("%s-card-%d", id, n))
		}
		b.Actors[id] = a
	}
	b.Settled.Stage = stageOffensivePlan
	openSettledWindowForActors(&b, "test", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningKeep, command.TypePlanningReroll, command.TypePlanningAbility, command.TypePlanningTargets, command.TypePlanningPass}, []string{"player"}, true)
	return b, lib
}

func planningCommand(b *state.Battle, kind command.Type, payload any) command.Command {
	pending := b.Flow.PendingInput["player"]
	switch p := payload.(type) {
	case *command.PlanningRollPayload:
		p.PendingInputID, p.Checkpoint = pending.ID, planningCheckpoint(pending)
	case *command.PlanningKeepPayload:
		p.PendingInputID, p.Checkpoint = pending.ID, planningCheckpoint(pending)
	case *command.PlanningRerollPayload:
		p.PendingInputID, p.Checkpoint = pending.ID, planningCheckpoint(pending)
	}
	return legalCommand(b.ID, "player", kind, payload)
}

func legalTypes(b *state.Battle, lib content.BattleLibrary) map[command.Type]int {
	counts := map[command.Type]int{}
	for _, a := range settledLegalActions(b, lib, "player", b.Flow.PendingInput["player"]) {
		counts[a.Type]++
	}
	return counts
}

func TestAmbushRollsBonusDiceThenKeepsFive(t *testing.T) {
	ambush := []state.StatusState{{InstanceID: "ambush-1", DefinitionID: "ambush", Stacks: 3}}
	b, lib := expeditionStatusFixture(t, ambush, nil)
	e := NewEngine()
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningRoll, &command.PlanningRollPayload{})); err != nil {
		t.Fatal(err)
	}
	r := b.Settled.Actors["player"]
	if len(r.FinalDice) != 8 || r.RollsUsed != 1 || r.AmbushChoose != 5 || len(r.RollHistory) != 1 || len(r.RollHistory[0].Dice) != 8 {
		t.Fatalf("an Ambush first roll rolls 5+3 dice: %d dice, rolls %d, choose %d", len(r.FinalDice), r.RollsUsed, r.AmbushChoose)
	}
	for i, die := range r.FinalDice {
		if die.Index != i || die.Face < 1 || die.Face > 6 || die.DieID != "standard_d6" || len(die.Symbols) != 1 {
			t.Fatalf("bonus die %d is malformed: %+v", i, die)
		}
	}
	if stacks(&b, "player", "ambush") != 0 {
		t.Fatal("Ambush is used up by the first roll")
	}
	// Only "keep exactly five" is legal while the choice is open.
	kinds := legalTypes(&b, lib)
	if kinds[command.TypePlanningKeep] != 56 || len(kinds) != 1 {
		t.Fatalf("while choosing, only the 56 five-dice keeps are legal: %v", kinds)
	}
	before, _ := json.Marshal(b)
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningKeep, &command.PlanningKeepPayload{KeptIndices: []int{0, 1, 2, 3}})); err == nil || !strings.Contains(err.Error(), "exactly 5") {
		t.Fatalf("keeping four of eight must be refused: %v", err)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningReroll, &command.PlanningRerollPayload{RerollIndices: []int{5, 6, 7}})); err == nil || !strings.Contains(err.Error(), "exactly 5") {
		t.Fatalf("rerolling before the choice must be refused: %v", err)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningKeep, &command.PlanningKeepPayload{KeptIndices: []int{0, 1, 2, 3, 9}})); err == nil {
		t.Fatal("an out-of-range index must be refused")
	}
	after, _ := json.Marshal(b)
	if string(before) != string(after) {
		t.Fatal("refused commands must not change the battle")
	}
	// Reload mid-choice: the saved battle continues identically.
	var twin state.Battle
	if err := json.Unmarshal(before, &twin); err != nil {
		t.Fatal(err)
	}
	chosen := []int{7, 5, 3, 1, 0}
	wantFaces := []int{}
	for _, i := range chosen {
		wantFaces = append(wantFaces, r.FinalDice[i].Face)
	}
	sort.Ints(wantFaces)
	for _, battle := range []*state.Battle{&b, &twin} {
		if _, err := e.handleOffensivePlanningCommand(battle, lib, planningCommand(battle, command.TypePlanningKeep, &command.PlanningKeepPayload{KeptIndices: chosen})); err != nil {
			t.Fatal(err)
		}
		r = battle.Settled.Actors["player"]
		gotFaces := []int{}
		for i, die := range r.FinalDice {
			gotFaces = append(gotFaces, die.Face)
			if die.Index != i || die.OwnedID == "" {
				t.Fatalf("kept die %d must be re-stamped onto owned die %d: %+v", i, i, die)
			}
		}
		sort.Ints(gotFaces)
		if len(r.FinalDice) != 5 || fmt.Sprint(gotFaces) != fmt.Sprint(wantFaces) || r.AmbushChoose != 0 || r.RollsUsed != 1 || len(r.KeptIndices) != 0 {
			t.Fatalf("after choosing: %d dice %v (want %v), choose %d, rolls %d, kept %v", len(r.FinalDice), gotFaces, wantFaces, r.AmbushChoose, r.RollsUsed, r.KeptIndices)
		}
		if len(r.RollHistory[0].KeptIndices) != 5 {
			t.Fatalf("the roll history records the chosen five: %+v", r.RollHistory[0])
		}
	}
	// The usual flow continues: keep three, reroll two, back to five; max 3 rolls.
	kinds = legalTypes(&b, lib)
	if kinds[command.TypePlanningReroll] == 0 || kinds[command.TypePlanningKeep] == 0 || kinds[command.TypePlanningPass] == 0 {
		t.Fatalf("normal planning resumes after the choice: %v", kinds)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningKeep, &command.PlanningKeepPayload{KeptIndices: []int{0, 1, 2}})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningReroll, &command.PlanningRerollPayload{RerollIndices: []int{3, 4}})); err != nil {
		t.Fatal(err)
	}
	r = b.Settled.Actors["player"]
	if len(r.FinalDice) != 5 || r.RollsUsed != 2 {
		t.Fatalf("a reroll returns to five dice: %d, rolls %d", len(r.FinalDice), r.RollsUsed)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningReroll, &command.PlanningRerollPayload{RerollIndices: []int{3}})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningReroll, &command.PlanningRerollPayload{RerollIndices: []int{3}})); err == nil {
		t.Fatal("max rolls is still 3")
	}
}

func TestAmbushIsNotReusedAndExpiresUnused(t *testing.T) {
	ambush := []state.StatusState{{InstanceID: "ambush-1", DefinitionID: "ambush", Stacks: 3}}
	b, lib := expeditionStatusFixture(t, ambush, nil)
	e := NewEngine()
	// Without an Ambush (or once it is used) the first roll is five dice.
	plain, _ := expeditionStatusFixture(t, nil, nil)
	if _, err := e.handleOffensivePlanningCommand(&plain, lib, planningCommand(&plain, command.TypePlanningRoll, &command.PlanningRollPayload{})); err != nil {
		t.Fatal(err)
	}
	if len(plain.Settled.Actors["player"].FinalDice) != 5 || plain.Settled.Actors["player"].AmbushChoose != 0 {
		t.Fatal("no Ambush: five dice")
	}
	// Used in round 1, then round 2's roll is an ordinary five dice.
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningRoll, &command.PlanningRollPayload{})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningKeep, &command.PlanningKeepPayload{KeptIndices: []int{0, 1, 2, 3, 4}})); err != nil {
		t.Fatal(err)
	}
	r := b.Settled.Actors["player"]
	r.RollsUsed, r.FinalDice, r.RollHistory, r.KeptIndices = 0, nil, nil, nil
	b.Settled.Actors["player"] = r
	b.Segment.Round = 2
	if _, err := e.handleOffensivePlanningCommand(&b, lib, planningCommand(&b, command.TypePlanningRoll, &command.PlanningRollPayload{})); err != nil {
		t.Fatal(err)
	}
	if len(b.Settled.Actors["player"].FinalDice) != 5 || stacks(&b, "player", "ambush") != 0 {
		t.Fatal("Ambush never applies twice")
	}
	// Unused: the status expires at the end of the first Offensive segment.
	unused, _ := expeditionStatusFixture(t, ambush, nil)
	if stacks(&unused, "player", "ambush") != 3 {
		t.Fatal("fixture carries the Ambush")
	}
	expireFirstRollBonuses(&unused, lib)
	if stacks(&unused, "player", "ambush") != 0 {
		t.Fatal("an unused Ambush expires at Offensive exit")
	}
	// The exit path itself removes it: close planning without rolling.
	unused2, _ := expeditionStatusFixture(t, ambush, nil)
	unused2.Settled.Window = nil
	unused2.Settled.Stage = "complete"
	if _, err := e.advanceSettledSegment(&unused2); err != nil {
		t.Fatal(err)
	}
	if stacks(&unused2, "player", "ambush") != 0 || unused2.Segment.Current == segment.Offensive {
		t.Fatalf("Offensive exit expires Ambush: stacks %d segment %s", stacks(&unused2, "player", "ambush"), unused2.Segment.Current)
	}
}

func TestFortifiedAndConquerorAddDefensiveDice(t *testing.T) {
	e := NewEngine()
	for _, c := range []struct {
		name     string
		statuses []state.StatusState
		want     int
	}{
		{"plain", nil, 1},
		{"fortified", []state.StatusState{{InstanceID: "f", DefinitionID: "fortified", Stacks: 1}}, 2},
		{"fortified and conqueror", []state.StatusState{{InstanceID: "f", DefinitionID: "fortified", Stacks: 1}, {InstanceID: "c", DefinitionID: "conqueror", Stacks: 1}}, 3},
		{"two fortified stacks", []state.StatusState{{InstanceID: "f", DefinitionID: "fortified", Stacks: 2}}, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, lib := expeditionStatusFixture(t, nil, c.statuses)
			b.Segment.Current = segment.Defensive
			selection := state.SettledDefense{ActorID: "enemy", AbilityID: "salt_veil", SourceID: "incoming"}
			events, err := e.rollSelectedDefense(&b, lib, &selection)
			if err != nil {
				t.Fatal(err)
			}
			if len(selection.RolledFaces) != c.want || len(selection.RolledDice) != c.want || len(events) != 1 || len(events[0].Dice) != c.want {
				t.Fatalf("Salt Veil rolls %d dice with %s, got %d", c.want, c.name, len(selection.RolledFaces))
			}
			// Every rolled face feeds its own outcome.
			if faces := defenseFaces(selection); len(faces) != c.want {
				t.Fatalf("per-face outcomes cover %d faces, got %d", c.want, len(faces))
			}
			for _, face := range selection.RolledFaces {
				if face < 1 || face > 6 {
					t.Fatalf("face %d", face)
				}
			}
		})
	}
}

func TestLieutenantsFavorAddsDamagePerStack(t *testing.T) {
	for _, stacks := range []int{0, 1, 2} {
		var statuses []state.StatusState
		if stacks > 0 {
			statuses = []state.StatusState{{InstanceID: "lf", DefinitionID: "lieutenants_favor", Stacks: stacks}}
		}
		b, lib := expeditionStatusFixture(t, statuses, nil)
		// Three Swords: Strike's lowest tier.
		adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
		r := b.Settled.Actors["player"]
		r.SelectedAbilityID = "adventurer_strike"
		r.SelectedTargetIDs = []string{"enemy"}
		b.Settled.Actors["player"] = r
		ops, ok := resolvedOffensiveOperations(&b, lib, "player")
		if !ok {
			t.Fatal("Strike qualifies on three Swords")
		}
		base := summarizeOffensiveOutcome(ops, []string{"enemy"})["base_damage"].(int)
		b2, _ := expeditionStatusFixture(t, nil, nil)
		adventurerRoll(&b2, lib, []int{1, 2, 3, 4, 5})
		r2 := b2.Settled.Actors["player"]
		r2.SelectedAbilityID = "adventurer_strike"
		r2.SelectedTargetIDs = []string{"enemy"}
		b2.Settled.Actors["player"] = r2
		plainOps, _ := resolvedOffensiveOperations(&b2, lib, "player")
		plain := summarizeOffensiveOutcome(plainOps, []string{"enemy"})["base_damage"].(int)
		if base != plain+stacks {
			t.Fatalf("%d stacks: damage %d, want %d+%d", stacks, base, plain, stacks)
		}
		// A miss stays a miss: no Swords, no damage, no bonus.
		adventurerRoll(&b, lib, []int{6, 6, 6, 6, 6})
		r = b.Settled.Actors["player"]
		r.SelectedAbilityID = "adventurer_strike"
		b.Settled.Actors["player"] = r
		if _, ok := resolvedOffensiveOperations(&b, lib, "player"); ok {
			t.Fatal("no bonus without a qualifying attack")
		}
	}
}

// Map conditions come back at the next Ongoing Effects after an ordinary
// removal; a removal for the battle (Siege Dispel) keeps them gone.
func TestMapConditionsReturnUnlessRemovedForBattle(t *testing.T) {
	e := NewEngine()
	b, lib := expeditionStatusFixture(t, nil, []state.StatusState{{InstanceID: "f", DefinitionID: "fortified", Stacks: 1}, {InstanceID: "c", DefinitionID: "conqueror", Stacks: 1}})
	recordRestoredStatuses(&b, lib)
	if err := e.applyEffectMutations(&b, lib, "", effectResult{StatusRemovals: []state.SettledStatusRemoval{
		{ActorID: "enemy", StatusID: "fortified", ForBattle: true},
		{ActorID: "enemy", StatusID: "conqueror"},
	}}); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy", "fortified") != 0 || stacks(&b, "enemy", "conqueror") != 0 {
		t.Fatal("both removals apply this round")
	}
	for round := 0; round < 2; round++ {
		restoreRoundStatuses(&b, lib)
		if stacks(&b, "enemy", "fortified") != 0 || stacks(&b, "enemy", "conqueror") != 1 {
			t.Fatalf("round %d: fortified %d (removed for the battle), conqueror %d (returns)", round, stacks(&b, "enemy", "fortified"), stacks(&b, "enemy", "conqueror"))
		}
	}
	// The removal survives a save: it lives in the actor's runtime.
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var loaded state.Battle
	if err = json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	restoreRoundStatuses(&loaded, lib)
	if stacks(&loaded, "enemy", "fortified") != 0 {
		t.Fatal("reload restored Fortified")
	}
}
