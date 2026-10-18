package expedition

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

// withMapCards adds map-only test cards (blank in battle) to the rules.
func withMapCards(r Rules, energy int, effects map[string]content.CardMapEffect) Rules {
	r.Cards = maps.Clone(r.Cards)
	for id, e := range effects {
		effect := e
		if effect.Text == "" {
			effect.Text = "Test."
		}
		r.Cards[id] = content.PrepareMapCard(content.BattleCardDefinition{SchemaVersion: 1, ID: id, Name: id, Type: "map", Cost: content.BattleCost{Energy: energy}, MapEffect: &effect,
			Play: content.CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "discard"}, Targeting: content.TargetingDefinition{Selector: "self", Maximum: 1}})
	}
	return r
}

func TestMapEffectValidation(t *testing.T) {
	for _, e := range []content.CardMapEffect{
		{Kind: "move", Amount: 4, ChooseTerrain: true, CrossCost: 2, CarryOver: true, ExploreDiscount: 1, ExploreReach: 4, Sneak: true, Text: "x"},
		{Kind: "sight", Amount: 1, RevealTiles: true, Text: "x"},
		{Kind: "look", Amount: 5, Draw: 1, Bottom: true, Text: "x"},
		{Kind: "sneak", Text: "x"}, {Kind: "fall_back", Text: "x"}, {Kind: "lure", Text: "x"},
		{Kind: "high_ground", Text: "x"}, {Kind: "high_ground", Amount: 2, Text: "x"},
		{Kind: "mark", Amount: 3, Text: "x"}, {Kind: "extend_time", Amount: 1, Text: "x"},
		{Kind: "influence", Amount: 2, Text: "x"}, {Kind: "haggle", Amount: 1, Text: "x"},
		{Kind: "field_medic", Amount: 1, Text: "x"}, {Kind: "spare_bandage", Amount: 1, Text: "x"}, {Kind: "hide", Amount: 1, Text: "x"},
	} {
		if err := content.ValidateCardMapEffect(&e); err != nil {
			t.Fatalf("%+v: %v", e, err)
		}
		if content.CardMapEffectFace(&e) == "" || !strings.HasPrefix(content.CardMapEffectRules(&e), "Map: ") {
			t.Fatalf("%s needs face and rules text", e.Kind)
		}
	}
	for _, c := range []struct {
		e    content.CardMapEffect
		want string
	}{
		{content.CardMapEffect{Kind: "sneak", Amount: 1, Text: "x"}, "has no amount"},
		{content.CardMapEffect{Kind: "sight", Amount: 1, Sneak: true, Text: "x"}, "belong to move"},
		{content.CardMapEffect{Kind: "move", Amount: 1, RevealTiles: true, Text: "x"}, "reveal_tiles"},
		{content.CardMapEffect{Kind: "look", Amount: 3, Draw: 2, Text: "x"}, "draws 0 or 1"},
		{content.CardMapEffect{Kind: "move", Amount: 3, ExploreReach: 1, Text: "x"}, "normal reach"},
	} {
		if err := content.ValidateCardMapEffect(&c.e); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%+v: want %q, got %v", c.e, c.want, err)
		}
	}
}

