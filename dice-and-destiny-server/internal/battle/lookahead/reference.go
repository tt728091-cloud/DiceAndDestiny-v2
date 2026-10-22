package lookahead

import "time"

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
	runJobs(pool, futures*len(options), time.Time{}, func(w *searchWorker) func(t int) {
		future := s.future(w, d, time.Time{})
		return func(t int) {
			j, k := t/len(options), t%len(options)
			values[k][j], _ = future(options[k], base+uint64(j)*0x9e3779b97f4a7c15)
		}
	})
	s.Futures += futures * len(options)
	return values, nil
}
