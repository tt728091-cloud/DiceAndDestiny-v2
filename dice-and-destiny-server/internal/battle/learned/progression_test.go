package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
)

func TestProgressionPurchasesPersistenceAndIsolation(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for id, lib := range catalogs {
		p, err := loadout.ReadProgress(dir, id, economy, lib)
		if err != nil {
			t.Fatal(err)
		}
		if p.XP != economy.StartingXP || p.Revision != 1 {
			t.Fatal("missing starter allowance")
		}
		if id == "adventurer" && (!reflect.DeepEqual(p.Abilities.Defensive, []string{"adventurer_guard"}) || countProgress(p.Deck, "brace") != 3 || countProgress(p.Deck, "brace_plus") != 0) {
			t.Fatalf("wrong progression starter: %+v", p)
		}
		if _, err = loadout.Write(dir, id, []loadout.Entry{{CardID: "tip_it", Count: 20}}, lib.Cards); err != nil {
			t.Fatal(err)
		}
		same, _ := loadout.ReadProgress(dir, id, economy, lib)
		if !reflect.DeepEqual(p, same) {
			t.Fatal("sandbox overwrote progression")
		}
		p, err = loadout.Buy(dir, id, economy, lib, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: economy.Price(id, "tip_it")})
		if err != nil {
			t.Fatal(err)
		}
		if p.XP != economy.StartingXP-economy.Price(id, "tip_it") {
			t.Fatal("XP not deducted")
		}
		reopened, err := loadout.ReadProgress(dir, id, economy, lib)
		if err != nil || !reflect.DeepEqual(p, reopened) {
			t.Fatal("purchase not persisted")
		}
		changed := economy
		changed.StartingXP = 1000000
		reopened, _ = loadout.ReadProgress(dir, id, changed, lib)
		if reopened.XP != p.XP {
			t.Fatal("reload granted XP again")
		}
	}
	lib := catalogs["adventurer"]
	p, _ := loadout.ReadProgress(dir, "adventurer", economy, lib)
	beforeHealth := totalProgress(p.Deck)
	p, err = loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "upgrade_card", ID: "brace", Revision: p.Revision, ExpectedCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	if countProgress(p.Deck, "brace") != 2 || countProgress(p.Deck, "brace_plus") != 1 || totalProgress(p.Deck) != beforeHealth {
		t.Fatal("upgrade did not replace exactly one card")
	}
	p, err = loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Abilities.Defensive, []string{"adventurer_guard_plus"}) {
		t.Fatal("ability slot not replaced")
	}
	filename := filepath.Join(dir, "progression", "adventurer.json")
	before, _ := os.ReadFile(filename)
	for _, request := range []loadout.Purchase{
		{Kind: "buy_card", ID: "tip_it", Revision: p.Revision - 1, ExpectedCost: 10},
		{Kind: "buy_card", ID: "missing", Revision: p.Revision, ExpectedCost: 10},
		{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: 1},
		{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25},
		{Kind: "upgrade_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: 10},
		{Kind: "grant_xp", ID: "tip_it", Revision: p.Revision, ExpectedCost: 10},
	} {
		if _, err = loadout.Buy(dir, "adventurer", economy, lib, request); err == nil {
			t.Fatalf("invalid purchase accepted: %+v", request)
		}
		after, _ := os.ReadFile(filename)
		if string(before) != string(after) {
			t.Fatal("failed purchase wrote partial state")
		}
	}
	expensive := economy
	expensive.DefaultCardPrice = p.XP + 1
	if _, err = loadout.Buy(dir, "adventurer", expensive, lib, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: p.XP + 1}); err == nil {
		t.Fatal("overspend accepted")
	}
	// Two callers with the same quote may spend only once.
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: 10})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("duplicate purchase applied %d times", success)
	}
}

