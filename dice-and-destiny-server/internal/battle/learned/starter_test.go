package learned

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/content"
)

// The Starter mirrors the Adventurer's card mix and board, but every card is
// the base of a published General card tree.
func TestStarterDeckUsesOnlyTreeBases(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	// Read (never write) the tracked published trees.
	t.Setenv(content.AuthoredRootEnv, filepath.Join(contentRoot, "authored"))
	catalogs, err := CharacterCatalogs(contentRoot, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["starter"]
	starter, adventurer := lib.Combatants["starter"], lib.Combatants["adventurer"]
	total, trees := 0, map[string]int{}
	for _, entry := range starter.Decklist {
		total += entry.Count
		tree, _, variant := content.TreeCardOwner(lib.CardTrees, entry.CardID)
		if tree == "" || variant {
			t.Fatalf("%s is not a card tree base", entry.CardID)
		}
		if access := lib.Cards[entry.CardID].AccessType; access != "" && access != "general" {
			t.Fatalf("%s is not a General card", entry.CardID)
		}
		trees[tree] += entry.Count
	}
	if total != 12 {
		t.Fatalf("Starter has %d cards (health); want 12", total)
	}
	adventurerTrees := map[string]int{}
	for _, entry := range adventurer.Decklist {
		tree, _, _ := content.TreeCardOwner(lib.CardTrees, entry.CardID)
		adventurerTrees[tree] += entry.Count
	}
	for tree, count := range adventurerTrees {
		if trees[tree] != count {
			t.Fatalf("Starter tree mix %v differs from the Adventurer's %v", trees, adventurerTrees)
		}
	}
	if len(starter.AbilityBoard.Defensive) != 1 || starter.AbilityBoard.Defensive[0] != "adventurer_guard" {
		t.Fatalf("Starter defends with the basic Guard only: %v", starter.AbilityBoard.Defensive)
	}
}

func TestStarterFullGame(t *testing.T) {
	root := testServerRoot(t)
	s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: t.TempDir(), ModelPath: filepath.Join(root, "../dice-and-destiny-client/models/learned", "blade-warden-seed-11-final-v1.json"), InferenceTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.ResetCharacter("starter-0", 911, "seat-a", false, "starter")
	if err != nil {
		t.Fatal(err)
	}
	actors := object(object(view["snapshot"])["actors"])
	if object(actors[HumanAlias])["definition_id"] != "starter" {
		t.Fatalf("wrong character: %v", actors)
	}
	cardPlays := map[int]int{}
	for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal; step++ {
		if s.current.ActorID == s.modelSeat {
			if _, err = s.AdvanceModel(); err != nil {
				t.Fatalf("model step %d: %v", step, err)
			}
			continue
		}
		action := adventurerTestAction(s.current.Result.LegalActions, cardPlays[s.current.Result.Snapshot.Round])
		if action.Type == command.TypePlanningCards {
			cardPlays[s.current.Result.Snapshot.Round]++
		}
		encoded, _ := json.Marshal(aliasValue(commandMap(t, action), map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias}))
		if _, err = s.SubmitHuman(string(encoded)); err != nil {
			t.Fatalf("human step %d: %s: %v", step, encoded, err)
		}
	}
	if !s.current.Terminal || s.current.TruncationReason != "" {
		t.Fatalf("did not complete: %+v", s.current.Metrics)
	}
}
