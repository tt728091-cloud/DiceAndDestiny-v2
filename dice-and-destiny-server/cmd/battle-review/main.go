// Command battle-review replays finished battles against the look-ahead
// mirror from the game's battle log and writes one HTML page that shows,
// round by round, every roll, reroll, card, ability and defense, with the
// AI's private dice and hand revealed and its choices checked against the
// greedy heuristic and the exact dice solver.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func main() {
	serverRoot := defaultServerRoot()
	workspace := filepath.Join(serverRoot, "..", "dice-and-destiny-client", ".godot", "runtime", "workspace", "client", "user")
	logPath := flag.String("log", filepath.Join(workspace, "phase3", "learned-battles.jsonl"), "the game's battle log")
	loadoutRoot := flag.String("loadout-root", filepath.Join(workspace, "character_loadouts"), "the game's character loadouts (decks must be unchanged since the games)")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards (read only)")
	character := flag.String("character", "", "only battles with this character (default all)")
	version := flag.String("version", "", "only battles against this AI version: v1 or v2 (default all)")
	only := flag.String("games", "", "only these game numbers, e.g. 8,9,10,11 (default all)")
	out := flag.String("out", filepath.Join(workspace, "battle-reviews", "lookahead-mirror-review.html"), "HTML report path")
	flag.Parse()

	// The game launcher points sessions at the published cards this way.
	if os.Getenv(content.AuthoredRootEnv) == "" {
		os.Setenv(content.AuthoredRootEnv, *authoredRoot)
	}
	battles, err := learned.ReadBattleLog(*logPath)
	if err != nil {
		fail(err)
	}
	runState, err := os.MkdirTemp("", "battle-review-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	config := learned.SessionConfig{ContentRoot: filepath.Join(serverRoot, "content"), LoadoutRoot: *loadoutRoot, RunStateRoot: runState, OpponentPolicy: learned.OpponentPolicyLookahead}

	// Games keep their number among all look-ahead games, so a filtered page
	// names them the same way as the full one.
	var games []*game
	number := 0
	for _, battle := range battles {
		if battle.Model.ModelID != learned.LookaheadModelID {
			continue
		}
		number++
		v := "v1"
		if battle.Model.Architecture == learned.LookaheadArchitecture {
			v = "v2"
		}
		if (*character != "" && battle.Character != *character) || (*version != "" && v != *version) || (*only != "" && !listed(*only, number)) {
			continue
		}
		g, err := review(config, battle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", battle.BattleID, err)
			g = &game{Battle: battle, Error: err.Error(), Version: v}
		}
		g.Number = number
		games = append(games, g)
		fmt.Fprintf(os.Stderr, "game %d · %s · %s · %d steps\n", g.Number, battle.Character, battle.Result, len(g.Rows))
	}
	if len(games) == 0 {
		fail(fmt.Errorf("no finished look-ahead mirror battles in %s", *logPath))
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, []byte(render(games)), 0o644); err != nil {
		fail(err)
	}
	fmt.Println(*out)
}

type game struct {
	Version    string
	Number     int
	Battle     learned.LoggedBattle
	Error      string
	Rows       []row
	Rounds     int
	YouHealth  int
	AIHealth   int
	MaxHealth  int
	Overrides  int
	AIChoices  int
	CardTiming map[string]map[string]int // card -> timing -> count
	Abilities  map[string]int
	EarlyStops []string
}

type row struct {
	ID         string
	Round      int
	AI         bool
	Phase      string
	Action     string
	DiceBefore []state.RolledDie
	Dice       []state.RolledDie
	// DiceLabel names the dice step ("roll 1", "reroll", a card name) and
	// Touched the dice it rerolled or changed, for the proof layout.
	DiceLabel string
	Touched   map[int]bool
	Seat      string
	CardID    string
	CardStart bool
	Notes     []string
	Override  bool
	Greedy    string
	Energy    int
	Hand      []string
	Summary   bool
}

