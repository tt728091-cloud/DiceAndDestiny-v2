// Package expedition owns the Expedition map rules: a hex region built tile by
// tile, the Day/Night clock, movement paid with cards and energy, sites,
// enemies, the Warlord's agenda, and the hand-off into and out of battles.
//
// Everything here is pure and deterministic over a serializable State; the
// learned runtime wraps it in ops and stores the State inside the campaign
// save. The Godot client only renders the viewer-safe view and submits actions.
package expedition

import "fmt"

// Hex is an axial coordinate (q, r) on a pointy-top hex grid.
type Hex struct {
	Q int `json:"q"`
	R int `json:"r"`
}

func (h Hex) String() string { return fmt.Sprintf("%d,%d", h.Q, h.R) }

func (h Hex) Add(o Hex) Hex { return Hex{h.Q + o.Q, h.R + o.R} }
func (h Hex) Sub(o Hex) Hex { return Hex{h.Q - o.Q, h.R - o.R} }

// Directions lists the six neighbour offsets, clockwise from east.
var Directions = [6]Hex{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

// FlowerOffsets are the centre offsets of the six tiles around a tile: a tile
// is a seven-hex "flower" (centre plus its six neighbours), and these offsets
// tile the plane with flowers.
var FlowerOffsets = [6]Hex{{2, 1}, {-1, 3}, {-3, 2}, {-2, -1}, {1, -3}, {3, -2}}

// FlowerHexes are the seven hex offsets of a tile relative to its centre.
var FlowerHexes = [7]Hex{{0, 0}, {1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Distance is the hex distance between two axial coordinates.
func Distance(a, b Hex) int {
	d := a.Sub(b)
	return (abs(d.Q) + abs(d.R) + abs(d.Q+d.R)) / 2
}

// Neighbors returns the six adjacent hexes.
func Neighbors(h Hex) [6]Hex {
	var out [6]Hex
	for i, d := range Directions {
		out[i] = h.Add(d)
	}
	return out
}

// Adjacent reports whether two hexes share an edge.
func Adjacent(a, b Hex) bool { return Distance(a, b) == 1 }

// TileHexes lists the seven hexes of a tile placed at centre.
func TileHexes(center Hex) [7]Hex {
	var out [7]Hex
	for i, o := range FlowerHexes {
		out[i] = center.Add(o)
	}
	return out
}

// IsFlowerCenter reports whether c sits on the flower lattice that contains
// the origin: every tile centre must, so tiles never overlap.
func IsFlowerCenter(c Hex) bool {
	// The lattice is spanned by (2,1) and (-1,3). Solve c = a*(2,1) + b*(-1,3):
	// q = 2a - b, r = a + 3b  =>  a = (3q + r)/7, b = (2r - q)/7.
	return (3*c.Q+c.R)%7 == 0 && (2*c.R-c.Q)%7 == 0
}

// Line walks the hexes from a to b (inclusive) along the straight hex line,
// using cube-coordinate rounding. It is deterministic for equal inputs.
func Line(a, b Hex) []Hex {
	n := Distance(a, b)
	if n == 0 {
		return []Hex{a}
	}
	out := make([]Hex, 0, n+1)
	ax, az := float64(a.Q), float64(a.R)
	bx, bz := float64(b.Q), float64(b.R)
	ay, by := -ax-az, -bx-bz
	// A tiny nudge keeps the line off hex edges so rounding is stable.
	bx += 1e-6
	by += 2e-6
	bz -= 3e-6
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		x := ax + (bx-ax)*t
		y := ay + (by-ay)*t
		z := az + (bz-az)*t
		out = append(out, cubeRound(x, y, z))
	}
	return out
}

func cubeRound(x, y, z float64) Hex {
	rx, ry, rz := roundF(x), roundF(y), roundF(z)
	dx, dy, dz := absF(rx-x), absF(ry-y), absF(rz-z)
	if dx > dy && dx > dz {
		rx = -ry - rz
	} else if dy > dz {
		ry = -rx - rz
	} else {
		rz = -rx - ry
	}
	return Hex{int(rx), int(rz)}
}

func roundF(v float64) float64 {
	if v < 0 {
		return float64(int(v - 0.5))
	}
	return float64(int(v + 0.5))
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ParseHex reads a two-element YAML/JSON list.
func hexFromPair(pair []int) (Hex, error) {
	if len(pair) != 2 {
		return Hex{}, fmt.Errorf("a hex is [q, r], not %v", pair)
	}
	return Hex{pair[0], pair[1]}, nil
}
