package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

const tacticsContent = "../../../content"

func tacticsLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	lib := settledTestLibrary(t)
	var err error
	for _, pack := range []string{"venom_v1", "general_v1", "minions_v1", "adventurer_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join(tacticsContent, pack)); err != nil {
			t.Fatal(err)
		}
	}
	// Map Move cards (published cards in the real game) for the tests.
	for id, effect := range map[string]content.CardMapEffect{
		"test_forced_march":  {Kind: "move", Amount: 4, IgnoreTerrain: true, Text: "March."},
		"test_know_the_land": {Kind: "move", Amount: 4, ChooseTerrain: true, Text: "Know."},
		"test_trailblaze":    {Kind: "move", Amount: 4, Text: "Blaze."},
	} {
		card := lib.Cards["shed_ribbon"]
		card.ID, card.Name = id, id
		e := effect
		card.MapEffect = &e
		card.Cost.Energy = 5
		lib.Cards[id] = content.PrepareMapCard(card)
	}
	if lib, _, err = content.ApplyTacticsPack(lib, filepath.Join(tacticsContent, "tactics_v1")); err != nil {
		t.Fatal(err)
	}
	return lib
}

func tacticsRules(t *testing.T) tactics.Rules {
	t.Helper()
	rules, err := tactics.LoadRules(filepath.Join(tacticsContent, "tactics_v1", "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

type seat struct {
	id, definition, team string
	x, y                 int
	hand                 []string
}

// tacticsFixture is a tactics battle at Offensive entry: every seat external,
// on the given arena rows ('.' floor, '~' swamp, '#' wall).
func tacticsFixture(t *testing.T, rows []string, seats []seat, tune func(*tactics.Rules)) (state.Battle, content.BattleLibrary, Engine) {
	t.Helper()
	lib := tacticsLibrary(t)
	catalog, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	var actors []state.ActorSetup
	runtimes := map[string]state.SettledActorRuntime{}
	placement := map[string]tactics.Square{}
	arena := tactics.Arena{SchemaVersion: 1, ID: "test", Name: "Test", Legend: map[string]string{".": "floor", "~": "swamp", "#": "wall"}, Grid: rows}
	for _, s := range seats {
		def := lib.Combatants[s.definition]
		var loadout []state.DiceLoadoutEntry
		for _, entry := range def.DiceLoadout {
			loadout = append(loadout, state.DiceLoadoutEntry{DiceID: entry.DiceID, Count: entry.Count})
		}
		actors = append(actors, state.ActorSetup{ID: s.id, TeamID: s.team, DefinitionID: s.definition, ControllerType: state.ControllerExternal, DiceLoadout: loadout, Resources: state.ResourceState{EnergyPoints: 20, MaxEnergyPoints: 75}})
		runtimes[s.id] = state.SettledActorRuntime{MaxRolls: 3, HandLimit: 99}
		placement[s.id] = tactics.Square{X: s.x, Y: s.y}
		if s.team == "heroes" {
			arena.PlayerStart = placement[s.id]
		} else {
			arena.EnemyStarts = append(arena.EnemyStarts, placement[s.id])
		}
	}
	b, err := state.NewBattleFromSetup("tactics-test", state.BattleSetup{Actors: actors, SettledCatalog: catalog, SettledActors: runtimes})
	if err != nil {
		t.Fatal(err)
	}
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 5}
	b.Settled.UnifiedDefense = true
	b.Settled.SimultaneousPlay = true
	b.Settled.Initialized = true
	for _, s := range seats {
		def := lib.Combatants[s.definition]
		r := b.Settled.Actors[s.id]
		r.OffensiveAbilityIDs = def.AbilityBoard.Offensive
		r.DefensiveAbilityIDs = def.AbilityBoard.Defensive
		r.UsedAbilities = map[string]int{}
		r.CardInstances = map[string]state.CardInstance{}
		a := b.Actors[s.id]
		a.Cards = state.CardZones{}
		for i, card := range s.hand {
			instance := fmt.Sprintf("%s-hand-%d-%s", s.id, i, card)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: card}
			a.Cards.Hand = append(a.Cards.Hand, instance)
		}
		for i := 0; i < 6; i++ {
			instance := fmt.Sprintf("%s-deck-%d", s.id, i)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: def.Decklist[0].CardID}
			a.Cards.Deck = append(a.Cards.Deck, instance)
		}
		a.Health = state.HealthMetadata{Model: "card_zones", MaxHealth: len(a.Cards.Hand) + len(a.Cards.Deck)}
		b.Actors[s.id] = a
		b.Settled.Actors[s.id] = r
	}
	rules := tacticsRules(t)
	if tune != nil {
		tune(&rules)
	}
	if err := InitTactics(&b, command.TacticsSetup{Arena: arena, Rules: rules, Placement: placement}); err != nil {
		t.Fatal(err)
	}
	b.Segment.Current = segment.Offensive
	b.Flow = state.NewSegmentFlowState(b.Segment)
	b.Flow.Entered = true
	e := NewEngine()
	if _, err := e.progressSettled(&b); err != nil {
		t.Fatal(err)
	}
	return b, lib, e
}

func openRows(w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = strings.Repeat(".", w)
	}
	return rows
}

