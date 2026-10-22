package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

func boardRuntimeCall(t *testing.T, request map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(request)
	var reply map[string]any
	if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(raw))), &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}

// board_price only reads: it prices the picks and returns the report and,
// on request, the keep policy after rolls 1 and 2.
func TestBoardPriceOp(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := t.TempDir()
	board := map[string]any{"die": "forge_d6", "template": "paired_standard", "slots": []map[string]any{{"slot": "flex_a", "rider": "protect_1"}}}
	reply := boardRuntimeCall(t, map[string]any{"op": "board_price", "content_root": contentRoot, "loadout_root": loadoutRoot, "board": board, "include_policy": true})
	if reply["ok"] != true {
		t.Fatalf("board_price: %v", reply["error"])
	}
	result := reply["result"].(map[string]any)
	report := result["report"].(map[string]any)
	tiers := report["tiers"].([]any)
	if len(tiers) != 10 || report["whole_average"].(float64) < 6.0 || report["whole_average"].(float64) > 6.4 {
		t.Fatalf("report: %d tiers, average %v", len(tiers), report["whole_average"])
	}
	policy := report["policy"].(map[string]any)
	if len(policy["after_roll_1"].([]any)) != 252 || len(policy["after_roll_2"].([]any)) != 252 {
		t.Fatalf("keep policy has %d / %d hands", len(policy["after_roll_1"].([]any)), len(policy["after_roll_2"].([]any)))
	}
	slots := result["slots"].([]any)
	flex := slots[1].(map[string]any)
	if flex["slot"] != "flex_a" || flex["rider"] != "protect_1" || flex["label"] != "Rivet ×2" || len(flex["options"].([]any)) == 0 {
		t.Fatalf("flex slot view %v", flex)
	}
	if entries, _ := os.ReadDir(loadoutRoot); len(entries) != 0 {
		t.Fatalf("board_price wrote %d entries to the loadout root", len(entries))
	}
	// Without include_policy the report leaves the policy out.
	reply = boardRuntimeCall(t, map[string]any{"op": "board_price", "content_root": contentRoot, "board": board})
	if p := reply["result"].(map[string]any)["report"].(map[string]any)["policy"].(map[string]any); p["after_roll_1"] != nil {
		t.Fatal("policy returned without include_policy")
	}
	// Off-menu picks are refused by the authority.
	bad := map[string]any{"die": "forge_d6", "template": "lean"}
	if reply := boardRuntimeCall(t, map[string]any{"op": "board_price", "content_root": contentRoot, "board": bad}); reply["ok"] == true {
		t.Fatal("a 3-2-1 template priced on a 2-2-1-1 die")
	}
}

func TestBoardBuilderOp(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	reply := boardRuntimeCall(t, map[string]any{"op": "board_builder", "content_root": contentRoot, "loadout_root": t.TempDir(), "board": map[string]any{"die": "grove_d6"}})
	if reply["ok"] != true {
		t.Fatalf("board_builder: %v", reply["error"])
	}
	result := reply["result"].(map[string]any)
	if len(result["dice"].([]any)) != 5 || len(result["templates"].([]any)) != 13 || result["die"] != "grove_d6" {
		t.Fatalf("menus: %d dice, %d templates, die %v", len(result["dice"].([]any)), len(result["templates"].([]any)), result["die"])
	}
	if stats := result["template_stats"].([]any); len(stats) != 3 {
		t.Fatalf("grove templates priced: %d", len(stats))
	}
	for _, c := range result["costs"].([]any) {
		cost := c.(map[string]any)
		if cost["chase"].(float64) <= 0 || cost["band"] == "" || cost["label"] == "" {
			t.Fatalf("cost view %v", cost)
		}
	}
	if sheet := result["sheet"].(map[string]any); sheet["dice"] != "standard_d6" {
		t.Fatalf("default sheet die %v", sheet["dice"])
	}
}

// boardSaveCall confirms a board through the authority at the sheet's
// current revision.
func boardSaveCall(t *testing.T, contentRoot, loadoutRoot string, board map[string]any) map[string]any {
	t.Helper()
	builder := boardRuntimeCall(t, map[string]any{"op": "board_builder", "content_root": contentRoot, "loadout_root": loadoutRoot})
	if builder["ok"] != true {
		t.Fatalf("board_builder: %v", builder["error"])
	}
	revision := builder["result"].(map[string]any)["sheet"].(map[string]any)["revision"]
	reply := boardRuntimeCall(t, map[string]any{"op": "board_save", "content_root": contentRoot, "loadout_root": loadoutRoot, "board": board, "revision": revision})
	if reply["ok"] != true {
		t.Fatalf("board_save: %v", reply["error"])
	}
	return reply["result"].(map[string]any)
}

