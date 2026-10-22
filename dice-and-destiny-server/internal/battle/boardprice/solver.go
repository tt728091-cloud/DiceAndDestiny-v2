package boardprice

import (
	"math"
	"sync"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// sides is the faces of every board die; faces are numbered 1–6 in face order.
const sides = 6

// shareTolerance treats two expected values this close as equal: tied tiers
// share their use and tied keeps prefer keeping more dice.
const shareTolerance = 1e-12

type multiset [sides]int8

// handTable indexes every multiset of up to `dice` dice over faces 1–6, with
// the outcomes of rerolling the rest and every distinct keep of each hand.
type handTable struct {
	dice   int
	sets   []multiset
	size   []int
	index  map[multiset]int
	full   []int       // hands of all the dice
	weight []float64   // chance of each hand in one roll of all the dice (by set index)
	outs   [][]outcome // by number of dice rolled
	join   [][]int     // by kept set: the hand each outcome of the rest makes
	keeps  [][]int     // by hand: every distinct kept set, the hand itself included
}

type outcome struct {
	set  int
	prob float64
}

var (
	tablesMu sync.Mutex
	tables   = map[int]*handTable{}
)

func handsFor(dice int) *handTable {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	if t := tables[dice]; t != nil {
		return t
	}
	t := newHandTable(dice)
	tables[dice] = t
	return t
}

func newHandTable(dice int) *handTable {
	t := &handTable{dice: dice, index: map[multiset]int{}}
	var counts multiset
	var visit func(face, left int)
	visit = func(face, left int) {
		if face == sides {
			total := 0
			for _, c := range counts {
				total += int(c)
			}
			t.index[counts] = len(t.sets)
			t.sets = append(t.sets, counts)
			t.size = append(t.size, total)
			return
		}
		for c := 0; c <= left; c++ {
			counts[face] = int8(c)
			visit(face+1, left-c)
		}
		counts[face] = 0
	}
	visit(0, dice)
	t.outs = make([][]outcome, dice+1)
	for k := 0; k <= dice; k++ {
		for i, s := range t.sets {
			if t.size[i] == k {
				t.outs[k] = append(t.outs[k], outcome{set: i, prob: arrangements(s) / math.Pow(sides, float64(k))})
			}
		}
	}
	t.weight = make([]float64, len(t.sets))
	for _, o := range t.outs[dice] {
		t.full = append(t.full, o.set)
		t.weight[o.set] = o.prob
	}
	t.join = make([][]int, len(t.sets))
	for i, kept := range t.sets {
		rest := dice - t.size[i]
		for _, o := range t.outs[rest] {
			var sum multiset
			for f := range sum {
				sum[f] = kept[f] + t.sets[o.set][f]
			}
			t.join[i] = append(t.join[i], t.index[sum])
		}
	}
	t.keeps = make([][]int, len(t.sets))
	for _, hand := range t.full {
		h := t.sets[hand]
		for i, sub := range t.sets {
			inside := true
			for f := range sub {
				inside = inside && sub[f] <= h[f]
			}
			if inside {
				t.keeps[hand] = append(t.keeps[hand], i)
			}
		}
	}
	return t
}

// arrangements is how many ordered rolls make the multiset.
func arrangements(s multiset) float64 {
	n, out := 0, 1.0
	for _, c := range s {
		for i := 1; i <= int(c); i++ {
			n++
			out *= float64(n) / float64(i)
		}
	}
	return out
}

// faces lists the hand's faces in ascending order.
func (t *handTable) faces(set int) []int {
	var out []int
	for f, c := range t.sets[set] {
		for n := 0; n < int(c); n++ {
			out = append(out, f+1)
		}
	}
	return out
}

// solver plays one board on one die: which tiers each hand qualifies for.
type solver struct {
	hands   *handTable
	qualify map[int][]bool // by hand: tier qualified
}

func newSolver(die Die, tiers []Tier, dice int) *solver {
	s := &solver{hands: handsFor(dice), qualify: map[int][]bool{}}
	abilities := make([]content.BattleAbilityDefinition, len(tiers))
	for i, tier := range tiers {
		abilities[i] = content.BattleAbilityDefinition{Type: "offensive", Qualification: &content.AbilityQualification{ActivationTiers: []content.AbilityTier{{ID: tier.Key(), Requirements: tier.Requirements(die.Layout)}}}}
	}
	for _, hand := range s.hands.full {
		rolled := make([]state.RolledDie, 0, dice)
		for i, face := range s.hands.faces(hand) {
			rolled = append(rolled, state.RolledDie{Index: i, DieID: die.ID, Face: face, Value: face, Symbols: []string{die.symbol(face)}})
		}
		row := make([]bool, len(tiers))
		for i, ability := range abilities {
			_, row[i] = engine.QualifiedAbilityTier(ability, rolled)
		}
		s.qualify[hand] = row
	}
	return s
}

// play is optimal play of the board with these tier values: after every
// roll keep the dice that maximise the expected value of the best tier the
// turn ends on (ties keep more dice), then use the most valuable qualified
// tier (tied tiers share the use). It returns the expected value, each tier's
// use share, the no-hit share and, when asked, the keep after each roll
// (policy[k] is the keep with k rerolls left).
func (s *solver) play(values []float64, rolls int, withPolicy bool) (float64, []float64, float64, []map[int]int) {
	h := s.hands
	stop := make([]float64, len(h.sets))
	tops := map[int][]int{}
	for _, hand := range h.full {
		best, top := 0.0, []int(nil)
		for i, ok := range s.qualify[hand] {
			if !ok {
				continue
			}
			switch {
			case top == nil || values[i] > best+shareTolerance:
				best, top = values[i], []int{i}
			case math.Abs(values[i]-best) <= shareTolerance:
				top = append(top, i)
			}
		}
		stop[hand], tops[hand] = best, top
	}
	value := stop
	policy := make([]map[int]int, rolls)
	expected := make([]float64, len(h.sets))
	for k := 1; k < rolls; k++ {
		for i := range h.sets {
			if h.size[i] == h.dice {
				expected[i] = value[i]
				continue
			}
			sum := 0.0
			for o, next := range h.join[i] {
				sum += h.outs[h.dice-h.size[i]][o].prob * value[next]
			}
			expected[i] = sum
		}
		next := make([]float64, len(h.sets))
		choice := map[int]int{}
		for _, hand := range h.full {
			best, keep := math.Inf(-1), -1
			for _, kept := range h.keeps[hand] {
				e := expected[kept]
				if e > best+shareTolerance || (math.Abs(e-best) <= shareTolerance && h.size[kept] > h.size[keep]) {
					best, keep = e, kept
				}
			}
			next[hand], choice[hand] = best, keep
		}
		value, policy[k] = next, choice
	}
	dist := make([]float64, len(h.sets))
	for _, hand := range h.full {
		dist[hand] = h.weight[hand]
	}
	for k := rolls - 1; k >= 1; k-- {
		moved := make([]float64, len(h.sets))
		for _, hand := range h.full {
			if dist[hand] == 0 {
				continue
			}
			kept := policy[k][hand]
			for o, next := range h.join[kept] {
				moved[next] += dist[hand] * h.outs[h.dice-h.size[kept]][o].prob
			}
		}
		dist = moved
	}
	used := make([]float64, len(values))
	ev, noHit := 0.0, 0.0
	for _, hand := range h.full {
		w := dist[hand]
		if len(tops[hand]) == 0 {
			noHit += w
			continue
		}
		ev += w * stop[hand]
		for _, i := range tops[hand] {
			used[i] += w / float64(len(tops[hand]))
		}
	}
	if !withPolicy {
		policy = nil
	}
	return ev, used, noHit, policy
}

// chase is the chance to qualify for one tier when every keep chases it.
func (s *solver) chase(tier, count, rolls int) float64 {
	values := make([]float64, count)
	values[tier] = 1
	only := &solver{hands: s.hands, qualify: map[int][]bool{}}
	for hand, row := range s.qualify {
		only.qualify[hand] = []bool{row[tier]}
	}
	ev, _, _, _ := only.play([]float64{1}, rolls, false)
	return ev
}

// Die is a die ready to price on: its faces by number and its roles.
type Die struct {
	ID     string
	Name   string
	Faces  []content.BattleDieFace
	Layout content.DieLayout
}

// NewDie describes a catalog die by roles. Only dice with roles price.
func NewDie(def content.BattleDieDefinition) (Die, error) {
	layout, err := content.DieRoles(def)
	if err != nil {
		return Die{}, err
	}
	faces := make([]content.BattleDieFace, sides)
	for _, f := range def.Faces {
		faces[f.Number-1] = f
	}
	return Die{ID: def.ID, Name: def.Name, Faces: faces, Layout: layout}, nil
}

func (d Die) symbol(face int) string {
	if face < 1 || face > len(d.Faces) {
		return ""
	}
	return d.Faces[face-1].Symbol
}
