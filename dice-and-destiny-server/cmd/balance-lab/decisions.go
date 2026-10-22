package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"diceanddestiny/server/internal/battle/decisions"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// labelConfig sets how -label measures each recorded decision's options.
type labelConfig struct {
	futures, rounds, topRerolls, topAbilities, searchWorkers int
	sample                                                   float64
	// policyModel and segments, when set, let a policy network make the
	// deciding seat's own moves in the reference futures (in those
	// segments; greedy elsewhere): the values of the next policy iteration.
	policyModel, segments string
}

// reference is the player config the reference futures are played with.
func (lc labelConfig) reference(base playerConfig) playerConfig {
	c := base
	c.rounds, c.searchWorkers, c.thinkMS, c.valueModel, c.trace = lc.rounds, lc.searchWorkers, 0, "", false
	c.policyModel, c.networkFutures, c.networkSegments = lc.policyModel, lc.policyModel != "", lc.segments
	return c
}

// newRecorders returns the observer factory for -label: each runner gets its
// own recorder and reference player.
func newRecorders(writer *decisions.Writer, hero, opponent string, foe lookahead.PolicyFactory, heroFoe bool, base playerConfig, lc labelConfig) (func() lookahead.Observer, error) {
	c := lc.reference(base)
	factory, err := playerFactory("lookahead", foe, heroFoe, c)
	if err != nil {
		return nil, err
	}
	describe := fmt.Sprintf("futures=%d (halving) rounds=%d top-rerolls=%d top-abilities=%d %s", lc.futures, lc.rounds, lc.topRerolls, lc.topAbilities, c.describe("lookahead"))
	return func() lookahead.Observer {
		p, err := factory()
		if err != nil {
			fail(err)
		}
		reference := p.(*lookahead.SearchPolicy)
		reference.TopRerolls, reference.TopAbilities = lc.topRerolls, lc.topAbilities
		return &decisions.Recorder{Hero: hero, Opponent: opponent, Reference: reference, Describe: describe, Futures: lc.futures, Sample: lc.sample, Writer: writer}
	}, nil
}

// scoreRow is one player's verdicts on the benchmark.
type scoreRow struct {
	gap, sq           float64
	rated, best, same int
	unrated           int
}

func (s *scoreRow) add(gap float64, best, same bool) {
	s.rated++
	s.gap += gap
	s.sq += gap * gap
	if best {
		s.best++
	}
	if same {
		s.same++
	}
}

func (s scoreRow) mean() float64 { return s.gap / float64(max(s.rated, 1)) }

func (s scoreRow) se() float64 {
	n := float64(max(s.rated, 2))
	m := s.mean()
	return math.Sqrt(math.Max(s.sq/n-m*m, 0) / (n - 1))
}

// runScore asks each player for its choice on every recorded decision and
// reports the value it loses against the reference's best option.
func runScore(paths []string, contentRoot string, specs []string, base playerConfig, workers int) error {
	type item struct {
		r decisions.Record
		b *state.Battle
	}
	var items []item
	if err := decisions.Read(paths, func(r decisions.Record, b *state.Battle) error {
		items = append(items, item{r, b})
		return nil
	}); err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no decisions in %v", paths)
	}
	names := []string{"reference best", "anchor (greedy)"}
	names = append(names, specs...)
	// Each player's model of a minion is the minion's script, of a hero the
	// greedy player.
	factories := map[string][]lookahead.PolicyFactory{}
	factoriesFor := func(opponent string) ([]lookahead.PolicyFactory, error) {
		if f, ok := factories[opponent]; ok {
			return f, nil
		}
		_, scriptErr := learned.LoadScriptedMinion(contentRoot, opponent)
		heroFoe := scriptErr != nil
		foe := func() (lookahead.Policy, error) {
			if heroFoe {
				return lookahead.NewGreedyPolicy(), nil
			}
			m, err := learned.LoadScriptedMinion(contentRoot, opponent)
			if err != nil {
				return nil, err
			}
			return lookahead.Scripted{Label: opponent, Policy: m}, nil
		}
		var list []lookahead.PolicyFactory
		for _, spec := range specs {
			name, c, err := parsePlayer(spec, base)
			if err != nil {
				return nil, err
			}
			f, err := playerFactory(name, foe, heroFoe, c)
			if err != nil {
				return nil, err
			}
			list = append(list, f)
		}
		factories[opponent] = list
		return list, nil
	}
	for _, it := range items {
		if _, err := factoriesFor(it.r.Opponent); err != nil {
			return err
		}
	}
	kinds := map[string]bool{}
	for _, it := range items {
		kinds[it.r.Kind] = true
	}
	type key struct {
		player int
		kind   string
	}
	rows := map[key]*scoreRow{}
	row := func(player int, kind string) *scoreRow {
		k := key{player, kind}
		if rows[k] == nil {
			rows[k] = &scoreRow{}
		}
		return rows[k]
	}
	var mu sync.Mutex
	var firstErr error
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(1, workers); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sim, err := lookahead.NewSimulator()
			if err != nil {
				fail(err)
			}
			players := map[string][]lookahead.Policy{}
			for i := range jobs {
				it := items[i]
				list := players[it.r.Opponent]
				if list == nil {
					for _, f := range factories[it.r.Opponent] {
						p, err := f()
						if err != nil {
							fail(err)
						}
						list = append(list, p)
					}
					players[it.r.Opponent] = list
				}
				verdicts := make([]int, len(list))
				for p, player := range list {
					b := it.b.Clone()
					d := &lookahead.Decision{Sim: sim, Battle: &b, Seat: it.r.Seat, Actions: it.r.Actions, Rand: rand.New(rand.NewSource(int64(it.r.Seed)*31 + int64(it.r.Step)))}
					index, err := player.Choose(d)
					if err != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("%s on seed %d step %d: %w", specs[p], it.r.Seed, it.r.Step, err)
						}
						mu.Unlock()
						verdicts[p] = -1
						continue
					}
					verdicts[p] = it.r.Option(it.b, index)
				}
				anchor := it.r.Option(it.b, it.r.Anchor)
				mu.Lock()
				for _, kind := range []string{"all", it.r.Kind} {
					row(0, kind).add(0, true, it.r.Best == anchor)
					if anchor >= 0 {
						row(1, kind).add(it.r.Gap[anchor], anchor == it.r.Best, true)
					}
					for p, k := range verdicts {
						if k < 0 {
							row(p+2, kind).unrated++
							continue
						}
						row(p+2, kind).add(it.r.Gap[k], k == it.r.Best, k == anchor)
					}
				}
				mu.Unlock()
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	var noise float64
	for _, it := range items {
		var se float64
		for k := range it.r.Options {
			se = math.Max(se, it.r.GapSE[k])
		}
		noise += se
	}
	fmt.Printf("%d decisions from %d files · reference: %s\n", len(items), len(paths), items[0].r.Reference)
	fmt.Printf("Value lost per decision against the reference's best option (search scale: win +1, loss -1; half of it is win chance). Mean largest standard error of a gap: %.4f\n\n", noise/float64(len(items)))
	list := []string{"all"}
	var rest []string
	for k := range kinds {
		rest = append(rest, k)
	}
	sort.Strings(rest)
	list = append(list, rest...)
	for _, kind := range list {
		fmt.Printf("%s (%d decisions)\n", kind, row(0, kind).rated)
		fmt.Printf("  %-44s %10s %8s %8s %8s %8s\n", "player", "lost", "±95%", "best", "=greedy", "unrated")
		for p, name := range names {
			r := row(p, kind)
			if p == 0 {
				continue
			}
			fmt.Printf("  %-44s %10.4f %8.4f %7.1f%% %7.1f%% %8d\n", name, r.mean(), 1.96*r.se(), 100*float64(r.best)/float64(max(r.rated, 1)), 100*float64(r.same)/float64(max(r.rated, 1)), r.unrated)
		}
		fmt.Printf("  %-44s %10s %8s %7.1f%%\n\n", "(greedy already best)", "", "", 100*float64(row(0, kind).same)/float64(max(row(0, kind).rated, 1)))
	}
	return nil
}