func review(config learned.SessionConfig, battle learned.LoggedBattle) (*game, error) {
	g := &game{Battle: battle, CardTiming: map[string]map[string]int{}, Abilities: map[string]int{}}
	sim, err := lookahead.NewSimulator()
	if err != nil {
		return nil, err
	}
	// Compare with the heuristic of the AI version that played the game.
	greedy, version := lookahead.NewGreedyPolicy(), "v2"
	if battle.Model.Architecture != learned.LookaheadArchitecture {
		greedy, version = lookahead.NewGreedyPolicyV1(), "v1"
	}
	g.Version = version
	solver := lookahead.NewDiceSolver()
	rng := rand.New(rand.NewSource(1))
	final, err := learned.ReplayLoggedBattle(config, battle, func(step learned.ReplayStep) error {
		before, after := step.Before, step.After
		lib, err := engine.SettledLibrary(&before)
		if err != nil {
			return err
		}
		seat := step.Decision.ActorSeat
		ai := seat == battle.ModelSeat
		action := step.Decision.Command
		stage := ""
		if before.Settled != nil && before.Settled.Window != nil {
			stage = before.Settled.Window.Stage
		}
		r := row{Round: before.Segment.Round, AI: ai, Phase: phaseName(string(before.Segment.Current), stage), Energy: after.Actors[seat].Resources.EnergyPoints}
		rtBefore, rtAfter := before.Settled.Actors[seat], after.Settled.Actors[seat]
		if action.Type == command.TypePlanningKeep && before.Settled.Actors[seat].AmbushChoose == 0 {
			return nil // marking dice as kept changes nothing; the reroll row shows what was kept
		}
		r.Action = describe(lib, &before, &after, seat, action, stage, step.Events)
		r.Seat = seat
		choice := lookahead.CardChoice(action)
		if ids := lookahead.CardIDs(action); len(ids) == 1 {
			r.CardID, r.CardStart = ids[0], choice.Verb == "start"
		}
		switch {
		case action.Type == command.TypePlanningRoll:
			r.Dice, r.DiceLabel = rtAfter.FinalDice, "roll 1"
		case action.Type == command.TypePlanningReroll:
			var p command.PlanningRerollPayload
			_ = json.Unmarshal(action.Payload, &p)
			r.DiceBefore, r.Dice, r.DiceLabel, r.Touched = rtBefore.FinalDice, rtAfter.FinalDice, fmt.Sprintf("reroll %d", rtBefore.RollsUsed+1), indexSet(p.RerollIndices)
		case choice.Kind == "offensive_die" && stage == "planning":
			r.DiceBefore, r.Dice, r.DiceLabel, r.Touched = rtBefore.FinalDice, rtAfter.FinalDice, cardName(lib, &before, seat, r.CardID), indexSet([]int{choice.Die})
		case len(rtAfter.FinalDice) > 0 && stage == "planning":
			r.Dice, r.DiceLabel = rtAfter.FinalDice, "dice"
		case action.Type == command.TypeRollDice:
			r.Dice, r.DiceLabel = defenseDice(&after, seat), "defense"
		}
		if ai {
			r.Hand = handNames(lib, &after, seat)
			g.AIChoices++
			options := lookahead.Useful(&before, seat, step.Legal)
			if len(options) > 1 {
				d := &lookahead.Decision{Sim: sim, Battle: &before, Seat: seat, Actions: step.Legal, Rand: rng}
				if index, err := greedy.Choose(d); err == nil && !sameCommand(step.Legal[index], action) {
					r.Override = true
					r.Greedy = humanize(lib, &before, seat, step.Legal[index])
					g.Overrides++
				}
			}
			r.Notes = append(r.Notes, diceNotes(solver, lib, &before, seat, action, stage, step.Legal)...)
			noteTendencies(g, lib, &before, seat, action, stage)
		}
		r.ID = fmt.Sprintf("g%d-s%d", battle.Seed%100000, len(g.Rows))
		// A card's choice step joins the row that started the card.
		if last := len(g.Rows) - 1; last >= 0 && !r.CardStart && r.CardID != "" && g.Rows[last].CardStart && g.Rows[last].CardID == r.CardID && g.Rows[last].Seat == seat {
			prev := &g.Rows[last]
			prev.Action += " → " + dieChoiceText(lookahead.CardChoice(action).Label)
			if r.DiceLabel != "" {
				prev.DiceBefore, prev.Dice, prev.DiceLabel, prev.Touched = r.DiceBefore, r.Dice, r.DiceLabel, r.Touched
			}
			prev.Notes = append(prev.Notes, r.Notes...)
			prev.Energy, prev.Hand = r.Energy, r.Hand
			prev.CardStart = false
			if r.Override && !prev.Override {
				prev.Override, prev.Greedy = true, r.Greedy
			}
		} else {
			g.Rows = append(g.Rows, r)
		}
		for _, e := range step.Events {
			if e.Type == event.TypeDamageCommitted {
				g.Rows = append(g.Rows, row{Round: before.Segment.Round, Summary: true, Action: fmt.Sprintf("Round %d damage lands · You %d/%d · AI %d/%d", before.Segment.Round,
					after.Actors[battle.HumanSeat].CurrentHealth(), lookahead.MaxHealth(after.Actors[battle.HumanSeat]), after.Actors[battle.ModelSeat].CurrentHealth(), lookahead.MaxHealth(after.Actors[battle.ModelSeat]))})
			}
		}
		return nil
	})
	if err != nil {
		return g, err
	}
	g.Rounds = final.Segment.Round
	g.YouHealth, g.AIHealth = final.Actors[battle.HumanSeat].CurrentHealth(), final.Actors[battle.ModelSeat].CurrentHealth()
	g.MaxHealth = lookahead.MaxHealth(final.Actors[battle.HumanSeat])
	return g, nil
}

