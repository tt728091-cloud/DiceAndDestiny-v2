package loadout

import (
	"fmt"
	"slices"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/content"
)

// A priced board holds XP (economy.yaml board_xp): its damage level and each
// defense on it. It is part of the sheet's total XP, like the deck: Budget =
// XP + DeckValue + CollectionValue + WoundValue + UpgradeSpent + BoardValue.
// The custom adventurer's sheets always hold one; a campaign save of another
// character holds one once it has a board choice (campaign.yaml boards).
// Authored boards are free.

// HoldsBoard reports a sheet whose board is priced in XP.
func HoldsBoard(p Progress) bool {
	return p.Character == CustomAdventurer || p.BoardChoice != nil
}

// putBoard gives a sheet a priced board: its die, abilities, choice, level
// XP and the generated abilities the board uses.
func putBoard(p *Progress, board SheetBoard) {
	p.Dice, p.BoardChoice, p.BoardLevelXP = board.Dice, board.Choice, board.LevelXP
	p.Abilities = content.AbilityBoard{Offensive: append([]string(nil), board.Abilities.Offensive...), Defensive: append([]string{}, board.Abilities.Defensive...)}
	p.GeneratedAbilities = nil
	for _, id := range board.Abilities.Offensive {
		if ability, ok := board.Generated[id]; ok {
			if p.GeneratedAbilities == nil {
				p.GeneratedAbilities = map[string]content.BattleAbilityDefinition{}
			}
			p.GeneratedAbilities[id] = ability
		}
	}
}

// AdoptBoard moves a campaign sheet that still holds its authored abilities
// onto a priced board. The board is part of the kit: the total grows by its
// value and the XP to spend stays the same. XP already spent upgrading the
// abilities it replaces (Guard → Guard+) moves from UpgradeSpent onto the
// board, where the defense that replaces them is priced.
func AdoptBoard(p Progress, e Economy, lib content.BattleLibrary, start SheetBoard) (Progress, error) {
	if HoldsBoard(p) {
		return p, nil
	}
	if start.Choice == nil {
		return p, fmt.Errorf("%s has no priced board to move onto", p.Character)
	}
	if p.Budget == nil {
		return p, fmt.Errorf("the sheet has no XP total yet")
	}
	paths := e.Characters[p.Character].AbilityUpgrades
	spent := 0
	for _, id := range append(slices.Clone(p.Abilities.Offensive), p.Abilities.Defensive...) {
		spent += legacyAbilitySpend(id, paths, map[string]bool{})
	}
	spent = min(spent, p.UpgradeSpent)
	next := cloneProgress(p)
	putBoard(&next, start)
	next.UpgradeSpent -= spent
	base := *p.Budget - spent
	value, err := boardValueWithin(next, e, lib, base)
	if err != nil {
		return p, err
	}
	budget := base + value
	next.Budget, next.BoardValue = &budget, &value
	next.Revision++
	if err := validateProgress(next, lib); err != nil {
		return p, err
	}
	return next, nil
}

// boardDie is the die a sheet's defenses roll: the sheet's own, or the
// combatant's.
func boardDie(p Progress, lib content.BattleLibrary) (content.BattleDieDefinition, error) {
	id := p.Dice
	if id == "" {
		if loadout := lib.Combatants[p.Character].DiceLoadout; len(loadout) > 0 {
			id = loadout[0].DiceID
		}
	}
	die, ok := lib.Dice[id]
	if !ok {
		return die, fmt.Errorf("unknown die %q", id)
	}
	return die, nil
}

// DefensePrice is one defense's XP price on a sheet's die at a total XP.
func DefensePrice(e Economy, lib content.BattleLibrary, p Progress, id string, total int) (int, boardprice.DefenseValue, error) {
	ability, ok := lib.Abilities[id]
	if !ok {
		return 0, boardprice.DefenseValue{}, fmt.Errorf("unknown defense %q", id)
	}
	die, err := boardDie(p, lib)
	if err != nil {
		return 0, boardprice.DefenseValue{}, err
	}
	value, err := boardprice.ValueDefense(ability, die)
	if err != nil {
		return 0, value, err
	}
	return e.BoardXP.DefenseXP(id, value, total), value, nil
}

// boardValue is the XP a sheet's board holds at a total XP: its level XP and
// the price of every defense on it.
func boardValue(p Progress, e Economy, lib content.BattleLibrary, total int) (int, error) {
	if !HoldsBoard(p) {
		return 0, nil
	}
	value := p.BoardLevelXP
	for _, id := range p.Abilities.Defensive {
		xp, _, err := DefensePrice(e, lib, p, id, total)
		if err != nil {
			return 0, err
		}
		value += xp
	}
	return value, nil
}

// boardValueWithin solves a board's value when the total XP includes it
// (a new sheet, or one older than board pricing): total = base + value.
// Block prices grow by less than 1 XP per XP, so the loop settles.
func boardValueWithin(p Progress, e Economy, lib content.BattleLibrary, base int) (int, error) {
	value := 0
	for i := 0; i < 64; i++ {
		next, err := boardValue(p, e, lib, base+value)
		if err != nil {
			return 0, err
		}
		if next == value {
			break
		}
		value = next
	}
	return value, nil
}

