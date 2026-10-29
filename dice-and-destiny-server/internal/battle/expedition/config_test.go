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
	if cfg.Warlord.StartDistance != 5 || cfg.Warlord.MarchPerRest != 2 || cfg.Warlord.EarlyReward.XPPerRound != 30 || cfg.Warlord.EarlyReward.ExtraRelicChoices != 1 {
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
	if cfg.Gold.Prices["bandage"].Min != 10 {
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
		// Gold never buys XP: the removed trainer rate is an unknown field now.
		{"gold for XP", "healer_grievous: 50}", "healer_grievous: 50}\n  trainer_gold_per_xp: 3", "field trainer_gold_per_xp not found"},
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

func TestLoadConfigReadsTheCalendar(t *testing.T) {
	cfg, _, _ := loadShipped(t)
	cal := cfg.Calendar
	if len(cal.Months) != 12 || len(cal.Seasons) != 4 || cal.HalvesPerYear() != 24 {
		t.Fatalf("the shipped calendar has 12 months and four seasons: %d months, %d seasons", len(cal.Months), len(cal.Seasons))
	}
	perSeason := map[string]int{}
	for _, m := range cal.Months {
		perSeason[m.Season]++
	}
	for _, s := range cal.Seasons {
		if perSeason[s.ID] != 3 {
			t.Fatalf("season %s has %d months, not three", s.ID, perSeason[s.ID])
		}
		if len(s.Effects) != 0 || !strings.Contains(s.RulesText, "No seasonal map effects yet") {
			t.Fatalf("season %s promises no effects yet: %+v", s.ID, s)
		}
	}
	if cal.Months[0].ID != "early_spring" || cal.Months[0].Name != "Early Spring" {
		t.Fatalf("the year opens in early spring, not %+v", cal.Months[0])
	}
	if i, ok := cal.MonthIndex("late_summer"); !ok || i != 5 || cal.Months[i].Name != "Late Summer" {
		t.Fatalf("Late Summer is month 6, not index %d (%v)", i, ok)
	}
}

func TestLoadConfigRejectsBadCalendars(t *testing.T) {
	lib := testLibrary(t, contentRoot(t))
	noMonths := func(s string) string {
		i := strings.Index(s, "  months:\n")
		if i < 0 {
			return s
		}
		return s[:i] + "  months: []\n"
	}
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown season on a month", replaceOnce("{id: late_summer,  name: Late Summer,  season: summer}", "{id: late_summer,  name: Late Summer,  season: sumer}"), `calendar month "late_summer" names unknown season "sumer"`},
		{"duplicate month id", replaceOnce("{id: mid_spring,   name: Midspring", "{id: early_spring,   name: Midspring"), `calendar month "early_spring" needs a unique snake_case id`},
		{"duplicate season id", replaceOnce("{id: winter, name: Winter", "{id: autumn, name: Winter"), `calendar season "autumn" needs a unique snake_case id`},
		{"empty month list", noMonths, "calendar.months needs at least one month"},
		{"seasonal effects", replaceOnce("No seasonal map effects yet.\", effects: []}\n  months:", "No seasonal map effects yet.\", effects: [{terrain: snow}]}\n  months:"), "seasonal effects arrive in Phase 4"},
		{"season without a month", replaceOnce("  months:\n", "    - {id: monsoon, name: Monsoon, rules_text: \"Rain.\", effects: []}\n  months:\n"), `calendar season "monsoon" has no month`},
		{"nameless month", replaceOnce("name: Midwinter,", "name: \"\","), `calendar month "mid_winter" needs a name`},
		{"unknown key under calendar", replaceOnce("calendar:\n", "calendar:\n  leap_months: 1\n"), "field leap_months not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := scratchContent(t)
			rewrite(t, filepath.Join(root, "progression_v1", "expedition.yaml"), c.edit)
			_, err := LoadConfig(root, lib)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
}
