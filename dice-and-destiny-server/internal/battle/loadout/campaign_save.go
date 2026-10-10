package loadout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/content"
)

// CampaignSave is one named campaign slot: the player's own character sheet,
// copied from a starting character when the campaign begins. Its deck, stored
// cards, abilities, XP and campaign position change only through campaign
// play and the campaign's Prepare screen. The original character sheet and the
// editors' progression saves are never touched.
//
// Card and ability definitions are not copied: a save holds IDs, so catalog
// edits reach campaigns in progress (see TODO.md, content versioning).
type CampaignSave struct {
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Character string `json:"character"`
	CreatedAt string `json:"created_at"`
	// MigratedFrom names the editors' progression save this slot was
	// converted from, before campaigns had their own saves.
	MigratedFrom string   `json:"migrated_from,omitempty"`
	Sheet        Progress `json:"sheet"`
}

const maxCampaignSaveName = 40

var campaignSaveID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)

func campaignSavePath(root, id string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("loadout root is required")
	}
	if !campaignSaveID.MatchString(id) {
		return "", fmt.Errorf("invalid campaign save %q", id)
	}
	return filepath.Join(root, "campaigns", id+".json"), nil
}

// campaignEconomy prices a save with the current catalog, but without the
// admin budget overrides, which belong to the editors' progression saves.
func campaignEconomy(e Economy) Economy {
	e.Budgets = nil
	e.BudgetAllocations = nil
	return e
}

func readCampaignSave(root, id string, e Economy, catalogs map[string]content.BattleLibrary) (CampaignSave, error) {
	var s CampaignSave
	filename, err := campaignSavePath(root, id)
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return s, fmt.Errorf("no campaign save %q", id)
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("campaign save %q: %w", id, err)
	}
	lib, ok := catalogs[s.Character]
	if s.Version != 1 || s.ID != id || !ok || s.Sheet.Character != s.Character || s.Sheet.Budget == nil {
		return s, fmt.Errorf("campaign save %q is invalid", id)
	}
	if s.Sheet.Deck == nil {
		s.Sheet.Deck = []Entry{}
	}
	if err = validateProgress(s.Sheet, lib); err != nil {
		return s, fmt.Errorf("campaign save %q: %w", s.Name, err)
	}
	// Repricing cards keeps the sheet's total XP and moves the difference
	// into (or out of) the XP left to spend.
	before, _ := json.Marshal(s.Sheet)
	if err = reconcileBudget(&s.Sheet, campaignEconomy(e)); err != nil {
		return s, fmt.Errorf("campaign save %q: %w", s.Name, err)
	}
	if after, _ := json.Marshal(s.Sheet); string(before) != string(after) {
		err = writeJSON(filename, s)
	}
	return s, err
}

func writeCampaignSave(root string, s CampaignSave) error {
	filename, err := campaignSavePath(root, s.ID)
	if err != nil {
		return err
	}
	return writeJSON(filename, s)
}

// ReadCampaignSave loads one save, repriced against the current catalog.
func ReadCampaignSave(root, id string, e Economy, catalogs map[string]content.BattleLibrary) (CampaignSave, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, err
	}
	return readCampaignSave(root, id, e, catalogs)
}

