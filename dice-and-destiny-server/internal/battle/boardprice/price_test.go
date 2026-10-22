package boardprice

import (
	"math"
	"strings"
	"testing"
	"time"
)

// parity is board_balance.py's output (docs/game-design/dice-throne-reference/
// analysis), the reference implementation of the pricing.
type parity struct {
	die, costs                   string
	target                       float64
	chase, relative, used, value []float64
	whole                        []int
	wholeUsed                    []float64
	sum, multiplier, noHit       float64
	wholeAverage, wholeNoHit     float64
	roundings                    int
}

var parityBoards = map[string]parity{
	// python3 -I board_balance.py --board starter --target 6.2 --no-marginal
	"starter": {die: "standard_d6", costs: "main3/main4/main5, main2+side3, main2+apex2, SS, LS, apex4", target: 6.2,
		chase:     []float64{0.983948, 0.879272, 0.512909, 0.557051, 0.575789, 0.615442, 0.261095, 0.104426},
		relative:  []float64{1.003242, 1.026066, 1.142857, 1.124141, 1.116728, 1.101952, 1.308098, 1.571224},
		used:      []float64{0.046298, 0.14061, 0.214612, 0.124657, 0.132875, 0.17335, 0.147537, 0.013892},
		value:     []float64{5.502782, 5.627973, 6.268572, 6.165917, 6.125254, 6.044205, 7.174919, 8.618163},
		whole:     []int{5, 5, 6, 6, 6, 6, 8, 8},
		wholeUsed: []float64{0.115773, 0.055165, 0.109137, 0.139868, 0.160905, 0.222197, 0.17207, 0.012869},
		sum:       1.130355, multiplier: 5.485001, noHit: 0.006169, wholeAverage: 6.126839, wholeNoHit: 0.012017, roundings: 60},
	// --board lean --target 6.2 --no-marginal
	"lean": {die: "standard_d6", costs: "main3/main4/main5, main2+side3, main2+apex2, SS/LS, apex4, apex5", target: 6.2,
		chase:     []float64{0.983948, 0.879272, 0.512909, 0.557051, 0.575789, 0.615442, 0.261095, 0.104426, 0.013272},
		relative:  []float64{1.003242, 1.026066, 1.142857, 1.124141, 1.116728, 1.101952, 1.308098, 1.571224, 2.373626},
		used:      []float64{0.045994, 0.138761, 0.212755, 0.123149, 0.13971, 0.1718, 0.146549, 0.010942, 0.003789},
		value:     []float64{5.488358, 5.613221, 6.252141, 6.149755, 6.109198, 6.028362, 7.156112, 8.595573, 12.985216},
		whole:     []int{5, 5, 6, 6, 6, 6, 8, 8, 12},
		wholeUsed: []float64{0.113693, 0.054184, 0.107658, 0.138851, 0.165089, 0.220694, 0.171349, 0.011528, 0.003804},
		sum:       1.133326, multiplier: 5.470623, noHit: 0.006551, wholeAverage: 6.141803, wholeNoHit: 0.01315, roundings: 120},
	// --die 2-1-2-1 --costs "A3/A4/A5, C3, A3+B1, B3, SS, D4, LS, D5" --target 8.58 (Monk; our Loom die)
	"monk": {die: "loom_d6", costs: "main3/main4/main5, side3, main3+odd1, odd3, SS, apex4, LS, apex5", target: 8.58,
		chase:     []float64{0.841785, 0.535855, 0.172564, 0.841785, 0.601283, 0.35485, 0.615442, 0.104426, 0.261095, 0.013272},
		relative:  []float64{1.035046, 1.132897, 1.421053, 1.035046, 1.107093, 1.230243, 1.101952, 1.571224, 1.308098, 2.373626},
		used:      []float64{0.079631, 0.231113, 0.132464, 0.128016, 0.079449, 0.049375, 0.144407, 0.007998, 0.113235, 0.00294},
		value:     []float64{7.786757, 8.522899, 10.690722, 7.786757, 8.328775, 9.255245, 8.290093, 11.820475, 9.840953, 17.857031},
		whole:     []int{7, 9, 10, 7, 8, 10, 8, 12, 10, 17},
		wholeUsed: []float64{0.085925, 0.240722, 0.133607, 0.094884, 0.064447, 0.08226, 0.141133, 0.008276, 0.10722, 0.003033},
		sum:       1.140487, multiplier: 7.523101, noHit: 0.031373, wholeAverage: 8.458544, wholeNoHit: 0.038493, roundings: 288},
}

// costBoard is a board of riderless slots from comma-separated costs.
func costBoard(t testing.TB, costs string) Board {
	t.Helper()
	var board Board
	for i, text := range strings.Split(costs, ",") {
		cost, err := ParseCost(text)
		if err != nil {
			t.Fatal(err)
		}
		board.Slots = append(board.Slots, Slot{ID: string(rune('a' + i)), Cost: cost, RiderID: RiderNone})
	}
	return board
}

