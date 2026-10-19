package expedition

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

func TestWorldClockArithmetic(t *testing.T) {
	cfg, _, _ := loadShipped(t)
	cal := cfg.Calendar
	cases := []struct {
		half   int
		year   int
		month  string
		season string
		night  bool
		label  string
	}{
		{0, 1, "early_spring", "spring", false, "Early Spring · Day"},
		{1, 1, "early_spring", "spring", true, "Early Spring · Night"},
		{10, 1, "late_summer", "summer", false, "Late Summer · Day"},
		{23, 1, "late_winter", "winter", true, "Late Winter · Night"},
		{24, 2, "early_spring", "spring", false, "Early Spring · Day"},
		{24 + 15, 2, "mid_autumn", "autumn", true, "Midautumn · Night"},
	}
	for _, c := range cases {
		clock := WorldClock{Half: c.half}
		if clock.Year(cal) != c.year || clock.Month(cal).ID != c.month || clock.Season(cal).ID != c.season || clock.Night() != c.night || clock.Label(cal) != c.label {
			t.Fatalf("half %d: year %d month %s season %s night %v label %q", c.half, clock.Year(cal), clock.Month(cal).ID, clock.Season(cal).ID, clock.Night(), clock.Label(cal))
		}
	}
	// Every month's two halves read their own month and its season.
	for i, m := range cal.Months {
		for _, half := range []int{2 * i, 2*i + 1} {
			clock := WorldClock{Half: half}
			if clock.Month(cal).ID != m.ID || clock.Season(cal).ID != m.Season || clock.Night() != (half%2 == 1) {
				t.Fatalf("half %d is %s (%s), want %s (%s)", half, clock.Month(cal).ID, clock.Season(cal).ID, m.ID, m.Season)
			}
		}
	}
	if start, ok := MonthStart(cal, "late_summer", 1); !ok || start.Half != 10 || start.Night() {
		t.Fatalf("Late Summer opens on Day half 10 of year 1, not %+v (%v)", start, ok)
	}
	if start, ok := MonthStart(cal, "early_spring", 2); !ok || start.Half != 24 {
		t.Fatalf("year 2 opens on half 24, not %+v", start)
	}
	if _, ok := MonthStart(cal, "high_summer", 1); ok {
		t.Fatal("an unknown month has no start")
	}
}

// The Salt Marches table: round 1 Late Summer Day, 2 Late Summer Night,
// 3 Early Autumn Day, 4 Early Autumn Night; the deadline on Midautumn Day.
func TestSaltMarchesRunFromLateSummerIntoAutumn(t *testing.T) {
	r := testRules(t)
	cal := r.Config.Calendar
	s := newState(t, r, 3)
	if s.Round() != 1 || s.Night() || s.Clock.Half != s.RegionStart || s.Clock.Label(cal) != "Late Summer · Day" || s.Clock.Year(cal) != 1 {
		t.Fatalf("a new expedition starts on Late Summer, Day, round 1: half %d start %d round %d (%s)", s.Clock.Half, s.RegionStart, s.Round(), s.Clock.Label(cal))
	}
	table := []string{"Late Summer · Day", "Late Summer · Night", "Early Autumn · Day", "Early Autumn · Night"}
	for round := 1; round <= len(table); round++ {
		if s.Round() != round || s.Clock.Label(cal) != table[round-1] || s.PendingBattle != nil {
			t.Fatalf("round %d reads %q (round %d, pending %v)", round, s.Clock.Label(cal), s.Round(), s.PendingBattle != nil)
		}
		rng := s.RNGPosition
		act(t, s, r, Action{Kind: "make_camp"})
		if s.Camp.Round != round || s.Round() != round {
			t.Fatalf("camp opens in round %d, not %d", s.Camp.Round, round)
		}
		half := s.Clock.Half
		act(t, s, r, Action{Kind: "break_camp"})
		if s.Clock.Half != half+1 || s.LastRest.RoundAfter != round+1 || s.LastRest.NightAfter != s.Night() {
			t.Fatalf("a rest advances the clock by one half: %d → %d, summary %+v", half, s.Clock.Half, s.LastRest)
		}
		if s.RNGPosition == rng {
			t.Fatal("the rest still shuffles (the clock itself draws nothing)")
		}
	}
	// After round 4 the deadline fight starts on Midautumn, Day.
	if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || s.Round() != 5 || s.Clock.Label(cal) != "Midautumn · Day" {
		t.Fatalf("the deadline comes after round 4 on Midautumn Day: pending %+v round %d (%s)", s.PendingBattle, s.Round(), s.Clock.Label(cal))
	}
	if s.Clock.Half != s.RegionStart+4 {
		t.Fatalf("four rests moved the clock four halves: %d vs start %d", s.Clock.Half, s.RegionStart)
	}
}

