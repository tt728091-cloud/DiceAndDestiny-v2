package learned

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/loadout"
)

// The Board Builder over the custom adventurer's content catalog (never a
// player's sheet), cached per content root until a menu file changes.
var boardBuilders struct {
	sync.Mutex
	entries map[string]boardBuilderEntry
}

type boardBuilderEntry struct {
	stamp   string
	builder *boardprice.Builder
}

func boardBuilder(contentRoot string) (*boardprice.Builder, error) {
	if contentRoot == "" {
		return nil, fmt.Errorf("content_root is required")
	}
	stamp := ""
	for _, name := range []string{"board_templates.yaml", "riders.yaml"} {
		info, err := os.Stat(filepath.Join(contentRoot, "progression_v1", name))
		if err != nil {
			return nil, err
		}
		stamp += fmt.Sprintf("%s:%d:%d;", name, info.Size(), info.ModTime().UnixNano())
	}
	boardBuilders.Lock()
	defer boardBuilders.Unlock()
	if entry, ok := boardBuilders.entries[contentRoot]; ok && entry.stamp == stamp {
		return entry.builder, nil
	}
	lib, err := characterCatalog(contentRoot, loadout.CustomAdventurer)
	if err != nil {
		return nil, err
	}
	builder, err := boardprice.NewBuilder(contentRoot, lib)
	if err != nil {
		return nil, err
	}
	if boardBuilders.entries == nil {
		boardBuilders.entries = map[string]boardBuilderEntry{}
	}
	boardBuilders.entries[contentRoot] = boardBuilderEntry{stamp: stamp, builder: builder}
	return builder, nil
}

func decodeBoardChoice(raw json.RawMessage) (boardprice.Choice, error) {
	var choice boardprice.Choice
	if len(raw) == 0 {
		return choice, nil
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return choice, fmt.Errorf("decode board: %w", err)
	}
	return choice, nil
}

// boardBuilderView is everything the Board Builder screen shows before any
// pricing: the menus, the chosen die's faces and roles, the chase odds and
// band of every cost its family uses, and each of its templates' average and
// no-hit rate at default picks.
func boardBuilderView(contentRoot, loadoutRoot, dieID string) (map[string]any, error) {
	b, err := boardBuilder(contentRoot)
	if err != nil {
		return nil, err
	}
	sheet := map[string]any{}
	if loadoutRoot != "" {
		catalogs, err := CharacterCatalogs(contentRoot, loadoutRoot)
		if err != nil {
			return nil, err
		}
		economy, err := loadout.LoadEconomy(contentRoot, catalogs)
		if err != nil {
			return nil, err
		}
		lib := catalogs[loadout.CustomAdventurer]
		progress, err := loadout.ReadProgress(loadoutRoot, loadout.CustomAdventurer, economy, lib)
		if err != nil {
			return nil, err
		}
		sheet = map[string]any{"revision": progress.Revision, "dice": lib.Combatants[loadout.CustomAdventurer].DiceLoadout[0].DiceID, "ability_board": progress.Abilities}
		if dieID == "" {
			dieID = lib.Combatants[loadout.CustomAdventurer].DiceLoadout[0].DiceID
		}
	}
	if dieID == "" {
		dieID = b.Menus.Dice[0].ID
	}
	die, err := b.Die(dieID)
	if err != nil {
		return nil, err
	}
	var dice []map[string]any
	for _, entry := range b.Menus.Dice {
		d, err := b.Die(entry.ID)
		if err != nil {
			return nil, err
		}
		dice = append(dice, map[string]any{"id": d.ID, "name": d.Name, "family": d.Layout.Family, "order": d.Layout.Order, "faces": b.Faces(d), "unlock": entry.Unlock})
	}
	var riders []map[string]any
	for _, r := range b.Menus.Riders {
		value, _ := b.Menus.Rates.Value(r.Operations)
		riders = append(riders, map[string]any{"id": r.ID, "name": r.Name, "value": value, "jobs": r.Jobs, "unlock": r.Unlock})
	}
	var defenses []map[string]any
	for _, d := range b.Menus.Defenses {
		a := b.Lib.Abilities[d.Ability]
		defenses = append(defenses, map[string]any{"ability": d.Ability, "name": a.Name, "rules_text": a.Presentation.RulesText, "unlock": d.Unlock})
	}
	// Every cost the die's family uses, once: flex menu and core slots.
	var costs []map[string]any
	seen := map[string]bool{}
	addCost := func(text string, flex bool) {
		cost, err := boardprice.ParseCost(text)
		if err != nil || seen[cost.String()] {
			return
		}
		seen[cost.String()] = true
		odds := b.Chase(die, cost)
		costs = append(costs, map[string]any{"cost": cost.String(), "label": b.Label(die, cost), "chase": odds, "band": b.Band(odds), "flex": flex})
	}
	for _, entry := range b.Menus.Families[die.Layout.Family].FlexCosts {
		addCost(entry.Cost, true)
	}
	var templates, stats []map[string]any
	for _, t := range b.Menus.Templates {
		var slots []map[string]any
		for _, s := range t.Slots {
			slots = append(slots, map[string]any{"id": s.ID, "job": s.Job, "job_name": b.Menus.Jobs[s.Job].Name, "cost": s.Cost, "bands": s.Bands, "flex": s.Flex(), "takes_rider": s.TakesRider()})
		}
		templates = append(templates, map[string]any{"id": t.ID, "name": t.Name, "family": t.Family, "defenses": t.Defenses, "slots": slots, "unlock": t.Unlock})
		if t.Family != die.Layout.Family {
			continue
		}
		for _, s := range t.Slots {
			if !s.Flex() {
				addCost(s.Cost, false)
			}
		}
		_, report, err := b.Price(b.Default(t.ID, die.ID))
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", t.ID, err)
		}
		stats = append(stats, map[string]any{"template": t.ID, "average": report.WholeAverage, "no_hit": report.WholeNoHit})
	}
	return map[string]any{
		"pricing": b.Menus.Pricing, "bands": b.Menus.Bands, "jobs": b.Menus.Jobs, "dice": dice, "riders": riders, "defenses": defenses, "templates": templates,
		"die": die.ID, "costs": costs, "template_stats": stats, "sheet": sheet,
	}, nil
}

