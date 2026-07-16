package battle

import (
	"encoding/json"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/devhistory"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
)

func TestHistoryAuthorityDisabled(t *testing.T) {
	authority := NewHistoryAuthority(HistoryAuthorityConfig{Gameplay: &recordingHandler{}})
	result := authority.HandleCommand(command.Command{BattleID: "battle-source", ActorID: "blade", Type: command.TypeListHistory, Payload: json.RawMessage(`{}`)})
	if result.Accepted || result.Error != "developer history tooling is disabled" {
		t.Fatalf("disabled history result = %#v", result)
	}
}

func TestHistoryAuthorityMarksJumpsReviewsBranchesAndReturns(t *testing.T) {
	repo := repository.NewInMemory()
	source := snapshotAuthorityCheckpoint(t, "battle-history-source")
	if err := repo.Create(source); err != nil {
		t.Fatal(err)
	}
	gameplay := NewAuthority(engine.NewEngine(), repo, &recordingAssembler{})
	copyNumber := 0
	store := devhistory.Store{Root: t.TempDir()}
	authority := NewHistoryAuthority(HistoryAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: gameplay, Repository: repo, Store: store,
		IDGenerator: func(devhistory.Point) (string, error) {
			copyNumber++
			if copyNumber == 1 {
				return "battle-history-review-one", nil
			}
			return "battle-history-review-two", nil
		},
	})

	marked := authority.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Roll 5 Dice","kind":"decision","presented_sequence":1,"client_state":{"selected_indices":[1,3]},"action":{"type":"planning_roll"}}`),
	})
	if !marked.Accepted {
		t.Fatalf("mark history = %#v", marked)
	}
	timeline, err := store.List(source.BattleID)
	if err != nil || len(timeline.Points) != 1 {
		t.Fatalf("timeline = %#v, %v", timeline, err)
	}
	pointID := timeline.Points[0].ID

	review := authority.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeJumpHistory,
		Payload: json.RawMessage(`{"point_id":"` + pointID + `"}`),
	})
	if !review.Accepted || review.Snapshot == nil || review.Snapshot.BattleID != "battle-history-review-one" || len(review.Events) != len(source.Events) {
		t.Fatalf("jump history = %#v", review)
	}
	if review.Snapshot.Actors["goblin"].Hand != nil {
		t.Fatal("history review exposed the enemy hidden hand")
	}
	cloned, err := repo.Load(review.Snapshot.BattleID)
	if err != nil || cloned.Battle.Random.Cursor != source.Battle.Random.Cursor || len(cloned.Events) != len(source.Events) {
		t.Fatalf("history clone = %#v, %v", cloned, err)
	}
	blocked := authority.HandleCommand(command.Command{BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypePlanningRoll, Payload: json.RawMessage(`{}`)})
	if blocked.Accepted || blocked.Error != "history review is read-only; preserve or replace the future before continuing" {
		t.Fatalf("review gameplay block = %#v", blocked)
	}
	returned := authority.HandleCommand(command.Command{BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReturnHistory, Payload: json.RawMessage(`{}`)})
	if !returned.Accepted || returned.Snapshot == nil || returned.Snapshot.BattleID != source.BattleID {
		t.Fatalf("return latest = %#v", returned)
	}
	committed := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeCommitHistory, Payload: json.RawMessage(`{"mode":"preserve"}`),
	})
	if !committed.Accepted || len(committed.Events) != len(source.Events) {
		t.Fatalf("preserve future = %#v", committed)
	}
	branch, err := store.Branch(review.Snapshot.BattleID)
	if err != nil || branch.Status != devhistory.BranchReplay || branch.CursorPointID != pointID || branch.HeadPointID != pointID {
		t.Fatalf("preserved branch = %#v, %v", branch, err)
	}
	mismatch := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReplayHistory,
		Payload: json.RawMessage(`{"action":{"type":"different_action"}}`),
	})
	mismatchData, _ := mismatch.Data.(map[string]any)
	mismatchHistory, _ := mismatchData["history"].(map[string]any)
	if mismatch.Accepted || mismatchHistory["divergence"] != true {
		t.Fatalf("divergent replay was not reported for confirmation: %#v", mismatch)
	}
	replayed := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReplayHistory,
		Payload: json.RawMessage(`{"action":{"type":"planning_roll"}}`),
	})
	if !replayed.Accepted || replayed.Snapshot == nil || replayed.Snapshot.BattleID != review.Snapshot.BattleID {
		t.Fatalf("matching replay did not advance to the retained future: %#v", replayed)
	}
	branch, err = store.Branch(review.Snapshot.BattleID)
	if err != nil || branch.Status != devhistory.BranchActive {
		t.Fatalf("replay did not become active after exhausting the retained future: %#v, %v", branch, err)
	}

	// The active replay battle is now a replacement future whose inherited
	// latest ID still names the original source. Record another action on this
	// branch, advance it, then rewind again. The nested replay must finish at
	// the active replacement battle rather than the stale original checkpoint.
	activeCheckpoint, err := repo.Load(review.Snapshot.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendEvents(&activeCheckpoint, []event.Event{{Type: event.TypeDiceRolled, ActorID: "blade"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(activeCheckpoint); err != nil {
		t.Fatal(err)
	}
	nestedMark := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Final reroll","kind":"decision","presented_sequence":2,"action":{"type":"reroll_unkept"}}`),
	})
	if !nestedMark.Accepted {
		t.Fatalf("mark nested replacement history = %#v", nestedMark)
	}
	nestedTimeline, err := store.List(review.Snapshot.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	nestedPointID := nestedTimeline.Points[len(nestedTimeline.Points)-1].ID
	if _, err := repository.AppendEvents(&activeCheckpoint, []event.Event{{Type: event.TypeDiceRolled, ActorID: "blade"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(activeCheckpoint); err != nil {
		t.Fatal(err)
	}
	nestedReview := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeJumpHistory,
		Payload: json.RawMessage(`{"point_id":"` + nestedPointID + `"}`),
	})
	if !nestedReview.Accepted || nestedReview.Snapshot == nil {
		t.Fatalf("jump nested replacement history = %#v", nestedReview)
	}
	nestedCommitted := authority.HandleCommand(command.Command{
		BattleID: nestedReview.Snapshot.BattleID, ActorID: "blade", Type: command.TypeCommitHistory,
		Payload: json.RawMessage(`{"mode":"preserve"}`),
	})
	if !nestedCommitted.Accepted {
		t.Fatalf("preserve nested replacement future = %#v", nestedCommitted)
	}
	nestedBranch, err := store.Branch(nestedReview.Snapshot.BattleID)
	if err != nil || nestedBranch.LatestBattleID != review.Snapshot.BattleID {
		t.Fatalf("nested branch latest endpoint = %#v, %v", nestedBranch, err)
	}
	nestedReplayed := authority.HandleCommand(command.Command{
		BattleID: nestedReview.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReplayHistory,
		Payload: json.RawMessage(`{"action":{"type":"reroll_unkept"}}`),
	})
	if !nestedReplayed.Accepted || len(nestedReplayed.Events) != len(activeCheckpoint.Events) {
		t.Fatalf("nested replay did not finish at active replacement future: %#v", nestedReplayed)
	}
	original, err := repo.Load(source.BattleID)
	if err != nil || original.Battle.Random.Cursor != source.Battle.Random.Cursor {
		t.Fatal("history operations mutated the original battle")
	}
}
