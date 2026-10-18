package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func programFixture(t *testing.T, steps ...content.CardStep) (state.Battle, content.BattleLibrary) {
	t.Helper()
	b, lib := generalFixture(t)
	b.Segment.Current = segment.Offensive
	b.Settled.Initialized = true
	c := content.BattleCardDefinition{SchemaVersion: 1, ID: "custom_test", Name: "Custom Test", Cost: content.BattleCost{Energy: 2}, Play: content.CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "discard"}, Targeting: content.TargetingDefinition{Selector: "card_program", Minimum: 1, Maximum: 1}, Program: &content.CardProgram{Version: 1, Windows: []string{"offensive_planning"}, RollRequirement: "any", Steps: steps}}
	c = content.PrepareProgramCard(c, &lib)
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[c.ID] = c
	giveGeneralCard(&b, c.ID)
	b.SettledCatalog, _ = json.Marshal(lib)
	return b, lib
}
func selfStep(effect string, params map[string]any) content.CardStep {
	return content.CardStep{Effect: effect, Target: content.CardTarget{Owner: "self", Mode: "one", Selection: "choose", Count: 1}, Params: params}
}
func programAction(t *testing.T, b *state.Battle, lib content.BattleLibrary, verb string) command.Command {
	t.Helper()
	// The starter deck is program cards too; start the card under test, which
	// tests add to hand last.
	hand := b.Actors["player"].Cards.Hand
	var fallback *command.Command
	for _, a := range programActions(b, lib, "player", b.Flow.PendingInput["player"]) {
		id, key := programPayload(a)
		var c programChoice
		_ = json.Unmarshal([]byte(key), &c)
		if c.Verb != verb || c.Then != "" {
			continue
		}
		if verb != "start" || len(hand) == 0 || id == hand[len(hand)-1] {
			return a
		}
		if fallback == nil {
			fallback = &a
		}
	}
	if fallback != nil {
		return *fallback
	}
	t.Fatalf("no %s action", verb)
	return command.Command{}
}

// playProgramCard plays one program card instance through the program handler,
// answering each target or option with the first choice pick accepts (nil
// accepts any) and confirming multi-target selections. A card without an
// acceptable choice is cancelled and reported as an error, so callers can
// assert rejections as well as legal plays.
func playProgramCard(e Engine, b *state.Battle, lib content.BattleLibrary, actor, instance string, pick func(programChoice) bool) error {
	find := func(match func(string, programChoice) bool) (command.Command, bool) {
		for _, a := range programActions(b, lib, actor, b.Flow.PendingInput[actor]) {
			id, key := programPayload(a)
			var c programChoice
			if json.Unmarshal([]byte(key), &c) == nil && match(id, c) {
				return a, true
			}
		}
		return command.Command{}, false
	}
	start, ok := find(func(id string, c programChoice) bool { return id == instance && c.Verb == "start" && c.Then == "" })
	if !ok {
		return fmt.Errorf("%s is not playable now", instance)
	}
	if _, err := e.handleProgramCommand(b, lib, start); err != nil {
		return err
	}
	for i := 0; i < 32 && b.Settled.Actors[actor].CardExecution != nil; i++ {
		next, ok := find(func(_ string, c programChoice) bool {
			return (c.Verb == "target" || c.Verb == "option") && (pick == nil || pick(c))
		})
		if !ok {
			next, ok = find(func(_ string, c programChoice) bool { return c.Verb == "confirm" })
		}
		if !ok {
			if cancel, found := find(func(_ string, c programChoice) bool { return c.Verb == "cancel" }); found {
				_, _ = e.handleProgramCommand(b, lib, cancel)
			}
			return fmt.Errorf("%s has no acceptable choice", instance)
		}
		if _, err := e.handleProgramCommand(b, lib, next); err != nil {
			return err
		}
	}
	return nil
}

