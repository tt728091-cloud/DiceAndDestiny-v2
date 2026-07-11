package battle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"diceanddestiny/server/internal/save"
)

type FileParticipantAssembler struct {
	ContentRoot  string
	RunStateRoot string
}

func NewFileParticipantAssembler(contentRoot, runStateRoot string) *FileParticipantAssembler {
	return &FileParticipantAssembler{
		ContentRoot:  contentRoot,
		RunStateRoot: runStateRoot,
	}
}

func (assembler *FileParticipantAssembler) AssembleParticipants(
	participants []participant.Participant,
) (state.BattleSetup, error) {
	if assembler == nil || assembler.ContentRoot == "" || assembler.RunStateRoot == "" {
		return state.BattleSetup{}, fmt.Errorf("file participant assembler is not configured")
	}
	settledRoot := filepath.Join(assembler.ContentRoot, "battle_v1")
	if content.BattleLibraryExists(settledRoot) {
		library, settledErr := content.LoadBattleLibrary(settledRoot)
		if settledErr != nil {
			return state.BattleSetup{}, settledErr
		}
		allSettled := true
		for _, requested := range participants {
			if _, ok := library.Combatants[requested.DefinitionID]; !ok {
				allSettled = false
				break
			}
		}
		if allSettled {
			return assembleSettledParticipants(participants, library)
		}
	}

	library, err := content.LoadContentLibrary(assembler.ContentRoot)
	if err != nil {
		return state.BattleSetup{}, err
	}

	combined := state.BattleSetup{}
	combined.Content = runtimeContentCatalog(library)
	for _, requested := range participants {
		var participantSetup state.BattleSetup
		source := requested.Source
		if source == "" {
			if requested.Controller == state.ControllerHuman {
				source = participant.SourceRunPlayer
			} else if requested.Controller == state.ControllerAI {
				source = participant.SourceEnemyDefinition
			}
		}
		switch source {
		case participant.SourceRunPlayer:
			if requested.Controller != state.ControllerHuman {
				err = fmt.Errorf("run_player source requires human controller")
				break
			}
			participantSetup, err = assembler.loadPlayer(requested, library)
		case participant.SourceCharacterDefinition:
			if requested.Controller != state.ControllerHuman {
				err = fmt.Errorf("character_definition source requires human controller")
				break
			}
			participantSetup, err = assembler.loadCharacter(requested, library)
		case participant.SourceEnemyDefinition:
			if requested.Controller != state.ControllerAI {
				err = fmt.Errorf("enemy_definition source requires AI controller")
				break
			}
			participantSetup, err = assembler.loadEnemy(requested, library)
		default:
			err = fmt.Errorf("unsupported participant source %q", source)
		}
		if err != nil {
			return state.BattleSetup{}, err
		}
		if err := mergeBattleSetup(&combined, participantSetup); err != nil {
			return state.BattleSetup{}, err
		}
	}
	return combined, nil
}

