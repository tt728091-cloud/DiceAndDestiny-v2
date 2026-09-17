package random

import (
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/state"
)

func TestReproducibleRollsDoNotDependOnClickTiming(t *testing.T) {
	b := state.Battle{Random: state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 1789498827225662}}
	copy := b.Clone()
	fast, delayed := BattleSource{Battle: &b}, BattleSource{Battle: &copy}
	for i := 0; i < 12; i++ {
		a, err := fast.Intn(6)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
		z, err := delayed.Intn(6)
		if err != nil || a != z {
			t.Fatalf("timing changed draw %d: %d / %d (%v)", i, a, z, err)
		}
	}
	if b.Random.Cursor != 12 || copy.Random.Cursor != 12 {
		t.Fatal("draw count was not preserved")
	}
}

// Fixed seeds make this regression deterministic. These broad distribution
// checks catch gross bias/correlation, not prove that a generator is random.
func TestSeededD6DistributionAndRepeats(t *testing.T) {
	const perSeed = 100000
	seeds := []uint64{0, 1, 42, 20260915, 1789498827225662, ^uint64(0)}
	var counts [6]int
	var pairs [6][6]int
	equal, triples, tripleRepeats := 0, 0, 0
	for _, seed := range seeds {
		b := state.Battle{Random: state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: seed}}
		source := BattleSource{Battle: &b}
		previous := -1
		var lastThree, currentThree [3]int
		for i := 0; i < perSeed; i++ {
			x, err := source.Intn(6)
			if err != nil || x < 0 || x >= 6 {
				t.Fatalf("invalid D6: %d %v", x, err)
			}
			counts[x]++
			if previous >= 0 {
				pairs[previous][x]++
				if previous == x {
					equal++
				}
			}
			previous = x
			currentThree[i%3] = x
			if i%3 == 2 {
				if i >= 5 {
					triples++
					if currentThree == lastThree {
						tripleRepeats++
					}
				}
				lastThree = currentThree
			}
		}
		if b.Random.Cursor != perSeed {
			t.Fatal("cursor stopped advancing")
		}
	}
	for face, n := range counts {
		if n < 97000 || n > 103000 {
			t.Fatalf("face %d frequency: %d", face+1, n)
		}
	}
	for a, row := range pairs {
		for b, n := range row {
			if n < 15700 || n > 17700 {
				t.Fatalf("transition %d -> %d: %d", a+1, b+1, n)
			}
		}
	}
	if tripleRepeats < 700 || tripleRepeats > 1150 {
		t.Fatalf("three-die repeats: %d / %d", tripleRepeats, triples)
	}
	t.Logf("600000 dice: faces=%v; adjacent repeats=%d/599994; three-die repeats=%d/%d (expected ~%.1f)", counts, equal, tripleRepeats, triples, float64(triples)/216)
}

func TestNormalSourceAdvancesAndRejectsInvalidBounds(t *testing.T) {
	b := state.Battle{}
	source := BattleSource{Battle: &b}
	if _, err := source.Intn(0); err == nil || b.Random.Cursor != 0 {
		t.Fatal("invalid bound accepted or consumed a draw")
	}
	for i := 0; i < 128; i++ {
		x, err := source.Intn(6)
		if err != nil || x < 0 || x >= 6 {
			t.Fatalf("invalid D6: %d %v", x, err)
		}
	}
	if b.Random.Cursor != 128 || b.Random.Algorithm != state.RandomAlgorithmCrypto {
		t.Fatal("normal source did not use fresh crypto draws")
	}
}
