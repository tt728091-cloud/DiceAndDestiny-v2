package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/loadout"
)

func TestSharedDeckEditsTradesAndBudgets(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(contentRoot)
	e, _ := loadout.LoadEconomy(contentRoot, catalogs)
	dir := t.TempDir()
	lib := catalogs["adventurer"]
	p, err := loadout.ReadProgress(dir, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	// Both positive and negative free allocations compose with admin budgets.
	for _, copies := range []int{20, 1, 4} {
		old := p
		deck := []loadout.Entry{{CardID: "steady_guard", Count: copies}}
		if _, err = loadout.WriteSharedDeck(dir, "adventurer", deck, e, lib); err != nil {
			t.Fatal(err)
		}
		p, err = loadout.ReadProgress(dir, "adventurer", e, lib)
		if err != nil || p.XP != old.XP || !reflect.DeepEqual(p.Deck, deck) {
			t.Fatalf("free edit: %+v %v", p, err)
		}
		if _, err = loadout.Buy(dir, "adventurer", e, lib, loadout.Purchase{Kind: "buy_card", ID: "steady_guard", Revision: old.Revision, ExpectedCost: 10}); err == nil {
			t.Fatal("pre-edit quote accepted")
		}
		_, _, admin, err := loadout.ProgressSnapshot(dir, e, catalogs)
		if err != nil {
			t.Fatal(err)
		}
		admin.Budgets = map[string]int{"adventurer": copies*10 + 50}
		if err = loadout.SaveAdmin(dir, e, catalogs, admin); err != nil {
			t.Fatal(err)
		}
		p, err = loadout.ReadProgress(dir, "adventurer", e, lib)
		if err != nil || p.XP != 50 {
			t.Fatalf("admin total applied twice: %+v %v", p, err)
		}
		p, err = loadout.Buy(dir, "adventurer", e, lib, loadout.Purchase{Kind: "sell_card", ID: "steady_guard", Revision: p.Revision, ExpectedCost: 10})
		if err != nil || p.XP != 60 {
			t.Fatalf("sale: %+v %v", p, err)
		}
		again, err := loadout.ReadProgress(dir, "adventurer", e, lib)
		if err != nil || !reflect.DeepEqual(p, again) {
			t.Fatalf("reload changed ledger: %+v %v", again, err)
		}
	}
	// Real catalogs and both battle modes must use precisely the same card instances.
	for _, op := range []string{"character_catalogs", "progression_catalogs"} {
		req, _ := json.Marshal(map[string]any{"op": op, "content_root": contentRoot, "loadout_root": dir})
		var response struct {
			OK     bool `json:"ok"`
			Result map[string]struct {
				Deck []loadout.Entry `json:"owned_decklist"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(req))), &response); err != nil || !response.OK || !reflect.DeepEqual(response.Result["adventurer"].Deck, p.Deck) {
			t.Fatalf("%s diverged: %+v %v", op, response, err)
		}
	}
	s, err := NewSession(SessionConfig{ContentRoot: contentRoot, RunStateRoot: t.TempDir(), LoadoutRoot: dir, OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"sandbox", "progression"} {
		if _, err = s.ResetCharacterLoadout("shared-"+mode, 91, "seat-a", false, "adventurer", true, mode); err != nil {
			t.Fatal(err)
		}
		if s.current.Result.Snapshot.Actors["seat-a"].MaxHealth != 3 {
			t.Fatal("battle used another deck")
		}
	}
}

func TestSharedDeckMigration(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(contentRoot)
	e, _ := loadout.LoadEconomy(contentRoot, catalogs)
	lib := catalogs["adventurer"]
	for _, scenario := range []string{"custom-progression", "newer-sandbox", "newer-progression"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			p, err := loadout.ReadProgress(dir, "adventurer", e, lib)
			if err != nil {
				t.Fatal(err)
			}
			p.SharedDeck = false
			p.Deck = []loadout.Entry{{CardID: "steady_guard", Count: 3}, {CardID: "nudge", Count: 2}, {CardID: "try_again", Count: 2}, {CardID: "strong_swing", Count: 2}, {CardID: "take_stock", Count: 2}, {CardID: "second_wind", Count: 1}}
			p.DeckValue = 120
			budget := 220
			p.Budget = &budget
			if scenario == "custom-progression" {
				p.Deck[0].Count = 4
				p.DeckValue = 130
				p.XP = 90
			}
			filename := filepath.Join(dir, "progression", "adventurer.json")
			data, _ := json.Marshal(p)
			if err = os.WriteFile(filename, data, 0600); err != nil {
				t.Fatal(err)
			}
			// Cover real saved admin override 220, without losing the user's 100 XP.
			if err = os.WriteFile(filepath.Join(dir, "economy_admin.json"), []byte(`{"revision":1,"budgets":{"adventurer":220}}`), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "newer-sandbox" || scenario == "newer-progression" {
				if _, err = loadout.Write(dir, "adventurer", []loadout.Entry{{CardID: "emergency_ward", Count: 2}}, lib.Cards); err != nil {
					t.Fatal(err)
				}
				a, b := time.Unix(100, 0), time.Unix(200, 0)
				if scenario == "newer-progression" {
					a, b = b, a
				}
				os.Chtimes(filename, a, a)
				os.Chtimes(filepath.Join(dir, "adventurer.json"), b, b)
			}
			migrated, err := loadout.ReadProgress(dir, "adventurer", e, lib)
			if err != nil || !migrated.SharedDeck || migrated.XP != p.XP {
				t.Fatalf("migration: %+v %v", migrated, err)
			}
			switch scenario {
			case "newer-sandbox":
				if len(migrated.Deck) != 1 || countProgress(migrated.Deck, "emergency_ward") != 2 {
					t.Fatal("newer sandbox lost")
				}
			default:
				if !reflect.DeepEqual(migrated.Deck, p.Deck) {
					t.Fatal("saved progression changed")
				}
			}
			// Old files are archival after migration, never a second source of truth.
			loadout.Write(dir, "adventurer", []loadout.Entry{{CardID: "steady_guard", Count: 1}}, lib.Cards)
			again, err := loadout.ReadProgress(dir, "adventurer", e, lib)
			if err != nil || !reflect.DeepEqual(again, migrated) {
				t.Fatalf("migration repeated: %+v %v", again, err)
			}
		})
	}
}