func assembleSettledParticipants(participants []participant.Participant, library content.BattleLibrary) (state.BattleSetup, error) {
	catalog, err := json.Marshal(library)
	if err != nil {
		return state.BattleSetup{}, fmt.Errorf("marshal settled content catalog: %w", err)
	}
	setupResult := state.BattleSetup{SettledCatalog: catalog, SettledActors: make(map[string]state.SettledActorRuntime)}
	seenDice := map[string]bool{}
	for _, requested := range participants {
		definition := library.Combatants[requested.DefinitionID]
		wantController := state.ControllerHuman
		if definition.ControllerDefaults.Type == "ai" {
			wantController = state.ControllerAI
		}
		if requested.Controller != wantController {
			return state.BattleSetup{}, fmt.Errorf("combatant %q controller must be %s", definition.ID, wantController)
		}
		deck, instances := instantiateSettledDeck(requested.InstanceID, definition.Decklist)
		statuses := make([]state.StatusState, len(definition.StartingStatuses))
		for i, status := range definition.StartingStatuses {
			instanceID := status.InstanceID
			if instanceID == "" {
				instanceID = fmt.Sprintf("%s-status-%s-%02d", requested.InstanceID, status.DefinitionID, i+1)
			}
			statuses[i] = state.StatusState{InstanceID: instanceID, DefinitionID: status.DefinitionID, Stacks: status.Stacks}
		}
		abilities := append(append([]string(nil), definition.AbilityBoard.Offensive...), definition.AbilityBoard.Defensive...)
		setupResult.Actors = append(setupResult.Actors, state.ActorSetup{ID: requested.InstanceID, DefinitionID: definition.ID, ControllerType: wantController, Character: state.CharacterMetadata{ID: definition.ID, Name: definition.Name, Class: definition.Class}, Resources: state.ResourceState{StartingHandSize: definition.Resources.StartingHandSize, MaxHandSize: definition.Resources.HandLimit, StartingEnergyPoints: definition.Resources.StartingEnergy, EnergyPoints: definition.Resources.StartingEnergy}, Health: state.HealthMetadata{Model: "card_zones", MaxHealth: len(deck)}, Decklist: convertSettledDecklist(definition.Decklist), Deck: deck, DiceLoadout: convertSettledDiceLoadout(definition.DiceLoadout), AbilityIDs: abilities, Statuses: statuses, RollPreferences: state.RollPreferences{StatusEffects: state.RollMode(definition.RollPreferences.StatusEffects), Offensive: state.RollMode(definition.RollPreferences.Offensive)}})
		setupResult.SettledActors[requested.InstanceID] = state.SettledActorRuntime{IncomeCards: definition.Income.Cards, IncomeEnergy: definition.Income.Energy, HandLimit: definition.Resources.HandLimit, OffensiveAbilityIDs: append([]string(nil), definition.AbilityBoard.Offensive...), DefensiveAbilityIDs: append([]string(nil), definition.AbilityBoard.Defensive...), CardInstances: instances, MaxRolls: 3, UsedAbilities: map[string]int{}}
		for _, entry := range definition.DiceLoadout {
			if seenDice[entry.DiceID] {
				continue
			}
			seenDice[entry.DiceID] = true
			die := library.Dice[entry.DiceID]
			faces := make([]state.DiceFace, len(die.Faces))
			for i, face := range die.Faces {
				faces[i] = state.DiceFace{Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}
			}
			setupResult.DiceDefinitions = append(setupResult.DiceDefinitions, state.DiceDefinition{ID: die.ID, Name: die.Name, DieType: die.DieType, SideCount: die.SideCount, Faces: faces})
		}
	}
	return setupResult, nil
}

func instantiateSettledDeck(actorID string, decklist []content.DecklistEntry) ([]string, map[string]state.CardInstance) {
	var deck []string
	instances := map[string]state.CardInstance{}
	sequence := 1
	for _, entry := range decklist {
		for i := 0; i < entry.Count; i++ {
			id := fmt.Sprintf("%s-card-%03d", actorID, sequence)
			sequence++
			deck = append(deck, id)
			instances[id] = state.CardInstance{InstanceID: id, DefinitionID: entry.CardID}
		}
	}
	return deck, instances
}
func convertSettledDecklist(values []content.DecklistEntry) []state.DecklistEntry {
	result := make([]state.DecklistEntry, len(values))
	for i, value := range values {
		result[i] = state.DecklistEntry{CardID: value.CardID, Count: value.Count}
	}
	return result
}
func convertSettledDiceLoadout(values []content.DiceLoadoutEntry) []state.DiceLoadoutEntry {
	result := make([]state.DiceLoadoutEntry, len(values))
	for i, value := range values {
		result[i] = state.DiceLoadoutEntry{DiceID: value.DiceID, Count: value.Count}
	}
	return result
}

func (assembler *FileParticipantAssembler) loadPlayer(
	participant participant.Participant,
	library content.ContentLibrary,
) (state.BattleSetup, error) {
	path, err := definitionPath(assembler.RunStateRoot, participant.DefinitionID, ".json")
	if err != nil {
		return state.BattleSetup{}, err
	}
	player, err := save.LoadRunPlayerState(path)
	if err != nil {
		return state.BattleSetup{}, err
	}
	player.ActorID = participant.InstanceID
	if err := validateRunPlayerReferences(player, library); err != nil {
		return state.BattleSetup{}, err
	}

	definitions, err := diceDefinitionsForLoadout(player.DiceLoadout, library)
	if err != nil {
		return state.BattleSetup{}, err
	}
	return setup.BattleSetupFromRunPlayer(player, setup.WithDiceDefinitions(definitions))
}

