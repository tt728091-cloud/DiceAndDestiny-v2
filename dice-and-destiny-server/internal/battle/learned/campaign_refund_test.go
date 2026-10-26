package learned

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// refundSheet applies Prepare transactions to one campaign or expedition save
// through the authority's real write path.
type refundSheet struct {
	t     *testing.T
	save  loadout.CampaignSave
	apply func(loadout.Purchase) (loadout.CampaignSave, error)
	done  int
}

type xpLedger struct{ XP, Budget, UpgradeSpent int }

func (s *refundSheet) ledger() xpLedger {
	p := s.save.Sheet
	return xpLedger{p.XP, *p.Budget, p.UpgradeSpent}
}

func (s *refundSheet) owned() map[string]int {
	out := map[string]int{}
	for _, pile := range [][]loadout.Entry{s.save.Sheet.Deck, s.save.Sheet.Collection} {
		for _, e := range pile {
			out[e.CardID+"|"+e.Tree] += e.Count
		}
	}
	return out
}

// do applies one transaction quoted at cost and checks it moved XP by exactly
// that much: sales and downgrades refund it, everything else spends it.
func (s *refundSheet) do(r loadout.Purchase, cost int) {
	s.t.Helper()
	before := s.save.Sheet.XP
	r.Revision, r.ExpectedCost = s.save.Sheet.Revision, cost
	next, err := s.apply(r)
	if err != nil {
		s.t.Fatalf("%s %s → %s (tree %q, ×%d) at %d XP: %v", r.Kind, r.ID, r.TargetID, r.Tree, max(1, r.Count), cost, err)
	}
	change := -cost
	if r.Kind == "sell_card" || r.Kind == "sell_collection_card" || r.Kind == "downgrade_ability" {
		change = cost
	}
	if next.Sheet.XP != before+change {
		s.t.Fatalf("%s %s → %s changed XP by %d, not %d", r.Kind, r.ID, r.TargetID, next.Sheet.XP-before, change)
	}
	s.save = next
	s.done++
}

func (s *refundSheet) expect(want xpLedger, what string) {
	s.t.Helper()
	if got := s.ledger(); got != want {
		s.t.Fatalf("%s: XP/Budget/UpgradeSpent %+v, want %+v", what, got, want)
	}
}

// trade moves count copies one step through a tree, priced by the tree's XP.
func (s *refundSheet) trade(tr content.CardTree, from, to string, count int) {
	s.t.Helper()
	a, _ := tr.Node(from)
	b, _ := tr.Node(to)
	s.do(loadout.Purchase{Kind: "tree_card_deck", ID: a.Card.ID, TargetID: b.Card.ID, Tree: tr.ID, Count: count}, count*(b.Value()-a.Value()))
}

type treeStep struct{ from, to string }

// reversiblePaths finds a path from the base to every node it reaches by
// reversible connections, so every step can be traded back down.
func reversiblePaths(tr content.CardTree) map[string][]treeStep {
	paths := map[string][]treeStep{tr.Root: nil}
	queue := []string{tr.Root}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, e := range tr.Edges {
			if !e.Reversible {
				continue
			}
			for _, ends := range [][2]string{{e.From, e.To}, {e.To, e.From}} {
				if _, seen := paths[ends[1]]; ends[0] != at || seen {
					continue
				}
				paths[ends[1]] = append(slices.Clone(paths[at]), treeStep{ends[0], ends[1]})
				queue = append(queue, ends[1])
			}
		}
	}
	return paths
}

