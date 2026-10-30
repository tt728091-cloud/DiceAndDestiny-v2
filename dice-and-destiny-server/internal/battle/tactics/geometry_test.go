package tactics

import (
	"math/rand"
	"testing"
)

// §11.1: Chebyshev and alternating distances; corner cutting refused; swamp
// costs 2; walls impassable.
func TestDistances(t *testing.T) {
	cases := []struct {
		a, b               Square
		chebyshev, altered int
	}{
		{Square{0, 0}, Square{3, 1}, 3, 3},
		{Square{0, 0}, Square{1, 1}, 1, 1},
		{Square{0, 0}, Square{2, 2}, 2, 3},
		{Square{0, 0}, Square{4, 4}, 4, 6},
		{Square{5, 2}, Square{1, 7}, 5, 7},
		{Square{3, 3}, Square{3, 3}, 0, 0},
	}
	for _, c := range cases {
		if got := Distance(DiagonalChebyshev, c.a, c.b); got != c.chebyshev {
			t.Errorf("chebyshev %v→%v = %d, want %d", c.a, c.b, got, c.chebyshev)
		}
		if got := Distance(DiagonalAlternating, c.a, c.b); got != c.altered {
			t.Errorf("alternating %v→%v = %d, want %d", c.a, c.b, got, c.altered)
		}
		if Distance(DiagonalAlternating, c.a, c.b) != Distance(DiagonalAlternating, c.b, c.a) {
			t.Errorf("alternating distance is not symmetric for %v %v", c.a, c.b)
		}
	}
}

func TestCornerCuttingIsRefused(t *testing.T) {
	rows := []string{"...", ".#.", "..."}
	g := grid(testRules(), rows...)
	// (0,1)→(1,0) passes the wall at (1,1).
	if _, ok := g.StepCost(Square{0, 1}, Direction{1, -1}, 0, CostModifiers{}); ok {
		t.Fatal("a diagonal past a wall was allowed")
	}
	if cost, ok := g.StepCost(Square{0, 0}, Direction{1, 0}, 0, CostModifiers{}); !ok || cost != 1 {
		t.Fatal("an orthogonal step beside a wall must cost 1")
	}
	field := NewPathField(g, CostModifiers{}, []Square{{2, 2}}, nil, Square{2, 2})
	if got := field.CostTo(Square{0, 0}, 0); got != 4 {
		t.Fatalf("around the pillar without cutting costs %d, want 4", got)
	}
	cutting := testRules()
	cutting.CornerCutting = true
	g2 := grid(cutting, rows...)
	if _, ok := g2.StepCost(Square{0, 1}, Direction{1, -1}, 0, CostModifiers{}); !ok {
		t.Fatal("corner cutting on: the diagonal must be allowed")
	}
	if got := NewPathField(g2, CostModifiers{}, []Square{{2, 2}}, nil, Square{2, 2}).CostTo(Square{0, 0}, 0); got != 3 {
		t.Fatalf("with corner cutting the route costs %d, want 3", got)
	}
}

func TestSwampCostsTwoAndWallsAreImpassable(t *testing.T) {
	g := grid(testRules(), ".~.", "###")
	if cost, ok := g.EntryCost(Square{1, 0}, CostModifiers{}); !ok || cost != 2 {
		t.Fatalf("swamp entry = %d %v, want 2", cost, ok)
	}
	if got := NewPathField(g, CostModifiers{}, []Square{{2, 0}}, nil, Square{2, 0}).CostTo(Square{0, 0}, 0); got != 3 {
		t.Fatalf("floor→swamp→floor costs %d, want 3", got)
	}
	if _, ok := g.EntryCost(Square{1, 1}, CostModifiers{}); ok {
		t.Fatal("a wall was enterable")
	}
	if _, ok := g.EntryCost(Square{1, 0}, CostModifiers{IgnoreTerrain: true}); !ok {
		t.Fatal("ignore_terrain must keep swamp passable")
	}
	if cost, _ := g.EntryCost(Square{1, 0}, CostModifiers{IgnoreTerrain: true}); cost != 1 {
		t.Fatal("ignore_terrain: swamp must cost 1")
	}
	if cost, _ := g.EntryCost(Square{1, 0}, CostModifiers{ChosenTerrain: []string{"swamp"}}); cost != 1 {
		t.Fatal("choose_terrain swamp: swamp must cost 1")
	}
	if _, ok := g.EntryCost(Square{1, 1}, CostModifiers{IgnoreTerrain: true}); ok {
		t.Fatal("ignore_terrain must not make walls passable")
	}
	// A cheaper route around the swamp wins.
	g2 := grid(testRules(), ".~.", "...")
	if got := NewPathField(g2, CostModifiers{}, []Square{{2, 0}}, nil, Square{2, 0}).CostTo(Square{0, 0}, 0); got != 2 {
		t.Fatalf("around the swamp costs %d, want 2", got)
	}
	// A goal walled in is unreachable.
	g3 := grid(testRules(), ".#.", ".##", "...")
	if NewPathField(g3, CostModifiers{}, []Square{{2, 0}}, nil, Square{2, 0}).Reachable(Square{0, 0}, 0) {
		t.Fatal("a walled-in square was reachable")
	}
}

