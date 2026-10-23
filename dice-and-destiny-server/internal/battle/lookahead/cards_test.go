package lookahead_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/mlsim"
)

// attacks counts the Starter's offensive ability hits.
func attacks(sources map[string]mlsim.SourceDamage) int {
	n := 0
	for _, id := range []string{"adventurer_strike", "guarded_strike", "measured_strike", "adventurer_small_straight", "adventurer_large_straight", "decisive_blow"} {
		n += sources[id].Hits
	}
	return n
}

func finalDamage(sources map[string]mlsim.SourceDamage) int {
	total := 0
	for _, d := range sources {
		total += d.Final
	}
	return total
}

// A card filter never plays the cards it blocks, and plays the others; the
// lab's damage record holds every commit, so it is at least the health each
// side lost, and each side's resolved offensive turns are counted.
func TestCardFilterAndDamageRecord(t *testing.T) {
	runner, err := lookahead.NewRunner(lookahead.Matchup{ContentRoot: filepath.Join(serverRoot(t), "content"), AuthoredRoot: authoredCopy(t), RunStateRoot: t.TempDir(),
		Hero: "starter", Opponent: "starter", Unified: true, HeroPlansFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	all := func(string) bool { return true }
	played := map[string]int{}
	for seed := uint64(1); seed <= 6; seed++ {
		// Kept from every card, the player also plans its dice without
		// them: exactly like a player whose solver ignores dice cards.
		skipping := lookahead.NewGreedyPolicy()
		skipping.SkipCards = all
		record, err := runner.Play(seed, &lookahead.CardFilter{Inner: skipping, Label: "no cards"}, lookahead.NewGreedyPolicy())
		if err != nil {
			t.Fatal(err)
		}
		if len(record.CardsPlayed) > 0 {
			t.Fatalf("seed %d: blocked every card, played %v", seed, record.CardsPlayed)
		}
		blind := lookahead.NewGreedyPolicy()
		blind.DiceCards = false
		same, err := runner.Play(seed, &lookahead.CardFilter{Inner: blind, Label: "no cards"}, lookahead.NewGreedyPolicy())
		if err != nil {
			t.Fatal(err)
		}
		if a, b := fmt.Sprint(record.Actions, record.Abilities, record.Dealt), fmt.Sprint(same.Actions, same.Abilities, same.Dealt); a != b {
			t.Fatalf("seed %d: skipping every card %s, ignoring dice cards %s", seed, a, b)
		}
		noNudge := &lookahead.CardFilter{Inner: lookahead.NewGreedyPolicy(), Label: "no nudge", Blocked: func(id string) bool { return id == "nudge" }}
		other, err := runner.Play(seed, noNudge, lookahead.NewGreedyPolicy())
		if err != nil {
			t.Fatal(err)
		}
		if other.CardsPlayed["nudge"] > 0 {
			t.Fatalf("seed %d: blocked Nudge, played %v", seed, other.CardsPlayed)
		}
		for id, n := range other.CardsPlayed {
			played[id] += n
		}
		for _, r := range []lookahead.BattleRecord{record, other} {
			if r.HeroTurns == 0 || r.FoeTurns == 0 || r.HeroTurns > r.Rounds || r.FoeTurns > r.Rounds {
				t.Fatalf("seed %d: %d hero and %d foe turns in %d rounds", seed, r.HeroTurns, r.FoeTurns, r.Rounds)
			}
			// Every attack belongs to a counted turn, and most turns attack.
			if hits := attacks(r.Dealt); hits > r.HeroTurns || 2*hits < r.HeroTurns {
				t.Fatalf("seed %d: %d attacks in %d hero turns", seed, hits, r.HeroTurns)
			}
			if dealt, lost := finalDamage(r.Dealt), r.FoeMax-r.FoeHealth; dealt < lost || dealt == 0 {
				t.Fatalf("seed %d: recorded %d damage dealt, the foe lost %d health", seed, dealt, lost)
			}
			if taken, lost := finalDamage(r.Taken), r.HeroMax-r.HeroHealth; taken < lost {
				t.Fatalf("seed %d: recorded %d damage taken, the hero lost %d health", seed, taken, lost)
			}
		}
	}
	if len(played) == 0 {
		t.Fatal("blocking Nudge blocked every card")
	}
}
