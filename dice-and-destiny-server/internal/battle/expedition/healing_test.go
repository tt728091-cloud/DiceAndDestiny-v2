package expedition

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// --- invariants --------------------------------------------------------------

// allCards is every copy the player owns: the live piles plus every wounded
// card, in no particular order.
func allCards(s *State) []string {
	out := append(append(append([]string{}, s.Hand...), s.Draw...), s.Spent...)
	return append(out, s.Wounds.Cards()...)
}

// checkWoundInvariants pins what must hold after every action: health is the
// cards (live + wounded = the deck), no copy is duplicated or lost, every
// wound holds at least one card, wound ids are unique, and the view agrees.
func checkWoundInvariants(t *testing.T, s *State, r Rules, deck []string, where string) {
	t.Helper()
	if got, want := sortedStrings(allCards(s)), sortedStrings(deck); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: the player's cards changed: %v, want %v (hand %v draw %v spent %v wounds %+v)", where, got, want, s.Hand, s.Draw, s.Spent, s.Wounds)
	}
	if s.LiveCards()+s.Wounds.Damage() != len(deck) {
		t.Fatalf("%s: health is not conserved: live %d + wounds %d != %d", where, s.LiveCards(), s.Wounds.Damage(), len(deck))
	}
	ids := map[string]bool{}
	for _, w := range s.Wounds {
		if len(w.Cards) == 0 {
			t.Fatalf("%s: a wound with no cards lingers: %+v", where, w)
		}
		if w.Size < len(w.Cards) {
			t.Fatalf("%s: a wound grew past its size: %+v", where, w)
		}
		if w.ID == "" || ids[w.ID] {
			t.Fatalf("%s: duplicate or empty wound id %q in %+v", where, w.ID, s.Wounds)
		}
		ids[w.ID] = true
	}
	v := s.View(r, 1)
	if v.Health != s.LiveCards() || v.MaxHealth != len(deck) || len(v.WoundCards) != s.Wounds.Damage() || len(v.Wounds) != len(s.Wounds) {
		t.Fatalf("%s: the view disagrees: health %d/%d wound cards %d wounds %d", where, v.Health, v.MaxHealth, len(v.WoundCards), len(v.Wounds))
	}
	for i, vw := range v.Wounds {
		if vw.Damage != s.Wounds[i].Damage() || !reflect.DeepEqual(vw.Cards, s.Wounds[i].Cards) || vw.ID != s.Wounds[i].ID {
			t.Fatalf("%s: view wound %d disagrees: %+v vs %+v", where, i, vw, s.Wounds[i])
		}
	}
	if s.Camp != nil {
		if v.Camp == nil {
			t.Fatalf("%s: a camp without a camp view", where)
		}
		healed := 0
		for _, h := range v.Camp.Heals {
			healed += h.Healed
			if h.Healed != len(h.Cards) || h.Healed < 1 || h.Roll < 1 || h.Roll > h.Die {
				t.Fatalf("%s: a heal roll is inconsistent: %+v", where, h)
			}
		}
		if healed != v.Camp.Healed || healed != s.Camp.Healed {
			t.Fatalf("%s: camp healed %d but the rolls sum to %d", where, v.Camp.Healed, healed)
		}
	}
}

// stepTwin applies one action to a state and to its JSON-reloaded twin and
// requires both to agree on the error, the state and the view; on an error
// the state must not have changed.
func stepTwin(t *testing.T, r Rules, s, twin *State, a Action, where string) error {
	t.Helper()
	before, _ := json.Marshal(s)
	_, err1 := s.Act(r, a)
	_, err2 := twin.Act(r, a)
	if (err1 == nil) != (err2 == nil) {
		t.Fatalf("%s: %+v diverged: %v vs %v", where, a, err1, err2)
	}
	after, _ := json.Marshal(s)
	if err1 != nil {
		if string(before) != string(after) {
			t.Fatalf("%s: %+v failed with %v but changed the state", where, a, err1)
		}
		return err1
	}
	j2, _ := json.Marshal(twin)
	if string(after) != string(j2) {
		t.Fatalf("%s: %+v: the reloaded twin diverged\n%s\n%s", where, a, after, j2)
	}
	v1, _ := json.Marshal(s.View(r, 1))
	v2, _ := json.Marshal(twin.View(r, 1))
	if string(v1) != string(v2) {
		t.Fatalf("%s: %+v: the views diverged", where, a)
	}
	return nil
}

// --- property tests -----------------------------------------------------------

