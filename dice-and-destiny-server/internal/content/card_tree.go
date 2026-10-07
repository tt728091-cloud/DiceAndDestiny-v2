package content

import (
	"fmt"
	"maps"
	"math"
	"strings"
)

// Card trees are directed acyclic graphs. A node is a complete card variant,
// so converging paths have one unambiguous result rather than order-dependent patches.
type CardTree struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Root  string         `json:"root"`
	Nodes []CardTreeNode `json:"nodes"`
	Edges []CardTreeEdge `json:"edges"`
}
type CardTreeNode struct {
	ID   string               `json:"id"`
	Card BattleCardDefinition `json:"card"`
	X    float64              `json:"x"`
	Y    float64              `json:"y"`
}
type CardTreeEdge struct {
	ID           string                `json:"id"`
	From         string                `json:"from"`
	To           string                `json:"to"`
	Reversible   bool                  `json:"reversible"`
	Requirements []CardTreeRequirement `json:"requirements"`
}
type CardTreeRequirement struct {
	CardID      string `json:"card_id"`
	Minimum     int    `json:"minimum"`
	Maximum     *int   `json:"maximum,omitempty"`
	Descendants bool   `json:"include_descendants"`
}

func TreeCardOwner(trees map[string]CardTree, card string) (string, string, bool) {
	for id, t := range trees {
		for _, n := range t.Nodes {
			if n.Card.ID == card {
				return id, n.ID, n.ID != t.Root
			}
		}
	}
	return "", "", false
}
func (t CardTree) Node(id string) (CardTreeNode, bool) {
	for _, n := range t.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return CardTreeNode{}, false
}
func ValidateCardTrees(trees map[string]CardTree, lib BattleLibrary) error {
	owners := map[string]string{}
	for id, t := range trees {
		if id != t.ID {
			return fmt.Errorf("tree ID mismatch")
		}
		if err := validateStableNamed("card tree", t.ID, t.Name); err != nil {
			return err
		}
		if len(t.Nodes) < 1 || len(t.Nodes) > 100 || len(t.Edges) > 300 {
			return fmt.Errorf("tree needs 1–100 nodes and at most 300 connections")
		}
		nodes := map[string]bool{}
		pairs := map[string]bool{}
		edgeIDs := map[string]bool{}
		for _, n := range t.Nodes {
			if err := validateStableNamed("tree node", n.ID, n.Card.Name); err != nil {
				return err
			}
			if nodes[n.ID] {
				return fmt.Errorf("duplicate tree node %s", n.ID)
			}
			nodes[n.ID] = true
			if _, exists := lib.Cards[n.Card.ID]; !exists {
				return fmt.Errorf("tree card %s is missing", n.Card.ID)
			}
			if owner := owners[n.Card.ID]; owner != "" {
				return fmt.Errorf("card %s is already used by tree %s", n.Card.ID, owner)
			}
			owners[n.Card.ID] = id
			if n.Card.Economy == nil || n.Card.Economy.Buy < 1 || n.Card.Economy.Sell != n.Card.Economy.Buy {
				return fmt.Errorf("tree cards need a positive XP value and equal buy/sell values")
			}
			if len(n.Card.Economy.Upgrades) != 0 {
				return fmt.Errorf("use tree connections instead of legacy card upgrades")
			}
			if math.IsNaN(n.X) || math.IsNaN(n.Y) || math.IsInf(n.X, 0) || math.IsInf(n.Y, 0) || math.Abs(n.X) > 10000 || math.Abs(n.Y) > 10000 {
				return fmt.Errorf("node position must be within 10000 units")
			}
		}
		if !nodes[t.Root] {
			return fmt.Errorf("tree root is missing")
		}
		for _, e := range t.Edges {
			if err := validateStableNamed("connection", e.ID, e.ID); err != nil {
				return err
			}
			if edgeIDs[e.ID] || pairs[e.From+"/"+e.To] {
				return fmt.Errorf("duplicate connection")
			}
			edgeIDs[e.ID] = true
			pairs[e.From+"/"+e.To] = true
			if !nodes[e.From] || !nodes[e.To] || e.From == e.To || e.To == t.Root {
				return fmt.Errorf("connection must lead between distinct nodes and cannot lead into the base")
			}
			if len(e.Requirements) > 20 {
				return fmt.Errorf("at most 20 requirements per connection")
			}
			for _, r := range e.Requirements {
				if _, ok := lib.Cards[r.CardID]; !ok {
					return fmt.Errorf("requirement card %s is missing", r.CardID)
				}
				if r.Minimum < 0 || r.Minimum > 100 || (r.Maximum != nil && (*r.Maximum < r.Minimum || *r.Maximum > 100)) || (r.Minimum == 0 && r.Maximum == nil) {
					return fmt.Errorf("invalid requirement count for %s", r.CardID)
				}
			}
		}
		state := map[string]int{}
		var visit func(string) error
		visit = func(n string) error {
			if state[n] == 1 {
				return fmt.Errorf("tree connections must not form a cycle")
			}
			if state[n] == 2 {
				return nil
			}
			state[n] = 1
			for _, e := range t.Edges {
				if e.From == n {
					if err := visit(e.To); err != nil {
						return err
					}
				}
			}
			state[n] = 2
			return nil
		}
		if err := visit(t.Root); err != nil {
			return err
		}
		if len(state) != len(nodes) {
			return fmt.Errorf("every node must be connected to the base")
		}
	}
	return nil
}

