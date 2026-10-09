package loadout

import (
	"diceanddestiny/server/internal/content"
	"fmt"
)

// validateTreeTags checks that shared-card copies name a tree that lends the
// card, and that no other card carries a tree tag.
func validateTreeTags(deck []Entry, trees map[string]content.CardTree) error {
	for _, e := range deck {
		if !content.IsSharedCard(trees, e.CardID) {
			if e.Tree != "" {
				return fmt.Errorf("%s is not a shared card; its copies cannot name a tree", e.CardID)
			}
			continue
		}
		n, ok := trees[e.Tree].CardNode(e.CardID)
		if !ok || !n.Shared {
			return fmt.Errorf("shared card %s needs the card tree its copies came through", e.CardID)
		}
	}
	return nil
}

func ValidateTreeDeck(deck []Entry, trees map[string]content.CardTree) error {
	if err := validateTreeTags(deck, trees); err != nil {
		return err
	}
	// Requirements count every copy of a card; a shared card's path is checked
	// only in the trees its equipped copies came through.
	counts := map[string]int{}
	fromTree := map[string]int{}
	for _, e := range deck {
		counts[e.CardID] += e.Count
		fromTree[e.Tree+"|"+e.CardID] += e.Count
	}
	for _, t := range trees {
		for _, n := range t.Nodes {
			present := counts[n.Card.ID] > 0
			if n.Shared {
				present = fromTree[t.ID+"|"+n.Card.ID] > 0
			}
			if present && !t.LegalNode(n.ID, counts, trees) {
				return fmt.Errorf("%s: deck does not meet a complete path's requirements in %s", n.Card.Name, t.Name)
			}
		}
	}
	return nil
}

// treeTrade resolves one connection inside one tree. Exclusive cards imply
// their tree; a trade between shared cards must name it. Copies of a shared
// card are tagged with the trade's tree, so they never cross into another tree.
type treeTrade struct {
	tree           content.CardTree
	start, target  content.CardTreeNode
	fromTag, toTag string
}

func resolveTreeTrade(lib content.BattleLibrary, from, to, tree string) (treeTrade, error) {
	if tree == "" {
		for _, id := range []string{from, to} {
			if refs := content.TreeCardPlacements(lib.CardTrees, id); len(refs) == 1 && !refs[0].Shared {
				tree = refs[0].Tree
				break
			}
		}
	}
	t, ok := lib.CardTrees[tree]
	if !ok {
		return treeTrade{}, fmt.Errorf("no matching card tree path")
	}
	start, okStart := t.CardNode(from)
	target, okTarget := t.CardNode(to)
	if !okStart || !okTarget || start.ID == target.ID {
		return treeTrade{}, fmt.Errorf("no matching card tree path")
	}
	tag := func(n content.CardTreeNode) string {
		if n.Shared {
			return t.ID
		}
		return ""
	}
	return treeTrade{tree: t, start: start, target: target, fromTag: tag(start), toTag: tag(target)}, nil
}

func TreeTransition(deck []Entry, from, to, tree string, e Economy, lib content.BattleLibrary, character string) ([]Entry, int, error) {
	trade, err := resolveTreeTrade(lib, from, to, tree)
	if err != nil {
		return deck, 0, err
	}
	if deckCountIn(deck, from, trade.fromTag) < 1 {
		return deck, 0, fmt.Errorf("you need an owned copy of the starting node")
	}
	legal := false
	for _, edge := range trade.tree.Edges {
		if edge.From == trade.start.ID && edge.To == trade.target.ID || edge.Reversible && edge.To == trade.start.ID && edge.From == trade.target.ID {
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
	candidate := append([]Entry(nil), deck...)
	candidate = changeCountIn(candidate, from, trade.fromTag, -1)
	candidate = changeCountIn(candidate, to, trade.toTag, 1)
	// The tree owns both values, so the cost is the XP between its two nodes.
	return candidate, trade.target.Value() - trade.start.Value(), nil
}

// Collection cards are owned but not equipped and do not contribute health.
// Deck requirements apply at equip time, including to all cards already equipped.
func MoveCollectionCard(p Progress, id, tree string, equip bool, e Economy, lib content.BattleLibrary, character string) (Progress, error) {
	p.Deck = append([]Entry(nil), p.Deck...)
	p.Collection = append([]Entry(nil), p.Collection...)
	source := p.Deck
	if equip {
		source = p.Collection
	}
	tag, err := copyTree(source, id, tree, lib.CardTrees)
	if err != nil {
		return p, err
	}
	if equip {
		if deckCountIn(p.Collection, id, tag) < 1 {
			return p, fmt.Errorf("no collected copy to equip")
		}
		if err := e.Access.Check(character, "cards", id); err != nil {
			return p, err
		}
		p.Collection = changeCountIn(p.Collection, id, tag, -1)
		p.Deck = changeCountIn(p.Deck, id, tag, 1)
	} else {
		if deckCountIn(p.Deck, id, tag) < 1 {
			return p, fmt.Errorf("no equipped copy to move")
		}
		p.Deck = changeCountIn(p.Deck, id, tag, -1)
		p.Collection = changeCountIn(p.Collection, id, tag, 1)
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

func UpgradeTreeCard(p Progress, from, to, tree string, e Economy, lib content.BattleLibrary, character string) (Progress, int, error) {
	trade, err := resolveTreeTrade(lib, from, to, tree)
	if err != nil {
		return p, 0, err
	}
	p.Deck = append([]Entry(nil), p.Deck...)
	p.Collection = append([]Entry(nil), p.Collection...)
	if deckCountIn(p.Collection, from, trade.fromTag) == 0 && deckCountIn(p.Deck, from, trade.fromTag) > 0 {
		p.Deck = changeCountIn(p.Deck, from, trade.fromTag, -1)
		p.Collection = changeCountIn(p.Collection, from, trade.fromTag, 1)
	}
	var cost int
	p.Collection, cost, err = TreeTransition(p.Collection, from, to, trade.tree.ID, e, lib, character)
	if err != nil {
		return p, 0, err
	}
	if err = ValidateTreeDeck(p.Deck, lib.CardTrees); err != nil {
		return p, 0, err
	}
	return p, cost, nil
}
