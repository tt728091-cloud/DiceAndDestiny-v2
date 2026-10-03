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

func TestAdventurerFullGamesAgainstPreservedModels(t *testing.T) {
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
				view, err := s.ResetCharacter(fmt.Sprintf("adventurer-%d", seatN), uint64(711+seatN), seat, seatN > 0, "adventurer")
				if err != nil {
					t.Fatal(err)
				}
				actors := object(object(view["snapshot"])["actors"])
				if object(actors[HumanAlias])["definition_id"] != "adventurer" || object(actors[ModelAlias])["definition_id"] != "blade_warden" {
					t.Fatalf("wrong characters: %v", actors)
				}
				cardPlays := map[int]int{}
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
					action := adventurerTestAction(actions, cardPlays[s.current.Result.Snapshot.Round])
					if action.Type == command.TypePlanningCards {
						cardPlays[s.current.Result.Snapshot.Round]++
					}
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

func TestAdventurerMinionEncounters(t *testing.T) {
	for _, count := range []int{1, 2} {
		s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(testServerRoot(t), "content"), RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: count})
		if err != nil {
			t.Fatal(err)
		}
		for _, seat := range []string{"seat-a", "seat-b"} {
			for seed := uint64(1); seed <= 3; seed++ {
				if _, err = s.ResetCharacter(fmt.Sprintf("adventurer-minion-%d-%s-%d", count, seat, seed), seed, seat, true, "adventurer"); err != nil {
					t.Fatal(err)
				}
				cardPlays := map[int]int{}
				for step := 0; step < 1200 && !s.current.Terminal; step++ {
					if s.isModelSeat(s.current.ActorID) {
						_, err = s.AdvanceModel()
					} else {
						actions := s.current.Result.LegalActions
						if len(actions) == 0 {
							t.Fatal("missing human choices")
						}
						cmd := adventurerTestAction(actions, cardPlays[s.current.Result.Snapshot.Round])
						if cmd.Type == command.TypePlanningCards {
							cardPlays[s.current.Result.Snapshot.Round]++
						}
						encoded, _ := json.Marshal(aliasValue(commandMap(t, cmd), s.aliases(false)))
						_, err = s.SubmitHuman(string(encoded))
					}
					if err != nil {
						t.Fatalf("count %d seat %s seed %d stage %s: %v", count, seat, seed, s.current.Result.Snapshot.Stage, err)
					}
				}
				if !s.current.Terminal || s.current.TruncationReason != "" {
					t.Fatal("incomplete minion battle")
				}
				stats, _ := s.Telemetry()
				if stats.AuthorityRejects+stats.InvalidActions+stats.FallbackCount > 0 {
					t.Fatalf("unclean battle: %+v", stats)
				}
			}
		}
	}
}

// A player may keep cycling a very small surviving deck. This smoke player
// deliberately attacks after three planning cards instead of cycling forever.
func adventurerTestAction(actions []command.Command, cardsPlayed int) command.Command {
	if cardsPlayed < 3 {
		return venomTestAction(actions)
	}
	var choices []command.Command
	for _, action := range actions {
		if action.Type != command.TypePlanningCards {
			choices = append(choices, action)
		}
	}
	return venomTestAction(choices)
}