func apply(t *testing.T, e Engine, b *state.Battle, cmd command.Command) []event.Event {
	t.Helper()
	progressed, err := e.ApplyBattleCommand(b, cmd)
	if err != nil {
		t.Fatalf("%s %s: %v", cmd.ActorID, cmd.Type, err)
	}
	return progressed.Events
}

func pendingFor(t *testing.T, b *state.Battle, actor string) state.PendingInput {
	t.Helper()
	pending, ok := b.Flow.PendingInput[actor]
	if !ok {
		t.Fatalf("%s has no pending input at %s/%s", actor, b.Settled.Stage, simultaneousStep(b))
	}
	return pending
}

// lockOffense locks every seat in: by rolling, or by sprinting (skipping).
func lockOffense(t *testing.T, e Engine, b *state.Battle, sprinters ...string) {
	t.Helper()
	for _, id := range sortedSettledActorIDs(b) {
		pending := pendingFor(t, b, id)
		if containsString(sprinters, id) {
			apply(t, e, b, legalCommand(b.ID, id, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
			continue
		}
		apply(t, e, b, legalCommand(b.ID, id, command.TypePlanningRoll, command.PlanningRollPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
	}
}

// forceDice sets an actor's rolled faces (as if they had rolled them).
func forceDice(t *testing.T, b *state.Battle, lib content.BattleLibrary, actor string, faces ...int) {
	t.Helper()
	r := b.Settled.Actors[actor]
	r.FinalDice = rolledCombatFaces(lib, b.Actors[actor].DiceLoadout, faces)
	r.QualifiedAbilityIDs = qualifiedAbilities(lib, r.OffensiveAbilityIDs, r.FinalDice, r.AbilityModifiers)
	r.RollsUsed = r.MaxRolls
	b.Settled.Actors[actor] = r
}

func attackActions(e Engine, b *state.Battle, actor, ability string) []command.PlanningAbilityPayload {
	var out []command.PlanningAbilityPayload
	for _, a := range e.LegalActions(b, actor) {
		if a.Type != command.TypePlanningAbility {
			continue
		}
		var p command.PlanningAbilityPayload
		_ = json.Unmarshal(a.Payload, &p)
		if p.AbilityID == ability {
			out = append(out, p)
		}
	}
	return out
}

// attack locks in an ability on a creature or a square.
func attack(t *testing.T, e Engine, b *state.Battle, actor, ability, target string, square *tactics.Square, tier string) []event.Event {
	t.Helper()
	for _, a := range e.LegalActions(b, actor) {
		if a.Type != command.TypePlanningAbility {
			continue
		}
		var p command.PlanningAbilityPayload
		_ = json.Unmarshal(a.Payload, &p)
		if p.AbilityID != ability || (tier != "" && p.TierID != tier) {
			continue
		}
		if square != nil && (p.TargetSquare == nil || *p.TargetSquare != *square) {
			continue
		}
		if target != "" && first(p.TargetIDs) != target {
			continue
		}
		return apply(t, e, b, a)
	}
	t.Fatalf("%s cannot attack %s with %s (%v)", actor, target, ability, square)
	return nil
}

func passOffense(t *testing.T, e Engine, b *state.Battle, actor string) {
	t.Helper()
	pending := pendingFor(t, b, actor)
	apply(t, e, b, legalCommand(b.ID, actor, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
}

type movePlan struct {
	intent tactics.Intention
	cards  []string
	maps   []command.MovementMapCard
	energy int
}

func plan(t *testing.T, e Engine, b *state.Battle, actor string, p movePlan, lock bool) []event.Event {
	t.Helper()
	pending := pendingFor(t, b, actor)
	return apply(t, e, b, legalCommand(b.ID, actor, command.TypeMovementPlan, command.MovementPlanPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), Intention: p.intent, CardIDs: p.cards, MapCards: p.maps, Energy: p.energy, LockIn: lock}))
}

func planErr(b *state.Battle, e Engine, actor string, p movePlan) error {
	pending := b.Flow.PendingInput[actor]
	_, err := e.ApplyBattleCommand(b, legalCommand(b.ID, actor, command.TypeMovementPlan, command.MovementPlanPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), Intention: p.intent, CardIDs: p.cards, MapCards: p.maps, Energy: p.energy}))
	return err
}

func toSquare(x, y int) tactics.Intention {
	s := tactics.Square{X: x, Y: y}
	return tactics.Intention{Kind: tactics.IntentMoveTo, Square: &s}
}
func holdPlan() movePlan { return movePlan{intent: tactics.Intention{Kind: tactics.IntentHold}} }

// holdAll locks a Hold for every seat still planning, except those listed.
func holdAll(t *testing.T, e Engine, b *state.Battle, except ...string) []event.Event {
	t.Helper()
	var events []event.Event
	for _, id := range sortedSettledActorIDs(b) {
		if _, ok := b.Flow.PendingInput[id]; ok && !containsString(except, id) {
			events = append(events, plan(t, e, b, id, holdPlan(), true)...)
		}
	}
	return events
}

func handCard(t *testing.T, b *state.Battle, actor, definition string) string {
	t.Helper()
	for _, id := range b.Actors[actor].Cards.Hand {
		if b.Settled.Actors[actor].CardInstances[id].DefinitionID == definition {
			return id
		}
	}
	t.Fatalf("%s holds no %s", actor, definition)
	return ""
}

func sourcesOn(b *state.Battle, attacker string) map[string]int {
	out := map[string]int{}
	for _, s := range b.Settled.OffensiveSources {
		if s.SourceActorID == attacker {
			out[s.TargetActorID] += s.BaseAmount
		}
	}
	return out
}

func attackOf(b *state.Battle, attacker string) state.TacticsAttack {
	for _, a := range b.Settled.TacticsRuntime.Attacks {
		if a.Attacker == attacker {
			return a
		}
	}
	return state.TacticsAttack{}
}

func sq(x, y int) *tactics.Square { return &tactics.Square{X: x, Y: y} }

// --- Offense legality ------------------------------------------------------

func TestTargetsMustBeInRangeAndSightFromTheRoundStart(t *testing.T) {
	rows := []string{
		"............",
		"............",
		"....#.......",
		"............",
	}
	b, lib, e := tacticsFixture(t, rows, []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 0, y: 2},
		{id: "near", definition: "tactics_bell_diver", team: "monsters", x: 1, y: 2},
		{id: "far", definition: "tactics_bell_diver", team: "monsters", x: 9, y: 0},
		{id: "hidden", definition: "tactics_bell_diver", team: "monsters", x: 6, y: 2},
	}, nil)
	lockOffense(t, e, &b)
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 6)
	targets := map[string][]string{}
	for _, ability := range []string{"adventurer_strike", "magic_missile", "charge"} {
		for _, p := range attackActions(e, &b, "hero", ability) {
			if !containsString(targets[ability], first(p.TargetIDs)) {
				targets[ability] = append(targets[ability], first(p.TargetIDs))
			}
		}
	}
	// Strike is melee; the missile reaches 6 squares with sight (the wall at
	// (4,2) hides "hidden"; "near" itself blocks the row too); the charge
	// reaches 4 + 1 squares.
	if fmt.Sprint(targets["adventurer_strike"]) != "[near]" {
		t.Fatalf("Strike targets = %v", targets["adventurer_strike"])
	}
	if fmt.Sprint(targets["magic_missile"]) != "[near]" {
		t.Fatalf("Magic Missile targets = %v", targets["magic_missile"])
	}
	if fmt.Sprint(targets["charge"]) != "[near]" {
		t.Fatalf("Charge targets = %v", targets["charge"])
	}
	pending := pendingFor(t, &b, "hero")
	_, err := e.ApplyBattleCommand(&b, legalCommand(b.ID, "hero", command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: "magic_missile", TargetIDs: []string{"hidden"}}))
	if err == nil || !strings.Contains(err.Error(), "out of range or out of sight") {
		t.Fatalf("an unseen target was accepted: %v", err)
	}
}

