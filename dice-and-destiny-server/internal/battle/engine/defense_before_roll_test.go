package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// guardCard authors a prevent-1 program card with the given play windows.
func guardCard(t *testing.T, lib *content.BattleLibrary, id string, windows ...string) content.BattleCardDefinition {
	t.Helper()
	c, err := content.EditableGeneralCard(lib.Cards["brace"])
	if err != nil {
		t.Fatal(err)
	}
	c.ID, c.Name = id, id
	c.Program.Windows = windows
	c.Program.Steps[0].Params["amount"] = 1
	c = content.PrepareProgramCard(c, lib)
	if err = content.ValidateCardProgram(c, *lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[id] = c
	return c
}

func giveCardCopy(b *state.Battle, instance, definition string) {
	a := b.Actors["player"]
	a.Cards.Hand = append(a.Cards.Hand, instance)
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: definition}
	b.Settled.Actors["player"] = rt
}

// startActions maps each legally startable card instance to its command.
func startActions(e Engine, b *state.Battle) map[string]command.Command {
	out := map[string]command.Command{}
	for _, a := range e.LegalActions(b, "player") {
		id, key := programPayload(a)
		var c programChoice
		if id != "" && json.Unmarshal([]byte(key), &c) == nil && c.Verb == "start" {
			out[id] = a
		}
	}
	return out
}

func applyFirst(t *testing.T, e Engine, b *state.Battle, kind command.Type) {
	t.Helper()
	for _, a := range e.LegalActions(b, "player") {
		if a.Type == kind {
			if _, err := e.ApplyBattleCommand(b, a); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no %s action at %s", kind, b.Settled.Stage)
}

func remainingDamage(b *state.Battle) int {
	total := 0
	for _, s := range b.Settled.PendingDamage.Sources {
		total += settledSourceAmount(s)
	}
	return total
}

func TestDefenseBeforeRollWindowClosesAfterFirstDefenseRoll(t *testing.T) {
	b, lib, e := unifiedFixture(t, 4, 3)
	early := guardCard(t, &lib, "early_guard", "defense_before_roll")
	guardCard(t, &lib, "any_guard", "defense_selection")
	if !strings.Contains(early.Presentation.RulesText, "Play only before you roll any defense this round.") {
		t.Fatalf("rules text omits timing: %q", early.Presentation.RulesText)
	}
	giveCardCopy(&b, "early-1", "early_guard")
	giveCardCopy(&b, "early-2", "early_guard")
	giveCardCopy(&b, "any-1", "any_guard")
	b.SettledCatalog, _ = json.Marshal(lib)

	// Before any defense roll both timings are offered.
	starts := startActions(e, &b)
	for _, id := range []string{"early-1", "early-2", "any-1"} {
		if _, ok := starts[id]; !ok {
			t.Fatalf("%s not playable before the first defense roll", id)
		}
	}
	staleEarly := starts["early-2"]

	// Play one early copy against a chosen attack through the real commands.
	before := remainingDamage(&b)
	if _, err := e.ApplyBattleCommand(&b, starts["early-1"]); err != nil {
		t.Fatal(err)
	}
	target := command.Command{}
	for _, a := range e.LegalActions(&b, "player") {
		_, key := programPayload(a)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "target" && c.Source == "a" {
			target = a
		}
	}
	if target.Type == "" {
		t.Fatal("attack a not offered as a target")
	}
	if _, err := e.ApplyBattleCommand(&b, target); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Actors["player"].CardExecution != nil || remainingDamage(&b) != before-1 || settledSourceAmount(*batchSourceByID(b.Settled.PendingDamage, "a")) != 3 {
		t.Fatalf("early guard prevented %d damage, want 1", before-remainingDamage(&b))
	}

	// Roll one defense and apply it; the hub reopens for the second attack.
	applyFirst(t, e, &b, command.TypePlanningAbility)
	if b.Settled.Stage == stageDefenseRoll {
		if _, err := e.ApplyBattleCommand(&b, e.LegalActions(&b, "player")[0]); err != nil {
			t.Fatal(err)
		}
	}
	if b.Settled.Stage != stageDefenseReact {
		t.Fatalf("missing roll review, stage %s", b.Settled.Stage)
	}
	if _, ok := startActions(e, &b)["early-2"]; ok {
		t.Fatal("before-roll card offered during the roll review")
	}
	applyFirst(t, e, &b, command.TypePlanningPass)
	if b.Settled.Stage != stageDefenseSelect || b.Settled.DefensePassed["player"] {
		t.Fatalf("defense did not return to the hub: %s", b.Settled.Stage)
	}

	starts = startActions(e, &b)
	if _, ok := starts["early-2"]; ok {
		t.Fatal("before-roll card still offered after a defense roll")
	}
	if _, ok := starts["any-1"]; !ok {
		t.Fatal("any-time defense card unavailable between defenses")
	}
	if _, err := e.ApplyBattleCommand(&b, staleEarly); err == nil {
		t.Fatal("stale before-roll card command accepted after a defense roll")
	}
}

func TestDefenseBeforeRollWindowRules(t *testing.T) {
	b, lib, _ := unifiedFixture(t, 4)
	windows := []string{"defense_before_roll"}
	if b.Settled.Stage != stageDefenseSelect || !programWindowOpen(&b, "player", windows) {
		t.Fatal("window closed on a fresh Defense screen")
	}
	b.Settled.DefenseHistory = map[string]state.SettledDefense{"skipped": {ActorID: "player", SourceID: "skipped", Finalized: true}}
	if !programWindowOpen(&b, "player", windows) {
		t.Fatal("skipping an attack without rolling closed the window")
	}
	b.Settled.DefenseHistory["other"] = state.SettledDefense{ActorID: "enemy", AbilityID: "guard", SourceID: "other", Finalized: true}
	if !programWindowOpen(&b, "player", windows) {
		t.Fatal("another participant's defense roll closed the window")
	}
	b.Settled.DefenseHistory["rolled"] = state.SettledDefense{ActorID: "player", AbilityID: "adventurer_guard", SourceID: "rolled", Finalized: true}
	if programWindowOpen(&b, "player", windows) {
		t.Fatal("window open after the player's defense roll")
	}
	if !programWindowOpen(&b, "player", []string{"defense_selection"}) {
		t.Fatal("any-time defense window closed after a roll")
	}

	// Offensive-only effects cannot use a defense window.
	c := guardCard(t, &lib, "both_guard", "defense_before_roll", "defense_selection")
	if strings.Contains(c.Presentation.RulesText, "before you roll") {
		t.Fatal("any-time card described as before-roll only")
	}
	c.Program.Steps = []content.CardStep{selfStep("set_die", map[string]any{"faces": []int{6}})}
	c.Program.Windows = []string{"defense_before_roll"}
	if err := content.ValidateCardProgram(c, lib); err == nil {
		t.Fatal("offensive die effect accepted in the before-roll defense window")
	}
}
