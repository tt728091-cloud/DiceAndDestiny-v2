package expedition

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func statusStacks(list []StatusStack, id string) int {
	for _, st := range list {
		if st.ID == id {
			return st.Stacks
		}
	}
	return 0
}

func TestProvokedFightsStartImmediatelyWithoutAmbush(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	placeAll(s, r)
	g, _ := s.hexOfGroup("brine_lone") // unwary: ambushable day or night
	a := g.Pos.Add(Hex{1, 0})
	s.teleport(r, a)
	s.Turn.StartHex = Hex{-9, -9} // far away: "unseen" by distance
	s.Turn.Move = 5
	// The view warns which steps provoke; take the first such step.
	var b Hex
	found := false
	for _, m := range s.View(r, 1).Moves {
		if m.Provokes != "" && m.Blocked == "" {
			b, found = m.To, true
			break
		}
	}
	if !found || !Adjacent(b, g.Pos) || !containsProvoke(s.View(r, 1).Moves, b) {
		t.Fatalf("the view must warn that a step from %v around the group provokes", a)
	}
	hand := append([]string{}, s.Hand...)
	act(t, s, r, Action{Kind: "move", Target: &b})
	pb := s.PendingBattle
	if pb == nil || pb.Kind != "provoked" || pb.Ambush || pb.Target != g.ID || pb.Origin != a || pb.Position != b || s.Phase != PhaseBattle || !s.Turn.Fought {
		t.Fatalf("provoke: %+v phase %s", pb, s.Phase)
	}
	if pb.Opponent != "drowned_oracle_brine_mask" || pb.Count != 1 || pb.EnergyIn != s.Energy || len(pb.HandBefore) != len(hand) {
		t.Fatalf("pending battle context: %+v", pb)
	}
	_, err := s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "a battle is pending")
	// Stepping away from the group (not to another adjacent hex) is fine.
	s2 := newState(t, r, 6)
	placeAll(s2, r)
	s2.teleport(r, a)
	s2.Turn.Move = 5
	away := a.Add(Hex{1, 0})
	if Adjacent(away, g.Pos) {
		t.Fatal("test geometry: away must not touch the group")
	}
	if _, _, ok := s2.moveCost(r, away); ok {
		act(t, s2, r, Action{Kind: "move", Target: &away})
		if s2.PendingBattle != nil {
			t.Fatal("leaving a group's side does not provoke")
		}
	}
}

func containsProvoke(moves []ViewMove, to Hex) bool {
	for _, m := range moves {
		if m.To == to && m.Provokes != "" {
			return true
		}
	}
	return false
}

