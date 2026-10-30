package tactics

// SightCrossing is one place a line of sight passes: a square it runs
// through, or a corner it passes exactly, touching the two squares beside it.
type SightCrossing struct {
	Square Square   `json:"square"`
	Corner []Square `json:"corner,omitempty"`
}

// SightLine lists, in order, every square a line from the centre of one
// square to the centre of another touches (a supercover line), excluding the
// two end squares. Where the line passes exactly through a grid corner, the
// two squares meeting there are listed as that corner instead.
func SightLine(from, to Square) []SightCrossing {
	dx, dy := abs(to.X-from.X), abs(to.Y-from.Y)
	sx, sy := sign(to.X-from.X), sign(to.Y-from.Y)
	var out []SightCrossing
	x, y := from.X, from.Y
	for nx, ny := 0, 0; nx < dx || ny < dy; {
		// The line meets the next vertical grid line at (0.5+nx)/dx of the way
		// and the next horizontal one at (0.5+ny)/dy; compare exactly.
		cross := int64(1+2*nx)*int64(dy) - int64(1+2*ny)*int64(dx)
		switch {
		case cross == 0:
			out = append(out, SightCrossing{Corner: []Square{{x + sx, y}, {x, y + sy}}})
			x, y = x+sx, y+sy
			nx, ny = nx+1, ny+1
		case cross < 0:
			x += sx
			nx++
		default:
			y += sy
			ny++
		}
		if (Square{x, y}) != to {
			out = append(out, SightCrossing{Square: Square{x, y}})
		}
	}
	return out
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// LineOfSight reports whether nothing blocks the line between two square
// centres. Walls block; occupied reports other creatures' squares, which
// block when the rules say so. The two end squares never block (an attacker
// and its target never block themselves), and a corner the line passes
// exactly is blocked only if both squares at it block.
func (g Grid) LineOfSight(from, to Square, occupied func(Square) bool) bool {
	blocks := func(s Square) bool {
		if g.BlocksSight(s) {
			return true
		}
		return g.Rules.LOSBlockedByCreatures && occupied != nil && occupied(s)
	}
	for _, c := range SightLine(from, to) {
		if len(c.Corner) == 2 {
			if blocks(c.Corner[0]) && blocks(c.Corner[1]) {
				return false
			}
			continue
		}
		if blocks(c.Square) {
			return false
		}
	}
	return true
}
