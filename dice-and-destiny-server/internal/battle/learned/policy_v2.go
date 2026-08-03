package learned

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"diceanddestiny/server/internal/battle/mlsim"
)

const PolicyFormatV2 = "dice-and-destiny-candidate-policy-v2"

// PolicyV2 is deliberately separate from the accepted v1 runtime. Loading a
// candidate never changes the pinned v1 policy or reinterprets its tensors.
type PolicyV2 struct {
	metadata   PolicyMetadata
	context1   layer
	context2   layer
	candidate1 layer
	candidate2 layer
	score1     layer
	score2     layer
}

func LoadCandidatePolicyV2(path, expectedSHA256 string) (*PolicyV2, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read v2 learned policy: %w", err)
	}
	actualHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if expectedSHA256 == "" || actualHash != expectedSHA256 {
		return nil, fmt.Errorf("v2 learned policy export SHA-256 mismatch: got %s, want %s", actualHash, expectedSHA256)
	}
	var file policyFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return nil, fmt.Errorf("decode v2 learned policy: %w", err)
	}
	if file.Format != PolicyFormatV2 ||
		file.EnvironmentSchema != mlsim.EnvironmentSchemaV2 ||
		file.ObservationSchema != mlsim.ObservationSchemaV2 ||
		file.ActionSchema != mlsim.ActionSchemaV2 ||
		file.ContentVersion != AcceptedContentVersion ||
		file.ObservationSize != mlsim.EncodedV2ObservationSize ||
		file.MaximumActions != mlsim.EncodedV2MaxActions ||
		file.BaseFeatures != mlsim.EncodedV2BaseFeatures || file.ActionFeatures != mlsim.EncodedV2ActionFeatures {
		return nil, fmt.Errorf("v2 learned policy metadata or dimensions are incompatible")
	}
	policy := &PolicyV2{metadata: PolicyMetadata{
		ModelID: file.ModelID, PolicyExportSHA256: actualHash,
		Algorithm: file.Algorithm, Architecture: file.Architecture,
		SourceCheckpoint: file.SourceCheckpoint, SourceCheckpointSHA256: file.SourceCheckpointSHA256,
		SourceParameterSHA256: file.SourceParameterSHA256, SourceRevision: file.SourceRevision,
		TrainingEngineRevision: file.TrainingEngineRevision, ContentVersion: file.ContentVersion,
		EnvironmentSchema: file.EnvironmentSchema, ObservationSchema: file.ObservationSchema,
		ActionSchema: file.ActionSchema,
	}}
	if policy.context1, err = layerFrom(file, "action_net.context.0", 96, mlsim.EncodedV2BaseFeatures); err != nil {
		return nil, err
	}
	if policy.context2, err = layerFrom(file, "action_net.context.2", 96, 96); err != nil {
		return nil, err
	}
	if policy.candidate1, err = layerFrom(file, "action_net.candidate.0", 96, 128); err != nil {
		return nil, err
	}
	if policy.candidate2, err = layerFrom(file, "action_net.candidate.2", 96, 96); err != nil {
		return nil, err
	}
	if policy.score1, err = layerFrom(file, "action_net.score.0", 96, 192); err != nil {
		return nil, err
	}
	if policy.score2, err = layerFrom(file, "action_net.score.2", 1, 96); err != nil {
		return nil, err
	}
	return policy, nil
}

func (p *PolicyV2) Metadata() PolicyMetadata {
	if p == nil {
		return PolicyMetadata{}
	}
	return p.metadata
}

func (p *PolicyV2) Select(transition mlsim.Transition) (int, time.Duration, error) {
	if p == nil {
		return 0, 0, fmt.Errorf("v2 learned policy is not loaded")
	}
	observation, mask, err := mlsim.EncodeDecisionV2ForRuntime(transition)
	if err != nil {
		return 0, 0, err
	}
	started := time.Now()
	context := p.context2.tanh(p.context1.tanh(observation[:mlsim.EncodedV2BaseFeatures]))
	selected := -1
	var best float32
	for index, valid := range mask {
		if !valid {
			continue
		}
		offset := mlsim.EncodedV2BaseFeatures + index*mlsim.EncodedV2ActionFeatures
		candidate := p.candidate2.tanh(p.candidate1.tanh(observation[offset : offset+mlsim.EncodedV2ActionFeatures]))
		combined := append(append(make([]float32, 0, 192), context...), candidate...)
		score := p.score2.linear(p.score1.tanh(combined))[0]
		if selected < 0 || score > best {
			selected, best = index, score
		}
	}
	latency := time.Since(started)
	if selected < 0 {
		return 0, latency, fmt.Errorf("v2 learned policy received no legal candidate")
	}
	return selected, latency, nil
}