func TestHealthIsConservedThroughCampHealing(t *testing.T) {
	r := testRules(t)
	for seed := uint64(1); seed <= 8; seed++ {
		s := newState(t, r, seed)
		rng := rand.New(rand.NewSource(int64(seed)))
		// Several hits of random size, inflicted the authority's way, plus a
		// few bandages. The last live card is never taken.
		for i := 0; i < 2+rng.Intn(3); i++ {
			points := 1 + rng.Intn(3)
			liveBefore := s.LiveCards()
			got := s.inflictWound(fmt.Sprintf("hit %d", i), points)
			if got != min(points, liveBefore-1) || s.LiveCards() != liveBefore-got {
				t.Fatalf("seed %d: inflictWound(%d) dealt %d with %d live cards before", seed, points, got, liveBefore)
			}
		}
		s.Inventory["bandage"] = rng.Intn(4)
		checkWoundInvariants(t, s, r, starterDeck, fmt.Sprintf("seed %d start", seed))
		twin := reloaded(t, s)
		for step := 0; step < 60; step++ {
			where := fmt.Sprintf("seed %d step %d (%s)", seed, step, s.Phase)
			var a Action
			switch s.Phase {
			case PhaseTurn:
				switch rng.Intn(4) {
				case 0, 1:
					if len(s.Hand) == 0 {
						a = Action{Kind: "end_turn"}
					} else {
						a = Action{Kind: "discard", Cards: s.Hand[:1+rng.Intn(len(s.Hand))]}
					}
				case 2:
					a = Action{Kind: "end_turn"}
				default:
					a = Action{Kind: "make_camp"}
				}
			case PhaseCamp:
				switch rng.Intn(7) {
				case 0:
					a = Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"}
				case 1:
					a = Action{Kind: "camp_service", Service: "basic_aid"}
				case 2:
					a = Action{Kind: "camp_service", Service: "bandage"}
				case 3, 4:
					id := "w" + fmt.Sprint(1+rng.Intn(6))
					if len(s.Wounds) > 0 && rng.Intn(2) == 0 {
						id = s.Wounds[rng.Intn(len(s.Wounds))].ID
					}
					svc := []string{"basic_aid", "bandage"}[rng.Intn(2)]
					a = Action{Kind: "camp_service", Service: svc, Wound: id}
				case 5:
					if len(s.Hand) > 0 {
						a = Action{Kind: "camp_service", Service: "influence", Cards: s.Hand[:1]}
					} else {
						a = Action{Kind: "break_camp"}
					}
				default:
					a = Action{Kind: "break_camp"}
				}
			case PhaseBattle, PhaseEvent:
				// The deadline fight or an event: this test stops there.
				step = 60
				continue
			default:
				step = 60
				continue
			}
			bandagesBefore := s.Inventory["bandage"]
			healedBefore := 0
			if s.Camp != nil {
				healedBefore = s.Camp.Healed
			}
			err := stepTwin(t, r, s, twin, a, where)
			if err != nil {
				if s.Inventory["bandage"] != bandagesBefore {
					t.Fatalf("%s: a failed %+v spent a bandage", where, a)
				}
			} else if a.Kind == "camp_service" && a.Service == "bandage" {
				if s.Inventory["bandage"] != bandagesBefore-len(s.Camp.Heals[len(s.Camp.Heals)-1:]) || s.Camp.Healed <= healedBefore {
					t.Fatalf("%s: a bandage must spend exactly one and heal: bandages %d→%d healed %d→%d", where, bandagesBefore, s.Inventory["bandage"], healedBefore, s.Camp.Healed)
				}
			}
			checkWoundInvariants(t, s, r, starterDeck, where)
			twin = reloaded(t, twin)
		}
	}
}

