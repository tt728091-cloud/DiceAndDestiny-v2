package engine

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// The shipped General cards are program cards. These tests play each one
// through the program handler and check the rule its card text promises.
func loadShippedGeneral(t *testing.T, b *state.Battle, lib content.BattleLibrary) content.BattleLibrary {
	t.Helper()
	lib, err := content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "general_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range generalCardIDs {
		if lib.Cards[id].Program == nil {
			t.Fatalf("%s is not a program card", id)
		}
	}
	b.SettledCatalog, _ = json.Marshal(lib)
	return lib
}
func addCardInstance(b *state.Battle, instance, definition string) {
	rt := b.Settled.Actors["player"]
	rt.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: definition}
	b.Settled.Actors["player"] = rt
}

func TestShippedGeneralDieCards(t *testing.T) {
	for _, tc := range []struct {
		id   string
		pick func(programChoice) bool
		want int
	}{
		{"matchmaker", func(c programChoice) bool { return c.Die == 0 && c.Face == 6 }, 6},
		{"turn_the_die", func(c programChoice) bool { return c.Die == 0 }, 6},
	} {
		t.Run(tc.id, func(t *testing.T) {
			b, lib := adventurerFixture(t)
			lib = loadShippedGeneral(t, &b, lib)
			giveGeneralCard(&b, tc.id)
			e := NewEngine()
			if err := playProgramCard(e, &b, lib, "player", tc.id, tc.pick); err == nil {
				t.Fatal("die card played before the first roll")
			}
			adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
			if err := playProgramCard(e, &b, lib, "player", tc.id, tc.pick); err != nil {
				t.Fatal(err)
			}
			r := b.Settled.Actors["player"]
			if r.FinalDice[0].Face != tc.want || r.FinalDice[4].Face != 6 || r.RollsUsed != 3 || b.Actors["player"].Resources.EnergyPoints != 5 {
				t.Fatalf("die %d, rolls %d, energy %d", r.FinalDice[0].Face, r.RollsUsed, b.Actors["player"].Resources.EnergyPoints)
			}
		})
	}
}

func TestShippedReclaimExcludesRecoveryAndPreservesHealth(t *testing.T) {
	b, lib := adventurerFixture(t)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "reclaim")
	addCardInstance(&b, "reclaim-2", "reclaim")
	a := b.Actors["player"]
	a.Cards.Hand = removeString(a.Cards.Hand, "nudge-0")
	a.Cards.Discard = append(a.Cards.Discard, "nudge-0", "reclaim-2")
	b.Actors["player"] = a
	health := a.CurrentHealth()
	// The other Reclaim is not eligible, so the only target resolves at once.
	if err := playProgramCard(NewEngine(), &b, lib, "player", "reclaim", nil); err != nil {
		t.Fatal(err)
	}
	a = b.Actors["player"]
	if !containsString(a.Cards.Hand, "nudge-0") || !containsString(a.Cards.Discard, "reclaim-2") || !containsString(a.Cards.Discard, "reclaim") || a.CurrentHealth() != health || a.Resources.EnergyPoints != 0 {
		t.Fatalf("Reclaim result %+v", a.Cards)
	}
}

func TestShippedDispelRemovesOnePositiveStack(t *testing.T) {
	b, lib := adventurerFixture(t)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "dispel")
	applyStatus(&b, lib, "enemy", "protect", 2)
	applyStatus(&b, lib, "enemy", "bleed", 2)
	// Bleed is negative, so Protect is the only target.
	if err := playProgramCard(NewEngine(), &b, lib, "player", "dispel", nil); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "enemy", "protect") != 1 || stacks(&b, "enemy", "bleed") != 2 {
		t.Fatal("Dispel must remove exactly one positive stack")
	}
}