func testDie(t testing.TB, id string) Die {
	t.Helper()
	die, err := NewDie(testCatalog(t).Dice[id])
	if err != nil {
		t.Fatal(err)
	}
	return die
}

func near(a, b, tolerance float64) bool { return math.Abs(a-b) <= tolerance }

// The Go port reproduces board_balance.py: use shares and the multiplier to
// ±0.01, whole damage exactly. The script has no ladder step, so parity runs
// without one.
func TestPriceMatchesPythonReference(t *testing.T) {
	m := testMenus(t)
	for name, want := range parityBoards {
		opts := m.Options()
		opts.Target = want.target
		opts.LadderStep = 0
		report, err := Price(testDie(t, want.die), costBoard(t, want.costs), opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(report.Tiers) != len(want.whole) {
			t.Fatalf("%s: %d tiers", name, len(report.Tiers))
		}
		worst := 0.0
		for i, tier := range report.Tiers {
			if !near(tier.Chase, want.chase[i], 1e-5) || !near(tier.Relative, want.relative[i], 1e-5) || !near(tier.Value, want.value[i], 0.01) {
				t.Errorf("%s %s: chase %.6f relative %.6f value %.4f; want %.6f %.6f %.4f", name, tier.Tier, tier.Chase, tier.Relative, tier.Value, want.chase[i], want.relative[i], want.value[i])
			}
			if !near(tier.UseExact, want.used[i], 0.01) || !near(tier.UseWhole, want.wholeUsed[i], 0.01) {
				t.Errorf("%s %s: use %.4f / %.4f; want %.4f / %.4f", name, tier.Tier, tier.UseExact, tier.UseWhole, want.used[i], want.wholeUsed[i])
			}
			if tier.WholeDamage != want.whole[i] {
				t.Errorf("%s %s: whole damage %d; want %d", name, tier.Tier, tier.WholeDamage, want.whole[i])
			}
			worst = math.Max(worst, math.Max(math.Abs(tier.UseExact-want.used[i]), math.Abs(tier.UseWhole-want.wholeUsed[i])))
		}
		if !near(report.Multiplier, want.multiplier, 0.01) || !near(report.Sum, want.sum, 0.001) || !near(report.ExactNoHit, want.noHit, 0.001) {
			t.Errorf("%s: sum %.4f multiplier %.4f no hit %.4f; want %.4f %.4f %.4f", name, report.Sum, report.Multiplier, report.ExactNoHit, want.sum, want.multiplier, want.noHit)
		}
		if !near(report.WholeAverage, want.wholeAverage, 0.01) || !near(report.WholeNoHit, want.wholeNoHit, 0.001) || report.Roundings != want.roundings {
			t.Errorf("%s: whole average %.4f no hit %.4f roundings %d; want %.4f %.4f %d", name, report.WholeAverage, report.WholeNoHit, report.Roundings, want.wholeAverage, want.wholeNoHit, want.roundings)
		}
		t.Logf("%s: multiplier %.4f (python %.4f), largest use difference %.5f", name, report.Multiplier, want.multiplier, worst)
	}
}

// The Monk's printed board plays to 8.58 per turn with 5.3% no hit.
func TestMonkPrintedBoard(t *testing.T) {
	m := testMenus(t)
	opts := m.Options()
	opts.Target = 8.58
	ev, noHit, _, err := Evaluate(testDie(t, "loom_d6"), costBoard(t, parityBoards["monk"].costs), []float64{6, 7, 8, 7.66, 8.28, 10.5, 8, 10, 12.66, 21.32}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !near(ev, 8.581298, 0.005) || !near(noHit, 0.053013, 0.001) {
		t.Fatalf("printed Monk board plays to %.4f with %.4f no hit; want 8.5813 and 0.0530", ev, noHit)
	}
}

// One board prices well under a second, so the Board Builder previews live.
func TestPriceIsFast(t *testing.T) {
	m := testMenus(t)
	board := costBoard(t, "main3/main4/main5, side3+apex1, main2+apex2, main2+side3, main3+apex2, SS/LS, apex4, apex5")
	die := testDie(t, "hearth_d6")
	handsFor(5) // the shared hand table is built once per process
	start := time.Now()
	if _, err := Price(die, board, m.Options()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("pricing a Wide board took %s", elapsed)
	} else {
		t.Logf("Wide board priced in %s", elapsed)
	}
	start = time.Now()
	if _, err := Price(die, board, m.Options()); err != nil || time.Since(start) > 5*time.Millisecond {
		t.Fatalf("cached price took %s (%v)", time.Since(start), err)
	}
}
