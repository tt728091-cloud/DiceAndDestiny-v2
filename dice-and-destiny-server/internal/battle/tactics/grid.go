package tactics

// Grid is an arena's terrain under one battle's rules.
type Grid struct {
	Rules  Rules
	Arena  Arena
	width  int
	height int
}

// NewGrid pairs an arena with the rules it is played under.
func NewGrid(arena Arena, rules Rules) Grid {
	return Grid{Rules: rules, Arena: arena, width: arena.Width(), height: arena.Height()}
}

func (g Grid) Width() int  { return g.width }
func (g Grid) Height() int { return g.height }

// InBounds reports a square on the grid.
func (g Grid) InBounds(s Square) bool {
	return s.X >= 0 && s.Y >= 0 && s.X < g.width && s.Y < g.height
}

// Terrain is the terrain of an in-bounds square.
func (g Grid) Terrain(s Square) (string, Terrain, bool) {
	if !g.InBounds(s) {
		return "", Terrain{}, false
	}
	id := g.Arena.TerrainAt(s)
	t, ok := g.Rules.Terrain[id]
	return id, t, ok
}

// Passable reports a square that can be entered.
func (g Grid) Passable(s Square) bool {
	_, t, ok := g.Terrain(s)
	return ok && t.Passable
}

// BlocksSight reports terrain that blocks line of sight (walls).
func (g Grid) BlocksSight(s Square) bool {
	_, t, ok := g.Terrain(s)
	return !ok || t.BlocksLineOfSight
}

// CostModifiers change one mover's terrain costs for one round: a map Move
// card with ignore_terrain makes every square cost 1; with choose_terrain one
// named terrain costs 1.
type CostModifiers struct {
	IgnoreTerrain bool     `json:"ignore_terrain,omitempty"`
	ChosenTerrain []string `json:"chosen_terrain,omitempty"`
}

// EntryCost is the Move to enter a passable square for this mover, before
// the diagonal rule; ok is false for impassable squares.
func (g Grid) EntryCost(s Square, mods CostModifiers) (int, bool) {
	id, t, ok := g.Terrain(s)
	if !ok || !t.Passable {
		return 0, false
	}
	if mods.IgnoreTerrain {
		return 1, true
	}
	for _, chosen := range mods.ChosenTerrain {
		if chosen == id {
			return 1, true
		}
	}
	return t.Cost, true
}

// Direction is one of the eight steps, in the fixed order used for every
// deterministic tie: N, NE, E, SE, S, SW, W, NW.
type Direction struct{ DX, DY int }

var Directions = []Direction{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

func (d Direction) Diagonal() bool { return d.DX != 0 && d.DY != 0 }

// Step returns the square one step away in a direction.
func (s Square) Step(d Direction) Square { return Square{s.X + d.DX, s.Y + d.DY} }

// Adjacent reports two different squares that touch, diagonals included.
func Adjacent(a, b Square) bool {
	return a != b && abs(a.X-b.X) <= 1 && abs(a.Y-b.Y) <= 1
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Distance is the grid distance between two squares under the diagonal
// rule: Chebyshev (a diagonal step counts 1) or alternating (diagonal steps
// count 1, 2, 1, 2…).
func Distance(rule string, a, b Square) int {
	dx, dy := abs(a.X-b.X), abs(a.Y-b.Y)
	long, short := dx, dy
	if short > long {
		long, short = short, long
	}
	if rule == DiagonalAlternating {
		return long + short/2
	}
	return long
}

// Distance under this grid's rule.
func (g Grid) Distance(a, b Square) int { return Distance(g.Rules.DiagonalRule, a, b) }

// DiagonalAllowed reports whether a diagonal step may pass the two squares
// beside it: with corner cutting off, neither may be impassable.
func (g Grid) DiagonalAllowed(from Square, d Direction) bool {
	if !d.Diagonal() || g.Rules.CornerCutting {
		return true
	}
	return g.Passable(Square{from.X + d.DX, from.Y}) && g.Passable(Square{from.X, from.Y + d.DY})
}

// StepCost is the Move a step costs this mover, given how many diagonal
// steps it has already taken this round (only the alternating rule cares):
// the entered square's cost, doubled for every second diagonal under the
// alternating rule.
func (g Grid) StepCost(from Square, d Direction, diagonalsTaken int, mods CostModifiers) (int, bool) {
	to := from.Step(d)
	cost, ok := g.EntryCost(to, mods)
	if !ok || !g.DiagonalAllowed(from, d) {
		return 0, false
	}
	if d.Diagonal() && g.Rules.DiagonalRule == DiagonalAlternating && diagonalsTaken%2 == 1 {
		cost *= 2
	}
	return cost, true
}

// SquaresWithin lists the in-bounds squares within a Chebyshev radius of a
// centre (an area's squares), row by row.
func (g Grid) SquaresWithin(centre Square, radius int) []Square {
	var out []Square
	for y := centre.Y - radius; y <= centre.Y+radius; y++ {
		for x := centre.X - radius; x <= centre.X+radius; x++ {
			if s := (Square{x, y}); g.InBounds(s) {
				out = append(out, s)
			}
		}
	}
	return out
}
