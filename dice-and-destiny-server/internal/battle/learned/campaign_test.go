package learned

import (
	"encoding/json"
	"fmt"
	"os"
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

const campaignCharacter = "starter"

// campaignLoadoutRoot copies the published card trees into a disposable root:
// campaign decks may use only card-tree cards.
func campaignLoadoutRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"authored_cards.json", "economy_admin.json"} {
		raw, err := os.ReadFile(filepath.Join(testServerRoot(t), "content", "authored", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func readCampaignProgress(t *testing.T, contentRoot, loadoutRoot string) loadout.Progress {
	t.Helper()
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(loadoutRoot, campaignCharacter, c.economy, c.catalogs[campaignCharacter])
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCampaignAwardsVictoryXPAndAdvancesEncounters(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounters := c.campaign.Encounters
	if len(encounters) != 3 || c.campaign.Characters[0] != campaignCharacter || !c.campaign.Allows("adventurer") {
		t.Fatalf("expected three encounters for Starter and Adventurer: %+v", c.campaign)
	}
	// The runtime pins one opponent per session, as the client does per encounter.
	sessions := map[string]*Session{}
	session := func(e loadout.Encounter) *Session {
		if sessions[e.Opponent] == nil {
			s, err := NewSession(SessionConfig{ContentRoot: contentRoot, LoadoutRoot: loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: e.Opponent, OpponentCount: e.OpponentCount})
			if err != nil {
				t.Fatal(err)
			}
			sessions[e.Opponent] = s
		}
		return sessions[e.Opponent]
	}
	first := session(encounters[0])
	if _, err = session(encounters[1]).ResetCampaignEncounter("wrong", 1, campaignCharacter, encounters[1].ID); err == nil || !strings.Contains(err.Error(), encounters[0].Name) {
		t.Fatalf("a later encounter started out of order: %v", err)
	}
	if encounters[1].Opponent != encounters[0].Opponent {
		if _, err = session(encounters[1]).ResetCampaignEncounter("mismatch", 1, campaignCharacter, encounters[0].ID); err == nil {
			t.Fatal("an encounter started against the wrong opponent")
		}
	}
	if _, err = first.ResetCampaignEncounter("venom", 1, "venom", encounters[0].ID); err == nil {
		t.Fatal("a Venom character entered the General campaign")
	}
	opponents := map[string]bool{}

	// Campaign decks use only card-tree cards. A legacy card is refused by the
	// campaign editors, blocks a campaign battle when equipped elsewhere, and
	// can still be sold.
	lib := c.catalogs[campaignCharacter]
	if !loadout.IsTreeCard(lib.CardTrees, "take_stock") || loadout.IsTreeCard(lib.CardTrees, "tip_it") {
		t.Fatal("expected Take Stock in a tree and Tip It outside every tree")
	}
	p := readCampaignProgress(t, contentRoot, loadoutRoot)
	if bad := loadout.NonTreeCards(p.Deck, lib.CardTrees); len(bad) > 0 {
		t.Fatalf("the Starter's deck has non-tree cards: %v", bad)
	}
	tip := loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: c.economy.Price(campaignCharacter, "tip_it"), TreeCardsOnly: true}
	if _, err = loadout.Buy(loadoutRoot, campaignCharacter, c.economy, lib, tip); err == nil || !strings.Contains(err.Error(), "card-tree") {
		t.Fatalf("the campaign editors bought a legacy card: %v", err)
	}
	tip.TreeCardsOnly = false
	if p, err = loadout.Buy(loadoutRoot, campaignCharacter, c.economy, lib, tip); err != nil {
		t.Fatal(err)
	}
	if _, err = first.ResetCampaignEncounter("legacy", 1, campaignCharacter, encounters[0].ID); err == nil || !strings.Contains(err.Error(), "Tip It") {
		t.Fatalf("a campaign battle started with a legacy card: %v", err)
	}
	status, err := campaignStatus(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	if conflicts := object(status["characters"])[campaignCharacter].(map[string]any)["tree_conflicts"].([]string); len(conflicts) != 1 || conflicts[0] != "Tip It" {
		t.Fatalf("campaign status did not flag the legacy card: %v", conflicts)
	}
	if _, err = loadout.Buy(loadoutRoot, campaignCharacter, c.economy, lib, loadout.Purchase{Kind: "sell_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: tip.ExpectedCost, TreeCardsOnly: true}); err != nil {
		t.Fatalf("the campaign could not sell a legacy card: %v", err)
	}

	victories, defeats := 0, 0
	for seed := uint64(1); seed <= 200 && (victories < len(encounters) || defeats == 0); seed++ {
		before := readCampaignProgress(t, contentRoot, loadoutRoot)
		next := c.campaign.NextEncounter(before)
		s := session(next)
		view, err := s.ResetCampaignEncounter(fmt.Sprintf("campaign-%d", seed), seed, campaignCharacter, next.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.current.Result.Snapshot.Actors[s.modelSeat].DefinitionID; got != next.Opponent {
			t.Fatalf("encounter %s fought %s", next.ID, got)
		}
		opponents[next.Opponent] = true
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
			price := c.economy.Price(campaignCharacter, "take_stock")
			if _, err = loadout.Buy(loadoutRoot, campaignCharacter, c.economy, c.catalogs[campaignCharacter], loadout.Purchase{Kind: "buy_card", ID: "take_stock", Revision: after.Revision, ExpectedCost: price}); err != nil {
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
		if _, err = loadout.RecordCampaignBattle(loadoutRoot, campaignCharacter, c.economy, c.catalogs[campaignCharacter], c.campaign, next.ID, outcome.BattleID, "victory"); err == nil {
			t.Fatal("a battle was rewarded twice")
		}
		if again := readCampaignProgress(t, contentRoot, loadoutRoot); again.XP != after.XP || again.Revision != after.Revision {
			t.Fatal("re-recording changed the ledger")
		}
	}
	if victories < len(encounters) || defeats == 0 || len(opponents) != 3 {
		t.Fatalf("missing coverage: %d victories, %d defeats, opponents %v", victories, defeats, opponents)
	}
	s := first

	// An ordinary progression battle afterwards earns nothing.
	before := readCampaignProgress(t, contentRoot, loadoutRoot)
	if _, err = s.ResetCharacterLoadout("plain", 7, "seat-a", false, campaignCharacter, true, "progression"); err != nil {
		t.Fatal(err)
	}
	if view := playCampaignBattle(t, s); view["campaign"] != nil || s.campaign != nil {
		t.Fatal("a non-campaign battle was treated as a campaign battle")
	}
	if after := readCampaignProgress(t, contentRoot, loadoutRoot); after.XP != before.XP {
		t.Fatal("a non-campaign battle changed XP")
	}

	status, err = campaignStatus(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	starter := object(status["characters"])[campaignCharacter].(map[string]any)
	if starter["xp"] != before.XP || starter["campaign"].(loadout.CampaignProgress).Victories != victories {
		t.Fatalf("campaign status disagrees with the ledger: %+v", starter)
	}
	// Each character keeps its own place: the Adventurer has not started.
	if adventurer := object(status["characters"])["adventurer"].(map[string]any); adventurer["campaign"].(loadout.CampaignProgress).Victories != 0 {
		t.Fatalf("the Adventurer shared the Starter's campaign: %+v", adventurer)
	}
}

// Rewards earned after an admin budget override still count; the override
// only fixes the total at the time it was saved.
func TestCampaignXPSurvivesAdminBudgetOverride(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
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
