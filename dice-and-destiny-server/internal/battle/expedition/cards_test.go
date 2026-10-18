package expedition

import (
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

// authoredLibrary overlays the tracked authored cards (read-only) onto the
// character library, as the game does for the Starter.
func authoredLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	lib := testLibrary(t, contentRoot(t))
	lib, err := content.OverlayAuthoredCards(lib, filepath.Join(contentRoot(t), "authored"))
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestAuthoredMapCardsAndTrees(t *testing.T) {
	lib := authoredLibrary(t)
	// Take Stock and Second Wind are "both" cards: battle program + map effect.
	for id, kind, amount := "take_stock", "draw", 2; ; {
		card := lib.Cards[id]
		if card.MapEffect == nil || card.MapEffect.Kind != kind || card.MapEffect.Amount != amount || card.Program == nil {
			t.Fatalf("%s should be a 'both' card with map %s %d: %+v", id, kind, amount, card.MapEffect)
		}
		if !strings.Contains(card.Presentation.EffectSummary, "Map: ") || !strings.Contains(card.Presentation.RulesText, "Map: ") {
			t.Fatalf("%s face/rules must show the map effect: %q / %q", id, card.Presentation.EffectSummary, card.Presentation.RulesText)
		}
		if id == "second_wind" {
			break
		}
		id, kind, amount = "second_wind", "energy", 15
	}
	// The map trees: map-only cards that are blank in battle, and Foresight's
	// "both" cards that look at the top of the draw pile.
	shape := map[string][2]int{"pathfinder_paths": {9, 8}, "lookout_paths": {6, 5}, "unseen_paths": {6, 5}, "wayfarer_paths": {4, 3}, "foresight_paths": {7, 8}}
	for id, n := range shape {
		tree, ok := lib.CardTrees[id]
		if !ok || tree.Root != "base" || len(tree.Nodes) != n[0] || len(tree.Edges) != n[1] {
			t.Fatalf("%s should have %d cards and %d paths: %+v", id, n[0], n[1], tree)
		}
		for _, node := range tree.Nodes {
			card := lib.Cards[node.Card.ID]
			if card.MapEffect == nil || content.ValidateCardMapEffect(card.MapEffect) != nil {
				t.Fatalf("%s needs a valid map effect", card.Name)
			}
			if id == "foresight_paths" {
				if card.Program == nil || card.MapEffect.Kind != "look" {
					t.Fatalf("%s must work in battle and on the map", card.Name)
				}
				continue
			}
			if !content.IsMapOnlyCard(card) || !strings.HasPrefix(card.Presentation.EffectSummary, "Map only") || !strings.Contains(card.Presentation.RulesText, "cannot be played in battle") {
				t.Fatalf("%s must be a map-only (blank) battle card: %q", card.Name, card.Presentation.EffectSummary)
			}
		}
	}
	want := map[string]struct {
		kind, flag string
		amount     int
		energy     int
	}{
		"trailblaze":                      {"move", "", 4, 5},
		"pathfinder_paths_variant_node_1": {"move", "terrain", 4, 5},
		"pathfinder_paths_variant_node_2": {"move", "night", 4, 5},
		"lookout_paths_variant_node_1":    {"scout", "", 3, 0},
	}
	for id, w := range want {
		card, ok := lib.Cards[id]
		if !ok {
			t.Fatalf("card %s is missing", id)
		}
		e := card.MapEffect
		if e.Kind != w.kind || e.Amount != w.amount || card.Cost.Energy != w.energy || e.IgnoreTerrain != (w.flag == "terrain") || e.IgnoreNight != (w.flag == "night") {
			t.Fatalf("%s map effect is %+v at %d energy", id, e, card.Cost.Energy)
		}
	}
	// Slip Away is lent to both Pathfinder and Unseen at one price.
	if refs := content.TreeCardPlacements(lib.CardTrees, "slip_away"); len(refs) != 2 || !refs[0].Shared || !refs[1].Shared {
		t.Fatalf("Slip Away is shared by two trees: %+v", refs)
	}
	// Names are original placeholders; none echoes a Dice Throne card.
	for _, name := range []string{"Trailblaze", "Forced March", "Lantern Road", "Scout Ahead"} {
		found := false
		for _, card := range lib.Cards {
			found = found || card.Name == name
		}
		if !found {
			t.Fatalf("card %q is missing", name)
		}
	}
}
