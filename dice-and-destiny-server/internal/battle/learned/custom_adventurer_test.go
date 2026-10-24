package learned

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// boardDice are the dice the Board Builder offers, one per family order.
var boardDice = []string{"standard_d6", "hearth_d6", "grove_d6", "forge_d6", "loom_d6"}

// straightsBoard qualifies on any die: straights name numbers, not symbols.
var straightsBoard = content.AbilityBoard{Offensive: []string{"board_ss", "board_ls"}, Defensive: []string{"holdfast"}}

// saveCustomSheet writes a die and board to the custom adventurer's sheet
// through the authority's validated save.
func saveCustomSheet(t *testing.T, root string, sheet loadout.SheetBoard) (loadout.Progress, error) {
	t.Helper()
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	economy, err := loadout.LoadEconomy(contentRoot, catalogs)
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs[loadout.CustomAdventurer]
	p, err := loadout.ReadProgress(root, loadout.CustomAdventurer, economy, lib)
	if err != nil {
		t.Fatal(err)
	}
	return loadout.SaveSheetBoard(root, loadout.CustomAdventurer, economy, lib, sheet, p.Revision)
}

func customSession(t *testing.T, root string) *Session {
	t.Helper()
	server := testServerRoot(t)
	s, err := NewSession(SessionConfig{LoadoutRoot: root, ContentRoot: filepath.Join(server, "content"), RunStateRoot: t.TempDir(), ModelPath: filepath.Join(server, "../dice-and-destiny-client/models/learned", "blade-warden-seed-11-final-v1.json"), InferenceTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// playToEnd finishes the session's battle with the test player against the
// learned opponent.
func playToEnd(t *testing.T, s *Session) {
	t.Helper()
	cardPlays := map[int]int{}
	for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal; step++ {
		if s.current.ActorID == s.modelSeat {
			if _, err := s.AdvanceModel(); err != nil {
				t.Fatalf("model step %d: %v", step, err)
			}
			continue
		}
		action := adventurerTestAction(s.current.Result.LegalActions, cardPlays[s.current.Result.Snapshot.Round])
		if action.Type == command.TypePlanningCards {
			cardPlays[s.current.Result.Snapshot.Round]++
		}
		encoded, _ := json.Marshal(aliasValue(commandMap(t, action), map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias}))
		if _, err := s.SubmitHuman(string(encoded)); err != nil {
			t.Fatalf("human step %d: %s: %v", step, encoded, err)
		}
	}
	if !s.current.Terminal || s.current.TruncationReason != "" {
		t.Fatalf("did not complete: %+v", s.current.Metrics)
	}
}

// A full authority battle completes with the custom adventurer on each die,
// rolling that die for attacks and for its role-based defense.
func TestCustomAdventurerBattleOnEveryDie(t *testing.T) {
	defended := 0
	for i, die := range boardDice {
		root := t.TempDir()
		if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: die, Abilities: straightsBoard}); err != nil {
			t.Fatalf("%s: %v", die, err)
		}
		s := customSession(t, root)
		view, err := s.ResetCharacter("custom-"+die, uint64(4100+i), "seat-a", false, loadout.CustomAdventurer)
		if err != nil {
			t.Fatalf("%s: %v", die, err)
		}
		if got := object(object(object(view["snapshot"])["actors"])[HumanAlias])["definition_id"]; got != loadout.CustomAdventurer {
			t.Fatalf("%s: wrong character %v", die, got)
		}
		start, _ := s.environment.InspectBattle()
		hero := start.Actors[s.humanSeat]
		if len(hero.DiceLoadout) != 1 || hero.DiceLoadout[0] != (state.DiceLoadoutEntry{DiceID: die, Count: content.LoadoutDiceCount}) {
			t.Fatalf("%s: battle loadout %v", die, hero.DiceLoadout)
		}
		if _, ok := start.DiceDefinitions[die]; !ok {
			t.Fatalf("%s: die definition not pinned", die)
		}
		playToEnd(t, s)
		final, _ := s.environment.InspectBattle()
		for _, defense := range final.Settled.DefenseHistory {
			if defense.ActorID != s.humanSeat || defense.AbilityID != "holdfast" {
				continue
			}
			defended++
			for _, rolled := range defense.RolledDice {
				if rolled.DieID != die {
					t.Fatalf("%s: defense rolled %s", die, rolled.DieID)
				}
			}
		}
		t.Logf("%s: %d rounds, winner %q, %d Holdfast defenses so far", die, final.Segment.Round, final.WinnerActorID, defended)
	}
	if defended == 0 {
		t.Fatal("no battle rolled a Holdfast defense")
	}
}

