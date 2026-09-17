package mlsim

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

const (
	EncodedV2MaxActions      = 128
	encodedV2MaxDice         = 10
	encodedV2MaxSymbols      = 8
	encodedV2MaxAbilities    = 12
	encodedV2MaxTiers        = 6
	encodedV2MaxRequirements = 2
	EncodedV2BaseFeatures    = 2560
	EncodedV2ActionFeatures  = 128
	encodedV2BaseFeatures    = EncodedV2BaseFeatures
	encodedV2ActionFeatures  = EncodedV2ActionFeatures
	EncodedV2ObservationSize = encodedV2BaseFeatures + EncodedV2MaxActions*encodedV2ActionFeatures
)

func encodeDecisionV2(transition Transition) (EncodedDecision, error) {
	snap := transition.Result.Snapshot
	if snap == nil || snap.ContentCatalog == nil {
		return EncodedDecision{}, fmt.Errorf("observation v2 requires a snapshot with public content catalog")
	}
	viewer := snap.ViewerActorID
	if viewer == "" {
		viewer = transition.ActorID
	}
	if !validSeat(viewer) {
		return EncodedDecision{}, fmt.Errorf("invalid v2 viewer %q", viewer)
	}
	opponent := otherSeat(viewer)
	own, other := snap.Actors[viewer], snap.Actors[opponent]
	symbols := sortedKeys(snap.ContentCatalog.Symbols)
	baseSymbols := symbols[:0]
	for _, symbol := range symbols {
		if !content.IsVenomIdentifier(symbol) {
			baseSymbols = append(baseSymbols, symbol)
		}
	}
	symbols = baseSymbols
	board := append(append([]string(nil), own.OffensiveAbilities...), own.DefensiveAbilities...)
	if err := auditV2Capacity(symbols, board, own); err != nil {
		return EncodedDecision{}, err
	}
	values := make([]float32, EncodedV2ObservationSize)
	values[0] = boolFloat(viewer == SeatIDs[0])
	values[1] = boolFloat(viewer == SeatIDs[1])
	values[2] = minFloat32(float32(snap.Round)/50, 1)
	values[3] = minFloat32(float32(snap.CompletedRounds)/50, 1)
	oneHot(values, 4, []string{"income", "offensive", "defensive", "status"}, string(snap.Segment))
	values[8] = boolFloat(snap.PriorityActorID == viewer)
	values[9] = boolFloat(snap.PriorityActorID == opponent)
	values[10] = boolFloat(snap.PriorityActorID == "")
	encodeV2ActorScalars(values, 16, own)
	encodeV2ActorScalars(values, 32, other)
	var dice []state.RolledDie
	kept := map[int]bool{}
	if own.Dice != nil {
		values[48] = float32(own.Dice.RollsUsed) / 3
		values[49] = float32(own.Dice.MaxRolls) / 3
		values[50] = float32(own.Dice.RollsRemaining) / 3
		values[51] = boolFloat(own.Dice.Complete)
		dice = own.Dice.Dice
		for _, index := range own.Dice.KeptIndices {
			kept[index] = true
		}
		values[52] = float32(len(dice)) / encodedV2MaxDice
		values[53] = float32(len(kept)) / encodedV2MaxDice
		for index, symbol := range symbols {
			values[64+index] = float32(own.Dice.SymbolCounts[symbol]) / encodedV2MaxDice
		}
	}
	values[54] = float32(len(own.QualifiedAbilities)) / encodedV2MaxAbilities
	values[55] = float32(len(board)) / encodedV2MaxAbilities
	actions := transition.Result.LegalActions
	if len(actions) > EncodedV2MaxActions {
		return EncodedDecision{}, fmt.Errorf("authority produced %d candidates; v2 capacity is %d", len(actions), EncodedV2MaxActions)
	}
	values[56] = float32(len(actions)) / EncodedV2MaxActions
	if err := encodeV2Dice(values, 128, dice, kept, symbols); err != nil {
		return EncodedDecision{}, err
	}
	qualified := stringSet(own.QualifiedAbilities)
	for index, abilityID := range board {
		ability, ok := snap.ContentCatalog.Abilities[abilityID]
		if !ok {
			return EncodedDecision{}, fmt.Errorf("ability board references missing definition %q", abilityID)
		}
		ability = effectiveV2Ability(abilityID, own, *snap.ContentCatalog)
		if err := encodeV2Ability(values[256+index*184:256+(index+1)*184], ability, qualified[abilityID], abilityID == own.SelectedAbility, dice, symbols); err != nil {
			return EncodedDecision{}, err
		}
	}
	mask := make([]byte, EncodedV2MaxActions/8)
	types := make([]command.Type, len(actions))
	validCandidates := 0
	for index, action := range actions {
		start := encodedV2BaseFeatures + index*encodedV2ActionFeatures
		if err := encodeV2Action(values[start:start+encodedV2ActionFeatures], action, viewer, opponent, own, *snap.ContentCatalog, board, dice, symbols); err != nil {
			return EncodedDecision{}, fmt.Errorf("candidate %d: %w", index, err)
		}
		if v2ActionProgresses(action, kept, own, *snap.ContentCatalog) {
			mask[index/8] |= 1 << (index % 8)
			validCandidates++
		}
		types[index] = action.Type
	}
	if validCandidates == 0 && !transition.Terminal && transition.TruncationReason == "" {
		return EncodedDecision{}, fmt.Errorf("nonterminal v2 decision has no legal candidates")
	}
	encoded := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(encoded[index*4:], math.Float32bits(value))
	}
	return EncodedDecision{
		ObservationSchema: ObservationSchemaV2,
		ObservationBase64: base64.StdEncoding.EncodeToString(encoded),
		ActionMaskBase64:  base64.StdEncoding.EncodeToString(mask),
		CandidateCount:    len(actions),
		CandidateTypes:    types,
	}, nil
}

