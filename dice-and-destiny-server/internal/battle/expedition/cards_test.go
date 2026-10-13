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
		id, kind, amount = "second_wind", "energy", 10
	}
	// The Pathfinder tree: four map-only cards that are blank in battle.
	tree, ok := lib.CardTrees["pathfinder_paths"]
	if !ok || tree.Root != "base" || len(tree.Nodes) != 4 || len(tree.Edges) != 3 {
		t.Fatalf("pathfinder_paths should have a base and three branches: %+v", tree)
	}
	want := map[string]struct {
		kind, flag string
		amount     int
		energy     int
	}{
		"trailblaze":                      {"move", "", 3, 0},
		"pathfinder_paths_variant_node_1": {"move", "terrain", 4, 0},
		"pathfinder_paths_variant_node_2": {"move", "night", 2, 0},
		"pathfinder_paths_variant_node_3": {"scout", "", 3, 0},
	}
	for id, w := range want {
		card, ok := lib.Cards[id]
		if !ok {
			t.Fatalf("card %s is missing", id)
		}
		if !content.IsMapOnlyCard(card) {
			t.Fatalf("%s must be a map-only (blank) battle card", id)
		}
		e := card.MapEffect
		if e.Kind != w.kind || e.Amount != w.amount || e.EnergyCost != w.energy || e.IgnoreTerrain != (w.flag == "terrain") || e.IgnoreNight != (w.flag == "night") {
			t.Fatalf("%s map effect is %+v", id, e)
		}
		if !strings.HasPrefix(card.Presentation.EffectSummary, "Map only") || !strings.Contains(card.Presentation.RulesText, "cannot be played in battle") {
			t.Fatalf("%s face must say it is map only: %q", id, card.Presentation.EffectSummary)
		}
	}
	// The Steady Guard tree gained two "both" branches off the base.
	sg := lib.CardTrees["steady_guard_paths"]
	both := map[string]string{"steady_guard_paths_variant_node_11": "move", "steady_guard_paths_variant_node_12": "sight"}
	for id, kind := range both {
		card := lib.Cards[id]
		if card.Program == nil || card.MapEffect == nil || card.MapEffect.Kind != kind || content.IsBlankCard(card) {
			t.Fatalf("%s should prevent in battle and %s on the map: %+v", id, kind, card.MapEffect)
		}
		linked := false
		for _, e := range sg.Edges {
			node, _ := sg.Node(e.To)
			linked = linked || (e.From == sg.Root && node.Card.ID == id)
		}
		if !linked {
			t.Fatalf("%s must branch off Steady Guard's base", id)
		}
	}
	// Names are original placeholders; none echoes a Dice Throne card.
	for _, name := range []string{"Trailblaze", "Forced March", "Lantern Road", "Scout Ahead", "Warden's March", "Sentry's Watch"} {
		found := false
		for _, card := range lib.Cards {
			found = found || card.Name == name
		}
		if !found {
			t.Fatalf("card %q is missing", name)
		}
	}
}
