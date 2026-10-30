package tactics

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// §11.3: speeds 1–10 over mixed terrain enter each square at exactly C/M,
// with no rounding anywhere.
func TestExactEntryTimes(t *testing.T) {
	row := ".~.~~......."
	g := grid(testRules(), row)
	costs := []int{2, 1, 2, 2, 1, 1, 1, 1, 1, 1, 1}
	for speed := 1; speed <= 10; speed++ {
		r := Simulate(g, []Mover{mover("a", "x", 0, 0, speed, moveTo(11, 0))}, 1)
		var want []string
		total := 0
		for i, c := range costs {
			if total+c > speed {
				break
			}
			total += c
			want = append(want, fmt.Sprintf("%d,0@%s", i+1, F(int64(total), int64(speed))))
		}
		if got := steps(r, "a"); got != strings.Join(want, " ") {
			t.Fatalf("speed %d: steps %q, want %q", speed, got, strings.Join(want, " "))
		}
		if r.Spent["a"] != total {
			t.Fatalf("speed %d spent %d, want %d", speed, r.Spent["a"], total)
		}
		for _, e := range entries(r, EntryStep, "a") {
			// Every entry time is an exact fraction with the mover's speed as
			// a denominator multiple: t × M is a whole number of Move.
			if (e.At.Num*int64(speed))%e.At.Den != 0 {
				t.Fatalf("speed %d entered at %s, not a multiple of 1/%d", speed, e.At, speed)
			}
		}
	}
	if !F(2, 6).Equal(F(1, 3)) || !F(1, 3).Less(F(3, 7)) || F(7, 7) != One {
		t.Fatal("fraction comparisons are not exact")
	}
}

func TestMoversOfDifferentSpeedsShareExactMoments(t *testing.T) {
	g := grid(testRules(), open(12, 3)...)
	r := Simulate(g, []Mover{
		mover("a", "x", 0, 0, 3, moveTo(11, 0)),
		mover("b", "x", 0, 2, 6, moveTo(11, 2)),
	}, 1)
	if got := steps(r, "a"); got != "1,0@1/3 2,0@2/3 3,0@1" {
		t.Fatalf("a = %q", got)
	}
	if got := steps(r, "b"); got != "1,2@1/6 2,2@1/3 3,2@1/2 4,2@2/3 5,2@5/6 6,2@1" {
		t.Fatalf("b = %q", got)
	}
}

// §11.4: crossing paths collide only when their occupancy windows overlap.
func TestCrossingPathsWithoutCollision(t *testing.T) {
	g := grid(testRules(), open(8, 6)...)
	// The runner holds (3,2) during [1/2, 2/3); the walker arrives at t=1.
	r := Simulate(g, []Mover{
		mover("runner", "heroes", 0, 2, 6, moveTo(6, 2)),
		mover("walker", "monsters", 3, 3, 1, moveTo(3, 2)),
	}, 1)
	expectFinal(t, r, "runner", 6, 2)
	expectFinal(t, r, "walker", 3, 2)
	if w := entries(r, EntryWait, ""); len(w) != 0 {
		t.Fatalf("no one should wait:\n%s", dump(r))
	}
}

func TestCrossingPathsWithCollision(t *testing.T) {
	g := grid(testRules(), open(8, 6)...)
	// The runner reaches (3,2) at 3/4 and stays: the walker's entry at t=1
	// overlaps its window and is blocked.
	r := Simulate(g, []Mover{
		mover("runner", "heroes", 0, 2, 4, moveTo(3, 2)),
		mover("walker", "monsters", 3, 3, 1, moveTo(3, 1)),
	}, 1)
	expectFinal(t, r, "runner", 3, 2)
	expectFinal(t, r, "walker", 3, 3)
	waits := entries(r, EntryWait, "walker")
	if len(waits) != 1 || waits[0].Reason != ReasonBlocked || !waits[0].At.Equal(One) {
		t.Fatalf("walker should be blocked at t=1:\n%s", dump(r))
	}
}