func TestPathfinderMoveOptions(t *testing.T) {
	r := withMapCards(testRules(t), 5, map[string]content.CardMapEffect{
		"know":  {Kind: "move", Amount: 4, ChooseTerrain: true},
		"ford":  {Kind: "move", Amount: 4, CrossCost: 2},
		"pace":  {Kind: "move", Amount: 4, CarryOver: true},
		"sense": {Kind: "move", Amount: 4, ExploreDiscount: 1},
		"far":   {Kind: "move", Amount: 4, ExploreReach: 4},
	})
	// Know the Land: the terrain is a required choice; it then costs 1.
	s := newState(t, r, 2)
	s.Hand = []string{"know", "ford", "pace"}
	v := s.View(r, 1)
	if v.Hand[0].MapKind != "move" || len(v.Hand[0].Targets) != 4 || v.Hand[0].Targets[0].ID != "forest" {
		t.Fatalf("Know the Land offers the passable terrains: %+v", v.Hand[0])
	}
	_, err := s.Act(r, Action{Kind: "play_map_card", Card: "know"})
	mustFail(t, err, "choose one of its targets")
	_, err = s.Act(r, Action{Kind: "play_map_card", Card: "know", Choice: "lake"})
	mustFail(t, err, "choose one of its targets")
	act(t, s, r, Action{Kind: "play_map_card", Card: "know", Choice: "forest"})
	if s.Energy != 5 || s.Turn.Move != 4 {
		t.Fatalf("5 energy for Move 4: energy %d move %d", s.Energy, s.Turn.Move)
	}
	if cost, _, ok := s.moveCost(r, Hex{1, -1}); !ok || cost != 1 {
		t.Fatalf("the chosen forest costs 1, got %d", cost)
	}
	if !strings.Contains(strings.Join(s.View(r, 1).Turn.Effects, "|"), "forest costs 1 Move") {
		t.Fatalf("the HUD names the turn's effects: %v", s.View(r, 1).Turn.Effects)
	}
	// Ford the Way: one lake hex for 2 Move; ending the turn there goes back.
	lake := Hex{-1, 0}
	if _, _, ok := s.moveCost(r, lake); ok {
		t.Fatal("the lake is impassable without Ford the Way")
	}
	act(t, s, r, Action{Kind: "play_map_card", Card: "ford"})
	if cost, _, ok := s.moveCost(r, lake); !ok || cost != 2 {
		t.Fatalf("Ford the Way: the lake costs 2, got %d %v", cost, ok)
	}
	start := s.Player
	act(t, s, r, Action{Kind: "move", Target: &lake})
	if s.Player != lake || s.Turn.Move != 6 {
		t.Fatalf("on the lake with 6 Move left: %v %d", s.Player, s.Turn.Move)
	}
	for _, n := range Neighbors(lake) {
		if _, th, ok := s.tileAt(r.Pack, n); ok && r.Config.Terrain[th.Terrain].Impassable {
			if _, _, ok := s.moveCost(r, n); ok {
				t.Fatal("only one lake or mountain hex per turn")
			}
		}
	}
	act(t, s, r, Action{Kind: "end_turn"})
	if s.Player != start {
		t.Fatalf("a turn ending on the lake goes back to %v, not %v", start, s.Player)
	}
	// Steady Pace: leftover Move is a visible status the next turn spends.
	s.Hand, s.Energy = append(s.Hand, "pace"), 10
	act(t, s, r, Action{Kind: "play_map_card", Card: "pace"})
	act(t, s, r, Action{Kind: "end_turn"})
	if s.statusStacks("carried_move") != 0 || s.Turn.Move != 4 {
		t.Fatalf("the carried 4 Move joins the new turn: status %d move %d", s.statusStacks("carried_move"), s.Turn.Move)
	}
	// It shows as a status between turns and never enters a battle.
	s.addStatus("carried_move", 2)
	if len(s.battleStatuses(r)) != 0 {
		t.Fatal("map-only statuses never enter a battle")
	}
	// Trail Sense and Far Explore.
	s2 := newState(t, r, 2)
	s2.Hand = []string{"sense", "far"}
	before := len(s2.exploreOptions(r))
	act(t, s2, r, Action{Kind: "play_map_card", Card: "sense"})
	if s2.exploreCost(r) != r.Config.Move.ExploreCost-1 {
		t.Fatalf("Trail Sense: exploring costs %d", s2.exploreCost(r))
	}
	act(t, s2, r, Action{Kind: "play_map_card", Card: "far"})
	if after := len(s2.exploreOptions(r)); after <= before {
		t.Fatalf("Far Explore reaches more empty slots: %d → %d", before, after)
	}
	for _, o := range s2.exploreOptions(r) {
		touches := false
		for _, h := range TileHexes(o.Center) {
			for _, n := range Neighbors(h) {
				if _, _, placed := s2.tileAt(r.Pack, n); placed {
					touches = true
				}
			}
		}
		if !touches {
			t.Fatalf("slot %s does not touch a placed tile", o.Slot)
		}
	}
}

