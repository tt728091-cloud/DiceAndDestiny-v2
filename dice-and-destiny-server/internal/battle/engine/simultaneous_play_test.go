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
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// simCard authors a remove-status program card for these tests.
func simCard(t *testing.T, lib *content.BattleLibrary, id, owner, polarity string, windows ...string) {
	t.Helper()
	c, err := content.EditableGeneralCard(lib.Cards["dispel"])
	if err != nil {
		t.Fatal(err)
	}
	c.ID, c.Name = id, id
	c.Program.Windows = windows
	c.Program.Steps[0].Target.Owner = owner
	c.Program.Steps[0].Target.Polarity = polarity
	c = content.PrepareProgramCard(c, lib)
	if err = content.ValidateCardProgram(c, *lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[id] = c
}

func simLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	lib := settledTestLibrary(t)
	var err error
	for _, pack := range []string{"general_v1", "adventurer_v1", "minions_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", pack)); err != nil {
			t.Fatal(err)
		}
	}
	// Any time / after-defense versions of the status cards.
	simCard(t, &lib, "sim_dispel_after", "enemy", "positive", "defense_after_roll", "damage_reaction")
	simCard(t, &lib, "sim_salve", "self", "negative", "offensive_planning", "defense_selection", "damage_reaction")
	simCard(t, &lib, "sim_salve_after", "self", "negative", "defense_after_roll", "damage_reaction")
	draw, err := content.EditableGeneralCard(lib.Cards["take_stock"])
	if err != nil {
		t.Fatal(err)
	}
	draw.ID, draw.Name = "sim_draw", "sim_draw"
	draw.Program.Windows = []string{"offensive_planning", "defense_selection", "damage_reaction"}
	draw = content.PrepareProgramCard(draw, &lib)
	lib.Cards["sim_draw"] = draw
	// Set one enemy attack die to 1 (a Tamper), only after attacks are revealed.
	tamper, err := content.EditableGeneralCard(lib.Cards["disrupt"])
	if err != nil {
		t.Fatal(err)
	}
	tamper.ID, tamper.Name = "sim_tamper", "sim_tamper"
	tamper.Program.Steps[0].Effect = "set_die"
	tamper.Program.Steps[0].Params = map[string]any{"faces": []int{1}}
	tamper = content.PrepareProgramCard(tamper, &lib)
	if err = content.ValidateCardProgram(tamper, lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards["sim_tamper"] = tamper
	// A Before-defense-only prevention card (the published Steady Guard's timing).
	brace, err := content.EditableGeneralCard(lib.Cards["steady_guard"])
	if err != nil {
		t.Fatal(err)
	}
	brace.ID, brace.Name = "sim_brace", "sim_brace"
	brace.Program.Windows = []string{"defense_before_roll"}
	brace = content.PrepareProgramCard(brace, &lib)
	lib.Cards["sim_brace"] = brace
	// A Guard without a per-segment limit, so one defense can cover every attack.
	guard := lib.Abilities["adventurer_guard"]
	guard.ID, guard.Name = "sim_guard", "sim_guard"
	guard.Usage.MaximumPerSegment = 0
	lib.Abilities["sim_guard"] = guard
	return lib
}

var simHand = []string{"dispel", "dispel", "sim_dispel_after", "sim_salve", "sim_salve_after", "take_stock", "nudge", "steady_guard", "disrupt", "sim_draw", "sim_tamper", "second_guard", "sim_brace"}

// simultaneousFixture is two external participants, alice (seat 1) and bob
// (seat 2), both Adventurers with every test card in hand, at Offensive entry
// under the pinned simultaneous rule.
func simultaneousFixture(t *testing.T, alice, bob []state.StatusState) (state.Battle, content.BattleLibrary, Engine) {
	t.Helper()
	lib := simLibrary(t)
	catalog, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string][]state.StatusState{"alice": alice, "bob": bob}
	controllers := map[string]state.ControllerType{"alice": state.ControllerHuman, "bob": state.ControllerExternal}
	var actors []state.ActorSetup
	runtimes := map[string]state.SettledActorRuntime{}
	for _, id := range []string{"alice", "bob"} {
		actors = append(actors, state.ActorSetup{ID: id, DefinitionID: "adventurer", DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "standard_d6", Count: 5}}, ControllerType: controllers[id], Statuses: statuses[id], Resources: state.ResourceState{EnergyPoints: 40, MaxEnergyPoints: 75}})
		runtimes[id] = state.SettledActorRuntime{MaxRolls: 3, HandLimit: 99}
	}
	b, err := state.NewBattleFromSetup("simultaneous-test", state.BattleSetup{Actors: actors, SettledCatalog: catalog, SettledActors: runtimes})
	if err != nil {
		t.Fatal(err)
	}
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 11}
	b.Settled.UnifiedDefense = true
	b.Settled.SimultaneousPlay = true
	b.Settled.Initialized = true
	for _, id := range []string{"alice", "bob"} {
		r := b.Settled.Actors[id]
		r.OffensiveAbilityIDs = lib.Combatants["adventurer"].AbilityBoard.Offensive
		r.DefensiveAbilityIDs = append(append([]string(nil), lib.Combatants["adventurer"].AbilityBoard.Defensive...), "sim_guard")
		r.UsedAbilities = map[string]int{}
		r.CardInstances = map[string]state.CardInstance{}
		a := b.Actors[id]
		a.Cards = state.CardZones{}
		for i, card := range simHand {
			instance := fmt.Sprintf("%s-%s-%d", id, card, i)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: card}
			a.Cards.Hand = append(a.Cards.Hand, instance)
		}
		for i := 0; i < 8; i++ {
			instance := fmt.Sprintf("%s-deck-%d", id, i)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: "steady_guard"}
			a.Cards.Deck = append(a.Cards.Deck, instance)
		}
		b.Actors[id] = a
		b.Settled.Actors[id] = r
	}
	recordRestoredStatuses(&b, lib)
	b.Segment.Current = segment.Offensive
	b.Flow = state.NewSegmentFlowState(b.Segment)
	b.Flow.Entered = true
	e := NewEngine()
	if _, err := e.progressSettled(&b); err != nil {
		t.Fatal(err)
	}
	return b, lib, e
}

