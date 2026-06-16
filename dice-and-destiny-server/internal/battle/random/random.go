package random

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"

	"diceanddestiny/server/internal/battle/state"
)

type Source interface {
	Intn(maxExclusive int) (int, error)
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
