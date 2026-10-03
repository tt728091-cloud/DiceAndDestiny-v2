package loadout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"diceanddestiny/server/internal/content"
	"gopkg.in/yaml.v3"
)

type Upgrade struct {
	To string `yaml:"to" json:"to"`
	XP int    `yaml:"xp" json:"xp"`
}
type CharacterEconomy struct {
	StartingDeck      []Entry               `yaml:"starting_decklist" json:"-"`
	StartingAbilities *content.AbilityBoard `yaml:"starting_abilities" json:"-"`
	CardPrices        map[string]int        `yaml:"card_prices" json:"card_prices"`
	CardUpgrades      map[string]Upgrade    `yaml:"card_upgrades" json:"card_upgrades"`
	AbilityUpgrades   map[string]Upgrade    `yaml:"ability_upgrades" json:"ability_upgrades"`
}
type Economy struct {
	Version          int                         `yaml:"schema_version"`
	StartingXP       int                         `yaml:"starting_xp"`
	DefaultCardPrice int                         `yaml:"default_card_price"`
	Characters       map[string]CharacterEconomy `yaml:"characters"`
}
type Progress struct {
	Version   int                  `json:"version"`
	Character string               `json:"character"`
	Revision  int                  `json:"revision"`
	XP        int                  `json:"xp"`
	Deck      []Entry              `json:"decklist"`
	Abilities content.AbilityBoard `json:"ability_board"`
}
type Purchase struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	Revision     int    `json:"revision"`
	ExpectedCost int    `json:"expected_cost"`
}

var progressMu sync.Mutex

