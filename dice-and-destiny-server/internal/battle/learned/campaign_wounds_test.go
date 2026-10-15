package learned

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// firstUpgrade is a tree's first reversible connection from its base that
// costs XP, and its price.
func firstUpgrade(t *testing.T, tr content.CardTree) (content.CardTreeNode, int) {
	t.Helper()
	base, _ := tr.Node(tr.Root)
	for _, e := range tr.Edges {
		if e.From == tr.Root && e.Reversible {
			if to, _ := tr.Node(e.To); !to.Shared && to.Value() > base.Value() {
				return to, to.Value() - base.Value()
			}
		}
	}
	t.Fatalf("%s has no reversible upgrade from its base", tr.Name)
	return content.CardTreeNode{}, 0
}

func woundCounts(p loadout.Progress) map[string]int {
	out := map[string]int{}
	for _, e := range p.Wounds {
		out[e.CardID+"|"+e.Tree] += e.Count
	}
	return out
}

// woundInSave stands in for a battle hit that removed cards as one wound:
// the copies leave the map piles and the sheet's deck for its wounds.
func woundInSave(s *loadout.CampaignSave, ids ...string) {
	st := s.Expedition
	for _, id := range ids {
		for _, pile := range []*[]string{&st.Spent, &st.Draw, &st.Hand} {
			if i := slices.Index(*pile, id); i >= 0 {
				*pile = append((*pile)[:i:i], (*pile)[i+1:]...)
				break
			}
		}
		for i := range s.Sheet.Deck {
			if s.Sheet.Deck[i].CardID == id {
				s.Sheet.Deck[i].Count--
				if s.Sheet.Deck[i].Count == 0 {
					s.Sheet.Deck = append(s.Sheet.Deck[:i:i], s.Sheet.Deck[i+1:]...)
				}
				break
			}
		}
		s.Sheet.Wounds = append(s.Sheet.Wounds, loadout.Entry{CardID: id, Count: 1})
	}
	st.WoundSeq++
	st.Wounds = append(st.Wounds, expedition.Wound{ID: fmt.Sprintf("w%d", st.WoundSeq), Cards: append([]string{}, ids...), Size: len(ids), Source: "a test hit", Round: st.Round, Turn: st.Turn.Number})
}

