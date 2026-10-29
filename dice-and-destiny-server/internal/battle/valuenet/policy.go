package valuenet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/lookahead"
)

// PolicyFormat names exported policy networks (ml/dice_destiny_ml/
// policy_train.py).
const PolicyFormat = "dice-and-destiny-policy-v2"

type policyFile struct {
	Format          string `json:"format"`
	StateSchema     string `json:"state_schema"`
	StateSchemaHash string `json:"state_schema_hash"`
	StateWidth      int    `json:"state_width"`
	OptionSchema    string `json:"option_schema"`
	OptionHash      string `json:"option_schema_hash"`
	OptionWidth     int    `json:"option_width"`
	Architecture    struct {
		Trunk      []int   `json:"trunk"`
		Scorer     []int   `json:"scorer"`
		Activation string  `json:"activation"`
		ValueScale float64 `json:"value_scale"`
	} `json:"architecture"`
	Tensors map[string]tensor `json:"tensors"`
	Check   []struct {
		State   []float32   `json:"state"`
		Options [][]float32 `json:"options"`
		Scores  []float64   `json:"scores"`
		Value   float64     `json:"value"`
		Result  float64     `json:"result"`
	} `json:"check"`
}

// PolicyModel is a loaded policy network: a state trunk, an option scorer
// (the trunk's output and one option's inputs give that option's score) and
// two state heads, the decision's value and the battle's result. It is
// read-only and safe to share.
type PolicyModel struct {
	SHA256        string
	trunk, scorer []layer
	score         layer
	value, result layer
	valueScale    float64
}

// LoadPolicyFile reads a policy network file.
func LoadPolicyFile(path string) (*PolicyModel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadPolicy(raw)
}

// LoadPolicy decodes a policy network, refusing one built for other inputs,
// and checks it reproduces the scores recorded at export.
func LoadPolicy(raw []byte) (*PolicyModel, error) {
	var file policyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode policy network: %w", err)
	}
	if file.Format != PolicyFormat || file.StateSchema != SchemaNameV2 || file.OptionSchema != OptionSchemaName {
		return nil, fmt.Errorf("policy network format %q schemas %q/%q, want %q %q/%q", file.Format, file.StateSchema, file.OptionSchema, PolicyFormat, SchemaNameV2, OptionSchemaName)
	}
	if file.StateSchemaHash != SchemaHashV2() || file.StateWidth != WidthV2() || file.OptionHash != OptionSchemaHash() || file.OptionWidth != OptionWidth() {
		return nil, fmt.Errorf("policy network was trained on other inputs")
	}
	if file.Architecture.Activation != "gelu" || len(file.Architecture.Trunk) == 0 {
		return nil, fmt.Errorf("policy network architecture %+v", file.Architecture)
	}
	sum := sha256.Sum256(raw)
	m := &PolicyModel{SHA256: hex.EncodeToString(sum[:]), valueScale: file.Architecture.ValueScale}
	width := WidthV2()
	for i, size := range file.Architecture.Trunk {
		l, err := layerOf(file.asModel(), fmt.Sprintf("trunk.%d", i), size, width)
		if err != nil {
			return nil, err
		}
		m.trunk = append(m.trunk, l)
		width = size
	}
	trunk := width
	width = trunk + OptionWidth()
	for i, size := range file.Architecture.Scorer {
		l, err := layerOf(file.asModel(), fmt.Sprintf("scorer.%d", i), size, width)
		if err != nil {
			return nil, err
		}
		m.scorer = append(m.scorer, l)
		width = size
	}
	var err error
	if m.score, err = layerOf(file.asModel(), "score", 1, width); err != nil {
		return nil, err
	}
	if m.value, err = layerOf(file.asModel(), "value", 1, trunk); err != nil {
		return nil, err
	}
	if m.result, err = layerOf(file.asModel(), "result", 1, trunk); err != nil {
		return nil, err
	}
	if len(file.Check) == 0 {
		return nil, fmt.Errorf("policy network has no check rows")
	}
	e := m.NewEvaluator()
	for i, check := range file.Check {
		if len(check.State) != WidthV2() {
			return nil, fmt.Errorf("policy check row %d has %d state inputs", i, len(check.State))
		}
		copy(e.state, check.State)
		e.runTrunk()
		for k, option := range check.Options {
			if got := e.scoreOption(option); math.Abs(got-check.Scores[k]) > 1e-3 {
				return nil, fmt.Errorf("policy check row %d option %d: got %.6f, exported %.6f", i, k, got, check.Scores[k])
			}
		}
		if v, r := e.heads(); math.Abs(v-check.Value) > 1e-3 || math.Abs(r-check.Result) > 1e-3 {
			return nil, fmt.Errorf("policy check row %d heads: got %.6f/%.6f, exported %.6f/%.6f", i, v, r, check.Value, check.Result)
		}
	}
	return m, nil
}

func (f policyFile) asModel() modelFile { return modelFile{Tensors: f.Tensors} }

func gelu(v []float32) {
	for j, x := range v {
		f := float64(x)
		v[j] = float32(0.5 * f * (1 + math.Erf(f/math.Sqrt2)))
	}
}

// PolicyEvaluator scores decisions with the model for one goroutine.
type PolicyEvaluator struct {
	model   *PolicyModel
	enc     *Encoder
	state   []float32
	trunk   [][]float32
	joined  []float32
	scorer  [][]float32
	out     []float32
	options []float32
}