// EncodeDecisionV2ForRuntime exposes the exact mechanics-aware feature vector
// used by the transport so an exported native policy cannot drift from Python.
func EncodeDecisionV2ForRuntime(transition Transition) ([]float32, []bool, error) {
	encoded, err := encodeDecisionV2(transition)
	if err != nil {
		return nil, nil, err
	}
	payload, err := base64.StdEncoding.DecodeString(encoded.ObservationBase64)
	if err != nil {
		return nil, nil, err
	}
	values := make([]float32, EncodedV2ObservationSize)
	for index := range values {
		values[index] = math.Float32frombits(binary.LittleEndian.Uint32(payload[index*4:]))
	}
	maskBits, err := base64.StdEncoding.DecodeString(encoded.ActionMaskBase64)
	if err != nil {
		return nil, nil, err
	}
	mask := make([]bool, EncodedV2MaxActions)
	for index := 0; index < encoded.CandidateCount; index++ {
		mask[index] = maskBits[index/8]&(1<<(index%8)) != 0
	}
	return values, mask, nil
}

func v2ActionProgresses(action command.Command, kept map[int]bool, actor snapshot.Actor, catalog snapshot.ContentCatalog) bool {
	switch action.Type {
	case command.TypePlanningKeep:
		return false
	case command.TypePlanningAbility:
		var payload command.PlanningAbilityPayload
		if json.Unmarshal(action.Payload, &payload) != nil {
			return false
		}
		return payload.AbilityID != actor.SelectedAbility || !sameV2Strings(payload.TargetIDs, actor.SelectedTargets)
	case command.TypePlanningTargets:
		var payload command.PlanningTargetsPayload
		if json.Unmarshal(action.Payload, &payload) != nil {
			return false
		}
		return !sameV2Strings(payload.TargetIDs, actor.SelectedTargets)
	case command.TypePlanningCards:
		if actor.DeckCount != 0 {
			return true
		}
		var payload command.PlanningCardsPayload
		if json.Unmarshal(action.Payload, &payload) != nil {
			return false
		}
		for _, instanceID := range payload.CardIDs {
			instance := actor.CardInstances[instanceID]
			definition := catalog.Cards[instance.DefinitionID]
			for _, operation := range definition.Operations {
				if operation.Type == "draw_cards" {
					return false
				}
			}
		}
	}
	return true
}