func LoadEconomy(root string, catalogs map[string]content.BattleLibrary) (Economy, error) {
	var e Economy
	data, err := os.ReadFile(filepath.Join(root, "progression_v1", "economy.yaml"))
	if err != nil {
		return e, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err = decoder.Decode(&e); err != nil {
		return e, err
	}
	if e.Version != 1 || e.StartingXP < 0 || e.StartingXP > 1000000 || e.DefaultCardPrice < 1 || e.DefaultCardPrice > 1000000 {
		return e, fmt.Errorf("invalid progression economy values")
	}
	for id, c := range e.Characters {
		lib, ok := catalogs[id]
		if !ok {
			return e, fmt.Errorf("unknown economy character %q", id)
		}
		if c.StartingDeck != nil {
			if _, err = Validate(c.StartingDeck, lib.Cards); err != nil {
				return e, err
			}
		}
		if c.StartingAbilities != nil {
			if err = ValidateAbilities(*c.StartingAbilities, lib); err != nil {
				return e, err
			}
		}
		for card, price := range c.CardPrices {
			if _, ok = lib.Cards[card]; !ok || price < 1 || price > 1000000 {
				return e, fmt.Errorf("invalid card price for %q", card)
			}
		}
		for from, u := range c.CardUpgrades {
			_, a := lib.Cards[from]
			_, b := lib.Cards[u.To]
			if !a || !b || from == u.To || u.XP < 1 || u.XP > 1000000 {
				return e, fmt.Errorf("invalid card upgrade %q", from)
			}
		}
		for from, u := range c.AbilityUpgrades {
			a, okA := lib.Abilities[from]
			b, okB := lib.Abilities[u.To]
			if !okA || !okB || a.Type != b.Type || from == u.To || u.XP < 1 || u.XP > 1000000 {
				return e, fmt.Errorf("invalid ability upgrade %q", from)
			}
		}
	}
	return e, nil
}
func ValidateAbilities(board content.AbilityBoard, lib content.BattleLibrary) error {
	seen := map[string]bool{}
	for kind, ids := range map[string][]string{"offensive": board.Offensive, "defensive": board.Defensive} {
		if len(ids) == 0 || len(ids) > 20 {
			return fmt.Errorf("ability board needs 1–20 %s abilities", kind)
		}
		for _, id := range ids {
			ability, ok := lib.Abilities[id]
			if !ok || ability.Type != kind || seen[id] {
				return fmt.Errorf("invalid %s ability %q", kind, id)
			}
			seen[id] = true
		}
	}
	return nil
}
func (e Economy) Price(character, id string) int {
	if p, ok := e.Characters[character].CardPrices[id]; ok {
		return p
	}
	return e.DefaultCardPrice
}
func (e Economy) Offers(character string) map[string]any {
	c := e.Characters[character]
	return map[string]any{"default_card_price": e.DefaultCardPrice, "card_prices": c.CardPrices, "card_upgrades": c.CardUpgrades, "ability_upgrades": c.AbilityUpgrades}
}

// Progression files have their own namespace: sandbox edits never grant XP or cards.
func progressPath(root, character string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("loadout root is required")
	}
	return path(filepath.Join(root, "progression"), character)
}
func readProgress(root, character string, e Economy, lib content.BattleLibrary) (Progress, error) {
	filename, err := progressPath(root, character)
	if err != nil {
		return Progress{}, err
	}
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		c := lib.Combatants[character]
		cfg := e.Characters[character]
		p := Progress{Version: 1, Character: character, Revision: 1, XP: e.StartingXP, Abilities: c.AbilityBoard}
		for _, entry := range c.Decklist {
			p.Deck = append(p.Deck, Entry{CardID: entry.CardID, Count: entry.Count})
		}
		if cfg.StartingDeck != nil {
			p.Deck = append([]Entry(nil), cfg.StartingDeck...)
		}
		if cfg.StartingAbilities != nil {
			p.Abilities = *cfg.StartingAbilities
		}
		p.Abilities = content.AbilityBoard{Offensive: append([]string(nil), p.Abilities.Offensive...), Defensive: append([]string(nil), p.Abilities.Defensive...)}
		if err = validateProgress(p, lib); err != nil {
			return p, err
		}
		err = writeProgress(filename, p)
		return p, err
	}
	if err != nil {
		return Progress{}, err
	}
	var p Progress
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.Character != character {
		return p, fmt.Errorf("progression character mismatch")
	}
	if p.Deck == nil {
		p.Deck = []Entry{}
	}
	return p, validateProgress(p, lib)
}
func validateProgress(p Progress, lib content.BattleLibrary) error {
	if p.Version != 1 || p.Revision < 1 || p.XP < 0 {
		return fmt.Errorf("invalid progression state")
	}
	// An empty progression deck is valid between battles while rebuilding.
	if len(p.Deck) > 0 {
		if _, err := Validate(p.Deck, lib.Cards); err != nil {
			return err
		}
	}
	return ValidateAbilities(p.Abilities, lib)
}
func writeProgress(filename string, p Progress) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filename)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".progress-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filename)
}
func ReadProgress(root, character string, e Economy, lib content.BattleLibrary) (Progress, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	return readProgress(root, character, e, lib)
}
func Buy(root, character string, e Economy, lib content.BattleLibrary, request Purchase) (Progress, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	p, err := readProgress(root, character, e, lib)
	if err != nil {
		return p, err
	}
	if request.Revision != p.Revision {
		return p, fmt.Errorf("loadout changed; refresh before trading")
	}
	cost := 0
	cfg := e.Characters[character]
	switch request.Kind {
	case "buy_card":
		if _, ok := lib.Cards[request.ID]; !ok {
			return p, fmt.Errorf("card unavailable")
		}
		cost = e.Price(character, request.ID)
		p.Deck = changeCount(p.Deck, request.ID, 1)
	case "sell_card":
		if _, ok := lib.Cards[request.ID]; !ok || deckCount(p.Deck, request.ID) < 1 {
			return p, fmt.Errorf("no owned copy to sell")
		}
		cost = e.Price(character, request.ID)
		p.Deck = changeCount(p.Deck, request.ID, -1)
	case "upgrade_card":
		upgrade, ok := cfg.CardUpgrades[request.ID]
		if !ok || deckCount(p.Deck, request.ID) < 1 {
			return p, fmt.Errorf("no owned copy or upgrade path")
		}
		cost = upgrade.XP
		p.Deck = changeCount(p.Deck, request.ID, -1)
		p.Deck = changeCount(p.Deck, upgrade.To, 1)
	case "upgrade_ability":
		upgrade, ok := cfg.AbilityUpgrades[request.ID]
		if !ok {
			return p, fmt.Errorf("no ability upgrade path")
		}
		cost = upgrade.XP
		replaced := false
		for _, ids := range [][]string{p.Abilities.Offensive, p.Abilities.Defensive} {
			for i, id := range ids {
				if id == upgrade.To {
					return p, fmt.Errorf("upgraded ability already equipped")
				}
				if id == request.ID {
					ids[i] = upgrade.To
					replaced = true
				}
			}
		}
		if !replaced {
			return p, fmt.Errorf("ability is not equipped")
		}
	default:
		return p, fmt.Errorf("unknown transaction kind")
	}
	if cost != request.ExpectedCost {
		return p, fmt.Errorf("price changed; refresh before trading")
	}
	if request.Kind != "sell_card" && p.XP < cost {
		return p, fmt.Errorf("not enough XP")
	}
	if request.Kind == "sell_card" {
		p.XP += cost
	} else {
		p.XP -= cost
	}
	p.Revision++
	if err = validateProgress(p, lib); err != nil {
		return p, err
	}
	filename, _ := progressPath(root, character)
	if err = writeProgress(filename, p); err != nil {
		return p, err
	}
	return p, nil
}
func deckCount(deck []Entry, id string) int {
	for _, entry := range deck {
		if entry.CardID == id {
			return entry.Count
		}
	}
	return 0
}
func changeCount(deck []Entry, id string, delta int) []Entry {
	for i := range deck {
		if deck[i].CardID == id {
			deck[i].Count += delta
			if deck[i].Count == 0 {
				return append(deck[:i], deck[i+1:]...)
			}
			return deck
		}
	}
	return append(deck, Entry{CardID: id, Count: delta})
}
