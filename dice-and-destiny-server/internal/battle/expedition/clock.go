package expedition

import "fmt"

// The world clock. Every time-based rule (the region's deadline today; quest
// timers, critical-wound durations and Dark Tiers later) counts the same
// clock, through these methods and the calendar in the config. The clock
// never draws from the RNG.

// WorldClock is world time, not the hero's: it counts half-months since the
// world began (0 = Day of month 1, year 1). Odd halves are Night.
type WorldClock struct {
	Half int `json:"half"`
}

// Night reports whether this half is the Night half of its month.
func (c WorldClock) Night() bool { return c.Half%2 == 1 }

// Year counts from 1.
func (c WorldClock) Year(cal CalendarConfig) int { return c.Half/cal.HalvesPerYear() + 1 }

// Month is the calendar month this half falls in.
func (c WorldClock) Month(cal CalendarConfig) MonthConfig {
	return cal.Months[c.Half%cal.HalvesPerYear()/2]
}

// Season is the season of this half's month.
func (c WorldClock) Season(cal CalendarConfig) SeasonConfig {
	season, _ := cal.Season(c.Month(cal).Season)
	return season
}

// Label names the half for the player: "Late Summer · Day".
func (c WorldClock) Label(cal CalendarConfig) string {
	return fmt.Sprintf("%s · %s", c.Month(cal).Name, dayName(c.Night()))
}

// MonthStart is the clock at the Day half that opens a month in a year.
func MonthStart(cal CalendarConfig, month string, year int) (WorldClock, bool) {
	i, ok := cal.MonthIndex(month)
	if !ok || year < 1 {
		return WorldClock{}, false
	}
	return WorldClock{Half: (year-1)*cal.HalvesPerYear() + 2*i}, true
}

// regionStart is the clock half at which a region's first round begins: the
// Day half opening its calendar_start month in year 1.
func regionStart(cfg Config, region Region) int {
	start, _ := MonthStart(cfg.Calendar, region.CalendarStart.Month, 1)
	return start.Half
}

// Round is the scenario's round, the deadline counter: 1 on the region's
// first half, one more after every rest.
func (s *State) Round() int {
	if s.LegacyRound > 0 {
		// A save from before the world clock, not migrated yet.
		return s.LegacyRound
	}
	return s.Clock.Half - s.RegionStart + 1
}

// Night reports whether the world clock is in a Night half.
func (s *State) Night() bool {
	if s.LegacyRound > 0 {
		// Round 1 was always Day and every rest flipped it.
		return s.LegacyRound%2 == 0
	}
	return s.Clock.Night()
}

// roundClock is the world clock at a round of this region (round 1 is the
// region's first half); records stamped with a round read their date here.
func (s *State) roundClock(round int) WorldClock {
	return WorldClock{Half: s.RegionStart + round - 1}
}

// advanceClock moves world time on by one half. Today the trigger is one
// hero's pass through the deck (the rest); multi-hero play can change the
// trigger without touching this format.
func (s *State) advanceClock() {
	s.Clock.Half++
}

// migrateClock gives a save written before the world clock its clock: the
// region's calendar start plus the rounds it had played. Round 1 was always
// Day and every rest flipped Day and Night, so the stored night value follows
// from the round; when it disagrees the round wins. The legacy fields are
// cleared and never written again.
func (s *State) migrateClock(r Rules) {
	if s.LegacyRound <= 0 {
		return
	}
	s.RegionStart = regionStart(r.Config, r.Pack.Regions[s.Region])
	s.Clock = s.roundClock(s.LegacyRound)
	s.LegacyRound, s.LegacyNight = 0, false
}
