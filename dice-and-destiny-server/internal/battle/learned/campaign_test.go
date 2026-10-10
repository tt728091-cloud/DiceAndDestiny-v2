package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
)

// playCampaignBattle drives the started battle to its end with simple human play.
func playCampaignBattle(t *testing.T, s *Session) map[string]any {
	t.Helper()
	var view map[string]any
	var err error
	for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal && s.current.TruncationReason == ""; step++ {
		if s.isModelSeat(s.current.ActorID) {
			view, err = s.AdvanceModel()
		} else {
			alias := aliasValue(commandMap(t, venomTestAction(s.current.Result.LegalActions)), s.aliases(false))
			encoded, _ := json.Marshal(alias)
			view, err = s.SubmitHuman(string(encoded))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !s.current.Terminal {
		t.Fatalf("campaign battle did not finish: %s", s.current.TruncationReason)
	}
	return view
}

func readCampaignProgress(t *testing.T, contentRoot, loadoutRoot string) loadout.Progress {
	t.Helper()
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(loadoutRoot, "adventurer", c.economy, c.catalogs["adventurer"])
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCampaignAwardsVictoryXPAndAdvancesEncounters(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := t.TempDir()
	s, err := NewSession(SessionConfig{ContentRoot: contentRoot, LoadoutRoot: loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounters := c.campaign.Encounters
	if len(encounters) != 3 {
		t.Fatalf("expected three encounters, got %d", len(encounters))
	}
	if _, err = s.ResetCampaignEncounter("wrong", 1, "adventurer", encounters[1].ID); err == nil || !strings.Contains(err.Error(), encounters[0].Name) {
		t.Fatalf("a later encounter started out of order: %v", err)
	}
	if _, err = s.ResetCampaignEncounter("venom", 1, "venom", encounters[0].ID); err == nil {
		t.Fatal("a Venom character entered the General campaign")
	}

	victories, defeats := 0, 0
	for seed := uint64(1); seed <= 200 && (victories < len(encounters) || defeats == 0); seed++ {
		before := readCampaignProgress(t, contentRoot, loadoutRoot)
		next := c.campaign.NextEncounter(before)
		view, err := s.ResetCampaignEncounter(fmt.Sprintf("campaign-%d", seed), seed, "adventurer", next.ID)
		if err != nil {
			t.Fatal(err)
		}
		health := 0
		for _, entry := range before.Deck {
			health += entry.Count
		}
		if got := s.current.Result.Snapshot.Actors[s.humanSeat].MaxHealth; got != health {
			t.Fatalf("campaign battle used a %d-card deck, not the %d-card progression deck", got, health)
		}
		if object(view["campaign"])["outcome"] != nil {
			t.Fatal("a new battle carried an outcome")
		}
		view = playCampaignBattle(t, s)
		after := readCampaignProgress(t, contentRoot, loadoutRoot)
		outcome := s.campaign.outcome
		if outcome == nil || object(view["campaign"])["outcome"] == nil {
			t.Fatalf("battle end did not record the campaign: %+v", s.campaign)
		}
		if outcome.Result != view["battle_result"] || outcome.XP != after.XP || outcome.EncounterID != next.ID {
			t.Fatalf("outcome disagrees with authority: %+v view %v", outcome, view["battle_result"])
		}
		switch outcome.Result {
		case "victory":
			victories++
			was := loadout.CampaignProgress{}
			if before.Campaign != nil {
				was = *before.Campaign
			}
			wantNext := (was.Next + 1) % len(encounters)
			if after.XP != before.XP+next.XP || outcome.XPAwarded != next.XP || after.EarnedXP != before.EarnedXP+next.XP || *after.Budget != *before.Budget+next.XP || after.Campaign.Next != wantNext {
				t.Fatalf("victory reward wrong: before %+v after %+v", before, after)
			}
			if outcome.RunCompleted != (wantNext == 0) || (wantNext == 0 && after.Campaign.RunsCompleted != was.RunsCompleted+1) {
				t.Fatalf("run completion wrong: %+v %+v", outcome, after.Campaign)
			}
		default:
			if outcome.Result == "defeat" {
				defeats++
			}
			if after.XP != before.XP || outcome.XPAwarded != 0 || c.campaign.NextEncounter(after).ID != next.ID {
				t.Fatalf("%s changed XP or the encounter: before %+v after %+v", outcome.Result, before, after)
			}
		}
		// Spend the first reward between battles: the next battle fights with it.
		if outcome.Result == "victory" && victories == 1 {
			price := c.economy.Price("adventurer", "take_stock")
			if _, err = loadout.Buy(loadoutRoot, "adventurer", c.economy, c.catalogs["adventurer"], loadout.Purchase{Kind: "buy_card", ID: "take_stock", Revision: after.Revision, ExpectedCost: price}); err != nil {
				t.Fatal(err)
			}
			if bought := readCampaignProgress(t, contentRoot, loadoutRoot); bought.XP != after.XP-price {
				t.Fatal("purchase did not spend earned XP")
			}
			after = readCampaignProgress(t, contentRoot, loadoutRoot)
		}
		// Re-recording the same battle is refused and changes nothing.
		s.mu.Lock()
		s.recordCampaignBattle()
		s.mu.Unlock()
		if _, err = loadout.RecordCampaignBattle(loadoutRoot, "adventurer", c.economy, c.catalogs["adventurer"], c.campaign, next.ID, outcome.BattleID, "victory"); err == nil {
			t.Fatal("a battle was rewarded twice")
		}
		if again := readCampaignProgress(t, contentRoot, loadoutRoot); again.XP != after.XP || again.Revision != after.Revision {
			t.Fatal("re-recording changed the ledger")
		}
	}
	if victories < len(encounters) || defeats == 0 {
		t.Fatalf("missing coverage: %d victories, %d defeats", victories, defeats)
	}

	// An ordinary progression battle afterwards earns nothing.
	before := readCampaignProgress(t, contentRoot, loadoutRoot)
	if _, err = s.ResetCharacterLoadout("plain", 7, "seat-a", false, "adventurer", true, "progression"); err != nil {
		t.Fatal(err)
	}
	if view := playCampaignBattle(t, s); view["campaign"] != nil || s.campaign != nil {
		t.Fatal("a non-campaign battle was treated as a campaign battle")
	}
	if after := readCampaignProgress(t, contentRoot, loadoutRoot); after.XP != before.XP {
		t.Fatal("a non-campaign battle changed XP")
	}

	status, err := campaignStatus(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	adventurer := object(status["characters"])["adventurer"].(map[string]any)
	if adventurer["xp"] != before.XP || adventurer["campaign"].(loadout.CampaignProgress).Victories != victories {
		t.Fatalf("campaign status disagrees with the ledger: %+v", adventurer)
	}
}

// Rewards earned after an admin budget override still count; the override
// only fixes the total at the time it was saved.
func TestCampaignXPSurvivesAdminBudgetOverride(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := t.TempDir()
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	lib := c.catalogs["adventurer"]
	first := c.campaign.Encounters[0]
	if _, err = loadout.RecordCampaignBattle(loadoutRoot, "adventurer", c.economy, lib, c.campaign, first.ID, "b1", "victory"); err != nil {
		t.Fatal(err)
	}
	all, _, admin, err := loadout.ProgressSnapshot(loadoutRoot, c.economy, c.catalogs)
	if err != nil {
		t.Fatal(err)
	}
	budget := *all["adventurer"].Budget + 50
	admin.Budgets = map[string]int{"adventurer": budget}
	if err = loadout.SaveAdmin(loadoutRoot, c.economy, c.catalogs, admin); err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(loadoutRoot, "adventurer", c.economy, lib)
	if err != nil || *p.Budget != budget {
		t.Fatalf("override not applied: %v %+v", err, p)
	}
	second := c.campaign.Encounters[1]
	if _, err = loadout.RecordCampaignBattle(loadoutRoot, "adventurer", c.economy, lib, c.campaign, second.ID, "b2", "victory"); err != nil {
		t.Fatal(err)
	}
	after, err := loadout.ReadProgress(loadoutRoot, "adventurer", c.economy, lib)
	if err != nil || after.XP != p.XP+second.XP || *after.Budget != budget+second.XP {
		t.Fatalf("reward lost to the admin override: %v before %+v after %+v", err, p, after)
	}
}
