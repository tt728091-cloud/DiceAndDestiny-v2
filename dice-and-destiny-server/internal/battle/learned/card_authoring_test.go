package learned

import (
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryGeneralCardHasEditableProgram(t *testing.T) {
	root := filepath.Join("..", "..", "..", "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	ids := []string{"antidote", "battle_focus", "emergency_ward", "loaded_die", "sharpen_blade", "tip_it", "brace", "brace_plus", "nudge", "try_again", "strong_swing", "take_stock", "second_wind", "matchmaker", "turn_the_die", "disrupt", "second_guard", "reclaim", "reinforce", "triage", "dispel"}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			c, err := content.EditableGeneralCard(lib.Cards[id])
			if err != nil {
				t.Fatal(err)
			}
			c.ID = "renamed_" + id
			c.Name = "Renamed " + c.Name
			c = content.PrepareProgramCard(c, &lib)
			if err = content.ValidateCardProgram(c, lib); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestAuthoringPublishReloadAndRevision(t *testing.T) {
	root := filepath.Join("..", "..", "..", "content")
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, err := content.EditableGeneralCard(lib.Cards["brace"])
	if err != nil {
		t.Fatal(err)
	}
	c.ID = "custom_brace"
	c.Name = "Custom Brace"
	c.Cost.Energy = 2
	c.Program.Steps[0].Params["amount"] = 5
	c.Economy = &content.CardEconomy{Buy: 25, Sell: 12, CopyLimit: 3}
	dir := t.TempDir()
	saved, err := content.SaveAuthoredCard(dir, lib, c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 {
		t.Fatal("revision not advanced")
	}
	if _, err = content.SaveAuthoredCard(dir, lib, c, 0); err == nil {
		t.Fatal("stale publication allowed")
	}
	loaded, err := CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded["adventurer"].Cards[c.ID]
	if got.Program == nil || got.Cost.Energy != 2 || got.Economy.Sell != 12 {
		t.Fatal("authored parameters did not survive load")
	}
	if _, err = json.Marshal(got); err != nil {
		t.Fatal(err)
	}
}
func TestProgramRejectsIgnoredParameters(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, _ := content.EditableGeneralCard(lib.Cards["triage"])
	c.Program.Steps[0].Params["amount"] = 3
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("ignored amount accepted")
	}
	delete(c.Program.Steps[0].Params, "amount")
	c.Program.Steps[0].Params["destination"] = "missing"
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("unknown destination accepted")
	}
}

func TestAuthoredDeckFullBattlesAndPinnedRevision(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	rev := 0
	deck := []loadout.Entry{}
	for _, id := range []string{"brace", "brace_plus", "nudge", "try_again", "strong_swing", "take_stock", "second_wind", "matchmaker", "turn_the_die", "battle_focus", "loaded_die", "emergency_ward"} {
		c, err := content.EditableGeneralCard(lib.Cards[id])
		if err != nil {
			t.Fatal(err)
		}
		c.ID = "authored_" + id
		c.Name = "Authored " + c.Name
		if _, err = content.SaveAuthoredCard(dir, lib, c, rev); err != nil {
			t.Fatal(err)
		}
		rev++
		deck = append(deck, loadout.Entry{CardID: c.ID, Count: 1})
	}
	catalogs, err = CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadout.WriteSharedDeck(dir, "adventurer", deck, economy, catalogs["adventurer"]); err != nil {
		t.Fatal(err)
	}
	played := 0
	for _, count := range []int{1, 2} {
		s, err := NewSession(SessionConfig{ContentRoot: root, LoadoutRoot: dir, RunStateRoot: t.TempDir(), OpponentDefinition: "drowned_oracle_brine_mask", OpponentCount: count})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResetCharacter(fmt.Sprintf("authored-%d", count), uint64(11+count), "seat-a", false, "adventurer", true); err != nil {
			t.Fatal(err)
		}
		pinned, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
		if !strings.Contains(string(pinned), "authored_brace") {
			t.Fatal("new battle did not pin authored definitions")
		}
		if count == 1 {
			c := catalogs["adventurer"].Cards["authored_brace"]
			c.Cost.Energy = 9
			if _, err = content.SaveAuthoredCard(dir, lib, c, rev); err != nil {
				t.Fatal(err)
			}
			rev++
			after, _ := json.Marshal(s.current.Result.Snapshot.ContentCatalog)
			if string(pinned) != string(after) {
				t.Fatal("publish changed in-progress battle")
			}
		}
		for step := 0; step < 1200 && !s.current.Terminal; step++ {
			if s.isModelSeat(s.current.ActorID) {
				_, err = s.AdvanceModel()
			} else {
				actions := s.current.Result.LegalActions
				if len(actions) == 0 {
					t.Fatal("no legal actions")
				}
				action := actions[0]
				foundProgram := false
				// Prioritize actual authored card commands; never cancel a selected program.
				for _, a := range actions {
					var p map[string]any
					_ = json.Unmarshal(a.Payload, &p)
					key, _ := p["status_id"].(string)
					if c, ok := p["commitment"].(map[string]any); ok {
						key, _ = c["choice_id"].(string)
					}
					var choice map[string]any
					if json.Unmarshal([]byte(key), &choice) == nil && choice["verb"] != nil && choice["verb"] != "cancel" {
						action = a
						foundProgram = true
						played++
						break
					}
				}
				if !foundProgram {
					action = adventurerTestAction(actions, 3)
				}
				encoded, _ := json.Marshal(aliasValue(commandMap(t, action), s.aliases(false)))
				_, err = s.SubmitHuman(string(encoded))
			}
			if err != nil {
				t.Fatalf("%d enemies step %d stage %s: %v", count, step, s.current.Result.Snapshot.Stage, err)
			}
		}
		if !s.current.Terminal || s.current.TruncationReason != "" {
			t.Fatalf("authored deck did not complete cleanly: %v", s.current.TruncationReason)
		}
		telemetry, _ := s.Telemetry()
		if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 {
			t.Fatalf("invalid actions: %+v", telemetry)
		}
	}
	if played < 5 {
		t.Fatalf("insufficient authored card coverage: %d", played)
	}
}

func TestAuthoredEconomyBranchesSaleAndLimit(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, _ := content.EditableGeneralCard(lib.Cards["brace"])
	c.ID = "branching_ward"
	c.Name = "Branching Ward"
	c.Economy = &content.CardEconomy{Buy: 20, Sell: 7, CopyLimit: 1, Upgrades: []content.CardUpgrade{{To: "brace_plus", XP: 5}, {To: "emergency_ward", XP: 8}}}
	if _, err = content.SaveAuthoredCard(dir, lib, c, 0); err != nil {
		t.Fatal(err)
	}
	catalogs, err = CharacterCatalogs(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	lib = catalogs["adventurer"]
	e, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadout.ReadProgress(dir, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	start := p.XP
	trade := func(kind, to string, cost int) error {
		var next loadout.Progress
		next, err = loadout.Buy(dir, "adventurer", e, lib, loadout.Purchase{Kind: kind, ID: c.ID, TargetID: to, Revision: p.Revision, ExpectedCost: cost})
		if err == nil {
			p = next
		}
		return err
	}
	if err = trade("buy_card", "", 20); err != nil {
		t.Fatal(err)
	}
	if err = trade("buy_card", "", 20); err == nil {
		t.Fatal("copy limit ignored")
	}
	if err = trade("sell_card", "", 7); err != nil {
		t.Fatal(err)
	}
	if p.XP != start-13 {
		t.Fatal("independent sale price ignored")
	}
	if err = trade("buy_card", "", 20); err != nil {
		t.Fatal(err)
	}
	if err = trade("upgrade_card", "", 5); err == nil {
		t.Fatal("ambiguous upgrade accepted")
	}
	if err = trade("upgrade_card", "brace_plus", 5); err != nil {
		t.Fatal(err)
	}
}

func TestWorkshopRejectsPublicationThatBreaksSavedDeck(t *testing.T) {
	root := filepath.Join(testServerRoot(t), "content")
	dir := t.TempDir()
	catalogs, err := CharacterCatalogs(root)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(root, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadout.ReadProgress(dir, "adventurer", economy, catalogs["adventurer"]); err != nil {
		t.Fatal(err)
	}
	c, _ := content.EditableGeneralCard(catalogs["adventurer"].Cards["brace"])
	for _, eco := range []content.CardEconomy{{Buy: 9999, Sell: 1, CopyLimit: 20}, {Buy: 10, Sell: 10, CopyLimit: 1}} {
		c.Economy = &eco
		raw, _ := json.Marshal(map[string]any{"op": "publish_card", "content_root": root, "loadout_root": dir, "catalog_revision": 0, "card": c})
		var response map[string]any
		_ = json.Unmarshal([]byte(HandleRuntimeRequest(string(raw))), &response)
		if response["ok"] != false {
			t.Fatal("publication stranded saved character", response)
		}
		store, _ := content.ReadAuthoredCards(dir)
		if store.Revision != 0 {
			t.Fatal("rejected publication wrote definitions")
		}
	}
}

func TestGuidedCardCapabilityMatrix(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	base, _ := content.EditableGeneralCard(lib.Cards["brace"])
	base.Program.RollRequirement = "any"
	valid, invalid := 0, 0
	makeStep := func(effect string) content.CardStep {
		s := content.CardStep{Effect: effect, Target: content.CardTarget{Owner: "self", Mode: "one", Count: 2, Selection: "choose"}, Params: map[string]any{}}
		if content.CardCapabilities()[effect].Target == "card" {
			s.Target.Zones = []string{"hand"}
		}
		if effect == "sacrifice" {
			s.Target.Mode = "exact"
		}
		if effect == "apply_status" {
			s.Params["status_id"] = "protect"
		}
		if effect == "choice" {
			s.Choices = []content.CardOption{{Name: "Option", Steps: []content.CardStep{{Effect: "energy", Target: content.CardTarget{Owner: "self", Mode: "one", Selection: "choose"}, Params: map[string]any{"amount": 2}}}}}
		}
		return s
	}
	for effect, spec := range content.CardCapabilities() {
		rules := content.CardTargetCapabilities(effect)
		for _, owner := range []string{"self", "enemy", "any"} {
			for _, mode := range []string{"one", "exact", "up_to", "all"} {
				for _, selection := range []string{"choose", "random"} {
					for _, window := range content.CardWindows {
						s := makeStep(effect)
						s.Target.Owner = owner
						s.Target.Mode = mode
						s.Target.Selection = selection
						c := base
						p := *base.Program
						c.Program = &p
						p.Windows = []string{window}
						p.Steps = []content.CardStep{s}
						want := content.ProgramContains(content.CardStepWindows(s), window)
						if spec.Target != "none" {
							want = want && content.ProgramContains(rules.Owners, owner) && content.ProgramContains(rules.Modes, mode) && content.ProgramContains(rules.Selections, selection)
						}
						if owner == "self" && mode == "exact" && rules.SelfCountMax > 0 && s.Target.Count > rules.SelfCountMax {
							want = false
						}
						got := content.ValidateCardProgram(c, lib)
						if (got == nil) != want {
							t.Fatalf("%s/%s/%s/%s/%s: allowed=%v error=%v", effect, owner, mode, selection, window, want, got)
						}
						if want {
							valid++
						} else {
							invalid++
						}
					}
				}
			}
		}
		for key, field := range spec.Parameters {
			values := []any{}
			bad := []any{"unsupported"}
			switch field.Type {
			case "enum":
				for _, option := range field.Options {
					values = append(values, option)
				}
			case "boolean":
				values = []any{false, true}
			case "integer":
				values = []any{field.Minimum, field.Maximum}
				bad = []any{field.Minimum - 1, field.Maximum + 1, 1.25}
			case "integers":
				values = []any{[]int{field.Minimum, field.Maximum}}
				bad = []any{[]int{}, []int{field.Minimum - 1}, []int{field.Maximum + 1}}
			case "status":
				values = []any{"protect"}
			case "status_optional":
				values = []any{"", "protect"}
			}
			for i, value := range append(values, bad...) {
				s := makeStep(effect)
				s.Params[key] = value
				if effect == "adjust_die" {
					if key == "minimum" {
						s.Params["maximum"] = 100
					}
					if key == "maximum" {
						s.Params["minimum"] = 1
					}
				}
				if effect == "ability_bonus" && key == "damage" {
					s.Params["status_id"] = "protect"
				}
				c := base
				p := *base.Program
				c.Program = &p
				p.Windows = []string{spec.Windows[0]}
				p.Steps = []content.CardStep{s}
				if got := content.ValidateCardProgram(c, lib); (got == nil) != (i < len(values)) {
					t.Fatalf("%s.%s=%v: allowed=%v err=%v", effect, key, value, i < len(values), got)
				}
			}
		}
	}
	if valid < 300 || invalid < 300 {
		t.Fatalf("incomplete matrix %d valid %d invalid", valid, invalid)
	}
	t.Logf("checked %d supported and %d incompatible target/timing combinations", valid, invalid)
	for a := range content.CardCapabilities() {
		for b := range content.CardCapabilities() {
			steps := []content.CardStep{makeStep(a), makeStep(b)}
			windows := content.CardCompatibleWindows(steps)
			c := base
			p := *base.Program
			c.Program = &p
			p.Steps = steps
			p.Windows = windows
			want := len(windows) > 0 && b != "sacrifice"
			if got := content.ValidateCardProgram(c, lib); (got == nil) != want {
				t.Fatalf("ordered %s -> %s: expected=%v err=%v", a, b, want, got)
			}
		}
	}
}

func TestCardCreationConflictsAndConditions(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, _ := content.EditableGeneralCard(lib.Cards["try_again"])
	c.Program.RollRequirement = "before_first"
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("reroll before first roll was accepted")
	}
	c.Program.RollRequirement = "after_first"
	for _, kind := range []string{"symbol_count", "number_pattern", "exact_faces"} {
		r := content.BattleRequirement{Type: kind}
		switch kind {
		case "symbol_count":
			for id := range lib.Symbols {
				r.SymbolID = id
				break
			}
			r.Minimum = 2
		case "number_pattern":
			r.Pattern = "small_straight"
		case "exact_faces":
			r.Faces = []int{1, 2, 3, 4, 5}
		}
		c.Program.Steps[0].Condition = &content.RequirementGroup{All: []content.BattleRequirement{r}}
		if err := content.ValidateCardProgram(c, lib); err != nil {
			t.Fatal(err)
		}
		c.Program.Steps[0].Condition.All[0].Minimum = -1
		if content.ValidateCardProgram(c, lib) == nil {
			t.Fatal("conflicting condition accepted")
		}
	}
}

func TestCardCreationNestedAndDrawDependencies(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, _ := content.EditableGeneralCard(lib.Cards["battle_focus"])
	c.Program.Steps = []content.CardStep{c.Program.Steps[0], {Effect: "move_cards", Target: content.CardTarget{Owner: "self", Mode: "one", Selection: "choose", Zones: []string{"hand"}, DrawnThisPlay: true}, Params: map[string]any{"destination": "discard"}}}
	// A card can select what its draw produced; reversing that dependency is invalid.
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal(err)
	}
	steps := append([]content.CardStep{}, c.Program.Steps...)
	c.Program.Steps = []content.CardStep{steps[1], steps[0]}
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("drawn-only target allowed before its draw")
	}
	c.Program.Steps = []content.CardStep{steps[0], {Effect: "choice", Choices: []content.CardOption{{Name: "Keep", Steps: []content.CardStep{steps[1]}}}}}
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal("nested option lost parent draw dependency:", err)
	}
	before := c.Program.Steps[1]
	c.Program.Steps = []content.CardStep{before}
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("nested target allowed without its parent draw")
	}
	guard, _ := content.EditableGeneralCard(lib.Cards["brace"])
	c.Program.Steps = []content.CardStep{{Effect: "choice", Choices: []content.CardOption{{Name: "Guard", Steps: guard.Program.Steps}}}}
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("nested defense effect accepted in offensive timing")
	}
	c.Program.Windows = guard.Program.Windows
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatal(err)
	}
	nested := guard.Program.Steps
	for i := 0; i < 5; i++ {
		nested = []content.CardStep{{Effect: "choice", Choices: []content.CardOption{{Name: "Nested", Steps: nested}}}}
	}
	c.Program.Steps = nested
	if content.ValidateCardProgram(c, lib) == nil {
		t.Fatal("excessive nesting accepted")
	}
}