// Wounded copies are frozen in both campaign modes (Phase 0, task 0.4): with
// wounded and live copies of the same card, every kind of trade works on the
// live copies and leaves the wounds alone; once only wounded copies remain,
// every kind is refused without changing XP, Budget or the wounds; and a
// reload keeps the wounds identical.
func TestWoundedCopiesAreFrozenInBothModes(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	root := campaignLoadoutRoot(t)
	x, err := loadExpeditionContext(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	lib := x.catalogs["starter"]
	treeOf := func(card string) content.CardTree {
		refs := content.TreeCardPlacements(lib.CardTrees, card)
		if len(refs) == 0 {
			t.Fatalf("%s is in no tree", card)
		}
		return lib.CardTrees[refs[0].Tree]
	}
	guardTree, nudgeTree := treeOf("steady_guard"), treeOf("nudge")
	guardUp, guardCost := firstUpgrade(t, guardTree)
	nudgeUp, nudgeCost := firstUpgrade(t, nudgeTree)
	upgraded := nudgeUp.Card.ID

	t.Run("linear", func(t *testing.T) {
		save := startCampaign(t, contentRoot, root, "starter", "Frozen wounds")
		trade := func(r loadout.Purchase) (loadout.CampaignSave, error) {
			return loadout.BuyCampaign(root, save.ID, x.economy, x.catalogs, r)
		}
		reload := func() loadout.CampaignSave { return readSave(t, contentRoot, root, save.ID) }
		// Upgrade one Nudge, then win a battle that removes one Steady Guard
		// and the upgraded copy: both become wounds through the real recorder.
		if _, err := trade(loadout.Purchase{Kind: "tree_card_deck", ID: "nudge", TargetID: upgraded, Revision: save.Sheet.Revision, ExpectedCost: nudgeCost}); err != nil {
			t.Fatal(err)
		}
		encounter := x.campaign.NextEncounter(reload().Sheet)
		outcome, err := loadout.RecordCampaignBattle(root, save.ID, x.economy, x.catalogs, x.campaign, encounter.ID, "frozen-1", "victory", map[string]int{"steady_guard": 1, upgraded: 1})
		if err != nil || outcome.Wounds != 2 {
			t.Fatalf("the victory leaves two wounds: %+v (%v)", outcome, err)
		}
		checkFrozenWounds(t, trade, reload, guardUp.Card.ID, guardCost, upgraded, nudgeCost, func(loadout.CampaignSave) {})
	})

	t.Run("expedition", func(t *testing.T) {
		rules := x.rules["starter"]
		save, err := newExpeditionCampaign(contentRoot, root, "starter", "Frozen wounds", "", 5)
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
		trade := func(r loadout.Purchase) (loadout.CampaignSave, error) {
			return loadout.ExpeditionDeckChange(root, save.ID, x.economy, x.catalogs, rules, r)
		}
		reload := func() loadout.CampaignSave {
			s, err := loadout.ReadExpeditionSave(root, save.ID, x.economy, x.catalogs)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
		// At camp with the deck opened (Influence paid in cards), upgrade one
		// Nudge; then a hit removes one Steady Guard and the upgraded copy.
		camp := act(expedition.Action{Kind: "make_camp"})
		fee := rules.Config.DeckChangeInfluence["camp"]
		paid := act(expedition.Action{Kind: "camp_service", Service: "influence", Cards: camp.Expedition.Hand[:fee]})
		if _, err := trade(loadout.Purchase{Kind: "tree_card_deck", ID: "nudge", TargetID: upgraded, Revision: paid.Sheet.Revision, ExpectedCost: nudgeCost}); err != nil {
			t.Fatal(err)
		}
		editSave(t, root, save.ID, func(s *loadout.CampaignSave) { woundInSave(s, "steady_guard", upgraded) })
		mapWounds := reload().Expedition.Wounds
		if len(mapWounds) != 1 || len(mapWounds[0].Cards) != 2 {
			t.Fatalf("the map holds the hit as one wound of two cards: %+v", mapWounds)
		}
		checkFrozenWounds(t, trade, reload, guardUp.Card.ID, guardCost, upgraded, nudgeCost, func(s loadout.CampaignSave) {
			t.Helper()
			if !reflect.DeepEqual(s.Expedition.Wounds, mapWounds) {
				t.Fatalf("the map's wounds changed: %+v, was %+v", s.Expedition.Wounds, mapWounds)
			}
		})
	})
}

// checkFrozenWounds runs every kind of trade against a save holding two live
// and one wounded Steady Guard and one wounded copy of an upgraded Nudge.
func checkFrozenWounds(t *testing.T, trade func(loadout.Purchase) (loadout.CampaignSave, error), reload func() loadout.CampaignSave, guardUp string, guardCost int, upgraded string, nudgeCost int, mapCheck func(loadout.CampaignSave)) {
	t.Helper()
	start := reload()
	wounds := woundCounts(start.Sheet)
	if wounds["steady_guard|"] != 1 || wounds[upgraded+"|"] != 1 || deckCount(start.Sheet.Deck, "steady_guard") != 2 || deckCount(start.Sheet.Deck, upgraded) != 0 {
		t.Fatalf("setup: wounds %v, deck %v", wounds, start.Sheet.Deck)
	}
	same := func(s loadout.CampaignSave, what string) {
		t.Helper()
		if got := woundCounts(s.Sheet); !reflect.DeepEqual(got, wounds) {
			t.Fatalf("%s changed the wounds: %v, was %v", what, got, wounds)
		}
		mapCheck(s)
	}
	ok := func(r loadout.Purchase, cost int) {
		t.Helper()
		r.Revision, r.ExpectedCost = reload().Sheet.Revision, cost
		next, err := trade(r)
		if err != nil {
			t.Fatalf("%s %s → %s ×%d on live copies: %v", r.Kind, r.ID, r.TargetID, max(1, r.Count), err)
		}
		same(next, r.Kind+" "+r.ID)
	}
	refused := func(r loadout.Purchase, cost int, want string) {
		t.Helper()
		before := reload()
		r.Revision, r.ExpectedCost = before.Sheet.Revision, cost
		if _, err := trade(r); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s %s → %s ×%d: expected a refusal containing %q, got %v", r.Kind, r.ID, r.TargetID, max(1, r.Count), want, err)
		}
		after := reload()
		if after.Sheet.XP != before.Sheet.XP || *after.Sheet.Budget != *before.Sheet.Budget || !reflect.DeepEqual(after.Sheet, before.Sheet) {
			t.Fatalf("a refused %s changed the sheet: xp %d → %d, budget %d → %d", r.Kind, before.Sheet.XP, after.Sheet.XP, *before.Sheet.Budget, *after.Sheet.Budget)
		}
		same(after, "a refused "+r.Kind)
	}

	// Live copies trade freely: upgrade, trade down, batches, store, sell.
	ok(loadout.Purchase{Kind: "tree_card_deck", ID: "steady_guard", TargetID: guardUp}, guardCost)
	ok(loadout.Purchase{Kind: "tree_card_deck", ID: guardUp, TargetID: "steady_guard"}, -guardCost)
	ok(loadout.Purchase{Kind: "tree_card_deck", ID: "steady_guard", TargetID: guardUp, Count: 2}, 2*guardCost)
	ok(loadout.Purchase{Kind: "tree_card_deck", ID: guardUp, TargetID: "steady_guard", Count: 2}, -2*guardCost)
	refused(loadout.Purchase{Kind: "tree_card_deck", ID: "steady_guard", TargetID: guardUp, Count: 3}, 3*guardCost, "Steady Guard has 2 live copies")
	ok(loadout.Purchase{Kind: "unequip_collection_card", ID: "steady_guard"}, 0)
	ok(loadout.Purchase{Kind: "equip_collection_card", ID: "steady_guard"}, 0)
	ok(loadout.Purchase{Kind: "sell_card", ID: "steady_guard"}, 10)
	ok(loadout.Purchase{Kind: "sell_card", ID: "steady_guard"}, 10)

	// Only wounded copies are left: every kind is refused.
	for _, r := range []loadout.Purchase{
		{Kind: "sell_card", ID: "steady_guard"},
		{Kind: "tree_card_deck", ID: "steady_guard", TargetID: guardUp},
		{Kind: "tree_card", ID: "steady_guard", TargetID: guardUp},
		{Kind: "unequip_collection_card", ID: "steady_guard"},
		{Kind: "upgrade_card", ID: "steady_guard", TargetID: guardUp},
		{Kind: "tree_card_deck", ID: upgraded, TargetID: "nudge"},
		{Kind: "sell_card", ID: upgraded},
		{Kind: "unequip_collection_card", ID: upgraded},
	} {
		refused(r, nudgeCost, "until mended")
	}

	// A reload keeps the wounds identical.
	same(reload(), "reloading")
}

func deckCount(deck []loadout.Entry, id string) int {
	n := 0
	for _, e := range deck {
		if e.CardID == id {
			n += e.Count
		}
	}
	return n
}
