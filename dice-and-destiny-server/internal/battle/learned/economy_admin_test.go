package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
)

func TestAdminEconomyRevaluesAllCharactersAndTrades(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join(testServerRoot(t), "content"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := loadout.LoadEconomy(filepath.Join(testServerRoot(t), "content"), catalogs)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for id, lib := range catalogs {
		p, err := loadout.ReadProgress(root, id, e, lib)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loadout.Buy(root, id, e, lib, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: 10}); err != nil {
			t.Fatal(err)
		}
	}
	before, _, _, err := loadout.ProgressSnapshot(root, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	settings := loadout.AdminSettings{CardPrices: map[string]int{"steady_guard": 12, "tip_it": 8}, Budgets: map[string]int{"adventurer": 260}}
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	after, effective, admin, err := loadout.ProgressSnapshot(root, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	for id, p := range after {
		old := before[id]
		budget := *old.Budget
		if id == "adventurer" {
			budget = 260
		}
		delta := 2*countProgress(p.Deck, "steady_guard") - 2*countProgress(p.Deck, "tip_it")
		if *p.Budget != budget || p.XP != old.XP+(budget-*old.Budget)-delta || p.DeckValue != old.DeckValue+delta || p.Revision <= old.Revision {
			t.Fatalf("bad revaluation %s: %+v", id, p)
		}
		if effective.Price(id, "tip_it") != 8 {
			t.Fatal("global price missing")
		}
		if _, err := loadout.Buy(root, id, e, catalogs[id], loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: old.Revision, ExpectedCost: 10}); err == nil {
			t.Fatal("stale trade accepted")
		}
		sold, err := loadout.Buy(root, id, e, catalogs[id], loadout.Purchase{Kind: "sell_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: 8})
		if err != nil {
			t.Fatal(err)
		}
		if sold.XP != p.XP+8 || *sold.Budget != budget {
			t.Fatal("wrong refund")
		}
	}
	filename := filepath.Join(root, "economy_admin.json")
	bytesBefore, _ := os.ReadFile(filename)
	// Impossible and stale changes cannot partially update prices or balances.
	settings.Revision = admin.Revision
	settings.Budgets["adventurer"] = 1
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err == nil {
		t.Fatal("over-budget settings accepted")
	}
	bytesAfter, _ := os.ReadFile(filename)
	if string(bytesBefore) != string(bytesAfter) {
		t.Fatal("failed update changed settings")
	}
	settings.Budgets["adventurer"] = 260
	settings.Revision = 0
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err == nil {
		t.Fatal("stale admin update accepted")
	}
	settings.Revision = admin.Revision
	settings.CardPrices["unknown"] = 8
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err == nil {
		t.Fatal("unknown card accepted")
	}
	// Re-reading cannot repeatedly credit the budget adjustment.
	p, _ := loadout.ReadProgress(root, "adventurer", e, catalogs["adventurer"])
	again, _ := loadout.ReadProgress(root, "adventurer", e, catalogs["adventurer"])
	if p.XP != again.XP || p.Revision != again.Revision {
		t.Fatal("repeated reconciliation changed balance")
	}
}

func TestAdminEconomyPreservesUpgradeInvestmentAndMigrates(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(contentRoot)
	e, _ := loadout.LoadEconomy(contentRoot, catalogs)
	root := t.TempDir()
	lib := catalogs["adventurer"]
	p, _ := loadout.ReadProgress(root, "adventurer", e, lib)
	p, err := loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25})
	if err != nil {
		t.Fatal(err)
	}
	if *p.Budget != 220 || p.UpgradeSpent != 25 {
		t.Fatal("upgrade spending not recorded")
	}
	// Simulate a pre-ledger save: retain existing XP, deck and upgraded board.
	data, _ := json.Marshal(p)
	var legacy map[string]any
	json.Unmarshal(data, &legacy)
	for _, key := range []string{"total_budget", "deck_value", "upgrade_spent", "admin_revision"} {
		delete(legacy, key)
	}
	data, _ = json.Marshal(legacy)
	os.WriteFile(filepath.Join(root, "progression", "adventurer.json"), data, 0600)
	p, err = loadout.ReadProgress(root, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	if *p.Budget != 220 || p.XP != 75 || p.UpgradeSpent != 25 {
		t.Fatalf("migration changed investment: %+v", p)
	}
	settings := loadout.AdminSettings{CardPrices: map[string]int{"steady_guard": 12}, Budgets: map[string]int{"adventurer": 260}}
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, err = loadout.ReadProgress(root, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	if p.XP != 109 || p.DeckValue != 126 || p.UpgradeSpent != 25 || *p.Budget != 260 {
		t.Fatalf("upgrade revaluation failed: %+v", p)
	}
	// Budget reductions are also supported when all equipped investments fit.
	settings.Revision = 1
	settings.Budgets["adventurer"] = 151
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, _ = loadout.ReadProgress(root, "adventurer", e, lib)
	if p.XP != 0 {
		t.Fatal("budget reduction not applied")
	}
}

func TestAdminRepricingKeepsUpgradeAndNewCharacterBudgetConsistent(t *testing.T) {
	contentRoot := legacyUpgradeContent(t)
	catalogs, _ := CharacterCatalogs(contentRoot)
	e, _ := loadout.LoadEconomy(contentRoot, catalogs)
	root := t.TempDir()
	lib := catalogs["adventurer"]
	settings := loadout.AdminSettings{CardPrices: map[string]int{"steady_guard": 8}, Budgets: map[string]int{"adventurer": 260}}
	if err := loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, effective, _, err := loadout.ProgressSnapshot(root, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if effective.CardUpgrades("adventurer")["steady_guard"].XP != 12 {
		t.Fatal("upgrade underfunds increased deck value")
	}
	upgraded, err := loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_card", ID: "steady_guard", Revision: p["adventurer"].Revision, ExpectedCost: 12})
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.XP != 134 || upgraded.DeckValue != 126 || upgraded.UpgradeSpent != 0 {
		t.Fatalf("wrong upgraded ledger: %+v", upgraded)
	}
	// Simulate a newly created character while preserving the configured admin budget.
	if err := os.Remove(filepath.Join(root, "progression", "adventurer.json")); err != nil {
		t.Fatal(err)
	}
	fresh, err := loadout.ReadProgress(root, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	if *fresh.Budget != 260 || fresh.XP != 146 || fresh.DeckValue != 114 {
		t.Fatalf("new character ignored budget: %+v", fresh)
	}
}
