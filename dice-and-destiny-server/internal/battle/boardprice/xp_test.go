package boardprice

import (
	"math"
	"testing"
)

var testRates = XPRates{DamagePoint: 40, PreventionPoint: 40, Block: BlockRate{Share: 0.5, Offset: 200}, DefenseExtra: map[string]int{"holdfast_plus": 38}}

// Every menu defense has the same value on every menu die, and prices at 40
// XP a point of prevention (flat) or block chance × ½ × (total XP + 200).
func TestMenuDefensesPriceTheSameOnEveryDie(t *testing.T) {
	b := testBuilder(t)
	want := map[string]struct {
		value     float64
		blocks    bool
		xp, xp900 int
	}{
		"holdfast":      {3.5, false, 140, 140},
		"holdfast_plus": {3.5, false, 178, 178},
		"light_guard":   {7.0 / 6, false, 47, 47},
		"firm_guard":    {7.0 / 3, false, 93, 93},
		"bastion":       {14.0 / 3, false, 187, 187},
		"clean_deflect": {1.0 / 6, true, 17, 92},
		"twin_deflect":  {11.0 / 36, true, 31, 168},
	}
	for _, entry := range b.Menus.Defenses {
		w, ok := want[entry.Ability]
		if !ok {
			t.Fatalf("no expected price for %s", entry.Ability)
		}
		for _, d := range b.Menus.Dice {
			v, err := ValueDefense(b.Lib.Abilities[entry.Ability], b.Lib.Dice[d.ID])
			if err != nil {
				t.Fatalf("%s on %s: %v", entry.Ability, d.ID, err)
			}
			got := v.Prevention
			if v.Blocks {
				got = v.Block
			}
			if v.Blocks != w.blocks || math.Abs(got-w.value) > 1e-9 {
				t.Fatalf("%s on %s: %+v, want %.4f blocks=%v", entry.Ability, d.ID, v, w.value, w.blocks)
			}
			if xp, xp900 := testRates.DefenseXP(entry.Ability, v, 0), testRates.DefenseXP(entry.Ability, v, 900); xp != w.xp || xp900 != w.xp900 {
				t.Fatalf("%s on %s: %d XP at 0 and %d at 900, want %d and %d", entry.Ability, d.ID, xp, xp900, w.xp, w.xp900)
			}
		}
	}
}

// The damage level costs 40 XP a point of average damage over the standard
// step: free at (and below) the standard level, never less on a higher step,
// and the top step costs the whole climb.
func TestLevelXPClimbsTheStaircase(t *testing.T) {
	b := testBuilder(t)
	_, sc, err := b.Staircase(b.Default("lean", "standard_d6"))
	if err != nil {
		t.Fatal(err)
	}
	std := b.Menus.Pricing.Target
	if xp := testRates.LevelXP(sc, std, std); xp != 0 {
		t.Fatalf("standard level costs %d XP", xp)
	}
	if xp := testRates.LevelXP(sc, std, b.Menus.Pricing.Staircase.From); xp != 0 {
		t.Fatalf("a level below standard costs %d XP", xp)
	}
	last := 0
	for _, step := range sc.Steps[sc.At(std):] {
		xp := testRates.LevelXP(sc, std, step.At)
		if xp < last {
			t.Fatalf("step at %.3f costs %d XP, less than the step below (%d)", step.At, xp, last)
		}
		last = xp
	}
	top := sc.Steps[len(sc.Steps)-1].Average - sc.Steps[sc.At(std)].Average
	if want := int(math.Round(top * 40)); last != want || last < 400 {
		t.Fatalf("top step costs %d XP, want %d", last, want)
	}
}

// A defense slot may be left empty, and the level must stay on the
// staircase from the standard target up.
func TestResolveTakesNoDefenseAndLevels(t *testing.T) {
	b := testBuilder(t)
	c := b.Default("lean", "standard_d6")
	c.Defenses = []string{NoDefense}
	resolved, _, _, err := b.Resolve(c)
	if err != nil {
		t.Fatal(err)
	}
	_, report, err := b.Price(resolved)
	if err != nil {
		t.Fatal(err)
	}
	if _, board, err := b.Generate(resolved, report); err != nil || len(board.Defensive) != 0 {
		t.Fatalf("no-defense board %v: %v", board.Defensive, err)
	}
	for _, level := range []float64{5, 20.5} {
		c.Level = level
		if _, _, _, err := b.Resolve(c); err == nil {
			t.Fatalf("level %.1f was accepted", level)
		}
	}
	c.Level = 9
	resolved, _, _, err = b.Resolve(c)
	if err != nil || resolved.Level != 9 {
		t.Fatalf("level 9: %+v %v", resolved, err)
	}
	_, high, err := b.Price(resolved)
	if err != nil || high.WholeAverage < 8.5 {
		t.Fatalf("level 9 prices at %.2f: %v", high.WholeAverage, err)
	}
	c.Level = b.Menus.Pricing.Target
	if resolved, _, _, _ = b.Resolve(c); resolved.Level != 0 {
		t.Fatalf("the standard level is stored as %v, want 0", resolved.Level)
	}
}