func sortedTrees(lib content.BattleLibrary) []string {
	ids := make([]string, 0, len(lib.CardTrees))
	for id := range lib.CardTrees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Every XP spend on a campaign or expedition sheet can be undone for exactly
// the XP it cost (Phase 0, task 0.2), over the shipped card trees and ability
// upgrade paths: XP, Budget and UpgradeSpent end every round trip as they began.
func TestCampaignXPSpendsRoundTripExactly(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	root := campaignLoadoutRoot(t)
	x, err := loadExpeditionContext(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, character := range x.campaign.Characters {
		lib := x.catalogs[character]
		rules := x.rules[character]
		t.Run(character+"/linear", func(t *testing.T) {
			save := startCampaign(t, contentRoot, root, character, "Refunds")
			s := &refundSheet{t: t, save: save, apply: func(r loadout.Purchase) (loadout.CampaignSave, error) {
				return loadout.BuyCampaign(root, save.ID, x.economy, x.catalogs, r)
			}}
			roundTripEverything(s, lib, x.economy.Characters[character].AbilityUpgrades)
			roundTripBoard(t, contentRoot, root, save.ID, character)
			if character != loadout.CustomAdventurer {
				// An older save on its authored abilities: the Guard → Guard+
				// path round-trips too.
				old := startAuthoredCampaign(t, contentRoot, root, character, "Old refunds")
				s := &refundSheet{t: t, save: old, apply: func(r loadout.Purchase) (loadout.CampaignSave, error) {
					return loadout.BuyCampaign(root, old.ID, x.economy, x.catalogs, r)
				}}
				roundTripEverything(s, lib, x.economy.Characters[character].AbilityUpgrades)
			}
		})
		t.Run(character+"/expedition", func(t *testing.T) {
			save, err := newExpeditionCampaign(contentRoot, root, character, "Refunds", "", 5)
			if err != nil {
				t.Fatal(err)
			}
			act := func(a expedition.Action) loadout.CampaignSave {
				next, _, err := loadout.ExpeditionAct(root, save.ID, x.economy, x.catalogs, rules, a, 0)
				if err != nil {
					t.Fatalf("%s: %v", a.Kind, err)
				}
				return next
			}
			// Camp opens the deck for Influence: cards played there, not XP.
			fee := rules.Config.DeckChangeInfluence["camp"]
			camp := act(expedition.Action{Kind: "make_camp"})
			if len(camp.Expedition.Hand) < fee {
				t.Fatalf("the camp hand holds %d cards; opening the deck costs %d Influence", len(camp.Expedition.Hand), fee)
			}
			paid := act(expedition.Action{Kind: "camp_service", Service: "influence", Cards: camp.Expedition.Hand[:fee]})
			if paid.Sheet.XP != save.Sheet.XP || paid.Expedition.Turn.Influence != fee {
				t.Fatalf("Influence is cards played, never XP: xp %d → %d, influence %d", save.Sheet.XP, paid.Sheet.XP, paid.Expedition.Turn.Influence)
			}
			s := &refundSheet{t: t, save: paid, apply: func(r loadout.Purchase) (loadout.CampaignSave, error) {
				return loadout.ExpeditionDeckChange(root, save.ID, x.economy, x.catalogs, rules, r)
			}}
			// The first change spends the fee and opens the deck; do() checks it
			// still moved XP by exactly its price.
			roundTripEverything(s, lib, x.economy.Characters[character].AbilityUpgrades)
			roundTripBoard(t, contentRoot, root, save.ID, character)
			s.save = readSave(t, contentRoot, root, save.ID)
			if s.save.Expedition.Turn.DeckOpen != "camp" || s.save.Expedition.Turn.Influence != 0 {
				t.Fatalf("the camp fee was paid once: open %q, influence left %d", s.save.Expedition.Turn.DeckOpen, s.save.Expedition.Turn.Influence)
			}
			piles := len(s.save.Expedition.Hand) + len(s.save.Expedition.Draw) + len(s.save.Expedition.Spent)
			if piles != deckHealth(s.save.Sheet.Deck) {
				t.Fatalf("the map piles hold %d cards, the deck %d", piles, deckHealth(s.save.Sheet.Deck))
			}
		})
	}
}

func roundTripEverything(s *refundSheet, lib content.BattleLibrary, abilityPaths map[string]loadout.Upgrade) {
	t := s.t
	t.Helper()
	start, owned := s.ledger(), s.owned()

	// Every connection of every tree: buy the base, trade up to the connection
	// and across it, trade back down to the base, then sell the base.
	for _, id := range sortedTrees(lib) {
		tr := lib.CardTrees[id]
		base, _ := tr.Node(tr.Root)
		before := s.ledger()
		s.do(loadout.Purchase{Kind: "buy_card", ID: base.Card.ID}, base.Value())
		bought := s.ledger()
		paths := reversiblePaths(tr)
		for _, edge := range tr.Edges {
			path, ok := paths[edge.From]
			if !ok {
				t.Fatalf("%s: %s is not reachable from the base by reversible connections; extend this test", tr.Name, edge.From)
			}
			for _, step := range path {
				s.trade(tr, step.from, step.to, 1)
			}
			s.trade(tr, edge.From, edge.To, 1)
			if !edge.Reversible {
				// A one-way connection has no trade down; selling the copy
				// refunds everything it cost along the way.
				to, _ := tr.Node(edge.To)
				tag := ""
				if to.Shared {
					tag = tr.ID
				}
				s.do(loadout.Purchase{Kind: "sell_card", ID: to.Card.ID, Tree: tag}, to.Value())
				s.expect(before, tr.Name+" "+edge.ID+" sold")
				s.do(loadout.Purchase{Kind: "buy_card", ID: base.Card.ID}, base.Value())
				continue
			}
			s.trade(tr, edge.To, edge.From, 1)
			for i := len(path) - 1; i >= 0; i-- {
				s.trade(tr, path[i].to, path[i].from, 1)
			}
			s.expect(bought, tr.Name+" "+edge.ID)
		}
		s.do(loadout.Purchase{Kind: "sell_card", ID: base.Card.ID}, base.Value())
		s.expect(before, tr.Name+" sold back")
	}

	// Batched trades: every copy of a held card up one connection and back.
	batches := 0
	for _, entry := range slices.Clone(s.save.Sheet.Deck) {
		if entry.Count < 2 {
			continue
		}
		for _, ref := range content.TreeCardPlacements(lib.CardTrees, entry.CardID) {
			if ref.Shared && ref.Tree != entry.Tree {
				continue
			}
			tr := lib.CardTrees[ref.Tree]
			for _, edge := range tr.Edges {
				if !edge.Reversible || edge.From != ref.Node && edge.To != ref.Node {
					continue
				}
				other := edge.To
				if other == ref.Node {
					other = edge.From
				}
				before := s.ledger()
				s.trade(tr, ref.Node, other, entry.Count)
				s.trade(tr, other, ref.Node, entry.Count)
				s.expect(before, "batch "+entry.CardID)
				batches++
				break
			}
		}
	}
	if batches == 0 {
		t.Fatal("no batched trade was tried")
	}

	// Storing a copy and equipping it again keeps every figure.
	first := s.save.Sheet.Deck[0]
	before := s.ledger()
	s.do(loadout.Purchase{Kind: "unequip_collection_card", ID: first.CardID, Tree: first.Tree}, 0)
	s.do(loadout.Purchase{Kind: "equip_collection_card", ID: first.CardID, Tree: first.Tree}, 0)
	s.expect(before, "store and equip "+first.CardID)

	// Every equipped ability with an upgrade path: up the whole chain, then down.
	abilities := 0
	for _, id := range append(slices.Clone(s.save.Sheet.Abilities.Offensive), s.save.Sheet.Abilities.Defensive...) {
		if _, ok := abilityPaths[id]; !ok {
			continue
		}
		before := s.ledger()
		chain := []string{id}
		for u, ok := abilityPaths[id]; ok && !slices.Contains(chain, u.To); u, ok = abilityPaths[u.To] {
			s.do(loadout.Purchase{Kind: "upgrade_ability", ID: chain[len(chain)-1]}, u.XP)
			chain = append(chain, u.To)
		}
		for i := len(chain) - 1; i > 0; i-- {
			s.do(loadout.Purchase{Kind: "downgrade_ability", ID: chain[i], TargetID: chain[i-1]}, abilityPaths[chain[i-1]].XP)
		}
		s.expect(before, "ability "+id)
		abilities++
	}
	if abilities == 0 && len(abilityPaths) > 0 && !loadout.HoldsBoard(s.save.Sheet) {
		t.Fatal("no ability upgrade path was tried")
	}

	s.expect(start, "after every round trip")
	if !reflect.DeepEqual(s.owned(), owned) {
		t.Fatalf("the round trips changed the cards owned: %v, was %v", s.owned(), owned)
	}
	t.Logf("%d transactions, %d batched trades, %d ability chains round-tripped exactly", s.done, batches, abilities)
}

// roundTripBoard raises the custom adventurer's damage level step by step and
// swaps its defense through every menu defense, then undoes it all: each
// change moves XP by exactly its shown cost, and XP, Budget and UpgradeSpent
// end as they began.
func roundTripBoard(t *testing.T, contentRoot, root, saveID, character string) {
	t.Helper()
	if !loadout.HoldsBoard(readSave(t, contentRoot, root, saveID).Sheet) {
		return
	}
	ledger := func() xpLedger {
		p := readSave(t, contentRoot, root, saveID).Sheet
		return xpLedger{p.XP, *p.Budget, p.UpgradeSpent}
	}
	start := ledger()
	change := func(choice map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(choice)
		c, err := decodeBoardChoice(raw)
		if err != nil {
			t.Fatal(err)
		}
		view, err := campaignBoardView(contentRoot, root, saveID, c)
		if err != nil {
			t.Fatal(err)
		}
		xp := view["xp"].(map[string]any)
		before := ledger()
		if _, err = campaignBoardChange(contentRoot, root, saveID, view["choice"].(boardprice.Choice), view["revision"].(int), xp["cost"].(int)); err != nil {
			t.Fatalf("board %v: %v", choice, err)
		}
		if got := ledger(); got.XP != before.XP-xp["cost"].(int) || got.Budget != before.Budget || got.UpgradeSpent != before.UpgradeSpent {
			t.Fatalf("board %v moved the ledger %+v → %+v, cost %v", choice, before, got, xp["cost"])
		}
	}
	view, err := campaignBoardView(contentRoot, root, saveID, boardprice.Choice{})
	if err != nil {
		t.Fatal(err)
	}
	saved := view["choice"].(boardprice.Choice)
	steps := view["staircase"].(map[string]any)["steps"].([]map[string]any)
	lowest := view["staircase"].(map[string]any)["lowest"].(int)
	base := map[string]any{"die": saved.Die, "template": saved.Template, "slots": saved.Slots}
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	for k := lowest + 1; k <= lowest+3; k++ {
		change(with(map[string]any{"level": steps[k]["at"]}))
	}
	for _, d := range []string{"none", "light_guard", "firm_guard", "clean_deflect", "twin_deflect", "holdfast_plus"} {
		change(with(map[string]any{"level": steps[lowest+3]["at"], "defenses": []string{d}}))
	}
	change(with(map[string]any{"defenses": saved.Defenses}))
	if got := ledger(); got != start {
		t.Fatalf("the board round trip ended at %+v, began at %+v", got, start)
	}
}
