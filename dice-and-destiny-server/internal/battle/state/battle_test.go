package state_test

import (
	"diceanddestiny/server/internal/battle/command"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestNewBattleInitializesEmptyBattleState(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	if battle.ID != "battle-1" {
		t.Fatalf("battle ID = %q, want %q", battle.ID, "battle-1")
	}
	if battle.Status != state.BattleActive {
		t.Fatalf("battle status = %q, want active", battle.Status)
	}

	wantSegment := segment.NewManager().InitialState()
	if battle.Segment != wantSegment {
		t.Fatalf("battle segment = %#v, want %#v", battle.Segment, wantSegment)
	}

	if battle.Segment.Current != segment.OngoingEffects {
		t.Fatalf("battle segment current = %q, want %q", battle.Segment.Current, segment.OngoingEffects)
	}

	if battle.Segment.Round != 1 {
		t.Fatalf("battle segment round = %d, want 1", battle.Segment.Round)
	}

	if len(battle.Actors) != 0 {
		t.Fatalf("actors = %#v, want no hardcoded actors", battle.Actors)
	}
	if len(battle.DiceDefinitions) != 0 {
		t.Fatalf("dice definitions = %#v, want no hardcoded definitions", battle.DiceDefinitions)
	}
}

func TestBattleCloneDeepCopiesResolutionAndInteractionState(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-resolution", state.BattleSetup{
		Actors: []state.ActorSetup{{ID: "player"}},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	value := 4
	battle.ActiveResolutionID = "resolution-1"
	battle.Resolutions["resolution-1"] = state.ResolutionState{
		ID:             "resolution-1",
		ActiveWindowID: "window-1",
		Batch: state.ProposalBatch{
			ID: "batch-1",
			Proposals: []state.Proposal{{
				ID: "proposal-1",
				Data: state.ProposalData{
					Selection: &state.SelectionData{OptionIDs: []string{"option-1"}},
				},
			}},
		},
		Windows: map[string]state.InteractionWindow{
			"window-1": {
				ID:              "window-1",
				EligibleActors:  []string{"player"},
				RequiredActors:  []string{"player"},
				AllowedCommands: []command.Type{command.TypeCommitInteraction},
				ActorProgress: map[string]state.InteractionActorStatus{
					"player": state.InteractionActorCommitted,
				},
				Commitments: map[string]state.InteractionCommitment{
					"player": {
						ID:      "commitment-1",
						ActorID: "player",
						Data: state.InteractionCommitmentData{
							CardIDs: []string{"secret-card"},
							Value:   &value,
						},
					},
				},
			},
		},
		ReactionPolicy: &state.ReactionWindowPolicy{
			EligibleActors:  []string{"player"},
			RequiredActors:  []string{"player"},
			AllowedCommands: []command.Type{command.TypePass},
		},
		SuspendedPendingInput: map[string]state.PendingInput{
			"player": {
				AllowedCommands: []command.Type{command.TypeCommitInteraction},
			},
		},
	}

	cloned := battle.Clone()
	resolution := cloned.Resolutions["resolution-1"]
	resolution.Batch.Proposals[0].Data.Selection.OptionIDs[0] = "mutated"
	window := resolution.Windows["window-1"]
	window.EligibleActors[0] = "mutated"
	window.AllowedCommands[0] = command.TypePass
	commitment := window.Commitments["player"]
	commitment.Data.CardIDs[0] = "mutated"
	*commitment.Data.Value = 99
	window.Commitments["player"] = commitment
	resolution.Windows["window-1"] = window
	resolution.ReactionPolicy.EligibleActors[0] = "mutated"
	pending := resolution.SuspendedPendingInput["player"]
	pending.AllowedCommands[0] = command.TypePass
	resolution.SuspendedPendingInput["player"] = pending
	cloned.Resolutions["resolution-1"] = resolution

	original := battle.Resolutions["resolution-1"]
	if original.Batch.Proposals[0].Data.Selection.OptionIDs[0] != "option-1" ||
		original.Windows["window-1"].EligibleActors[0] != "player" ||
		original.Windows["window-1"].AllowedCommands[0] != command.TypeCommitInteraction ||
		original.Windows["window-1"].Commitments["player"].Data.CardIDs[0] != "secret-card" ||
		*original.Windows["window-1"].Commitments["player"].Data.Value != 4 ||
		original.ReactionPolicy.EligibleActors[0] != "player" ||
		original.SuspendedPendingInput["player"].AllowedCommands[0] != command.TypeCommitInteraction {
		t.Fatalf("clone mutation leaked into original resolution: %#v", original)
	}
}

func TestNewBattleFromSetupPersistsDefinitionAndController(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "goblin-1",
				DefinitionID:   "goblin",
				ControllerType: state.ControllerAI,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	actor := battle.Actors["goblin-1"]
	if actor.DefinitionID != "goblin" || actor.Controller != state.ControllerAI {
		t.Fatalf("actor = %#v, want goblin AI", actor)
	}
}

func TestNewBattleFromSetupRejectsDuplicateActorID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "goblin-1"},
			{ID: "goblin-1"},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}

