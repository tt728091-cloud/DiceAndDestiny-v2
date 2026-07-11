package random

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"diceanddestiny/server/internal/battle/state"
)

type Source interface {
	Intn(maxExclusive int) (int, error)
}

// NamedSource isolates unrelated random domains so inserting a status roll does
// not silently perturb card draws or AI planning.
type NamedSource interface {
	IntnNamed(stream string, maxExclusive int) (int, error)
}

type NamedFallback struct{ Source Source }

func (source NamedFallback) IntnNamed(_ string, maxExclusive int) (int, error) {
	if source.Source == nil {
		return cryptoIntn(maxExclusive)
	}
	return source.Source.Intn(maxExclusive)
}

type ScriptedValue struct {
	Stream string
	Bound  int
	Value  int
}

// Scripted is a deterministic test dependency. It fails on the first wrong
// category, bound, value, or exhausted call and can prove complete consumption.
type Scripted struct {
	Values []ScriptedValue
	Cursor int
}

func (source *Scripted) IntnNamed(stream string, maxExclusive int) (int, error) {
	if source == nil || source.Cursor >= len(source.Values) {
		return 0, fmt.Errorf("scripted random exhausted at %q bound %d", stream, maxExclusive)
	}
	want := source.Values[source.Cursor]
	if want.Stream != stream || want.Bound != maxExclusive {
		return 0, fmt.Errorf("scripted random call %d = %q/%d, want %q/%d", source.Cursor, stream, maxExclusive, want.Stream, want.Bound)
	}
	if want.Value < 0 || want.Value >= maxExclusive {
		return 0, fmt.Errorf("scripted random value %d out of range [0,%d)", want.Value, maxExclusive)
	}
	source.Cursor++
	return want.Value, nil
}

func (source *Scripted) AssertExhausted() error {
	if source == nil {
		return errors.New("scripted random source is nil")
	}
	if source.Cursor != len(source.Values) {
		return fmt.Errorf("scripted random consumed %d of %d values", source.Cursor, len(source.Values))
	}
	return nil
}

type BattleSource struct {
	Battle   *state.Battle
	Fallback Source
}

func (source BattleSource) Intn(maxExclusive int) (int, error) {
	if maxExclusive <= 0 {
		return 0, errors.New("random bound must be positive")
	}
	if source.Battle == nil {
		if source.Fallback == nil {
			return cryptoIntn(maxExclusive)
		}
		return source.Fallback.Intn(maxExclusive)
	}

	randomState := &source.Battle.Random
	switch randomState.Mode {
	case "", state.RandomModeNormal:
		value, err := cryptoIntn(maxExclusive)
		if err != nil {
			return 0, err
		}
		randomState.Mode = state.RandomModeNormal
		randomState.Algorithm = state.RandomAlgorithmCrypto
		randomState.Cursor++
		return value, nil
	case state.RandomModeReproducible:
		if randomState.Algorithm != state.RandomAlgorithmSHA256 {
			return 0, errors.New("unsupported reproducible random algorithm")
		}
		var input [16]byte
		binary.BigEndian.PutUint64(input[:8], randomState.Seed)
		binary.BigEndian.PutUint64(input[8:], randomState.Cursor)
		sum := sha256.Sum256(input[:])
		randomState.Cursor++
		return int(binary.BigEndian.Uint64(sum[:8]) % uint64(maxExclusive)), nil
	default:
		return 0, errors.New("unsupported battle random mode")
	}
}

func cryptoIntn(maxExclusive int) (int, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(maxExclusive)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
