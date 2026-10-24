// Command card-tree-shift moves a whole card tree with its base: it sets
// the base card's XP and shifts every other node's XP by the same amount,
// and shifts every non-base card's energy by -energy (free cards stay free,
// and no cost drops below 0). A shared card's energy changes everywhere it
// is lent; its XP in this tree is copied to every other tree lending it, so
// a shared card costs the same in each.
//
// The tree is published the way the card editor publishes it: previewed for
// every character, validated against a loadout root (copy your saves there
// first), then saved with the catalog revision bumped. -dry-run validates
// and prints the changes without saving.
//
//	go run ./cmd/card-tree-shift -tree nudge_paths -base-xp 18 -energy 1 \
//	  -loadout-root /tmp/copy-of-character_loadouts -dry-run
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards to edit")
	loadoutRoot := flag.String("loadout-root", "", "saves to validate against (a copy of character_loadouts); required")
	treeID := flag.String("tree", "", "card tree ID, e.g. nudge_paths")
	baseXP := flag.Int("base-xp", 0, "the base card's new XP; every node moves by the same amount (0 keeps XP)")
	energy := flag.Int("energy", 0, "energy shift for every card except the base")
	dryRun := flag.Bool("dry-run", false, "validate and print the changes without saving")
	flag.Parse()
	if *treeID == "" || *loadoutRoot == "" {
		fail(fmt.Errorf("-tree and -loadout-root are required"))
	}
	// The launcher's authored root, as in a normal run.
	os.Setenv(content.AuthoredRootEnv, *authoredRoot)

	saved, err := content.ReadAuthoredCards(*loadoutRoot)
	if err != nil {
		fail(err)
	}
	tree, ok := saved.Trees[*treeID]
	if !ok {
		fail(fmt.Errorf("no tree %q", *treeID))
	}
	base, _ := tree.Node(tree.Root)
	dXP := 0
	if *baseXP > 0 {
		dXP = *baseXP - base.Value()
	}
	shared := map[string]int{} // shared card -> its new XP in this tree
	fmt.Printf("%-24s %-28s %13s %13s\n", "node", "card", "energy", "XP")
	tree.Nodes = append([]content.CardTreeNode(nil), tree.Nodes...)
	for i := range tree.Nodes {
		n := &tree.Nodes[i]
		oldEnergy, oldXP := n.Card.Cost.Energy, n.Value()
		if n.ID != tree.Root && *energy != 0 && oldEnergy > 0 {
			n.Card.Cost.Energy = max(0, oldEnergy+*energy)
		}
		n.XP = oldXP + dXP
		if n.Shared {
			shared[n.Card.ID] = n.XP
		}
		fmt.Printf("%-24s %-28s %5d → %-5d %5d → %-5d%s\n", n.ID, n.Card.Name, oldEnergy, n.Card.Cost.Energy, oldXP, n.XP, map[bool]string{true: "  (shared)"}[n.Shared])
	}
	if err := publish(*contentRoot, *loadoutRoot, &saved, tree, *dryRun); err != nil {
		fail(err)
	}
	// Other trees lending a shared card take its new XP. Its definition
	// (energy) already follows the one shared copy.
	var others []string
	for id := range saved.Trees {
		if id != *treeID {
			others = append(others, id)
		}
	}
	sort.Strings(others)
	for _, id := range others {
		other := saved.Trees[id]
		other.Nodes = append([]content.CardTreeNode(nil), other.Nodes...)
		changed := false
		for i := range other.Nodes {
			n := &other.Nodes[i]
			if xp, ok := shared[n.Card.ID]; ok && n.Shared && n.Value() != xp {
				fmt.Printf("%s: %-14s XP %d → %d\n", id, n.Card.Name, n.Value(), xp)
				n.XP, changed = xp, true
			}
		}
		if changed {
			if err := publish(*contentRoot, *loadoutRoot, &saved, other, *dryRun); err != nil {
				fail(fmt.Errorf("%s: %w", id, err))
			}
		}
	}
	if *dryRun {
		fmt.Println("dry run: validated, nothing saved")
	} else {
		fmt.Printf("saved, catalog revision %d\n", saved.Revision)
	}
}

// publish previews the tree for every character, validates the saves and,
// unless dryRun, saves it (bumping the revision). saved follows the result.
func publish(contentRoot, loadoutRoot string, saved *content.AuthoredCards, tree content.CardTree, dryRun bool) error {
	catalogs, err := learned.CharacterCatalogs(contentRoot, loadoutRoot)
	if err != nil {
		return err
	}
	lib := catalogs["adventurer"]
	next, _, err := content.PreviewCardTree(*saved, lib, tree)
	if err != nil {
		return err
	}
	for id, candidate := range catalogs {
		_, updated, err := content.PreviewCardTree(*saved, candidate, tree)
		if err != nil {
			return err
		}
		catalogs[id] = updated
	}
	economy, err := loadout.LoadEconomy(contentRoot, catalogs)
	if err != nil {
		return err
	}
	if err := loadout.ValidateCatalogUpdate(loadoutRoot, economy, catalogs); err != nil {
		return fmt.Errorf("the saves would break: %w", err)
	}
	if dryRun {
		*saved = next
		return nil
	}
	written, err := content.SaveCardTree(loadoutRoot, lib, tree, saved.Revision)
	if err != nil {
		return err
	}
	*saved = written
	return nil
}

func defaultServerRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