func changesDice(action command.Command, stage string) bool {
	switch action.Type {
	case command.TypePlanningRoll, command.TypePlanningReroll:
		return true
	case command.TypePlanningCards:
		return lookahead.CardChoice(action).Kind == "offensive_die"
	}
	return false
}

func phaseName(segment, stage string) string {
	switch stage {
	case "planning":
		return "Offense"
	case "offensive_reaction":
		return "Offense · reaction"
	case "defense_selection":
		return "Defense"
	case "defense_roll":
		return "Defense · roll"
	case "defense_reaction":
		return "Defense · after roll"
	case "damage_reaction":
		return "Damage"
	case "discard_to_hand_limit":
		return "Hand limit"
	}
	if stage == "" {
		return segment
	}
	return strings.ReplaceAll(stage, "_", " ")
}

func cardName(lib content.BattleLibrary, b *state.Battle, seat, instance string) string {
	definition := b.Settled.Actors[seat].CardInstances[instance].DefinitionID
	if c, ok := lib.Cards[definition]; ok && c.Name != "" {
		return c.Name
	}
	return definition
}

func abilityName(lib content.BattleLibrary, id string) string {
	if a, ok := lib.Abilities[id]; ok && a.Name != "" {
		return a.Name
	}
	return id
}

func handNames(lib content.BattleLibrary, b *state.Battle, seat string) []string {
	var out []string
	for _, id := range b.Actors[seat].Cards.Hand {
		out = append(out, cardName(lib, b, seat, id))
	}
	sort.Strings(out)
	return out
}

// tierDamage is a tier's authored damage plus active card bonuses.
func tierDamage(lib content.BattleLibrary, b *state.Battle, seat, abilityID, tierID string) (int, int) {
	base := 0
	if a, ok := lib.Abilities[abilityID]; ok && a.Qualification != nil {
		for i, tier := range a.Qualification.ActivationTiers {
			if tier.ID == tierID || (tierID == "" && i == 0) {
				for _, op := range tier.Operations {
					if op.Type == "deal_damage" {
						if n, ok := op.Amount.(float64); ok {
							base += int(n)
						} else if n, ok := op.Amount.(int); ok {
							base += n
						}
					}
				}
				break
			}
		}
	}
	bonus := 0
	for _, m := range b.Settled.Actors[seat].AbilityModifiers {
		if m.AbilityID != abilityID {
			continue
		}
		var s content.CardStep
		if json.Unmarshal(m.ProgramBonus, &s) == nil {
			bonus += content.ProgramInt(s, "damage")
		}
	}
	return base, bonus
}

func humanize(lib content.BattleLibrary, b *state.Battle, seat string, action command.Command) string {
	switch action.Type {
	case command.TypePlanningRoll:
		return "roll"
	case command.TypePlanningReroll:
		var p command.PlanningRerollPayload
		_ = json.Unmarshal(action.Payload, &p)
		return "reroll " + diePositions(b, seat, p.RerollIndices)
	case command.TypePlanningAbility:
		var p command.PlanningAbilityPayload
		_ = json.Unmarshal(action.Payload, &p)
		return "choose " + abilityName(lib, p.AbilityID) + tierLabel(p.TierID)
	case command.TypePlanningPass, command.TypePass:
		return "pass"
	case command.TypeRollDice:
		return "roll defense"
	}
	ids := lookahead.CardIDs(action)
	if len(ids) == 1 {
		choice := lookahead.CardChoice(action)
		name := cardName(lib, b, seat, ids[0])
		if choice.Verb == "" {
			return "discard " + name
		}
		return name + ": " + strings.TrimPrefix(choice.Label, "Play ")
	}
	return string(action.Type)
}

func tierLabel(tier string) string {
	if tier == "" || tier == "base" || tier == "straight" {
		return ""
	}
	return " (" + strings.ReplaceAll(tier, "_", " ") + ")"
}

// diePositions names dice as players count them: die 1–5 with their faces.
func diePositions(b *state.Battle, seat string, indices []int) string {
	dice := b.Settled.Actors[seat].FinalDice
	var parts []string
	for _, i := range indices {
		face := "?"
		for _, d := range dice {
			if d.Index == i {
				face = fmt.Sprint(d.Face)
			}
		}
		parts = append(parts, fmt.Sprintf("die %d (%s)", i+1, face))
	}
	return strings.Join(parts, ", ")
}

