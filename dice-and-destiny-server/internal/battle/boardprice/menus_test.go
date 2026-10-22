package boardprice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

func contentRoot() string { return filepath.Join("..", "..", "..", "content") }

// testCatalog is the custom adventurer's catalog: the base rules, the
// character packs and the board dice.
func testCatalog(t testing.TB) content.BattleLibrary {
	t.Helper()
	lib, err := content.LoadBattleLibrary(filepath.Join(contentRoot(), "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"venom_v1", "curse_v1", "adventurer_v1", "general_v1", "minions_v1", "custom_adventurer_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join(contentRoot(), pack)); err != nil {
			t.Fatalf("%s: %v", pack, err)
		}
	}
	return lib
}

func testMenus(t testing.TB) Menus {
	t.Helper()
	m, err := LoadMenus(contentRoot(), testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestShippedMenusLoad(t *testing.T) {
	m := testMenus(t)
	if len(m.Dice) != 5 || len(m.Defenses) != 2 || len(m.Templates) != 13 {
		t.Fatalf("menus: %d dice, %d defenses, %d templates", len(m.Dice), len(m.Defenses), len(m.Templates))
	}
	perFamily := map[string]int{}
	for _, tpl := range m.Templates {
		perFamily[tpl.Family]++
	}
	if perFamily["321"] != 6 || perFamily["3111"] != 3 || perFamily["2211"] != 4 {
		t.Fatalf("templates per family %v", perFamily)
	}
	want := map[string]float64{"none": 0, "protect_1": 1, "protect_2": 2, "energy_5": 1, "energy_10": 2, "draw_1": 1, "heal_1": 1, "heal_2": 2}
	for id, value := range want {
		rider, ok := m.Rider(id)
		if !ok {
			t.Fatalf("rider %s missing", id)
		}
		got, err := m.Rates.Value(rider.Operations)
		if err != nil || got != value {
			t.Fatalf("rider %s is worth %v (%v); want %v", id, got, err, value)
		}
	}
	if m.Rates.Status["protect"] != 1.0 || m.Rates.Energy != 0.2 {
		t.Fatalf("agreed exchange rates changed: %+v", m.Rates)
	}
}

func TestParseCost(t *testing.T) {
	for text, want := range map[string]string{
		"main3/main4/main5":       "main3_main4_main5",
		"side3+main2":             "main2_side3",
		"SS/LS":                   "ss_ls",
		"main1+odd1+odd_b1+apex1": "main1_odd1_odd_b1_apex1",
		"apex5":                   "apex5",
	} {
		cost, err := ParseCost(text)
		if err != nil || cost.Key() != want {
			t.Fatalf("%s: key %q (%v); want %q", text, cost.Key(), err, want)
		}
	}
	for _, bad := range []string{"", "wild2", "main0", "main2+main1", "main3/main3", "SS+main1", "Main3"} {
		if _, err := ParseCost(bad); err == nil {
			t.Fatalf("%q parsed", bad)
		}
	}
}

// withMenus copies the shipped menu files into a scratch content root after
// replacing old with new in one of them, and loads them.
func withMenus(t *testing.T, file, old, new string) error {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "progression_v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"board_templates.yaml", "riders.yaml"} {
		data, err := os.ReadFile(filepath.Join(contentRoot(), "progression_v1", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if name == file {
			if !strings.Contains(text, old) {
				t.Fatalf("%s does not contain %q", name, old)
			}
			text = strings.Replace(text, old, new, 1)
		}
		if err := os.WriteFile(filepath.Join(root, "progression_v1", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := LoadMenus(root, testCatalog(t))
	return err
}

func TestMenusAreStrict(t *testing.T) {
	if err := withMenus(t, "riders.yaml", "", ""); err != nil {
		t.Fatalf("unchanged copy rejected: %v", err)
	}
	cases := []struct{ name, file, old, new string }{
		{"unknown field", "board_templates.yaml", "  alpha: 0.2", "  alpha: 0.2\n  bonus: 1"},
		{"locked die", "board_templates.yaml", "{id: forge_d6, unlock: open}", "{id: forge_d6, unlock: level_3}"},
		{"locked rider", "riders.yaml", "    jobs: [flex, apex, ultimate]\n    unlock: open", "    jobs: [flex, apex, ultimate]\n    unlock: later"},
		{"unknown die", "board_templates.yaml", "{id: loom_d6, unlock: open}", "{id: loom_d8, unlock: open}"},
		{"unreachable template cost", "board_templates.yaml", "{id: full_house, job: full_house, cost: main3+side2, rider: true}", "{id: full_house, job: full_house, cost: main3+odd2, rider: true}"},
		{"unreachable menu cost", "board_templates.yaml", "      - {cost: side3+apex2, unlock: open}", "      - {cost: odd_b2+apex2, unlock: open}"},
		{"too many dice", "board_templates.yaml", "{id: full_house, job: full_house, cost: main3+side2, rider: true}", "{id: full_house, job: full_house, cost: main3+side3, rider: true}"},
		{"flex default off menu", "board_templates.yaml", "{id: flex_b, job: flex, bands: [medium], cost: main2+apex2}", "{id: flex_b, job: flex, bands: [medium], cost: main4+side1}"},
		{"unknown band", "board_templates.yaml", "bands: [easy], cost: main1+side3}", "bands: [trivial], cost: main1+side3}"},
		{"unknown status", "riders.yaml", "status_id: protect, stack_count: 1", "status_id: shiny, stack_count: 1"},
		{"negative status", "riders.yaml", "status_id: protect, stack_count: 1", "status_id: poison, stack_count: 1"},
		{"over the cap", "riders.yaml", "status_id: protect, stack_count: 2", "status_id: protect, stack_count: 3"},
		{"enemy target", "riders.yaml", "{type: draw_cards, target: self, amount: 1}", "{type: draw_cards, target: enemy, amount: 1}"},
		{"no rate", "riders.yaml", "{type: draw_cards, target: self, amount: 1}", "{type: deal_damage, target: self, amount: 1}"},
		{"unknown job", "riders.yaml", "jobs: [flex, apex, ultimate]", "jobs: [flex, apex, finisher]"},
		{"face defense", "board_templates.yaml", "{ability: holdfast_plus, unlock: open}", "{ability: adventurer_guard_plus, unlock: open}"},
	}
	for _, c := range cases {
		err := withMenus(t, c.file, c.old, c.new)
		if err == nil {
			t.Fatalf("%s accepted", c.name)
		}
		t.Logf("%s: %v", c.name, err)
	}
}
