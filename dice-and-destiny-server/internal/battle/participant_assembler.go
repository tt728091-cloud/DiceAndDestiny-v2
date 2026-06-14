package battle

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

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
	participants []Participant,
) (state.BattleSetup, error) {
	if assembler == nil || assembler.ContentRoot == "" || assembler.RunStateRoot == "" {
		return state.BattleSetup{}, fmt.Errorf("file participant assembler is not configured")
	}

	library, err := content.LoadContentLibrary(assembler.ContentRoot)
	if err != nil {
		return state.BattleSetup{}, err
	}

	combined := state.BattleSetup{}
	for _, participant := range participants {
		var participantSetup state.BattleSetup
		switch participant.Controller {
		case state.ControllerHuman:
			participantSetup, err = assembler.loadPlayer(participant, library)
		case state.ControllerAI:
			participantSetup, err = assembler.loadEnemy(participant, library)
		default:
			err = fmt.Errorf("unsupported participant controller %q", participant.Controller)
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

func (assembler *FileParticipantAssembler) loadPlayer(
	participant Participant,
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

func (assembler *FileParticipantAssembler) loadEnemy(
	participant Participant,
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