func TestExtendTimeLetsTheClockRunOneHalfFurther(t *testing.T) {
	r := withMapCards(testRules(t), 0, map[string]content.CardMapEffect{"extend": {Kind: "extend_time", Amount: 1}})
	s := newState(t, r, 3)
	// Gaining time needs a marked enemy first.
	s.Marks = []Mark{{Target: "warlord", Name: "The Drowned Regent", RestsLeft: 9}}
	s.Hand = append(s.Hand, "extend")
	act(t, s, r, Action{Kind: "play_map_card", Card: "extend"})
	if s.rounds(r) != 5 {
		t.Fatalf("extend_time adds a round: %d", s.rounds(r))
	}
	for round := 1; round <= 5; round++ {
		if s.PendingBattle != nil {
			t.Fatalf("the deadline came early, in round %d", round)
		}
		act(t, s, r, Action{Kind: "make_camp"})
		act(t, s, r, Action{Kind: "break_camp"})
	}
	if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || s.Round() != 6 || s.Clock.Label(r.Config.Calendar) != "Midautumn · Night" {
		t.Fatalf("with extend_time the deadline comes after round 5: pending %+v round %d (%s)", s.PendingBattle, s.Round(), s.Clock.Label(r.Config.Calendar))
	}
}

// legacyJSON rewrites a state's save as one written before the world clock:
// no clock and no region start, the round and Day/Night stored flat.
func legacyJSON(t *testing.T, s *State, round int, night bool) []byte {
	t.Helper()
	raw, err := json.Marshal(s)
	must(t, err)
	var generic map[string]any
	must(t, json.Unmarshal(raw, &generic))
	delete(generic, "clock")
	delete(generic, "region_start")
	generic["round"], generic["night"] = round, night
	raw, err = json.Marshal(generic)
	must(t, err)
	return raw
}

func TestLegacySaveWithoutAClockLoadsAndPlaysOn(t *testing.T) {
	r := testRules(t)
	cal := r.Config.Calendar
	base := newState(t, r, 3)
	start := base.RegionStart
	var legacy State
	must(t, json.Unmarshal(legacyJSON(t, base, 3, false), &legacy))
	if legacy.Round() != 3 || legacy.Night() {
		t.Fatalf("a legacy save reads its stored round before migrating: round %d night %v", legacy.Round(), legacy.Night())
	}
	// The view shows the migrated clock without changing the save.
	rng := legacy.RNGPosition
	v := legacy.View(r, 1)
	if v.Round != 3 || v.Night || legacy.LegacyRound != 3 || legacy.RNGPosition != rng {
		t.Fatalf("the view of a legacy save: round %d night %v (legacy round %d left in place)", v.Round, v.Night, legacy.LegacyRound)
	}
	// A modern save at the same round plays identically.
	modern := base.Clone()
	modern.Clock.Half = start + 2
	for _, a := range []Action{{Kind: "make_camp"}, {Kind: "break_camp"}} {
		act(t, &legacy, r, a)
		act(t, modern, r, a)
		if legacy.LegacyRound != 0 || legacy.RegionStart != start {
			t.Fatalf("the first action migrates the clock: legacy round %d, region start %d (want %d)", legacy.LegacyRound, legacy.RegionStart, start)
		}
		got, _ := json.Marshal(&legacy)
		want, _ := json.Marshal(modern)
		if string(got) != string(want) {
			t.Fatalf("after %s the migrated save and the modern one differ:\n%s\n%s", a.Kind, got, want)
		}
	}
	if legacy.Round() != 4 || !legacy.Night() || legacy.Clock.Label(cal) != "Early Autumn · Night" || legacy.PendingBattle != nil {
		t.Fatalf("the legacy save rested into round 4, Early Autumn Night: round %d (%s)", legacy.Round(), legacy.Clock.Label(cal))
	}
	// Migrated saves write the clock and never the legacy fields.
	raw, _ := json.Marshal(&legacy)
	var written map[string]any
	must(t, json.Unmarshal(raw, &written))
	if _, ok := written["round"]; ok {
		t.Fatal("a migrated save writes no flat round")
	}
	if _, ok := written["night"]; ok {
		t.Fatal("a migrated save writes no flat night")
	}
	if clock, ok := written["clock"].(map[string]any); !ok || int(clock["half"].(float64)) != start+3 || int(written["region_start"].(float64)) != start {
		t.Fatalf("a migrated save writes its clock: %v / %v", written["clock"], written["region_start"])
	}
}

