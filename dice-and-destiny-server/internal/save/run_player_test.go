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

func TestLoadRunPlayerStateRejectsEmptyDeck(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cards.deck must not be empty") {
		t.Fatalf("LoadRunPlayerState() error = %q, want explicit empty deck behavior", err.Error())
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
		ID:      "player",
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
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
