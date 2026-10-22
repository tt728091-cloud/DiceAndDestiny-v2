package valuenet_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/valuenet"
)

// allDecisions plays a mirror with the greedy player and returns every
// decision with two or more useful options, from both seats.
func allDecisions(t *testing.T, hero string, seed uint64) []*lookahead.Decision {
	t.Helper()
	runner, err := lookahead.NewRunner(mirror(t, hero))
	if err != nil {
		t.Fatal(err)
	}
	keep := func(d *lookahead.Decision) bool {
		return len(lookahead.Useful(d.Battle, d.Seat, d.Actions)) > 1
	}
	a := &recorder{inner: lookahead.NewGreedyPolicy(), keep: keep}
	b := &recorder{inner: lookahead.NewGreedyPolicy(), keep: keep}
	if _, err := runner.Play(seed, a, b); err != nil {
		t.Fatal(err)
	}
	return append(a.got, b.got...)
}

func optionsOf(t *testing.T, d *lookahead.Decision) ([]int, int) {
	t.Helper()
	search := lookahead.NewSearchPolicy(func() lookahead.Policy { return lookahead.NewGreedyPolicy() }, 0)
	search.TopRerolls, search.TopAbilities = 8, 4
	copy := *d
	copy.Rand = rand.New(rand.NewSource(1))
	options, anchor, err := search.Options(&copy)
	if err != nil {
		t.Fatal(err)
	}
	return options, anchor
}

// encodeV2 returns the seat's value-v2 inputs followed by every option's.
func encodeV2(t *testing.T, enc *valuenet.Encoder, d *lookahead.Decision, b *state.Battle, options []int, anchor int) []float32 {
	t.Helper()
	pos, err := valuenet.NewPosition(b, d.Seat, d.Seat)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, valuenet.WidthV2()+valuenet.OptionWidth()*len(options))
	if err := enc.EncodeV2(pos, out[:valuenet.WidthV2()]); err != nil {
		t.Fatal(err)
	}
	if err := enc.EncodeOptions(pos, d.Actions, options, anchor, out[valuenet.WidthV2():]); err != nil {
		t.Fatal(err)
	}
	return out
}

func v2Names(options int) []string {
	names := valuenet.FeaturesV2()
	for k := 0; k < options; k++ {
		names = append(names, valuenet.OptionFeatures()...)
	}
	return names
}

// The value-v2 and option schemas publish one unique name per input, and
// value-v2 starts with value-v1.
func TestV2SchemaWidth(t *testing.T) {
	for _, names := range [][]string{valuenet.FeaturesV2(), valuenet.OptionFeatures()} {
		seen := map[string]bool{}
		for _, n := range names {
			if seen[n] {
				t.Fatalf("duplicate input %q", n)
			}
			seen[n] = true
		}
	}
	if valuenet.WidthV2() <= valuenet.Width() || strings.Join(valuenet.FeaturesV2()[:valuenet.Width()], ",") != strings.Join(valuenet.Features(), ",") {
		t.Fatal("value-v2 does not extend value-v1")
	}
	if len(valuenet.SchemaHashV2()) != 64 || len(valuenet.OptionSchemaHash()) != 64 || valuenet.OptionWidth() < 20 {
		t.Fatalf("schemas %q %q, option width %d", valuenet.SchemaHashV2(), valuenet.OptionSchemaHash(), valuenet.OptionWidth())
	}
}