func TestAreaAttacksTargetSquaresInRangeAndSight(t *testing.T) {
	b, lib, e := tacticsFixture(t, []string{"............", "......#.....", "............"}, []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 0, y: 1},
		{id: "foe", definition: "tactics_bell_diver", team: "monsters", x: 11, y: 1},
	}, nil)
	lockOffense(t, e, &b)
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 5)
	squares := map[tactics.Square]bool{}
	for _, p := range attackActions(e, &b, "hero", "fireball") {
		if p.TargetSquare == nil || len(p.TargetIDs) != 0 {
			t.Fatalf("Fireball must target a square: %+v", p)
		}
		squares[*p.TargetSquare] = true
	}
	if !squares[tactics.Square{X: 5, Y: 1}] || squares[tactics.Square{X: 6, Y: 1}] || squares[tactics.Square{X: 7, Y: 1}] || squares[tactics.Square{X: 6, Y: 0}] {
		t.Fatalf("Fireball squares: range 5, no wall, no square behind the wall: %v", squares)
	}
	if squares[tactics.Square{X: 0, Y: 1}] != true {
		t.Fatal("the caster may aim at its own square")
	}
}

// --- Connections (§11.8, 9) ------------------------------------------------

func TestDirectAttackLandsEvenWhenTheTargetRunsOutOfRange(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
		{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 5, y: 4, hand: []string{"shed_ribbon", "shed_ribbon", "shed_ribbon"}},
	}, nil)
	lockOffense(t, e, &b, "eel")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 6)
	attack(t, e, &b, "hero", "magic_missile", "eel", nil, "")
	if b.Settled.Stage != stageMovement {
		t.Fatalf("stage %s, want movement", b.Settled.Stage)
	}
	eelCards := b.Actors["eel"].Cards.Hand
	plan(t, e, &b, "eel", movePlan{intent: tactics.Intention{Kind: tactics.IntentFlee, Target: "hero"}, cards: eelCards}, true)
	plan(t, e, &b, "hero", holdPlan(), true)
	if got := b.Settled.TacticsRuntime.Positions["eel"]; tactics.Distance(tactics.DiagonalChebyshev, got, tactics.Square{X: 1, Y: 4}) <= 6 {
		t.Fatalf("the eel did not run out of range: %v", got)
	}
	if got := sourcesOn(&b, "hero"); got["eel"] != 6 {
		t.Fatalf("the homing missile must still land: %v", got)
	}
	if b.Segment.Current != segment.Defensive {
		t.Fatalf("segment %s, want defense", b.Segment.Current)
	}
}

