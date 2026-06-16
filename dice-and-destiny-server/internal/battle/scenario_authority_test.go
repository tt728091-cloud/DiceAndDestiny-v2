package battle

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/scenario"
	"diceanddestiny/server/internal/battle/state"
)

func TestScenarioAuthorityDisabled(t *testing.T) {
	authority := NewScenarioAuthority(ScenarioAuthorityConfig{
		Gameplay: NewAuthority(engine.NewEngine(), repository.NewInMemory(), &recordingAssembler{}),
	})
	result := authority.HandleCommand(command.Command{
		BattleID: "scenario-control",
		ActorID:  "player",
		Type:     command.TypeListScenarios,
		Payload:  json.RawMessage(`{}`),
	})
	if result.Accepted || result.Error != "scenario tooling is disabled" {
		t.Fatalf("disabled result = %#v", result)
	}
}

func TestScenarioAuthorityCreationPersistenceOpenRestartAndRouting(t *testing.T) {
	root := participantTestServerRoot(t)
	normalRepo := repository.NewInMemory()
	scenarioRepo := repository.NewInMemory()
	router := repository.Router{Normal: normalRepo, Scenario: scenarioRepo}
	assembler := NewFileParticipantAssembler(
		filepath.Join(root, "content"),
		filepath.Join(root, "save", "run_players"),
	)
	gameplay := NewAuthority(engine.NewEngine(), router, assembler)
	idGenerator := func(string) (string, error) {
		return "scenario-round-2-poisoned-player-fixed", nil
	}
	config := ScenarioAuthorityConfig{
		BuildEnabled:   true,
		RuntimeEnabled: true,
		Catalog:        scenario.Catalog{Root: filepath.Join(root, "scenarios")},
		Builder:        scenario.Builder{Assembler: assembler},
		Engine:         engine.NewEngine(),
		Repository:     router,
		Gameplay:       gameplay,
		IDGenerator:    idGenerator,
	}
	authority := NewScenarioAuthority(config)

	listed := authority.HandleCommand(command.Command{
		BattleID: "scenario-control",
		ActorID:  "player",
		Type:     command.TypeListScenarios,
		Payload:  json.RawMessage(`{}`),
	})
	if !listed.Accepted || listed.Data == nil {
		t.Fatalf("list_scenarios = %#v", listed)
	}
	validated := authority.HandleCommand(command.Command{
		BattleID: "scenario-control",
		ActorID:  "player",
		Type:     command.TypeValidateScenario,
		Payload:  json.RawMessage(`{"scenario_id":"round-2-poisoned-player"}`),
	})
	if !validated.Accepted {
		t.Fatalf("validate_scenario = %#v", validated)
	}
	started := authority.HandleCommand(command.Command{
		BattleID: "scenario-control",
		ActorID:  "player",
		Type:     command.TypeStartScenario,
		Payload:  json.RawMessage(`{"scenario_id":"round-2-poisoned-player"}`),
	})
	if !started.Accepted || started.Snapshot == nil {
		t.Fatalf("start_scenario = %#v", started)
	}
	battleID := started.Snapshot.BattleID
	if !strings.HasPrefix(battleID, "scenario-") ||
		started.Snapshot.Origin == nil ||
		started.Snapshot.Origin.ScenarioID != "round-2-poisoned-player" {
		t.Fatalf("scenario snapshot = %#v", started.Snapshot)
	}
	if started.Snapshot.Actors["goblin-1"].Hand != nil {
		t.Fatal("viewer snapshot exposed enemy hand")
	}
	for _, battleEvent := range started.Events {
		if battleEvent.ActorID == "goblin-1" && battleEvent.Commitment != nil &&
			!battleEvent.Commitment.Revealed {
			t.Fatal("viewer events exposed an enemy hidden commitment")
		}
	}
	if _, err := normalRepo.Load(battleID); !reflect.DeepEqual(err, repository.ErrBattleNotFound) {
		t.Fatalf("normal repository load error = %v", err)
	}
	before, err := scenarioRepo.Load(battleID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Battle.Origin.Kind != state.BattleOriginScenario ||
		before.Battle.Random.Cursor == 0 {
		t.Fatalf("checkpoint metadata = %#v / %#v", before.Battle.Origin, before.Battle.Random)
	}

	restartedGameplay := NewAuthority(engine.NewEngine(), router, ParticipantAssemblerFunc(
		func([]Participant) (state.BattleSetup, error) {
			t.Fatal("open_battle reassembled participants")
			return state.BattleSetup{}, nil
		},
	))
	opened := restartedGameplay.HandleCommand(command.Command{
		BattleID: battleID,
		ActorID:  "player",
		Type:     command.TypeOpenBattle,
		Payload:  json.RawMessage(`{}`),
	})
	if !opened.Accepted || len(opened.Events) != 0 || opened.Snapshot == nil {
		t.Fatalf("open_battle = %#v", opened)
	}
	after, err := scenarioRepo.Load(battleID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("open_battle mutated checkpoint")
	}

	leftScenarioRepo := repository.NewInMemory()
	rightScenarioRepo := repository.NewInMemory()
	if err := leftScenarioRepo.Create(before); err != nil {
		t.Fatal(err)
	}
	if err := rightScenarioRepo.Create(before); err != nil {
		t.Fatal(err)
	}
	pending := before.Battle.Flow.PendingInput["player"]
	rollPayload, _ := json.Marshal(command.RollDicePayload{PendingInputID: pending.ID})
	roll := command.Command{
		BattleID: battleID,
		ActorID:  "player",
		Type:     command.TypeRollDice,
		Payload:  rollPayload,
	}
	for _, routed := range []repository.Router{
		{Normal: repository.NewInMemory(), Scenario: leftScenarioRepo},
		{Normal: repository.NewInMemory(), Scenario: rightScenarioRepo},
	} {
		restarted := NewAuthority(engine.NewEngine(), routed, ParticipantAssemblerFunc(
			func([]Participant) (state.BattleSetup, error) {
				return state.BattleSetup{}, errors.New("assembler must not run")
			},
		))
		if result := restarted.HandleCommand(roll); !result.Accepted {
			t.Fatalf("restarted deterministic roll = %#v", result)
		}
	}
	left, err := leftScenarioRepo.Load(battleID)
	if err != nil {
		t.Fatal(err)
	}
	right, err := rightScenarioRepo.Load(battleID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.Battle.Actors["player"].Dice, right.Battle.Actors["player"].Dice) ||
		left.Battle.Random != right.Battle.Random ||
		left.Battle.Random.Cursor <= before.Battle.Random.Cursor {
		t.Fatal("persisted random state did not reproduce after restart")
	}

	terminal := before
	terminal.Battle.Status = state.BattleVictory
	terminal.BattleID = "scenario-terminal-fixed"
	terminal.Battle.ID = terminal.BattleID
	terminal.Events = nil
	terminal.NextEventSequence = 1
	terminalCheckpoint, err := repository.NewCheckpoint(terminal.Battle)
	if err != nil {
		t.Fatal(err)
	}
	if err := scenarioRepo.Create(terminalCheckpoint); err != nil {
		t.Fatal(err)
	}
	terminalResult := restartedGameplay.HandleCommand(command.Command{
		BattleID: terminal.BattleID,
		ActorID:  "player",
		Type:     command.TypeOpenBattle,
		Payload:  json.RawMessage(`{}`),
	})
	if !terminalResult.Accepted || terminalResult.Status != engine.ProgressBattleComplete ||
		terminalResult.BattleResult != state.BattleVictory {
		t.Fatalf("terminal open = %#v", terminalResult)
	}

	duplicate := authority.HandleCommand(command.Command{
		BattleID: "scenario-control",
		ActorID:  "player",
		Type:     command.TypeStartScenario,
		Payload:  json.RawMessage(`{"scenario_id":"round-2-poisoned-player"}`),
	})
	if duplicate.Accepted || duplicate.Error != "battle already exists" {
		t.Fatalf("duplicate start = %#v", duplicate)
	}
}