func TestAlternatingDiagonalsCostOneThenTwo(t *testing.T) {
	rules := testRules()
	rules.DiagonalRule = DiagonalAlternating
	g := grid(rules, open(6, 6)...)
	if c, _ := g.StepCost(Square{0, 0}, Direction{1, 1}, 0, CostModifiers{}); c != 1 {
		t.Fatalf("first diagonal = %d, want 1", c)
	}
	if c, _ := g.StepCost(Square{1, 1}, Direction{1, 1}, 1, CostModifiers{}); c != 2 {
		t.Fatalf("second diagonal = %d, want 2", c)
	}
	if got := NewPathField(g, CostModifiers{}, []Square{{4, 4}}, nil, Square{4, 4}).CostTo(Square{0, 0}, 0); got != 6 {
		t.Fatalf("four diagonals cost %d, want 6 (1+2+1+2)", got)
	}
	if got := NewPathField(g, CostModifiers{}, []Square{{4, 4}}, nil, Square{4, 4}).CostTo(Square{0, 0}, 1); got != 6 {
		t.Fatalf("after an odd diagonal, four more cost %d, want 6 (2+1+2+1)", got)
	}
	r := Simulate(g, []Mover{mover("a", "x", 0, 0, 3, moveTo(3, 3))}, 1)
	if got := steps(r, "a"); got != "1,1@1/3 2,2@1" {
		t.Fatalf("alternating diagonal steps = %q", got)
	}
	r = Simulate(grid(testRules(), open(6, 6)...), []Mover{mover("a", "x", 0, 0, 3, moveTo(3, 3))}, 1)
	if got := steps(r, "a"); got != "1,1@1/3 2,2@2/3 3,3@1" {
		t.Fatalf("chebyshev diagonal steps = %q", got)
	}
}

// §11.2: walls block; creatures block when configured; permissive corners;
// attacker and target never block themselves.
func TestLineOfSight(t *testing.T) {
	g := grid(testRules(), ".....", "..#..", ".....")
	none := func(Square) bool { return false }
	if g.LineOfSight(Square{0, 1}, Square{4, 1}, none) {
		t.Fatal("a wall did not block")
	}
	if !g.LineOfSight(Square{0, 0}, Square{4, 0}, none) {
		t.Fatal("a clear row was blocked")
	}

	open5 := grid(testRules(), open(5, 5)...)
	creature := func(s Square) bool { return s == Square{2, 0} }
	if open5.LineOfSight(Square{0, 0}, Square{4, 0}, creature) {
		t.Fatal("a creature did not block with los_blocked_by_creatures on")
	}
	rules := testRules()
	rules.LOSBlockedByCreatures = false
	if !grid(rules, open(5, 5)...).LineOfSight(Square{0, 0}, Square{4, 0}, creature) {
		t.Fatal("a creature blocked with los_blocked_by_creatures off")
	}

	// The attacker's and target's own squares never block, even occupied.
	ends := func(s Square) bool {
		return s == Square{0, 0} || s == Square{3, 1} || s == Square{1, 1} || s == Square{1, 0}
	}
	if !open5.LineOfSight(Square{0, 0}, Square{1, 0}, ends) || !open5.LineOfSight(Square{0, 0}, Square{1, 1}, func(s Square) bool { return s == Square{0, 0} || s == Square{1, 1} }) {
		t.Fatal("the attacker's or target's own square blocked")
	}
	if open5.LineOfSight(Square{0, 0}, Square{3, 1}, ends) {
		t.Fatal("a creature between attacker and target did not block")
	}
	everyone := func(Square) bool { return true }
	if !open5.LineOfSight(Square{0, 0}, Square{1, 0}, everyone) {
		t.Fatal("orthogonal neighbours always see each other")
	}

	// A line through a corner: blocked only when both squares there block.
	oneSide := grid(testRules(), ".#...", ".....", ".....")
	if !oneSide.LineOfSight(Square{0, 0}, Square{2, 2}, none) {
		t.Fatal("one wall at a corner blocked (corners are permissive)")
	}
	bothSides := grid(testRules(), ".#...", "#....", ".....")
	if bothSides.LineOfSight(Square{0, 0}, Square{2, 2}, none) || bothSides.LineOfSight(Square{0, 0}, Square{1, 1}, none) {
		t.Fatal("two walls at a corner did not block")
	}
	creatureCorner := func(s Square) bool { return s == Square{1, 0} || s == Square{0, 1} }
	if open5.LineOfSight(Square{0, 0}, Square{2, 2}, creatureCorner) {
		t.Fatal("two creatures at a corner did not block")
	}

	// A shallow line touches every square under it.
	line := SightLine(Square{0, 0}, Square{4, 1})
	var touched []Square
	for _, c := range line {
		touched = append(touched, c.Square)
	}
	want := []Square{{1, 0}, {2, 0}, {2, 1}, {3, 1}}
	if len(touched) != len(want) {
		t.Fatalf("supercover = %v, want %v", touched, want)
	}
	for i := range want {
		if touched[i] != want[i] {
			t.Fatalf("supercover = %v, want %v", touched, want)
		}
	}
}

func TestLineOfSightIsSymmetric(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for trial := 0; trial < 40; trial++ {
		rows := make([]string, 9)
		for y := range rows {
			row := []byte("............")
			for x := range row {
				if random.Intn(6) == 0 {
					row[x] = '#'
				}
			}
			rows[y] = string(row)
		}
		g := grid(testRules(), rows...)
		for i := 0; i < 50; i++ {
			a := Square{random.Intn(12), random.Intn(9)}
			b := Square{random.Intn(12), random.Intn(9)}
			if g.LineOfSight(a, b, nil) != g.LineOfSight(b, a, nil) {
				t.Fatalf("line of sight %v↔%v is not symmetric on %v", a, b, rows)
			}
		}
	}
}
