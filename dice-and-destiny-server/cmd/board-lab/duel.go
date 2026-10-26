package main

// Duels with the cards taken out, shared by the pure duel (-pure) and the
// round robin (-roundrobin). Both sides are the custom adventurer's resources
// with a deck of blank cards (health only, nothing to play), one defense and
// no riders, so only the boards and their dice differ. The battles run on a
// scratch copy of the content holding the blank cards, the lab's own
// defenses and a character for each opponent board; the real content is
// never written.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/content"
)

const (
	blankCard = "zz_blank"
	// blankCopies is the most copies of one card a deck may hold; a blank
	// deck spreads its health over as many blank cards as it needs.
	blankCopies = 20
	// noDefense is Holdfast with its prevention replaced by energy (the
	// engine needs a defense on every board): -defense none.
	noDefense = "zz_no_defense"
	// lightGuard rolls one die: 1–2 prevent nothing, 3–4 prevent 1 and
	// 5–6 prevent 2 (-defense light).
	lightGuard = "zz_light_guard"
	// Hypothetical defenses for the pricing research (-exchange-check):
	// light2 rolls two Light Guard dice (2 on average); heavy and bulwark
	// roll three of your dice like Holdfast, but the apex face prevents 2
	// (heavy, 4.5 on average) and also each side face 3 (bulwark, 5.5).
	lightGuard2 = "zz_light_guard_2"
	// halfGuard rolls one die: 1–3 prevent nothing, 4–6 prevent 1 (0.5 on average).
	halfGuard = "zz_half_guard"
	// blockGuard2 rolls two dice: any 6 prevents the whole attack (99).
	blockGuard2 = "zz_block_guard_2"
	heavyGuard  = "zz_heavy_guard"
	bulwark     = "zz_bulwark"
)

// The duel defenses: none, light (Light Guard) and holdfast.
var duelDefenses = []string{"none", "light", "holdfast"}

// defenseAbility is the defensive ability a duel defense puts on both sides.
func defenseAbility(defense string) string {
	switch defense {
	case "none":
		return noDefense
	case "light":
		return lightGuard
	case "light2":
		return lightGuard2
	case "half":
		return halfGuard
	case "block2":
		return blockGuard2
	case "heavy":
		return heavyGuard
	case "bulwark":
		return bulwark
	}
	return defense
}

// defenseOdds is what a duel defense prevents when rolled on a die: odds[k]
// is the chance of preventing k. Holdfast rolls three dice by role: main and
// odd faces prevent 1, side 2, the second odd (3-1-1-1) 3 and apex 0.
func defenseOdds(defense string, die boardprice.Die) []float64 {
	switch defense {
	case "none":
		return []float64{1}
	case "light":
		return []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
	case "light2":
		return []float64{1.0 / 9, 2.0 / 9, 3.0 / 9, 2.0 / 9, 1.0 / 9}
	case "half":
		return []float64{0.5, 0.5}
	case "block2":
		odds := make([]float64, 100)
		odds[0], odds[99] = 25.0/36, 11.0/36
		return odds
	case "holdfast", "holdfast_plus", "heavy", "bulwark":
		prevents := map[string]int{"main": 1, "odd": 1, "side": 2, "odd_b": 3, "apex": 0}
		if defense == "heavy" {
			prevents["apex"] = 2
		}
		if defense == "bulwark" {
			prevents["apex"], prevents["side"] = 2, 3
		}
		dist := []float64{1}
		for i := 0; i < 3; i++ {
			next := make([]float64, len(dist)+3)
			for k, pk := range dist {
				for _, f := range die.Faces {
					next[k+prevents[die.Layout.Roles[f.Symbol]]] += pk / float64(len(die.Faces))
				}
			}
			dist = next
		}
		return dist
	}
	fail(fmt.Errorf("unknown duel defense %q (none, light or holdfast)", defense))
	return nil
}