func TestChallengeAndAmbushByHabitDayNightAndCover(t *testing.T) {
	r := testRules(t)
	for _, night := range []bool{false, true} {
		for _, habit := range Habits {
			for _, approach := range []string{"seen", "unseen", "cover"} {
				s := newState(t, r, 6)
				placeAll(s, r)
				s.Night = night
				// Use the lone Brine Mask's hex and rewrite its habit for the case.
				g, _ := s.hexOfGroup("brine_lone")
				r2 := r
				groups := map[string]Group{}
				for id, def := range r.Pack.Groups {
					groups[id] = def
				}
				def := groups["brine_lone"]
				def.Habit = habit
				groups["brine_lone"] = def
				pack := r.Pack
				pack.Groups = groups
				r2.Pack = pack
				from := g.Pos.Add(Hex{1, 0})
				s.teleport(r2, from)
				switch approach {
				case "seen":
					s.Turn.StartHex = from // began the turn next to it: seen
				case "unseen":
					s.Turn.StartHex = Hex{-9, -9}
				case "cover":
					s.Turn.StartHex = from
					// Stand on a forest hex next to the group if there is one.
					cover := false
					for _, n := range Neighbors(g.Pos) {
						if _, th, ok := s.tileAt(pack, n); ok && pack.Sites[th.Site].Cover || ok && r.Config.Terrain[th.Terrain].Cover {
							s.teleport(r2, n)
							s.Turn.StartHex = n
							cover = true
							break
						}
					}
					if !cover {
						continue
					}
				}
				act(t, s, r2, Action{Kind: "challenge", Group: g.ID})
				wantUnseen := approach != "seen"
				wantHabit := map[string]bool{"unwary": true, "sleeper": night, "night_hunter": !night, "always_alert": false}[habit]
				want := wantUnseen && wantHabit
				pb := s.PendingBattle
				if pb == nil || pb.Kind != "challenge" || pb.Ambush != want {
					t.Fatalf("habit %s night %v approach %s: ambush %v, want %v", habit, night, approach, pb != nil && pb.Ambush, want)
				}
				if want && statusStacks(pb.PlayerStatuses, "ambush") != r.Config.Ambush.BonusDice {
					t.Fatalf("the Ambush status carries %d stacks: %+v", r.Config.Ambush.BonusDice, pb.PlayerStatuses)
				}
				if !want && statusStacks(pb.PlayerStatuses, "ambush") != 0 {
					t.Fatal("no Ambush status without an ambush")
				}
				if len(pb.EnemyStatuses) != 0 {
					t.Fatalf("an open group is never Fortified: %+v", pb.EnemyStatuses)
				}
			}
		}
	}
	// A challenge needs adjacency and the turn's one action.
	s := newState(t, r, 6)
	placeAll(s, r)
	g, _ := s.hexOfGroup("brine_lone")
	s.teleport(r, g.Pos.Add(Hex{2, 0}))
	_, err := s.Act(r, Action{Kind: "challenge", Group: g.ID})
	mustFail(t, err, "not next to you")
	s.teleport(r, g.Pos.Add(Hex{1, 0}))
	s.Turn.ActionUsed = true
	_, err = s.Act(r, Action{Kind: "challenge", Group: g.ID})
	mustFail(t, err, "already taken your action")
	if len(s.View(r, 1).Challenges) != 0 {
		t.Fatal("no challenge is offered once the action is used")
	}
}

func TestGarrisonsAreFortifiedAndSiegeHooksRemoveIt(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	placeAll(s, r)
	keep := s.hexOfSite("brinehold_keep")
	s.teleport(r, keep.Add(Hex{0, 1}))
	// Walking onto a held site is refused.
	s.Turn.Move = 9
	_, err := s.Act(r, Action{Kind: "move", Target: &keep})
	mustFail(t, err, "defenders must be fought first")
	_, err = s.Act(r, Action{Kind: "enter_site", Site: "brinehold_keep", Item: "siege_hooks"})
	mustFail(t, err, "you have no siege_hooks")
	v := s.View(r, 1)
	if len(v.Enters) != 1 || v.Enters[0].ID != "brinehold_keep" || !v.Enters[0].Fortified || v.Enters[0].Siege {
		t.Fatalf("enter offer: %+v", v.Enters)
	}
	act(t, s, r, Action{Kind: "enter_site", Site: "brinehold_keep"})
	pb := s.PendingBattle
	if pb == nil || pb.Kind != "garrison" || pb.Group != "eel_warden" || statusStacks(pb.EnemyStatuses, "fortified") != 1 || statusStacks(pb.EnemyStatuses, "lieutenant") != 1 || pb.Ambush || s.Player != keep {
		t.Fatalf("keep garrison: %+v player %v", pb, s.Player)
	}
	// With Siege Hooks the garrison is not Fortified and the hooks are spent.
	s2 := newState(t, r, 6)
	placeAll(s2, r)
	s2.teleport(r, keep.Add(Hex{0, 1}))
	s2.Inventory["siege_hooks"] = 1
	if v := s2.View(r, 1); !v.Enters[0].Siege {
		t.Fatal("the view offers the siege hooks")
	}
	act(t, s2, r, Action{Kind: "enter_site", Site: "brinehold_keep", Item: "siege_hooks"})
	if statusStacks(s2.PendingBattle.EnemyStatuses, "fortified") != 0 || s2.Inventory["siege_hooks"] != 0 || !s2.PendingBattle.UsedSiege {
		t.Fatalf("siege hooks: %+v inventory %v", s2.PendingBattle, s2.Inventory)
	}
	// An occupied village is fortified too; ruins guardians are not, and
	// they change with the night.
	s3 := newState(t, r, 6)
	placeAll(s3, r)
	village := s3.hexOfSite("saltreed_village")
	s3.teleport(r, village.Add(Hex{0, -1}))
	act(t, s3, r, Action{Kind: "enter_site", Site: "saltreed_village"})
	if statusStacks(s3.PendingBattle.EnemyStatuses, "fortified") != 1 || s3.PendingBattle.Group != "brine_squat" {
		t.Fatalf("occupied village: %+v", s3.PendingBattle)
	}
	for _, night := range []bool{false, true} {
		s4 := newState(t, r, 6)
		placeAll(s4, r)
		s4.Night = night
		cairn := s4.hexOfSite("drowned_cairn")
		s4.teleport(r, cairn.Add(Hex{0, 1}))
		act(t, s4, r, Action{Kind: "enter_site", Site: "drowned_cairn"})
		pb := s4.PendingBattle
		want := "brine_lone"
		if night {
			want = "brine_pair"
		}
		if pb.Kind != "guardian" || pb.Group != want || statusStacks(pb.EnemyStatuses, "fortified") != 0 {
			t.Fatalf("ruins at night=%v: %+v", night, pb)
		}
	}
}