func TestAreaAttackHitsWhoStaysAndMissesWhoLeaves(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
		{id: "stay", definition: "tactics_bell_diver", team: "monsters", x: 5, y: 4},
		{id: "go", definition: "tactics_ribbon_eel", team: "monsters", x: 5, y: 5},
	}, nil)
	lockOffense(t, e, &b, "stay", "go")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 5)
	attack(t, e, &b, "hero", "fireball", "", sq(5, 4), "")
	if a := attackOf(&b, "hero"); a.Delivery != state.DeliveryArea || len(a.Area) != 9 {
		t.Fatalf("fireball attack = %+v", a)
	}
	plan(t, e, &b, "go", movePlan{intent: toSquare(5, 7)}, true)
	holdAll(t, e, &b)
	got := sourcesOn(&b, "hero")
	if got["stay"] != 8 || got["go"] != 0 || len(got) != 1 {
		t.Fatalf("the fireball must hit only the one that stayed: %v", got)
	}
	if a := attackOf(&b, "hero"); a.Outcome != state.TacticsHit || fmt.Sprint(a.Hits) != "[stay]" {
		t.Fatalf("attack = %+v", a)
	}
}

func TestAreaAttackThatHitsNobodyIsDodged(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
		{id: "go", definition: "tactics_ribbon_eel", team: "monsters", x: 5, y: 4},
	}, nil)
	lockOffense(t, e, &b, "go")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 5)
	attack(t, e, &b, "hero", "fireball", "", sq(5, 4), "")
	plan(t, e, &b, "go", movePlan{intent: toSquare(5, 1)}, true)
	plan(t, e, &b, "hero", holdPlan(), true)
	if len(b.Settled.OffensiveSources) != 0 {
		t.Fatalf("a dodged fireball made damage: %+v", b.Settled.OffensiveSources)
	}
	if a := attackOf(&b, "hero"); a.Outcome != state.TacticsDodged {
		t.Fatalf("outcome %s, want dodged", a.Outcome)
	}
}

func TestAreaDamageIsOneSourcePerCreatureDefendedSeparately(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
		{id: "a", definition: "tactics_bell_diver", team: "monsters", x: 5, y: 4},
		{id: "b", definition: "tactics_bell_diver", team: "monsters", x: 6, y: 5},
	}, nil)
	lockOffense(t, e, &b, "a", "b")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 5)
	attack(t, e, &b, "hero", "fireball", "", sq(5, 4), "")
	holdAll(t, e, &b)
	var ids []string
	for _, s := range b.Settled.OffensiveSources {
		if s.SourceActorID == "hero" {
			ids = append(ids, s.ID)
			if s.BaseAmount != 8 {
				t.Fatalf("each creature hit takes the full 8: %+v", s)
			}
		}
	}
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("want two separate sources, got %v", ids)
	}
	if simultaneousStep(&b) != state.SimultaneousDefenseBefore {
		t.Fatalf("step %s, want Defense Before", simultaneousStep(&b))
	}
	// Each one hit chooses its own defense against its own source.
	for _, id := range []string{"a", "b"} {
		var picked bool
		for _, action := range e.LegalActions(&b, id) {
			var p command.PlanningAbilityPayload
			if action.Type == command.TypePlanningAbility && json.Unmarshal(action.Payload, &p) == nil && len(p.TargetIDs) == 1 {
				apply(t, e, &b, action)
				picked = true
				break
			}
		}
		if !picked {
			t.Fatalf("%s has no defense against its own source", id)
		}
	}
	defended := map[string]bool{}
	for source, defense := range b.Settled.DefenseHistory {
		defended[source] = defense.AbilityID == "brass_helm"
	}
	for _, id := range ids {
		if !defended[id] {
			t.Fatalf("source %s was not defended separately: %+v", id, b.Settled.DefenseHistory)
		}
	}
}

func TestChargeConnectsOnContactAndMissesWhenOutrun(t *testing.T) {
	setup := func(t *testing.T) (state.Battle, content.BattleLibrary, Engine) {
		b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
			{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
			{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 5, y: 4, hand: []string{"shed_ribbon", "shed_ribbon", "shed_ribbon"}},
		}, nil)
		lockOffense(t, e, &b, "eel")
		forceDice(t, &b, lib, "hero", 1, 1, 1, 1, 4)
		attack(t, e, &b, "hero", "charge", "eel", nil, "4_swords")
		return b, lib, e
	}
	b, _, e := setup(t)
	if _, waiting := b.Flow.PendingInput["hero"]; waiting {
		t.Fatal("a fixed Charge needs no movement input")
	}
	if p := b.Settled.TacticsRuntime.Plans["hero"]; !p.Locked || p.Intention.Kind != tactics.IntentPursue || p.Budget.Total != 4 || p.Budget.Running {
		t.Fatalf("charge plan = %+v", p)
	}
	plan(t, e, &b, "eel", holdPlan(), true)
	if got := sourcesOn(&b, "hero"); got["eel"] != 4 {
		t.Fatalf("the charge must connect: %v", got)
	}
	if a := attackOf(&b, "hero"); a.ContactAt == nil || !a.ContactAt.Equal(tactics.F(3, 4)) {
		t.Fatalf("contact = %v", a.ContactAt)
	}
	if statusStackCount(b.Actors["hero"].Statuses, "running") != 0 {
		t.Fatal("a charge is not running")
	}

	b, _, e = setup(t)
	plan(t, e, &b, "eel", movePlan{intent: tactics.Intention{Kind: tactics.IntentFlee, Target: "hero"}, cards: b.Actors["eel"].Cards.Hand}, true)
	if got := sourcesOn(&b, "hero"); len(got) != 0 {
		t.Fatalf("an outrun charge connected: %v", got)
	}
	if a := attackOf(&b, "hero"); a.Outcome != state.TacticsOutOfReach {
		t.Fatalf("outcome %s, want out_of_reach", a.Outcome)
	}
}

