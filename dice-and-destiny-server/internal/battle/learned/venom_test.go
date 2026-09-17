package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
)

func TestVenomFullGamesAgainstPreservedModels(t *testing.T) {
	root := testServerRoot(t)
	models := []struct{ file, pin string }{
		{"blade-warden-seed-11-final-v1.json", ""},
		{"blade-warden-decision-quality-seed-22-v2.json", "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"},
		{"blade-warden-optimized-5m-seed-22-v3.json", "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"},
		{"blade-warden-global-champion-cp193-winner-health-v2.json", "cd3d7451e91d071274c874b017e3c7e73358862090fd71954e282e5bf505b814"},
		{"blade-warden-global-champion-cp38-winner-health-v2.json", "6aeb05e0657c8c221298c79780895bcc6aa527f1780c6c5eb66833293daf6693"},
		{"blade-warden-global-champion-cp480-v3.json", "96755c199f4d93261695d4928a0f95e00858d3a9d6a5c429e46911fbd0a3cac6"},
	}
	for _, model := range models {
		t.Run(model.file, func(t *testing.T) {
			s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: filepath.Join(root, "save/run_players"), ModelPath: filepath.Join(root, "../dice-and-destiny-client/models/learned", model.file), ModelSHA256: model.pin, InferenceTimeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			for seatN, seat := range []string{"seat-a", "seat-b"} {
				view, err := s.ResetCharacter(fmt.Sprintf("venom-%d", seatN), uint64(711+seatN), seat, seatN > 0, "venom")
				if err != nil {
					t.Fatal(err)
				}
				actors := object(object(view["snapshot"])["actors"])
				if object(actors[HumanAlias])["definition_id"] != "venom" || object(actors[ModelAlias])["definition_id"] != "blade_warden" {
					t.Fatalf("wrong characters: %v", actors)
				}
				for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal; step++ {
					if s.current.ActorID == s.modelSeat {
						if _, err = s.AdvanceModel(); err != nil {
							t.Fatalf("model step %d stage %s: %v", step, s.current.Result.Snapshot.Stage, err)
						}
						continue
					}
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatalf("no human actions at %s", s.current.Result.Snapshot.Stage)
					}
					action := venomTestAction(actions)
					alias := aliasValue(commandMap(t, action), map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias})
					encoded, _ := json.Marshal(alias)
					if _, err = s.SubmitHuman(string(encoded)); err != nil {
						t.Fatalf("human step %d: %s: %v", step, encoded, err)
					}
				}
				if !s.current.Terminal || s.current.TruncationReason != "" {
					t.Fatalf("did not complete: %+v", s.current.Metrics)
				}
				telemetry, _ := s.Telemetry()
				if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 || telemetry.FallbackCount != 0 {
					t.Fatalf("unclean game: %+v", telemetry)
				}
			}
		})
	}
}
func venomTestAction(actions []command.Command) command.Command {
	// Exercise cards and optional choices whenever available, then finish rolls
	// and select an attack. Keep/reroll candidates cannot trap this test in place.
	for _, kind := range []command.Type{command.TypePlanningCards, command.TypeCommitInteraction, command.TypePlanningRoll, command.TypePlanningAbility, command.TypeRollDice, command.TypePass, command.TypePlanningPass} {
		for _, action := range actions {
			if action.Type == kind {
				return action
			}
		}
	}
	return actions[0]
}
