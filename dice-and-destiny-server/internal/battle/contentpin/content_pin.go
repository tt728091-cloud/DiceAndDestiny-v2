package contentpin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"diceanddestiny/server/internal/battle/state"
)

const (
	SchemaVersion          = 1
	CompiledContentVersion = 1
	Algorithm              = "sha256"
)

var ErrContentMismatch = errors.New("content fingerprint mismatch")

type Pin struct {
	SchemaVersion          int    `json:"schema_version"`
	CompiledContentVersion int    `json:"compiled_content_version"`
	Algorithm              string `json:"algorithm"`
	Fingerprint            string `json:"fingerprint"`
}

type canonicalContent struct {
	Cards     []namedRuntimeContent `json:"cards"`
	Abilities []namedRuntimeContent `json:"abilities"`
	Statuses  []namedRuntimeStatus  `json:"statuses"`
	Dice      []namedDiceDefinition `json:"dice"`
}

type namedRuntimeContent struct {
	Key        string                         `json:"key"`
	Definition state.RuntimeContentDefinition `json:"definition"`
}

type namedRuntimeStatus struct {
	Key        string                        `json:"key"`
	Definition state.RuntimeStatusDefinition `json:"definition"`
}

type namedDiceDefinition struct {
	Key        string               `json:"key"`
	Definition state.DiceDefinition `json:"definition"`
}

func Compute(battle state.Battle) (Pin, error) {
	canonical := canonicalContent{
		Cards:     sortedRuntimeContent(battle.Content.Cards),
		Abilities: sortedRuntimeContent(battle.Content.Abilities),
		Statuses:  sortedRuntimeStatuses(battle.Content.Statuses),
		Dice:      sortedDiceDefinitions(battle.DiceDefinitions),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return Pin{}, err
	}
	sum := sha256.Sum256(encoded)
	return Pin{
		SchemaVersion:          SchemaVersion,
		CompiledContentVersion: CompiledContentVersion,
		Algorithm:              Algorithm,
		Fingerprint:            hex.EncodeToString(sum[:]),
	}, nil
}

func Validate(pin Pin, battle state.Battle) error {
	if pin.SchemaVersion != SchemaVersion ||
		pin.CompiledContentVersion != CompiledContentVersion ||
		pin.Algorithm != Algorithm {
		return ErrContentMismatch
	}
	actual, err := Compute(battle)
	if err != nil {
		return err
	}
	if pin != actual {
		return ErrContentMismatch
	}
	return nil
}

func sortedRuntimeContent(values map[string]state.RuntimeContentDefinition) []namedRuntimeContent {
	keys := sortedKeys(values)
	result := make([]namedRuntimeContent, 0, len(keys))
	for _, key := range keys {
		result = append(result, namedRuntimeContent{Key: key, Definition: values[key]})
	}
	return result
}

func sortedRuntimeStatuses(values map[string]state.RuntimeStatusDefinition) []namedRuntimeStatus {
	keys := sortedKeys(values)
	result := make([]namedRuntimeStatus, 0, len(keys))
	for _, key := range keys {
		result = append(result, namedRuntimeStatus{Key: key, Definition: values[key]})
	}
	return result
}

func sortedDiceDefinitions(values map[string]state.DiceDefinition) []namedDiceDefinition {
	keys := sortedKeys(values)
	result := make([]namedDiceDefinition, 0, len(keys))
	for _, key := range keys {
		result = append(result, namedDiceDefinition{Key: key, Definition: values[key]})
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
