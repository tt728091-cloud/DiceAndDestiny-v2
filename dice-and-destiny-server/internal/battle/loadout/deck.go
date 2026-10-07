// Package loadout stores owned decks separately from shared content and battles.
package loadout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"diceanddestiny/server/internal/content"
)

const MaxCards = 100
const MaxCopies = 20

type Entry struct {
	CardID string `json:"card_id" yaml:"card_id"`
	Count  int    `json:"count" yaml:"count"`
}

type Saved struct {
	Version   int     `json:"version"`
	Character string  `json:"character"`
	Deck      []Entry `json:"decklist"`
}

func Validate(deck []Entry, cards map[string]content.BattleCardDefinition) ([]Entry, error) {
	total := 0
	seen := map[string]bool{}
	for _, entry := range deck {
		if _, ok := cards[entry.CardID]; !ok {
			return nil, fmt.Errorf("card %q is unavailable for this character", entry.CardID)
		}
		if seen[entry.CardID] {
			return nil, fmt.Errorf("duplicate deck entry %q", entry.CardID)
		}
		seen[entry.CardID] = true
		limit := MaxCopies
		if c := cards[entry.CardID].Economy; c != nil {
			limit = c.CopyLimit
		}
		if entry.Count < 1 || entry.Count > limit {
			return nil, fmt.Errorf("card quantities must be 1–%d", limit)
		}
		total += entry.Count
	}
	if total < 1 || total > MaxCards {
		return nil, fmt.Errorf("deck must contain 1–%d cards", MaxCards)
	}
	result := append([]Entry(nil), deck...)
	sort.Slice(result, func(i, j int) bool { return result[i].CardID < result[j].CardID })
	return result, nil
}

func path(root, character string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("loadout root is required")
	}
	switch character {
	case "adventurer", "venom", "curse", "blade_warden":
	default:
		return "", fmt.Errorf("unknown playable character %q", character)
	}
	return filepath.Join(root, character+".json"), nil
}

func ReadLegacy(root, character string, cards map[string]content.BattleCardDefinition) ([]Entry, error) {
	if root == "" {
		return nil, nil
	} // Headless simulations retain templates.
	filename, err := path(root, character)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved Saved
	if err = json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("read saved deck: %w", err)
	}
	if saved.Version != 1 || saved.Character != character {
		return nil, fmt.Errorf("saved deck version or character does not match")
	}
	return Validate(saved.Deck, cards)
}

// Write creates a legacy-format import fixture. Live editors use WriteSharedDeck.
func Write(root, character string, deck []Entry, cards map[string]content.BattleCardDefinition) ([]Entry, error) {
	filename, err := path(root, character)
	if err != nil {
		return nil, err
	}
	deck, err = Validate(deck, cards)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(Saved{Version: 1, Character: character, Deck: deck}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(root, ".deck-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(f.Name(), filename); err != nil {
		return nil, err
	}
	return deck, nil
}

// Read exposes the shared deck to older read-only consumers.
func Read(root, character string, cards map[string]content.BattleCardDefinition) ([]Entry, error) {
	if root != "" {
		filename, err := progressPath(root, character)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filename)
		if err == nil {
			var p Progress
			if err = json.Unmarshal(data, &p); err != nil {
				return nil, err
			}
			if p.Version != 1 || p.Character != character {
				return nil, fmt.Errorf("saved deck version or character does not match")
			}
			if p.SharedDeck {
				if len(p.Deck) == 0 {
					return []Entry{}, nil
				}
				return Validate(p.Deck, cards)
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return ReadLegacy(root, character, cards)
}