func TestUnseenCards(t *testing.T) {
	r := withMapCards(testRules(t), 0, map[string]content.CardMapEffect{
		"sneak": {Kind: "sneak"}, "fall_back": {Kind: "fall_back"}, "hide": {Kind: "hide", Amount: 1},
		"high": {Kind: "high_ground"}, "sharper": {Kind: "high_ground", Amount: 2},
	})
	slip := withMapCards(r, 8, map[string]content.CardMapEffect{"slip": {Kind: "move", Amount: 4, Sneak: true}})
	setup := func(rules Rules) (*State, GroupState, Hex, Hex) {
		s := newState(t, rules, 6)
		placeAll(s, rules)
		g, _ := s.hexOfGroup("brine_lone")
		a := g.Pos.Add(Hex{1, 0})
		s.teleport(rules, a)
		s.Turn.Move = 5
		for _, m := range s.View(rules, 1).Moves {
			if m.Provokes != "" && m.Blocked == "" {
				return s, g, a, m.To
			}
		}
		t.Fatal("no provoking step")
		return nil, g, a, a
	}
	// Sneak: no provoked fight this turn.
	s, _, _, b := setup(r)
	s.Hand = append(s.Hand, "sneak")
	act(t, s, r, Action{Kind: "play_map_card", Card: "sneak"})
	if containsProvoke(s.View(r, 1).Moves, b) {
		t.Fatal("the view no longer warns of a provoke")
	}
	act(t, s, r, Action{Kind: "move", Target: &b})
	if s.PendingBattle != nil {
		t.Fatal("Sneak: walking past the group provokes nothing")
	}
	// Slip Away: Move 4 for 8 energy and the same sneak.
	s, _, _, b = setup(slip)
	s.Hand, s.Energy = append(s.Hand, "slip"), 10
	act(t, s, slip, Action{Kind: "play_map_card", Card: "slip"})
	act(t, s, slip, Action{Kind: "move", Target: &b})
	if s.PendingBattle != nil || s.Energy != 2 || !s.Turn.Sneak {
		t.Fatalf("Slip Away: battle %v energy %d", s.PendingBattle != nil, s.Energy)
	}
	// Fall back: only against a pending provoked fight; it undoes the fight.
	s, _, a, b := setup(r)
	s.Hand = append(s.Hand, "fall_back")
	if v := s.View(r, 1); v.Hand[len(v.Hand)-1].MapPlayable {
		t.Fatal("Fall back waits for a provoked fight")
	}
	act(t, s, r, Action{Kind: "move", Target: &b})
	if s.PendingBattle == nil || !s.Turn.Fought {
		t.Fatal("the step provokes")
	}
	_, err := s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "a battle is pending")
	act(t, s, r, Action{Kind: "play_map_card", Card: "fall_back"})
	if s.PendingBattle != nil || s.Phase != PhaseTurn || s.Player != a || s.Turn.Fought || s.Turn.ActionUsed || countOf(s.Spent, "fall_back") != 1 {
		t.Fatalf("Fall back: battle %v phase %s at %v fought %v", s.PendingBattle != nil, s.Phase, s.Player, s.Turn.Fought)
	}
	// High Ground: an Ambush even when seen; Sharper Ambush adds 2 dice.
	for _, card := range []string{"high", "sharper"} {
		s, g, a, _ := setup(r)
		s.Turn.StartHex = a // began the turn next to it: seen
		s.Hand = append(s.Hand, card)
		act(t, s, r, Action{Kind: "play_map_card", Card: card})
		act(t, s, r, Action{Kind: "challenge", Group: g.ID})
		want := r.Config.Ambush.BonusDice + map[string]int{"high": 0, "sharper": 2}[card]
		if pb := s.PendingBattle; pb == nil || !pb.Ambush || statusStacks(pb.PlayerStatuses, "ambush") != want {
			t.Fatalf("%s: ambush %+v, want %d dice", card, pb, want)
		}
	}
	// Hide: the next turn starts unseen at a distance enemies would see.
	s, g, _, _ := setup(r)
	near := g.Pos.Add(Hex{2, 0})
	s.teleport(r, near)
	if s.ambushAllowed(r, "challenge", "unwary", g.Pos) {
		t.Fatal("two hexes away by day is seen")
	}
	s.Hand = append(s.Hand, "hide")
	act(t, s, r, Action{Kind: "play_map_card", Card: "hide"})
	act(t, s, r, Action{Kind: "end_turn"})
	if s.statusStacks("hidden") != 1 || !s.ambushAllowed(r, "challenge", "unwary", g.Pos) {
		t.Fatal("Hidden: enemies see 1 hex less, so the turn began unseen")
	}
	s.Turn.CardsUsed = 1
	act(t, s, r, Action{Kind: "end_turn"})
	if s.statusStacks("hidden") != 0 {
		t.Fatal("Hidden ends after the next turn")
	}
}

