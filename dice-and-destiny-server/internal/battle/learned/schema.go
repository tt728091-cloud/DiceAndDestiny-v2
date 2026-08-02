package learned

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"diceanddestiny/server/internal/battle/mlsim"
)

const (
	MaximumActions  = 128
	BaseFeatures    = 128
	ActionFeatures  = 32
	ObservationSize = BaseFeatures + MaximumActions*ActionFeatures
)

var commandTypes = []string{
	"planning_roll",
	"planning_keep",
	"planning_reroll",
	"planning_commit_cards",
	"planning_select_ability",
	"planning_select_targets",
	"planning_pass",
	"roll_dice",
	"commit_interaction",
	"pass",
}

var segments = []string{"income", "offensive", "defensive", "status"}

type EncodedDecision struct {
	Observation []float32
	ActionMask  []bool
}

// EncodeDecision mirrors dice_destiny_ml.schema.SchemaEncoder. Its only input
// is the acting seat's viewer-safe transition and the complete authority action
// candidates already attached to that transition.
func EncodeDecision(transition mlsim.Transition) (EncodedDecision, error) {
	encoded, err := json.Marshal(transition)
	if err != nil {
		return EncodedDecision{}, fmt.Errorf("marshal viewer transition: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return EncodedDecision{}, fmt.Errorf("decode viewer transition: %w", err)
	}
	return encodeDecisionMap(root)
}

func encodeDecisionMap(transition map[string]any) (EncodedDecision, error) {
	result := object(transition["result"])
	snapshot := object(result["snapshot"])
	viewer := text(snapshot["viewer_actor_id"])
	if viewer == "" {
		viewer = text(transition["actor_id"])
	}
	if viewer != "seat-a" && viewer != "seat-b" {
		return EncodedDecision{}, fmt.Errorf("invalid viewer actor %q", viewer)
	}
	opponent := "seat-a"
	if viewer == "seat-a" {
		opponent = "seat-b"
	}
	actors := object(snapshot["actors"])
	own := object(actors[viewer])
	other := object(actors[opponent])
	base := make([]float32, BaseFeatures)
	base[0] = truth(viewer == "seat-a")
	base[1] = truth(viewer == "seat-b")
	base[2] = min32(number(snapshot["round"])/50, 1)
	base[3] = min32(number(snapshot["completed_rounds"])/50, 1)
	oneHot(base, 4, segments, text(snapshot["segment"]))
	bucketOneHot(base, 8, 16, text(snapshot["stage"]))
	priority := text(snapshot["priority_actor_id"])
	base[24] = truth(priority == viewer)
	base[25] = truth(priority == opponent)
	base[26] = truth(priority == "")
	actorScalars(base, 27, own)
	actorScalars(base, 39, other)
	diceFeatures(base, 51, own)
	diceFeatures(base, 61, other)
	statusBuckets(base, 71, own)
	statusBuckets(base, 79, other)
	handBuckets(base, 87, own)
	idBuckets(base, 99, 8, stringsOf(own["qualified_abilities"]))
	idBuckets(base, 107, 8, []string{text(own["selected_ability"])})
	idBuckets(base, 115, 8, []string{text(other["selected_ability"])})
	actions := array(result["legal_actions"])
	if len(actions) > MaximumActions {
		return EncodedDecision{}, fmt.Errorf("authority produced %d candidates; schema capacity is %d", len(actions), MaximumActions)
	}
	base[123] = float32(len(actions)) / MaximumActions
	base[124] = truth(text(snapshot["status"]) == "victory")
	base[125] = truth(text(snapshot["status"]) == "defeat")
	base[126] = truth(text(own["defeat_state"]) == "defeated")
	base[127] = truth(text(other["defeat_state"]) == "defeated")

	observation := make([]float32, ObservationSize)
	copy(observation, base)
	mask := make([]bool, MaximumActions)
	for index, value := range actions {
		features, featureErr := actionFeatureVector(object(value), viewer, opponent)
		if featureErr != nil {
			return EncodedDecision{}, fmt.Errorf("candidate %d: %w", index, featureErr)
		}
		copy(observation[BaseFeatures+index*ActionFeatures:], features)
		mask[index] = true
	}
	if len(actions) == 0 && !boolean(transition["terminal"]) && text(transition["truncation_reason"]) == "" {
		return EncodedDecision{}, fmt.Errorf("nonterminal decision has no legal candidates")
	}
	return EncodedDecision{Observation: observation, ActionMask: mask}, nil
}

func actorScalars(output []float32, offset int, actor map[string]any) {
	maxHealth := number(actor["max_health"])
	if maxHealth == 0 {
		maxHealth = 20
	}
	maxEnergy := number(actor["max_energy_points"])
	if maxEnergy == 0 {
		maxEnergy = 10
	}
	values := []float32{
		number(actor["current_health"]) / max32(maxHealth, 1),
		number(actor["max_health"]) / 30,
		number(actor["energy_points"]) / max32(maxEnergy, 1),
		number(actor["hand_count"]) / 20,
		number(actor["deck_count"]) / 30,
		number(actor["discard_count"]) / 30,
		number(actor["removed_count"]) / 30,
		number(actor["dice_count"]) / 10,
		number(actor["ability_count"]) / 10,
		float32(len(array(actor["statuses"]))) / 10,
		float32(len(array(actor["tokens"]))) / 10,
		truth(text(actor["defeat_state"]) == "defeated"),
	}
	copy(output[offset:], values)
}

func diceFeatures(output []float32, offset int, actor map[string]any) {
	history := array(actor["roll_history"])
	latest := map[string]any{}
	if len(history) > 0 {
		latest = object(history[len(history)-1])
	}
	dice := array(latest["dice"])
	output[offset] = min32(number(latest["number"])/3, 1)
	for index, value := range dice {
		if index >= 5 {
			break
		}
		output[offset+1+index] = number(object(value)["face"]) / 6
	}
	output[offset+6] = float32(len(dice)) / 5
	output[offset+7] = float32(len(array(latest["kept_indices"]))) / 5
	output[offset+8] = truth(text(actor["selected_ability"]) != "")
	output[offset+9] = float32(len(array(actor["selected_targets"]))) / 2
}

func statusBuckets(output []float32, offset int, actor map[string]any) {
	for _, value := range array(actor["statuses"]) {
		status := object(value)
		identifier := text(status["definition_id"])
		if identifier == "" {
			identifier = text(status["id"])
		}
		stacks := number(status["stack_count"])
		if stacks == 0 {
			stacks = number(status["stacks"])
		}
		if stacks == 0 {
			stacks = 1
		}
		output[offset+stableBucket(identifier, 8)] += min32(stacks/5, 1)
	}
}

func handBuckets(output []float32, offset int, actor map[string]any) {
	instances := object(actor["card_instances"])
	for _, instanceID := range stringsOf(actor["hand"]) {
		definition := text(object(instances[instanceID])["definition_id"])
		output[offset+stableBucket(definition, 12)] += 0.2
	}
}

func idBuckets(output []float32, offset, count int, identifiers []string) {
	for _, identifier := range identifiers {
		if identifier != "" {
			output[offset+stableBucket(identifier, count)] = 1
		}
	}
}

func actionFeatureVector(action map[string]any, viewer, opponent string) ([]float32, error) {
	features := make([]float32, ActionFeatures)
	kind := text(action["type"])
	oneHot(features, 0, commandTypes, kind)
	payload, err := payloadObject(action["payload"])
	if err != nil {
		return nil, err
	}
	features[10] = truth(kind == "planning_pass" || kind == "pass")
	features[11] = truth(kind == "planning_roll" || kind == "roll_dice")
	features[12] = truth(kind == "planning_keep" || kind == "planning_reroll")
	commitment := object(payload["commitment"])
	targets := stringsOf(payload["target_ids"])
	if len(targets) == 0 {
		targets = stringsOf(commitment["target_ids"])
	}
	features[13] = truth(contains(targets, viewer))
	features[14] = truth(contains(targets, opponent))
	features[15] = min32(float32(len(targets))/2, 1)
	indices := integers(payload["reroll_indices"])
	if len(indices) == 0 {
		indices = integers(payload["kept_indices"])
	}
	if len(indices) == 0 {
		indices = integers(commitment["die_indices"])
	}
	features[16] = min32(float32(len(indices))/5, 1)
	maskValue := 0
	for _, index := range indices {
		if index >= 0 && index < 5 {
			maskValue += 1 << index
		}
	}
	features[17] = float32(maskValue) / 31
	cards := stringsOf(payload["card_ids"])
	if len(cards) == 0 {
		cards = stringsOf(commitment["card_ids"])
	}
	features[18] = min32(float32(len(cards))/5, 1)
	for _, cardID := range cards {
		definition := cardID
		if index := strings.LastIndex(definition, "-"); index >= 0 {
			definition = definition[:index]
		}
		features[19+stableBucket(definition, 4)] = 1
	}
	ability := text(payload["ability_id"])
	if ability == "" {
		ability = text(commitment["choice_id"])
	}
	if ability != "" {
		features[23+stableBucket(ability, 4)] = 1
	}
	adjustments := array(commitment["planning_adjustments"])
	features[27] = truth(len(adjustments) > 0)
	if len(adjustments) > 0 {
		adjustment := object(adjustments[0])
		features[28] = truth(text(adjustment["actor_id"]) == viewer)
		features[29] = truth(text(adjustment["actor_id"]) == opponent)
		features[30] = number(adjustment["face"]) / 6
	}
	features[31] = float32(stableBucket(pythonJSON(payload), 1024)) / 1023
	return features, nil
}

func payloadObject(value any) (map[string]any, error) {
	if mapped, ok := value.(map[string]any); ok {
		return mapped, nil
	}
	if raw := text(value); raw != "" {
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		var parsed map[string]any
		if err := decoder.Decode(&parsed); err != nil {
			return nil, fmt.Errorf("decode action payload: %w", err)
		}
		return parsed, nil
	}
	return map[string]any{}, nil
}

// pythonJSON matches json.dumps(payload, sort_keys=True), including its
// default comma/colon spacing. That string is part of observation schema v1.
func pythonJSON(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			encodedKey, _ := json.Marshal(key)
			parts = append(parts, string(encodedKey)+": "+pythonJSON(typed[key]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		parts := make([]string, len(typed))
		for index := range typed {
			parts[index] = pythonJSON(typed[index])
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case string:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	case json.Number:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case int:
		return strconv.Itoa(typed)
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}

func stableBucket(identifier string, count int) int {
	digest := sha256.Sum256([]byte(identifier))
	return int(binary.BigEndian.Uint64(digest[:8]) % uint64(count))
}

func oneHot(output []float32, offset int, values []string, selected string) {
	for index, value := range values {
		if value == selected {
			output[offset+index] = 1
			return
		}
	}
}

func bucketOneHot(output []float32, offset, count int, identifier string) {
	if identifier != "" {
		output[offset+stableBucket(identifier, count)] = 1
	}
}

func object(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func array(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return nil
}

func stringsOf(value any) []string {
	values := array(value)
	result := make([]string, 0, len(values))
	for _, entry := range values {
		result = append(result, text(entry))
	}
	return result
}

func integers(value any) []int {
	values := array(value)
	result := make([]int, 0, len(values))
	for _, entry := range values {
		result = append(result, int(number(entry)))
	}
	return result
}

func text(value any) string {
	result, _ := value.(string)
	return result
}

func number(value any) float32 {
	switch typed := value.(type) {
	case json.Number:
		parsed, _ := strconv.ParseFloat(typed.String(), 32)
		return float32(parsed)
	case float64:
		return float32(typed)
	case float32:
		return typed
	case int:
		return float32(typed)
	}
	return 0
}

func boolean(value any) bool {
	result, _ := value.(bool)
	return result
}

func truth(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func min32(left, right float32) float32 {
	if left < right {
		return left
	}
	return right
}

func max32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