func TestBattleBridgeVictoryDefeatAndEnergyCap(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	placeAll(s, r)
	g, _ := s.hexOfGroup("brine_lone")
	from := g.Pos.Add(Hex{1, 0})
	s.teleport(r, from)
	s.Energy = 15
	act(t, s, r, Action{Kind: "challenge", Group: g.ID})
	pb := *s.PendingBattle
	// Victory: the piles come back as the battle left them; energy is capped
	// at what came in; the group's XP and gold are paid; the turn ends.
	end := BattleEnd{BattleID: pb.ID, Result: "victory", Hand: []string{"strong_swing"}, Draw: []string{"take_stock", "nudge", "nudge", "try_again", "try_again"}, Spent: []string{"steady_guard", "steady_guard", "steady_guard"}, Removed: []string{"second_wind", "strong_swing", "take_stock"}, Energy: 28}
	rewards, report, err := s.ResolveBattle(r, end)
	must(t, err)
	if rewards.XP != 20 || report.Gold != 8 || s.Gold != 8 || s.Energy != 15 || report.EnergyKept != 15 {
		t.Fatalf("victory rewards: %+v %+v energy %d", rewards, report, s.Energy)
	}
	if len(s.Wounds) != 3 || len(s.Spent) != 3 || s.Phase != PhaseTurn || s.Turn.Number != 3 || s.PendingBattle != nil {
		t.Fatalf("after victory the turn ended and piles returned: wounds %v spent %d phase %s turn %d", s.Wounds, len(s.Spent), s.Phase, s.Turn.Number)
	}
	// The hand was [strong_swing] + 3 drawn at end of turn from the 5-card draw pile.
	if len(s.Hand) != 4 || s.Hand[0] != "strong_swing" || len(s.Draw) != 2 {
		t.Fatalf("end-of-turn draw after the fight: hand %v draw %v", s.Hand, s.Draw)
	}
	if gs := s.group(g.ID); !gs.Defeated {
		t.Fatal("the beaten group leaves the map")
	}
	// Energy cap: 28 at the end, 15 in → 15. A smaller end carries as is.
	s2 := newState(t, r, 6)
	placeAll(s2, r)
	s2.teleport(r, from)
	s2.Energy = 15
	act(t, s2, r, Action{Kind: "challenge", Group: g.ID})
	_, report, err = s2.ResolveBattle(r, BattleEnd{BattleID: s2.PendingBattle.ID, Result: "victory", Hand: s2.Hand, Draw: s2.Draw, Spent: s2.Spent, Energy: 4})
	must(t, err)
	if s2.Energy != 4 || report.EnergyKept != 4 {
		t.Fatalf("a lower end energy carries as is: %d", s2.Energy)
	}
	// A wrong battle id is refused by the save layer; here the state resolves whatever is pending.
	// Defeat: pre-battle piles and wounds plus 2 defeat wounds, pushed back, forced rest.
	s3 := newState(t, r, 6)
	placeAll(s3, r)
	s3.teleport(r, from)
	woundCard(t, s3, "second_wind")
	hand, draw, spent := append([]string{}, s3.Hand...), append([]string{}, s3.Draw...), append([]string{}, s3.Spent...)
	act(t, s3, r, Action{Kind: "challenge", Group: g.ID})
	_, report, err = s3.ResolveBattle(r, BattleEnd{BattleID: s3.PendingBattle.ID, Result: "defeat", Hand: nil, Draw: nil, Spent: nil, Removed: append(hand, draw...), Energy: 0})
	must(t, err)
	if len(s3.Wounds) != 3 || report.DefeatWounds != 2 || s3.Wounds[0] != "second_wind" || s3.LiveCards() != 9 {
		t.Fatalf("defeat wounds: %v live %d", s3.Wounds, s3.LiveCards())
	}
	if s3.Player != from || report.PushedBack == nil || *report.PushedBack != from || s3.Phase != PhaseCamp || s3.Camp.Reason != "defeat" || !report.ForcedRest {
		t.Fatalf("defeat pushes back and rests: player %v phase %s camp %+v", s3.Player, s3.Phase, s3.Camp)
	}
	if gs := s3.group(g.ID); gs.Defeated {
		t.Fatal("the enemy stays after a defeat")
	}
	// Spent cards were reshuffled by the forced rest; the pre-battle piles were restored first.
	if len(s3.Spent) != 0 || len(s3.Hand) != len(hand) || len(s3.Draw) != len(draw)+len(spent)-2 {
		t.Fatalf("forced rest after defeat: hand %d draw %d spent %d", len(s3.Hand), len(s3.Draw), len(s3.Spent))
	}
	// A defeat never takes the last live card.
	s4 := newState(t, r, 6)
	placeAll(s4, r)
	s4.teleport(r, from)
	s4.Hand, s4.Draw, s4.Spent = []string{"strong_swing"}, []string{"nudge"}, []string{}
	s4.Wounds = starterDeck[2:]
	act(t, s4, r, Action{Kind: "challenge", Group: g.ID})
	_, report, err = s4.ResolveBattle(r, BattleEnd{BattleID: s4.PendingBattle.ID, Result: "defeat"})
	must(t, err)
	if s4.LiveCards() != 1 || report.DefeatWounds != 1 {
		t.Fatalf("the last card survives: live %d wounds %d", s4.LiveCards(), report.DefeatWounds)
	}
	// A draw records nothing: the pre-battle piles, wounds and energy come
	// back (even when the battle wiped out every card), the enemy stays, the
	// turn ends.
	s5 := newState(t, r, 6)
	placeAll(s5, r)
	s5.teleport(r, from)
	woundCard(t, s5, "second_wind")
	act(t, s5, r, Action{Kind: "challenge", Group: g.ID})
	before := reloaded(t, s5)
	all := append(append(append([]string{}, s5.Hand...), s5.Draw...), s5.Spent...)
	rewards, report, err = s5.ResolveBattle(r, BattleEnd{BattleID: s5.PendingBattle.ID, Result: "draw", Removed: append(all, s5.Wounds...), Energy: 40})
	must(t, err)
	if rewards.XP != 0 || s5.Gold != 0 || s5.group(g.ID).Defeated || report.Result != "draw" {
		t.Fatal("a draw earns nothing")
	}
	if !reflect.DeepEqual(sortedStrings(s5.Wounds), sortedStrings(before.Wounds)) || s5.LiveCards() != before.LiveCards() || s5.Energy != before.PendingBattle.EnergyIn {
		t.Fatalf("a draw restores the pre-battle piles: wounds %v vs %v, live %d vs %d, energy %d", s5.Wounds, before.Wounds, s5.LiveCards(), before.LiveCards(), s5.Energy)
	}
	if s5.PendingBattle != nil || s5.Turn.Number != before.Turn.Number+1 && s5.Phase != PhaseCamp {
		t.Fatalf("a draw ends the turn: phase %s turn %d", s5.Phase, s5.Turn.Number)
	}
}

