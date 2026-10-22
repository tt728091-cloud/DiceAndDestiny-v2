package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
