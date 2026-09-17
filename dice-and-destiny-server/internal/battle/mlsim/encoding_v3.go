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

var segmentsV3 = []string{"income", "offensive", "defensive", "ongoing_effects"}

func encodeDecisionV3(transition Transition, manifest *observationManifestV3) (EncodedDecision, error) {
	if manifest == nil {
		return EncodedDecision{}, fmt.Errorf("observation v3 manifest is not loaded")
	}
	snap := transition.Result.Snapshot
	if snap == nil || snap.ContentCatalog == nil {
		return EncodedDecision{}, fmt.Errorf("observation v3 requires a snapshot with public content catalog")
	}
	viewer := snap.ViewerActorID
	if viewer == "" {
		viewer = transition.ActorID
	}
	if !validSeat(viewer) {
		return EncodedDecision{}, fmt.Errorf("invalid v3 viewer %q", viewer)
	}
	if err := auditV3Catalog(*snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	opponent := otherSeat(viewer)
	own, other := snap.Actors[viewer], snap.Actors[opponent]
	values := make([]float32, manifest.Layout.ObservationSize)
	encodeV3Context(v3Row(values, manifest.Layout.Context, 0), transition, snap, viewer, opponent, own, other, manifest)
	if err := encodeV3Actor(v3Row(values, manifest.Layout.Actors, 0), own, true, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Actor(v3Row(values, manifest.Layout.Actors, 1), other, false, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Dice(values, own, 0, true, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Dice(values, other, 1, false, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Abilities(values, own, 0, true, *snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Abilities(values, other, 1, false, *snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Statuses(values, own, 0, true, snap, *snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Statuses(values, other, 1, false, snap, *snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	if err := encodeV3Cards(values, own, *snap.ContentCatalog, manifest); err != nil {
		return EncodedDecision{}, err
	}
	actions := transition.Result.LegalActions
	if len(actions) > manifest.MaximumLegalCandidates {
		return EncodedDecision{}, fmt.Errorf("authority produced %d candidates; frozen v3 capacity is %d", len(actions), manifest.MaximumLegalCandidates)
	}
	mask := make([]byte, (manifest.MaximumLegalCandidates+7)/8)
	types := make([]command.Type, len(actions))
	kept := map[int]bool{}
	if own.Dice != nil {
		for _, index := range own.Dice.KeptIndices {
			kept[index] = true
		}
	}
	board := append(append(append([]string(nil), own.OffensiveAbilities...), own.DefensiveAbilities...), own.PassiveAbilities...)
	valid := 0
	for index, action := range actions {
		if err := encodeV3Candidate(v3Row(values, manifest.Layout.Candidates, index), action, viewer, opponent, own, board, *snap.ContentCatalog, manifest); err != nil {
			return EncodedDecision{}, fmt.Errorf("candidate %d: %w", index, err)
		}
		if v2ActionProgresses(action, kept, own, *snap.ContentCatalog) {
			mask[index/8] |= 1 << (index % 8)
			valid++
		}
		types[index] = action.Type
	}
	if valid == 0 && !transition.Terminal && transition.TruncationReason == "" {
		return EncodedDecision{}, fmt.Errorf("nonterminal v3 decision has no progressing legal candidates")
	}
	encoded := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(encoded[index*4:], math.Float32bits(value))
	}
	return EncodedDecision{
		ObservationSchema: ObservationSchemaV3,
		ObservationBase64: base64.StdEncoding.EncodeToString(encoded),
		ActionMaskBase64:  base64.StdEncoding.EncodeToString(mask),
		CandidateCount:    len(actions),
		CandidateTypes:    types,
		ManifestSHA256:    manifest.ManifestSHA256,
	}, nil
}

func encodeV3Context(output []float32, transition Transition, snap *snapshot.Battle, viewer, opponent string, own, other snapshot.Actor, manifest *observationManifestV3) {
	output[0] = 1
	output[1] = boolFloat(viewer == SeatIDs[0])
	output[2] = boolFloat(viewer == SeatIDs[1])
	output[3] = v3Scaled(float32(snap.Round), "round", manifest)
	output[4] = v3Scaled(float32(snap.CompletedRounds), "round", manifest)
	oneHot(output, 5, segmentsV3, string(snap.Segment))
	output[9] = boolFloat(snap.PriorityActorID == viewer)
	output[10] = boolFloat(snap.PriorityActorID == opponent)
	output[11] = boolFloat(snap.PriorityActorID == "")
	output[12] = boolFloat(transition.Terminal)
	output[13] = boolFloat(transition.Winner == viewer)
	output[14] = boolFloat(transition.Winner == opponent)
	output[15] = boolFloat(snap.Status == "draw" || (transition.Terminal && transition.Winner == ""))
	output[16] = float32(len(transition.Result.LegalActions)) / float32(max(manifest.MaximumLegalCandidates, 1))
	output[17], _ = v3VocabularyIndex(own.CurrentForm, manifest.FormVocabulary, true)
	output[18], _ = v3VocabularyIndex(other.CurrentForm, manifest.FormVocabulary, true)
	output[19] = boolFloat(own.Dice != nil)
	output[20] = boolFloat(other.Dice != nil)
	output[21] = boolFloat(snap.Stage == "planning")
	output[22] = boolFloat(string(snap.Segment) == "defensive")
	output[23] = boolFloat(string(snap.Segment) == "ongoing_effects")
	if snap.Flow != nil {
		output[24] = float32(snap.Flow.Iteration) / 20
	}
	output[25] = boolFloat(snap.Resolution != nil && snap.Resolution.ActiveWindow != nil)
	output[26] = boolFloat(snap.Damage != nil)
	output[27] = boolFloat(snap.SettledDamage != nil)
}

func encodeV3Actor(output []float32, actor snapshot.Actor, own bool, manifest *observationManifestV3) error {
	if actor.DefinitionID == "" {
		return nil
	}
	output[0] = 1
	output[1] = boolFloat(own)
	output[2] = boolFloat(!own)
	var err error
	output[3], err = v3VocabularyIndex(actor.DefinitionID, manifest.CombatantVocabulary, false)
	if err != nil {
		return err
	}
	output[4] = v3Scaled(float32(actor.CurrentHealth), "health", manifest)
	output[5] = v3Scaled(float32(actor.MaxHealth), "health", manifest)
	output[6] = float32(actor.CurrentHealth) / float32(max(actor.MaxHealth, 1))
	output[7] = v3Scaled(float32(actor.EnergyPoints), "energy", manifest)
	output[8] = v3Scaled(float32(actor.MaxEnergyPoints), "energy", manifest)
	output[9] = v3Scaled(float32(actor.HandCount), "hand", manifest)
	output[10] = v3Scaled(float32(actor.DeckCount), "deck", manifest)
	output[11] = v3Scaled(float32(actor.DiscardCount), "deck", manifest)
	output[12] = v3Scaled(float32(actor.RemovedCount), "deck", manifest)
	output[13] = v3Scaled(float32(actor.DiceCount), "dice", manifest)
	output[14] = v3Scaled(float32(actor.AbilityCount), "abilities", manifest)
	output[15] = v3Scaled(float32(len(actor.Statuses)), "statuses", manifest)
	output[16] = float32(len(actor.Tokens)) / float32(max(manifest.MaximumTokensPerActor, 1))
	output[17] = boolFloat(actor.DefeatState == "defeated")
	output[18] = v3Scaled(float32(actor.MaxHandSize), "hand", manifest)
	output[19] = boolFloat(own && len(actor.Hand) > 0)
	output[20], err = v3VocabularyIndex(actor.CurrentForm, manifest.FormVocabulary, true)
	output[21] = boolFloat(actor.CurrentForm != "")
	return err
}

func encodeV3Dice(values []float32, actor snapshot.Actor, ownerSlot int, own bool, manifest *observationManifestV3) error {
	if actor.Dice == nil {
		return nil
	}
	dice := actor.Dice.Dice
	if len(dice) > manifest.MaximumDicePerActor {
		return fmt.Errorf("actor has %d visible dice; frozen v3 capacity is %d", len(dice), manifest.MaximumDicePerActor)
	}
	kept := map[int]bool{}
	for _, index := range actor.Dice.KeptIndices {
		kept[index] = true
	}
	base := ownerSlot * manifest.MaximumDicePerActor
	for slot, die := range dice {
		output := v3Row(values, manifest.Layout.Dice, base+slot)
		output[0], output[1], output[2], output[3] = 1, boolFloat(own), boolFloat(!own), 1
		index, err := v3VocabularyIndex(die.DieID, manifest.DieVocabulary, false)
		if err != nil {
			return err
		}
		output[4] = index
		output[5] = float32(die.Index) / float32(max(manifest.MaximumDicePerActor, 1))
		output[6] = v3Scaled(float32(die.Face), "face", manifest)
		output[7] = v3Scaled(float32(die.Value), "face", manifest)
		output[8] = boolFloat(kept[die.Index])
		output[9] = boolFloat(!kept[die.Index])
		output[10] = boolFloat(actor.Dice.Complete)
		output[11] = v3Scaled(float32(actor.Dice.RollsRemaining), "rolls", manifest)
		for _, symbol := range die.Symbols {
			symbolIndex := indexOf(manifest.SymbolVocabulary, symbol)
			if symbolIndex < 0 {
				if content.IsVenomIdentifier(symbol) {
					continue
				}
				return fmt.Errorf("visible die references unknown symbol %q", symbol)
			}
			feature := 12 + symbolIndex
			if feature >= len(output) {
				return fmt.Errorf("symbol vocabulary exceeds die feature width")
			}
			output[feature]++
		}
	}
	return nil
}

func encodeV3Abilities(values []float32, actor snapshot.Actor, ownerSlot int, own bool, catalog snapshot.ContentCatalog, manifest *observationManifestV3) error {
	type boardEntry struct{ id, role string }
	board := make([]boardEntry, 0, len(actor.OffensiveAbilities)+len(actor.DefensiveAbilities)+len(actor.PassiveAbilities))
	for _, id := range actor.OffensiveAbilities {
		board = append(board, boardEntry{id, "offensive"})
	}
	for _, id := range actor.DefensiveAbilities {
		board = append(board, boardEntry{id, "defensive"})
	}
	for _, id := range actor.PassiveAbilities {
		board = append(board, boardEntry{id, "passive"})
	}
	if len(board) > manifest.MaximumActiveAbilitiesPerActor {
		return fmt.Errorf("active ability board has %d rows; frozen v3 capacity is %d", len(board), manifest.MaximumActiveAbilitiesPerActor)
	}
	qualified := stringSet(actor.QualifiedAbilities)
	base := ownerSlot * manifest.MaximumActiveAbilitiesPerActor
	var dice []state.RolledDie
	if actor.Dice != nil {
		dice = actor.Dice.Dice
	}
	for slot, entry := range board {
		output := v3Row(values, manifest.Layout.Abilities, base+slot)
		idIndex, err := v3VocabularyIndex(entry.id, manifest.AbilityVocabulary, false)
		if err != nil {
			return err
		}
		definition, exists := catalog.Abilities[entry.id]
		if !exists {
			return fmt.Errorf("catalog omits active ability %q", entry.id)
		}
		definition = effectiveV2Ability(entry.id, actor, catalog)
		output[0], output[1], output[2], output[3] = 1, boolFloat(own), boolFloat(!own), idIndex
		output[4], output[5], output[6] = boolFloat(entry.role == "offensive"), boolFloat(entry.role == "defensive"), boolFloat(entry.role == "passive")
		output[7], output[8] = boolFloat(qualified[entry.id]), boolFloat(entry.id == actor.SelectedAbility)
		output[9] = v3Scaled(float32(definition.Cost.Energy), "energy", manifest)
		output[10] = float32(definition.Usage.MaximumPerSegment) / 10
		if definition.Targeting != nil {
			output[11] = v3Scaled(float32(definition.Targeting.Minimum), "targets", manifest)
			output[12] = v3Scaled(float32(definition.Targeting.Maximum), "targets", manifest)
			output[13] = boolFloat(definition.Targeting.Selector == "self")
			output[14] = boolFloat(strings.Contains(definition.Targeting.Selector, "enemy"))
			output[15] = boolFloat(strings.Contains(definition.Targeting.Selector, "status"))
		}
		output[16] = boolFloat(actor.CurrentForm != "")
		tiers := v2Tiers(definition)
		if len(tiers) > manifest.MaximumTiersPerAbility {
			return fmt.Errorf("ability %q exceeds frozen tier capacity", entry.id)
		}
		output[17] = float32(len(tiers)) / float32(max(manifest.MaximumTiersPerAbility, 1))
		operations := []content.BattleOperation{}
		requirements := 0
		met := 0
		for _, tier := range tiers {
			progress := v2TierProgress(tier.tier, dice)
			if progress > output[18] {
				output[18] = progress
			}
			if !tier.conditional && len(tier.tier.Requirements.All) > 0 {
				qualifiedTier := true
				for _, requirement := range tier.tier.Requirements.All {
					if !v2RequirementMet(requirement, dice) {
						qualifiedTier = false
					}
				}
				if qualifiedTier {
					met++
				}
			}
			requirements += len(tier.tier.Requirements.All)
			operations = append(operations, tier.tier.Operations...)
		}
		output[19] = float32(met) / float32(max(manifest.MaximumTiersPerAbility, 1))
		output[20] = float32(requirements) / float32(max(manifest.MaximumTiersPerAbility*manifest.MaximumRequirementsPerTier, 1))
		copy(output[24:31], v2OperationSummary(operations))
		if err := encodeV3OperationTypes(output, 32, operations, manifest); err != nil {
			return err
		}
	}
	return nil
}

func encodeV3Statuses(values []float32, actor snapshot.Actor, ownerSlot int, own bool, snap *snapshot.Battle, catalog snapshot.ContentCatalog, manifest *observationManifestV3) error {
	statuses := append([]state.StatusState(nil), actor.Statuses...)
	if len(statuses) > manifest.MaximumStatusesPerActor {
		return fmt.Errorf("actor has %d active statuses; frozen v3 capacity is %d", len(statuses), manifest.MaximumStatusesPerActor)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].DefinitionID == statuses[j].DefinitionID {
			return statuses[i].InstanceID < statuses[j].InstanceID
		}
		return statuses[i].DefinitionID < statuses[j].DefinitionID
	})
	base := ownerSlot * manifest.MaximumStatusesPerActor
	for slot, status := range statuses {
		output := v3Row(values, manifest.Layout.Statuses, base+slot)
		idIndex, err := v3VocabularyIndex(status.DefinitionID, manifest.StatusVocabulary, false)
		if err != nil {
			return err
		}
		definition, exists := catalog.Statuses[status.DefinitionID]
		if !exists {
			return fmt.Errorf("catalog omits active status %q", status.DefinitionID)
		}
		output[0], output[1], output[2], output[3] = 1, boolFloat(own), boolFloat(!own), idIndex
		output[4] = v3Scaled(float32(status.Stacks), "stacks", manifest)
		output[5] = v3Scaled(float32(definition.Stacking.StackLimit), "stacks", manifest)
		output[6], output[7] = boolFloat(definition.Polarity == "positive"), boolFloat(definition.Polarity == "negative")
		output[8] = boolFloat(definition.Polarity != "positive" && definition.Polarity != "negative")
		output[9], output[10] = boolFloat(definition.ActivationMode == "automatic"), boolFloat(definition.ActivationMode == "optional")
		output[11] = boolFloat(definition.ActivationMode != "" && definition.ActivationMode != "automatic" && definition.ActivationMode != "optional")
		output[12] = boolFloat(definition.Lifecycle.Persistent)
		output[13] = boolFloat(definition.Lifecycle.ConsumeOnTriggerCheckpoint)
		output[14] = boolFloat(definition.Lifecycle.ConsumeOnPlay)
		output[15] = boolFloat(definition.Lifecycle.RemoveAfterResolution)
		output[16] = boolFloat(definition.Lifecycle.RemoveOnDurationZero)
		output[17] = float32(len(definition.Triggers)) / 10
		output[18] = boolFloat(string(snap.Segment) == "ongoing_effects")
		output[19] = boolFloat(snap.PriorityActorID != "")
		operations := append([]content.BattleOperation(nil), definition.Operations...)
		for _, trigger := range definition.Triggers {
			operations = append(operations, trigger.Operations...)
		}
		copy(output[20:27], v2OperationSummary(operations))
		if err := encodeV3OperationTypes(output, 28, operations, manifest); err != nil {
			return err
		}
	}
	return nil
}

func encodeV3Cards(values []float32, actor snapshot.Actor, catalog snapshot.ContentCatalog, manifest *observationManifestV3) error {
	if len(actor.Hand) > manifest.MaximumVisibleHandCards {
		return fmt.Errorf("viewer has %d visible hand cards; frozen v3 capacity is %d", len(actor.Hand), manifest.MaximumVisibleHandCards)
	}
	for slot, instanceID := range actor.Hand {
		output := v3Row(values, manifest.Layout.Cards, slot)
		definitionID := actor.CardInstances[instanceID].DefinitionID
		idIndex, err := v3VocabularyIndex(definitionID, manifest.CardVocabulary, false)
		if err != nil {
			return err
		}
		definition, exists := catalog.Cards[definitionID]
		if !exists {
			return fmt.Errorf("catalog omits visible card %q", definitionID)
		}
		output[0], output[1], output[2] = 1, 1, idIndex
		output[3] = v3Scaled(float32(definition.Cost.Energy), "energy", manifest)
		output[4], output[5] = boolFloat(definition.Type == "reaction"), boolFloat(definition.Type == "status_response")
		output[6] = boolFloat(definition.Type != "" && definition.Type != "reaction" && definition.Type != "status_response")
		output[7] = v3Scaled(float32(definition.Targeting.Minimum), "targets", manifest)
		output[8] = v3Scaled(float32(definition.Targeting.Maximum), "targets", manifest)
		output[9] = boolFloat(definition.Targeting.Selector == "self")
		output[10] = boolFloat(strings.Contains(definition.Targeting.Selector, "enemy"))
		output[11] = boolFloat(strings.Contains(definition.Targeting.Selector, "status"))
		copy(output[12:19], v2OperationSummary(definition.Operations))
		if err := encodeV3OperationTypes(output, 20, definition.Operations, manifest); err != nil {
			return err
		}
	}
	return nil
}

func encodeV3Candidate(output []float32, action command.Command, viewer, opponent string, actor snapshot.Actor, board []string, catalog snapshot.ContentCatalog, manifest *observationManifestV3) error {
	kind := string(action.Type)
	if indexOf(manifest.CommandVocabulary, kind) < 0 {
		return fmt.Errorf("authority emitted unsupported v3 command type %q", kind)
	}
	output[0] = 1
	oneHot(output, 1, manifest.CommandVocabulary, kind)
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(action.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode v3 candidate: %w", err)
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
	if _, ok := catalog.Abilities[ability]; !ok {
		ability = ""
	}
	output[12], output[13], output[14] = boolFloat(kind == "planning_pass" || kind == "pass"), boolFloat(kind == "planning_roll" || kind == "roll_dice"), boolFloat(kind == "planning_keep" || kind == "planning_reroll")
	output[15], output[16] = boolFloat(containsText(targets, viewer)), boolFloat(containsText(targets, opponent))
	output[17], output[18], output[19] = v3Scaled(float32(len(targets)), "targets", manifest), v3Scaled(float32(len(indices)), "dice", manifest), v3Scaled(float32(len(cards)), "hand", manifest)
	output[20], output[21] = boolFloat(ability != ""), boolFloat(len(cards) > 0)
	for _, index := range indices {
		if index < 0 || index >= manifest.MaximumDicePerActor {
			return fmt.Errorf("candidate die index %d exceeds frozen v3 capacity", index)
		}
		output[24+index] = 1
	}
	operations := []content.BattleOperation{}
	if ability != "" {
		if indexOf(board, ability) < 0 {
			return fmt.Errorf("candidate references ability outside active board: %q", ability)
		}
		idIndex, err := v3VocabularyIndex(ability, manifest.AbilityVocabulary, false)
		if err != nil {
			return err
		}
		output[32] = idIndex
		definition := effectiveV2Ability(ability, actor, catalog)
		output[33], output[34] = boolFloat(stringSet(actor.QualifiedAbilities)[ability]), boolFloat(ability == actor.SelectedAbility)
		output[35] = v3Scaled(float32(definition.Cost.Energy), "energy", manifest)
		for _, tier := range v2Tiers(definition) {
			operations = append(operations, tier.tier.Operations...)
		}
	}
	for _, instanceID := range cards {
		definitionID := actor.CardInstances[instanceID].DefinitionID
		if indexOf(manifest.CardVocabulary, definitionID) < 0 {
			return fmt.Errorf("candidate references missing viewer card definition %q", definitionID)
		}
		definition, ok := catalog.Cards[definitionID]
		if !ok {
			return fmt.Errorf("candidate references missing viewer card definition %q", definitionID)
		}
		operations = append(operations, definition.Operations...)
	}
	output[36] = boolFloat(len(cards) > 0)
	if status, ok := catalog.Statuses[choiceID]; ok {
		idIndex, err := v3VocabularyIndex(choiceID, manifest.StatusVocabulary, false)
		if err != nil {
			return err
		}
		output[37] = idIndex
		operations = append(operations, status.Operations...)
		for _, trigger := range status.Triggers {
			operations = append(operations, trigger.Operations...)
		}
	}
	copy(output[40:47], v2OperationSummary(operations))
	return encodeV3OperationTypes(output, 48, operations, manifest)
}

func encodeV3OperationTypes(output []float32, offset int, operations []content.BattleOperation, manifest *observationManifestV3) error {
	for _, operation := range operations {
		if operation.Type != "" {
			index := indexOf(manifest.OperationVocabulary, operation.Type)
			if index < 0 {
				if content.IsVenomIdentifier(operation.Type) {
					continue
				}
				return fmt.Errorf("unsupported operation in frozen v3 content: %q", operation.Type)
			}
			if offset+index >= len(output) {
				return fmt.Errorf("operation vocabulary exceeds entity feature width")
			}
			output[offset+index]++
		}
		for _, outcome := range operation.Outcomes {
			if err := encodeV3OperationTypes(output, offset, outcome.Operations, manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func auditV3Catalog(catalog snapshot.ContentCatalog, manifest *observationManifestV3) error {
	groups := []struct {
		name           string
		actual, frozen []string
	}{
		{"symbols", sortedKeys(catalog.Symbols), manifest.SymbolVocabulary},
		{"dice", sortedKeys(catalog.Dice), manifest.DieVocabulary},
		{"abilities", sortedKeys(catalog.Abilities), manifest.AbilityVocabulary},
		{"statuses", sortedKeys(catalog.Statuses), manifest.StatusVocabulary},
		{"cards", sortedKeys(catalog.Cards), manifest.CardVocabulary},
	}
	for _, group := range groups {
		for _, id := range group.actual {
			if indexOf(group.frozen, id) < 0 && !content.IsVenomIdentifier(id) {
				return fmt.Errorf("runtime catalog has content outside frozen v3 %s vocabulary: %q", group.name, id)
			}
		}
	}
	return nil
}

func v3Row(values []float32, entity entityRangeV3, index int) []float32 {
	start := entity.Offset + index*entity.Features
	return values[start : start+entity.Features]
}

func v3Scaled(value float32, name string, manifest *observationManifestV3) float32 {
	scale := manifest.Normalization[name]
	if scale < 1 {
		scale = 1
	}
	return value / scale
}

func v3VocabularyIndex(id string, vocabulary []string, allowEmpty bool) (float32, error) {
	if id == "" && allowEmpty {
		return 0, nil
	}
	index := indexOf(vocabulary, id)
	if index < 0 {
		if content.IsVenomIdentifier(id) {
			return 0, nil
		}
		return 0, fmt.Errorf("identifier %q is outside frozen v3 vocabulary", id)
	}
	return float32(index+1) / float32(max(len(vocabulary), 1)), nil
}
