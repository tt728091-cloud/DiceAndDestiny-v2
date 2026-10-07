package learned

import (
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
)

func TestGeneralCardsAvailableToEveryCharacterAndPinToBattle(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"matchmaker", "turn_the_die", "disrupt", "second_guard", "reclaim", "reinforce", "dispel", "triage"}
	for character, catalog := range catalogs {
		t.Run(character, func(t *testing.T) {
			dir := t.TempDir()
			p, err := loadout.ReadProgress(dir, character, economy, catalog)
			if err != nil {
				t.Fatal(err)
			}
			startingXP, startingValue := p.XP, p.DeckValue
			for _, id := range ids {
				def, ok := catalog.Cards[id]
				if !ok || def.Program == nil || def.Presentation.RulesText == "" {
					t.Fatalf("missing usable definition %s", id)
				}
				p, err = loadout.Buy(dir, character, economy, catalog, loadout.Purchase{Kind: "buy_card", ID: id, ExpectedCost: 10, Revision: p.Revision})
				if err != nil {
					t.Fatalf("buy %s: %v", id, err)
				}
			}
			if p.XP != startingXP-80 || p.DeckValue != startingValue+80 {
				t.Fatal("purchase accounting mismatch")
			}
			s, err := NewSession(SessionConfig{ContentRoot: root, RunStateRoot: t.TempDir(), LoadoutRoot: dir, OpponentDefinition: "drowned_oracle_brine_mask"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ResetCharacterLoadout("general-"+character, 71, "seat-a", false, character, true, "progression"); err != nil {
				t.Fatal(err)
			}
			actor := s.current.Result.Snapshot.Actors["seat-a"]
			for _, id := range ids {
				found := false
				for _, card := range actor.CardInstances {
					if card.DefinitionID == id {
						found = true
					}
				}
				if !found {
					t.Fatalf("battle missing purchased %s", id)
				}
			}
			if s.current.Result.Snapshot.Actors["seat-b"].MaxHealth != 16 {
				t.Fatal("changed opponent starter")
			}
			for _, id := range ids {
				p, err = loadout.Buy(dir, character, economy, catalog, loadout.Purchase{Kind: "sell_card", ID: id, ExpectedCost: 10, Revision: p.Revision})
				if err != nil {
					t.Fatal(err)
				}
			}
			if p.XP != startingXP || p.DeckValue != startingValue {
				t.Fatal("sell-back accounting mismatch")
			}
		})
	}
}
