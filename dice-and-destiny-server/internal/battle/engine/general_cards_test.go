package engine

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

var generalCardIDs = []string{"matchmaker", "turn_the_die", "disrupt", "second_guard", "reclaim", "reinforce", "dispel", "triage"}

func giveGeneralCard(b *state.Battle, id string) {
	a := b.Actors["player"]
	a.Cards.Hand = append(a.Cards.Hand, id)
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances[id] = state.CardInstance{InstanceID: id, DefinitionID: id}
	b.Settled.Actors["player"] = rt
}
func playGeneralChoice(t *testing.T, e Engine, b *state.Battle, lib content.BattleLibrary, id string, c generalCardChoice) {
	t.Helper()
	if err := e.playSettledCard(b, lib, "player", id, c.targets(), "", 0, c.key()); err != nil {
		t.Fatal(err)
	}
}
func TestGeneralDiceCards(t *testing.T) {
	for _, id := range []string{"matchmaker", "turn_the_die"} {
		t.Run(id, func(t *testing.T) {
			b, lib := generalFixture(t)
			giveGeneralCard(&b, id)
			e := NewEngine()
			if len(generalCardChoices(&b, lib, "player", lib.Cards[id])) != 0 {
				t.Fatal("playable before rolling")
			}
			adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
			choices := generalCardChoices(&b, lib, "player", lib.Cards[id])
			if len(choices) == 0 {
				t.Fatal("no choices")
			}
			var selected generalCardChoice
			for _, c := range choices {
				if c.Die == 0 && c.Face == 6 {
					selected = c
					break
				}
			}
			before := b.Actors["player"].CurrentHealth()
			turns := b.Settled.Actors["player"].RollsUsed
			bad := selected
			bad.Face = 99
			raw, _ := json.Marshal(b)
			if e.playSettledCard(&b, lib, "player", id, bad.targets(), "", 0, bad.key()) == nil {
				t.Fatal("forged face accepted")
			}
			after, _ := json.Marshal(b)
			if string(raw) != string(after) {
				t.Fatal("rejection mutated battle")
			}
			playGeneralChoice(t, e, &b, lib, id, selected)
			rt := b.Settled.Actors["player"]
			if rt.FinalDice[0].Face != 6 || rt.FinalDice[1].Face != 2 || rt.RollsUsed != turns || b.Actors["player"].CurrentHealth() != before || b.Actors["player"].Resources.EnergyPoints != 9 || !containsString(b.Actors["player"].Cards.Discard, id) {
				t.Fatal("wrong die/cost/health/roll count")
			}
			if err := e.playSettledCard(&b, lib, "player", id, selected.targets(), "", 0, selected.key()); err == nil {
				t.Fatal("played same instance twice")
			}
		})
	}
}
func TestGeneralDisruptRevalidatesOnlyEnemy(t *testing.T) {
	b, lib := generalFixture(t)
	giveGeneralCard(&b, "disrupt")
	adventurerRoll(&b, lib, []int{1, 1, 1, 1, 1})
	rt := b.Settled.Actors["enemy"]
	rt.FinalDice = rolledCombatFaces(lib, b.Actors["enemy"].DiceLoadout, []int{1, 1, 1, 4, 5})
	rt.OffensiveAbilityIDs = []string{"adventurer_strike"}
	rt.SelectedAbilityID = "adventurer_strike"
	rt.SelectedTierID = "three_swords"
	rt.SelectedTargetIDs = []string{"player"}
	b.Settled.Actors["enemy"] = rt
	openSettledWindow(&b, "react", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	choices := generalCardChoices(&b, lib, "player", lib.Cards["disrupt"])
	if len(choices) != 5 {
		t.Fatalf("expected five enemy choices: %+v", choices)
	}
	e := NewEngine()
	e.namedRandom = &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "combat_dice", Bound: 6, Value: 5}}}
	c := choices[0]
	payload, _ := json.Marshal(command.CommitInteractionPayload{Commitment: command.InteractionCommitmentData{CardIDs: []string{"disrupt"}, ProposalIDs: c.targets(), ChoiceID: c.key()}})
	events, err := e.handleOffensiveReactionCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypeCommitInteraction, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 || b.Settled.Actors["enemy"].FinalDice[0].Face != 6 || b.Settled.Actors["player"].FinalDice[0].Face != 1 || b.Settled.Actors["enemy"].SelectedAbilityID != "" {
		t.Fatal("disruption did not revalidate invalid enemy attack")
	}
}
func TestGeneralRecoveryPreservesHealthAndExcludesRecoveryCards(t *testing.T) {
	b, lib := generalFixture(t)
	giveGeneralCard(&b, "reclaim")
	e := NewEngine()
	a := b.Actors["player"]
	moveCard(&a.Cards, "nudge-0", operation.ZoneHand, operation.ZoneDiscard)
	a.Cards.Discard = append(a.Cards.Discard, "other-reclaim")
	a.Cards.Removed = []string{"gone"}
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances["other-reclaim"] = state.CardInstance{InstanceID: "other-reclaim", DefinitionID: "reclaim"}
	b.Settled.Actors["player"] = rt
	choices := generalCardChoices(&b, lib, "player", lib.Cards["reclaim"])
	if len(choices) != 1 || choices[0].Card != "nudge-0" {
		t.Fatalf("invalid recovery choices: %+v", choices)
	}
	// A reservation follows a card; recovering it must not release it.
	b.Settled.PendingDamage = &state.SettledDamageBatch{Removals: []state.ProposedCardRemoval{{CardID: "nudge-0", Accepted: true, TargetActorID: "player"}}}
	health := b.Actors["player"].CurrentHealth()
	playGeneralChoice(t, e, &b, lib, "reclaim", choices[0])
	a = b.Actors["player"]
	if a.CurrentHealth() != health || !containsString(a.Cards.Hand, "nudge-0") || !containsString(a.Cards.Discard, "reclaim") || !reflect.DeepEqual(a.Cards.Removed, []string{"gone"}) || !b.Settled.PendingDamage.Removals[0].Accepted || a.Resources.EnergyPoints != 8 {
		t.Fatal("recovery healed, duplicated, or canceled pending damage")
	}
}
func TestGeneralDispelOnePositiveStackAndImmunity(t *testing.T) {
	b, lib := generalFixture(t)
	giveGeneralCard(&b, "dispel")
	e := NewEngine()
	applyStatus(&b, lib, "enemy", "protect", 2)
	applyStatus(&b, lib, "enemy", "poison", 2)
	applyStatus(&b, lib, "player", "protect", 2)
	choices := generalCardChoices(&b, lib, "player", lib.Cards["dispel"])
	if len(choices) != 1 || choices[0].Actor != "enemy" || choices[0].Status != "protect" {
		t.Fatalf("wrong status targets: %+v", choices)
	}
	def := lib.Statuses["protect"]
	def.DispelImmune = true
	lib.Statuses["protect"] = def
	if len(generalCardChoices(&b, lib, "player", lib.Cards["dispel"])) != 0 {
		t.Fatal("immune status offered")
	}
	def.DispelImmune = false
	lib.Statuses["protect"] = def
	playGeneralChoice(t, e, &b, lib, "dispel", choices[0])
	if stacks(&b, "enemy", "protect") != 1 || stacks(&b, "enemy", "poison") != 2 || stacks(&b, "player", "protect") != 2 {
		t.Fatal("dispel did not remove exactly one enemy positive stack")
	}
}
func TestGeneralReinforceCostsSeparateSourcesAndConfiguredDestination(t *testing.T) {
	for _, destination := range []string{"original", "discard"} {
		for _, boost := range []bool{false, true} {
			t.Run(destination+map[bool]string{false: "base", true: "boost"}[boost], func(t *testing.T) {
				b, lib, e := generalUnifiedFixture(t, 4, 6)
				giveGeneralCard(&b, "reinforce")
				def := lib.Cards["reinforce"]
				def.SavedCardDestination = destination
				lib.Cards["reinforce"] = def
				unchanged := activeReservations(b, "a")
				var choice generalCardChoice
				for _, c := range generalCardChoices(&b, lib, "player", def) {
					if c.Source == "b" && c.Boost == boost {
						choice = c
					}
				}
				playGeneralChoice(t, e, &b, lib, "reinforce", choice)
				want := 4
				energy := 9
				if boost {
					want = 2
					energy = 8
				}
				if len(activeReservations(b, "b")) != want || !reflect.DeepEqual(unchanged, activeReservations(b, "a")) || b.Actors["player"].Resources.EnergyPoints != energy {
					t.Fatal("wrong source, prevention, or energy")
				}
				for _, r := range b.Settled.PendingDamage.Removals {
					if r.Released && destination == "discard" && r.ReleasedDestination != operation.ZoneDiscard {
						t.Fatal("ignored configured destination")
					}
				}
			})
		}
	}
	b, lib, _ := generalUnifiedFixture(t, 4)
	giveGeneralCard(&b, "reinforce")
	a := b.Actors["player"]
	a.Resources.EnergyPoints = 1
	b.Actors["player"] = a
	choices := generalCardChoices(&b, lib, "player", lib.Cards["reinforce"])
	if len(choices) != 1 || choices[0].Boost {
		t.Fatal("unaffordable boost offered")
	}
}
func TestGeneralTriageExactCardOverageSaveReloadAndPlayedCard(t *testing.T) {
	for _, self := range []bool{false, true} {
		t.Run(map[bool]string{false: "selected", true: "itself"}[self], func(t *testing.T) {
			b, lib, e := generalUnifiedFixture(t, 20)
			giveGeneralCard(&b, "triage")
			if err := e.reconcileUnifiedDamage(&b, true); err != nil {
				t.Fatal(err)
			}
			choices := generalCardChoices(&b, lib, "player", lib.Cards["triage"])
			var selected generalCardChoice
			for _, c := range choices {
				if (c.Card == "triage") == self {
					selected = c
					break
				}
			}
			if selected.Card == "" {
				t.Fatal("missing threatened choice")
			}
			playGeneralChoice(t, e, &b, lib, "triage", selected)
			if containsString(activeReservations(b, "a"), selected.Card) {
				t.Fatal("chosen card was reselected by excess damage")
			}
			raw, _ := json.Marshal(b)
			if err := json.Unmarshal(raw, &b); err != nil {
				t.Fatal(err)
			}
			if err := e.reconcileUnifiedDamage(&b, true); err != nil {
				t.Fatal(err)
			}
			if containsString(activeReservations(b, "a"), selected.Card) {
				t.Fatal("save/reload lost targeted protection")
			}
			if self && !containsString(b.Actors["player"].Cards.Discard, "triage") {
				t.Fatal("saved played card returned to hand")
			}
			if _, err := e.finishDamageBatch(&b, lib); err != nil {
				t.Fatal(err)
			}
			if containsString(b.Actors["player"].Cards.Removed, selected.Card) || b.Actors["player"].CurrentHealth() != 1 {
				t.Fatal("chosen card was lost at final commit")
			}
		})
	}
}
func TestGeneralSecondGuardRerollsSubsetWithoutDuplicateDefenseRewards(t *testing.T) {
	b, lib, e := generalUnifiedFixture(t, 8)
	giveGeneralCard(&b, "second_guard")
	selection := state.SettledDefense{ActorID: "player", AbilityID: "adventurer_guard_plus", SourceID: "a"}
	e.namedRandom = &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "defense_dice", Bound: 6, Value: 0}, {Stream: "defense_dice", Bound: 6, Value: 3}, {Stream: "defense_dice", Bound: 6, Value: 5}}}
	if _, err := e.rollSelectedDefense(&b, lib, &selection); err != nil {
		t.Fatal(err)
	}
	b.Settled.DefenseSelections["player"] = selection
	openSettledWindowForActors(&b, "defense-react", stageDefenseReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePlanningPass}, []string{"player"}, false)
	choices := generalCardChoices(&b, lib, "player", lib.Cards["second_guard"])
	if len(choices) != 7 {
		t.Fatalf("expected seven nonempty subsets: %d", len(choices))
	}
	var selected generalCardChoice
	for _, c := range choices {
		if reflect.DeepEqual(c.Indices, []int{0}) {
			selected = c
		}
	}
	original := b.Clone()
	e.namedRandom = &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "defense_dice", Bound: 6, Value: 4}}}
	playGeneralChoice(t, e, &b, lib, "second_guard", selected)
	got := b.Settled.DefenseSelections["player"]
	if !reflect.DeepEqual(got.RolledFaces, []int{5, 4, 6}) || original.Settled.DefenseSelections["player"].RolledDice[0].Face != 1 || b.Actors["player"].Resources.EnergyPoints != 9 || len(activeReservations(b, "a")) != 8 {
		t.Fatal("reroll changed unselected dice, original clone, or resolved rewards early")
	}
	e.namedRandom = nil
	// Keep this test in the human hub after applying the result.
	if _, err := e.finalizeDefenses(&b, lib); err != nil {
		t.Fatal(err)
	}
	if len(activeReservations(b, "a")) != 4 || b.Actors["player"].Resources.EnergyPoints != 10 || !b.Settled.DefenseHistory["a"].Finalized {
		t.Fatal("defense rewards did not apply exactly once to the final roll")
	}
	if len(generalCardChoices(&b, lib, "player", lib.Cards["second_guard"])) != 0 {
		t.Fatal("reroll offered after defense finalized")
	}
}
func TestGeneralCardsOfferOnlyLegalWindowsAndCosts(t *testing.T) {
	for _, id := range generalCardIDs {
		t.Run(id, func(t *testing.T) {
			b, lib := generalFixture(t)
			giveGeneralCard(&b, id)
			a := b.Actors["player"]
			a.Resources.EnergyPoints = 0
			b.Actors["player"] = a
			if len(generalCardChoices(&b, lib, "player", lib.Cards[id])) != 0 {
				t.Fatal("unaffordable play offered")
			}
			a.Resources.EnergyPoints = 10
			b.Actors["player"] = a
			b.Segment.Current = segment.Income
			if len(generalCardChoices(&b, lib, "player", lib.Cards[id])) != 0 {
				t.Fatal("income play offered")
			}
		})
	}
}

