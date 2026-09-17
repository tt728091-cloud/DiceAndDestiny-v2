package mlsim

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

const observationManifestSchemaV3 = "dice-and-destiny-observation-v3-manifest-v1"

type entityRangeV3 struct {
	Offset   int `json:"offset"`
	Rows     int `json:"rows"`
	Features int `json:"features"`
}

func (r entityRangeV3) end() int { return r.Offset + r.Rows*r.Features }

type observationLayoutV3 struct {
	Context         entityRangeV3 `json:"context"`
	Actors          entityRangeV3 `json:"actors"`
	Dice            entityRangeV3 `json:"dice"`
	Abilities       entityRangeV3 `json:"abilities"`
	Statuses        entityRangeV3 `json:"statuses"`
	Cards           entityRangeV3 `json:"cards"`
	Candidates      entityRangeV3 `json:"candidates"`
	ObservationSize int           `json:"observation_size"`
}

type observationManifestV3 struct {
	Schema                         string              `json:"schema"`
	ObservationSchema              string              `json:"observation_schema"`
	ActionSchema                   string              `json:"action_schema"`
	EnvironmentSchema              string              `json:"environment_schema"`
	ContentSHA256                  string              `json:"content_sha256"`
	EligibleCombatants             []string            `json:"eligible_combatants"`
	CombatantVocabulary            []string            `json:"combatant_vocabulary"`
	FormVocabulary                 []string            `json:"form_vocabulary"`
	SymbolVocabulary               []string            `json:"symbol_vocabulary"`
	DieVocabulary                  []string            `json:"die_vocabulary"`
	AbilityVocabulary              []string            `json:"ability_vocabulary"`
	StatusVocabulary               []string            `json:"status_vocabulary"`
	CardVocabulary                 []string            `json:"card_vocabulary"`
	OperationVocabulary            []string            `json:"operation_vocabulary"`
	CommandVocabulary              []string            `json:"command_vocabulary"`
	MaximumDicePerActor            int                 `json:"maximum_dice_per_actor"`
	MaximumActiveAbilitiesPerActor int                 `json:"maximum_active_abilities_per_actor"`
	MaximumPassivesPerActor        int                 `json:"maximum_passives_per_actor"`
	MaximumStatusesPerActor        int                 `json:"maximum_statuses_per_actor"`
	MaximumTokensPerActor          int                 `json:"maximum_tokens_per_actor"`
	MaximumVisibleHandCards        int                 `json:"maximum_visible_hand_cards"`
	MaximumTiersPerAbility         int                 `json:"maximum_tiers_per_ability"`
	MaximumRequirementsPerTier     int                 `json:"maximum_requirements_per_tier"`
	MaximumTargets                 int                 `json:"maximum_targets"`
	MaximumLegalCandidates         int                 `json:"maximum_legal_candidates"`
	Normalization                  map[string]float32  `json:"normalization"`
	Layout                         observationLayoutV3 `json:"layout"`
	CapacityEvidence               map[string]any      `json:"capacity_evidence"`
	ManifestSHA256                 string              `json:"manifest_sha256"`
}

func loadObservationManifestV3(path string) (*observationManifestV3, error) {
	if path == "" {
		return nil, fmt.Errorf("observation v3 requires -observation-manifest")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read observation v3 manifest: %w", err)
	}
	return decodeObservationManifestV3(data)
}

func decodeObservationManifestV3(data []byte) (*observationManifestV3, error) {
	var manifest observationManifestV3
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode observation v3 manifest: %w", err)
	}
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// V3EntityRange and V3Layout expose only the dimensions needed by a pinned
// deployment actor. The complete manifest remains validated inside mlsim.
type V3EntityRange struct {
	Offset   int
	Rows     int
	Features int
}

type V3Layout struct {
	Context, Actors, Dice, Abilities, Statuses, Cards, Candidates V3EntityRange
	ObservationSize, MaximumLegalCandidates                       int
	ManifestSHA256, ContentSHA256                                 string
}

// V3RuntimeEncoder owns one decoded frozen manifest across inference calls.
type V3RuntimeEncoder struct{ manifest *observationManifestV3 }

func NewV3RuntimeEncoder(manifestJSON []byte) (*V3RuntimeEncoder, error) {
	manifest, err := decodeObservationManifestV3(manifestJSON)
	if err != nil {
		return nil, err
	}
	return &V3RuntimeEncoder{manifest: manifest}, nil
}

func (e *V3RuntimeEncoder) Layout() V3Layout {
	convert := func(value entityRangeV3) V3EntityRange {
		return V3EntityRange{Offset: value.Offset, Rows: value.Rows, Features: value.Features}
	}
	manifest := e.manifest
	return V3Layout{
		Context: convert(manifest.Layout.Context), Actors: convert(manifest.Layout.Actors),
		Dice: convert(manifest.Layout.Dice), Abilities: convert(manifest.Layout.Abilities),
		Statuses: convert(manifest.Layout.Statuses), Cards: convert(manifest.Layout.Cards),
		Candidates:      convert(manifest.Layout.Candidates),
		ObservationSize: manifest.Layout.ObservationSize, MaximumLegalCandidates: manifest.MaximumLegalCandidates,
		ManifestSHA256: manifest.ManifestSHA256, ContentSHA256: manifest.ContentSHA256,
	}
}

func (e *V3RuntimeEncoder) Encode(transition Transition) ([]float32, []bool, error) {
	if e == nil || e.manifest == nil {
		return nil, nil, fmt.Errorf("observation v3 runtime encoder is not loaded")
	}
	encoded, err := encodeDecisionV3(transition, e.manifest)
	if err != nil {
		return nil, nil, err
	}
	payload, err := base64.StdEncoding.DecodeString(encoded.ObservationBase64)
	if err != nil || len(payload) != e.manifest.Layout.ObservationSize*4 {
		return nil, nil, fmt.Errorf("decode observation v3 runtime vector")
	}
	values := make([]float32, e.manifest.Layout.ObservationSize)
	for index := range values {
		values[index] = math.Float32frombits(binary.LittleEndian.Uint32(payload[index*4:]))
	}
	maskPayload, err := base64.StdEncoding.DecodeString(encoded.ActionMaskBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode observation v3 runtime mask")
	}
	mask := make([]bool, e.manifest.MaximumLegalCandidates)
	for index := range mask {
		mask[index] = maskPayload[index/8]&(1<<uint(index%8)) != 0
	}
	return values, mask, nil
}

func (m observationManifestV3) validate() error {
	if m.Schema != observationManifestSchemaV3 || m.ObservationSchema != ObservationSchemaV3 || m.ActionSchema != ActionSchemaV3 || m.EnvironmentSchema != EnvironmentSchemaV3 {
		return fmt.Errorf("unsupported observation v3 manifest versions")
	}
	ranges := []entityRangeV3{m.Layout.Context, m.Layout.Actors, m.Layout.Dice, m.Layout.Abilities, m.Layout.Statuses, m.Layout.Cards, m.Layout.Candidates}
	expected := 0
	for _, value := range ranges {
		if value.Offset != expected || value.Rows < 1 || value.Features < 1 {
			return fmt.Errorf("invalid observation v3 layout at offset %d, expected %d", value.Offset, expected)
		}
		expected = value.end()
	}
	if expected != m.Layout.ObservationSize || m.Layout.Candidates.Rows != m.MaximumLegalCandidates {
		return fmt.Errorf("observation v3 manifest layout size/capacity mismatch")
	}
	if m.ManifestSHA256 == "" || m.ContentSHA256 == "" {
		return fmt.Errorf("observation v3 manifest hashes are required")
	}
	return nil
}