func TestLeavingAtTheSameMomentFreesTheSquare(t *testing.T) {
	g := grid(testRules(), open(8, 6)...)
	// The runner holds (3,2) during [3/4, 1) and leaves exactly when the
	// walker enters: half-open windows do not overlap.
	r := Simulate(g, []Mover{
		mover("runner", "heroes", 0, 2, 4, moveTo(6, 2)),
		mover("walker", "monsters", 3, 3, 1, moveTo(3, 1)),
	}, 1)
	expectFinal(t, r, "runner", 4, 2)
	expectFinal(t, r, "walker", 3, 2)
}

// §11.5: each tiebreak level decides one case, down to the seeded random.
func TestTiebreakSquaresMoved(t *testing.T) {
	g := grid(testRules(), ".....", "..~..", ".....")
	a := mover("a", "heroes", 0, 1, 3, moveTo(4, 1))
	a.Costs = CostModifiers{IgnoreTerrain: true}
	b := mover("b", "monsters", 2, 0, 3, moveTo(2, 1))
	r := Simulate(g, []Mover{a, b}, 1)
	expectTiebreak(t, r, "a", TiebreakSquaresMoved, F(2, 3))
	if r.Final["b"] == (Square{2, 1}) {
		t.Fatal("the loser entered the contested square")
	}
}

func TestTiebreakMoveRate(t *testing.T) {
	g := grid(testRules(), ".....", "..~..", ".....")
	a := mover("a", "heroes", 1, 1, 2, moveTo(3, 1))
	a.Costs = CostModifiers{IgnoreTerrain: true}
	b := mover("b", "monsters", 2, 2, 4, moveTo(2, 1))
	r := Simulate(g, []Mover{a, b}, 1)
	expectTiebreak(t, r, "b", TiebreakMoveRate, F(1, 2))
	expectFinal(t, r, "b", 2, 1)
}

func TestTiebreakPaidMove(t *testing.T) {
	g := grid(testRules(), open(5, 3)...)
	a := mover("a", "heroes", 1, 1, 2, moveTo(2, 1))
	a.Paid = 1
	b := mover("b", "monsters", 3, 1, 2, moveTo(2, 1))
	r := Simulate(g, []Mover{a, b}, 1)
	expectTiebreak(t, r, "a", TiebreakPaidMove, F(1, 2))
	expectFinal(t, r, "a", 2, 1)
	expectFinal(t, r, "b", 3, 1)
}

func TestTiebreakSeededRandom(t *testing.T) {
	g := grid(testRules(), open(5, 3)...)
	movers := []Mover{mover("a", "heroes", 1, 1, 1, moveTo(2, 1)), mover("b", "monsters", 3, 1, 1, moveTo(2, 1))}
	winners := map[string]int{}
	for seed := uint64(1); seed <= 60; seed++ {
		first := Simulate(g, movers, seed)
		again := Simulate(g, movers, seed)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("seed %d is not deterministic", seed)
		}
		tie := entries(first, EntryTiebreak, "")
		if len(tie) != 1 || tie[0].Rule != TiebreakRandom {
			t.Fatalf("seed %d: want one random tiebreak:\n%s", seed, dump(first))
		}
		winners[tie[0].Winner]++
		expectFinal(t, first, tie[0].Winner, 2, 1)
	}
	if winners["a"] == 0 || winners["b"] == 0 {
		t.Fatalf("the random tiebreak always picks the same side: %v", winners)
	}
}

func expectTiebreak(t *testing.T, r Result, winner, rule string, at Frac) {
	t.Helper()
	tie := entries(r, EntryTiebreak, "")
	if len(tie) != 1 || tie[0].Winner != winner || tie[0].Rule != rule || !tie[0].At.Equal(at) {
		t.Fatalf("want %s to win by %s at %s:\n%s", winner, rule, at, dump(r))
	}
}

// §11.6: opponents never swap head-on (both wait); allies may swap and pass
// through each other but never end on the same square.
func TestHeadOnSwapBetweenOpponentsIsRefused(t *testing.T) {
	g := grid(testRules(), "......")
	r := Simulate(g, []Mover{mover("a", "heroes", 2, 0, 3, moveTo(5, 0)), mover("b", "monsters", 3, 0, 3, moveTo(0, 0))}, 1)
	expectFinal(t, r, "a", 2, 0)
	expectFinal(t, r, "b", 3, 0)
	if len(entries(r, EntryWait, "a")) == 0 || len(entries(r, EntryWait, "b")) == 0 {
		t.Fatalf("both must wait:\n%s", dump(r))
	}
}

