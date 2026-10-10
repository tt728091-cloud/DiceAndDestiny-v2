package learned

import (
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestEntireCardCatalogEditableAndPublishable(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	revision := 0
	blank := map[string]bool{}
	for id, original := range lib.Cards {
		// Blank minion health cards have no effects; the editor never publishes those.
		if content.IsBlankCard(original) {
			blank[id] = true
			continue
		}
		c, err := content.EditableCard(original)
		if err != nil {
			t.Fatalf("%s is not editable: %v", id, err)
		}
		c.ID = "catalog_copy_" + id
		c.Name = "Catalog Copy " + c.Name
		c.Cost.Energy = 2
		if _, err = content.SaveAuthoredCard(dir, lib, c, revision); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		revision++
	}
	loaded, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	for id := range lib.Cards {
		if blank[id] {
			continue
		}
		c, ok := loaded["adventurer"].Cards["catalog_copy_"+id]
		if !ok || c.Cost.Energy != 2 || c.Program == nil && c.Mechanic == nil {
			t.Fatalf("%s did not round-trip", id)
		}
	}
}

func TestRenamedSpecializedDecksCompleteBattles(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	for _, family := range []string{"venom", "curse"} {
		t.Run(family, func(t *testing.T) {
			dir := t.TempDir()
			catalogs, err := CharacterCatalogs(root)
			if err != nil {
				t.Fatal(err)
			}
			lib := catalogs[family]
			revision := 0
			deck := []loadout.Entry{}
			for _, entry := range lib.Combatants[family].Decklist {
				c, err := content.EditableCard(lib.Cards[entry.CardID])
				if err != nil {
					t.Fatal(err)
				}
				c.ID = "battle_copy_" + c.ID
				c.Name = "Battle Copy " + c.Name
				c.Cost.Energy = 0
				if _, err = content.SaveAuthoredCard(dir, lib, c, revision); err != nil {
					t.Fatal(err)
				}
				revision++
				deck = append(deck, loadout.Entry{CardID: c.ID, Count: entry.Count})
			}
			catalogs, err = CharacterCatalogs(root, dir)
			if err != nil {
				t.Fatal(err)
			}
			economy, err := loadout.LoadEconomy(root, catalogs)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = loadout.WriteSharedDeck(dir, family, deck, economy, catalogs[family]); err != nil {
				t.Fatal(err)
			}
			s, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: 2})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ResetCharacter("specialized-"+family, 47, "seat-a", false, family, true); err != nil {
				t.Fatal(err)
			}
			played := 0
			for step := 0; step < 1800 && !s.current.Terminal; step++ {
				if s.isModelSeat(s.current.ActorID) {
					_, err = s.AdvanceModel()
				} else {
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatal("no legal actions")
					}
					action := adventurerTestAction(actions, 3)
					for _, candidate := range actions {
						var payload map[string]any
						_ = json.Unmarshal(candidate.Payload, &payload)
						cards, _ := payload["card_ids"].([]any)
						if commitment, ok := payload["commitment"].(map[string]any); ok {
							cards, _ = commitment["card_ids"].([]any)
						}
						if len(cards) == 1 {
							action = candidate
							played++
							break
						}
					}
					encoded, _ := json.Marshal(aliasValue(commandMap(t, action), s.aliases(false)))
					_, err = s.SubmitHuman(string(encoded))
				}
				if err != nil {
					t.Fatalf("step %d stage %s: %v", step, s.current.Result.Snapshot.Stage, err)
				}
			}
			if !s.current.Terminal || s.current.TruncationReason != "" {
				t.Fatalf("battle incomplete: %s", s.current.TruncationReason)
			}
			if played == 0 {
				t.Fatal("no custom cards played")
			}
			telemetry, _ := s.Telemetry()
			if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 {
				t.Fatal(fmt.Sprint(telemetry))
			}
		})
	}
}
