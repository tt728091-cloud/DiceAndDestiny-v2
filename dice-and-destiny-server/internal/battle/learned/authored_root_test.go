package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// With a launcher-assigned authored root, published cards and admin prices and
// types land in the shared (tracked) directory while budgets stay with the
// player's loadout.
func TestAuthoredRootSeparatesSharedContentFromPlayerState(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot, shared := t.TempDir(), t.TempDir()
	t.Setenv(content.AuthoredRootEnv, shared)

	catalogs, err := CharacterCatalogs(contentRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, err := content.EditableGeneralCard(catalogs["adventurer"].Cards["brace"])
	if err != nil {
		t.Fatal(err)
	}
	c.ID, c.Name = "shared_brace", "Shared Brace"
	if _, err = content.SaveAuthoredCard(loadoutRoot, catalogs["adventurer"], c, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(shared, "authored_cards.json")); err != nil {
		t.Fatalf("card not published to shared root: %v", err)
	}
	if _, err = os.Stat(filepath.Join(loadoutRoot, "authored_cards.json")); !os.IsNotExist(err) {
		t.Fatal("card published into the loadout root")
	}
	if catalogs, err = CharacterCatalogs(contentRoot, loadoutRoot); err != nil {
		t.Fatal(err)
	}
	if _, ok := catalogs["venom"].Cards["shared_brace"]; !ok {
		t.Fatal("shared card missing from catalog")
	}

	e, err := loadout.LoadEconomy(contentRoot, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	settings := loadout.AdminSettings{CardPrices: map[string]int{"tip_it": 8}, CardTypes: map[string]string{"tip_it": "venom"}, Budgets: map[string]int{"adventurer": 260}}
	if err = loadout.SaveAdmin(loadoutRoot, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	var sharedFile, playerFile map[string]any
	readJSON := func(name string, out *map[string]any) []byte {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	sharedBytes := readJSON(filepath.Join(shared, "economy_admin.json"), &sharedFile)
	readJSON(filepath.Join(loadoutRoot, "economy_admin.json"), &playerFile)
	if sharedFile["card_prices"] == nil || sharedFile["card_types"] == nil || sharedFile["budgets"] != nil {
		t.Fatalf("shared admin file has wrong fields: %v", sharedFile)
	}
	if playerFile["budgets"] == nil || playerFile["card_prices"] != nil || playerFile["card_types"] != nil {
		t.Fatalf("player admin file has wrong fields: %v", playerFile)
	}
	_, effective, admin, err := loadout.ProgressSnapshot(loadoutRoot, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Revision != 1 || effective.Price("adventurer", "tip_it") != 8 || admin.CardTypes["tip_it"] != "venom" || admin.Budgets["adventurer"] != 260 {
		t.Fatalf("combined settings wrong: %+v", admin)
	}

	// A budget-only change advances the revision without touching tracked content.
	settings.Revision, settings.Budgets["adventurer"] = admin.Revision, 270
	if err = loadout.SaveAdmin(loadoutRoot, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(filepath.Join(shared, "economy_admin.json")); string(after) != string(sharedBytes) {
		t.Fatal("budget-only change rewrote the shared admin file")
	}
	if _, _, admin, err = loadout.ProgressSnapshot(loadoutRoot, e, catalogs); err != nil || admin.Revision != 2 || admin.Budgets["adventurer"] != 270 {
		t.Fatalf("budget-only revision wrong: %+v %v", admin, err)
	}
	settings.Revision = 1
	if err = loadout.SaveAdmin(loadoutRoot, e, catalogs, settings); err == nil {
		t.Fatal("stale admin settings accepted")
	}
}
