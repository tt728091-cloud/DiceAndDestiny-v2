package expedition

import (
	"fmt"
	"sort"

	"diceanddestiny/server/internal/content"
)

// Rules bundles everything the map rules read: the config, the content pack,
// and the card and status definitions of the player's library (map effects,
// hand-size modifiers, names). It is immutable while a state is acted on.
type Rules struct {
	Config   Config
	Pack     Pack
	Cards    map[string]content.BattleCardDefinition
	Statuses map[string]content.BattleStatusDefinition
}

func (r Rules) cardName(id string) string {
	if c, ok := r.Cards[id]; ok {
		return c.Name
	}
	return id
}

func (r Rules) handDelta(statusID string) int {
	if def, ok := r.Statuses[statusID]; ok && def.Modifiers != nil {
		return def.Modifiers.HandSizeDelta
	}
	return 0
}

// New builds a region from a seed: the start tile placed, the countryside
// stack shuffled, the core tiles dealt to the core slots, the Warlord's route
// laid out and the opening hand drawn.
//
// deck lists the player's live card copies by definition ID (wounded copies
// are passed separately in wounds and stay out of every pile).
func New(r Rules, regionID string, seed uint64, deck, wounds []string, baseEnergy int) (*State, error) {
	cfg, pack := r.Config, r.Pack
	region, ok := pack.Regions[regionID]
	if !ok {
		return nil, fmt.Errorf("unknown expedition region %q", regionID)
	}
	if len(deck) == 0 {
		return nil, fmt.Errorf("the expedition deck is empty")
	}
	s := &State{Version: 1, Region: regionID, Seed: seed, Round: 1, Phase: PhaseTurn, CoreDeal: map[string]string{}, Seen: map[string]bool{}, Inventory: map[string]int{}, Statuses: []StatusStack{}, Groups: []GroupState{}, Sites: []SiteState{}, Hand: []string{}, Spent: []string{}, BaseEnergy: baseEnergy, Energy: baseEnergy, Log: []string{}, Placed: []PlacedTile{}}
	// Wounds carried in from before the expedition arrive as one grouping.
	s.addWound(wounds, "earlier wounds", 0)
	s.Draw = append([]string{}, deck...)
	sort.Strings(s.Draw)
	s.shuffle(s.Draw)
	// Countryside stack and core deal.
	s.Stack = append([]string{}, region.CountrysideTiles...)
	s.shuffle(s.Stack)
	core := append([]string{}, region.CoreTiles...)
	s.shuffle(core)
	coreSlots := []string{}
	for _, slot := range region.Slots {
		if slot.Kind == "core" {
			coreSlots = append(coreSlots, slot.ID)
		}
	}
	for i, slot := range coreSlots {
		s.CoreDeal[slot] = core[i]
	}
	// The start tile.
	s.placeTile(pack, "start", region.StartTile, Hex{})
	// The Warlord: his goal is on the dealt goal tile; he starts
	// start_distance hexes from it on the line toward the player's start.
	goalSlot := ""
	for slot, tile := range s.CoreDeal {
		if tile == region.Warlord.GoalTile {
			goalSlot = slot
		}
	}
	goalCenter := slotCenter(region, goalSlot)
	goal := goalCenter
	for _, h := range pack.Tiles[region.Warlord.GoalTile].Hexes {
		if h.Site == region.Warlord.GoalSite {
			off, _ := hexFromPair(h.Offset)
			goal = goalCenter.Add(off)
		}
	}
	s.Warlord = WarlordState{Group: region.Warlord.Group, Route: warlordRoute(region, goal, cfg.Warlord.StartDistance), Goal: goal, GoalSite: region.Warlord.GoalSite}
	// The first turn.
	s.Turn = TurnState{Number: 1, StartHex: s.Player}
	s.drawUpTo(r)
	s.refreshSight(r)
	s.log("Day 1 begins at %s.", pack.Tiles[region.StartTile].Name)
	return s, nil
}

func slotCenter(region Region, slotID string) Hex {
	for _, slot := range region.Slots {
		if slot.ID == slotID {
			c, _ := hexFromPair(slot.Center)
			return c
		}
	}
	return Hex{}
}

func slotKind(region Region, slotID string) string {
	for _, slot := range region.Slots {
		if slot.ID == slotID {
			return slot.Kind
		}
	}
	return ""
}

// placeTile adds a tile's hexes, sites and open groups to the map.
func (s *State) placeTile(pack Pack, slot, tileID string, center Hex) {
	s.Placed = append(s.Placed, PlacedTile{Slot: slot, Tile: tileID, Center: center})
	tile := pack.Tiles[tileID]
	for _, h := range tile.Hexes {
		off, _ := hexFromPair(h.Offset)
		pos := center.Add(off)
		if h.Site != "" {
			s.Sites = append(s.Sites, SiteState{ID: h.Site, Tile: tileID, Pos: pos})
		}
		if h.Group != "" {
			s.Groups = append(s.Groups, GroupState{ID: tileID + "/" + h.Group, Group: h.Group, Home: pos, Pos: pos})
		}
	}
}

