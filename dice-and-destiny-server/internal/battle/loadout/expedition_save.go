package loadout

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/content"
)

// ExpeditionRules loads the expedition config and content pack against a
// character's library (which holds the combatants, cards and statuses).
func ExpeditionRules(contentRoot string, lib content.BattleLibrary) (expedition.Rules, error) {
	cfg, err := expedition.LoadConfig(contentRoot, lib)
	if err != nil {
		return expedition.Rules{}, err
	}
	pack, err := expedition.LoadPack(contentRoot, cfg, lib)
	if err != nil {
		return expedition.Rules{}, err
	}
	return expedition.Rules{Config: cfg, Pack: pack, Cards: lib.Cards, Statuses: lib.Statuses}, nil
}

// expandEntries lists one card ID per copy, sorted so the expansion is stable.
func expandEntries(entries []Entry) []string {
	var out []string
	for _, e := range entries {
		for i := 0; i < e.Count; i++ {
			out = append(out, e.CardID)
		}
	}
	sort.Strings(out)
	return out
}

func countMap(ids []string) map[string]int {
	out := map[string]int{}
	for _, id := range ids {
		out[id]++
	}
	return out
}

// syncSheetFromState rewrites the sheet's deck/wound split from the state's
// wound list (keeping tree tags) and reprices both.
func syncSheetFromState(save *CampaignSave, e Economy) {
	p := &save.Sheet
	all := append(append([]Entry(nil), p.Deck...), p.Wounds...)
	p.Deck, p.Wounds = woundCopies(all, p.Wounds, countMap(save.Expedition.Wounds.Cards()))
	p.DeckValue = deckValue(p.Deck, save.Character, e)
	p.WoundValue = deckValue(p.Wounds, save.Character, e)
}

// checkStateMatchesSheet refuses a save whose piles disagree with its sheet.
func checkStateMatchesSheet(save CampaignSave) error {
	st := save.Expedition
	live := append(append(append([]string{}, st.Hand...), st.Draw...), st.Spent...)
	sort.Strings(live)
	if !slices.Equal(live, expandEntries(save.Sheet.Deck)) {
		return fmt.Errorf("campaign save %q: the map piles do not match the deck", save.ID)
	}
	wounds := st.Wounds.Cards()
	sort.Strings(wounds)
	if !slices.Equal(wounds, expandEntries(save.Sheet.Wounds)) {
		return fmt.Errorf("campaign save %q: the map wounds do not match the sheet", save.ID)
	}
	return nil
}

// NewExpeditionSave starts an expedition campaign: a named save with a copy
// of the starting sheet, the bonus XP, and a freshly generated region. start
// is the custom adventurer's starting board, when it builds one.
func NewExpeditionSave(root, character, name, regionID string, seed uint64, c Campaign, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules, start ...SheetBoard) (CampaignSave, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxCampaignSaveName {
		return CampaignSave{}, fmt.Errorf("name the campaign (1–%d characters)", maxCampaignSaveName)
	}
	if !c.Allows(character) {
		return CampaignSave{}, fmt.Errorf("%s cannot start a campaign", character)
	}
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, err
	}
	lib := catalogs[character]
	sheet, err := startingSheet(root, character, c.BonusXP, e, lib, start)
	if err != nil {
		return CampaignSave{}, err
	}
	if blocked := NonTreeCards(sheet.Deck, lib.CardTrees); len(blocked) > 0 {
		return CampaignSave{}, fmt.Errorf("the expedition uses only card-tree cards; the starting sheet holds %s", strings.Join(blocked, ", "))
	}
	if regionID == "" {
		ids := make([]string, 0, len(rules.Pack.Regions))
		for id := range rules.Pack.Regions {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return CampaignSave{}, fmt.Errorf("no expedition region is authored")
		}
		regionID = ids[0]
	}
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}
	base := lib.Combatants[character].Resources.StartingEnergy
	state, err := expedition.New(rules, regionID, seed, expandEntries(sheet.Deck), nil, base)
	if err != nil {
		return CampaignSave{}, err
	}
	s := CampaignSave{Version: 1, ID: newCampaignSaveID(root, character), Name: name, Character: character, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Sheet: sheet, Mode: ModeExpedition, Expedition: state}
	return s, writeCampaignSave(root, s)
}

func readExpeditionSave(root, id string, e Economy, catalogs map[string]content.BattleLibrary) (CampaignSave, error) {
	save, err := readCampaignSave(root, id, e, catalogs)
	if err != nil {
		return save, err
	}
	if !save.IsExpedition() {
		return save, fmt.Errorf("campaign save %q is not an expedition", id)
	}
	if err = checkStateMatchesSheet(save); err != nil {
		return save, err
	}
	return save, nil
}

// ReadExpeditionSave loads an expedition save.
func ReadExpeditionSave(root, id string, e Economy, catalogs map[string]content.BattleLibrary) (CampaignSave, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, err
	}
	return readExpeditionSave(root, id, e, catalogs)
}

func creditXP(p *Progress, xp int) {
	if xp <= 0 {
		return
	}
	p.XP += xp
	p.EarnedXP += xp
	budget := *p.Budget + xp
	p.Budget = &budget
}

func finishExpeditionWrite(root string, save *CampaignSave, e Economy, lib content.BattleLibrary) error {
	syncSheetFromState(save, e)
	// Earned XP raises the total, and block defenses' prices with it.
	if err := reconcileBudget(&save.Sheet, campaignEconomy(e), lib); err != nil {
		return err
	}
	save.Sheet.Revision++
	if err := validateProgress(save.Sheet, lib); err != nil {
		return err
	}
	if err := checkStateMatchesSheet(*save); err != nil {
		return err
	}
	return writeCampaignSave(root, *save)
}

