package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
)

// Complete encounters under the pinned simultaneous rule, with the minion
// policy on the other seats. Venom and Curse keep the unified flow.
func TestSimultaneousPlayCompleteEncounters(t *testing.T) {
	for _, character := range []string{"adventurer", "starter", "blade_warden", "venom", "curse"} {
		for _, count := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s-%d", character, count), func(t *testing.T) {
				s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(testServerRoot(t), "content"), RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: count})
				if err != nil {
					t.Fatal(err)
				}
				pinned := character != "venom" && character != "curse"
				for seed := uint64(41); seed <= 43; seed++ {
					if _, err = s.ResetCharacter(fmt.Sprintf("simultaneous-%s-%d-%d", character, count, seed), seed, "seat-a", false, character, true, true); err != nil {
						t.Fatal(err)
					}
					steps := map[string]bool{}
					for step := 0; step < 1500 && !s.current.Terminal; step++ {
						snap := s.current.Result.Snapshot
						if snap.SimultaneousPlay != pinned {
							t.Fatalf("simultaneous rule pinned = %v, want %v", snap.SimultaneousPlay, pinned)
						}
						if !snap.UnifiedDefense {
							t.Fatal("unified rule lost from snapshot")
						}
						if snap.Segment == segment.DamageResolution {
							t.Fatal("separate damage segment opened")
						}
						steps[snap.SimultaneousStep] = true
						if s.isModelSeat(s.current.ActorID) {
							_, err = s.AdvanceModel()
						} else {
							actions := s.current.Result.LegalActions
							if len(actions) == 0 {
								t.Fatalf("no human legal actions at %s/%s", snap.Stage, snap.SimultaneousStep)
							}
							cmd := adventurerTestAction(actions, 3)
							encoded, _ := json.Marshal(aliasValue(commandMap(t, cmd), s.aliases(false)))
							_, err = s.SubmitHuman(string(encoded))
						}
						if err != nil {
							t.Fatalf("seed %d step %d stage %s/%s: %v", seed, step, s.current.Result.Snapshot.Stage, s.current.Result.Snapshot.SimultaneousStep, err)
						}
					}
					if !s.current.Terminal || s.current.TruncationReason != "" {
						t.Fatalf("incomplete encounter: %+v", s.current.Metrics)
					}
					if pinned {
						for _, want := range []string{"offense_before", "offense_rolling", "defense_before", "defense_after"} {
							if !steps[want] {
								t.Fatalf("never reached %s: %v", want, steps)
							}
						}
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
