package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A campaign save written before the world clock (a flat round, no clock)
// loads through the runtime ops at the right month and plays on; the next
// write stores the clock instead of the flat round.
func TestLegacyExpeditionSaveShowsItsMonth(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	base := map[string]any{"content_root": contentRoot, "loadout_root": loadoutRoot}
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	created := runtimeOp(t, with(map[string]any{"op": "campaign_new", "character": "starter", "name": "Old clock", "mode": "expedition", "seed": 5}), "")
	saveID := str(created["id"])
	path := filepath.Join(loadoutRoot, "campaigns", saveID+".json")
	readSave := func() map[string]any {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var save map[string]any
		if err = json.Unmarshal(raw, &save); err != nil {
			t.Fatal(err)
		}
		return save
	}
	save := readSave()
	x := save["expedition"].(map[string]any)
	if _, ok := x["clock"]; !ok {
		t.Fatalf("a new expedition save stores its clock: %v", x)
	}
	delete(x, "clock")
	delete(x, "region_start")
	x["round"], x["night"] = 3, false
	raw, _ := json.Marshal(save)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	status := runtimeOp(t, with(map[string]any{"op": "expedition_status", "campaign_save": saveID}), "")
	view := status["view"].(map[string]any)
	clock := view["clock"].(map[string]any)
	if num(view["round"]) != 3 || view["night"] != false || clock["month"] != "early_autumn" || view["day_label"] != "Early Autumn · Day · round 3 of 4" {
		t.Fatalf("the legacy save shows round 3 in Early Autumn: round %v %v %v", view["round"], clock, view["day_label"])
	}
	for _, s := range runtimeOp(t, with(map[string]any{"op": "campaign_status"}), "")["saves"].([]any) {
		if s := s.(map[string]any); str(s["id"]) == saveID {
			if x := s["expedition"].(map[string]any); num(x["round"]) != 3 || x["night"] != false {
				t.Fatalf("campaign_status reads the legacy round: %v", x)
			}
		}
	}
	revision := num(status["revision"])
	for _, kind := range []string{"make_camp", "break_camp"} {
		reply := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "action": map[string]any{"kind": kind}, "revision": revision}), "")
		revision = num(reply["revision"])
		view = reply["view"].(map[string]any)
	}
	if num(view["round"]) != 4 || view["night"] != true || view["day_label"] != "Early Autumn · Night · round 4 of 4" {
		t.Fatalf("the legacy save rests into Early Autumn, Night: %v %v", view["round"], view["day_label"])
	}
	x = readSave()["expedition"].(map[string]any)
	if _, flat := x["round"]; flat || x["clock"] == nil || x["region_start"] == nil {
		t.Fatalf("the rewritten save stores the clock, not a flat round: round %v clock %v start %v", x["round"], x["clock"], x["region_start"])
	}
}
