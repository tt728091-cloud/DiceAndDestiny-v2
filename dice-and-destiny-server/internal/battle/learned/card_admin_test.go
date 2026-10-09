package learned

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

func TestAdminCardDeletionLifecycle(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, _ := content.EditableGeneralCard(lib.Cards["steady_guard"])
	c.ID, c.Name = "delete_test", "Delete Test"
	c.Economy = &content.CardEconomy{Buy: 10, Sell: 10, CopyLimit: 20}
	if _, err = content.SaveAuthoredCard(dir, lib, c, 0); err != nil {
		t.Fatal(err)
	}
	request := func(op, token, id string, revision int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(runtimeRequest{Op: op, AdminToken: token, CardID: id, CatalogRevision: revision, ContentRoot: root, LoadoutRoot: dir})
		var response map[string]any
		if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(raw))), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	for _, token := range []string{"", "forged"} {
		if request("admin_delete_card", token, c.ID, 1)["ok"] == true {
			t.Fatal("deletion without admin session accepted")
		}
	}
	open := request("open_card_admin", "", "", 0)
	token := open["result"].(map[string]any)["admin_token"].(string)
	defer request("close_card_admin", token, "", 0)
	if request("admin_delete_card", token, c.ID, 0)["ok"] == true {
		t.Fatal("stale deletion accepted")
	}
	blocked := request("preview_delete_card", token, "steady_guard", 1)["result"].(map[string]any)
	if blocked["can_delete"] != false || !strings.Contains(blocked["reason"].(string), "starter deck") {
		t.Fatalf("starter reference not explained: %v", blocked)
	}
	catalogs, _ = CharacterCatalogs(root, dir)
	economy, _ := loadout.LoadEconomy(root, catalogs)
	deck := []loadout.Entry{{CardID: c.ID, Count: 2}, {CardID: "steady_guard", Count: 2}}
	if _, err = loadout.WriteSharedDeck(dir, "adventurer", deck, economy, catalogs["adventurer"]); err != nil {
		t.Fatal(err)
	}
	active, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = active.ResetCharacter("delete-pinned-card", 47, "seat-a", false, "adventurer", true); err != nil {
		t.Fatal(err)
	}
	activePinned, _ := json.Marshal(active.current.Result.Snapshot.ContentCatalog)
	blocked = request("preview_delete_card", token, c.ID, 1)["result"].(map[string]any)
	if blocked["can_delete"] != false || !strings.Contains(blocked["reason"].(string), "owns 2 copies") {
		t.Fatalf("owned copies not explained: %v", blocked)
	}
	if request("admin_delete_card", token, c.ID, 1)["ok"] == true {
		t.Fatal("direct delete bypassed deck guard")
	}
	if _, err = loadout.WriteSharedDeck(dir, "adventurer", []loadout.Entry{{CardID: "steady_guard", Count: 2}}, economy, catalogs["adventurer"]); err != nil {
		t.Fatal(err)
	}
	parent := c
	parent.ID, parent.Name = "delete_parent", "Delete Parent"
	parent.Economy = &content.CardEconomy{Buy: 10, Sell: 10, CopyLimit: 20, Upgrades: []content.CardUpgrade{{To: c.ID, XP: 1}}}
	if _, err = content.SaveAuthoredCard(dir, catalogs["adventurer"], parent, 1); err != nil {
		t.Fatal(err)
	}
	blocked = request("preview_delete_card", token, c.ID, 2)["result"].(map[string]any)
	if blocked["can_delete"] != false || !strings.Contains(blocked["reason"].(string), "Delete Parent") {
		t.Fatalf("upgrade dependency not explained: %v", blocked)
	}
	if response := request("admin_delete_card", token, parent.ID, 2); response["ok"] != true {
		t.Fatal(response)
	}
	catalogs, _ = CharacterCatalogs(root, dir)
	economy, _ = loadout.LoadEconomy(root, catalogs)
	_, _, admin, err := loadout.ProgressSnapshot(dir, economy, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	admin.CardPrices[c.ID] = 17
	admin.CardTypes = map[string]string{c.ID: "general"}
	if err = loadout.SaveAdmin(dir, economy, catalogs, admin); err != nil {
		t.Fatal(err)
	}
	pinned, _ := json.Marshal(catalogs["adventurer"])
	if response := request("admin_delete_card", token, c.ID, 3); response["ok"] != true {
		t.Fatal(response)
	}
	afterDelete, _ := json.Marshal(active.current.Result.Snapshot.ContentCatalog)
	if string(activePinned) != string(afterDelete) {
		t.Fatal("deletion changed an active battle's pinned catalog")
	}
	for step := 0; step < 1200 && !active.current.Terminal; step++ {
		if active.isModelSeat(active.current.ActorID) {
			_, err = active.AdvanceModel()
		} else {
			if len(active.current.Result.LegalActions) == 0 {
				t.Fatal("active battle lost legal actions after deletion")
			}
			action := adventurerTestAction(active.current.Result.LegalActions, 3)
			encoded, _ := json.Marshal(aliasValue(commandMap(t, action), active.aliases(false)))
			_, err = active.SubmitHuman(string(encoded))
		}
		if err != nil {
			t.Fatal("active battle broke after deletion: ", err)
		}
	}
	if !active.current.Terminal || active.current.TruncationReason != "" {
		t.Fatal("active battle failed to finish after deletion")
	}
	if _, ok := catalogs["adventurer"].Cards[c.ID]; !ok {
		t.Fatal("deletion mutated retained catalog")
	}
	var retained content.BattleLibrary
	if err = json.Unmarshal(pinned, &retained); err != nil || retained.Cards[c.ID].Name != c.Name {
		t.Fatal("saved catalog lost card")
	}
	for _, op := range []string{"character_catalogs", "progression_catalogs", "card_authoring"} {
		response := request(op, "", "", 0)
		if response["ok"] != true {
			t.Fatal(response)
		}
		raw, _ := json.Marshal(response["result"])
		if strings.Contains(string(raw), `"delete_test"`) {
			t.Fatalf("deleted card reappeared in %s", op)
		}
	}
	loaded, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = content.SaveAuthoredCard(dir, loaded["adventurer"], c, 4); err == nil {
		t.Fatal("stale editor resurrected deleted ID")
	}
	economy, _ = loadout.LoadEconomy(root, loaded)
	_, _, admin, err = loadout.ProgressSnapshot(dir, economy, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if err = loadout.SaveAdmin(dir, economy, loaded, admin); err != nil {
		t.Fatal("stale admin overrides broke next save: ", err)
	}
	request("close_card_admin", token, "", 0)
	if request("preview_delete_card", token, "steady_guard", 4)["ok"] == true {
		t.Fatal("closed admin session accepted")
	}
}