// programOffers reports whether any program action targets one card instance;
// the starter deck's own program cards may be playable alongside it.
func programOffers(b *state.Battle, lib content.BattleLibrary, instance string) bool {
	for _, a := range programActions(b, lib, "player", b.Flow.PendingInput["player"]) {
		if id, _ := programPayload(a); id == instance {
			return true
		}
	}
	return false
}
func runProgramAction(t *testing.T, b *state.Battle, lib content.BattleLibrary, verb string) {
	t.Helper()
	a := programAction(t, b, lib, verb)
	if _, err := NewEngine().ApplyBattleCommand(b, a); err != nil {
		t.Fatal(err)
	}
}
func TestProgramOrderedDrawThenSelectSacrificeDestination(t *testing.T) {
	move := selfStep("move_cards", map[string]any{"destination": "removed"})
	move.Target.Zones = []string{"hand"}
	move.Target.DrawnThisPlay = true
	b, lib := programFixture(t, selfStep("draw", map[string]any{"amount": 2}), move, selfStep("energy", map[string]any{"amount": 5}))
	a := b.Actors["player"]
	a.Cards.Deck = append(a.Cards.Deck, a.Cards.Hand[:2]...)
	a.Cards.Hand = a.Cards.Hand[2:]
	b.Actors["player"] = a
	before := a.CurrentHealth()
	energy := a.Resources.EnergyPoints
	runProgramAction(t, &b, lib, "start")
	if b.Settled.Actors["player"].CardExecution == nil {
		t.Fatal("must wait for drawn-card choice")
	}
	raw, _ := json.Marshal(b)
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	b = restored
	choices := programActions(&b, lib, "player", b.Flow.PendingInput["player"])
	if len(choices) != 2 {
		t.Fatalf("want exactly two newly drawn targets, got %d", len(choices))
	}
	runProgramAction(t, &b, lib, "target")
	if b.Actors["player"].CurrentHealth() != before-1 {
		t.Fatal("removed card must lose health")
	}
	if b.Actors["player"].Resources.EnergyPoints != energy-2+5 {
		t.Fatal("ordered costs/reward incorrect")
	}
	if b.Settled.Actors["player"].CardExecution != nil {
		t.Fatal("program did not complete")
	}
}
func TestProgramCancellationAndStaleChoice(t *testing.T) {
	s := selfStep("set_die", map[string]any{"faces": []int{2, 5}})
	b, lib := programFixture(t, s)
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
	before := b.Actors["player"].Resources.EnergyPoints
	runProgramAction(t, &b, lib, "start")
	old := programAction(t, &b, lib, "target")
	runProgramAction(t, &b, lib, "cancel")
	if b.Actors["player"].Resources.EnergyPoints != before {
		t.Fatal("cancel spent energy")
	}
	if _, err := NewEngine().ApplyBattleCommand(&b, old); err == nil {
		t.Fatal("stale choice accepted")
	}
}
func TestProgramSacrificeCostBeforeReward(t *testing.T) {
	s := selfStep("sacrifice", nil)
	s.Target.Mode = "exact"
	s.Target.Count = 2
	s.Target.Zones = []string{"hand"}
	b, lib := programFixture(t, s, selfStep("energy", map[string]any{"amount": 9}))
	health := b.Actors["player"].CurrentHealth()
	energy := b.Actors["player"].Resources.EnergyPoints
	runProgramAction(t, &b, lib, "start")
	runProgramAction(t, &b, lib, "target")
	if b.Actors["player"].CurrentHealth() != health || b.Actors["player"].Resources.EnergyPoints != energy {
		t.Fatal("partial selection paid cost")
	}
	runProgramAction(t, &b, lib, "target")
	if b.Actors["player"].CurrentHealth() != health-2 || b.Actors["player"].Resources.EnergyPoints != energy+7 {
		t.Fatal("sacrifice/reward incorrect")
	}
}
func TestProgramBonusDurations(t *testing.T) {
	for _, duration := range []string{"offensive", "round", "rounds", "next_use", "battle"} {
		t.Run(duration, func(t *testing.T) {
			s := selfStep("ability_bonus", map[string]any{"damage": 4, "duration": duration, "rounds": 2})
			b, lib := programFixture(t, s)
			adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
			runProgramAction(t, &b, lib, "start")
			runProgramAction(t, &b, lib, "target")
			rt := b.Settled.Actors["player"]
			if len(rt.AbilityModifiers) != 1 {
				t.Fatal("missing modifier")
			}
			m := rt.AbilityModifiers[0]
			if stacks(&b, "player", m.StatusID) != 1 {
				t.Fatal("missing visible status")
			}
			switch duration {
			case "offensive":
				expireOffensiveModifiers(&b)
			case "round":
				expireProgramRoundBonuses(&b)
			case "rounds":
				expireProgramRoundBonuses(&b)
				if len(b.Settled.Actors["player"].AbilityModifiers) != 1 {
					t.Fatal("expired a round early")
				}
				b.Segment.Round++
				expireProgramRoundBonuses(&b)
			case "next_use":
				consumeProgramBonuses(&b, "player", m.AbilityID)
			case "battle":
				expireOffensiveModifiers(&b)
				expireProgramRoundBonuses(&b)
				if len(b.Settled.Actors["player"].AbilityModifiers) != 1 {
					t.Fatal("battle duration expired")
				}
				clearCompletedProgramBonuses(&b)
			}
			if len(b.Settled.Actors["player"].AbilityModifiers) != 0 || stacks(&b, "player", m.StatusID) != 0 {
				t.Fatal("modifier/status expiration disagree")
			}
		})
	}
}

