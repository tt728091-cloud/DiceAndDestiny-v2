package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"diceanddestiny/server/internal/battle/setup"
)

var ErrInvalidRunPlayerSave = errors.New("invalid run player save")

type SavedRunPlayerState struct {
	ActorID string         `json:"actor_id"`
	Cards   SavedCardZones `json:"cards"`
}

type SavedCardZones struct {
	Deck    []string `json:"deck"`
	Hand    []string `json:"hand,omitempty"`
	Discard []string `json:"discard,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type savedRunPlayerStateFile struct {
	ActorID string              `json:"actor_id"`
	Cards   *savedCardZonesFile `json:"cards"`
}

type savedCardZonesFile struct {
	Deck    *[]string `json:"deck"`
	Hand    []string  `json:"hand,omitempty"`
	Discard []string  `json:"discard,omitempty"`
	Removed []string  `json:"removed,omitempty"`
}

func LoadRunPlayerState(path string) (setup.RunPlayerState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return setup.RunPlayerState{}, fmt.Errorf("load run player state: read %q: %w", path, err)
	}

	var saved savedRunPlayerStateFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return setup.RunPlayerState{}, fmt.Errorf("load run player state: parse JSON %q: %w", path, err)
	}

	if err := validateSavedRunPlayerState(saved); err != nil {
		return setup.RunPlayerState{}, err
	}

	return setup.RunPlayerState{
		ActorID: saved.ActorID,
		Cards: setup.RunCardZones{
			Deck:    copyStrings(*saved.Cards.Deck),
			Hand:    copyStrings(saved.Cards.Hand),
			Discard: copyStrings(saved.Cards.Discard),
			Removed: copyStrings(saved.Cards.Removed),
		},
	}, nil
}

func validateSavedRunPlayerState(saved savedRunPlayerStateFile) error {
	switch {
	case saved.ActorID == "":
		return fmt.Errorf("%w: actor_id is required", ErrInvalidRunPlayerSave)
	case saved.Cards == nil:
		return fmt.Errorf("%w: cards is required", ErrInvalidRunPlayerSave)
	case saved.Cards.Deck == nil:
		return fmt.Errorf("%w: cards.deck is required", ErrInvalidRunPlayerSave)
	case len(*saved.Cards.Deck) == 0:
		return fmt.Errorf("%w: cards.deck must not be empty", ErrInvalidRunPlayerSave)
	}

	return nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}
