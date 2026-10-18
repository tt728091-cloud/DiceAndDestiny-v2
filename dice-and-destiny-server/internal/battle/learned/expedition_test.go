package learned

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
)

// runtimeOp sends one JSON op through the native entry point and decodes the
// envelope, failing the test on a runtime error unless wantErr names it.
func runtimeOp(t *testing.T, payload map[string]any, wantErr string) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(payload)
	var reply map[string]any
	if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(raw))), &reply); err != nil {
		t.Fatal(err)
	}
	if wantErr != "" {
		if reply["ok"] == true || !strings.Contains(str(reply["error"]), wantErr) {
			t.Fatalf("op %s: expected an error containing %q, got %v", payload["op"], wantErr, reply)
		}
		return reply
	}
	if reply["ok"] != true {
		t.Fatalf("op %s failed: %v", payload["op"], reply["error"])
	}
	return reply["result"].(map[string]any)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func TestExpeditionRuntimeOpsAndPersistence(t *testing.T) {
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
	// A new campaign from the menu is an expedition; the old op without a
	// mode still makes a linear save, so the existing flow is untouched.
	created := runtimeOp(t, with(map[string]any{"op": "campaign_new", "character": "starter", "name": "Marsh run", "mode": "expedition", "seed": 5}), "")
	saveID := str(created["id"])
	if created["mode"] != "expedition" || saveID == "" {
		t.Fatalf("campaign_new in expedition mode: %v", created)
	}
	linear := runtimeOp(t, with(map[string]any{"op": "campaign_new", "character": "starter", "name": "Old road"}), "")
	status := runtimeOp(t, with(map[string]any{"op": "campaign_status"}), "")
	modes := map[string]string{}
	for _, s := range status["saves"].([]any) {
		save := s.(map[string]any)
		modes[str(save["id"])] = str(save["mode"])
		if str(save["id"]) == saveID {
			x := save["expedition"].(map[string]any)
			if num(x["round"]) != 1 || x["night"] != false || x["phase"] != "turn" {
				t.Fatalf("campaign_status expedition summary: %v", x)
			}
		}
	}
	if modes[saveID] != "expedition" || modes[str(linear["id"])] != "linear" {
		t.Fatalf("campaign_status modes: %v", modes)
	}
	// The status is viewer-safe and complete.
	view := runtimeOp(t, with(map[string]any{"op": "expedition_status", "campaign_save": saveID}), "")
	raw, _ := json.Marshal(view)
	for _, forbidden := range []string{`"stack"`, `"core_deal"`, `"rng_position"`, `"route"`, `"hand_before"`, "greywharf_strand", "drowned_reach"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("expedition_status leaks %s", forbidden)
		}
	}
	ctx, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	startXP, trailblaze := ctx.campaign.BonusXP, cardPrice(t, contentRoot, loadoutRoot, "trailblaze")
	v := view["view"].(map[string]any)
	revision := num(view["revision"])
	if num(view["xp"]) != startXP || num(view["health"]) != 12 || len(v["hand"].([]any)) != 4 || num(v["energy"]) != 10 || num(v["seed"]) != 5 {
		t.Fatalf("status: xp %v health %v hand %d energy %v", view["xp"], view["health"], len(v["hand"].([]any)), v["energy"])
	}
	catalog := view["catalog"].(map[string]any)
	if _, ok := catalog["cards"].(map[string]any)["take_stock"]; !ok {
		t.Fatal("the catalog carries the card definitions the hand shows")
	}
	// Acting with a stale revision is refused; a fresh one moves the player.
	hand := v["hand"].([]any)
	card := str(hand[0].(map[string]any)["id"])
	runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": revision + 7, "action": map[string]any{"kind": "move", "target": map[string]int{"q": 1, "r": 0}, "cards": []string{card}}}), "refresh before acting")
	moved := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": revision, "action": map[string]any{"kind": "move", "target": map[string]int{"q": 1, "r": 0}, "cards": []string{card}}}), "")
	mv := moved["view"].(map[string]any)
	player := mv["player"].(map[string]any)
	if num(player["q"]) != 1 || num(player["r"]) != 0 || num(moved["revision"]) != revision+1 || len(mv["spent"].([]any)) != 1 {
		t.Fatalf("move through the op: player %v revision %v spent %v", player, moved["revision"], mv["spent"])
	}
	// An illegal action is a clear error and changes nothing.
	runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "action": map[string]any{"kind": "move", "target": map[string]int{"q": 1, "r": -1}}}), "costs 2 Move")
	again := runtimeOp(t, with(map[string]any{"op": "expedition_status", "campaign_save": saveID}), "")
	a1, _ := json.Marshal(moved["view"])
	a2, _ := json.Marshal(again["view"])
	if string(a1) != string(a2) {
		t.Fatal("re-reading the save gives the identical view (save/reload equivalence)")
	}
	// The save on disk holds the state; the sheet's deck still has 12 live copies.
	save := readSave(t, contentRoot, loadoutRoot, saveID)
	if !save.IsExpedition() || deckHealth(save.Sheet.Deck) != 12 || save.Expedition.Player != (expedition.Hex{Q: 1, R: 0}) {
		t.Fatalf("save on disk: mode %s deck %d player %v", save.Mode, deckHealth(save.Sheet.Deck), save.Expedition.Player)
	}
	// Deck changes: refused through the linear purchase op; allowed through
	// the map at a site with the Influence paid; the new card tops the draw pile.
	runtimeOp(t, with(map[string]any{"op": "campaign_purchase", "campaign_save": saveID, "purchase": map[string]any{"kind": "buy_card", "id": "trailblaze", "revision": num(moved["revision"]), "expected_cost": trailblaze}}), "only at a site or camp")
	runtimeOp(t, with(map[string]any{"op": "expedition_deck_change", "campaign_save": saveID, "purchase": map[string]any{"kind": "buy_card", "id": "trailblaze", "revision": num(moved["revision"]), "expected_cost": trailblaze}}), "there is no site here")
	// Walk back to the start and on to the village at (-1,1).
	rev := num(moved["revision"])
	hand = mv["hand"].([]any)
	step := func(q, r int, cards []string) map[string]any {
		res := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": rev, "action": map[string]any{"kind": "move", "target": map[string]int{"q": q, "r": r}, "cards": cards}}), "")
		rev = num(res["revision"])
		return res["view"].(map[string]any)
	}
	step(0, 0, []string{str(hand[0].(map[string]any)["id"])})
	vv := step(-1, 1, []string{str(hand[1].(map[string]any)["id"])})
	site := vv["site_here"].(map[string]any)
	if site["id"] != "hollow_ford" || site["open"] != true || num(site["deck_change"]) != 2 {
		t.Fatalf("the village offers deck changes for 2 Influence: %v", site)
	}
	runtimeOp(t, with(map[string]any{"op": "expedition_deck_change", "campaign_save": saveID, "purchase": map[string]any{"kind": "buy_card", "id": "trailblaze", "revision": rev, "expected_cost": trailblaze}}), "costs 2 Influence; you have 0")
	// End the turn at the village so the hand refills, then pay Influence.
	ended := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": rev, "action": map[string]any{"kind": "end_turn"}}), "")
	rev = num(ended["revision"])
	vv = ended["view"].(map[string]any)
	hand = vv["hand"].([]any)
	if len(hand) != 4 {
		t.Fatalf("the hand refills to 4 at end of turn: %d", len(hand))
	}
	paid := runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": rev, "action": map[string]any{"kind": "use_site", "service": "influence", "cards": []string{str(hand[0].(map[string]any)["id"]), str(hand[1].(map[string]any)["id"])}}}), "")
	rev = num(paid["revision"])
	if num(paid["view"].(map[string]any)["turn"].(map[string]any)["influence"]) != 2 {
		t.Fatal("two cards played at the village are 2 Influence")
	}
	// Gold never buys XP. A stale client may still send the removed trainer's
	// gold field: the request decodes (unknown fields are ignored) and the
	// service is refused like any unknown one, changing nothing.
	runtimeOp(t, with(map[string]any{"op": "expedition_act", "campaign_save": saveID, "revision": rev, "action": map[string]any{"kind": "use_site", "service": "trainer", "gold": 9}}), `unknown site service "trainer"`)
	if unchanged := readSave(t, contentRoot, loadoutRoot, saveID); unchanged.Sheet.Revision != rev || unchanged.Sheet.XP != startXP || unchanged.Expedition.Turn.Influence != 2 {
		t.Fatalf("a refused trainer request changed the save: revision %d xp %d", unchanged.Sheet.Revision, unchanged.Sheet.XP)
	}
	bought := runtimeOp(t, with(map[string]any{"op": "expedition_deck_change", "campaign_save": saveID, "purchase": map[string]any{"kind": "buy_card", "id": "trailblaze", "revision": rev, "expected_cost": trailblaze}}), "")
	bv := bought["view"].(map[string]any)
	if num(bought["xp"]) != startXP-trailblaze || num(bv["turn"].(map[string]any)["influence"]) != 0 || num(bv["health"]) != 13 {
		t.Fatalf("deck change: xp %v influence %v health %v", bought["xp"], bv["turn"], bv["health"])
	}
	if site := bv["site_here"].(map[string]any); site["deck_open"] != true {
		t.Fatalf("the first change opens the deck for this visit: %v", site)
	}
	// With the deck open, a second change costs XP only, no more Influence.
	second := runtimeOp(t, with(map[string]any{"op": "expedition_deck_change", "campaign_save": saveID, "purchase": map[string]any{"kind": "buy_card", "id": "trailblaze", "revision": num(bought["revision"]), "expected_cost": trailblaze}}), "")
	if num(second["xp"]) != startXP-2*trailblaze || num(second["view"].(map[string]any)["health"]) != 14 {
		t.Fatalf("a second change at the open deck: xp %v health %v", second["xp"], second["view"].(map[string]any)["health"])
	}
	save = readSave(t, contentRoot, loadoutRoot, saveID)
	if save.Expedition.Draw[0] != "trailblaze" || save.Expedition.Draw[1] != "trailblaze" || deckHealth(save.Sheet.Deck) != 14 || save.Sheet.XP != startXP-2*trailblaze {
		t.Fatalf("the bought cards top the draw pile and the sheet agrees: %v", save.Expedition.Draw)
	}
	// Prepare's read-only view still works for expedition saves.
	prepare := runtimeOp(t, with(map[string]any{"op": "campaign_loadout", "campaign_save": saveID}), "")
	if num(prepare["xp"]) != startXP-2*trailblaze {
		t.Fatalf("campaign_loadout on an expedition: %v", prepare["xp"])
	}
	// The linear save is untouched by all of this and keeps its encounter path.
	old := readSave(t, contentRoot, loadoutRoot, str(linear["id"]))
	if old.IsExpedition() || old.Sheet.XP != startXP || deckHealth(old.Sheet.Deck) != 12 {
		t.Fatalf("the linear save changed: %+v", old)
	}
	c, err := loadCampaignContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadout.RecordCampaignBattle(loadoutRoot, saveID, c.economy, c.catalogs, c.campaign, "tide_pool", "b1", "victory", nil); err == nil || !strings.Contains(err.Error(), "expedition") {
		t.Fatalf("the linear recorder must refuse expedition saves: %v", err)
	}
	// Recording a battle that is not pending is refused.
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadout.RecordExpeditionBattle(loadoutRoot, saveID, x.economy, x.catalogs, x.rules["starter"], expedition.BattleEnd{BattleID: "nope", Result: "victory"}); err == nil || !strings.Contains(err.Error(), "not the save's pending battle") {
		t.Fatalf("recording a stray battle: %v", err)
	}
}