func TestCampHealingRulesAtTheEdges(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 12)
	// inflictWound never takes the last live card and reports what it took;
	// non-positive damage adds no wound.
	if got := s.inflictWound("nothing", 0); got != 0 || len(s.Wounds) != 0 {
		t.Fatalf("zero damage is no wound: %d %+v", got, s.Wounds)
	}
	if got := s.inflictWound("nothing", -3); got != 0 || len(s.Wounds) != 0 {
		t.Fatalf("negative damage is no wound: %d %+v", got, s.Wounds)
	}
	if got := s.inflictWound("overkill", 50); got != 11 || s.LiveCards() != 1 || len(s.Wounds) != 1 || s.Wounds[0].Size != 11 {
		t.Fatalf("overkill stops at the last live card: dealt %d live %d wounds %+v", got, s.LiveCards(), s.Wounds)
	}
	if got := s.inflictWound("again", 2); got != 0 || len(s.Wounds) != 1 {
		t.Fatalf("the last live card is never taken: dealt %d wounds %+v", got, s.Wounds)
	}
	checkWoundInvariants(t, s, r, starterDeck, "overkill")
	// Healing non-positive amounts changes nothing; healing more than the
	// damage heals everything and closes the wounds.
	if healed := s.healPoints(0); len(healed) != 0 || s.Wounds.Damage() != 11 {
		t.Fatalf("healPoints(0) healed %v", healed)
	}
	if healed := s.healPoints(-1); len(healed) != 0 || s.Wounds.Damage() != 11 {
		t.Fatalf("healPoints(-1) healed %v", healed)
	}
	if healed := s.healFromWound(&s.Wounds[0], 0); len(healed) != 0 || s.Wounds.Damage() != 11 {
		t.Fatalf("healFromWound(0) healed %v", healed)
	}
	if healed := s.healPoints(100); len(healed) != 11 || len(s.Wounds) != 0 || s.LiveCards() != 12 {
		t.Fatalf("healing past the damage heals it all: %v wounds %+v", healed, s.Wounds)
	}
	if healed := s.healPoints(3); len(healed) != 0 {
		t.Fatalf("nothing to heal: %v", healed)
	}
	s.dropClosedWounds()
	checkWoundInvariants(t, s, r, starterDeck, "healed")

	// Camp rules: "all" with fewer bandages than treated wounds is refused
	// whole, spending nothing and flagging nothing.
	s = newState(t, r, 13)
	for i := 0; i < 3; i++ {
		s.inflictWound(fmt.Sprintf("hit %d", i), 2)
	}
	ids := []string{s.Wounds[0].ID, s.Wounds[1].ID, s.Wounds[2].ID}
	act(t, s, r, Action{Kind: "make_camp"})
	_, err := s.Act(r, Action{Kind: "camp_service", Service: "bandage", Wound: "all"})
	mustFail(t, err, "no wound can take a bandage now")
	_, err = s.Act(r, Action{Kind: "camp_service", Service: "bandage", Wound: ids[0]})
	mustFail(t, err, "treat it with basic aid first")
	_, err = s.Act(r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "w99"})
	mustFail(t, err, `you have no wound "w99"`)
	_, err = s.Act(r, Action{Kind: "camp_service", Service: "mend_roll", Item: "bandage"})
	mustFail(t, err, "unknown camp service")
	act(t, s, r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	treated := 0
	for _, w := range s.Wounds {
		if w.Treated {
			treated++
		}
	}
	if treated != len(s.Wounds) || len(s.Camp.Heals) != 3 {
		t.Fatalf("basic aid on all treats every wound: %+v heals %d", s.Wounds, len(s.Camp.Heals))
	}
	before := reloaded(t, s)
	s.Inventory["bandage"] = 1
	before.Inventory["bandage"] = 1
	if len(s.Wounds) > 0 {
		// Only gear that heals can be spent as a bandage.
		s.Inventory["lantern"] = 1
		_, err = s.Act(r, Action{Kind: "camp_service", Service: "bandage", Wound: s.Wounds[0].ID, Item: "lantern"})
		mustFail(t, err, "is not something that heals")
		delete(s.Inventory, "lantern")
	}
	if len(s.Wounds) >= 2 {
		_, err = s.Act(r, Action{Kind: "camp_service", Service: "bandage", Wound: "all"})
		mustFail(t, err, "you have 1 Bandage, not")
		if s.Inventory["bandage"] != 1 || len(s.Camp.Heals) != 3 || !reflect.DeepEqual(s.Wounds, before.Wounds) {
			t.Fatalf("a refused bandage spends and flags nothing: bandages %d heals %d wounds %+v", s.Inventory["bandage"], len(s.Camp.Heals), s.Wounds)
		}
	}
	// The view only offers what the authority accepts.
	v := s.View(r, 1)
	if v.Camp.Untreated != 0 || v.Camp.CanBasicAid || v.Camp.Bandageable != len(s.Wounds) || !v.Camp.CanBandage || v.Camp.Bandages != 1 {
		t.Fatalf("camp view after treating all: %+v", v.Camp)
	}
	for _, vw := range v.Wounds {
		if vw.BasicAid || !vw.Bandage || vw.Reason != "" {
			t.Fatalf("treated wounds take a bandage: %+v", vw)
		}
	}
	// One bandage on the first treated wound, then none are left.
	act(t, s, r, Action{Kind: "camp_service", Service: "bandage"})
	if s.Inventory["bandage"] != 0 {
		t.Fatal("the bandage was spent")
	}
	v = s.View(r, 1)
	if v.Camp.CanBandage || v.Camp.Bandages != 0 {
		t.Fatalf("no bandages left: %+v", v.Camp)
	}
	for _, vw := range v.Wounds {
		if vw.Bandage || (vw.Bandaged && vw.Reason != "treated and bandaged at this camp") || (!vw.Bandaged && vw.Reason != "treated at this camp; a bandage would heal more") {
			t.Fatalf("wound reasons: %+v", vw)
		}
	}
	if len(s.Wounds) > 0 && v.Camp.HealReason != "every wound has been treated at this camp; a bandage would heal more" {
		t.Fatalf("heal reason: %q", v.Camp.HealReason)
	}
	if _, err = s.Act(r, Action{Kind: "camp_service", Service: "bandage"}); err == nil {
		t.Fatal("a bandage with none in the pack must be refused")
	}
	checkWoundInvariants(t, s, r, starterDeck, "camp edges")
	// A camp with no wounds says so and refuses every heal.
	s2 := newState(t, r, 14)
	act(t, s2, r, Action{Kind: "make_camp"})
	if v := s2.View(r, 1); v.Camp.HealReason != "no wounds" || v.Camp.CanBasicAid || v.Camp.CanBandage || len(v.Wounds) != 0 {
		t.Fatalf("camp without wounds: %+v", v.Camp)
	}
	_, err = s2.Act(r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	mustFail(t, err, "you have no wounds to heal")
	// Healed cards go back to the draw pile, never the hand or Spent, and
	// each flag resets at the next rest while persisting until then.
	s3 := newState(t, r, 15)
	s3.inflictWound("hit", 3)
	act(t, s3, r, Action{Kind: "make_camp"})
	hand, spent := append([]string{}, s3.Hand...), append([]string{}, s3.Spent...)
	act(t, s3, r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	if !reflect.DeepEqual(s3.Hand, hand) || !reflect.DeepEqual(s3.Spent, spent) {
		t.Fatalf("healing touches only the draw pile: hand %v→%v spent %v→%v", hand, s3.Hand, spent, s3.Spent)
	}
	for _, card := range s3.Camp.Heals[0].Cards {
		if countOf(s3.Draw, card) == 0 {
			t.Fatalf("healed %s is not in the draw pile %v", card, s3.Draw)
		}
	}
	act(t, s3, r, Action{Kind: "break_camp"})
	if w := s3.Wounds.find("w1"); w != nil && !w.Treated {
		t.Fatalf("the treated flag persists until the next rest: %+v", w)
	}
	if v := s3.View(r, 1); len(v.Wounds) > 0 && (v.Wounds[0].BasicAid || v.Wounds[0].Bandage) {
		t.Fatalf("no healing is offered outside camp: %+v", v.Wounds)
	}
	act(t, s3, r, Action{Kind: "discard", Cards: s3.Hand[:1]})
	act(t, s3, r, Action{Kind: "make_camp"})
	for _, w := range s3.Wounds {
		if w.Treated || w.Bandaged {
			t.Fatalf("a new camp resets the flags: %+v", w)
		}
	}
	checkWoundInvariants(t, s3, r, starterDeck, "second camp")
}

// --- the battle bridge ---------------------------------------------------------

// pendingFight builds a state with a pending challenge against the lone
// Brine Mask, after wounding the given cards into one earlier wound.
func pendingFight(t *testing.T, r Rules, seed uint64, earlier []string) *State {
	t.Helper()
	s := newState(t, r, seed)
	placeAll(s, r)
	g, ok := s.hexOfGroup("brine_lone")
	if !ok {
		t.Fatal("no Brine Mask on the map")
	}
	from, ok := quietApproach(s, r, g.Pos)
	if !ok {
		t.Fatal("no quiet approach")
	}
	s.teleport(r, from)
	for _, id := range earlier {
		for _, pile := range []*[]string{&s.Hand, &s.Draw, &s.Spent} {
			if next, ok := removeFirst(*pile, id); ok {
				*pile = next
				break
			}
		}
	}
	s.addWound(earlier, "an old hit", 0)
	act(t, s, r, Action{Kind: "challenge", Group: g.ID})
	return s
}

func TestWoundsAfterBattleNeverLoseOrDuplicateCards(t *testing.T) {
	r := testRules(t)
	base := pendingFight(t, r, 6, []string{"nudge", "take_stock"})
	live := append(append(append([]string{}, base.Hand...), base.Draw...), base.Spent...)
	if len(live) != 10 || base.Wounds.Damage() != 2 {
		t.Fatalf("fixture: live %v wounds %+v", live, base.Wounds)
	}
	// Removed holds the earlier wound plus three hits; the ledger disagrees
	// with it in every way a battle can.
	hitA := []string{"nudge"} // the second copy of nudge: the same id as the old wound
	hitB := []string{"steady_guard", "steady_guard"}
	hitC := []string{"second_wind"}
	taken := append(append(append([]string{}, hitA...), hitB...), hitC...)
	rest := remove(live, taken)
	removedAll := append([]string{"nudge", "take_stock"}, taken...)
	cases := []struct {
		name      string
		removed   []string
		hand      []string
		ledger    []BattleWound
		groupings int
		newWounds int
	}{
		{"no ledger", removedAll, rest, nil, 2, 1},
		{"full ledger", removedAll, rest, []BattleWound{{Round: 1, Source: "Brine Lash", Cards: hitA}, {Round: 2, Cards: hitB}, {Round: 3, Source: "Brine Lash", Cards: hitC}}, 4, 3},
		{"partial ledger", removedAll, rest, []BattleWound{{Round: 2, Cards: hitB}}, 3, 2},
		{"ledger with cards that were healed back", removedAll, rest, []BattleWound{{Round: 1, Cards: []string{"strong_swing", "strong_swing"}}, {Round: 2, Cards: hitB}}, 3, 2},
		{"ledger entry with no surviving cards", removedAll, rest, []BattleWound{{Round: 1, Cards: []string{"try_again"}}}, 2, 1},
		{"duplicate ids split across hits", append([]string{"take_stock"}, "nudge", "nudge", "second_wind"), remove(live, []string{"nudge", "second_wind"}), []BattleWound{{Round: 1, Cards: []string{"nudge"}}, {Round: 2, Cards: []string{"nudge"}}, {Round: 3, Cards: []string{"second_wind"}}}, 3, 2},
		{"ledger claims a card twice", removedAll, rest, []BattleWound{{Round: 1, Cards: []string{"second_wind", "second_wind"}}, {Round: 2, Cards: []string{"second_wind"}}}, 3, 2},
		{"old wound partly healed in battle", []string{"nudge", "steady_guard"}, append(remove(live, []string{"steady_guard"}), "take_stock"), []BattleWound{{Round: 1, Cards: []string{"steady_guard"}}}, 2, 1},
		{"old wound fully healed in battle", []string{"second_wind"}, append(remove(live, []string{"second_wind"}), "nudge", "take_stock"), []BattleWound{{Round: 1, Cards: hitC}}, 1, 1},
		{"nothing removed at all", nil, append(append([]string{}, live...), "nudge", "take_stock"), nil, 0, 0},
		{"every card removed but one", append([]string{"nudge", "take_stock"}, live[1:]...), live[:1], []BattleWound{{Round: 1, Cards: live[1:4]}, {Round: 2, Cards: live[4:]}}, 3, 2},
	}
	for _, tc := range cases {
		s := reloaded(t, base)
		end := BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: tc.hand, Removed: tc.removed, Energy: 3, Wounds: tc.ledger}
		seqBefore := s.WoundSeq
		_, report, err := s.ResolveBattle(r, end)
		must(t, err)
		where := "case " + tc.name
		// The end-of-turn draw may have rested; the piles still hold the
		// same cards either way.
		checkWoundInvariants(t, s, r, starterDeck, where)
		// A battle may not heal below the pre-battle damage: what it healed
		// goes back to the removed pile, so the wounds keep every card the
		// battle removed plus enough to reach the old damage.
		if floor := base.Wounds.Damage(); len(tc.removed) < floor {
			got := s.Wounds.Cards()
			if s.Wounds.Damage() != floor || report.HeldBack < floor-len(tc.removed) || len(remove(got, tc.removed)) != floor-len(tc.removed) {
				t.Fatalf("%s: a battle never heals below the pre-battle %d damage: wounds %v, held back %d", where, floor, got, report.HeldBack)
			}
			continue
		}
		if got, want := sortedStrings(s.Wounds.Cards()), sortedStrings(tc.removed); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: the wounds hold exactly the removed pile: %v vs %v", where, got, want)
		}
		if len(s.Wounds) != tc.groupings || report.Wounds != tc.groupings || report.NewWounds != tc.newWounds || report.WoundCards != len(tc.removed) || report.WoundsBefore != 1 {
			t.Fatalf("%s: groupings %d (want %d) report %+v", where, len(s.Wounds), tc.groupings, report)
		}
		for _, w := range s.Wounds {
			if w.ID == "w1" && w.Source != "an old hit" {
				t.Fatalf("%s: the old wound changed its source: %+v", where, w)
			}
			if w.ID != "w1" && (w.Source == "" || w.Round != s.Round() || w.Size != len(w.Cards)) {
				t.Fatalf("%s: a new wound is named, dated and sized: %+v", where, w)
			}
			if w.Treated || w.Bandaged {
				t.Fatalf("%s: a wound after battle carries camp flags: %+v", where, w)
			}
		}
		if s.WoundSeq != seqBefore+tc.newWounds {
			t.Fatalf("%s: wound ids advance once per new wound: %d → %d", where, seqBefore, s.WoundSeq)
		}
		for _, bw := range tc.ledger {
			if w := s.Wounds.find(fmt.Sprintf("w%d", seqBefore+1)); w != nil && bw.Round == w.BattleRound && len(bw.Cards) > 0 && w.Source == "" {
				t.Fatalf("%s: a ledger wound without a source must be named after the enemy: %+v", where, w)
			}
		}
	}
	// A draw and a normal defeat restore the earlier wound exactly, however
	// the battle mangled the piles; the defeat adds one wound of 2.
	for _, result := range []string{"draw", "defeat"} {
		s := reloaded(t, base)
		_, report, err := s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: result, Removed: append([]string{}, live...), Hand: []string{"nudge", "take_stock"}, Energy: 99,
			Wounds: []BattleWound{{Round: 1, Cards: live}}})
		must(t, err)
		checkWoundInvariants(t, s, r, starterDeck, result)
		if w := s.Wounds.find("w1"); w == nil || !reflect.DeepEqual(w.Cards, []string{"nudge", "take_stock"}) || w.Source != "an old hit" {
			t.Fatalf("%s: the earlier wound comes back unchanged: %+v", result, s.Wounds)
		}
		if result == "draw" && (len(s.Wounds) != 1 || report.NewWounds != 0 || report.DefeatWounds != 0) {
			t.Fatalf("a draw adds nothing: %+v report %+v", s.Wounds, report)
		}
		if result == "defeat" && (len(s.Wounds) != 2 || report.NewWounds != 1 || report.DefeatWounds != 2 || s.Wounds[1].Damage() != 2 || s.Wounds[1].Source != "defeat by Brine Mask" || s.Phase != PhaseCamp || s.Camp.Reason != "defeat") {
			t.Fatalf("a defeat is one wound of 2 and a forced rest: %+v report %+v phase %s", s.Wounds, report, s.Phase)
		}
		if result == "defeat" {
			for _, w := range s.Wounds {
				if w.Treated || w.Bandaged {
					t.Fatalf("the forced rest opens with untreated wounds: %+v", w)
				}
			}
			if v := s.View(r, 1); v.Camp.Untreated != 2 || !v.Camp.CanBasicAid {
				t.Fatalf("the forced rest offers basic aid on both wounds: %+v", v.Camp)
			}
		}
	}
}