func TestProgramMultiDieTargetsAreDistinct(t *testing.T) {
	for _, mode := range []string{"exact", "up_to", "all"} {
		t.Run(mode, func(t *testing.T) {
			s := selfStep("set_die", map[string]any{"faces": []int{5, 6}})
			s.Target.Mode = mode
			s.Target.Count = 2
			b, lib := programFixture(t, s)
			adventurerRoll(&b, lib, []int{1, 2, 3, 4, 1})
			runProgramAction(t, &b, lib, "start")
			runProgramAction(t, &b, lib, "target")
			for _, a := range programActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
				_, key := programPayload(a)
				var c programChoice
				_ = json.Unmarshal([]byte(key), &c)
				if c.Verb == "target" && c.Die == 0 {
					t.Fatal("selected die offered twice")
				}
			}
			count := 2
			if mode == "all" {
				count = 5
			}
			if mode == "up_to" {
				runProgramAction(t, &b, lib, "confirm")
				count = 1
			} else {
				for i := 1; i < count; i++ {
					runProgramAction(t, &b, lib, "target")
				}
			}
			if b.Settled.Actors["player"].CardExecution != nil {
				t.Fatal("targeting never completed")
			}
			changed := 0
			for _, d := range b.Settled.Actors["player"].FinalDice {
				if d.Face == 5 {
					changed++
				}
			}
			if changed != count {
				t.Fatalf("changed %d dice, want %d", changed, count)
			}
		})
	}
}
func TestProgramDiceFamilies(t *testing.T) {
	cases := []struct {
		effect string
		params map[string]any
		want   int
	}{
		{"adjust_die", map[string]any{"deltas": []int{-2}, "minimum": 1, "maximum": 6, "wrap": true}, 6},
		{"flip_die", map[string]any{"sum": 7}, 5},
		{"copy_die", nil, 3},
	}
	for _, tc := range cases {
		t.Run(tc.effect, func(t *testing.T) {
			b, lib := programFixture(t, selfStep(tc.effect, tc.params))
			adventurerRoll(&b, lib, []int{2, 3, 4, 5, 6})
			runProgramAction(t, &b, lib, "start")
			runProgramAction(t, &b, lib, "target")
			if got := b.Settled.Actors["player"].FinalDice[0].Face; got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
	for _, keep := range []string{"higher", "lower", "replace"} {
		t.Run(keep, func(t *testing.T) {
			s := selfStep("reroll", map[string]any{"result": keep, "consume_roll": true})
			s.Target.Mode = "all"
			b, lib := programFixture(t, s)
			adventurerRoll(&b, lib, []int{3, 3, 3, 3, 3})
			rt0 := b.Settled.Actors["player"]
			rt0.RollsUsed = 1
			rt0.MaxRolls = 3
			b.Settled.Actors["player"] = rt0
			used := rt0.RollsUsed
			runProgramAction(t, &b, lib, "start")
			rt := b.Settled.Actors["player"]
			if rt.RollsUsed != used+1 {
				t.Fatal("multi-reroll must consume one attempt")
			}
			for _, d := range rt.FinalDice {
				if keep == "higher" && d.Face < 3 || keep == "lower" && d.Face > 3 {
					t.Fatal("keep rule ignored")
				}
			}
		})
	}
}
func TestProgramNoImplicitRecyclingAndExactCount(t *testing.T) {
	b, lib := programFixture(t, selfStep("draw", map[string]any{"amount": 100}))
	a := b.Actors["player"]
	a.Cards.Discard = append(a.Cards.Discard, a.Cards.Deck...)
	a.Cards.Deck = nil
	b.Actors["player"] = a
	n := len(a.Cards.Hand)
	health := a.CurrentHealth()
	runProgramAction(t, &b, lib, "start")
	if len(b.Actors["player"].Cards.Hand) != n-1 || b.Actors["player"].CurrentHealth() != health {
		t.Fatal("empty draw changed health or recycled discard")
	}
	s := selfStep("move_cards", map[string]any{"destination": "removed"})
	s.Target.Zones = []string{"hand"}
	s.Target.Mode = "exact"
	s.Target.Count = 100
	b, lib = programFixture(t, s)
	if programOffers(&b, lib, "custom_test") {
		t.Fatal("unpayable exact selection offered")
	}
}
func TestProgramChoiceAndStatus(t *testing.T) {
	status := selfStep("apply_status", map[string]any{"status_id": "protect", "stacks": 2})
	option := content.CardStep{Effect: "choice", Choices: []content.CardOption{{Name: "Ward", Energy: 1, Steps: []content.CardStep{status}}, {Name: "Energy", Steps: []content.CardStep{selfStep("energy", map[string]any{"amount": 3})}}}}
	b, lib := programFixture(t, option)
	before := b.Actors["player"].Resources.EnergyPoints
	runProgramAction(t, &b, lib, "start")
	runProgramAction(t, &b, lib, "option")
	if stacks(&b, "player", "protect") != 2 || b.Actors["player"].Resources.EnergyPoints != before-3 {
		t.Fatal("option price/status not applied")
	}
	remove := selfStep("remove_status", map[string]any{"stacks": 1})
	remove.Target.Polarity = "positive"
	b, lib = programFixture(t, remove)
	applyStatus(&b, lib, "player", "protect", 2)
	runProgramAction(t, &b, lib, "start")
	if stacks(&b, "player", "protect") != 1 {
		t.Fatal("configured stack removal failed")
	}
}
func TestProgramPreventionDestinations(t *testing.T) {
	for _, unified := range []bool{false, true} {
		for _, dest := range []string{"original", "discard", "deck", "hand", "removed"} {
			for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneHand, operation.ZoneDiscard} {
				t.Run(fmt.Sprintf("%v/%s/%s", unified, dest, zone), func(t *testing.T) {
					b, lib := adventurerFixture(t)
					b.Settled.UnifiedDefense = unified
					card, _ := content.EditableGeneralCard(lib.Cards["steady_guard"])
					card.ID = "configured_guard"
					card.Name = "Configured Guard"
					card.Program.Steps[0].Params["amount"] = 3
					card.Program.Steps[0].Params["destination"] = dest
					lib.Cards[card.ID] = card
					giveGeneralCard(&b, card.ID)
					b.SettledCatalog, _ = json.Marshal(lib)
					id := "configured_guard-test"
					for key, inst := range b.Settled.Actors["player"].CardInstances {
						if inst.DefinitionID == card.ID {
							id = key
						}
					}
					a := b.Actors["player"]
					a.Cards = state.CardZones{Hand: []string{id}}
					setZone(&a.Cards, zone, append(zoneCards(a.Cards, zone), "nudge-0", "nudge-1", "strong_swing-0", "strong_swing-1"))
					b.Actors["player"] = a
					b.Segment.Current = segment.DamageResolution
					b.Settled.Stage = stageDamageReact
					if unified {
						b.Segment.Current = segment.Defensive
						b.Settled.Stage = stageDefenseSelect
					}
					e := NewEngine()
					batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 4}})
					if err != nil {
						t.Fatal(err)
					}
					b.Settled.PendingDamage = batch
					openSettledWindow(&b, "damage", b.Settled.Stage, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
					// Exercise the real card command handler, leaving phase progression to separate full-battle tests.
					cmd := programAction(t, &b, lib, "start")
					if _, err = e.handleProgramCommand(&b, lib, cmd); err != nil {
						t.Fatal(err)
					}
					if batch.Sources[0].FinalAmount != 1 {
						t.Fatalf("wrong prevention %d", batch.Sources[0].FinalAmount)
					}
					saved := 0
					for _, r := range batch.Removals {
						if !r.Released {
							continue
						}
						saved++
						want := operation.CardZone(dest)
						if dest == "original" {
							want = zone
						}
						if r.CardID == id && dest != "removed" {
							want = operation.ZoneDiscard
						}
						if programCardZone(&b, "player", r.CardID) != want || r.ReleasedDestination != want {
							t.Fatalf("destination mismatch %+v, live %s want %s", r, programCardZone(&b, "player", r.CardID), want)
						}
					}
					if saved != 3 {
						t.Fatalf("want 3 saved, got %d", saved)
					}
					before, _ := json.Marshal(b.Actors["player"].Cards)
					if err = e.reconcilePreventionDestination(&b, dest); err != nil {
						t.Fatal(err)
					}
					after, _ := json.Marshal(b.Actors["player"].Cards)
					if string(before) != string(after) {
						t.Fatal("reconciliation moved cards twice")
					}
					if _, err = e.finishDamageBatch(&b, lib); err != nil {
						t.Fatal(err)
					}
					wantHealth := 4
					if dest == "removed" {
						wantHealth = 1
					}
					if b.Actors["player"].CurrentHealth() != wantHealth {
						t.Fatalf("damage commit health=%d want %d", b.Actors["player"].CurrentHealth(), wantHealth)
					}
				})
			}
		}
	}
}

