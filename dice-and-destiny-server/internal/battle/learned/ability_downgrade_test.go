package learned

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/loadout"
)

func TestAbilityDowngradeRefundPersistenceAndBattle(t *testing.T) {
	contentRoot := filepath.Join(testServerRoot(t), "content")
	catalogs, _ := CharacterCatalogs(contentRoot)
	e, _ := loadout.LoadEconomy(contentRoot, catalogs)
	e.StartingXP = 25 // Downgrading must work with no unspent XP.
	root := t.TempDir()
	lib := catalogs["adventurer"]
	p, err := loadout.ReadProgress(root, "adventurer", e, lib)
	if err != nil {
		t.Fatal(err)
	}
	p, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(SessionConfig{ContentRoot: contentRoot, RunStateRoot: t.TempDir(), LoadoutRoot: root, OpponentDefinition: "drowned_oracle_brine_mask"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResetCharacterLoadout("before-downgrade", 941, "seat-a", false, "adventurer", true, "progression"); err != nil {
		t.Fatal(err)
	}
	request := loadout.Purchase{Kind: "downgrade_ability", ID: "adventurer_guard_plus", TargetID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25}
	p, err = loadout.Buy(root, "adventurer", e, lib, request)
	if err != nil {
		t.Fatal(err)
	}
	if p.XP != 25 || p.UpgradeSpent != 0 || *p.Budget != 145 || totalProgress(p.Deck) != 12 || !reflect.DeepEqual(p.Abilities.Defensive, []string{"adventurer_guard"}) {
		t.Fatalf("invalid refund: %+v", p)
	}
	if !reflect.DeepEqual(s.current.Result.Snapshot.Actors["seat-a"].DefensiveAbilities, []string{"adventurer_guard_plus"}) {
		t.Fatal("downgrade changed active battle")
	}
	again, err := loadout.ReadProgress(root, "adventurer", e, lib)
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatal("downgrade not persisted")
	}
	filename := filepath.Join(root, "progression", "adventurer.json")
	before, _ := os.ReadFile(filename)
	if _, err = loadout.Buy(root, "adventurer", e, lib, request); err == nil {
		t.Fatal("duplicate refund accepted")
	}
	request.Revision = p.Revision
	if _, err = loadout.Buy(root, "adventurer", e, lib, request); err == nil {
		t.Fatal("unequipped upgrade refunded")
	}
	after, _ := os.ReadFile(filename)
	if string(before) != string(after) {
		t.Fatal("rejected downgrade changed saved state")
	}
	for _, seat := range []string{"seat-a", "seat-b"} {
		if _, err = s.ResetCharacterLoadout("downgraded-"+seat, 943, seat, false, "adventurer", true, "progression"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.current.Result.Snapshot.Actors[seat].DefensiveAbilities, []string{"adventurer_guard"}) {
			t.Fatal("next battle did not use base ability")
		}
		finishOwnedBattle(t, s)
	}
	p, err = loadout.Buy(root, "adventurer", e, lib, loadout.Purchase{Kind: "upgrade_ability", ID: "adventurer_guard", Revision: p.Revision, ExpectedCost: 25})
	if err != nil || p.XP != 0 || p.UpgradeSpent != 25 {
		t.Fatal("could not repurchase upgrade")
	}
	request.Revision = p.Revision
	request.ExpectedCost = 26
	if _, err = loadout.Buy(root, "adventurer", e, lib, request); err == nil {
		t.Fatal("incorrect refund quote accepted")
	}
	request.ExpectedCost = 25
	request.TargetID = "adventurer_strike"
	if _, err = loadout.Buy(root, "adventurer", e, lib, request); err == nil {
		t.Fatal("unconfigured downgrade accepted")
	}
}
