package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
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
	for _, a := range programActions(b, lib, "player", b.Flow.PendingInput["player"]) {
		_, key := programPayload(a)
		var c programChoice
		_ = json.Unmarshal([]byte(key), &c)
		if c.Verb == verb {
			return a
		}
	}
	t.Fatalf("no %s action", verb)
	return command.Command{}
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
	if len(programActions(&b, lib, "player", b.Flow.PendingInput["player"])) != 0 {
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
					card, _ := content.EditableGeneralCard(lib.Cards["brace"])
					card.ID = "configured_brace"
					card.Name = "Configured Brace"
					card.Program.Steps[0].Params["amount"] = 3
					card.Program.Steps[0].Params["destination"] = dest
					lib.Cards[card.ID] = card
					giveGeneralCard(&b, card.ID)
					b.SettledCatalog, _ = json.Marshal(lib)
					id := "configured_brace-test"
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
	if len(programActions(&b, lib, "player", b.Flow.PendingInput["player"])) != 0 {
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
	c, _ := content.EditableGeneralCard(lib.Cards["brace"])
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
