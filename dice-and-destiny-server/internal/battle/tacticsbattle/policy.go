package tacticsbattle

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

// enemyPolicy picks one of a seat's legal actions from its own view.
type enemyPolicy interface {
	choose(seat string, view engine.Result) (command.Command, error)
}

// scriptedPolicy is the sandbox enemies' behaviour (plan §8). It reads only
// the seat's own viewer-safe view and legal actions.
//
//   - Offense: the shipped single-ability roll policy (keep the target faces,
//     reroll the rest while rolls remain, play the configured card on a
//     qualified attack). A ranged or area enemy prefers its ranged or area
//     attack when it has a legal target, then its backup melee; area attacks
//     aim at the player's current square; with no legal target it does not
//     attack.
//   - Movement: melee enemies pursue the player. Ranged and area enemies
//     flee when adjacent, move to the nearest square in range and sight
//     when out of range or sight, and otherwise hold. An enemy commits up to
//     ai.max_cards_per_round cards only when that brings it into range next
//     round, and never its last card in hand.
//   - Defense: its defense on each incoming attack while legal (only
//     on-the-move defenses after running), otherwise pass.
type scriptedPolicy struct {
	enemies map[string]EnemyConfig
}

func (p scriptedPolicy) choose(seat string, view engine.Result) (command.Command, error) {
	actions := view.LegalActions
	if len(actions) == 0 || view.Snapshot == nil {
		return command.Command{}, errors.New("no legal action")
	}
	config, ok := p.enemies[seat]
	if !ok {
		return command.Command{}, fmt.Errorf("no scripted behaviour for %s", seat)
	}
	snap := view.Snapshot
	own := snap.Actors[seat]
	// A mandatory hand-limit discard: any commitment will do.
	if snap.Stage == "discard_to_hand_limit" {
		for _, a := range actions {
			if a.Type == command.TypeCommitInteraction {
				return a, nil
			}
		}
	}
	if a, ok := find(actions, command.TypeMovementPlan); ok {
		return p.movement(seat, config, snap, a)
	}
	// Offense: roll first (locking in by rolling, never sprinting).
	if a, ok := find(actions, command.TypePlanningRoll); ok {
		return a, nil
	}
	if own.Dice != nil && own.Dice.RollsUsed < own.Dice.MaxRolls && hasType(actions, command.TypePlanningReroll) {
		if a, ok := keepOrReroll(config.Keep, own, actions); ok {
			return a, nil
		}
	}
	if hasType(actions, command.TypePlanningAbility) || (snap.Stage == "planning" && hasType(actions, command.TypePlanningPass)) {
		if snap.Tactics != nil && snap.Stage == "planning" {
			return p.offense(seat, config, snap, own, actions)
		}
	}
	// Defense: the seat's defense on an incoming attack while it is legal.
	for _, a := range actions {
		var payload command.PlanningAbilityPayload
		if a.Type == command.TypePlanningAbility && json.Unmarshal(a.Payload, &payload) == nil && slices.Contains(own.DefensiveAbilities, payload.AbilityID) {
			return a, nil
		}
	}
	for _, kind := range []command.Type{command.TypeRollDice, command.TypePass, command.TypePlanningPass} {
		for _, a := range actions {
			var payload command.PlanningPassPayload
			if a.Type == kind && (kind != command.TypePlanningPass || json.Unmarshal(a.Payload, &payload) == nil && payload.SourceID == "") {
				return a, nil
			}
		}
	}
	return command.Command{}, fmt.Errorf("no supported legal action at %s", snap.Stage)
}

func find(actions []command.Command, kind command.Type) (command.Command, bool) {
	for _, a := range actions {
		if a.Type == kind {
			return a, true
		}
	}
	return command.Command{}, false
}

func hasType(actions []command.Command, kind command.Type) bool {
	_, ok := find(actions, kind)
	return ok
}

// keepOrReroll keeps every target face and rerolls the rest (the shipped
// single-ability policy); a roll that is all target faces stops early.
func keepOrReroll(keep content.SingleAbilityPolicy, own snapshot.Actor, actions []command.Command) (command.Command, bool) {
	bound := map[int]bool{}
	for _, d := range own.OwnedDice {
		bound[d.Index] = d.Entombed
	}
	var kept, reroll []int
	for _, die := range own.Dice.Dice {
		if keep.Keeps(die.Face) && !bound[die.Index] {
			kept = append(kept, die.Index)
		} else {
			reroll = append(reroll, die.Index)
		}
	}
	if kept == nil {
		kept = []int{}
	}
	if !slices.Equal(kept, own.Dice.KeptIndices) && !(len(kept) == 0 && len(own.Dice.KeptIndices) == 0) {
		for _, a := range actions {
			var payload command.PlanningKeepPayload
			if a.Type == command.TypePlanningKeep && json.Unmarshal(a.Payload, &payload) == nil && slices.Equal(payload.KeptIndices, kept) {
				return a, true
			}
		}
	}
	if len(reroll) > 0 {
		for _, a := range actions {
			var payload command.PlanningRerollPayload
			if a.Type == command.TypePlanningReroll && json.Unmarshal(a.Payload, &payload) == nil && slices.Equal(payload.RerollIndices, reroll) {
				return a, true
			}
		}
	}
	return command.Command{}, false
}

