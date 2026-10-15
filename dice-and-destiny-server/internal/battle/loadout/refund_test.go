package loadout

import (
	"encoding/json"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

// refundFixture is a minimal library for the purchase rules: a legacy card
// authored with a sale price below its value and an upgrade path into a tree
// base, a plain legacy card with a character upgrade path, and one card tree.
func refundFixture(t *testing.T) (Progress, Economy, content.BattleLibrary) {
	t.Helper()
	card := func(id string, economy *content.CardEconomy) content.BattleCardDefinition {
		return content.BattleCardDefinition{ID: id, Name: id, Economy: economy}
	}
	base := card("tree_base", &content.CardEconomy{Buy: 30, Sell: 30, CopyLimit: 3})
	lib := content.BattleLibrary{
		Cards: map[string]content.BattleCardDefinition{
			"legacy":    card("legacy", &content.CardEconomy{Buy: 20, Sell: 5, CopyLimit: 3, Upgrades: []content.CardUpgrade{{To: "tree_base", XP: 25}}}),
			"worn":      card("worn", nil),
			"plain":     card("plain", nil),
			"tree_base": base,
		},
		Abilities: map[string]content.BattleAbilityDefinition{"strike": {ID: "strike", Type: "offensive"}, "guard": {ID: "guard", Type: "defensive"}},
		CardTrees: map[string]content.CardTree{"base_paths": {ID: "base_paths", Name: "Base paths", Root: "base", Nodes: []content.CardTreeNode{{ID: "base", Card: base, XP: 30}}}},
	}
	e := Economy{Trees: lib.CardTrees, DefaultCardPrice: 10, Authored: map[string]content.CardEconomy{},
		Characters: map[string]CharacterEconomy{"hero": {CardUpgrades: map[string]Upgrade{"worn": {To: "plain", XP: 5}}}}}
	for id, c := range lib.Cards {
		if c.Economy != nil {
			e.Authored[id] = *c.Economy
		}
	}
	p := Progress{Version: 1, Revision: 1, Character: "hero", XP: 100, Deck: []Entry{{CardID: "legacy", Count: 2}, {CardID: "worn", Count: 1}}, Abilities: content.AbilityBoard{Offensive: []string{"strike"}, Defensive: []string{"guard"}}}
	initializeBudget(&p, e)
	if err := validateProgress(p, lib); err != nil {
		t.Fatal(err)
	}
	return p, e, lib
}

// trade applies and settles one transaction the way Buy and BuyCampaign do.
func trade(p Progress, e Economy, lib content.BattleLibrary, r Purchase) (Progress, int, error) {
	next, cost, err := applyPurchase(cloneProgress(p), p.Character, e, lib, r)
	if err != nil {
		return p, cost, err
	}
	next, err = settlePurchase(next, p.Character, e, lib, r.Kind, cost)
	return next, cost, err
}

// A campaign or expedition sheet refunds every XP spend in full; the admin
// editors keep their authored sale prices and upgrade costs (Phase 0, 0.2).
func TestCampaignRefundsEveryXPSpend(t *testing.T) {
	p, e, lib := refundFixture(t)
	same := func(a, b Progress) bool {
		return a.XP == b.XP && *a.Budget == *b.Budget && a.UpgradeSpent == b.UpgradeSpent
	}

	// Selling: the editors honour the authored sale price (5), so 15 XP sinks
	// into UpgradeSpent; a campaign refunds the copy's full value (20).
	sold, cost, err := trade(p, e, lib, Purchase{Kind: "sell_card", ID: "legacy"})
	if err != nil || cost != 5 || sold.UpgradeSpent != p.UpgradeSpent+15 {
		t.Fatalf("editor sale: cost %d upgrade spent %d (%v)", cost, sold.UpgradeSpent, err)
	}
	sold, cost, err = trade(p, e, lib, Purchase{Kind: "sell_card", ID: "legacy"}.InCampaign())
	if err != nil || cost != 20 || sold.XP != p.XP+20 || sold.UpgradeSpent != p.UpgradeSpent || *sold.Budget != *p.Budget {
		t.Fatalf("campaign sale: cost %d xp %d upgrade spent %d (%v)", cost, sold.XP, sold.UpgradeSpent, err)
	}

	// A legacy upgrade has no way back. The editors charge its authored 25 XP
	// (15 more than the value it adds); a campaign charges the 10 it adds, and
	// selling the result returns the upgrade's XP on top of the old copy's value.
	up, cost, err := trade(p, e, lib, Purchase{Kind: "upgrade_card", ID: "legacy", TargetID: "tree_base"})
	if err != nil || cost != 25 || up.UpgradeSpent != p.UpgradeSpent+15 {
		t.Fatalf("editor upgrade: cost %d upgrade spent %d (%v)", cost, up.UpgradeSpent, err)
	}
	up, cost, err = trade(p, e, lib, Purchase{Kind: "upgrade_card", ID: "legacy", TargetID: "tree_base"}.InCampaign())
	if err != nil || cost != 10 || up.UpgradeSpent != p.UpgradeSpent {
		t.Fatalf("campaign upgrade: cost %d upgrade spent %d (%v)", cost, up.UpgradeSpent, err)
	}
	back, cost, err := trade(up, e, lib, Purchase{Kind: "sell_card", ID: "tree_base"}.InCampaign())
	if err != nil || cost != 30 || !same(back, sold) {
		t.Fatalf("selling the upgraded copy: cost %d, ledger %d/%d/%d, want the plain sale's %d/%d/%d (%v)", cost, back.XP, *back.Budget, back.UpgradeSpent, sold.XP, *sold.Budget, sold.UpgradeSpent, err)
	}

	// A campaign checks the card an upgrade really adds, not the target the
	// request names: Worn's path leads to Plain, which is not a tree card.
	if _, _, err = trade(p, e, lib, Purchase{Kind: "upgrade_card", ID: "worn", TargetID: "tree_base"}.InCampaign()); err == nil || !strings.Contains(err.Error(), "plain can be sold but not added") {
		t.Fatalf("a campaign added a non-tree card through a legacy upgrade: %v", err)
	}
	if _, _, err = trade(p, e, lib, Purchase{Kind: "upgrade_card", ID: "worn"}); err != nil {
		t.Fatalf("the editors keep legacy upgrades: %v", err)
	}

	// Only the authority marks a campaign transaction; a request cannot.
	var decoded Purchase
	if err = json.Unmarshal([]byte(`{"kind":"sell_card","id":"legacy","Campaign":true,"campaign":true}`), &decoded); err != nil || decoded.Campaign {
		t.Fatalf("a request set the campaign flag: %+v (%v)", decoded, err)
	}
}