func TestProgramSacrificeLethalAndUsageLimit(t *testing.T) {
	b, lib := programFixture(t, selfStep("energy", map[string]any{"amount": 1}))
	c := lib.Cards["custom_test"]
	c.Play.Destination = "removed"
	lib.Cards[c.ID] = c
	b.SettledCatalog, _ = json.Marshal(lib)
	a := b.Actors["player"]
	a.Cards = state.CardZones{Hand: []string{c.ID}}
	b.Actors["player"] = a
	a = b.Actors["enemy"]
	a.Cards.Deck = []string{"enemy-card"}
	b.Actors["enemy"] = a
	runProgramAction(t, &b, lib, "start")
	if !state.IsTerminalBattleStatus(b.Status) || b.Actors["player"].CurrentHealth() != 0 || len(b.Wounds) != 1 {
		t.Fatal("last-card removal must end battle and record wound")
	}
	b, lib = programFixture(t, selfStep("energy", map[string]any{"amount": 1}))
	c = lib.Cards["custom_test"]
	c.Play.Destination = "hand"
	c.Program.UsesPerBattle = 1
	lib.Cards[c.ID] = c
	b.SettledCatalog, _ = json.Marshal(lib)
	runProgramAction(t, &b, lib, "start")
	if programOffers(&b, lib, "custom_test") {
		t.Fatal("once-per-battle ignored")
	}
}
func TestProgramBonusStacksDispelAndReapplication(t *testing.T) {
	s := selfStep("ability_bonus", map[string]any{"damage": 3, "duration": "next_use", "stack_limit": 2, "stacking": "stack"})
	b, lib := programFixture(t, s)
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
	target := programChoice{Actor: "player", Ability: "adventurer_small_straight"}
	for i := 0; i < 3; i++ {
		if err := applyProgramBonus(&b, lib, "player", "custom_test", target, s); err != nil {
			t.Fatal(err)
		}
	}
	status := content.ProgramStatusID("custom_test", s)
	if stacks(&b, "player", status) != 2 || len(b.Settled.Actors["player"].AbilityModifiers) != 2 {
		t.Fatal("stack cap ignored")
	}
	removeStatus(&b, "player", status, 1)
	if len(b.Settled.Actors["player"].AbilityModifiers) != 1 {
		t.Fatal("dispel left hidden bonus")
	}
	removeStatus(&b, "player", status, 0)
	if err := applyProgramBonus(&b, lib, "player", "custom_test", target, s); err != nil {
		t.Fatal(err)
	}
	if len(b.Settled.Actors["player"].AbilityModifiers) != 1 {
		t.Fatal("reapplication revived old modifiers")
	}
	rt := b.Settled.Actors["player"]
	rt.SelectedAbilityID = "adventurer_small_straight"
	b.Settled.Actors["player"] = rt
	ops, _ := resolvedOffensiveOperations(&b, lib, "player")
	bonus := false
	for _, op := range ops {
		if op.Type == "deal_damage" && op.Amount == 3 {
			bonus = true
		}
	}
	if !bonus {
		t.Fatal("configured damage bonus absent from attack")
	}
	consumeProgramBonuses(&b, "player", "adventurer_small_straight")
	if stacks(&b, "player", status) != 0 {
		t.Fatal("next-use bonus not consumed")
	}
}

