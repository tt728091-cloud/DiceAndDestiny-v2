package learned

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"diceanddestiny/server/internal/battle/mlsim"
)

const (
	PolicyFormat                   = "dice-and-destiny-candidate-policy-v1"
	AcceptedModelID                = "blade-warden-maskable-ppo-seed-11-final-v1"
	AcceptedPolicyFileSHA256       = "dea4a6681fcd12d368dee6e720bb0742643a25dd46366080ab4e79fba59e3937"
	AcceptedCheckpointSHA256       = "e2e98c2ecadfe9e06892676962e751a116cb07183aa193882d73927866d760b8"
	AcceptedParameterSHA256        = "b53c6633893bbaca6dd429edb6f1aeb94585e6c24cf05c2dbc1451e2fd758f7a"
	AcceptedContentVersion         = "9eed6066ea8c95f8a60038647de935e88ed8d6e9cbc618229070a4d78945edc4"
	AcceptedSourceRevision         = "fff39759360ce89041940cec99410082af0671dd"
	AcceptedTrainingEngineRevision = "5d81e8f6de350a3bdcc3f4ccf42a3447443f83fe"
)

type tensor struct {
	Shape  []int     `json:"shape"`
	Values []float32 `json:"values"`
}

type policyFile struct {
	Format                 string            `json:"format"`
	ModelID                string            `json:"model_id"`
	Algorithm              string            `json:"algorithm"`
	Architecture           string            `json:"architecture"`
	SourceCheckpoint       string            `json:"source_checkpoint"`
	SourceCheckpointSHA256 string            `json:"source_checkpoint_sha256"`
	SourceParameterSHA256  string            `json:"source_parameter_sha256"`
	SourceRevision         string            `json:"source_revision"`
	TrainingEngineRevision string            `json:"training_engine_revision"`
	ContentVersion         string            `json:"content_version"`
	EnvironmentSchema      string            `json:"environment_schema"`
	ObservationSchema      string            `json:"observation_schema"`
	ActionSchema           string            `json:"action_schema"`
	ObservationSize        int               `json:"observation_size"`
	MaximumActions         int               `json:"maximum_actions"`
	BaseFeatures           int               `json:"base_features"`
	ActionFeatures         int               `json:"action_features"`
	Tensors                map[string]tensor `json:"tensors"`
}

type layer struct {
	rows    int
	columns int
	weights []float32
	bias    []float32
}

type PolicyMetadata struct {
	ModelID                string `json:"model_id"`
	PolicyExportSHA256     string `json:"policy_export_sha256"`
	Algorithm              string `json:"algorithm"`
	Architecture           string `json:"architecture"`
	SourceCheckpoint       string `json:"source_checkpoint"`
	SourceCheckpointSHA256 string `json:"source_checkpoint_sha256"`
	SourceParameterSHA256  string `json:"source_parameter_sha256"`
	SourceRevision         string `json:"source_revision"`
	TrainingEngineRevision string `json:"training_engine_revision"`
	ContentVersion         string `json:"content_version"`
	EnvironmentSchema      string `json:"environment_schema"`
	ObservationSchema      string `json:"observation_schema"`
	ActionSchema           string `json:"action_schema"`
}

type Policy struct {
	metadata   PolicyMetadata
	context1   layer
	context2   layer
	candidate1 layer
	candidate2 layer
	score1     layer
	score2     layer
}

func LoadAcceptedPolicy(path string) (*Policy, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read learned policy: %w", err)
	}
	actualFileHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if actualFileHash != AcceptedPolicyFileSHA256 {
		return nil, fmt.Errorf(
			"learned policy export SHA-256 mismatch: got %s, want %s",
			actualFileHash,
			AcceptedPolicyFileSHA256,
		)
	}
	var file policyFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return nil, fmt.Errorf("decode learned policy: %w", err)
	}
	if err := validatePolicyFile(file); err != nil {
		return nil, err
	}
	policy := &Policy{
		metadata: PolicyMetadata{
			ModelID: file.ModelID, PolicyExportSHA256: AcceptedPolicyFileSHA256,
			Algorithm: file.Algorithm, Architecture: file.Architecture,
			SourceCheckpoint: file.SourceCheckpoint, SourceCheckpointSHA256: file.SourceCheckpointSHA256,
			SourceParameterSHA256: file.SourceParameterSHA256, SourceRevision: file.SourceRevision,
			TrainingEngineRevision: file.TrainingEngineRevision,
			ContentVersion:         file.ContentVersion, EnvironmentSchema: file.EnvironmentSchema,
			ObservationSchema: file.ObservationSchema, ActionSchema: file.ActionSchema,
		},
	}
	if policy.context1, err = layerFrom(file, "action_net.context.0", 64, 128); err != nil {
		return nil, err
	}
	if policy.context2, err = layerFrom(file, "action_net.context.2", 64, 64); err != nil {
		return nil, err
	}
	if policy.candidate1, err = layerFrom(file, "action_net.candidate.0", 64, 32); err != nil {
		return nil, err
	}
	if policy.candidate2, err = layerFrom(file, "action_net.candidate.2", 64, 64); err != nil {
		return nil, err
	}
	if policy.score1, err = layerFrom(file, "action_net.score.0", 64, 128); err != nil {
		return nil, err
	}
	if policy.score2, err = layerFrom(file, "action_net.score.2", 1, 64); err != nil {
		return nil, err
	}
	return policy, nil
}