// effectiveHandSize is the hand size after map statuses (hand symmetry).
func (s *State) effectiveHandSize(r Rules) int {
	return max(1, r.Config.Hand.Size+s.statusHandDelta(r))
}

func (s *State) effectiveHandLimit(r Rules) int {
	return max(s.effectiveHandSize(r), r.Config.Hand.Limit+s.statusHandDelta(r))
}

func (s *State) statusHandDelta(r Rules) int {
	delta := 0
	for _, st := range s.Statuses {
		delta += st.Stacks * r.handDelta(st.ID)
	}
	return delta
}

// drawUpTo draws until the hand holds the hand size, from the draw pile only.
func (s *State) drawUpTo(r Rules) int {
	drew := 0
	for len(s.Hand) < s.effectiveHandSize(r) && len(s.Draw) > 0 {
		s.Hand = append(s.Hand, s.Draw[0])
		s.Draw = s.Draw[1:]
		drew++
	}
	return drew
}

// drawCards draws n cards (a map effect), never past the draw pile.
func (s *State) drawCards(n int) int {
	drew := 0
	for i := 0; i < n && len(s.Draw) > 0; i++ {
		s.Hand = append(s.Hand, s.Draw[0])
		s.Draw = s.Draw[1:]
		drew++
	}
	return drew
}

// sightRange is how far the player sees now, with gear and card bonuses.
func (s *State) sightRange(r Rules) int {
	reach := r.Config.SightRange(s.Night) + s.Turn.SightBonus
	if s.Night {
		for _, id := range sortedKeys(s.Inventory) {
			if g, ok := r.Pack.Gear[id]; ok && g.Effect == "night_sight" && s.Inventory[id] > 0 {
				reach += g.Amount
			}
		}
	}
	return reach
}

// inSight reports whether h is within the player's sight now.
func (s *State) inSight(r Rules, h Hex) bool {
	return Distance(s.Player, h) <= s.sightRange(r)
}

// refreshSight marks everything in sight (and scouted) as seen and records
// where enemies were last seen.
func (s *State) refreshSight(r Rules) {
	pack := r.Pack
	reach := s.sightRange(r)
	if s.Turn.ScoutRange > reach {
		reach = s.Turn.ScoutRange
	}
	for _, h := range s.placedHexes(pack) {
		if Distance(s.Player, h) <= reach {
			s.Seen[hexKey(h)] = true
		}
	}
	for i := range s.Groups {
		g := &s.Groups[i]
		if g.Defeated {
			continue
		}
		if Distance(s.Player, g.Pos) <= reach {
			pos := g.Pos
			g.Seen, g.LastSeen = true, &pos
		}
	}
	for i := range s.Sites {
		if Distance(s.Player, s.Sites[i].Pos) <= reach {
			s.Sites[i].Seen = true
		}
	}
	if !s.Warlord.Defeated {
		if pos := s.Warlord.Pos(); Distance(s.Player, pos) <= reach {
			if _, _, placed := s.tileAt(pack, pos); placed || s.Warlord.Arrived {
				p := pos
				s.Warlord.Seen, s.Warlord.LastSeen = true, &p
			}
		}
	}
}

// warlordRoute lays out the Warlord's march at region build: he starts
// start_distance hexes from his goal, as close to the player's start as the
// region allows, and walks the shortest path to the goal that stays on the
// region's tiles and off every slot centre (where open groups stand), so he
// never shares a hex with a group he passes. Deterministic for equal inputs.
func warlordRoute(region Region, goal Hex, startDistance int) []Hex {
	footprint := map[Hex]bool{}
	forbidden := map[Hex]bool{}
	centers := []Hex{{}}
	for _, slot := range region.Slots {
		c, _ := hexFromPair(slot.Center)
		centers = append(centers, c)
	}
	for _, c := range centers {
		for _, h := range TileHexes(c) {
			footprint[h] = true
		}
		if c != goal {
			forbidden[c] = true
		}
	}
	// Breadth-first distances from the goal over allowed hexes.
	dist := map[Hex]int{goal: 0}
	prev := map[Hex]Hex{}
	frontier := []Hex{goal}
	for len(frontier) > 0 {
		h := frontier[0]
		frontier = frontier[1:]
		for _, n := range Neighbors(h) {
			if !footprint[n] || forbidden[n] {
				continue
			}
			if _, seen := dist[n]; seen {
				continue
			}
			dist[n] = dist[h] + 1
			prev[n] = h
			frontier = append(frontier, n)
		}
	}
	// Choose the start: the farthest reachable hex up to start_distance along
	// the march, nearest the player's start, ties by coordinate.
	var start Hex
	found := false
	for want := startDistance; want >= 1 && !found; want-- {
		for h, d := range dist {
			if d != want || h == (Hex{}) {
				continue
			}
			if !found || Distance(h, Hex{}) < Distance(start, Hex{}) || Distance(h, Hex{}) == Distance(start, Hex{}) && (h.Q < start.Q || h.Q == start.Q && h.R < start.R) {
				start, found = h, true
			}
		}
	}
	if !found {
		return []Hex{goal}
	}
	route := []Hex{start}
	for h := start; h != goal; {
		h = prev[h]
		route = append(route, h)
	}
	return route
}
