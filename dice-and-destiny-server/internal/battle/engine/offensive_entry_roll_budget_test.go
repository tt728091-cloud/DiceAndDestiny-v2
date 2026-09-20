package engine

import "testing"

// The client must be able to render the entire roll budget from the entry
// snapshot, after Entangle is consumed and before a dice_rolled event exists.
func TestOffensiveEntrySnapshotIncludesRollBudgetBeforeFirstRoll(t *testing.T) {
	library := settledTestLibrary(t)
	for _, statusID := range []string{"", "entangle"} {
		t.Run("status="+statusID, func(t *testing.T) {
			battle := settledStatusBattle(t, library, statusID, 1)
			eng := NewEngine()
			if err := eng.applyOffensiveEntryTriggers(&battle, library); err != nil {
				t.Fatal(err)
			}
			want := 3
			if statusID == "entangle" {
				want = 2
			}
			result := eng.OpenResult(&battle, "player")
			dice := result.Snapshot.Actors["player"].Dice
			if dice == nil || dice.MaxRolls != want || dice.RollsRemaining != want || dice.RollsUsed != 0 || len(dice.Dice) != 0 {
				t.Fatalf("pre-roll dice snapshot=%+v; want %d/%d with no rolled dice", dice, want, want)
			}
			if len(battle.Actors["player"].Statuses) != 0 {
				t.Fatal("entry effect should already be consumed; client cannot infer budget from remaining statuses")
			}
		})
	}
}
