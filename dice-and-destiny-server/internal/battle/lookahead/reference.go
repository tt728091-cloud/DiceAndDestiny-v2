package lookahead

import (
	"sync"
	"time"
)

// Options lists a decision's options the way the search narrows them (see
// candidates: TopRerolls and TopAbilities set how many of the solver's
// rerolls, die choices and ability choices stay) and the greedy player's
// choice, which is always among them.
func (s *SearchPolicy) Options(d *Decision) (options []int, anchor int, err error) {
	anchor, err = s.Rollout.Choose(d)
	if err != nil {
		return nil, 0, err
	}
	useful := Useful(d.Battle, d.Seat, d.Actions)
	if len(useful) == 1 {
		return useful, useful[0], nil
	}
	options, err = s.candidates(d, useful, anchor)
	return options, anchor, err
}

// OptionValues plays `futures` futures of each listed option with the
// search's playout settings and returns their values, option by option.
// Future j uses the same seed (base, j) for every option, so options are
// compared on equal luck, as in the search. Reference values for the
// decision benchmark and the network's training labels come from here.
func (s *SearchPolicy) OptionValues(d *Decision, options []int, futures int, base uint64) ([][]float64, error) {
	pool, err := s.workers()
	if err != nil {
		return nil, err
	}
	values := make([][]float64, len(options))
	for k := range values {
		values[k] = make([]float64, futures)
	}
	type job struct{ k, j int }
	next := make(chan job)
	var wg sync.WaitGroup
	for _, w := range pool {
		wg.Add(1)
		go func(w *searchWorker) {
			defer wg.Done()
			future := s.future(w, d, time.Time{})
			for jb := range next {
				values[jb.k][jb.j], _ = future(options[jb.k], base+uint64(jb.j)*0x9e3779b97f4a7c15)
			}
		}(w)
	}
	for j := 0; j < futures; j++ {
		for k := range options {
			next <- job{k, j}
		}
	}
	close(next)
	wg.Wait()
	s.Futures += futures * len(options)
	return values, nil
}
