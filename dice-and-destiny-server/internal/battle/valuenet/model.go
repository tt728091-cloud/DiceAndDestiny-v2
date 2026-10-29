package valuenet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// ModelFormat names exported value networks (ml/dice_destiny_ml/value_train.py).
const ModelFormat = "dice-and-destiny-value-v1"

type tensor struct {
	Shape  []int     `json:"shape"`
	Values []float32 `json:"values"`
}

type modelFile struct {
	Format       string `json:"format"`
	Schema       string `json:"schema"`
	SchemaHash   string `json:"schema_hash"`
	FeatureCount int    `json:"feature_count"`
	Architecture struct {
		Hidden     []int  `json:"hidden"`
		Activation string `json:"activation"`
	} `json:"architecture"`
	Tensors map[string]tensor `json:"tensors"`
	Check   []struct {
		Input  []float32 `json:"input"`
		Win    float64   `json:"win"`
		Margin float64   `json:"margin"`
	} `json:"check"`
	TrainingContentHash string `json:"training_content_hash"`
}

type layer struct {
	rows, columns int
	weights, bias []float32
}

func (l layer) apply(input, output []float32) {
	for row := 0; row < l.rows; row++ {
		value := l.bias[row]
		weights := l.weights[row*l.columns : (row+1)*l.columns]
		for column, w := range weights {
			value += w * input[column]
		}
		output[row] = value
	}
}

// Model is a loaded value network: hidden layers and two heads, the win
// probability and the final health margin. It is read-only and safe to
// share; each search worker evaluates through its own Evaluator.
type Model struct {
	// SHA256 is the model file's hash; ContentHash the content it was
	// trained on (for reference: models load without a content pin).
	SHA256      string
	ContentHash string
	activation  string
	hidden      []layer
	win, margin layer
}

// LoadFile reads a model file.
func LoadFile(path string) (*Model, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Load(raw)
}

// Load decodes a model, refusing one built for other inputs, and checks it
// reproduces the predictions recorded at export.
func Load(raw []byte) (*Model, error) {
	var file modelFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode value network: %w", err)
	}
	if file.Format != ModelFormat || file.Schema != SchemaName {
		return nil, fmt.Errorf("value network format %q schema %q, want %q %q", file.Format, file.Schema, ModelFormat, SchemaName)
	}
	if file.SchemaHash != SchemaHash() || file.FeatureCount != Width() {
		return nil, fmt.Errorf("value network was trained on other inputs (schema %s, %d inputs; this encoder %s, %d)", file.SchemaHash, file.FeatureCount, SchemaHash(), Width())
	}
	if file.Architecture.Activation != "gelu" && file.Architecture.Activation != "relu" {
		return nil, fmt.Errorf("value network activation %q", file.Architecture.Activation)
	}
	sum := sha256.Sum256(raw)
	m := &Model{SHA256: hex.EncodeToString(sum[:]), ContentHash: file.TrainingContentHash, activation: file.Architecture.Activation}
	width := Width()
	for i, size := range file.Architecture.Hidden {
		l, err := layerOf(file, fmt.Sprintf("hidden.%d", i), size, width)
		if err != nil {
			return nil, err
		}
		m.hidden = append(m.hidden, l)
		width = size
	}
	var err error
	if m.win, err = layerOf(file, "win", 1, width); err != nil {
		return nil, err
	}
	if m.margin, err = layerOf(file, "margin", 1, width); err != nil {
		return nil, err
	}
	if len(file.Check) == 0 {
		return nil, fmt.Errorf("value network has no check rows")
	}
	scratch := m.scratch()
	for i, check := range file.Check {
		if len(check.Input) != Width() {
			return nil, fmt.Errorf("value network check row %d has %d inputs", i, len(check.Input))
		}
		win, margin := m.predict(check.Input, scratch)
		if math.Abs(win-check.Win) > 1e-4 || math.Abs(margin-check.Margin) > 1e-4 {
			return nil, fmt.Errorf("value network check row %d: got %.6f/%.6f, exported %.6f/%.6f", i, win, margin, check.Win, check.Margin)
		}
	}
	return m, nil
}

func layerOf(file modelFile, name string, rows, columns int) (layer, error) {
	w, ok := file.Tensors[name+".weight"]
	if !ok || len(w.Shape) != 2 || w.Shape[0] != rows || w.Shape[1] != columns || len(w.Values) != rows*columns {
		return layer{}, fmt.Errorf("value network tensor %s.weight is not %dx%d", name, rows, columns)
	}
	b, ok := file.Tensors[name+".bias"]
	if !ok || len(b.Shape) != 1 || b.Shape[0] != rows || len(b.Values) != rows {
		return layer{}, fmt.Errorf("value network tensor %s.bias is not %d", name, rows)
	}
	return layer{rows: rows, columns: columns, weights: w.Values, bias: b.Values}, nil
}

// scratch holds one buffer per hidden layer.
func (m *Model) scratch() [][]float32 {
	out := make([][]float32, len(m.hidden)+1)
	for i, l := range m.hidden {
		out[i] = make([]float32, l.rows)
	}
	out[len(m.hidden)] = make([]float32, 1)
	return out
}

func (m *Model) predict(input []float32, scratch [][]float32) (win, margin float64) {
	x := input
	for i, l := range m.hidden {
		l.apply(x, scratch[i])
		for j, v := range scratch[i] {
			switch m.activation {
			case "relu":
				if v < 0 {
					scratch[i][j] = 0
				}
			default:
				f := float64(v)
				scratch[i][j] = float32(0.5 * f * (1 + math.Erf(f/math.Sqrt2)))
			}
		}
		x = scratch[i]
	}
	out := scratch[len(m.hidden)]
	m.win.apply(x, out)
	win = 1 / (1 + math.Exp(-float64(out[0])))
	m.margin.apply(x, out)
	margin = math.Tanh(float64(out[0]))
	return win, margin
}

// Evaluator judges positions with the model for one search worker.
type Evaluator struct {
	model   *Model
	enc     *Encoder
	input   []float32
	scratch [][]float32
	// Failures counts positions it could not encode, which fell back to the
	// health race.
	Failures int
}

// NewEvaluator builds an evaluator; each goroutine needs its own.
func (m *Model) NewEvaluator() *Evaluator {
	return &Evaluator{model: m, enc: NewEncoder(), input: make([]float32, Width()), scratch: m.scratch()}
}

// Evaluators returns a factory for SearchPolicy.Evaluator.
func (m *Model) Evaluators() func() lookahead.Evaluator {
	return func() lookahead.Evaluator { return m.NewEvaluator() }
}

// Evaluate scores the position for the hero from the hero's own view: 2p-1
// for win probability p (the scale of a finished battle, like the health-race
// evaluator it replaces) and the expected final health margin. A position it
// cannot encode is scored by the health race instead, never an error.
func (e *Evaluator) Evaluate(b *state.Battle, hero, acting string) (float64, float64) {
	pos, err := NewPosition(b, hero, acting)
	if err == nil {
		err = e.enc.Encode(pos, e.input)
	}
	if err != nil {
		e.Failures++
		m := Margin(b, hero)
		return math.Tanh(2 * m), m
	}
	win, margin := e.model.predict(e.input, e.scratch)
	return 2*win - 1, margin
}

// Predict scores encoded inputs: the win probability and expected final
// health margin.
func (e *Evaluator) Predict(input []float32) (win, margin float64) {
	return e.model.predict(input, e.scratch)
}
