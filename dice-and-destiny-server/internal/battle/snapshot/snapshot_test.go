package snapshot_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestFromBattleIncludesBattleIDCurrentSegmentAndRound(t *testing.T) {
	battle := state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.Income,
			Round:   3,
		},
	}

	got := snapshot.FromBattle(battle)
	want := snapshot.Battle{
		BattleID: "battle-1",
		Segment:  segment.Income,
		Round:    3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromBattle() = %#v, want %#v", got, want)
	}
}

func TestFromBattleIncludesActorEnergyPoints(t *testing.T) {
	battle := state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.Income,
			Round:   3,
		},
		Actors: map[string]state.ActorState{
			"player": {
				EnergyPoints: 2,
				Cards: state.CardZones{
					Deck:    []string{"draw-next", "draw-later"},
					Hand:    []string{"strike"},
					Discard: []string{"spent"},
					Removed: []string{"lost"},
				},
			},
		},
	}

	got := snapshot.FromBattle(battle)
	want := snapshot.Battle{
		BattleID: "battle-1",
		Segment:  segment.Income,
		Round:    3,
		Actors: map[string]snapshot.Actor{
			"player": {
				EnergyPoints: 2,
				HandCount:    1,
				DeckCount:    2,
				DiscardCount: 1,
				RemovedCount: 1,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromBattle() = %#v, want %#v", got, want)
	}
}

func TestFromBattleForViewerIncludesOwnHandCardIDs(t *testing.T) {
	battle := battleWithCardVisibilityState()

	got := snapshot.FromBattleForViewer(battle, "player-1")
	want := snapshot.Battle{
		BattleID:      "battle-1",
		Segment:       segment.Income,
		Round:         1,
		ViewerActorID: "player-1",
		Actors: map[string]snapshot.Actor{
			"player-1": {
				EnergyPoints: 2,
				Hand:         []string{"strike", "guard"},
				HandCount:    2,
				DeckCount:    1,
				DiscardCount: 1,
				RemovedCount: 1,
			},
			"player-2": {
				EnergyPoints: 1,
				HandCount:    2,
				DeckCount:    1,
				DiscardCount: 1,
				RemovedCount: 1,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromBattleForViewer() = %#v, want %#v", got, want)
	}
}

func TestFromBattleForViewerHidesOpponentHandCardIDs(t *testing.T) {
	battle := battleWithCardVisibilityState()

	got := snapshot.FromBattleForViewer(battle, "player-1")
	opponent := got.Actors["player-2"]

	if len(opponent.Hand) != 0 {
		t.Fatalf("opponent hand = %#v, want hidden card IDs", opponent.Hand)
	}
	if opponent.HandCount != 2 {
		t.Fatalf("opponent hand count = %d, want 2", opponent.HandCount)
	}
}

func TestFromBattleForViewerCopiesVisibleHandCardIDs(t *testing.T) {
	battle := battleWithCardVisibilityState()

	got := snapshot.FromBattleForViewer(battle, "player-1")
	viewer := got.Actors["player-1"]
	viewer.Hand[0] = "mutated"

	wantHand := []string{"strike", "guard"}
	if !reflect.DeepEqual(battle.Actors["player-1"].Cards.Hand, wantHand) {
		t.Fatalf("battle hand after snapshot mutation = %#v, want %#v", battle.Actors["player-1"].Cards.Hand, wantHand)
	}
}

func TestBattleSnapshotJSONShape(t *testing.T) {
	got, err := json.Marshal(snapshot.FromBattleForViewer(battleWithCardVisibilityState(), "player-1"))
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	want := `{"battle_id":"battle-1","segment":"income","round":1,"viewer_actor_id":"player-1","actors":{"player-1":{"energy_points":2,"hand":["strike","guard"],"hand_count":2,"deck_count":1,"discard_count":1,"removed_count":1},"player-2":{"energy_points":1,"hand_count":2,"deck_count":1,"discard_count":1,"removed_count":1}}}`
	if string(got) != want {
		t.Fatalf("snapshot JSON = %s, want %s", got, want)
	}
}

func TestBattleSnapshotRoundTripsThroughJSON(t *testing.T) {
	want := snapshot.Battle{
		BattleID:      "battle-1",
		Segment:       segment.Defensive,
		Round:         2,
		ViewerActorID: "player",
		Actors: map[string]snapshot.Actor{
			"player": {
				EnergyPoints: 2,
				Hand:         []string{"strike"},
				HandCount:    1,
				DeckCount:    2,
				DiscardCount: 3,
				RemovedCount: 4,
			},
		},
	}

	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var got snapshot.Battle
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-tripped snapshot = %#v, want %#v", got, want)
	}
}

func TestDefenseReactionSnapshotRevealsEverySelectedDefense(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-defense",
		Segment: segment.State{Current: segment.Defensive, Round: 2},
		Settled: &state.SettledRuntime{
			Stage: "defense_reaction",
			DefenseSelections: map[string]state.SettledDefense{
				"player": {ActorID: "player", AbilityID: "basic_defense", SourceID: "enemy-attack", RolledFace: 4},
				"enemy":  {ActorID: "enemy", AbilityID: "protect", SourceID: "player-attack"},
			},
		},
	}

	got := snapshot.FromBattleForViewer(battle, "player")
	if len(got.SettledDefenses) != 2 || got.SettledDefenses["player"].RolledFace != 4 || got.SettledDefenses["enemy"].AbilityID != "protect" {
		t.Fatalf("defense reaction snapshot = %#v, want both revealed defenses", got.SettledDefenses)
	}

	battle.Settled.Stage = "defense_selection"
	if hidden := snapshot.FromBattleForViewer(battle, "player").SettledDefenses; len(hidden) != 0 {
		t.Fatalf("defenses leaked before reveal: %#v", hidden)
	}
}

func battleWithCardVisibilityState() state.Battle {
	return state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.Income,
			Round:   1,
		},
		Actors: map[string]state.ActorState{
			"player-1": {
				EnergyPoints: 2,
				Cards: state.CardZones{
					Deck:    []string{"focus"},
					Hand:    []string{"strike", "guard"},
					Discard: []string{"spent"},
					Removed: []string{"lost"},
				},
			},
			"player-2": {
				EnergyPoints: 1,
				Cards: state.CardZones{
					Deck:    []string{"counter"},
					Hand:    []string{"curse", "hex"},
					Discard: []string{"spent-2"},
					Removed: []string{"lost-2"},
				},
			},
		},
	}
}

func TestSnapshotProductionCodeDoesNotImportPresentationPackages(t *testing.T) {
	forbiddenImportFragments := []string{
		"dice-and-destiny-client",
		"gdextension",
		"godot",
		"/ui",
		"ui/",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("finding Go files: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing imports in %s: %v", file, err)
		}

		for _, imp := range parsed.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquoting import path %s in %s: %v", imp.Path.Value, file, err)
			}

			lowerImportPath := strings.ToLower(importPath)
			for _, forbidden := range forbiddenImportFragments {
				if strings.Contains(lowerImportPath, forbidden) {
					t.Fatalf("snapshot production file %s imports forbidden package %q", file, importPath)
				}
			}
		}
	}
}
