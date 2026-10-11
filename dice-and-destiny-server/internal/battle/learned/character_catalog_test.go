package learned

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/content"
)

func TestCharacterCatalogMatchesBattleConfiguration(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 5 {
		t.Fatal("expected five playable characters")
	}
	assembler := battle.NewFileParticipantAssembler(root, t.TempDir())
	for id, catalog := range catalogs {
		setup, err := assembler.AssembleParticipants([]participant.Participant{{InstanceID: "preview", DefinitionID: id}})
		if err != nil {
			t.Fatal(err)
		}
		var pinned content.BattleLibrary
		if err := json.Unmarshal(setup.SettledCatalog, &pinned); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(catalog.Combatants[id], pinned.Combatants[id]) {
			t.Fatalf("%s preview loadout differs from battle", id)
		}
		for _, entry := range catalog.Combatants[id].Decklist {
			a, _ := json.Marshal(catalog.Cards[entry.CardID])
			b, _ := json.Marshal(pinned.Cards[entry.CardID])
			if string(a) != string(b) {
				t.Fatalf("%s card differs from battle", entry.CardID)
			}
		}
	}
	learnedRuntime.Lock()
	previous := learnedRuntime.session
	learnedRuntime.Unlock()
	request, _ := json.Marshal(map[string]string{"op": "character_catalogs", "content_root": root})
	var result map[string]any
	if err := json.Unmarshal([]byte(HandleRuntimeRequest(string(request))), &result); err != nil || result["ok"] != true {
		t.Fatalf("read-only catalog: %v %v", result, err)
	}
	adventurer := result["result"].(map[string]any)["adventurer"].(map[string]any)["combatants"].(map[string]any)["adventurer"].(map[string]any)
	entry := adventurer["decklist"].([]any)[0].(map[string]any)
	if entry["card_id"] != "steady_guard" || entry["count"] != float64(3) {
		t.Fatalf("incorrect client loadout schema: %v", entry)
	}
	learnedRuntime.Lock()
	unchanged := learnedRuntime.session == previous
	learnedRuntime.Unlock()
	if !unchanged {
		t.Fatal("catalog read changed runtime session")
	}
	if _, err := CharacterCatalogs(""); err == nil {
		t.Fatal("missing root accepted")
	}
}
