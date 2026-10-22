package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// Paired mode measures riders without a network: at the start of a round's
// offensive planning it forks the battle into identically seeded copies,
// adds one rider to a copy, and plays every copy to the end with the same
// players. A rider's worth is its mean change in result divided by the
// change for one more card removed from the opponent (the damage unit).
//
// The copies share their seeds, so they roll the same dice until the
// rider changes what happens; the pairing removes most of the luck.

// pairedSample is one position: the result of each change's futures minus
// the unchanged futures', summed over the futures.
type pairedSample struct {
	Matchup string             `json:"matchup"`
	Round   int                `json:"round"`
	Health  float64            `json:"own_health_share"`
	Futures int                `json:"futures"`
	Base    float64            `json:"base_sum"`
	Deltas  map[string]float64 `json:"delta_sums"`
}

type pairedConfig struct {
	futures int
	every   int // probe every n-th round start of the hero
	player  string
}

// pairedProber plays the futures for one runner.
type pairedProber struct {
	cfg     pairedConfig
	engine  engine.Engine
	sim     *lookahead.Simulator
	hero    lookahead.Policy
	foe     func() lookahead.Policy
	foeP    lookahead.Policy
	collect *pairedCollector
	matchup string
	seed    uint64
	seat    string
	starts  int
	round   int
}

type pairedCollector struct {
	mu      sync.Mutex
	samples []pairedSample
}

func (p *pairedProber) Begin(seed uint64, heroSeat string) {
	p.seed, p.seat, p.starts, p.round = seed, heroSeat, 0, 0
}

func (p *pairedProber) Finish(*state.Battle, lookahead.BattleRecord, lookahead.Policy, lookahead.Policy) {
}

// changes lists the riders tried at a position, in a fixed order.
func pairedChanges(b *state.Battle, seat, foe string, eng engine.Engine) ([]string, map[string]func(*state.Battle) bool) {
	lib, err := engine.SettledLibrary(b)
	if err != nil {
		fail(err)
	}
	fns := map[string]func(*state.Battle) bool{
		"unit": func(c *state.Battle) bool {
			return removeOne(c, foe, rand.New(rand.NewSource(int64(c.Random.Seed>>1))))
		},
		"+5 energy": func(c *state.Battle) bool { engine.GainEnergy(c, seat, 5); return true },
		"+1 card in hand (from the draw pile)": func(c *state.Battle) bool {
			drawn, err := eng.DrawCard(c, seat)
			return err == nil && drawn != ""
		},
	}
	names := []string{"unit", "+5 energy", "+1 card in hand (from the draw pile)"}
	markers := preparationMarkers(lib)
	ids := make([]string, 0, len(lib.Statuses))
	for id := range lib.Statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		def := lib.Statuses[id]
		if def.Modifiers.MapOnly() || markers[id] {
			continue
		}
		target, label := foe, "enemy +1 "+def.Name
		if def.Polarity == "positive" {
			target, label = seat, "self +1 "+def.Name
		}
		label += " (" + id + ")"
		id := id
		fns[label] = func(c *state.Battle) bool { return engine.ApplyStatus(c, target, id, 1) == nil }
		names = append(names, label)
	}
	return names, fns
}

func (p *pairedProber) Decision(d *lookahead.Decision) {
	b := d.Battle
	if d.Seat != p.seat || b.Segment.Round == p.round || !lookahead.PlanningStart(b) {
		return
	}
	p.round = b.Segment.Round
	p.starts++
	if (p.starts-1)%p.cfg.every != 0 {
		return
	}
	foes := lookahead.Opponents(b, d.Seat)
	if len(foes) == 0 {
		return
	}
	names, fns := pairedChanges(b, d.Seat, foes[0], p.engine)
	own := b.Actors[d.Seat]
	s := pairedSample{Matchup: p.matchup, Round: b.Segment.Round, Futures: p.cfg.futures, Deltas: map[string]float64{},
		Health: float64(own.CurrentHealth()) / float64(max(lookahead.MaxHealth(own), 1))}
	if p.foeP == nil {
		p.foeP = p.foe()
	}
	policies := map[string]lookahead.Policy{}
	for id := range b.Actors {
		policies[id] = p.foeP
	}
	policies[d.Seat] = p.hero
	playout := lookahead.Playout{Sim: p.sim, Hero: d.Seat, Policies: policies}
	play := func(seed uint64, change func(*state.Battle) bool) (float64, bool) {
		c := lookahead.Fork(b, d.Seat, seed)
		if change != nil && !change(&c) {
			return 0, false
		}
		v, err := playout.Run(&c, nil, rand.New(rand.NewSource(int64(seed>>1))))
		if err != nil {
			return 0, false
		}
		return v, true
	}
	ok := map[string]bool{}
	for j := 0; j < p.cfg.futures; j++ {
		seed := mixSeed(p.seed, uint64(b.Segment.Round)<<20|uint64(j))
		base, good := play(seed, nil)
		if !good {
			return
		}
		s.Base += base
		for _, name := range names {
			v, good := play(seed, fns[name])
			if !good {
				continue
			}
			if j == 0 {
				ok[name] = true
			}
			if ok[name] {
				s.Deltas[name] += v - base
			}
		}
	}
	for name := range s.Deltas {
		if !ok[name] {
			delete(s.Deltas, name)
		}
	}
	if _, has := s.Deltas["unit"]; !has {
		return
	}
	p.collect.mu.Lock()
	p.collect.samples = append(p.collect.samples, s)
	p.collect.mu.Unlock()
}

