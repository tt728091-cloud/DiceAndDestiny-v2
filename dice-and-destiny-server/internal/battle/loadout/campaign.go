package loadout

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"diceanddestiny/server/internal/content"
	"gopkg.in/yaml.v3"
)

// Encounter is one campaign battle: a scripted minion opponent and the XP a
// victory earns.
type Encounter struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Description   string `yaml:"description" json:"description"`
	Opponent      string `yaml:"opponent" json:"opponent"`
	OpponentCount int    `yaml:"opponent_count" json:"opponent_count"`
	XP            int    `yaml:"xp" json:"xp"`
}

// Campaign is the authored encounter sequence. It loops: after the last
// encounter the next run starts again from the first.
type Campaign struct {
	Version    int      `yaml:"schema_version" json:"-"`
	Characters []string `yaml:"characters" json:"characters"`
	// BonusXP is what a new campaign save starts with, on top of the starting sheet.
	BonusXP    int         `yaml:"bonus_xp" json:"bonus_xp"`
	Encounters []Encounter `yaml:"encounters" json:"encounters"`
}

// CampaignProgress is one character's place in the campaign, saved in the
// progression ledger so XP and position commit together.
type CampaignProgress struct {
	Next          int `json:"next_encounter"`
	Victories     int `json:"victories"`
	Defeats       int `json:"defeats"`
	RunsCompleted int `json:"runs_completed"`
	// Recent battle IDs already recorded, so a battle is never rewarded twice.
	Battles []string `json:"recorded_battles,omitempty"`
}

// CampaignOutcome is what one finished campaign battle changed.
type CampaignOutcome struct {
	BattleID      string `json:"battle_id"`
	EncounterID   string `json:"encounter_id"`
	Result        string `json:"result"`
	XPAwarded     int    `json:"xp_awarded"`
	XP            int    `json:"xp"`
	NextEncounter int    `json:"next_encounter"`
	RunCompleted  bool   `json:"run_completed"`
}

const recordedBattleLimit = 20

func LoadCampaign(root string, catalogs map[string]content.BattleLibrary, access AccessRules) (Campaign, error) {
	var c Campaign
	data, err := os.ReadFile(filepath.Join(root, "progression_v1", "campaign.yaml"))
	if err != nil {
		return c, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err = decoder.Decode(&c); err != nil {
		return c, err
	}
	if c.Version != 1 || len(c.Characters) == 0 || len(c.Encounters) == 0 || len(c.Encounters) > 20 || c.BonusXP < 0 || c.BonusXP > 1000000 {
		return c, fmt.Errorf("campaign needs schema_version 1, characters, bonus_xp 0–1000000 and 1–20 encounters")
	}
	for _, id := range c.Characters {
		if _, ok := catalogs[id]; !ok {
			return c, fmt.Errorf("unknown campaign character %q", id)
		}
		// The campaign deals only in General cards and trees.
		if pool(access.Characters, id) != "general" {
			return c, fmt.Errorf("campaign character %q must have the General type", id)
		}
	}
	seen := map[string]bool{}
	for i, e := range c.Encounters {
		if e.OpponentCount == 0 {
			c.Encounters[i].OpponentCount = 1
		}
		if e.ID == "" || e.Name == "" || seen[e.ID] || e.Opponent == "" || c.Encounters[i].OpponentCount > 2 || e.XP < 1 || e.XP > 1000000 {
			return c, fmt.Errorf("invalid campaign encounter %q", e.ID)
		}
		seen[e.ID] = true
		for _, lib := range catalogs {
			if def, ok := lib.Combatants[e.Opponent]; !ok || def.SingleAbilityPolicy == nil {
				return c, fmt.Errorf("campaign encounter %q needs a scripted minion opponent, not %q", e.ID, e.Opponent)
			}
		}
	}
	return c, nil
}

func (c Campaign) Allows(character string) bool { return slices.Contains(c.Characters, character) }

func (c Campaign) Encounter(id string) (Encounter, bool) {
	for _, e := range c.Encounters {
		if e.ID == id {
			return e, true
		}
	}
	return Encounter{}, false
}

// NextEncounter is the encounter the character fights next. Shortening the
// authored campaign wraps a saved position back into range.
func (c Campaign) NextEncounter(p Progress) Encounter {
	next := 0
	if p.Campaign != nil {
		next = p.Campaign.Next % len(c.Encounters)
	}
	return c.Encounters[next]
}

// RecordCampaignBattle commits a finished battle to its campaign save: a
// victory earns the encounter's XP and advances to the next encounter; anything
// else leaves the save facing the same encounter.
func RecordCampaignBattle(root, saveID string, e Economy, catalogs map[string]content.BattleLibrary, c Campaign, encounterID, battleID, result string) (CampaignOutcome, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	outcome := CampaignOutcome{BattleID: battleID, EncounterID: encounterID, Result: result}
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return outcome, err
	}
	save, err := readCampaignSave(root, saveID, e, catalogs)
	if err != nil {
		return outcome, err
	}
	p := &save.Sheet
	if p.Campaign == nil {
		p.Campaign = &CampaignProgress{}
	}
	state := p.Campaign
	if battleID == "" || slices.Contains(state.Battles, battleID) {
		return outcome, fmt.Errorf("campaign battle %q was already recorded", battleID)
	}
	encounter := c.NextEncounter(*p)
	if encounter.ID != encounterID {
		return outcome, fmt.Errorf("campaign moved on from %q; this battle earns nothing", encounterID)
	}
	state.Next %= len(c.Encounters)
	if result == "victory" {
		outcome.XPAwarded = encounter.XP
		p.XP += encounter.XP
		p.EarnedXP += encounter.XP
		budget := *p.Budget + encounter.XP
		p.Budget = &budget
		state.Victories++
		state.Next++
		if state.Next == len(c.Encounters) {
			state.Next = 0
			state.RunsCompleted++
			outcome.RunCompleted = true
		}
	} else if result == "defeat" {
		state.Defeats++
	}
	state.Battles = append(state.Battles, battleID)
	if len(state.Battles) > recordedBattleLimit {
		state.Battles = state.Battles[len(state.Battles)-recordedBattleLimit:]
	}
	p.Revision++
	if err = validateProgress(*p, catalogs[save.Character]); err != nil {
		return outcome, err
	}
	if err = writeCampaignSave(root, save); err != nil {
		return outcome, err
	}
	outcome.XP = p.XP
	outcome.NextEncounter = state.Next
	return outcome, nil
}

// IsTreeCard reports whether a card belongs to any published card tree, as a
// base, variant or shared card. Campaign decks use only these cards.
func IsTreeCard(trees map[string]content.CardTree, id string) bool {
	return len(content.TreeCardPlacements(trees, id)) > 0
}

// NonTreeCards lists a deck's cards the campaign cannot use, once each.
func NonTreeCards(deck []Entry, trees map[string]content.CardTree) []string {
	var ids []string
	for _, entry := range deck {
		if entry.Count > 0 && !IsTreeCard(trees, entry.CardID) && !slices.Contains(ids, entry.CardID) {
			ids = append(ids, entry.CardID)
		}
	}
	return ids
}
