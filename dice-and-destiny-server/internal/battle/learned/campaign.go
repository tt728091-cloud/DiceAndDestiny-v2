package learned

import (
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// campaignBattle marks the session's battle as a campaign encounter. The
// authority records its result in the campaign save exactly once, when the
// battle ends; the client never names the reward.
type campaignBattle struct {
	saveID    string
	saveName  string
	deck      []loadout.Entry
	board     content.AbilityBoard
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
	view := map[string]any{"save_id": b.saveID, "save_name": b.saveName, "character": b.character, "encounter": b.encounter, "index": b.index, "total": b.total}
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

// ResetCampaignEncounter starts a campaign save's next encounter with that
// save's deck and abilities. The session must already be initialized with the
// encounter's opponent.
func (s *Session) ResetCampaignEncounter(battleID string, seed uint64, saveID, encounterID string) (map[string]any, error) {
	if s.config.LoadoutRoot == "" {
		return nil, fmt.Errorf("the campaign requires loadout storage")
	}
	c, err := loadCampaignContext(s.config.ContentRoot, s.config.LoadoutRoot)
	if err != nil {
		return nil, err
	}
	campaign := c.campaign
	save, err := loadout.ReadCampaignSave(s.config.LoadoutRoot, saveID, c.economy, c.catalogs)
	if err != nil {
		return nil, err
	}
	if !campaign.Allows(save.Character) {
		return nil, fmt.Errorf("%s can no longer play the campaign", save.Character)
	}
	encounter, ok := campaign.Encounter(encounterID)
	if !ok {
		return nil, fmt.Errorf("unknown campaign encounter %q", encounterID)
	}
	next := campaign.NextEncounter(save.Sheet)
	if next.ID != encounterID {
		return nil, fmt.Errorf("the next campaign encounter is %s", next.Name)
	}
	lib := c.catalogs[save.Character]
	if blocked := loadout.NonTreeCards(save.Sheet.Deck, lib.CardTrees); len(blocked) > 0 {
		return nil, fmt.Errorf("the campaign uses only card-tree cards; sell or store %s first", strings.Join(cardNames(lib, blocked), ", "))
	}
	if len(save.Sheet.Deck) == 0 {
		return nil, fmt.Errorf("the campaign deck is empty; buy at least one card before fighting")
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
	battle := &campaignBattle{saveID: save.ID, saveName: save.Name, deck: save.Sheet.Deck, board: save.Sheet.Abilities, character: save.Character, encounter: encounter, index: index, total: len(campaign.Encounters)}
	return s.resetLoadout(battleID, seed, "seat-a", false, save.Character, true, "campaign", battle)
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
		outcome, err = loadout.RecordCampaignBattle(s.config.LoadoutRoot, b.saveID, c.economy, c.catalogs, c.campaign, b.encounter.ID, b.battleID, s.telemetry.Result)
	}
	if err != nil {
		b.err = err.Error()
		s.telemetry.Errors = append(s.telemetry.Errors, b.err)
		s.recordDiagnostic("campaign_error", map[string]any{"error": b.err, "encounter_id": b.encounter.ID, "campaign_save": b.saveID})
		return
	}
	b.outcome = &outcome
	s.recordDiagnostic("campaign_recorded", map[string]any{"campaign_save": b.saveID, "outcome": outcome})
}

// campaignStatus is the campaign menu: the encounters, the starting character
// sheets a new campaign can copy, and every campaign save with its XP, health
// and place in the campaign.
func campaignStatus(contentRoot, loadoutRoot string) (map[string]any, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	campaign := c.campaign
	if err = loadout.MigrateCampaignProgress(loadoutRoot, campaign, c.economy, c.catalogs); err != nil {
		return nil, err
	}
	saves, problems, err := loadout.ListCampaignSaves(loadoutRoot, c.economy, c.catalogs)
	if err != nil {
		return nil, err
	}
	encounterIndex := func(id string) int {
		for i, e := range campaign.Encounters {
			if e.ID == id {
				return i
			}
		}
		return 0
	}
	views := []map[string]any{}
	for _, save := range saves {
		lib := c.catalogs[save.Character]
		state := loadout.CampaignProgress{}
		if save.Sheet.Campaign != nil {
			state = *save.Sheet.Campaign
			state.Battles = nil
		}
		next := campaign.NextEncounter(save.Sheet)
		state.Next = encounterIndex(next.ID)
		views = append(views, map[string]any{
			"id": save.ID, "name": save.Name, "character": save.Character, "character_name": lib.Combatants[save.Character].Name,
			"created_at": save.CreatedAt, "playable": campaign.Allows(save.Character),
			"xp": save.Sheet.XP, "earned_xp": save.Sheet.EarnedXP, "health": deckHealth(save.Sheet.Deck),
			"campaign": state, "next_encounter": next,
			"type_conflicts": c.access.Problems(save.Character, save.Sheet.Deck, save.Sheet.Abilities),
			// Equipped cards outside every card tree; the campaign refuses them.
			"tree_conflicts": cardNames(lib, loadout.NonTreeCards(save.Sheet.Deck, lib.CardTrees)),
		})
	}
	sheets := []map[string]any{}
	for _, id := range campaign.Characters {
		combatant := c.catalogs[id].Combatants[id]
		health := 0
		for _, entry := range combatant.Decklist {
			health += entry.Count
		}
		sheets = append(sheets, map[string]any{"id": id, "name": combatant.Name, "health": health})
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
	return map[string]any{"encounters": encounters, "sheets": sheets, "saves": views, "problems": problems, "bonus_xp": campaign.BonusXP}, nil
}

// newCampaign starts a named campaign save from a starting character sheet.
func newCampaign(contentRoot, loadoutRoot, character, name string) (loadout.CampaignSave, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return loadout.CampaignSave{}, err
	}
	return loadout.NewCampaignSave(loadoutRoot, character, name, c.campaign, c.economy, c.catalogs)
}

// campaignPurchase applies one Prepare transaction to one campaign save.
func campaignPurchase(contentRoot, loadoutRoot, saveID string, request loadout.Purchase) (loadout.CampaignSave, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return loadout.CampaignSave{}, err
	}
	return loadout.BuyCampaign(loadoutRoot, saveID, c.economy, c.catalogs, request)
}

func deckHealth(deck []loadout.Entry) int {
	health := 0
	for _, entry := range deck {
		health += entry.Count
	}
	return health
}

func cardNames(lib content.BattleLibrary, ids []string) []string {
	names := []string{}
	for _, id := range ids {
		names = append(names, lib.Cards[id].Name)
	}
	return names
}

// campaignLoadout is the focused between-battles view of one campaign
// character: its deck, stored cards and abilities, with every trade open to
// them. Each offer is dry-run through the purchase rules, so the client shows
// exactly what the authority would accept, and why not otherwise.
func campaignLoadout(contentRoot, loadoutRoot, saveID string) (map[string]any, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadCampaignSave(loadoutRoot, saveID, c.economy, c.catalogs)
	if err != nil {
		return nil, err
	}
	character, p := save.Character, save.Sheet
	lib := c.catalogs[character]
	e, err := loadout.EffectiveEconomy(loadoutRoot, c.economy)
	if err != nil {
		return nil, err
	}
	offer := func(request loadout.Purchase) map[string]any {
		request.TreeCardsOnly = true
		cost, why := loadout.PreviewPurchase(p, character, e, lib, request)
		change := -cost
		if request.Kind == "sell_card" || request.Kind == "sell_collection_card" || request.Kind == "downgrade_ability" {
			change = cost
		}
		o := map[string]any{"request": map[string]any{"kind": request.Kind, "id": request.ID, "target_id": request.TargetID, "tree": request.Tree}, "cost": cost, "xp_change": change, "available": why == nil}
		if why != nil {
			o["reason"] = why.Error()
		}
		return o
	}
	treeIDs := make([]string, 0, len(lib.CardTrees))
	for id := range lib.CardTrees {
		treeIDs = append(treeIDs, id)
	}
	sort.Strings(treeIDs)
	// Upgrades and trade-downs of an equipped copy keep it in the deck.
	trades := []map[string]any{}
	seen := map[string]bool{}
	for _, entry := range p.Deck {
		for _, ref := range content.TreeCardPlacements(lib.CardTrees, entry.CardID) {
			if ref.Shared && ref.Tree != entry.Tree {
				continue
			}
			t := lib.CardTrees[ref.Tree]
			for _, edge := range t.Edges {
				for _, ends := range [][2]string{{edge.From, edge.To}, {edge.To, edge.From}} {
					if ends[0] != ref.Node || ends[0] == edge.To && !edge.Reversible {
						continue
					}
					target, _ := t.Node(ends[1])
					key := ref.Tree + "|" + entry.CardID + "|" + target.Card.ID
					if seen[key] {
						continue
					}
					seen[key] = true
					o := offer(loadout.Purchase{Kind: "tree_card_deck", ID: entry.CardID, TargetID: target.Card.ID, Tree: ref.Tree})
					o["tree_id"], o["from_node"], o["to_node"], o["edge"] = ref.Tree, ref.Node, target.ID, edge.ID
					trades = append(trades, o)
				}
			}
		}
	}
	// New cards enter the deck as tree bases.
	bases := []map[string]any{}
	for _, id := range treeIDs {
		t := lib.CardTrees[id]
		root, _ := t.Node(t.Root)
		o := offer(loadout.Purchase{Kind: "buy_card", ID: root.Card.ID})
		o["tree_id"], o["tree_name"] = id, t.Name
		bases = append(bases, o)
	}
	deck := []map[string]any{}
	for _, entry := range p.Deck {
		deck = append(deck, map[string]any{"card_id": entry.CardID, "tree": entry.Tree, "count": entry.Count, "tree_card": loadout.IsTreeCard(lib.CardTrees, entry.CardID),
			"sell":  offer(loadout.Purchase{Kind: "sell_card", ID: entry.CardID, Tree: entry.Tree}),
			"store": offer(loadout.Purchase{Kind: "unequip_collection_card", ID: entry.CardID, Tree: entry.Tree})})
	}
	stored := []map[string]any{}
	for _, entry := range p.Collection {
		stored = append(stored, map[string]any{"card_id": entry.CardID, "tree": entry.Tree, "count": entry.Count, "tree_card": loadout.IsTreeCard(lib.CardTrees, entry.CardID),
			"sell":  offer(loadout.Purchase{Kind: "sell_collection_card", ID: entry.CardID, Tree: entry.Tree}),
			"equip": offer(loadout.Purchase{Kind: "equip_collection_card", ID: entry.CardID, Tree: entry.Tree})})
	}
	paths := e.Characters[character].AbilityUpgrades
	abilities := []map[string]any{}
	for _, slot := range []struct {
		kind string
		ids  []string
	}{{"offensive", p.Abilities.Offensive}, {"defensive", p.Abilities.Defensive}} {
		for _, id := range slot.ids {
			a := map[string]any{"id": id, "type": slot.kind}
			if u, ok := paths[id]; ok {
				a["upgrade"] = offer(loadout.Purchase{Kind: "upgrade_ability", ID: id})
				a["upgrade"].(map[string]any)["to"] = u.To
			}
			for from, u := range paths {
				if u.To == id {
					a["downgrade"] = offer(loadout.Purchase{Kind: "downgrade_ability", ID: id, TargetID: from})
					a["downgrade"].(map[string]any)["to"] = from
				}
			}
			abilities = append(abilities, a)
		}
	}
	health := 0
	for _, entry := range p.Deck {
		health += entry.Count
	}
	return map[string]any{
		"save_id": save.ID, "save_name": save.Name,
		// Card and ability definitions for presentation; no other save is read.
		"catalog": characterCatalogView(map[string]content.BattleLibrary{character: lib})[character],
		"character": character, "name": lib.Combatants[character].Name, "revision": p.Revision, "xp": p.XP, "health": health,
		"deck": deck, "collection": stored, "trades": trades, "bases": bases, "abilities": abilities,
		"trees": lib.CardTrees, "tree_order": treeIDs,
		"tree_conflicts": cardNames(lib, loadout.NonTreeCards(p.Deck, lib.CardTrees)),
		"max_cards":      loadout.MaxCards,
	}, nil
}