func TestSimultaneousSwapAttemptBothWait(t *testing.T) {
	// Two opponents that both step into each other's square at the same
	// moment: neither enters.
	g := grid(testRules(), "....")
	a := &moverState{m: mover("a", "heroes", 1, 0, 1, moveTo(2, 0)), pos: Square{1, 0}, attempt: true, attemptTo: Square{2, 0}, attemptDir: Direction{1, 0}, cost: 1, attemptAt: One}
	b := &moverState{m: mover("b", "monsters", 2, 0, 1, moveTo(1, 0)), pos: Square{2, 0}, attempt: true, attemptTo: Square{1, 0}, attemptDir: Direction{-1, 0}, cost: 1, attemptAt: One}
	s := &simulation{g: g, states: []*moverState{a, b}, byID: map[string]*moverState{"a": a, "b": b}, contact: map[string]Frac{}}
	s.resolve(One, []*moverState{a, b})
	if a.pos != (Square{1, 0}) || b.pos != (Square{2, 0}) {
		t.Fatal("opponents swapped squares")
	}
	r := Result{Timeline: s.timeline}
	if w := entries(r, EntryWait, ""); len(w) != 2 || w[0].Reason != ReasonSwap || w[1].Reason != ReasonSwap {
		t.Fatalf("both must wait for the swap:\n%s", dump(r))
	}
}

func TestAlliesSwapAndPassThrough(t *testing.T) {
	g := grid(testRules(), "......")
	r := Simulate(g, []Mover{mover("a", "heroes", 2, 0, 1, moveTo(3, 0)), mover("b", "heroes", 3, 0, 1, moveTo(2, 0))}, 1)
	expectFinal(t, r, "a", 3, 0)
	expectFinal(t, r, "b", 2, 0)
	r = Simulate(g, []Mover{mover("a", "heroes", 0, 0, 2, moveTo(2, 0)), mover("b", "heroes", 1, 0, 0, hold())}, 1)
	expectFinal(t, r, "a", 2, 0)
	if got := steps(r, "a"); got != "1,0@1/2 2,0@1" {
		t.Fatalf("passing through an ally: %q", got)
	}
}

func TestAlliesNeverEndOnTheSameSquare(t *testing.T) {
	g := grid(testRules(), "......")
	r := Simulate(g, []Mover{mover("a", "heroes", 0, 0, 2, moveTo(2, 0)), mover("b", "heroes", 2, 0, 0, hold())}, 1)
	expectFinal(t, r, "a", 1, 0)
	expectFinal(t, r, "b", 2, 0)
	stops := entries(r, EntryStop, "a")
	if len(stops) != 1 || stops[0].Reason != ReasonAllyEnd {
		t.Fatalf("a must stop short of its ally:\n%s", dump(r))
	}

	// Two allies racing for one empty square: the first to enter keeps it.
	g2 := grid(testRules(), open(7, 5)...)
	r = Simulate(g2, []Mover{mover("fast", "heroes", 1, 2, 3, moveTo(3, 2)), mover("slow", "heroes", 3, 4, 2, moveTo(3, 2))}, 1)
	expectFinal(t, r, "fast", 3, 2)
	if r.Final["slow"] == r.Final["fast"] {
		t.Fatalf("allies share a square:\n%s", dump(r))
	}
	expectFinal(t, r, "slow", 3, 3)

	// Many allies converging still end apart.
	movers := []Mover{mover("t", "monsters", 3, 2, 0, hold())}
	for i, start := range []Square{{0, 0}, {6, 0}, {0, 4}, {6, 4}, {3, 0}, {0, 2}} {
		movers = append(movers, mover(fmt.Sprintf("h%d", i), "heroes", start.X, start.Y, 6, moveTo(3, 3)))
	}
	r = Simulate(g2, movers, 3)
	seen := map[Square]string{}
	for id, s := range r.Final {
		if other, dup := seen[s]; dup {
			t.Fatalf("%s and %s share %v:\n%s", id, other, s, dump(r))
		}
		seen[s] = id
	}
}

