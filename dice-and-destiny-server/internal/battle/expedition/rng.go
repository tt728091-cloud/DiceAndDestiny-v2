package expedition

// The expedition's randomness is a pure function of the save's seed and a
// counter stored in the save, so the same seed and the same actions always
// produce the same map, rolls and loot, and a reloaded save continues the
// same sequence. Nothing here uses math/rand or the clock.

func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	z := x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// next draws the next 64-bit value and advances the position.
func (s *State) next() uint64 {
	s.RNGPosition++
	return splitmix64(s.Seed ^ (s.RNGPosition * 0xD1B54A32D192ED03))
}

// intn draws a value in [0, n).
func (s *State) intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(s.next() % uint64(n))
}

// rangeIn draws a value in [r.Min, r.Max].
func (s *State) rangeIn(r Range) int {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + s.intn(r.Max-r.Min+1)
}

// d6 rolls a six-sided die.
func (s *State) d6() int { return 1 + s.intn(6) }

// die rolls a sides-sided die (1..sides).
func (s *State) die(sides int) int { return 1 + s.intn(max(1, sides)) }

// shuffle is a seeded Fisher–Yates shuffle in place.
func (s *State) shuffle(list []string) {
	for i := len(list) - 1; i > 0; i-- {
		j := s.intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}