func describe(lib content.BattleLibrary, before, after *state.Battle, seat string, action command.Command, stage string, events []event.Event) string {
	rt := before.Settled.Actors[seat]
	switch action.Type {
	case command.TypePlanningRoll:
		return "Rolls the offensive dice (roll 1 of " + fmt.Sprint(rt.MaxRolls) + ")"
	case command.TypePlanningReroll:
		var p command.PlanningRerollPayload
		_ = json.Unmarshal(action.Payload, &p)
		return fmt.Sprintf("Rerolls %s (roll %d of %d)", diePositions(before, seat, p.RerollIndices), rt.RollsUsed+1, rt.MaxRolls)
	case command.TypePlanningKeep:
		var p command.PlanningKeepPayload
		_ = json.Unmarshal(action.Payload, &p)
		return "Marks dice as kept: " + diePositions(before, seat, p.KeptIndices)
	case command.TypePlanningAbility:
		var p command.PlanningAbilityPayload
		_ = json.Unmarshal(action.Payload, &p)
		if stage == "defense_selection" {
			against := ""
			for _, source := range before.Settled.OffensiveSources {
				if len(p.TargetIDs) == 1 && source.ID == p.TargetIDs[0] {
					against = fmt.Sprintf(" against %s (%d damage)", abilityName(lib, source.SourceContentID), source.FinalAmount)
				}
			}
			return "Defends with " + abilityName(lib, p.AbilityID) + against
		}
		base, bonus := tierDamage(lib, before, seat, p.AbilityID, p.TierID)
		text := fmt.Sprintf("Chooses %s%s: %d damage", abilityName(lib, p.AbilityID), tierLabel(p.TierID), base)
		if bonus > 0 {
			text += fmt.Sprintf(" +%d from cards = %d", bonus, base+bonus)
		}
		if left := rt.MaxRolls - rt.RollsUsed; left > 0 {
			text += fmt.Sprintf(" · stops with %d roll(s) left", left)
		}
		return text
	case command.TypeRollDice:
		for _, d := range defenses(after) {
			if d.ActorID == seat && len(d.RolledDice) > 0 {
				return "Rolls " + abilityName(lib, d.AbilityID) + preventionText(events)
			}
		}
		return "Rolls defense" + preventionText(events)
	case command.TypePlanningPass:
		switch stage {
		case "planning":
			if rt.SelectedAbilityID == "" {
				return "Ends offense without attacking"
			}
			return "Locks in the attack"
		case "defense_reaction":
			return "Applies the defense roll" + preventionText(events)
		case "defense_selection":
			return "Ends defense (Pass)" + handNote(lib, before, seat)
		}
		return "Pass"
	case command.TypePass:
		if stage == "offensive_reaction" {
			return "No offensive reaction"
		}
		return "Pass" + handNote(lib, before, seat)
	case command.TypePlanningCards, command.TypeCommitInteraction:
		ids := lookahead.CardIDs(action)
		choice := lookahead.CardChoice(action)
		if len(ids) != 1 {
			if stage == "damage_reaction" || stage == "defense_selection" {
				for _, status := range before.Actors[seat].Statuses {
					if status.DefinitionID == "protect" {
						return fmt.Sprintf("Spends Protect (%d)", status.Stacks) + preventionText(events)
					}
				}
				return "Spends a prevention status" + preventionText(events)
			}
			return string(action.Type)
		}
		name := cardName(lib, before, seat, ids[0])
		if stage == "discard_to_hand_limit" {
			return "Discards " + name + " (hand limit)"
		}
		timing := ""
		if stage == "planning" {
			if rt.RollsUsed == 0 {
				timing = " · before rolling"
			} else {
				timing = fmt.Sprintf(" · after roll %d of %d", rt.RollsUsed, rt.MaxRolls)
			}
		}
		cost := ""
		if c, ok := lib.Cards[before.Settled.Actors[seat].CardInstances[ids[0]].DefinitionID]; ok && c.Cost.Energy > 0 && choice.Verb == "start" {
			cost = fmt.Sprintf(" (%d energy)", c.Cost.Energy)
		}
		label := strings.TrimPrefix(choice.Label, "Play ")
		switch {
		case choice.Verb == "start" && choice.Kind == "offensive_die":
			return fmt.Sprintf("Plays %s%s%s → %s%s", name, cost, timing, dieChoiceText(choice.Label), preventionText(events))
		case choice.Verb == "start" && label != "" && label != name:
			return fmt.Sprintf("Plays %s%s on %s%s%s", name, cost, label, timing, preventionText(events))
		case choice.Verb == "start":
			return fmt.Sprintf("Plays %s%s%s%s", name, cost, timing, preventionText(events))
		default:
			return fmt.Sprintf("%s: picks %s%s%s", name, label, timing, preventionText(events))
		}
	}
	return string(action.Type)
}

