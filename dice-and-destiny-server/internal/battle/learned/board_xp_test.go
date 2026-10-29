package learned

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// firstPaidStep is the first staircase step above the standard level that
// costs XP: its target and its XP.
func firstPaidStep(t *testing.T, staircase map[string]any) (float64, int) {
	t.Helper()
	for _, s := range staircase["steps"].([]any) {
		step := s.(map[string]any)
		if xp := num(step["xp"]); xp > 0 {
			return step["at"].(float64), xp
		}
	}
	t.Fatal("no step above the standard level costs XP")
	return 0, 0
}

func defensePrice(t *testing.T, xp map[string]any, id string) int {
	t.Helper()
	for _, d := range xp["defenses"].([]any) {
		entry := d.(map[string]any)
		if entry["ability"] == id {
			return num(entry["xp"])
		}
	}
	t.Fatalf("no price for %s", id)
	return 0
}

// The editors' sheet pays for its board: the default Holdfast board is part
// of the starting kit; a damage level costs 40 XP per point of average it
// adds; Holdfast refunds 140 XP when dropped; a board the XP can't cover is
// refused.
func TestEditorBoardSaveChargesXP(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	t.Setenv(content.AuthoredRootEnv, t.TempDir())
	loadoutRoot := t.TempDir()
	builder := boardRuntimeCall(t, map[string]any{"op": "board_builder", "content_root": contentRoot, "loadout_root": loadoutRoot})
	sheet := builder["result"].(map[string]any)["sheet"].(map[string]any)
	startXP := num(sheet["xp"])
	if startXP != 150 {
		t.Fatalf("a new custom adventurer has %d XP to spend, want the starting 150 (the Holdfast board is part of the kit)", startXP)
	}
	board := map[string]any{"die": "standard_d6", "template": "lean"}
	priced := boardRuntimeCall(t, map[string]any{"op": "board_price", "content_root": contentRoot, "loadout_root": loadoutRoot, "board": board})["result"].(map[string]any)
	xp := priced["xp"].(map[string]any)
	if num(xp["cost"]) != 0 || num(xp["held"]) != 140 || defensePrice(t, xp, "holdfast") != 140 || defensePrice(t, xp, "light_guard") != 47 {
		t.Fatalf("standard board XP view: %v", xp)
	}
	staircase := priced["staircase"].(map[string]any)
	lowest := num(staircase["lowest"])
	steps := staircase["steps"].([]any)
	if num(steps[lowest].(map[string]any)["xp"]) != 0 {
		t.Fatalf("the standard step costs %v XP", steps[lowest])
	}
	// Each step's XP is 40 per point of average it adds over the standard step.
	base := steps[lowest].(map[string]any)["average"].(float64)
	for _, s := range steps[lowest:] {
		step := s.(map[string]any)
		if want := int(math.Round((step["average"].(float64) - base) * 40)); num(step["xp"]) != want {
			t.Fatalf("step at %v costs %v XP, want %d", step["at"], step["xp"], want)
		}
	}
	level, cost := firstPaidStep(t, staircase)
	board["level"] = level
	up := boardRuntimeCall(t, map[string]any{"op": "board_price", "content_root": contentRoot, "loadout_root": loadoutRoot, "board": board})["result"].(map[string]any)
	if num(up["xp"].(map[string]any)["cost"]) != cost {
		t.Fatalf("previewing level %.3f costs %v, want %d", level, up["xp"].(map[string]any)["cost"], cost)
	}
	saved := boardSaveCall(t, contentRoot, loadoutRoot, board)
	if num(saved["xp"]) != startXP-cost {
		t.Fatalf("after the level: %v XP, want %d", saved["xp"], startXP-cost)
	}
	// Dropping Holdfast refunds its price.
	board["defenses"] = []string{"none"}
	saved = boardSaveCall(t, contentRoot, loadoutRoot, board)
	if num(saved["xp"]) != startXP-cost+140 || len(saved["ability_board"].(map[string]any)["defensive"].([]any)) != 0 {
		t.Fatalf("without a defense: %v XP, board %v", saved["xp"], saved["ability_board"])
	}
	// The top level costs far more than the sheet holds.
	board["level"] = 20
	revision := boardRuntimeCall(t, map[string]any{"op": "board_builder", "content_root": contentRoot, "loadout_root": loadoutRoot})["result"].(map[string]any)["sheet"].(map[string]any)["revision"]
	if reply := boardRuntimeCall(t, map[string]any{"op": "board_save", "content_root": contentRoot, "loadout_root": loadoutRoot, "board": board, "revision": revision}); reply["ok"] == true {
		t.Fatal("a board the XP can't cover was saved")
	}
}