func mixSeed(a, b uint64) uint64 {
	z := a*0x9e3779b97f4a7c15 + b + 0x632be59bd9b4e019
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// runPaired plays the listed matchups (hero:opponent:foe) and probes the
// hero's round starts.
func runPaired(contentRoot, authoredRoot, runState, matchups string, first uint64, battles, workers int, cfg pairedConfig, out string) {
	collect := &pairedCollector{}
	for _, spec := range strings.Split(matchups, ",") {
		parts := strings.Split(strings.TrimSpace(spec), ":")
		if len(parts) != 3 {
			fail(fmt.Errorf("-matchups wants hero:opponent:foe, got %q", spec))
		}
		hero, opponent, foeKind := parts[0], parts[1], parts[2]
		foe := func() lookahead.Policy {
			if foeKind != "script" {
				return lookahead.NewGreedyPolicy()
			}
			minion, err := learned.LoadScriptedMinion(contentRoot, opponent)
			if err != nil {
				fail(err)
			}
			return lookahead.Scripted{Label: opponent, Policy: minion}
		}
		matchup := hero + " vs " + opponent
		m := lookahead.Matchup{ContentRoot: contentRoot, AuthoredRoot: authoredRoot, RunStateRoot: runState, Hero: hero, Opponent: opponent, Unified: true, HeroPlansFirst: true,
			Observe: func() lookahead.Observer {
				eng, err := engine.NewEngineWithConfig(engine.Config{OmitSnapshotContentCatalog: true}, engine.DefaultFlows()...)
				if err != nil {
					fail(err)
				}
				sim, err := lookahead.NewSimulator()
				if err != nil {
					fail(err)
				}
				return &pairedProber{cfg: cfg, engine: eng, sim: sim, hero: lookahead.NewGreedyPolicy(), foe: foe, collect: collect, matchup: matchup}
			}}
		greedy := func() (lookahead.Policy, error) { return lookahead.NewGreedyPolicy(), nil }
		foeFactory := func() (lookahead.Policy, error) { return foe(), nil }
		if _, err := lookahead.RunInterleaved(m, []lookahead.Entrant{{Name: "greedy", Hero: greedy}}, foeFactory, []bool{false, true}, first, battles, workers, nil); err != nil {
			fail(fmt.Errorf("%s: %w", matchup, err))
		}
		fmt.Fprintf(os.Stderr, "%s: %d positions so far\n", matchup, len(collect.samples))
	}
	fmt.Print(pairedReport(collect.samples))
	if out != "" {
		raw, err := json.Marshal(collect.samples)
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			fail(err)
		}
	}
}

// ratio is a rider's damage-equivalent over positions: the ratio of its
// mean change to the unit's, with a 95% interval by the delta method
// (positions are the independent units).
func ratio(samples []pairedSample, name string) (r, half float64, n int, meanDelta float64) {
	var xs, ys []float64
	for _, s := range samples {
		d, ok := s.Deltas[name]
		if !ok {
			continue
		}
		xs = append(xs, d/float64(s.Futures))
		ys = append(ys, s.Deltas["unit"]/float64(s.Futures))
	}
	n = len(xs)
	if n < 2 {
		return 0, 0, n, 0
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var vx, vy, cxy float64
	for i := range xs {
		vx += (xs[i] - mx) * (xs[i] - mx)
		vy += (ys[i] - my) * (ys[i] - my)
		cxy += (xs[i] - mx) * (ys[i] - my)
	}
	vx /= float64(n - 1)
	vy /= float64(n - 1)
	cxy /= float64(n - 1)
	if my == 0 {
		return 0, math.Inf(1), n, mx
	}
	r = mx / my
	variance := (vx + r*r*vy - 2*r*cxy) / (my * my) / float64(n)
	return r, 1.96 * math.Sqrt(math.Max(variance, 0)), n, mx
}

func pairedReport(samples []pairedSample) string {
	var b strings.Builder
	if len(samples) == 0 {
		return "no positions\n"
	}
	futures := 0
	for _, s := range samples {
		futures += s.Futures
	}
	_, _, _, unit := ratio(samples, "unit")
	fmt.Fprintf(&b, "%d positions, %d paired futures each change. The unit (one more card removed from the opponent) moves the result by %+.4f (on +1 win / -1 loss; half of it is win chance).\n\n", len(samples), futures, unit)
	names := map[string]bool{}
	for _, s := range samples {
		for name := range s.Deltas {
			names[name] = true
		}
	}
	var list []string
	for name := range names {
		if name != "unit" {
			list = append(list, name)
		}
	}
	sort.Strings(list)
	fmt.Fprintf(&b, "| Change | Positions | Mean change | Damage-equivalent (95%%) | 2026-10-15 rate |\n|---|---|---|---|---|\n")
	for _, name := range list {
		r, half, n, mean := ratio(samples, name)
		rate := pricing[name]
		if rate == "" {
			rate = "—"
		}
		fmt.Fprintf(&b, "| %s | %d | %+.4f | %.2f ± %.2f | %s |\n", name, n, mean, r, half, rate)
	}
	return b.String()
}