func sameV2Strings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range left {
		found := false
		for _, candidate := range right {
			found = found || value == candidate
		}
		if !found {
			return false
		}
	}
	return true
}

func auditV2Capacity(symbols, board []string, actor snapshot.Actor) error {
	if len(symbols) > encodedV2MaxSymbols {
		return fmt.Errorf("content has %d symbols; v2 capacity is %d", len(symbols), encodedV2MaxSymbols)
	}
	if len(board) > encodedV2MaxAbilities {
		return fmt.Errorf("actor has %d authored abilities; v2 capacity is %d", len(board), encodedV2MaxAbilities)
	}
	if actor.Dice != nil && len(actor.Dice.Dice) > encodedV2MaxDice {
		return fmt.Errorf("actor has %d current dice; v2 capacity is %d", len(actor.Dice.Dice), encodedV2MaxDice)
	}
	return nil
}

func encodeV2ActorScalars(values []float32, offset int, actor snapshot.Actor) {
	maxHealth, maxEnergy := actor.MaxHealth, actor.MaxEnergyPoints
	if maxHealth == 0 {
		maxHealth = 20
	}
	if maxEnergy == 0 {
		maxEnergy = 10
	}
	copy(values[offset:], []float32{
		float32(actor.CurrentHealth) / float32(max(maxHealth, 1)),
		float32(actor.MaxHealth) / 30,
		float32(actor.EnergyPoints) / float32(max(maxEnergy, 1)),
		float32(actor.HandCount) / 20,
		float32(actor.DeckCount) / 30,
		float32(actor.DiscardCount) / 30,
		float32(actor.RemovedCount) / 30,
		float32(actor.DiceCount) / encodedV2MaxDice,
		float32(actor.AbilityCount) / encodedV2MaxAbilities,
		float32(len(actor.Statuses)) / 10,
		float32(len(actor.Tokens)) / 10,
		boolFloat(actor.DefeatState == "defeated"),
	})
}

func encodeV2Dice(values []float32, offset int, dice []state.RolledDie, kept map[int]bool, symbols []string) error {
	for slot, die := range dice {
		start := offset + slot*14
		values[start] = 1
		values[start+1] = float32(die.Index) / encodedV2MaxDice
		values[start+2] = float32(die.Face) / 20
		values[start+3] = float32(die.Value) / 20
		values[start+4] = boolFloat(kept[die.Index])
		values[start+5] = boolFloat(!kept[die.Index])
		for _, symbol := range die.Symbols {
			index := indexOf(symbols, symbol)
			if index < 0 {
				if content.IsVenomIdentifier(symbol) {
					continue
				}
				return fmt.Errorf("rolled die references unknown symbol %q", symbol)
			}
			values[start+6+index] = 1
		}
	}
	return nil
}

