package expedition

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var starterDeck = []string{"steady_guard", "steady_guard", "steady_guard", "nudge", "nudge", "try_again", "try_again", "strong_swing", "strong_swing", "take_stock", "take_stock", "second_wind"}

func testRules(t *testing.T) Rules {
	t.Helper()
	lib := authoredLibrary(t)
	root := contentRoot(t)
	cfg, err := LoadConfig(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := LoadPack(root, cfg, lib)
	if err != nil {
		t.Fatal(err)
	}
	return Rules{Config: cfg, Pack: pack, Cards: lib.Cards, Statuses: lib.Statuses}
}

func newState(t *testing.T, r Rules, seed uint64) *State {
	t.Helper()
	s, err := New(r, "salt_marches", seed, starterDeck, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustFail(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

func act(t *testing.T, s *State, r Rules, a Action) Rewards {
	t.Helper()
	rewards, err := s.Act(r, a)
	must(t, err)
	return rewards
}

// placeAll explores every slot in stack order from wherever the tiles must
// go, placing the countryside stack then the core deal, without spending
// Move: a test fixture for rules that need the whole map.
func placeAll(s *State, r Rules) {
	region := r.Pack.Regions[s.Region]
	for _, slot := range region.Slots {
		if slot.Kind != "countryside" {
			continue
		}
		center, _ := hexFromPair(slot.Center)
		tile := s.Stack[0]
		s.Stack = s.Stack[1:]
		s.placeTile(r.Pack, slot.ID, tile, center)
	}
	for _, slot := range region.Slots {
		if slot.Kind == "core" {
			center, _ := hexFromPair(slot.Center)
			s.placeTile(r.Pack, slot.ID, s.CoreDeal[slot.ID], center)
		}
	}
	s.refreshSight(r)
}

func (s *State) hexOfGroup(groupDef string) (GroupState, bool) {
	for _, g := range s.Groups {
		if g.Group == groupDef && !g.Defeated {
			return g, true
		}
	}
	return GroupState{}, false
}

func (s *State) hexOfSite(id string) Hex {
	if st := s.site(id); st != nil {
		return st.Pos
	}
	return Hex{}
}

// teleport puts the player somewhere for a scenario, as the start of a turn.
func (s *State) teleport(r Rules, h Hex) {
	s.Player = h
	s.Turn = TurnState{Number: s.Turn.Number + 1, StartHex: h}
	s.Phase = PhaseTurn
	s.refreshSight(r)
}

// woundCard moves one copy of a card out of whichever pile holds it into the wounds.
func woundCard(t *testing.T, s *State, id string) {
	t.Helper()
	for _, pile := range []*[]string{&s.Hand, &s.Draw, &s.Spent} {
		if next, ok := removeFirst(*pile, id); ok {
			*pile = next
			s.Wounds = append(s.Wounds, id)
			return
		}
	}
	t.Fatalf("no live copy of %s to wound", id)
}

// approachFrom finds a placed, passable hex next to target from which moving
// onto target provokes nobody.
func quietApproach(s *State, r Rules, target Hex) (Hex, bool) {
	for _, n := range Neighbors(target) {
		if _, _, ok := s.moveCost(r, n); !ok {
			continue
		}
		if s.provokedBy(n, target) != nil || s.openGroupAt(n) != nil || s.warlordAt(n) {
			continue
		}
		return n, true
	}
	return Hex{}, false
}

func approachFrom(t *testing.T, s *State, r Rules, target Hex) Hex {
	t.Helper()
	if h, ok := quietApproach(s, r, target); ok {
		return h
	}
	t.Fatalf("no quiet approach to %v", target)
	return Hex{}
}

func reloaded(t *testing.T, s *State) *State {
	t.Helper()
	raw, err := json.Marshal(s)
	must(t, err)
	var out State
	must(t, json.Unmarshal(raw, &out))
	return &out
}

func TestNewRegionIsDeterministicAndStartsWithTheStartTile(t *testing.T) {
	r := testRules(t)
	a, b := newState(t, r, 7), newState(t, r, 7)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("the same seed must build the identical region and hand")
	}
	c := newState(t, r, 8)
	if reflect.DeepEqual(a.Stack, c.Stack) && reflect.DeepEqual(a.Draw, c.Draw) {
		t.Fatal("a different seed should shuffle differently")
	}
	if len(a.Placed) != 1 || a.Placed[0].Tile != "meadow_crossing" || a.Placed[0].Center != (Hex{}) {
		t.Fatalf("only the start tile is placed: %+v", a.Placed)
	}
	if len(a.Stack) != 5 || len(a.CoreDeal) != 2 || len(a.Hand) != 4 || len(a.Draw) != 8 || a.Energy != 10 || a.Round != 1 || a.Night || a.Phase != PhaseTurn {
		t.Fatalf("unexpected start: stack %d core %d hand %d draw %d energy %d", len(a.Stack), len(a.CoreDeal), len(a.Hand), len(a.Draw), a.Energy)
	}
	if len(a.Warlord.Route) != r.Config.Warlord.StartDistance+1 || a.Warlord.Route[len(a.Warlord.Route)-1] != a.Warlord.Goal {
		t.Fatalf("the Warlord's route runs %d hexes to his goal: %v", r.Config.Warlord.StartDistance, a.Warlord.Route)
	}
	if _, _, placed := a.tileAt(r.Pack, a.Warlord.Goal); placed {
		t.Fatal("the goal lies on an unplaced core tile at the start")
	}
	// The start tile's village is in sight (2 hexes by day) and the start is seen.
	v := a.View(r, 1)
	seen := 0
	for _, h := range v.Hexes {
		if h.Seen {
			seen++
		}
	}
	if seen != 7 || v.Hexes[0].Tile != "meadow_crossing" {
		t.Fatalf("the whole start tile is in sight by day: %d seen", seen)
	}
}

func TestViewNeverLeaksHiddenState(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 11)
	// Explore east at night so most of the new tile is out of sight.
	s.Night = true
	s.refreshSight(r)
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: s.Hand[:1]})
	act(t, s, r, Action{Kind: "explore", Slot: "east", Cards: s.Hand[:2]})
	raw, _ := json.Marshal(s.View(r, 1))
	text := string(raw)
	for _, forbidden := range []string{`"stack"`, `"core_deal"`, `"rng_position"`, `"route"`, `"hand_before"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("the view leaks %s", forbidden)
		}
	}
	var view View
	must(t, json.Unmarshal(raw, &view))
	hidden := 0
	for _, h := range view.Hexes {
		if h.Tile != "meadow_crossing" && !h.Seen {
			hidden++
			if h.Site != nil || h.Group != nil || h.Warlord {
				t.Fatalf("an unseen hex shows its contents: %+v", h)
			}
		}
	}
	if hidden == 0 {
		t.Fatal("at night with sight 1, part of the new tile must be unseen")
	}
	// The unplaced core tiles are never named.
	for _, tile := range []string{"greywharf_strand", "drowned_reach"} {
		if strings.Contains(text, tile) {
			t.Fatalf("the view names the unexplored core tile %s", tile)
		}
	}
	for _, slot := range view.Slots {
		if slot.Kind == "core" && slot.Placed {
			t.Fatal("core slots are not placed yet")
		}
	}
}

// Every list in the view is a JSON array even when empty; only the documented
// optional blocks may be null or absent. Clients read lists without guards.
func TestViewListsAreNeverNull(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 11)
	s.Draw, s.Spent, s.Wounds, s.Hand, s.Log, s.Inventory, s.Statuses = nil, nil, nil, nil, nil, nil, nil
	s.Turn.CardsUsed = 0
	for _, v := range []View{s.View(r, 1), func() View {
		w := newState(t, r, 11)
		act(t, w, r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: w.Hand[:1]})
		act(t, w, r, Action{Kind: "explore", Slot: "east", Cards: w.Hand[:2]})
		return w.View(r, 2)
	}()} {
		raw, _ := json.Marshal(v)
		var generic map[string]any
		must(t, json.Unmarshal(raw, &generic))
		var walk func(prefix string, value any)
		walk = func(prefix string, value any) {
			switch typed := value.(type) {
			case map[string]any:
				for key, child := range typed {
					if child == nil {
						t.Fatalf("%s.%s is null in the view: %s", prefix, key, raw)
					}
					walk(prefix+"."+key, child)
				}
			case []any:
				for i, child := range typed {
					walk(fmt.Sprintf("%s[%d]", prefix, i), child)
				}
			}
		}
		walk("view", generic)
	}
}

func TestExploreFollowsTheStackCountrysideBeforeCore(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 3)
	stack := append([]string{}, s.Stack...)
	// From the centre of the start tile no slot is adjacent; walk to its edge.
	if len(s.View(r, 1).Explores) != 0 {
		t.Fatal("the start hex touches no empty slot")
	}
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: s.Hand[:1]})
	for _, e := range s.View(r, 1).Explores {
		if e.ID == "strand" || e.ID == "reach" || e.ID == "far_east" {
			t.Fatalf("%s is not next to the start tile's edge", e.ID)
		}
	}
	_, err := s.Act(r, Action{Kind: "explore", Slot: "strand"})
	mustFail(t, err, "not an empty tile slot next to you")
	_, err = s.Act(r, Action{Kind: "explore", Slot: "east"})
	mustFail(t, err, "exploring costs 2 Move; you have 0")
	_, err = s.Act(r, Action{Kind: "explore", Slot: "east", Cards: s.Hand[:1]})
	mustFail(t, err, "exploring costs 2 Move; you have 1")
	if s.Turn.CardsUsed != 1 || len(s.Spent) != 1 {
		t.Fatal("a refused action must not spend cards")
	}
	hand := append([]string{}, s.Hand...)
	act(t, s, r, Action{Kind: "explore", Slot: "east", Cards: hand[:2]})
	if s.Placed[1].Tile != stack[0] || s.Placed[1].Slot != "east" || len(s.Stack) != 4 || s.Turn.Move != 0 || s.Turn.CardsUsed != 3 || len(s.Spent) != 3 {
		t.Fatalf("explore placed %+v, stack %v, move %d", s.Placed[1], s.Stack, s.Turn.Move)
	}
	// Energy pays too: 10 energy = 2 Move.
	act(t, s, r, Action{Kind: "explore", Slot: "northeast", Energy: 10})
	if s.Energy != 0 || s.Placed[2].Tile != stack[1] {
		t.Fatalf("energy payment failed: energy %d tile %s", s.Energy, s.Placed[2].Tile)
	}
	// Core slots wait for the countryside stack.
	s.teleport(r, Hex{3, 1})
	s.Turn.Move = 10
	for _, e := range s.View(r, 1).Explores {
		if e.ID == "far_east" && e.Ready {
			continue
		}
		if e.ID == "far_east" {
			t.Fatalf("far_east should be explorable: %+v", e)
		}
	}
	act(t, s, r, Action{Kind: "explore", Slot: "far_east"})
	s.teleport(r, Hex{5, 2})
	s.Turn.Move = 10
	_, err = s.Act(r, Action{Kind: "explore", Slot: "strand"})
	mustFail(t, err, "core tiles come after the countryside tiles")
	for _, slot := range []string{"south", "west"} {
		center := slotCenter(r.Pack.Regions[s.Region], slot)
		s.placeTile(r.Pack, slot, s.Stack[0], center)
		s.Stack = s.Stack[1:]
	}
	s.teleport(r, Hex{5, 2})
	s.Turn.Move = 10
	act(t, s, r, Action{Kind: "explore", Slot: "strand"})
	if last := s.Placed[len(s.Placed)-1]; last.Tile != s.CoreDeal["strand"] || r.Pack.Tiles[last.Tile].Kind != "core" {
		t.Fatalf("the core slot received %s", last.Tile)
	}
}

func TestMovementCostsPaymentsAndImpassableTerrain(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 5)
	// The start tile: east (1,0) plains 1, northeast (1,-1) forest 2, west (-1,0) lake.
	_, err := s.Act(r, Action{Kind: "move", Target: &Hex{-1, 0}, Cards: s.Hand[:1]})
	mustFail(t, err, "lake is impassable")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{2, 0}, Cards: s.Hand[:1]})
	mustFail(t, err, "adjacent")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{1, -1}, Cards: s.Hand[:1]})
	mustFail(t, err, "costs 2 Move; you have 1")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{1, 0}, Energy: 3})
	mustFail(t, err, "energy pays for Move 5 at a time")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{1, 0}, Energy: 15})
	mustFail(t, err, "you have 10 energy")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: []string{"trailblaze"}})
	mustFail(t, err, "not in your hand")
	// A card and energy together: forest for 1 card + 5 energy.
	hand := append([]string{}, s.Hand...)
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, -1}, Cards: hand[:1], Energy: 5})
	if s.Player != (Hex{1, -1}) || s.Energy != 5 || s.Turn.Move != 0 || len(s.Spent) != 1 {
		t.Fatalf("after the forest: %v energy %d move %d", s.Player, s.Energy, s.Turn.Move)
	}
	// Leftover Move stays this turn: pay 2 cards for plains (1) and keep 1.
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: hand[1:3]})
	if s.Turn.Move != 1 {
		t.Fatalf("leftover Move should be 1, got %d", s.Turn.Move)
	}
	act(t, s, r, Action{Kind: "move", Target: &Hex{0, 0}})
	if s.Turn.Move != 0 || s.Player != (Hex{}) {
		t.Fatal("the leftover Move paid for the plains")
	}
	// Night: forest costs 3.
	s.Night = true
	s.Turn.Move = 2
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{1, -1}})
	mustFail(t, err, "costs 3 Move; you have 2")
	s.Turn.Move = 3
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, -1}})
	// The view offers only legal moves with their costs and blocked reasons.
	s.teleport(r, Hex{})
	for _, m := range s.View(r, 1).Moves {
		switch m.To {
		case Hex{-1, 0}:
			if m.Blocked == "" || m.Cost != 0 {
				t.Fatalf("the lake must be blocked: %+v", m)
			}
		case Hex{1, -1}:
			if m.Cost != 3 || m.Terrain != "forest" {
				t.Fatalf("forest at night costs 3: %+v", m)
			}
		}
	}
	// Move pool is lost at end of turn.
	s.Night = false
	s.Turn.Move, s.Turn.CardsUsed = 4, 1
	act(t, s, r, Action{Kind: "end_turn"})
	if s.Turn.Move != 0 {
		t.Fatal("unspent Move is lost at end of turn")
	}
}

func TestMapCardsAndEnergyRules(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 2)
	s.Hand = []string{"strong_swing", "second_wind", "take_stock", "pathfinder_paths_variant_node_1", "pathfinder_paths_variant_node_3", "trailblaze"}
	_, err := s.Act(r, Action{Kind: "play_map_card", Card: "strong_swing"})
	mustFail(t, err, "has no map effect")
	act(t, s, r, Action{Kind: "play_map_card", Card: "second_wind"})
	if s.Energy != 20 || s.Turn.CardsUsed != 1 || countOf(s.Spent, "second_wind") != 1 {
		t.Fatalf("Second Wind gives +10 energy on the map: energy %d", s.Energy)
	}
	draw := len(s.Draw)
	act(t, s, r, Action{Kind: "play_map_card", Card: "take_stock"})
	if len(s.Draw) != draw-2 || len(s.Hand) != 6 {
		t.Fatalf("Take Stock draws 2: draw %d hand %d", len(s.Draw), len(s.Hand))
	}
	act(t, s, r, Action{Kind: "play_map_card", Card: "trailblaze"})
	if s.Turn.Move != 2 {
		t.Fatalf("Trailblaze gives Move 2, got %d", s.Turn.Move)
	}
	// Forced March costs 10 energy and ignores terrain for the turn.
	act(t, s, r, Action{Kind: "play_map_card", Card: "pathfinder_paths_variant_node_1"})
	if s.Energy != 10 || s.Turn.Move != 6 || !s.Turn.IgnoreTerrain {
		t.Fatalf("Forced March: energy %d move %d ignore %v", s.Energy, s.Turn.Move, s.Turn.IgnoreTerrain)
	}
	if cost, _, ok := s.moveCost(r, Hex{1, -1}); !ok || cost != 1 {
		t.Fatalf("forest costs 1 under Forced March, got %d", cost)
	}
	if _, _, ok := s.moveCost(r, Hex{-1, 0}); ok {
		t.Fatal("the lake stays impassable under Forced March")
	}
	s.Energy = 5
	s.Hand = append(s.Hand, "pathfinder_paths_variant_node_1")
	_, err = s.Act(r, Action{Kind: "play_map_card", Card: "pathfinder_paths_variant_node_1"})
	mustFail(t, err, "costs 10 energy; you have 5")
	// Scout Ahead reveals contents up to 3 away this turn.
	s.Night = true
	s.Seen = map[string]bool{}
	s.refreshSight(r)
	placeAll(s, r)
	s.Seen = map[string]bool{}
	s.refreshSight(r)
	before := len(s.Seen)
	act(t, s, r, Action{Kind: "play_map_card", Card: "pathfinder_paths_variant_node_3"})
	after := len(s.Seen)
	if after <= before || s.Turn.ScoutRange != 3 {
		t.Fatalf("Scout Ahead should reveal more hexes: %d → %d", before, after)
	}
	for _, h := range s.placedHexes(r.Pack) {
		if Distance(s.Player, h) <= 3 && !s.Seen[hexKey(h)] {
			t.Fatalf("hex %v within 3 is still unseen after scouting", h)
		}
	}
	// Ordinary cards never become energy on the map: there is no such action.
	s.Energy = 0
	_, err = s.Act(r, Action{Kind: "play_map_card", Card: "strong_swing"})
	mustFail(t, err, "no map effect")
	if s.Energy != 0 {
		t.Fatal("energy must not change")
	}
}

func TestOneCardRuleEndTurnDrawAndRest(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 4)
	_, err := s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "play or discard at least 1 card first")
	if v := s.View(r, 1); v.Turn.CanEndTurn || v.Turn.EndTurnReason == "" {
		t.Fatal("the view must say the turn cannot end yet")
	}
	kept := append([]string{}, s.Hand[1:]...)
	act(t, s, r, Action{Kind: "discard", Cards: s.Hand[:1]})
	act(t, s, r, Action{Kind: "end_turn"})
	if len(s.Hand) != 4 || !reflect.DeepEqual(s.Hand[:3], kept) || s.Turn.Number != 2 || s.Turn.CardsUsed != 0 {
		t.Fatalf("unplayed cards stay and the hand refills to 4: %v (kept %v)", s.Hand, kept)
	}
	// A fight counts as the card.
	s.Turn.Fought = true
	act(t, s, r, Action{Kind: "end_turn"})
	// Run the draw pile dry: discard whole hands until the pile is empty.
	for s.Phase == PhaseTurn {
		act(t, s, r, Action{Kind: "discard", Cards: append([]string{}, s.Hand...)})
		act(t, s, r, Action{Kind: "end_turn"})
	}
	if s.Phase != PhaseCamp || s.Camp == nil || s.Camp.Reason != "deck_empty" {
		t.Fatalf("an empty draw pile at end of turn rests: phase %s camp %+v", s.Phase, s.Camp)
	}
	// Rest step 1: Spent reshuffled into the draw pile; wounds stay out.
	s2 := newState(t, r, 4)
	woundCard(t, s2, "nudge")
	s2.Energy = 3
	s2.Spent = append(s2.Spent, s2.Draw...)
	s2.Draw = []string{}
	act(t, s2, r, Action{Kind: "discard", Cards: s2.Hand[:1]})
	act(t, s2, r, Action{Kind: "end_turn"})
	if s2.Phase != PhaseCamp || len(s2.Spent) != 0 || len(s2.Draw) != 11-len(s2.Hand) || s2.Wounds[0] != "nudge" || s2.Energy != 10 || s2.LiveCards() != 11 {
		t.Fatalf("rest: spent %d draw %d wounds %v energy %d", len(s2.Spent), len(s2.Draw), s2.Wounds, s2.Energy)
	}
	if v := s2.View(r, 1); v.Camp == nil || v.Camp.Reason != "deck_empty" || v.Camp.RollNeeded != 4 || v.Camp.DeckChange != 3 {
		t.Fatalf("camp services are offered: %+v", v.Camp)
	}
	// Energy resets down as well as up.
	s3 := newState(t, r, 4)
	s3.Energy = 40
	act(t, s3, r, Action{Kind: "make_camp"})
	if s3.Energy != 10 || s3.Camp.Reason != "make_camp" || s3.Camp.Forfeited != 4 || len(s3.Hand) != 0 {
		t.Fatalf("make camp forfeits the hand and resets energy: %+v hand %v energy %d", s3.Camp, s3.Hand, s3.Energy)
	}
	// Break camp: Warlord marches 2, Day → Night, round 2, hand drawn.
	from := s3.Warlord.Pos()
	act(t, s3, r, Action{Kind: "break_camp"})
	if s3.Round != 2 || !s3.Night || s3.Phase != PhaseTurn || len(s3.Hand) != 4 || s3.Warlord.Index != 2 || Distance(from, s3.Warlord.Pos()) != 2 {
		t.Fatalf("break camp: round %d night %v hand %d warlord %d→%d", s3.Round, s3.Night, len(s3.Hand), s3.Warlord.Index, s3.Warlord.Index)
	}
	if s3.LastRest == nil || s3.LastRest.WarlordSteps != 2 || !s3.LastRest.NightAfter || s3.LastRest.RoundAfter != 2 {
		t.Fatalf("rest summary: %+v", s3.LastRest)
	}
}

func TestReloadedStateContinuesIdentically(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 21)
	s.Inventory["bandage"] = 2
	woundCard(t, s, "nudge")
	s.Hand = append(s.Hand, s.Draw[0])
	s.Draw = s.Draw[1:]
	steps := []Action{
		{Kind: "move", Target: &Hex{1, 0}, Cards: []string{s.Hand[0]}},
		{Kind: "explore", Slot: "east", Cards: []string{s.Hand[1], s.Hand[2]}},
		{Kind: "discard", Cards: []string{s.Hand[3]}},
		{Kind: "end_turn"},
		{Kind: "make_camp"},
		{Kind: "camp_service", Service: "mend_roll", Item: "bandage"},
		{Kind: "break_camp"},
	}
	twin := reloaded(t, s)
	for i, a := range steps {
		_, err1 := s.Act(r, a)
		_, err2 := twin.Act(r, a)
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("step %d diverged: %v vs %v", i, err1, err2)
		}
		if err1 != nil {
			t.Fatalf("step %d: %v", i, err1)
		}
		j1, _ := json.Marshal(s)
		j2, _ := json.Marshal(twin)
		if string(j1) != string(j2) {
			t.Fatalf("step %d: the reloaded save diverged", i)
		}
		v1, _ := json.Marshal(s.View(r, 1))
		v2, _ := json.Marshal(twin.View(r, 1))
		if string(v1) != string(v2) {
			t.Fatalf("step %d: the views diverged", i)
		}
		twin = reloaded(t, twin)
	}
}

func TestSitesInfluenceMendingShopsAndTrainer(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 9)
	placeAll(s, r)
	village := s.hexOfSite("hollow_ford")
	s.teleport(r, village)
	for _, id := range []string{"nudge", "nudge", "try_again"} {
		woundCard(t, s, id)
	}
	s.drawUpTo(r)
	// Mending at a village costs 2 Influence per wound; Influence is cards played here.
	_, err := s.Act(r, Action{Kind: "use_site", Service: "mend"})
	mustFail(t, err, "costs 2 Influence; you have 0")
	hand := append([]string{}, s.Hand...)
	act(t, s, r, Action{Kind: "use_site", Service: "influence", Cards: hand[:1]})
	if s.Turn.Influence != 1 || s.Turn.InfluenceSite != "hollow_ford" || s.Turn.CardsUsed != 1 {
		t.Fatalf("influence %d at %s", s.Turn.Influence, s.Turn.InfluenceSite)
	}
	act(t, s, r, Action{Kind: "use_site", Service: "mend", Cards: hand[1:2]})
	if len(s.Wounds) != 2 || s.Turn.Influence != 0 || s.LiveCards() != 10 {
		t.Fatalf("one wound mended: %v influence %d live %d", s.Wounds, s.Turn.Influence, s.LiveCards())
	}
	// Shops take gold only.
	_, err = s.Act(r, Action{Kind: "use_site", Service: "shop", Item: "bandage"})
	mustFail(t, err, "Bandage costs 10 gold; you have 0")
	s.Gold = 25
	act(t, s, r, Action{Kind: "use_site", Service: "shop", Item: "bandage"})
	if s.Gold != 15 || s.Inventory["bandage"] != 1 {
		t.Fatalf("bought a bandage: gold %d inventory %v", s.Gold, s.Inventory)
	}
	_, err = s.Act(r, Action{Kind: "use_site", Service: "shop", Item: "lantern"})
	mustFail(t, err, "does not sell that")
	// Influence is bound to the site and lost when the turn ends.
	act(t, s, r, Action{Kind: "use_site", Service: "influence", Cards: hand[2:3]})
	act(t, s, r, Action{Kind: "end_turn"})
	if s.Turn.Influence != 0 {
		t.Fatal("influence is never stored")
	}
	// Monastery: 1 Influence per wound, open at night.
	monastery := s.hexOfSite("stillwater_monastery")
	s.teleport(r, monastery)
	s.Night = true
	s.refreshSight(r)
	act(t, s, r, Action{Kind: "use_site", Service: "mend", Cards: s.Hand[:1]})
	if len(s.Wounds) != 1 {
		t.Fatalf("the monastery mends for 1 Influence at night: %v", s.Wounds)
	}
	// Villages close at night, except the inn (make camp there).
	s.teleport(r, village)
	_, err = s.Act(r, Action{Kind: "use_site", Service: "influence", Cards: s.Hand[:1]})
	mustFail(t, err, "closed at night")
	act(t, s, r, Action{Kind: "make_camp"})
	if s.Camp == nil || !s.Camp.Inn || s.Camp.Site != "hollow_ford" {
		t.Fatalf("camping at a village uses the inn: %+v", s.Camp)
	}
	act(t, s, r, Action{Kind: "break_camp"})
	// Town: healer for gold, trainer 3 gold → 1 XP, gear for sale; no Influence needed for deck changes.
	town := s.hexOfSite("greywharf")
	s.teleport(r, town)
	s.Night = false
	s.Gold = 100
	_, err = s.Act(r, Action{Kind: "use_site", Service: "trainer", Gold: 4})
	mustFail(t, err, "converts 3 gold into 1 XP")
	rewards := act(t, s, r, Action{Kind: "use_site", Service: "trainer", Gold: 9})
	if rewards.XP != 3 || s.Gold != 91 || s.XPEarned != 3 {
		t.Fatalf("trainer: %+v gold %d", rewards, s.Gold)
	}
	act(t, s, r, Action{Kind: "use_site", Service: "mend"})
	if len(s.Wounds) != 0 || s.Gold != 91-r.Config.Mending.TownGoldPerWound {
		t.Fatalf("the healer takes gold: wounds %v gold %d", s.Wounds, s.Gold)
	}
	act(t, s, r, Action{Kind: "use_site", Service: "shop", Item: "lantern"})
	if s.Inventory["lantern"] != 1 {
		t.Fatal("the town sells gear")
	}
	if cost, where, err := s.DeckChangeCost(r); err != nil || cost != 0 || where != "town" {
		t.Fatalf("town deck changes cost 0 Influence: %d %s %v", cost, where, err)
	}
	s.teleport(r, monastery)
	if cost, _, _ := s.DeckChangeCost(r); cost != 1 {
		t.Fatalf("monastery deck changes cost 1, got %d", cost)
	}
	s.teleport(r, village)
	if cost, _, _ := s.DeckChangeCost(r); cost != 2 {
		t.Fatalf("village deck changes cost 2, got %d", cost)
	}
	s.teleport(r, Hex{1, 0})
	if _, _, err := s.DeckChangeCost(r); err == nil {
		t.Fatal("deck changes need a site or camp")
	}
	act(t, s, r, Action{Kind: "make_camp"})
	if cost, where, _ := s.DeckChangeCost(r); cost != 3 || where != "camp" {
		t.Fatalf("camp deck changes cost 3, got %d at %s", cost, where)
	}
	mustFail(t, s.PayDeckChange(r), "costs 3 Influence; you have 0")
	s.Hand = []string{"nudge", "nudge", "try_again"}
	act(t, s, r, Action{Kind: "camp_service", Service: "influence", Cards: s.Hand[:3]})
	must(t, s.PayDeckChange(r))
	if s.Turn.Influence != 0 || s.Camp.DeckChanges != 1 {
		t.Fatal("a camp deck change spends 3 Influence")
	}
}

func TestApplyDeckChangeKeepsPilesInStep(t *testing.T) {
	s := &State{Hand: []string{"steady_guard", "nudge"}, Draw: []string{"steady_guard", "take_stock"}, Spent: []string{"steady_guard", "nudge"}}
	before := map[string]int{"steady_guard": 3, "nudge": 2, "take_stock": 1}
	// Upgrade one Steady Guard to Brace: replaced in the hand first.
	after := map[string]int{"steady_guard": 2, "nudge": 2, "take_stock": 1, "steady_guard_paths_variant_node_6": 1}
	s.ApplyDeckChange(before, after, map[string]string{"steady_guard": "steady_guard_paths_variant_node_6"})
	if s.Hand[0] != "steady_guard_paths_variant_node_6" || s.Draw[0] != "steady_guard" || s.Spent[0] != "steady_guard" {
		t.Fatalf("in-place upgrade: %v / %v / %v", s.Hand, s.Draw, s.Spent)
	}
	// Sell a Nudge: it leaves Spent first; buy a Trailblaze: top of the draw pile.
	before = map[string]int{"steady_guard": 2, "nudge": 2, "take_stock": 1, "steady_guard_paths_variant_node_6": 1}
	after = map[string]int{"steady_guard": 2, "nudge": 1, "take_stock": 1, "steady_guard_paths_variant_node_6": 1, "trailblaze": 1}
	s.ApplyDeckChange(before, after, nil)
	if countOf(s.Spent, "nudge") != 0 || countOf(s.Hand, "nudge") != 1 || s.Draw[0] != "trailblaze" || len(s.Draw) != 3 {
		t.Fatalf("sell/buy: %v / %v / %v", s.Hand, s.Draw, s.Spent)
	}
}

func TestCampMendingRollsUseSeededDice(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 30)
	for _, id := range []string{"nudge", "nudge", "try_again", "take_stock"} {
		woundCard(t, s, id)
	}
	act(t, s, r, Action{Kind: "make_camp"})
	_, err := s.Act(r, Action{Kind: "camp_service", Service: "mend_roll"})
	mustFail(t, err, "costs a bandage or 3 Influence")
	_, err = s.Act(r, Action{Kind: "camp_service", Service: "mend_roll", Item: "bandage"})
	mustFail(t, err, "you have no bandage")
	s.Inventory["bandage"] = 3
	twin := reloaded(t, s)
	results := []MendRoll{}
	for i := 0; i < 3; i++ {
		act(t, s, r, Action{Kind: "camp_service", Service: "mend_roll", Item: "bandage"})
		act(t, twin, r, Action{Kind: "camp_service", Service: "mend_roll", Item: "bandage"})
		results = append(results, s.Camp.MendRolls[i])
	}
	if !reflect.DeepEqual(s.Camp.MendRolls, twin.Camp.MendRolls) {
		t.Fatal("mending rolls come from the seeded RNG")
	}
	for _, roll := range results {
		if roll.Roll < 1 || roll.Roll > 6 || roll.Success != (roll.Roll >= 4) || roll.Paid != "bandage" || roll.Bonus != 0 {
			t.Fatalf("a camp roll succeeds on 4+: %+v", roll)
		}
	}
	mended := 0
	for _, roll := range results {
		if roll.Success {
			mended++
		}
	}
	if len(s.Wounds) != 4-mended || s.Inventory["bandage"] != 0 || s.LiveCards() != 8+mended {
		t.Fatalf("wounds %v, bandages %d, live %d", s.Wounds, s.Inventory["bandage"], s.LiveCards())
	}
	// Three Influence buys a roll too; an inn adds +1.
	s.Camp.Inn = true
	s.Hand = []string{"nudge", "nudge", "try_again"}
	act(t, s, r, Action{Kind: "camp_service", Service: "influence", Cards: s.Hand[:3]})
	if len(s.Wounds) > 0 {
		act(t, s, r, Action{Kind: "camp_service", Service: "mend_roll"})
		last := s.Camp.MendRolls[len(s.Camp.MendRolls)-1]
		if last.Paid != "influence" || last.Bonus != 1 || last.Success != (last.Roll+1 >= 4) || s.Turn.Influence != 0 {
			t.Fatalf("influence roll at an inn: %+v", last)
		}
	}
	// Different seeds give different roll sequences over enough rolls.
	rolls := func(seed uint64) []int {
		x := newState(t, r, seed)
		var out []int
		for i := 0; i < 12; i++ {
			out = append(out, x.d6())
		}
		return out
	}
	if reflect.DeepEqual(rolls(1), rolls(2)) {
		t.Fatal("two seeds rolled the same twelve dice")
	}
}

func TestEventsEnforceRequirementsAndOutcomes(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 12)
	placeAll(s, r)
	// Wayside Shrine: the offering needs 1 Influence (cards played here).
	shrine := s.hexOfSite("shrine_site")
	s.teleport(r, approachFrom(t, s, r, shrine))
	s.Turn.Move = 5
	act(t, s, r, Action{Kind: "move", Target: &shrine})
	if s.Phase != PhaseEvent || s.PendingEvent == nil || s.PendingEvent.Event != "wayside_shrine" {
		t.Fatalf("stepping on an event site opens it: %s %+v", s.Phase, s.PendingEvent)
	}
	_, err := s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "resolve the event first")
	_, err = s.Act(r, Action{Kind: "event_choice", Choice: "offering"})
	mustFail(t, err, "needs 1 Influence")
	woundCard(t, s, "nudge")
	rewards := act(t, s, r, Action{Kind: "event_choice", Choice: "offering", Cards: s.Hand[:1]})
	if rewards.XP != 5 || len(s.Wounds) != 0 || s.Phase != PhaseTurn || !s.site("shrine_site").Resolved || s.LastEvent == nil || s.LastEvent.Choice != "offering" {
		t.Fatalf("offering: %+v wounds %v phase %s", rewards, s.Wounds, s.Phase)
	}
	back := approachFrom(t, s, r, shrine)
	s.Turn.Move = 9
	act(t, s, r, Action{Kind: "move", Target: &back})
	act(t, s, r, Action{Kind: "move", Target: &shrine})
	if s.Phase != PhaseTurn {
		t.Fatal("a resolved event site is just a hex")
	}
	// Taking the silver: gold and a wound.
	s2 := newState(t, r, 12)
	placeAll(s2, r)
	s2.teleport(r, approachFrom(t, s2, r, shrine))
	s2.Turn.Move = 5
	act(t, s2, r, Action{Kind: "move", Target: &shrine})
	live := s2.LiveCards()
	act(t, s2, r, Action{Kind: "event_choice", Choice: "take_silver"})
	if s2.Gold != 15 || len(s2.Wounds) != 1 || s2.LiveCards() != live-1 {
		t.Fatalf("silver: gold %d wounds %v", s2.Gold, s2.Wounds)
	}
	// Wrecked Barge: a roll; the mudlark needs 5 gold and gives a bandage.
	tested := 0
	for seed := uint64(1); seed <= 60 && tested < 20; seed++ {
		x := newState(t, r, seed)
		placeAll(x, r)
		barge := x.hexOfSite("barge_site")
		from, ok := quietApproach(x, r, barge)
		if !ok {
			continue // in this layout the barge sits inside a group's zone
		}
		tested++
		x.teleport(r, from)
		x.Turn.Move = 5
		act(t, x, r, Action{Kind: "move", Target: &barge})
		act(t, x, r, Action{Kind: "event_choice", Choice: "search_hold"})
		res := x.LastEvent
		if res.Roll < 1 || res.Roll > 6 || res.Needed != 4 {
			t.Fatalf("roll choice: %+v", res)
		}
		if res.Roll >= 4 && x.Gold != 25 {
			t.Fatalf("success pays 25 gold: %+v gold %d", res, x.Gold)
		}
		if res.Roll < 4 && (x.PendingBattle == nil || x.PendingBattle.Group != "brine_lone" || x.PendingBattle.Kind != "event") {
			t.Fatalf("failure starts a fight: %+v %+v", res, x.PendingBattle)
		}
	}
	if tested < 10 {
		t.Fatalf("only %d layouts allowed a quiet approach to the barge", tested)
	}
	var x *State
	var barge Hex
	for seed := uint64(1); ; seed++ {
		x = newState(t, r, seed)
		placeAll(x, r)
		barge = x.hexOfSite("barge_site")
		if _, ok := quietApproach(x, r, barge); ok {
			break
		}
	}
	x.teleport(r, approachFrom(t, x, r, barge))
	x.Turn.Move = 5
	act(t, x, r, Action{Kind: "move", Target: &barge})
	_, err = x.Act(r, Action{Kind: "event_choice", Choice: "salvage"})
	mustFail(t, err, "needs 5 gold")
	x.Gold = 5
	act(t, x, r, Action{Kind: "event_choice", Choice: "salvage"})
	if x.Gold != 0 || x.Inventory["bandage"] != 1 {
		t.Fatalf("salvage: gold %d inventory %v", x.Gold, x.Inventory)
	}
	// Hermit's Fire: robbing applies Shaken (hand size −1 until the next rest).
	hermit := x.hexOfSite("hermit_site")
	x.teleport(r, approachFrom(t, x, r, hermit))
	x.Turn.Move = 5
	act(t, x, r, Action{Kind: "move", Target: &hermit})
	act(t, x, r, Action{Kind: "event_choice", Choice: "rob"})
	if x.Gold != 20 || len(x.Statuses) != 1 || x.Statuses[0].ID != "shaken" || x.effectiveHandSize(r) != 3 || x.effectiveHandLimit(r) != 5 {
		t.Fatalf("rob: gold %d statuses %v hand size %d", x.Gold, x.Statuses, x.effectiveHandSize(r))
	}
	if v := x.View(r, 1); len(v.Statuses) != 1 || v.Statuses[0].Name != "Shaken" || !strings.Contains(v.Statuses[0].Rules, "next rest") {
		t.Fatalf("the status shows its rules: %+v", v.Statuses)
	}
	act(t, x, r, Action{Kind: "make_camp"})
	act(t, x, r, Action{Kind: "break_camp"})
	if len(x.Statuses) != 0 || x.LastRest.Statuses[0] != "shaken" {
		t.Fatal("Shaken expires at the rest")
	}
	_, err = x.Act(r, Action{Kind: "event_choice", Choice: "share"})
	mustFail(t, err, "not during turn")
}

func TestPatrolsNightHuntersAndGuards(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 15)
	placeAll(s, r)
	hunter, ok := s.hexOfGroup("eel_hunter")
	if !ok {
		t.Fatal("the Ribbon Eel night hunter is on the map")
	}
	// Give the patrols room: beat every other group.
	for i := range s.Groups {
		if s.Groups[i].Group != "eel_hunter" && s.Groups[i].Group != "bell_marshal" {
			s.Groups[i].Defeated = true
		}
	}
	// By day, night hunters do not roam at end of turn.
	s.teleport(r, Hex{})
	act(t, s, r, Action{Kind: "discard", Cards: s.Hand[:1]})
	act(t, s, r, Action{Kind: "end_turn"})
	if g, _ := s.hexOfGroup("eel_hunter"); g.Pos != hunter.Pos {
		t.Fatal("night hunters rest by day")
	}
	s.Night = true
	act(t, s, r, Action{Kind: "discard", Cards: s.Hand[:1]})
	act(t, s, r, Action{Kind: "end_turn"})
	if g, _ := s.hexOfGroup("eel_hunter"); g.Pos == hunter.Pos || g.Patrol != 1 {
		t.Fatalf("night hunters roam one hex per turn at night: %+v", g)
	}
	// Rests step every patrol (the roaming Lieutenant included).
	marshal, _ := s.hexOfGroup("bell_marshal")
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	if g, _ := s.hexOfGroup("bell_marshal"); g.Pos == marshal.Pos || g.Patrol != 1 {
		t.Fatalf("the roaming Lieutenant steps at rest: %+v", g)
	}
	// The guard: a step that would bring a group within two hexes of another
	// living open group, onto the player or onto the Warlord is skipped.
	s2 := newState(t, r, 15)
	placeAll(s2, r)
	for _, g := range s2.Groups {
		for _, o := range s2.Groups {
			if g.ID != o.ID && Distance(g.Pos, o.Pos) <= 2 {
				t.Fatalf("groups %s and %s start touching", g.ID, o.ID)
			}
		}
	}
	for round := 0; round < 4; round++ {
		s2.Night = true
		act(t, s2, r, Action{Kind: "make_camp"})
		act(t, s2, r, Action{Kind: "break_camp"})
		for _, g := range s2.Groups {
			for _, o := range s2.Groups {
				if g.ID != o.ID && !g.Defeated && !o.Defeated && Distance(g.Pos, o.Pos) <= 2 {
					t.Fatalf("after rest %d groups %s and %s touch", round, g.ID, o.ID)
				}
			}
			if !g.Defeated && s2.warlordAt(g.Pos) {
				t.Fatalf("group %s shares the Warlord's hex", g.ID)
			}
		}
	}
	g := s2.group(marshal.ID)
	def := r.Pack.Groups[g.Group]
	next, _ := hexFromPair(def.Patrol[(g.Patrol+1)%len(def.Patrol)])
	s2.Player = g.Home.Add(next)
	if s2.patrolStep(r, g) {
		t.Fatal("a patrol never steps onto the player")
	}
}

func TestConfigValuesChangeBehaviour(t *testing.T) {
	lib := authoredLibrary(t)
	scratch := scratchContent(t)
	path := filepath.Join(scratch, "progression_v1", "expedition.yaml")
	rewrite(t, path, replaceOnce("rounds_per_region: 4", "rounds_per_region: 2"))
	rewrite(t, path, replaceOnce("hand: {size: 4, limit: 6}", "hand: {size: 3, limit: 5}"))
	rewrite(t, path, replaceOnce("energy_per_move: 5", "energy_per_move: 10"))
	rewrite(t, path, replaceOnce("hills:    {day: 2, night: 2}", "hills:    {day: 4, night: 4}"))
	rewrite(t, path, replaceOnce("defeat: {wounds: 2}", "defeat: {wounds: 1}"))
	rewrite(t, path, replaceOnce("explore_cost: 2", "explore_cost: 1"))
	cfg, err := LoadConfig(scratch, lib)
	must(t, err)
	pack, err := LoadPack(scratch, cfg, lib)
	must(t, err)
	r := Rules{Config: cfg, Pack: pack, Cards: lib.Cards, Statuses: lib.Statuses}
	s := newState(t, r, 1)
	if len(s.Hand) != 3 {
		t.Fatalf("hand size 3 from config, got %d", len(s.Hand))
	}
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{0, -1}, Energy: 5})
	mustFail(t, err, "energy pays for Move 10 at a time")
	_, err = s.Act(r, Action{Kind: "move", Target: &Hex{0, -1}, Energy: 10})
	mustFail(t, err, "costs 4 Move; you have 1")
	act(t, s, r, Action{Kind: "move", Target: &Hex{1, 0}, Cards: s.Hand[:1]})
	act(t, s, r, Action{Kind: "explore", Slot: "east", Cards: s.Hand[:1]})
	if len(s.Placed) != 2 {
		t.Fatal("explore cost 1 from config")
	}
	// Two rounds: the deadline comes after the second rest.
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	act(t, s, r, Action{Kind: "make_camp"})
	act(t, s, r, Action{Kind: "break_camp"})
	if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || s.Round != 3 {
		t.Fatalf("rounds_per_region 2: %+v round %d", s.PendingBattle, s.Round)
	}
	// Defeat wounds 1 from config.
	s2 := newState(t, r, 1)
	s2.Hand = []string{}
	s2.Turn.Move = 3
	placeAll(s2, r)
	g, _ := s2.hexOfGroup("brine_lone")
	s2.teleport(r, g.Pos.Add(Hex{1, 0}))
	act(t, s2, r, Action{Kind: "challenge", Group: g.ID})
	_, _, err = s2.ResolveBattle(r, BattleEnd{BattleID: s2.PendingBattle.ID, Result: "defeat"})
	must(t, err)
	if len(s2.Wounds) != 1 {
		t.Fatalf("defeat.wounds 1 from config, got %d", len(s2.Wounds))
	}
}
