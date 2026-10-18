package expedition

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/content"
)

// Map cards: what each map effect does, when it can be played and what it
// targets. Every number comes from the card's map effect or the config.

// MapTarget is one thing a map card can be played on: a terrain to make
// cheap, an enemy to mark or a site to lure defenders from.
type MapTarget struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// rounds is how many rounds this quest has: the region's plus any added.
func (s *State) rounds(r Rules) int { return r.Config.RoundsPerRegion + s.ExtraRounds }

// mapCardReady reports whether a map effect can be played now.
func (s *State) mapCardReady(r Rules, e *content.CardMapEffect) (bool, string) {
	if s.PendingLook != nil {
		return false, "finish arranging the cards you are looking at first"
	}
	if s.Result != "" {
		return false, "the expedition is over"
	}
	switch e.Kind {
	case "fall_back":
		if s.PendingBattle == nil || s.PendingBattle.Kind != "provoked" {
			return false, "play it when you provoke a fight"
		}
		return true, ""
	case "influence", "haggle":
		if s.Phase == PhaseCamp {
			return true, ""
		}
		if s.Phase != PhaseTurn {
			return false, "not during " + s.Phase
		}
		if _, _, closed := s.siteHere(r); closed != "" {
			return false, "play it at an open site or at camp"
		}
		return true, ""
	case "field_medic":
		if s.Phase != PhaseCamp {
			return false, "play it at camp"
		}
		return true, ""
	case "spare_bandage":
		if s.Phase != PhaseTurn && s.Phase != PhaseCamp {
			return false, "not during " + s.Phase
		}
		return true, ""
	}
	if s.Phase != PhaseTurn {
		return false, "not during " + s.Phase
	}
	switch e.Kind {
	case "look":
		if len(s.Draw) == 0 {
			return false, "your draw pile is empty"
		}
	case "lure":
		if s.Turn.ActionUsed {
			return false, "you have already taken your action this turn"
		}
		if len(s.mapCardTargets(r, e)) == 0 {
			return false, "no keep, tower or occupied site with defenders is next to you"
		}
	case "mark":
		if len(s.mapCardTargets(r, e)) == 0 {
			return false, "no Lieutenant lives and the Warlord is not in sight"
		}
	case "extend_time":
		if s.TimeExtended {
			return false, "you have already gained time this quest"
		}
		if len(s.Marks) == 0 {
			return false, "mark an enemy first"
		}
	}
	return true, ""
}