// NewEvaluator builds an evaluator; each goroutine needs its own.
func (m *PolicyModel) NewEvaluator() *PolicyEvaluator {
	e := &PolicyEvaluator{model: m, enc: NewEncoder(), state: make([]float32, WidthV2()), out: make([]float32, 1)}
	for _, l := range m.trunk {
		e.trunk = append(e.trunk, make([]float32, l.rows))
	}
	e.joined = make([]float32, m.trunk[len(m.trunk)-1].rows+OptionWidth())
	for _, l := range m.scorer {
		e.scorer = append(e.scorer, make([]float32, l.rows))
	}
	return e
}

func (e *PolicyEvaluator) runTrunk() {
	x := e.state
	for i, l := range e.model.trunk {
		l.apply(x, e.trunk[i])
		gelu(e.trunk[i])
		x = e.trunk[i]
	}
}

func (e *PolicyEvaluator) scoreOption(option []float32) float64 {
	h := e.trunk[len(e.trunk)-1]
	copy(e.joined, h)
	copy(e.joined[len(h):], option)
	x := e.joined
	for i, l := range e.model.scorer {
		l.apply(x, e.scorer[i])
		gelu(e.scorer[i])
		x = e.scorer[i]
	}
	e.model.score.apply(x, e.out)
	return float64(e.out[0])
}

func (e *PolicyEvaluator) heads() (value, result float64) {
	h := e.trunk[len(e.trunk)-1]
	e.model.value.apply(h, e.out)
	value = math.Tanh(float64(e.out[0])) * e.model.valueScale
	e.model.result.apply(h, e.out)
	return value, 1 / (1 + math.Exp(-float64(e.out[0])))
}

// Scores rates the listed options of a decision (indices into d.Actions;
// anchor is the greedy player's pick) from the deciding seat's view.
func (e *PolicyEvaluator) Scores(d *lookahead.Decision, options []int, anchor int) ([]float64, error) {
	pos, err := NewPosition(d.Battle, d.Seat, d.Seat)
	if err != nil {
		return nil, err
	}
	if err := e.enc.EncodeV2(pos, e.state); err != nil {
		return nil, err
	}
	width := OptionWidth()
	if cap(e.options) < width*len(options) {
		e.options = make([]float32, width*len(options))
	}
	rows := e.options[:width*len(options)]
	if err := e.enc.EncodeOptions(pos, d.Actions, options, anchor, rows); err != nil {
		return nil, err
	}
	e.runTrunk()
	out := make([]float64, len(options))
	for k := range options {
		out[k] = e.scoreOption(rows[k*width : (k+1)*width])
	}
	return out, nil
}

// DiceDecision reports a decision about the seat's offensive dice: one of
// its options is a reroll or a die choice.
func DiceDecision(d *lookahead.Decision, options []int) bool {
	for _, i := range options {
		if d.Actions[i].Type == command.TypePlanningReroll || lookahead.CardChoice(d.Actions[i]).Kind == "offensive_die" {
			return true
		}
	}
	return false
}

// SegmentAdvisor is an evaluator that only advises in some segments;
// elsewhere it gives no advice and the search picks its options as usual.
type SegmentAdvisor struct {
	*PolicyEvaluator
	Segments map[string]bool
}

func (a SegmentAdvisor) Scores(d *lookahead.Decision, options []int, greedy int) ([]float64, error) {
	if !a.Segments[string(d.Battle.Segment.Current)] {
		return nil, nil
	}
	return a.PolicyEvaluator.Scores(d, options, greedy)
}

// NetworkPlayer plays the policy network's pick among the search's options
// (no search). It keeps the greedy player's pick unless the network rates
// another option higher by more than Margin.
type NetworkPlayer struct {
	Label   string
	Options *lookahead.SearchPolicy
	Margin  float64
	// Segments, when set, limits the network to decisions in these
	// segments (offensive, defensive); elsewhere it keeps the greedy pick.
	Segments map[string]bool
	// SolverDice leaves dice decisions (any option a reroll or a die
	// choice) to the greedy player's exact dice solver.
	SolverDice bool
	eval       *PolicyEvaluator
	// Changed counts decisions where it left the greedy pick.
	Changed int
}

// NewNetworkPlayer builds a player; options lists each decision's options
// (its TopRerolls and TopAbilities), which no futures are played for.
func NewNetworkPlayer(m *PolicyModel, options *lookahead.SearchPolicy, margin float64) *NetworkPlayer {
	return &NetworkPlayer{Label: "network", Options: options, Margin: margin, eval: m.NewEvaluator()}
}

func (p *NetworkPlayer) Name() string { return p.Label }

// AdvisedCount reports the decisions where it left the greedy pick.
func (p *NetworkPlayer) AdvisedCount() int { return p.Changed }

func (p *NetworkPlayer) Choose(d *lookahead.Decision) (int, error) {
	if p.Segments != nil && !p.Segments[string(d.Battle.Segment.Current)] {
		return p.Options.Rollout.Choose(d)
	}
	options, anchor, err := p.Options.Options(d)
	if err != nil || len(options) < 2 {
		return anchor, err
	}
	if p.SolverDice && DiceDecision(d, options) {
		return anchor, nil
	}
	scores, err := p.eval.Scores(d, options, anchor)
	if err != nil {
		return anchor, nil
	}
	best, at := 0, -1
	for k, i := range options {
		if i == anchor {
			at = k
		}
		if scores[k] > scores[best] {
			best = k
		}
	}
	if at < 0 || scores[best]-scores[at] <= p.Margin {
		return anchor, nil
	}
	p.Changed++
	return options[best], nil
}