// An expedition's custom adventurer starts from its Board Builder board at
// the standard level, then buys damage levels and defenses at a site for XP
// (and the visit's Influence), and its battles fight with that board.
func TestCustomAdventurerBuysBoardInExpedition(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{"content_root": contentRoot, "loadout_root": loadoutRoot}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	// The editors' board: the Forge die, raised a level, without a defense.
	editorBoard := map[string]any{"die": "forge_d6", "template": "paired_standard", "level": 8, "defenses": []string{"none"}}
	boardSaveCall(t, contentRoot, loadoutRoot, editorBoard)

	created := runtimeOp(t, with(map[string]any{"op": "campaign_new", "character": loadout.CustomAdventurer, "name": "Built run", "mode": "expedition", "seed": 5}), "")
	saveID := str(created["id"])
	save := readSave(t, contentRoot, loadoutRoot, saveID)
	p := save.Sheet
	if p.Dice != "forge_d6" || p.BoardChoice == nil || p.BoardChoice.Template != "paired_standard" || p.BoardChoice.Level != 0 || !slices.Equal(p.Abilities.Defensive, []string{"holdfast"}) {
		t.Fatalf("the expedition starts from the editors' build at the standard kit: die %s choice %+v defenses %v", p.Dice, p.BoardChoice, p.Abilities.Defensive)
	}
	if p.XP != 150 || p.BoardValue == nil || *p.BoardValue != 140 {
		t.Fatalf("starting XP %d, board value %v", p.XP, p.BoardValue)
	}
	view := runtimeOp(t, with(map[string]any{"op": "campaign_board", "campaign_save": saveID}), "")
	xp := view["xp"].(map[string]any)
	level, cost := firstPaidStep(t, view["staircase"].(map[string]any))
	if num(xp["available"]) != 150 || num(xp["held"]) != 140 || num(view["defense_slots"]) != 1 {
		t.Fatalf("campaign_board: %v", xp)
	}
	choice := view["choice"].(map[string]any)
	choice["level"] = level
	// Away from a site or camp the board can't change.
	runtimeOp(t, with(map[string]any{"op": "campaign_board_change", "campaign_save": saveID, "board": choice, "revision": num(view["revision"]), "expected_cost": cost}), "there is no site here")
	// A rebuilt die or template is refused: campaigns change the level and
	// defenses only.
	other := map[string]any{"die": "standard_d6", "template": "lean"}
	runtimeOp(t, with(map[string]any{"op": "campaign_board", "campaign_save": saveID, "board": other}), "keeps its die")

	// Walk to the village, end the turn and pay its Influence.
	status := runtimeOp(t, with(map[string]any{"op": "expedition_status", "campaign_save": saveID}), "")
	rev := num(status["revision"])
	hand := status["view"].(map[string]any)["hand"].([]any)
	act := func(action map[string]any) map[string]any {
		res := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": rev, "action": action}), "")
		rev = num(res["revision"])
		return res["view"].(map[string]any)
	}
	act(map[string]any{"kind": "move", "target": map[string]int{"q": 1, "r": 0}, "cards": []string{str(hand[0].(map[string]any)["id"])}})
	act(map[string]any{"kind": "move", "target": map[string]int{"q": 0, "r": 0}, "cards": []string{str(hand[1].(map[string]any)["id"])}})
	act(map[string]any{"kind": "move", "target": map[string]int{"q": -1, "r": 1}, "cards": []string{str(hand[2].(map[string]any)["id"])}})
	ended := act(map[string]any{"kind": "end_turn"})
	hand = ended["hand"].([]any)
	act(map[string]any{"kind": "use_site", "service": "influence", "cards": []string{str(hand[0].(map[string]any)["id"]), str(hand[1].(map[string]any)["id"])}})

	// A stale price is refused; the shown one buys the level.
	runtimeOp(t, with(map[string]any{"op": "campaign_board_change", "campaign_save": saveID, "board": choice, "revision": rev, "expected_cost": cost + 1}), "price changed")
	changed := runtimeOp(t, with(map[string]any{"op": "campaign_board_change", "campaign_save": saveID, "board": choice, "revision": rev, "expected_cost": cost}), "")
	if num(changed["xp"]) != 150-cost {
		t.Fatalf("after the level: %v XP, want %d", changed["xp"], 150-cost)
	}
	if site := changed["view"].(map[string]any)["site_here"].(map[string]any); site["deck_open"] != true {
		t.Fatalf("the board change paid the visit's Influence and opened the deck: %v", site)
	}
	// Swapping Holdfast for a one-die block: the refund less the block's
	// price at the sheet's total XP, with no more Influence.
	view = runtimeOp(t, with(map[string]any{"op": "campaign_board", "campaign_save": saveID, "board": map[string]any{"die": choice["die"], "template": choice["template"], "slots": choice["slots"], "level": level, "defenses": []string{"clean_deflect"}}}), "")
	xp = view["xp"].(map[string]any)
	total := num(xp["total"])
	block := int(math.Round(1.0 / 6 * 0.5 * float64(total+200)))
	if defensePrice(t, xp, "clean_deflect") != block || num(xp["cost"]) != block-140 {
		t.Fatalf("block price %d at %d total XP, change cost %v", defensePrice(t, xp, "clean_deflect"), total, xp["cost"])
	}
	swapped := runtimeOp(t, with(map[string]any{"op": "campaign_board_change", "campaign_save": saveID, "board": view["choice"], "revision": num(view["revision"]), "expected_cost": num(xp["cost"])}), "")
	if num(swapped["xp"]) != 150-cost+140-block {
		t.Fatalf("after the swap: %v XP", swapped["xp"])
	}
	save = readSave(t, contentRoot, loadoutRoot, saveID)
	if !slices.Equal(save.Sheet.Abilities.Defensive, []string{"clean_deflect"}) || save.Sheet.BoardLevelXP != cost || save.Expedition.Turn.Influence != 0 {
		t.Fatalf("saved board: %v level XP %d influence %d", save.Sheet.Abilities.Defensive, save.Sheet.BoardLevelXP, save.Expedition.Turn.Influence)
	}
	if *save.Sheet.Budget != total {
		t.Fatalf("board changes moved the total XP: %d → %d", total, *save.Sheet.Budget)
	}
}

