package engine

import (
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func roleDiceLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	lib, err := content.LoadBattleExtension(settledTestLibrary(t), filepath.Join("..", "..", "..", "content", "adventurer_v1"))
	if err != nil {
		t.Fatal(err)
	}
	if lib, err = content.LoadBattleExtension(lib, filepath.Join("..", "..", "..", "content", "custom_adventurer_v1")); err != nil {
		t.Fatal(err)
	}
	return lib
}

// Holdfast rolls its owner's die and resolves each face by its symbol's role.
// On the 2-2-1-1 Forge die: Anvil (main) prevents 1, Tongs (side) 2, Rivet
// (odd) 1, and the Ingot (apex) prevents nothing but grants 5 energy.
func TestHoldfastResolvesByRoleOnTwoTwoOneOneDie(t *testing.T) {
	lib := roleDiceLibrary(t)
	b := settledStatusBattle(t, lib, "", 0)
	player := b.Actors["player"]
	player.DiceLoadout = []state.DiceLoadoutEntry{{DiceID: "forge_d6", Count: 5}}
	b.Actors["player"] = player
	holdfast := lib.Abilities["holdfast"]
	want := map[int][2]int{1: {1, 0}, 2: {1, 0}, 3: {2, 0}, 4: {2, 0}, 5: {1, 0}, 6: {0, 5}}
	e := NewEngine()
	for face, expected := range want {
		for _, dieID := range []string{"forge_d6", ""} { // the rolled die, or the owner's die
			ctx := effectContext{SourceActorID: "player", SourceContentID: holdfast.ID, SourceContentType: "ability", ProposalIDs: []string{"incoming"}, TargetActorIDs: []string{"player"}, RolledFace: face, RolledDieID: dieID}
			result, err := e.executeResolvedEffects(&b, lib, ctx, holdfast.Resolution.Operations)
			if err != nil {
				t.Fatalf("face %d: %v", face, err)
			}
			prevented := 0
			for _, p := range result.Preventions {
				prevented += p.Amount
			}
			if prevented != expected[0] || result.ResourceDeltas["player"] != expected[1] {
				t.Fatalf("face %d (%s) prevents %d and gains %d energy; want %v", face, lib.Dice["forge_d6"].Faces[face-1].Symbol, prevented, result.ResourceDeltas["player"], expected)
			}
		}
		if got := configuredPrevention(lib, holdfast.Resolution.Operations, "forge_d6", face); got != expected[0] {
			t.Fatalf("preview of face %d prevents %d; want %d", face, got, expected[0])
		}
	}
}

// Holdfast prevents what Guard does on the standard die, on every board die,
// and grants energy just as often (one apex face).
func TestHoldfastMatchesGuardOnEveryDie(t *testing.T) {
	lib := roleDiceLibrary(t)
	expected := func(ability, die string) (float64, int) {
		total, apex := 0, 0
		for face := 1; face <= 6; face++ {
			n := configuredPrevention(lib, lib.Abilities[ability].Resolution.Operations, die, face)
			total += n
			if faceRole(lib, die, face) == content.RoleApex {
				apex++
			}
		}
		return float64(total) / 6 * 3, apex
	}
	guard, _ := expected("adventurer_guard", "standard_d6")
	if guard != 3.5 {
		t.Fatalf("Guard prevents %.2f per roll on the standard die; want 3.5", guard)
	}
	for _, ability := range []string{"holdfast", "holdfast_plus"} {
		for _, die := range []string{"standard_d6", "hearth_d6", "grove_d6", "forge_d6", "loom_d6"} {
			got, apex := expected(ability, die)
			if got != guard || apex != 1 {
				t.Fatalf("%s on %s prevents %.2f per roll with %d apex faces; want %.2f and 1", ability, die, got, apex, guard)
			}
		}
	}
}