// duelist is an opponent character carrying a priced board: the custom
// adventurer's resources, the board's generated abilities under the
// character's own prefix and names, the duel's defense and a blank deck.
type duelist struct {
	ID, Prefix, Label string
	Choice            boardprice.Choice
	Report            boardprice.Report
}

// writeScratch copies the content to scratch and adds the blank cards for
// up to maxHealth, the lab's own defenses and the duelists to the custom
// adventurer's pack there.
func (l lab) writeScratch(scratch string, maxHealth int, defense string, duelists []duelist) error {
	if err := copyTree(l.contentRoot, scratch); err != nil {
		return err
	}
	pack := filepath.Join(scratch, battle.CustomAdventurerPack)
	for i, entry := range blankDeck(maxHealth) {
		blank := map[string]any{
			"schema_version":  1,
			"id":              entry.CardID,
			"name":            fmt.Sprintf("Blank %d", i+1),
			"type":            "blank",
			"presentation":    map[string]any{"rules_text": "Blank. This card has no effect and cannot be played; it only counts as health."},
			"cost":            map[string]any{"energy": 0},
			"play":            map[string]any{"source_zones": []string{"hand"}, "destination": "discard", "playable_during": []any{}},
			"targeting":       map[string]any{"selector": "self", "minimum": 0, "maximum": 1},
			"reaction_window": map[string]any{"opens": false},
			"operations":      []any{},
		}
		if err := writeJSON(filepath.Join(pack, "cards", entry.CardID+".yaml"), blank); err != nil {
			return err
		}
	}
	catalogs, err := learned.CharacterCatalogs(l.contentRoot)
	if err != nil {
		return err
	}
	lib := catalogs[loadout.CustomAdventurer]
	if err := writeNoDefense(filepath.Join(pack, "abilities", noDefense+".yaml"), lib.Abilities["holdfast"]); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(pack, "abilities", lightGuard+".yaml"), lightGuardAbility(lightGuard, "Zz Light Guard", 1)); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(pack, "abilities", lightGuard2+".yaml"), lightGuardAbility(lightGuard2, "Zz Double Light Guard", 2)); err != nil {
		return err
	}
	half := lightGuardAbility(halfGuard, "Zz Half Guard", 1)
	half["presentation"] = map[string]any{"rules_text": "Roll 1 die: 1–3 prevent nothing, 4–6 prevent 1."}
	roll := half["resolution"].(map[string]any)["operations"].([]any)[0].(map[string]any)
	roll["id"] = "half_guard_roll"
	roll["outcomes"] = []any{
		map[string]any{"faces": []int{1, 2, 3}, "operations": []any{map[string]any{"type": "gain_resource", "target": "self", "amount": 1, "resource": "energy"}}},
		map[string]any{"faces": []int{4, 5, 6}, "operations": []any{map[string]any{"type": "prevent_damage", "target": "selected_proposal", "amount": 1}}},
	}
	if err := writeJSON(filepath.Join(pack, "abilities", halfGuard+".yaml"), half); err != nil {
		return err
	}
	block := lightGuardAbility(blockGuard2, "Zz Block Guard", 2)
	block["presentation"] = map[string]any{"rules_text": "Roll 2 dice: any 6 prevents all damage from the attack."}
	broll := block["resolution"].(map[string]any)["operations"].([]any)[0].(map[string]any)
	broll["id"] = "block_guard_roll"
	broll["outcomes"] = []any{
		map[string]any{"faces": []int{1, 2, 3, 4, 5}, "operations": []any{map[string]any{"type": "gain_resource", "target": "self", "amount": 1, "resource": "energy"}}},
		map[string]any{"faces": []int{6}, "operations": []any{map[string]any{"type": "prevent_damage", "target": "selected_proposal", "amount": boardprice.BlockAmount}}},
	}
	if err := writeJSON(filepath.Join(pack, "abilities", blockGuard2+".yaml"), block); err != nil {
		return err
	}
	if err := writeRoleDefense(filepath.Join(pack, "abilities", heavyGuard+".yaml"), lib.Abilities["holdfast"], heavyGuard, "Zz Heavy Guard", 2, 2); err != nil {
		return err
	}
	if err := writeRoleDefense(filepath.Join(pack, "abilities", bulwark+".yaml"), lib.Abilities["holdfast"], bulwark, "Zz Bulwark Guard", 3, 2); err != nil {
		return err
	}
	for _, d := range duelists {
		abilities, board, err := l.builder.Generate(d.Choice, d.Report)
		if err != nil {
			return err
		}
		rename := func(id string) string { return d.Prefix + strings.TrimPrefix(id, boardprice.GeneratedPrefix) }
		for _, a := range abilities {
			a.ID, a.Name = rename(a.ID), d.Label+" "+a.Name
			if err := writeJSON(filepath.Join(pack, "abilities", a.ID+".yaml"), a); err != nil {
				return err
			}
		}
		c := lib.Combatants[loadout.CustomAdventurer]
		c.ID, c.Name = d.ID, "Zz "+d.Label
		c.DiceLoadout = []content.DiceLoadoutEntry{{DiceID: d.Choice.Die, Count: content.LoadoutDiceCount}}
		c.AbilityBoard = content.AbilityBoard{Defensive: []string{defenseAbility(defense)}}
		for _, id := range board.Offensive {
			c.AbilityBoard.Offensive = append(c.AbilityBoard.Offensive, rename(id))
		}
		// Battles replace the deck (HeroDeck/FoeDeck); this one only has to
		// be valid.
		c.Decklist = []content.DecklistEntry{{CardID: blankDeck(1)[0].CardID, Count: 1}}
		raw, err := yaml.Marshal(c)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(pack, "combatants", d.ID+".yaml"), raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// playDuel writes a board with the duel's defense to a scratch sheet and
// plays it against an opponent duelist from both seats on paired seeds.
func (l lab) playDuel(contentRoot string, health int, defense string, choice boardprice.Choice, report boardprice.Report, opponent string, battles int) lookahead.Summary {
	root, err := os.MkdirTemp("", "board-lab-sheet-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(root)
	abilities, board, err := l.builder.Generate(choice, report)
	if err != nil {
		fail(err)
	}
	generated := map[string]content.BattleAbilityDefinition{}
	for _, a := range abilities {
		generated[a.ID] = a
	}
	board.Defensive = []string{defenseAbility(defense)}
	scratch := l
	scratch.contentRoot = contentRoot
	if err := scratch.writeBoard(root, loadout.SheetBoard{Dice: choice.Die, Abilities: board, Generated: generated}); err != nil {
		fail(fmt.Errorf("%+v: %w", choice, err))
	}
	runState, err := os.MkdirTemp("", "board-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	deck := blankDeck(health)
	var records []lookahead.BattleRecord
	for _, seatB := range []bool{false, true} {
		m := lookahead.Matchup{ContentRoot: contentRoot, AuthoredRoot: root, RunStateRoot: runState, Hero: loadout.CustomAdventurer, Opponent: opponent, Unified: true, HeroPlansFirst: true, HeroInSeatB: seatB, HeroDeck: deck, FoeDeck: deck}
		summary, err := lookahead.RunBattles(m, greedy, greedy, l.first, battles/2, l.workers, nil)
		if err != nil {
			fail(fmt.Errorf("%s · %s vs %s: %w", choice.Template, choice.Die, opponent, err))
		}
		records = append(records, summary.Records...)
	}
	return lookahead.Summarize(records)
}

// blankDeck is health blank cards over as few blank cards as the per-card
// limit allows.
func blankDeck(health int) []loadout.Entry {
	var out []loadout.Entry
	for left, i := health, 1; left > 0; i++ {
		n := min(left, blankCopies)
		out = append(out, loadout.Entry{CardID: fmt.Sprintf("%s_%d", blankCard, i), Count: n})
		left -= n
	}
	return out
}

// writeNoDefense writes Holdfast with every prevention replaced by 1 energy,
// which blank decks cannot spend: it rolls and prevents nothing.
func writeNoDefense(path string, holdfast content.BattleAbilityDefinition) error {
	raw, err := json.Marshal(holdfast)
	if err != nil {
		return err
	}
	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		return err
	}
	def["id"], def["name"] = noDefense, "No Defense"
	def["presentation"] = map[string]any{"rules_text": "Rolls 3 dice and prevents nothing; each symbol gains energy."}
	resolution, _ := def["resolution"].(map[string]any)
	operations, _ := resolution["operations"].([]any)
	if len(operations) != 1 {
		return fmt.Errorf("holdfast: expected one roll, got %d operations", len(operations))
	}
	roll, _ := operations[0].(map[string]any)
	outcomes, _ := roll["outcomes"].([]any)
	for _, o := range outcomes {
		outcome, _ := o.(map[string]any)
		ops, _ := outcome["operations"].([]any)
		for i, op := range ops {
			if m, _ := op.(map[string]any); m["type"] == "prevent_damage" {
				ops[i] = map[string]any{"type": "gain_resource", "target": "self", "amount": 1, "resource": "energy"}
			}
		}
	}
	return writeJSON(path, def)
}

// lightGuardAbility is the weak defense: one die, 1–2 gain 1 energy (which
// blank decks cannot spend), 3–4 prevent 1, 5–6 prevent 2.
func lightGuardAbility(id, name string, dice int) map[string]any {
	prevent := func(n int) []any {
		return []any{map[string]any{"type": "prevent_damage", "target": "selected_proposal", "amount": n}}
	}
	return map[string]any{
		"schema_version": 1,
		"id":             id,
		"name":           name,
		"type":           "defensive",
		"presentation":   map[string]any{"rules_text": "Roll 1 die: 1–2 prevent nothing, 3–4 prevent 1, 5–6 prevent 2."},
		"cost":           map[string]any{"energy": 0},
		"usage":          map[string]any{"maximum_per_segment": 1},
		"selection":      map[string]any{"requires_incoming_proposal": true, "allowed_proposal_types": []string{"damage_source"}, "target_count": 1},
		"resolution": map[string]any{
			"reaction_window": map[string]any{"opens": true, "pass_required": true},
			"operations": []any{map[string]any{
				"id": "light_guard_roll", "type": "roll_dice", "target": "self", "dice_id": "standard_d6", "dice_count": dice,
				"reaction_window": map[string]any{"opens": true, "pass_required": true},
				"outcomes": []any{
					map[string]any{"faces": []int{1, 2}, "operations": []any{map[string]any{"type": "gain_resource", "target": "self", "amount": 1, "resource": "energy"}}},
					map[string]any{"faces": []int{3, 4}, "operations": prevent(1)},
					map[string]any{"faces": []int{5, 6}, "operations": prevent(2)},
				},
			}},
		},
		"saved_card_destination": "discard",
	}
}

// writeRoleDefense writes Holdfast with its outcomes replaced: main and odd
// faces prevent 1, side faces `side`, the second odd 3 and apex `apex`.
func writeRoleDefense(path string, holdfast content.BattleAbilityDefinition, id, name string, side, apex int) error {
	raw, err := json.Marshal(holdfast)
	if err != nil {
		return err
	}
	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		return err
	}
	prevent := func(n int) []any {
		return []any{map[string]any{"type": "prevent_damage", "target": "selected_proposal", "amount": n}}
	}
	def["id"], def["name"] = id, name
	def["presentation"] = map[string]any{"rules_text": fmt.Sprintf("Roll 3 of your dice: main and odd symbols prevent 1, side %d, the second odd 3, apex %d.", side, apex)}
	resolution, _ := def["resolution"].(map[string]any)
	operations, _ := resolution["operations"].([]any)
	if len(operations) != 1 {
		return fmt.Errorf("holdfast: expected one roll, got %d operations", len(operations))
	}
	roll, _ := operations[0].(map[string]any)
	roll["outcomes"] = []any{
		map[string]any{"roles": []string{"main", "odd"}, "operations": prevent(1)},
		map[string]any{"roles": []string{"side"}, "operations": prevent(side)},
		map[string]any{"roles": []string{"odd_b"}, "operations": prevent(3)},
		map[string]any{"roles": []string{"apex"}, "operations": prevent(apex)},
	}
	return writeJSON(path, def)
}

// damageOdds is a board's damage per offensive turn under the pricing's
// play: odds[d] is the chance of dealing d (a miss deals 0).
func damageOdds(r boardprice.Report) []float64 {
	odds := []float64{1}
	for _, t := range r.Tiers {
		for len(odds) <= t.WholeDamage {
			odds = append(odds, 0)
		}
		odds[t.WholeDamage] += t.UseWhole
		odds[0] -= t.UseWhole
	}
	odds[0] = math.Max(0, odds[0])
	return odds
}

// defend is the damage that gets through a defense roll, prevent[k] being
// the chance it prevents k.
func defend(odds, prevent []float64) []float64 {
	out := make([]float64, len(odds))
	for d, pd := range odds {
		for k, pk := range prevent {
			out[max(0, d-k)] += pd * pk
		}
	}
	return out
}

// through is the average damage through a defense.
func through(odds, prevent []float64) float64 {
	total := 0.0
	for d, p := range defend(odds, prevent) {
		total += float64(d) * p
	}
	return total
}

// spread is the standard deviation of the damage per turn.
func spread(odds []float64) float64 {
	m, square := 0.0, 0.0
	for d, p := range odds {
		m += float64(d) * p
		square += float64(d*d) * p
	}
	return math.Sqrt(math.Max(0, square-m*m))
}

// killRounds is the chance that damage per round drawn from odds first
// reaches health on round t (index t-1), up to the rounds where almost
// nothing is left alive.
func killRounds(odds []float64, health int) []float64 {
	alive := make([]float64, health) // alive[h]: dealt h so far, h < health
	alive[0] = 1
	var out []float64
	for t := 0; t < 500; t++ {
		next := make([]float64, health)
		killed := 0.0
		for h, ph := range alive {
			if ph == 0 {
				continue
			}
			for d, pd := range odds {
				if h+d >= health {
					killed += ph * pd
				} else {
					next[h+d] += ph * pd
				}
			}
		}
		out = append(out, killed)
		alive = next
		left := 0.0
		for _, ph := range alive {
			left += ph
		}
		if left < 1e-12 {
			break
		}
	}
	return out
}

// race is the expected score of a side whose damage through the other's
// defense is `ours` against one dealing `theirs`, both attacking every round
// and starting at `health`: a win if it reaches the other's health first, a
// half if both fall in the same round.
func race(ours, theirs []float64, health int) float64 {
	return raceRounds(killRounds(ours, health), killRounds(theirs, health))
}

// raceRounds is race from the two sides' kill-round distributions.
func raceRounds(a, b []float64) float64 {
	theirLeft := 1.0 // chance they have not killed us before round t
	score := 0.0
	for t := 0; t < len(a); t++ {
		bt := 0.0
		if t < len(b) {
			bt = b[t]
		}
		score += a[t] * (theirLeft - bt + 0.5*bt)
		theirLeft -= bt
	}
	return score
}

func correlation(x, y []float64) float64 {
	mx, my := mean(x), mean(y)
	sxy, sxx, syy := 0.0, 0.0, 0.0
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0
	}
	return sxy / math.Sqrt(sxx*syy)
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}