// A linear campaign's battles fight with the save's own board (its level
// and defense), not the editors' sheet.
func TestCampaignBattleFightsWithTheSavesBoard(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	created := runtimeOp(t, map[string]any{"op": "campaign_new", "content_root": contentRoot, "loadout_root": loadoutRoot, "character": loadout.CustomAdventurer, "name": "Linear build"}, "")
	saveID := str(created["id"])
	view := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID}, "")
	level, cost := firstPaidStep(t, view["staircase"].(map[string]any))
	choice := view["choice"].(map[string]any)
	choice["level"], choice["defenses"] = level, []string{"light_guard"}
	priced := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID, "board": choice}, "")
	if want := cost + 47 - 140; num(priced["xp"].(map[string]any)["cost"]) != want {
		t.Fatalf("level and Light Guard cost %v, want %d", priced["xp"].(map[string]any)["cost"], want)
	}
	runtimeOp(t, map[string]any{"op": "campaign_board_change", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID, "board": priced["choice"], "revision": num(priced["revision"]), "expected_cost": cost + 47 - 140}, "")
	save := readSave(t, contentRoot, loadoutRoot, saveID)
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounter := c.campaign.NextEncounter(save.Sheet)
	s, err := NewSession(SessionConfig{ContentRoot: contentRoot, LoadoutRoot: loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: encounter.Opponent, OpponentCount: encounter.OpponentCount})
	if err != nil {
		t.Fatal(err)
	}
	battle, err := s.ResetCampaignEncounter("built-campaign", 3, saveID, encounter.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := object(battle["snapshot"])
	hero := object(object(snapshot["actors"])[HumanAlias])
	defenses := hero["defensive_abilities"].([]any)
	if len(defenses) != 1 || defenses[0] != "light_guard" {
		t.Fatalf("the battle's defenses %v, want the save's Light Guard", defenses)
	}
	catalog := object(object(snapshot["content_catalog"])["abilities"])
	raised := false
	for id, ability := range save.Sheet.GeneratedAbilities {
		for i, tier := range ability.Qualification.ActivationTiers {
			want := tier.Operations[0].Amount
			pinned := object(object(catalog[id])["qualification"])["activation_tiers"].([]any)
			got := object(object(pinned[i])["operations"].([]any)[0])["amount"]
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("%s deals %v in battle, the save's board %v", id, got, want)
			}
			// The editors' sheet stays at the standard level.
			standard := c.catalogs[loadout.CustomAdventurer].Abilities[id].Qualification.ActivationTiers[i].Operations[0].Amount
			if fmt.Sprint(want) != fmt.Sprint(standard) {
				raised = true
			}
		}
	}
	if !raised {
		t.Fatal("the battle's board is the standard one, not the save's raised level")
	}
}

