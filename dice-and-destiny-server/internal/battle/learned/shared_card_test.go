package learned

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// sharedTreesFixture lends one shared card, "Lent Guard", to two trees that
// price it differently: 15 XP in guard_tree (base Steady Guard) and 25 XP in
// nudge_tree (base Nudge).
func sharedTreesFixture(t *testing.T) (string, string, content.CardTree, content.CardTree) {
	t.Helper()
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	libs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := libs["adventurer"]
	shared, err := content.EditableGeneralCard(lib.Cards["steady_guard"])
	if err != nil {
		t.Fatal(err)
	}
	shared.ID, shared.Name, shared.Economy = "lent_guard", "Lent Guard", nil
	shared.Program.Steps[0].Params = map[string]any{"amount": 2, "destination": "discard"}
	tree := func(id, name, baseID string, xp int) content.CardTree {
		base, err := content.EditableGeneralCard(lib.Cards[baseID])
		if err != nil {
			t.Fatal(err)
		}
		return content.CardTree{ID: id, Name: name, Root: "base",
			Nodes: []content.CardTreeNode{{ID: "base", Card: base, XP: 10}, {ID: "lent", Card: shared, XP: xp, Shared: true, Y: -360}},
			Edges: []content.CardTreeEdge{{ID: "up", From: "base", To: "lent", Reversible: true, Requirements: []content.CardTreeRequirement{}}}}
	}
	guard, nudge := tree("guard_tree", "Guard tree", "steady_guard", 15), tree("nudge_tree", "Nudge tree", "nudge", 25)
	saved, err := content.SaveCardTree(dir, lib, guard, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = content.SaveCardTree(dir, lib, nudge, saved.Revision); err != nil {
		t.Fatal(err)
	}
	return root, dir, guard, nudge
}

func TestSharedCardHasOneDefinitionAndTreeOwnedXP(t *testing.T) {
	root, dir, guard, _ := sharedTreesFixture(t)
	saved, _ := content.ReadAuthoredCards(dir)
	if c := saved.Cards["lent_guard"]; c.Economy == nil || c.Economy.Buy != 0 || c.Economy.Sell != 0 {
		t.Fatalf("a shared card has no XP of its own: %+v", c.Economy)
	}
	for id, xp := range map[string]int{"guard_tree": 15, "nudge_tree": 25} {
		if n, ok := saved.Trees[id].CardNode("lent_guard"); !ok || !n.Shared || n.Value() != xp {
			t.Fatalf("%s owns its XP for the shared card: %+v", id, n)
		}
	}
	// Editing the card in one tree changes the one definition everywhere.
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	guard = saved.Trees["guard_tree"]
	guard.Nodes[1].Card.Cost.Energy = 10
	if saved, err = content.SaveCardTree(dir, libs["adventurer"], guard, saved.Revision); err != nil {
		t.Fatal(err)
	}
	other, _ := saved.Trees["nudge_tree"].CardNode("lent_guard")
	libs, _ = CharacterCatalogs(root, dir)
	if other.Card.Cost.Energy != 10 || libs["adventurer"].Cards["lent_guard"].Cost.Energy != 10 || other.Value() != 25 {
		t.Fatalf("shared edit must reach every tree and keep each tree's XP: energy %d, catalog %d, xp %d", other.Card.Cost.Energy, libs["adventurer"].Cards["lent_guard"].Cost.Energy, other.Value())
	}
	// Dropping the card from one tree keeps it for the other.
	guard.Nodes, guard.Edges = guard.Nodes[:1], nil
	if saved, err = content.SaveCardTree(dir, libs["adventurer"], guard, saved.Revision); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Cards["lent_guard"]; !ok {
		t.Fatal("a shared card still lent to another tree was deleted")
	}
	// Once no tree lends it, it has no XP anywhere and leaves the catalog
	// instead of lingering as a free card.
	libs, _ = CharacterCatalogs(root, dir)
	nudge := saved.Trees["nudge_tree"]
	nudge.Nodes, nudge.Edges = nudge.Nodes[:1], nil
	if saved, err = content.SaveCardTree(dir, libs["adventurer"], nudge, saved.Revision); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Cards["lent_guard"]; ok {
		t.Fatal("an unlent shared card stayed in the catalog without a price")
	}
}

func TestSharedCardRules(t *testing.T) {
	root, dir, _, nudge := sharedTreesFixture(t)
	libs, _ := CharacterCatalogs(root, dir)
	lib := libs["adventurer"]
	saved, _ := content.ReadAuthoredCards(dir)
	clone := func(tree content.CardTree) content.CardTree {
		raw, _ := json.Marshal(tree)
		var c content.CardTree
		_ = json.Unmarshal(raw, &c)
		return c
	}
	cases := map[string]func() content.CardTree{
		"shared base": func() content.CardTree {
			c := clone(nudge)
			c.Nodes[0].Shared = true
			return c
		},
		"another tree's base reused": func() content.CardTree {
			c := clone(nudge)
			c.ID, c.Name = "third_tree", "Third tree"
			c.Nodes[0].Card, _ = content.EditableGeneralCard(lib.Cards["steady_guard"])
			return c
		},
		"tree variant ID as shared card": func() content.CardTree {
			c := clone(nudge)
			c.Nodes[1].Card.ID = "nudge_tree_variant_lent"
			return c
		},
		"same shared card twice": func() content.CardTree {
			c := clone(nudge)
			twin := c.Nodes[1]
			twin.ID = "lent_again"
			c.Nodes = append(c.Nodes, twin)
			c.Edges = append(c.Edges, content.CardTreeEdge{ID: "again", From: "base", To: "lent_again", Requirements: []content.CardTreeRequirement{}})
			return c
		},
	}
	for name, build := range cases {
		if _, _, err := content.PreviewCardTree(saved, lib, build()); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSharedCardCopiesStayInTheirTree(t *testing.T) {
	root, dir, _, _ := sharedTreesFixture(t)
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	lib := libs["adventurer"]
	e, err := loadout.LoadEconomy(root, libs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(dir, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	startXP, startBudget := p.XP, *p.Budget
	trade := func(req loadout.Purchase) error {
		req.Revision = p.Revision
		next, err := loadout.Buy(dir, "adventurer", e, lib, req)
		if err == nil {
			p = next
		}
		return err
	}
	// Each tree charges its own step to the shared card.
	if err := trade(loadout.Purchase{Kind: "tree_card", ID: "steady_guard", TargetID: "lent_guard", Tree: "guard_tree", ExpectedCost: 5}); err != nil {
		t.Fatal(err)
	}
	if err := trade(loadout.Purchase{Kind: "tree_card", ID: "nudge", TargetID: "lent_guard", Tree: "nudge_tree", ExpectedCost: 15}); err != nil {
		t.Fatal(err)
	}
	if p.XP != startXP-20 || *p.Budget != startBudget {
		t.Fatalf("tree-owned XP: xp %d budget %d", p.XP, *p.Budget)
	}
	tags := map[string]int{}
	for _, entry := range p.Collection {
		if entry.CardID == "lent_guard" {
			tags[entry.Tree] += entry.Count
		}
	}
	if tags["guard_tree"] != 1 || tags["nudge_tree"] != 1 {
		t.Fatalf("each copy remembers its tree: %+v", p.Collection)
	}
	// Selling names the copy; each refunds its own tree's value.
	if err := trade(loadout.Purchase{Kind: "sell_collection_card", ID: "lent_guard", ExpectedCost: 25}); err == nil || !strings.Contains(err.Error(), "which tree") {
		t.Fatalf("ambiguous sale accepted: %v", err)
	}
	xp := p.XP
	if err := trade(loadout.Purchase{Kind: "sell_collection_card", ID: "lent_guard", Tree: "nudge_tree", ExpectedCost: 25}); err != nil || p.XP != xp+25 {
		t.Fatalf("nudge_tree copy refunds 25: %v %d", err, p.XP-xp)
	}
	// No bridge: the remaining Guard-tree copy cannot step down into Nudge's
	// base, but it can step back down its own tree.
	if err := trade(loadout.Purchase{Kind: "tree_card", ID: "lent_guard", TargetID: "nudge", Tree: "nudge_tree", ExpectedCost: -15}); err == nil {
		t.Fatal("a copy from guard_tree crossed into nudge_tree")
	}
	// The remaining copy equips by inference and battles see one card.
	if err := trade(loadout.Purchase{Kind: "equip_collection_card", ID: "lent_guard"}); err != nil {
		t.Fatal(err)
	}
	battle := loadout.BattleDeck(append(append([]loadout.Entry(nil), p.Deck...), loadout.Entry{CardID: "lent_guard", Count: 1, Tree: "nudge_tree"}))
	count := 0
	for _, entry := range battle {
		if entry.CardID == "lent_guard" {
			count += entry.Count
			if entry.Tree != "" {
				t.Fatal("battle decks carry no tree tags")
			}
		}
	}
	if count != 2 {
		t.Fatalf("battle deck merges copies from different trees: %d", count)
	}
	// Reload keeps the tags and the ledger.
	again, err := loadout.ReadProgress(dir, "adventurer", e, lib)
	if err != nil || again.XP != p.XP || *again.Budget != startBudget {
		t.Fatalf("reload: %v %+v", err, again)
	}
}
