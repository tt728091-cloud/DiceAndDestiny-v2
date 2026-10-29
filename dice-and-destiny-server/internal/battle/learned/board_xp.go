package learned

import (
	"fmt"
	"slices"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// A priced board holds XP (economy.yaml board_xp): its damage level climbs
// the board's staircase at the damage-point rate, and each defense costs its
// price on the sheet's die at the sheet's total XP. The custom adventurer
// always builds one; other campaign characters hold their campaign.yaml
// board. The authority prices every change; the client only shows these
// numbers.

// pricedBoard is a choice priced at its damage level: the sheet board to
// save (die, board, generated abilities, choice, level XP), the report and
// the staircase.
type pricedBoard struct {
	choice    boardprice.Choice
	sheet     loadout.SheetBoard
	report    boardprice.Report
	staircase boardprice.Staircase
	abilities []content.BattleAbilityDefinition
}

func priceSheetBoard(b *boardprice.Builder, rates boardprice.XPRates, choice boardprice.Choice) (pricedBoard, error) {
	resolved, report, err := b.Price(choice)
	if err != nil {
		return pricedBoard{}, err
	}
	_, sc, err := b.Staircase(resolved)
	if err != nil {
		return pricedBoard{}, err
	}
	abilities, board, err := b.Generate(resolved, report)
	if err != nil {
		return pricedBoard{}, err
	}
	generated := map[string]content.BattleAbilityDefinition{}
	for _, a := range abilities {
		generated[a.ID] = a
	}
	levelXP := rates.LevelXP(sc, b.Menus.Pricing.Target, b.LevelOf(resolved))
	sheet := loadout.SheetBoard{Dice: resolved.Die, Abilities: board, Generated: generated, Choice: &resolved, LevelXP: levelXP}
	return pricedBoard{choice: resolved, sheet: sheet, report: report, staircase: sc, abilities: abilities}, nil
}

// sheetChoice is the Board Builder choice a sheet holds: its own, or the
// default Lean board on its die.
func sheetChoice(b *boardprice.Builder, lib content.BattleLibrary, p loadout.Progress) (boardprice.Choice, error) {
	if p.BoardChoice != nil {
		return *p.BoardChoice, nil
	}
	die := p.Dice
	if die == "" {
		die = lib.Combatants[loadout.CustomAdventurer].DiceLoadout[0].DiceID
	}
	resolved, _, _, err := b.Resolve(b.Default("lean", die))
	return resolved, err
}

// startBoard is a character's starting board in a campaign, at the standard
// level; ok is false for a character without one. The custom adventurer
// starts from the die, template and slot picks of the editors' Board Builder
// board with the template's default defenses (the starting kit, the same for
// everyone who builds this board). Other characters start from their
// campaign.yaml board; sheet, when set, is a save moving onto it, whose
// authored defenses become the menu defenses that replace them.
func startBoard(contentRoot, loadoutRoot string, c campaignContext, character string, sheet *loadout.Progress) (loadout.SheetBoard, bool, error) {
	cfg, configured := c.campaign.Boards[character]
	if character != loadout.CustomAdventurer && !configured {
		return loadout.SheetBoard{}, false, nil
	}
	b, err := boardBuilder(contentRoot)
	if err != nil {
		return loadout.SheetBoard{}, false, err
	}
	lib := c.catalogs[character]
	choice := boardprice.Choice{Die: cfg.Die, Template: cfg.Template}
	if character == loadout.CustomAdventurer {
		editor, err := loadout.ReadProgress(loadoutRoot, loadout.CustomAdventurer, c.economy, lib)
		if err != nil {
			return loadout.SheetBoard{}, false, err
		}
		if choice, err = sheetChoice(b, lib, editor); err != nil {
			return loadout.SheetBoard{}, false, err
		}
		choice.Level, choice.Defenses = 0, nil
	} else if sheet != nil {
		tpl, ok := b.Menus.Template(cfg.Template)
		if !ok {
			return loadout.SheetBoard{}, false, fmt.Errorf("unknown board template %q", cfg.Template)
		}
		for _, id := range sheet.Abilities.Defensive {
			if to, ok := cfg.Replaces[id]; ok && !slices.Contains(choice.Defenses, to) && len(choice.Defenses) < tpl.Defenses {
				choice.Defenses = append(choice.Defenses, to)
			}
		}
		for len(choice.Defenses) < tpl.Defenses {
			choice.Defenses = append(choice.Defenses, boardprice.NoDefense)
		}
	}
	priced, err := priceSheetBoard(b, c.economy.BoardXP, choice)
	priced.sheet.Character = character
	return priced.sheet, err == nil, err
}

// boardSheet is the sheet a save's board is priced on: its own when it holds
// a priced board, otherwise the save moved onto its start board (what its
// first board change does). start is that board, nil when the save holds
// one already.
func boardSheet(contentRoot, loadoutRoot string, c campaignContext, save loadout.CampaignSave) (loadout.Progress, *loadout.SheetBoard, error) {
	if loadout.HoldsBoard(save.Sheet) {
		return save.Sheet, nil, nil
	}
	start, ok, err := startBoard(contentRoot, loadoutRoot, c, save.Character, &save.Sheet)
	if err != nil {
		return save.Sheet, nil, err
	}
	if !ok {
		return save.Sheet, nil, fmt.Errorf("%s does not build its board", save.Character)
	}
	sheet, err := loadout.AdoptBoard(save.Sheet, c.economy, c.catalogs[save.Character], start)
	return sheet, &start, err
}

// boardXPView prices a board on a sheet: every step's level XP, every menu
// defense's price on the board's die at the sheet's total XP and, per
// defense slot, what picking it changes against the saved board's defense
// there (negative: XP back), and what saving the board would cost
// (negative: a refund) and leave.
func boardXPView(b *boardprice.Builder, e loadout.Economy, lib content.BattleLibrary, p loadout.Progress, priced pricedBoard) (map[string]any, []int, error) {
	std := b.Menus.Pricing.Target
	stepXP := make([]int, len(priced.staircase.Steps))
	for k, step := range priced.staircase.Steps {
		stepXP[k] = e.BoardXP.LevelXP(priced.staircase, std, step.At)
	}
	onDie := p
	onDie.Dice = priced.choice.Die
	total := *p.Budget
	// The saved board's defense in each slot and the XP it holds: a menu
	// defense's change is its price less that (negative: XP back).
	saved := p.Abilities.Defensive
	if p.BoardChoice != nil {
		saved = p.BoardChoice.Defenses
	}
	if saved == nil {
		saved = []string{}
	}
	slots := len(priced.choice.Defenses)
	savedXP := make([]int, slots)
	for i := range savedXP {
		if i < len(saved) && saved[i] != boardprice.NoDefense {
			xp, _, err := loadout.DefensePrice(e, lib, p, saved[i], total)
			if err != nil {
				return nil, nil, err
			}
			savedXP[i] = xp
		}
	}
	noneChange := make([]int, slots)
	for i, xp := range savedXP {
		noneChange[i] = -xp
	}
	var defenses []map[string]any
	for _, d := range b.Menus.Defenses {
		xp, value, err := loadout.DefensePrice(e, lib, onDie, d.Ability, total)
		if err != nil {
			return nil, nil, err
		}
		change := make([]int, slots)
		for i := range change {
			change[i] = xp - savedXP[i]
		}
		entry := map[string]any{"ability": d.Ability, "name": lib.Abilities[d.Ability].Name, "xp": xp, "change": change, "blocks": value.Blocks, "prevention": value.Prevention, "block": value.Block, "chosen": slices.Contains(priced.choice.Defenses, d.Ability)}
		if value.Blocks {
			entry["share"] = e.BoardXP.BlockShare(value)
		}
		defenses = append(defenses, entry)
	}
	held := 0
	if p.BoardValue != nil {
		held = *p.BoardValue
	}
	view := map[string]any{
		"available": p.XP, "total": total, "held": held, "level_xp": priced.sheet.LevelXP,
		"defenses": defenses, "rates": e.BoardXP,
		// What emptying each slot changes (the saved defense's XP back).
		"none_change": noneChange, "saved_defenses": saved,
	}
	cost, value, err := loadout.BoardChangeCost(p, e, lib, priced.sheet)
	view["cost"], view["value"], view["left"] = cost, value, p.XP-cost
	if err != nil {
		view["reason"] = err.Error()
	}
	view["affordable"] = err == nil
	return view, stepXP, nil
}

// staircaseView is the damage slider's data: each step's target, average,
// tier damage (report order), the tiers it raises and the XP its level
// holds.
func staircaseView(b *boardprice.Builder, priced pricedBoard, stepXP []int) map[string]any {
	sc, report := priced.staircase, priced.report
	steps := make([]map[string]any, len(sc.Steps))
	for k, step := range sc.Steps {
		raised := make([]string, len(step.Raised))
		for j, i := range step.Raised {
			raised[j] = report.Tiers[i].Tier
		}
		steps[k] = map[string]any{"at": step.At, "average": step.Average, "no_hit": step.NoHit, "damage": step.Damage, "raised": raised}
		if stepXP != nil {
			steps[k]["xp"] = stepXP[k]
		}
	}
	std := b.Menus.Pricing.Target
	return map[string]any{"from": sc.From, "to": sc.To, "default_target": std, "lowest": sc.At(std), "target": b.LevelOf(priced.choice), "step": report.Step, "steps": steps, "raises": tierRaises(priced, stepXP)}
}

// tierRaises answers "what does one more point on this tier cost?" for
// every tier (report order): the first step above the shown one where the
// tier deals more, its target, the tier's damage there, the XP that climb
// adds (when the steps are priced) and the other tiers the climb raises on
// the way, since the board rises as a whole. A tier at the top has none.
func tierRaises(priced pricedBoard, stepXP []int) []any {
	sc, report := priced.staircase, priced.report
	k := report.Step
	out := make([]any, len(report.Tiers))
	if k < 0 || k >= len(sc.Steps) {
		return out
	}
	for i, tier := range report.Tiers {
		now := sc.Steps[k].Damage[i]
		for j := k + 1; j < len(sc.Steps); j++ {
			if sc.Steps[j].Damage[i] <= now {
				continue
			}
			also := []map[string]any{}
			for t, other := range report.Tiers {
				if t != i && sc.Steps[j].Damage[t] > sc.Steps[k].Damage[t] {
					also = append(also, map[string]any{"tier": other.Tier, "slot": other.Slot, "from": sc.Steps[k].Damage[t], "to": sc.Steps[j].Damage[t]})
				}
			}
			raise := map[string]any{"tier": tier.Tier, "slot": tier.Slot, "step": j, "at": sc.Steps[j].At, "from": now, "to": sc.Steps[j].Damage[i], "average": sc.Steps[j].Average, "also": also}
			if stepXP != nil {
				raise["xp"] = stepXP[j] - stepXP[k]
			}
			out[i] = raise
			break
		}
	}
	return out
}

// sameBuild reports a choice that keeps a sheet's die, template and slot
// picks: campaigns change only the damage level and the defenses for now.
func sameBuild(a, b boardprice.Choice) bool {
	return a.Die == b.Die && a.Template == b.Template && slices.Equal(a.Slots, b.Slots)
}

// campaignBoardView is a campaign or expedition save's board, priced at the
// given choice's level and defenses (the save's own board when empty).
func campaignBoardView(contentRoot, loadoutRoot, saveID string, choice boardprice.Choice) (map[string]any, error) {
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadCampaignSave(loadoutRoot, saveID, c.economy, c.catalogs)
	if err != nil {
		return nil, err
	}
	sheet, start, err := boardSheet(contentRoot, loadoutRoot, c, save)
	if err != nil {
		return nil, err
	}
	b, err := boardBuilder(contentRoot)
	if err != nil {
		return nil, err
	}
	e, err := loadout.EffectiveEconomy(loadoutRoot, c.economy)
	if err != nil {
		return nil, err
	}
	lib := c.catalogs[save.Character]
	saved, err := sheetChoice(b, lib, sheet)
	if err != nil {
		return nil, err
	}
	savedResolved, _, _, err := b.Resolve(saved)
	if err != nil {
		return nil, err
	}
	if choice.Template == "" {
		choice = savedResolved
	}
	priced, err := priceSheetBoard(b, e.BoardXP, choice)
	if err != nil {
		return nil, err
	}
	if !sameBuild(priced.choice, savedResolved) {
		return nil, fmt.Errorf("a campaign keeps its die, template and slot picks; only the damage level and defenses change")
	}
	xp, stepXP, err := boardXPView(b, e, lib, sheet, priced)
	if err != nil {
		return nil, err
	}
	tpl, _ := b.Menus.Template(priced.choice.Template)
	die, _ := b.Die(priced.choice.Die)
	var tiers []map[string]any
	for _, t := range priced.report.Tiers {
		cost, _ := boardprice.ParseCost(t.Tier)
		tiers = append(tiers, map[string]any{"slot": t.Slot, "tier": t.Tier, "label": b.Label(die, cost)})
	}
	var abilities []map[string]any
	for _, a := range priced.abilities {
		abilities = append(abilities, map[string]any{"id": a.ID, "name": a.Name, "rules_text": a.Presentation.RulesText})
	}
	return map[string]any{
		"id": save.ID, "revision": save.Sheet.Revision, "mode": save.Mode, "character": save.Character,
		// moving: the save still holds its authored abilities; its first
		// change moves it onto this board (as part of its kit).
		"moving": start != nil,
		"choice": priced.choice, "saved": savedResolved, "die_name": die.Name, "template_name": tpl.Name, "defense_slots": tpl.Defenses,
		"average": priced.report.WholeAverage, "no_hit": priced.report.WholeNoHit,
		"tiers": tiers, "abilities": abilities, "staircase": staircaseView(b, priced, stepXP), "xp": xp,
	}, nil
}

// campaignBoardChange saves a campaign or expedition save's new damage level
// and defenses for XP; an expedition pays the visit's Influence at a site or
// camp like any deck change.
func campaignBoardChange(contentRoot, loadoutRoot, saveID string, choice boardprice.Choice, revision, expectedCost int) (map[string]any, error) {
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		return nil, err
	}
	save, err := loadout.ReadCampaignSave(loadoutRoot, saveID, x.economy, x.catalogs)
	if err != nil {
		return nil, err
	}
	sheet, start, err := boardSheet(contentRoot, loadoutRoot, x.campaignContext, save)
	if err != nil {
		return nil, err
	}
	b, err := boardBuilder(contentRoot)
	if err != nil {
		return nil, err
	}
	lib := x.catalogs[save.Character]
	saved, err := sheetChoice(b, lib, sheet)
	if err != nil {
		return nil, err
	}
	savedResolved, _, _, err := b.Resolve(saved)
	if err != nil {
		return nil, err
	}
	priced, err := priceSheetBoard(b, x.economy.BoardXP, choice)
	if err != nil {
		return nil, err
	}
	if !sameBuild(priced.choice, savedResolved) {
		return nil, fmt.Errorf("a campaign keeps its die, template and slot picks; only the damage level and defenses change")
	}
	if save.IsExpedition() {
		save, err = loadout.ExpeditionBoardChange(loadoutRoot, saveID, x.economy, x.catalogs, x.rules[save.Character], start, priced.sheet, revision, expectedCost)
	} else {
		save, err = loadout.CampaignBoardChange(loadoutRoot, saveID, x.economy, x.catalogs, start, priced.sheet, revision, expectedCost)
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{"id": save.ID, "revision": save.Sheet.Revision, "xp": save.Sheet.XP, "xp_total": *save.Sheet.Budget, "board_choice": save.Sheet.BoardChoice, "ability_board": save.Sheet.Abilities}
	if save.IsExpedition() {
		out["view"] = save.Expedition.View(x.rules[save.Character], save.Sheet.Revision)
	}
	return out, nil
}