func TestLegacySaveTrustsTheRoundOverItsNight(t *testing.T) {
	r := testRules(t)
	var legacy State
	must(t, json.Unmarshal(legacyJSON(t, newState(t, r, 3), 3, true), &legacy))
	if legacy.Night() {
		t.Fatal("round 3 is a Day round whatever the stored night says")
	}
	act(t, &legacy, r, Action{Kind: "discard", Cards: legacy.Hand[:1]})
	if legacy.Round() != 3 || legacy.Night() || legacy.Clock.Label(r.Config.Calendar) != "Early Autumn · Day" {
		t.Fatalf("a legacy round 3 migrates to Early Autumn, Day: round %d (%s)", legacy.Round(), legacy.Clock.Label(r.Config.Calendar))
	}
	if strings.Contains(strings.Join(legacy.Log, "\n"), "night") {
		t.Fatalf("the disagreement is not reported to the player: %v", legacy.Log)
	}
}

func TestSaveRoundTripKeepsTheClock(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 3)
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	again := reloaded(t, s)
	if again.Clock != s.Clock || again.RegionStart != s.RegionStart || again.Round() != 2 || !again.Night() || again.LegacyRound != 0 {
		t.Fatalf("a reload keeps the clock and region start: %+v/%d vs %+v/%d", again.Clock, again.RegionStart, s.Clock, s.RegionStart)
	}
}

func TestRegionCalendarStartIsValidated(t *testing.T) {
	lib := testLibrary(t, contentRoot(t))
	for name, c := range map[string]struct{ old, new, want string }{
		"unknown month": {"calendar_start: {month: late_summer}", "calendar_start: {month: high_summer}", `calendar_start.month "high_summer" is not a month`},
		"missing":       {"calendar_start: {month: late_summer}\n", "", `calendar_start.month "" is not a month`},
		"unknown key":   {"calendar_start: {month: late_summer}", "calendar_start: {month: late_summer, half: night}", "field half not found"},
	} {
		t.Run(name, func(t *testing.T) {
			root := scratchContent(t)
			rewrite(t, filepath.Join(root, "expedition_v1", "regions", "salt_marches.yaml"), replaceOnce(c.old, c.new))
			cfg, err := LoadConfig(root, lib)
			must(t, err)
			_, err = LoadPack(root, cfg, lib)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestViewShowsTheClock(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 3)
	v := viewJSON(t, s, r)
	clock, ok := v["clock"].(map[string]any)
	if !ok {
		t.Fatalf("the view has a clock object: %v", v["clock"])
	}
	want := map[string]any{"year": 1.0, "month": "late_summer", "month_name": "Late Summer", "season": "summer", "season_name": "Summer", "night": false, "label": "Late Summer · Day"}
	for key, value := range want {
		if clock[key] != value {
			t.Fatalf("clock.%s is %v, not %v (%v)", key, clock[key], value, clock)
		}
	}
	if rules, _ := clock["season_rules"].(string); !strings.Contains(rules, "No seasonal map effects yet") {
		t.Fatalf("the season's rules text reaches the view: %q", rules)
	}
	if effects := requireList(t, v, "clock", "season_effects"); len(effects) != 0 {
		t.Fatalf("season_effects is an empty list: %v", effects)
	}
	if v["day_label"] != "Late Summer · Day · round 1 of 4" || v["round"] != 1.0 || v["rounds"] != 4.0 || v["night"] != false {
		t.Fatalf("day_label and the round fields: %v / %v / %v / %v", v["day_label"], v["round"], v["rounds"], v["night"])
	}
	// The rest names the half that begins, in the log and in its summary.
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	view := s.View(r, 1)
	if view.DayLabel != "Late Summer · Night · round 2 of 4" || view.Clock.Label != "Late Summer · Night" || !view.Clock.Night {
		t.Fatalf("after the first rest: %q / %+v", view.DayLabel, view.Clock)
	}
	if after := view.LastRest.ClockAfter; after == nil || after.MonthName != "Late Summer" || !after.Night || view.LastRest.RoundAfter != 2 {
		t.Fatalf("the rest summary carries the half that began: %+v", view.LastRest.ClockAfter)
	}
	if s.LastRest.ClockAfter != nil {
		t.Fatal("the saved rest summary stores no time of its own")
	}
	if last := s.Log[len(s.Log)-1]; last != "Night falls in Late Summer." {
		t.Fatalf("the rest's log line: %q", last)
	}
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	view = s.View(r, 1)
	if view.DayLabel != "Early Autumn · Day · round 3 of 4" || view.LastRest.ClockAfter.MonthName != "Early Autumn" || view.LastRest.ClockAfter.Night || s.Log[len(s.Log)-1] != "Early Autumn begins." {
		t.Fatalf("after the second rest: %q, %+v, %q", view.DayLabel, view.LastRest.ClockAfter, s.Log[len(s.Log)-1])
	}
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	view = s.View(r, 1)
	if view.DayLabel != "The deadline" || view.Clock.MonthName != "Midautumn" || view.Clock.Night {
		t.Fatalf("at the deadline: %q, %+v", view.DayLabel, view.Clock)
	}
}