// loadGeneralLibrary loads the pre-program general_choice definitions from
// testdata. Shipped General cards are program cards now, but pinned battles may
// still carry these, so their dedicated engine path stays covered here.
func loadGeneralLibrary(t *testing.T, b *state.Battle, lib content.BattleLibrary) content.BattleLibrary {
	t.Helper()
	lib, err := content.LoadBattleExtension(lib, filepath.Join("testdata", "general_v1_legacy"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	b.SettledCatalog = raw
	return lib
}
func generalFixture(t *testing.T) (state.Battle, content.BattleLibrary) {
	b, lib := adventurerFixture(t)
	lib = loadGeneralLibrary(t, &b, lib)
	return b, lib
}
func generalUnifiedFixture(t *testing.T, amounts ...int) (state.Battle, content.BattleLibrary, Engine) {
	b, lib, e := unifiedFixture(t, amounts...)
	lib = loadGeneralLibrary(t, &b, lib)
	return b, lib, e
}

func TestGeneralCardsThroughAuthoritativeCommands(t *testing.T) {
	for _, id := range generalCardIDs {
		t.Run(id, func(t *testing.T) {
			b, lib, e := generalUnifiedFixture(t, 5)
			giveGeneralCard(&b, id)
			switch id {
			case "matchmaker", "turn_the_die", "reclaim", "dispel":
				b.Segment.Current = segment.Offensive
				openSettledWindowForActors(&b, "plan", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningPass}, []string{"player"}, false)
				adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
				a := b.Actors["player"]
				moveCard(&a.Cards, a.Cards.Hand[0], operation.ZoneHand, operation.ZoneDiscard)
				b.Actors["player"] = a
				applyStatus(&b, lib, "enemy", "protect", 2)
			case "disrupt":
				b.Segment.Current = segment.Offensive
				rt := b.Settled.Actors["enemy"]
				rt.FinalDice = rolledCombatFaces(lib, b.Actors["enemy"].DiceLoadout, []int{1, 1, 1, 4, 5})
				b.Settled.Actors["enemy"] = rt
				openSettledWindowForActors(&b, "react", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass}, []string{"player", "enemy"}, false)
				moveSettledWindowToActor(&b, "player")
			case "second_guard":
				selection := state.SettledDefense{ActorID: "player", AbilityID: "adventurer_guard_plus", SourceID: "a"}
				if _, err := e.rollSelectedDefense(&b, lib, &selection); err != nil {
					t.Fatal(err)
				}
				b.Settled.DefenseSelections["player"] = selection
				openSettledWindowForActors(&b, "defense-react", stageDefenseReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePlanningPass}, []string{"player"}, false)
			}
			actions := e.OpenResult(&b, "player").LegalActions
			var selected command.Command
			for _, action := range actions {
				var payload struct {
					CardIDs    []string `json:"card_ids"`
					Commitment struct {
						CardIDs []string `json:"card_ids"`
					} `json:"commitment"`
				}
				if err := json.Unmarshal(action.Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if containsString(payload.CardIDs, id) || containsString(payload.Commitment.CardIDs, id) {
					selected = action
					break
				}
			}
			if selected.Type == "" {
				t.Fatalf("no authority action for %s", id)
			}
			before := b.Actors["player"].Resources.EnergyPoints
			result := e.HandleBattleCommand(&b, selected)
			if !result.Accepted {
				t.Fatal(result.Error)
			}
			if containsString(b.Actors["player"].Cards.Hand, id) || b.Actors["player"].Resources.EnergyPoints > before-lib.Cards[id].Cost.Energy {
				t.Fatal("card not spent through real command")
			}
		})
	}
}