func encodeV2Ability(output []float32, ability content.BattleAbilityDefinition, qualified, selected bool, dice []state.RolledDie, symbols []string) error {
	tiers := v2Tiers(ability)
	if len(tiers) > encodedV2MaxTiers {
		return fmt.Errorf("ability %q has %d tiers; v2 capacity is %d", ability.ID, len(tiers), encodedV2MaxTiers)
	}
	output[0] = 1
	output[1] = boolFloat(ability.Type == "offensive")
	output[2] = boolFloat(ability.Type == "defensive")
	output[3] = boolFloat(qualified)
	output[4] = boolFloat(selected)
	output[5] = float32(ability.Cost.Energy) / 10
	output[6] = float32(ability.Usage.MaximumPerSegment) / 10
	if ability.Targeting != nil {
		output[7] = float32(ability.Targeting.Minimum) / 5
		output[8] = float32(ability.Targeting.Maximum) / 5
		output[9] = boolFloat(ability.Targeting.Selector == "self")
		output[10] = boolFloat(strings.Contains(ability.Targeting.Selector, "enemy"))
		output[11] = boolFloat(ability.Targeting.Selector != "" && output[9] == 0 && output[10] == 0)
	}
	activationCount, bonusCount := 0, 0
	if ability.Qualification != nil {
		activationCount = len(ability.Qualification.ActivationTiers)
		bonusCount = len(ability.Qualification.ConditionalBonuses)
	}
	output[12] = float32(activationCount) / encodedV2MaxTiers
	output[13] = float32(bonusCount) / encodedV2MaxTiers
	if ability.Selection != nil {
		output[14] = boolFloat(ability.Selection.RequiresIncomingProposal)
		output[15] = float32(ability.Selection.TargetCount) / 5
	}
	for index, tier := range tiers {
		if err := encodeV2Tier(output[16+index*28:16+(index+1)*28], tier.tier, tier.conditional, dice, symbols); err != nil {
			return err
		}
	}
	return nil
}

type v2Tier struct {
	tier        content.AbilityTier
	conditional bool
}

func v2Tiers(ability content.BattleAbilityDefinition) []v2Tier {
	if ability.Qualification == nil {
		return nil
	}
	result := make([]v2Tier, 0, len(ability.Qualification.ActivationTiers)+len(ability.Qualification.ConditionalBonuses))
	for _, tier := range ability.Qualification.ActivationTiers {
		result = append(result, v2Tier{tier: tier})
	}
	for _, tier := range ability.Qualification.ConditionalBonuses {
		result = append(result, v2Tier{tier: tier, conditional: true})
	}
	return result
}

func encodeV2Tier(output []float32, tier content.AbilityTier, conditional bool, dice []state.RolledDie, symbols []string) error {
	requirements := tier.Requirements.All
	if len(requirements) > encodedV2MaxRequirements {
		return fmt.Errorf("tier %q has %d requirements; v2 capacity is %d", tier.ID, len(requirements), encodedV2MaxRequirements)
	}
	output[0] = 1
	output[1] = boolFloat(conditional)
	met, minimumProgress := len(requirements) > 0, float32(1)
	if len(requirements) == 0 {
		minimumProgress = 0
	}
	for _, requirement := range requirements {
		progress := v2RequirementProgress(requirement, dice)
		if !v2RequirementMet(requirement, dice) {
			met = false
		}
		if progress < minimumProgress {
			minimumProgress = progress
		}
	}
	output[2] = boolFloat(met)
	output[3] = minimumProgress
	output[4] = float32(len(requirements)) / encodedV2MaxRequirements
	for index, requirement := range requirements {
		start := 5 + index*8
		output[start] = 1
		output[start+1] = boolFloat(requirement.Type == "symbol_count")
		output[start+2] = boolFloat(requirement.Type == "exact_faces")
		output[start+3] = boolFloat(requirement.Type == "number_pattern")
		if symbolIndex := indexOf(symbols, requirement.SymbolID); symbolIndex >= 0 {
			output[start+4] = float32(symbolIndex+1) / encodedV2MaxSymbols
		}
		current, target := v2RequirementCurrentTarget(requirement, dice)
		output[start+5] = current / encodedV2MaxDice
		output[start+6] = target / encodedV2MaxDice
		output[start+7] = v2RequirementProgress(requirement, dice)
	}
	copy(output[21:28], v2OperationSummary(tier.Operations))
	return nil
}

