package battle

import (
	"encoding/json"
	"fmt"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// WithDeckLoadouts pins owned deck copies into participant setup, before opening
// draws. Templates and other seats (even the same character) stay unchanged.
func WithDeckLoadouts(base ParticipantAssembler, decks map[string][]loadout.Entry) ParticipantAssembler {
	return ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		setup, err := base.AssembleParticipants(participants)
		if err != nil || len(decks) == 0 {
			return setup, err
		}
		var catalog content.BattleLibrary
		if err = json.Unmarshal(setup.SettledCatalog, &catalog); err != nil {
			return state.BattleSetup{}, err
		}
		seen := map[string]bool{}
		for i := range setup.Actors {
			actor := &setup.Actors[i]
			entries, ok := decks[actor.ID]
			if !ok {
				continue
			}
			seen[actor.ID] = true
			entries, err = loadout.Validate(entries, catalog.Cards)
			if err != nil {
				return state.BattleSetup{}, err
			}
			converted := make([]content.DecklistEntry, len(entries))
			for j, e := range entries {
				converted[j] = content.DecklistEntry{CardID: e.CardID, Count: e.Count}
			}
			deck, instances := instantiateSettledDeck(actor.ID, converted)
			actor.Decklist = convertSettledDecklist(converted)
			actor.Deck = deck
			actor.Health.MaxHealth = len(deck)
			runtime := setup.SettledActors[actor.ID]
			runtime.CardInstances = instances
			setup.SettledActors[actor.ID] = runtime
		}
		for seat := range decks {
			if !seen[seat] {
				return state.BattleSetup{}, fmt.Errorf("unknown loadout seat %q", seat)
			}
		}
		return setup, nil
	})
}

// Ability slots are pinned per participant, just like the starting deck.
func WithAbilityLoadouts(base ParticipantAssembler, boards map[string]content.AbilityBoard) ParticipantAssembler {
	return ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		setup, err := base.AssembleParticipants(participants)
		if err != nil || len(boards) == 0 {
			return setup, err
		}
		var catalog content.BattleLibrary
		if err = json.Unmarshal(setup.SettledCatalog, &catalog); err != nil {
			return state.BattleSetup{}, err
		}
		seen := map[string]bool{}
		for i := range setup.Actors {
			actor := &setup.Actors[i]
			board, ok := boards[actor.ID]
			if !ok {
				continue
			}
			seen[actor.ID] = true
			if err = loadout.ValidateAbilities(board, catalog); err != nil {
				return state.BattleSetup{}, err
			}
			actor.AbilityIDs = append(append([]string(nil), board.Offensive...), board.Defensive...)
			runtime := setup.SettledActors[actor.ID]
			runtime.OffensiveAbilityIDs = append([]string(nil), board.Offensive...)
			runtime.DefensiveAbilityIDs = append([]string(nil), board.Defensive...)
			setup.SettledActors[actor.ID] = runtime
		}
		for seat := range boards {
			if !seen[seat] {
				return state.BattleSetup{}, fmt.Errorf("unknown ability loadout seat %q", seat)
			}
		}
		return setup, nil
	})
}
