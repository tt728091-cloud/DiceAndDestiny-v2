package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestSingleAbilityKeepsEveryTargetFace(t *testing.T) {
	p, err := loadSingleAbilityPolicy(filepath.Join(testServerRoot(t), "content"), "drowned_oracle_brine_mask")
	if err != nil {
		t.Fatal(err)
	}
	// Exhaust every 5d6 roll, including no targets and all targets, at each budget.
	for code := 0; code < 7776; code++ {
		dice := make([]state.RolledDie, 5)
		reroll := []int{}
		n := code
		for i := range dice {
			face := n%6 + 1
			n /= 6
			dice[i] = state.RolledDie{Index: i, Face: face}
			if face != 3 {
				reroll = append(reroll, i)
			}
		}
		for _, remaining := range []int{0, 1, 2} {
			tr := mlsim.Transition{ActorID: "seat-a"}
			tr.Result.Snapshot = &snapshot.Battle{}
			tr.Result.Snapshot.Actors = map[string]snapshot.Actor{"seat-a": {Dice: &snapshot.DiceRollState{Dice: dice, RollsUsed: 3 - remaining, MaxRolls: 3}}}
			pass, _ := json.Marshal(command.PlanningPassPayload{})
			tr.Result.LegalActions = []command.Command{{Type: command.TypePlanningPass, Payload: pass}}
			if remaining > 0 && len(reroll) > 0 {
				payload, _ := json.Marshal(command.PlanningRerollPayload{RerollIndices: reroll})
				tr.Result.LegalActions = append(tr.Result.LegalActions, command.Command{Type: command.TypePlanningReroll, Payload: payload})
			}
			choice, _, err := p.Select(tr)
			if err != nil {
				t.Fatal(err)
			}
			expected := 0
			if remaining > 0 && len(reroll) > 0 {
				expected = 1
			}
			if choice != expected {
				t.Fatalf("roll %v remaining %d chose %d", dice, remaining, choice)
			}
		}
	}
}

func TestBrineMaskFullBattles(t *testing.T) {
	root := testServerRoot(t)
	s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	wins := map[string]int{}
	totalRounds := 0
	sawMiss, sawSurge := false, false
	for seed := uint64(1); seed <= 100; seed++ {
		for _, seat := range []string{"seat-a", "seat-b"} {
			view, err := s.ResetCharacter(fmt.Sprintf("brine-%d-%s", seed, seat), seed, seat, seed > 1, "venom")
			if err != nil {
				t.Fatal(err)
			}
			actors := object(object(view["snapshot"])["actors"])
			if object(actors[ModelAlias])["definition_id"] != "drowned_oracle_brine_mask" {
				t.Fatal("wrong opponent")
			}
			cardPlays := map[int]int{}
			for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal && s.current.TruncationReason == ""; step++ {
				if s.current.ActorID == s.modelSeat {
					before := s.current.Result.Snapshot
					own := before.Actors[s.modelSeat]
					for _, mod := range own.AbilityModifiers {
						if mod.ExpiresAfterRound != before.Round {
							t.Fatalf("stale bonus leaked into round %d: %+v", before.Round, mod)
						}
					}
					idx, _, err := s.policy.Select(s.current)
					if err != nil {
						t.Fatal(err)
					}
					action := s.current.Result.LegalActions[idx]
					if action.Type == command.TypePlanningCards {
						cardPlays[before.Round]++
						sawSurge = true
						if cardPlays[before.Round] > 1 {
							t.Fatal("double surge")
						}
					}
					if action.Type == command.TypePlanningPass && own.Dice != nil && !slices.Contains(own.QualifiedAbilities, "brine_lash") {
						sawMiss = true
					}
					if action.Type == command.TypePlanningReroll {
						var payload command.PlanningRerollPayload
						_ = json.Unmarshal(action.Payload, &payload)
						expected := []int{}
						for _, die := range own.Dice.Dice {
							if die.Face != 3 {
								expected = append(expected, die.Index)
							}
						}
						if !slices.Equal(payload.RerollIndices, expected) {
							t.Fatalf("incorrect keep decision: %+v", payload)
						}
					}
					if _, err = s.AdvanceModel(); err != nil {
						t.Fatalf("seed %d seat %s step %d stage %s: %v", seed, seat, step, before.Stage, err)
					}
				} else {
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatal("no human actions")
					}
					action := venomTestAction(actions)
					// Alternate proactive card-heavy and ordinary attack-focused play.
					if seed%2 == 0 {
						for _, a := range actions {
							if a.Type == command.TypePlanningAbility {
								action = a
								break
							}
						}
					}
					alias := aliasValue(commandMap(t, action), map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias})
					encoded, _ := json.Marshal(alias)
					if _, err = s.SubmitHuman(string(encoded)); err != nil {
						t.Fatalf("seed %d seat %s human: %v", seed, seat, err)
					}
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
	if !sawMiss || !sawSurge {
		t.Fatalf("missing coverage miss=%v surge=%v", sawMiss, sawSurge)
	}
	t.Logf("200 completed Venom games: %v; average rounds %.2f; misses and automatic cards exercised", wins, float64(totalRounds)/200)
}

func TestMinionPackPreservesFrozenOpponentCatalog(t *testing.T) {
	env, err := mlsim.New(mlsim.Config{ContentRoot: filepath.Join(testServerRoot(t), "content"), RunStateRoot: t.TempDir(), IncludeContentCatalog: true})
	if err != nil {
		t.Fatal(err)
	}
	var baseline string
	for i, enemy := range []string{"blade_warden", "drowned_oracle_brine_mask", "blade_warden"} {
		_, err := env.Reset(mlsim.ResetRequest{Seed: 1, BattleID: fmt.Sprintf("catalog-%d", i), SeatDefinitions: map[string]string{"seat-a": "venom", "seat-b": enemy}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := env.Observe("seat-a")
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result.Snapshot.ContentCatalog)
		if i == 0 {
			baseline = string(encoded)
		}
		_, hasMinion := result.Snapshot.ContentCatalog.Abilities["brine_lash"]
		if hasMinion != (i == 1) {
			t.Fatal("minion content leaked into a frozen opponent matchup")
		}
		if i == 2 && string(encoded) != baseline {
			t.Fatal("cached learned catalog changed after a minion battle")
		}
	}
}

func TestSingleAbilityDiscardsExcessCardsWithoutPlayingReactions(t *testing.T) {
	p, err := loadSingleAbilityPolicy(filepath.Join(testServerRoot(t), "content"), "drowned_oracle_brine_mask")
	if err != nil {
		t.Fatal(err)
	}
	tr := mlsim.Transition{ActorID: "seat-a"}
	tr.Result.Snapshot = &snapshot.Battle{Stage: "discard_to_hand_limit", Actors: map[string]snapshot.Actor{"seat-a": {Hand: []string{"a", "b", "c", "d"}, MaxHandSize: 3}}}
	payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: command.InteractionCommitmentData{CardIDs: []string{"a"}}})
	tr.Result.LegalActions = []command.Command{{Type: command.TypeCommitInteraction, Payload: payload}}
	choice, _, err := p.Select(tr)
	if err != nil || choice != 0 {
		t.Fatalf("mandatory discard: %d %v", choice, err)
	}
	// The same command type must not cause optional card reactions to be played.
	tr.Result.Snapshot.Stage = "damage_reaction"
	tr.Result.LegalActions = append(tr.Result.LegalActions, command.Command{Type: command.TypePass})
	choice, _, err = p.Select(tr)
	if err != nil || choice != 1 {
		t.Fatalf("optional reaction should pass: %d %v", choice, err)
	}
}
