package expedition

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAcceptsShippedValues(t *testing.T) {
	cfg, _, _ := loadShipped(t)
	if cfg.RoundsPerRegion != 4 || cfg.Hand.Size != 4 || cfg.Hand.Limit != 6 || cfg.Move.EnergyPerMove != 5 || cfg.Move.ExploreCost != 2 {
		t.Fatalf("shipped expedition.yaml changed its first values: %+v", cfg)
	}
	if cost, ok := cfg.TerrainCost("forest", true); !ok || cost != 3 {
		t.Fatalf("forest at night costs 3, not %d (%v)", cost, ok)
	}
	if cost, ok := cfg.TerrainCost("forest", false); !ok || cost != 2 {
		t.Fatalf("forest by day costs 2, not %d", cost)
	}
	if _, ok := cfg.TerrainCost("lake", false); ok {
		t.Fatal("lakes are impassable")
	}
	if _, ok := cfg.TerrainCost("mountain", true); ok {
		t.Fatal("mountains are impassable")
	}
	if cfg.SightRange(false) != 2 || cfg.SightRange(true) != 1 {
		t.Fatal("sight is 2 by day and 1 at night")
	}
	if cfg.Ambush.BonusDice != 3 || cfg.Ambush.MaxKept != 5 || cfg.Fortified.ExtraDefenseDice != 1 || cfg.Conqueror.ExtraDefenseDice != 1 || cfg.Defeat.Wounds != 2 {
		t.Fatalf("ambush/fortified/defeat values changed: %+v", cfg)
	}
	if cfg.CampHealing.BasicAidDie != 3 || cfg.CampHealing.BandageDie != 6 || cfg.CampHealing.InnBonus != 1 {
		t.Fatalf("camp healing values changed: %+v", cfg.CampHealing)
	}
	if cfg.Warlord.StartDistance != 5 || cfg.Warlord.MarchPerRest != 2 || cfg.Warlord.EarlyReward.XPPerRound != 20 || cfg.Warlord.EarlyReward.ExtraRelicChoices != 1 {
		t.Fatalf("warlord values changed: %+v", cfg.Warlord)
	}
	want := map[string]int{"camp": 3, "village": 2, "monastery": 1, "town": 0}
	for site, n := range want {
		if cfg.DeckChangeInfluence[site] != n {
			t.Fatalf("influence to open the deck at %s is %d, not %d", site, cfg.DeckChangeInfluence[site], n)
		}
	}
	if cfg.Mending.VillageInfluencePerWound != 2 || cfg.Mending.MonasteryInfluencePerWound != 1 {
		t.Fatalf("mending costs changed: %+v", cfg.Mending)
	}
	if d := cfg.Gold.Drops["minion_group"]; d.Min != 5 || d.Max != 15 {
		t.Fatalf("minion gold drop is %v", d)
	}
	if d := cfg.Gold.Drops["lieutenant"]; d.Min != 30 || d.Max != 30 {
		t.Fatalf("a scalar range reads as min == max: %v", d)
	}
	if cfg.Gold.TrainerGoldPerXP != 3 || cfg.Gold.Prices["bandage"].Min != 10 {
		t.Fatalf("gold economy changed: %+v", cfg.Gold)
	}
	if cfg.Statuses.Ambush != "ambush" || cfg.Statuses.Fortified != "fortified" || cfg.Statuses.Conqueror != "conqueror" || cfg.Statuses.Lieutenant != "lieutenants_favor" {
		t.Fatalf("status ids changed: %+v", cfg.Statuses)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	lib := testLibrary(t, contentRoot(t))
	cases := []struct {
		name string
		old  string
		new  string
		want string
	}{
		{"unknown field", "rounds_per_region: 4", "rounds_per_region: 4\nturn_timer: 30", "field turn_timer not found"},
		{"schema version", "schema_version: 1", "schema_version: 2", "schema_version must be 1"},
		{"zero rounds", "rounds_per_region: 4", "rounds_per_region: 0", "rounds_per_region must be 1–100"},
		{"hand limit below size", "hand: {size: 4, limit: 6}", "hand: {size: 4, limit: 3}", "hand.limit must be 4–20"},
		{"energy per move", "energy_per_move: 5", "energy_per_move: 0", "move.energy_per_move must be"},
		{"missing terrain", "  marsh:    {day: 3, night: 3}\n", "", "terrain \"marsh\" is missing"},
		{"impassable with cost", "lake:     {impassable: true}", "lake:     {impassable: true, day: 1}", "impassable terrain \"lake\" has no cost"},
		{"terrain cost range", "forest:   {day: 2, night: 3, cover: true}", "forest:   {day: 2, night: 0, cover: true}", "terrain.forest.night must be 1–20"},
		{"ambush keep", "max_kept: 5", "max_kept: 4", "ambush.max_kept must be 5–5"},
		{"basic aid die", "basic_aid_die: 3", "basic_aid_die: 0", "camp_healing.basic_aid_die must be 1–20"},
		{"bandage die", "bandage_die: 6", "bandage_die: 21", "camp_healing.bandage_die must be 1–20"},
		{"missing deck-change site", "deck_change_influence: {camp: 3, village: 2, monastery: 1, town: 0}", "deck_change_influence: {camp: 3, village: 2, monastery: 1}", "deck_change_influence.town is missing"},
		{"unknown deck-change site", "monastery: 1, town: 0}", "monastery: 1, town: 0, inn: 1}", "unknown deck-change site \"inn\""},
		{"bad gold range", "minion_group: [5, 15]", "minion_group: [15, 5]", "gold.drops.minion_group is not a valid range"},
		{"missing price", "bandage: 10, ", "", "gold.prices.bandage is missing"},
		{"bad range shape", "treasure: [20, 40]", "treasure: [20, 30, 40]", "a range is one number or [min, max]"},
		{"unknown status", "ambush: ambush,", "ambush: surprise,", "names unknown status \"surprise\""},
		{"status without the right modifier", "fortified: fortified,", "fortified: blind,", "status \"blind\" must add defensive dice per stack"},
		{"lieutenant status without damage", "lieutenant: lieutenants_favor", "lieutenant: fortified", "status \"fortified\" must add attack damage per stack"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := scratchContent(t)
			rewrite(t, filepath.Join(root, "progression_v1", "expedition.yaml"), replaceOnce(c.old, c.new))
			_, err := LoadConfig(root, lib)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
}

// Changing a value changes behaviour without code changes (criterion 23).
func TestConfigValuesDriveTerrainAndSight(t *testing.T) {
	lib := testLibrary(t, contentRoot(t))
	root := scratchContent(t)
	path := filepath.Join(root, "progression_v1", "expedition.yaml")
	rewrite(t, path, replaceOnce("marsh:    {day: 3, night: 3}", "marsh:    {day: 4, night: 5}"))
	rewrite(t, path, replaceOnce("sight: {day: 2, night: 1}", "sight: {day: 3, night: 2}"))
	cfg, err := LoadConfig(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	if day, _ := cfg.TerrainCost("marsh", false); day != 4 {
		t.Fatalf("marsh by day now costs 4, not %d", day)
	}
	if night, _ := cfg.TerrainCost("marsh", true); night != 5 {
		t.Fatalf("marsh at night now costs 5, not %d", night)
	}
	if cfg.SightRange(false) != 3 || cfg.SightRange(true) != 2 {
		t.Fatal("sight follows the config")
	}
}