// remove drops one copy of each id in taken from list.
func remove(list, taken []string) []string {
	out := append([]string{}, list...)
	for _, id := range taken {
		out, _ = removeFirst(out, id)
	}
	return out
}

func TestWarlordOutcomesRegroupAndMendWounds(t *testing.T) {
	r := testRules(t)
	// The deadline camp heals like any other, and the fight starts from the
	// healed piles.
	s := newState(t, r, 6)
	// Eight damage in two wounds: basic aid (two 1d3 rolls) cannot close both.
	s.inflictWound("Brine Lash", 5)
	s.inflictWound("defeat by Bell Diver", 3)
	for s.Camp == nil || !s.Camp.Deadline {
		if s.Phase == PhaseTurn {
			act(t, s, r, Action{Kind: "make_camp"})
		} else {
			act(t, s, r, Action{Kind: "break_camp"})
		}
	}
	if v := s.View(r, 1); !v.Camp.Deadline || v.Camp.Untreated != len(s.Wounds) {
		t.Fatalf("the deadline camp: %+v", v.Camp)
	}
	twin := reloaded(t, s)
	for _, a := range []Action{{Kind: "camp_service", Service: "basic_aid", Wound: "all"}, {Kind: "break_camp"}} {
		if err := stepTwin(t, r, s, twin, a, "deadline camp"); err != nil {
			t.Fatal(err)
		}
		twin = reloaded(t, twin)
	}
	checkWoundInvariants(t, s, r, starterDeck, "deadline fight pending")
	if len(s.Wounds) == 0 || s.Wounds.Damage() < 2 {
		t.Fatalf("the deadline camp cannot close eight damage with basic aid: %+v", s.Wounds)
	}
	if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || len(s.PendingBattle.WoundsBefore) != len(s.Wounds) {
		t.Fatalf("the deadline fight starts from the healed wounds: %+v", s.PendingBattle)
	}
	setup, err := s.Setup(r)
	must(t, err)
	if !reflect.DeepEqual(sortedStrings(setup.Wounds), sortedStrings(s.Wounds.Cards())) || len(setup.Hand)+len(setup.Draw)+len(setup.Spent) != s.LiveCards() {
		t.Fatalf("the battle setup carries the wounded cards: %+v", setup)
	}
	// Losing to the Warlord ends the run with the battle's regrouped wounds.
	lost := reloaded(t, s)
	live := append(append(append([]string{}, lost.Hand...), lost.Draw...), lost.Spent...)
	ledger := []BattleWound{{Round: 2, Source: "Regent's Blade", Cards: live[:2]}, {Round: 5, Cards: live[2:4]}}
	_, report, err := lost.ResolveBattle(r, BattleEnd{BattleID: lost.PendingBattle.ID, Result: "defeat", Hand: live[4:], Removed: append(append([]string{}, lost.Wounds.Cards()...), live[:4]...), Energy: 1, Wounds: ledger})
	must(t, err)
	checkWoundInvariants(t, lost, r, starterDeck, "warlord defeat")
	if !report.RunOver || lost.Result != "defeat" || lost.Phase != PhaseDefeat || report.NewWounds != 2 || len(lost.Wounds) != len(s.Wounds)+2 || report.WoundCards != lost.Wounds.Damage() {
		t.Fatalf("the Warlord's win regroups the wounds and ends the run: %+v wounds %+v", report, lost.Wounds)
	}
	if w := lost.Wounds[len(lost.Wounds)-2]; w.Source != "Regent's Blade" || w.BattleRound != 2 || !reflect.DeepEqual(w.Cards, live[:2]) {
		t.Fatalf("the named hit: %+v", w)
	}
	if w := lost.Wounds[len(lost.Wounds)-1]; w.Source != lost.LastBattle.Name || w.BattleRound != 5 {
		t.Fatalf("the unnamed hit is named after the Warlord: %+v", w)
	}
	if _, err = lost.Act(r, Action{Kind: "make_camp"}); err == nil {
		t.Fatal("a finished run accepts no action")
	}
	if v := lost.View(r, 1); v.Health != lost.LiveCards() || v.MaxHealth != 12 || len(v.WoundCards) != lost.Wounds.Damage() {
		t.Fatalf("the final view: %d/%d", v.Health, v.MaxHealth)
	}
	// Beating the Warlord mends every wound: every card is live again, the
	// report counts the mended damage, and the state is terminal.
	won := reloaded(t, s)
	damage := won.Wounds.Damage() + 2
	_, report, err = won.ResolveBattle(r, BattleEnd{BattleID: won.PendingBattle.ID, Result: "victory", Hand: live[2:], Removed: append(append([]string{}, won.Wounds.Cards()...), live[:2]...), Energy: 4, Wounds: []BattleWound{{Round: 1, Cards: live[:2]}}})
	must(t, err)
	checkWoundInvariants(t, won, r, starterDeck, "warlord victory")
	if !report.RegionWon || won.Result != "victory" || report.WoundsMended != damage || len(won.Wounds) != 0 || won.LiveCards() != 12 || report.WoundCards != 0 || report.Wounds != 0 {
		t.Fatalf("the region win mends everything: %+v wounds %+v live %d", report, won.Wounds, won.LiveCards())
	}
	if v := won.View(r, 1); v.Health != 12 || v.MaxHealth != 12 || len(v.Wounds) != 0 {
		t.Fatalf("the winning view: %+v", v)
	}
	// The drawn deadline fight repeats from the same wounds.
	drawn := reloaded(t, s)
	_, _, err = drawn.ResolveBattle(r, BattleEnd{BattleID: drawn.PendingBattle.ID, Result: "draw", Removed: live, Wounds: []BattleWound{{Round: 1, Cards: live}}})
	must(t, err)
	checkWoundInvariants(t, drawn, r, starterDeck, "deadline draw")
	if drawn.PendingBattle == nil || !reflect.DeepEqual(drawn.Wounds.Cards(), s.Wounds.Cards()) || len(drawn.Wounds) != len(s.Wounds) || !reflect.DeepEqual(drawn.PendingBattle.WoundsBefore.Cards(), s.Wounds.Cards()) {
		t.Fatalf("the repeated fight starts from the pre-battle wounds: %+v vs %+v", drawn.Wounds, s.Wounds)
	}
}