func simChoice(action command.Command) (string, programChoice) {
	id, key := programPayload(action)
	var c programChoice
	_ = json.Unmarshal([]byte(key), &c)
	return id, c
}

// simAct applies the first legal action of the actor that matches.
func simAct(t *testing.T, e Engine, b *state.Battle, actor string, match func(command.Command) bool) []event.Event {
	t.Helper()
	for _, a := range e.LegalActions(b, actor) {
		if match(a) {
			progressed, err := e.ApplyBattleCommand(b, a)
			if err != nil {
				t.Fatal(err)
			}
			return progressed.Events
		}
	}
	t.Fatalf("%s has no matching action at %s/%s", actor, b.Settled.Stage, simultaneousStep(b))
	return nil
}

func simHas(e Engine, b *state.Battle, actor string, match func(command.Command) bool) bool {
	for _, a := range e.LegalActions(b, actor) {
		if match(a) {
			return true
		}
	}
	return false
}

func simType(kind command.Type) func(command.Command) bool {
	return func(a command.Command) bool { return a.Type == kind }
}

// simCardOn starts a card on a status (or any first target) and finishes it.
func simCardOn(t *testing.T, e Engine, b *state.Battle, actor, card, targetActor, status string) []event.Event {
	t.Helper()
	instance := ""
	for id, inst := range b.Settled.Actors[actor].CardInstances {
		if inst.DefinitionID == card && containsString(b.Actors[actor].Cards.Hand, id) && !secretPending(b, actor, id) && (instance == "" || id < instance) {
			instance = id
		}
	}
	if instance == "" {
		t.Fatalf("%s holds no %s", actor, card)
	}
	var events []event.Event
	targets := func(c programChoice) bool {
		return (targetActor == "" || c.Actor == targetActor) && (status == "" || c.Status == status)
	}
	switch {
	case simHas(e, b, actor, func(a command.Command) bool {
		id, c := simChoice(a)
		return id == instance && c.Verb == "start" && c.Then == "target" && targets(c)
	}):
		events = simAct(t, e, b, actor, func(a command.Command) bool {
			id, c := simChoice(a)
			return id == instance && c.Verb == "start" && c.Then == "target" && targets(c)
		})
	default:
		events = simAct(t, e, b, actor, func(a command.Command) bool {
			id, c := simChoice(a)
			return id == instance && c.Verb == "start" && c.Then == ""
		})
	}
	for b.Settled.Actors[actor].CardExecution != nil {
		events = append(events, simAct(t, e, b, actor, func(a command.Command) bool {
			id, c := simChoice(a)
			return id == instance && (c.Verb == "target" && targets(c) || c.Verb == "confirm")
		})...)
	}
	return events
}

func simStatus(b *state.Battle, actor, status string) int {
	return statusStackCount(b.Actors[actor].Statuses, status)
}