func TestLookoutCards(t *testing.T) {
	r := withMapCards(testRules(t), 0, map[string]content.CardMapEffect{
		"read": {Kind: "sight", Amount: 1, RevealTiles: true}, "mark": {Kind: "mark", Amount: 3},
		"extend": {Kind: "extend_time", Amount: 1}, "lure": {Kind: "lure"},
	})
	s := newState(t, r, 6)
	placeAll(s, r)
	s.Seen = map[string]bool{}
	for i := range s.Groups {
		s.Groups[i].Seen, s.Groups[i].LastSeen = false, nil
	}
	s.refreshSight(r)
	// Hunter's Mark: any Lieutenant, seen or not; the Warlord only in sight.
	s.Hand = []string{"mark", "extend", "read"}
	targets := s.View(r, 1).Hand[0].Targets
	warlord, keep := false, false
	for _, target := range targets {
		warlord = warlord || target.ID == "warlord"
		keep = keep || target.ID == "site:brinehold_keep"
	}
	if warlord || !keep {
		t.Fatalf("mark targets: %+v", targets)
	}
	if v := s.View(r, 1); v.Hand[1].MapPlayable {
		t.Fatal("gaining time needs a mark first")
	}
	act(t, s, r, Action{Kind: "play_map_card", Card: "mark", Group: "site:brinehold_keep"})
	v := s.View(r, 1)
	if len(v.Marks) != 1 || v.Marks[0].RestsLeft != 3 || v.Marks[0].Hex == nil || *v.Marks[0].Hex != s.hexOfSite("brinehold_keep") || !s.site("brinehold_keep").Seen {
		t.Fatalf("the marked Lieutenant's site is shown: %+v", v.Marks)
	}
	// Run Him Down: +1 round, once per quest.
	rounds := s.rounds(r)
	act(t, s, r, Action{Kind: "play_map_card", Card: "extend"})
	if s.rounds(r) != rounds+1 || s.View(r, 1).Rounds != rounds+1 {
		t.Fatalf("one more round: %d → %d", rounds, s.rounds(r))
	}
	s.Hand = append(s.Hand, "extend")
	_, err := s.Act(r, Action{Kind: "play_map_card", Card: "extend"})
	mustFail(t, err, "already gained time this quest")
	// The mark fades after three rests.
	for i := 0; i < 3; i++ {
		must(t, s.makeCamp(r))
		must(t, s.breakCamp(r))
	}
	if len(s.Marks) != 0 {
		t.Fatalf("the mark lasts three rests: %+v", s.Marks)
	}
	// Marking the Warlord in sight shows him even off the map.
	s2 := newState(t, r, 6)
	s2.Hand = []string{"mark"}
	s2.teleport(r, s2.Warlord.Pos().Add(Hex{1, 0}))
	if _, _, placed := s2.tileAt(r.Pack, s2.Warlord.Pos()); placed {
		act(t, s2, r, Action{Kind: "play_map_card", Card: "mark", Group: "warlord"})
		s2.teleport(r, Hex{})
		if w := s2.View(r, 1).Warlord; !w.Marked || w.Position == nil || *w.Position != s2.Warlord.Pos() {
			t.Fatalf("a marked Warlord's position is shown: %+v", w)
		}
	}
	// Read the Enemy: enemies on tiles in sight are revealed.
	seen := 0
	for _, g := range s.Groups {
		if g.Seen {
			seen++
		}
	}
	s.teleport(r, Hex{})
	s.Hand = []string{"read"}
	act(t, s, r, Action{Kind: "play_map_card", Card: "read"})
	after := 0
	for _, g := range s.Groups {
		if g.Seen {
			after++
		}
	}
	if after < seen || !s.Turn.RevealTiles {
		t.Fatalf("Read the Enemy reveals groups: %d → %d", seen, after)
	}
	// Lure: the keep's defenders fight in the open, not Fortified.
	s3 := newState(t, r, 6)
	placeAll(s3, r)
	keepHex := s3.hexOfSite("brinehold_keep")
	s3.teleport(r, keepHex.Add(Hex{0, 1}))
	s3.Hand = []string{"lure"}
	act(t, s3, r, Action{Kind: "play_map_card", Card: "lure", Site: "brinehold_keep"})
	if pb := s3.PendingBattle; pb == nil || pb.Kind != "garrison" || statusStacks(pb.EnemyStatuses, "fortified") != 0 || !s3.Turn.ActionUsed {
		t.Fatalf("lured defenders are not Fortified: %+v", pb)
	}
}

