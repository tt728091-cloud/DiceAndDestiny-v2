package main

import (
	"fmt"
	"math"
	"os"
	"time"
)

// The sweep (-sweep) prices every default board at targets across its
// staircase and scores the field against itself at each, the way the round
// robin does at one target: each board's mean score against the other
// boards under each duel defense, and their mean.

// sweepResult is the field at every target.
type sweepResult struct {
	Healths  []int                  `json:"healths"`
	Defenses []string               `json:"defenses"`
	Boards   []string               `json:"boards"`
	Targets  []float64              `json:"targets"`
	Averages [][]float64            `json:"averages"` // [board][target]: whole average per turn
	Steps    [][]int                `json:"steps"`    // [board][target]: the staircase step in force
	Means    map[string][][]float64 `json:"means"`    // defense → [board][target]
	Overall  [][]float64            `json:"overall"`  // [board][target]
}

func runSweep(l lab, healths string, step float64, out string) {
	started := time.Now()
	rr := roundRobin{lab: l, defenses: duelDefenses, target: l.builder.Menus.Pricing.Target}
	var err error
	if rr.healths, err = parseRange(healths); err != nil {
		fail(err)
	}
	opts := l.builder.Menus.Options()
	base := rr.defaultBoards()
	res := sweepResult{Healths: rr.healths, Defenses: rr.defenses, Means: map[string][][]float64{}}
	for _, b := range base {
		res.Boards = append(res.Boards, b.Label)
	}
	n := int(math.Round((opts.To-opts.From)/step)) + 1
	res.Averages, res.Steps, res.Overall = grid(len(base), n), make([][]int, len(base)), grid(len(base), n)
	for i := range res.Steps {
		res.Steps[i] = make([]int, n)
	}
	for _, d := range rr.defenses {
		res.Means[d] = grid(len(base), n)
	}
	for k := 0; k < n; k++ {
		t := math.Min(opts.From+float64(k)*step, opts.To)
		res.Targets = append(res.Targets, t)
		boards := append([]rrBoard(nil), base...)
		for i := range boards {
			if err := boards[i].price(t, rr.builder); err != nil {
				fail(fmt.Errorf("%s at %.2f: %w", boards[i].Label, t, err))
			}
			res.Averages[i][k], res.Steps[i][k] = boards[i].Average, boards[i].report.Step
		}
		f := rr.field(fmt.Sprintf("%.2f", t), boards)
		for i, s := range f.Summaries {
			for _, d := range rr.defenses {
				res.Means[d][i][k] = s.ByDefense[d]
			}
			res.Overall[i][k] = s.Overall
		}
		if k%10 == 0 {
			fmt.Fprintf(os.Stderr, "swept %.2f (%s)\n", t, time.Since(started).Round(time.Second))
		}
	}
	if out == "" {
		fail(fmt.Errorf("-sweep needs -out"))
	}
	if err := writeJSON(out, res); err != nil {
		fail(err)
	}
	fmt.Printf("swept %d boards over %d targets %.2f–%.2f in %s; wrote %s\n", len(base), n, opts.From, opts.To, time.Since(started).Round(time.Second), out)
}

func grid(rows, cols int) [][]float64 {
	out := make([][]float64, rows)
	for i := range out {
		out[i] = make([]float64, cols)
	}
	return out
}