func simView(t *testing.T, e Engine, b *state.Battle, viewer string, events []event.Event) string {
	t.Helper()
	result := e.ResultForViewer(b, viewer, ProgressionResult{Events: events})
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func fortified(id string) []state.StatusState {
	return []state.StatusState{{InstanceID: id, DefinitionID: "fortified", Stacks: 1}}
}

// --- Offense -----------------------------------------------------------------

func TestSimultaneousOffenseBeforePlaysStayHiddenUntilReveal(t *testing.T) {
	b, _, e := simultaneousFixture(t, fortified("af"), fortified("bf"))
	if simultaneousStep(&b) != state.SimultaneousOffenseBefore || len(b.Flow.PendingInput) != 2 {
		t.Fatalf("Offense Before opens for both participants at once: %s %v", simultaneousStep(&b), b.Flow.PendingInput)
	}
	aliceStart := b.Actors["alice"]
	var aliceEvents []event.Event
	aliceEvents = append(aliceEvents, simCardOn(t, e, &b, "alice", "dispel", "bob", "fortified")...)
	aliceEvents = append(aliceEvents, simCardOn(t, e, &b, "alice", "take_stock", "", "")...)
	bobEvents := simCardOn(t, e, &b, "bob", "dispel", "alice", "fortified")

	// The queued Dispels change nothing yet; alice's draw happened privately.
	if simStatus(&b, "alice", "fortified") != 1 || simStatus(&b, "bob", "fortified") != 1 {
		t.Fatal("queued Dispels resolved before the reveal")
	}
	if len(b.Actors["alice"].Cards.Hand) != len(aliceStart.Cards.Hand)+1 || b.Actors["alice"].Resources.EnergyPoints != aliceStart.Resources.EnergyPoints-10 {
		t.Fatalf("alice's draw and energy apply at once for her: hand %d energy %d", len(b.Actors["alice"].Cards.Hand), b.Actors["alice"].Resources.EnergyPoints)
	}
	// Bob sees none of it: not the cards, the energy, the hand size or events.
	bobView := e.battleSnapshotForViewer(&b, "bob")
	seen := bobView.Actors["alice"]
	if seen.EnergyPoints != aliceStart.Resources.EnergyPoints || seen.HandCount != len(aliceStart.Cards.Hand) || seen.DeckCount != len(aliceStart.Cards.Deck) || seen.DiscardCount != 0 {
		t.Fatalf("bob sees alice's secret changes: %+v", seen)
	}
	if got := event.ForViewer(aliceEvents, "bob"); len(got) != 0 {
		t.Fatalf("bob received alice's secret events: %+v", got)
	}
	if got := event.ForViewer(bobEvents, "alice"); len(got) != 0 {
		t.Fatalf("alice received bob's secret events: %+v", got)
	}
	if len(bobView.SecretPlays) != 1 || bobView.SecretPlays[0].ActorID != "bob" {
		t.Fatalf("bob sees only his own pending play: %+v", bobView.SecretPlays)
	}
	// Alice locks in by rolling: no dice yet, and bob only learns she locked in.
	locked := simAct(t, e, &b, "alice", simType(command.TypePlanningRoll))
	if b.Settled.Actors["alice"].RollsUsed != 0 || len(b.Settled.Actors["alice"].FinalDice) != 0 {
		t.Fatal("alice's dice rolled before the reveal")
	}
	if got := event.ForViewer(locked, "bob"); len(got) != 1 || got[0].Type != event.TypeSimultaneousLockedIn {
		t.Fatalf("bob sees only alice's lock-in: %+v", got)
	}
	if view := e.battleSnapshotForViewer(&b, "bob"); !reflect.DeepEqual(view.LockedIn, []string{"alice"}) || view.Actors["alice"].EnergyPoints != aliceStart.Resources.EnergyPoints {
		t.Fatalf("bob's view after alice locks in: %v %+v", view.LockedIn, view.Actors["alice"])
	}
	if simHas(e, &b, "alice", func(command.Command) bool { return true }) {
		t.Fatal("a locked-in participant can't play Before cards")
	}
	// Bob locks in: Reveal 1 resolves both Dispels, in seat order, then rolls.
	reveal := simAct(t, e, &b, "bob", simType(command.TypePlanningRoll))
	if simStatus(&b, "alice", "fortified") != 0 || simStatus(&b, "bob", "fortified") != 0 {
		t.Fatal("both Dispels resolve at the reveal")
	}
	var revealed []state.RevealedPlay
	for _, ev := range event.ForViewer(reveal, "bob") {
		if ev.Type == event.TypeSecretPlaysRevealed {
			raw, _ := json.Marshal(ev.Data["plays"])
			_ = json.Unmarshal(raw, &revealed)
		}
	}
	if len(revealed) != 3 || revealed[0].ActorID != "alice" || revealed[1].ActorID != "alice" || revealed[2].ActorID != "bob" || revealed[0].Outcome != "resolved" || revealed[2].Outcome != "resolved" {
		t.Fatalf("one reveal lists every play in seat, then play order: %+v", revealed)
	}
	if b.Settled.Actors["alice"].RollsUsed != 1 || b.Settled.Actors["bob"].RollsUsed != 1 || simultaneousStep(&b) != state.SimultaneousOffenseRolling {
		t.Fatal("everyone's first roll happens after the reveal")
	}
	if view := e.battleSnapshotForViewer(&b, "bob"); view.Actors["alice"].EnergyPoints != b.Actors["alice"].Resources.EnergyPoints || len(view.RevealedPlays) != 3 {
		t.Fatal("after the reveal every play and its energy are public")
	}
}

func TestSimultaneousRollingIsSelfOnlyAndTheNextStepReachesEnemies(t *testing.T) {
	conqueror := []state.StatusState{{InstanceID: "bc", DefinitionID: "conqueror", Stacks: 1}}
	b, lib, e := simultaneousFixture(t, nil, conqueror)
	simAct(t, e, &b, "alice", simType(command.TypePlanningRoll))
	simAct(t, e, &b, "bob", simType(command.TypePlanningRoll))
	// After the first roll, no card can reach bob: not his statuses or dice.
	for _, a := range e.LegalActions(&b, "alice") {
		if _, c := simChoice(a); c.Actor != "" && c.Actor != "alice" {
			t.Fatalf("rolling offers a play on bob: %+v", c)
		}
	}
	// Bob's rolling play (Nudge on his own die) resolves at once, hidden from alice.
	events := simCardOn(t, e, &b, "bob", "nudge", "bob", "")
	if got := event.ForViewer(events, "alice"); len(got) != 0 {
		t.Fatalf("alice saw bob's rolling play: %+v", got)
	}
	if view := e.battleSnapshotForViewer(&b, "alice"); view.Actors["bob"].EnergyPoints != b.Settled.Simultaneous.Public["bob"].EnergyPoints || view.Actors["bob"].Dice != nil {
		t.Fatal("alice sees bob's energy and dice as they were at the reveal")
	}
	for _, id := range []string{"alice", "bob"} {
		simAct(t, e, &b, id, simType(command.TypePlanningPass))
	}
	// Reveal 2 sets the attacks; Offense After is the Defense Before step,
	// with no stop in between, and it reaches bob in secret.
	if b.Segment.Current != segment.Defensive || simultaneousStep(&b) != state.SimultaneousDefenseBefore || len(b.Flow.PendingInput) != 2 {
		t.Fatalf("Defense Before follows the ability reveal for both at once: %s %s %v", b.Segment.Current, simultaneousStep(&b), b.Flow.PendingInput)
	}
	if !simStartable(e, &b, "alice", "dispel") || len(programCandidates(&b, lib, "alice", "", lib.Cards["dispel"].Program.Steps[0], nil)) != 1 {
		t.Fatal("an any-time Dispel reaches bob between the attacks and the defenses")
	}
	if !simStartable(e, &b, "alice", "steady_guard") && remainingDamage(&b) > 0 {
		t.Fatal("Before-defense cards share the step")
	}
	simCardOn(t, e, &b, "alice", "dispel", "bob", "conqueror")
	if simStatus(&b, "bob", "conqueror") != 1 || !secretPending(&b, "alice", simPending(&b, "alice")) {
		t.Fatal("plays between the attacks and the defenses wait for the reveal")
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	if simStatus(&b, "bob", "conqueror") != 0 {
		t.Fatal("the Dispel resolves at the reveal")
	}
}

// simPending is an actor's first queued card.
func simPending(b *state.Battle, actor string) string {
	for _, play := range b.Settled.Simultaneous.Queue {
		if play.ActorID == actor {
			return play.CardID
		}
	}
	return ""
}

// The AI fixture: a human against a planned Brine Mask (Lieutenant's Favor).
func simAIFixture(t *testing.T, player, enemy []state.StatusState) (state.Battle, content.BattleLibrary, Engine) {
	t.Helper()
	b, lib := expeditionStatusFixture(t, player, enemy)
	lib = simLibrary(t)
	// A planned enemy: Brine Lash on three Brine faces (6 damage).
	mask := lib.Combatants["drowned_oracle_brine_mask"]
	mask.AI = &content.CombatantAI{
		RevealProfiles:    map[string][]int{"brine_lash": {3, 3, 3, 1, 1}},
		OffensivePlanning: content.OffensivePlanningAI{Charts: map[string]content.D100Chart{"3_rolls": {Abilities: []content.D100AbilityEntry{{AbilityID: "brine_lash", ActivationRanges: content.AbilityActivationRanges{FirstRoll: &content.D100Range{Start: 1, End: 100}}}}}}},
	}
	lib.Combatants["drowned_oracle_brine_mask"] = mask
	catalog, _ := json.Marshal(lib)
	b.SettledCatalog = catalog
	b.SettledCatalogHash = [32]byte{}
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 3}
	b.Settled.UnifiedDefense, b.Settled.SimultaneousPlay, b.Settled.Initialized = true, true, true
	for _, id := range []string{"player", "enemy"} {
		r := b.Settled.Actors[id]
		r.HandLimit = 99
		a := b.Actors[id]
		a.Resources.EnergyPoints = 40
		for i, card := range simHand {
			instance := fmt.Sprintf("%s-%s-%d", id, card, i)
			r.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: card}
			if id == "player" {
				a.Cards.Hand = append(a.Cards.Hand, instance)
			}
		}
		for _, card := range a.Cards.Deck {
			r.CardInstances[card] = state.CardInstance{InstanceID: card, DefinitionID: "steady_guard"}
		}
		b.Actors[id] = a
		b.Settled.Actors[id] = r
	}
	closeSettledWindow(&b)
	b.Settled.Stage = ""
	e := NewEngine()
	if _, err := e.progressSettled(&b); err != nil {
		t.Fatal(err)
	}
	return b, lib, e
}