// offense picks the attack: the ranged or area ability when it has a legal
// target (area: the player's current square), then the backup melee; the
// configured card is played on that attack first, once a round.
func (p scriptedPolicy) offense(seat string, config EnemyConfig, snap *snapshot.Battle, own snapshot.Actor, actions []command.Command) (command.Command, error) {
	tv := snap.Tactics
	player := nearestOpponent(seat, snap)
	choice, chosen := command.Command{}, ""
	for _, ability := range own.OffensiveAbilities {
		delivery := tv.Creatures[seat].Deliveries[ability]
		for _, a := range actions {
			var payload command.PlanningAbilityPayload
			if a.Type != command.TypePlanningAbility || json.Unmarshal(a.Payload, &payload) != nil || payload.AbilityID != ability {
				continue
			}
			if delivery.Type == content.DeliveryTypeArea {
				if payload.TargetSquare == nil || player == "" || *payload.TargetSquare != tv.Creatures[player].Position {
					continue
				}
			} else if len(payload.TargetIDs) != 1 || payload.TargetIDs[0] != player {
				continue
			}
			choice, chosen = a, ability
			break
		}
		if chosen != "" {
			break
		}
	}
	if chosen == "" {
		// No legal target for any qualified attack: no attack this round.
		if a, ok := find(actions, command.TypePlanningPass); ok {
			return a, nil
		}
		return command.Command{}, errors.New("no attack and no pass")
	}
	if card := config.Keep.CardID; card != "" && own.CardUses[fmt.Sprintf("%d:%s", snap.Round, card)] == 0 {
		played := false
		for _, m := range own.AbilityModifiers {
			if m.ExpiresAfterRound == snap.Round {
				played = true
			}
		}
		if !played {
			for _, a := range actions {
				var payload command.PlanningCardsPayload
				if a.Type == command.TypePlanningCards && json.Unmarshal(a.Payload, &payload) == nil && len(payload.CardIDs) == 1 && own.CardInstances[payload.CardIDs[0]].DefinitionID == card && payload.AbilityID == chosen {
					return a, nil
				}
			}
		}
	}
	return choice, nil
}

// nearestOpponent is the closest living opponent (the player in the sandbox).
func nearestOpponent(seat string, snap *snapshot.Battle) string {
	tv := snap.Tactics
	from := tv.Creatures[seat].Position
	best, bestDistance := "", 0
	var ids []string
	for id := range snap.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		actor := snap.Actors[id]
		if id == seat || tv.Creatures[id].Defeated || !tactics.Opponents(actor.TeamID, snap.Actors[seat].TeamID) {
			continue
		}
		d := tactics.Distance(tv.Rules.DiagonalRule, from, tv.Creatures[id].Position)
		if best == "" || d < bestDistance {
			best, bestDistance = id, d
		}
	}
	return best
}

// gridOf rebuilds the battle's grid from the viewer-safe arena view.
func gridOf(tv *snapshot.TacticsView) tactics.Grid {
	legend := map[string]string{}
	glyphs := map[string]string{}
	next := 'a'
	rows := make([]string, len(tv.Arena.Terrain))
	for y, row := range tv.Arena.Terrain {
		line := []rune{}
		for _, terrain := range row {
			glyph, ok := glyphs[terrain]
			if !ok {
				glyph = string(next)
				next++
				glyphs[terrain], legend[glyph] = glyph, terrain
			}
			line = append(line, []rune(glyph)...)
		}
		rows[y] = string(line)
	}
	return tactics.NewGrid(tactics.Arena{SchemaVersion: 1, ID: tv.Arena.ID, Name: tv.Arena.Name, Legend: legend, Grid: rows}, tv.Rules)
}