// At every kind of decision the state and option inputs only know what the
// deciding seat sees: forks (fresh randomness, a re-dealt hidden hand) and
// changes to the opponent's hidden hand and private dice leave them
// unchanged.
func TestV2IsViewerSafe(t *testing.T) {
	enc := valuenet.NewEncoder()
	checked, private, kinds := 0, 0, map[string]bool{}
	for _, hero := range []string{"starter", "venom", "curse"} {
		for _, d := range allDecisions(t, hero, 5) {
			options, anchor := optionsOf(t, d)
			names := v2Names(len(options))
			want := encodeV2(t, enc, d, d.Battle, options, anchor)
			for seed := uint64(1); seed <= 2; seed++ {
				fork := lookahead.Fork(d.Battle, d.Seat, seed*7919)
				if got := encodeV2(t, valuenet.NewEncoder(), d, &fork, options, anchor); diff(names, want, got) != "" {
					t.Fatalf("%s %s round %d %s: fork %d changed %s", hero, d.Seat, d.Battle.Segment.Round, d.Battle.Settled.Window.Stage, seed, diff(names, want, got))
				}
			}
			other := "seat-a"
			if d.Seat == "seat-a" {
				other = "seat-b"
			}
			tampered := d.Battle.Clone()
			foe := tampered.Actors[other]
			n := min(len(foe.Cards.Hand), len(foe.Cards.Deck))
			for i := 0; i < n; i++ {
				foe.Cards.Hand[i], foe.Cards.Deck[i] = foe.Cards.Deck[i], foe.Cards.Hand[i]
			}
			tampered.Actors[other] = foe
			if rt := tampered.Settled.Actors[other]; len(rt.FinalDice) > 0 && state.SettledPlanningPrivate(tampered) {
				private++
				for i := range rt.FinalDice {
					rt.FinalDice[i].Face = 7 - rt.FinalDice[i].Face
				}
				tampered.Settled.Actors[other] = rt
			}
			if got := encodeV2(t, enc, d, &tampered, options, anchor); diff(names, want, got) != "" {
				t.Fatalf("%s %s: hidden change moved %s", hero, d.Seat, diff(names, want, got))
			}
			checked++
			kinds[string(d.Battle.Segment.Current)+"/"+d.Battle.Settled.Window.Stage] = true
		}
	}
	if checked < 50 || private == 0 || len(kinds) < 3 {
		t.Fatalf("checked %d decisions (%d with the opponent's private dice) at %v", checked, private, kinds)
	}
}

// The same decision encodes to the same bytes from fresh encoders, and the
// option inputs mark the greedy pick and score rerolls with the solver.
func TestV2EncodingIsDeterministic(t *testing.T) {
	decisions := allDecisions(t, "starter", 7)
	rerolls := 0
	for _, d := range decisions {
		options, anchor := optionsOf(t, d)
		names := v2Names(len(options))
		a := encodeV2(t, valuenet.NewEncoder(), d, d.Battle, options, anchor)
		b := encodeV2(t, valuenet.NewEncoder(), d, d.Battle, options, anchor)
		if diff(names, a, b) != "" {
			t.Fatalf("fresh encoders differ at %s", diff(names, a, b))
		}
		pick := indexOfName("option.greedy_pick")
		solver := indexOfName("option.solver_scored")
		marked := 0
		for k, i := range options {
			row := a[valuenet.WidthV2()+k*valuenet.OptionWidth():]
			if row[pick] == 1 {
				marked++
				if i != anchor {
					t.Fatalf("option %d is marked as greedy's pick, greedy chose %d", i, anchor)
				}
			}
			if row[indexOfName("option.kind.reroll")] == 1 && row[solver] == 1 {
				rerolls++
			}
		}
		if marked != 1 {
			t.Fatalf("%d options marked as greedy's pick", marked)
		}
	}
	if rerolls == 0 {
		t.Fatal("no reroll option was scored by the solver")
	}
}

func indexOfName(name string) int {
	for i, n := range valuenet.OptionFeatures() {
		if n == name {
			return i
		}
	}
	return -1
}