// --- the view ------------------------------------------------------------------

// viewJSON decodes the view to a generic map for shape checks.
func viewJSON(t *testing.T, s *State, r Rules) map[string]any {
	t.Helper()
	raw, err := json.Marshal(s.View(r, 1))
	must(t, err)
	var out map[string]any
	must(t, json.Unmarshal(raw, &out))
	return out
}

func requireList(t *testing.T, m map[string]any, path ...string) []any {
	t.Helper()
	var cur any = m
	for i, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%v: %q is not an object", path[:i], path[i-1])
		}
		cur, ok = obj[key]
		if !ok {
			t.Fatalf("%v: missing key %q", path, key)
		}
	}
	list, ok := cur.([]any)
	if !ok {
		t.Fatalf("%v must be a list, got %T (%v)", path, cur, cur)
	}
	return list
}

func TestViewWoundListsAreNeverNull(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 17)
	check := func(where string, wantCamp bool) {
		t.Helper()
		m := viewJSON(t, s, r)
		requireList(t, m, "wounds")
		requireList(t, m, "wound_cards")
		for _, w := range requireList(t, m, "wounds") {
			wm := w.(map[string]any)
			if _, ok := wm["cards"].([]any); !ok {
				t.Fatalf("%s: wound cards must be a list: %v", where, w)
			}
		}
		if camp, ok := m["camp"]; ok && camp != nil {
			requireList(t, m, "camp", "heals")
			for _, h := range requireList(t, m, "camp", "heals") {
				hm := h.(map[string]any)
				if _, ok := hm["cards"].([]any); !ok {
					t.Fatalf("%s: heal cards must be a list: %v", where, h)
				}
			}
		} else if wantCamp {
			t.Fatalf("%s: the camp view is missing", where)
		}
		if lr, ok := m["last_rest"]; ok && lr != nil {
			requireList(t, m, "last_rest", "camp", "heals")
		}
	}
	check("fresh", false)
	act(t, s, r, Action{Kind: "make_camp"})
	check("camp without wounds", true)
	act(t, s, r, Action{Kind: "break_camp"})
	check("after a rest without wounds", false)
	s.inflictWound("Brine Lash", 2)
	check("wounded", false)
	act(t, s, r, Action{Kind: "make_camp"})
	check("camp with wounds", true)
	act(t, s, r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	check("camp after healing", true)
	act(t, s, r, Action{Kind: "break_camp"})
	check("after a healing rest", false)
	if s.LastRest == nil || s.LastRest.Healed != s.LastRest.Camp.Healed || s.LastRest.Drew != s.LastRest.Camp.Drew {
		t.Fatalf("the rest summary reports the camp's healing and draw: %+v", s.LastRest)
	}
	// The same through a save: null-free after reload too, and the hidden
	// state stays hidden.
	s = reloaded(t, s)
	check("reloaded", false)
	raw, _ := json.Marshal(s.View(r, 1))
	for _, forbidden := range []string{`"stack"`, `"core_deal"`, `"rng_position"`, `"route"`, `"hand_before"`, `"wounds_before"`, `"wound_seq"`} {
		if containsBytes(raw, forbidden) {
			t.Fatalf("the view leaks %s", forbidden)
		}
	}
	// The view's wound text and health fields.
	v := s.View(r, 1)
	if len(v.Wounds) > 0 {
		w := v.Wounds[0]
		if w.Source != "Brine Lash" || w.Taken == "" || w.Size != 2 || w.Damage != len(w.Cards) || w.Damage > w.Size {
			t.Fatalf("the view wound: %+v", w)
		}
		if _, ok := v.CardNames[w.Cards[0]]; !ok {
			t.Fatalf("wounded cards are named: %v", v.CardNames)
		}
	}
	if v.MaxHealth != v.Health+len(v.WoundCards) || v.MaxHealth != 12 {
		t.Fatalf("health: %d/%d with %d wound cards", v.Health, v.MaxHealth, len(v.WoundCards))
	}
}

