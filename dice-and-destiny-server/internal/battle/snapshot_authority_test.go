package battle

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/devhistory"
	"diceanddestiny/server/internal/battle/devsnapshot"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestSnapshotAuthorityDisabled(t *testing.T) {
	authority := NewSnapshotAuthority(SnapshotAuthorityConfig{Gameplay: &recordingHandler{}})
	result := authority.HandleCommand(command.Command{
		BattleID: "battle-source", ActorID: "blade", Type: command.TypeListSnapshots, Payload: json.RawMessage(`{}`),
	})
	if result.Accepted || result.Error != "developer snapshot tooling is disabled" {
		t.Fatalf("disabled result = %#v", result)
	}
}

func TestSnapshotAuthorityCapturesListsLoadsAndRestartsIndependentBattles(t *testing.T) {
	repo := repository.NewInMemory()
	source := snapshotAuthorityCheckpoint(t, "battle-source")
	if _, err := repository.AppendEvents(&source, []event.Event{{Type: event.TypeCardsDrawn, ActorID: "goblin", Cards: []string{"goblin-1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(source); err != nil {
		t.Fatal(err)
	}
	gameplay := NewAuthority(engine.NewEngine(), repo, &recordingAssembler{})
	historyStore := devhistory.Store{Root: t.TempDir()}
	firstHistory, err := historyStore.Mark(source, "blade", "Roll 5 Dice", devhistory.KindDecision, 1, map[string]any{"selected_indices": []any{1.0, 3.0}}, map[string]any{"type": "planning_roll"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := historyStore.Mark(source, "blade", "Offensive Dice 1/3", devhistory.KindDecision, 1, map[string]any{"selected_indices": []any{3.0}}, nil); err != nil {
		t.Fatal(err)
	}
	copyNumber := 0
	authority := NewSnapshotAuthority(SnapshotAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: gameplay, Repository: repo,
		HistoryStore: historyStore,
		Store: devsnapshot.Store{Root: t.TempDir(), Now: func() time.Time {
			return time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC)
		}},
		IDGenerator: func(devsnapshot.Record) (string, error) {
			copyNumber++
			if copyNumber == 1 {
				return "battle-snapshot-copy-one", nil
			}
			return "battle-snapshot-copy-two", nil
		},
	})

	saved := authority.HandleCommand(command.Command{
		BattleID: "battle-source", ActorID: "blade", Type: command.TypeSaveSnapshot,
		Payload: json.RawMessage(`{"name":"round-2-effects"}`),
	})
	if !saved.Accepted || saved.Data == nil {
		t.Fatalf("save_dev_snapshot = %#v", saved)
	}
	savedData := saved.Data.(map[string]any)
	savedMetadata := savedData["snapshot"].(devsnapshot.Metadata)
	if !savedMetadata.HistoryIncluded || savedMetadata.HistoryPointCount != 2 {
		t.Fatalf("snapshot did not report bundled history: %#v", savedMetadata)
	}
	duplicate := authority.HandleCommand(command.Command{
		BattleID: "battle-source", ActorID: "blade", Type: command.TypeSaveSnapshot,
		Payload: json.RawMessage(`{"name":"round-2-effects"}`),
	})
	if duplicate.Accepted {
		t.Fatalf("duplicate save unexpectedly accepted: %#v", duplicate)
	}
	listed := authority.HandleCommand(command.Command{
		BattleID: "battle-source", ActorID: "blade", Type: command.TypeListSnapshots, Payload: json.RawMessage(`{}`),
	})
	if !listed.Accepted {
		t.Fatalf("list_dev_snapshots = %#v", listed)
	}

	mutated, err := repo.Load("battle-source")
	if err != nil {
		t.Fatal(err)
	}
	mutated.Battle.Random.Cursor = 99
	actor := mutated.Battle.Actors["blade"]
	actor.Resources.EnergyPoints = 0
	mutated.Battle.Actors["blade"] = actor
	if err := repo.Save(mutated); err != nil {
		t.Fatal(err)
	}

	first := authority.HandleCommand(command.Command{
		BattleID: "battle-source", ActorID: "blade", Type: command.TypeLoadSnapshot,
		Payload: json.RawMessage(`{"name":"round-2-effects"}`),
	})
	if !first.Accepted || first.Snapshot == nil || first.Snapshot.BattleID != "battle-snapshot-copy-one" || len(first.Events) != len(source.Events) {
		t.Fatalf("first load_dev_snapshot = %#v", first)
	}
	if first.Events[0].BattleID != "battle-snapshot-copy-one" || first.Events[0].ID == source.Events[0].ID {
		t.Fatalf("loaded snapshot events were not independently re-keyed: %#v", first.Events)
	}
	if len(first.Events[1].Cards) != 0 || first.Events[1].Count != 1 {
		t.Fatalf("loaded snapshot events exposed an enemy card identity: %#v", first.Events[1])
	}
	if first.Snapshot.Actors["goblin"].Hand != nil {
		t.Fatal("loaded snapshot exposed the enemy hidden hand")
	}
	firstTimeline, err := historyStore.List(first.Snapshot.BattleID)
	if err != nil || len(firstTimeline.Points) != 2 || firstTimeline.Points[0].ID == firstHistory.ID || firstTimeline.Branch.Status != devhistory.BranchActive {
		t.Fatalf("first loaded snapshot history = %#v, %v", firstTimeline, err)
	}
	firstData := first.Data.(map[string]any)
	if firstData["history"] == nil {
		t.Fatalf("load result omitted restored history context: %#v", first.Data)
	}
	firstCheckpoint, err := repo.Load(first.Snapshot.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	if firstCheckpoint.Battle.Random.Cursor != source.Battle.Random.Cursor ||
		firstCheckpoint.Battle.Actors["blade"].Resources.EnergyPoints != source.Battle.Actors["blade"].Resources.EnergyPoints ||
		len(firstCheckpoint.Events) != len(source.Events) {
		t.Fatal("loaded battle did not preserve captured state, RNG, and history")
	}

	second := authority.HandleCommand(command.Command{
		BattleID: first.Snapshot.BattleID, ActorID: "blade", Type: command.TypeLoadSnapshot,
		Payload: json.RawMessage(`{"name":"round-2-effects"}`),
	})
	if !second.Accepted || second.Snapshot == nil || second.Snapshot.BattleID != "battle-snapshot-copy-two" {
		t.Fatalf("restart load_dev_snapshot = %#v", second)
	}
	secondTimeline, err := historyStore.List(second.Snapshot.BattleID)
	if err != nil || len(secondTimeline.Points) != 2 || secondTimeline.Points[0].ID == firstTimeline.Points[0].ID {
		t.Fatalf("second loaded snapshot did not receive independent history ids: %#v, %v", secondTimeline, err)
	}
	secondCheckpoint, err := repo.Load(second.Snapshot.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	firstCheckpoint.BattleID = secondCheckpoint.BattleID
	firstCheckpoint.Battle.ID = secondCheckpoint.Battle.ID
	for i := range firstCheckpoint.Events {
		firstCheckpoint.Events[i].BattleID = secondCheckpoint.Events[i].BattleID
		firstCheckpoint.Events[i].ID = secondCheckpoint.Events[i].ID
	}
	if !reflect.DeepEqual(firstCheckpoint, secondCheckpoint) {
		t.Fatal("restarting the same snapshot did not create equivalent independent battle state")
	}
	original, err := repo.Load("battle-source")
	if err != nil || original.Battle.Random.Cursor != 99 {
		t.Fatal("loading snapshot mutated the source battle")
	}
}

func TestSnapshotAuthorityRestoresReviewHistoryAndItsLatestFuture(t *testing.T) {
	repo := repository.NewInMemory()
	source := snapshotAuthorityCheckpoint(t, "battle-snapshot-history-source")
	if err := repo.Create(source); err != nil {
		t.Fatal(err)
	}
	historyStore := devhistory.Store{Root: t.TempDir()}
	core := NewAuthority(engine.NewEngine(), repo, &recordingAssembler{})
	snapshotAuthority := NewSnapshotAuthority(SnapshotAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: core, Repository: repo,
		Store: devsnapshot.Store{Root: t.TempDir()}, HistoryStore: historyStore,
		IDGenerator: func(devsnapshot.Record) (string, error) { return "battle-snapshot-history-restored", nil },
	})
	reviewNumber := 0
	authority := NewHistoryAuthority(HistoryAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: snapshotAuthority, Repository: repo, Store: historyStore,
		IDGenerator: func(devhistory.Point) (string, error) {
			reviewNumber++
			return fmt.Sprintf("battle-snapshot-history-review-%d", reviewNumber), nil
		},
	})
	first := authority.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Roll 5 Dice","kind":"decision","presented_sequence":1,"client_state":{"selected_indices":[1,3]},"action":{"type":"planning_roll"}}`),
	})
	if !first.Accepted {
		t.Fatalf("first history mark = %#v", first)
	}
	second := authority.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Offensive Dice 1/3","kind":"decision","presented_sequence":1,"client_state":{"selected_indices":[3]}}`),
	})
	if !second.Accepted {
		t.Fatalf("second history mark = %#v", second)
	}
	sourceTimeline, err := historyStore.List(source.BattleID)
	if err != nil {
		t.Fatal(err)
	}
	review := authority.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeJumpHistory,
		Payload: json.RawMessage(`{"point_id":"` + sourceTimeline.Points[0].ID + `"}`),
	})
	if !review.Accepted || review.Snapshot == nil {
		t.Fatalf("open source review = %#v", review)
	}
	saved := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeSaveSnapshot,
		Payload: json.RawMessage(`{"name":"review-with-future"}`),
	})
	if !saved.Accepted {
		t.Fatalf("save reviewed history snapshot = %#v", saved)
	}
	loaded := authority.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeLoadSnapshot,
		Payload: json.RawMessage(`{"name":"review-with-future"}`),
	})
	if !loaded.Accepted || loaded.Snapshot == nil || loaded.Snapshot.BattleID != "battle-snapshot-history-restored" {
		t.Fatalf("load reviewed history snapshot = %#v", loaded)
	}
	restoredTimeline, err := historyStore.List(loaded.Snapshot.BattleID)
	if err != nil || restoredTimeline.Branch.Status != devhistory.BranchReview || len(restoredTimeline.Points) != 2 || restoredTimeline.Points[0].ID == sourceTimeline.Points[0].ID {
		t.Fatalf("restored review timeline = %#v, %v", restoredTimeline, err)
	}
	if restoredTimeline.Branch.ParentBattleID != "battle-snapshot-history-restored-latest" {
		t.Fatalf("restored review did not point at its independent latest battle: %#v", restoredTimeline.Branch)
	}
	returned := authority.HandleCommand(command.Command{BattleID: loaded.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReturnHistory, Payload: json.RawMessage(`{}`)})
	if !returned.Accepted || returned.Snapshot == nil || returned.Snapshot.BattleID != "battle-snapshot-history-restored-latest" {
		t.Fatalf("restored review could not return to its bundled latest future: %#v", returned)
	}
	committed := authority.HandleCommand(command.Command{
		BattleID: loaded.Snapshot.BattleID, ActorID: "blade", Type: command.TypeCommitHistory, Payload: json.RawMessage(`{"mode":"preserve"}`),
	})
	if !committed.Accepted {
		t.Fatalf("restored review could not resume its future: %#v", committed)
	}
	replayed := authority.HandleCommand(command.Command{
		BattleID: loaded.Snapshot.BattleID, ActorID: "blade", Type: command.TypeReplayHistory,
		Payload: json.RawMessage(`{"action":{"type":"planning_roll"}}`),
	})
	if !replayed.Accepted {
		t.Fatalf("restored review could not replay its recorded action: %#v", replayed)
	}
	restoredBranch, err := historyStore.Branch(loaded.Snapshot.BattleID)
	if err != nil || restoredBranch.Status != devhistory.BranchActive {
		t.Fatalf("restored replay did not reach its terminal state: %#v, %v", restoredBranch, err)
	}
}

