package learned

import (
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// campaignBattle marks the session's battle as a campaign encounter. The
// authority records its result in the campaign save exactly once, when the
// battle ends; the client never names the reward.
type campaignBattle struct {
	saveID    string
	saveName  string
	deck      []loadout.Entry
	wounds    []loadout.Entry
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
		if len(save.Sheet.Wounds) > 0 {
			return nil, fmt.Errorf("every card in the campaign deck is wounded; buy a new card before fighting")
		}
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
	battle := &campaignBattle{saveID: save.ID, saveName: save.Name, deck: save.Sheet.Deck, wounds: save.Sheet.Wounds, board: save.Sheet.Abilities, character: save.Character, encounter: encounter, index: index, total: len(campaign.Encounters)}
	return s.resetLoadout(battleID, seed, "seat-a", false, save.Character, BattleRules{UnifiedDefense: true, SimultaneousPlay: true}, "campaign", battle, nil)
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
	var final engine.Result
	if err == nil {
		// The removed pile carries over as wounds; read it from the human's view.
		final, err = s.environment.Observe(s.humanSeat)
	}
	if err == nil {
		removed := final.Snapshot.Actors[s.humanSeat].RemovedComposition
		outcome, err = loadout.RecordCampaignBattle(s.config.LoadoutRoot, b.saveID, c.economy, c.catalogs, c.campaign, b.encounter.ID, b.battleID, s.telemetry.Result, removed)
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
		mode := save.Mode
		if mode == "" {
			mode = "linear"
		}
		var exp map[string]any
		if save.IsExpedition() {
			x := save.Expedition
			exp = map[string]any{"region": x.Region, "round": x.Round, "night": x.Night, "phase": x.Phase, "result": x.Result, "pending_battle": x.PendingBattle != nil, "gold": x.Gold}
		}
		views = append(views, map[string]any{
			"mode": mode, "expedition": exp,
			"id": save.ID, "name": save.Name, "character": save.Character, "character_name": lib.Combatants[save.Character].Name,
			"created_at": save.CreatedAt, "playable": campaign.Allows(save.Character),
			"xp": save.Sheet.XP, "earned_xp": save.Sheet.EarnedXP, "xp_total": *save.Sheet.Budget, "health": deckHealth(save.Sheet.Deck),
			"wounds": deckHealth(save.Sheet.Wounds), "max_health": deckHealth(save.Sheet.Deck) + deckHealth(save.Sheet.Wounds),
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
		request = request.InCampaign()
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
					// The most copies one batched trade can move now: held copies,
					// XP, copy limits and deck rules all apply to the batch.
					o["max_count"] = 0
					for n := entry.Count; n >= 1; n-- {
						if _, why := loadout.PreviewPurchase(p, character, e, lib, loadout.Purchase{Kind: "tree_card_deck", ID: entry.CardID, TargetID: target.Card.ID, Tree: ref.Tree, Count: n}.InCampaign()); why == nil {
							o["max_count"] = n
							break
						}
					}
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
	wounded := []map[string]any{}
	for _, entry := range p.Wounds {
		wounded = append(wounded, map[string]any{"card_id": entry.CardID, "tree": entry.Tree, "count": entry.Count, "value": e.Value(character, entry)})
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
		"catalog":   characterCatalogView(map[string]content.BattleLibrary{character: lib})[character],
		"character": character, "name": lib.Combatants[character].Name, "revision": p.Revision, "xp": p.XP, "health": health,
		"deck": deck, "collection": stored, "trades": trades, "bases": bases, "abilities": abilities,
		// Wounded copies: owned and worth their XP, but out of play until mended.
		"wounds": wounded, "max_health": health + deckHealth(p.Wounds),
		"trees": lib.CardTrees, "tree_order": treeIDs,
		"tree_conflicts": cardNames(lib, loadout.NonTreeCards(p.Deck, lib.CardTrees)),
		"max_cards":      loadout.MaxCards,
	}, nil
}

// --- expedition ops -----------------------------------------------------------

type expeditionContext struct {
	campaignContext
	rules map[string]expedition.Rules
}

func loadExpeditionContext(contentRoot, loadoutRoot string) (expeditionContext, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return expeditionContext{}, err
	}
	x := expeditionContext{campaignContext: c, rules: map[string]expedition.Rules{}}
	for _, id := range c.campaign.Characters {
		if x.rules[id], err = loadout.ExpeditionRules(contentRoot, c.catalogs[id]); err != nil {
			return x, err
		}
	}
	return x, nil
}

func newExpeditionCampaign(contentRoot, loadoutRoot, character, name, region string, seed uint64) (loadout.CampaignSave, error) {
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		return loadout.CampaignSave{}, err
	}
	rules, ok := x.rules[character]
	if !ok {
		return loadout.CampaignSave{}, fmt.Errorf("%s cannot start an expedition", character)
	}
	return loadout.NewExpeditionSave(loadoutRoot, character, name, region, seed, x.campaign, x.economy, x.catalogs, rules)
}

// expeditionView is the viewer-safe status of an expedition save.
func expeditionView(x expeditionContext, save loadout.CampaignSave) map[string]any {
	lib := x.catalogs[save.Character]
	view := save.Expedition.View(x.rules[save.Character], save.Sheet.Revision)
	cards := map[string]any{}
	for _, id := range append(append(append([]string{}, save.Expedition.Hand...), save.Expedition.Draw...), append(append([]string{}, save.Expedition.Spent...), save.Expedition.Wounds.Cards()...)...) {
		if _, done := cards[id]; !done {
			cards[id] = lib.Cards[id]
		}
	}
	statuses := map[string]any{}
	for id, def := range lib.Statuses {
		statuses[id] = def
	}
	var pending map[string]any
	if pb := save.Expedition.PendingBattle; pb != nil {
		pending = map[string]any{"id": pb.ID, "opponent": pb.Opponent, "opponent_count": pb.Count, "name": pb.Name, "kind": pb.Kind, "ambush": pb.Ambush, "warlord": pb.Warlord}
	}
	return map[string]any{
		"save_id": save.ID, "save_name": save.Name, "character": save.Character, "character_name": lib.Combatants[save.Character].Name,
		// xp_total is the sheet's total XP (Budget): spent and unspent.
		"revision": save.Sheet.Revision, "xp": save.Sheet.XP, "earned_xp": save.Sheet.EarnedXP, "xp_total": *save.Sheet.Budget,
		"health": deckHealth(save.Sheet.Deck), "max_health": deckHealth(save.Sheet.Deck) + deckHealth(save.Sheet.Wounds),
		"view": view, "pending_battle": pending,
		// Card and status definitions for presentation (faces, hover rules).
		"catalog": map[string]any{"cards": cards, "statuses": statuses},
	}
}

func expeditionStatus(contentRoot, loadoutRoot, saveID string) (map[string]any, error) {
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadExpeditionSave(loadoutRoot, saveID, x.economy, x.catalogs)
	if err != nil {
		return nil, err
	}
	return expeditionView(x, save), nil
}

func expeditionAct(contentRoot, loadoutRoot, saveID string, action expedition.Action, revision int) (map[string]any, error) {
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadExpeditionSave(loadoutRoot, saveID, x.economy, x.catalogs)
	if err != nil {
		return nil, err
	}
	save, rewards, err := loadout.ExpeditionAct(loadoutRoot, saveID, x.economy, x.catalogs, x.rules[save.Character], action, revision)
	if err != nil {
		return nil, err
	}
	view := expeditionView(x, save)
	view["rewards"] = rewards
	return view, nil
}

func expeditionDeckChange(contentRoot, loadoutRoot, saveID string, request loadout.Purchase) (map[string]any, error) {
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadExpeditionSave(loadoutRoot, saveID, x.economy, x.catalogs)
	if err != nil {
		return nil, err
	}
	save, err = loadout.ExpeditionDeckChange(loadoutRoot, saveID, x.economy, x.catalogs, x.rules[save.Character], request)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": save.ID, "revision": save.Sheet.Revision, "xp": save.Sheet.XP, "xp_total": *save.Sheet.Budget, "view": save.Expedition.View(x.rules[save.Character], save.Sheet.Revision)}, nil
}

// --- expedition battles ---------------------------------------------------------

// expeditionBattle marks the session's battle as an expedition fight: it
// started from the map's piles and the map records its result once.
type expeditionBattle struct {
	saveID    string
	saveName  string
	character string
	deck      []loadout.Entry
	wounds    []loadout.Entry
	board     content.AbilityBoard
	setup     expedition.BattleSetup
	pending   expedition.PendingBattle
	region    string
	round     int
	night     bool
	battleID  string
	recorded  bool
	outcome   *loadout.ExpeditionOutcome
	err       string
}

func (b *expeditionBattle) view() map[string]any {
	view := map[string]any{"save_id": b.saveID, "save_name": b.saveName, "character": b.character, "expedition": map[string]any{
		"kind": b.pending.Kind, "name": b.pending.Name, "opponent": b.pending.Opponent, "count": b.pending.Count, "ambush": b.pending.Ambush, "warlord": b.pending.Warlord,
		"region": b.region, "round": b.round, "night": b.night, "energy_in": b.pending.EnergyIn,
	}}
	if b.outcome != nil {
		view["outcome"] = b.outcome
	}
	if b.err != "" {
		view["error"] = b.err
	}
	return view
}

// presets builds the bridge: the human seat starts from the map piles, every
// enemy seat gets the pending battle's statuses.
func (b *expeditionBattle) presets(humanSeat, modelSeat string, opponentCount int) map[string]battle.PresetPiles {
	var playerStatuses, enemyStatuses []content.StartingStatus
	for _, st := range b.setup.PlayerStatuses {
		playerStatuses = append(playerStatuses, content.StartingStatus{DefinitionID: st.ID, Stacks: st.Stacks})
	}
	for _, st := range b.setup.EnemyStatuses {
		enemyStatuses = append(enemyStatuses, content.StartingStatus{DefinitionID: st.ID, Stacks: st.Stacks})
	}
	presets := map[string]battle.PresetPiles{humanSeat: {Piles: true, Hand: b.setup.Hand, Deck: b.setup.Draw, Discard: b.setup.Spent, Removed: b.setup.Wounds, Energy: b.setup.Energy, HandLimit: b.setup.HandLimit, Statuses: playerStatuses}}
	if len(enemyStatuses) > 0 {
		presets[modelSeat] = battle.PresetPiles{Statuses: enemyStatuses}
		if opponentCount == 2 {
			presets["seat-c"] = battle.PresetPiles{Statuses: enemyStatuses}
		}
	}
	return presets
}

// ResetExpeditionBattle starts the save's pending expedition battle from the
// map's piles. The session must already be initialized with the pending
// battle's opponent; the battle id and seed come from the pending battle so a
// relaunch restarts the identical battle.
func (s *Session) ResetExpeditionBattle(saveID string) (map[string]any, error) {
	if s.config.LoadoutRoot == "" {
		return nil, fmt.Errorf("the expedition requires loadout storage")
	}
	x, err := loadExpeditionContext(s.config.ContentRoot, s.config.LoadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadExpeditionSave(s.config.LoadoutRoot, saveID, x.economy, x.catalogs)
	if err != nil {
		return nil, err
	}
	if !x.campaign.Allows(save.Character) {
		return nil, fmt.Errorf("%s can no longer play the campaign", save.Character)
	}
	rules := x.rules[save.Character]
	_, setup, err := loadout.ExpeditionBattleSetup(s.config.LoadoutRoot, saveID, x.economy, x.catalogs, rules)
	if err != nil {
		return nil, err
	}
	if setup.Opponent != s.config.OpponentDefinition || setup.Count != max(1, s.config.OpponentCount) {
		return nil, fmt.Errorf("initialize the battle runtime with %s ×%d first", setup.Opponent, setup.Count)
	}
	if len(setup.Hand)+len(setup.Draw)+len(setup.Spent) == 0 {
		return nil, fmt.Errorf("every card is wounded; the expedition cannot fight")
	}
	b := &expeditionBattle{saveID: save.ID, saveName: save.Name, character: save.Character, deck: save.Sheet.Deck, wounds: save.Sheet.Wounds, board: save.Sheet.Abilities, setup: setup, pending: *save.Expedition.PendingBattle, region: rules.Pack.Regions[save.Expedition.Region].Name, round: save.Expedition.Round, night: save.Expedition.Night}
	return s.resetLoadout(setup.BattleID, setup.Seed, "seat-a", false, save.Character, BattleRules{UnifiedDefense: true, SimultaneousPlay: true}, "expedition", nil, b)
}

// recordExpeditionBattle runs under s.mu once the battle is terminal: it
// reads the human's piles and energy from the authoritative battle and hands
// them back to the map.
func (s *Session) recordExpeditionBattle() {
	b := s.expedition
	if b == nil || b.recorded {
		return
	}
	b.recorded = true
	fail := func(err error) {
		b.err = err.Error()
		s.telemetry.Errors = append(s.telemetry.Errors, b.err)
		s.recordDiagnostic("expedition_error", map[string]any{"error": b.err, "campaign_save": b.saveID, "battle_id": b.battleID})
	}
	x, err := loadExpeditionContext(s.config.ContentRoot, s.config.LoadoutRoot)
	if err != nil {
		fail(err)
		return
	}
	final, err := s.environment.InspectBattle()
	if err != nil {
		fail(err)
		return
	}
	actor, ok := final.Actors[s.humanSeat]
	if !ok {
		fail(fmt.Errorf("the human seat is missing from the finished battle"))
		return
	}
	instances := final.Settled.Actors[s.humanSeat].CardInstances
	definitions := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, instances[id].DefinitionID)
		}
		return out
	}
	end := expedition.BattleEnd{BattleID: b.battleID, Result: s.telemetry.Result, Hand: definitions(actor.Cards.Hand), Draw: definitions(actor.Cards.Deck), Spent: definitions(actor.Cards.Discard), Removed: definitions(actor.Cards.Removed), Energy: actor.Resources.EnergyPoints}
	// The battle's wound ledger, so the map keeps each hit's cards grouped.
	lib := x.catalogs[b.character]
	for _, w := range final.Wounds {
		if w.TargetActorID != s.humanSeat || len(w.Cards) == 0 {
			continue
		}
		cards := make([]string, 0, len(w.Cards))
		for _, c := range w.Cards {
			id := c.CardDefinitionID
			if id == "" {
				id = instances[c.CardID].DefinitionID
			}
			cards = append(cards, id)
		}
		end.Wounds = append(end.Wounds, expedition.BattleWound{Round: w.Round, Source: woundSourceName(lib, final, w), Cards: cards})
	}
	outcome, err := loadout.RecordExpeditionBattle(s.config.LoadoutRoot, b.saveID, x.economy, x.catalogs, x.rules[b.character], end)
	if err != nil {
		fail(err)
		return
	}
	b.outcome = &outcome
	s.recordDiagnostic("expedition_recorded", map[string]any{"campaign_save": b.saveID, "outcome": outcome})
}

// woundSourceName names what dealt a battle wound: the ability or card, else
// the attacker, else the battle itself.
func woundSourceName(lib content.BattleLibrary, final state.Battle, w state.Wound) string {
	if a, ok := lib.Abilities[w.SourceContentID]; ok && a.Name != "" {
		return a.Name
	}
	if c, ok := lib.Cards[w.SourceContentID]; ok && c.Name != "" {
		return c.Name
	}
	if actor, ok := final.Actors[w.SourceActorID]; ok {
		if def, ok := lib.Combatants[actor.DefinitionID]; ok && def.Name != "" {
			return def.Name
		}
	}
	return ""
}
