package tactics

import (
	"encoding/binary"
	"hash/fnv"
	"sort"
)

// Intention kinds of a movement plan.
const (
	IntentMoveTo = "move_to"
	IntentPursue = "pursue"
	IntentFlee   = "flee"
	IntentHold   = "hold"
)

// Intention is what a mover tries to do each step.
type Intention struct {
	Kind string `json:"kind"`
	// Square is a Move to destination.
	Square *Square `json:"square,omitempty"`
	// Target is the creature to Pursue or Flee from.
	Target string `json:"target,omitempty"`
}

// Mover is one creature in a round's movement. Every living creature is a
// mover; a creature that stays put holds its square all round.
type Mover struct {
	ID    string
	Team  string
	Start Square
	// Budget is the Move paid for this round; it is also the creature's Move
	// rate: entering a square of cost c takes c/Budget of the round.
	Budget int
	// Paid is the part of the budget paid with cards and energy (a tiebreak).
	Paid      int
	Intention Intention
	Costs     CostModifiers
	// ContactTarget records the first moment the mover is adjacent to that
	// creature (a Charge connects then).
	ContactTarget string
}

// Opponents reports creatures on different teams; a creature without a team
// opposes everyone.
func Opponents(teamA, teamB string) bool {
	return teamA == "" || teamB == "" || teamA != teamB
}

// Timeline entry kinds.
const (
	EntryStep     = "step"
	EntryWait     = "wait"
	EntryTiebreak = "tiebreak"
	EntryContact  = "contact"
	EntryStop     = "stop"
)

// Wait and stop reasons.
const (
	ReasonBlocked      = "blocked"
	ReasonNoRoute      = "no_route"
	ReasonAdjacent     = "adjacent"
	ReasonNoEscape     = "no_escape"
	ReasonLostTiebreak = "lost_tiebreak"
	ReasonSwap         = "swap"
	ReasonArrived      = "arrived"
	ReasonBudget       = "budget"
	ReasonTime         = "time"
	ReasonAllyEnd      = "ally_square"
)

