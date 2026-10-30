package tacticsbattle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

func packRules(t *testing.T) (tactics.Pack, tactics.Rules) {
	t.Helper()
	pack, err := tactics.LoadPack(filepath.Join("..", "..", "..", "content", "tactics_v1"))
	if err != nil {
		t.Fatal(err)
	}
	return pack, pack.Rules
}

var deliveries = map[string]map[string]snapshot.DeliveryView{
	"melee":  {"sunken_toll": {AbilityDelivery: content.AbilityDelivery{Type: "direct", Range: 1, LineOfSight: true}}},
	"ranged": {"brine_lash": {AbilityDelivery: content.AbilityDelivery{Type: "direct", Range: 4, LineOfSight: true}}, "brine_slap": {AbilityDelivery: content.AbilityDelivery{Type: "direct", Range: 1, LineOfSight: true}}},
	"area":   {"tide_crown": {AbilityDelivery: content.AbilityDelivery{Type: "area", Range: 5, LineOfSight: true, Area: &content.DeliveryArea{Shape: "square", Radius: 1}}}, "crown_strike": {AbilityDelivery: content.AbilityDelivery{Type: "direct", Range: 1, LineOfSight: true}}},
}
var attacks = map[string][]string{"melee": {"sunken_toll"}, "ranged": {"brine_lash", "brine_slap"}, "area": {"tide_crown", "crown_strike"}}

// view is a synthetic viewer-safe view of the enemy seat "foe" (role) and
// the hero on an arena.
func view(t *testing.T, arena, role string, foe, hero tactics.Square, hand int, others map[string]tactics.Square) *snapshot.Battle {
	t.Helper()
	pack, rules := packRules(t)
	a := pack.Arenas[arena]
	tv := &snapshot.TacticsView{Rules: rules, Creatures: map[string]snapshot.TacticsCreatureView{}}
	for y := 0; y < a.Height(); y++ {
		row := make([]string, a.Width())
		for x := range row {
			row[x] = a.TerrainAt(tactics.Square{X: x, Y: y})
		}
		tv.Arena.Terrain = append(tv.Arena.Terrain, row)
	}
	speed := 1
	tv.Creatures["foe"] = snapshot.TacticsCreatureView{Position: foe, BaseSpeed: speed, Role: role, Deliveries: deliveries[role]}
	tv.Creatures[HeroSeat] = snapshot.TacticsCreatureView{Position: hero, BaseSpeed: 1}
	actors := map[string]snapshot.Actor{HeroSeat: {TeamID: "heroes"}}
	var cards []string
	for i := 0; i < hand; i++ {
		cards = append(cards, fmt.Sprintf("card-%d", i))
	}
	actors["foe"] = snapshot.Actor{TeamID: "monsters", Hand: cards, OffensiveAbilities: attacks[role]}
	for id, at := range others {
		tv.Creatures[id] = snapshot.TacticsCreatureView{Position: at, BaseSpeed: 1}
		actors[id] = snapshot.Actor{TeamID: "monsters"}
	}
	return &snapshot.Battle{Stage: "movement", Actors: actors, Tactics: tv}
}

