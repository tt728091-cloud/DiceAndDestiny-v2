package learned

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The battle screen knows its human as HumanAlias. The tactics ops present
// the sandbox hero under that name and accept commands written with it.
func TestTacticsOpsAliasTheHeroForTheBattleScreen(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	loadoutRoot, battleRoot := t.TempDir(), t.TempDir()
	call := func(op, commandJSON string, setup any) map[string]any {
		t.Helper()
		request := runtimeRequest{Op: op, BattleID: "tactics-alias", ContentRoot: root, LoadoutRoot: loadoutRoot, BattleRoot: battleRoot, CommandJSON: commandJSON}
		if setup != nil {
			request.Tactics, _ = json.Marshal(setup)
		}
		raw, _ := json.Marshal(request)
		var view map[string]any
		if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(raw))), &view); err != nil {
			t.Fatal(err)
		}
		if view["ok"] == false {
			t.Fatalf("%s refused: %v", op, view["error"])
		}
		return view
	}
	view := call("tactics_start", "", map[string]any{"hero": "starter_movement", "enemies": []string{"tactics_ribbon_eel"}, "arena": "open_field", "seed": 3})
	snap := view["snapshot"].(map[string]any)
	actors := snap["actors"].(map[string]any)
	if _, ok := actors[HumanAlias]; !ok || actors["hero"] != nil {
		t.Fatalf("the hero is not presented as %s: %v", HumanAlias, keys(actors))
	}
	if _, ok := snap["tactics"].(map[string]any)["creatures"].(map[string]any)[HumanAlias]; !ok {
		t.Fatal("the tactics view still names the hero seat")
	}
	var roll map[string]any
	for _, value := range view["legal_actions"].([]any) {
		action := value.(map[string]any)
		if action["actor_id"] != HumanAlias {
			t.Fatalf("a legal action names %v", action["actor_id"])
		}
		if action["type"] == "planning_roll" {
			roll = action
		}
	}
	if roll == nil {
		t.Fatal("no roll offered")
	}
	encoded, _ := json.Marshal(roll)
	after := call("tactics_submit", string(encoded), nil)
	// The stored setup names its hero definition under the field "hero".
	delete(after["tactics_session"].(map[string]any), "setup")
	if raw, _ := json.Marshal(after); strings.Contains(string(raw), `"hero"`) {
		i := strings.Index(string(raw), `"hero"`)
		t.Fatalf("a submitted result still names the hero seat: …%s…", string(raw)[max(0, i-300):min(len(raw), i+100)])
	}
	history := call("tactics_history", "", nil)
	if _, ok := history["rounds"].([]any); !ok {
		t.Fatalf("tactics_history returned no rounds: %v", history)
	}
	if raw, _ := json.Marshal(history); strings.Contains(string(raw), `"hero"`) {
		t.Fatal("the history still names the hero seat")
	}
}

func keys(m map[string]any) []string {
	var out []string
	for key := range m {
		out = append(out, key)
	}
	return out
}
