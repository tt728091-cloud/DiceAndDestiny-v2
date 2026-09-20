package learned

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestTwoBrineMasksFullBattles(t *testing.T) {
	s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(testServerRoot(t), "content"), RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	doubleDefense, continued, secondTarget, skipped := false, false, false, false
	wins := map[string]int{}
	for seed := uint64(1); seed <= 30; seed++ {
		for _, seat := range []string{"seat-a", "seat-b"} {
			view, err := s.ResetCharacter(fmt.Sprintf("pair-%d-%s", seed, seat), seed, seat, true, "venom")
			if err != nil {
				t.Fatal(err)
			}
			if len(object(object(view["snapshot"])["actors"])) != 3 {
				t.Fatal("not three actors")
			}
			defenses := map[int]map[string]bool{}
			for step := 0; step < 1200 && !s.current.Terminal; step++ {
				snap := s.current.Result.Snapshot
				for id, a := range snap.Actors {
					if id != s.humanSeat && len(a.Hand) > 0 && s.current.ActorID == s.humanSeat {
						t.Fatal("enemy hand exposed")
					}
					if a.DefeatState == state.ActorDefeated && id != s.humanSeat {
						continued = true
						if s.current.ActorID == id {
							t.Fatal("dead enemy received turn")
						}
					}
				}
				if s.isModelSeat(s.current.ActorID) {
					idx, _, err := s.policy.Select(s.current)
					if err != nil {
						t.Fatal(err)
					}
					action := s.current.Result.LegalActions[idx]
					if action.Type == command.TypePlanningAbility && snap.Stage == "planning" {
						var p command.PlanningAbilityPayload
						_ = json.Unmarshal(action.Payload, &p)
						if len(p.TargetIDs) != 1 || p.TargetIDs[0] != s.humanSeat {
							t.Fatalf("friendly fire: %+v", p)
						}
					}
					_, err = s.AdvanceModel()
					if err != nil {
						t.Fatalf("seed %d step %d stage %s: %v", seed, step, snap.Stage, err)
					}
				} else {
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatal("no actions")
					}
					action := venomTestAction(actions)
					// Use every defensive ability on both sources. Alternate first/second target and skip one source.
					for _, a := range actions {
						if a.Type == command.TypePlanningAbility {
							var p command.PlanningAbilityPayload
							_ = json.Unmarshal(a.Payload, &p)
							if snap.Stage == "defense_selection" || snap.Stage == "defense_select" {
								if p.AbilityID == "shedskin" {
									action = a
									break
								}
							} else if seed%2 == 0 && len(p.TargetIDs) > 0 && p.TargetIDs[0] == "seat-c" {
								action = a
								secondTarget = true
							}
						}
					}
					if snap.Stage == "defense_selection" || snap.Stage == "defense_select" {
						if seed%5 == 0 && len(snap.DefenseHistory) == 0 {
							for _, a := range actions {
								if a.Type == command.TypePlanningPass {
									action = a
									skipped = true
									break
								}
							}
						}
						if action.Type == command.TypePlanningAbility {
							var p command.PlanningAbilityPayload
							_ = json.Unmarshal(action.Payload, &p)
							if defenses[snap.Round] == nil {
								defenses[snap.Round] = map[string]bool{}
							}
							if defenses[snap.Round][p.TargetIDs[0]] {
								t.Fatal("defended same attack twice")
							}
							defenses[snap.Round][p.TargetIDs[0]] = true
							if len(defenses[snap.Round]) == 2 {
								doubleDefense = true
							}
						}
					}
					encoded, _ := json.Marshal(aliasValue(commandMap(t, action), s.aliases(false)))
					_, err = s.SubmitHuman(string(encoded))
					if err != nil {
						t.Fatalf("seed %d seat %s step %d stage %s: %v", seed, seat, step, snap.Stage, err)
					}
				}
			}
			if !s.current.Terminal || s.current.TruncationReason != "" {
				t.Fatalf("unfinished seed %d: %+v", seed, s.current.Metrics)
			}
			tel, _ := s.Telemetry()
			if tel.AuthorityRejects != 0 || len(tel.Errors) > 0 {
				t.Fatalf("errors: %+v", tel)
			}
			wins[tel.Result]++
			if seed == 1 {
				record := *s.current.Replay
				replay, err := s.environment.Replay(record)
				if err != nil || replay.Winner != record.Winner {
					t.Fatalf("replay: %v", err)
				}
			}
		}
	}
	if !doubleDefense || !continued || !secondTarget || !skipped {
		t.Fatalf("coverage defense=%v continuation=%v target=%v skip=%v", doubleDefense, continued, secondTarget, skipped)
	}
	t.Logf("60 completed pair battles: %v; separate defenses, defeat continuation, second target, skips and replay verified", wins)
}
