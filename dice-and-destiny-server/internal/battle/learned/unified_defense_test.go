package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
)

func TestUnifiedDefenseCompleteEncounters(t *testing.T) {
	for _, character := range []string{"adventurer", "blade_warden", "venom", "curse"} {
		for _, count := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s-%d", character, count), func(t *testing.T) {
				s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(testServerRoot(t), "content"), RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: count})
				if err != nil {
					t.Fatal(err)
				}
				for seed := uint64(41); seed <= 42; seed++ {
					if _, err = s.ResetCharacter(fmt.Sprintf("unified-%s-%d-%d", character, count, seed), seed, "seat-a", false, character, true); err != nil {
						t.Fatal(err)
					}
					seenHub := false
					for step := 0; step < 1200 && !s.current.Terminal; step++ {
						snap := s.current.Result.Snapshot
						if !snap.UnifiedDefense {
							t.Fatal("rule lost from snapshot")
						}
						if snap.Segment == segment.DamageResolution {
							t.Fatal("separate damage segment opened")
						}
						if snap.Stage == "defense_selection" {
							seenHub = true
							if snap.SettledDamage == nil {
								t.Fatal("no early threatened cards")
							}
						}
						if s.isModelSeat(s.current.ActorID) {
							_, err = s.AdvanceModel()
						} else {
							actions := s.current.Result.LegalActions
							if len(actions) == 0 {
								t.Fatal("no human legal actions")
							}
							cmd := adventurerTestAction(actions, 3)
							// Applying a rolled defense continues the hub instead of ending it.
							if snap.Stage == "defense_reaction" {
								for _, a := range actions {
									if a.Type == command.TypePlanningPass {
										cmd = a
									}
								}
							}
							encoded, _ := json.Marshal(aliasValue(commandMap(t, cmd), s.aliases(false)))
							_, err = s.SubmitHuman(string(encoded))
						}
						if err != nil {
							t.Fatalf("seed %d step %d stage %s: %v", seed, step, snap.Stage, err)
						}
					}
					if !seenHub || !s.current.Terminal || s.current.TruncationReason != "" {
						t.Fatalf("incomplete encounter: %+v", s.current.Metrics)
					}
					stats, _ := s.Telemetry()
					if stats.AuthorityRejects+stats.InvalidActions+stats.FallbackCount > 0 {
						t.Fatalf("unclean: %+v", stats)
					}
				}
			})
		}
	}
}

func TestUnifiedDefenseWithFrozenPolicies(t *testing.T) {
	for _, file := range []string{"blade-warden-seed-11-final-v1.json", "blade-warden-decision-quality-seed-22-v2.json"} {
		t.Run(file, func(t *testing.T) {
			root := testServerRoot(t)
			pin := ""
			if file == "blade-warden-decision-quality-seed-22-v2.json" {
				pin = "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
			}
			s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: t.TempDir(), ModelPath: filepath.Join(root, "../dice-and-destiny-client/models/learned", file), ModelSHA256: pin})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ResetCharacter("unified-policy", 113, "seat-a", false, "adventurer", true); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 1200 && !s.current.Terminal; step++ {
				if s.isModelSeat(s.current.ActorID) {
					_, err = s.AdvanceModel()
				} else {
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatal("no legal action")
					}
					cmd := adventurerTestAction(actions, 3)
					if s.current.Result.Snapshot.Stage == "defense_reaction" {
						for _, a := range actions {
							if a.Type == command.TypePlanningPass {
								cmd = a
							}
						}
					}
					encoded, _ := json.Marshal(aliasValue(commandMap(t, cmd), s.aliases(false)))
					_, err = s.SubmitHuman(string(encoded))
				}
				if err != nil {
					t.Fatalf("stage %s: %v", s.current.Result.Snapshot.Stage, err)
				}
			}
			if !s.current.Terminal || s.current.TruncationReason != "" {
				t.Fatal("frozen policy failed to finish")
			}
			stats, _ := s.Telemetry()
			if stats.AuthorityRejects+stats.InvalidActions+stats.FallbackCount > 0 {
				t.Fatalf("unclean: %+v", stats)
			}
		})
	}
}
