package save_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/save"
)

func TestLoadRunPlayerStateLoadsActorIDAndDeck(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": ["strike", "guard", "focus"]
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}

	want := setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck: []string{"strike", "guard", "focus"},
		},
		RollPreferences: state.RollPreferences{
			StatusEffects: state.RollModeAutomatic,
			Offensive:     state.RollModeManual,
			Defensive:     state.RollModeManual,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadRunPlayerState() = %#v, want %#v", got, want)
	}
}

func TestLoadRunPlayerStateLoadsOptionalCardZones(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": ["strike", "guard"],
			"hand": ["opener"],
			"discard": ["spent"],
			"removed": ["lost"]
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}

	wantCards := setup.RunCardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(got.Cards, wantCards) {
		t.Fatalf("loaded card zones = %#v, want %#v", got.Cards, wantCards)
	}
}

func TestLoadRunPlayerStateLoadsCompleteCombatState(t *testing.T) {
	path := writeSaveFile(t, `{
		"schema_version": 1,
		"actor_id": "player",
		"character": {"id": "hero", "name": "Hero", "class": "paladin"},
		"resources": {
			"starting_hand_size": 4,
			"max_hand_size": 7,
			"starting_energy_points": 2,
			"max_energy_points": 10,
			"energy_points": 3
		},
		"health": {"model": "card_zones", "max_health": 3},
		"decklist": [{"card_id": "strike", "count": 3}],
		"cards": {"deck": [], "hand": ["strike"], "discard": ["strike"], "removed": ["strike"]},
		"dice_loadout": [{"dice_id": "d6", "count": 2}],
		"abilities": ["smite"],
		"statuses": [{"instance_id": "injury-1", "definition_id": "injury", "stacks": 1}],
		"tokens": [{"id": "blessing", "value": 2}],
		"roll_preferences": {
			"status_effects": "automatic",
			"offensive": "manual",
			"defensive": "manual"
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}
	if got.Character.ID != "hero" || got.Resources.EnergyPoints != 3 ||
		got.Health.MaxHealth != 3 || len(got.Decklist) != 1 ||
		len(got.DiceLoadout) != 1 || len(got.AbilityIDs) != 1 ||
		len(got.Statuses) != 1 || len(got.Tokens) != 1 ||
		got.RollPreferences.StatusEffects != state.RollModeAutomatic {
		t.Fatalf("loaded run player = %#v, want complete combat state", got)
	}
}

func TestLoadRunPlayerStateRejectsInvalidJSON(t *testing.T) {
	path := writeSaveFile(t, `{"actor_id": "player",`)

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !strings.Contains(err.Error(), "parse JSON") {
		t.Fatalf("LoadRunPlayerState() error = %q, want JSON parse context", err.Error())
	}
}

func TestLoadRunPlayerStateRejectsMissingActorID(t *testing.T) {
	path := writeSaveFile(t, `{
		"cards": {
			"deck": ["strike"]
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !errors.Is(err, save.ErrInvalidRunPlayerSave) {
		t.Fatalf("LoadRunPlayerState() error = %v, want ErrInvalidRunPlayerSave", err)
	}
	if !strings.Contains(err.Error(), "actor_id is required") {
		t.Fatalf("LoadRunPlayerState() error = %q, want actor_id requirement", err.Error())
	}
}

func TestLoadRunPlayerStateRejectsMissingCards(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player"
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !errors.Is(err, save.ErrInvalidRunPlayerSave) {
		t.Fatalf("LoadRunPlayerState() error = %v, want ErrInvalidRunPlayerSave", err)
	}
	if !strings.Contains(err.Error(), "cards is required") {
		t.Fatalf("LoadRunPlayerState() error = %q, want cards requirement", err.Error())
	}
}

func TestLoadRunPlayerStateRejectsMissingDeck(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !errors.Is(err, save.ErrInvalidRunPlayerSave) {
		t.Fatalf("LoadRunPlayerState() error = %v, want ErrInvalidRunPlayerSave", err)
	}
	if !strings.Contains(err.Error(), "cards.deck is required") {
		t.Fatalf("LoadRunPlayerState() error = %q, want deck requirement", err.Error())
	}
}

func TestLoadRunPlayerStateRejectsNoRemainingHealthCards(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": []
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !errors.Is(err, save.ErrInvalidRunPlayerSave) {
		t.Fatalf("LoadRunPlayerState() error = %v, want ErrInvalidRunPlayerSave", err)
	}
	if !strings.Contains(err.Error(), "at least one health card") {
		t.Fatalf("LoadRunPlayerState() error = %q, want health-card requirement", err.Error())
	}
}

func TestLoadRunPlayerStateAllowsEmptyDeckWhenHealthCardsRemain(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": [],
			"hand": ["opener"],
			"discard": ["spent"]
		}
	}`)

	got, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}
	if len(got.Cards.Deck) != 0 || len(got.Cards.Hand) != 1 || len(got.Cards.Discard) != 1 {
		t.Fatalf("loaded card zones = %#v, want empty deck with remaining health cards", got.Cards)
	}
}

func TestLoadRunPlayerStateReportsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-save.json")

	got, err := save.LoadRunPlayerState(path)
	if err == nil {
		t.Fatalf("LoadRunPlayerState() succeeded with state %#v", got)
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadRunPlayerState() error = %v, want os.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "read") {
		t.Fatalf("LoadRunPlayerState() error = %q, want read context", err.Error())
	}
}

func TestLoadRunPlayerStateReturnsIndependentCardZoneSlices(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": ["strike", "guard"],
			"hand": ["opener"],
			"discard": ["spent"],
			"removed": ["lost"]
		}
	}`)

	loaded, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}

	battleSetup, err := setup.BattleSetupFromRunPlayer(loaded)
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	loaded.Cards.Deck[0] = "mutated"
	loaded.Cards.Hand[0] = "mutated"
	loaded.Cards.Discard[0] = "mutated"
	loaded.Cards.Removed[0] = "mutated"

	wantActor := state.ActorSetup{
		ID: "player",
		Decklist: []state.DecklistEntry{
			{CardID: "strike", Count: 1}, {CardID: "guard", Count: 1},
			{CardID: "opener", Count: 1}, {CardID: "spent", Count: 1},
			{CardID: "lost", Count: 1},
		},
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
		RollPreferences: state.RollPreferences{
			StatusEffects: state.RollModeAutomatic,
			Offensive:     state.RollModeManual,
			Defensive:     state.RollModeManual,
		},
	}
	if !reflect.DeepEqual(battleSetup.Actors[0], wantActor) {
		t.Fatalf("actor setup after loaded state mutation = %#v, want %#v", battleSetup.Actors[0], wantActor)
	}

	reloaded, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() reload returned error: %v", err)
	}
	wantCards := setup.RunCardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(reloaded.Cards, wantCards) {
		t.Fatalf("reloaded card zones = %#v, want %#v", reloaded.Cards, wantCards)
	}
}

func TestLoadedRunPlayerStateCanCreateBattle(t *testing.T) {
	path := writeSaveFile(t, `{
		"actor_id": "player",
		"cards": {
			"deck": ["strike", "guard"],
			"hand": ["opener"],
			"discard": ["spent"],
			"removed": ["lost"]
		}
	}`)

	loaded, err := save.LoadRunPlayerState(path)
	if err != nil {
		t.Fatalf("LoadRunPlayerState() returned error: %v", err)
	}

	battleSetup, err := setup.BattleSetupFromRunPlayer(loaded)
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	battle, err := state.NewBattleFromSetup("battle-1", battleSetup)
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	wantCards := state.CardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantCards) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantCards)
	}
}

func writeSaveFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "run-player.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	return path
}
