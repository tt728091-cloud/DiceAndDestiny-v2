package learned

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
)

func TestAccessTypesAndAdminOverrides(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(contentRoot)
	if err != nil {
		t.Fatal(err)
	}
	e, err := loadout.LoadEconomy(contentRoot, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for character, lib := range catalogs {
		p, err := loadout.ReadProgress(root, character, e, lib)
		if err != nil {
			t.Fatal(err)
		}
		if problems := e.Access.Problems(character, p.Deck, p.Abilities); len(problems) != 0 {
			t.Fatalf("starter %s: %v", character, problems)
		}
		if !e.Access.Allows(character, "cards", "brace") {
			t.Fatal("General missing")
		}
		if e.Access.Allows(character, "cards", "pinprick") != (character == "venom") {
			t.Fatal("Venom access")
		}
		if e.Access.Allows(character, "cards", "black_fingerprint") != (character == "curse") {
			t.Fatal("Curse access")
		}
	}
	p, _ := loadout.ReadProgress(root, "adventurer", e, catalogs["adventurer"])
	blocked := loadout.Purchase{Kind: "buy_card", ID: "pinprick", ExpectedCost: 10, Revision: p.Revision}
	if _, err = loadout.Buy(root, "adventurer", e, catalogs["adventurer"], blocked); err == nil {
		t.Fatal("forged cross-type buy accepted")
	}
	unchanged, _ := loadout.ReadProgress(root, "adventurer", e, catalogs["adventurer"])
	if !reflect.DeepEqual(p, unchanged) {
		t.Fatal("rejected trade changed state")
	}
	settings := loadout.AdminSettings{CardTypes: map[string]string{"pinprick": "curse"}, AbilityTypes: map[string]string{"adventurer_guard_plus": "venom"}}
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	all, effective, settings, err := loadout.ProgressSnapshot(root, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Access.Allows("venom", "cards", "pinprick") || !effective.Access.Allows("curse", "cards", "pinprick") {
		t.Fatal("reassignment failed")
	}
	p = all["adventurer"]
	if _, err = loadout.Buy(root, "adventurer", e, catalogs["adventurer"], loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", ExpectedCost: 25, Revision: p.Revision}); err == nil {
		t.Fatal("restricted ability upgrade accepted")
	}
	settings.CardTypes["pinprick"] = "general"
	settings.CardTypes["black_fingerprint"] = "general"
	settings.AbilityTypes["adventurer_guard_plus"] = "general"
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, _ = loadout.ReadProgress(root, "adventurer", e, catalogs["adventurer"])
	p, err = loadout.Buy(root, "adventurer", e, catalogs["adventurer"], loadout.Purchase{Kind: "buy_card", ID: "black_fingerprint", ExpectedCost: 10, Revision: p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	// The previously unavailable Curse pack must also be pinned in the actual battle.
	session, err := NewSession(SessionConfig{ContentRoot: contentRoot, RunStateRoot: t.TempDir(), LoadoutRoot: root, OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.ResetCharacterLoadout("shared-card", 31, "seat-a", false, "adventurer", true, "progression"); err != nil {
		t.Fatal(err)
	}
	_, _, settings, _ = loadout.ProgressSnapshot(root, e, catalogs)
	settings.CardTypes["black_fingerprint"] = "curse"
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	after, access, _, err := loadout.ProgressSnapshot(root, e, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if countProgress(after["adventurer"].Deck, "black_fingerprint") != 1 || after["adventurer"].XP != p.XP {
		t.Fatal("type edit deleted cards or XP")
	}
	if len(access.Access.Problems("adventurer", after["adventurer"].Deck, after["adventurer"].Abilities)) != 1 {
		t.Fatal("owned conflict not reported")
	}
	if _, err = session.ResetCharacterLoadout("blocked", 31, "seat-a", false, "adventurer", true, "progression"); err == nil {
		t.Fatal("incompatible deck entered battle")
	}
	p = after["adventurer"]
	if _, err = loadout.Buy(root, "adventurer", e, catalogs["adventurer"], loadout.Purchase{Kind: "sell_card", ID: "black_fingerprint", ExpectedCost: 10, Revision: p.Revision}); err != nil {
		t.Fatal("cannot sell incompatible card", err)
	}
	_, _, settings, _ = loadout.ProgressSnapshot(root, e, catalogs)
	settings.CharacterTypes = map[string]string{"adventurer": "venom"}
	settings.CardTypes["pinprick"] = "venom"
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	_, access, settings, _ = loadout.ProgressSnapshot(root, e, catalogs)
	if !access.Access.Allows("adventurer", "cards", "pinprick") || access.Access.Allows("adventurer", "cards", "black_fingerprint") {
		t.Fatal("character type override ignored")
	}
	settings.CardTypes["pinprick"] = "misspelled"
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err == nil {
		t.Fatal("unknown type accepted")
	}
	// Sandbox edits are subject to the same rules, including direct native calls.
	request, _ := json.Marshal(map[string]any{"op": "save_character_deck", "content_root": contentRoot, "loadout_root": root, "character": "adventurer", "decklist": []loadout.Entry{{CardID: "black_fingerprint", Count: 1}}})
	var response map[string]any
	json.Unmarshal([]byte(HandleRuntimeRequest(string(request))), &response)
	if response["ok"] != false {
		t.Fatal("sandbox bypassed types", response)
	}
}

func TestTypeRestrictedUpgradeTargetsAndAbilityBattleConflict(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(contentRoot)
	if err != nil {
		t.Fatal(err)
	}
	e, err := loadout.LoadEconomy(contentRoot, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	lib := catalogs["adventurer"]
	settings := loadout.AdminSettings{CardTypes: map[string]string{"brace_plus": "venom"}, AbilityTypes: map[string]string{}}
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, _ := loadout.ReadProgress(root, "adventurer", e, lib)
	if _, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_card", ID: "brace", ExpectedCost: 10, Revision: p.Revision}); err == nil {
		t.Fatal("cross-type card upgrade accepted")
	}
	p, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", ExpectedCost: 25, Revision: p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	_, _, settings, _ = loadout.ProgressSnapshot(root, e, catalogs)
	settings.AbilityTypes = map[string]string{"adventurer_guard": "curse", "adventurer_guard_plus": "venom"}
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, _ = loadout.ReadProgress(root, "adventurer", e, lib)
	if _, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "downgrade_ability", ID: "adventurer_guard_plus", TargetID: "adventurer_guard", ExpectedCost: 25, Revision: p.Revision}); err == nil {
		t.Fatal("cross-type downgrade accepted")
	}
	session, err := NewSession(SessionConfig{ContentRoot: contentRoot, RunStateRoot: t.TempDir(), LoadoutRoot: root, OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.ResetCharacterLoadout("ability-conflict", 42, "seat-a", false, "adventurer", true, "progression"); err == nil {
		t.Fatal("incompatible ability entered battle")
	}
	// Default Sandbox boards are checked too, rather than bypassing type rules.
	if _, err = session.ResetCharacterLoadout("sandbox-conflict", 42, "seat-a", false, "adventurer", true, "sandbox"); err == nil {
		t.Fatal("Sandbox board bypassed type rules")
	}
	_, _, settings, _ = loadout.ProgressSnapshot(root, e, catalogs)
	settings.AbilityTypes["adventurer_guard"] = "general"
	settings.CardTypes["brace_plus"] = "general" // Shared starter now contains Brace+.
	if err = loadout.SaveAdmin(root, e, catalogs, settings); err != nil {
		t.Fatal(err)
	}
	p, _ = loadout.ReadProgress(root, "adventurer", e, lib)
	p, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "downgrade_ability", ID: "adventurer_guard_plus", TargetID: "adventurer_guard", ExpectedCost: 25, Revision: p.Revision})
	if err != nil || p.XP != 100 {
		t.Fatal("cannot resolve conflict with eligible downgrade", err, p.XP)
	}
	if _, err = session.ResetCharacterLoadout("resolved-conflict", 42, "seat-a", false, "adventurer", true, "progression"); err != nil {
		t.Fatal(err)
	}
}