func enemyAttack(b *state.Battle) int {
	for _, s := range b.Settled.OffensiveSources {
		if s.SourceActorID == "enemy" {
			return s.BaseAmount
		}
	}
	return -1
}

func TestSimultaneousLieutenantsFavorCountsUntilTheDefenseReveal(t *testing.T) {
	favor := []state.StatusState{{InstanceID: "lf", DefinitionID: "lieutenants_favor", Stacks: 2}}
	run := func(dispel string) (int, int) {
		b, _, e := simAIFixture(t, nil, favor)
		if b.Settled.Actors["enemy"].SelectedAbilityID == "" {
			t.Fatal("the seed must give the Brine Mask an attack")
		}
		if dispel == "before" {
			simCardOn(t, e, &b, "player", "dispel", "enemy", "lieutenants_favor")
		}
		simAct(t, e, &b, "player", simType(command.TypePlanningPass))
		if b.Segment.Current != segment.Defensive || simultaneousStep(&b) != state.SimultaneousDefenseBefore {
			t.Fatalf("Defense Before follows: %s %s", b.Segment.Current, simultaneousStep(&b))
		}
		attack := enemyAttack(&b)
		if dispel == "between" {
			simCardOn(t, e, &b, "player", "dispel", "enemy", "lieutenants_favor")
		}
		simAct(t, e, &b, "player", simType(command.TypePlanningPass))
		if dispel == "after" {
			simCardOn(t, e, &b, "player", "sim_dispel_after", "enemy", "lieutenants_favor")
		}
		return attack, enemyAttack(&b)
	}
	plain, _ := run("")
	before, _ := run("before")
	between, rescored := run("between")
	final, after := run("after")
	if before != plain-1 {
		t.Fatalf("a Before Dispel removes one stack of Lieutenant's Favor and lowers the attack: %d vs %d", before, plain)
	}
	if between != plain || rescored != plain-1 {
		t.Fatalf("a Dispel between the attacks and the defenses lowers the attack at the reveal: %d → %d (plain %d)", between, rescored, plain)
	}
	if final != plain || after != plain {
		t.Fatalf("an After-defense Dispel leaves the final attack: %d → %d (plain %d)", final, after, plain)
	}
}

// TestSimultaneousTheNextStepReScoresChangedAttacks changes an enemy's attack
// die in secret: the attack keeps its score until the reveal, then drops a
// tier before any defense rolls, and its released cards stay where they are.
func TestSimultaneousTheNextStepReScoresChangedAttacks(t *testing.T) {
	b, lib, e := simAIFixture(t, nil, nil)
	simAct(t, e, &b, "player", simType(command.TypePlanningPass))
	if enemyAttack(&b) != 6 || b.Settled.Actors["enemy"].SelectedTierID != "brine_3" {
		t.Fatalf("Brine Lash on three Brine faces deals 6: %d %s", enemyAttack(&b), b.Settled.Actors["enemy"].SelectedTierID)
	}
	if simStartable(e, &b, "player", "nudge") {
		t.Fatal("the player skipped their attack, so there are no dice of theirs to change")
	}
	hand := append([]string(nil), b.Actors["player"].Cards.Hand...)
	simCardOn(t, e, &b, "player", "sim_tamper", "enemy", "")
	if enemyAttack(&b) != 6 || remainingDamage(&b) != 6 || b.Settled.Actors["enemy"].FinalDice[0].Face != 3 {
		t.Fatal("the die change waits for the reveal")
	}
	if view := e.battleSnapshotForViewer(&b, "enemy"); len(view.SecretPlays) != 0 {
		t.Fatal("the enemy can't see the queued play")
	}
	events := simAct(t, e, &b, "player", simType(command.TypePlanningPass))
	if got := b.Settled.Actors["enemy"].FinalDice[0].Face; got != 1 {
		t.Fatalf("the revealed Tamper set the die: %d", got)
	}
	if enemyAttack(&b) != 4 || b.Settled.Actors["enemy"].SelectedTierID != "brine_2" {
		t.Fatalf("the attack is re-scored at the reveal: %d %s", enemyAttack(&b), b.Settled.Actors["enemy"].SelectedTierID)
	}
	if b.Segment.Current == segment.Defensive && remainingDamage(&b) != 4 {
		t.Fatalf("the attack reserves 4 cards: %d", remainingDamage(&b))
	}
	modified := false
	for _, ev := range events {
		if ev.Type == event.TypeDamageModified && ev.Data["damage_before"] == 6 && ev.Data["damage_after"] == 4 {
			modified = true
		}
	}
	if !modified {
		t.Fatal("the reveal publishes the attack's new damage")
	}
	// Released cards are not moved: every card the player held is still in hand,
	// except the Tamper itself.
	for _, card := range hand {
		if b.Settled.Actors["player"].CardInstances[card].DefinitionID == "sim_tamper" {
			continue
		}
		if !containsString(b.Actors["player"].Cards.Hand, card) && b.Segment.Current == segment.Defensive {
			t.Fatalf("%s left the hand", card)
		}
	}
	_ = lib
}

