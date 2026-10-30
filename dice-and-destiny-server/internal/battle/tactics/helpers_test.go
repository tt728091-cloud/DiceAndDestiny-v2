package tactics

import (
	"fmt"
	"strings"
	"testing"
)

// testRules are the shipped defaults, built in code so the pure tests do not
// depend on the content files.
func testRules() Rules {
	return Rules{
		SchemaVersion: 1, BaseSpeed: 1, CardMove: 1, EnergyPerMove: 5, MapCardMoveMultiplier: 1,
		SprintBaseMultiplier: 2, SprintCardBonus: 1, DiagonalRule: DiagonalChebyshev,
		LOSBlockedByCreatures: true, TiebreakOrder: []string{TiebreakSquaresMoved, TiebreakMoveRate, TiebreakPaidMove, TiebreakRandom},
		RunningStatus: "running", DefaultDelivery: DefaultDelivery{Type: "direct", Range: 1, LineOfSight: true},
		Terrain: map[string]Terrain{
			"floor": {Name: "Floor", Cost: 1, Passable: true, Color: "#000000"},
			"swamp": {Name: "Swamp", Cost: 2, Passable: true, Color: "#000000"},
			"wall":  {Name: "Wall", Passable: false, BlocksLineOfSight: true, Color: "#000000"},
		},
		AI: AIRules{MaxCardsPerRound: 1},
	}
}

// grid builds a grid from rows of '.', '~' (swamp) and '#' (wall).
func grid(rules Rules, rows ...string) Grid {
	arena := Arena{SchemaVersion: 1, ID: "test", Name: "Test", Legend: map[string]string{".": "floor", "~": "swamp", "#": "wall"}, Grid: rows, EnemyStarts: []Square{{0, 0}}}
	return NewGrid(arena, rules)
}

func open(w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = strings.Repeat(".", w)
	}
	return rows
}

func moveTo(x, y int) Intention  { s := Square{x, y}; return Intention{Kind: IntentMoveTo, Square: &s} }
func pursue(id string) Intention { return Intention{Kind: IntentPursue, Target: id} }
func flee(id string) Intention   { return Intention{Kind: IntentFlee, Target: id} }
func hold() Intention            { return Intention{Kind: IntentHold} }

func mover(id, team string, x, y, budget int, intent Intention) Mover {
	return Mover{ID: id, Team: team, Start: Square{x, y}, Budget: budget, Intention: intent}
}

// steps lists one creature's steps as "to@t".
func steps(r Result, id string) string {
	var out []string
	for _, e := range r.Timeline {
		if e.Kind == EntryStep && e.Creature == id {
			out = append(out, fmt.Sprintf("%d,%d@%s", e.To.X, e.To.Y, e.At))
		}
	}
	return strings.Join(out, " ")
}

func entries(r Result, kind, id string) []Entry {
	var out []Entry
	for _, e := range r.Timeline {
		if e.Kind == kind && (id == "" || e.Creature == id) {
			out = append(out, e)
		}
	}
	return out
}

func expectFinal(t *testing.T, r Result, id string, x, y int) {
	t.Helper()
	if got := r.Final[id]; got != (Square{x, y}) {
		t.Fatalf("%s ended at %v, want (%d,%d)\n%s", id, got, x, y, dump(r))
	}
}

func dump(r Result) string {
	var b strings.Builder
	for _, e := range r.Timeline {
		fmt.Fprintf(&b, "  %s %s t=%s", e.Kind, e.Creature, e.At)
		if e.From != nil {
			fmt.Fprintf(&b, " from %v", *e.From)
		}
		if e.To != nil {
			fmt.Fprintf(&b, " to %v", *e.To)
		}
		if e.Square != nil {
			fmt.Fprintf(&b, " at %v", *e.Square)
		}
		if e.Reason != "" {
			fmt.Fprintf(&b, " (%s)", e.Reason)
		}
		if e.Winner != "" {
			fmt.Fprintf(&b, " winner %s losers %v by %s", e.Winner, e.Losers, e.Rule)
		}
		b.WriteString("\n")
	}
	return b.String()
}