// --- Movement plans (§11.10) -----------------------------------------------

func TestSprintMapCardsEnergyAndSpentCommitments(t *testing.T) {
	rows := []string{"....~~~~....", "....~~~~....", "....~~~~...."}
	b, lib, e := tacticsFixture(t, rows, []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 0, y: 1, hand: []string{"nudge", "try_again", "test_forced_march", "test_know_the_land", "test_trailblaze"}},
		{id: "foe", definition: "tactics_bell_diver", team: "monsters", x: 11, y: 1},
	}, nil)
	lockOffense(t, e, &b, "hero", "foe")
	if !b.Settled.TacticsRuntime.Sprinting["hero"] {
		t.Fatal("skipping offense must sprint")
	}
	nudge, again := handCard(t, &b, "hero", "nudge"), handCard(t, &b, "hero", "try_again")
	march, know := handCard(t, &b, "hero", "test_forced_march"), handCard(t, &b, "hero", "test_know_the_land")
	// Sprint: base 1 × 2, +1 per card and +1 sprint bonus per card.
	plan(t, e, &b, "hero", movePlan{intent: toSquare(11, 0), cards: []string{nudge, again}}, false)
	if got := b.Settled.TacticsRuntime.Plans["hero"].Budget; got.Total != 6 || !got.Running {
		t.Fatalf("sprint with two cards = %+v", got)
	}
	// Each hand card shows its Move in the owner's view: +1 discarded, a map
	// card's amount played, and +1 more of each while sprinting.
	hand := e.ResultForViewer(&b, "hero", ProgressionResult{}).Snapshot.Tactics.HandMove
	if hand[nudge] != (snapshot.TacticsCardMove{Discard: 2}) || hand[march] != (snapshot.TacticsCardMove{Discard: 2, Map: 5}) || len(hand) != 5 {
		t.Fatalf("hand Move while sprinting = %+v", hand)
	}
	if foe := e.ResultForViewer(&b, "foe", ProgressionResult{}).Snapshot.Tactics.HandMove; foe[nudge].Discard != 0 {
		t.Fatalf("another viewer sees the hero's hand Move: %+v", foe)
	}
	// A map card adds its amount (+1 sprint bonus) and pays its energy.
	plan(t, e, &b, "hero", movePlan{intent: toSquare(11, 0), maps: []command.MovementMapCard{{CardID: march}}}, false)
	if got := b.Settled.TacticsRuntime.Plans["hero"].Budget; got.Total != 2+4+1 {
		t.Fatalf("sprint with Forced March = %+v", got)
	}
	// ignore_terrain: the swamp costs 1, so the preview crosses it at full rate.
	preview := b.Settled.TacticsRuntime.Plans["hero"].Preview
	if preview == nil || preview.Spent != 7 || preview.Final.X != 7 {
		t.Fatalf("Forced March preview = %+v", preview)
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(11, 0), maps: []command.MovementMapCard{{CardID: know}}}); err == nil {
		t.Fatal("Know the Land without a terrain was accepted")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(11, 0), maps: []command.MovementMapCard{{CardID: march, Terrain: "swamp"}}}); err == nil {
		t.Fatal("a terrain on Forced March was accepted")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(11, 0), maps: []command.MovementMapCard{{CardID: nudge}}}); err == nil {
		t.Fatal("an ordinary card was played as a map Move card")
	}
	plan(t, e, &b, "hero", movePlan{intent: toSquare(11, 0), maps: []command.MovementMapCard{{CardID: know, Terrain: "swamp"}}}, false)
	if preview := b.Settled.TacticsRuntime.Plans["hero"].Preview; preview == nil || preview.Final.X != 7 {
		t.Fatalf("Know the Land (swamp) preview = %+v", preview)
	}
	// Energy: 5 energy = 1 Move; steps of 5 only.
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(11, 0), energy: 7}); err == nil {
		t.Fatal("energy outside 5-steps was accepted")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(11, 0), energy: 25}); err == nil {
		t.Fatal("more energy than the hero has was accepted")
	}
	// Committed cards are spent even if unused: a one-square move with two
	// cards and 10 energy still discards both and spends the energy.
	energy := b.Actors["hero"].Resources.EnergyPoints
	plan(t, e, &b, "hero", movePlan{intent: toSquare(1, 1), cards: []string{nudge, again}, energy: 10}, true)
	plan(t, e, &b, "foe", holdPlan(), true)
	hero := b.Actors["hero"]
	if containsString(hero.Cards.Hand, nudge) || !containsString(hero.Cards.Discard, nudge) || !containsString(hero.Cards.Discard, again) {
		t.Fatalf("committed cards must go to discard: hand %v discard %v", hero.Cards.Hand, hero.Cards.Discard)
	}
	if hero.Resources.EnergyPoints != energy-10 {
		t.Fatalf("energy %d, want %d", hero.Resources.EnergyPoints, energy-10)
	}
	if got := b.Settled.TacticsRuntime.Positions["hero"]; got != (tactics.Square{X: 1, Y: 1}) {
		t.Fatalf("hero at %v", got)
	}
	_ = lib
}