func TestRoleOutcomesValidate(t *testing.T) {
	lib := roleDiceLibrary(t)
	base := lib.Abilities["holdfast"]
	mutate := func(edit func(*content.BattleOperation)) error {
		a := base
		a.ID, a.Name = "holdfast_test", "Holdfast Test"
		res := *a.Resolution
		res.Operations = append([]content.BattleOperation(nil), res.Operations...)
		op := res.Operations[0]
		op.Outcomes = append([]content.BattleOutcome(nil), op.Outcomes...)
		edit(&op)
		res.Operations[0] = op
		a.Resolution = &res
		return content.ValidateAuthoredAbility(a, lib)
	}
	if err := mutate(func(*content.BattleOperation) {}); err != nil {
		t.Fatalf("Holdfast copy rejected: %v", err)
	}
	for name, edit := range map[string]func(*content.BattleOperation){
		"missing role":     func(op *content.BattleOperation) { op.Outcomes = op.Outcomes[:len(op.Outcomes)-1] },
		"unknown role":     func(op *content.BattleOperation) { op.Outcomes[0].Roles = []string{"main", "wild"} },
		"faces with owner": func(op *content.BattleOperation) { op.Outcomes[0].Faces = []int{1} },
		"overlap":          func(op *content.BattleOperation) { op.Outcomes[1].Roles = []string{"side", "main"} },
		"both dice":        func(op *content.BattleOperation) { op.DiceID = "standard_d6" },
		"roles on a named die": func(op *content.BattleOperation) {
			op.Dice, op.DiceID = "", "standard_d6"
		},
	} {
		if err := mutate(edit); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// heal_cards (a board rider) returns removed cards to discard, most recently
// removed first, and heals each from the wound that removed it.
func TestHealCardsReturnsMostRecentlyRemovedCards(t *testing.T) {
	lib := roleDiceLibrary(t)
	b := settledStatusBattle(t, lib, "", 0)
	player := b.Actors["player"]
	player.Cards.Removed = []string{"old", "mid", "new"}
	player.Cards.Discard = nil
	b.Actors["player"] = player
	b.Wounds = []state.Wound{
		{ID: "w1", TargetActorID: "player", Cards: []state.WoundCard{{CardID: "old"}, {CardID: "mid"}}},
		{ID: "w2", TargetActorID: "player", Cards: []state.WoundCard{{CardID: "new"}}},
	}
	e := NewEngine()
	ctx := effectContext{SourceActorID: "player", SourceContentID: "board_ss", SourceContentType: "ability"}
	result, err := e.executeEffects(&b, lib, ctx, []content.BattleOperation{{Type: "heal_cards", Target: "self", Amount: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.applyEffectMutations(&b, lib, "", result); err != nil {
		t.Fatal(err)
	}
	got := b.Actors["player"].Cards
	if len(got.Removed) != 1 || got.Removed[0] != "old" || len(got.Discard) != 2 || got.Discard[0] != "new" || got.Discard[1] != "mid" {
		t.Fatalf("piles after heal 2: removed %v, discard %v", got.Removed, got.Discard)
	}
	if len(b.Wounds) != 1 || b.Wounds[0].ID != "w1" || len(b.Wounds[0].Cards) != 1 || b.Wounds[0].Cards[0].CardID != "old" {
		t.Fatalf("wounds after heal 2: %+v", b.Wounds)
	}
	// With nothing left to heal past the last card, the heal stops.
	result, _ = e.executeEffects(&b, lib, ctx, []content.BattleOperation{{Type: "heal_cards", Target: "self", Amount: 3}})
	if err := e.applyEffectMutations(&b, lib, "", result); err != nil {
		t.Fatal(err)
	}
	if got := b.Actors["player"].Cards; len(got.Removed) != 0 || len(got.Discard) != 3 || len(b.Wounds) != 0 {
		t.Fatalf("over-heal: removed %v, discard %v, wounds %v", got.Removed, got.Discard, b.Wounds)
	}
}