func containsBytes(raw []byte, needle string) bool {
	return indexOfBytes(raw, []byte(needle)) >= 0
}

func indexOfBytes(hay, needle []byte) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

// --- legacy saves --------------------------------------------------------------

func TestLegacySaveWithFlatWoundsHealsAndFights(t *testing.T) {
	r := testRules(t)
	// A save written before wounds were grouped: a flat list of card ids, no
	// wound_seq, and a camp without heals or hand_drawn.
	s := newState(t, r, 6)
	placeAll(s, r)
	for _, id := range []string{"nudge", "nudge"} {
		for _, pile := range []*[]string{&s.Hand, &s.Draw, &s.Spent} {
			if next, ok := removeFirst(*pile, id); ok {
				*pile = next
				break
			}
		}
	}
	raw, _ := json.Marshal(s)
	var generic map[string]any
	must(t, json.Unmarshal(raw, &generic))
	generic["wounds"] = []string{"nudge", "nudge"}
	delete(generic, "wound_seq")
	raw, _ = json.Marshal(generic)
	var legacy State
	must(t, json.Unmarshal(raw, &legacy))
	if len(legacy.Wounds) != 1 || legacy.Wounds[0].ID != "w0" || legacy.Wounds.Damage() != 2 || legacy.Wounds[0].Size != 2 || legacy.Wounds[0].Source != "earlier wounds" || legacy.WoundSeq != 0 {
		t.Fatalf("legacy wounds load as one grouping: %+v", legacy.Wounds)
	}
	checkWoundInvariants(t, &legacy, r, starterDeck, "legacy loaded")
	// The legacy wound has no date, so the view shows none.
	if v := legacy.View(r, 1); v.Wounds[0].Taken != "" || v.Wounds[0].Damage != 2 || len(v.WoundCards) != 2 {
		t.Fatalf("legacy view: %+v", v.Wounds)
	}
	// It is re-saved in the grouped form and reloads identically.
	again := reloaded(t, &legacy)
	if !reflect.DeepEqual(again.Wounds, legacy.Wounds) {
		t.Fatalf("the grouped form round-trips: %+v vs %+v", again.Wounds, legacy.Wounds)
	}
	// Camp, heal and the ids never collide with the legacy w0.
	s = &legacy
	s.Inventory["bandage"] = 1
	twin := reloaded(t, s)
	for _, a := range []Action{{Kind: "make_camp"}, {Kind: "camp_service", Service: "basic_aid", Wound: "w0"}, {Kind: "camp_service", Service: "bandage", Wound: "w0"}, {Kind: "break_camp"}} {
		if err := stepTwin(t, r, s, twin, a, "legacy camp"); err != nil && !(a.Service == "bandage") {
			t.Fatal(err)
		}
		twin = reloaded(t, twin)
		checkWoundInvariants(t, s, r, starterDeck, "legacy "+a.Kind)
	}
	s.inflictWound("Wayside Shrine", 1)
	if s.Wounds[len(s.Wounds)-1].ID != "w1" {
		t.Fatalf("the first new wound after a legacy save is w1: %+v", s.Wounds)
	}
	// It fights: the pending battle remembers the legacy grouping and the
	// result regroups around it.
	g, ok := s.hexOfGroup("brine_lone")
	if !ok {
		t.Fatal("no Brine Mask")
	}
	from, ok := quietApproach(s, r, g.Pos)
	if !ok {
		t.Fatal("no quiet approach")
	}
	s.teleport(r, from)
	act(t, s, r, Action{Kind: "challenge", Group: g.ID})
	if !reflect.DeepEqual(s.PendingBattle.WoundsBefore, s.Wounds) {
		t.Fatalf("the pending battle keeps the groupings: %+v", s.PendingBattle.WoundsBefore)
	}
	live := append(append(append([]string{}, s.Hand...), s.Draw...), s.Spent...)
	_, report, err := s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: live[1:], Removed: append(s.Wounds.Cards(), live[0]), Energy: 2, Wounds: []BattleWound{{Round: 3, Source: "Brine Lash", Cards: live[:1]}}})
	must(t, err)
	checkWoundInvariants(t, s, r, starterDeck, "legacy fought")
	if report.NewWounds != 1 || s.Wounds[len(s.Wounds)-1].Source != "Brine Lash" {
		t.Fatalf("the fight added one named wound: %+v", s.Wounds)
	}
	// A legacy pending battle with flat wounds_before resolves too.
	var pending State
	must(t, json.Unmarshal([]byte(`{"wounds":["nudge"],"hand":["try_again"],"draw":["steady_guard"],"spent":[],"pending_battle":{"id":"old","kind":"challenge","group":"brine_lone","name":"Brine Mask","target":"x","energy_in":3,"hand_before":["try_again"],"draw_before":["steady_guard"],"spent_before":[],"wounds_before":["nudge"]}}`), &pending))
	if len(pending.PendingBattle.WoundsBefore) != 1 || pending.PendingBattle.WoundsBefore[0].ID != "w0" {
		t.Fatalf("legacy wounds_before: %+v", pending.PendingBattle.WoundsBefore)
	}
	// A legacy camp (no fresh hand drawn at the rest) draws it at break camp,
	// and its rest summary still lists heals as a list.
	s2 := newState(t, r, 6)
	act(t, s2, r, Action{Kind: "make_camp"})
	raw, _ = json.Marshal(s2)
	must(t, json.Unmarshal(raw, &generic))
	camp := generic["camp"].(map[string]any)
	delete(camp, "heals")
	delete(camp, "hand_drawn")
	delete(camp, "drew")
	camp["mend_rolls"] = []map[string]any{{"card": "nudge", "roll": 4, "success": true}}
	generic["draw"] = append(append([]any{}, generic["draw"].([]any)...), generic["hand"].([]any)...)
	generic["hand"] = []any{}
	raw, _ = json.Marshal(generic)
	var old State
	must(t, json.Unmarshal(raw, &old))
	if old.Camp == nil || old.Camp.HandDrawn || len(old.Hand) != 0 {
		t.Fatalf("legacy camp fixture: %+v hand %d", old.Camp, len(old.Hand))
	}
	m := viewJSON(t, &old, r)
	requireList(t, m, "camp", "heals")
	act(t, &old, r, Action{Kind: "break_camp"})
	if len(old.Hand) != 4 || old.LastRest.Drew != 4 || old.Phase != PhaseTurn {
		t.Fatalf("a legacy camp draws at break camp: hand %d drew %d", len(old.Hand), old.LastRest.Drew)
	}
	checkWoundInvariants(t, &old, r, starterDeck, "legacy camp broken")
}

