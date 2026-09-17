package mlsim

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/snapshot"
)

const (
	EncodedMaxActions      = 128
	encodedBaseFeatures    = 128
	encodedActionFeatures  = 32
	EncodedObservationSize = encodedBaseFeatures + EncodedMaxActions*encodedActionFeatures
	TransportModeFull      = "full"
	TransportModeEncoded   = "encoded"
	TransportModeParity    = "parity"
)

type EncodedDecision struct {
	ObservationSchema string         `json:"observation_schema"`
	ObservationBase64 string         `json:"observation_f32_le_base64"`
	ActionMaskBase64  string         `json:"action_mask_bits_base64"`
	CandidateCount    int            `json:"candidate_count"`
	CandidateTypes    []command.Type `json:"candidate_types"`
	ManifestSHA256    string         `json:"manifest_sha256,omitempty"`
}

func encodeDecision(transition Transition) (EncodedDecision, error) {
	snap := transition.Result.Snapshot
	if snap == nil {
		return EncodedDecision{}, fmt.Errorf("encoded decision requires a snapshot")
	}
	viewer := snap.ViewerActorID
	if viewer == "" {
		viewer = transition.ActorID
	}
	if !validSeat(viewer) {
		return EncodedDecision{}, fmt.Errorf("invalid encoded viewer %q", viewer)
	}
	opponent := otherSeat(viewer)
	own := snap.Actors[viewer]
	other := snap.Actors[opponent]
	values := make([]float32, EncodedObservationSize)
	if viewer == SeatIDs[0] {
		values[0] = 1
	} else {
		values[1] = 1
	}
	values[2] = minFloat32(float32(snap.Round)/50, 1)
	values[3] = minFloat32(float32(snap.CompletedRounds)/50, 1)
	oneHot(values, 4, []string{"income", "offensive", "defensive", "status"}, string(snap.Segment))
	bucketOneHot(values, 8, 16, snap.Stage)
	values[24] = boolFloat(snap.PriorityActorID == viewer)
	values[25] = boolFloat(snap.PriorityActorID == opponent)
	values[26] = boolFloat(snap.PriorityActorID == "")
	encodeActorScalars(values, 27, own)
	encodeActorScalars(values, 39, other)
	encodeDice(values, 51, own)
	encodeDice(values, 61, other)
	encodeStatuses(values, 71, own)
	encodeStatuses(values, 79, other)
	encodeHand(values, 87, own)
	encodeIDs(values, 99, 8, own.QualifiedAbilities)
	encodeIDs(values, 107, 8, []string{own.SelectedAbility})
	encodeIDs(values, 115, 8, []string{other.SelectedAbility})
	actions := transition.Result.LegalActions
	if len(actions) > EncodedMaxActions {
		return EncodedDecision{}, fmt.Errorf("authority produced %d candidates; encoded capacity is %d", len(actions), EncodedMaxActions)
	}
	values[123] = float32(len(actions)) / EncodedMaxActions
	values[124] = boolFloat(snap.Status == "victory")
	values[125] = boolFloat(snap.Status == "defeat")
	values[126] = boolFloat(own.DefeatState == "defeated")
	values[127] = boolFloat(other.DefeatState == "defeated")
	mask := make([]byte, EncodedMaxActions/8)
	types := make([]command.Type, len(actions))
	for index, action := range actions {
		if err := encodeAction(values[encodedBaseFeatures+index*encodedActionFeatures:], action, viewer, opponent); err != nil {
			return EncodedDecision{}, err
		}
		mask[index/8] |= 1 << (index % 8)
		types[index] = action.Type
	}
	encoded := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(encoded[index*4:], math.Float32bits(value))
	}
	return EncodedDecision{
		ObservationSchema: ObservationSchemaVersion,
		ObservationBase64: base64.StdEncoding.EncodeToString(encoded),
		ActionMaskBase64:  base64.StdEncoding.EncodeToString(mask),
		CandidateCount:    len(actions),
		CandidateTypes:    types,
	}, nil
}

func encodeActorScalars(values []float32, offset int, actor snapshot.Actor) {
	maximumHealth := actor.MaxHealth
	if maximumHealth == 0 {
		maximumHealth = 20
	}
	maximumEnergy := actor.MaxEnergyPoints
	if maximumEnergy == 0 {
		maximumEnergy = 10
	}
	encoded := []float32{
		float32(actor.CurrentHealth) / float32(max(maximumHealth, 1)),
		float32(actor.MaxHealth) / 30,
		float32(actor.EnergyPoints) / float32(max(maximumEnergy, 1)),
		float32(actor.HandCount) / 20,
		float32(actor.DeckCount) / 30,
		float32(actor.DiscardCount) / 30,
		float32(actor.RemovedCount) / 30,
		float32(actor.DiceCount) / 10,
		float32(actor.AbilityCount) / 10,
		float32(len(actor.Statuses)) / 10,
		float32(len(actor.Tokens)) / 10,
		boolFloat(actor.DefeatState == "defeated"),
	}
	copy(values[offset:], encoded)
}