func TestRuinsLootLieutenantsAndFreedSites(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	placeAll(s, r)
	cairn := s.hexOfSite("drowned_cairn")
	s.teleport(r, cairn.Add(Hex{0, 1}))
	act(t, s, r, Action{Kind: "enter_site", Site: "drowned_cairn"})
	_, report, err := s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: s.Hand, Draw: s.Draw, Spent: s.Spent, Energy: 10})
	must(t, err)
	if report.Loot < 20 || report.Loot > 30 || s.Gold != 8+report.Loot || !s.site("drowned_cairn").Cleared {
		t.Fatalf("ruins loot by day is 20–30 on top of the guardian's gold: %+v gold %d", report, s.Gold)
	}
	// Cleared ruins are just a hex (with cover).
	if s.siteDefended(r.Pack, s.site("drowned_cairn")) {
		t.Fatal("cleared ruins have no guardian")
	}
	// Beating the keep's Lieutenant removes a Lieutenant's Favor stack.
	keep := s.hexOfSite("brinehold_keep")
	s.teleport(r, keep.Add(Hex{0, 1}))
	if s.livingLieutenants(r.Pack) != 2 {
		t.Fatalf("two Lieutenants live: %d", s.livingLieutenants(r.Pack))
	}
	act(t, s, r, Action{Kind: "enter_site", Site: "brinehold_keep"})
	_, report, err = s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: s.Hand, Draw: s.Draw, Spent: s.Spent, Energy: 10})
	must(t, err)
	if !report.Lieutenant || report.SiteFreed != "Brinehold Keep" || s.livingLieutenants(r.Pack) != 1 {
		t.Fatalf("keep victory removes a Lieutenant: %+v living %d", report, s.livingLieutenants(r.Pack))
	}
	if !s.site("brinehold_keep").Freed {
		t.Fatal("the keep is freed")
	}
	// The Warlord's buff counts living open Lieutenants (the roaming one).
	marshal, _ := s.hexOfGroup("bell_marshal")
	s.teleport(r, marshal.Pos.Add(Hex{1, 0}))
	act(t, s, r, Action{Kind: "challenge", Group: marshal.ID})
	_, _, err = s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: s.Hand, Draw: s.Draw, Spent: s.Spent, Energy: 10})
	must(t, err)
	if s.livingLieutenants(r.Pack) != 0 {
		t.Fatalf("no Lieutenant left: %d", s.livingLieutenants(r.Pack))
	}
}