// A confirmed board lives on the player's sheet (never the authored
// catalog), survives reload, and a battle pins its generated abilities with
// their full rules for the hover.
func TestBoardSaveSurvivesReloadAndPins(t *testing.T) {
	server := testServerRoot(t)
	contentRoot := filepath.Join(server, "content")
	authored := t.TempDir()
	t.Setenv(content.AuthoredRootEnv, authored)
	loadoutRoot := t.TempDir()
	board := map[string]any{"die": "forge_d6", "template": "paired_standard", "slots": []map[string]any{{"slot": "flex_a", "rider": "protect_2"}, {"slot": "ultimate", "rider": "heal_2"}}}
	saved := boardSaveCall(t, contentRoot, loadoutRoot, board)
	if saved["dice"] != "forge_d6" {
		t.Fatalf("saved die %v", saved["dice"])
	}
	if entries, _ := os.ReadDir(authored); len(entries) != 0 {
		t.Fatal("a player's board was written to the authored catalog")
	}
	catalogs, err := CharacterCatalogs(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	economy, _ := loadout.LoadEconomy(contentRoot, catalogs)
	lib := catalogs[loadout.CustomAdventurer]
	progress, err := loadout.ReadProgress(loadoutRoot, loadout.CustomAdventurer, economy, lib)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Dice != "forge_d6" || progress.BoardChoice == nil || progress.BoardChoice.Template != "paired_standard" || len(progress.GeneratedAbilities) != 8 {
		t.Fatalf("reloaded sheet: die %s choice %+v, %d generated", progress.Dice, progress.BoardChoice, len(progress.GeneratedAbilities))
	}
	odd := progress.GeneratedAbilities["board_odd2"]
	if odd.Name != "Rivet ×2" || !strings.Contains(odd.Presentation.RulesText, "2 Protect") || lib.Abilities["board_odd2"].Presentation.RulesText != odd.Presentation.RulesText {
		t.Fatalf("generated board_odd2 %q: %s", odd.Name, odd.Presentation.RulesText)
	}
	// Re-pricing reuses the same IDs.
	again := boardSaveCall(t, contentRoot, loadoutRoot, board)
	if !reflect.DeepEqual(again["ability_board"], saved["ability_board"]) {
		t.Fatalf("re-pricing an unchanged board changed its IDs: %v vs %v", again["ability_board"], saved["ability_board"])
	}

	// A battle against a minion (its view carries the pinned catalog) pins
	// the generated abilities and the die.
	s, err := NewSession(SessionConfig{LoadoutRoot: loadoutRoot, ContentRoot: contentRoot, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.ResetCharacter("pinned-board", 31, "seat-a", false, loadout.CustomAdventurer)
	if err != nil {
		t.Fatal(err)
	}
	catalog := object(object(view["snapshot"])["content_catalog"])
	pinned := object(object(catalog["abilities"])["board_odd2"])
	if object(pinned["presentation"])["rules_text"] != odd.Presentation.RulesText {
		t.Fatalf("battle catalog rules %v", object(pinned["presentation"])["rules_text"])
	}
	hero := object(object(object(view["snapshot"])["actors"])[HumanAlias])
	abilities := []string{}
	for _, id := range hero["offensive_abilities"].([]any) {
		abilities = append(abilities, id.(string))
	}
	if !strings.Contains(strings.Join(abilities, ","), "board_odd2") {
		t.Fatalf("hero board %v", abilities)
	}
	if roles := object(object(catalog["dice"])["forge_d6"])["roles"]; roles == nil {
		t.Fatal("pinned die has no published roles")
	}
}

// Re-pricing after a rider change keeps the ability ID and updates the
// damage (and the battle's copy).
func TestBoardRepriceAfterRiderChange(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	t.Setenv(content.AuthoredRootEnv, t.TempDir())
	loadoutRoot := t.TempDir()
	damage := func(result map[string]any, id string) (int, string) {
		for _, a := range result["abilities"].([]any) {
			ability := a.(map[string]any)
			if ability["id"] == id {
				tiers := object(ability["qualification"])["activation_tiers"].([]any)
				ops := object(tiers[0])["operations"].([]any)
				return int(object(ops[0])["amount"].(float64)), object(ability["presentation"])["rules_text"].(string)
			}
		}
		t.Fatalf("%s not generated", id)
		return 0, ""
	}
	plain := boardSaveCall(t, contentRoot, loadoutRoot, map[string]any{"die": "standard_d6", "template": "lean"})
	before, _ := damage(plain, "board_main2_side3")
	guarded := boardSaveCall(t, contentRoot, loadoutRoot, map[string]any{"die": "standard_d6", "template": "lean", "slots": []map[string]any{{"slot": "flex_a", "rider": "protect_2"}}})
	after, rules := damage(guarded, "board_main2_side3")
	if after >= before || !strings.Contains(rules, "gain 2 Protect") {
		t.Fatalf("rider change: damage %d -> %d, rules %q", before, after, rules)
	}
	catalogs, err := CharacterCatalogs(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	ops := catalogs[loadout.CustomAdventurer].Abilities["board_main2_side3"].Qualification.ActivationTiers[0].Operations
	amount, _ := ops[0].Amount.(float64) // JSON-decoded from the sheet
	if int(amount) != after || len(ops) != 2 || ops[1].StatusID != "protect" {
		t.Fatalf("catalog copy after re-pricing: %+v", ops)
	}
}