// The same seed and the same actions give the identical map and state.
func TestExpeditionSeedDeterminismThroughTheRuntime(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	var views []string
	for run := 0; run < 2; run++ {
		loadoutRoot := campaignLoadoutRoot(t)
		base := map[string]any{"content_root": contentRoot, "loadout_root": loadoutRoot}
		req := func(extra map[string]any) map[string]any {
			m := map[string]any{}
			for k, v := range base {
				m[k] = v
			}
			for k, v := range extra {
				m[k] = v
			}
			return m
		}
		created := runtimeOp(t, req(map[string]any{"op": "campaign_new", "character": "starter", "name": "Twin", "mode": "expedition", "seed": 99}), "")
		id := str(created["id"])
		status := runtimeOp(t, req(map[string]any{"op": "expedition_status", "campaign_save": id}), "")
		hand := status["view"].(map[string]any)["hand"].([]any)
		actions := []map[string]any{
			{"kind": "move", "target": map[string]int{"q": 1, "r": 0}, "cards": []string{str(hand[0].(map[string]any)["id"])}},
			{"kind": "explore", "slot": "east", "cards": []string{str(hand[1].(map[string]any)["id"]), str(hand[2].(map[string]any)["id"])}},
			{"kind": "end_turn"},
			{"kind": "make_camp"},
			{"kind": "break_camp"},
		}
		var last map[string]any
		for _, a := range actions {
			last = runtimeOp(t, req(map[string]any{"op": "expedition_act", "campaign_save": id, "action": a}), "")
		}
		raw, _ := json.Marshal(last["view"])
		views = append(views, strings.ReplaceAll(string(raw), id, "SAVE"))
	}
	if views[0] != views[1] {
		t.Fatal("two expeditions with the same seed and actions diverged")
	}
}
