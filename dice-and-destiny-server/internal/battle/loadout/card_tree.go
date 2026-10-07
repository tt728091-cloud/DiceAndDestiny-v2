package loadout

import (
	"diceanddestiny/server/internal/content"
	"fmt"
)

func ValidateTreeDeck(deck []Entry, trees map[string]content.CardTree) error {
	counts := map[string]int{}
	for _, e := range deck {
		counts[e.CardID] += e.Count
	}
	for _, t := range trees {
		for _, n := range t.Nodes {
			if counts[n.Card.ID] > 0 && !t.LegalNode(n.ID, counts, trees) {
				return fmt.Errorf("%s: deck does not meet a complete path's requirements in %s", n.Card.Name, t.Name)
			}
		}
	}
	return nil
}
func TreeTransition(deck []Entry, from, to string, e Economy, lib content.BattleLibrary, character string) ([]Entry, int, error) {
	if deckCount(deck, from) < 1 {
		return deck, 0, fmt.Errorf("you need an owned copy of the starting node")
	}
	owner, start, _ := content.TreeCardOwner(lib.CardTrees, from)
	targetOwner, target, _ := content.TreeCardOwner(lib.CardTrees, to)
	t, ok := lib.CardTrees[owner]
	if !ok || owner != targetOwner || start == target {
		return deck, 0, fmt.Errorf("no matching card tree path")
	}
	candidate := append([]Entry(nil), deck...)
	candidate = changeCount(candidate, from, -1)
	candidate = changeCount(candidate, to, 1)
	legal := false
	for _, edge := range t.Edges {
		if edge.From == start && edge.To == target || edge.Reversible && edge.To == start && edge.From == target {
			legal = true
			break
		}
	}
	if !legal {
		return deck, 0, fmt.Errorf("no connection between these cards")
	}
	if err := e.Access.Check(character, "cards", to); err != nil {
		return deck, 0, err
	}
	return candidate, e.Price(character, to) - e.Price(character, from), nil
}

// Collection cards are owned but not equipped and do not contribute health.
// Deck requirements apply at equip time, including to all cards already equipped.
func MoveCollectionCard(p Progress, id string, equip bool, e Economy, lib content.BattleLibrary, character string) (Progress, error) {
	p.Deck = append([]Entry(nil), p.Deck...)
	p.Collection = append([]Entry(nil), p.Collection...)
	if equip {
		if deckCount(p.Collection, id) < 1 {
			return p, fmt.Errorf("no collected copy to equip")
		}
		if err := e.Access.Check(character, "cards", id); err != nil {
			return p, err
		}
		p.Collection = changeCount(p.Collection, id, -1)
		p.Deck = changeCount(p.Deck, id, 1)
	} else {
		if deckCount(p.Deck, id) < 1 {
			return p, fmt.Errorf("no equipped copy to move")
		}
		p.Deck = changeCount(p.Deck, id, -1)
		p.Collection = changeCount(p.Collection, id, 1)
	}
	if len(p.Deck) > 0 {
		if _, err := Validate(p.Deck, lib.Cards); err != nil {
			return p, err
		}
	}
	if err := ValidateTreeDeck(p.Deck, lib.CardTrees); err != nil {
		return p, err
	}
	return p, nil
}

func UpgradeTreeCard(p Progress, from, to string, e Economy, lib content.BattleLibrary, character string) (Progress, int, error) {
	p.Deck = append([]Entry(nil), p.Deck...)
	p.Collection = append([]Entry(nil), p.Collection...)
	if deckCount(p.Collection, from) == 0 && deckCount(p.Deck, from) > 0 {
		p.Deck = changeCount(p.Deck, from, -1)
		p.Collection = changeCount(p.Collection, from, 1)
	}
	var cost int
	var err error
	p.Collection, cost, err = TreeTransition(p.Collection, from, to, e, lib, character)
	if err != nil {
		return p, 0, err
	}
	if err = ValidateTreeDeck(p.Deck, lib.CardTrees); err != nil {
		return p, 0, err
	}
	return p, cost, nil
}
