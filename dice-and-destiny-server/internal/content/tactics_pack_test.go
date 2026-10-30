package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tacticsBaseLibrary(t *testing.T) BattleLibrary {
	t.Helper()
	root := filepath.Join("..", "..", "content")
	lib, err := LoadBattleLibrary(filepath.Join(root, "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"venom_v1", "general_v1", "minions_v1", "adventurer_v1"} {
		if lib, err = LoadBattleExtension(lib, filepath.Join(root, pack)); err != nil {
			t.Fatalf("%s: %v", pack, err)
		}
	}
	return lib
}

func TestTacticsPackLoadsCopiesAndSandboxCombatants(t *testing.T) {
	base := tacticsBaseLibrary(t)
	shippedLash, _ := json.Marshal(base.Abilities["brine_lash"])
	lib, manifest, err := ApplyTacticsPack(base, filepath.Join("..", "..", "content", "tactics_v1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Heroes) != 2 || len(manifest.Enemies) != 4 {
		t.Fatalf("manifest = %+v", manifest)
	}
	lash := lib.Abilities["brine_lash"]
	if lash.Delivery == nil || lash.Delivery.Range != 4 || !lash.Delivery.LineOfSight || lash.Name != "Brine Lash" {
		t.Fatalf("tactics Brine Lash = %+v", lash.Delivery)
	}
	if after, _ := json.Marshal(base.Abilities["brine_lash"]); string(after) != string(shippedLash) {
		t.Fatal("applying the pack changed the shipped library")
	}
	if crown := lib.Abilities["tide_crown"].Delivery; crown == nil || crown.Type != DeliveryTypeArea || crown.Area.Radius != 1 || crown.Range != 5 {
		t.Fatalf("Tide Crown delivery = %+v", crown)
	}
	if !lib.Abilities["slip_the_current"].OnTheMove || !lib.Abilities["evade"].OnTheMove || lib.Abilities["adventurer_guard"].OnTheMove {
		t.Fatal("on-the-move tags are wrong")
	}
	if charge := lib.Abilities["charge"].Delivery; charge == nil || charge.Charge == nil || charge.Charge.Range != 4 || charge.Range != 1 {
		t.Fatalf("Charge delivery = %+v", charge)
	}
	if mask := lib.Combatants["tactics_brine_mask"]; mask.Name != "Brine Mask" || mask.Tactics.Role != TacticsRoleRanged || len(mask.AbilityBoard.Offensive) != 2 {
		t.Fatalf("tactics Brine Mask = %+v", mask)
	}
	if eel := lib.Combatants["tactics_ribbon_eel"]; eel.Tactics.BaseSpeed != 2 {
		t.Fatalf("Ribbon Eel speed = %+v", eel.Tactics)
	}
	if running := lib.Statuses["running"]; running.Modifiers == nil || !running.Modifiers.OnTheMoveDefensesOnly {
		t.Fatal("Running status must carry on_the_move_defenses_only")
	}
	// Every new ability has full rules text that names its delivery.
	for id, phrase := range map[string]string{"magic_missile": "Range 6, line of sight", "fireball": "Area 3×3, range 5", "charge": "Charge 4", "brine_lash": "Range 4, line of sight", "tide_crown": "Area 3×3, range 5", "evade": "On the move"} {
		if !strings.Contains(lib.Abilities[id].Presentation.RulesText, phrase) {
			t.Errorf("%s rules text lacks %q", id, phrase)
		}
	}
}

func TestShippedCatalogJSONIgnoresTacticsFields(t *testing.T) {
	lib := tacticsBaseLibrary(t)
	raw, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"delivery"`, `"on_the_move"`, `"tactics"`, `"on_the_move_defenses_only"`} {
		if strings.Contains(string(raw), field) {
			t.Errorf("shipped catalog JSON contains %s", field)
		}
	}
}

func TestTacticsPackRefusesInvalidContent(t *testing.T) {
	base := tacticsBaseLibrary(t)
	src := filepath.Join("..", "..", "content", "tactics_v1")
	cases := map[string]func(dir string){
		"area without area shape": func(dir string) {
			rewrite(t, filepath.Join(dir, "abilities", "fireball.yaml"), "  area: {shape: square, radius: 1}\n", "")
		},
		"charge on an area": func(dir string) {
			rewrite(t, filepath.Join(dir, "abilities", "fireball.yaml"), "  area: {shape: square, radius: 1}\n", "  area: {shape: square, radius: 1}\n  charge: {range: 3}\n")
		},
		"zero range": func(dir string) {
			rewrite(t, filepath.Join(dir, "abilities", "magic_missile.yaml"), "range: 6", "range: 0")
		},
		"unknown delivery field": func(dir string) {
			rewrite(t, filepath.Join(dir, "abilities", "magic_missile.yaml"), "line_of_sight: true}", "line_of_sight: true, curve: true}")
		},
		"on the move on an attack": func(dir string) {
			rewrite(t, filepath.Join(dir, "abilities", "magic_missile.yaml"), "delivery:", "on_the_move: true\ndelivery:")
		},
		"undeclared copy": func(dir string) {
			rewrite(t, filepath.Join(dir, "pack.yaml"), "ability_copies: [brine_lash, tide_crown, slip_the_current]", "ability_copies: [brine_lash, tide_crown]")
		},
		"enemy without speed": func(dir string) {
			rewrite(t, filepath.Join(dir, "combatants", "tactics_bell_diver.yaml"), "base_speed: 1", "base_speed: 0")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, src, dir)
			mutate(dir)
			_, _, err := ApplyTacticsPack(base, dir)
			if err == nil {
				t.Fatal("invalid tactics pack was accepted")
			}
			t.Log(err)
		})
	}
}

func rewrite(t *testing.T, path, old, replacement string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("%s lacks %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