func TestWarlordMarchesGoalConquerorChallengeAndDeadline(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	placeAll(s, r)
	start := s.Warlord.Pos()
	if Distance(start, s.Warlord.Goal) != 5 {
		t.Fatalf("he starts 5 hexes from his goal: %v → %v", start, s.Warlord.Goal)
	}
	// Three ordinary rests bring him to his goal; extra rests speed him up.
	for i := 1; i <= 3; i++ {
		act(t, s, r, Action{Kind: "make_camp"})
		act(t, s, r, Action{Kind: "break_camp"})
		if i < 3 && (s.Warlord.Index != 2*i || s.Warlord.GoalTaken) {
			t.Fatalf("after rest %d the Warlord is at step %d", i, s.Warlord.Index)
		}
	}
	if !s.Warlord.GoalTaken || s.Warlord.Pos() != s.Warlord.Goal || !s.site("greywharf").Conquered || !s.LastRest.GoalTaken {
		t.Fatalf("at the third rest he takes his goal: %+v", s.Warlord)
	}
	// The conquered town's services are gone.
	s.teleport(r, s.Warlord.Goal.Add(Hex{1, 0}))
	s.Night = false
	before := s.Player
	s.teleport(r, s.Warlord.Goal)
	if _, _, closed := s.siteHere(r); !strings.Contains(closed, "held by the Warlord") {
		t.Fatalf("a conquered site has no services: %q", closed)
	}
	s.teleport(r, before)
	v := s.View(r, 1)
	if !v.Warlord.GoalTaken || !v.Warlord.Conqueror || len(v.Challenges) != 1 || !v.Challenges[0].Warlord || !v.Challenges[0].Fortified {
		t.Fatalf("challenging him at his goal is a fortified fight: %+v %+v", v.Warlord, v.Challenges)
	}
	act(t, s, r, Action{Kind: "challenge", Group: "warlord"})
	pb := s.PendingBattle
	if !pb.Warlord || statusStacks(pb.EnemyStatuses, "fortified") != 1 || statusStacks(pb.EnemyStatuses, "conqueror") != 1 || statusStacks(pb.EnemyStatuses, "lieutenants_favor") != 2 {
		t.Fatalf("Warlord at his goal: %+v", pb.EnemyStatuses)
	}
	// Losing to the Warlord ends the run.
	_, report, err := s.ResolveBattle(r, BattleEnd{BattleID: pb.ID, Result: "defeat", Hand: s.Hand, Draw: s.Draw, Spent: s.Spent})
	must(t, err)
	if !report.RunOver || s.Result != "defeat" || s.Phase != PhaseDefeat {
		t.Fatalf("losing to the Warlord ends the run: %+v", report)
	}
	_, err = s.Act(r, Action{Kind: "end_turn"})
	mustFail(t, err, "the expedition is over")

	// Catch him on the road, unfortified, with an ambush at night (sleeper),
	// and win early: +20 XP per unused round and an extra relic choice.
	s2 := newState(t, r, 6)
	placeAll(s2, r)
	act(t, s2, r, Action{Kind: "make_camp"})
	act(t, s2, r, Action{Kind: "break_camp"}) // round 2, night
	pos := s2.Warlord.Pos()
	s2.teleport(r, pos.Add(Hex{1, 0}))
	s2.Turn.StartHex = Hex{-9, -9}
	v = s2.View(r, 1)
	var road *ViewTarget
	for i := range v.Challenges {
		if v.Challenges[i].Warlord {
			road = &v.Challenges[i]
		}
	}
	if road == nil || !road.Ambush || road.Fortified {
		t.Fatalf("the Warlord on the road at night: %+v", v.Challenges)
	}
	act(t, s2, r, Action{Kind: "challenge", Group: "warlord"})
	pb = s2.PendingBattle
	if !pb.Ambush || statusStacks(pb.EnemyStatuses, "fortified") != 0 || statusStacks(pb.EnemyStatuses, "lieutenants_favor") != 2 {
		t.Fatalf("road fight: %+v", pb)
	}
	s2.Wounds = []string{"nudge"}
	rewards, report, err := s2.ResolveBattle(r, BattleEnd{BattleID: pb.ID, Result: "victory", Hand: s2.Hand, Draw: s2.Draw, Spent: s2.Spent, Removed: []string{"nudge"}, Energy: 3})
	must(t, err)
	if rewards.XP != 100+2*20 || report.EarlyXP != 40 || report.RelicChoices != 1 || s2.PendingRelicChoices != 1 || s2.Gold != 60 || !report.RegionWon || s2.Result != "victory" || len(s2.Wounds) != 0 || report.WoundsMended != 1 {
		t.Fatalf("early victory in round 2: %+v %+v", rewards, report)
	}
	// The deadline: after round 4's rest he comes to the player, no Ambush, not Fortified.
	s3 := newState(t, r, 6)
	placeAll(s3, r)
	for i := 0; i < 4; i++ {
		act(t, s3, r, Action{Kind: "make_camp"})
		act(t, s3, r, Action{Kind: "break_camp"})
	}
	pb = s3.PendingBattle
	if pb == nil || pb.Kind != "deadline" || !pb.Warlord || pb.Ambush || statusStacks(pb.EnemyStatuses, "fortified") != 0 || !s3.Warlord.Arrived || s3.Warlord.Pos() != s3.Player || len(s3.Hand) != 4 {
		t.Fatalf("deadline fight: %+v arrived %v hand %d", pb, s3.Warlord.Arrived, len(s3.Hand))
	}
	if statusStacks(pb.EnemyStatuses, "conqueror") != 1 {
		t.Fatal("he reached his goal before the deadline, so he is a Conqueror")
	}
	rewards, report, err = s3.ResolveBattle(r, BattleEnd{BattleID: pb.ID, Result: "victory", Hand: s3.Hand, Draw: s3.Draw, Spent: s3.Spent})
	must(t, err)
	if rewards.XP != 100 || report.EarlyXP != 0 || report.RelicChoices != 0 {
		t.Fatalf("no early bonus at the deadline: %+v", report)
	}
	raw, _ := json.Marshal(s3.View(r, 1))
	if !strings.Contains(string(raw), `"result":"victory"`) {
		t.Fatal("the view reports the region as won")
	}
}