// Once you lock your ability in, your own attack is final: no card of yours
// can change its dice or bonuses in the step after; another participant's can.
func TestSimultaneousYourOwnAttackIsFinalAfterLockIn(t *testing.T) {
	b, _, e := simAIFixture(t, nil, nil)
	simAct(t, e, &b, "player", simType(command.TypePlanningRoll))
	simAct(t, e, &b, "player", simType(command.TypePlanningAbility))
	if simultaneousStep(&b) != state.SimultaneousDefenseBefore || !attackPending(&b, "player") {
		t.Fatalf("the player's attack is set: %s", simultaneousStep(&b))
	}
	if simStartable(e, &b, "player", "nudge") {
		t.Fatal("a dice card on your own locked-in attack is not offered")
	}
	if !simStartable(e, &b, "player", "sim_tamper") {
		t.Fatal("a dice card on the enemy's attack still is")
	}
}

// --- Defense -----------------------------------------------------------------

type simSource struct {
	id, from, to string
	amount       int
	status       string
}

// simDefense skips to Defense with the given attacks.
func simDefense(t *testing.T, e Engine, b *state.Battle, sources ...simSource) {
	t.Helper()
	closeSettledWindow(b)
	endSimultaneousStep(b)
	b.Segment.Current = segment.Defensive
	b.Flow = state.NewSegmentFlowState(b.Segment)
	b.Flow.Entered = true
	b.Settled.Stage = ""
	b.Settled.OffensiveSources = nil
	for _, s := range sources {
		source := state.SettledDamageSource{ID: s.id, SourceActorID: s.from, SourceContentID: "adventurer_strike", TargetActorID: s.to, BaseAmount: s.amount}
		if s.status != "" {
			source.StatusApplications = []state.SettledStatusApplication{{SourceActorID: s.from, TargetActorID: s.to, StatusID: s.status, Stacks: 1}}
		}
		b.Settled.OffensiveSources = append(b.Settled.OffensiveSources, source)
	}
	if _, err := e.progressSettled(b); err != nil {
		t.Fatal(err)
	}
}

func simDefend(ability string, sources ...string) func(command.Command) bool {
	return func(a command.Command) bool {
		var p command.PlanningAbilityPayload
		return a.Type == command.TypePlanningAbility && json.Unmarshal(a.Payload, &p) == nil && p.AbilityID == ability && reflect.DeepEqual(p.TargetIDs, sources)
	}
}

func simStartable(e Engine, b *state.Battle, actor, card string) bool {
	return simHas(e, b, actor, func(a command.Command) bool {
		id, c := simChoice(a)
		return c.Verb == "start" && b.Settled.Actors[actor].CardInstances[id].DefinitionID == card
	})
}