func TestWayfarerCards(t *testing.T) {
	r := withMapCards(testRules(t), 0, map[string]content.CardMapEffect{
		"tongue": {Kind: "influence", Amount: 2}, "haggle": {Kind: "haggle", Amount: 1},
		"medic": {Kind: "field_medic", Amount: 1}, "spare": {Kind: "spare_bandage", Amount: 1},
	})
	s := newState(t, r, 6)
	placeAll(s, r)
	var village *SiteState
	for i := range s.Sites {
		if r.Pack.Sites[s.Sites[i].ID].Type == "village" && !s.siteDefended(r.Pack, &s.Sites[i]) {
			village = &s.Sites[i]
			break
		}
	}
	if village == nil {
		t.Skip("no open village in this region")
	}
	s.teleport(r, village.Pos)
	s.Hand = []string{"tongue", "haggle", "medic", "spare", "nudge"}
	if v := s.View(r, 1); v.Hand[2].MapPlayable {
		t.Fatal("Field Medic waits for camp")
	}
	act(t, s, r, Action{Kind: "play_map_card", Card: "tongue"})
	if s.Turn.Influence != 2 || s.Turn.InfluenceSite != village.ID {
		t.Fatalf("Silver Tongue counts as 2 Influence here: %d at %s", s.Turn.Influence, s.Turn.InfluenceSite)
	}
	// Energy buys Influence at the configured rate.
	s.Energy = 10
	act(t, s, r, Action{Kind: "use_site", Service: "influence", Energy: 5})
	if s.Turn.Influence != 3 || s.Energy != 5 {
		t.Fatalf("5 energy is 1 Influence: influence %d energy %d", s.Turn.Influence, s.Energy)
	}
	// Haggle: each payment here costs 1 less, never below 1.
	per := r.Config.Mending.VillageInfluencePerWound
	cost, _, err := s.DeckChangeCost(r)
	must(t, err)
	act(t, s, r, Action{Kind: "play_map_card", Card: "haggle"})
	after, _, _ := s.DeckChangeCost(r)
	if after != max(1, cost-1) || s.influencePrice(r, per) != max(1, per-1) || s.influencePrice(r, 1) != 1 {
		t.Fatalf("Haggle: deck change %d → %d, mending %d", cost, after, s.influencePrice(r, per))
	}
	// Spare Bandage: a temporary bandage used at camp, gone when camp breaks.
	act(t, s, r, Action{Kind: "play_map_card", Card: "spare"})
	if s.statusStacks("spare_bandage") != 1 || len(s.battleStatuses(r)) != 0 {
		t.Fatal("a spare bandage is a map-only status")
	}
	woundCard(t, s, "nudge")
	woundCard(t, s, "steady_guard")
	s.Hand = append(s.Hand, "medic", "spare")
	s.Inventory = map[string]int{}
	must(t, s.makeCamp(r))
	s.Hand = append(s.Hand, "medic", "spare")
	act(t, s, r, Action{Kind: "play_map_card", Card: "medic"})
	if s.Camp.AidBonus != 1 {
		t.Fatal("Field Medic adds 1 to basic aid this camp")
	}
	act(t, s, r, Action{Kind: "camp_service", Service: "basic_aid", Wound: "all"})
	for _, h := range s.Camp.Heals {
		if h.Bonus < 1 {
			t.Fatalf("basic aid rolls with the bonus: %+v", h)
		}
	}
	if s.bandages(r) != 1 {
		t.Fatalf("the spare bandage counts at camp: %d", s.bandages(r))
	}
	if len(s.Wounds) > 0 {
		act(t, s, r, Action{Kind: "camp_service", Service: "bandage", Wound: s.Wounds[0].ID})
		if s.statusStacks("spare_bandage") != 0 {
			t.Fatal("the spare bandage is used up")
		}
	}
	act(t, s, r, Action{Kind: "play_map_card", Card: "spare"})
	must(t, s.breakCamp(r))
	if s.statusStacks("spare_bandage") != 0 {
		t.Fatal("unused spare bandages are gone when camp breaks")
	}
}