func TestHoldCommitsNothingAndChargersCannotAddCards(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4, hand: []string{"nudge"}},
		{id: "foe", definition: "tactics_bell_diver", team: "monsters", x: 5, y: 4},
	}, nil)
	lockOffense(t, e, &b, "foe")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 6)
	attack(t, e, &b, "hero", "magic_missile", "foe", nil, "")
	nudge := handCard(t, &b, "hero", "nudge")
	if err := planErr(&b, e, "hero", movePlan{intent: tactics.Intention{Kind: tactics.IntentHold}, cards: []string{nudge}}); err == nil {
		t.Fatal("Hold committed a card")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: tactics.Intention{Kind: tactics.IntentPursue, Target: "hero"}}); err == nil {
		t.Fatal("pursuing yourself was accepted")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(1, 4)}); err == nil {
		t.Fatal("moving to your own square was accepted")
	}

	b, lib, e = tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4, hand: []string{"nudge"}},
		{id: "foe", definition: "tactics_bell_diver", team: "monsters", x: 5, y: 4},
	}, func(r *tactics.Rules) { r.ChargeAllowsExtraMove = true })
	lockOffense(t, e, &b, "foe")
	forceDice(t, &b, lib, "hero", 1, 1, 1, 4, 4)
	attack(t, e, &b, "hero", "charge", "foe", nil, "")
	if _, ok := b.Flow.PendingInput["hero"]; !ok {
		t.Fatal("charge_allows_extra_move: the charger may add cards")
	}
	if err := planErr(&b, e, "hero", movePlan{intent: toSquare(3, 3)}); err == nil {
		t.Fatal("a charger changed its intention")
	}
	nudge = handCard(t, &b, "hero", "nudge")
	plan(t, e, &b, "hero", movePlan{intent: tactics.Intention{Kind: tactics.IntentPursue, Target: "foe"}, cards: []string{nudge}}, false)
	if got := b.Settled.TacticsRuntime.Plans["hero"].Budget.Total; got != 5 {
		t.Fatalf("charge 4 + 1 card = %d", got)
	}
}

// --- Running (§11.11) -----------------------------------------------------

func TestRunningAllowsOnlyOnTheMoveDefensesAndExpires(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4, hand: []string{"nudge"}},
		{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 2, y: 4, hand: []string{"shed_ribbon", "shed_ribbon"}},
	}, nil)
	lockOffense(t, e, &b)
	forceDice(t, &b, lib, "hero", 1, 1, 1, 4, 4)
	forceDice(t, &b, lib, "eel", 2, 4, 6, 1, 1)
	attack(t, e, &b, "hero", "adventurer_strike", "eel", nil, "")
	attack(t, e, &b, "eel", "coiling_snare", "hero", nil, "")
	nudge := handCard(t, &b, "hero", "nudge")
	plan(t, e, &b, "hero", movePlan{intent: toSquare(1, 7), cards: []string{nudge}}, true)
	ribbon := b.Actors["eel"].Cards.Hand[0]
	plan(t, e, &b, "eel", movePlan{intent: toSquare(5, 4), cards: []string{ribbon}}, true)
	if statusStackCount(b.Actors["hero"].Statuses, "running") != 1 || statusStackCount(b.Actors["eel"].Statuses, "running") != 1 {
		t.Fatal("both ran: both must show Running")
	}
	defenses := func(actor string) []string {
		var out []string
		for _, a := range e.LegalActions(&b, actor) {
			var p command.PlanningAbilityPayload
			if a.Type == command.TypePlanningAbility && json.Unmarshal(a.Payload, &p) == nil && !containsString(out, p.AbilityID) {
				out = append(out, p.AbilityID)
			}
		}
		return out
	}
	if got := defenses("hero"); fmt.Sprint(got) != "[evade]" {
		t.Fatalf("a running hero may use only Evade, got %v", got)
	}
	if got := defenses("eel"); fmt.Sprint(got) != "[slip_the_current]" {
		t.Fatalf("a running Ribbon Eel may use Slip the Current, got %v", got)
	}
	source := ""
	for _, s := range b.Settled.OffensiveSources {
		if s.TargetActorID == "hero" {
			source = s.ID
		}
	}
	pending := pendingFor(t, &b, "hero")
	_, err := e.ApplyBattleCommand(&b, legalCommand(b.ID, "hero", command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: "adventurer_guard", TargetIDs: []string{source}}))
	if err == nil || !strings.Contains(err.Error(), "on-the-move") {
		t.Fatalf("Guard while running was accepted: %v", err)
	}
	view := e.ResultForViewer(&b, "hero", ProgressionResult{})
	if view.Snapshot.Tactics == nil || view.Snapshot.Tactics.DisabledDefenses["adventurer_guard"] == "" || view.Snapshot.Tactics.DisabledDefenses["evade"] != "" {
		t.Fatalf("the view must show Guard disabled with a reason: %+v", view.Snapshot.Tactics)
	}
	// Evade is allowed; then everyone passes and the round ends.
	apply(t, e, &b, legalCommand(b.ID, "hero", command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: "evade", TargetIDs: []string{source}}))
	finishRound(t, e, &b)
	if b.Segment.Round != 2 || b.Segment.Current != segment.Offensive {
		t.Fatalf("at %s round %d, want the next Offense", b.Segment.Current, b.Segment.Round)
	}
	if statusStackCount(b.Actors["hero"].Statuses, "running") != 0 || statusStackCount(b.Actors["eel"].Statuses, "running") != 0 {
		t.Fatal("Running must expire at the end of the round")
	}
	if got := b.Settled.TacticsRuntime.RoundStart["hero"]; got != (tactics.Square{X: 1, Y: 6}) {
		t.Fatalf("the next round starts where movement ended: %v", got)
	}
	// The round's record keeps each hit's committed result for the playback.
	hits := map[string]state.TacticsHitResult{}
	for _, hit := range b.Settled.TacticsRuntime.Movement.Hits {
		hits[hit.Target] = hit
	}
	if hits["hero"].Defense != "evade" || hits["hero"].Base != 3 || hits["eel"].Base != 4 || hits["eel"].Final != 4 || len(b.Settled.TacticsRuntime.Movement.Attacks) != 2 {
		t.Fatalf("hits = %+v", b.Settled.TacticsRuntime.Movement.Hits)
	}
}