func TestSimultaneousDefenseRollsWhenChosenInPrivate(t *testing.T) {
	b, _, e := simultaneousFixture(t, nil, fortified("bf"))
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 4, "poison"}, simSource{"a2", "bob", "alice", 3, ""}, simSource{"b1", "alice", "bob", 4, ""})
	if simultaneousStep(&b) != state.SimultaneousDefenseBefore || len(b.Flow.PendingInput) != 2 {
		t.Fatalf("Defense opens for both participants at once: %v", b.Flow.PendingInput)
	}
	if simHas(e, &b, "alice", simDefend("adventurer_guard", "a1", "a2")) {
		t.Fatal("a once-per-segment defense can't cover both attacks")
	}
	if simHas(e, &b, "alice", simType(command.TypeRollDice)) {
		t.Fatal("there is no separate roll to lock in")
	}
	// Alice's Dispel on bob's Fortified waits for the reveal.
	simCardOn(t, e, &b, "alice", "dispel", "bob", "fortified")
	// Choosing a defense rolls and resolves it at once, for alice alone.
	events := simAct(t, e, &b, "alice", simDefend("sim_guard", "a1", "a2"))
	for _, id := range []string{"a1", "a2"} {
		d := b.Settled.DefenseHistory[id]
		if len(d.RolledDice) != 3 || !d.Finalized || d.AbilityID != "sim_guard" {
			t.Fatalf("the defense against %s rolled and resolved when chosen: %+v", id, d)
		}
	}
	if got := pendingSourceAmount(&b, "a1"); got != 4-b.Settled.DefenseHistory["a1"].Prevented {
		t.Fatalf("a1 shows its prevention at once: %d after preventing %d", got, b.Settled.DefenseHistory["a1"].Prevented)
	}
	if got := event.ForViewer(events, "bob"); len(got) != 0 {
		t.Fatalf("bob sees alice's defense before the reveal: %+v", got)
	}
	view := e.battleSnapshotForViewer(&b, "bob")
	if _, seen := view.DefenseHistory["a1"]; seen || simSourceAmount(view.SettledDamage, "a1") != 4 {
		t.Fatalf("bob sees a1 as the step began: %+v", view.SettledDamage.Sources)
	}
	if simStatus(&b, "alice", "poison") != 0 {
		t.Fatal("the defended attack's poison waits for the reveal")
	}
	// Bob defends with Fortified still on him: the Dispel hasn't resolved.
	simAct(t, e, &b, "bob", simDefend("adventurer_guard", "b1"))
	if got := len(b.Settled.DefenseHistory["b1"].RolledDice); got != 4 {
		t.Fatalf("Fortified adds its die until the reveal: %d", got)
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	// The reveal: the Dispel resolves, the poison lands, After opens for both.
	if simStatus(&b, "bob", "fortified") != 0 || simultaneousStep(&b) != state.SimultaneousDefenseAfter || len(b.Flow.PendingInput) != 2 {
		t.Fatalf("After follows the reveal: %s %v", simultaneousStep(&b), b.Flow.PendingInput)
	}
	if simStatus(&b, "alice", "poison") != 1 {
		t.Fatal("the defended attack's poison lands at the reveal, before After")
	}
	if view := e.battleSnapshotForViewer(&b, "bob"); simSourceAmount(view.SettledDamage, "a1") != pendingSourceAmount(&b, "a1") {
		t.Fatal("after the reveal bob sees alice's defense")
	}
	simCardOn(t, e, &b, "alice", "sim_salve_after", "alice", "poison")
	if simStatus(&b, "alice", "poison") != 0 {
		t.Fatal("Salve removed the poison before it ticks")
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	commits := simDefenseCommits(simAct(t, e, &b, "bob", simType(command.TypePlanningPass)))
	if commits != 1 || simStatus(&b, "alice", "poison") != 0 || b.Segment.Current == segment.Defensive {
		t.Fatalf("one final commit, and the landed poison is not applied twice: commits %d poison %d", commits, simStatus(&b, "alice", "poison"))
	}
	if simStatus(&b, "bob", "fortified") != 1 {
		t.Fatalf("Fortified is back next round: %v", b.Actors["bob"].Statuses)
	}
}

func simSourceAmount(batch *state.SettledDamageBatch, id string) int {
	if batch != nil {
		for _, s := range batch.Sources {
			if s.ID == id {
				return settledSourceAmount(s)
			}
		}
	}
	return -1
}

// A Before-defense card plays on an attack only until that attack has its
// defense (rolled, or none).
func TestSimultaneousBeforeDefenseCardsCloseWithEachDefense(t *testing.T) {
	b, lib, e := simultaneousFixture(t, nil, nil)
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 4, ""}, simSource{"a2", "bob", "alice", 3, ""})
	brace := ""
	for id, inst := range b.Settled.Actors["alice"].CardInstances {
		if inst.DefinitionID == "sim_brace" {
			brace = id
		}
	}
	targets := func() []string {
		var out []string
		for _, c := range programCandidates(&b, lib, "alice", brace, lib.Cards["sim_brace"].Program.Steps[0], nil) {
			if c.Source != "" && !containsString(out, c.Source) {
				out = append(out, c.Source)
			}
		}
		return out
	}
	if got := targets(); !reflect.DeepEqual(got, []string{"a1", "a2"}) {
		t.Fatalf("before any defense, it reaches both attacks: %v", got)
	}
	simAct(t, e, &b, "alice", simDefend("adventurer_guard", "a1"))
	if got := targets(); !reflect.DeepEqual(got, []string{"a2"}) || !simStartable(e, &b, "alice", "sim_brace") {
		t.Fatalf("once a1's defense rolled, only a2: %v", got)
	}
	simAct(t, e, &b, "alice", func(a command.Command) bool {
		var p command.PlanningPassPayload
		return a.Type == command.TypePlanningPass && json.Unmarshal(a.Payload, &p) == nil && p.SourceID == "a2"
	})
	if simStartable(e, &b, "alice", "sim_brace") {
		t.Fatal("once every attack has its defense, the Before card is gone")
	}
	if !simStartable(e, &b, "alice", "steady_guard") {
		t.Fatal("an any-time prevention card still plays")
	}
}

func TestSimultaneousRerollReResolvesADefense(t *testing.T) {
	b, lib, e := simultaneousFixture(t, nil, nil)
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 9, ""})
	energy := b.Actors["alice"].Resources.EnergyPoints
	simAct(t, e, &b, "alice", simDefend("adventurer_guard", "a1"))
	check := func(stage string, spent int) {
		t.Helper()
		d := b.Settled.DefenseHistory["a1"]
		result, err := e.defenseResult(&b, lib, d)
		if err != nil {
			t.Fatal(err)
		}
		prevent := 0
		for _, p := range result.Preventions {
			prevent += p.Amount
		}
		want := max(0, 9-prevent)
		if got := pendingSourceAmount(&b, "a1"); got != want || d.Prevented != 9-want {
			t.Fatalf("%s: a1 is %d (want %d), recorded prevention %d", stage, got, want, d.Prevented)
		}
		if d.EnergyGained != result.ResourceDeltas["alice"] || b.Actors["alice"].Resources.EnergyPoints != energy-spent+d.EnergyGained {
			t.Fatalf("%s: energy %d with %d gained", stage, b.Actors["alice"].Resources.EnergyPoints, d.EnergyGained)
		}
		reserved := 0
		for _, r := range b.Settled.PendingDamage.Removals {
			if r.Accepted && !r.Released {
				reserved++
			}
		}
		if reserved != want {
			t.Fatalf("%s: %d cards reserved for %d damage", stage, reserved, want)
		}
	}
	check("rolled", lib.Abilities["adventurer_guard"].Cost.Energy)
	// Second Guard reaches the dice beside the defended attack and re-resolves it.
	if !simHas(e, &b, "alice", func(a command.Command) bool {
		_, c := simChoice(a)
		return c.Kind == "defensive_die" && c.Source == "a1"
	}) {
		t.Fatal("the rolled defense's dice are targetable")
	}
	simCardOn(t, e, &b, "alice", "second_guard", "alice", "")
	check("rerolled", lib.Abilities["adventurer_guard"].Cost.Energy+lib.Cards["second_guard"].Cost.Energy)
}