// expandPaths turns comma-separated paths and globs into files.
func expandPaths(spec string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		matches, err := filepath.Glob(part)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			if _, err := os.Stat(part); err != nil {
				return nil, err
			}
			matches = []string{part}
		}
		out = append(out, matches...)
	}
	return out, nil
}

// runRelabel measures recorded decisions again with another reference (the
// -label-* settings), keeping each decision's options, and writes them to a
// new file in the same order.
func runRelabel(paths []string, out, contentRoot string, base playerConfig, lc labelConfig, workers int) error {
	type item struct {
		r decisions.Record
		b *state.Battle
	}
	var items []item
	if err := decisions.Read(paths, func(r decisions.Record, b *state.Battle) error {
		items = append(items, item{r, b})
		return nil
	}); err != nil {
		return err
	}
	c := lc.reference(base)
	describe := fmt.Sprintf("futures=%d (halving) rounds=%d top-rerolls=%d top-abilities=%d %s (relabelled)", lc.futures, lc.rounds, lc.topRerolls, lc.topAbilities, c.describe("lookahead"))
	factories := map[string]lookahead.PolicyFactory{}
	for _, it := range items {
		if factories[it.r.Opponent] != nil {
			continue
		}
		opponent := it.r.Opponent
		_, scriptErr := learned.LoadScriptedMinion(contentRoot, opponent)
		foe := func() (lookahead.Policy, error) {
			if scriptErr != nil {
				return lookahead.NewGreedyPolicy(), nil
			}
			m, err := learned.LoadScriptedMinion(contentRoot, opponent)
			if err != nil {
				return nil, err
			}
			return lookahead.Scripted{Label: opponent, Policy: m}, nil
		}
		f, err := playerFactory("lookahead", foe, scriptErr != nil, c)
		if err != nil {
			return err
		}
		factories[opponent] = f
	}
	results := make([]decisions.Record, len(items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < max(1, workers); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sim, err := lookahead.NewSimulator()
			if err != nil {
				fail(err)
			}
			references := map[string]*lookahead.SearchPolicy{}
			for i := range jobs {
				it := items[i]
				reference := references[it.r.Opponent]
				if reference == nil {
					p, err := factories[it.r.Opponent]()
					if err != nil {
						fail(err)
					}
					reference = p.(*lookahead.SearchPolicy)
					references[it.r.Opponent] = reference
				}
				seed := it.r.Seed*1_000_003 + uint64(it.r.Step)*7919 + 17
				d := &lookahead.Decision{Sim: sim, Battle: it.b, Seat: it.r.Seat, Actions: it.r.Actions, Rand: rand.New(rand.NewSource(int64(seed)))}
				values, err := decisions.Measure(reference, d, it.r.Options, lc.futures, seed)
				if err != nil {
					fail(err)
				}
				r := it.r
				r.Summarize(values)
				r.Reference = describe
				results[i] = r
				mu.Lock()
				done++
				if done%50 == 0 {
					fmt.Fprintf(os.Stderr, "\r%d/%d decisions", done, len(items))
				}
				mu.Unlock()
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	writer, err := decisions.Create(out)
	if err != nil {
		return err
	}
	for i, r := range results {
		if err := writer.Write(r, items[i].b); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "\nrelabelled %d decisions into %s\n", writer.Count, out)
	return writer.Close()
}
