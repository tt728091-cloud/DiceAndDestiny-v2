package learned

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func findNull(v any, path string) string {
	switch x := v.(type) {
	case nil:
		return path
	case map[string]any:
		for k, child := range x {
			if p := findNull(child, path+"."+k); p != "" {
				return p
			}
		}
	case []any:
		for i, child := range x {
			if p := findNull(child, fmt.Sprintf("%s[%d]", path, i)); p != "" {
				return p
			}
		}
	}
	return ""
}

// The read-only quest_catalog op returns every shipped quest with its graph;
// no list in it is null.
func TestQuestCatalogOp(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	view := runtimeOp(t, map[string]any{"op": "quest_catalog", "content_root": contentRoot}, "")
	if p := findNull(view, "result"); p != "" {
		t.Fatalf("the quest catalog holds a null at %s", p)
	}
	sets := view["sets"].([]any)
	if len(sets) != 2 || str(sets[0].(map[string]any)["id"]) != "salt_marches" || str(sets[1].(map[string]any)["id"]) != "paper" {
		t.Fatalf("two sets, the Salt Marches first: %v", sets)
	}
	ids := map[string]map[string]any{}
	for _, q := range view["quests"].([]any) {
		q := q.(map[string]any)
		ids[str(q["id"])] = q
	}
	for _, id := range []string{"stop_the_drowned_regent", "hunt_the_bell_marshal", "take_brinehold_keep", "free_saltreed", "the_wayside_shrine", "the_wrecked_barge", "the_hermits_fire", "kidnapped_daughter", "spring_farms"} {
		if ids[id] == nil {
			t.Fatalf("the op returns %s", id)
		}
	}
	if len(ids) != 9 {
		t.Fatalf("the op returns every shipped quest and no more: %d", len(ids))
	}
	daughter := ids["kidnapped_daughter"]
	found := false
	for _, e := range daughter["edges"].([]any) {
		e := e.(map[string]any)
		if e["from"] == "rescue" && e["to"] == "turned" && e["outcome"] == "failure" {
			found = true
		}
	}
	nodes := map[string]map[string]any{}
	for _, n := range daughter["nodes"].([]any) {
		n := n.(map[string]any)
		nodes[str(n["id"])] = n
	}
	if !found || nodes["turned"] == nil || nodes["turned"]["kind"] != "stage" {
		t.Fatalf("the kidnapped daughter's graph has a failure edge from the rescue to the werewolf stage: %v", daughter["edges"])
	}
	for _, n := range ids["stop_the_drowned_regent"]["nodes"].([]any) {
		n := n.(map[string]any)
		if n["id"] == "stop_him" {
			timer := n["timer"].(map[string]any)
			if timer["chip"] != "Urgent · 4 rounds" || !strings.Contains(str(timer["text"]), "rounds_per_region") {
				t.Fatalf("the main quest's timer: %v", timer)
			}
		}
	}
	runtimeOp(t, map[string]any{"op": "quest_catalog", "content_root": filepath.Join(t.TempDir(), "missing")}, "content")
}
