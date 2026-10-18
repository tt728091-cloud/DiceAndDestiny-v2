package lookahead_test

import (
	"encoding/json"
	"math"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// recorder keeps the hero's decisions while another policy plays.
type recorder struct {
	inner lookahead.Policy
	keep  func(*lookahead.Decision) bool
	got   []*lookahead.Decision
}

func (r *recorder) Name() string { return "recorder" }

func (r *recorder) Choose(d *lookahead.Decision) (int, error) {
	if r.keep(d) {
		r.got = append(r.got, d)
	}
	return r.inner.Choose(d)
}

func stage(d *lookahead.Decision) string {
	if d.Battle.Settled == nil || d.Battle.Settled.Window == nil {
		return ""
	}
	return d.Battle.Settled.Window.Stage
}

// firstRollDecision returns the hero's opening offensive decision, before
// any dice are rolled.
func firstRollDecision(t *testing.T) *lookahead.Decision {
	t.Helper()
	runner, foe := brineMaskRunner(t)
	r := &recorder{inner: lookahead.ProxyPolicy{}, keep: func(d *lookahead.Decision) bool {
		return stage(d) == "planning" && len(d.Battle.Settled.Actors[d.Seat].FinalDice) == 0
	}}
	if _, err := runner.Play(4, r, foe); err != nil {
		t.Fatal(err)
	}
	if len(r.got) == 0 {
		t.Fatal("no opening decision recorded")
	}
	return r.got[0]
}

func rollCommand(t *testing.T, d *lookahead.Decision) command.Command {
	t.Helper()
	for _, a := range d.Actions {
		if a.Type == command.TypePlanningRoll {
			return a
		}
	}
	t.Fatal("no roll command")
	return command.Command{}
}

func faces(b *state.Battle, seat string) []int {
	var out []int
	for _, die := range b.Settled.Actors[seat].FinalDice {
		out = append(out, die.Face)
	}
	return out
}

// A look-ahead copy must not know the real battle's future rolls: the same
// seed repeats, different seeds differ, and none simply replays the real
// random stream.
func TestForkHidesTheRealFuture(t *testing.T) {
	d := firstRollDecision(t)
	roll := rollCommand(t, d)
	real := d.Battle.Clone()
	if err := d.Sim.Apply(&real, roll); err != nil {
		t.Fatal(err)
	}
	truth := faces(&real, d.Seat)
	differs := 0
	for seed := uint64(1); seed <= 8; seed++ {
		a := lookahead.Fork(d.Battle, d.Seat, seed)
		b := lookahead.Fork(d.Battle, d.Seat, seed)
		if err := d.Sim.Apply(&a, roll); err != nil {
			t.Fatal(err)
		}
		if err := d.Sim.Apply(&b, roll); err != nil {
			t.Fatal(err)
		}
		if !equal(faces(&a, d.Seat), faces(&b, d.Seat)) {
			t.Fatalf("seed %d is not reproducible", seed)
		}
		if !equal(faces(&a, d.Seat), truth) {
			differs++
		}
	}
	if differs < 6 {
		t.Fatalf("only %d of 8 forks rolled differently from the real battle %v", differs, truth)
	}
	if d.Battle.Random.Seed == lookahead.Fork(d.Battle, d.Seat, 1).Random.Seed {
		t.Fatal("fork kept the real random seed")
	}
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setDice replaces the hero's offensive dice with chosen faces.
func setDice(b *state.Battle, seat string, values []int, rollsUsed int) {
	rt := b.Settled.Actors[seat]
	def := b.DiceDefinitions[rt.FinalDice[0].DieID]
	for i, v := range values {
		for _, face := range def.Faces {
			if face.Face == v {
				rt.FinalDice[i].Face, rt.FinalDice[i].Value, rt.FinalDice[i].Symbols = face.Face, face.Value, face.Symbols
			}
		}
	}
	rt.RollsUsed = rollsUsed
	b.Settled.Actors[seat] = rt
}

// The solver's numbers can be checked by hand: with 3-1-4-3-2 and one roll
// left the small straight (6 damage) is worth 6 now; rerolling one of the 3s
// keeps it and makes the large straight (8) on a 5, so 6 + 2/6.
func TestDiceSolverByHand(t *testing.T) {
	d := firstRollDecision(t)
	b := d.Battle.Clone()
	if err := d.Sim.Apply(&b, rollCommand(t, d)); err != nil {
		t.Fatal(err)
	}
	rt := b.Settled.Actors[d.Seat]
	// No energy, so no dice card in hand can be played.
	actor := b.Actors[d.Seat]
	actor.Resources.EnergyPoints = 0
	b.Actors[d.Seat] = actor
	setDice(&b, d.Seat, []int{3, 1, 4, 3, 2}, rt.MaxRolls-1)
	plan, err := lookahead.NewDiceSolver().Plan(&b, d.Seat)
	if err != nil || plan == nil {
		t.Fatal(err, plan == nil)
	}
	if plan.Stop != 6 || plan.BestAbility != "adventurer_small_straight" {
		t.Fatalf("stop %v with %s, want 6 with the small straight", plan.Stop, plan.BestAbility)
	}
	if got, want := plan.Reroll([]int{0}), 6+2.0/6; math.Abs(got-want) > 1e-9 {
		t.Fatalf("reroll a 3: %v, want %v", got, want)
	}
	if got := plan.Best(); math.Abs(got-(6+2.0/6)) > 1e-9 {
		t.Fatalf("best %v, want the reroll", got)
	}
	// With no rolls left only the dice on the table count.
	setDice(&b, d.Seat, []int{3, 1, 4, 3, 2}, rt.MaxRolls)
	plan, _ = lookahead.NewDiceSolver().Plan(&b, d.Seat)
	if plan.Best() != 6 || plan.Reroll([]int{0}) != 6 {
		t.Fatalf("no rolls left: best %v reroll %v", plan.Best(), plan.Reroll([]int{0}))
	}
}

// With Nudges in hand the solver keeps them as insurance. The position is
// from a reviewed game: 2-6-6-4-5, two rerolls left, two Nudges and 20
// energy. Priced at 5 energy × 0.1, rerolling one 6 is worth 7.4167 and
// nudging the 2 to a 3 first only 6.75 (the same numbers as an independent
// exact calculation); rerolling both 6s is worth 7.3125.
func TestDiceSolverKeepsNudgesForLater(t *testing.T) {
	d := firstRollDecision(t)
	b := d.Battle.Clone()
	if err := d.Sim.Apply(&b, rollCommand(t, d)); err != nil {
		t.Fatal(err)
	}
	rt := b.Settled.Actors[d.Seat]
	actor := b.Actors[d.Seat]
	var nudges []string
	for id, instance := range rt.CardInstances {
		if instance.DefinitionID == "nudge" {
			nudges = append(nudges, id)
		}
	}
	if len(nudges) < 2 {
		t.Fatalf("Starter deck has %d Nudges", len(nudges))
	}
	actor.Cards.Hand = nudges[:2]
	actor.Resources.EnergyPoints = 20
	b.Actors[d.Seat] = actor
	setDice(&b, d.Seat, []int{2, 6, 6, 4, 5}, rt.MaxRolls-2)
	solver := lookahead.NewDiceSolver()
	solver.CardValue = 0
	plan, err := solver.Plan(&b, d.Seat)
	if err != nil || plan == nil {
		t.Fatal(err, plan == nil)
	}
	lib, err := engine.SettledLibrary(&b)
	if err != nil {
		t.Fatal(err)
	}
	nudge := lib.Cards["nudge"]
	close := func(name string, got, want float64) {
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: %.6f, want %.6f", name, got, want)
		}
	}
	// Nudge costs 7 energy (card-value-rules.md); each play is charged
	// 0.1 per energy plus CardValue (0 here).
	close("best", plan.Best(), 7.227777777777778)
	close("reroll one 6", plan.Reroll([]int{1}), 7.227777777777778)
	close("reroll both 6s", plan.Reroll([]int{1, 2}), 7.131327160493827)
	close("Nudge the 2 to a 3", plan.Nudge(nudge.ID, nudge.Cost.Energy, 0, 3), 6.45)
	if v, ok := plan.PlayCard(nudge, b.Settled.Actors[d.Seat].FinalDice); !ok || v >= plan.Reroll([]int{1}) {
		t.Fatalf("playing Nudge now (%.4f) should lose to rerolling a 6", v)
	}
}

// Useful drops kept-dice bookkeeping, start-then-cancel and duplicate card
// copies, and keeps every real choice.
func TestUsefulDropsBookkeeping(t *testing.T) {
	runner, foe := brineMaskRunner(t)
	r := &recorder{inner: lookahead.ProxyPolicy{}, keep: func(d *lookahead.Decision) bool {
		return stage(d) == "planning" && len(d.Battle.Settled.Actors[d.Seat].FinalDice) > 0
	}}
	if _, err := runner.Play(4, r, foe); err != nil {
		t.Fatal(err)
	}
	d := r.got[0]
	useful := lookahead.Useful(d.Battle, d.Seat, d.Actions)
	kept := map[int]bool{}
	for _, i := range useful {
		kept[i] = true
	}
	seen := map[string]bool{}
	rerolls := 0
	for i, a := range d.Actions {
		switch {
		case a.Type == command.TypePlanningKeep && kept[i]:
			t.Fatal("kept a keep-dice command")
		case lookahead.CardChoice(a).Verb == "cancel" && kept[i]:
			t.Fatal("kept a cancel")
		case a.Type == command.TypePlanningReroll && kept[i]:
			rerolls++
		}
		if kept[i] && len(lookahead.CardIDs(a)) == 1 {
			instance := d.Battle.Settled.Actors[d.Seat].CardInstances[lookahead.CardIDs(a)[0]].DefinitionID
			key := instance + lookahead.CardChoice(a).Label
			if seen[key] {
				t.Fatalf("kept two copies of %s", key)
			}
			seen[key] = true
		}
	}
	if rerolls != 31 {
		t.Fatalf("kept %d rerolls, want all 31", rerolls)
	}
}

// The look-ahead player finishes real battles through the authority.
func TestLookaheadFinishesBattles(t *testing.T) {
	runner, foe := brineMaskRunner(t)
	_, model := brineMaskRunner(t)
	search := lookahead.NewSearchPolicy(func() lookahead.Policy { return model }, 12)
	for seed := uint64(1); seed <= 2; seed++ {
		record, err := runner.Play(seed, search, foe)
		if err != nil {
			t.Fatal(err)
		}
		if record.Result != "win" && record.Result != "draw" && record.Result != "loss" {
			t.Fatalf("seed %d ended %q", seed, record.Result)
		}
	}
	if searched, _ := search.Counts(); searched == 0 {
		t.Fatal("no decision was searched")
	}
}

// The greedy player's discard and card choices only name listed commands.
func TestGreedyFinishesBattles(t *testing.T) {
	runner, foe := brineMaskRunner(t)
	for seed := uint64(1); seed <= 10; seed++ {
		record, err := runner.Play(seed, lookahead.NewGreedyPolicy(), foe)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(record)
		if record.Result == "truncated" {
			t.Fatalf("seed %d truncated: %s", seed, raw)
		}
	}
}
