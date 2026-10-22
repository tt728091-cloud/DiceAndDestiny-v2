package lookahead_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/lookahead"
)

// renamedContent copies the content root and adds a copy of a combatant
// under another ID and display name.
func renamedContent(t *testing.T, from, id, name string) string {
	t.Helper()
	source := filepath.Join(serverRoot(t), "content")
	root := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(root, rel), 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	combatants := filepath.Join(root, "adventurer_v1", "combatants")
	raw, err := os.ReadFile(filepath.Join(combatants, from+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var definition map[string]any
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	definition["id"], definition["name"] = id, name
	if raw, err = json.Marshal(definition); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(combatants, id+".yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A hero's name never changes how the greedy player plays. The authority
// sorts commands by their text, and a dice card's targets carry the
// character's name ("Starter · die 1: 5 → 4"), so a name that sorts before
// "Play Nudge" once made the greedy player commit the card to the first
// listed die. The Adventurer (the Starter's board) lost to the Starter for it.
func TestRenamingAHeroChangesNoBattle(t *testing.T) {
	contentRoot := renamedContent(t, "starter", "aardvark", "Aardvark")
	authored := authoredCopy(t)
	runner := func(hero string, seatB bool) *lookahead.Runner {
		r, err := lookahead.NewRunner(lookahead.Matchup{
			ContentRoot:    contentRoot,
			AuthoredRoot:   authored,
			RunStateRoot:   t.TempDir(),
			Hero:           hero,
			Opponent:       "starter",
			Unified:        true,
			HeroPlansFirst: true,
			HeroInSeatB:    seatB,
		})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	diceCards := 0
	for _, seatB := range []bool{false, true} {
		starter, renamed := runner("starter", seatB), runner("aardvark", seatB)
		for seed := uint64(1); seed <= 12; seed++ {
			want, err := starter.Play(seed, lookahead.NewGreedyPolicy(), lookahead.NewGreedyPolicy())
			if err != nil {
				t.Fatal(err)
			}
			got, err := renamed.Play(seed, lookahead.NewGreedyPolicy(), lookahead.NewGreedyPolicy())
			if err != nil {
				t.Fatal(err)
			}
			want.HeroThinkMS, got.HeroThinkMS = 0, 0
			if !reflect.DeepEqual(want, got) {
				a, _ := json.Marshal(want)
				b, _ := json.Marshal(got)
				t.Fatalf("seed %d (seat b %v): renaming the Starter changed the battle\nStarter:  %s\nAardvark: %s", seed, seatB, a, b)
			}
			diceCards += want.CardsPlayed["nudge"] + want.CardsPlayed["try_again"]
		}
	}
	if diceCards == 0 {
		t.Fatal("no Nudge or Try Again was played, so the test checks nothing")
	}
}
