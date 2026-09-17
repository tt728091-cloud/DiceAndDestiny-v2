package learned

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"diceanddestiny/server/internal/battle/mlsim"
)

const PolicyFormatV3 = "dice-and-destiny-candidate-policy-v3"

type policyArchitectureV3 struct {
	EntityWidth int    `json:"entity_width"`
	EntityDepth int    `json:"entity_depth"`
	Activation  string `json:"activation"`
}

type policyFileV3 struct {
	Format                    string               `json:"format"`
	ModelID                   string               `json:"model_id"`
	Algorithm                 string               `json:"algorithm"`
	Architecture              string               `json:"architecture"`
	ArchitectureConfig        policyArchitectureV3 `json:"architecture_config"`
	SourceCheckpoint          string               `json:"source_checkpoint"`
	SourceCheckpointSHA256    string               `json:"source_checkpoint_sha256"`
	SourceParameterSHA256     string               `json:"source_parameter_sha256"`
	SourceRevision            string               `json:"source_revision"`
	TrainingEngineRevision    string               `json:"training_engine_revision"`
	ContentVersion            string               `json:"content_version"`
	EnvironmentSchema         string               `json:"environment_schema"`
	ObservationSchema         string               `json:"observation_schema"`
	ActionSchema              string               `json:"action_schema"`
	ObservationSize           int                  `json:"observation_size"`
	MaximumActions            int                  `json:"maximum_actions"`
	ActionFeatures            int                  `json:"action_features"`
	ObservationManifestSHA256 string               `json:"observation_manifest_sha256"`
	ObservationManifest       json.RawMessage      `json:"observation_manifest"`
	Tensors                   map[string]tensor    `json:"tensors"`
}

type networkV3 struct {
	layers     []layer
	activation string
}

func (n networkV3) forward(input []float32) []float32 {
	value := input
	for index, current := range n.layers {
		value = current.linear(value)
		if index+1 == len(n.layers) {
			continue
		}
		for item := range value {
			switch n.activation {
			case "tanh":
				value[item] = float32(math.Tanh(float64(value[item])))
			case "relu":
				if value[item] < 0 {
					value[item] = 0
				}
			case "gelu":
				x := float64(value[item])
				value[item] = float32(0.5 * x * (1 + math.Erf(x/math.Sqrt2)))
			}
		}
	}
	return value
}

type PolicyV3 struct {
	metadata  PolicyMetadata
	encoder   *mlsim.V3RuntimeEncoder
	layout    mlsim.V3Layout
	context   networkV3
	actors    networkV3
	dice      networkV3
	abilities networkV3
	statuses  networkV3
	cards     networkV3
	candidate networkV3
	score     networkV3
}

