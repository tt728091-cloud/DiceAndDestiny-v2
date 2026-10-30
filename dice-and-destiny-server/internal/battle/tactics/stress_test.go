package tactics

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// TestRandomRoundsKeepEveryInvariant resolves thousands of random rounds and
// checks the movement rules hold in all of them: opponents never hold the same
// square at the same time, allies never end together, every step is a legal
// neighbour paid for at exactly its cost and rate, nothing passes t = 1 or the
// budget, and the result is deterministic.
func TestRandomRoundsKeepEveryInvariant(t *testing.T) {
	random := rand.New(rand.NewSource(20261029))
	trials := 3000
	if testing.Short() {
		trials = 300
	}
	for trial := 0; trial < trials; trial++ {
		rules := testRules()
		if random.Intn(4) == 0 {
			rules.DiagonalRule = DiagonalAlternating
		}
		rules.CornerCutting = random.Intn(5) == 0
		w, h := 4+random.Intn(9), 3+random.Intn(7)
		rows := make([]string, h)
		for y := range rows {
			row := make([]byte, w)
			for x := range row {
				switch random.Intn(9) {
				case 0:
					row[x] = '#'
				case 1, 2:
					row[x] = '~'
				default:
					row[x] = '.'
				}
			}
			rows[y] = string(row)
		}
		g := grid(rules, rows...)
		var free []Square
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if g.Passable(Square{x, y}) {
					free = append(free, Square{x, y})
				}
			}
		}
		if len(free) < 3 {
			continue
		}
		random.Shuffle(len(free), func(i, j int) { free[i], free[j] = free[j], free[i] })
		count := 2 + random.Intn(min(5, len(free)-1))
		movers := make([]Mover, count)
		for i := range movers {
			team := []string{"heroes", "monsters", "monsters"}[random.Intn(3)]
			m := Mover{ID: fmt.Sprintf("c%d", i), Team: team, Start: free[i], Budget: random.Intn(9), Paid: random.Intn(3)}
			switch random.Intn(5) {
			case 0:
				m.Intention = hold()
			case 1:
				dest := free[random.Intn(len(free))]
				m.Intention = Intention{Kind: IntentMoveTo, Square: &dest}
			case 2, 3:
				m.Intention = pursue(fmt.Sprintf("c%d", random.Intn(count)))
				if m.Intention.Target == m.ID {
					m.Intention = hold()
				}
				m.ContactTarget = m.Intention.Target
			default:
				m.Intention = flee(fmt.Sprintf("c%d", (i+1)%count))
			}
			if random.Intn(6) == 0 {
				m.Costs = CostModifiers{IgnoreTerrain: true}
			} else if random.Intn(6) == 0 {
				m.Costs = CostModifiers{ChosenTerrain: []string{"swamp"}}
			}
			movers[i] = m
		}
		seed := random.Uint64()
		r := Simulate(g, movers, seed)
		context := fmt.Sprintf("trial %d rows %q movers %+v\n%s", trial, rows, movers, dump(r))
		if again := Simulate(g, movers, seed); !reflect.DeepEqual(r, again) {
			t.Fatalf("not deterministic: %s", context)
		}
		checkInvariants(t, g, movers, r, context)
	}
}

func checkInvariants(t *testing.T, g Grid, movers []Mover, r Result, context string) {
	t.Helper()
	byID := map[string]Mover{}
	for _, m := range movers {
		byID[m.ID] = m
	}
	// Steps: adjacent, passable, corner rule, exact cost, time and budget.
	for _, m := range movers {
		at, spent, diagonals := Zero, 0, 0
		pos := m.Start
		for _, e := range entries(r, EntryStep, m.ID) {
			if *e.From != pos || !Adjacent(pos, *e.To) {
				t.Fatalf("%s stepped %v→%v from %v: %s", m.ID, *e.From, *e.To, pos, context)
			}
			d := Direction{e.To.X - pos.X, e.To.Y - pos.Y}
			cost, ok := g.StepCost(pos, d, diagonals, m.Costs)
			if !ok || cost != e.Cost {
				t.Fatalf("%s step %v→%v cost %d, want %d (%v): %s", m.ID, pos, *e.To, e.Cost, cost, ok, context)
			}
			earliest := at.Add(F(int64(cost), int64(m.Budget)))
			if e.At.Less(earliest) {
				t.Fatalf("%s entered %v at %s, before %s allows: %s", m.ID, *e.To, e.At, earliest, context)
			}
			if One.Less(e.At) {
				t.Fatalf("%s moved after t=1: %s", m.ID, context)
			}
			spent += cost
			if d.Diagonal() {
				diagonals++
			}
			at, pos = e.At, *e.To
		}
		if spent > m.Budget || spent != r.Spent[m.ID] || pos != r.Final[m.ID] {
			t.Fatalf("%s spent %d of %d (reported %d), ended %v (reported %v): %s", m.ID, spent, m.Budget, r.Spent[m.ID], pos, r.Final[m.ID], context)
		}
		if (m.Intention.Kind == IntentHold || m.Budget == 0) && pos != m.Start {
			t.Fatalf("%s held but moved: %s", m.ID, context)
		}
	}
	// Opponents never share a square at the same moment.
	type window struct {
		s          Square
		start, end Frac
		final      bool
	}
	windows := map[string][]window{}
	for id, path := range r.Paths {
		for i, wp := range path {
			w := window{s: wp.Square, start: wp.At, end: One, final: i == len(path)-1}
			if i+1 < len(path) {
				w.end = path[i+1].At
			}
			windows[id] = append(windows[id], w)
		}
	}
	for a, wa := range windows {
		for b, wb := range windows {
			if a >= b || !Opponents(byID[a].Team, byID[b].Team) {
				continue
			}
			for _, x := range wa {
				for _, y := range wb {
					if x.s != y.s {
						continue
					}
					// Half-open [start, end); the final square is held through t = 1.
					overlap := x.start.Less(y.end) && y.start.Less(x.end)
					if x.final && y.final {
						overlap = true
					}
					if overlap {
						t.Fatalf("opponents %s and %s both held %v (%s–%s and %s–%s): %s", a, b, x.s, x.start, x.end, y.start, y.end, context)
					}
				}
			}
		}
	}
	// Nobody ends on another creature's square.
	seen := map[Square]string{}
	for id, s := range r.Final {
		if other, dup := seen[s]; dup {
			t.Fatalf("%s and %s both ended on %v: %s", id, other, s, context)
		}
		seen[s] = id
	}
}