func movementOf(t *testing.T, role string, snap *snapshot.Battle) command.MovementPlanPayload {
	t.Helper()
	template := command.Command{BattleID: "b", ActorID: "foe", Type: command.TypeMovementPlan, Payload: mustJSON(command.MovementPlanPayload{PendingInputID: "p"})}
	p := scriptedPolicy{enemies: map[string]EnemyConfig{"foe": {Role: role}}}
	action, err := p.choose("foe", engine.Result{Snapshot: snap, LegalActions: []command.Command{template}})
	if err != nil {
		t.Fatal(err)
	}
	var plan command.MovementPlanPayload
	if err := json.Unmarshal(action.Payload, &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.LockIn || plan.PendingInputID != "p" {
		t.Fatalf("the plan must lock in on the pending input: %+v", plan)
	}
	return plan
}

func sqr(x, y int) tactics.Square { return tactics.Square{X: x, Y: y} }

func TestMeleeEnemiesPursueAndSpendOnlyToReachRange(t *testing.T) {
	cases := []struct {
		name  string
		foe   tactics.Square
		hand  int
		cards int
	}{
		{"base speed reaches", sqr(5, 4), 3, 0},
		{"one card reaches", sqr(6, 4), 3, 1},
		{"one card is not enough", sqr(9, 4), 3, 0},
		{"never the last card", sqr(6, 4), 1, 0},
	}
	for _, c := range cases {
		plan := movementOf(t, "melee", view(t, "open_field", "melee", c.foe, sqr(3, 4), c.hand, nil))
		if plan.Intention.Kind != tactics.IntentPursue || plan.Intention.Target != HeroSeat || len(plan.CardIDs) != c.cards {
			t.Errorf("%s: plan %+v, want pursue with %d cards", c.name, plan, c.cards)
		}
	}
}

func TestRangedEnemiesFleeHoldOrMoveIntoRange(t *testing.T) {
	plan := movementOf(t, "ranged", view(t, "open_field", "ranged", sqr(4, 4), sqr(3, 4), 3, nil))
	if plan.Intention.Kind != tactics.IntentFlee || len(plan.CardIDs) != 0 {
		t.Fatalf("adjacent: %+v, want flee", plan)
	}
	plan = movementOf(t, "ranged", view(t, "open_field", "ranged", sqr(6, 4), sqr(3, 4), 3, nil))
	if plan.Intention.Kind != tactics.IntentHold {
		t.Fatalf("in range: %+v, want hold", plan)
	}
	plan = movementOf(t, "ranged", view(t, "open_field", "ranged", sqr(9, 4), sqr(3, 4), 3, nil))
	if plan.Intention.Kind != tactics.IntentMoveTo || plan.Intention.Square == nil || *plan.Intention.Square != sqr(7, 4) || len(plan.CardIDs) != 1 {
		t.Fatalf("out of range: %+v, want move to (7,4) with one card", plan)
	}
	// Out of sight behind a pillar: move to the nearest square that sees.
	hidden := view(t, "doorway", "area", sqr(7, 2), sqr(3, 2), 1, nil)
	plan = movementOf(t, "area", hidden)
	if plan.Intention.Kind != tactics.IntentMoveTo {
		t.Fatalf("out of sight: %+v, want move to a square with sight", plan)
	}
	grid := gridOf(hidden.Tactics)
	if !grid.LineOfSight(*plan.Intention.Square, sqr(3, 2), nil) || grid.Distance(*plan.Intention.Square, sqr(3, 2)) > 5 {
		t.Fatalf("the chosen square %v cannot hit the hero", *plan.Intention.Square)
	}
}

func abilityAction(ability string, targets []string, square *tactics.Square) command.Command {
	return command.Command{Type: command.TypePlanningAbility, Payload: mustJSON(command.PlanningAbilityPayload{AbilityID: ability, TargetIDs: targets, TargetSquare: square})}
}

func offenseOf(t *testing.T, role string, snap *snapshot.Battle, actions ...command.Command) command.Command {
	t.Helper()
	snap.Stage = "planning"
	actions = append(actions, command.Command{Type: command.TypePlanningPass, Payload: mustJSON(command.PlanningPassPayload{})})
	p := scriptedPolicy{enemies: map[string]EnemyConfig{"foe": {Role: role}}}
	action, err := p.choose("foe", engine.Result{Snapshot: snap, LegalActions: actions})
	if err != nil {
		t.Fatal(err)
	}
	return action
}

func TestRangedAndAreaEnemiesPreferRangeThenBackupMelee(t *testing.T) {
	snap := view(t, "open_field", "ranged", sqr(5, 4), sqr(3, 4), 1, nil)
	got := offenseOf(t, "ranged", snap, abilityAction("brine_slap", []string{HeroSeat}, nil), abilityAction("brine_lash", []string{HeroSeat}, nil))
	if !strings.Contains(string(got.Payload), "brine_lash") {
		t.Fatalf("with a ranged target it uses Brine Lash: %s", got.Payload)
	}
	got = offenseOf(t, "ranged", snap, abilityAction("brine_slap", []string{HeroSeat}, nil))
	if !strings.Contains(string(got.Payload), "brine_slap") {
		t.Fatalf("without one it falls back to Brine Slap: %s", got.Payload)
	}
	got = offenseOf(t, "ranged", snap)
	if got.Type != command.TypePlanningPass {
		t.Fatalf("with no legal target it does not attack: %s", got.Type)
	}
	// Area: aim at the player's current square, not another square.
	area := view(t, "open_field", "area", sqr(7, 4), sqr(3, 4), 1, nil)
	got = offenseOf(t, "area", area, abilityAction("tide_crown", nil, &tactics.Square{X: 4, Y: 4}), abilityAction("tide_crown", nil, &tactics.Square{X: 3, Y: 4}))
	if !strings.Contains(string(got.Payload), `"target_square":{"x":3,"y":4}`) {
		t.Fatalf("Tide Crown must aim at the hero's square: %s", got.Payload)
	}
}

func TestEnemyDefendsWithItsLegalDefenseOrPasses(t *testing.T) {
	snap := &snapshot.Battle{Stage: "defense_selection", Actors: map[string]snapshot.Actor{"foe": {DefensiveAbilities: []string{"slip_the_current"}}}, Tactics: &snapshot.TacticsView{}}
	pass := command.Command{Type: command.TypePlanningPass, Payload: mustJSON(command.PlanningPassPayload{})}
	perSource := command.Command{Type: command.TypePlanningPass, Payload: mustJSON(command.PlanningPassPayload{SourceID: "s"})}
	defend := command.Command{Type: command.TypePlanningAbility, Payload: mustJSON(command.PlanningAbilityPayload{AbilityID: "slip_the_current", TargetIDs: []string{"s"}})}
	p := scriptedPolicy{enemies: map[string]EnemyConfig{"foe": {Role: "melee"}}}
	got, _ := p.choose("foe", engine.Result{Snapshot: snap, LegalActions: []command.Command{pass, defend, perSource}})
	if got.Type != command.TypePlanningAbility {
		t.Fatalf("it defends when it can: %s", got.Type)
	}
	got, _ = p.choose("foe", engine.Result{Snapshot: snap, LegalActions: []command.Command{perSource, pass}})
	if string(got.Payload) != string(pass.Payload) {
		t.Fatalf("with no legal defense it takes the main pass: %s", got.Payload)
	}
}