// boardPriceView prices a board for the preview. It only reads: the
// authority resolves the picks against the menus and prices them; the client
// never computes a price.
func boardPriceView(contentRoot string, choice boardprice.Choice, includePolicy bool) (map[string]any, error) {
	b, err := boardBuilder(contentRoot)
	if err != nil {
		return nil, err
	}
	resolved, report, err := b.Price(choice)
	if err != nil {
		return nil, err
	}
	if !includePolicy {
		report.Policy = boardprice.KeepPolicy{}
	}
	die, _ := b.Die(resolved.Die)
	tpl, _ := b.Menus.Template(resolved.Template)
	var slots []map[string]any
	for i, s := range tpl.Slots {
		pick := resolved.Slots[i]
		cost, _ := boardprice.ParseCost(pick.Cost)
		rider, _ := b.Menus.Rider(pick.Rider)
		var riders []string
		for _, r := range b.Menus.Riders {
			if r.ID == boardprice.RiderNone || (s.TakesRider() && r.RiderFits(s.Job)) {
				riders = append(riders, r.ID)
			}
		}
		slot := map[string]any{"slot": s.ID, "job": s.Job, "job_name": b.Menus.Jobs[s.Job].Name, "cost": pick.Cost, "label": b.Label(die, cost), "rider": pick.Rider, "rider_name": rider.Name, "riders": riders, "flex": s.Flex(), "bands": s.Bands}
		if s.Flex() {
			taken := map[string]bool{}
			for j, other := range resolved.Slots {
				if j == i {
					continue
				}
				otherCost, _ := boardprice.ParseCost(other.Cost)
				for _, tier := range otherCost {
					taken[tier.String()] = true
				}
			}
			var options []map[string]any
			for _, option := range b.FlexCosts(die, tpl.Family, s) {
				odds := b.Chase(die, option)
				options = append(options, map[string]any{"cost": option.String(), "label": b.Label(die, option), "chase": odds, "band": b.Band(odds), "taken": taken[option[0].String()]})
			}
			slot["options"] = options
		}
		slots = append(slots, slot)
	}
	return map[string]any{"choice": resolved, "report": report, "slots": slots, "die_name": die.Name}, nil
}
