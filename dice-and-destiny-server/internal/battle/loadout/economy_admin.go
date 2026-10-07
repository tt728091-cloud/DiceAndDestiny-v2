package loadout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"diceanddestiny/server/internal/content"
)

// One atomic settings file is the commit point. Progress ledgers reconcile against
// it before every read or trade, including recovery after an interrupted refresh.
type AdminSettings struct {
	BudgetAllocations map[string]int    `json:"budget_allocations,omitempty"`
	CardTypes         map[string]string `json:"card_types"`
	AbilityTypes      map[string]string `json:"ability_types"`
	CharacterTypes    map[string]string `json:"character_types"`
	Revision          int               `json:"revision"`
	CardPrices        map[string]int    `json:"card_prices"`
	Budgets           map[string]int    `json:"budgets"`
}

func deckValue(deck []Entry, character string, e Economy) int {
	value := 0
	for _, entry := range deck {
		value += entry.Count * e.Price(character, entry.CardID)
	}
	return value
}
func initializeBudget(p *Progress, e Economy) {
	p.DeckValue = deckValue(p.Deck, p.Character, e)
	p.CollectionValue = deckValue(p.Collection, p.Character, e)
	budget := p.XP + p.DeckValue + p.CollectionValue + p.UpgradeSpent
	p.Budget = &budget
}
func legacyAbilitySpend(id string, paths map[string]Upgrade, seen map[string]bool) int {
	if seen[id] {
		return 0
	}
	seen[id] = true
	for from, u := range paths {
		if u.To == id {
			return u.XP + legacyAbilitySpend(from, paths, seen)
		}
	}
	return 0
}
func reconcileBudget(p *Progress, e Economy) error {
	budget := *p.Budget
	if override, ok := e.Budgets[p.Character]; ok {
		budget = override + p.FreeDeckBudget - e.BudgetAllocations[p.Character]
	}
	value := deckValue(p.Deck, p.Character, e)
	collectionValue := deckValue(p.Collection, p.Character, e)
	available := budget - value - collectionValue - p.UpgradeSpent
	if available < 0 {
		return fmt.Errorf("%s needs a budget of at least %d XP (proposed %d XP); raise its budget or sell cards first", p.Character, value+collectionValue+p.UpgradeSpent, budget)
	}
	if budget != *p.Budget || value != p.DeckValue || collectionValue != p.CollectionValue || e.AdminRevision != p.AdminRevision {
		p.Revision++
	}
	p.Budget = &budget
	p.DeckValue = value
	p.CollectionValue = collectionValue
	p.XP = available
	p.AdminRevision = e.AdminRevision
	return nil
}
func effectiveEconomy(root string, e Economy) (Economy, AdminSettings, error) {
	a := AdminSettings{CardPrices: map[string]int{}, Budgets: map[string]int{}}
	if root == "" {
		return e, a, fmt.Errorf("loadout root is required")
	}
	data, err := os.ReadFile(filepath.Join(root, "economy_admin.json"))
	if err != nil && !os.IsNotExist(err) {
		return e, a, err
	}
	if err == nil {
		if err = json.Unmarshal(data, &a); err != nil {
			return e, a, err
		}
	}
	if a.Revision < 0 {
		return e, a, fmt.Errorf("invalid admin revision")
	}
	// Deleted definitions can leave harmless old overrides on disk. Exclude
	// those from the editable snapshot and access rules; the next save drops them.
	if e.KnownCards != nil {
		for id := range a.CardPrices {
			if owner, _, _ := content.TreeCardOwner(e.Trees, id); !e.KnownCards[id] || owner != "" {
				delete(a.CardPrices, id)
			}
		}
		for id := range a.CardTypes {
			if !e.KnownCards[id] {
				delete(a.CardTypes, id)
			}
		}
	}
	for _, price := range a.CardPrices {
		if price < 1 || price > 1000000 {
			return e, a, fmt.Errorf("invalid admin card price")
		}
	}
	for _, budget := range a.Budgets {
		if budget < 0 || budget > 1000000 {
			return e, a, fmt.Errorf("invalid admin budget")
		}
	}
	e.Access = e.Access.withOverrides(a)
	e.GlobalPrices = a.CardPrices
	e.Budgets = a.Budgets
	e.BudgetAllocations = a.BudgetAllocations
	e.AdminRevision = a.Revision
	return e, a, nil
}

func ProgressSnapshot(root string, e Economy, catalogs map[string]content.BattleLibrary) (map[string]Progress, Economy, AdminSettings, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	return progressSnapshot(root, e, catalogs)
}
func progressSnapshot(root string, e Economy, catalogs map[string]content.BattleLibrary) (map[string]Progress, Economy, AdminSettings, error) {
	e, a, err := effectiveEconomy(root, e)
	all := map[string]Progress{}
	if err != nil {
		return all, e, a, err
	}
	for id, lib := range catalogs {
		p, err := readProgress(root, id, e, lib)
		if err != nil {
			return all, e, a, err
		}
		all[id] = p
	}
	return all, e, a, nil
}

// Settings are workspace-local admin overrides, not edits to source YAML or a
// remote authorization system. Prices apply to a card ID across every character.
func SaveAdmin(root string, base Economy, catalogs map[string]content.BattleLibrary, proposed AdminSettings) error {
	progressMu.Lock()
	defer progressMu.Unlock()
	all, _, current, err := progressSnapshot(root, base, catalogs)
	if err != nil {
		return err
	}
	if proposed.Revision != current.Revision {
		return fmt.Errorf("admin settings changed; reopen Admin settings before saving")
	}
	for id, price := range proposed.CardPrices {
		if owner, _, _ := content.TreeCardOwner(base.Trees, id); owner != "" && price != base.Authored[id].Buy {
			return fmt.Errorf("change %s XP in its card tree %s", id, owner)
		}
		found := false
		for _, lib := range catalogs {
			if _, ok := lib.Cards[id]; ok {
				found = true
			}
		}
		if !found || price < 1 || price > 1000000 {
			return fmt.Errorf("invalid admin price for %q (use 1–1000000 XP)", id)
		}
	}
	for id, budget := range proposed.Budgets {
		if _, ok := catalogs[id]; !ok || budget < 0 || budget > 1000000 {
			return fmt.Errorf("invalid admin budget for %q", id)
		}
	}
	if err := base.Access.withOverrides(proposed).validate(catalogs); err != nil {
		return err
	}
	// Record the free allocation included in each explicit total-budget override.
	proposed.BudgetAllocations = map[string]int{}
	for id := range proposed.Budgets {
		proposed.BudgetAllocations[id] = all[id].FreeDeckBudget
	}
	proposed.Revision++
	next := base
	next.GlobalPrices = proposed.CardPrices
	next.Budgets = proposed.Budgets
	next.BudgetAllocations = proposed.BudgetAllocations
	next.AdminRevision = proposed.Revision
	for _, p := range all {
		if err := reconcileBudget(&p, next); err != nil {
			return err
		}
	}
	// Validation of every character completes before this single atomic commit.
	return writeJSON(filepath.Join(root, "economy_admin.json"), proposed)
}