func TestProgressionBattlePinsUpgradedAbilitiesAndReplay(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(root)
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	lib := catalogs["adventurer"]
	p, err := loadout.ReadProgress(dir, "adventurer", economy, lib)
	if err != nil {
		t.Fatal(err)
	}
	p, err = loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25})
	if err != nil {
		t.Fatal(err)
	}
	p, err = loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "upgrade_card", ID: "brace", Revision: p.Revision, ExpectedCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(SessionConfig{ContentRoot: root, RunStateRoot: t.TempDir(), LoadoutRoot: dir, OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{"seat-a", "seat-b"} {
		if _, err = s.ResetCharacterLoadout("progress-"+seat, 931, seat, false, "adventurer", true, "progression"); err != nil {
			t.Fatal(err)
		}
		actor := s.current.Result.Snapshot.Actors[seat]
		if actor.MaxHealth != 12 || !reflect.DeepEqual(actor.DefensiveAbilities, []string{"adventurer_guard_plus"}) {
			t.Fatalf("wrong battle loadout: %+v", actor)
		}
		if s.current.Result.Snapshot.Actors[otherSeat(seat)].MaxHealth != 16 {
			t.Fatal("enemy changed")
		}
		finishOwnedBattle(t, s)
		record := *s.current.Replay
		if !reflect.DeepEqual(record.SeatAbilityBoards[seat].Defensive, []string{"adventurer_guard_plus"}) {
			t.Fatal("ability board missing from replay")
		}
		env, err := mlsim.New(mlsim.Config{ContentRoot: root, RunStateRoot: t.TempDir(), IncludeContentCatalog: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = env.Replay(record); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProgressionRuntimeAndConfig(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	request, _ := json.Marshal(map[string]any{"op": "progression_catalogs", "content_root": root, "loadout_root": dir})
	var reply map[string]any
	if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(request))), &reply); err != nil || reply["ok"] != true {
		t.Fatalf("runtime: %v %v", reply, err)
	}
	if len(reply["result"].(map[string]any)) != 4 {
		t.Fatal("missing progression characters")
	}
	// Price and upgrade adjustments are consumed directly from configuration.
	catalogs, _ := CharacterCatalogs(root)
	configRoot := t.TempDir()
	os.MkdirAll(filepath.Join(configRoot, "progression_v1"), 0700)
	filename := filepath.Join(configRoot, "progression_v1", "economy.yaml")
	os.WriteFile(filename, []byte("schema_version: 1\nstarting_xp: 77\ndefault_card_price: 4\ncharacters:\n  adventurer:\n    card_upgrades:\n      brace: {to: brace_plus, xp: 2}\n"), 0600)
	e, err := loadout.LoadEconomy(configRoot, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	fresh := t.TempDir()
	p, err := loadout.ReadProgress(fresh, "adventurer", e, catalogs["adventurer"])
	if err != nil {
		t.Fatal(err)
	}
	p, err = loadout.Buy(fresh, "adventurer", e, catalogs["adventurer"], loadout.Purchase{Kind: "upgrade_card", ID: "brace", Revision: p.Revision, ExpectedCost: 2})
	if err != nil || p.XP != 75 {
		t.Fatalf("configured price ignored: %+v %v", p, err)
	}
	for _, bad := range []string{
		"schema_version: 1\nstarting_xp: -1\ndefault_card_price: 10\n",
		"schema_version: 1\nstarting_xp: 100\ndefault_card_price: 10\ncharacters:\n  adventurer:\n    ability_upgrades:\n      adventurer_guard: {to: adventurer_strike, xp: 2}\n",
	} {
		os.WriteFile(filename, []byte(bad), 0600)
		if _, err = loadout.LoadEconomy(configRoot, catalogs); err == nil {
			t.Fatal("invalid economy accepted")
		}
	}
}
func countProgress(deck []loadout.Entry, id string) int {
	for _, e := range deck {
		if e.CardID == id {
			return e.Count
		}
	}
	return 0
}
func totalProgress(deck []loadout.Entry) int {
	n := 0
	for _, e := range deck {
		n += e.Count
	}
	return n
}

func TestProgressionSellEntireStarterDeckAndRebuild(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(root)
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	for character, lib := range catalogs {
		t.Run(character, func(t *testing.T) {
			dir := t.TempDir()
			p, err := loadout.ReadProgress(dir, character, economy, lib)
			if err != nil {
				t.Fatal(err)
			}
			budget := p.XP
			starter := append([]loadout.Entry(nil), p.Deck...)
			for _, entry := range starter {
				budget += entry.Count * economy.Price(character, entry.CardID)
			}
			for _, entry := range starter {
				for copy := 0; copy < entry.Count; copy++ {
					p, err = loadout.Buy(dir, character, economy, lib, loadout.Purchase{Kind: "sell_card", ID: entry.CardID, Revision: p.Revision, ExpectedCost: economy.Price(character, entry.CardID)})
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(p.Deck) != 0 || p.XP != budget {
				t.Fatalf("liquidation mismatch: %+v budget %d", p, budget)
			}
			if character == "adventurer" && budget != 220 {
				t.Fatal("Adventurer's 120 XP starter plus 100 XP allowance must equal 220")
			}
			reopened, err := loadout.ReadProgress(dir, character, economy, lib)
			if err != nil || !reflect.DeepEqual(p, reopened) {
				t.Fatal("empty deck did not persist")
			}
			filename := filepath.Join(dir, "progression", character+".json")
			before, _ := os.ReadFile(filename)
			if _, err = loadout.Buy(dir, character, economy, lib, loadout.Purchase{Kind: "sell_card", ID: starter[0].CardID, Revision: p.Revision, ExpectedCost: economy.Price(character, starter[0].CardID)}); err == nil {
				t.Fatal("sold an unowned card")
			}
			after, _ := os.ReadFile(filename)
			if string(before) != string(after) {
				t.Fatal("rejected sale changed balance")
			}
			s, err := NewSession(SessionConfig{ContentRoot: root, RunStateRoot: t.TempDir(), LoadoutRoot: dir, OpponentDefinition: "drowned_oracle_brine_mask"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ResetCharacter("existing", 191, "seat-a", false, character, true); err != nil {
				t.Fatal(err)
			}
			previous := s.current.Result.Snapshot
			if _, err = s.ResetCharacterLoadout("empty", 192, "seat-b", false, character, true, "progression"); err == nil {
				t.Fatal("empty progression battle accepted")
			}
			if s.current.Result.Snapshot != previous {
				t.Fatal("rejected empty start replaced current battle")
			}
			p, err = loadout.Buy(dir, character, economy, lib, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: economy.Price(character, "tip_it")})
			if err != nil {
				t.Fatal(err)
			}
			if p.XP != budget-economy.Price(character, "tip_it") {
				t.Fatal("rebuy price differs")
			}
			if _, err = s.ResetCharacterLoadout("rebuilt", 193, "seat-a", false, character, true, "progression"); err != nil {
				t.Fatal(err)
			}
			if s.current.Result.Snapshot.Actors["seat-a"].MaxHealth != 1 {
				t.Fatal("rebuilt deck not used")
			}
			finishOwnedBattle(t, s)
		})
	}
}

func TestProgressionSalePriceOverridesAndStaleRequests(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(root)
	economy, _ := loadout.LoadEconomy(root, catalogs)
	dir := t.TempDir()
	lib := catalogs["adventurer"]
	p, _ := loadout.ReadProgress(dir, "adventurer", economy, lib)
	p, err := loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "upgrade_card", ID: "brace", Revision: p.Revision, ExpectedCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	revision := p.Revision
	sale := loadout.Purchase{Kind: "sell_card", ID: "brace_plus", Revision: revision, ExpectedCost: 20}
	bad := sale
	bad.ExpectedCost = 10
	if _, err = loadout.Buy(dir, "adventurer", economy, lib, bad); err == nil {
		t.Fatal("wrong sale quote accepted")
	}
	p, err = loadout.Buy(dir, "adventurer", economy, lib, sale)
	if err != nil {
		t.Fatal(err)
	}
	if p.XP != 110 || countProgress(p.Deck, "brace_plus") != 0 {
		t.Fatal("upgraded card did not sell at its configured purchase price")
	}
	if _, err = loadout.Buy(dir, "adventurer", economy, lib, sale); err == nil {
		t.Fatal("duplicate sale credited twice")
	}
	p, err = loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "buy_card", ID: "brace_plus", Revision: p.Revision, ExpectedCost: 20})
	if err != nil {
		t.Fatal(err)
	}
	if p.XP != 90 || countProgress(p.Deck, "brace_plus") != 1 {
		t.Fatal("sale and rebuy must cancel out")
	}
	// Concurrent sales of one owned copy can only credit once.
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := loadout.Buy(dir, "adventurer", economy, lib, loadout.Purchase{Kind: "sell_card", ID: "brace_plus", Revision: p.Revision, ExpectedCost: 20})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatal("concurrent sale was duplicated")
	}
}