func TestSnapshotAuthorityResavesLoadedBattleAfterHistoryReplacement(t *testing.T) {
	repo := repository.NewInMemory()
	source := snapshotAuthorityCheckpoint(t, "battle-snapshot-resave-source")
	if err := repo.Create(source); err != nil {
		t.Fatal(err)
	}
	historyStore := devhistory.Store{Root: t.TempDir()}
	snapshotStore := devsnapshot.Store{Root: t.TempDir()}
	core := NewAuthority(engine.NewEngine(), repo, &recordingAssembler{})
	snapshots := NewSnapshotAuthority(SnapshotAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: core, Repository: repo,
		Store: snapshotStore, HistoryStore: historyStore,
		IDGenerator: func(devsnapshot.Record) (string, error) { return "battle-snapshot-resave-loaded", nil },
	})
	history := NewHistoryAuthority(HistoryAuthorityConfig{
		BuildEnabled: true, RuntimeEnabled: true, Gameplay: snapshots, Repository: repo, Store: historyStore,
		IDGenerator: func(devhistory.Point) (string, error) { return "battle-snapshot-resave-review", nil },
	})

	marked := history.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Roll 5 Dice","kind":"decision","presented_sequence":1,"action":{"type":"planning_roll"}}`),
	})
	if !marked.Accepted {
		t.Fatalf("initial history mark = %#v", marked)
	}
	marked = history.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Offensive Dice 1/3","kind":"decision","presented_sequence":1}`),
	})
	if !marked.Accepted {
		t.Fatalf("history endpoint mark = %#v", marked)
	}
	saved := history.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeSaveSnapshot,
		Payload: json.RawMessage(`{"name":"before-rewind"}`),
	})
	if !saved.Accepted {
		t.Fatalf("initial snapshot save = %#v", saved)
	}
	loaded := history.HandleCommand(command.Command{
		BattleID: source.BattleID, ActorID: "blade", Type: command.TypeLoadSnapshot,
		Payload: json.RawMessage(`{"name":"before-rewind"}`),
	})
	if !loaded.Accepted || loaded.Snapshot == nil {
		t.Fatalf("snapshot load = %#v", loaded)
	}
	loadedTimeline, err := historyStore.List(loaded.Snapshot.BattleID)
	if err != nil || len(loadedTimeline.Points) != 2 {
		t.Fatalf("loaded history timeline = %#v, %v", loadedTimeline, err)
	}
	review := history.HandleCommand(command.Command{
		BattleID: loaded.Snapshot.BattleID, ActorID: "blade", Type: command.TypeJumpHistory,
		Payload: json.RawMessage(`{"point_id":"` + loadedTimeline.Points[0].ID + `"}`),
	})
	if !review.Accepted || review.Snapshot == nil {
		t.Fatalf("loaded history rewind = %#v", review)
	}
	replaced := history.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeCommitHistory,
		Payload: json.RawMessage(`{"mode":"replace"}`),
	})
	if !replaced.Accepted {
		t.Fatalf("loaded history replacement = %#v", replaced)
	}
	active, err := historyStore.Branch(review.Snapshot.BattleID)
	if err != nil || active.Status != devhistory.BranchActive || active.CursorPointID != "" || active.BasePointID != "" {
		t.Fatalf("replacement retained rewind-only cursor metadata: %#v, %v", active, err)
	}
	continued := history.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeMarkHistory,
		Payload: json.RawMessage(`{"label":"Current after replacement","kind":"decision","presented_sequence":1}`),
	})
	if !continued.Accepted {
		t.Fatalf("continued loaded history mark = %#v", continued)
	}
	resaved := history.HandleCommand(command.Command{
		BattleID: review.Snapshot.BattleID, ActorID: "blade", Type: command.TypeSaveSnapshot,
		Payload: json.RawMessage(`{"name":"after-rewind"}`),
	})
	if !resaved.Accepted {
		t.Fatalf("resave after load and history replacement = %#v", resaved)
	}
	record, err := snapshotStore.Load("after-rewind")
	if err != nil || record.History == nil || record.Metadata.HistoryPointCount != 1 {
		t.Fatalf("resaved snapshot history = %#v, %v", record.Metadata, err)
	}
}

func snapshotAuthorityCheckpoint(t *testing.T, battleID string) repository.Checkpoint {
	t.Helper()
	battle, err := state.NewBattleFromSetup(battleID, state.BattleSetup{Actors: []state.ActorSetup{
		{ID: "blade", ControllerType: state.ControllerHuman, Deck: []string{"blade-1", "blade-2"}, Resources: state.ResourceState{EnergyPoints: 4}},
		{ID: "goblin", ControllerType: state.ControllerAI, Deck: []string{"goblin-1", "goblin-2"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	battle.Segment.Round = 2
	battle.Random.Cursor = 12
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendEvents(&checkpoint, []event.Event{{Type: event.TypeSegmentEntered, ActorID: "blade"}}); err != nil {
		t.Fatal(err)
	}
	return checkpoint
}