// ExpeditionAct applies one map action to a save and writes it atomically.
// revision is the sheet revision the client saw; a stale one is refused.
func ExpeditionAct(root, id string, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules, action expedition.Action, revision int) (CampaignSave, expedition.Rewards, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, expedition.Rewards{}, err
	}
	save, err := readExpeditionSave(root, id, e, catalogs)
	if err != nil {
		return save, expedition.Rewards{}, err
	}
	if revision != 0 && revision != save.Sheet.Revision {
		return save, expedition.Rewards{}, fmt.Errorf("the expedition changed; refresh before acting")
	}
	rewards, err := save.Expedition.Act(rules, action)
	if err != nil {
		return save, rewards, err
	}
	creditXP(&save.Sheet, rewards.XP)
	return save, rewards, finishExpeditionWrite(root, &save, e, catalogs[save.Character])
}

// ExpeditionDeckChange applies one Prepare purchase at a site or camp,
// paying the site's Influence first and keeping the map piles in step.
func ExpeditionDeckChange(root, id string, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules, request Purchase) (CampaignSave, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, err
	}
	save, err := readExpeditionSave(root, id, e, catalogs)
	if err != nil {
		return save, err
	}
	if request.Revision != save.Sheet.Revision {
		return save, fmt.Errorf("campaign changed; refresh before trading")
	}
	state := save.Expedition
	if _, _, err = state.DeckChangeCost(rules); err != nil {
		return save, err
	}
	request = request.InCampaign()
	lib := catalogs[save.Character]
	next, cost, err := applyPurchase(cloneProgress(save.Sheet), save.Character, e, lib, request)
	if err != nil {
		return save, err
	}
	if cost != request.ExpectedCost {
		return save, fmt.Errorf("price changed; refresh before trading")
	}
	if next, err = settlePurchase(next, save.Character, e, lib, request.Kind, cost); err != nil {
		return save, err
	}
	// Wounded copies never move: a purchase that would touch them is refused.
	if err = keepsWounds(save.Sheet, next); err != nil {
		return save, err
	}
	if err = state.PayDeckChange(rules); err != nil {
		return save, err
	}
	before, after := countMap(expandEntries(save.Sheet.Deck)), countMap(expandEntries(next.Deck))
	replaced := map[string]string{}
	if request.Kind == "tree_card_deck" {
		replaced[request.ID] = request.TargetID
	}
	state.ApplyDeckChange(before, after, replaced)
	next.Wounds = save.Sheet.Wounds
	save.Sheet = next
	return save, finishExpeditionWrite(root, &save, e, lib)
}

// ExpeditionBattleSetup returns what the battle bridge needs for the save's
// pending battle.
func ExpeditionBattleSetup(root, id string, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules) (CampaignSave, expedition.BattleSetup, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, expedition.BattleSetup{}, err
	}
	save, err := readExpeditionSave(root, id, e, catalogs)
	if err != nil {
		return save, expedition.BattleSetup{}, err
	}
	setup, err := save.Expedition.Setup(rules)
	return save, setup, err
}

// ExpeditionOutcome is what recording a finished expedition battle changed.
type ExpeditionOutcome struct {
	BattleID string                   `json:"battle_id"`
	Result   string                   `json:"result"`
	XP       int                      `json:"xp_awarded"`
	Total    int                      `json:"xp"`
	Report   *expedition.BattleReport `json:"report"`
	Health   int                      `json:"health"`
	Wounds   int                      `json:"wounds"`
}

// RecordExpeditionBattle commits a finished pending battle to its save once.
func RecordExpeditionBattle(root, id string, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules, end expedition.BattleEnd) (ExpeditionOutcome, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	outcome := ExpeditionOutcome{BattleID: end.BattleID, Result: end.Result}
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return outcome, err
	}
	save, err := readExpeditionSave(root, id, e, catalogs)
	if err != nil {
		return outcome, err
	}
	p := &save.Sheet
	if p.Campaign == nil {
		p.Campaign = &CampaignProgress{}
	}
	if end.BattleID == "" || slices.Contains(p.Campaign.Battles, end.BattleID) {
		return outcome, fmt.Errorf("expedition battle %q was already recorded", end.BattleID)
	}
	pb := save.Expedition.PendingBattle
	if pb == nil || pb.ID != end.BattleID {
		return outcome, fmt.Errorf("battle %q is not the save's pending battle", end.BattleID)
	}
	rewards, report, err := save.Expedition.ResolveBattle(rules, end)
	if err != nil {
		return outcome, err
	}
	creditXP(p, rewards.XP)
	switch end.Result {
	case "victory":
		p.Campaign.Victories++
	case "defeat":
		p.Campaign.Defeats++
	}
	p.Campaign.Battles = append(p.Campaign.Battles, end.BattleID)
	if len(p.Campaign.Battles) > recordedBattleLimit {
		p.Campaign.Battles = p.Campaign.Battles[len(p.Campaign.Battles)-recordedBattleLimit:]
	}
	if err = finishExpeditionWrite(root, &save, e, catalogs[save.Character]); err != nil {
		return outcome, err
	}
	outcome.XP, outcome.Total, outcome.Report = rewards.XP, p.XP, report
	outcome.Health, outcome.Wounds = entryCount(p.Deck), entryCount(p.Wounds)
	return outcome, nil
}