// A draw in the forced deadline fight restores the pre-battle piles and the
// Warlord attacks again at once; the run neither ends nor is won.
func TestDeadlineDrawRepeatsTheFight(t *testing.T) {
	r := testRules(t)
	s := newState(t, r, 6)
	for s.PendingBattle == nil {
		act(t, s, r, Action{Kind: "make_camp"})
		act(t, s, r, Action{Kind: "break_camp"})
	}
	first := s.PendingBattle
	if first.Kind != "deadline" || !first.Warlord {
		t.Fatalf("the deadline fight: %+v", first)
	}
	before := reloaded(t, s)
	all := append(append(append([]string{}, s.Hand...), s.Draw...), s.Spent...)
	_, report, err := s.ResolveBattle(r, BattleEnd{BattleID: first.ID, Result: "draw", Removed: all, Energy: 30})
	must(t, err)
	if report.Result != "draw" || report.RunOver || report.RegionWon || s.Result != "" {
		t.Fatalf("a drawn deadline fight neither wins nor ends the run: %+v", report)
	}
	if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || s.PendingBattle.ID == first.ID || s.Phase != PhaseBattle {
		t.Fatalf("the Warlord attacks again: %+v", s.PendingBattle)
	}
	if s.LiveCards() != before.LiveCards() || len(s.Wounds) != len(before.Wounds) || s.PendingBattle.EnergyIn != before.PendingBattle.EnergyIn {
		t.Fatalf("the second fight starts from the pre-battle piles: live %d/%d wounds %d/%d energy %d/%d", s.LiveCards(), before.LiveCards(), len(s.Wounds), len(before.Wounds), s.PendingBattle.EnergyIn, before.PendingBattle.EnergyIn)
	}
	_, report, err = s.ResolveBattle(r, BattleEnd{BattleID: s.PendingBattle.ID, Result: "victory", Hand: s.Hand, Draw: s.Draw, Spent: s.Spent, Energy: 5})
	must(t, err)
	if !report.RegionWon || s.Result != "victory" {
		t.Fatalf("winning the repeated fight wins the region: %+v", report)
	}
}
