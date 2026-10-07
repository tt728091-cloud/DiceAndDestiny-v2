package loadout

import (
	"os"
	"reflect"
	"sort"

	"diceanddestiny/server/internal/content"
)

func templateDeck(lib content.BattleLibrary, character string) []Entry {
	var deck []Entry
	for _, entry := range lib.Combatants[character].Decklist {
		deck = append(deck, Entry{entry.CardID, entry.Count})
	}
	return deck
}
func sameDeck(a, b []Entry) bool {
	a = append([]Entry(nil), a...)
	b = append([]Entry(nil), b...)
	sort.Slice(a, func(i, j int) bool { return a[i].CardID < a[j].CardID })
	sort.Slice(b, func(i, j int) bool { return b[i].CardID < b[j].CardID })
	return reflect.DeepEqual(a, b)
}

// Free edits change the character's deck allocation, preserving spendable XP.
// The adjustment also survives later admin repricing/budget reconciliation.
func replaceDeck(p *Progress, deck []Entry, e Economy) {
	delta := deckValue(deck, p.Character, e) - p.DeckValue
	p.Deck = append([]Entry{}, deck...)
	p.FreeDeckBudget += delta
	budget := *p.Budget + delta
	p.Budget = &budget
	p.DeckValue += delta
	p.Revision++
}

// Migrate old separate saves once; thereafter the shared ledger is authoritative.
func migrateSharedDeck(root string, p *Progress, e Economy, lib content.BattleLibrary) error {
	if p.SharedDeck {
		return nil
	}
	deck, err := ReadLegacy(root, p.Character, lib.Cards)
	if err != nil {
		return err
	}
	if deck != nil {
		sandboxPath, _ := path(root, p.Character)
		progressFile, _ := progressPath(root, p.Character)
		a, err := os.Stat(sandboxPath)
		if err != nil {
			return err
		}
		b, err := os.Stat(progressFile)
		if err != nil {
			return err
		}
		// Retain the most recently saved deck when both old modes were customized.
		if a.ModTime().After(b.ModTime()) && !sameDeck(deck, p.Deck) {
			replaceDeck(p, deck, e)
		}
	} else if p.Character == "adventurer" {
		// The retired progression starter downgraded the template's Brace+.
		old := []Entry{{"brace", 3}, {"nudge", 2}, {"try_again", 2}, {"strong_swing", 2}, {"take_stock", 2}, {"second_wind", 1}}
		if sameDeck(p.Deck, old) {
			replaceDeck(p, templateDeck(lib, p.Character), e)
		}
	}
	p.SharedDeck = true
	return nil
}

func WriteSharedDeck(root, character string, deck []Entry, e Economy, lib content.BattleLibrary) ([]Entry, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	var err error
	if err = ValidateTreeDeck(deck, lib.CardTrees); err != nil {
		return nil, err
	}
	deck, err = Validate(deck, lib.Cards)
	if err != nil {
		return nil, err
	}
	e, _, err = effectiveEconomy(root, e)
	if err != nil {
		return nil, err
	}
	if err = e.Access.ValidateDeck(character, deck); err != nil {
		return nil, err
	}
	p, err := readProgress(root, character, e, lib)
	if err != nil {
		return nil, err
	}
	if sameDeck(p.Deck, deck) {
		return p.Deck, nil
	}
	replaceDeck(&p, deck, e)
	filename, _ := progressPath(root, character)
	if err = writeProgress(filename, p); err != nil {
		return nil, err
	}
	return p.Deck, nil
}