// A board may go without a defense, or carry a block defense: both fight a
// full battle, and the block rolls the owner's die.
func TestNoDefenseAndBlockBoardsFight(t *testing.T) {
	for i, defenses := range [][]string{{}, {"twin_deflect"}} {
		root := t.TempDir()
		if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "hearth_d6", Abilities: content.AbilityBoard{Offensive: straightsBoard.Offensive, Defensive: defenses}}); err != nil {
			t.Fatalf("%v: %v", defenses, err)
		}
		s := customSession(t, root)
		if _, err := s.ResetCharacter(fmt.Sprintf("defenses-%d", i), uint64(5200+i), "seat-a", false, loadout.CustomAdventurer); err != nil {
			t.Fatalf("%v: %v", defenses, err)
		}
		playToEnd(t, s)
		final, _ := s.environment.InspectBattle()
		rolled := 0
		for _, defense := range final.Settled.DefenseHistory {
			if defense.ActorID != s.humanSeat || defense.AbilityID == "" { // a pass
				continue
			}
			if len(defenses) == 0 || defense.AbilityID != defenses[0] {
				t.Fatalf("%v: the hero defended with %s", defenses, defense.AbilityID)
			}
			rolled++
			for _, die := range defense.RolledDice {
				if die.DieID != "hearth_d6" {
					t.Fatalf("the block rolled %s", die.DieID)
				}
			}
		}
		if len(defenses) > 0 && rolled == 0 {
			t.Fatalf("%v: the block was never rolled", defenses)
		}
		t.Logf("%v: %d rounds, winner %q, %d defenses", defenses, final.Segment.Round, final.WinnerActorID, rolled)
	}
}

// A block defense's price is a share of the total XP: a victory's XP raises
// the total and the block's price with it, and the XP to spend by the rest.
func TestBlockPriceGrowsWithTotalXP(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	created := runtimeOp(t, map[string]any{"op": "campaign_new", "content_root": contentRoot, "loadout_root": loadoutRoot, "character": loadout.CustomAdventurer, "name": "Block run"}, "")
	saveID := str(created["id"])
	view := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID}, "")
	choice := view["choice"].(map[string]any)
	choice["defenses"] = []string{"clean_deflect"}
	priced := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID, "board": choice}, "")
	cost := num(priced["xp"].(map[string]any)["cost"])
	runtimeOp(t, map[string]any{"op": "campaign_board_change", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID, "board": priced["choice"], "revision": num(priced["revision"]), "expected_cost": cost}, "")
	before := readSave(t, contentRoot, loadoutRoot, saveID).Sheet
	block := func(total int) int { return int(math.Round(1.0 / 6 * 0.5 * float64(total+200))) }
	if *before.BoardValue != block(*before.Budget) {
		t.Fatalf("board value %d at %d total XP, want %d", *before.BoardValue, *before.Budget, block(*before.Budget))
	}
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounter := c.campaign.NextEncounter(before)
	if _, err := loadout.RecordCampaignBattle(loadoutRoot, saveID, c.economy, c.catalogs, c.campaign, encounter.ID, "won-1", "victory", nil); err != nil {
		t.Fatal(err)
	}
	after := readSave(t, contentRoot, loadoutRoot, saveID).Sheet
	grew := block(*after.Budget) - block(*before.Budget)
	if *after.Budget != *before.Budget+encounter.XP || *after.BoardValue != block(*after.Budget) || grew <= 0 || after.XP != before.XP+encounter.XP-grew {
		t.Fatalf("after %d XP: total %d → %d, board %d → %d, XP %d → %d", encounter.XP, *before.Budget, *after.Budget, *before.BoardValue, *after.BoardValue, before.XP, after.XP)
	}
}