func encodeDice(values []float32, offset int, actor snapshot.Actor) {
	if len(actor.RollHistory) == 0 {
		values[offset+8] = boolFloat(actor.SelectedAbility != "")
		values[offset+9] = float32(len(actor.SelectedTargets)) / 2
		return
	}
	latest := actor.RollHistory[len(actor.RollHistory)-1]
	values[offset] = minFloat32(float32(latest.Number)/3, 1)
	for index, die := range latest.Dice {
		if index >= 5 {
			break
		}
		values[offset+1+index] = float32(die.Face) / 6
	}
	values[offset+6] = float32(len(latest.Dice)) / 5
	values[offset+7] = float32(len(latest.KeptIndices)) / 5
	values[offset+8] = boolFloat(actor.SelectedAbility != "")
	values[offset+9] = float32(len(actor.SelectedTargets)) / 2
}

func encodeStatuses(values []float32, offset int, actor snapshot.Actor) {
	for _, status := range actor.Statuses {
		identifier := status.DefinitionID
		if identifier == "" {
			identifier = status.InstanceID
		}
		stacks := status.Stacks
		if stacks == 0 {
			stacks = 1
		}
		values[offset+stableBucket(identifier, 8)] += minFloat32(float32(stacks)/5, 1)
	}
}

func encodeHand(values []float32, offset int, actor snapshot.Actor) {
	for _, instanceID := range actor.Hand {
		definitionID := actor.CardInstances[instanceID].DefinitionID
		values[offset+stableBucket(definitionID, 12)] += 0.2
	}
}

func encodeIDs(values []float32, offset, count int, identifiers []string) {
	for _, identifier := range identifiers {
		if identifier != "" {
			values[offset+stableBucket(identifier, count)] = 1
		}
	}
}

func encodeAction(values []float32, action command.Command, viewer, opponent string) error {
	kinds := []string{"planning_roll", "planning_keep", "planning_reroll", "planning_commit_cards", "planning_select_ability", "planning_select_targets", "planning_pass", "roll_dice", "commit_interaction", "pass"}
	kind := string(action.Type)
	oneHot(values, 0, kinds, kind)
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(action.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode encoded candidate: %w", err)
	}
	values[10] = boolFloat(kind == "planning_pass" || kind == "pass")
	values[11] = boolFloat(kind == "planning_roll" || kind == "roll_dice")
	values[12] = boolFloat(kind == "planning_keep" || kind == "planning_reroll")
	commitment := mapValue(payload["commitment"])
	targets := stringSlice(payload["target_ids"])
	if len(targets) == 0 {
		targets = stringSlice(commitment["target_ids"])
	}
	values[13] = boolFloat(containsText(targets, viewer))
	values[14] = boolFloat(containsText(targets, opponent))
	values[15] = minFloat32(float32(len(targets))/2, 1)
	indices := intSlice(payload["reroll_indices"])
	if len(indices) == 0 {
		indices = intSlice(payload["kept_indices"])
	}
	if len(indices) == 0 {
		indices = intSlice(commitment["die_indices"])
	}
	values[16] = minFloat32(float32(len(indices))/5, 1)
	mask := 0
	for _, index := range indices {
		if index >= 0 && index < 5 {
			mask += 1 << index
		}
	}
	values[17] = float32(mask) / 31
	cards := stringSlice(payload["card_ids"])
	if len(cards) == 0 {
		cards = stringSlice(commitment["card_ids"])
	}
	values[18] = minFloat32(float32(len(cards))/5, 1)
	for _, cardID := range cards {
		prefix := cardID
		if split := strings.LastIndex(prefix, "-"); split >= 0 {
			prefix = prefix[:split]
		}
		values[19+stableBucket(prefix, 4)] = 1
	}
	ability, _ := payload["ability_id"].(string)
	if ability == "" {
		ability, _ = commitment["choice_id"].(string)
	}
	if ability != "" {
		values[23+stableBucket(ability, 4)] = 1
	}
	adjustments := sliceValue(commitment["planning_adjustments"])
	values[27] = boolFloat(len(adjustments) > 0)
	if len(adjustments) > 0 {
		adjustment := mapValue(adjustments[0])
		actorID, _ := adjustment["actor_id"].(string)
		values[28] = boolFloat(actorID == viewer)
		values[29] = boolFloat(actorID == opponent)
		values[30] = float32(numberInt(adjustment["face"])) / 6
	}
	values[31] = float32(stableBucket(pythonJSON(payload), 1024)) / 1023
	return nil
}

func pythonJSON(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	case string:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	case []any:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = pythonJSON(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for index, key := range keys {
			encodedKey, _ := json.Marshal(key)
			parts[index] = string(encodedKey) + ": " + pythonJSON(typed[key])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}

func stableBucket(identifier string, count int) int {
	digest := sha256.Sum256([]byte(identifier))
	return int(binary.BigEndian.Uint64(digest[:8]) % uint64(count))
}

func oneHot(values []float32, offset int, options []string, selected string) {
	for index, option := range options {
		if option == selected {
			values[offset+index] = 1
			return
		}
	}
}

func bucketOneHot(values []float32, offset, count int, identifier string) {
	if identifier != "" {
		values[offset+stableBucket(identifier, count)] = 1
	}
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func minFloat32(left, right float32) float32 {
	if left < right {
		return left
	}
	return right
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func sliceValue(value any) []any {
	result, _ := value.([]any)
	return result
}

func stringSlice(value any) []string {
	items := sliceValue(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func intSlice(value any) []int {
	items := sliceValue(value)
	result := make([]int, 0, len(items))
	for _, item := range items {
		result = append(result, numberInt(item))
	}
	return result
}

func numberInt(value any) int {
	switch typed := value.(type) {
	case json.Number:
		result, _ := typed.Int64()
		return int(result)
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func containsText(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