// Reviewed runtime rules update: automatic Effects removes response windows and
// moves Antidote to planning. Card IDs, model inputs and action encoding remain
// compatible; the exported models retain their original training metadata.
const AutomaticEffectsContentVersion = "80449df424b039b7409372aa4f6bbb33c9f2143c72629372dda2cd2d7bbd3792"

func VerifyContentVersion(contentRoot string) error {
	paths := []string{}
	err := filepath.Walk(filepath.Join(contentRoot, "battle_v1"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && filepath.Ext(path) == ".yaml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan learned-policy content: %w", err)
	}
	sort.Strings(paths)
	digest := sha256.New()
	serverRoot := filepath.Dir(contentRoot)
	for _, path := range paths {
		relative, err := filepath.Rel(serverRoot, path)
		if err != nil {
			return err
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write(payload)
	}
	actual := fmt.Sprintf("%x", digest.Sum(nil))
	if actual != AcceptedContentVersion && actual != AutomaticEffectsContentVersion {
		return fmt.Errorf("learned policy content version mismatch: got %s, want %s", actual, AcceptedContentVersion)
	}
	return nil
}

func (p *Policy) Metadata() PolicyMetadata {
	if p == nil {
		return PolicyMetadata{}
	}
	return p.metadata
}

func (p *Policy) Select(transition mlsim.Transition) (int, time.Duration, error) {
	if p == nil {
		return 0, 0, fmt.Errorf("learned policy is not loaded")
	}
	decision, err := EncodeDecision(transition)
	if err != nil {
		return 0, 0, err
	}
	started := time.Now()
	context := p.context2.tanh(p.context1.tanh(decision.Observation[:BaseFeatures]))
	selected := -1
	var best float32
	for index, valid := range decision.ActionMask {
		if !valid {
			continue
		}
		offset := BaseFeatures + index*ActionFeatures
		candidate := p.candidate2.tanh(p.candidate1.tanh(decision.Observation[offset : offset+ActionFeatures]))
		combined := make([]float32, 0, len(context)+len(candidate))
		combined = append(combined, context...)
		combined = append(combined, candidate...)
		score := p.score2.linear(p.score1.tanh(combined))[0]
		if selected < 0 || score > best {
			selected = index
			best = score
		}
	}
	latency := time.Since(started)
	if selected < 0 {
		return 0, latency, fmt.Errorf("learned policy received no legal candidate")
	}
	return selected, latency, nil
}

func validatePolicyFile(file policyFile) error {
	checks := []struct {
		name, got, want string
	}{
		{"format", file.Format, PolicyFormat},
		{"model ID", file.ModelID, AcceptedModelID},
		{"checkpoint SHA-256", file.SourceCheckpointSHA256, AcceptedCheckpointSHA256},
		{"parameter SHA-256", file.SourceParameterSHA256, AcceptedParameterSHA256},
		{"source revision", file.SourceRevision, AcceptedSourceRevision},
		{"training engine revision", file.TrainingEngineRevision, AcceptedTrainingEngineRevision},
		{"content version", file.ContentVersion, AcceptedContentVersion},
		{"environment schema", file.EnvironmentSchema, mlsim.EnvironmentSchemaVersion},
		{"observation schema", file.ObservationSchema, mlsim.ObservationSchemaVersion},
		{"action schema", file.ActionSchema, mlsim.ActionSchemaVersion},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf("learned policy %s mismatch: got %q, want %q", check.name, check.got, check.want)
		}
	}
	if file.ObservationSize != ObservationSize || file.MaximumActions != MaximumActions ||
		file.BaseFeatures != BaseFeatures || file.ActionFeatures != ActionFeatures {
		return fmt.Errorf("learned policy dimensions are incompatible")
	}
	return nil
}

func layerFrom(file policyFile, prefix string, rows, columns int) (layer, error) {
	weights, exists := file.Tensors[prefix+".weight"]
	if !exists || len(weights.Shape) != 2 || weights.Shape[0] != rows || weights.Shape[1] != columns || len(weights.Values) != rows*columns {
		return layer{}, fmt.Errorf("learned policy tensor %s.weight has incompatible dimensions", prefix)
	}
	bias, exists := file.Tensors[prefix+".bias"]
	if !exists || len(bias.Shape) != 1 || bias.Shape[0] != rows || len(bias.Values) != rows {
		return layer{}, fmt.Errorf("learned policy tensor %s.bias has incompatible dimensions", prefix)
	}
	return layer{rows: rows, columns: columns, weights: weights.Values, bias: bias.Values}, nil
}

func (l layer) linear(input []float32) []float32 {
	output := make([]float32, l.rows)
	for row := 0; row < l.rows; row++ {
		value := l.bias[row]
		base := row * l.columns
		for column := 0; column < l.columns; column++ {
			value += l.weights[base+column] * input[column]
		}
		output[row] = value
	}
	return output
}

func (l layer) tanh(input []float32) []float32 {
	output := l.linear(input)
	for index := range output {
		output[index] = float32(math.Tanh(float64(output[index])))
	}
	return output
}
