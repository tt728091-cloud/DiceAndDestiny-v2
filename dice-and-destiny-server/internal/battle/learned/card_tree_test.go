package learned

import (
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func treeFixture(t *testing.T) (string, map[string]content.BattleLibrary, content.CardTree) {
	t.Helper()
	root := filepath.Join(testServerRoot(t), "content")
	libs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	base, err := content.EditableGeneralCard(libs["adventurer"].Cards["brace"])
	if err != nil {
		t.Fatal(err)
	}
	base.Economy = &content.CardEconomy{Buy: 10, Sell: 10, CopyLimit: 20}
	base.Cost.Energy = 1
	base.Program.Steps[0].Params = map[string]any{"amount": 1, "destination": "discard"}
	tree := content.CardTree{ID: "test_tree", Name: "Brace progression", Root: "base", Nodes: []content.CardTreeNode{{ID: "base", Card: base}}}
	for _, v := range []struct {
		id                     string
		xp, energy, prevention int
		destination            string
	}{{"guard", 13, 1, 2, "discard"}, {"original", 14, 1, 1, "original"}, {"both", 17, 1, 2, "original"}, {"heavy", 8, 2, 1, "discard"}} {
		raw, _ := json.Marshal(base)
		var c content.BattleCardDefinition
		json.Unmarshal(raw, &c)
		c.ID = "test_tree_variant_" + v.id
		c.Name = "Tree " + v.id
		c.Cost.Energy = v.energy
		c.Economy.Buy = v.xp
		c.Economy.Sell = v.xp
		c.Program.Steps[0].Params = map[string]any{"amount": v.prevention, "destination": v.destination}
		tree.Nodes = append(tree.Nodes, content.CardTreeNode{ID: v.id, Card: c, Y: float64((10 - v.xp) * 100)})
	}
	for i, pair := range [][2]string{{"base", "guard"}, {"base", "original"}, {"guard", "both"}, {"original", "both"}, {"base", "heavy"}} {
		tree.Edges = append(tree.Edges, content.CardTreeEdge{ID: []string{"guard", "original", "both_a", "both_b", "heavy"}[i], From: pair[0], To: pair[1], Reversible: true, Requirements: []content.CardTreeRequirement{}})
	}
	return root, libs, tree
}
func TestCardTreeTransactionsAndPersistence(t *testing.T) {
	root, libs, tree := treeFixture(t)
	dir := t.TempDir()
	saved, err := content.SaveCardTree(dir, libs["adventurer"], tree, 0)
	if err != nil {
		t.Fatal(err)
	}
	libs, err = CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	e, err := loadout.LoadEconomy(root, libs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	if err != nil {
		t.Fatal(err)
	}
	startXP := p.XP
	startValue := p.DeckValue
	trade := func(from, to string, cost int) {
		t.Helper()
		p, err = loadout.Buy(dir, "adventurer", e, libs["adventurer"], loadout.Purchase{Kind: "tree_card", ID: from, TargetID: to, ExpectedCost: cost, Revision: p.Revision})
		if err != nil {
			t.Fatal(err)
		}
	}
	trade("brace", "test_tree_variant_guard", 3)
	trade("test_tree_variant_guard", "test_tree_variant_both", 4)
	if p.XP != startXP-7 || p.DeckValue+p.CollectionValue != startValue+7 {
		t.Fatal("upgrade math", p)
	}
	trade("test_tree_variant_both", "test_tree_variant_original", -3)
	trade("test_tree_variant_original", "brace", -4)
	trade("brace", "test_tree_variant_heavy", -2)
	if p.XP != startXP+2 || p.DeckValue+p.CollectionValue != startValue-2 {
		t.Fatal("downgrade refund", p)
	}
	trade("test_tree_variant_heavy", "brace", 2)
	if p.XP != startXP || p.DeckValue+p.CollectionValue != startValue {
		t.Fatal("cycle minted XP", p)
	}
	reloaded, err := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	if err != nil || reloaded.XP != startXP {
		t.Fatal(err)
	}
	for _, req := range []loadout.Purchase{{Kind: "buy_card", ID: "test_tree_variant_both", ExpectedCost: 17, Revision: p.Revision}, {Kind: "tree_card", ID: "brace", TargetID: "test_tree_variant_both", ExpectedCost: 7, Revision: p.Revision}, {Kind: "tree_card", ID: "brace", TargetID: "test_tree_variant_guard", ExpectedCost: 0, Revision: p.Revision}, {Kind: "tree_card", ID: "brace", TargetID: "test_tree_variant_guard", ExpectedCost: 3, Revision: p.Revision - 1}} {
		if _, err := loadout.Buy(dir, "adventurer", e, libs["adventurer"], req); err == nil {
			t.Fatal("invalid transaction accepted", req)
		}
	}
	if _, err = content.SaveCardTree(dir, libs["adventurer"], tree, saved.Revision-1); err == nil {
		t.Fatal("stale tree saved")
	}
	if _, err = content.SaveAuthoredCard(dir, libs["adventurer"], tree.Nodes[1].Card, saved.Revision); err == nil {
		t.Fatal("managed node overwritten outside tree")
	}
}
func TestCardTreeGatesAndGraphValidation(t *testing.T) {
	root, libs, tree := treeFixture(t)
	zero := 0
	tree.Edges[0].Requirements = []content.CardTreeRequirement{{CardID: "strong_swing", Maximum: &zero}, {CardID: "brace_plus", Minimum: 1}}
	dir := t.TempDir()
	if _, err := content.SaveCardTree(dir, libs["adventurer"], tree, 0); err != nil {
		t.Fatal(err)
	}
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := loadout.LoadEconomy(root, libs)
	deck := []loadout.Entry{{CardID: "brace", Count: 2}, {CardID: "brace_plus", Count: 1}, {CardID: "strong_swing", Count: 1}}
	if _, _, err := loadout.TreeTransition(deck, "brace", "test_tree_variant_guard", e, libs["adventurer"], "adventurer"); err != nil {
		t.Fatal("deck gates must not block upgrades", err)
	}
	deck = deck[:2]
	changed, cost, err := loadout.TreeTransition(deck, "brace", "test_tree_variant_guard", e, libs["adventurer"], "adventurer")
	if err != nil || cost != 3 {
		t.Fatal(err)
	}
	changed = append(changed, loadout.Entry{CardID: "strong_swing", Count: 1})
	if err := loadout.ValidateTreeDeck(changed, libs["adventurer"].CardTrees); err == nil {
		t.Fatal("later deck edit bypasses gate")
	}
	if _, err := loadout.WriteSharedDeck(dir, "adventurer", changed, e, libs["adventurer"]); err == nil {
		t.Fatal("sandbox bypasses gate")
	}
	if err := loadout.ValidateTreeDeck([]loadout.Entry{{CardID: "test_tree_variant_both", Count: 1}, {CardID: "strong_swing", Count: 1}}, libs["adventurer"].CardTrees); err != nil {
		t.Fatal("alternative path blocked", err)
	}
	for _, mutate := range []func(*content.CardTree){
		func(x *content.CardTree) {
			x.Edges = append(x.Edges, content.CardTreeEdge{ID: "cycle", From: "both", To: "guard"})
		},
		func(x *content.CardTree) { x.Edges = x.Edges[:3] },
		func(x *content.CardTree) { x.Edges[0].Requirements[0].CardID = "missing" },
		func(x *content.CardTree) { x.Nodes[1].Card.Economy.Buy = 0 },
		func(x *content.CardTree) { x.Nodes[1].Card.ID = "brace_plus" },
	} {
		raw, _ := json.Marshal(tree)
		var invalid content.CardTree
		json.Unmarshal(raw, &invalid)
		mutate(&invalid)
		saved, _ := content.ReadAuthoredCards(dir)
		if _, _, err := content.PreviewCardTree(saved, libs["adventurer"], invalid); err == nil {
			t.Fatal("invalid tree accepted")
		}
	}
	tree.Edges[0].Requirements = []content.CardTreeRequirement{{CardID: "test_tree_variant_original", Maximum: &zero, Descendants: true}}
	tree.Edges[1].Requirements = []content.CardTreeRequirement{{CardID: "test_tree_variant_guard", Maximum: &zero, Descendants: true}}
	if err := loadout.ValidateTreeDeck([]loadout.Entry{{CardID: "test_tree_variant_guard", Count: 1}, {CardID: "test_tree_variant_original", Count: 1}}, map[string]content.CardTree{tree.ID: tree}); err == nil {
		t.Fatal("exclusive branches coexist")
	}
}
func TestCardTreeAdminAtomicPublication(t *testing.T) {
	root, _, tree := treeFixture(t)
	dir := t.TempDir()
	raw, _ := json.Marshal(tree)
	req := runtimeRequest{Op: "publish_card_tree", ContentRoot: root, LoadoutRoot: dir, CardTree: raw, Character: "adventurer"}
	if runtimeCall(t, req)["ok"] == true {
		t.Fatal("nonadmin publish")
	}
	opened := runtimeCall(t, runtimeRequest{Op: "open_card_admin", ContentRoot: root, LoadoutRoot: dir})
	req.AdminToken = opened["result"].(map[string]any)["admin_token"].(string)
	published := runtimeCall(t, req)
	if published["ok"] != true {
		t.Fatal(published)
	}
	if runtimeCall(t, req)["ok"] == true {
		t.Fatal("stale publish")
	}
	view := runtimeCall(t, runtimeRequest{Op: "card_trees", ContentRoot: root, LoadoutRoot: dir, Character: "adventurer"})
	if view["ok"] != true {
		t.Fatal(view)
	}
	libs, _ := CharacterCatalogs(root, dir)
	e, _ := loadout.LoadEconomy(root, libs)
	p, _ := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	_, err := loadout.Buy(dir, "adventurer", e, libs["adventurer"], loadout.Purchase{Kind: "tree_card", ID: "brace", TargetID: "test_tree_variant_guard", ExpectedCost: 3, Revision: p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	tree.Nodes = append(tree.Nodes[:1], tree.Nodes[2:]...)
	tree.Edges = tree.Edges[1:]
	tree.Edges = append(tree.Edges[:1], tree.Edges[2:]...)
	raw, _ = json.Marshal(tree)
	req.CardTree = raw
	req.CatalogRevision = 1
	if got := runtimeCall(t, req); got["ok"] == true {
		t.Fatal("owned node removed")
	}
	saved, _ := content.ReadAuthoredCards(dir)
	if saved.Revision != 1 || len(saved.Trees["test_tree"].Nodes) != 5 {
		t.Fatal("failed publication partially committed")
	}
}

func TestCardTreeVariantsCompleteBattlesAndPinDefinitions(t *testing.T) {
	root, libs, tree := treeFixture(t)
	dir := t.TempDir()
	if _, err := content.SaveCardTree(dir, libs["adventurer"], tree, 0); err != nil {
		t.Fatal(err)
	}
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(root, libs)
	if err != nil {
		t.Fatal(err)
	}
	deck := []loadout.Entry{}
	for _, n := range tree.Nodes {
		deck = append(deck, loadout.Entry{CardID: n.Card.ID, Count: 2})
	}
	if _, err := loadout.WriteSharedDeck(dir, "adventurer", deck, economy, libs["adventurer"]); err != nil {
		t.Fatal(err)
	}
	played := 0
	for _, count := range []int{1, 2} {
		s, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: count})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResetCharacter(fmt.Sprintf("tree-%d", count), uint64(11+count), "seat-a", false, "adventurer", true); err != nil {
			t.Fatal(err)
		}
		pinned, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
		for _, n := range tree.Nodes {
			if !strings.Contains(string(pinned), n.Card.ID) {
				t.Fatal("variant missing from pinned catalog", n.Card.ID)
			}
		}
		if count == 1 {
			tree.Nodes[1].Card.Cost.Energy = 3
			if _, err := content.SaveCardTree(dir, libs["adventurer"], tree, 1); err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
			if string(after) != string(pinned) {
				t.Fatal("tree publish changed active battle")
			}
			current, err := CharacterCatalogs(root, dir)
			if err != nil || current["adventurer"].Cards["test_tree_variant_guard"].Cost.Energy != 3 {
				t.Fatal("new catalog did not receive edit", err)
			}
		}
		for step := 0; step < 1200 && !s.current.Terminal; step++ {
			if s.isModelSeat(s.current.ActorID) {
				_, err = s.AdvanceModel()
			} else {
				actions := s.current.Result.LegalActions
				if len(actions) == 0 {
					t.Fatal("battle blocked without legal actions")
				}
				action := adventurerTestAction(actions, 3)
				for _, a := range actions {
					var payload map[string]any
					_ = json.Unmarshal(a.Payload, &payload)
					key, _ := payload["status_id"].(string)
					if c, ok := payload["commitment"].(map[string]any); ok {
						key, _ = c["choice_id"].(string)
					}
					var choice map[string]any
					if json.Unmarshal([]byte(key), &choice) == nil && choice["verb"] != nil && choice["verb"] != "cancel" {
						action = a
						played++
						break
					}
				}
				encoded, _ := json.Marshal(aliasValue(commandMap(t, action), s.aliases(false)))
				_, err = s.SubmitHuman(string(encoded))
			}
			if err != nil {
				t.Fatalf("%d enemies, step %d: %v", count, step, err)
			}
		}
		if !s.current.Terminal || s.current.TruncationReason != "" {
			t.Fatal("tree battle did not finish", s.current.TruncationReason)
		}
		telemetry, _ := s.Telemetry()
		if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 {
			t.Fatalf("invalid tree battle actions: %+v", telemetry)
		}
	}
	if played == 0 {
		t.Fatal("no tree cards played")
	}
}

func TestCardTreeRestrictionsApplyOnlyToEquippedDeck(t *testing.T) {
	root, libs, tree := treeFixture(t)
	dir := t.TempDir()
	zero := 0
	tree.Edges[0].Requirements = []content.CardTreeRequirement{{CardID: "strong_swing", Maximum: &zero}, {CardID: "brace_plus", Minimum: 1}}
	if _, err := content.SaveCardTree(dir, libs["adventurer"], tree, 0); err != nil {
		t.Fatal(err)
	}
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	e, err := loadout.LoadEconomy(root, libs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	if err != nil {
		t.Fatal(err)
	}
	initialXP, initialValue := p.XP, p.DeckValue
	trade := func(kind, id, target string, cost int) error {
		next, err := loadout.Buy(dir, "adventurer", e, libs["adventurer"], loadout.Purchase{Kind: kind, ID: id, TargetID: target, ExpectedCost: cost, Revision: p.Revision})
		if err == nil {
			p = next
		}
		return err
	}
	// Strong Swing is equipped, yet the upgrade succeeds and goes to collection.
	if err := trade("tree_card", "brace", "test_tree_variant_guard", 3); err != nil {
		t.Fatal(err)
	}
	if p.CollectionValue != 13 || p.XP != initialXP-3 || p.DeckValue != initialValue-10 || p.UpgradeSpent != 0 {
		t.Fatal("collection accounting", p)
	}
	before, _ := json.Marshal(p)
	if err := trade("equip_collection_card", "test_tree_variant_guard", "", 0); err == nil {
		t.Fatal("forbidden deck equipped")
	}
	reloaded, err := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	after, _ := json.Marshal(reloaded)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed equip mutated saved state", err)
	}
	// Moving forbidden cards out of the deck suffices; collection ownership is irrelevant.
	for i := 0; i < 2; i++ {
		if err := trade("unequip_collection_card", "strong_swing", "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := trade("equip_collection_card", "test_tree_variant_guard", "", 0); err != nil {
		t.Fatal(err)
	}
	if p.XP != initialXP-3 || p.CollectionValue != 20 {
		t.Fatal("equip charged extra XP", p)
	}
	if err := trade("unequip_collection_card", "brace_plus", "", 0); err == nil {
		t.Fatal("removed equipped prerequisite")
	}
	if err := trade("equip_collection_card", "strong_swing", "", 0); err == nil {
		t.Fatal("conflicting card added afterwards")
	}
	if err := trade("unequip_collection_card", "test_tree_variant_guard", "", 0); err != nil {
		t.Fatal(err)
	}
	if err := trade("unequip_collection_card", "brace_plus", "", 0); err != nil {
		t.Fatal(err)
	}
	if err := trade("equip_collection_card", "test_tree_variant_guard", "", 0); err == nil {
		t.Fatal("missing prerequisite equipped")
	}
	if err := trade("sell_collection_card", "test_tree_variant_guard", "", 13); err != nil {
		t.Fatal(err)
	}
	if p.XP != initialXP+10 {
		t.Fatal("sale did not refund stored XP", p)
	}
}

func TestCardTreeAuthorsNewBaseCard(t *testing.T) {
	root, libs, tree := treeFixture(t)
	dir := t.TempDir()
	rename := func(x *content.CardTree, id, name string) {
		x.ID, x.Name = "steady_tree", "Steady Guard paths"
		x.Nodes[0].Card.ID, x.Nodes[0].Card.Name = id, name
		for i := 1; i < len(x.Nodes); i++ {
			x.Nodes[i].Card.ID = "steady_tree_variant_" + x.Nodes[i].ID
			x.Nodes[i].Card.Name = name + " · " + x.Nodes[i].ID
		}
	}
	rename(&tree, "steady_guard", "Steady Guard")
	for _, invalid := range []struct{ id, name string }{{"Steady Guard", "Steady Guard"}, {"steady_tree_variant_base", "Steady Guard"}, {"steady_guard", "Brace"}} {
		raw, _ := json.Marshal(tree)
		var candidate content.CardTree
		json.Unmarshal(raw, &candidate)
		rename(&candidate, invalid.id, invalid.name)
		if _, err := content.SaveCardTree(dir, libs["adventurer"], candidate, 0); err == nil {
			t.Fatal("invalid new base accepted", invalid)
		}
	}
	if _, err := content.SaveCardTree(dir, libs["adventurer"], tree, 0); err != nil {
		t.Fatal(err)
	}
	libs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	base, ok := libs["adventurer"].Cards["steady_guard"]
	if !ok || base.Name != "Steady Guard" || libs["adventurer"].Cards["brace"].Name != "Brace" {
		t.Fatal("new base card missing or existing Brace changed")
	}
	if steps := libs["adventurer"].Cards["brace"].Program; steps != nil && content.ProgramInt(steps.Steps[0], "amount") == 1 {
		t.Fatal("existing Brace adopted the new base settings")
	}
	e, err := loadout.LoadEconomy(root, libs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(dir, "adventurer", e, libs["adventurer"])
	if err != nil {
		t.Fatal(err)
	}
	if p, err = loadout.Buy(dir, "adventurer", e, libs["adventurer"], loadout.Purchase{Kind: "buy_collection_card", ID: "steady_guard", ExpectedCost: 10, Revision: p.Revision}); err != nil {
		t.Fatal(err)
	}
	if _, err = loadout.Buy(dir, "adventurer", e, libs["adventurer"], loadout.Purchase{Kind: "tree_card", ID: "steady_guard", TargetID: "steady_tree_variant_guard", ExpectedCost: 3, Revision: p.Revision}); err != nil {
		t.Fatal(err)
	}
	// A published tree keeps the base it authored.
	saved, _ := content.ReadAuthoredCards(dir)
	tree.Nodes[0].Card.ID = "other_guard"
	if _, _, err := content.PreviewCardTree(saved, libs["adventurer"], tree); err == nil {
		t.Fatal("published tree changed its base")
	}
}
