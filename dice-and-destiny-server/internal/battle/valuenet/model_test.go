package valuenet_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/valuenet"
)

// tinyModel builds a model file with random weights and check rows computed
// independently in float64.
func tinyModel(t *testing.T, activation string) map[string]any {
	t.Helper()
	rng := rand.New(rand.NewSource(3))
	width, hidden := valuenet.Width(), []int{8, 8}
	random := func(n int) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = rng.NormFloat64() * 0.3
		}
		return out
	}
	tensors := map[string]any{}
	type dense struct {
		w, b       []float64
		rows, cols int
	}
	var layers []dense
	cols := width
	for i, rows := range hidden {
		l := dense{random(rows * cols), random(rows), rows, cols}
		layers = append(layers, l)
		tensors["hidden."+string(rune('0'+i))+".weight"] = map[string]any{"shape": []int{rows, cols}, "values": l.w}
		tensors["hidden."+string(rune('0'+i))+".bias"] = map[string]any{"shape": []int{rows}, "values": l.b}
		cols = rows
	}
	heads := map[string]dense{}
	for _, head := range []string{"win", "margin"} {
		l := dense{random(cols), random(1), 1, cols}
		heads[head] = l
		tensors[head+".weight"] = map[string]any{"shape": []int{1, cols}, "values": l.w}
		tensors[head+".bias"] = map[string]any{"shape": []int{1}, "values": l.b}
	}
	apply := func(l dense, x []float64) []float64 {
		out := make([]float64, l.rows)
		for r := range out {
			out[r] = float64(float32(l.b[r]))
			for c := 0; c < l.cols; c++ {
				out[r] += float64(float32(l.w[r*l.cols+c])) * x[c]
			}
		}
		return out
	}
	var checks []map[string]any
	for k := 0; k < 3; k++ {
		input := random(width)
		x := input
		for _, l := range layers {
			x = apply(l, x)
			for i, v := range x {
				if activation == "relu" {
					x[i] = math.Max(v, 0)
				} else {
					x[i] = 0.5 * v * (1 + math.Erf(v/math.Sqrt2))
				}
			}
		}
		win := 1 / (1 + math.Exp(-apply(heads["win"], x)[0]))
		checks = append(checks, map[string]any{"input": input, "win": win, "margin": math.Tanh(apply(heads["margin"], x)[0])})
	}
	return map[string]any{
		"format": valuenet.ModelFormat, "schema": valuenet.SchemaName, "schema_hash": valuenet.SchemaHash(),
		"feature_count": width, "architecture": map[string]any{"hidden": hidden, "activation": activation},
		"tensors": tensors, "check": checks, "training_content_hash": "test",
	}
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// A model loads when it reproduces its own check rows, and is refused when
// it was built for other inputs or computes something else.
func TestLoadChecksModels(t *testing.T) {
	for _, activation := range []string{"gelu", "relu"} {
		file := tinyModel(t, activation)
		model, err := valuenet.Load(marshal(t, file))
		if err != nil {
			t.Fatalf("%s: %v", activation, err)
		}
		if len(model.SHA256) != 64 || model.ContentHash != "test" {
			t.Fatalf("metadata %q %q", model.SHA256, model.ContentHash)
		}
	}
	cases := map[string]func(map[string]any){
		"schema":  func(f map[string]any) { f["schema_hash"] = "other" },
		"width":   func(f map[string]any) { f["feature_count"] = valuenet.Width() + 1 },
		"tensors": func(f map[string]any) { delete(f["tensors"].(map[string]any), "win.bias") },
		"check": func(f map[string]any) {
			f["check"].([]map[string]any)[1]["win"] = f["check"].([]map[string]any)[1]["win"].(float64) + 0.01
		},
	}
	for name, change := range cases {
		file := tinyModel(t, "gelu")
		change(file)
		if _, err := valuenet.Load(marshal(t, file)); err == nil {
			t.Fatalf("loaded a model with a wrong %s", name)
		}
	}
}

// The evaluator scores a real position on the -1..1 scale from the hero's
// view, and both seats' scores come from their own views.
func TestEvaluatorScoresPositions(t *testing.T) {
	model, err := valuenet.Load(marshal(t, tinyModel(t, "gelu")))
	if err != nil {
		t.Fatal(err)
	}
	ev := model.NewEvaluator()
	for _, d := range planningDecisions(t, "starter", 9) {
		for _, hero := range []string{"seat-a", "seat-b"} {
			value, margin := ev.Evaluate(d.Battle, hero, d.Seat)
			if value <= -1 || value >= 1 || margin <= -1 || margin >= 1 || math.IsNaN(value) {
				t.Fatalf("value %v margin %v", value, margin)
			}
			pos, err := valuenet.NewPosition(d.Battle, hero, d.Seat)
			if err != nil {
				t.Fatal(err)
			}
			input := make([]float32, valuenet.Width())
			if err := valuenet.NewEncoder().Encode(pos, input); err != nil {
				t.Fatal(err)
			}
			if win, _ := ev.Predict(input); math.Abs(2*win-1-value) > 1e-12 {
				t.Fatalf("Evaluate %v, Predict %v", value, 2*win-1)
			}
		}
	}
	if ev.Failures != 0 {
		t.Fatalf("%d positions fell back to the health race", ev.Failures)
	}
	if !strings.HasPrefix(valuenet.ModelFormat, "dice-and-destiny-value") {
		t.Fatal(valuenet.ModelFormat)
	}
}
