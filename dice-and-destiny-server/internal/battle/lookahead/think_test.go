package lookahead_test

import (
	"math"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/lookahead"
)

// A time-limited decision stops on time, still plays futures, and returns a
// listed command. The limit is hard: a future still running at the limit is
// abandoned, so only one engine step may overrun it.
func TestSearchThinkTimeIsAHardLimit(t *testing.T) {
	runner, foe := brineMaskRunner(t)
	r := &recorder{inner: lookahead.ProxyPolicy{}, keep: func(d *lookahead.Decision) bool {
		return stage(d) == "planning" && len(d.Battle.Settled.Actors[d.Seat].FinalDice) > 0
	}}
	if _, err := runner.Play(4, r, foe); err != nil {
		t.Fatal(err)
	}
	_, model := brineMaskRunner(t)
	search := lookahead.NewSearchPolicy(func() lookahead.Policy { return model }, 96)
	search.Think = 40 * time.Millisecond
	searched := 0
	for _, d := range r.got[:min(4, len(r.got))] {
		before := search.Searched
		started := time.Now()
		index, err := search.Choose(d)
		elapsed := time.Since(started)
		if err != nil {
			t.Fatal(err)
		}
		if index < 0 || index >= len(d.Actions) {
			t.Fatalf("chose %d of %d", index, len(d.Actions))
		}
		if search.Searched > before {
			searched++
			if elapsed > search.Think+40*time.Millisecond {
				t.Fatalf("decision took %s with a %s limit", elapsed, search.Think)
			}
		}
	}
	if searched == 0 || search.Futures == 0 {
		t.Fatalf("searched %d decisions with %d futures", searched, search.Futures)
	}
}

// The paired comparison matches a hand calculation: four battles, A better
// on two (by 1 and 0.5), equal on two.
func TestCompareIsPaired(t *testing.T) {
	a := lookahead.Summary{Player: "a", Records: []lookahead.BattleRecord{
		{Seed: 1, Seat: "seat-a", Result: "win"}, {Seed: 1, Seat: "seat-b", Result: "draw"},
		{Seed: 2, Seat: "seat-a", Result: "loss"}, {Seed: 2, Seat: "seat-b", Result: "win"},
	}}
	b := lookahead.Summary{Player: "b", Records: []lookahead.BattleRecord{
		{Seed: 2, Seat: "seat-b", Result: "win"}, {Seed: 2, Seat: "seat-a", Result: "loss"},
		{Seed: 1, Seat: "seat-b", Result: "loss"}, {Seed: 1, Seat: "seat-a", Result: "loss"},
		{Seed: 3, Seat: "seat-a", Result: "win"},
	}}
	c, err := lookahead.Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// Differences 1, 0.5, 0, 0: mean 0.375, sample sd 0.4787, SE 0.2394.
	if c.Pairs != 4 || c.Unpaired != 1 || c.Better != 2 || c.Worse != 0 {
		t.Fatalf("pairs %d unpaired %d better %d worse %d", c.Pairs, c.Unpaired, c.Better, c.Worse)
	}
	se := math.Sqrt((0.625*0.625 + 0.125*0.125 + 2*0.375*0.375) / 3 / 4)
	if math.Abs(c.Diff-0.375) > 1e-12 || math.Abs(c.Z-0.375/se) > 1e-9 || math.Abs(c.Low-(0.375-1.96*se)) > 1e-9 {
		t.Fatalf("diff %v z %v low %v, want 0.375, %v, %v", c.Diff, c.Z, c.Low, 0.375/se, 0.375-1.96*se)
	}
	if c.A3 != [3]int{2, 1, 1} || c.B3 != [3]int{1, 0, 3} {
		t.Fatalf("tallies %v %v", c.A3, c.B3)
	}
}

// Interleaving players seed by seed changes nothing for deterministic
// players: each entrant gets the battles it would get alone.
func TestInterleavedMatchesSeparateRuns(t *testing.T) {
	contentRoot := serverRoot(t) + "/content"
	m := lookahead.Matchup{ContentRoot: contentRoot, AuthoredRoot: authoredCopy(t), RunStateRoot: t.TempDir(), Hero: "starter", Opponent: "starter", Unified: true, HeroPlansFirst: true}
	greedy := func() (lookahead.Policy, error) { return lookahead.NewGreedyPolicy(), nil }
	proxy := func() (lookahead.Policy, error) { return lookahead.ProxyPolicy{}, nil }
	both, err := lookahead.RunInterleaved(m, []lookahead.Entrant{{Name: "greedy", Hero: greedy}, {Name: "proxy", Hero: proxy}}, greedy, []bool{false, true}, 1, 3, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	for e, hero := range []lookahead.PolicyFactory{greedy, proxy} {
		for k, seatB := range []bool{false, true} {
			alone := m
			alone.HeroInSeatB = seatB
			s, err := lookahead.RunBattles(alone, hero, greedy, 1, 3, 2, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, r := range s.Records {
				got := both[e][k*3+i]
				if got.Seed != r.Seed || got.Seat != r.Seat || got.Result != r.Result || got.HeroHealth != r.HeroHealth || got.FoeHealth != r.FoeHealth || got.Rounds != r.Rounds {
					t.Fatalf("entrant %d seat %v seed %d: interleaved %+v, alone %+v", e, seatB, r.Seed, got, r)
				}
			}
		}
	}
}
