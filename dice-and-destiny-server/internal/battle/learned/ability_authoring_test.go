package learned

import (
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryAbilityCanBeRenamedPublishedAssignedAndPlayed(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	for _, character := range []string{"adventurer", "venom", "curse", "blade_warden"} {
		t.Run(character, func(t *testing.T) {
			dir := t.TempDir()
			catalogs, err := CharacterCatalogs(root)
			if err != nil {
				t.Fatal(err)
			}
			lib := catalogs[character]
			revision := 0
			board := lib.Combatants[character].AbilityBoard
			for _, ids := range [][]string{board.Offensive, board.Defensive} {
				for i, id := range ids {
					a := lib.Abilities[id]
					a.ConfigurationVersion = 1
					a.ID = "authored_" + id
					a.Name = "Authored " + a.Name
					a.Presentation.RulesText = content.AbilityRules(a, lib)
					if _, err = content.SaveAuthoredAbility(dir, lib, &a, "", nil, revision); err != nil {
						t.Fatalf("%s: %v", id, err)
					}
					revision++
					lib.Abilities[a.ID] = a
					ids[i] = a.ID
				}
			}
			if _, err = content.SaveAuthoredAbility(dir, lib, nil, character, &board, revision); err != nil {
				t.Fatal(err)
			}
			s, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: 2})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ResetCharacter("authored-abilities-"+character, 47, "seat-a", false, character, true); err != nil {
				t.Fatal(err)
			}
			played := 0
			for step := 0; step < 1800 && !s.current.Terminal; step++ {
				if s.isModelSeat(s.current.ActorID) {
					_, err = s.AdvanceModel()
				} else {
					actions := s.current.Result.LegalActions
					if len(actions) == 0 {
						t.Fatal("no legal actions")
					}
					action := adventurerTestAction(actions, 3)
					var p map[string]any
					_ = json.Unmarshal(action.Payload, &p)
					if strings.HasPrefix(asString(p["ability_id"]), "authored_") {
						played++
					}
					encoded, _ := json.Marshal(aliasValue(commandMap(t, action), s.aliases(false)))
					_, err = s.SubmitHuman(string(encoded))
				}
				if err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
			}
			if !s.current.Terminal || s.current.TruncationReason != "" || played == 0 {
				t.Fatalf("incomplete battle or no authored activations: %d / %s", played, s.current.TruncationReason)
			}
			telemetry, _ := s.Telemetry()
			if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 {
				t.Fatalf("invalid commands: %+v", telemetry)
			}
		})
	}
}
func asString(v any) string { s, _ := v.(string); return s }
func TestAbilityAuthoringValidationAndRevision(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	a := lib.Abilities["adventurer_guard"]
	a.ConfigurationVersion = 1
	a.ID = "shared_guard"
	a.Name = "Shared Guard"
	a.Cost.Energy = 2
	a.Hooks = []content.AbilityHook{{Timing: "after_defense", Operations: []content.BattleOperation{{Type: "draw_cards", Target: "self", Amount: 1}, {Type: "apply_status", Target: "self", StatusID: "protect", StackCount: 2}}}}
	dir := t.TempDir()
	saved, err := content.SaveAuthoredAbility(dir, lib, &a, "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = content.SaveAuthoredAbility(dir, lib, &a, "", nil, 0); err == nil {
		t.Fatal("stale publication accepted")
	}
	raw, _ := json.Marshal(a)
	for name, change := range map[string]func(*content.BattleAbilityDefinition){"wrong checkpoint": func(x *content.BattleAbilityDefinition) { x.Hooks[0].Timing = "after_damage" }, "negative payment": func(x *content.BattleAbilityDefinition) {
		x.OptionalPayment = &content.AbilityPayment{StatusID: "catalyst", Stacks: -1, Prevention: 2}
	}, "unknown status": func(x *content.BattleAbilityDefinition) { x.Hooks[0].Operations[1].StatusID = "missing" }, "ignored damage": func(x *content.BattleAbilityDefinition) {
		x.Resolution.Operations = []content.BattleOperation{{Type: "deal_damage", Target: "enemy", Amount: 2}}
	}, "too many dice": func(x *content.BattleAbilityDefinition) { x.Resolution.Operations[0].DiceCount = 6 }} {
		t.Run(name, func(t *testing.T) {
			var bad content.BattleAbilityDefinition
			_ = json.Unmarshal(raw, &bad)
			change(&bad)
			if content.ValidateAuthoredAbility(bad, lib) == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	after, _ := content.ReadAuthoredAbilities(dir)
	if after.Revision != saved.Revision {
		t.Fatal("failed saves changed publication")
	}
}

func TestAuthoredAbilityBoardsSharedAcrossModesAndPinned(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	a := lib.Abilities["measured_strike"]
	a.ID = "shared_strike"
	a.Name = "Shared Strike"
	a.Cost.Energy = 1
	if _, err = content.SaveAuthoredAbility(dir, lib, &a, "", nil, 0); err != nil {
		t.Fatal(err)
	}
	lib.Abilities[a.ID] = a
	board := content.AbilityBoard{Offensive: []string{a.ID}, Defensive: []string{"adventurer_guard_plus"}}
	if _, err = content.SaveAuthoredAbility(dir, lib, nil, "adventurer", &board, 1); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"sandbox", "progression"} {
		s, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResetCharacterLoadout("authored-board-"+mode, 47, "seat-a", false, "adventurer", true, mode); err != nil {
			t.Fatal(err)
		}
		snap, _ := json.Marshal(s.current.Result.Snapshot)
		var view map[string]any
		_ = json.Unmarshal(snap, &view)
		pinned, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
		if !strings.Contains(string(pinned), `"shared_strike"`) {
			t.Fatal("authored definition missing from pinned catalog")
		}
		// Verify through public character state, not only presence in the catalog.
		actors := view["actors"].(map[string]any)
		found := false
		for _, value := range actors {
			actor := value.(map[string]any)
			if actor["definition_id"] == "adventurer" {
				ids := actor["offensive_abilities"].([]any)
				found = len(ids) == 1 && ids[0] == a.ID
			}
		}
		if !found {
			t.Fatalf("%s did not use assigned board", mode)
		}
		if mode == "sandbox" {
			a.Cost.Energy = 2
			if _, err = content.SaveAuthoredAbility(dir, lib, &a, "", nil, 2); err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
			if string(pinned) != string(after) {
				t.Fatal("publication modified active battle")
			}
		}
	}
}

func TestAbilityNativeValidationRejectsUnknownAndConflictingDefinitions(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, _ := CharacterCatalogs(root)
	lib := catalogs["adventurer"]
	base, _ := json.Marshal(lib.Abilities["adventurer_guard"])
	for _, name := range []string{"unknown field", "unknown effect", "empty defense", "actor effect on proposal", "offensive checkpoint", "status-stack repeat", "invalid incoming kind", "invalid face", "overlapping outcomes"} {
		t.Run(name, func(t *testing.T) {
			var a map[string]any
			_ = json.Unmarshal(base, &a)
			resolution := a["resolution"].(map[string]any)
			ops := resolution["operations"].([]any)
			op := ops[0].(map[string]any)
			switch name {
			case "unknown field":
				a["silent_option"] = true
			case "unknown effect":
				op["type"] = "missing"
			case "empty defense":
				resolution["operations"] = []any{}
			case "actor effect on proposal":
				resolution["operations"] = []any{map[string]any{"type": "draw_cards", "target": "selected_proposal", "amount": 1}}
			case "offensive checkpoint":
				a["hooks"] = []any{map[string]any{"timing": "after_damage", "operations": []any{map[string]any{"type": "draw_cards", "target": "self", "amount": 1}}}}
			case "status-stack repeat":
				op["repeat"] = "one_per_status_stack"
			case "invalid incoming kind":
				a["selection"].(map[string]any)["allowed_proposal_types"] = []any{"card"}
			case "invalid face":
				op["outcomes"].([]any)[0].(map[string]any)["faces"] = []any{9}
			case "overlapping outcomes":
				op["outcomes"].([]any)[1].(map[string]any)["faces"] = []any{1, 4, 5}
			}
			req, _ := json.Marshal(map[string]any{"op": "validate_ability", "content_root": root, "loadout_root": dir, "ability": a})
			var response map[string]any
			_ = json.Unmarshal([]byte(HandleRuntimeRequest(string(req))), &response)
			if response["ok"] == true {
				t.Fatalf("accepted %s", name)
			}
		})
	}
}
