package learned

import (
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
)

// totalXP follows one save's total XP (the sheet's Budget) through campaign
// play: it must never drop, and the status the client reads must report it.
type totalXP struct {
	t        *testing.T
	last     int
	reported func() int
}

func (x *totalXP) check(save loadout.CampaignSave, want int, what string) {
	x.t.Helper()
	got := *save.Sheet.Budget
	if got < x.last {
		x.t.Fatalf("%s: total XP dropped from %d to %d", what, x.last, got)
	}
	if got != want {
		x.t.Fatalf("%s: total XP %d, want %d", what, got, want)
	}
	if p := save.Sheet; p.XP+p.DeckValue+p.CollectionValue+p.WoundValue+p.UpgradeSpent != got {
		x.t.Fatalf("%s: the ledger does not add up to the total: %+v", what, p)
	}
	if reported := x.reported(); reported != got {
		x.t.Fatalf("%s: the status reports xp_total %d, the save holds %d", what, reported, got)
	}
	x.last = got
}

// Total XP (Phase 0, task 0.3): rewards raise it; spending, selling and wounds
// move XP inside it; reloading keeps it; the status sends it as xp_total.
func TestTotalXPNeverDropsAndIsReported(t *testing.T) {
	t.Run("expedition", func(t *testing.T) {
		f := newExpeditionFixture(t, 1, "Total XP")
		rules := f.x.rules["starter"]
		x := &totalXP{t: t, reported: func() int {
			view, err := expeditionStatus(f.contentRoot, f.loadoutRoot, f.saveID)
			if err != nil {
				t.Fatal(err)
			}
			return view["xp_total"].(int)
		}}
		start := f.save(t)
		total := *start.Sheet.Budget
		x.check(start, total, "a new expedition")

		// Credit XP: win the pending challenge. The battle (scripted here; the
		// session reports the same shape) removes two Spent cards as one wound.
		pending := f.walkToChallenge(t)
		x.check(pending, total, "walking and challenging")
		pb, st := pending.Expedition.PendingBattle, pending.Expedition
		removed := append([]string{}, st.Spent[:2]...)
		outcome, err := loadout.RecordExpeditionBattle(f.loadoutRoot, f.saveID, f.x.economy, f.x.catalogs, rules, expedition.BattleEnd{
			BattleID: pb.ID, Result: "victory", Hand: st.Hand, Draw: st.Draw, Spent: st.Spent[2:], Removed: removed, Energy: pb.EnergyIn,
			Wounds: []expedition.BattleWound{{Round: 1, Source: "test", Cards: removed}},
		})
		if err != nil {
			t.Fatal(err)
		}
		won := f.save(t)
		if outcome.XP <= 0 || len(won.Sheet.Wounds) == 0 || won.Sheet.WoundValue == 0 {
			t.Fatalf("the victory credits XP and leaves wounds: %+v, wounds %v", outcome, won.Sheet.Wounds)
		}
		total += outcome.XP
		x.check(won, total, "a victory with wounds")

		// Spend and sell at camp: XP moves inside the total.
		if won.Expedition.Phase != expedition.PhaseCamp {
			won = f.act(t, expedition.Action{Kind: "make_camp"})
		}
		fee := rules.Config.DeckChangeInfluence["camp"]
		f.act(t, expedition.Action{Kind: "camp_service", Service: "influence", Cards: won.Expedition.Hand[:fee]})
		change := func(r loadout.Purchase) loadout.CampaignSave {
			t.Helper()
			r.Revision = f.save(t).Sheet.Revision
			save, err := loadout.ExpeditionDeckChange(f.loadoutRoot, f.saveID, f.x.economy, f.x.catalogs, rules, r)
			if err != nil {
				t.Fatalf("%s %s: %v", r.Kind, r.ID, err)
			}
			return save
		}
		// Card prices are the published cards' (balance work moves them), so
		// the expected costs are read, not hard-coded.
		price := func(card string) int { return f.x.economy.Price(campaignCharacter, card) }
		bought := change(loadout.Purchase{Kind: "buy_card", ID: "nudge", ExpectedCost: price("nudge")})
		x.check(bought, total, "buying a card")
		upgraded := change(loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", ExpectedCost: 38})
		x.check(upgraded, total, "upgrading an ability")
		sold := change(loadout.Purchase{Kind: "sell_card", ID: "try_again", ExpectedCost: price("try_again")})
		x.check(sold, total, "selling a card")
		x.check(f.save(t), total, "reloading the save")
	})

	t.Run("linear", func(t *testing.T) {
		contentRoot := filepath.Join(testServerRoot(t), "content")
		root := campaignLoadoutRoot(t)
		c, err := loadCampaignContext(contentRoot, root)
		if err != nil {
			t.Fatal(err)
		}
		save := startCampaign(t, contentRoot, root, campaignCharacter, "Total XP")
		x := &totalXP{t: t, reported: func() int {
			status, err := campaignStatus(contentRoot, root)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range status["saves"].([]map[string]any) {
				if s["id"] == save.ID {
					return s["xp_total"].(int)
				}
			}
			t.Fatal("the save is missing from campaign_status")
			return 0
		}}
		total := *save.Sheet.Budget
		x.check(save, total, "a new campaign")

		// Credit XP and take wounds: a victory whose removed pile holds two cards.
		encounter := c.campaign.NextEncounter(save.Sheet)
		outcome, err := loadout.RecordCampaignBattle(root, save.ID, c.economy, c.catalogs, c.campaign, encounter.ID, "total-xp-1", "victory", map[string]int{"steady_guard": 1, "nudge": 1})
		if err != nil {
			t.Fatal(err)
		}
		won := readSave(t, contentRoot, root, save.ID)
		if outcome.XPAwarded != encounter.XP || outcome.Wounds != 2 {
			t.Fatalf("the victory credits %d XP and 2 wounds: %+v", encounter.XP, outcome)
		}
		total += encounter.XP
		x.check(won, total, "a victory with wounds")

		buy := func(r loadout.Purchase) loadout.CampaignSave {
			t.Helper()
			r.Revision = readSave(t, contentRoot, root, save.ID).Sheet.Revision
			next, err := campaignPurchase(contentRoot, root, save.ID, r)
			if err != nil {
				t.Fatalf("%s %s: %v", r.Kind, r.ID, err)
			}
			return next
		}
		price := func(card string) int { return c.economy.Price(campaignCharacter, card) }
		x.check(buy(loadout.Purchase{Kind: "buy_card", ID: "nudge", ExpectedCost: price("nudge")}), total, "buying a card")
		x.check(buy(loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", ExpectedCost: 38}), total, "upgrading an ability")
		x.check(buy(loadout.Purchase{Kind: "sell_card", ID: "try_again", ExpectedCost: price("try_again")}), total, "selling a card")
		x.check(readSave(t, contentRoot, root, save.ID), total, "reloading the save")
	})
}
