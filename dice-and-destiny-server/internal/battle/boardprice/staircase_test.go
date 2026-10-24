package boardprice

import (
	"math"
	"testing"
)

// defaultBoards calls f with every shipped template on every die of its
// family at default picks.
func defaultBoards(t *testing.T, b *Builder, f func(name string, die Die, board Board)) {
	t.Helper()
	for _, tpl := range b.Menus.Templates {
		for _, entry := range b.Menus.Dice {
			die, _ := b.Die(entry.ID)
			if die.Layout.Family != tpl.Family {
				continue
			}
			_, d, board, err := b.Resolve(b.Default(tpl.ID, die.ID))
			if err != nil {
				t.Fatalf("%s on %s: %v", tpl.ID, die.ID, err)
			}
			f(tpl.ID+" on "+die.ID, d, board)
		}
	}
}

// Every default board's staircase runs from From to To, and each step
// raises at least one tier, lowers none and never lowers the average. Every
// step keeps the ladder steps and never pays a harder tier less than an
// easier one on another ability.
func TestStaircaseNeverLowersDamage(t *testing.T) {
	b := testBuilder(t)
	opts := b.Menus.Options()
	defaultBoards(t, b, func(name string, die Die, board Board) {
		sc, err := PriceStaircase(die, board, opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		p, err := prepare(die, board, opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(sc.Steps) < 50 || sc.Steps[0].At != opts.From || sc.From != opts.From || sc.To != opts.To {
			t.Fatalf("%s: %d steps from %g (range %g–%g)", name, len(sc.Steps), sc.Steps[0].At, sc.From, sc.To)
		}
		for k, step := range sc.Steps {
			if !p.keepsOrder(step.Damage) {
				t.Fatalf("%s step %d: a harder tier pays less than an easier one: %v", name, k, step.Damage)
			}
			for _, ladder := range p.ladders {
				for j := 1; j < len(ladder); j++ {
					if step.Damage[ladder[j]] < step.Damage[ladder[j-1]]+opts.LadderStep {
						t.Fatalf("%s step %d: ladder %v breaks the step: %v", name, k, ladder, step.Damage)
					}
				}
			}
			if k == 0 {
				continue
			}
			prev := sc.Steps[k-1]
			if step.At <= prev.At || step.At > opts.To || len(step.Raised) == 0 || step.Average < prev.Average-1e-9 {
				t.Fatalf("%s step %d at %.4f (previous %.4f): raised %v, average %.4f after %.4f", name, k, step.At, prev.At, step.Raised, step.Average, prev.Average)
			}
			raised := map[int]bool{}
			for _, i := range step.Raised {
				raised[i] = true
			}
			for i := range step.Damage {
				if step.Damage[i] < prev.Damage[i] || (step.Damage[i] > prev.Damage[i]) != raised[i] {
					t.Fatalf("%s step %d: tier %d goes %d → %d (raised %v)", name, k, i, prev.Damage[i], step.Damage[i], step.Raised)
				}
			}
		}
		if last := sc.Steps[len(sc.Steps)-1]; math.Abs(last.Average-opts.To) > wholeWithin*opts.To || math.Abs(sc.Steps[0].Average-opts.From) > 2*wholeWithin*opts.From {
			t.Fatalf("%s: staircase averages %.3f to %.3f over %g–%g", name, sc.Steps[0].Average, last.Average, opts.From, opts.To)
		}
	})
}

// Price at a target is the staircase step in force there.
func TestPriceIsTheStepInForce(t *testing.T) {
	b := testBuilder(t)
	_, die, board, err := b.Resolve(b.Default("lean", "hearth_d6"))
	if err != nil {
		t.Fatal(err)
	}
	opts := b.Menus.Options()
	sc, err := PriceStaircase(die, board, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []int{0, 1, 17, len(sc.Steps) / 2, len(sc.Steps) - 1} {
		step := sc.Steps[k]
		next := opts.To
		if k+1 < len(sc.Steps) {
			next = sc.Steps[k+1].At
		}
		for _, target := range []float64{step.At, (step.At + next) / 2} {
			opts.Target = target
			report, err := Price(die, board, opts)
			if err != nil {
				t.Fatal(err)
			}
			if report.Step != k || math.Abs(report.WholeAverage-step.Average) > 1e-12 {
				t.Fatalf("at %.4f the report is step %d (average %.3f); want step %d (%.3f)", target, report.Step, report.WholeAverage, k, step.Average)
			}
			for i, tier := range report.Tiers {
				if tier.WholeDamage != step.Damage[i] {
					t.Fatalf("at %.4f %s deals %d; the step deals %d", target, tier.Tier, tier.WholeDamage, step.Damage[i])
				}
			}
		}
	}
	for _, target := range []float64{opts.From - 0.1, opts.To + 0.1} {
		opts.Target = target
		if _, err := Price(die, board, opts); err == nil {
			t.Fatalf("priced at %.1f, outside the staircase", target)
		}
	}
}

// Damage rounds to nearest: on the Lean board on the standard die at 6.2,
// 2 Sword + 2 Gold Coin (5.94) and the small straight (5.86) deal 6, where
// rounding within 2% of the target for the least change in play dealt 5.
func TestPriceRoundsToNearest(t *testing.T) {
	b := testBuilder(t)
	_, report, err := b.Price(b.Default("lean", "standard_d6"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tier := range report.Tiers {
		if (tier.Tier == "main2+apex2" || tier.Tier == "SS") && (tier.WholeDamage != 6 || math.Abs(tier.Damage-6) > 0.2) {
			t.Fatalf("%s is priced %.2f and deals %d; want 6", tier.Tier, tier.Damage, tier.WholeDamage)
		}
	}
}
