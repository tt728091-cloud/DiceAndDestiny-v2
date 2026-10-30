package tacticsbattle

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
)

func testRoots(t *testing.T) Roots {
	t.Helper()
	return Roots{ContentRoot: "../../../content", LoadoutRoot: "../../../content/authored", BattleRoot: t.TempDir()}
}

func legal(t *testing.T, view map[string]any) []command.Command {
	t.Helper()
	raw, _ := json.Marshal(view["legal_actions"])
	var actions []command.Command
	if err := json.Unmarshal(raw, &actions); err != nil {
		t.Fatal(err)
	}
	return actions
}

func TestCatalogOffersHeroesEnemiesArenasAndMapCards(t *testing.T) {
	catalog, err := Catalog(testRoots(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog["heroes"].([]map[string]any)) != 2 || len(catalog["enemies"].([]map[string]any)) != 4 || len(catalog["arenas"].([]map[string]any)) != 4 {
		t.Fatalf("catalog = %v", catalog)
	}
	var names []string
	for _, card := range catalog["map_cards"].([]map[string]any) {
		names = append(names, card["name"].(string))
	}
	if len(names) == 0 {
		t.Fatal("the published map Move cards are missing")
	}
	t.Logf("map Move cards: %v", names)
}

// playOut submits the hero's first preferred legal action until the battle ends.
func playOut(t *testing.T, roots Roots, id string, view map[string]any, prefer func([]command.Command) command.Command) (map[string]any, int, time.Duration) {
	t.Helper()
	var slowest time.Duration
	for n := 0; n < 600; n++ {
		if view["battle_result"] != nil {
			return view, n, slowest
		}
		actions := legal(t, view)
		if len(actions) == 0 {
			t.Fatalf("the hero has nothing to do at %v/%v", view["snapshot"].(map[string]any)["stage"], view["snapshot"].(map[string]any)["simultaneous_step"])
		}
		raw, _ := json.Marshal(prefer(actions))
		started := time.Now()
		next, err := Submit(roots, id, string(raw))
		if err != nil {
			t.Fatal(err)
		}
		if took := time.Since(started); took > slowest {
			slowest = took
		}
		view = next
	}
	t.Fatal("the battle did not end")
	return nil, 0, 0
}

// simpleHero rolls, attacks when it can, pursues the nearest enemy, takes
// the first defense and otherwise passes.
func simpleHero(view map[string]any) func([]command.Command) command.Command {
	return func(actions []command.Command) command.Command {
		var plan *command.MovementPlanPayload
		for _, kind := range []command.Type{command.TypePlanningRoll, command.TypePlanningAbility} {
			for _, a := range actions {
				if a.Type == kind {
					return a
				}
			}
		}
		for _, a := range actions {
			if a.Type != command.TypeMovementPlan {
				continue
			}
			var p command.MovementPlanPayload
			_ = json.Unmarshal(a.Payload, &p)
			if p.LockIn {
				plan = &p
				if p.Intention.Kind == "pursue" && p.Intention.Target != HeroSeat {
					return a
				}
			}
		}
		if plan != nil {
			for _, a := range actions {
				var p command.MovementPlanPayload
				if a.Type == command.TypeMovementPlan && json.Unmarshal(a.Payload, &p) == nil && !p.LockIn && p.Intention.Kind == "pursue" && p.Intention.Target != HeroSeat && len(p.CardIDs) == 0 && p.Energy == 0 {
					return a
				}
			}
		}
		for _, kind := range []command.Type{command.TypeMovementPlan, command.TypePlanningPass, command.TypeCommitInteraction} {
			for _, a := range actions {
				if a.Type == kind {
					return a
				}
			}
		}
		return actions[0]
	}
}

func TestRunnerPlaysASandboxBattleToTheEnd(t *testing.T) {
	roots := testRoots(t)
	setup := Setup{Hero: "starter_movement", Enemies: []string{"tactics_bell_diver"}, Arena: "open_field", Seed: 7}
	view, err := Start(roots, "tactics-runner-test", setup)
	if err != nil {
		t.Fatal(err)
	}
	if view["snapshot"].(map[string]any)["tactics"] == nil {
		t.Fatal("the hero's view has no tactics section")
	}
	// The hero always attacks when it can and otherwise locks in plans.
	end, n, slowest := playOut(t, roots, "tactics-runner-test", view, simpleHero(view))
	t.Logf("result %v after %d hero commands; slowest %s", end["battle_result"], n, slowest)
	// A stored battle reopens as it was.
	again, err := Open(roots, "tactics-runner-test")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(again["battle_result"]) != fmt.Sprint(end["battle_result"]) {
		t.Fatal("the reopened battle differs")
	}
}

func TestSubmitAcceptsGodotFloats(t *testing.T) {
	got, err := integralNumbers(`{"a": 1.0, "b": [2.0, 2.5], "c": {"d": 0.0}}`)
	if err != nil || got != `{"a":1,"b":[2,2.5],"c":{"d":0}}` {
		t.Fatalf("integralNumbers = %s %v", got, err)
	}
}
