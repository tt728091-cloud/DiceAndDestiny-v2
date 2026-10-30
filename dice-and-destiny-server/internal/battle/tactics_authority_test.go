package battle

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/tactics"
)

func tacticsStart(t *testing.T, repo repository.Repository, id string, unified, simultaneous bool, enemies int) (*Authority, engine.Result) {
	t.Helper()
	assembler, _, err := NewTacticsParticipantAssembler(filepath.Join("..", "..", "content"), "")
	if err != nil {
		t.Fatal(err)
	}
	pack, err := tactics.LoadPack(filepath.Join("..", "..", "content", TacticsPack))
	if err != nil {
		t.Fatal(err)
	}
	arena := pack.Arenas["pillar_hall"]
	placement := map[string]tactics.Square{"hero": arena.PlayerStart}
	seats := []command.ParticipantDescriptor{{InstanceID: "hero", DefinitionID: "adventurer_movement", TeamID: "heroes"}}
	for i := 0; i < enemies; i++ {
		id := []string{"e1", "e2", "e3", "e4"}[i]
		placement[id] = arena.EnemyStarts[i]
		seats = append(seats, command.ParticipantDescriptor{InstanceID: id, DefinitionID: "tactics_brine_mask", TeamID: "monsters"})
	}
	authority := NewAuthority(engine.NewEngine(), repo, assembler)
	seed := uint64(3)
	payload, _ := json.Marshal(command.StartBattlePayload{UnifiedDefense: unified, SimultaneousPlay: simultaneous, Seats: seats, Seed: &seed, Tactics: &command.TacticsSetup{Arena: arena, Rules: pack.Rules, Placement: placement}})
	return authority, authority.HandleCommand(command.Command{BattleID: id, ActorID: "hero", Type: command.TypeStartBattle, Payload: payload})
}

// §11.15: the tactics ruleset is refused without unified defense and
// simultaneous play.
func TestAuthorityRefusesTacticsWithoutTheOtherTwoRules(t *testing.T) {
	for _, flags := range [][2]bool{{false, true}, {true, false}, {false, false}} {
		_, result := tacticsStart(t, repository.NewInMemory(), "tactics-refused", flags[0], flags[1], 1)
		if result.Accepted || !strings.Contains(result.Error, "unified defense and simultaneous play") {
			t.Fatalf("unified=%v simultaneous=%v: %+v", flags[0], flags[1], result.Error)
		}
	}
	_, result := tacticsStart(t, repository.NewInMemory(), "tactics-ok", true, true, 4)
	if !result.Accepted || result.Snapshot.Tactics == nil {
		t.Fatalf("a hero and four enemies must start: %s", result.Error)
	}
	if len(result.Snapshot.Tactics.Creatures) != 5 {
		t.Fatalf("creatures = %d", len(result.Snapshot.Tactics.Creatures))
	}
}

func TestFiveSeatsNeedTheTacticsRule(t *testing.T) {
	authority := NewAuthority(engine.NewEngine(), repository.NewInMemory(), NewFileParticipantAssembler(filepath.Join("..", "..", "content"), filepath.Join("..", "..", "save", "run_players")))
	var seats []command.ParticipantDescriptor
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		seats = append(seats, command.ParticipantDescriptor{InstanceID: id, DefinitionID: "drowned_oracle_brine_mask"})
	}
	payload, _ := json.Marshal(command.StartBattlePayload{UnifiedDefense: true, SimultaneousPlay: true, Seats: seats})
	result := authority.HandleCommand(command.Command{BattleID: "five", ActorID: "a", Type: command.TypeStartBattle, Payload: payload})
	if result.Accepted || !strings.Contains(result.Error, "two to 4 seats") {
		t.Fatalf("five seats without tactics: %+v", result.Error)
	}
}

func TestTacticsBattlesPersistOnDisk(t *testing.T) {
	root := t.TempDir()
	_, result := tacticsStart(t, repository.NewDisk(root), "tactics-disk", true, true, 2)
	if !result.Accepted {
		t.Fatal(result.Error)
	}
	reopened := NewAuthority(engine.NewEngine(), repository.NewDisk(root), ParticipantAssemblerFunc(nil))
	stored, err := reopened.InspectBattleState("tactics-disk")
	if err != nil {
		t.Fatal(err)
	}
	if !engine.TacticsBattle(&stored) || stored.Settled.TacticsRuntime.Arena.ID != "pillar_hall" || len(stored.Settled.TacticsRuntime.Positions) != 3 {
		t.Fatalf("stored tactics state = %+v", stored.Settled.TacticsRuntime)
	}
}
