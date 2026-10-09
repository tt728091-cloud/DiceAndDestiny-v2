package content

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"sort"
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
	// XP is this tree's value for the card: what a copy obtained through this
	// tree is worth. The tree owns it, so a shared card can cost a different
	// amount in each tree. Older stores fall back to the card's own economy.
	XP int `json:"xp,omitempty"`
	// Shared marks a card that exists once in the catalog and is lent to this
	// tree. Its definition (energy, effects, targets) is the single shared copy;
	// editing it in any tree changes it everywhere. It is never a tree's base.
	Shared bool `json:"shared,omitempty"`
}

// Value is the XP a copy of this node's card is worth in its tree.
func (n CardTreeNode) Value() int {
	if n.XP > 0 {
		return n.XP
	}
	if n.Card.Economy != nil {
		return n.Card.Economy.Buy
	}
	return 0
}

// TreeRef locates one placement of a card in a tree.
type TreeRef struct {
	Tree, Node string
	Shared     bool
}

// TreeCardPlacements lists every tree placement of a card, sorted by tree ID.
// An exclusive card has at most one; a shared card may have many.
func TreeCardPlacements(trees map[string]CardTree, card string) []TreeRef {
	var refs []TreeRef
	for _, id := range sortedTreeIDs(trees) {
		for _, n := range trees[id].Nodes {
			if n.Card.ID == card {
				refs = append(refs, TreeRef{Tree: id, Node: n.ID, Shared: n.Shared})
			}
		}
	}
	return refs
}

// IsSharedCard reports whether a card is lent to trees rather than owned by one.
func IsSharedCard(trees map[string]CardTree, card string) bool {
	refs := TreeCardPlacements(trees, card)
	return len(refs) > 0 && refs[0].Shared
}

// CardNode finds the node holding a card in this tree.
func (t CardTree) CardNode(card string) (CardTreeNode, bool) {
	for _, n := range t.Nodes {
		if n.Card.ID == card {
			return n, true
		}
	}
	return CardTreeNode{}, false
}

