package boardprice

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

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
			if !report.Monotone || math.Abs(report.WholeAverage-target) > b.Menus.Pricing.Tolerance*target {
				t.Fatalf("%s on %s averages %.3f (monotone %v); want within %.0f%% of %.1f", tpl.ID, die.ID, report.WholeAverage, report.Monotone, b.Menus.Pricing.Tolerance*100, target)
			}
			for i := 1; i < len(report.Tiers); i++ {
				for j := 0; j < len(report.Tiers); j++ {
					a, c := report.Tiers[i], report.Tiers[j]
					if a.Chase < c.Chase && a.WholeDamage < c.WholeDamage {
						t.Fatalf("%s on %s: harder %s pays %d, less than easier %s's %d", tpl.ID, die.ID, a.Tier, a.WholeDamage, c.Tier, c.WholeDamage)
					}
				}
			}
			t.Logf("%-16s %-11s Σ %.3f ×%.2f  whole %.2f  no hit %.1f%%", tpl.ID, die.ID, report.Sum, report.Multiplier, report.WholeAverage, report.WholeNoHit*100)
		}
	}
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
		if tier.RiderValue != 2 || math.Abs(tier.Value-plain.Tiers[i].Value) > 1e-9 || math.Abs(tier.Damage-(tier.Value-2)) > 1e-9 {
			t.Fatalf("protect rider: value %.3f (plain %.3f), rider %.1f, damage %.3f", tier.Value, plain.Tiers[i].Value, tier.RiderValue, tier.Damage)
		}
		if tier.WholeDamage >= plain.Tiers[i].WholeDamage {
			t.Fatalf("protect rider did not lower whole damage: %d vs %d", tier.WholeDamage, plain.Tiers[i].WholeDamage)
		}
	}
	// The rider is counted in play: the board still averages its target.
	if math.Abs(guarded.WholeAverage-b.Menus.Pricing.Target) > b.Menus.Pricing.Tolerance*b.Menus.Pricing.Target {
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