// ListCampaignSaves returns every readable save, oldest first, and a message
// for each save that could not be read, so one broken file never hides the rest.
func ListCampaignSaves(root string, e Economy, catalogs map[string]content.BattleLibrary) ([]CampaignSave, []string, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return nil, nil, err
	}
	files, err := os.ReadDir(filepath.Join(root, "campaigns"))
	if os.IsNotExist(err) {
		return []CampaignSave{}, []string{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	saves, problems := []CampaignSave{}, []string{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		s, err := readCampaignSave(root, strings.TrimSuffix(f.Name(), ".json"), e, catalogs)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		saves = append(saves, s)
	}
	sort.SliceStable(saves, func(i, j int) bool { return saves[i].CreatedAt < saves[j].CreatedAt })
	return saves, problems, nil
}

// startingSheet copies a character's starting sheet: its template deck and its
// published (or configured) ability board, plus the campaign's bonus XP.
func startingSheet(root, character string, bonusXP int, e Economy, lib content.BattleLibrary) (Progress, error) {
	c, ok := lib.Combatants[character]
	if !ok {
		return Progress{}, fmt.Errorf("unknown character %q", character)
	}
	p := Progress{Version: 1, Character: character, Revision: 1, XP: bonusXP, SharedDeck: true, Deck: []Entry{}, Abilities: c.AbilityBoard}
	for _, entry := range c.Decklist {
		p.Deck = append(p.Deck, Entry{CardID: entry.CardID, Count: entry.Count})
	}
	if cfg := e.Characters[character]; cfg.StartingAbilities != nil {
		p.Abilities = *cfg.StartingAbilities
	}
	authored, err := content.ReadAuthoredAbilities(root)
	if err != nil {
		return p, err
	}
	if board, ok := authored.Boards[character]; ok {
		p.Abilities = board
		p.AuthoredAbilityRevision = authored.BoardRevisions[character]
	}
	p.Abilities = content.AbilityBoard{Offensive: append([]string(nil), p.Abilities.Offensive...), Defensive: append([]string(nil), p.Abilities.Defensive...)}
	if err = validateProgress(p, lib); err != nil {
		return p, err
	}
	initializeBudget(&p, campaignEconomy(e))
	return p, nil
}

func newCampaignSaveID(root, character string) string {
	base := fmt.Sprintf("%s-%s", character, time.Now().UTC().Format("20060102-150405"))
	id := base
	for n := 2; ; n++ {
		filename, _ := campaignSavePath(root, id)
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// NewCampaignSave starts a campaign: a new named save holding a copy of the
// character's starting sheet and the campaign's bonus XP.
func NewCampaignSave(root, character, name string, c Campaign, e Economy, catalogs map[string]content.BattleLibrary) (CampaignSave, error) {
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
	sheet, err := startingSheet(root, character, c.BonusXP, e, catalogs[character])
	if err != nil {
		return CampaignSave{}, err
	}
	s := CampaignSave{Version: 1, ID: newCampaignSaveID(root, character), Name: name, Character: character, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Sheet: sheet}
	return s, writeCampaignSave(root, s)
}

// DeleteCampaignSave permanently removes one campaign save.
func DeleteCampaignSave(root, id string) error {
	progressMu.Lock()
	defer progressMu.Unlock()
	filename, err := campaignSavePath(root, id)
	if err != nil {
		return err
	}
	if err = os.Remove(filename); os.IsNotExist(err) {
		return fmt.Errorf("no campaign save %q", id)
	}
	return err
}

// BuyCampaign applies one Prepare transaction to a campaign save only.
func BuyCampaign(root, id string, e Economy, catalogs map[string]content.BattleLibrary, request Purchase) (CampaignSave, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return CampaignSave{}, err
	}
	s, err := readCampaignSave(root, id, e, catalogs)
	if err != nil {
		return s, err
	}
	if request.Revision != s.Sheet.Revision {
		return s, fmt.Errorf("campaign changed; refresh before trading")
	}
	// The campaign uses only card-tree cards, whatever the client asks.
	request.TreeCardsOnly = true
	next, cost, err := applyPurchase(cloneProgress(s.Sheet), s.Character, e, catalogs[s.Character], request)
	if err != nil {
		return s, err
	}
	if cost != request.ExpectedCost {
		return s, fmt.Errorf("price changed; refresh before trading")
	}
	if next, err = settlePurchase(next, s.Character, e, catalogs[s.Character], request.Kind, cost); err != nil {
		return s, err
	}
	s.Sheet = next
	return s, writeCampaignSave(root, s)
}

// MigrateCampaignProgress converts campaign progress kept in the editors'
// progression saves (before campaigns had their own saves) into a campaign
// save per character, once, then clears it from the progression save.
func MigrateCampaignProgress(root string, c Campaign, e Economy, catalogs map[string]content.BattleLibrary) error {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return err
	}
	for _, character := range c.Characters {
		filename, err := progressPath(root, character)
		if err != nil {
			return err
		}
		if _, err = os.Stat(filename); os.IsNotExist(err) {
			continue
		}
		p, err := readProgress(root, character, e, catalogs[character])
		if err != nil {
			return err
		}
		if p.Campaign == nil {
			continue
		}
		sheet := cloneProgress(p)
		sheet.Campaign = &CampaignProgress{Next: p.Campaign.Next, Victories: p.Campaign.Victories, Defeats: p.Campaign.Defeats, RunsCompleted: p.Campaign.RunsCompleted, Battles: slices.Clone(p.Campaign.Battles)}
		budget := *p.Budget
		sheet.Budget = &budget
		sheet.FreeDeckBudget = 0
		name := catalogs[character].Combatants[character].Name + " campaign"
		s := CampaignSave{Version: 1, ID: newCampaignSaveID(root, character), Name: name, Character: character, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), MigratedFrom: character, Sheet: sheet}
		if err = writeCampaignSave(root, s); err != nil {
			return err
		}
		p.Campaign = nil
		p.Revision++
		if err = writeProgress(filename, p); err != nil {
			return err
		}
	}
	return nil
}