// legacyCamp builds a camp as the old code saved it: no heals list, the hand
// not yet drawn, mend_rolls present.
func legacyCamp(t *testing.T, r Rules) *State {
	t.Helper()
	s := newState(t, r, 6)
	act(t, s, r, Action{Kind: "make_camp"})
	raw, _ := json.Marshal(s)
	var generic map[string]any
	must(t, json.Unmarshal(raw, &generic))
	camp := generic["camp"].(map[string]any)
	delete(camp, "heals")
	delete(camp, "hand_drawn")
	delete(camp, "drew")
	camp["mend_rolls"] = []map[string]any{{"card": "nudge", "roll": 4, "success": true}}
	raw, _ = json.Marshal(generic)
	var old State
	must(t, json.Unmarshal(raw, &old))
	if old.Camp == nil || old.Camp.HandDrawn || old.Camp.Heals != nil {
		t.Fatalf("legacy camp fixture: %+v", old.Camp)
	}
	return &old
}

// The rest summary copies the camp as it was; a camp saved by the old code
// has no heals list, so last_rest.camp.heals comes out null once.
func TestLegacyCampRestSummaryListsHealsAsAList(t *testing.T) {
	r := testRules(t)
	old := legacyCamp(t, r)
	act(t, old, r, Action{Kind: "break_camp"})
	m := viewJSON(t, old, r)
	requireList(t, m, "last_rest", "camp", "heals")
}