// §11.7: pursuit and flight.
func TestPursueCatchesASlowerFleeingTarget(t *testing.T) {
	g := grid(testRules(), open(12, 9)...)
	hunter := mover("hunter", "monsters", 0, 4, 3, pursue("prey"))
	hunter.ContactTarget = "prey"
	r := Simulate(g, []Mover{hunter, mover("prey", "heroes", 3, 4, 1, flee("hunter"))}, 1)
	at, ok := r.Contact["hunter"]
	if !ok || !at.Less(One) {
		t.Fatalf("the faster hunter never caught its prey:\n%s", dump(r))
	}
	if len(entries(r, EntryWait, "hunter")) == 0 {
		t.Fatalf("the hunter should wait while adjacent:\n%s", dump(r))
	}
}

func TestFasterTargetEscapesPursuit(t *testing.T) {
	g := grid(testRules(), open(12, 9)...)
	hunter := mover("hunter", "monsters", 0, 4, 2, pursue("prey"))
	hunter.ContactTarget = "prey"
	r := Simulate(g, []Mover{hunter, mover("prey", "heroes", 3, 4, 4, flee("hunter"))}, 1)
	if _, ok := r.Contact["hunter"]; ok {
		t.Fatalf("the faster prey was caught:\n%s", dump(r))
	}
	if Distance(DiagonalChebyshev, r.Final["hunter"], r.Final["prey"]) < 4 {
		t.Fatalf("the prey did not open distance:\n%s", dump(r))
	}
}

func TestFleeInACornerWaits(t *testing.T) {
	g := grid(testRules(), open(6, 6)...)
	r := Simulate(g, []Mover{mover("prey", "heroes", 0, 0, 3, flee("hunter")), mover("hunter", "monsters", 1, 1, 0, hold())}, 1)
	expectFinal(t, r, "prey", 0, 0)
	waits := entries(r, EntryWait, "prey")
	if len(waits) != 1 || waits[0].Reason != ReasonNoEscape {
		t.Fatalf("a cornered creature waits:\n%s", dump(r))
	}
	if r.Spent["prey"] != 0 {
		t.Fatal("waiting must not spend Move")
	}
}

func TestFleeTiesPreferCheaperTerrainThenDirectionOrder(t *testing.T) {
	// From (2,2), fleeing a hunter at (2,4), N, NE and NW all gain a square.
	first := func(rows ...string) string {
		r := Simulate(grid(testRules(), rows...), []Mover{mover("prey", "heroes", 2, 2, 2, flee("hunter")), mover("hunter", "monsters", 2, 4, 0, hold())}, 1)
		return strings.Fields(steps(r, "prey"))[0]
	}
	if got := first(open(5, 5)...); got != "2,1@1/2" {
		t.Fatalf("equal terrain: first step %s, want N (2,1)", got)
	}
	if got := first(".....", "..~..", ".....", ".....", "....."); got != "3,1@1/2" {
		t.Fatalf("N is swamp: first step %s, want NE (3,1)", got)
	}
	if got := first(".....", "..~~.", ".....", ".....", "....."); got != "1,1@1/2" {
		t.Fatalf("N and NE are swamp: first step %s, want NW (1,1)", got)
	}
}

func TestPursuitRepathsAroundABlocker(t *testing.T) {
	g := grid(testRules(), open(10, 2)...)
	r := Simulate(g, []Mover{
		mover("hunter", "heroes", 0, 0, 4, pursue("prey")),
		mover("prey", "monsters", 8, 0, 0, hold()),
		mover("blocker", "monsters", 3, 1, 4, moveTo(3, 0)),
	}, 1)
	for _, e := range entries(r, EntryStep, "hunter") {
		if *e.To == (Square{3, 0}) {
			t.Fatalf("the hunter walked through the blocker:\n%s", dump(r))
		}
	}
	got := steps(r, "hunter")
	if !strings.Contains(got, "3,1@") {
		t.Fatalf("the hunter did not re-path around the blocker: %q\n%s", got, dump(r))
	}
}