func v2RequirementCurrentTarget(requirement content.BattleRequirement, dice []state.RolledDie) (float32, float32) {
	switch requirement.Type {
	case "symbol_count":
		current := 0
		for _, die := range dice {
			if containsText(die.Symbols, requirement.SymbolID) {
				current++
			}
		}
		target := requirement.Minimum
		if requirement.Exact != nil {
			target = *requirement.Exact
		} else if target == 0 {
			target = requirement.Maximum
		}
		return float32(current), float32(target)
	case "exact_faces":
		remaining := append([]int(nil), requirement.Faces...)
		matched := 0
		for _, die := range dice {
			for index, face := range remaining {
				if face == die.Face {
					remaining = append(remaining[:index], remaining[index+1:]...)
					matched++
					break
				}
			}
		}
		return float32(matched), float32(len(requirement.Faces))
	case "number_pattern":
		counts := map[int]int{}
		current := 0
		for _, die := range dice {
			counts[die.Face]++
			if counts[die.Face] > current {
				current = counts[die.Face]
			}
		}
		target := 2
		if requirement.Pattern == "three_of_a_kind" {
			target = 3
		}
		return float32(current), float32(target)
	default:
		return 0, 1
	}
}

func v2RequirementProgress(requirement content.BattleRequirement, dice []state.RolledDie) float32 {
	current, target := v2RequirementCurrentTarget(requirement, dice)
	if target <= 0 {
		return 0
	}
	if requirement.Type == "symbol_count" {
		if requirement.Exact != nil && current > float32(*requirement.Exact) {
			return maxFloat32(0, 1-(current-float32(*requirement.Exact))/encodedV2MaxDice)
		}
		if requirement.Maximum > 0 && current > float32(requirement.Maximum) {
			return maxFloat32(0, 1-(current-float32(requirement.Maximum))/encodedV2MaxDice)
		}
	}
	return minFloat32(current/target, 1)
}

func v2OperationSummary(operations []content.BattleOperation) []float32 {
	result := make([]float32, 7)
	for _, operation := range operations {
		amount := v2NumericAmount(operation.Amount)
		switch operation.Type {
		case "deal_damage":
			result[0] += amount / 20
		case "prevent_damage", "scale_damage":
			result[1] += amount / 20
		case "apply_status", "remove_status":
			result[2] += float32(operation.StackCount) / 10
		case "gain_resource", "spend_resource":
			result[3] += amount / 10
		case "roll_dice":
			result[4] += float32(operation.DiceCount) / encodedV2MaxDice
		case "apply_ability_modifier", "modify_die":
			result[5]++
		}
		result[6] += amount / 20
		if len(operation.Outcomes) > 0 {
			faces := map[int]bool{}
			for _, outcome := range operation.Outcomes {
				for _, face := range outcome.Faces {
					faces[face] = true
				}
			}
			denominator := max(len(faces), 1)
			for _, outcome := range operation.Outcomes {
				probability := float32(len(outcome.Faces)) / float32(denominator)
				nested := v2OperationSummary(outcome.Operations)
				for index := range result {
					result[index] += nested[index] * probability
				}
			}
		}
	}
	return result
}