func (assembler *FileParticipantAssembler) loadCharacter(
	participant participant.Participant,
	library content.ContentLibrary,
) (state.BattleSetup, error) {
	path, err := definitionPath(
		filepath.Join(assembler.ContentRoot, "characters"),
		participant.DefinitionID,
		".yaml",
	)
	if err != nil {
		return state.BattleSetup{}, err
	}
	sheet, err := content.LoadCharacterCombatSheetWithLibrary(path, library)
	if err != nil {
		return state.BattleSetup{}, err
	}
	sheet.ActorID = participant.InstanceID
	return setup.BattleSetupFromCharacterCombatSheet(sheet, library)
}

func (assembler *FileParticipantAssembler) loadEnemy(
	participant participant.Participant,
	library content.ContentLibrary,
) (state.BattleSetup, error) {
	path, err := definitionPath(
		filepath.Join(assembler.ContentRoot, "enemies"),
		participant.DefinitionID,
		".yaml",
	)
	if err != nil {
		return state.BattleSetup{}, err
	}
	definition, err := content.LoadEnemyDefinition(path, library)
	if err != nil {
		return state.BattleSetup{}, err
	}
	if definition.ID != participant.DefinitionID {
		return state.BattleSetup{}, fmt.Errorf(
			"enemy definition id %q does not match requested id %q",
			definition.ID,
			participant.DefinitionID,
		)
	}
	for _, status := range definition.Statuses {
		statusDefinition, ok := library.Statuses[status.DefinitionID]
		if !ok {
			return state.BattleSetup{}, fmt.Errorf("enemy referenced status %q was not found", status.DefinitionID)
		}
		if status.Stacks > statusDefinition.StackLimit {
			return state.BattleSetup{}, fmt.Errorf(
				"enemy status %q stacks %d exceed stack limit %d",
				status.DefinitionID,
				status.Stacks,
				statusDefinition.StackLimit,
			)
		}
	}
	return setup.BattleSetupFromEnemyDefinition(participant.InstanceID, definition, library)
}

