package card

import (
	"errors"
	"fmt"
	"math/rand"
)

var ErrInvalidShuffle = errors.New("invalid deck shuffle")

type ShuffleSource interface {
	Intn(n int) int
}

type SeededShuffleSource struct {
	rng *rand.Rand
}

func NewSeededShuffleSource(seed int64) *SeededShuffleSource {
	return &SeededShuffleSource{
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (s *SeededShuffleSource) Intn(n int) int {
	return s.rng.Intn(n)
}

func ShuffleDeck(deck []string, source ShuffleSource) error {
	if source == nil {
		return fmt.Errorf("%w: shuffle source is required", ErrInvalidShuffle)
	}

	for i := len(deck) - 1; i > 0; i-- {
		j := source.Intn(i + 1)
		if j < 0 || j > i {
			return fmt.Errorf("%w: shuffle source returned index %d outside [0,%d]", ErrInvalidShuffle, j, i)
		}

		deck[i], deck[j] = deck[j], deck[i]
	}

	return nil
}
