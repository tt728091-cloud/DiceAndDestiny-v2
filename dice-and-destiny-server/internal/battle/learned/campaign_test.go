package learned

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func startCampaign(t *testing.T, contentRoot, loadoutRoot, character, name string) loadout.CampaignSave {
	t.Helper()
	save, err := newCampaign(contentRoot, loadoutRoot, character, name)
	if err != nil {
		t.Fatal(err)
	}
	return save
}

func readSave(t *testing.T, contentRoot, loadoutRoot, id string) loadout.CampaignSave {
	t.Helper()
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	save, err := loadout.ReadCampaignSave(loadoutRoot, id, c.economy, c.catalogs)
	if err != nil {
		t.Fatal(err)
	}
	return save
}

// editSave rewrites a save file directly, standing in for state the client
// can no longer produce (for example a legacy card left in an old deck).
func editSave(t *testing.T, loadoutRoot, id string, edit func(*loadout.CampaignSave)) {
	t.Helper()
	filename := filepath.Join(loadoutRoot, "campaigns", id+".json")
	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var save loadout.CampaignSave
	if err = json.Unmarshal(raw, &save); err != nil {
		t.Fatal(err)
	}
	edit(&save)
	raw, _ = json.Marshal(save)
	if err = os.WriteFile(filename, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func campaignState(p loadout.Progress) loadout.CampaignProgress {
	if p.Campaign == nil {
		return loadout.CampaignProgress{}
	}
	return *p.Campaign
}

func TestCampaignAwardsVictoryXPAndAdvancesEncounters(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounters := c.campaign.Encounters
	if len(encounters) != 3 || c.campaign.Characters[0] != campaignCharacter || !c.campaign.Allows("adventurer") || c.campaign.BonusXP != 100 {
		t.Fatalf("expected three encounters for Starter and Adventurer with 100 bonus XP: %+v", c.campaign)
	}
	save := startCampaign(t, contentRoot, loadoutRoot, campaignCharacter, "Test run")
	if save.Sheet.XP != c.campaign.BonusXP {
		t.Fatalf("a new campaign starts with %d XP, not the %d bonus", save.Sheet.XP, c.campaign.BonusXP)
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
	if _, err = session(encounters[1]).ResetCampaignEncounter("wrong", 1, save.ID, encounters[1].ID); err == nil || !strings.Contains(err.Error(), encounters[0].Name) {
		t.Fatalf("a later encounter started out of order: %v", err)
	}
	if encounters[1].Opponent != encounters[0].Opponent {
		if _, err = session(encounters[1]).ResetCampaignEncounter("mismatch", 1, save.ID, encounters[0].ID); err == nil {
			t.Fatal("an encounter started against the wrong opponent")
		}
	}
	if _, err = newCampaign(contentRoot, loadoutRoot, "venom", "Venom run"); err == nil {
		t.Fatal("a Venom character started the General campaign")
	}

	// A legacy card left in a save blocks its battles, is named, cannot be
	// bought, and can be sold.
	lib := c.catalogs[campaignCharacter]
	if !loadout.IsTreeCard(lib.CardTrees, "take_stock") || loadout.IsTreeCard(lib.CardTrees, "tip_it") {
		t.Fatal("expected Take Stock in a tree and Tip It outside every tree")
	}
	editSave(t, loadoutRoot, save.ID, func(s *loadout.CampaignSave) {
		s.Sheet.Deck = append(s.Sheet.Deck, loadout.Entry{CardID: "tip_it", Count: 1})
	})
	if _, err = first.ResetCampaignEncounter("legacy", 1, save.ID, encounters[0].ID); err == nil || !strings.Contains(err.Error(), "Tip It") {
		t.Fatalf("a campaign battle started with a legacy card: %v", err)
	}
	status, err := campaignStatus(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	if conflicts := status["saves"].([]map[string]any)[0]["tree_conflicts"].([]string); len(conflicts) != 1 || conflicts[0] != "Tip It" {
		t.Fatalf("campaign status did not flag the legacy card: %v", conflicts)
	}
	p := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet
	tip := c.economy.Price(campaignCharacter, "tip_it")
	if _, err = campaignPurchase(contentRoot, loadoutRoot, save.ID, loadout.Purchase{Kind: "buy_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: tip}); err == nil || !strings.Contains(err.Error(), "card-tree") {
		t.Fatalf("the campaign bought a legacy card: %v", err)
	}
	if _, err = campaignPurchase(contentRoot, loadoutRoot, save.ID, loadout.Purchase{Kind: "sell_card", ID: "tip_it", Revision: p.Revision, ExpectedCost: tip}); err != nil {
		t.Fatalf("the campaign could not sell a legacy card: %v", err)
	}

	opponents := map[string]bool{}
	victories, defeats := 0, 0
	for seed := uint64(1); seed <= 200 && (victories < len(encounters) || defeats == 0); seed++ {
		before := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet
		next := c.campaign.NextEncounter(before)
		s := session(next)
		view, err := s.ResetCampaignEncounter(fmt.Sprintf("campaign-%d", seed), seed, save.ID, next.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.current.Result.Snapshot.Actors[s.modelSeat].DefinitionID; got != next.Opponent {
			t.Fatalf("encounter %s fought %s", next.ID, got)
		}
		opponents[next.Opponent] = true
		if got := s.current.Result.Snapshot.Actors[s.humanSeat].MaxHealth; got != deckHealth(before.Deck) {
			t.Fatalf("campaign battle used a %d-card deck, not the save's %d cards", got, deckHealth(before.Deck))
		}
		if object(view["campaign"])["outcome"] != nil || object(view["campaign"])["save_id"] != save.ID {
			t.Fatal("a new battle carried an outcome or the wrong save")
		}
		view = playCampaignBattle(t, s)
		after := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet
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
			wantNext := (campaignState(before).Next + 1) % len(encounters)
			if after.XP != before.XP+next.XP || outcome.XPAwarded != next.XP || after.EarnedXP != before.EarnedXP+next.XP || *after.Budget != *before.Budget+next.XP || after.Campaign.Next != wantNext {
				t.Fatalf("victory reward wrong: before %+v after %+v", before, after)
			}
			if outcome.RunCompleted != (wantNext == 0) || (wantNext == 0 && after.Campaign.RunsCompleted != campaignState(before).RunsCompleted+1) {
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
			if _, err = campaignPurchase(contentRoot, loadoutRoot, save.ID, loadout.Purchase{Kind: "buy_card", ID: "take_stock", Revision: after.Revision, ExpectedCost: price}); err != nil {
				t.Fatal(err)
			}
			if bought := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet; bought.XP != after.XP-price {
				t.Fatal("purchase did not spend earned XP")
			}
			after = readSave(t, contentRoot, loadoutRoot, save.ID).Sheet
		}
		// Re-recording the same battle is refused and changes nothing.
		s.mu.Lock()
		s.recordCampaignBattle()
		s.mu.Unlock()
		if _, err = loadout.RecordCampaignBattle(loadoutRoot, save.ID, c.economy, c.catalogs, c.campaign, next.ID, outcome.BattleID, "victory"); err == nil {
			t.Fatal("a battle was rewarded twice")
		}
		if again := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet; again.XP != after.XP || again.Revision != after.Revision {
			t.Fatal("re-recording changed the save")
		}
	}
	if victories < len(encounters) || defeats == 0 || len(opponents) != 3 {
		t.Fatalf("missing coverage: %d victories, %d defeats, opponents %v", victories, defeats, opponents)
	}

	// An ordinary progression battle afterwards earns nothing for the save.
	before := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet
	if _, err = first.ResetCharacterLoadout("plain", 7, "seat-a", false, campaignCharacter, true, "progression"); err != nil {
		t.Fatal(err)
	}
	if view := playCampaignBattle(t, first); view["campaign"] != nil || first.campaign != nil {
		t.Fatal("a non-campaign battle was treated as a campaign battle")
	}
	if after := readSave(t, contentRoot, loadoutRoot, save.ID).Sheet; !reflect.DeepEqual(after, before) {
		t.Fatal("a non-campaign battle changed the campaign save")
	}
}

// Each campaign is its own copy of the starting sheet: playing or trading in
// one never changes another, the editors' progression save, or the template.
func TestCampaignSavesAreIndependentCopies(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	root := campaignLoadoutRoot(t)
	c, err := loadCampaignContext(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	template, _ := json.Marshal(c.catalogs[campaignCharacter].Combatants[campaignCharacter].Decklist)
	if _, err = newCampaign(contentRoot, root, campaignCharacter, "  "); err == nil {
		t.Fatal("a campaign started without a name")
	}
	a := startCampaign(t, contentRoot, root, campaignCharacter, "First")
	b := startCampaign(t, contentRoot, root, campaignCharacter, "Second")
	adv := startCampaign(t, contentRoot, root, "adventurer", "Adventurer run")
	if a.ID == b.ID || !reflect.DeepEqual(a.Sheet.Deck, b.Sheet.Deck) || deckHealth(a.Sheet.Deck) != 12 {
		t.Fatalf("saves are not separate copies of the starting sheet: %+v %+v", a, b)
	}
	view, err := campaignLoadout(contentRoot, root, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var upgrade map[string]any
	for _, o := range view["trades"].([]map[string]any) {
		if o["available"].(bool) && o["cost"].(int) > 0 {
			upgrade = o
			break
		}
	}
	r := upgrade["request"].(map[string]any)
	traded, err := campaignPurchase(contentRoot, root, a.ID, loadout.Purchase{Kind: r["kind"].(string), ID: r["id"].(string), TargetID: r["target_id"].(string), Tree: r["tree"].(string), Revision: a.Sheet.Revision, ExpectedCost: upgrade["cost"].(int)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = campaignPurchase(contentRoot, root, a.ID, loadout.Purchase{Kind: "buy_card", ID: "dispel", Revision: traded.Sheet.Revision, ExpectedCost: 10}); err != nil {
		t.Fatal(err)
	}
	if got := readSave(t, contentRoot, root, a.ID).Sheet; deckHealth(got.Deck) != 13 || got.XP == b.Sheet.XP {
		t.Fatalf("trades did not change the first save: %+v", got)
	}
	if got := readSave(t, contentRoot, root, b.ID).Sheet; !reflect.DeepEqual(got.Deck, b.Sheet.Deck) || got.XP != b.Sheet.XP {
		t.Fatal("trading in one campaign changed another")
	}
	if _, err = os.Stat(filepath.Join(root, "progression", campaignCharacter+".json")); !os.IsNotExist(err) {
		t.Fatal("the campaign created or touched the editors' progression save")
	}
	if now, _ := json.Marshal(c.catalogs[campaignCharacter].Combatants[campaignCharacter].Decklist); string(now) != string(template) {
		t.Fatal("the starting sheet changed")
	}
	// Admin budget overrides belong to the editors' saves, not campaigns.
	all, _, admin, err := loadout.ProgressSnapshot(root, c.economy, c.catalogs)
	if err != nil {
		t.Fatal(err)
	}
	admin.Budgets = map[string]int{campaignCharacter: *all[campaignCharacter].Budget + 500}
	if err = loadout.SaveAdmin(root, c.economy, c.catalogs, admin); err != nil {
		t.Fatal(err)
	}
	if got := readSave(t, contentRoot, root, b.ID).Sheet; got.XP != b.Sheet.XP {
		t.Fatal("an admin budget override changed a campaign save")
	}
	status, err := campaignStatus(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	if saves := status["saves"].([]map[string]any); len(saves) != 3 || saves[0]["name"] != "First" || saves[2]["character"] != "adventurer" {
		t.Fatalf("campaign status lists the saves wrongly: %v", saves)
	}
	if sheets := status["sheets"].([]map[string]any); len(sheets) != 2 || sheets[0]["id"] != campaignCharacter {
		t.Fatalf("campaign status lists the starting sheets wrongly: %v", sheets)
	}
	if err = loadout.DeleteCampaignSave(root, b.ID); err != nil {
		t.Fatal(err)
	}
	status, _ = campaignStatus(contentRoot, root)
	if saves := status["saves"].([]map[string]any); len(saves) != 2 || saves[0]["id"] != a.ID || saves[1]["id"] != adv.ID {
		t.Fatalf("deleting a save removed the wrong one: %v", saves)
	}
	if _, err = campaignLoadout(contentRoot, root, "../progression/starter"); err == nil {
		t.Fatal("a save ID escaped the campaigns directory")
	}
}

// Campaign progress kept in the editors' progression save (before campaigns
// had their own saves) becomes one campaign save, once.
func TestCampaignProgressMigratesIntoASave(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	root := campaignLoadoutRoot(t)
	c, err := loadCampaignContext(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	lib := c.catalogs[campaignCharacter]
	p, err := loadout.ReadProgress(root, campaignCharacter, c.economy, lib)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = loadout.Buy(root, campaignCharacter, c.economy, lib, loadout.Purchase{Kind: "buy_card", ID: "dispel", Revision: p.Revision, ExpectedCost: 10}); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "progression", campaignCharacter+".json")
	raw, _ := os.ReadFile(filename)
	var ledger map[string]any
	_ = json.Unmarshal(raw, &ledger)
	ledger["campaign"] = map[string]any{"next_encounter": 1, "victories": 1, "defeats": 2, "runs_completed": 0}
	raw, _ = json.Marshal(ledger)
	if err = os.WriteFile(filename, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = campaignStatus(contentRoot, root); err != nil {
			t.Fatal(err)
		}
	}
	status, _ := campaignStatus(contentRoot, root)
	saves := status["saves"].([]map[string]any)
	if len(saves) != 1 || saves[0]["name"] != "Starter campaign" || saves[0]["xp"] != p.XP || saves[0]["health"] != deckHealth(p.Deck) {
		t.Fatalf("migration did not copy the campaign once: %v", saves)
	}
	if got := saves[0]["campaign"].(loadout.CampaignProgress); got.Next != 1 || got.Victories != 1 || got.Defeats != 2 {
		t.Fatalf("migration lost the campaign position: %+v", got)
	}
	after, _ := loadout.ReadProgress(root, campaignCharacter, c.economy, lib)
	if after.Campaign != nil || !reflect.DeepEqual(after.Deck, p.Deck) || after.XP != p.XP {
		t.Fatalf("migration changed the progression save beyond clearing its campaign: %+v", after)
	}
}

// Prepare's offers are dry runs of the real purchases: each one marked
// available must succeed and each one refused must fail, on a copy of the save.
// A tree trade keeps the copy in the deck, so health never changes.
func TestCampaignLoadoutOffersMatchPurchases(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	root := campaignLoadoutRoot(t)
	save := startCampaign(t, contentRoot, root, campaignCharacter, "Offers")
	// Spend most XP so some offers are refused for cost.
	revision := save.Sheet.Revision
	for _, id := range []string{"dispel", "salve", "reclaim", "disrupt", "blood_price", "take_stock", "nudge", "try_again"} {
		bought, err := campaignPurchase(contentRoot, root, save.ID, loadout.Purchase{Kind: "buy_card", ID: id, Revision: revision, ExpectedCost: 10})
		if err != nil {
			t.Fatal(err)
		}
		revision = bought.Sheet.Revision
	}
	view, err := campaignLoadout(contentRoot, root, save.ID)
	if err != nil {
		t.Fatal(err)
	}
	var offers []map[string]any
	for _, key := range []string{"trades", "bases"} {
		offers = append(offers, view[key].([]map[string]any)...)
	}
	for _, entry := range view["deck"].([]map[string]any) {
		offers = append(offers, entry["sell"].(map[string]any), entry["store"].(map[string]any))
	}
	for _, a := range view["abilities"].([]map[string]any) {
		if u, ok := a["upgrade"].(map[string]any); ok {
			offers = append(offers, u)
		}
	}
	available, refused := 0, 0
	for _, o := range offers {
		r := o["request"].(map[string]any)
		copyRoot := t.TempDir()
		copyDir(t, root, copyRoot)
		before := readSave(t, contentRoot, copyRoot, save.ID).Sheet
		after, err := campaignPurchase(contentRoot, copyRoot, save.ID, loadout.Purchase{Kind: r["kind"].(string), ID: r["id"].(string), TargetID: r["target_id"].(string), Tree: r["tree"].(string), Revision: before.Revision, ExpectedCost: o["cost"].(int)})
		if (err == nil) != o["available"].(bool) {
			t.Fatalf("offer %v said available=%v but the purchase returned %v", r, o["available"], err)
		}
		if err != nil {
			refused++
			continue
		}
		available++
		if after.Sheet.XP != before.XP+o["xp_change"].(int) {
			t.Fatalf("offer %v changed XP by %d, not %d", r, after.Sheet.XP-before.XP, o["xp_change"])
		}
		if r["kind"] == "tree_card_deck" && deckHealth(after.Sheet.Deck) != deckHealth(before.Deck) {
			t.Fatalf("tree trade %v changed health", r)
		}
	}
	if available == 0 || refused == 0 {
		t.Fatalf("expected both kinds of offer: %d available, %d refused", available, refused)
	}
	downs := 0
	for _, o := range view["trades"].([]map[string]any) {
		if o["cost"].(int) < 0 {
			downs++
		}
	}
	if downs == 0 {
		t.Fatal("no trade-down offers")
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), raw, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