// The die is on the sheet: it survives save and reload, and a battle keeps
// the die it started with even if the sheet changes mid-battle.
func TestCustomAdventurerSheetKeepsDie(t *testing.T) {
	root := t.TempDir()
	saved, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "forge_d6", Abilities: straightsBoard})
	if err != nil {
		t.Fatal(err)
	}
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, err := CharacterCatalogs(contentRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	economy, _ := loadout.LoadEconomy(contentRoot, catalogs)
	reloaded, err := loadout.ReadProgress(root, loadout.CustomAdventurer, economy, catalogs[loadout.CustomAdventurer])
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Dice != "forge_d6" || reloaded.Revision != saved.Revision || strings.Join(reloaded.Abilities.Offensive, ",") != "board_ss,board_ls" {
		t.Fatalf("reloaded sheet %+v", reloaded)
	}
	if got := catalogs[loadout.CustomAdventurer].Combatants[loadout.CustomAdventurer].DiceLoadout; len(got) != 1 || got[0].DiceID != "forge_d6" || got[0].Count != 5 {
		t.Fatalf("catalog loadout %v", got)
	}
	// Other characters never take a sheet die.
	if got := catalogs["starter"].Combatants["starter"].DiceLoadout[0].DiceID; got != "standard_d6" {
		t.Fatalf("starter rolls %s", got)
	}

	s := customSession(t, root)
	if _, err := s.ResetCharacter("pinned-die", 77, "seat-a", false, loadout.CustomAdventurer); err != nil {
		t.Fatal(err)
	}
	if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "loom_d6", Abilities: straightsBoard}); err != nil {
		t.Fatal(err)
	}
	running, _ := s.environment.InspectBattle()
	raw, err := json.Marshal(running)
	if err != nil {
		t.Fatal(err)
	}
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string]state.Battle{"running": running, "restored": restored} {
		if got := b.Actors[s.humanSeat].DiceLoadout[0].DiceID; got != "forge_d6" {
			t.Fatalf("%s battle rolls %s; it must keep the die it started with", name, got)
		}
	}
	if _, err := s.ResetCharacter("next-battle", 78, "seat-a", false, loadout.CustomAdventurer); err != nil {
		t.Fatal(err)
	}
	next, _ := s.environment.InspectBattle()
	if got := next.Actors[s.humanSeat].DiceLoadout[0].DiceID; got != "loom_d6" {
		t.Fatalf("next battle rolls %s; want the sheet's new die", got)
	}
}

// Board validation runs AbilityFitsDice against the chosen die: a board
// whose abilities name another die's symbols is refused.
func TestCustomAdventurerBoardMustFitDie(t *testing.T) {
	root := t.TempDir()
	defaultBoard := content.AbilityBoard{Offensive: []string{"board_main3_main4_main5", "board_ss"}, Defensive: []string{"holdfast"}}
	if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "forge_d6", Abilities: defaultBoard}); err == nil || !strings.Contains(err.Error(), "can never qualify on Forge Die") {
		t.Fatalf("standard-die ability accepted on the Forge die: %v", err)
	}
	if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "standard_d6", Abilities: defaultBoard}); err != nil {
		t.Fatalf("standard-die board refused on the standard die: %v", err)
	}
	if _, err := saveCustomSheet(t, root, loadout.SheetBoard{Dice: "no_such_d6", Abilities: straightsBoard}); err == nil {
		t.Fatal("unknown die accepted")
	}
	// The ability workshop never publishes this board to the shared catalog.
	if _, err := content.SaveAuthoredAbility(t.TempDir(), content.BattleLibrary{}, nil, loadout.CustomAdventurer, &straightsBoard, 0); err == nil {
		t.Fatal("custom adventurer board published to the authored catalog")
	}
}