func TestSimultaneousDispelAfterRollsDoesNotChangeThem(t *testing.T) {
	b, _, e := simultaneousFixture(t, nil, fortified("bf"))
	simDefense(t, e, &b, simSource{"b1", "alice", "bob", 4, ""})
	simAct(t, e, &b, "bob", simDefend("adventurer_guard", "b1"))
	if got := len(b.Settled.DefenseHistory["b1"].RolledDice); got != 4 {
		t.Fatalf("Fortified adds its die: %d", got)
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	simCardOn(t, e, &b, "alice", "sim_dispel_after", "bob", "fortified")
	if simStatus(&b, "bob", "fortified") != 0 || len(b.Settled.DefenseHistory["b1"].RolledDice) != 4 {
		t.Fatal("an After Dispel removes Fortified but not the extra die already rolled")
	}
}

func TestSimultaneousUndefendedStatusesCommitOnceAtTheEnd(t *testing.T) {
	b, _, e := simultaneousFixture(t, nil, nil)
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 2, "poison"})
	simAct(t, e, &b, "alice", func(a command.Command) bool {
		var p command.PlanningPassPayload
		return a.Type == command.TypePlanningPass && json.Unmarshal(a.Payload, &p) == nil && p.SourceID == "a1"
	})
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	if simultaneousStep(&b) != state.SimultaneousDefenseAfter || simStatus(&b, "alice", "poison") != 0 {
		t.Fatal("an undefended attack's status stays pending through After")
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	events := simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	var committed []event.Event
	for _, ev := range events {
		if ev.Type == event.TypeSegmentAdvanced {
			break
		}
		if ev.Type == event.TypeDamageCommitted {
			committed = append(committed, ev)
		}
	}
	if len(committed) != 1 {
		t.Fatalf("one final commit in Defense, got %d", len(committed))
	}
	raw, _ := json.Marshal(committed[0].Data)
	var data struct {
		Removals     []state.ProposedCardRemoval      `json:"removals"`
		Applications []state.SettledStatusApplication `json:"status_applications"`
	}
	_ = json.Unmarshal(raw, &data)
	removed := 0
	for _, r := range data.Removals {
		if r.Accepted && !r.Released {
			removed++
		}
	}
	if removed != 2 || len(data.Applications) != 1 || data.Applications[0].Applied || data.Applications[0].StatusID != "poison" {
		t.Fatalf("the remaining damage and the undefended status commit at the end: %+v", data)
	}
}

func TestSimultaneousOwnPreventionResolvesAtOnceInPrivate(t *testing.T) {
	b, _, e := simultaneousFixture(t, nil, nil)
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 3, ""})
	events := simCardOn(t, e, &b, "alice", "steady_guard", "alice", "")
	if remainingDamage(&b) != 2 || simPending(&b, "alice") == "" || secretPending(&b, "alice", simPending(&b, "alice")) {
		t.Fatalf("prevention on your own incoming attack resolves at once: %d", remainingDamage(&b))
	}
	if got := event.ForViewer(events, "bob"); len(got) != 0 {
		t.Fatalf("bob sees alice's prevention: %+v", got)
	}
	if view := e.battleSnapshotForViewer(&b, "bob"); simSourceAmount(view.SettledDamage, "a1") != 3 || len(view.SecretPlays) != 0 {
		t.Fatal("bob sees neither the prevention nor alice's play")
	}
	if view := e.battleSnapshotForViewer(&b, "alice"); simSourceAmount(view.SettledDamage, "a1") != 2 {
		t.Fatal("alice sees her prevention at once")
	}
	simAct(t, e, &b, "alice", simType(command.TypePlanningPass))
	if b.Settled.DefensePassed["alice"] {
		t.Fatal("locking in keeps the After step")
	}
	simAct(t, e, &b, "bob", simType(command.TypePlanningPass))
	if simultaneousStep(&b) != state.SimultaneousDefenseAfter || len(b.Flow.PendingInput) != 2 {
		t.Fatalf("After follows the reveal for both: %s %v", simultaneousStep(&b), b.Flow.PendingInput)
	}
	if view := e.battleSnapshotForViewer(&b, "bob"); simSourceAmount(view.SettledDamage, "a1") != 2 {
		t.Fatal("after the reveal bob sees the prevention")
	}
}

func TestSimultaneousAIDefenseUsesOnlyPublicInformation(t *testing.T) {
	plans := func(hidden bool) map[string]state.SettledDefense {
		b, lib, e := simAIFixture(t, nil, nil)
		r := b.Settled.Actors["enemy"]
		r.DefensiveAbilityIDs = []string{"salt_veil", "adventurer_guard", "adventurer_guard_plus"}
		b.Settled.Actors["enemy"] = r
		_ = lib
		simAct(t, e, &b, "player", simType(command.TypePlanningPass))
		simDefense(t, e, &b, simSource{"e1", "player", "enemy", 4, ""}, simSource{"e2", "player", "enemy", 2, ""})
		chosen := map[string]state.SettledDefense{}
		for id, plan := range b.Settled.DefensePlans {
			if plan.ActorID == "enemy" {
				chosen[id] = plan
			}
		}
		if len(chosen) != 2 {
			t.Fatalf("the AI chooses a defense for each attack at Defense entry: %+v", chosen)
		}
		if hidden {
			// A hidden draw consumes randomness; the AI has already chosen.
			simCardOn(t, e, &b, "player", "sim_draw", "", "")
		}
		simAct(t, e, &b, "player", simType(command.TypePlanningPass))
		return chosen
	}
	if quiet, hidden := plans(false), plans(true); !reflect.DeepEqual(quiet, hidden) {
		t.Fatalf("a hidden play changed the AI's choice: %+v vs %+v", quiet, hidden)
	}
}