func TestActionDecodesTheWoundField(t *testing.T) {
	var a Action
	must(t, json.Unmarshal([]byte(`{"kind":"camp_service","service":"bandage","wound":"w3","item":"bandage"}`), &a))
	if a.Kind != "camp_service" || a.Service != "bandage" || a.Wound != "w3" || a.Item != "bandage" {
		t.Fatalf("decoded %+v", a)
	}
	raw, _ := json.Marshal(Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	if string(raw) != `{"kind":"camp_service","service":"basic_aid","wound":"all"}` {
		t.Fatalf("encoded %s", raw)
	}
}

// --- rest ordering -------------------------------------------------------------

// Shaken ("hand size −1 ... expires at your next rest") must not shape the
// hand the rest deals for the round that begins after it expired. Before
// the rest drew the fresh hand, break camp expired the statuses first and
// then drew; now the draw happens at the start of the rest with the status
// still active, so the new round begins a card short.
func TestRestDrawsTheFreshHandAfterMapStatusesExpire(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 19)
	if r.handDelta("shaken") != -1 {
		t.Skip("shaken no longer changes the hand size")
	}
	act(t, s, r, Action{Kind: "discard", Cards: s.Hand[:1]})
	s.addStatus("shaken", 1)
	if s.effectiveHandSize(r) != 3 {
		t.Fatalf("shaken hand size %d", s.effectiveHandSize(r))
	}
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	if len(s.Statuses) != 0 {
		t.Fatalf("map statuses expire at the rest: %+v", s.Statuses)
	}
	if len(s.Hand) != 4 {
		t.Fatalf("the round after Shaken expired begins with a full hand of 4, got %d (drew %d at the rest)", len(s.Hand), s.LastRest.Drew)
	}
}

// --- site mending across groupings ------------------------------------------------

func TestSiteMendingHealsPointsOldestWoundFirst(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 9)
	placeAll(s, r)
	s.inflictWound("first", 1)
	s.inflictWound("second", 2)
	village := s.hexOfSite("hollow_ford")
	s.teleport(r, village)
	act(t, s, r, Action{Kind: "use_site", Service: "influence", Cards: append([]string{}, s.Hand[:4]...)})
	_, err := s.Act(r, Action{Kind: "use_site", Service: "mend", Count: 4})
	mustFail(t, err, "you have 3 damage to mend")
	act(t, s, r, Action{Kind: "use_site", Service: "mend", Count: 2})
	checkWoundInvariants(t, s, r, starterDeck, "village mend")
	if len(s.Wounds) != 1 || s.Wounds[0].Source != "second" || s.Wounds[0].Damage() != 1 || s.Wounds[0].Size != 2 || s.Turn.Influence != 0 {
		t.Fatalf("two points heal the oldest wound first: %+v influence %d", s.Wounds, s.Turn.Influence)
	}
	if w := s.Wounds[0]; countOf(s.Draw, "nudge")+countOf(s.Hand, "nudge")+countOf(s.Spent, "nudge")+countOf(w.Cards, "nudge") != 2 {
		t.Fatalf("both copies of nudge are accounted for: %v / %v / %v / %v", s.Hand, s.Draw, s.Spent, w.Cards)
	}
}