func TestProgramChosenSaveAndDefensiveReroll(t *testing.T) {
	b, lib := generalFixture(t)
	c, _ := content.EditableGeneralCard(lib.Cards["triage"])
	c.ID = "chosen_save"
	c.Program.Steps[0].Params["destination"] = "discard"
	lib.Cards[c.ID] = c
	giveGeneralCard(&b, c.ID)
	b.SettledCatalog, _ = json.Marshal(lib)
	b.Settled.UnifiedDefense = true
	b.Segment.Current = segment.Defensive
	b.Settled.Stage = stageDefenseSelect
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 3}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(&b, "save", stageDefenseSelect, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if _, err = e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "start")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "target")); err != nil {
		t.Fatal(err)
	}
	released := 0
	for _, r := range batch.Removals {
		if r.Released {
			released++
			if programCardZone(&b, "player", r.CardID) != operation.ZoneDiscard {
				t.Fatal("selected save destination ignored")
			}
		}
	}
	if released != 1 || batch.Sources[0].FinalAmount != 2 {
		t.Fatalf("chosen save released %d damage=%d", released, batch.Sources[0].FinalAmount)
	}
	b, lib = generalFixture(t)
	c, _ = content.EditableGeneralCard(lib.Cards["second_guard"])
	c.ID = "new_guard"
	c.Program.Steps[0].Target.Mode = "all"
	lib.Cards[c.ID] = c
	giveGeneralCard(&b, c.ID)
	b.SettledCatalog, _ = json.Marshal(lib)
	b.Segment.Current = segment.Defensive
	b.Settled.Stage = stageDefenseReact
	dice := rolledCombatFaces(lib, b.Actors["player"].DiceLoadout, []int{1, 2, 3})
	b.Settled.DefenseSelections["player"] = state.SettledDefense{RolledDice: dice, RolledFaces: []int{1, 2, 3}, RolledFace: 1}
	openSettledWindow(&b, "reroll", stageDefenseReact, "defense_reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if _, err = e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "start")); err != nil {
		t.Fatal(err)
	}
	sel := b.Settled.DefenseSelections["player"]
	for i, d := range sel.RolledDice {
		if d.Face < 1 || d.Face > 6 || sel.RolledFaces[i] != d.Face {
			t.Fatal("defense reroll state disagrees")
		}
	}
}