func TestMapLookArrangesTheDrawPile(t *testing.T) {
	r := withMapCards(testRules(t), 0, map[string]content.CardMapEffect{
		"ahead": {Kind: "look", Amount: 3, Draw: 1}, "clear": {Kind: "look", Amount: 3, Draw: 1, Bottom: true}, "road": {Kind: "look", Amount: 3},
	})
	s := newState(t, r, 2)
	s.Hand = []string{"ahead", "clear", "road"}
	s.Draw = []string{"nudge", "strong_swing", "try_again", "take_stock", "steady_guard"}
	act(t, s, r, Action{Kind: "play_map_card", Card: "ahead"})
	v := s.View(r, 1)
	if v.Look == nil || len(v.Look.Cards) != 3 || v.Look.Cards[0].ID != "nudge" || v.Turn.CanEndTurn {
		t.Fatalf("the look shows the top 3 and waits: %+v", v.Look)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), `"draw":["nudge","strong_swing"`) {
		t.Fatal("the draw pile's order stays hidden")
	}
	_, err := s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "finish arranging")
	_, err = s.Act(r, Action{Kind: "resolve_look", Card: "take_stock"})
	mustFail(t, err, "not one of the cards")
	act(t, s, r, Action{Kind: "resolve_look", Card: "strong_swing", Cards: []string{"try_again"}})
	if s.PendingLook != nil || countOf(s.Hand, "strong_swing") != 1 || strings.Join(s.Draw, ",") != "try_again,nudge,take_stock,steady_guard" {
		t.Fatalf("drew one and reordered: hand %v draw %v", s.Hand, s.Draw)
	}
	// Bottom only where the card allows it.
	act(t, s, r, Action{Kind: "play_map_card", Card: "road"})
	_, err = s.Act(r, Action{Kind: "resolve_look", Bottom: []string{"nudge"}})
	mustFail(t, err, "cannot put cards on the bottom")
	act(t, s, r, Action{Kind: "resolve_look", Cards: []string{"take_stock", "nudge", "try_again"}})
	act(t, s, r, Action{Kind: "play_map_card", Card: "clear"})
	act(t, s, r, Action{Kind: "resolve_look", Card: "nudge", Bottom: []string{"take_stock"}})
	if strings.Join(s.Draw, ",") != "try_again,steady_guard,take_stock" {
		t.Fatalf("one drawn, one to the bottom: %v", s.Draw)
	}
}