// A new Starter campaign holds its campaign.yaml board: the Starter template
// on the standard die at the standard level with Holdfast as part of the
// kit. It buys a damage level like the custom adventurer, and its battles
// fight with that board.
func TestStarterCampaignHoldsItsBoard(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	created := runtimeOp(t, map[string]any{"op": "campaign_new", "content_root": contentRoot, "loadout_root": loadoutRoot, "character": "starter", "name": "Starter board"}, "")
	saveID := str(created["id"])
	p := readSave(t, contentRoot, loadoutRoot, saveID).Sheet
	if p.Dice != "standard_d6" || p.BoardChoice == nil || p.BoardChoice.Template != "starter" || !slices.Equal(p.Abilities.Defensive, []string{"holdfast"}) || len(p.GeneratedAbilities) != 6 {
		t.Fatalf("the Starter starts on its board: die %s choice %+v board %v", p.Dice, p.BoardChoice, p.Abilities)
	}
	if p.XP != 150 || p.BoardValue == nil || *p.BoardValue != 140 {
		t.Fatalf("starting XP %d, board value %v", p.XP, p.BoardValue)
	}
	// Guarded Strike's +2 Protect and Measured Strike's +5 energy ride on
	// their slots.
	riders := map[string]bool{}
	for _, slot := range p.BoardChoice.Slots {
		riders[slot.Slot+"="+slot.Rider] = true
	}
	if !riders["guarded_strike=protect_2"] || !riders["measured_strike=energy_5"] {
		t.Fatalf("the Starter board's riders: %v", p.BoardChoice.Slots)
	}
	view := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID}, "")
	if view["moving"] == true || num(view["xp"].(map[string]any)["held"]) != 140 {
		t.Fatalf("campaign_board: moving %v, xp %v", view["moving"], view["xp"])
	}
	// Each defense is priced against the saved one in its slot (Holdfast,
	// 140): Bastion costs 47 more, Light Guard gives 93 back, an empty slot
	// gives all 140 back.
	changes := map[string]int{}
	for _, d := range view["xp"].(map[string]any)["defenses"].([]any) {
		entry := d.(map[string]any)
		changes[str(entry["ability"])] = num(entry["change"].([]any)[0])
	}
	if changes["holdfast"] != 0 || changes["bastion"] != 47 || changes["light_guard"] != -93 || num(view["xp"].(map[string]any)["none_change"].([]any)[0]) != -140 {
		t.Fatalf("defense changes against Holdfast: %v, none %v", changes, view["xp"].(map[string]any)["none_change"])
	}
	level, cost := firstPaidStep(t, view["staircase"].(map[string]any))
	choice := view["choice"].(map[string]any)
	choice["level"] = level
	changed := runtimeOp(t, map[string]any{"op": "campaign_board_change", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": saveID, "board": choice, "revision": num(view["revision"]), "expected_cost": cost}, "")
	if num(changed["xp"]) != 150-cost {
		t.Fatalf("after the level: %v XP, want %d", changed["xp"], 150-cost)
	}
	save := readSave(t, contentRoot, loadoutRoot, saveID)
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	encounter := c.campaign.NextEncounter(save.Sheet)
	s, err := NewSession(SessionConfig{ContentRoot: contentRoot, LoadoutRoot: loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: encounter.Opponent, OpponentCount: encounter.OpponentCount})
	if err != nil {
		t.Fatal(err)
	}
	battle, err := s.ResetCampaignEncounter("starter-board", 3, saveID, encounter.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := object(battle["snapshot"])
	hero := object(object(snapshot["actors"])[HumanAlias])
	if defenses := hero["defensive_abilities"].([]any); len(defenses) != 1 || defenses[0] != "holdfast" {
		t.Fatalf("the battle's defenses %v, want Holdfast", defenses)
	}
	catalog := object(object(snapshot["content_catalog"])["abilities"])
	for id, ability := range save.Sheet.GeneratedAbilities {
		for i, tier := range ability.Qualification.ActivationTiers {
			pinned := object(object(catalog[id])["qualification"])["activation_tiers"].([]any)
			if got := object(object(pinned[i])["operations"].([]any)[0])["amount"]; fmt.Sprint(got) != fmt.Sprint(tier.Operations[0].Amount) {
				t.Fatalf("%s deals %v in battle, the save's board %v", id, got, tier.Operations[0].Amount)
			}
		}
	}
	playToEnd(t, s)
}

// A Starter save from before boards keeps its authored abilities until its
// first board change, which moves it onto its board as part of its kit: the
// XP to spend is unchanged by the move, Guard becomes Holdfast and Guard+
// (bought for 38 XP) becomes Holdfast+, whose price takes over that spend.
func TestOlderStarterSaveMovesOntoItsBoard(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	for _, upgraded := range []bool{false, true} {
		loadoutRoot := campaignLoadoutRoot(t)
		c, err := loadCampaignContext(contentRoot, loadoutRoot)
		if err != nil {
			t.Fatal(err)
		}
		old, err := loadout.NewCampaignSave(loadoutRoot, "starter", "Old run", c.campaign, c.economy, c.catalogs)
		if err != nil {
			t.Fatal(err)
		}
		defense, refund := "holdfast", 140
		if upgraded {
			if old, err = loadout.BuyCampaign(loadoutRoot, old.ID, c.economy, c.catalogs, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: old.Sheet.Revision, ExpectedCost: 38}); err != nil {
				t.Fatal(err)
			}
			defense, refund = "holdfast_plus", 178
		}
		before := old.Sheet
		if loadout.HoldsBoard(before) || before.Abilities.Offensive[0] != "adventurer_strike" {
			t.Fatalf("the old save holds its authored board: %v", before.Abilities)
		}
		view := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": old.ID}, "")
		xp := view["xp"].(map[string]any)
		if view["moving"] != true || num(xp["cost"]) != 0 || num(xp["available"]) != before.XP || num(xp["total"]) != *before.Budget+refund-before.UpgradeSpent {
			t.Fatalf("moving onto the board: moving %v, xp %v (before: %d XP, total %d)", view["moving"], xp, before.XP, *before.Budget)
		}
		choice := view["choice"].(map[string]any)
		if defenses := choice["defenses"].([]any); len(defenses) != 1 || defenses[0] != defense {
			t.Fatalf("the authored defense becomes %v, want %s", defenses, defense)
		}
		// Dropping the defense refunds its whole price, Guard+'s 38 included.
		choice["defenses"] = []string{"none"}
		priced := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": old.ID, "board": choice}, "")
		if num(priced["xp"].(map[string]any)["cost"]) != -refund {
			t.Fatalf("dropping %s: cost %v, want %d", defense, priced["xp"].(map[string]any)["cost"], -refund)
		}
		runtimeOp(t, map[string]any{"op": "campaign_board_change", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": old.ID, "board": priced["choice"], "revision": num(priced["revision"]), "expected_cost": -refund}, "")
		after := readSave(t, contentRoot, loadoutRoot, old.ID).Sheet
		if !loadout.HoldsBoard(after) || after.XP != before.XP+refund || len(after.Abilities.Defensive) != 0 || after.UpgradeSpent != 0 || after.Abilities.Offensive[0][:6] != "board_" {
			t.Fatalf("after the move: XP %d (was %d), board %v, upgrade spent %d", after.XP, before.XP, after.Abilities, after.UpgradeSpent)
		}
	}
}

// Every tier offers its cheapest raise: the first step above the shown one
// where that tier deals more (no step in between does), the XP that climb
// adds, and the other tiers it raises on the way.
func TestTierRaisesAreTheCheapestClimb(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	created := runtimeOp(t, map[string]any{"op": "campaign_new", "content_root": contentRoot, "loadout_root": loadoutRoot, "character": "starter", "name": "Raises"}, "")
	view := runtimeOp(t, map[string]any{"op": "campaign_board", "content_root": contentRoot, "loadout_root": loadoutRoot, "campaign_save": str(created["id"])}, "")
	sc := view["staircase"].(map[string]any)
	steps := sc["steps"].([]any)
	k := num(sc["step"])
	damage := func(j, i int) int { return num(steps[j].(map[string]any)["damage"].([]any)[i]) }
	stepXP := func(j int) int { return num(steps[j].(map[string]any)["xp"]) }
	raises := sc["raises"].([]any)
	if len(raises) != len(view["tiers"].([]any)) {
		t.Fatalf("%d raises for %d tiers", len(raises), len(view["tiers"].([]any)))
	}
	cheap := false
	for i, r := range raises {
		raise := r.(map[string]any)
		j := num(raise["step"])
		if j <= k || damage(j, i) <= damage(k, i) || num(raise["to"]) != damage(j, i) || num(raise["from"]) != damage(k, i) || num(raise["xp"]) != stepXP(j)-stepXP(k) {
			t.Fatalf("tier %d: raise %v is not a climb that raises it", i, raise)
		}
		for m := k + 1; m < j; m++ {
			if damage(m, i) > damage(k, i) {
				t.Fatalf("tier %d: step %d raises it already, cheaper than %v", i, m, raise)
			}
		}
		for _, o := range raise["also"].([]any) {
			if o.(map[string]any)["tier"] == raise["tier"] {
				t.Fatalf("tier %d lists itself among the others it raises", i)
			}
		}
		if num(raise["xp"]) < 40 {
			cheap = true
		}
	}
	if !cheap {
		t.Fatal("no tier raise is cheaper than a whole point of average damage")
	}
}
