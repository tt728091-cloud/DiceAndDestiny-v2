package battle

import (
	"encoding/json"
	"fmt"
	"sort"

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

// WithCarriedWounds starts seats with some of their deck copies already in the
// removed pile: wounds carried over from earlier battles (campaign play). The
// copies still count toward max health, so a 12-card deck with 5 wounds starts
// at 7/12, and the opening draw never reaches them. Wrap it around
// WithDeckLoadouts so the instances exist.
func WithCarriedWounds(base ParticipantAssembler, wounds map[string][]loadout.Entry) ParticipantAssembler {
	return ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		setup, err := base.AssembleParticipants(participants)
		if err != nil || len(wounds) == 0 {
			return setup, err
		}
		seen := map[string]bool{}
		for i := range setup.Actors {
			actor := &setup.Actors[i]
			entries, ok := wounds[actor.ID]
			if !ok {
				continue
			}
			seen[actor.ID] = true
			instances := setup.SettledActors[actor.ID].CardInstances
			for _, entry := range entries {
				for n := 0; n < entry.Count; n++ {
					// Take the last matching copy, so the choice is deterministic.
					found := -1
					for j := len(actor.Deck) - 1; j >= 0; j-- {
						if instances[actor.Deck[j]].DefinitionID == entry.CardID {
							found = j
							break
						}
					}
					if found < 0 {
						return state.BattleSetup{}, fmt.Errorf("wounded %s is not in %s's deck", entry.CardID, actor.ID)
					}
					actor.Removed = append(actor.Removed, actor.Deck[found])
					actor.Deck = append(actor.Deck[:found:found], actor.Deck[found+1:]...)
				}
			}
		}
		for seat := range wounds {
			if !seen[seat] {
				return state.BattleSetup{}, fmt.Errorf("unknown wound seat %q", seat)
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

// PresetPiles starts a seat from piles chosen outside the battle (the
// Expedition map): the opening hand exactly as it is, the draw pile, Spent as
// discard, wounds as removed, the current energy, the shared hand limit and
// start statuses. Piles are lists of card definition IDs; every copy must be
// in the seat's deck loadout. Statuses are applied to any seat, with or
// without piles (Fortified, Conqueror and buffs on enemies).
type PresetPiles struct {
	Hand    []string `json:"hand,omitempty"`
	Deck    []string `json:"deck,omitempty"`
	Discard []string `json:"discard,omitempty"`
	Removed []string `json:"removed,omitempty"`
	// Piles reports whether the four pile lists replace the deck loadout's
	// shuffled deck (true) or only the statuses apply (false).
	Piles bool `json:"piles,omitempty"`
	// Energy is the starting energy, clamped to the seat's cap.
	Energy int `json:"energy,omitempty"`
	// HandLimit replaces the combatant's hand limit when positive.
	HandLimit int `json:"hand_limit,omitempty"`
	// Statuses are applied at setup, like a combatant's starting_statuses.
	Statuses []content.StartingStatus `json:"statuses,omitempty"`
}

// WithPresetPiles assigns a seat's card instances into the preset piles, sets
// its energy and hand limit, skips the opening draw (StartingHandSize 0) and
// adds the start statuses. Wrap it around WithDeckLoadouts so the instances
// exist. The battle then plays by its usual rules.
func WithPresetPiles(base ParticipantAssembler, presets map[string]PresetPiles) ParticipantAssembler {
	return ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		setup, err := base.AssembleParticipants(participants)
		if err != nil || len(presets) == 0 {
			return setup, err
		}
		var catalog content.BattleLibrary
		if err = json.Unmarshal(setup.SettledCatalog, &catalog); err != nil {
			return state.BattleSetup{}, err
		}
		seen := map[string]bool{}
		for i := range setup.Actors {
			actor := &setup.Actors[i]
			preset, ok := presets[actor.ID]
			if !ok {
				continue
			}
			seen[actor.ID] = true
			runtime := setup.SettledActors[actor.ID]
			if preset.Piles {
				// Every owned copy is an instance in the deck; deal them into
				// the preset piles by definition, in instance order.
				unused := append([]string(nil), actor.Deck...)
				unused = append(unused, actor.Removed...)
				sort.Strings(unused)
				take := func(definition string) (string, error) {
					for j, id := range unused {
						if runtime.CardInstances[id].DefinitionID == definition {
							unused = append(unused[:j:j], unused[j+1:]...)
							return id, nil
						}
					}
					return "", fmt.Errorf("preset pile card %s is not in %s's deck", definition, actor.ID)
				}
				piles := []struct {
					ids  []string
					dest *[]string
				}{{preset.Hand, &actor.Hand}, {preset.Deck, &actor.Deck}, {preset.Discard, &actor.Discard}, {preset.Removed, &actor.Removed}}
				assigned := map[*[]string][]string{}
				for _, pile := range piles {
					for _, definition := range pile.ids {
						id, err := take(definition)
						if err != nil {
							return state.BattleSetup{}, err
						}
						assigned[pile.dest] = append(assigned[pile.dest], id)
					}
				}
				if len(unused) != 0 {
					return state.BattleSetup{}, fmt.Errorf("preset piles leave %d of %s's cards unplaced", len(unused), actor.ID)
				}
				for _, pile := range piles {
					*pile.dest = append([]string{}, assigned[pile.dest]...)
				}
				energy := preset.Energy
				if energy < 0 {
					energy = 0
				}
				if cap := actor.Resources.MaxEnergyPoints; cap > 0 && energy > cap {
					energy = cap
				}
				actor.Resources.EnergyPoints = energy
				actor.Resources.StartingEnergyPoints = energy
				// The map hand is the opening hand exactly as it is: no draw.
				actor.Resources.StartingHandSize = 0
			}
			if preset.HandLimit > 0 {
				actor.Resources.MaxHandSize = preset.HandLimit
				runtime.HandLimit = preset.HandLimit
			}
			for n, status := range preset.Statuses {
				def, ok := catalog.Statuses[status.DefinitionID]
				if !ok || status.Stacks < 1 || (!def.Stacking.Uncapped && status.Stacks > def.Stacking.StackLimit) {
					return state.BattleSetup{}, fmt.Errorf("preset status %q with %d stacks is invalid for %s", status.DefinitionID, status.Stacks, actor.ID)
				}
				actor.Statuses = append(actor.Statuses, state.StatusState{InstanceID: fmt.Sprintf("%s-status-%s-p%02d", actor.ID, status.DefinitionID, n+1), DefinitionID: status.DefinitionID, Stacks: status.Stacks})
			}
			setup.SettledActors[actor.ID] = runtime
		}
		for seat := range presets {
			if !seen[seat] {
				return state.BattleSetup{}, fmt.Errorf("unknown preset seat %q", seat)
			}
		}
		return setup, nil
	})
}