func TestProgramSacrificeReservedCardCommitsOnce(t *testing.T) {
	b, lib := adventurerFixture(t)
	s := selfStep("sacrifice", nil)
	s.Target.Mode = "exact"
	s.Target.Count = 1
	s.Target.Zones = []string{"hand"}
	c, _ := content.EditableGeneralCard(lib.Cards["steady_guard"])
	c.ID = "sacrificial_ward"
	c.Program.Steps = []content.CardStep{s, selfStep("energy", map[string]any{"amount": 4})}
	c = content.PrepareProgramCard(c, &lib)
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[c.ID] = c
	giveGeneralCard(&b, c.ID)
	b.SettledCatalog, _ = json.Marshal(lib)
	b.Segment.Current = segment.DamageResolution
	b.Settled.Stage = stageDamageReact
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 3}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	health := b.Actors["player"].CurrentHealth()
	if _, err = e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "start")); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range programActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
		_, key := programPayload(cmd)
		var choice programChoice
		_ = json.Unmarshal([]byte(key), &choice)
		if choice.Verb != "target" {
			continue
		}
		for _, r := range batch.Removals {
			if choice.Card == r.CardID {
				if _, err = e.handleProgramCommand(&b, lib, cmd); err != nil {
					t.Fatal(err)
				}
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no reserved card selectable for sacrifice")
	}
	if _, err = e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Actors["player"].CurrentHealth() != health-3 {
		t.Fatal("reserved sacrifice was charged twice")
	}
	seen := map[string]bool{}
	for _, w := range b.Wounds {
		for _, card := range w.Cards {
			if seen[card.CardID] {
				t.Fatal("duplicate wound for sacrificed card")
			}
			seen[card.CardID] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("want 3 distinct wounds, got %d", len(seen))
	}
}

// An extra roll attempt raises this Offense's roll count, even after the last
// roll (the count returns to 3 at the next round's Ongoing Effects).
func TestProgramExtraRollAttempt(t *testing.T) {
	b, lib := programFixture(t, selfStep("extra_roll", map[string]any{"amount": 1}))
	adventurerRoll(&b, lib, []int{2, 3, 4, 5, 6})
	rt := b.Settled.Actors["player"]
	rt.RollsUsed, rt.MaxRolls = 3, 3
	b.Settled.Actors["player"] = rt
	// The real Offense planning window, as the round opens it.
	openSettledWindowForActors(&b, "offensive-planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningKeep, command.TypePlanningReroll, command.TypePlanningAbility, command.TypePlanningTargets, command.TypePlanningPass}, []string{"player"}, true)
	rerolls := func() bool {
		for _, a := range settledLegalActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
			if a.Type == command.TypePlanningReroll {
				return true
			}
		}
		return false
	}
	if rerolls() {
		t.Fatal("no reroll should be left after the third roll")
	}
	if err := playProgramCard(NewEngine(), &b, lib, "player", b.Actors["player"].Cards.Hand[len(b.Actors["player"].Cards.Hand)-1], nil); err != nil {
		t.Fatal(err)
	}
	if got := b.Settled.Actors["player"].MaxRolls; got != 4 {
		t.Fatalf("max rolls = %d, want 4", got)
	}
	if !rerolls() {
		t.Fatal("the extra attempt must offer a fourth roll")
	}
	text := content.CardProgramRulesWithStatuses(lib.Cards["custom_test"].Program, nil)
	if !strings.Contains(text, "Gain 1 more offensive roll attempt this Offense") {
		t.Fatalf("rules text: %q", text)
	}
}

// stockDeck puts n plain card instances in the player's draw pile.
func stockDeck(b *state.Battle, lib content.BattleLibrary, n int) {
	var def string
	for id, c := range lib.Cards {
		if c.Program != nil && id != "custom_test" && (def == "" || id < def) {
			def = id
		}
	}
	a := b.Actors["player"]
	rt := b.Settled.Actors["player"]
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("deck-%d", i)
		a.Cards.Deck = append(a.Cards.Deck, id)
		rt.CardInstances[id] = state.CardInstance{InstanceID: id, DefinitionID: def}
	}
	b.Actors["player"] = a
	b.Settled.Actors["player"] = rt
}