// finishRound passes every remaining choice until the next Offense.
func finishRound(t *testing.T, e Engine, b *state.Battle) {
	t.Helper()
	for guard := 0; guard < 50 && b.Segment.Current != segment.Offensive && !state.IsTerminalBattleStatus(b.Status); guard++ {
		acted := false
		for _, id := range sortedSettledActorIDs(b) {
			for _, a := range e.LegalActions(b, id) {
				var p command.PlanningPassPayload
				if a.Type == command.TypePlanningPass && json.Unmarshal(a.Payload, &p) == nil && p.SourceID == "" {
					apply(t, e, b, a)
					acted = true
					break
				}
			}
		}
		if !acted {
			t.Fatalf("stuck at %s/%s", b.Settled.Stage, simultaneousStep(b))
		}
	}
}

// --- Viewer safety (§11.12) -----------------------------------------------

func TestMovementPlansStaySecretUntilTheReveal(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4, hand: []string{"nudge", "try_again"}},
		{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 6, y: 4, hand: []string{"shed_ribbon"}},
	}, nil)
	lockOffense(t, e, &b, "eel")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 6)
	attack(t, e, &b, "hero", "magic_missile", "eel", nil, "")
	nudge := handCard(t, &b, "hero", "nudge")
	events := plan(t, e, &b, "hero", movePlan{intent: toSquare(9, 8), cards: []string{nudge}, energy: 5}, false)
	events = append(events, plan(t, e, &b, "hero", movePlan{intent: toSquare(9, 8), cards: []string{nudge}, energy: 5}, true)...)
	for _, ev := range events {
		if ev.Type == event.TypeTacticsMovementPlanned && ev.SecretActorID != "hero" {
			t.Fatalf("a plan event is not secret: %+v", ev)
		}
	}
	other := e.ResultForViewer(&b, "eel", ProgressionResult{Events: events})
	// The eel's own legal actions offer every square; its snapshot and
	// events must hold nothing of the hero's plan.
	raw, _ := json.Marshal([]any{other.Snapshot, other.Events})
	for _, leak := range []string{`"x":9,"y":8`, nudge, `"preview"`, `"move_to"`} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("the eel's view leaks %s", leak)
		}
	}
	if other.Snapshot.Tactics.Plan == nil || other.Snapshot.Tactics.Plan.Intention.Kind != tactics.IntentHold {
		t.Fatalf("the eel sees its own plan only: %+v", other.Snapshot.Tactics.Plan)
	}
	if fmt.Sprint(other.Snapshot.Tactics.LockedIn) != "[hero]" {
		t.Fatalf("others see only the lock-in marker: %v", other.Snapshot.Tactics.LockedIn)
	}
	for _, a := range other.LegalActions {
		if a.ActorID != "eel" {
			t.Fatalf("the eel's legal actions include %s's", a.ActorID)
		}
	}
	own := e.ResultForViewer(&b, "hero", ProgressionResult{})
	if own.Snapshot.Tactics.Plan == nil || own.Snapshot.Tactics.Plan.Preview == nil || own.Snapshot.Tactics.Plan.Budget.Total != 3 {
		t.Fatalf("the hero sees its own plan and preview: %+v", own.Snapshot.Tactics.Plan)
	}
	// After the reveal everyone sees the movement, but not the hero's card.
	reveal := plan(t, e, &b, "eel", holdPlan(), true)
	after := e.ResultForViewer(&b, "eel", ProgressionResult{Events: reveal})
	raw, _ = json.Marshal(after)
	if !strings.Contains(string(raw), `"tactics_movement_resolved"`) || after.Snapshot.Tactics.Movement == nil {
		t.Fatal("the reveal must be public")
	}
	if public := after.Snapshot.Tactics.Movement.Plans["hero"]; public.CardCount != 1 || len(public.Cards) != 0 || public.Energy != 5 {
		t.Fatalf("the public plan shows a card count, not the card: %+v", public)
	}
}

// --- Save and reload (§11.13) and determinism (§11.14) ---------------------