// policyTensors builds a small random policy network and its export.
func policyExport(t *testing.T, rng *rand.Rand, trunk, scorer int) map[string]any {
	t.Helper()
	tensors := map[string]any{}
	mat := func(name string, rows, cols int) [][]float64 {
		w := make([][]float64, rows)
		var flat []float64
		for r := range w {
			w[r] = make([]float64, cols)
			for c := range w[r] {
				w[r][c] = float64(float32(rng.NormFloat64() * 0.1))
				flat = append(flat, w[r][c])
			}
		}
		tensors[name+".weight"] = map[string]any{"shape": []int{rows, cols}, "values": flat}
		b := make([]float64, rows)
		for r := range b {
			b[r] = float64(float32(rng.NormFloat64() * 0.1))
		}
		tensors[name+".bias"] = map[string]any{"shape": []int{rows}, "values": b}
		return append(w, b)
	}
	ws, wo := valuenet.WidthV2(), valuenet.OptionWidth()
	layers := map[string][][]float64{
		"trunk.0":  mat("trunk.0", trunk, ws),
		"trunk.1":  mat("trunk.1", trunk, trunk),
		"scorer.0": mat("scorer.0", scorer, trunk+wo),
		"score":    mat("score", 1, scorer),
		"value":    mat("value", 1, trunk),
		"result":   mat("result", 1, trunk),
	}
	// A plain reference implementation computes the check rows.
	apply := func(name string, x []float64, act bool) []float64 {
		l := layers[name]
		rows := len(l) - 1
		out := make([]float64, rows)
		for r := 0; r < rows; r++ {
			v := l[rows][r]
			for c, w := range l[r] {
				v += w * x[c]
			}
			if act {
				v = 0.5 * v * (1 + math.Erf(v/math.Sqrt2))
			}
			out[r] = v
		}
		return out
	}
	var checks []map[string]any
	for i := 0; i < 2; i++ {
		s := make([]float64, ws)
		for j := range s {
			s[j] = float64(float32(rng.Float64()))
		}
		h := apply("trunk.1", apply("trunk.0", s, true), true)
		var options [][]float64
		var scores []float64
		for k := 0; k < 3; k++ {
			o := make([]float64, wo)
			for j := range o {
				o[j] = float64(float32(rng.Float64()))
			}
			options = append(options, o)
			scores = append(scores, apply("score", apply("scorer.0", append(append([]float64(nil), h...), o...), true), false)[0])
		}
		v := math.Tanh(apply("value", h, false)[0]) * 1.2
		r := 1 / (1 + math.Exp(-apply("result", h, false)[0]))
		checks = append(checks, map[string]any{"state": s, "options": options, "scores": scores, "value": v, "result": r})
	}
	return map[string]any{
		"format": valuenet.PolicyFormat, "state_schema": valuenet.SchemaNameV2, "state_schema_hash": valuenet.SchemaHashV2(), "state_width": ws,
		"option_schema": valuenet.OptionSchemaName, "option_schema_hash": valuenet.OptionSchemaHash(), "option_width": wo,
		"architecture": map[string]any{"trunk": []int{trunk, trunk}, "scorer": []int{scorer}, "activation": "gelu", "value_scale": 1.2},
		"tensors":      tensors, "check": checks,
	}
}

// A policy network loads only for its own inputs and only when it
// reproduces its check rows; a loaded network scores real decisions.
func TestPolicyModelLoads(t *testing.T) {
	file := policyExport(t, rand.New(rand.NewSource(3)), 16, 8)
	raw, _ := json.Marshal(file)
	model, err := valuenet.LoadPolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	file["state_schema_hash"] = "other"
	if raw, _ := json.Marshal(file); func() error { _, err := valuenet.LoadPolicy(raw); return err }() == nil {
		t.Fatal("loaded a network for other inputs")
	}
	file = policyExport(t, rand.New(rand.NewSource(3)), 16, 8)
	file["check"].([]map[string]any)[0]["scores"].([]float64)[1] += 0.01
	if raw, _ := json.Marshal(file); func() error { _, err := valuenet.LoadPolicy(raw); return err }() == nil {
		t.Fatal("loaded a network that misses its check rows")
	}
	eval := model.NewEvaluator()
	for _, d := range allDecisions(t, "starter", 3)[:10] {
		options, anchor := optionsOf(t, d)
		scores, err := eval.Scores(d, options, anchor)
		if err != nil || len(scores) != len(options) {
			t.Fatalf("scores %v, %v", scores, err)
		}
	}
}