// movement locks in the seat's plan for the round.
func (p scriptedPolicy) movement(seat string, config EnemyConfig, snap *snapshot.Battle, template command.Command) (command.Command, error) {
	var base command.MovementPlanPayload
	if err := json.Unmarshal(template.Payload, &base); err != nil {
		return command.Command{}, err
	}
	tv := snap.Tactics
	plan := command.MovementPlanPayload{PendingInputID: base.PendingInputID, Checkpoint: base.Checkpoint, LockIn: true, Intention: tactics.Intention{Kind: tactics.IntentHold}}
	if tv.Plan != nil && tv.Plan.Charge != "" {
		plan.Intention = tv.Plan.Intention
		return command.Command{BattleID: template.BattleID, ActorID: seat, Type: command.TypeMovementPlan, Payload: mustJSON(plan)}, nil
	}
	player := nearestOpponent(seat, snap)
	if player == "" {
		return command.Command{BattleID: template.BattleID, ActorID: seat, Type: command.TypeMovementPlan, Payload: mustJSON(plan)}, nil
	}
	grid := gridOf(tv)
	me, target := tv.Creatures[seat].Position, tv.Creatures[player].Position
	inRange := func(at tactics.Square) bool { return tactics.Adjacent(at, target) }
	if config.Role == content.TacticsRoleMelee {
		plan.Intention = tactics.Intention{Kind: tactics.IntentPursue, Target: player}
	} else {
		reach, sight := p.rangedReach(seat, tv)
		occupied := p.occupied(seat, snap)
		inRange = func(at tactics.Square) bool {
			return grid.Distance(at, target) <= reach && (!sight || grid.LineOfSight(at, target, func(s tactics.Square) bool { return occupied[s] && s != at }))
		}
		switch {
		case tactics.Adjacent(me, target):
			plan.Intention = tactics.Intention{Kind: tactics.IntentFlee, Target: player}
		case !inRange(me):
			if square, ok := p.nearestInRange(seat, snap, grid, inRange); ok {
				plan.Intention = tactics.Intention{Kind: tactics.IntentMoveTo, Square: &square}
			}
		}
	}
	if plan.Intention.Kind == tactics.IntentPursue || plan.Intention.Kind == tactics.IntentMoveTo {
		plan.CardIDs = p.cardsToReach(seat, snap, grid, plan.Intention, inRange)
	}
	return command.Command{BattleID: template.BattleID, ActorID: seat, Type: command.TypeMovementPlan, Payload: mustJSON(plan)}, nil
}

// rangedReach is the seat's longest ranged or area reach and whether it
// needs line of sight.
func (p scriptedPolicy) rangedReach(seat string, tv *snapshot.TacticsView) (int, bool) {
	reach, sight := 1, true
	for _, d := range tv.Creatures[seat].Deliveries {
		if d.Range > reach && d.Charge == nil {
			reach, sight = d.Range, d.LineOfSight
		}
	}
	return reach, sight
}

func (p scriptedPolicy) occupied(seat string, snap *snapshot.Battle) map[tactics.Square]bool {
	occupied := map[tactics.Square]bool{}
	for id, c := range snap.Tactics.Creatures {
		if id != seat && !c.Defeated {
			occupied[c.Position] = true
		}
	}
	return occupied
}

// nearestInRange is the square in range and sight that costs the least Move
// to reach; ties go to the square nearest in a straight line, then row order.
func (p scriptedPolicy) nearestInRange(seat string, snap *snapshot.Battle, grid tactics.Grid, inRange func(tactics.Square) bool) (tactics.Square, bool) {
	occupied := p.occupied(seat, snap)
	me := snap.Tactics.Creatures[seat].Position
	var goals []tactics.Square
	for y := 0; y < grid.Height(); y++ {
		for x := 0; x < grid.Width(); x++ {
			s := tactics.Square{X: x, Y: y}
			if grid.Passable(s) && !occupied[s] && inRange(s) {
				goals = append(goals, s)
			}
		}
	}
	if len(goals) == 0 {
		return tactics.Square{}, false
	}
	blocked := func(s tactics.Square) bool { return occupied[s] }
	bestCost := -1
	var best tactics.Square
	for _, goal := range goals {
		cost := tactics.NewPathField(grid, tactics.CostModifiers{}, []tactics.Square{goal}, blocked, goal).CostTo(me, 0)
		if cost < 0 {
			continue
		}
		if bestCost < 0 || cost < bestCost || (cost == bestCost && straight(me, goal) < straight(me, best)) {
			best, bestCost = goal, cost
		}
	}
	return best, bestCost >= 0
}

// cardsToReach commits the fewest cards (up to ai.max_cards_per_round, never
// the last card in hand) that bring the seat into range by the end of the
// round, if everyone else held; none when its base speed already does or
// when no allowed number of cards would.
func (p scriptedPolicy) cardsToReach(seat string, snap *snapshot.Battle, grid tactics.Grid, intention tactics.Intention, inRange func(tactics.Square) bool) []string {
	tv := snap.Tactics
	own := snap.Actors[seat]
	limit := min(tv.Rules.AI.MaxCardsPerRound, len(own.Hand)-1)
	var movers []tactics.Mover
	for id, c := range tv.Creatures {
		if c.Defeated {
			continue
		}
		movers = append(movers, tactics.Mover{ID: id, Team: snap.Actors[id].TeamID, Start: c.Position, Intention: tactics.Intention{Kind: tactics.IntentHold}})
	}
	sort.Slice(movers, func(i, j int) bool { return movers[i].ID < movers[j].ID })
	for cards := 0; cards <= limit; cards++ {
		for i := range movers {
			if movers[i].ID == seat {
				movers[i].Intention = intention
				movers[i].Budget = tv.Creatures[seat].BaseSpeed + cards*tv.Rules.CardMove
				movers[i].Paid = cards * tv.Rules.CardMove
			}
		}
		if inRange(tactics.Preview(grid, movers, seat, 0).Final[seat]) {
			hand := append([]string(nil), own.Hand...)
			sort.Strings(hand)
			return hand[:cards]
		}
	}
	return nil
}

func straight(a, b tactics.Square) int {
	return (a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y)
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
