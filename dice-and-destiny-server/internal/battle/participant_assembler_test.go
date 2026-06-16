package battle

import (
	"path/filepath"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestFileParticipantAssemblerIntegratesCompleteSetupWithDurableLifecycle(t *testing.T) {
	root := participantTestServerRoot(t)
	repo := repository.NewInMemory()
	authority := NewAuthority(
		engine.NewEngine(),
		repo,
		NewFileParticipantAssembler(
			filepath.Join(root, "content"),
			filepath.Join(root, "save", "run_players"),
		),
	)

	started := decodeAuthorityResult(t, authority.HandleCommandJSON(`{
		"battle_id": "phase-2-integration",
		"actor_id": "player",
		"type": "start_battle",
		"payload": {
			"player": {
				"instance_id": "player",
				"definition_id": "current_run_player"
			},
			"enemies": [
				{"instance_id": "goblin-1", "definition_id": "mock_goblin"},
				{"instance_id": "goblin-2", "definition_id": "mock_goblin"}
			]
		}
	}`))
	if !started.Accepted || started.Status != engine.ProgressWaitingForInput {
		t.Fatalf("start result = %#v, want accepted wait", started)
	}

	playerView := started.Snapshot.Actors["player"]
	if playerView.Character == nil || playerView.Character.ID != "Mock Paladin" ||
		len(playerView.AbilityIDs) != 4 || len(playerView.DiceLoadout) != 1 ||
		playerView.RollPreferences == nil {
		t.Fatalf("player snapshot = %#v, want complete viewer-owned loadout", playerView)
	}
	enemyView := started.Snapshot.Actors["goblin-1"]
	if enemyView.Character == nil || enemyView.Character.ID != "mock_goblin" ||
		len(enemyView.AbilityIDs) != 0 || len(enemyView.DiceLoadout) != 0 ||
		enemyView.AbilityCount != 1 || enemyView.DiceCount != 2 {
		t.Fatalf("enemy snapshot = %#v, want public counts without private loadout IDs", enemyView)
	}

	checkpoint, err := repo.Load("phase-2-integration")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	player := checkpoint.Battle.Actors["player"]
	if player.DefinitionID != "current_run_player" || player.Controller != state.ControllerHuman ||
		player.Resources.EnergyPoints != 3 || len(player.Decklist) != 3 ||
		len(player.AbilityIDs) != 4 || len(player.Statuses) != 1 || len(player.Tokens) != 1 {
		t.Fatalf("authoritative player state = %#v, want complete run state", player)
	}
	goblin1 := checkpoint.Battle.Actors["goblin-1"]
	goblin2 := checkpoint.Battle.Actors["goblin-2"]
	if goblin1.DefinitionID != "mock_goblin" || goblin1.Controller != state.ControllerAI ||
		len(goblin1.Cards.Deck) != 6 || len(goblin1.Statuses) != 1 {
		t.Fatalf("authoritative enemy state = %#v, want fresh definition state", goblin1)
	}
	if goblin1.Statuses[0].InstanceID == goblin2.Statuses[0].InstanceID {
		t.Fatalf("enemy status instance IDs collide: %q", goblin1.Statuses[0].InstanceID)
	}

	goblin1.Cards.Deck[0] = "mutated"
	goblin1.Statuses[0].Stacks = 99
	if goblin2.Cards.Deck[0] == "mutated" || goblin2.Statuses[0].Stacks == 99 {
		t.Fatal("same-definition enemy instances share mutable state")
	}
}

func TestDefaultExportedAuthorityUsesProductionParticipantAssembler(t *testing.T) {
	t.Setenv("DICE_AND_DESTINY_BATTLE_STATE_ROOT", t.TempDir())
	previous := defaultAuthority
	defaultAuthority = newDefaultAuthority()
	t.Cleanup(func() {
		defaultAuthority = previous
	})

	result := decodeAuthorityResult(t, HandleCommand(`{
		"battle_id": "phase-2-default-authority",
		"actor_id": "player",
		"type": "start_battle",
		"payload": {
			"player": {
				"instance_id": "player",
				"definition_id": "current_run_player"
			},
			"enemies": [
				{"instance_id": "goblin-1", "definition_id": "mock_goblin"}
			]
		}
	}`))
	if !result.Accepted || result.Snapshot == nil || len(result.Snapshot.Actors) != 2 {
		t.Fatalf("default authority result = %#v, want configured production setup", result)
	}
}

func participantTestServerRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