func reload(t *testing.T, b state.Battle) state.Battle {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var out state.Battle
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSaveAndReloadKeepPlansTimelineAndPositions(t *testing.T) {
	b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4, hand: []string{"nudge"}},
		{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 6, y: 4, hand: []string{"shed_ribbon"}},
		{id: "mask", definition: "tactics_brine_mask", team: "monsters", x: 8, y: 2},
	}, nil)
	lockOffense(t, e, &b, "eel", "mask")
	forceDice(t, &b, lib, "hero", 1, 2, 3, 4, 6)
	attack(t, e, &b, "hero", "magic_missile", "eel", nil, "")
	nudge := handCard(t, &b, "hero", "nudge")
	plan(t, e, &b, "hero", movePlan{intent: toSquare(4, 6), cards: []string{nudge}}, true)
	// Reload during the movement step: the locked plan survives.
	saved := reload(t, b)
	if !reflect.DeepEqual(saved.Settled.TacticsRuntime.Plans, b.Settled.TacticsRuntime.Plans) || !reflect.DeepEqual(saved.Settled.TacticsRuntime.Positions, b.Settled.TacticsRuntime.Positions) {
		t.Fatal("plans or positions changed on reload")
	}
	finish := func(b *state.Battle) {
		plan(t, e, b, "eel", movePlan{intent: tactics.Intention{Kind: tactics.IntentPursue, Target: "hero"}, cards: b.Actors["eel"].Cards.Hand}, true)
		plan(t, e, b, "mask", holdPlan(), true)
	}
	finish(&b)
	finish(&saved)
	if !reflect.DeepEqual(b.Settled.TacticsRuntime.Movement, saved.Settled.TacticsRuntime.Movement) {
		t.Fatal("the reloaded battle resolved a different movement")
	}
	// Reload during defense: timeline and positions are identical.
	again := reload(t, b)
	before, _ := json.Marshal(b.Settled.TacticsRuntime)
	after, _ := json.Marshal(again.Settled.TacticsRuntime)
	if string(before) != string(after) || simultaneousStep(&again) != state.SimultaneousDefenseBefore {
		t.Fatal("the tactics state changed on reload during defense")
	}
	finishRound(t, e, &b)
	finishRound(t, e, &again)
	if !reflect.DeepEqual(reload(t, b), reload(t, again)) {
		t.Fatal("the reloaded battle diverged")
	}
}

func TestSameSeedAndCommandsGiveTheSameRound(t *testing.T) {
	run := func() state.Battle {
		b, lib, e := tacticsFixture(t, openRows(12, 9), []seat{
			{id: "hero", definition: "adventurer_movement", team: "heroes", x: 3, y: 4, hand: []string{"nudge"}},
			{id: "eel", definition: "tactics_ribbon_eel", team: "monsters", x: 4, y: 4, hand: []string{"shed_ribbon"}},
			{id: "diver", definition: "tactics_bell_diver", team: "monsters", x: 4, y: 3},
		}, nil)
		lockOffense(t, e, &b, "hero")
		forceDice(t, &b, lib, "eel", 2, 4, 6, 1, 1)
		forceDice(t, &b, lib, "diver", 5, 6, 5, 1, 1)
		attack(t, e, &b, "eel", "coiling_snare", "hero", nil, "")
		attack(t, e, &b, "diver", "sunken_toll", "hero", nil, "")
		nudge := handCard(t, &b, "hero", "nudge")
		plan(t, e, &b, "hero", movePlan{intent: toSquare(4, 5), cards: []string{nudge}}, true)
		plan(t, e, &b, "eel", movePlan{intent: toSquare(5, 5)}, true)
		plan(t, e, &b, "diver", movePlan{intent: tactics.Intention{Kind: tactics.IntentPursue, Target: "hero"}}, true)
		finishRound(t, e, &b)
		return b
	}
	first, second := run(), run()
	if !reflect.DeepEqual(reload(t, first), reload(t, second)) {
		t.Fatal("same seed and commands gave a different battle")
	}
	if first.Settled.TacticsRuntime.Movement == nil || len(first.Settled.TacticsRuntime.Movement.Result.Timeline) == 0 {
		t.Fatal("no movement was resolved")
	}
}

// --- The ruleset needs the other two (§11.15) ---------------------------------

func TestTacticsNeedsUnifiedDefenseAndSimultaneousPlay(t *testing.T) {
	b, _, _ := tacticsFixture(t, openRows(12, 9), []seat{
		{id: "hero", definition: "adventurer_movement", team: "heroes", x: 1, y: 4},
		{id: "foe", definition: "tactics_bell_diver", team: "monsters", x: 5, y: 4},
	}, nil)
	setup := command.TacticsSetup{Arena: b.Settled.TacticsRuntime.Arena, Rules: b.Settled.TacticsRuntime.Rules, Placement: b.Settled.TacticsRuntime.Positions}
	for _, flags := range [][2]bool{{false, true}, {true, false}, {false, false}} {
		c := b.Clone()
		c.Settled.Tactics, c.Settled.TacticsRuntime = false, nil
		c.Settled.UnifiedDefense, c.Settled.SimultaneousPlay = flags[0], flags[1]
		if err := InitTactics(&c, setup); err == nil {
			t.Fatalf("tactics accepted with unified=%v simultaneous=%v", flags[0], flags[1])
		}
	}
}