func LoadCandidatePolicyV3(path, expectedSHA256 string) (*PolicyV3, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read v3 learned policy: %w", err)
	}
	actualHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if expectedSHA256 == "" || actualHash != expectedSHA256 {
		return nil, fmt.Errorf("v3 learned policy export SHA-256 mismatch: got %s, want %s", actualHash, expectedSHA256)
	}
	var file policyFileV3
	if err := json.Unmarshal(payload, &file); err != nil {
		return nil, fmt.Errorf("decode v3 learned policy: %w", err)
	}
	if file.Format != PolicyFormatV3 ||
		file.EnvironmentSchema != mlsim.EnvironmentSchemaV3 ||
		file.ObservationSchema != mlsim.ObservationSchemaV3 ||
		file.ActionSchema != mlsim.ActionSchemaV3 ||
		file.ArchitectureConfig.EntityWidth < 1 || file.ArchitectureConfig.EntityDepth < 1 ||
		(file.ArchitectureConfig.Activation != "tanh" && file.ArchitectureConfig.Activation != "relu" && file.ArchitectureConfig.Activation != "gelu") {
		return nil, fmt.Errorf("v3 learned policy metadata is incompatible")
	}
	encoder, err := mlsim.NewV3RuntimeEncoder(file.ObservationManifest)
	if err != nil {
		return nil, fmt.Errorf("load embedded v3 manifest: %w", err)
	}
	layout := encoder.Layout()
	if file.ObservationManifestSHA256 != layout.ManifestSHA256 || file.ContentVersion != layout.ContentSHA256 ||
		file.ObservationSize != layout.ObservationSize || file.MaximumActions != layout.MaximumLegalCandidates ||
		file.ActionFeatures != layout.Candidates.Features {
		return nil, fmt.Errorf("v3 learned policy manifest/dimensions are incompatible")
	}
	policy := &PolicyV3{
		metadata: PolicyMetadata{
			ModelID: file.ModelID, PolicyExportSHA256: actualHash,
			Algorithm: file.Algorithm, Architecture: file.Architecture,
			SourceCheckpoint: file.SourceCheckpoint, SourceCheckpointSHA256: file.SourceCheckpointSHA256,
			SourceParameterSHA256: file.SourceParameterSHA256, SourceRevision: file.SourceRevision,
			TrainingEngineRevision: file.TrainingEngineRevision, ContentVersion: file.ContentVersion,
			EnvironmentSchema: file.EnvironmentSchema, ObservationSchema: file.ObservationSchema,
			ActionSchema: file.ActionSchema,
		},
		encoder: encoder,
		layout:  layout,
	}
	width, depth, activation := file.ArchitectureConfig.EntityWidth, file.ArchitectureConfig.EntityDepth, file.ArchitectureConfig.Activation
	if policy.context, err = networkFromV3(file, "action_net.context", layout.Context.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.actors, err = networkFromV3(file, "action_net.state_encoders.actors", layout.Actors.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.dice, err = networkFromV3(file, "action_net.state_encoders.dice", layout.Dice.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.abilities, err = networkFromV3(file, "action_net.state_encoders.abilities", layout.Abilities.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.statuses, err = networkFromV3(file, "action_net.state_encoders.statuses", layout.Statuses.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.cards, err = networkFromV3(file, "action_net.state_encoders.cards", layout.Cards.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.candidate, err = networkFromV3(file, "action_net.candidate", layout.Candidates.Features, width, width, depth, activation); err != nil {
		return nil, err
	}
	if policy.score, err = networkFromV3(file, "action_net.score", width*7, width, 1, depth, activation); err != nil {
		return nil, err
	}
	return policy, nil
}

func networkFromV3(file policyFileV3, prefix string, input, hidden, output, depth int, activation string) (networkV3, error) {
	layers := make([]layer, 0, depth+1)
	columns := input
	for index := 0; index <= depth; index++ {
		rows := hidden
		if index == depth {
			rows = output
		}
		current, err := layerFromV3(file, fmt.Sprintf("%s.%d", prefix, index*2), rows, columns)
		if err != nil {
			return networkV3{}, err
		}
		layers = append(layers, current)
		columns = rows
	}
	return networkV3{layers: layers, activation: activation}, nil
}

func layerFromV3(file policyFileV3, prefix string, rows, columns int) (layer, error) {
	proxy := policyFile{Tensors: file.Tensors}
	return layerFrom(proxy, prefix, rows, columns)
}

func (p *PolicyV3) Metadata() PolicyMetadata {
	if p == nil {
		return PolicyMetadata{}
	}
	return p.metadata
}

func (p *PolicyV3) Select(transition mlsim.Transition) (int, time.Duration, error) {
	if p == nil || p.encoder == nil {
		return 0, 0, fmt.Errorf("v3 learned policy is not loaded")
	}
	observation, mask, err := p.encoder.Encode(transition)
	if err != nil {
		return 0, 0, err
	}
	started := time.Now()
	selected, err := p.SelectEncoded(observation, mask)
	return selected, time.Since(started), err
}

func (p *PolicyV3) SelectEncoded(observation []float32, mask []bool) (int, error) {
	if len(observation) != p.layout.ObservationSize || len(mask) != p.layout.MaximumLegalCandidates {
		return 0, fmt.Errorf("v3 learned policy received incompatible encoded dimensions")
	}
	state := p.context.forward(entityRowV3(observation, p.layout.Context, 0))
	state = append(state, pooledV3(observation, p.layout.Actors, p.actors)...)
	state = append(state, pooledV3(observation, p.layout.Dice, p.dice)...)
	state = append(state, pooledV3(observation, p.layout.Abilities, p.abilities)...)
	state = append(state, pooledV3(observation, p.layout.Statuses, p.statuses)...)
	state = append(state, pooledV3(observation, p.layout.Cards, p.cards)...)
	selected := -1
	var best float32
	for index, valid := range mask {
		if !valid {
			continue
		}
		candidate := p.candidate.forward(entityRowV3(observation, p.layout.Candidates, index))
		input := make([]float32, 0, len(state)+len(candidate))
		input = append(input, state...)
		input = append(input, candidate...)
		score := p.score.forward(input)[0]
		if selected < 0 || score > best {
			selected, best = index, score
		}
	}
	if selected < 0 {
		return 0, fmt.Errorf("v3 learned policy received no legal candidate")
	}
	return selected, nil
}

func entityRowV3(observation []float32, value mlsim.V3EntityRange, row int) []float32 {
	start := value.Offset + row*value.Features
	return observation[start : start+value.Features]
}

func pooledV3(observation []float32, value mlsim.V3EntityRange, network networkV3) []float32 {
	width := network.layers[len(network.layers)-1].rows
	result := make([]float32, width)
	count := 0
	for row := 0; row < value.Rows; row++ {
		input := entityRowV3(observation, value, row)
		if input[0] <= 0 {
			continue
		}
		encoded := network.forward(input)
		for index := range result {
			result[index] += encoded[index]
		}
		count++
	}
	if count > 0 {
		for index := range result {
			result[index] /= float32(count)
		}
	}
	return result
}