func TestNewBattleRejectsEmptyBattleID(t *testing.T) {
	battle, err := state.NewBattle("")
	if err == nil {
		t.Fatalf("NewBattle() succeeded with battle %#v", battle)
	}
}

func TestNewBattleFromSetupInitializesActorCardState(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      "player",
				Deck:    []string{"strike", "guard"},
				Hand:    []string{"opener"},
				Discard: []string{"spent"},
				Removed: []string{"lost"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	want := state.CardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, want) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, want)
	}
}

func TestNewBattleFromSetupCopiesCardZoneInputs(t *testing.T) {
	deck := []string{"strike", "guard"}
	hand := []string{"opener"}
	discard := []string{"spent"}
	removed := []string{"lost"}
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      "player",
				Deck:    deck,
				Hand:    hand,
				Discard: discard,
				Removed: removed,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	deck[0] = "mutated"
	hand[0] = "mutated"
	discard[0] = "mutated"
	removed[0] = "mutated"

	want := state.CardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, want) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, want)
	}
}

func TestNewBattleFromSetupRejectsMissingActorID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{Deck: []string{"strike"}},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}

func TestNewBattleFromSetupRejectsEmptyBattleID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike"}},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}

func TestNewBattleFromSetupCopiesCompleteActorCombatState(t *testing.T) {
	setupActor := state.ActorSetup{
		ID:        "player",
		Character: state.CharacterMetadata{ID: "hero", Name: "Hero", Class: "paladin"},
		Resources: state.ResourceState{MaxEnergyPoints: 10, EnergyPoints: 3},
		Health:    state.HealthMetadata{Model: "card_zones", MaxHealth: 2},
		Decklist:  []state.DecklistEntry{{CardID: "strike", Count: 2}},
		Deck:      []string{"strike"},
		Hand:      []string{"strike"},
		DiceLoadout: []state.DiceLoadoutEntry{
			{DiceID: "d6", Count: 2},
		},
		AbilityIDs: []string{"smite"},
		Statuses: []state.StatusState{
			{InstanceID: "injury-1", DefinitionID: "injury", Stacks: 1},
		},
		Tokens:          []state.TokenState{{ID: "blessing", Value: 2}},
		RollPreferences: state.RollPreferences{StatusEffects: state.RollModeAutomatic},
	}
	battle, err := state.NewBattleFromSetup("battle-full", state.BattleSetup{
		Actors: []state.ActorSetup{setupActor},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	setupActor.Decklist[0].Count = 99
	setupActor.Deck[0] = "mutated"
	setupActor.DiceLoadout[0].Count = 99
	setupActor.AbilityIDs[0] = "mutated"
	setupActor.Statuses[0].Stacks = 99
	setupActor.Tokens[0].Value = 99

	actor := battle.Actors["player"]
	if actor.Decklist[0].Count != 2 || actor.Cards.Deck[0] != "strike" ||
		actor.DiceLoadout[0].Count != 2 || actor.AbilityIDs[0] != "smite" ||
		actor.Statuses[0].Stacks != 1 || actor.Tokens[0].Value != 2 {
		t.Fatalf("battle actor aliased setup input: %#v", actor)
	}
	if actor.EnergyPoints != 3 || actor.Resources.EnergyPoints != 3 {
		t.Fatalf("energy state = %#v / %d, want synchronized value 3", actor.Resources, actor.EnergyPoints)
	}
}
