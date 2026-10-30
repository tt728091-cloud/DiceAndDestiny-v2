package tacticsbattle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
)

// attacksUsed counts the abilities each battle's revealed attacks used.
func attacksUsed(t *testing.T, roots Roots, id string, into map[string]int) {
	t.Helper()
	checkpoint, err := repository.NewDisk(filepath.Join(roots.BattleRoot, "tactics")).Load(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range checkpoint.Events {
		if e.Type != event.TypeInteractionRevealed {
			continue
		}
		raw, _ := json.Marshal(e.Data["commitments"])
		var commitments map[string]struct {
			AbilityID string `json:"ability_id"`
		}
		_ = json.Unmarshal(raw, &commitments)
		for seat, c := range commitments {
			if c.AbilityID != "" && seat != HeroSeat {
				into[c.AbilityID]++
			}
		}
	}
}

// Every arena, both heroes and every enemy, alone and mixed, play to the
// end through the runner with the scripted enemies: no refused command,
// no stuck decision.
func TestSandboxBattlesFinishInEveryArena(t *testing.T) {
	roots := testRoots(t)
	mixes := [][]string{
		{"tactics_ribbon_eel"},
		{"tactics_brine_mask"},
		{"tactics_drowned_regent"},
		{"tactics_bell_diver", "tactics_brine_mask"},
		{"tactics_ribbon_eel", "tactics_bell_diver", "tactics_brine_mask", "tactics_drowned_regent"},
	}
	if testing.Short() {
		mixes = mixes[3:4]
	}
	results, used := map[string]int{}, map[string]int{}
	n := 0
	for _, arena := range []string{"open_field", "doorway", "pillar_hall", "swamp_crossing"} {
		for i, enemies := range mixes {
			hero := []string{"starter_movement", "adventurer_movement"}[i%2]
			id := fmt.Sprintf("tactics-%s-%d", arena, i)
			view, err := Start(roots, id, Setup{Hero: hero, Enemies: enemies, Arena: arena, Seed: uint64(100 + n)})
			n++
			if err != nil {
				t.Fatalf("%s: %v", id, err)
			}
			end, commands, slowest := playOut(t, roots, id, view, simpleHero(view))
			results[fmt.Sprint(end["battle_result"])]++
			attacksUsed(t, roots, id, used)
			t.Logf("%s %s vs %v: %v after %d commands (slowest %s)", arena, hero, enemies, end["battle_result"], commands, slowest)
		}
	}
	t.Logf("results %v; enemy attacks used %v", results, used)
}
