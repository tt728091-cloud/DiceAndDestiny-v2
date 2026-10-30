package tactics

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const packRoot = "../../../content/tactics_v1"

func loadShippedPack(t *testing.T) Pack {
	t.Helper()
	pack, err := LoadPack(packRoot)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func TestShippedPackLoads(t *testing.T) {
	pack := loadShippedPack(t)
	r := pack.Rules
	if r.BaseSpeed != 1 || r.CardMove != 1 || r.EnergyPerMove != 5 || r.MapCardMoveMultiplier != 1 || r.SprintBaseMultiplier != 2 || r.SprintCardBonus != 1 {
		t.Fatalf("move numbers = %+v", r)
	}
	if r.DiagonalRule != DiagonalChebyshev || r.CornerCutting || !r.LOSBlockedByCreatures || r.AreaHitsAllies || r.ChargeCountsAsRunning || r.ChargeAllowsExtraMove || r.AI.MaxCardsPerRound != 1 {
		t.Fatalf("defaults = %+v", r)
	}
	if r.Terrain["swamp"].Cost != 2 || r.Terrain["wall"].Passable || !r.Terrain["wall"].BlocksLineOfSight {
		t.Fatalf("terrain = %+v", r.Terrain)
	}
	var ids []string
	for _, a := range pack.SortedArenas() {
		ids = append(ids, a.ID)
		if a.Width() != 12 || a.Height() != 9 || len(a.EnemyStarts) != 4 {
			t.Errorf("%s is %d×%d with %d enemy starts", a.ID, a.Width(), a.Height(), len(a.EnemyStarts))
		}
	}
	if strings.Join(ids, ",") != "open_field,doorway,pillar_hall,swamp_crossing" {
		t.Fatalf("arenas = %v", ids)
	}
	door := pack.Arenas["doorway"]
	if door.TerrainAt(Square{5, 4}) != "floor" || door.TerrainAt(Square{5, 3}) != "wall" {
		t.Fatal("the doorway must be a single floor square in the wall")
	}
	swamp := pack.Arenas["swamp_crossing"]
	if swamp.TerrainAt(Square{5, 0}) != "swamp" || swamp.TerrainAt(Square{3, 0}) != "floor" {
		t.Fatal("swamp band misplaced")
	}
}

func TestLoaderRefusesInvalidRulesAndArenas(t *testing.T) {
	cases := map[string][3]string{
		"unknown rule field":     {"rules.yaml", "base_speed: 1\n", "base_speed: 1\nwarp_speed: 9\n"},
		"zero base speed":        {"rules.yaml", "base_speed: 1\n", "base_speed: 0\n"},
		"bad diagonal rule":      {"rules.yaml", "diagonal_rule: chebyshev", "diagonal_rule: manhattan"},
		"random not last":        {"rules.yaml", "tiebreak_order: [squares_moved, move_rate, paid_move, random]", "tiebreak_order: [random, squares_moved]"},
		"unknown tiebreak":       {"rules.yaml", "tiebreak_order: [squares_moved, move_rate, paid_move, random]", "tiebreak_order: [initiative, random]"},
		"impassable with cost":   {"rules.yaml", "wall: {name: Wall, cost: 0", "wall: {name: Wall, cost: 3"},
		"ragged arena row":       {"arenas/doorway.yaml", "  - \"............\"\n", "  - \"...........\"\n"},
		"unknown glyph":          {"arenas/open_field.yaml", "  - \"............\"\n", "  - \"......X.....\"\n"},
		"start inside a wall":    {"arenas/doorway.yaml", "  - {x: 9, y: 4}", "  - {x: 5, y: 0}"},
		"too many enemy starts":  {"arenas/open_field.yaml", "  - {x: 8, y: 1}\n", "  - {x: 8, y: 1}\n  - {x: 7, y: 1}\n"},
		"duplicate start square": {"arenas/open_field.yaml", "  - {x: 8, y: 1}\n", "  - {x: 1, y: 4}\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			copyDir(t, packRoot, dir)
			path := filepath.Join(dir, c[0])
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), c[1]) {
				t.Fatalf("%s lacks %q", c[0], c[1])
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(raw), c[1], c[2], 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = LoadPack(dir)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("LoadPack = %v, want an invalid-config error", err)
			}
		})
	}
}

func TestOverridesMergeStrictly(t *testing.T) {
	rules := loadShippedPack(t).Rules
	got, err := ApplyOverrides(rules, json.RawMessage(`{"base_speed": 3, "terrain": {"swamp": {"cost": 3}}, "ai": {"max_cards_per_round": 2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseSpeed != 3 || got.Terrain["swamp"].Cost != 3 || got.Terrain["swamp"].Name != "Swamp" || got.AI.MaxCardsPerRound != 2 || got.CardMove != 1 {
		t.Fatalf("merged = %+v", got)
	}
	if rules.BaseSpeed != 1 || rules.Terrain["swamp"].Cost != 2 {
		t.Fatal("overrides changed the loaded rules")
	}
	for _, bad := range []string{`{"warp": 1}`, `{"base_speed": 0}`, `{"terrain": {"lava": {"cost": 5}}}`, `{"diagonal_rule": "hex"}`, `[1]`} {
		if _, err := ApplyOverrides(rules, json.RawMessage(bad)); err == nil {
			t.Errorf("override %s was accepted", bad)
		}
	}
}

func copyDir(t *testing.T, from, to string) {
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
