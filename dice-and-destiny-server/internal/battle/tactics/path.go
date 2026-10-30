package tactics

import "container/heap"

// pathState is a square plus how many diagonal steps have been taken, mod 2
// (only the alternating diagonal rule prices the second diagonal higher).
type pathState struct {
	S      Square
	Parity int
}

// PathField is the cost, in Move, from every square to the nearest goal for
// one mover, with some squares blocked (held by opponents right now). It is
// rebuilt for every decision, so movers always re-path against current
// positions.
type PathField struct {
	grid    Grid
	mods    CostModifiers
	blocked func(Square) bool
	aim     Square
	goals   map[Square]bool
	dist    map[pathState]int
}

// NewPathField computes costs to the goals. Blocked squares can be neither
// entered nor passed; walls are never entered. aim breaks ties between equal
// routes: the step whose square lies nearest the aim (straight-line) wins,
// then the fixed direction order.
func NewPathField(g Grid, mods CostModifiers, goals []Square, blocked func(Square) bool, aim Square) *PathField {
	f := &PathField{grid: g, mods: mods, blocked: blocked, aim: aim, goals: map[Square]bool{}, dist: map[pathState]int{}}
	parities := []int{0}
	if g.Rules.DiagonalRule == DiagonalAlternating {
		parities = []int{0, 1}
	}
	pq := &stateQueue{}
	for _, goal := range goals {
		if !g.Passable(goal) || (blocked != nil && blocked(goal)) {
			continue
		}
		f.goals[goal] = true
		for _, p := range parities {
			st := pathState{goal, p}
			f.dist[st] = 0
			heap.Push(pq, queued{st, 0, pq.next()})
		}
	}
	for pq.Len() > 0 {
		item := heap.Pop(pq).(queued)
		if item.cost != f.dist[item.state] {
			continue
		}
		n := item.state.S
		for _, d := range Directions {
			s := Square{n.X - d.DX, n.Y - d.DY}
			if !g.Passable(s) || (blocked != nil && blocked(s)) {
				continue
			}
			parity := item.state.Parity
			if d.Diagonal() && len(parities) == 2 {
				parity = 1 - parity
			}
			cost, ok := g.StepCost(s, d, parity, mods)
			if !ok {
				continue
			}
			prev := pathState{s, parity}
			total := item.cost + cost
			if old, seen := f.dist[prev]; !seen || total < old {
				f.dist[prev] = total
				heap.Push(pq, queued{prev, total, pq.next()})
			}
		}
	}
	return f
}

// Reachable reports whether a goal can be reached from a square.
func (f *PathField) Reachable(from Square, diagonals int) bool {
	_, ok := f.dist[f.state(from, diagonals)]
	return ok
}

// CostTo is the Move to reach the nearest goal, or -1.
func (f *PathField) CostTo(from Square, diagonals int) int {
	if d, ok := f.dist[f.state(from, diagonals)]; ok {
		return d
	}
	return -1
}

func (f *PathField) state(s Square, diagonals int) pathState {
	if f.grid.Rules.DiagonalRule == DiagonalAlternating {
		return pathState{s, diagonals % 2}
	}
	return pathState{s, 0}
}

// IsGoal reports a goal square.
func (f *PathField) IsGoal(s Square) bool { return f.goals[s] }

// NextStep is the first step of a cheapest route to a goal: its direction,
// square and Move cost. ok is false when no goal can be reached.
func (f *PathField) NextStep(from Square, diagonals int) (Direction, Square, int, bool) {
	best := -1
	var bestDir Direction
	var bestSquare Square
	bestCost, bestAim := 0, 0
	for _, d := range Directions {
		n := from.Step(d)
		if !f.grid.Passable(n) || (f.blocked != nil && f.blocked(n)) {
			continue
		}
		cost, ok := f.grid.StepCost(from, d, diagonals, f.mods)
		if !ok {
			continue
		}
		next := diagonals
		if d.Diagonal() {
			next++
		}
		rest, ok := f.dist[f.state(n, next)]
		if !ok {
			continue
		}
		total := cost + rest
		aim := (n.X-f.aim.X)*(n.X-f.aim.X) + (n.Y-f.aim.Y)*(n.Y-f.aim.Y)
		if best < 0 || total < best || (total == best && aim < bestAim) {
			best, bestDir, bestSquare, bestCost, bestAim = total, d, n, cost, aim
		}
	}
	return bestDir, bestSquare, bestCost, best >= 0
}

// Route follows cheapest steps from a square until a goal, at most maxSteps
// steps; it returns the entered squares and their costs.
func (f *PathField) Route(from Square, diagonals, maxSteps int) ([]Square, []int) {
	var squares []Square
	var costs []int
	at := from
	for step := 0; step < maxSteps && !f.goals[at]; step++ {
		d, n, cost, ok := f.NextStep(at, diagonals)
		if !ok {
			break
		}
		if d.Diagonal() {
			diagonals++
		}
		squares = append(squares, n)
		costs = append(costs, cost)
		at = n
	}
	return squares, costs
}

type queued struct {
	state pathState
	cost  int
	seq   int
}

type stateQueue struct {
	items []queued
	seq   int
}

func (q *stateQueue) next() int { q.seq++; return q.seq }
func (q *stateQueue) Len() int  { return len(q.items) }
func (q *stateQueue) Less(i, j int) bool {
	if q.items[i].cost != q.items[j].cost {
		return q.items[i].cost < q.items[j].cost
	}
	return q.items[i].seq < q.items[j].seq
}
func (q *stateQueue) Swap(i, j int) { q.items[i], q.items[j] = q.items[j], q.items[i] }
func (q *stateQueue) Push(x any)    { q.items = append(q.items, x.(queued)) }
func (q *stateQueue) Pop() any {
	last := q.items[len(q.items)-1]
	q.items = q.items[:len(q.items)-1]
	return last
}