// defenses lists every defense the battle is tracking.
func defenses(b *state.Battle) []state.SettledDefense {
	var out []state.SettledDefense
	for _, group := range []map[string]state.SettledDefense{b.Settled.DefenseSelections, b.Settled.DefensePlans, b.Settled.DefenseHistory} {
		for _, key := range sortedKeys(group) {
			out = append(out, group[key])
		}
	}
	return out
}

// defenseDice is the seat's latest defensive roll.
func defenseDice(b *state.Battle, seat string) []state.RolledDie {
	for _, d := range defenses(b) {
		if d.ActorID == seat && len(d.RolledDice) > 0 {
			return d.RolledDice
		}
	}
	return nil
}

func preventionText(events []event.Event) string {
	var parts []string
	for _, e := range events {
		if e.Type == event.TypeDamageModified {
			parts = append(parts, fmt.Sprintf("damage %v → %v", e.Data["damage_before"], e.Data["damage_after"]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " · " + strings.Join(parts, ", ")
}

func handNote(lib content.BattleLibrary, b *state.Battle, seat string) string {
	hand := handNames(lib, b, seat)
	if len(hand) == 0 {
		return ""
	}
	return fmt.Sprintf(" · holding %s with %d energy", strings.Join(hand, ", "), b.Actors[seat].Resources.EnergyPoints)
}

// diceNotes checks an offensive dice decision against the exact solver,
// which plans the dice cards in hand too (a card played is charged its
// energy at 0.1 per point plus 0.25 for the card).
func diceNotes(solver *lookahead.DiceSolver, lib content.BattleLibrary, b *state.Battle, seat string, action command.Command, stage string, legal []command.Command) []string {
	if stage != "planning" {
		return nil
	}
	plan, err := solver.Plan(b, seat)
	if err != nil || plan == nil {
		return nil
	}
	rt := b.Settled.Actors[seat]
	type option struct {
		text  string
		value float64
	}
	var options []option
	seen := map[string]bool{}
	for _, a := range legal {
		switch {
		case a.Type == command.TypePlanningReroll:
			var p command.PlanningRerollPayload
			_ = json.Unmarshal(a.Payload, &p)
			options = append(options, option{"reroll " + diePositions(b, seat, p.RerollIndices), plan.Reroll(p.RerollIndices)})
		case lookahead.CardChoice(a).Verb == "start" && rt.CardExecution == nil:
			ids := lookahead.CardIDs(a)
			if len(ids) != 1 {
				continue
			}
			card := lib.Cards[rt.CardInstances[ids[0]].DefinitionID]
			if seen[card.ID] {
				continue
			}
			if v, ok := plan.PlayCard(card, rt.FinalDice); ok {
				seen[card.ID] = true
				options = append(options, option{"play " + card.Name, v})
			}
		}
	}
	if plan.Stop > 0 {
		options = append(options, option{"stop with " + abilityName(lib, plan.BestAbility), plan.Stop})
	} else {
		options = append(options, option{"stop (misses)", 0})
	}
	sort.SliceStable(options, func(i, j int) bool { return options[i].value > options[j].value })
	best := options[0]
	chosen := math.NaN()
	switch {
	case action.Type == command.TypePlanningReroll:
		var p command.PlanningRerollPayload
		_ = json.Unmarshal(action.Payload, &p)
		chosen = plan.Reroll(p.RerollIndices)
	case action.Type == command.TypePlanningAbility:
		chosen = plan.Stop
	case lookahead.CardChoice(action).Verb == "start" && rt.CardExecution == nil:
		if ids := lookahead.CardIDs(action); len(ids) == 1 {
			card := lib.Cards[rt.CardInstances[ids[0]].DefinitionID]
			if v, ok := plan.PlayCard(card, rt.FinalDice); ok {
				chosen = v
			}
		}
	case lookahead.CardChoice(action).Kind == "offensive_die" && rt.CardExecution != nil:
		card := lib.Cards[rt.CardInstances[rt.CardExecution.CardID].DefinitionID]
		choice := lookahead.CardChoice(action)
		if choice.Face != nil {
			chosen = plan.Nudge(card.ID, card.Cost.Energy, choice.Die, *choice.Face)
		} else {
			chosen = plan.TryAgain(card.ID, card.Cost.Energy, choice.Die)
		}
		v, _ := plan.PlayCard(card, rt.FinalDice)
		note := fmt.Sprintf("Solver (cards counted): this die choice is worth %.2f; the best die for this card is worth %.2f.", chosen, v)
		if chosen+0.05 < v {
			note += " ⚠ Not the best die."
		}
		return []string{note}
	}
	if math.IsNaN(chosen) {
		return nil
	}
	var parts []string
	for i, o := range options {
		if i == 4 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.2f", o.text, o.value))
	}
	note := fmt.Sprintf("Solver (cards counted, %d roll(s) left): this play is worth %.2f. Options: %s.", plan.RollsLeft, chosen, strings.Join(parts, "; "))
	if chosen+0.05 < best.value {
		note += fmt.Sprintf(" ⚠ Best was: %s.", best.text)
	}
	return []string{note}
}

func noteTendencies(g *game, lib content.BattleLibrary, b *state.Battle, seat string, action command.Command, stage string) {
	rt := b.Settled.Actors[seat]
	switch action.Type {
	case command.TypePlanningAbility:
		if stage == "planning" {
			var p command.PlanningAbilityPayload
			_ = json.Unmarshal(action.Payload, &p)
			g.Abilities[abilityName(lib, p.AbilityID)]++
		}
	case command.TypePlanningCards, command.TypeCommitInteraction:
		ids := lookahead.CardIDs(action)
		if len(ids) != 1 || lookahead.CardChoice(action).Verb != "start" {
			return
		}
		name := cardName(lib, b, seat, ids[0])
		timing := phaseName(string(b.Segment.Current), stage)
		if stage == "planning" {
			if rt.RollsUsed == 0 {
				timing = "offense, before rolling"
			} else {
				timing = fmt.Sprintf("offense, after roll %d", rt.RollsUsed)
			}
		}
		if g.CardTiming[name] == nil {
			g.CardTiming[name] = map[string]int{}
		}
		g.CardTiming[name][timing]++
	}
}

func sameCommand(a, b command.Command) bool {
	if a.Type != b.Type || a.ActorID != b.ActorID {
		return false
	}
	var x, y any
	_ = json.Unmarshal(a.Payload, &x)
	_ = json.Unmarshal(b.Payload, &y)
	return reflect.DeepEqual(x, y)
}

func dieHTML(d state.RolledDie, extra string) string {
	symbol, class := "", "die"
	for _, sym := range d.Symbols {
		switch sym {
		case "sword":
			symbol, class = "⚔", "die sword"
		case "shield":
			symbol, class = "🛡", "die shield"
		case "gold_coin":
			symbol, class = "●", "die coin"
		}
	}
	return fmt.Sprintf(`<span class="%s" title="%s">%d<small>%s</small>%s</span>`, class, html.EscapeString(strings.Join(d.Symbols, ", ")), d.Face, symbol, extra)
}

// dieChoiceText shortens a card's die choice label ("Starter · die 1: 2 → 3")
// to the part about the die.
func dieChoiceText(label string) string {
	if i := strings.Index(label, "die "); i >= 0 {
		return label[i:]
	}
	return strings.TrimPrefix(label, "Play ")
}

func indexSet(indices []int) map[int]bool {
	out := map[int]bool{}
	for _, i := range indices {
		out[i] = true
	}
	return out
}

func byIndex(dice []state.RolledDie) map[int]state.RolledDie {
	out := map[int]state.RolledDie{}
	for _, d := range dice {
		out[d.Index] = d
	}
	return out
}

// proofHTML lays a dice step out like a proof: the dice held, then under
// each die the step's result there (a new face for every rerolled or
// changed die, even if it shows the same number; "keep" for the rest).
func proofHTML(r row) string {
	if len(r.Dice) == 0 {
		return ""
	}
	positions := []int{}
	for _, d := range r.Dice {
		positions = append(positions, d.Index)
	}
	sort.Ints(positions)
	var sb strings.Builder
	sb.WriteString(`<table class="proof"><tr><th></th>`)
	for i, p := range positions {
		n := p + 1
		if r.DiceLabel == "defense" {
			n = i + 1
		}
		fmt.Fprintf(&sb, `<th>die %d</th>`, n)
	}
	sb.WriteString(`</tr>`)
	after := byIndex(r.Dice)
	if len(r.DiceBefore) == 0 {
		fmt.Fprintf(&sb, `<tr><td class="lbl">%s</td>`, html.EscapeString(r.DiceLabel))
		for _, p := range positions {
			fmt.Fprintf(&sb, `<td>%s</td>`, dieHTML(after[p], ""))
		}
		sb.WriteString(`</tr></table>`)
		return sb.String()
	}
	before := byIndex(r.DiceBefore)
	sb.WriteString(`<tr><td class="lbl">have</td>`)
	for _, p := range positions {
		fmt.Fprintf(&sb, `<td>%s</td>`, dieHTML(before[p], ""))
	}
	fmt.Fprintf(&sb, `</tr><tr class="step"><td class="lbl">%s</td>`, html.EscapeString(r.DiceLabel))
	for _, p := range positions {
		if !r.Touched[p] {
			sb.WriteString(`<td class="keep">keep</td>`)
			continue
		}
		delta := ""
		if strings.EqualFold(r.DiceLabel, "nudge") {
			delta = fmt.Sprintf(`<em>%+d</em>`, after[p].Face-before[p].Face)
		}
		fmt.Fprintf(&sb, `<td class="new">%s</td>`, dieHTML(after[p], delta))
	}
	sb.WriteString(`</tr></table>`)
	return sb.String()
}

func render(games []*game) string {
	var sb strings.Builder
	sb.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Look-ahead mirror review</title><style>
body{font:14px/1.45 -apple-system,system-ui,sans-serif;margin:24px auto;max-width:1180px;color:#1d232b;background:#f6f4ef;padding:0 16px}
h1{margin:0 0 4px} h2{margin:28px 0 8px} .muted{color:#6b7480}
table{border-collapse:collapse;width:100%;background:#fff} th,td{border-bottom:1px solid #e6e2d9;padding:6px 8px;vertical-align:top;text-align:left}
th{background:#ece8de;font-weight:600}
details{background:#fff;border:1px solid #ddd6c8;border-radius:8px;margin:12px 0;padding:6px 12px}
summary{cursor:pointer;font-weight:600;padding:6px 0}
tr.ai td{background:#fff8e6} tr.you td{color:#596270} tr.summary td{background:#e9eef5;font-weight:600}
tr.override td:first-child{border-left:4px solid #d9822b}
.die{display:inline-block;min-width:30px;text-align:center;border:1px solid #b9b2a3;border-radius:5px;margin:0 2px;padding:1px 3px;background:#fff;font-weight:600}
.die small{font-size:11px;margin-left:2px} .die.sword{background:#eef4ff} .die.shield{background:#eef8ef} .die.coin{background:#fff6dc}
table.proof{width:auto;background:transparent;border-collapse:separate;border-spacing:2px 2px}
table.proof th{background:none;font-size:10px;color:#8a8f96;font-weight:500;padding:0 2px;text-align:center;border:0}
table.proof td{border:0;padding:0;text-align:center}
table.proof td.lbl{font-size:11px;color:#6b7480;text-align:right;padding-right:6px;white-space:nowrap}
table.proof tr.step td.new .die{outline:2px solid #d9822b}
table.proof td.keep{font-size:10px;color:#b0aa9c;font-style:italic}
.die{white-space:nowrap} .die em{font-style:normal;font-size:10px;color:#a35a12;margin-left:2px}
.note{font-size:12px;color:#5a4a20} .greedy{font-size:12px;color:#a35a12}
.hand{font-size:12px;color:#6b7480} .tag{display:inline-block;padding:0 6px;border-radius:9px;font-size:11px;font-weight:700}
.tag.ai{background:#f2c14e;color:#3b2b00} .tag.you{background:#cfd6df;color:#2b3440}
label{user-select:none} .hide-you tr.you{display:none}
</style></head><body>`)
	sb.WriteString(`<h1>Look-ahead mirror review</h1><p class="muted">Your finished games against the look-ahead AI, replayed from the game's battle log through the real battle engine. The AI's dice and hand are shown even where they were hidden from you during play. Rows in yellow are the AI's choices; an orange edge marks a choice where the look-ahead overrode its heuristic.</p>`)
	sb.WriteString(`<p><label><input type="checkbox" onclick="document.body.classList.toggle('hide-you',this.checked)"> Show only the AI's actions</label></p>`)

	// Summary table.
	sb.WriteString(`<h2>Games</h2><table><tr><th>#</th><th>AI version</th><th>Character</th><th>Your seat</th><th>Result</th><th>Rounds</th><th>Health left (you / AI)</th><th>AI choices</th><th>Overrides</th></tr>`)
	totals := map[string]int{}
	for _, g := range games {
		totals[g.Battle.Result]++
		fmt.Fprintf(&sb, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%d / %d</td><td>%d</td><td>%d</td></tr>`, g.Number, g.Version, g.Battle.Character, g.Battle.HumanSeat, resultLabel(g.Battle.Result), g.Rounds, g.YouHealth, g.AIHealth, g.AIChoices, g.Overrides)
	}
	fmt.Fprintf(&sb, `</table><p>Your record: %d wins, %d draws, %d losses.</p>`, totals["victory"], totals["draw"], totals["defeat"])

	// AI tendencies across games.
	cards := map[string]map[string]int{}
	abilities := map[string]int{}
	for _, g := range games {
		for c, m := range g.CardTiming {
			if cards[c] == nil {
				cards[c] = map[string]int{}
			}
			for k, v := range m {
				cards[c][k] += v
			}
		}
		for a, n := range g.Abilities {
			abilities[a] += n
		}
	}
	sb.WriteString(`<h2>What the AI played, and when</h2><table><tr><th>Card</th><th>When it was played (count)</th></tr>`)
	for _, c := range sortedKeys(cards) {
		var parts []string
		for _, k := range sortedKeys(cards[c]) {
			parts = append(parts, fmt.Sprintf("%s ×%d", k, cards[c][k]))
		}
		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%s</td></tr>`, html.EscapeString(c), html.EscapeString(strings.Join(parts, "; ")))
	}
	sb.WriteString(`</table><p class="muted">Try Again and Nudge can only be played after the AI's own first roll. Because the AI plans first and its dice stay hidden until the reveal, you see its card plays before you roll yours.</p>`)
	sb.WriteString(`<table><tr><th>Attack the AI chose</th><th>Times</th></tr>`)
	for _, a := range sortedKeys(abilities) {
		fmt.Fprintf(&sb, `<tr><td>%s</td><td>%d</td></tr>`, html.EscapeString(a), abilities[a])
	}
	sb.WriteString(`</table>`)

	sb.WriteString(`<h2>Worth a look</h2><p class="muted">Every AI choice the solver flags (⚠) or where the look-ahead overrode its heuristic. Click to jump to the row (its game opens).</p><ul>`)
	for _, g := range games {
		for _, r := range g.Rows {
			flagged := r.Override
			for _, n := range r.Notes {
				flagged = flagged || strings.Contains(n, "⚠")
			}
			if !r.AI || !flagged {
				continue
			}
			what := "override"
			for _, n := range r.Notes {
				if strings.Contains(n, "⚠") {
					what = "solver ⚠"
				}
			}
			fmt.Fprintf(&sb, `<li><a href="#%s" onclick="document.getElementById('game-%d').open=true">Game %d, round %d</a> · %s · %s</li>`, r.ID, g.Number, g.Number, r.Round, what, html.EscapeString(r.Action))
		}
	}
	sb.WriteString(`</ul>`)
	for _, g := range games {
		fmt.Fprintf(&sb, `<details id="game-%d"%s><summary>`, g.Number, map[bool]string{true: " open", false: ""}[g == games[0]])
		fmt.Fprintf(&sb, `Game %d · %s · you in %s · %s · %d rounds · you %d / AI %d health</summary>`, g.Number, g.Battle.Character, g.Battle.HumanSeat, resultLabel(g.Battle.Result), g.Rounds, g.YouHealth, g.AIHealth)
		if g.Error != "" {
			fmt.Fprintf(&sb, `<p>Could not replay this game: %s</p></details>`, html.EscapeString(g.Error))
			continue
		}
		sb.WriteString(`<table><tr><th>Round</th><th>Who</th><th>Step</th><th>Action</th><th>Dice</th><th>Energy</th></tr>`)
		for _, r := range g.Rows {
			if r.Summary {
				fmt.Fprintf(&sb, `<tr class="summary"><td colspan="6">%s</td></tr>`, html.EscapeString(r.Action))
				continue
			}
			class, who := "you", `<span class="tag you">YOU</span>`
			if r.AI {
				class, who = "ai", `<span class="tag ai">AI</span>`
			}
			if r.Override {
				class += " override"
			}
			var extra strings.Builder
			if r.Greedy != "" {
				fmt.Fprintf(&extra, `<div class="greedy">Look-ahead override: its %s heuristic would have chosen “%s”.</div>`, g.Version, html.EscapeString(r.Greedy))
			}
			for _, n := range r.Notes {
				fmt.Fprintf(&extra, `<div class="note">%s</div>`, html.EscapeString(n))
			}
			if r.AI && len(r.Hand) > 0 {
				fmt.Fprintf(&extra, `<div class="hand">AI hand after: %s</div>`, html.EscapeString(strings.Join(r.Hand, ", ")))
			}
			fmt.Fprintf(&sb, `<tr id="%s" class="%s"><td>%d</td><td>%s</td><td>%s</td><td>%s%s</td><td>%s</td><td>%d</td></tr>`, r.ID, class, r.Round, who, html.EscapeString(r.Phase), html.EscapeString(r.Action), extra.String(), proofHTML(r), r.Energy)
		}
		sb.WriteString(`</table></details>`)
	}
	fmt.Fprintf(&sb, `<p class="muted">Generated %s. Dice: ⚔ Sword (1–3), 🛡 Shield (4–5), ● Coin (6). In each dice step the top row is the dice held and the row beneath shows the result under every die that was rerolled or changed (outlined, even when it shows the same number); “keep” marks dice left alone.</p></body></html>`, time.Now().Format("2006-01-02 15:04"))
	return sb.String()
}

// listed reports whether n is in a comma-separated list of numbers.
func listed(list string, n int) bool {
	for _, part := range strings.Split(list, ",") {
		if strings.TrimSpace(part) == fmt.Sprint(n) {
			return true
		}
	}
	return false
}

func resultLabel(result string) string {
	switch result {
	case "victory":
		return "You won"
	case "defeat":
		return "AI won"
	case "draw":
		return "Draw"
	}
	return result
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func defaultServerRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
