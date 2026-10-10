package learned

import (
	"fmt"
	"strings"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// campaignBattle marks the session's battle as a campaign encounter. The
// authority records its result in the progression ledger exactly once, when
// the battle ends; the client never names the reward.
type campaignBattle struct {
	character string
	encounter loadout.Encounter
	index     int
	total     int
	battleID  string
	recorded  bool
	outcome   *loadout.CampaignOutcome
	err       string
}

func (b *campaignBattle) view() map[string]any {
	view := map[string]any{"character": b.character, "encounter": b.encounter, "index": b.index, "total": b.total}
	if b.outcome != nil {
		view["outcome"] = b.outcome
	}
	if b.err != "" {
		view["error"] = b.err
	}
	return view
}

type campaignContext struct {
	catalogs map[string]content.BattleLibrary
	economy  loadout.Economy
	access   loadout.AccessRules
	campaign loadout.Campaign
}

func loadCampaignContext(contentRoot, loadoutRoot string) (campaignContext, error) {
	var c campaignContext
	var err error
	if c.catalogs, err = CharacterCatalogs(contentRoot, loadoutRoot); err != nil {
		return c, err
	}
	if c.economy, err = loadout.LoadEconomy(contentRoot, c.catalogs); err != nil {
		return c, err
	}
	if c.access, err = loadout.ReadAccess(loadoutRoot, c.economy); err != nil {
		return c, err
	}
	c.campaign, err = loadout.LoadCampaign(contentRoot, c.catalogs, c.access)
	return c, err
}

// ResetCampaignEncounter starts the character's next campaign encounter with
// its progression deck. The session must already be initialized with that
// encounter's opponent.
func (s *Session) ResetCampaignEncounter(battleID string, seed uint64, character, encounterID string) (map[string]any, error) {
	if s.config.LoadoutRoot == "" {
		return nil, fmt.Errorf("the campaign requires loadout storage")
	}
	c, err := loadCampaignContext(s.config.ContentRoot, s.config.LoadoutRoot)
	if err != nil {
		return nil, err
	}
	campaign := c.campaign
	if !campaign.Allows(character) {
		return nil, fmt.Errorf("%s cannot enter the campaign", character)
	}
	encounter, ok := campaign.Encounter(encounterID)
	if !ok {
		return nil, fmt.Errorf("unknown campaign encounter %q", encounterID)
	}
	progress, err := loadout.ReadProgress(s.config.LoadoutRoot, character, c.economy, c.catalogs[character])
	if err != nil {
		return nil, err
	}
	next := campaign.NextEncounter(progress)
	if next.ID != encounterID {
		return nil, fmt.Errorf("the next campaign encounter is %s", next.Name)
	}
	if blocked := loadout.NonTreeCards(progress.Deck, c.catalogs[character].CardTrees); len(blocked) > 0 {
		return nil, fmt.Errorf("the campaign uses only card-tree cards; sell or store %s first", strings.Join(cardNames(c.catalogs[character], blocked), ", "))
	}
	if encounter.Opponent != s.config.OpponentDefinition || encounter.OpponentCount != max(1, s.config.OpponentCount) {
		return nil, fmt.Errorf("initialize the battle runtime with encounter %q's opponent first", encounterID)
	}
	index := 0
	for i, e := range campaign.Encounters {
		if e.ID == encounterID {
			index = i
		}
	}
	battle := &campaignBattle{character: character, encounter: encounter, index: index, total: len(campaign.Encounters)}
	return s.resetLoadout(battleID, seed, "seat-a", false, character, true, "progression", battle)
}

// recordCampaignBattle runs under s.mu once the battle is terminal or truncated.
func (s *Session) recordCampaignBattle() {
	b := s.campaign
	if b == nil || b.recorded {
		return
	}
	b.recorded = true
	c, err := loadCampaignContext(s.config.ContentRoot, s.config.LoadoutRoot)
	var outcome loadout.CampaignOutcome
	if err == nil {
		outcome, err = loadout.RecordCampaignBattle(s.config.LoadoutRoot, b.character, c.economy, c.catalogs[b.character], c.campaign, b.encounter.ID, b.battleID, s.telemetry.Result)
	}
	if err != nil {
		b.err = err.Error()
		s.telemetry.Errors = append(s.telemetry.Errors, b.err)
		s.recordDiagnostic("campaign_error", map[string]any{"error": b.err, "encounter_id": b.encounter.ID})
		return
	}
	b.outcome = &outcome
	s.recordDiagnostic("campaign_recorded", outcome)
}

// campaignStatus is the between-battles view: the encounter sequence and each
// campaign character's XP, health and place in it.
func campaignStatus(contentRoot, loadoutRoot string) (map[string]any, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	campaign := c.campaign
	characters := map[string]any{}
	for _, id := range campaign.Characters {
		p, err := loadout.ReadProgress(loadoutRoot, id, c.economy, c.catalogs[id])
		if err != nil {
			return nil, err
		}
		state := loadout.CampaignProgress{}
		if p.Campaign != nil {
			state = *p.Campaign
			state.Battles = nil
		}
		state.Next = 0
		next := campaign.NextEncounter(p)
		for i, e := range campaign.Encounters {
			if e.ID == next.ID {
				state.Next = i
			}
		}
		health := 0
		for _, entry := range p.Deck {
			health += entry.Count
		}
		characters[id] = map[string]any{
			"name":           c.catalogs[id].Combatants[id].Name,
			"xp":             p.XP,
			"earned_xp":      p.EarnedXP,
			"health":         health,
			"deck_value":     p.DeckValue,
			"stored_cards":   len(p.Collection),
			"campaign":       state,
			"next_encounter": next,
			"type_conflicts": c.access.Problems(id, p.Deck, p.Abilities),
			// Equipped cards outside every card tree; the campaign refuses them.
			"tree_conflicts": cardNames(c.catalogs[id], loadout.NonTreeCards(p.Deck, c.catalogs[id].CardTrees)),
		}
	}
	encounters := []map[string]any{}
	for _, e := range campaign.Encounters {
		opponent := e.Opponent
		for _, lib := range c.catalogs {
			opponent = lib.Combatants[e.Opponent].Name
			break
		}
		encounters = append(encounters, map[string]any{"id": e.ID, "name": e.Name, "description": e.Description, "opponent": e.Opponent, "opponent_name": opponent, "opponent_count": e.OpponentCount, "xp": e.XP})
	}
	return map[string]any{"encounters": encounters, "characters": characters, "character_order": campaign.Characters}, nil
}

func cardNames(lib content.BattleLibrary, ids []string) []string {
	names := []string{}
	for _, id := range ids {
		names = append(names, lib.Cards[id].Name)
	}
	return names
}
