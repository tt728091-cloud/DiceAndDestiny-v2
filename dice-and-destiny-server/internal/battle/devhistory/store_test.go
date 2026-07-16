package devhistory

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestStoreRecordsOrderedAncestryAndClientPresentationState(t *testing.T) {
	checkpoint := historyCheckpoint(t, "battle-history-source")
	nextID := 0
	store := Store{
		Root: t.TempDir(),
		Now:  func() time.Time { return time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC) },
		IDGenerator: func() (string, error) {
			nextID++
			return fmt.Sprintf("history-%016x", nextID), nil
		},
	}

	first, err := store.Mark(checkpoint, "blade", "Roll 5 Dice", KindDecision, 0, map[string]any{"selected_indices": []any{1.0, 3.0}}, map[string]any{"type": "planning_roll"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Mark(checkpoint, "blade", "Continue: Income Results", KindPresentation, 1, map[string]any{"selected_source": "damage-1"}, map[string]any{"type": "presentation_continue", "watermark": 1})
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := store.List(checkpoint.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline.Points) != 2 || timeline.Points[0].ID != first.ID || timeline.Points[1].ID != second.ID || timeline.Branch.HeadPointID != second.ID {
		t.Fatalf("timeline order or head is wrong: %#v", timeline)
	}
	loaded, branch, err := store.LoadPointForBranch(checkpoint.BattleID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if branch.Status != BranchActive || loaded.Metadata.Label != "Roll 5 Dice" || len(loaded.ClientState["selected_indices"].([]any)) != 2 {
		t.Fatalf("loaded history point = %#v, branch = %#v", loaded, branch)
	}
	if _, _, err := store.LoadPointForBranch(checkpoint.BattleID, "history-ffffffffffffffff"); !errors.Is(err, ErrInvalidPoint) {
		t.Fatalf("foreign point error = %v", err)
	}
	if _, err := store.Mark(checkpoint, "blade", "Impossible cursor", KindPresentation, 2, nil, nil); err == nil {
		t.Fatal("presented sequence beyond event history was accepted")
	}
}

func TestStoreReviewBranchesPreserveOrArchivePriorFuture(t *testing.T) {
	checkpoint := historyCheckpoint(t, "battle-history-source")
	nextID := 0
	store := Store{Root: t.TempDir(), IDGenerator: func() (string, error) {
		nextID++
		return fmt.Sprintf("history-%016x", nextID), nil
	}}
	point, err := store.Mark(checkpoint, "blade", "Before action", KindDecision, 1, nil, map[string]any{"type": "first_action"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Mark(checkpoint, "blade", "Later action", KindDecision, 1, nil, map[string]any{"type": "later_action"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.Branch(checkpoint.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	review, err := store.CreateReviewBranch(source, "battle-review-one", point.ID)
	if err != nil || review.Status != BranchReview || review.ParentBattleID != checkpoint.BattleID {
		t.Fatalf("review branch = %#v, %v", review, err)
	}
	active, err := store.CommitReview(review.BattleID, "preserve")
	if err != nil || active.Status != BranchReplay || active.CursorPointID != point.ID || active.HeadPointID != second.ID {
		t.Fatalf("preserved branch = %#v, %v", active, err)
	}
	preservedTimeline, err := store.List(review.BattleID)
	if err != nil || len(preservedTimeline.Points) != 2 {
		t.Fatalf("preserved future was dropped: %#v, %v", preservedTimeline, err)
	}
	mismatch, err := store.PrepareReplay(review.BattleID, map[string]any{"type": "different_action"})
	if err != nil || mismatch.Matches || mismatch.FutureCount != 2 {
		t.Fatalf("divergent replay was not detected: %#v, %v", mismatch, err)
	}
	matching, err := store.PrepareReplay(review.BattleID, map[string]any{"type": "first_action"})
	if err != nil || !matching.Matches || matching.Next == nil || matching.Next.Metadata.ID != second.ID {
		t.Fatalf("matching replay did not find its forward point: %#v, %v", matching, err)
	}
	advanced, err := store.AdvanceReplay(review.BattleID, point.ID)
	if err != nil || advanced.Status != BranchReplay || advanced.CursorPointID != second.ID || advanced.HeadPointID != second.ID {
		t.Fatalf("replay cursor did not advance without dropping future: %#v, %v", advanced, err)
	}
	truncated, err := store.TruncateReplay(review.BattleID)
	if err != nil || truncated.Status != BranchActive || truncated.HeadPointID != point.ID || truncated.DiscardedHeadPointID != second.ID {
		t.Fatalf("confirmed divergence did not replace only current/future points: %#v, %v", truncated, err)
	}
	nestedReview, err := store.CreateReviewBranch(truncated, "battle-review-nested", point.ID)
	if err != nil || nestedReview.LatestBattleID != truncated.BattleID {
		t.Fatalf("review of an active replacement branch retained a stale latest battle: %#v, %v", nestedReview, err)
	}
	// Compatibility for branches already written by the buggy implementation:
	// prefer the active parent over their persisted pre-divergence pointer.
	nestedReview.LatestBattleID = checkpoint.BattleID
	if err := store.writeAtomic(store.branchPath(nestedReview.BattleID), nestedReview); err != nil {
		t.Fatal(err)
	}
	resolvedLatest, err := store.ResolveLatestBattleID(nestedReview.BattleID)
	if err != nil || resolvedLatest != truncated.BattleID {
		t.Fatalf("stale nested latest was not repaired through its active parent: %q, %v", resolvedLatest, err)
	}
	parent, err := store.Branch(checkpoint.BattleID)
	if err != nil || parent.Status != BranchActive {
		t.Fatalf("preserve altered parent = %#v, %v", parent, err)
	}

	reviewTwo, err := store.CreateReviewBranch(source, "battle-review-two", point.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitReview(reviewTwo.BattleID, "replace"); err != nil {
		t.Fatal(err)
	}
	parent, err = store.Branch(checkpoint.BattleID)
	if err != nil || parent.Status != BranchArchived {
		t.Fatalf("replace did not archive parent = %#v, %v", parent, err)
	}
}

func TestStoreRecordsCurrentEndpointAndEnrichesItWithTheNextAction(t *testing.T) {
	checkpoint := historyCheckpoint(t, "battle-history-endpoint")
	nextID := 0
	store := Store{Root: t.TempDir(), IDGenerator: func() (string, error) {
		nextID++
		return fmt.Sprintf("history-%016x", nextID), nil
	}}
	current, err := store.Mark(checkpoint, "blade", "Offensive Dice 0/3", KindDecision, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	outgoing, err := store.Mark(checkpoint, "blade", "Roll 5 Dice", KindDecision, 1, nil, map[string]any{"type": "planning_roll"})
	if err != nil || outgoing.ID != current.ID || outgoing.ActionKey == "" || outgoing.Label != "Offensive Dice 0/3" || outgoing.ActionLabel != "Roll 5 Dice" {
		t.Fatalf("current state was duplicated instead of enriched: %#v, %v", outgoing, err)
	}
	if _, err := repository.AppendEvents(&checkpoint, []event.Event{{Type: event.TypeDiceRolled, ActorID: "blade"}}); err != nil {
		t.Fatal(err)
	}
	endpoint, err := store.Mark(checkpoint, "blade", "Offensive Dice 1/3", KindDecision, 2, map[string]any{"selected_indices": []any{}}, nil)
	if err != nil || endpoint.ID == current.ID || endpoint.ActionKey != "" || endpoint.ActionLabel != "" {
		t.Fatalf("resulting current state was not recorded as an actionless endpoint: %#v, %v", endpoint, err)
	}
	timeline, err := store.List(checkpoint.BattleID)
	if err != nil || len(timeline.Points) != 2 {
		t.Fatalf("state/action timeline = %#v, %v", timeline, err)
	}

	replayReview, err := store.CreateReviewBranch(timeline.Branch, "battle-endpoint-replay", current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitReview(replayReview.BattleID, "preserve"); err != nil {
		t.Fatal(err)
	}
	advanced, err := store.AdvanceReplay(replayReview.BattleID, current.ID)
	if err != nil || advanced.Status != BranchActive || advanced.HeadPointID != endpoint.ID {
		t.Fatalf("replay did not become active on the terminal result point: %#v, %v", advanced, err)
	}

	terminalReview, err := store.CreateReviewBranch(timeline.Branch, "battle-endpoint-terminal", endpoint.ID)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := store.CommitReview(terminalReview.BattleID, "preserve")
	if err != nil || terminal.Status != BranchActive || terminal.HeadPointID != endpoint.ID {
		t.Fatalf("resuming the terminal point incorrectly entered replay: %#v, %v", terminal, err)
	}
}

func TestStoreExportsAndImportsIndependentPortableTimeline(t *testing.T) {
	checkpoint := historyCheckpoint(t, "battle-history-portable")
	nextID := 0
	store := Store{Root: t.TempDir(), IDGenerator: func() (string, error) {
		nextID++
		return fmt.Sprintf("history-%016x", nextID), nil
	}}
	first, err := store.Mark(checkpoint, "blade", "Roll 5 Dice", KindDecision, 1, map[string]any{"selected_indices": []any{1.0, 3.0}}, map[string]any{"type": "planning_roll"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Mark(checkpoint, "blade", "Offensive Dice 1/3", KindDecision, 1, map[string]any{"selected_indices": []any{3.0}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := store.Export(checkpoint.BattleID)
	if err != nil || bundle == nil || len(bundle.Points) != 2 {
		t.Fatalf("Export() = %#v, %v", bundle, err)
	}
	imported, err := store.Import(*bundle, "battle-history-imported", "battle-history-imported")
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := store.List("battle-history-imported")
	if err != nil || timeline.Branch.Status != BranchActive || len(timeline.Points) != 2 {
		t.Fatalf("imported timeline = %#v, %v", timeline, err)
	}
	if timeline.Points[0].ID == first.ID || timeline.Points[1].ID == second.ID || timeline.Points[1].ParentPointID != timeline.Points[0].ID {
		t.Fatalf("import did not independently remap point identities: %#v", timeline.Points)
	}
	loaded, _, err := store.LoadPointForBranch("battle-history-imported", timeline.Points[1].ID)
	if err != nil || loaded.Checkpoint.BattleID != "battle-history-imported" || loaded.Metadata.BattleID != "battle-history-imported" || len(loaded.ClientState["selected_indices"].([]any)) != 1 {
		t.Fatalf("imported point = %#v, %v", loaded, err)
	}
	if imported.Current.Metadata.ID != timeline.Points[1].ID || imported.Branch.HeadPointID != timeline.Points[1].ID {
		t.Fatalf("import result did not identify the restored current state: %#v", imported)
	}
	original, err := store.List(checkpoint.BattleID)
	if err != nil || original.Points[0].ID != first.ID || original.Points[1].ID != second.ID {
		t.Fatalf("import mutated the source timeline: %#v, %v", original, err)
	}
}

func TestStoreImportsReviewTimelineWithIndependentLatestParent(t *testing.T) {
	checkpoint := historyCheckpoint(t, "battle-history-review-portable")
	nextID := 20
	store := Store{Root: t.TempDir(), IDGenerator: func() (string, error) {
		nextID++
		return fmt.Sprintf("history-%016x", nextID), nil
	}}
	first, err := store.Mark(checkpoint, "blade", "First choice", KindDecision, 1, nil, map[string]any{"type": "first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Mark(checkpoint, "blade", "Later state", KindDecision, 1, nil, nil); err != nil {
		t.Fatal(err)
	}
	source, err := store.Branch(checkpoint.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateReviewBranch(source, "battle-history-review-capture", first.ID); err != nil {
		t.Fatal(err)
	}
	bundle, err := store.Export("battle-history-review-capture")
	if err != nil || bundle == nil {
		t.Fatalf("review Export() = %#v, %v", bundle, err)
	}
	imported, err := store.Import(*bundle, "battle-history-review-restored", "battle-history-review-restored-latest")
	if err != nil {
		t.Fatal(err)
	}
	if imported.Branch.Status != BranchReview || imported.Branch.ParentBattleID != "battle-history-review-restored-latest" || imported.Branch.CursorPointID == first.ID {
		t.Fatalf("restored review branch = %#v", imported.Branch)
	}
	latest, err := store.Branch("battle-history-review-restored-latest")
	if err != nil || latest.Status != BranchActive || latest.HeadPointID != imported.Branch.HeadPointID {
		t.Fatalf("restored latest parent = %#v, %v", latest, err)
	}
	resolved, err := store.ResolveLatestBattleID(imported.Branch.BattleID)
	if err != nil || resolved != latest.BattleID {
		t.Fatalf("restored latest resolution = %q, %v", resolved, err)
	}
}

func historyCheckpoint(t *testing.T, battleID string) repository.Checkpoint {
	t.Helper()
	battle, err := state.NewBattleFromSetup(battleID, state.BattleSetup{Actors: []state.ActorSetup{
		{ID: "blade", ControllerType: state.ControllerHuman, Deck: []string{"blade-card"}},
		{ID: "goblin", ControllerType: state.ControllerAI, Deck: []string{"goblin-card"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendEvents(&checkpoint, []event.Event{{Type: event.TypeSegmentEntered, ActorID: "blade"}}); err != nil {
		t.Fatal(err)
	}
	return checkpoint
}
