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

func TestFromBattleForViewerPublishesOnlyOwnerCardZoneComposition(t *testing.T) {
	battle := battleWithCardVisibilityState()
	battle.Settled = &state.SettledRuntime{
		Actors: map[string]state.SettledActorRuntime{
			"player-1": {CardInstances: map[string]state.CardInstance{
				"focus": {InstanceID: "focus", DefinitionID: "focus"},
				"spent": {InstanceID: "spent", DefinitionID: "guard"},
				"lost":  {InstanceID: "lost", DefinitionID: "focus"},
			}},
			"player-2": {CardInstances: map[string]state.CardInstance{
				"hidden-draw": {InstanceID: "hidden-draw", DefinitionID: "secret"},
			}},
		},
	}
	got := snapshot.FromBattleForViewer(battle, "player-1")
	owner := got.Actors["player-1"]
	if !reflect.DeepEqual(owner.DeckComposition, map[string]int{"focus": 1}) ||
		!reflect.DeepEqual(owner.DiscardComposition, map[string]int{"guard": 1}) ||
		!reflect.DeepEqual(owner.RemovedComposition, map[string]int{"focus": 1}) {
		t.Fatalf("owner compositions = %#v/%#v/%#v", owner.DeckComposition, owner.DiscardComposition, owner.RemovedComposition)
	}
	opponent := got.Actors["player-2"]
	if opponent.DeckComposition != nil || opponent.DiscardComposition != nil || opponent.RemovedComposition != nil {
		t.Fatalf("opponent hidden composition leaked: %#v", opponent)
	}
}

func TestSettledOpponentDiceAppearOnlyAfterPlanningReveal(t *testing.T) {
	battle := state.Battle{
		ID:      "revealed-dice",
		Segment: segment.State{Current: segment.Offensive, Round: 2},
		Actors: map[string]state.ActorState{
			"seat-a": {},
			"seat-b": {},
		},
		Settled: &state.SettledRuntime{
			Stage: "planning",
			Actors: map[string]state.SettledActorRuntime{
				"seat-a": {FinalDice: []state.RolledDie{{DieID: "d6", Face: 6, Value: 6, Symbols: []string{"sword"}}}, RollsUsed: 3, MaxRolls: 3},
				"seat-b": {},
			},
		},
	}
	if hidden := snapshot.FromBattleForViewer(battle, "seat-b").Actors["seat-a"].Dice; hidden != nil {
		t.Fatalf("opponent dice leaked during planning: %#v", hidden)
	}
	battle.Settled.Stage = "offensive_reaction"
	revealed := snapshot.FromBattleForViewer(battle, "seat-b").Actors["seat-a"].Dice
	if revealed == nil || len(revealed.Dice) != 1 || revealed.Dice[0].Face != 6 || !revealed.Complete {
		t.Fatalf("revealed opponent dice = %#v", revealed)
	}
}

func TestSettledPlanningSnapshotUsesOpponentPublicBaseline(t *testing.T) {
	battle := state.Battle{
		ID:      "hidden-planning",
		Segment: segment.State{Current: segment.Offensive, Round: 1},
		Actors: map[string]state.ActorState{
			"seat-a": {Controller: state.ControllerExternal, Resources: state.ResourceState{EnergyPoints: 3}, Cards: state.CardZones{Deck: []string{"d1"}, Hand: []string{"h1", "h2"}, Discard: []string{"x1"}}},
			"seat-b": {Controller: state.ControllerExternal},
		},
		Settled: &state.SettledRuntime{
			Stage: "planning",
			Actors: map[string]state.SettledActorRuntime{
				"seat-a": {AbilityModifiers: []state.RuntimeAbilityModifier{{SourceCardInstanceID: "hidden-card", AbilityID: "sword_cut", BonusID: "hidden-bonus"}}},
				"seat-b": {},
			},
			PlanningPublic: map[string]state.SettledPlanningPublicState{
				"seat-a": {EnergyPoints: 2, HandCount: 4, DeckCount: 16},
			},
		},
	}
	opponentView := snapshot.FromBattleForViewer(battle, "seat-b").Actors["seat-a"]
	if opponentView.EnergyPoints != 2 || opponentView.HandCount != 4 || opponentView.DeckCount != 16 || len(opponentView.AbilityModifiers) != 0 {
		t.Fatalf("opponent planning baseline leaked: %#v", opponentView)
	}
	ownerView := snapshot.FromBattleForViewer(battle, "seat-a").Actors["seat-a"]
	if ownerView.EnergyPoints != 3 || ownerView.HandCount != 2 || len(ownerView.AbilityModifiers) != 1 {
		t.Fatalf("owner planning view lost private state: %#v", ownerView)
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

func TestEffectRollSnapshotHidesEnemyUntilReactionRevealInEverySegment(t *testing.T) {
	for _, parent := range []segment.Segment{segment.OngoingEffects, segment.Offensive, segment.Defensive, segment.DamageResolution} {
		battle := state.Battle{
			ID:      "effects",
			Segment: segment.State{Current: parent, Round: 2},
			Settled: &state.SettledRuntime{
				Stage: "status_roll",
				TriggerBatch: &state.SettledTriggerBatch{Rolls: []state.SettledEffectRoll{
					{ActorID: "player", StatusID: "poison", Die: state.RolledDie{DieID: "standard_d6", Face: 2, Value: 2, Symbols: []string{"cross"}}, Resolved: true},
					{ActorID: "enemy", StatusID: "poison", Die: state.RolledDie{DieID: "standard_d6", Face: 6}, Resolved: true},
				}},
			},
		}

		preReveal := snapshot.FromBattleForViewer(battle, "player")
		if len(preReveal.SettledEffectRolls) != 1 || preReveal.SettledEffectRolls[0].ActorID != "player" || preReveal.SettledEffectRolls[0].Die.Face != 0 || !preReveal.SettledEffectRolls[0].Resolved || len(preReveal.SettledEffectRolls[0].Die.Symbols) != 0 {
			t.Fatalf("pre-reveal effect rolls=%#v, want only the player's resolved-but-hidden die", preReveal.SettledEffectRolls)
		}
		battle.Settled.Stage = "status_roll_reaction"
		revealed := snapshot.FromBattleForViewer(battle, "player")
		if len(revealed.SettledEffectRolls) != 2 || revealed.SettledEffectRolls[1].ActorID != "enemy" || revealed.SettledEffectRolls[1].Die.Face != 6 {
			t.Fatalf("revealed effect rolls=%#v, want both actors' results", revealed.SettledEffectRolls)
		}
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

	battle.Settled.Stage = "defense_roll"
	rolling := snapshot.FromBattleForViewer(battle, "player").SettledDefenses
	if len(rolling) != 1 || rolling["player"].AbilityID != "basic_defense" {
		t.Fatalf("defense roll snapshot = %#v, want only the viewer's selected defense", rolling)
	}
	if _, leaked := rolling["enemy"]; leaked {
		t.Fatalf("enemy defense leaked before reveal: %#v", rolling)
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