func TestMoveToWaitsWhenAnOpponentHoldsTheOnlyWay(t *testing.T) {
	g := grid(testRules(), ".....#....", "..........", ".....#....")
	r := Simulate(g, []Mover{mover("a", "heroes", 0, 1, 6, moveTo(9, 1)), mover("door", "monsters", 5, 1, 0, hold())}, 1)
	if r.Final["a"].X >= 5 {
		t.Fatalf("passed an opponent in the doorway:\n%s", dump(r))
	}
	if w := entries(r, EntryWait, "a"); len(w) == 0 || w[len(w)-1].Reason != ReasonNoRoute {
		t.Fatalf("must wait at the door:\n%s", dump(r))
	}
}

func TestBudgetAndTimeStopMovement(t *testing.T) {
	g := grid(testRules(), "..~.....")
	r := Simulate(g, []Mover{mover("a", "x", 0, 0, 2, moveTo(7, 0))}, 1)
	expectFinal(t, r, "a", 1, 0)
	if stops := entries(r, EntryStop, "a"); len(stops) != 1 || stops[0].Reason != ReasonBudget {
		t.Fatalf("must stop for budget before the swamp:\n%s", dump(r))
	}
	r = Simulate(g, []Mover{mover("a", "x", 0, 0, 5, moveTo(2, 0))}, 1)
	if stops := entries(r, EntryStop, "a"); len(stops) != 1 || stops[0].Reason != ReasonArrived {
		t.Fatalf("must stop on arrival:\n%s", dump(r))
	}
	if r.Spent["a"] != 3 {
		t.Fatalf("spent %d, want 3", r.Spent["a"])
	}
}

func TestChargeContactMidMovementAndWhenOutrun(t *testing.T) {
	g := grid(testRules(), open(12, 9)...)
	charger := mover("charger", "heroes", 0, 4, 4, pursue("target"))
	charger.ContactTarget = "target"
	r := Simulate(g, []Mover{charger, mover("target", "monsters", 4, 4, 0, hold())}, 1)
	if at, ok := r.Contact["charger"]; !ok || !at.Equal(F(3, 4)) {
		t.Fatalf("contact = %v %v, want 3/4:\n%s", at, ok, dump(r))
	}
	r = Simulate(g, []Mover{charger, mover("target", "monsters", 4, 4, 5, flee("charger"))}, 1)
	if _, ok := r.Contact["charger"]; ok {
		t.Fatalf("an outrun charge connected:\n%s", dump(r))
	}
	adjacent := mover("charger", "heroes", 3, 4, 4, pursue("target"))
	adjacent.ContactTarget = "target"
	r = Simulate(g, []Mover{adjacent, mover("target", "monsters", 4, 4, 0, hold())}, 1)
	if at, ok := r.Contact["charger"]; !ok || !at.Equal(Zero) {
		t.Fatal("a charger already adjacent connects at t=0")
	}
}

func TestPreviewAssumesEveryoneElseHolds(t *testing.T) {
	g := grid(testRules(), open(8, 3)...)
	movers := []Mover{mover("a", "heroes", 0, 1, 3, moveTo(5, 1)), mover("b", "monsters", 7, 1, 5, moveTo(2, 1))}
	p := Preview(g, movers, "a", 1)
	expectFinal(t, p, "a", 3, 1)
	expectFinal(t, p, "b", 7, 1)
}

// §11.14: the same movers and seed give the same timeline.
func TestSimulationIsDeterministic(t *testing.T) {
	g := grid(testRules(), "............", "..#....#....", "......#.....", "...#..~~..#.", "......~~....", ".....#..#...", "..#.........", "........#...", "....#.......")
	movers := []Mover{
		mover("h", "heroes", 1, 4, 3, moveTo(10, 4)),
		mover("e1", "monsters", 10, 4, 2, pursue("h")),
		mover("e2", "monsters", 10, 1, 1, pursue("h")),
		mover("e3", "monsters", 10, 7, 4, flee("h")),
		mover("e4", "monsters", 8, 3, 3, moveTo(1, 4)),
	}
	first := Simulate(g, movers, 99)
	for i := 0; i < 5; i++ {
		if again := Simulate(g, movers, 99); !reflect.DeepEqual(first, again) {
			t.Fatal("same movers and seed gave a different result")
		}
	}
}