// Entry is one record of the resolved movement: a step (creature, from, to,
// time), a wait, a tiebreak (square, winner, losers and the rule that
// decided), a charge contact or a stop.
type Entry struct {
	Kind     string   `json:"kind"`
	Creature string   `json:"creature,omitempty"`
	At       Frac     `json:"t"`
	From     *Square  `json:"from,omitempty"`
	To       *Square  `json:"to,omitempty"`
	Cost     int      `json:"cost,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Square   *Square  `json:"square,omitempty"`
	Winner   string   `json:"winner,omitempty"`
	Losers   []string `json:"losers,omitempty"`
	Rule     string   `json:"rule,omitempty"`
	Target   string   `json:"target,omitempty"`
}

// Waypoint is a square a creature holds from a moment on.
type Waypoint struct {
	Square Square `json:"square"`
	At     Frac   `json:"t"`
}

// Result is a resolved round of movement.
type Result struct {
	Timeline []Entry               `json:"timeline"`
	Final    map[string]Square     `json:"final"`
	Paths    map[string][]Waypoint `json:"paths"`
	Spent    map[string]int        `json:"spent"`
	Moved    map[string]int        `json:"moved"`
	// Contact is when each mover with a ContactTarget first touched it.
	Contact map[string]Frac `json:"contact,omitempty"`
}

type moverState struct {
	m          Mover
	pos        Square
	spent      int
	diagonals  int
	moved      int
	stopped    bool
	stopAfter  int
	attempt    bool
	attemptTo  Square
	attemptDir Direction
	cost       int
	attemptAt  Frac
	decide     bool
	readyAt    Frac
	waiting    string
	enteredAt  Frac
	path       []Waypoint
}

// Simulate resolves one round of simultaneous movement. Each mover decides
// its next square from its intention against the positions at that moment
// and enters it c/M later; every attempt due at the same moment is resolved
// together: an opponent still holding the square blocks it, opponents
// entering the same square at the same moment are separated by the tiebreak
// chain (seeded random last), and opponents swapping squares head-on both
// wait. Allies pass through each other but never end on the same square: a
// mover that would is stopped at the last free square of its path and the
// round is resolved again. The same movers and seed give the same result.
func Simulate(g Grid, movers []Mover, seed uint64) Result {
	caps := map[string]int{}
	var result Result
	for iteration := 0; iteration < 64; iteration++ {
		var shared map[string]int
		result, shared = simulateOnce(g, movers, seed, caps)
		changed := false
		for id, limit := range shared {
			if old, ok := caps[id]; !ok || limit < old {
				caps[id] = limit
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return result
}

// Preview resolves one mover's plan as if every other creature held its
// square all round (the movement step's path preview).
func Preview(g Grid, movers []Mover, id string, seed uint64) Result {
	held := make([]Mover, len(movers))
	for i, m := range movers {
		if m.ID != id {
			m.Intention = Intention{Kind: IntentHold}
			m.ContactTarget = ""
		}
		held[i] = m
	}
	return Simulate(g, held, seed)
}

func simulateOnce(g Grid, movers []Mover, seed uint64, caps map[string]int) (Result, map[string]int) {
	states := make([]*moverState, 0, len(movers))
	byID := map[string]*moverState{}
	for _, m := range movers {
		st := &moverState{m: m, pos: m.Start, stopAfter: -1, enteredAt: Zero, readyAt: Zero, attemptAt: Zero, path: []Waypoint{{Square: m.Start, At: Zero}}}
		if limit, ok := caps[m.ID]; ok {
			st.stopAfter = limit
		}
		states = append(states, st)
		byID[m.ID] = st
	}
	sort.Slice(states, func(i, j int) bool { return states[i].m.ID < states[j].m.ID })
	sim := &simulation{g: g, states: states, byID: byID, seed: seed, contact: map[string]Frac{}}
	for _, st := range states {
		if st.m.Intention.Kind == IntentHold || st.m.Intention.Kind == "" || st.m.Budget <= 0 {
			st.stopped = true
			continue
		}
		st.decide, st.readyAt = true, Zero
	}
	sim.checkContacts(Zero)
	for {
		at, ok := sim.nextTime()
		if !ok || One.Less(at) {
			break
		}
		var attempts []*moverState
		for _, st := range states {
			if !st.stopped && st.attempt && st.attemptAt.Equal(at) {
				attempts = append(attempts, st)
			}
		}
		if len(attempts) > 0 {
			sim.resolve(at, attempts)
			sim.checkContacts(at)
		}
		for _, st := range states {
			if !st.stopped && st.decide && st.readyAt.Equal(at) {
				sim.decideNext(st, at)
			}
		}
	}
	result := Result{Timeline: sim.timeline, Final: map[string]Square{}, Paths: map[string][]Waypoint{}, Spent: map[string]int{}, Moved: map[string]int{}, Contact: sim.contact}
	for _, st := range states {
		result.Final[st.m.ID] = st.pos
		result.Paths[st.m.ID] = st.path
		result.Spent[st.m.ID] = st.spent
		result.Moved[st.m.ID] = st.moved
	}
	return result, sim.allySquareLimits()
}

type simulation struct {
	g        Grid
	states   []*moverState
	byID     map[string]*moverState
	seed     uint64
	timeline []Entry
	contact  map[string]Frac
}

func (s *simulation) nextTime() (Frac, bool) {
	var best Frac
	found := false
	for _, st := range s.states {
		if st.stopped {
			continue
		}
		var at Frac
		switch {
		case st.attempt:
			at = st.attemptAt
		case st.decide:
			at = st.readyAt
		default:
			continue
		}
		if !found || at.Less(best) {
			best, found = at, true
		}
	}
	return best, found
}

// opponentHeld reports squares held right now by the mover's opponents.
func (s *simulation) opponentHeld(mover *moverState) func(Square) bool {
	held := map[Square]bool{}
	for _, other := range s.states {
		if other != mover && Opponents(other.m.Team, mover.m.Team) {
			held[other.pos] = true
		}
	}
	return func(q Square) bool { return held[q] }
}

func (s *simulation) heldByAnyone(except *moverState, q Square) bool {
	for _, other := range s.states {
		if other != except && other.pos == q {
			return true
		}
	}
	return false
}

func (s *simulation) stop(st *moverState, at Frac, reason string) {
	st.stopped, st.decide, st.attempt = true, false, false
	s.timeline = append(s.timeline, Entry{Kind: EntryStop, Creature: st.m.ID, At: at, Reason: reason, Square: sq(st.pos)})
}

func (s *simulation) wait(st *moverState, at Frac, reason string) {
	if st.waiting != reason {
		s.timeline = append(s.timeline, Entry{Kind: EntryWait, Creature: st.m.ID, At: at, Reason: reason, Square: sq(st.pos)})
		st.waiting = reason
	}
	next := at.Add(F(1, int64(st.m.Budget)))
	if One.Less(next) {
		s.stop(st, at, ReasonTime)
		return
	}
	st.decide, st.readyAt = true, next
}

// decideNext picks the mover's next square from its intention, against the
// positions at this moment, and schedules entering it c/M later.
func (s *simulation) decideNext(st *moverState, at Frac) {
	st.decide = false
	if st.stopAfter >= 0 && st.moved >= st.stopAfter {
		s.stop(st, at, ReasonAllyEnd)
		return
	}
	blocked := s.opponentHeld(st)
	var dir Direction
	var next Square
	var cost int
	switch st.m.Intention.Kind {
	case IntentMoveTo:
		goal := *st.m.Intention.Square
		if st.pos == goal {
			s.stop(st, at, ReasonArrived)
			return
		}
		field := NewPathField(s.g, st.m.Costs, []Square{goal}, blocked, goal)
		var ok bool
		if dir, next, cost, ok = field.NextStep(st.pos, st.diagonals); !ok {
			s.wait(st, at, ReasonNoRoute)
			return
		}
	case IntentPursue:
		target, ok := s.byID[st.m.Intention.Target]
		if !ok {
			s.stop(st, at, ReasonNoRoute)
			return
		}
		if Adjacent(st.pos, target.pos) {
			s.wait(st, at, ReasonAdjacent)
			return
		}
		var goals []Square
		for _, d := range Directions {
			q := target.pos.Step(d)
			if s.g.Passable(q) && !s.heldByAnyone(st, q) {
				goals = append(goals, q)
			}
		}
		field := NewPathField(s.g, st.m.Costs, goals, blocked, target.pos)
		if dir, next, cost, ok = field.NextStep(st.pos, st.diagonals); !ok {
			s.wait(st, at, ReasonNoRoute)
			return
		}
	case IntentFlee:
		threat, ok := s.byID[st.m.Intention.Target]
		if !ok {
			s.stop(st, at, ReasonNoEscape)
			return
		}
		current := s.g.Distance(st.pos, threat.pos)
		bestDistance, bestCost, found, increasing := 0, 0, false, false
		for _, d := range Directions {
			q := st.pos.Step(d)
			if blocked(q) {
				continue
			}
			c, ok := s.g.StepCost(st.pos, d, st.diagonals, st.m.Costs)
			if !ok {
				continue
			}
			distance := s.g.Distance(q, threat.pos)
			if distance <= current {
				continue
			}
			increasing = true
			if st.spent+c > st.m.Budget || One.Less(at.Add(F(int64(c), int64(st.m.Budget)))) {
				continue
			}
			if !found || distance > bestDistance || (distance == bestDistance && c < bestCost) {
				bestDistance, bestCost, found = distance, c, true
				dir, next, cost = d, q, c
			}
		}
		if !found {
			if increasing {
				s.stop(st, at, ReasonBudget)
				return
			}
			s.wait(st, at, ReasonNoEscape)
			return
		}
	default:
		s.stop(st, at, ReasonArrived)
		return
	}
	if st.spent+cost > st.m.Budget {
		s.stop(st, at, ReasonBudget)
		return
	}
	enter := at.Add(F(int64(cost), int64(st.m.Budget)))
	if One.Less(enter) {
		s.stop(st, at, ReasonTime)
		return
	}
	st.attempt, st.attemptTo, st.attemptDir, st.cost, st.attemptAt = true, next, dir, cost, enter
}

// resolve settles every step due at one moment.
func (s *simulation) resolve(at Frac, attempts []*moverState) {
	failed := map[*moverState]string{}
	for i, a := range attempts {
		for _, b := range attempts[i+1:] {
			if a.attemptTo == b.pos && b.attemptTo == a.pos && Opponents(a.m.Team, b.m.Team) {
				failed[a], failed[b] = ReasonSwap, ReasonSwap
			}
		}
	}
	attempting := map[*moverState]bool{}
	for _, a := range attempts {
		attempting[a] = true
	}
	recorded := map[Square]bool{}
	for changed := true; changed; {
		changed = false
		// An opponent still holding the square blocks it, unless it leaves
		// at this same moment.
		for _, a := range attempts {
			if failed[a] != "" {
				continue
			}
			for _, h := range s.states {
				if h == a || h.pos != a.attemptTo || !Opponents(h.m.Team, a.m.Team) {
					continue
				}
				if !attempting[h] || failed[h] != "" {
					failed[a] = ReasonBlocked
					changed = true
					break
				}
			}
		}
		// Opponents entering the same square at the same moment.
		groups := map[Square][]*moverState{}
		var squares []Square
		for _, a := range attempts {
			if failed[a] != "" {
				continue
			}
			if _, ok := groups[a.attemptTo]; !ok {
				squares = append(squares, a.attemptTo)
			}
			groups[a.attemptTo] = append(groups[a.attemptTo], a)
		}
		sort.Slice(squares, func(i, j int) bool {
			return squares[i].Y < squares[j].Y || (squares[i].Y == squares[j].Y && squares[i].X < squares[j].X)
		})
		for _, q := range squares {
			group := groups[q]
			if !anyOpponents(group) {
				continue
			}
			winner, runnerUp, rule := s.tiebreak(group, at, q)
			var losers []string
			for _, g := range group {
				if Opponents(g.m.Team, winner.m.Team) {
					failed[g] = ReasonLostTiebreak
					losers = append(losers, g.m.ID)
					changed = true
				}
			}
			_ = runnerUp
			if !recorded[q] {
				recorded[q] = true
				s.timeline = append(s.timeline, Entry{Kind: EntryTiebreak, At: at, Square: sq(q), Winner: winner.m.ID, Losers: losers, Rule: rule})
			}
		}
	}
	for _, a := range attempts {
		a.attempt = false
		a.decide, a.readyAt = true, at
		if reason := failed[a]; reason != "" {
			s.timeline = append(s.timeline, Entry{Kind: EntryWait, Creature: a.m.ID, At: at, Reason: reason, Square: sq(a.pos), To: sq(a.attemptTo)})
			a.waiting = reason
			continue
		}
		from := a.pos
		a.pos = a.attemptTo
		a.spent += a.cost
		a.moved++
		if a.attemptDir.Diagonal() {
			a.diagonals++
		}
		a.enteredAt = at
		a.waiting = ""
		a.path = append(a.path, Waypoint{Square: a.pos, At: at})
		s.timeline = append(s.timeline, Entry{Kind: EntryStep, Creature: a.m.ID, At: at, From: sq(from), To: sq(a.pos), Cost: a.cost})
	}
}

func anyOpponents(group []*moverState) bool {
	for i, a := range group {
		for _, b := range group[i+1:] {
			if Opponents(a.m.Team, b.m.Team) {
				return true
			}
		}
	}
	return false
}

// tiebreak ranks movers entering the same square at the same moment by the
// configured chain (squares moved so far, Move rate, Move paid with cards
// and energy, then the seeded random), returning the winner, the best of its
// opponents and the rule that separated them.
func (s *simulation) tiebreak(group []*moverState, at Frac, q Square) (*moverState, *moverState, string) {
	value := func(st *moverState, rule string) uint64 {
		switch rule {
		case TiebreakSquaresMoved:
			return uint64(st.moved)
		case TiebreakMoveRate:
			return uint64(st.m.Budget)
		case TiebreakPaidMove:
			return uint64(st.m.Paid)
		}
		return tiebreakRoll(s.seed, at, q, st.m.ID)
	}
	ranked := append([]*moverState(nil), group...)
	sort.SliceStable(ranked, func(i, j int) bool {
		for _, rule := range s.g.Rules.TiebreakOrder {
			a, b := value(ranked[i], rule), value(ranked[j], rule)
			if a != b {
				return a > b
			}
		}
		return ranked[i].m.ID < ranked[j].m.ID
	})
	winner := ranked[0]
	var runnerUp *moverState
	for _, st := range ranked[1:] {
		if Opponents(st.m.Team, winner.m.Team) {
			runnerUp = st
			break
		}
	}
	rule := TiebreakRandom
	for _, r := range s.g.Rules.TiebreakOrder {
		if value(winner, r) != value(runnerUp, r) {
			rule = r
			break
		}
	}
	return winner, runnerUp, rule
}

// tiebreakRoll is the seeded random tiebreak: a hash of the round's seed,
// the moment, the square and the creature, so resolving the round again
// gives the same answer.
func tiebreakRoll(seed uint64, at Frac, q Square, id string) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for _, v := range []uint64{seed, uint64(at.Num), uint64(at.Den), uint64(int64(q.X)), uint64(int64(q.Y))} {
		binary.LittleEndian.PutUint64(buf[:], v)
		h.Write(buf[:])
	}
	h.Write([]byte(id))
	return h.Sum64()
}

func (s *simulation) checkContacts(at Frac) {
	for _, st := range s.states {
		if st.m.ContactTarget == "" {
			continue
		}
		if _, done := s.contact[st.m.ID]; done {
			continue
		}
		target, ok := s.byID[st.m.ContactTarget]
		if ok && Adjacent(st.pos, target.pos) {
			s.contact[st.m.ID] = at
			s.timeline = append(s.timeline, Entry{Kind: EntryContact, Creature: st.m.ID, Target: target.m.ID, At: at, Square: sq(st.pos)})
		}
	}
}

// allySquareLimits finds allies ending on the same square. The one that
// entered it first keeps it; each other stops at the last square of its own
// path that nobody holds at the end. It returns those step limits.
func (s *simulation) allySquareLimits() map[string]int {
	limits := map[string]int{}
	bySquare := map[Square][]*moverState{}
	for _, st := range s.states {
		bySquare[st.pos] = append(bySquare[st.pos], st)
	}
	for _, group := range bySquare {
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			if !group[i].enteredAt.Equal(group[j].enteredAt) {
				return group[i].enteredAt.Less(group[j].enteredAt)
			}
			return group[i].m.ID < group[j].m.ID
		})
		for _, st := range group[1:] {
			limit := 0
			for k := len(st.path) - 2; k >= 0; k-- {
				if !s.heldByAnyone(st, st.path[k].Square) {
					limit = k
					break
				}
			}
			if limit >= st.moved {
				limit = st.moved - 1
			}
			if limit < 0 {
				limit = 0
			}
			limits[st.m.ID] = limit
		}
	}
	return limits
}

func sq(s Square) *Square { return &s }