// --- General -----------------------------------------------------------------

// TestSimultaneousViewNeverLeaksUnrevealedPlays checks every per-viewer
// output: snapshot, events, legal actions and pending input.
func TestSimultaneousViewNeverLeaksUnrevealedPlays(t *testing.T) {
	b, _, e := simultaneousFixture(t, nil, fortified("bf"))
	var events []event.Event
	events = append(events, simCardOn(t, e, &b, "alice", "dispel", "bob", "fortified")...)
	events = append(events, simCardOn(t, e, &b, "alice", "take_stock", "", "")...)
	var secret []string
	for _, play := range b.Settled.Simultaneous.Queue {
		secret = append(secret, play.CardID)
	}
	for _, card := range b.Actors["alice"].Cards.Hand {
		secret = append(secret, card)
	}
	check := func(stage string) {
		view := simView(t, e, &b, "bob", events)
		for _, id := range secret {
			if strings.Contains(view, id) {
				t.Fatalf("%s: bob's view names alice's card %s", stage, id)
			}
		}
		for _, word := range []string{"card_queued", "cards_drawn", "card_played", "card_program_choice"} {
			if strings.Contains(view, word) {
				t.Fatalf("%s: bob's view carries alice's %s event", stage, word)
			}
		}
		snap := e.battleSnapshotForViewer(&b, "bob")
		if len(snap.SecretPlays) != 0 || snap.Actors["alice"].EnergyPoints != 40 || snap.Actors["alice"].HandCount != len(simHand) {
			t.Fatalf("%s: bob's snapshot shows alice's plays: %+v", stage, snap.Actors["alice"])
		}
	}
	check("before lock-in")
	events = simAct(t, e, &b, "alice", simType(command.TypePlanningRoll))
	check("after alice locks in")
	// Defense Before: alice's plan and prevention stay hidden too.
	simDefense(t, e, &b, simSource{"a1", "bob", "alice", 3, ""})
	events = simCardOn(t, e, &b, "alice", "steady_guard", "alice", "")
	events = append(events, simAct(t, e, &b, "alice", simDefend("adventurer_guard", "a1"))...)
	secret = nil
	for _, play := range b.Settled.Simultaneous.Queue {
		secret = append(secret, play.CardID)
	}
	view := simView(t, e, &b, "bob", events)
	for _, id := range secret {
		if strings.Contains(view, id) {
			t.Fatalf("defense: bob's view names alice's card %s", id)
		}
	}
	if strings.Contains(view, `"adventurer_guard","source_id":"a1"`) || strings.Contains(view, "defense_planned") {
		t.Fatal("defense: bob's view shows alice's defense choice")
	}
}

func simRoundTrip(t *testing.T, b state.Battle) state.Battle {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var loaded state.Battle
	if err = json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestSimultaneousSaveReloadKeepsQueuedPlaysAndLockIns(t *testing.T) {
	finish := func(e Engine, b *state.Battle) string {
		simCardOn(t, e, b, "bob", "dispel", "alice", "fortified")
		simAct(t, e, b, "bob", simType(command.TypePlanningRoll))
		for _, id := range []string{"alice", "bob"} {
			simAct(t, e, b, id, simType(command.TypePlanningPass))
		}
		simCardOn(t, e, b, "alice", "sim_draw", "", "")
		simAct(t, e, b, "alice", simType(command.TypePlanningPass))
		mid := simRoundTrip(t, *b)
		*b = mid
		simAct(t, e, b, "bob", simType(command.TypePlanningPass))
		after := simRoundTrip(t, *b)
		*b = after
		for _, id := range []string{"alice", "bob"} {
			if _, ok := b.Flow.PendingInput[id]; ok {
				simAct(t, e, b, id, simType(command.TypePlanningPass))
			}
		}
		raw, _ := json.Marshal(b)
		return string(raw)
	}
	b, _, e := simultaneousFixture(t, fortified("af"), fortified("bf"))
	simCardOn(t, e, &b, "alice", "dispel", "bob", "fortified")
	simAct(t, e, &b, "alice", simType(command.TypePlanningRoll))
	reloaded := simRoundTrip(t, b)
	if !reflect.DeepEqual(reloaded.Settled.Simultaneous, b.Settled.Simultaneous) {
		t.Fatalf("the queue and lock-ins survive a reload: %+v vs %+v", reloaded.Settled.Simultaneous, b.Settled.Simultaneous)
	}
	direct := finish(e, &b)
	again := finish(NewEngine(), &reloaded)
	if direct != again {
		t.Fatal("a reloaded battle rerolled or re-resolved something")
	}
}

// simDefenseCommits counts the damage commits before the segment ends.
func simDefenseCommits(events []event.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Type == event.TypeSegmentAdvanced {
			break
		}
		if ev.Type == event.TypeDamageCommitted {
			n++
		}
	}
	return n
}

func TestSimultaneousRuleNeedsSupportedCards(t *testing.T) {
	b, lib, _ := simultaneousFixture(t, nil, nil)
	if !SimultaneousPlayEligible(&b) {
		t.Fatal("program cards and self-only cards support simultaneous play")
	}
	for _, card := range []string{"alchemists_gamble", "accelerant"} {
		if _, ok := lib.Cards[card]; !ok {
			continue
		}
		c := b.Clone()
		r := c.Settled.Actors["bob"]
		r.CardInstances["bob-extra"] = state.CardInstance{InstanceID: "bob-extra", DefinitionID: card}
		c.Settled.Actors["bob"] = r
		if SimultaneousPlayEligible(&c) {
			t.Fatalf("%s keeps the battle on the unified flow", card)
		}
	}
}
