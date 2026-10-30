package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

// Tactics Sandbox (docs/game-design/tactics-sandbox-plan.md). A tactics
// battle adds a square grid to the simultaneous flow:
//
// Offense (unchanged, but targets must be in range and in line of sight from
// the round's starting squares; area abilities aim at a square; skipping the
// offense is a Sprint) → the attacks are revealed → Movement (every
// participant locks in one secret plan) → movement resolves (timeline,
// final squares, Running, which attacks connected) → Defense opens only for
// the attacks that connected → one final commit.

const stageMovement = "movement"

// TacticsBattle reports a battle pinned to the tactics rule.
func TacticsBattle(b *state.Battle) bool {
	return b != nil && b.Settled != nil && b.Settled.Tactics && b.Settled.TacticsRuntime != nil
}

func tacticsBattle(b *state.Battle) bool { return TacticsBattle(b) }

// InitTactics pins a new battle's grid: the arena, the rules and every
// seat's start square. The authority calls it at start, after unified
// defense and simultaneous play are pinned; the tactics rule needs both.
func InitTactics(b *state.Battle, setup command.TacticsSetup) error {
	if b == nil || b.Settled == nil || !b.Settled.UnifiedDefense || !b.Settled.SimultaneousPlay {
		return errors.New("the tactics ruleset needs unified defense and simultaneous play")
	}
	if err := tactics.ValidateRules(setup.Rules); err != nil {
		return err
	}
	if err := tactics.ValidateArena(setup.Arena, setup.Rules); err != nil {
		return err
	}
	library, err := settledLibrary(b)
	if err != nil {
		return err
	}
	running, ok := library.Statuses[setup.Rules.RunningStatus]
	if !ok || running.Modifiers == nil || !running.Modifiers.OnTheMoveDefensesOnly {
		return fmt.Errorf("running status %q must exist with on_the_move_defenses_only", setup.Rules.RunningStatus)
	}
	rt := &state.TacticsRuntime{Arena: setup.Arena, Rules: setup.Rules, Creatures: map[string]state.TacticsCreature{}, Positions: map[string]tactics.Square{}}
	grid := tactics.NewGrid(setup.Arena, setup.Rules)
	used := map[tactics.Square]string{}
	for _, id := range sortedSettledActorIDs(b) {
		square, ok := setup.Placement[id]
		if !ok {
			return fmt.Errorf("tactics placement has no square for %s", id)
		}
		if !grid.Passable(square) {
			return fmt.Errorf("%s starts on %v, outside the arena or impassable", id, square)
		}
		if other, taken := used[square]; taken {
			return fmt.Errorf("%s and %s start on the same square", id, other)
		}
		used[square] = id
		rt.Positions[id] = square
		creature := state.TacticsCreature{BaseSpeed: setup.Rules.BaseSpeed}
		if c := library.Combatants[b.Actors[id].DefinitionID].Tactics; c != nil {
			creature = state.TacticsCreature{BaseSpeed: c.BaseSpeed, Role: c.Role, Label: c.Label}
		}
		rt.Creatures[id] = creature
	}
	if len(setup.Placement) != len(rt.Positions) {
		return errors.New("tactics placement names a seat that is not in the battle")
	}
	b.Settled.Tactics = true
	b.Settled.TacticsRuntime = rt
	return nil
}

func tacticsGrid(b *state.Battle) tactics.Grid {
	rt := b.Settled.TacticsRuntime
	return tactics.NewGrid(rt.Arena, rt.Rules)
}

// beginTacticsRound starts a round's grid bookkeeping at Offensive entry:
// range and line of sight are measured from these squares all round.
func beginTacticsRound(b *state.Battle) {
	rt := b.Settled.TacticsRuntime
	rt.RoundStart = map[string]tactics.Square{}
	for id, square := range rt.Positions {
		rt.RoundStart[id] = square
	}
	rt.Squares = map[string]tactics.Square{}
	rt.Sprinting = map[string]bool{}
	rt.OffenseRevealed = false
	rt.Attacks = nil
	rt.Plans = nil
}