func lookedStep(effect string, mode string, count int, params map[string]any) content.CardStep {
	return content.CardStep{Effect: effect, Target: content.CardTarget{Owner: "self", Mode: mode, Count: count, Selection: "choose", Zones: []string{"deck"}, Looked: true}, Params: params}
}

// Read Ahead: look at the top 3 of the draw pile, draw one of them, and put
// the rest back on top in the chosen order; draws then follow that order.
func TestProgramLookDrawAndOrder(t *testing.T) {
	b, lib := programFixture(t,
		selfStep("look_deck", map[string]any{"amount": 3}),
		lookedStep("move_cards", "exact", 1, map[string]any{"destination": "hand"}),
		lookedStep("order_deck", "up_to", 2, map[string]any{}))
	stockDeck(&b, lib, 6)
	openSettledWindowForActors(&b, "offensive-planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningPass}, []string{"player"}, true)
	hand := b.Actors["player"].Cards.Hand
	card := hand[len(hand)-1]
	steps := []string{}
	err := playProgramCard(NewEngine(), &b, lib, "player", card, func(c programChoice) bool {
		steps = append(steps, c.Label)
		x := b.Settled.Actors["player"].CardExecution
		if x == nil || len(x.Looked) < 3 {
			return false
		}
		queue := programQueue(x)
		if queue[0].Effect == "move_cards" {
			return c.Card == x.Looked[1] // draw the second card
		}
		// Order: the third card first; then confirm.
		return c.Card == x.Looked[2] && len(x.Selected) == 0
	})
	if err != nil {
		t.Fatalf("%v (choices %v)", err, steps)
	}
	rt := b.Settled.Actors["player"]
	if rt.CardExecution != nil {
		t.Fatal("the card did not finish")
	}
	if !strings.Contains(strings.Join(steps, "|"), "from the top") {
		t.Fatalf("looked-at choices should name their place: %v", steps)
	}
	if len(rt.DeckTop) != 2 {
		t.Fatalf("known top = %v, want the two cards put back", rt.DeckTop)
	}
	drawn := b.Actors["player"].Cards.Hand
	if !containsString(drawn, rt.DeckTop[0]) && containsString(b.Actors["player"].Cards.Deck, rt.DeckTop[0]) {
		first, err := NewEngine().drawSettledCard(&b, "player", "card_draw")
		if err != nil || first != rt.DeckTop[0] {
			t.Fatalf("the next draw must be the card put on top: got %q, want %q (%v)", first, rt.DeckTop[0], err)
		}
		second, _ := NewEngine().drawSettledCard(&b, "player", "card_draw")
		if second != rt.DeckTop[1] {
			t.Fatalf("then the card under it: got %q, want %q", second, rt.DeckTop[1])
		}
	}
	text := content.CardProgramRulesWithStatuses(lib.Cards["custom_test"].Program, nil)
	for _, want := range []string{"Look at the top 3 cards of your draw pile.", "Draw 1 of the cards you looked at.", "back on top of your draw pile in the order you choose"} {
		if !strings.Contains(text, want) {
			t.Fatalf("rules text %q lacks %q", text, want)
		}
	}
}

// Clear the Path: a looked-at card put on the bottom is drawn last and is no
// longer offered when the rest are put back.
func TestProgramLookBottom(t *testing.T) {
	b, lib := programFixture(t,
		selfStep("look_deck", map[string]any{"amount": 3}),
		lookedStep("move_cards", "up_to", 3, map[string]any{"destination": "deck_bottom"}),
		lookedStep("order_deck", "up_to", 3, map[string]any{}))
	stockDeck(&b, lib, 6)
	openSettledWindowForActors(&b, "offensive-planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningPass}, []string{"player"}, true)
	hand := b.Actors["player"].Cards.Hand
	var bottom string
	orderOffered := []string{}
	err := playProgramCard(NewEngine(), &b, lib, "player", hand[len(hand)-1], func(c programChoice) bool {
		x := b.Settled.Actors["player"].CardExecution
		if programQueue(x)[0].Effect == "move_cards" {
			if len(x.Selected) > 0 {
				return false
			}
			bottom = x.Looked[0]
			return c.Card == bottom
		}
		orderOffered = append(orderOffered, c.Card)
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if containsString(orderOffered, bottom) {
		t.Fatal("a card put on the bottom must not be offered for the top")
	}
	if got := b.Settled.Actors["player"].DeckBottom; len(got) != 1 || got[0] != bottom {
		t.Fatalf("known bottom = %v, want [%s]", got, bottom)
	}
	last := ""
	for len(b.Actors["player"].Cards.Deck) > 0 {
		last, _ = NewEngine().drawSettledCard(&b, "player", "card_draw")
	}
	if last != bottom {
		t.Fatalf("the bottom card must be drawn last: got %q, want %q", last, bottom)
	}
}
