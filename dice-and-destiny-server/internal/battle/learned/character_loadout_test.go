package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
)

func TestOwnedDeckPersistenceAndValidation(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	call := func(op, character string, deck any) map[string]any {
		t.Helper()
		request, _ := json.Marshal(map[string]any{"op": op, "content_root": root, "loadout_root": dir, "character": character, "decklist": deck})
		var response map[string]any
		if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(request))), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	original := []loadout.Entry{{CardID: "brace", Count: 2}, {CardID: "tip_it", Count: 3}}
	if result := call("save_character_deck", "adventurer", original); result["ok"] != true {
		t.Fatal(result)
	}
	saved, err := loadout.Read(dir, "adventurer", catalogs["adventurer"].Cards)
	if err != nil || !reflect.DeepEqual(saved, original) {
		t.Fatalf("read: %v %v", saved, err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "progression", "adventurer.json"))
	for _, deck := range []any{
		[]loadout.Entry{}, []loadout.Entry{{CardID: "missing", Count: 1}}, []loadout.Entry{{CardID: "brace", Count: -1}}, []loadout.Entry{{CardID: "brace", Count: 0}}, []loadout.Entry{{CardID: "brace", Count: 21}}, []loadout.Entry{{CardID: "brace", Count: 1}, {CardID: "brace", Count: 2}},
		[]map[string]any{{"card_id": "brace", "count": 1.5}},
		[]loadout.Entry{{CardID: "brace", Count: 20}, {CardID: "brace_plus", Count: 20}, {CardID: "nudge", Count: 20}, {CardID: "try_again", Count: 20}, {CardID: "strong_swing", Count: 20}, {CardID: "take_stock", Count: 1}},
	} {
		if response := call("save_character_deck", "adventurer", deck); response["ok"] != false {
			t.Fatalf("accepted invalid deck: %v", response)
		}
		after, _ := os.ReadFile(filepath.Join(dir, "progression", "adventurer.json"))
		if string(before) != string(after) {
			t.Fatal("invalid save overwrote deck")
		}
	}
	if response := call("save_character_deck", "../adventurer", original); response["ok"] != false {
		t.Fatal("accepted path traversal")
	}
	response := call("character_catalogs", "", nil)
	view := response["result"].(map[string]any)
	adv := view["adventurer"].(map[string]any)
	venom, err := loadout.Read(dir, "venom", catalogs["venom"].Cards)
	if err != nil || adv["owned_decklist"] == nil || reflect.DeepEqual(venom, original) {
		t.Fatal("character decks not isolated")
	}
	template := adv["combatants"].(map[string]any)["adventurer"].(map[string]any)["decklist"].([]any)
	if len(template) != 7 {
		t.Fatal("template changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "progression", "adventurer.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	response = call("character_catalogs", "", nil)
	if response["result"].(map[string]any)["adventurer"].(map[string]any)["loadout_error"] == nil {
		t.Fatal("corrupt save hidden")
	}
	if result := call("save_character_deck", "adventurer", original); result["ok"] != false {
		t.Fatal("corrupt shared XP ledger was silently reset")
	}
}

func TestOwnedDecksPinToHumanBattleAndReplay(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	economy, _ := loadout.LoadEconomy(root, catalogs)
	for character, catalog := range catalogs {
		deck := []loadout.Entry{{CardID: "tip_it", Count: 3}, {CardID: "battle_focus", Count: 2}}
		if _, err := loadout.WriteSharedDeck(dir, character, deck, economy, catalog); err != nil {
			t.Fatal(err)
		}
		s, err := NewSession(SessionConfig{ContentRoot: root, RunStateRoot: t.TempDir(), LoadoutRoot: dir, OpponentDefinition: "drowned_oracle_brine_mask"})
		if err != nil {
			t.Fatal(err)
		}
		for _, seat := range []string{"seat-a", "seat-b"} {
			if _, err = s.ResetCharacter("owned-"+character+seat, 71, seat, false, character, true); err != nil {
				t.Fatal(err)
			}
			snapshot := s.current.Result.Snapshot
			actor := snapshot.Actors[seat]
			if actor.MaxHealth != 5 {
				t.Fatalf("%s wrong health: %+v", character, actor.MaxHealth)
			}
			other := snapshot.Actors[otherSeat(seat)]
			if other.MaxHealth != 16 {
				t.Fatal("custom deck altered opponent")
			}
			// Applying another deck cannot rewrite the active battle or its replay.
			if _, err = loadout.WriteSharedDeck(dir, character, []loadout.Entry{{CardID: "tip_it", Count: 1}}, economy, catalog); err != nil {
				t.Fatal(err)
			}
			if s.current.Result.Snapshot.Actors[seat].MaxHealth != 5 {
				t.Fatal("active battle changed")
			}
			finishOwnedBattle(t, s)
			record := *s.current.Replay
			if len(record.SeatDecklists) != 1 || len(record.SeatDecklists[seat]) != 2 {
				t.Fatal("replay missing owned deck")
			}
			env, err := mlsim.New(mlsim.Config{ContentRoot: root, RunStateRoot: t.TempDir(), IncludeContentCatalog: true})
			if err != nil {
				t.Fatal(err)
			}
			replay, err := env.Replay(record)
			if err != nil {
				t.Fatal(err)
			}
			if replay.Result.Snapshot.Actors[seat].MaxHealth != 5 {
				t.Fatal("replay read changed saved deck")
			}
			// Restore before next seat.
			if _, err = loadout.WriteSharedDeck(dir, character, deck, economy, catalog); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestOwnedDeckMirrorKeepsEnemyTemplate(t *testing.T) {
	root := testServerRoot(t)
	dir := t.TempDir()
	catalogs, _ := CharacterCatalogs(filepath.Join(root, "content"))
	if _, err := loadout.Write(dir, "blade_warden", []loadout.Entry{{CardID: "tip_it", Count: 4}}, catalogs["blade_warden"].Cards); err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(SessionConfig{ContentRoot: filepath.Join(root, "content"), RunStateRoot: t.TempDir(), LoadoutRoot: dir, ModelPath: filepath.Join(root, "../dice-and-destiny-client/models/learned/blade-warden-seed-11-final-v1.json")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResetCharacter("owned-mirror", 17, "seat-b", false, "blade_warden", true); err != nil {
		t.Fatal(err)
	}
	actors := s.current.Result.Snapshot.Actors
	expected := 0
	for _, e := range catalogs["blade_warden"].Combatants["blade_warden"].Decklist {
		expected += e.Count
	}
	if actors["seat-b"].MaxHealth != 4 || actors["seat-a"].MaxHealth != expected {
		t.Fatal("mirror deck contamination")
	}
	finishOwnedBattle(t, s)
}

func finishOwnedBattle(t *testing.T, s *Session) {
	t.Helper()
	var err error
	for step := 0; step < 1200 && !s.current.Terminal; step++ {
		if s.isModelSeat(s.current.ActorID) {
			_, err = s.AdvanceModel()
		} else {
			actions := s.current.Result.LegalActions
			if len(actions) == 0 {
				t.Fatal("no legal actions")
			}
			cmd := adventurerTestAction(actions, 3)
			encoded, _ := json.Marshal(aliasValue(commandMap(t, cmd), s.aliases(false)))
			_, err = s.SubmitHuman(string(encoded))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !s.current.Terminal {
		t.Fatal("custom deck battle did not finish")
	}
}