func sortedTreeIDs(trees map[string]CardTree) []string {
	ids := make([]string, 0, len(trees))
	for id := range trees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
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

// TreeCardOwner returns the first tree placement of a card. For a shared card
// this is one of several trees; callers that value or move copies must use the
// copy's own tree instead (see TreeCardPlacements).
func TreeCardOwner(trees map[string]CardTree, card string) (string, string, bool) {
	for _, id := range sortedTreeIDs(trees) {
		t := trees[id]
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
	// A card is either exclusive to one tree node or shared: shared cards are
	// never a base and may be lent to many trees, but only once per tree.
	placements := map[string][]TreeRef{}
	for _, id := range sortedTreeIDs(trees) {
		t := trees[id]
		for _, n := range t.Nodes {
			placements[n.Card.ID] = append(placements[n.Card.ID], TreeRef{Tree: id, Node: n.ID, Shared: n.Shared})
			if n.Shared && n.ID == t.Root {
				return fmt.Errorf("%s: a tree's base cannot be a shared card", t.Name)
			}
		}
	}
	for card, refs := range placements {
		seen := map[string]bool{}
		for _, r := range refs {
			if seen[r.Tree] {
				return fmt.Errorf("card %s appears twice in tree %s", card, r.Tree)
			}
			seen[r.Tree] = true
			if len(refs) > 1 && !r.Shared {
				return fmt.Errorf("card %s is already used by tree %s; share it to use it in more trees", card, refs[0].Tree)
			}
		}
	}
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
			if n.Value() < 1 {
				return fmt.Errorf("tree cards need a positive XP value")
			}
			if n.Shared {
				// The tree owns the XP; the shared card itself has no price.
				if c := lib.Cards[n.Card.ID].Economy; c != nil && (c.Buy != 0 || c.Sell != 0 || len(c.Upgrades) != 0) {
					return fmt.Errorf("shared card %s takes its XP from each tree, not its own price", n.Card.Name)
				}
			} else if n.Card.Economy == nil || n.Card.Economy.Sell != n.Card.Economy.Buy || n.Card.Economy.Buy != n.Value() {
				return fmt.Errorf("tree cards need equal buy/sell values matching the tree's XP")
			} else if len(n.Card.Economy.Upgrades) != 0 {
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
			for _, ref := range TreeCardPlacements(trees, r.CardID) {
				t, node := trees[ref.Tree], ref.Node
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
// Shared cards are stored once: publishing a tree that edits one updates the
// single definition and every other tree that lends it.
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
		if !n.Shared {
			oldCards[n.Card.ID] = true
		}
	}
	root, hasRoot := t.Node(t.Root)
	if !hasRoot {
		return saved, lib, fmt.Errorf("select a base card first")
	}
	if root.Shared {
		return saved, lib, fmt.Errorf("a tree's base cannot be a shared card")
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
	others := maps.Clone(saved.Trees)
	delete(others, t.ID)
	retained := map[string]bool{}
	sharedCards := map[string]BattleCardDefinition{}
	t.Nodes = append([]CardTreeNode(nil), t.Nodes...)
	for i := range t.Nodes {
		n := &t.Nodes[i]
		if ProgramContains(saved.Deleted, n.Card.ID) {
			return saved, lib, fmt.Errorf("card ID was deleted: %s", n.Card.ID)
		}
		if n.Card.Program == nil && n.Card.Mechanic == nil {
			return saved, lib, fmt.Errorf("tree node needs a configurable card")
		}
		if n.XP < 1 {
			n.XP = n.Value()
		}
		if n.Shared {
			// A shared card is its own catalog card: never a tree variant ID,
			// never another tree's base or exclusive card.
			if strings.Contains(n.Card.ID, "_variant_") {
				return saved, lib, fmt.Errorf("shared card %s needs its own card ID, not a tree variant ID", n.Card.Name)
			}
			if err := validateStableNamed("card", n.Card.ID, n.Card.Name); err != nil {
				return saved, lib, err
			}
			for _, ref := range TreeCardPlacements(others, n.Card.ID) {
				if !ref.Shared {
					return saved, lib, fmt.Errorf("%s belongs to tree %s; share it there first", n.Card.Name, ref.Tree)
				}
			}
			limit := 20
			if n.Card.Economy != nil && n.Card.Economy.CopyLimit > 0 {
				limit = n.Card.Economy.CopyLimit
			}
			n.Card.Economy = &CardEconomy{CopyLimit: limit}
		} else {
			if n.ID != t.Root && n.Card.ID != t.ID+"_variant_"+n.ID {
				return saved, lib, fmt.Errorf("variant IDs must be tree_id_variant_node_id")
			}
			if _, exists := lib.Cards[n.Card.ID]; exists && !oldCards[n.Card.ID] && n.ID != t.Root {
				return saved, lib, fmt.Errorf("variant ID already exists: %s", n.Card.ID)
			}
			// The tree owns the XP; the card's price mirrors it for every lookup.
			if n.Card.Economy == nil {
				n.Card.Economy = &CardEconomy{CopyLimit: 20}
			}
			n.Card.Economy.Buy, n.Card.Economy.Sell = n.XP, n.XP
		}
		n.Card = PrepareProgramCard(n.Card, &lib)
		n.Card = PrepareMechanicCard(n.Card, &lib)
		saved.Cards[n.Card.ID] = n.Card
		lib.Cards[n.Card.ID] = n.Card
		retained[n.Card.ID] = true
		if n.Shared {
			sharedCards[n.Card.ID] = n.Card
		}
	}
	for id := range oldCards {
		if !retained[id] {
			delete(saved.Cards, id)
			delete(lib.Cards, id)
		}
	}
	// A shared card no tree lends any more has no XP anywhere, so it leaves the
	// catalog rather than linger as a free card. Owned copies block this publish.
	for _, n := range old.Nodes {
		if n.Shared && !retained[n.Card.ID] && len(TreeCardPlacements(others, n.Card.ID)) == 0 {
			delete(saved.Cards, n.Card.ID)
			delete(lib.Cards, n.Card.ID)
		}
	}
	saved.Trees[t.ID] = t
	// Every tree lending a shared card shows its one current definition.
	for id, other := range others {
		changed := false
		nodes := append([]CardTreeNode(nil), other.Nodes...)
		for i := range nodes {
			if card, ok := sharedCards[nodes[i].Card.ID]; ok && nodes[i].Shared {
				before, _ := json.Marshal(nodes[i].Card)
				after, _ := json.Marshal(card)
				if string(before) != string(after) {
					nodes[i].Card = card
					changed = true
				}
			}
		}
		if changed {
			other.Nodes = nodes
			saved.Trees[id] = other
		}
	}
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