func livingCreature(b *state.Battle, id string) bool {
	actor, ok := b.Actors[id]
	return ok && actor.DefeatState != state.ActorDefeated
}

// AbilityDelivery is an ability's delivery in this tactics battle (its own,
// or the rules' default direct melee).
func AbilityDelivery(b *state.Battle, library content.BattleLibrary, abilityID string) content.AbilityDelivery {
	rules := b.Settled.TacticsRuntime.Rules
	fallback := content.AbilityDelivery{Type: rules.DefaultDelivery.Type, Range: rules.DefaultDelivery.Range, LineOfSight: rules.DefaultDelivery.LineOfSight}
	return content.EffectiveDelivery(library.Abilities[abilityID], fallback)
}

// roundStartSight reports line of sight between two squares at the round's
// starting positions; other living creatures block when the rules say so.
func roundStartSight(b *state.Battle, from, to tactics.Square) bool {
	rt := b.Settled.TacticsRuntime
	occupied := map[tactics.Square]bool{}
	for id, square := range rt.RoundStart {
		if livingCreature(b, id) {
			occupied[square] = true
		}
	}
	return tacticsGrid(b).LineOfSight(from, to, func(s tactics.Square) bool { return occupied[s] })
}

// tacticsCreatureTargets lists the opponents a direct or charge ability can
// target now: in range (charge: reach + 1) and in line of sight, measured
// from the round's starting squares.
func tacticsCreatureTargets(b *state.Battle, library content.BattleLibrary, actor, abilityID string) []string {
	rt := b.Settled.TacticsRuntime
	d := AbilityDelivery(b, library, abilityID)
	if d.Type == content.DeliveryTypeArea {
		return nil
	}
	from := rt.RoundStart[actor]
	grid := tacticsGrid(b)
	reach, sight := d.Range, d.LineOfSight
	if d.Charge != nil {
		reach, sight = d.Charge.Range+1, true
	}
	var out []string
	for _, id := range otherActorIDs(b, actor) {
		to, ok := rt.RoundStart[id]
		if !ok || grid.Distance(from, to) > reach || (sight && !roundStartSight(b, from, to)) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// tacticsSquareTargets lists the squares an area ability can aim at now: in
// range and in line of sight, and not inside a wall.
func tacticsSquareTargets(b *state.Battle, library content.BattleLibrary, actor, abilityID string) []tactics.Square {
	rt := b.Settled.TacticsRuntime
	d := AbilityDelivery(b, library, abilityID)
	if d.Type != content.DeliveryTypeArea {
		return nil
	}
	from := rt.RoundStart[actor]
	grid := tacticsGrid(b)
	var out []tactics.Square
	for y := 0; y < grid.Height(); y++ {
		for x := 0; x < grid.Width(); x++ {
			s := tactics.Square{X: x, Y: y}
			if grid.BlocksSight(s) || grid.Distance(from, s) > d.Range || (d.LineOfSight && !roundStartSight(b, from, s)) {
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

// offensiveChoice is one legal target of an attack: creatures, or a square.
type offensiveChoice struct {
	targets []string
	square  *tactics.Square
}

func offensiveTargetChoices(b *state.Battle, library content.BattleLibrary, actor, abilityID string) []offensiveChoice {
	if !tacticsBattle(b) {
		var out []offensiveChoice
		for _, targets := range actorTargetChoices(b, actor, library.Abilities[abilityID].Targeting) {
			out = append(out, offensiveChoice{targets: targets})
		}
		return out
	}
	var out []offensiveChoice
	if AbilityDelivery(b, library, abilityID).Type == content.DeliveryTypeArea {
		for _, s := range tacticsSquareTargets(b, library, actor, abilityID) {
			square := s
			out = append(out, offensiveChoice{square: &square})
		}
		return out
	}
	for _, id := range tacticsCreatureTargets(b, library, actor, abilityID) {
		out = append(out, offensiveChoice{targets: []string{id}})
	}
	return out
}

// validateTacticsTarget checks an attack's target on the grid: a creature
// in range and sight for direct and charge abilities, a square for area ones.
func validateTacticsTarget(b *state.Battle, library content.BattleLibrary, actor, abilityID string, targetIDs []string, square *tactics.Square) error {
	if AbilityDelivery(b, library, abilityID).Type == content.DeliveryTypeArea {
		if len(targetIDs) != 0 {
			return errors.New("an area attack targets a square, not a creature")
		}
		if square == nil {
			return nil
		}
		for _, s := range tacticsSquareTargets(b, library, actor, abilityID) {
			if s == *square {
				return nil
			}
		}
		return fmt.Errorf("square %v is out of range or out of sight", *square)
	}
	if square != nil {
		return errors.New("this attack targets a creature, not a square")
	}
	for _, id := range targetIDs {
		if !containsString(tacticsCreatureTargets(b, library, actor, abilityID), id) {
			return fmt.Errorf("%s is out of range or out of sight", id)
		}
	}
	return nil
}

// markSprint records a participant who skipped the offense: they sprint.
func markSprint(b *state.Battle, actor string) {
	if tacticsBattle(b) {
		rt := b.Settled.TacticsRuntime
		if rt.Sprinting == nil {
			rt.Sprinting = map[string]bool{}
		}
		rt.Sprinting[actor] = true
	}
}

// setTacticsSquare records (or clears, when square is nil) an area attack's
// target square while planning. Maps of a cloned battle may be nil.
func setTacticsSquare(b *state.Battle, actor string, square *tactics.Square) {
	rt := b.Settled.TacticsRuntime
	if square == nil {
		delete(rt.Squares, actor)
		return
	}
	if rt.Squares == nil {
		rt.Squares = map[string]tactics.Square{}
	}
	rt.Squares[actor] = *square
}

// --- Movement ---------------------------------------------------------------

// openMovement reveals the round's attacks with their delivery and opens the
// secret movement step for every participant at once.
func (e Engine) openMovement(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	rt := b.Settled.TacticsRuntime
	rt.OffenseRevealed = true
	rt.Attacks = e.revealedAttacks(b, library)
	var sprinting []string
	for id, sprint := range rt.Sprinting {
		if sprint {
			sprinting = append(sprinting, id)
		}
	}
	sort.Strings(sprinting)
	events := []event.Event{settledEvent(event.TypeTacticsAttacksRevealed, b, "", map[string]any{"attacks": rt.Attacks, "sprinting": sprinting})}
	beginSimultaneousStep(b, state.SimultaneousMovement)
	b.Settled.Stage = stageMovement
	b.Flow.Stage = stageMovement
	rt.Plans = map[string]state.TacticsPlan{}
	var open []string
	for _, id := range sortedSettledActorIDs(b) {
		if !livingCreature(b, id) {
			continue
		}
		plan := state.TacticsPlan{Intention: tactics.Intention{Kind: tactics.IntentHold}, Sprint: rt.Sprinting[id]}
		if charge := chargeOf(rt, id); charge != nil {
			plan.Charge = charge.Target
			plan.Intention = tactics.Intention{Kind: tactics.IntentPursue, Target: charge.Target}
		}
		if err := e.refreshPlan(b, library, id, &plan); err != nil {
			return nil, err
		}
		fixed := plan.Charge != "" && !rt.Rules.ChargeAllowsExtraMove
		if fixed || b.Actors[id].Controller == state.ControllerAI {
			plan.Locked = true
			b.Settled.Simultaneous.Locked[id] = true
		} else {
			open = append(open, id)
		}
		rt.Plans[id] = plan
	}
	if len(open) == 0 {
		next, err := e.resolveMovement(b, library)
		return append(events, next...), err
	}
	openSettledWindowForActors(b, "movement", stageMovement, "movement", []command.Type{command.TypeMovementPlan}, open, true)
	return events, nil
}

func chargeOf(rt *state.TacticsRuntime, actor string) *state.TacticsAttack {
	for i := range rt.Attacks {
		if rt.Attacks[i].Attacker == actor && rt.Attacks[i].Delivery == state.DeliveryCharge {
			return &rt.Attacks[i]
		}
	}
	return nil
}

// revealedAttacks records every attack set this round with its delivery.
func (e Engine) revealedAttacks(b *state.Battle, library content.BattleLibrary) []state.TacticsAttack {
	rt := b.Settled.TacticsRuntime
	grid := tacticsGrid(b)
	var out []state.TacticsAttack
	for _, id := range sortedSettledActorIDs(b) {
		run := b.Settled.Actors[id]
		if run.SelectedAbilityID == "" || !livingCreature(b, id) {
			continue
		}
		if _, ok := resolvedOffensiveOperations(b, library, id); !ok {
			continue
		}
		d := AbilityDelivery(b, library, run.SelectedAbilityID)
		attack := state.TacticsAttack{Attacker: id, AbilityID: run.SelectedAbilityID, Delivery: state.DeliveryDirect, Range: d.Range, LineOfSight: d.LineOfSight}
		switch {
		case d.Type == content.DeliveryTypeArea:
			square, ok := rt.Squares[id]
			if !ok {
				continue
			}
			attack.Delivery = state.DeliveryArea
			attack.Square = &square
			attack.Area = grid.SquaresWithin(square, d.Area.Radius)
		case d.Charge != nil:
			attack.Delivery = state.DeliveryCharge
			attack.ChargeRange = d.Charge.Range
			attack.Target = first(run.SelectedTargetIDs)
		default:
			attack.Target = first(run.SelectedTargetIDs)
		}
		out = append(out, attack)
	}
	return out
}

// planBudget prices a plan: its Move budget and terrain modifiers. A Hold
// plan commits nothing and never moves.
func planBudget(b *state.Battle, library content.BattleLibrary, actor string, plan state.TacticsPlan) (tactics.MoveBudget, tactics.CostModifiers, error) {
	rt := b.Settled.TacticsRuntime
	var mods tactics.CostModifiers
	if plan.Intention.Kind == tactics.IntentHold {
		return tactics.MoveBudget{}, mods, nil
	}
	in := tactics.BudgetInput{BaseSpeed: rt.Creatures[actor].BaseSpeed, Cards: len(plan.Cards), Energy: plan.Energy, Sprint: plan.Sprint}
	if plan.Charge != "" {
		if attack := chargeOf(rt, actor); attack != nil {
			in.ChargeRange = attack.ChargeRange
		}
	}
	run := b.Settled.Actors[actor]
	for _, played := range plan.MapCards {
		effect := library.Cards[run.CardInstances[played.CardID].DefinitionID].MapEffect
		if effect == nil {
			return tactics.MoveBudget{}, mods, fmt.Errorf("card %s has no map Move", played.CardID)
		}
		in.MapAmounts = append(in.MapAmounts, effect.Amount)
		if effect.IgnoreTerrain {
			mods.IgnoreTerrain = true
		}
		if effect.ChooseTerrain && played.Terrain != "" {
			mods.ChosenTerrain = append(mods.ChosenTerrain, played.Terrain)
		}
	}
	budget, err := tactics.ComputeBudget(rt.Rules, in)
	return budget, mods, err
}

// refreshPlan prices a plan and projects its path as if everyone else held.
func (e Engine) refreshPlan(b *state.Battle, library content.BattleLibrary, actor string, plan *state.TacticsPlan) error {
	budget, _, err := planBudget(b, library, actor, *plan)
	if err != nil {
		return err
	}
	plan.Budget = budget
	plan.Preview = nil
	if plan.Intention.Kind == tactics.IntentHold {
		return nil
	}
	plans := map[string]state.TacticsPlan{actor: *plan}
	movers, _, err := tacticsMovers(b, library, plans)
	if err != nil {
		return err
	}
	result := tactics.Preview(tacticsGrid(b), movers, actor, 0)
	preview := &state.TacticsPreview{Path: result.Paths[actor], Final: result.Final[actor], Spent: result.Spent[actor]}
	for _, entry := range result.Timeline {
		if entry.Creature == actor && entry.Kind == tactics.EntryStop {
			preview.Stop = entry.Reason
		}
	}
	if n := len(preview.Path); n > 1 {
		at := preview.Path[n-1].At
		preview.Arrive = &at
	}
	plan.Preview = preview
	return nil
}

// tacticsMovers builds every living creature's mover from the plans; a
// creature without a plan holds.
func tacticsMovers(b *state.Battle, library content.BattleLibrary, plans map[string]state.TacticsPlan) ([]tactics.Mover, map[string]tactics.MoveBudget, error) {
	rt := b.Settled.TacticsRuntime
	budgets := map[string]tactics.MoveBudget{}
	var movers []tactics.Mover
	for _, id := range sortedSettledActorIDs(b) {
		if !livingCreature(b, id) {
			continue
		}
		m := tactics.Mover{ID: id, Team: b.Actors[id].TeamID, Start: rt.Positions[id], Intention: tactics.Intention{Kind: tactics.IntentHold}}
		if m.Team == "" {
			m.Team = id
		}
		if plan, ok := plans[id]; ok {
			budget, mods, err := planBudget(b, library, id, plan)
			if err != nil {
				return nil, nil, err
			}
			budgets[id] = budget
			m.Budget, m.Paid, m.Costs, m.Intention = budget.Total, budget.Paid, mods, plan.Intention
			if plan.Charge != "" {
				m.ContactTarget = plan.Charge
			}
		}
		movers = append(movers, m)
	}
	return movers, budgets, nil
}

// validatePlan checks a movement plan: its intention, and that every
// committed card is in hand, every map card is a map Move card, the energy
// comes in steps the rules allow and everything is affordable.
func validatePlan(b *state.Battle, library content.BattleLibrary, actor string, plan state.TacticsPlan) error {
	rt := b.Settled.TacticsRuntime
	grid := tacticsGrid(b)
	in := plan.Intention
	switch in.Kind {
	case tactics.IntentHold:
		if len(plan.Cards) > 0 || len(plan.MapCards) > 0 || plan.Energy > 0 {
			return errors.New("Hold commits no cards or energy")
		}
	case tactics.IntentMoveTo:
		if in.Square == nil || in.Target != "" {
			return errors.New("Move to needs a square")
		}
		if !grid.Passable(*in.Square) {
			return fmt.Errorf("square %v cannot be entered", *in.Square)
		}
		if *in.Square == rt.Positions[actor] {
			return errors.New("choose another square, or Hold")
		}
	case tactics.IntentPursue, tactics.IntentFlee:
		if in.Square != nil || in.Target == actor || !livingCreature(b, in.Target) {
			return errors.New("Pursue and Flee need another living creature")
		}
	default:
		return fmt.Errorf("unknown intention %q", in.Kind)
	}
	if plan.Charge != "" && (in.Kind != tactics.IntentPursue || in.Target != plan.Charge) {
		return errors.New("a Charge pursues its target")
	}
	hand := map[string]bool{}
	for _, card := range b.Actors[actor].Cards.Hand {
		hand[card] = true
	}
	used := map[string]bool{}
	for _, card := range plan.Cards {
		if !hand[card] || used[card] {
			return fmt.Errorf("card %s is not in hand", card)
		}
		used[card] = true
	}
	cost := plan.Energy
	run := b.Settled.Actors[actor]
	for _, played := range plan.MapCards {
		if !hand[played.CardID] || used[played.CardID] {
			return fmt.Errorf("card %s is not in hand", played.CardID)
		}
		used[played.CardID] = true
		definition := library.Cards[run.CardInstances[played.CardID].DefinitionID]
		if definition.MapEffect == nil || definition.MapEffect.Kind != "move" {
			return fmt.Errorf("card %s is not a map Move card", played.CardID)
		}
		if definition.MapEffect.ChooseTerrain {
			terrain, ok := rt.Rules.Terrain[played.Terrain]
			if !ok || !terrain.Passable {
				return fmt.Errorf("%s names a terrain to make cost 1", definition.Name)
			}
		} else if played.Terrain != "" {
			return fmt.Errorf("%s does not choose a terrain", definition.Name)
		}
		cost += definition.Cost.Energy
	}
	if cost > b.Actors[actor].Resources.EnergyPoints {
		return errors.New("not enough energy")
	}
	_, _, err := planBudget(b, library, actor, plan)
	return err
}

func (e Engine) handleMovementCommand(b *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type != command.TypeMovementPlan {
		return nil, unsupportedCommand()
	}
	var payload command.MovementPlanPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	rt := b.Settled.TacticsRuntime
	actor := cmd.ActorID
	current, ok := rt.Plans[actor]
	if !ok || current.Locked || b.Settled.Simultaneous.Locked[actor] {
		return nil, errors.New("your movement is already locked in")
	}
	plan := state.TacticsPlan{Intention: payload.Intention, Cards: append([]string(nil), payload.CardIDs...), Energy: payload.Energy, Sprint: current.Sprint, Charge: current.Charge}
	for _, played := range payload.MapCards {
		plan.MapCards = append(plan.MapCards, state.TacticsMapCard{CardID: played.CardID, Terrain: played.Terrain})
	}
	if err := validatePlan(b, library, actor, plan); err != nil {
		return nil, err
	}
	if err := e.refreshPlan(b, library, actor, &plan); err != nil {
		return nil, err
	}
	plan.Locked = payload.LockIn
	if rt.Plans == nil {
		rt.Plans = map[string]state.TacticsPlan{}
	}
	rt.Plans[actor] = plan
	events := []event.Event{settledEvent(event.TypeTacticsMovementPlanned, b, actor, map[string]any{"locked": payload.LockIn})}
	if !payload.LockIn {
		rotateSettledPending(b, actor)
		return events, nil
	}
	b.Settled.Simultaneous.Locked[actor] = true
	closeSettledInputForActor(b, actor, state.ActorLockedIn)
	events = append(events, lockedInEvent(b, actor))
	if !simultaneousAllLocked(b) {
		return events, nil
	}
	next, err := e.resolveMovement(b, library)
	return append(events, next...), err
}

// resolveMovement is the movement reveal: committed cards and energy are
// spent, the round's movement is simulated, plans that ran get Running, and
// each attack connects or misses. The attacks that connected then become
// damage sources, and Defense opens for them.
func (e Engine) resolveMovement(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	rt := b.Settled.TacticsRuntime
	closeSettledWindow(b)
	endSimultaneousStep(b)
	movers, budgets, err := tacticsMovers(b, library, rt.Plans)
	if err != nil {
		return nil, err
	}
	for _, id := range sortedSettledActorIDs(b) {
		plan, ok := rt.Plans[id]
		if !ok || !livingCreature(b, id) {
			continue
		}
		a := b.Actors[id]
		spent := plan.Energy
		for _, card := range plan.Cards {
			moveCard(&a.Cards, card, operation.ZoneHand, operation.ZoneDiscard)
		}
		for _, played := range plan.MapCards {
			definition := library.Cards[b.Settled.Actors[id].CardInstances[played.CardID].DefinitionID]
			destination := operation.CardZone(definition.Play.Destination)
			if destination == "" || destination == operation.ZoneHand {
				destination = operation.ZoneDiscard
			}
			moveCard(&a.Cards, played.CardID, operation.ZoneHand, destination)
			spent += definition.Cost.Energy
		}
		b.Actors[id] = a
		spendEnergy(b, id, min(spent, b.Actors[id].Resources.EnergyPoints))
	}
	seed, err := e.namedIntn(b, "tactics_movement", 1<<30)
	if err != nil {
		return nil, err
	}
	start := map[string]tactics.Square{}
	for id, square := range rt.Positions {
		start[id] = square
	}
	result := tactics.Simulate(tacticsGrid(b), movers, uint64(seed))
	for id, square := range result.Final {
		rt.Positions[id] = square
	}
	var running []string
	for _, id := range sortedSettledActorIDs(b) {
		if budget, ok := budgets[id]; ok && budget.Running {
			applyStatus(b, library, id, rt.Rules.RunningStatus, 1)
			running = append(running, id)
		}
	}
	e.connectAttacks(b, result)
	public := map[string]state.TacticsPlan{}
	for id, plan := range rt.Plans {
		public[id] = plan
	}
	rt.Movement = &state.TacticsMovement{Round: b.Segment.Round, Start: start, Result: result, Plans: public, Budgets: budgets, Running: running, Seed: uint64(seed)}
	rt.Plans = nil
	events := []event.Event{settledEvent(event.TypeTacticsMovementResolved, b, "", map[string]any{
		"round": b.Segment.Round, "timeline": result.Timeline, "start": start, "final": result.Final, "budgets": budgets, "running": running, "attacks": rt.Attacks,
	})}
	next, err := e.finalizeOffensiveSources(b, library)
	return append(events, next...), err
}

// connectAttacks decides which attacks landed: a direct attack always does;
// an area attack hits every opponent of the caster (allies too when the
// rules say so) standing in its area at t = 1; a charge lands if the
// charger touched its target. The attacker's targets become the creatures
// hit, one damage source each.
func (e Engine) connectAttacks(b *state.Battle, result tactics.Result) {
	rt := b.Settled.TacticsRuntime
	for i := range rt.Attacks {
		attack := &rt.Attacks[i]
		attack.Resolved = true
		attack.Hits = nil
		attack.ContactAt = nil
		switch attack.Delivery {
		case state.DeliveryArea:
			inArea := map[tactics.Square]bool{}
			for _, s := range attack.Area {
				inArea[s] = true
			}
			for _, id := range sortedSettledActorIDs(b) {
				if id == attack.Attacker || !livingCreature(b, id) || !inArea[result.Final[id]] {
					continue
				}
				if tactics.Opponents(b.Actors[id].TeamID, b.Actors[attack.Attacker].TeamID) || rt.Rules.AreaHitsAllies {
					attack.Hits = append(attack.Hits, id)
				}
			}
			attack.Outcome = state.TacticsDodged
		case state.DeliveryCharge:
			if at, ok := result.Contact[attack.Attacker]; ok {
				contact := at
				attack.ContactAt = &contact
				attack.Hits = []string{attack.Target}
			}
			attack.Outcome = state.TacticsOutOfReach
		default:
			attack.Hits = []string{attack.Target}
		}
		if len(attack.Hits) > 0 {
			attack.Outcome = state.TacticsHit
		}
		run := b.Settled.Actors[attack.Attacker]
		run.SelectedTargetIDs = append([]string(nil), attack.Hits...)
		b.Settled.Actors[attack.Attacker] = run
	}
}

// tacticsMissed reports an attack that did not connect on the grid.
func tacticsMissed(b *state.Battle, actor string) bool {
	if !tacticsBattle(b) {
		return false
	}
	for _, attack := range b.Settled.TacticsRuntime.Attacks {
		if attack.Attacker == actor {
			return attack.Outcome != state.TacticsHit
		}
	}
	return true
}

// --- Legal actions ----------------------------------------------------------

// movementActions offers the participant's current plan locked in, and every
// plan one change away: another intention, a card or map card added or
// removed, another energy amount. The authority accepts any valid plan.
func movementActions(b *state.Battle, library content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	rt := b.Settled.TacticsRuntime
	current, ok := rt.Plans[actor]
	if !ok || current.Locked {
		return nil
	}
	base := command.MovementPlanPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), Intention: current.Intention, CardIDs: append([]string(nil), current.Cards...), Energy: current.Energy}
	for _, played := range current.MapCards {
		base.MapCards = append(base.MapCards, command.MovementMapCard{CardID: played.CardID, Terrain: played.Terrain})
	}
	var actions []command.Command
	seen := map[string]bool{}
	offer := func(p command.MovementPlanPayload) {
		plan := state.TacticsPlan{Intention: p.Intention, Cards: p.CardIDs, Energy: p.Energy, Sprint: current.Sprint, Charge: current.Charge}
		for _, played := range p.MapCards {
			plan.MapCards = append(plan.MapCards, state.TacticsMapCard{CardID: played.CardID, Terrain: played.Terrain})
		}
		if validatePlan(b, library, actor, plan) != nil {
			return
		}
		raw, _ := json.Marshal(p)
		if seen[string(raw)] {
			return
		}
		seen[string(raw)] = true
		actions = append(actions, legalCommand(b.ID, actor, command.TypeMovementPlan, p))
	}
	locked := base
	locked.LockIn = true
	offer(locked)
	clearCommitments := func(p command.MovementPlanPayload) command.MovementPlanPayload {
		p.CardIDs, p.MapCards, p.Energy = nil, nil, 0
		return p
	}
	if current.Charge == "" {
		hold := clearCommitments(base)
		hold.Intention = tactics.Intention{Kind: tactics.IntentHold}
		offer(hold)
		grid := tacticsGrid(b)
		for y := 0; y < grid.Height(); y++ {
			for x := 0; x < grid.Width(); x++ {
				square := tactics.Square{X: x, Y: y}
				p := base
				if current.Intention.Kind == tactics.IntentHold {
					p = clearCommitments(base)
				}
				p.Intention = tactics.Intention{Kind: tactics.IntentMoveTo, Square: &square}
				offer(p)
			}
		}
		for _, id := range sortedSettledActorIDs(b) {
			for _, kind := range []string{tactics.IntentPursue, tactics.IntentFlee} {
				p := base
				p.Intention = tactics.Intention{Kind: kind, Target: id}
				offer(p)
			}
		}
	}
	if current.Intention.Kind == tactics.IntentHold {
		return actions
	}
	run := b.Settled.Actors[actor]
	for _, card := range b.Actors[actor].Cards.Hand {
		p := base
		if index := indexString(p.CardIDs, card); index >= 0 {
			p.CardIDs = append(append([]string(nil), p.CardIDs[:index]...), p.CardIDs[index+1:]...)
		} else {
			p.CardIDs = append(append([]string(nil), p.CardIDs...), card)
		}
		offer(p)
		effect := library.Cards[run.CardInstances[card].DefinitionID].MapEffect
		if effect == nil || effect.Kind != "move" {
			continue
		}
		var without []command.MovementMapCard
		playing := false
		for _, played := range base.MapCards {
			if played.CardID == card {
				playing = true
				continue
			}
			without = append(without, played)
		}
		if playing {
			p := base
			p.MapCards = without
			offer(p)
			continue
		}
		terrains := []string{""}
		if effect.ChooseTerrain {
			terrains = nil
			for id, terrain := range rt.Rules.Terrain {
				if terrain.Passable {
					terrains = append(terrains, id)
				}
			}
			sort.Strings(terrains)
		}
		for _, terrain := range terrains {
			p := base
			p.CardIDs = removeString(base.CardIDs, card)
			p.MapCards = append(append([]command.MovementMapCard(nil), base.MapCards...), command.MovementMapCard{CardID: card, Terrain: terrain})
			offer(p)
		}
	}
	for energy := 0; energy <= b.Actors[actor].Resources.EnergyPoints; energy += rt.Rules.EnergyPerMove {
		p := base
		p.Energy = energy
		offer(p)
	}
	return actions
}

// --- Defense ---------------------------------------------------------------

// defenseAllowedNow reports whether a defense can be chosen now: a creature
// with a status that limits it to on-the-move defenses (Running) may use
// only defenses tagged on_the_move.
func defenseAllowedNow(b *state.Battle, library content.BattleLibrary, actor string, ability content.BattleAbilityDefinition) bool {
	return ability.OnTheMove || !onTheMoveOnly(b, library, actor)
}

func onTheMoveOnly(b *state.Battle, library content.BattleLibrary, actor string) bool {
	for _, status := range b.Actors[actor].Statuses {
		if m := library.Statuses[status.DefinitionID].Modifiers; m != nil && m.OnTheMoveDefensesOnly && status.Stacks > 0 {
			return true
		}
	}
	return false
}

// DisabledDefenses lists an actor's defenses unavailable right now, with the
// reason, for the viewer's UI.
func DisabledDefenses(b *state.Battle, actor string) map[string]string {
	library, err := settledLibrary(b)
	if err != nil || b.Settled == nil || !onTheMoveOnly(b, library, actor) {
		return nil
	}
	out := map[string]string{}
	for _, id := range b.Settled.Actors[actor].DefensiveAbilityIDs {
		if !library.Abilities[id].OnTheMove {
			out[id] = "Running: only on-the-move defenses this round"
		}
	}
	return out
}
