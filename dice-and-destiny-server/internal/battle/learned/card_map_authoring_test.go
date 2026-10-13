package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

// The card editor edits map effects: a map-only card (Trailblaze) validates
// and publishes through the same op as battle cards, with its map Move
// changed, and a "both" card can gain, change or lose a map effect.
func TestCardEditorEditsMapEffects(t *testing.T) {
	t.Setenv(content.AuthoredRootEnv, "")
	root := filepath.Join("..", "..", "..", "content")
	dir := t.TempDir()
	// The tracked authored content, copied so the test never writes to it.
	entries, err := os.ReadDir(filepath.Join(root, "authored"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "authored", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	call := func(op string, card any, revision int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(card)
		req, _ := json.Marshal(runtimeRequest{Op: op, Card: raw, CatalogRevision: revision, ContentRoot: root, LoadoutRoot: dir})
		var response map[string]any
		if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(req))), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	catalog := call("card_authoring", nil, 0)
	if catalog["ok"] != true {
		t.Fatal(catalog)
	}
	result := catalog["result"].(map[string]any)
	kinds, _ := json.Marshal(result["map_effect_kinds"])
	if string(kinds) != `["move","draw","energy","sight","scout"]` {
		t.Fatalf("the editor gets the map effect kinds: %s", kinds)
	}
	templates := result["templates"].(map[string]any)
	for _, blank := range []string{"ballast_stone", "drowned_standard", "shed_ribbon"} {
		if _, offered := templates[blank]; offered {
			t.Fatalf("the blank health card %s is not an editor template", blank)
		}
	}
	trailblaze, ok := templates["trailblaze"].(map[string]any)
	if !ok {
		t.Fatal("Trailblaze is offered as an editable template")
	}
	revision := int(result["revision"].(float64))
	// Move 2 → Move 3, and a 5 energy printed cost.
	trailblaze["map_effect"].(map[string]any)["amount"] = 3
	trailblaze["cost"].(map[string]any)["energy"] = 5
	validated := call("validate_card", trailblaze, revision)
	if validated["ok"] != true {
		t.Fatalf("a map-only card validates: %v", validated["error"])
	}
	rules := validated["result"].(map[string]any)["rules"].(string)
	if !strings.Contains(rules, "Gain 3 Move this turn") || !strings.Contains(rules, "cannot be played in battle") {
		t.Fatalf("the map card's rules come from its map effect: %q", rules)
	}
	// Trailblaze is the base of the Pathfinder tree, so the workshop
	// publishes it through the tree: the base becomes Move 3 for 5 energy and
	// a variant gets its own move and energy values.
	saved, err := content.ReadAuthoredCards(dir)
	if err != nil {
		t.Fatal(err)
	}
	tree, ok := saved.Trees["pathfinder_paths"]
	if !ok {
		t.Fatal("the Pathfinder tree exists")
	}
	variant := ""
	for i, node := range tree.Nodes {
		if node.Card.MapEffect == nil || node.Card.MapEffect.Kind != "move" {
			continue
		}
		if node.Card.ID == "trailblaze" {
			tree.Nodes[i].Card.MapEffect.Amount = 3
			tree.Nodes[i].Card.Cost.Energy = 5
		} else if variant == "" {
			variant = node.Card.ID
			tree.Nodes[i].Card.MapEffect.Amount = 5
			tree.Nodes[i].Card.MapEffect.EnergyCost = 0
			tree.Nodes[i].Card.Cost.Energy = 15
		}
	}
	if variant == "" {
		t.Fatal("the tree has a move variant to edit")
	}
	raw, _ := json.Marshal(tree)
	opened := runtimeCall(t, runtimeRequest{Op: "open_card_admin", ContentRoot: root, LoadoutRoot: dir})
	published := runtimeCall(t, runtimeRequest{Op: "publish_card_tree", ContentRoot: root, LoadoutRoot: dir, CardTree: raw, Character: "adventurer", CatalogRevision: saved.Revision, AdminToken: opened["result"].(map[string]any)["admin_token"].(string)})
	if published["ok"] != true {
		t.Fatalf("the tree publishes with the edited map effects: %v", published["error"])
	}
	catalogs, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := catalogs["adventurer"].Cards["trailblaze"]
	if got.MapEffect == nil || got.MapEffect.Amount != 3 || got.Cost.Energy != 5 || !content.IsMapOnlyCard(got) || !strings.Contains(got.Presentation.RulesText, "Gain 3 Move") {
		t.Fatalf("Trailblaze reloads as Move 3 for 5 energy, still map-only: %+v %q", got.MapEffect, got.Presentation.RulesText)
	}
	if v := catalogs["adventurer"].Cards[variant]; v.MapEffect == nil || v.MapEffect.Amount != 5 || v.MapEffect.EnergyCost != 0 || v.Cost.Energy != 15 {
		t.Fatalf("the variant reloads with its new map values: %+v cost %d", v.MapEffect, v.Cost.Energy)
	}
	revision = saved.Revision + 1
	// A map card cannot smuggle in battle operations.
	bad := map[string]any{}
	raw, _ = json.Marshal(trailblaze)
	_ = json.Unmarshal(raw, &bad)
	bad["play"].(map[string]any)["playable_during"] = []any{map[string]any{"segment": "offensive", "priority": "active"}}
	if r := call("validate_card", bad, revision); r["ok"] == true {
		t.Fatal("a map card with battle play timing must be refused")
	}
	// A map effect with a bad value is refused with the content rule.
	raw, _ = json.Marshal(trailblaze)
	bad = map[string]any{}
	_ = json.Unmarshal(raw, &bad)
	bad["map_effect"].(map[string]any)["amount"] = 0
	if r := call("validate_card", bad, revision); r["ok"] == true || !strings.Contains(r["error"].(string), "map_effect amount") {
		t.Fatalf("map amount 0 is refused: %v", r["error"])
	}
	// A battle card gains a map effect, then changes it, then loses it.
	nudge := templates["nudge"].(map[string]any)
	nudge["map_effect"] = map[string]any{"kind": "sight", "amount": 1, "text": "Look again."}
	if r := call("validate_card", nudge, revision); r["ok"] != true || !strings.Contains(r["result"].(map[string]any)["rules"].(string), "Map: See 1 hex further") {
		t.Fatalf("a battle card gains a map effect: %v", r)
	}
	nudge["map_effect"] = map[string]any{"kind": "move", "amount": 2, "energy_cost": 5, "ignore_night": true, "text": "Night walk."}
	if r := call("validate_card", nudge, revision); r["ok"] != true {
		t.Fatalf("a move map effect with night and extra energy validates: %v", r["error"])
	}
	delete(nudge, "map_effect")
	if r := call("validate_card", nudge, revision); r["ok"] != true {
		t.Fatalf("a battle card without a map effect still validates: %v", r["error"])
	}
	// A map-only card with no map effect is neither a battle nor a map card.
	delete(bad, "map_effect")
	bad["play"].(map[string]any)["playable_during"] = []any{}
	if r := call("validate_card", bad, revision); r["ok"] == true || !strings.Contains(r["error"].(string), "program, a specialized mechanic or a map effect") {
		t.Fatalf("a card with no effect at all is refused: %v", r["error"])
	}
}