// Each path's gates remain active while its variant is equipped. An alternative
// path may satisfy a converging node; all gates along that path must pass.
func (t CardTree) LegalNode(node string, counts map[string]int, trees map[string]CardTree) bool {
	memo := map[string]bool{t.Root: true}
	done := map[string]bool{t.Root: true}
	var visit func(string) bool
	visit = func(id string) bool {
		if done[id] {
			return memo[id]
		}
		done[id] = true
		for _, e := range t.Edges {
			if e.To == id && TreeRequirementsMet(e.Requirements, counts, trees) && visit(e.From) {
				memo[id] = true
				return true
			}
		}
		return false
	}
	return visit(node)
}
func TreeRequirementsMet(requirements []CardTreeRequirement, counts map[string]int, trees map[string]CardTree) bool {
	for _, r := range requirements {
		ids := map[string]bool{r.CardID: true}
		if r.Descendants {
			owner, node, _ := TreeCardOwner(trees, r.CardID)
			if t, ok := trees[owner]; ok {
				seen := map[string]bool{}
				var walk func(string)
				walk = func(id string) {
					if seen[id] {
						return
					}
					seen[id] = true
					n, _ := t.Node(id)
					ids[n.Card.ID] = true
					for _, e := range t.Edges {
						if e.From == id {
							walk(e.To)
						}
					}
				}
				walk(node)
			}
		}
		count := 0
		for id := range ids {
			count += counts[id]
		}
		if count < r.Minimum || r.Maximum != nil && count > *r.Maximum {
			return false
		}
	}
	return true
}

// Build one atomic authored-card store revision, including all graph metadata
// and variants. Removed variants cannot leave stale catalog references behind.
func PreviewCardTree(saved AuthoredCards, lib BattleLibrary, t CardTree) (AuthoredCards, BattleLibrary, error) {
	saved.Cards = maps.Clone(saved.Cards)
	saved.Trees = maps.Clone(saved.Trees)
	if saved.Trees == nil {
		saved.Trees = map[string]CardTree{}
	}
	lib.Cards = maps.Clone(lib.Cards)
	old, editing := saved.Trees[t.ID]
	oldCards := map[string]bool{}
	for _, n := range old.Nodes {
		oldCards[n.Card.ID] = true
	}
	root, hasRoot := t.Node(t.Root)
	if !hasRoot {
		return saved, lib, fmt.Errorf("select a base card first")
	}
	if editing {
		oldRoot, _ := old.Node(old.Root)
		if root.Card.ID != oldRoot.Card.ID || t.Root != old.Root {
			return saved, lib, fmt.Errorf("a published tree keeps its base; create a new tree for another base")
		}
	}
	if _, ok := lib.Cards[root.Card.ID]; !ok {
		// A new tree may author its own base card. It is validated with the rest
		// of the revision and must not reuse a deleted or tree-variant ID.
		if editing || root.Card.ID == "" || strings.Contains(root.Card.ID, "_variant_") {
			return saved, lib, fmt.Errorf("base card is unavailable")
		}
		if err := validateStableNamed("card", root.Card.ID, root.Card.Name); err != nil {
			return saved, lib, err
		}
	}
	retained := map[string]bool{}
	for i := range t.Nodes {
		n := &t.Nodes[i]
		if n.ID != t.Root && n.Card.ID != t.ID+"_variant_"+n.ID {
			return saved, lib, fmt.Errorf("variant IDs must be tree_id_variant_node_id")
		}
		if _, exists := lib.Cards[n.Card.ID]; exists && !oldCards[n.Card.ID] && n.ID != t.Root {
			return saved, lib, fmt.Errorf("variant ID already exists: %s", n.Card.ID)
		}
		if ProgramContains(saved.Deleted, n.Card.ID) {
			return saved, lib, fmt.Errorf("card ID was deleted: %s", n.Card.ID)
		}
		if n.Card.Program == nil && n.Card.Mechanic == nil {
			return saved, lib, fmt.Errorf("tree node needs a configurable card")
		}
		n.Card = PrepareProgramCard(n.Card, &lib)
		n.Card = PrepareMechanicCard(n.Card, &lib)
		saved.Cards[n.Card.ID] = n.Card
		lib.Cards[n.Card.ID] = n.Card
		retained[n.Card.ID] = true
	}
	for id := range oldCards {
		if !retained[id] {
			delete(saved.Cards, id)
			delete(lib.Cards, id)
		}
	}
	saved.Trees[t.ID] = t
	var err error
	lib, err = applyAuthoredCards(lib, saved)
	return saved, lib, err
}
func SaveCardTree(root string, lib BattleLibrary, t CardTree, revision int) (AuthoredCards, error) {
	authoredMu.Lock()
	defer authoredMu.Unlock()
	saved, err := ReadAuthoredCards(root)
	if err != nil {
		return saved, err
	}
	if saved.Revision != revision {
		return saved, fmt.Errorf("catalog changed; reload the tree before publishing")
	}
	saved, _, err = PreviewCardTree(saved, lib, t)
	if err != nil {
		return saved, err
	}
	saved.Revision++
	return saved, writeAuthoredCards(root, saved)
}