func TestShippedReinforceExtraEnergyChoice(t *testing.T) {
	for _, tc := range []struct {
		option, energy, want int
	}{{0, 5, 3}, {1, 10, 1}} {
		b, lib, e := unifiedFixture(t, 5)
		lib = loadShippedGeneral(t, &b, lib)
		giveGeneralCard(&b, "reinforce")
		a := b.Actors["player"]
		a.Resources.EnergyPoints = tc.energy
		b.Actors["player"] = a
		if err := playProgramCard(e, &b, lib, "player", "reinforce", func(c programChoice) bool {
			return c.Verb == "option" && c.Option == tc.option || c.Verb == "target"
		}); err != nil {
			t.Fatal(err)
		}
		if got := settledSourceAmount(b.Settled.PendingDamage.Sources[0]); got != tc.want || b.Actors["player"].Resources.EnergyPoints != 0 {
			t.Fatalf("option %d left %d damage, energy %d", tc.option, got, b.Actors["player"].Resources.EnergyPoints)
		}
	}
	// Five energy pays only the card; the boosted option is not offered.
	b, lib, e := unifiedFixture(t, 5)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "reinforce")
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 5
	b.Actors["player"] = a
	if _, err := e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "start")); err != nil {
		t.Fatal(err)
	}
	for _, action := range programActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
		_, key := programPayload(action)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "option" && c.Option == 1 {
			t.Fatal("unaffordable boost offered")
		}
	}
}

func TestShippedTriageSavesTheChosenCard(t *testing.T) {
	b, lib, e := unifiedFixture(t, 3)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "triage")
	chosen := activeReservations(b, "a")[0]
	if err := playProgramCard(e, &b, lib, "player", "triage", func(c programChoice) bool { return c.Card == chosen }); err != nil {
		t.Fatal(err)
	}
	if containsString(activeReservations(b, "a"), chosen) || settledSourceAmount(b.Settled.PendingDamage.Sources[0]) != 2 {
		t.Fatal("Triage must save the chosen card and prevent 1")
	}
}

func TestShippedSecondGuardRerollsDefenseDice(t *testing.T) {
	b, lib := adventurerFixture(t)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "second_guard")
	b.Segment.Current = segment.Defensive
	b.Settled.Stage = stageDefenseReact
	dice := rolledCombatFaces(lib, b.Actors["player"].DiceLoadout, []int{1, 2, 3})
	b.Settled.DefenseSelections["player"] = state.SettledDefense{ActorID: "player", RolledDice: dice, RolledFaces: []int{1, 2, 3}, RolledFace: 1}
	openSettledWindow(&b, "reroll", stageDefenseReact, "defense_reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if err := playProgramCard(NewEngine(), &b, lib, "player", "second_guard", func(c programChoice) bool { return c.Die == 0 }); err != nil {
		t.Fatal(err)
	}
	sel := b.Settled.DefenseSelections["player"]
	if sel.RolledFaces[1] != 2 || sel.RolledFaces[2] != 3 || sel.RolledDice[0].Face != sel.RolledFaces[0] {
		t.Fatalf("Second Guard changed unchosen dice: %+v", sel.RolledFaces)
	}
}

func TestShippedDisruptRerollsOnlyAnEnemyDie(t *testing.T) {
	b, lib := adventurerFixture(t)
	lib = loadShippedGeneral(t, &b, lib)
	giveGeneralCard(&b, "disrupt")
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
	enemy := b.Settled.Actors["enemy"]
	enemy.FinalDice = rolledCombatFaces(lib, b.Actors["enemy"].DiceLoadout, []int{6, 6, 6, 6, 6})
	enemy.RollsUsed = 3
	b.Settled.Actors["enemy"] = enemy
	b.Settled.Stage = stageOffensiveReact
	openSettledWindow(&b, "react", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if err := playProgramCard(NewEngine(), &b, lib, "player", "disrupt", func(c programChoice) bool { return c.Actor == "player" }); err == nil {
		t.Fatal("Disrupt offered the player's own die")
	}
	if err := playProgramCard(NewEngine(), &b, lib, "player", "disrupt", func(c programChoice) bool { return c.Actor == "enemy" && c.Die == 0 }); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Actors["player"].FinalDice[0].Face != 1 || b.Settled.Actors["enemy"].RollsUsed != 3 {
		t.Fatal("Disrupt changed the player's dice or spent an enemy roll")
	}
}