func sheetBoardValue(p Progress) int {
	if p.BoardValue == nil {
		return 0
	}
	return *p.BoardValue
}

// applyBoardChange puts a board the authority priced on a sheet and settles
// its XP: the XP left is the total less everything the sheet holds, and must
// not go below 0. It returns the XP the change cost (negative for a refund).
func applyBoardChange(p Progress, e Economy, lib content.BattleLibrary, sheet SheetBoard) (Progress, int, error) {
	if !HoldsBoard(p) {
		return p, 0, fmt.Errorf("%s does not build its board", p.Character)
	}
	used := map[string]content.BattleAbilityDefinition{}
	for _, id := range sheet.Abilities.Offensive {
		if ability, ok := sheet.Generated[id]; ok {
			used[id] = ability
		}
	}
	sheet.Generated = used
	if _, err := ApplySheetBoard(lib, p.Character, sheet); err != nil {
		return p, 0, err
	}
	if err := e.Access.ValidateBoard(p.Character, sheet.Abilities); err != nil {
		return p, 0, err
	}
	next := cloneProgress(p)
	next.Dice = sheet.Dice
	next.Abilities = content.AbilityBoard{Offensive: append([]string(nil), sheet.Abilities.Offensive...), Defensive: append([]string{}, sheet.Abilities.Defensive...)}
	next.GeneratedAbilities = nil
	if len(used) > 0 {
		next.GeneratedAbilities = used
	}
	next.BoardChoice = sheet.Choice
	next.BoardLevelXP = sheet.LevelXP
	value, err := boardValue(next, e, lib, *next.Budget)
	if err != nil {
		return p, 0, err
	}
	cost := value - sheetBoardValue(p)
	if cost > p.XP {
		return p, cost, fmt.Errorf("not enough XP: the board costs %d XP more and %d XP is left", cost, p.XP)
	}
	next.BoardValue = &value
	next.XP = p.XP - cost
	next.Revision++
	if err := validateProgress(next, lib); err != nil {
		return p, cost, err
	}
	return next, cost, nil
}

// BoardChangeCost previews a board change on a sheet: the XP it costs
// (negative for a refund) and the board's value after it.
func BoardChangeCost(p Progress, e Economy, lib content.BattleLibrary, sheet SheetBoard) (cost, value int, err error) {
	next, cost, err := applyBoardChange(p, e, lib, sheet)
	if err != nil {
		return cost, 0, err
	}
	return cost, sheetBoardValue(next), nil
}

// CampaignBoardChange puts a priced board on a linear campaign save and
// settles its XP. expectedCost is the cost the player was shown. start is
// the board a save still holding its authored abilities moves onto first
// (AdoptBoard), when it has one.
func CampaignBoardChange(root, id string, e Economy, catalogs map[string]content.BattleLibrary, start *SheetBoard, sheet SheetBoard, revision, expectedCost int) (CampaignSave, error) {
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
	if s.IsExpedition() {
		return s, fmt.Errorf("an expedition changes its board only at a site or camp, through the map")
	}
	if revision != s.Sheet.Revision {
		return s, fmt.Errorf("campaign changed; refresh before changing the board")
	}
	current, err := adoptStart(s.Sheet, campaignEconomy(e), catalogs[s.Character], start)
	if err != nil {
		return s, err
	}
	next, cost, err := applyBoardChange(current, campaignEconomy(e), catalogs[s.Character], sheet)
	if err != nil {
		return s, err
	}
	if cost != expectedCost {
		return s, fmt.Errorf("price changed; refresh before changing the board")
	}
	s.Sheet = next
	return s, writeCampaignSave(root, s)
}

// ExpeditionBoardChange puts a priced board on an expedition save at a site
// or camp, paying the visit's Influence like any deck change, and settles
// its XP. expectedCost is the cost the player was shown; start is as for
// CampaignBoardChange.
func ExpeditionBoardChange(root, id string, e Economy, catalogs map[string]content.BattleLibrary, rules expedition.Rules, start *SheetBoard, sheet SheetBoard, revision, expectedCost int) (CampaignSave, error) {
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
	if revision != save.Sheet.Revision {
		return save, fmt.Errorf("campaign changed; refresh before changing the board")
	}
	state := save.Expedition
	if _, _, err = state.DeckChangeCost(rules); err != nil {
		return save, err
	}
	lib := catalogs[save.Character]
	current, err := adoptStart(save.Sheet, campaignEconomy(e), lib, start)
	if err != nil {
		return save, err
	}
	next, cost, err := applyBoardChange(current, campaignEconomy(e), lib, sheet)
	if err != nil {
		return save, err
	}
	if cost != expectedCost {
		return save, fmt.Errorf("price changed; refresh before changing the board")
	}
	if err = state.PayDeckChange(rules); err != nil {
		return save, err
	}
	save.Sheet = next
	return save, finishExpeditionWrite(root, &save, e, lib)
}

// adoptStart moves a sheet onto its start board when it doesn't hold a
// priced board yet and has one to move onto.
func adoptStart(p Progress, e Economy, lib content.BattleLibrary, start *SheetBoard) (Progress, error) {
	if start == nil || HoldsBoard(p) {
		return p, nil
	}
	return AdoptBoard(p, e, lib, *start)
}
