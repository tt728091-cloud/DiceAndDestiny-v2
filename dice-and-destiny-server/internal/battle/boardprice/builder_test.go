package boardprice

import (
	"math"
	"math/rand"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// wholeWithin is how far a board's whole-damage average may sit from its
// target: whole damage moves in steps, and the staircase never lowers a tier.
const wholeWithin = 0.03

func testBuilder(t testing.TB) *Builder {
	t.Helper()
	b, err := NewBuilder(contentRoot(), testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Every shipped template prices inside its target at default picks on every
// die of its family, with every default flex cost inside its band.
func TestShippedTemplatesPriceInsideTarget(t *testing.T) {
	b := testBuilder(t)
	target := b.Menus.Pricing.Target
	for _, tpl := range b.Menus.Templates {
		for _, entry := range b.Menus.Dice {
			die, _ := b.Die(entry.ID)
			if die.Layout.Family != tpl.Family {
				continue
			}
			choice, report, err := b.Price(b.Default(tpl.ID, die.ID))
			if err != nil {
				t.Fatalf("%s on %s: %v", tpl.ID, die.ID, err)
			}
			if len(choice.Slots) != len(tpl.Slots) || len(choice.Defenses) != tpl.Defenses {
				t.Fatalf("%s on %s: default choice %+v", tpl.ID, die.ID, choice)
			}
			if math.Abs(report.WholeAverage-target) > wholeWithin*target {
				t.Fatalf("%s on %s averages %.3f; want within %.0f%% of %.1f", tpl.ID, die.ID, report.WholeAverage, wholeWithin*100, target)
			}
			checkLadders(t, tpl.ID+" on "+die.ID, report, b.Menus.Pricing.LadderStep)
			for i := 1; i < len(report.Tiers); i++ {
				for j := 0; j < len(report.Tiers); j++ {
					a, c := report.Tiers[i], report.Tiers[j]
					if a.Chase < c.Chase && a.WholeDamage < c.WholeDamage {
						t.Fatalf("%s on %s: harder %s pays %d, less than easier %s's %d", tpl.ID, die.ID, a.Tier, a.WholeDamage, c.Tier, c.WholeDamage)
					}
				}
			}
			t.Logf("%-16s %-11s Σ %.3f ×%.2f (damage ×%.2f)  whole %.2f  no hit %.1f%%", tpl.ID, die.ID, report.Sum, report.Multiplier, report.DamageMultiplier, report.WholeAverage, report.WholeNoHit*100)
		}
	}
}

// checkLadders fails when a harder tier of one ability deals less than the
// ladder step more than the tier below it.
func checkLadders(t *testing.T, name string, report Report, step int) {
	t.Helper()
	if step < 1 {
		t.Fatalf("%s: the shipped ladder step is %d", name, step)
	}
	slots := map[string][]TierReport{}
	var order []string
	for _, tier := range report.Tiers {
		if slots[tier.Slot] == nil {
			order = append(order, tier.Slot)
		}
		slots[tier.Slot] = append(slots[tier.Slot], tier)
	}
	for _, slot := range order {
		ladder := slots[slot]
		sort.SliceStable(ladder, func(a, b int) bool { return ladder[a].Chase > ladder[b].Chase })
		for k := 1; k < len(ladder); k++ {
			if ladder[k].WholeDamage < ladder[k-1].WholeDamage+step {
				t.Fatalf("%s: %s %s deals %d, not %d more than %s's %d", name, slot, ladder[k].Tier, ladder[k].WholeDamage, step, ladder[k-1].Tier, ladder[k-1].WholeDamage)
			}
			if ladder[k].Damage < ladder[k-1].Damage+float64(step)-1e-9 {
				t.Fatalf("%s: %s %s is priced %.3f, not %d more than %s's %.3f", name, slot, ladder[k].Tier, ladder[k].Damage, step, ladder[k-1].Tier, ladder[k-1].Damage)
			}
		}
	}
}

// Every ladder keeps its step with every rider it may take, on every die of
// its family, and the board still prices inside the target.
func TestLaddersStepWithEveryRider(t *testing.T) {
	b := testBuilder(t)
	target, tolerance := b.Menus.Pricing.Target, wholeWithin
	priced := 0
	for _, tpl := range b.Menus.Templates {
		for _, entry := range b.Menus.Dice {
			die, _ := b.Die(entry.ID)
			if die.Layout.Family != tpl.Family {
				continue
			}
			for _, slot := range tpl.Slots {
				if !slot.TakesRider() || !strings.Contains(slot.Cost, "/") {
					continue
				}
				for _, rider := range b.Menus.Riders {
					if rider.ID == RiderNone || !slices.Contains(rider.Jobs, slot.Job) {
						continue
					}
					choice := b.Default(tpl.ID, die.ID)
					for i := range choice.Slots {
						if choice.Slots[i].Slot == slot.ID {
							choice.Slots[i].Rider = rider.ID
						}
					}
					name := tpl.ID + " on " + die.ID + " with " + rider.ID + " on " + slot.ID
					_, report, err := b.Price(choice)
					if err != nil {
						t.Fatalf("%s: %v", name, err)
					}
					checkLadders(t, name, report, b.Menus.Pricing.LadderStep)
					if math.Abs(report.WholeAverage-target) > tolerance*target {
						t.Errorf("%s averages %.3f", name, report.WholeAverage)
					}
					priced++
				}
			}
		}
	}
	if priced == 0 {
		t.Fatal("no ladder takes a rider")
	}
	t.Logf("%d ladder and rider boards priced", priced)
}

// A ladder too flat for the step spreads around its average, never below
// the minimum; one already stepped is unchanged.
func TestStepLadder(t *testing.T) {
	for _, c := range []struct {
		damage, want []float64
	}{
		{[]float64{4.9, 5.3, 6.1}, []float64{4.433333, 5.433333, 6.433333}},
		{[]float64{5, 5.2, 8}, []float64{4.6, 5.6, 8}},
		{[]float64{4, 5.5, 7}, []float64{4, 5.5, 7}},
		{[]float64{1, 1, 1.2}, []float64{1, 2, 3}},
		{[]float64{6.04, 7.17}, []float64{6.04, 7.17}},
		{[]float64{6.5}, []float64{6.5}},
	} {
		got := stepLadder(c.damage, 1, 1)
		for k := range c.want {
			if !near(got[k], c.want[k], 1e-5) {
				t.Fatalf("stepLadder(%v) = %v; want %v", c.damage, got, c.want)
			}
		}
	}
}

// The reference script has no ladder step: the Starter's Sword ladder pays
// 5 / 5 / 6 there. With the step, 4 Sword is worth chasing again.
func TestLadderStepSeparatesTheStarterLadder(t *testing.T) {
	m := testMenus(t)
	want := parityBoards["starter"]
	opts := m.Options()
	opts.Target = want.target
	report, err := Price(testDie(t, want.die), costBoard(t, want.costs), opts)
	if err != nil {
		t.Fatal(err)
	}
	checkLadders(t, "starter", report, opts.LadderStep)
	if math.Abs(report.WholeAverage-want.target) > wholeWithin*want.target {
		t.Fatalf("stepped Starter board averages %.3f", report.WholeAverage)
	}
	if report.Tiers[1].UseWhole <= want.wholeUsed[1] {
		t.Fatalf("4 Sword is used on %.1f%% of turns, no more than the unstepped %.1f%%", report.Tiers[1].UseWhole*100, want.wholeUsed[1]*100)
	}
	t.Logf("Sword ladder %d / %d / %d, 4 Sword used %.1f%% (unstepped %.1f%%), damage ×%.3f", report.Tiers[0].WholeDamage, report.Tiers[1].WholeDamage, report.Tiers[2].WholeDamage, report.Tiers[1].UseWhole*100, want.wholeUsed[1]*100, report.DamageMultiplier)
}

func TestResolveRejectsOffMenuChoices(t *testing.T) {
	b := testBuilder(t)
	for name, c := range map[string]Choice{
		"family":        {Die: "forge_d6", Template: "lean"},
		"core cost":     {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "ladder", Cost: "main4/main5"}}},
		"off band":      {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "flex_a", Cost: "side3"}}},
		"off menu":      {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "flex_a", Cost: "main5+side0"}}},
		"repeat":        {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "flex_a", Cost: "main2+apex2"}}},
		"rider job":     {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "ladder", Rider: "heal_2"}}},
		"no rider":      {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "flex_a", Rider: "fireworks"}}},
		"defense count": {Die: "standard_d6", Template: "lean", Defenses: []string{"holdfast", "holdfast_plus"}},
		"face defense":  {Die: "standard_d6", Template: "lean", Defenses: []string{"adventurer_guard"}},
		"unknown slot":  {Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "bonus"}}},
		"off die menu":  {Die: "venom_d6", Template: "lean"},
	} {
		if _, _, _, err := b.Resolve(c); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// Riders change the damage, not the value: a +2 Protect rider on a tier
// costs two damage, and the board is re-priced with it in play.
func TestRiderLowersDamage(t *testing.T) {
	b := testBuilder(t)
	_, plain, err := b.Price(b.Default("lean", "standard_d6"))
	if err != nil {
		t.Fatal(err)
	}
	_, guarded, err := b.Price(Choice{Die: "standard_d6", Template: "lean", Slots: []SlotPick{{Slot: "flex_a", Rider: "protect_2"}}})
	if err != nil {
		t.Fatal(err)
	}
	for i, tier := range guarded.Tiers {
		if tier.Slot != "flex_a" {
			continue
		}
		if tier.RiderValue != 2 || math.Abs(tier.Value-plain.Tiers[i].Value) > 1e-9 || math.Abs(tier.Damage-(tier.Relative*guarded.DamageMultiplier-2)) > 1e-9 {
			t.Fatalf("protect rider: value %.3f (plain %.3f), rider %.1f, damage %.3f", tier.Value, plain.Tiers[i].Value, tier.RiderValue, tier.Damage)
		}
		if tier.WholeDamage >= plain.Tiers[i].WholeDamage {
			t.Fatalf("protect rider did not lower whole damage: %d vs %d", tier.WholeDamage, plain.Tiers[i].WholeDamage)
		}
	}
	// The rider is counted in play: the board still averages its target.
	if math.Abs(guarded.WholeAverage-b.Menus.Pricing.Target) > wholeWithin*b.Menus.Pricing.Target {
		t.Fatalf("board with a rider averages %.3f", guarded.WholeAverage)
	}
}

// RandomBoard draws valid, seeded boards of every family.
func TestRandomBoards(t *testing.T) {
	b := testBuilder(t)
	families := map[string]int{}
	riders := map[string]int{}
	for seed := int64(1); seed <= 60; seed++ {
		choice, err := b.RandomBoard(rand.New(rand.NewSource(seed)), "")
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		again, _ := b.RandomBoard(rand.New(rand.NewSource(seed)), "")
		if !reflect.DeepEqual(choice, again) {
			t.Fatalf("seed %d is not deterministic", seed)
		}
		_, report, err := b.Price(choice)
		if err != nil {
			t.Fatalf("seed %d %+v: %v", seed, choice, err)
		}
		die, _ := b.Die(choice.Die)
		families[die.Layout.Family]++
		for _, s := range choice.Slots {
			riders[s.Rider]++
		}
		if report.WholeAverage <= 0 {
			t.Fatalf("seed %d priced to %.2f", seed, report.WholeAverage)
		}
		checkLadders(t, "random board", report, b.Menus.Pricing.LadderStep)
	}
	for _, f := range []string{"321", "3111", "2211"} {
		if families[f] == 0 {
			t.Fatalf("no %s boards in 60 draws: %v", f, families)
		}
	}
	if len(riders) < 5 {
		t.Fatalf("riders drawn: %v", riders)
	}
	for _, family := range []string{"321", "3111", "2211"} {
		choice, err := b.RandomBoard(rand.New(rand.NewSource(7)), family)
		die, _ := b.Die(choice.Die)
		if err != nil || die.Layout.Family != family {
			t.Fatalf("family %s: %+v %v", family, choice, err)
		}
	}
	if _, err := b.RandomBoard(rand.New(rand.NewSource(1)), "42"); err == nil || !strings.Contains(err.Error(), "no menu die") {
		t.Fatalf("unknown family: %v", err)
	}
}
