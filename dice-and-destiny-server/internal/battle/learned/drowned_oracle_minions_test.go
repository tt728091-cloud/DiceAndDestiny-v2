package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
)

// Bell Diver and Ribbon Eel reuse the single-ability controller with a set of
// kept faces and decks of blank health cards that are never played.
func TestDrownedOracleBlankCardMinionsFullBattles(t *testing.T) {
	root := testServerRoot(t)
	for _, minion := range []struct {
		id, attack, card string
		kept             []int
		health           int
	}{
		{"drowned_oracle_bell_diver", "sunken_toll", "ballast_stone", []int{5, 6}, 18},
		{"drowned_oracle_ribbon_eel", "coiling_snare", "shed_ribbon", []int{2, 4, 6}, 15},
	} {
		t.Run(minion.id, func(t *testing.T) {
			s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: t.TempDir(), OpponentDefinition: minion.id})
			if err != nil {
				t.Fatal(err)
			}
			wins := map[string]int{}
			totalRounds, hits, misses := 0, 0, 0
			for seed := uint64(1); seed <= 40; seed++ {
				for _, seat := range []string{"seat-a", "seat-b"} {
					view, err := s.ResetCharacter(fmt.Sprintf("%s-%d-%s", minion.id, seed, seat), seed, seat, seed > 1, "venom")
					if err != nil {
						t.Fatal(err)
					}
					actor := object(object(object(view["snapshot"])["actors"])[ModelAlias])
					if actor["definition_id"] != minion.id {
						t.Fatal("wrong opponent")
					}
					if fmt.Sprint(actor["current_health"]) != fmt.Sprint(minion.health) {
						t.Fatalf("starting health %v, want %d", actor["current_health"], minion.health)
					}
					for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal && s.current.TruncationReason == ""; step++ {
						if s.current.ActorID != s.modelSeat {
							action := venomTestAction(s.current.Result.LegalActions)
							alias := aliasValue(commandMap(t, action), map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias})
							encoded, _ := json.Marshal(alias)
							if _, err = s.SubmitHuman(string(encoded)); err != nil {
								t.Fatalf("seed %d seat %s human: %v", seed, seat, err)
							}
							continue
						}
						before := s.current.Result.Snapshot
						own := before.Actors[s.modelSeat]
						for _, action := range s.current.Result.LegalActions {
							if action.Type == command.TypePlanningCards {
								t.Fatalf("blank card offered as playable: %s", action.Payload)
							}
						}
						for _, card := range own.CardInstances {
							if card.DefinitionID != minion.card {
								t.Fatalf("unexpected card %q", card.DefinitionID)
							}
						}
						idx, _, err := s.policy.Select(s.current)
						if err != nil {
							t.Fatal(err)
						}
						action := s.current.Result.LegalActions[idx]
						switch action.Type {
						case command.TypePlanningReroll:
							var payload command.PlanningRerollPayload
							_ = json.Unmarshal(action.Payload, &payload)
							expected := []int{}
							for _, die := range own.Dice.Dice {
								if !slices.Contains(minion.kept, die.Face) {
									expected = append(expected, die.Index)
								}
							}
							if !slices.Equal(payload.RerollIndices, expected) {
								t.Fatalf("incorrect keep decision for %v: %+v", own.Dice.Dice, payload)
							}
						case command.TypePlanningAbility:
							hits++
						case command.TypePlanningPass:
							if own.Dice != nil && !slices.Contains(own.QualifiedAbilities, minion.attack) {
								misses++
							}
						}
						if _, err = s.AdvanceModel(); err != nil {
							t.Fatalf("seed %d seat %s step %d stage %s: %v", seed, seat, step, before.Stage, err)
						}
					}
					telemetry, lifetime := s.Telemetry()
					if !s.current.Terminal || telemetry.TruncationReason != "" || telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 || len(telemetry.Errors) > 0 || lifetime.ModelLoadCount != 0 {
						t.Fatalf("unclean game: %+v %+v", telemetry, lifetime)
					}
					wins[telemetry.Result]++
					totalRounds += s.current.Metrics.Rounds
				}
			}
			if hits == 0 {
				t.Fatal("minion never attacked")
			}
			t.Logf("80 Venom games: %v; average rounds %.2f; %d ability choices, %d misses", wins, float64(totalRounds)/80, hits, misses)
		})
	}
}