// mapCardTargets lists what the effect must be played on (nil when it needs
// no target).
func (s *State) mapCardTargets(r Rules, e *content.CardMapEffect) []MapTarget {
	pack := r.Pack
	out := []MapTarget{}
	switch {
	case e.Kind == "move" && e.ChooseTerrain:
		for _, name := range sortedKeys(r.Config.Terrain) {
			if t := r.Config.Terrain[name]; !t.Impassable {
				out = append(out, MapTarget{ID: name, Label: strings.ToUpper(name[:1]) + name[1:]})
			}
		}
	case e.Kind == "mark":
		if !s.Warlord.Defeated && s.warlordVisible(r) {
			out = append(out, MapTarget{ID: "warlord", Label: pack.Groups[s.Warlord.Group].Name + " (the Warlord)"})
		}
		for _, g := range s.Groups {
			if !g.Defeated && pack.Groups[g.Group].Lieutenant {
				out = append(out, MapTarget{ID: g.ID, Label: pack.Groups[g.Group].Name + " (Lieutenant)"})
			}
		}
		for _, st := range s.Sites {
			if holder := siteLieutenant(pack, st); holder != "" {
				out = append(out, MapTarget{ID: "site:" + st.ID, Label: pack.Groups[holder].Name + " (Lieutenant, holding a site)"})
			}
		}
	case e.Kind == "lure":
		for _, st := range s.Sites {
			if !st.Seen || !Adjacent(s.Player, st.Pos) || !s.siteDefended(pack, &st) {
				continue
			}
			if t := pack.Sites[st.ID].Type; t == "ruins" || t == "grotto" {
				continue
			}
			out = append(out, MapTarget{ID: st.ID, Label: pack.Sites[st.ID].Name})
		}
	default:
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// siteLieutenant names the Lieutenant holding a site, if one still does.
func siteLieutenant(pack Pack, st SiteState) string {
	def := pack.Sites[st.ID]
	holder := def.Garrison
	if holder == "" {
		holder = def.OccupiedBy
	}
	if holder == "" || st.Freed || st.Conquered || !pack.Groups[holder].Lieutenant {
		return ""
	}
	return holder
}

// warlordVisible reports the Warlord standing on a placed hex in sight.
func (s *State) warlordVisible(r Rules) bool {
	pos := s.Warlord.Pos()
	if _, _, placed := s.tileAt(r.Pack, pos); !placed && !s.Warlord.Arrived {
		return false
	}
	return s.inSight(r, pos)
}

func hasTarget(targets []MapTarget, id string) bool {
	for _, t := range targets {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (s *State) playMapCard(r Rules, a Action) error {
	if countOf(s.Hand, a.Card) == 0 {
		return fmt.Errorf("%s is not in your hand", r.cardName(a.Card))
	}
	def, ok := r.Cards[a.Card]
	if !ok || def.MapEffect == nil {
		return fmt.Errorf("%s has no map effect; it is worth 1 Move or 1 Influence", r.cardName(a.Card))
	}
	e := def.MapEffect
	if ok, why := s.mapCardReady(r, e); !ok {
		return fmt.Errorf("%s: %s", def.Name, why)
	}
	cost := mapEnergyCost(def)
	if cost > s.Energy {
		return fmt.Errorf("%s costs %d energy; you have %d", def.Name, cost, s.Energy)
	}
	// Choose the target before anything is spent.
	target := ""
	if targets := s.mapCardTargets(r, e); targets != nil {
		switch e.Kind {
		case "mark":
			target = a.Group
		case "lure":
			target = a.Site
		default:
			target = a.Choice
		}
		if !hasTarget(targets, target) {
			return fmt.Errorf("%s: choose one of its targets", def.Name)
		}
	}
	s.Hand, _ = removeFirst(s.Hand, a.Card)
	s.Spent = append(s.Spent, a.Card)
	s.Turn.CardsUsed++
	s.Energy -= cost
	switch e.Kind {
	case "move":
		s.Turn.Move += e.Amount
		s.Turn.IgnoreTerrain = s.Turn.IgnoreTerrain || e.IgnoreTerrain
		s.Turn.IgnoreNight = s.Turn.IgnoreNight || e.IgnoreNight
		if e.ChooseTerrain {
			s.Turn.CheapTerrain = target
		}
		if e.CrossCost > 0 && !s.Turn.Crossed && (s.Turn.CrossCost == 0 || e.CrossCost < s.Turn.CrossCost) {
			s.Turn.CrossCost = e.CrossCost
		}
		s.Turn.CarryOver = s.Turn.CarryOver || e.CarryOver
		s.Turn.ExploreDiscount += e.ExploreDiscount
		s.Turn.ExploreReach = max(s.Turn.ExploreReach, e.ExploreReach)
		s.Turn.Sneak = s.Turn.Sneak || e.Sneak
	case "draw":
		s.drawCards(e.Amount)
	case "energy":
		s.Energy += e.Amount
	case "sight":
		s.Turn.SightBonus += e.Amount
		s.Turn.RevealTiles = s.Turn.RevealTiles || e.RevealTiles
	case "scout":
		if e.Amount > s.Turn.ScoutRange {
			s.Turn.ScoutRange = e.Amount
		}
	case "look":
		n := min(e.Amount, len(s.Draw))
		s.PendingLook = &PendingLook{Card: a.Card, Cards: append([]string{}, s.Draw[:n]...), Draw: min(e.Draw, n), Bottom: e.Bottom}
	case "influence":
		s.gainInfluence(r, e.Amount)
	case "sneak":
		s.Turn.Sneak = true
	case "hide":
		s.addStatus(r.Config.Statuses.Hidden, e.Amount)
		s.HiddenUntil = s.Turn.Number + 1
	case "fall_back":
		pb := s.PendingBattle
		s.PendingBattle = nil
		s.Phase = PhaseTurn
		s.Player = pb.Origin
		s.Turn.Fought, s.Turn.ActionUsed = pb.FoughtBefore, pb.ActionBefore
		s.log("Fell back from %s to %s.", pb.Name, pb.Origin)
	case "high_ground":
		s.Turn.HighGround = true
		s.Turn.AmbushBonus += e.Amount
	case "mark":
		s.mark(r, target, e.Amount)
	case "extend_time":
		s.ExtraRounds += e.Amount
		s.TimeExtended = true
		s.log("You gained time: this quest now has %d rounds.", s.rounds(r))
	case "lure":
		st := s.site(target)
		s.log("Played %s: lured the defenders of %s into the open.", def.Name, r.Pack.Sites[st.ID].Name)
		return s.siteBattle(r, st, "", true)
	case "haggle":
		visit := s.visitHere(r)
		if s.Turn.HaggleAt != visit {
			s.Turn.HaggleAt, s.Turn.Haggle = visit, 0
		}
		s.Turn.Haggle += e.Amount
	case "field_medic":
		s.Camp.AidBonus += e.Amount
	case "spare_bandage":
		s.addStatus(r.Config.Statuses.SpareBandage, e.Amount)
	}
	s.refreshSight(r)
	s.log("Played %s on the map.", def.Name)
	return nil
}

// visitHere names the visit Influence is paid for: the camp, or the site the
// player stands on.
func (s *State) visitHere(r Rules) string {
	if s.Phase == PhaseCamp {
		return "camp"
	}
	if st := s.siteAt(s.Player); st != nil {
		return st.ID
	}
	return ""
}

// gainInfluence adds Influence for this visit.
func (s *State) gainInfluence(r Rules, n int) {
	visit := s.visitHere(r)
	if s.Turn.InfluenceSite != visit {
		s.Turn.Influence, s.Turn.InfluenceSite = 0, visit
	}
	s.Turn.Influence += n
}

// influencePrice is what an Influence payment costs at this visit, after
// Haggle (never below 1; a free payment stays free).
func (s *State) influencePrice(r Rules, base int) int {
	if base <= 0 || s.Turn.Haggle == 0 || s.Turn.HaggleAt != s.visitHere(r) {
		return base
	}
	return max(1, base-s.Turn.Haggle)
}

// payInfluence turns the offered cards and energy into Influence at this
// visit (energy at the configured rate), all or nothing.
func (s *State) payInfluence(r Rules, cards []string, energy int) error {
	if _, err := s.checkPayment(r, cards, 0); err != nil {
		return err
	}
	if energy < 0 || energy > s.Energy {
		return fmt.Errorf("you have %d energy, not %d", s.Energy, energy)
	}
	per := r.Config.Influence.EnergyPerInfluence
	if energy%per != 0 {
		return fmt.Errorf("energy pays for Influence %d at a time", per)
	}
	visit := s.visitHere(r)
	if s.Turn.InfluenceSite != visit {
		s.Turn.Influence, s.Turn.InfluenceSite = 0, visit
	}
	for _, id := range cards {
		s.Hand, _ = removeFirst(s.Hand, id)
		s.Spent = append(s.Spent, id)
		s.Turn.CardsUsed++
		s.Turn.Influence++
	}
	s.Energy -= energy
	s.Turn.Influence += energy / per
	return nil
}

// mark starts or refreshes a Hunter's Mark on the target.
func (s *State) mark(r Rules, target string, rests int) {
	name := target
	switch {
	case target == "warlord":
		name = r.Pack.Groups[s.Warlord.Group].Name
	case strings.HasPrefix(target, "site:"):
		if st := s.site(strings.TrimPrefix(target, "site:")); st != nil {
			name = r.Pack.Groups[siteLieutenant(r.Pack, *st)].Name
		}
	default:
		if g := s.group(target); g != nil {
			name = r.Pack.Groups[g.Group].Name
		}
	}
	for i := range s.Marks {
		if s.Marks[i].Target == target {
			s.Marks[i].RestsLeft = max(s.Marks[i].RestsLeft, rests)
			s.log("Marked %s again.", name)
			return
		}
	}
	s.Marks = append(s.Marks, Mark{Target: target, Name: name, RestsLeft: rests})
	s.log("Marked %s: you will see where it is for %d rest(s).", name, rests)
}

// followMarks keeps every marked enemy's position known: the Warlord's hex,
// a marked group's hex, or the site a marked Lieutenant holds. A mark on an
// enemy that is gone ends.
func (s *State) followMarks(r Rules) {
	kept := s.Marks[:0]
	for _, m := range s.Marks {
		switch {
		case m.Target == "warlord":
			if s.Warlord.Defeated {
				continue
			}
			pos := s.Warlord.Pos()
			s.Warlord.Seen, s.Warlord.LastSeen = true, &pos
		case strings.HasPrefix(m.Target, "site:"):
			st := s.site(strings.TrimPrefix(m.Target, "site:"))
			if st == nil || siteLieutenant(r.Pack, *st) == "" {
				continue
			}
			st.Seen = true
			s.Seen[hexKey(st.Pos)] = true
		default:
			g := s.group(m.Target)
			if g == nil || g.Defeated {
				continue
			}
			pos := g.Pos
			g.Seen, g.LastSeen = true, &pos
			s.Seen[hexKey(pos)] = true
		}
		kept = append(kept, m)
	}
	if len(kept) == 0 {
		s.Marks = nil
		return
	}
	s.Marks = kept
}

// marked reports a mark on the target.
func (s *State) marked(target string) bool {
	for _, m := range s.Marks {
		if m.Target == target {
			return true
		}
	}
	return false
}

// revealTiles marks every enemy on each tile with a hex within reach as seen.
func (s *State) revealTiles(r Rules, reach int) {
	for _, p := range s.Placed {
		near := false
		for _, h := range TileHexes(p.Center) {
			near = near || Distance(s.Player, h) <= reach
		}
		if !near {
			continue
		}
		for _, h := range TileHexes(p.Center) {
			for i := range s.Groups {
				if g := &s.Groups[i]; !g.Defeated && g.Pos == h {
					pos := g.Pos
					g.Seen, g.LastSeen = true, &pos
					s.Seen[hexKey(h)] = true
				}
			}
			if st := s.siteAt(h); st != nil && s.siteDefended(r.Pack, st) {
				st.Seen = true
				s.Seen[hexKey(h)] = true
			}
		}
	}
}

// statusStacks counts the player's stacks of a map status.
func (s *State) statusStacks(id string) int {
	for _, st := range s.Statuses {
		if st.ID == id {
			return st.Stacks
		}
	}
	return 0
}

// takeStatus removes up to n stacks of a status.
func (s *State) takeStatus(id string, n int) {
	kept := s.Statuses[:0]
	for _, st := range s.Statuses {
		if st.ID == id {
			st.Stacks -= n
			if st.Stacks <= 0 {
				continue
			}
		}
		kept = append(kept, st)
	}
	s.Statuses = kept
}

// hiddenSight is how many hexes less enemies see the player.
func (s *State) hiddenSight(r Rules) int {
	n := 0
	for _, st := range s.Statuses {
		if def, ok := r.Statuses[st.ID]; ok && def.Modifiers != nil {
			n += def.Modifiers.HiddenSight * st.Stacks
		}
	}
	return n
}

// battleStatuses are the map statuses that enter a battle (map-only ones never
// do).
func (s *State) battleStatuses(r Rules) []StatusStack {
	out := []StatusStack{}
	for _, st := range s.Statuses {
		if def, ok := r.Statuses[st.ID]; ok && def.Modifiers.MapOnly() {
			continue
		}
		out = append(out, st)
	}
	return out
}

// resolveLook arranges the looked-at cards: draw the chosen one, put the
// chosen ones on the bottom, and the rest back on top in the given order
// (top first). Cards are definition IDs, so copies are counted.
func (s *State) resolveLook(r Rules, a Action) error {
	look := s.PendingLook
	if look == nil {
		return errors.New("you are not looking at any cards")
	}
	left := map[string]int{}
	for _, id := range look.Cards {
		left[id]++
	}
	take := func(id, what string) error {
		if left[id] == 0 {
			return fmt.Errorf("%s is not one of the cards you are looking at (%s)", r.cardName(id), what)
		}
		left[id]--
		return nil
	}
	drawn := ""
	if look.Draw > 0 {
		if a.Card == "" {
			return errors.New("choose the card to draw")
		}
		if err := take(a.Card, "draw"); err != nil {
			return err
		}
		drawn = a.Card
	} else if a.Card != "" {
		return errors.New("this look draws no card")
	}
	if len(a.Bottom) > 0 && !look.Bottom {
		return errors.New("this look cannot put cards on the bottom")
	}
	for _, id := range a.Bottom {
		if err := take(id, "bottom"); err != nil {
			return err
		}
	}
	top := []string{}
	for _, id := range a.Cards {
		if err := take(id, "top"); err != nil {
			return err
		}
		top = append(top, id)
	}
	// Anything not placed keeps its order under the placed cards.
	for _, id := range look.Cards {
		if left[id] > 0 {
			left[id]--
			top = append(top, id)
		}
	}
	rest := append([]string{}, s.Draw[len(look.Cards):]...)
	s.Draw = append(append(top, rest...), a.Bottom...)
	if drawn != "" {
		s.Hand = append(s.Hand, drawn)
	}
	s.PendingLook = nil
	s.log("Looked at the top %d of your draw pile%s.", len(look.Cards), map[bool]string{true: " and drew one", false: ""}[drawn != ""])
	return nil
}
