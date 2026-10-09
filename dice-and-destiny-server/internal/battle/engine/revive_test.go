package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// reviveStep returns removed cards to hand; the card's cost removes others.
func reviveStep(mode string, count int) content.CardStep {
	s := selfStep("move_cards", map[string]any{"destination": "hand"})
	s.Target.Mode, s.Target.Count, s.Target.Zones, s.Target.ExcludeRecovery = mode, count, []string{"removed"}, true
	return s
}

// reviveFixture holds one revive card (removed when played) and an earlier
// damage wound that removed nudge-0, nudge-1 and strong_swing-0.
func reviveFixture(t *testing.T, steps ...content.CardStep) (state.Battle, content.BattleLibrary) {
	t.Helper()
	b, lib := generalFixture(t)
	b.Segment.Current = segment.Offensive
	b.Settled.Initialized = true
	c := content.BattleCardDefinition{SchemaVersion: 1, ID: "revive_test", Name: "Revive Test", Cost: content.BattleCost{Energy: 2}, Play: content.CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "removed"}, Targeting: content.TargetingDefinition{Selector: "card_program", Minimum: 1, Maximum: 1}, Program: &content.CardProgram{Version: 1, Windows: []string{"offensive_planning"}, RollRequirement: "any", Steps: steps}}
	c = content.PrepareProgramCard(c, &lib)
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[c.ID] = c
	giveGeneralCard(&b, c.ID)
	a := b.Actors["player"]
	lost := []string{"nudge-0", "nudge-1", "strong_swing-0"}
	var cards []state.WoundCard
	for _, id := range lost {
		moveCard(&a.Cards, id, operation.ZoneHand, operation.ZoneRemoved)
		cards = append(cards, state.WoundCard{CardID: id, CardDefinitionID: b.Settled.Actors["player"].CardInstances[id].DefinitionID, OriginalZone: operation.ZoneHand})
	}
	b.Actors["player"] = a
	b.Wounds = []state.Wound{{ID: "hit", BatchID: "hit", Round: 1, Segment: segment.DamageResolution, SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", Cards: cards}}
	b.SettledCatalog, _ = json.Marshal(lib)
	return b, lib
}

func woundCards(b *state.Battle, id string) []string {
	for _, w := range b.Wounds {
		if w.ID == id {
			var cards []string
			for _, c := range w.Cards {
				cards = append(cards, c.CardID)
			}
			return cards
		}
	}
	return nil
}

func TestReviveTradesThisCardForARemovedCardAndHealsItsWound(t *testing.T) {
	b, lib := reviveFixture(t, reviveStep("one", 1))
	health := b.Actors["player"].CurrentHealth()
	if err := playProgramCard(NewEngine(), &b, lib, "player", "revive_test", func(c programChoice) bool { return c.Card == "nudge-0" }); err != nil {
		t.Fatal(err)
	}
	a := b.Actors["player"]
	if a.CurrentHealth() != health || !containsString(a.Cards.Hand, "nudge-0") || !containsString(a.Cards.Removed, "revive_test") {
		t.Fatalf("revive must trade one card for another: health %d->%d %+v", health, a.CurrentHealth(), a.Cards)
	}
	if got := woundCards(&b, "hit"); !reflect.DeepEqual(got, []string{"nudge-1", "strong_swing-0"}) {
		t.Fatalf("old wound must shrink by the revived card: %v", got)
	}
	var trade *state.Wound
	for i := range b.Wounds {
		if b.Wounds[i].SourceContentID == "revive_test" {
			trade = &b.Wounds[i]
		}
	}
	if trade == nil || len(trade.Cards) != 1 || trade.Cards[0].CardID != "revive_test" {
		t.Fatalf("the trade must record a new wound for this card: %+v", b.Wounds)
	}
	// The healed ledger persists like any other battle state.
	raw, _ := json.Marshal(b)
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(woundCards(&restored, "hit"), []string{"nudge-1", "strong_swing-0"}) {
		t.Fatalf("healed wound not persisted: %v", err)
	}
}

func TestReviveFullyHealedWoundDisappears(t *testing.T) {
	b, lib := reviveFixture(t, reviveStep("one", 1))
	b.Wounds[0].Cards = b.Wounds[0].Cards[:1]
	if err := playProgramCard(NewEngine(), &b, lib, "player", "revive_test", func(c programChoice) bool { return c.Card == "nudge-0" }); err != nil {
		t.Fatal(err)
	}
	if woundCards(&b, "hit") != nil {
		t.Fatalf("a wound with no cards left must heal completely: %+v", b.Wounds)
	}
}

func TestReviveTwoNeedsASecondSacrificeAndCannotReviveIt(t *testing.T) {
	sacrifice := selfStep("sacrifice", nil)
	sacrifice.Target.Mode, sacrifice.Target.Count, sacrifice.Target.Zones = "exact", 1, []string{"hand"}
	b, lib := reviveFixture(t, sacrifice, reviveStep("exact", 2))
	health := b.Actors["player"].CurrentHealth()
	offered := map[string]bool{}
	err := playProgramCard(NewEngine(), &b, lib, "player", "revive_test", func(c programChoice) bool {
		if c.Verb == "target" && c.Card != "" {
			offered[c.Card] = true
		}
		return c.Card == "take_stock-0" || c.Card == "nudge-0" || c.Card == "strong_swing-0"
	})
	if err != nil {
		t.Fatal(err)
	}
	a := b.Actors["player"]
	if offered["take_stock-0"] && containsString(a.Cards.Hand, "take_stock-0") {
		t.Fatal("this play revived the card it sacrificed")
	}
	if a.CurrentHealth() != health || !containsString(a.Cards.Hand, "nudge-0") || !containsString(a.Cards.Hand, "strong_swing-0") || !containsString(a.Cards.Removed, "take_stock-0") {
		t.Fatalf("two-card trade must stay health-neutral: health %d->%d %+v", health, a.CurrentHealth(), a.Cards)
	}
	if got := woundCards(&b, "hit"); !reflect.DeepEqual(got, []string{"nudge-1"}) {
		t.Fatalf("both revived cards must heal: %v", got)
	}
}

func TestReviveNeedsARemovedCardAndSkipsRecoveryCards(t *testing.T) {
	b, lib := reviveFixture(t, reviveStep("one", 1))
	a := b.Actors["player"]
	for _, id := range []string{"nudge-0", "nudge-1", "strong_swing-0"} {
		moveCard(&a.Cards, id, operation.ZoneRemoved, operation.ZoneHand)
	}
	b.Actors["player"] = a
	if programOffers(&b, lib, "revive_test") {
		t.Fatal("revive offered with an empty removed pile")
	}
	// A removed recovery card (another revive) is never a legal target.
	giveGeneralCard(&b, "revive_test-2")
	a = b.Actors["player"]
	a.Cards.Hand = a.Cards.Hand[:len(a.Cards.Hand)-1]
	a.Cards.Removed = append(a.Cards.Removed, "revive_test-2")
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances["revive_test-2"] = state.CardInstance{InstanceID: "revive_test-2", DefinitionID: "revive_test"}
	b.Settled.Actors["player"] = rt
	if programOffers(&b, lib, "revive_test") {
		t.Fatal("revive offered with no eligible removed card")
	}
}

func TestReviveMustBeATrade(t *testing.T) {
	_, lib := generalFixture(t)
	base := content.BattleCardDefinition{SchemaVersion: 1, ID: "bad_revive", Name: "Bad Revive", Play: content.CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "removed"}, Targeting: content.TargetingDefinition{Selector: "card_program", Minimum: 1, Maximum: 1}, Program: &content.CardProgram{Version: 1, Windows: []string{"offensive_planning"}, RollRequirement: "any"}}
	mixed := reviveStep("one", 1)
	mixed.Target.Zones = []string{"removed", "discard"}
	all := reviveStep("all", 1)
	toRemoved := reviveStep("one", 1)
	toRemoved.Params["destination"] = "removed"
	for name, c := range map[string]func(content.BattleCardDefinition) content.BattleCardDefinition{
		"free heal": func(c content.BattleCardDefinition) content.BattleCardDefinition {
			c.Play.Destination = "discard"
			c.Program.Steps = []content.CardStep{reviveStep("one", 1)}
			return c
		},
		"two for one": func(c content.BattleCardDefinition) content.BattleCardDefinition {
			c.Program.Steps = []content.CardStep{reviveStep("up_to", 2)}
			return c
		},
		"mixed piles": func(c content.BattleCardDefinition) content.BattleCardDefinition {
			c.Program.Steps = []content.CardStep{mixed}
			return c
		},
		"unbounded": func(c content.BattleCardDefinition) content.BattleCardDefinition {
			c.Program.Steps = []content.CardStep{all}
			return c
		},
		"to removed": func(c content.BattleCardDefinition) content.BattleCardDefinition {
			c.Program.Steps = []content.CardStep{toRemoved}
			return c
		},
	} {
		raw, _ := json.Marshal(base)
		var card content.BattleCardDefinition
		_ = json.Unmarshal(raw, &card)
		if err := content.ValidateCardProgram(c(card), lib); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	ok := base
	ok.Program = &content.CardProgram{Version: 1, Windows: []string{"offensive_planning"}, RollRequirement: "any", Steps: []content.CardStep{reviveStep("one", 1)}}
	ok = content.PrepareProgramCard(ok, &lib)
	if err := content.ValidateCardProgram(ok, lib); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Revive one of your permanently removed cards", "heals 1 health of the wound", "permanently removed instead of discarded"} {
		if !strings.Contains(ok.Presentation.RulesText, want) {
			t.Errorf("rules missing %q: %s", want, ok.Presentation.RulesText)
		}
	}
	if ok.Presentation.EffectSummary != "Revive 1 removed card\nRemoves itself (−1 health)" {
		t.Errorf("face = %q", ok.Presentation.EffectSummary)
	}
}