func encodeV2Action(output []float32, action command.Command, viewer, opponent string, own snapshot.Actor, catalog snapshot.ContentCatalog, board []string, dice []state.RolledDie, symbols []string) error {
	kinds := []string{"planning_roll", "planning_keep", "planning_reroll", "planning_commit_cards", "planning_select_ability", "planning_select_targets", "planning_pass", "roll_dice", "commit_interaction", "pass"}
	kind := string(action.Type)
	oneHot(output, 0, kinds, kind)
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(action.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode v2 candidate: %w", err)
	}
	commitment := mapValue(payload["commitment"])
	targets := stringSlice(payload["target_ids"])
	if len(targets) == 0 {
		targets = stringSlice(commitment["target_ids"])
	}
	indices, exists := v2IntSliceIfPresent(payload, "reroll_indices")
	if !exists {
		indices, exists = v2IntSliceIfPresent(payload, "kept_indices")
	}
	if !exists {
		indices = intSlice(commitment["die_indices"])
	}
	cards := stringSlice(payload["card_ids"])
	if len(cards) == 0 {
		cards = stringSlice(commitment["card_ids"])
	}
	ability, _ := payload["ability_id"].(string)
	choiceID, _ := commitment["choice_id"].(string)
	if ability == "" {
		ability = choiceID
	}
	if _, exists := catalog.Abilities[ability]; !exists {
		ability = ""
	}
	output[10] = boolFloat(kind == "planning_pass" || kind == "pass")
	output[11] = boolFloat(kind == "planning_roll" || kind == "roll_dice")
	output[12] = boolFloat(kind == "planning_keep" || kind == "planning_reroll")
	output[13] = boolFloat(containsText(targets, viewer))
	output[14] = boolFloat(containsText(targets, opponent))
	output[15] = minFloat32(float32(len(targets))/5, 1)
	output[16] = minFloat32(float32(len(indices))/encodedV2MaxDice, 1)
	output[17] = minFloat32(float32(len(cards))/10, 1)
	output[18] = boolFloat(ability != "")
	output[19] = boolFloat(len(cards) > 0)
	if own.Dice != nil {
		output[21] = float32(own.Dice.RollsRemaining) / 3
	}
	for _, index := range indices {
		if index < 0 || index >= encodedV2MaxDice {
			return fmt.Errorf("candidate die index %d exceeds v2 capacity", index)
		}
		output[22+index] = 1
	}
	linked := output[32:]
	qualified := stringSet(own.QualifiedAbilities)
	if ability != "" {
		boardIndex := indexOf(board, ability)
		if boardIndex < 0 {
			return fmt.Errorf("candidate references ability outside viewer board: %q", ability)
		}
		linked[boardIndex] = 1
		definition, ok := catalog.Abilities[ability]
		if !ok {
			return fmt.Errorf("candidate references missing ability definition %q", ability)
		}
		definition = effectiveV2Ability(ability, own, catalog)
		linked[12] = 1
		linked[13] = boolFloat(qualified[ability])
		linked[14] = boolFloat(ability == own.SelectedAbility)
		linked[15] = float32(definition.Cost.Energy) / 10
		linked[16] = boolFloat(definition.Type == "offensive")
		linked[17] = boolFloat(definition.Type == "defensive")
		tiers := v2Tiers(definition)
		linked[18] = float32(len(tiers)) / encodedV2MaxTiers
		for tierIndex, tier := range tiers {
			progress := v2TierProgress(tier.tier, dice)
			if progress > linked[19] {
				linked[19] = progress
			}
			effect := v2OperationSummary(tier.tier.Operations)
			for effectIndex, value := range effect {
				if value > linked[20+effectIndex] {
					linked[20+effectIndex] = value
				}
			}
			if tierIndex >= 4 {
				continue
			}
			compact := linked[32+tierIndex*16 : 32+(tierIndex+1)*16]
			compact[0] = 1
			compact[1] = boolFloat(tier.conditional)
			compact[2] = progress
			met := len(tier.tier.Requirements.All) > 0
			for _, requirement := range tier.tier.Requirements.All {
				if !v2RequirementMet(requirement, dice) {
					met = false
				}
			}
			compact[3] = boolFloat(met)
			compact[4] = float32(len(tier.tier.Requirements.All)) / encodedV2MaxRequirements
			copy(compact[5:12], effect)
		}
		if definition.Targeting != nil {
			linked[27] = float32(definition.Targeting.Minimum) / 5
			linked[28] = float32(definition.Targeting.Maximum) / 5
		}
	}
	if len(cards) > 0 {
		linked[29] = 1
		var energy int
		for _, instanceID := range cards {
			definitionID := own.CardInstances[instanceID].DefinitionID
			definition, ok := catalog.Cards[definitionID]
			if !ok {
				return fmt.Errorf("candidate references missing viewer card definition %q", definitionID)
			}
			energy += definition.Cost.Energy
			effect := v2OperationSummary(definition.Operations)
			for index := range effect {
				linked[20+index] += effect[index]
			}
		}
		output[20] = float32(energy) / 10
	}
	if status, exists := catalog.Statuses[choiceID]; exists {
		linked[30] = 1
		operations := append([]content.BattleOperation(nil), status.Operations...)
		for _, trigger := range status.Triggers {
			operations = append(operations, trigger.Operations...)
		}
		effect := v2OperationSummary(operations)
		for index := range effect {
			linked[20+index] += effect[index]
		}
	}
	return nil
}