func definitionPath(root, definitionID, extension string) (string, error) {
	if definitionID == "" {
		return "", fmt.Errorf("definition id is required")
	}
	if filepath.Base(definitionID) != definitionID ||
		strings.Contains(definitionID, "/") ||
		strings.Contains(definitionID, `\`) {
		return "", fmt.Errorf("invalid definition id %q", definitionID)
	}
	return filepath.Join(root, definitionID+extension), nil
}

func validateRunPlayerReferences(player setup.RunPlayerState, library content.ContentLibrary) error {
	for _, entry := range player.Decklist {
		if _, ok := library.Cards[entry.CardID]; !ok {
			return fmt.Errorf("run player referenced card %q was not found", entry.CardID)
		}
	}
	for _, zone := range [][]string{
		player.Cards.Deck,
		player.Cards.Hand,
		player.Cards.Discard,
		player.Cards.Removed,
	} {
		for _, cardID := range zone {
			if _, ok := library.Cards[cardID]; !ok {
				return fmt.Errorf("run player referenced card %q was not found", cardID)
			}
		}
	}
	for _, abilityID := range player.AbilityIDs {
		if _, ok := library.Abilities[abilityID]; !ok {
			return fmt.Errorf("run player referenced ability %q was not found", abilityID)
		}
	}
	for _, entry := range player.DiceLoadout {
		if _, ok := library.Dice[entry.DiceID]; !ok {
			return fmt.Errorf("run player referenced dice %q was not found", entry.DiceID)
		}
	}
	for _, status := range player.Statuses {
		definition, ok := library.Statuses[status.DefinitionID]
		if !ok {
			return fmt.Errorf("run player referenced status %q was not found", status.DefinitionID)
		}
		if status.Stacks > definition.StackLimit {
			return fmt.Errorf(
				"run player status %q stacks %d exceed stack limit %d",
				status.DefinitionID,
				status.Stacks,
				definition.StackLimit,
			)
		}
	}
	return nil
}

func diceDefinitionsForLoadout(
	loadout []state.DiceLoadoutEntry,
	library content.ContentLibrary,
) ([]state.DiceDefinition, error) {
	definitions := make([]state.DiceDefinition, 0, len(loadout))
	for _, entry := range loadout {
		die, ok := library.Dice[entry.DiceID]
		if !ok {
			return nil, fmt.Errorf("run player referenced dice %q was not found", entry.DiceID)
		}
		faces := make([]state.DiceFace, len(die.Faces))
		for i, face := range die.Faces {
			var symbols []string
			if face.Symbols != nil {
				symbols = append([]string{}, face.Symbols...)
			}
			faces[i] = state.DiceFace{
				Face:    face.Face,
				Value:   face.Value,
				Symbols: symbols,
			}
		}
		definitions = append(definitions, state.DiceDefinition{
			ID:        die.ID,
			Name:      die.Name,
			DieType:   die.DieType,
			SideCount: die.SideCount,
			Faces:     faces,
		})
	}
	return definitions, nil
}

func mergeBattleSetup(target *state.BattleSetup, source state.BattleSetup) error {
	target.Actors = append(target.Actors, source.Actors...)
	existing := make(map[string]state.DiceDefinition, len(target.DiceDefinitions))
	for _, definition := range target.DiceDefinitions {
		existing[definition.ID] = definition
	}
	for _, definition := range source.DiceDefinitions {
		if current, ok := existing[definition.ID]; ok {
			if !reflect.DeepEqual(current, definition) {
				return fmt.Errorf("conflicting dice definition %q", definition.ID)
			}
			continue
		}
		target.DiceDefinitions = append(target.DiceDefinitions, definition)
		existing[definition.ID] = definition
	}
	return nil
}

func runtimeContentCatalog(library content.ContentLibrary) state.ContentCatalog {
	catalog := state.ContentCatalog{
		Cards:     make(map[string]state.RuntimeContentDefinition, len(library.Cards)),
		Abilities: make(map[string]state.RuntimeContentDefinition, len(library.Abilities)),
		Statuses:  make(map[string]state.RuntimeStatusDefinition, len(library.Statuses)),
	}
	for id, card := range library.Cards {
		catalog.Cards[id] = state.RuntimeContentDefinition{
			ID:             id,
			Segments:       contentSegments(card.PhaseRestrictions),
			EnergyCost:     card.Cost.EnergyPoints,
			RequiresTarget: operationsRequireSelectedTarget(card.Operations),
			Operations:     operation.ClonePlans(card.Operations),
		}
	}
	for id, ability := range library.Abilities {
		catalog.Abilities[id] = state.RuntimeContentDefinition{
			ID:              id,
			Segments:        contentSegments(ability.PhaseRestrictions),
			EnergyCost:      ability.Cost.EnergyPoints,
			RequiresTarget:  ability.RequiresTarget,
			DiceRequirement: ability.DiceRequirement.Kind,
			Operations:      operation.ClonePlans(ability.Operations),
		}
	}
	for id, status := range library.Statuses {
		definition := state.RuntimeStatusDefinition{
			ID:                  id,
			StackLimit:          status.StackLimit,
			StackOverflowPolicy: status.StackOverflowPolicy,
			Triggers:            make([]state.RuntimeStatusTrigger, len(status.Triggers)),
		}
		for i, trigger := range status.Triggers {
			definition.Triggers[i] = state.RuntimeStatusTrigger{
				ID:         trigger.ID,
				Segment:    segment.Segment(trigger.Segment),
				Phase:      state.FlowPhase(trigger.Phase),
				Priority:   trigger.Priority,
				Operations: operation.ClonePlans(trigger.Operations),
			}
		}
		catalog.Statuses[id] = definition
	}
	return catalog
}

func operationsRequireSelectedTarget(values []operation.Plan) bool {
	for _, plan := range values {
		if plan.Target == operation.TargetSelectedTargets {
			return true
		}
		if operationsRequireSelectedTarget(plan.Operations) {
			return true
		}
		for _, outcome := range plan.Outcomes {
			if operationsRequireSelectedTarget(outcome.Operations) {
				return true
			}
		}
	}
	return false
}

func contentSegments(values []string) []segment.Segment {
	segments := make([]segment.Segment, len(values))
	for i, value := range values {
		segments[i] = segment.Segment(value)
	}
	return segments
}