func v2TierProgress(tier content.AbilityTier, dice []state.RolledDie) float32 {
	if len(tier.Requirements.All) == 0 {
		return 0
	}
	result := float32(1)
	for _, requirement := range tier.Requirements.All {
		if progress := v2RequirementProgress(requirement, dice); progress < result {
			result = progress
		}
	}
	return result
}

func v2RequirementMet(requirement content.BattleRequirement, dice []state.RolledDie) bool {
	switch requirement.Type {
	case "symbol_count":
		count := 0
		for _, die := range dice {
			if containsText(die.Symbols, requirement.SymbolID) {
				count++
			}
		}
		if requirement.Exact != nil && count != *requirement.Exact {
			return false
		}
		if requirement.Minimum > 0 && count < requirement.Minimum {
			return false
		}
		if requirement.Maximum > 0 && count > requirement.Maximum {
			return false
		}
		return true
	case "exact_faces":
		if len(dice) != len(requirement.Faces) {
			return false
		}
		got := make([]int, len(dice))
		for index, die := range dice {
			got[index] = die.Face
		}
		want := append([]int(nil), requirement.Faces...)
		sort.Ints(got)
		sort.Ints(want)
		for index := range got {
			if got[index] != want[index] {
				return false
			}
		}
		return true
	case "number_pattern":
		counts := map[int]int{}
		for _, die := range dice {
			counts[die.Face]++
		}
		for _, count := range counts {
			switch requirement.Pattern {
			case "three_of_a_kind":
				if count >= 3 {
					return true
				}
			case "exact_pair":
				if count == 2 {
					return true
				}
			case "pair_or_better":
				if count >= 2 {
					return true
				}
			}
		}
	}
	return false
}

func effectiveV2Ability(abilityID string, actor snapshot.Actor, catalog snapshot.ContentCatalog) content.BattleAbilityDefinition {
	base := catalog.Abilities[abilityID]
	modifiers := make([]state.RuntimeAbilityModifier, 0)
	for _, modifier := range actor.AbilityModifiers {
		if modifier.AbilityID == abilityID {
			modifiers = append(modifiers, modifier)
		}
	}
	if len(modifiers) == 0 {
		return base
	}
	qualification := content.AbilityQualification{}
	if base.Qualification != nil {
		qualification = *base.Qualification
		qualification.ActivationTiers = append([]content.AbilityTier(nil), base.Qualification.ActivationTiers...)
		qualification.ConditionalBonuses = append([]content.AbilityTier(nil), base.Qualification.ConditionalBonuses...)
	}
	resolved := map[string]int{}
	for _, modifier := range modifiers {
		instance := actor.CardInstances[modifier.SourceCardInstanceID]
		card := catalog.Cards[instance.DefinitionID]
		for _, operation := range card.Operations {
			if operation.Modifier == nil || operation.Modifier.AddConditionalBonus == nil {
				continue
			}
			tier := *operation.Modifier.AddConditionalBonus
			if tier.ID == modifier.BonusID {
				if index, exists := resolved[tier.ID]; exists {
					qualification.ConditionalBonuses[index].Operations = append(
						qualification.ConditionalBonuses[index].Operations,
						tier.Operations...,
					)
				} else {
					qualification.ConditionalBonuses = append(qualification.ConditionalBonuses, tier)
					resolved[tier.ID] = len(qualification.ConditionalBonuses) - 1
				}
			}
		}
	}
	base.Qualification = &qualification
	return base
}

func v2NumericAmount(value any) float32 {
	switch typed := value.(type) {
	case int:
		return float32(typed)
	case float64:
		return float32(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return float32(parsed)
	default:
		return 0
	}
}

func v2IntSliceIfPresent(value map[string]any, key string) ([]int, bool) {
	raw, exists := value[key]
	if !exists {
		return nil, false
	}
	return intSlice(raw), true
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

func maxFloat32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
