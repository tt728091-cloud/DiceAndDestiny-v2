package expedition

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/content"
)

// Action is one expedition_act request. Unused fields stay empty.
type Action struct {
	Kind    string   `json:"kind"`
	Target  *Hex     `json:"target,omitempty"`  // move
	Slot    string   `json:"slot,omitempty"`    // explore
	Cards   []string `json:"cards,omitempty"`   // payment, Influence, discard
	Energy  int      `json:"energy,omitempty"`  // payment
	Card    string   `json:"card,omitempty"`    // play_map_card
	Service string   `json:"service,omitempty"` // use_site / camp_service
	Item    string   `json:"item,omitempty"`    // shop, enter_site (siege), the bandage to spend
	Gold    int      `json:"gold,omitempty"`    // trainer
	Count   int      `json:"count,omitempty"`   // mend wounds (points of damage)
	Wound   string   `json:"wound,omitempty"`   // camp_service: the wound to heal ("" = the first eligible, "all" = every eligible)
	Choice  string   `json:"choice,omitempty"`  // event_choice
	Group   string   `json:"group,omitempty"`   // challenge: group instance or "warlord"
	Site    string   `json:"site,omitempty"`    // enter_site
}

// ActionKinds the authority accepts.
var ActionKinds = []string{"move", "explore", "play_map_card", "use_site", "event_choice", "challenge", "enter_site", "discard", "end_turn", "make_camp", "camp_service", "break_camp"}

// Rewards are what an action earned outside the map state: XP belongs to the
// campaign sheet, which the loadout layer credits.
type Rewards struct {
	XP int `json:"xp,omitempty"`
}

// Act applies one action. It validates everything first and returns a clear
// error without changing the state when the action is illegal.
func (s *State) Act(r Rules, a Action) (Rewards, error) {
	if s.Result != "" {
		return Rewards{}, fmt.Errorf("the expedition is over (%s)", s.Result)
	}
	if s.PendingBattle != nil {
		return Rewards{}, errors.New("a battle is pending; fight it first")
	}
	switch a.Kind {
	case "move":
		return Rewards{}, s.move(r, a)
	case "explore":
		return Rewards{}, s.explore(r, a)
	case "play_map_card":
		return Rewards{}, s.playMapCard(r, a)
	case "use_site":
		return s.useSite(r, a)
	case "event_choice":
		return s.eventChoice(r, a)
	case "challenge":
		return Rewards{}, s.challenge(r, a)
	case "enter_site":
		return Rewards{}, s.enterSite(r, a)
	case "discard":
		return Rewards{}, s.discard(r, a)
	case "end_turn":
		return Rewards{}, s.endTurn(r)
	case "make_camp":
		return Rewards{}, s.makeCamp(r)
	case "camp_service":
		return Rewards{}, s.campService(r, a)
	case "break_camp":
		return Rewards{}, s.breakCamp(r)
	}
	return Rewards{}, fmt.Errorf("unknown expedition action %q", a.Kind)
}

func (s *State) requirePhase(phase string) error {
	if s.Phase != phase {
		switch s.Phase {
		case PhaseEvent:
			return errors.New("resolve the event first")
		case PhaseCamp:
			return errors.New("you are resting; break camp first")
		case PhaseBattle:
			return errors.New("a battle is pending")
		}
		return fmt.Errorf("not during %s", s.Phase)
	}
	return nil
}

// --- payment ----------------------------------------------------------------

// payMove converts cards and energy into Move. It is all-or-nothing: the
// caller validates the total before applying.
func (s *State) checkPayment(r Rules, cards []string, energy int) (int, error) {
	seen := map[string]int{}
	for _, id := range cards {
		seen[id]++
		if countOf(s.Hand, id) < seen[id] {
			return 0, fmt.Errorf("%s is not in your hand", r.cardName(id))
		}
	}
	if energy < 0 || energy > s.Energy {
		return 0, fmt.Errorf("you have %d energy, not %d", s.Energy, energy)
	}
	per := r.Config.Move.EnergyPerMove
	if energy%per != 0 {
		return 0, fmt.Errorf("energy pays for Move %d at a time", per)
	}
	return len(cards) + energy/per, nil
}

func (s *State) applyPayment(r Rules, cards []string, energy int) {
	for _, id := range cards {
		s.Hand, _ = removeFirst(s.Hand, id)
		s.Spent = append(s.Spent, id)
		s.Turn.CardsUsed++
	}
	s.Energy -= energy
	s.Turn.Move += len(cards) + energy/r.Config.Move.EnergyPerMove
}

// moveCost is the Move cost of entering h now, and whether it can be entered.
func (s *State) moveCost(r Rules, h Hex) (int, string, bool) {
	_, th, placed := s.tileAt(r.Pack, h)
	if !placed {
		return 0, "that hex is not on the map", false
	}
	terrain := r.Config.Terrain[th.Terrain]
	if terrain.Impassable {
		return 0, fmt.Sprintf("%s is impassable", th.Terrain), false
	}
	if s.Turn.IgnoreTerrain {
		return 1, "", true
	}
	night := s.Night && !s.Turn.IgnoreNight
	cost, _ := r.Config.TerrainCost(th.Terrain, night)
	return cost, "", true
}

// --- move -------------------------------------------------------------------

// provokedBy returns the living open group the player provokes by stepping
// from one hex next to it to another hex next to it.
func (s *State) provokedBy(from, to Hex) *GroupState {
	var found *GroupState
	for i := range s.Groups {
		g := &s.Groups[i]
		if g.Defeated || !Adjacent(from, g.Pos) || !Adjacent(to, g.Pos) {
			continue
		}
		if found == nil || g.ID < found.ID {
			found = g
		}
	}
	return found
}

func (s *State) move(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if a.Target == nil {
		return errors.New("move needs a target hex")
	}
	to := *a.Target
	if !Adjacent(s.Player, to) {
		return errors.New("you can only move to an adjacent hex")
	}
	cost, why, ok := s.moveCost(r, to)
	if !ok {
		return errors.New(why)
	}
	if why, blocked := s.hexBlocked(r.Pack, to); blocked {
		return errors.New(why)
	}
	paid, err := s.checkPayment(r, a.Cards, a.Energy)
	if err != nil {
		return err
	}
	if s.Turn.Move+paid < cost {
		return fmt.Errorf("entering that hex costs %d Move; you have %d", cost, s.Turn.Move+paid)
	}
	s.applyPayment(r, a.Cards, a.Energy)
	s.Turn.Move -= cost
	from := s.Player
	s.Player = to
	s.refreshSight(r)
	s.log("Moved to %s for %d Move.", to, cost)
	if g := s.provokedBy(from, to); g != nil {
		s.startBattle(r, battleStart{kind: "provoked", group: g, origin: from, position: to})
		return nil
	}
	if st := s.siteAt(to); st != nil && r.Pack.Sites[st.ID].Type == "event" && !st.Resolved {
		s.PendingEvent = &PendingEvent{Site: st.ID, Event: r.Pack.Sites[st.ID].Event}
		s.Phase = PhaseEvent
		s.log("%s: %s", r.Pack.Sites[st.ID].Name, r.Pack.Events[s.PendingEvent.Event].Name)
	}
	return nil
}

// --- explore ----------------------------------------------------------------

// exploreOptions lists empty slots next to the player and whether each can be
// explored now (countryside while the stack lasts, core afterwards).
type exploreOption struct {
	Slot   string
	Kind   string
	Center Hex
	Ready  bool
	Why    string
}

func (s *State) exploreOptions(r Rules) []exploreOption {
	region := r.Pack.Regions[s.Region]
	var out []exploreOption
	for _, slot := range region.Slots {
		if s.slotPlaced(slot.ID) {
			continue
		}
		center, _ := hexFromPair(slot.Center)
		adjacent := false
		for _, h := range TileHexes(center) {
			adjacent = adjacent || Adjacent(s.Player, h)
		}
		if !adjacent {
			continue
		}
		o := exploreOption{Slot: slot.ID, Kind: slot.Kind, Center: center, Ready: true}
		if slot.Kind == "countryside" && len(s.Stack) == 0 {
			o.Ready, o.Why = false, "the countryside tiles are all placed"
		}
		if slot.Kind == "core" && len(s.Stack) > 0 {
			o.Ready, o.Why = false, "core tiles come after the countryside tiles"
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out
}

func (s *State) explore(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	var option *exploreOption
	for _, o := range s.exploreOptions(r) {
		if o.Slot == a.Slot {
			copy := o
			option = &copy
		}
	}
	if option == nil {
		return fmt.Errorf("slot %q is not an empty tile slot next to you", a.Slot)
	}
	if !option.Ready {
		return errors.New(option.Why)
	}
	cost := r.Config.Move.ExploreCost
	paid, err := s.checkPayment(r, a.Cards, a.Energy)
	if err != nil {
		return err
	}
	if s.Turn.Move+paid < cost {
		return fmt.Errorf("exploring costs %d Move; you have %d", cost, s.Turn.Move+paid)
	}
	s.applyPayment(r, a.Cards, a.Energy)
	s.Turn.Move -= cost
	var tile string
	if option.Kind == "countryside" {
		tile, s.Stack = s.Stack[0], s.Stack[1:]
	} else {
		tile = s.CoreDeal[option.Slot]
	}
	s.placeTile(r.Pack, option.Slot, tile, option.Center)
	s.refreshSight(r)
	s.log("Explored %s: %s.", option.Slot, r.Pack.Tiles[tile].Name)
	return nil
}

// --- cards ------------------------------------------------------------------

// mapEnergyCost is what playing a card for its map effect costs: the card's
// printed energy cost, as in battle, plus any map-only extra its map effect
// authors. Spending a card as 1 Move or 1 Influence costs no energy.
func mapEnergyCost(def content.BattleCardDefinition) int {
	cost := def.Cost.Energy
	if def.MapEffect != nil {
		cost += def.MapEffect.EnergyCost
	}
	return cost
}

func (s *State) playMapCard(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if countOf(s.Hand, a.Card) == 0 {
		return fmt.Errorf("%s is not in your hand", r.cardName(a.Card))
	}
	def, ok := r.Cards[a.Card]
	if !ok || def.MapEffect == nil {
		return fmt.Errorf("%s has no map effect; it is worth 1 Move or 1 Influence", r.cardName(a.Card))
	}
	e := def.MapEffect
	cost := mapEnergyCost(def)
	if cost > s.Energy {
		return fmt.Errorf("%s costs %d energy; you have %d", def.Name, cost, s.Energy)
	}
	s.Hand, _ = removeFirst(s.Hand, a.Card)
	s.Spent = append(s.Spent, a.Card)
	s.Turn.CardsUsed++
	s.Energy -= cost
	switch e.Kind {
	case "move":
		s.Turn.Move += e.Amount
		if e.IgnoreTerrain {
			s.Turn.IgnoreTerrain = true
		}
		if e.IgnoreNight {
			s.Turn.IgnoreNight = true
		}
	case "draw":
		s.drawCards(e.Amount)
	case "energy":
		s.Energy += e.Amount
	case "sight":
		s.Turn.SightBonus += e.Amount
	case "scout":
		if e.Amount > s.Turn.ScoutRange {
			s.Turn.ScoutRange = e.Amount
		}
	}
	s.refreshSight(r)
	s.log("Played %s on the map.", def.Name)
	return nil
}

func (s *State) discard(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if len(a.Cards) == 0 {
		return errors.New("choose a card to discard")
	}
	if _, err := s.checkPayment(r, a.Cards, 0); err != nil {
		return err
	}
	for _, id := range a.Cards {
		s.Hand, _ = removeFirst(s.Hand, id)
		s.Spent = append(s.Spent, id)
		s.Turn.CardsUsed++
	}
	s.log("Discarded %d card(s).", len(a.Cards))
	return nil
}

// --- turn end, rest ---------------------------------------------------------

func (s *State) canEndTurn() (bool, string) {
	if s.Turn.CardsUsed < 1 && !s.Turn.Fought {
		return false, "play or discard at least 1 card first"
	}
	return true, ""
}

func (s *State) endTurn(r Rules) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if ok, why := s.canEndTurn(); !ok {
		return errors.New(why)
	}
	s.nightHuntersRoam(r)
	drew := s.drawUpTo(r)
	s.log("Turn %d ends; drew %d.", s.Turn.Number, drew)
	if len(s.Draw) == 0 {
		// One pass through the deck is one round: the round ends with a rest.
		s.log("The draw pile is empty: round %d is over.", s.Round)
		s.beginRest(r, "deck_empty", 0)
		return nil
	}
	s.nextTurn(r)
	return nil
}

func (s *State) nextTurn(r Rules) {
	s.Turn = TurnState{Number: s.Turn.Number + 1, StartHex: s.Player}
	s.Phase = PhaseTurn
	s.refreshSight(r)
}

func (s *State) makeCamp(r Rules) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	forfeited := len(s.Hand)
	s.Spent = append(s.Spent, s.Hand...)
	s.Hand = []string{}
	s.beginRest(r, "make_camp", forfeited)
	return nil
}

// beginRest runs rest steps 1 to 3 (reshuffle Spent into the draw pile, reset
// energy, draw the fresh hand) and opens the camp for healing and services;
// breakCamp finishes the rest. Every wound is untreated again at a new camp.
func (s *State) beginRest(r Rules, reason string, forfeited int) {
	camp := &CampState{Reason: reason, Turn: s.Turn.Number, Round: s.Round, EnergyBefore: s.Energy, Forfeited: forfeited, Heals: []HealRoll{}, Deadline: s.Round >= r.Config.RoundsPerRegion}
	if st := s.siteAt(s.Player); st != nil && r.Pack.Sites[st.ID].HasInn() && !s.siteDefended(r.Pack, st) && !st.Conquered {
		camp.Inn, camp.Site = true, st.ID
	}
	camp.Reshuffled = len(s.Spent)
	s.Draw = append(s.Draw, s.Spent...)
	s.Spent = []string{}
	s.shuffle(s.Draw)
	s.Energy = s.BaseEnergy
	for i := range s.Wounds {
		s.Wounds[i].Treated, s.Wounds[i].Bandaged = false, false
	}
	s.Camp = camp
	s.Phase = PhaseCamp
	s.Turn.Influence, s.Turn.InfluenceSite, s.Turn.DeckOpen = 0, "camp", ""
	camp.Drew = s.drawUpTo(r)
	camp.HandDrawn, camp.HandSize = true, s.effectiveHandSize(r)
	s.log("Rest (%s): %d Spent cards reshuffled into the draw pile, energy reset to %d, drew %d.", reason, camp.Reshuffled, s.BaseEnergy, camp.Drew)
}

func (s *State) campService(r Rules, a Action) error {
	if err := s.requirePhase(PhaseCamp); err != nil {
		return err
	}
	switch a.Service {
	case "influence":
		if len(a.Cards) == 0 {
			return errors.New("choose cards to play for Influence")
		}
		if _, err := s.checkPayment(r, a.Cards, 0); err != nil {
			return err
		}
		for _, id := range a.Cards {
			s.Hand, _ = removeFirst(s.Hand, id)
			s.Spent = append(s.Spent, id)
			s.Turn.CardsUsed++
			s.Turn.Influence++
		}
		return nil
	case "basic_aid":
		return s.campHeal(r, a, "basic_aid")
	case "bandage":
		return s.campHeal(r, a, "bandage")
	}
	return fmt.Errorf("unknown camp service %q", a.Service)
}

// campHeal rolls one healing attempt per eligible wound: basic aid on an
// untreated wound (then it is treated), a bandage on a treated wound (then it
// is bandaged). With Wound "all" every eligible wound is tried in order.
func (s *State) campHeal(r Rules, a Action, method string) error {
	cfg := r.Config
	if len(s.Wounds) == 0 {
		return errors.New("you have no wounds to heal")
	}
	eligible := func(w Wound) (bool, string) {
		switch method {
		case "basic_aid":
			if w.Treated {
				return false, "already treated at this camp; a bandage can heal it further"
			}
		case "bandage":
			if !w.Treated {
				return false, "treat it with basic aid first"
			}
			if w.Bandaged {
				return false, "already bandaged at this camp"
			}
		}
		return true, ""
	}
	var targets []string
	switch a.Wound {
	case "all":
		for _, w := range s.Wounds {
			if ok, _ := eligible(w); ok {
				targets = append(targets, w.ID)
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no wound can take %s now", healName(method))
		}
	case "":
		for _, w := range s.Wounds {
			if ok, _ := eligible(w); ok {
				targets = append(targets, w.ID)
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no wound can take %s now", healName(method))
		}
	default:
		w := s.Wounds.find(a.Wound)
		if w == nil {
			return fmt.Errorf("you have no wound %q", a.Wound)
		}
		if ok, why := eligible(*w); !ok {
			return fmt.Errorf("%s: %s", healName(method), why)
		}
		targets = []string{a.Wound}
	}
	item, die := "", cfg.CampHealing.BasicAidDie
	if method == "bandage" {
		item = a.Item
		if item == "" {
			item = "bandage"
		}
		g, ok := r.Pack.Gear[item]
		if !ok || g.Effect != "mend_roll" {
			return fmt.Errorf("%s is not something that heals", item)
		}
		if s.Inventory[item] < len(targets) {
			return fmt.Errorf("you have %d %s, not %d", s.Inventory[item], g.Name, len(targets))
		}
		die = cfg.CampHealing.BandageDie
	}
	bonus := s.innBonus(r)
	for _, id := range targets {
		w := s.Wounds.find(id)
		if method == "bandage" {
			s.Inventory[item]--
			w.Bandaged = true
		} else {
			w.Treated = true
		}
		roll := s.die(die)
		healed := s.healFromWound(w, roll+bonus)
		result := HealRoll{Wound: id, Source: w.Source, Method: method, Item: item, Die: die, Roll: roll, Bonus: bonus, Healed: len(healed), Cards: healed, Remaining: w.Damage(), Closed: w.Damage() == 0}
		s.Camp.Heals = append(s.Camp.Heals, result)
		s.Camp.Healed += len(healed)
		s.log("%s on the %s wound: rolled %d%s, healed %d; %d damage remains.", healName(method), w.Source, roll, map[bool]string{true: fmt.Sprintf("+%d", bonus), false: ""}[bonus > 0], len(healed), w.Damage())
	}
	s.dropClosedWounds()
	return nil
}

func healName(method string) string {
	if method == "bandage" {
		return "a bandage"
	}
	return "basic aid"
}

// healFromWound returns up to n of the wound's cards to the draw pile, each at
// a seeded position, and reports them. The wound keeps the rest.
func (s *State) healFromWound(w *Wound, n int) []string {
	healed := []string{}
	for len(w.Cards) > 0 && len(healed) < n {
		card := w.Cards[0]
		w.Cards = append([]string{}, w.Cards[1:]...)
		at := s.intn(len(s.Draw) + 1)
		s.Draw = append(s.Draw[:at], append([]string{card}, s.Draw[at:]...)...)
		healed = append(healed, card)
	}
	return healed
}

// healPoints heals n points of damage, oldest wound first (a site service, an
// event, or winning the region), and reports the cards that came back.
func (s *State) healPoints(n int) []string {
	healed := []string{}
	for i := range s.Wounds {
		if len(healed) >= n {
			break
		}
		healed = append(healed, s.healFromWound(&s.Wounds[i], n-len(healed))...)
	}
	s.dropClosedWounds()
	return healed
}

func (s *State) breakCamp(r Rules) error {
	if err := s.requirePhase(PhaseCamp); err != nil {
		return err
	}
	cfg := r.Config
	summary := &RestSummary{Camp: *s.Camp, Patrols: []string{}}
	// 4. The Warlord marches; roaming Lieutenants and patrols step.
	summary.WarlordFrom = s.Warlord.Pos()
	summary.WarlordSteps = s.marchWarlord(r, cfg.Warlord.MarchPerRest)
	summary.WarlordTo = s.Warlord.Pos()
	summary.GoalTaken = s.Warlord.GoalTaken && summary.WarlordSteps > 0 && summary.WarlordTo == s.Warlord.Goal
	for i := range s.Groups {
		if moved := s.patrolStep(r, &s.Groups[i]); moved {
			summary.Patrols = append(summary.Patrols, s.Groups[i].ID)
		}
	}
	// Map statuses expire at rest.
	for _, st := range s.Statuses {
		summary.Statuses = append(summary.Statuses, st.ID)
	}
	s.Statuses = []StatusStack{}
	// 5. Day <-> Night, the round advances, draw up to hand size.
	s.Round++
	s.Night = !s.Night
	summary.RoundAfter, summary.NightAfter = s.Round, s.Night
	summary.Drew, summary.Healed = s.Camp.Drew, s.Camp.Healed
	if summary.Camp.Heals == nil {
		summary.Camp.Heals = []HealRoll{}
	}
	if !s.Camp.HandDrawn {
		// A camp saved before the rest drew the hand: draw it now.
		summary.Drew = s.drawUpTo(r)
	} else if grow := s.effectiveHandSize(r) - s.Camp.HandSize; grow > 0 {
		// A status that shrank the hand expired with the rest: top the hand
		// up by what it freed, never refilling cards played at camp.
		for ; grow > 0 && len(s.Hand) < s.effectiveHandSize(r) && len(s.Draw) > 0; grow-- {
			s.Hand = append(s.Hand, s.Draw[0])
			s.Draw = s.Draw[1:]
			summary.Drew++
		}
	}
	s.Camp = nil
	s.Turn = TurnState{Number: s.Turn.Number + 1, StartHex: s.Player}
	s.LastRest = summary
	if s.Round > cfg.RoundsPerRegion {
		// The deadline: the Warlord comes to the player, in the open.
		summary.Deadline = true
		s.Warlord.Arrived = true
		s.Warlord.Route = append(s.Warlord.Route, s.Player)
		s.Warlord.Index = len(s.Warlord.Route) - 1
		s.refreshSight(r)
		s.log("The last round is over: the Warlord comes for you.")
		s.startBattle(r, battleStart{kind: "deadline", warlord: true, origin: s.Player, position: s.Player})
		return nil
	}
	s.Phase = PhaseTurn
	s.refreshSight(r)
	s.log("%s %d begins.", dayName(s.Night), (s.Round+1)/2)
	return nil
}

func dayName(night bool) string {
	if night {
		return "Night"
	}
	return "Day"
}

// marchWarlord moves him up to steps hexes along his route toward his goal.
func (s *State) marchWarlord(r Rules, steps int) int {
	if s.Warlord.Defeated || s.Warlord.GoalTaken || s.Warlord.Arrived {
		return 0
	}
	moved := 0
	for i := 0; i < steps && s.Warlord.Index < len(s.Warlord.Route)-1; i++ {
		next := s.Warlord.Route[s.Warlord.Index+1]
		if next == s.Player || s.openGroupAt(next) != nil {
			// He waits a step short of the player or of a group on his road
			// rather than sharing a hex with them.
			break
		}
		s.Warlord.Index++
		moved++
	}
	if s.Warlord.Index == len(s.Warlord.Route)-1 && !s.Warlord.GoalTaken {
		s.Warlord.GoalTaken = true
		if st := s.site(s.Warlord.GoalSite); st != nil {
			st.Conquered = true
		}
		s.log("The Warlord has taken %s.", r.Pack.Sites[s.Warlord.GoalSite].Name)
	}
	return moved
}

// patrolStep moves a patrolling group one step along its cycle, unless the
// step would put it within two hexes of another living open group, onto the
// player, or onto a site or impassable hex.
func (s *State) patrolStep(r Rules, g *GroupState) bool {
	def := r.Pack.Groups[g.Group]
	if g.Defeated || len(def.Patrol) == 0 {
		return false
	}
	nextIndex := (g.Patrol + 1) % len(def.Patrol)
	off, _ := hexFromPair(def.Patrol[nextIndex])
	next := g.Home.Add(off)
	if next == s.Player || s.warlordAt(next) {
		return false
	}
	for _, other := range s.Groups {
		if other.ID != g.ID && !other.Defeated && Distance(other.Pos, next) <= 2 {
			return false
		}
	}
	g.Patrol = nextIndex
	g.Pos = next
	return true
}

// nightHuntersRoam steps every night hunter once per turn at night.
func (s *State) nightHuntersRoam(r Rules) {
	if !s.Night {
		return
	}
	for i := range s.Groups {
		if r.Pack.Groups[s.Groups[i].Group].Habit == "night_hunter" {
			s.patrolStep(r, &s.Groups[i])
		}
	}
}

// --- sites ------------------------------------------------------------------

// siteOpen reports whether the site at the player's hex offers services now.
func (s *State) siteHere(r Rules) (*SiteState, Site, string) {
	st := s.siteAt(s.Player)
	if st == nil {
		return nil, Site{}, "there is no site here"
	}
	def := r.Pack.Sites[st.ID]
	if st.Conquered {
		return st, def, fmt.Sprintf("%s is held by the Warlord; its services are gone", def.Name)
	}
	if s.siteDefended(r.Pack, st) {
		return st, def, fmt.Sprintf("%s is held by enemies", def.Name)
	}
	if s.Night && !def.OpenAtNight() {
		return st, def, fmt.Sprintf("%s is closed at night; only the inn is open (make camp here)", def.Name)
	}
	return st, def, ""
}

func (s *State) useSite(r Rules, a Action) (Rewards, error) {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return Rewards{}, err
	}
	cfg := r.Config
	st, def, closed := s.siteHere(r)
	if closed != "" {
		return Rewards{}, errors.New(closed)
	}
	if s.Turn.InfluenceSite != st.ID {
		s.Turn.Influence, s.Turn.InfluenceSite = 0, st.ID
	}
	// Cards offered with any service become Influence here first.
	if len(a.Cards) > 0 {
		if _, err := s.checkPayment(r, a.Cards, 0); err != nil {
			return Rewards{}, err
		}
		for _, id := range a.Cards {
			s.Hand, _ = removeFirst(s.Hand, id)
			s.Spent = append(s.Spent, id)
			s.Turn.CardsUsed++
			s.Turn.Influence++
		}
	}
	switch a.Service {
	case "influence", "":
		return Rewards{}, nil
	case "mend":
		count := max(1, a.Count)
		if s.Wounds.Damage() < count {
			return Rewards{}, fmt.Errorf("you have %d damage to mend", s.Wounds.Damage())
		}
		switch def.Type {
		case "village", "monastery":
			per := cfg.Mending.VillageInfluencePerWound
			if def.Type == "monastery" {
				per = cfg.Mending.MonasteryInfluencePerWound
			}
			if s.Turn.Influence < per*count {
				return Rewards{}, fmt.Errorf("mending %d wound(s) here costs %d Influence; you have %d", count, per*count, s.Turn.Influence)
			}
			s.Turn.Influence -= per * count
		case "town":
			price := cfg.Mending.TownGoldPerWound * count
			if s.Gold < price {
				return Rewards{}, fmt.Errorf("the healer asks %d gold; you have %d", price, s.Gold)
			}
			s.Gold -= price
		default:
			return Rewards{}, fmt.Errorf("%s does not mend wounds", def.Name)
		}
		s.healPoints(count)
		s.Turn.ActionUsed = true
		s.log("Mended %d damage at %s.", count, def.Name)
		return Rewards{}, nil
	case "shop":
		if !containsString(def.Shop, a.Item) {
			return Rewards{}, fmt.Errorf("%s does not sell that", def.Name)
		}
		g := r.Pack.Gear[a.Item]
		if s.Gold < g.Price {
			return Rewards{}, fmt.Errorf("%s costs %d gold; you have %d", g.Name, g.Price, s.Gold)
		}
		s.Gold -= g.Price
		s.Inventory[a.Item]++
		s.log("Bought %s for %d gold.", g.Name, g.Price)
		return Rewards{}, nil
	case "trainer":
		if def.Type != "town" {
			return Rewards{}, fmt.Errorf("%s has no trainer", def.Name)
		}
		rate := cfg.Gold.TrainerGoldPerXP
		if a.Gold < rate || a.Gold%rate != 0 {
			return Rewards{}, fmt.Errorf("the trainer converts %d gold into 1 XP", rate)
		}
		if s.Gold < a.Gold {
			return Rewards{}, fmt.Errorf("you have %d gold", s.Gold)
		}
		s.Gold -= a.Gold
		xp := a.Gold / rate
		s.XPEarned += xp
		s.log("Trained: %d gold for %d XP.", a.Gold, xp)
		return Rewards{XP: xp}, nil
	}
	return Rewards{}, fmt.Errorf("unknown site service %q", a.Service)
}

// DeckChangeCost is the Influence that opens the deck for changes where the
// player is (paid once per visit: at camp until camp breaks, at a site until
// the turn ends), or an error when deck changes are not possible here and now.
func (s *State) DeckChangeCost(r Rules) (int, string, error) {
	if s.Result != "" || s.PendingBattle != nil {
		return 0, "", errors.New("deck changes wait until the expedition is in a turn")
	}
	if s.Phase == PhaseCamp {
		return r.Config.DeckChangeInfluence["camp"], "camp", nil
	}
	if s.Phase != PhaseTurn {
		return 0, "", errors.New("deck changes happen at a site or at camp")
	}
	st, def, closed := s.siteHere(r)
	if st == nil || closed != "" {
		if closed == "" {
			closed = "deck changes happen at a site or at camp"
		}
		return 0, "", errors.New(closed)
	}
	tier := def.DeckChangeTier()
	if tier == "" {
		return 0, "", fmt.Errorf("%s does not change decks", def.Name)
	}
	if s.Turn.InfluenceSite != st.ID {
		s.Turn.Influence, s.Turn.InfluenceSite = 0, st.ID
	}
	return r.Config.DeckChangeInfluence[tier], tier, nil
}

// deckVisit names the current deck-change visit: the camp, or the site the
// player stands on.
func (s *State) deckVisit(r Rules) string {
	if s.Phase == PhaseCamp {
		return "camp"
	}
	if st, _, _ := s.siteHere(r); st != nil {
		return st.ID
	}
	return ""
}

// DeckOpen reports whether this visit's deck-change fee is already paid.
func (s *State) DeckOpen(r Rules) bool {
	visit := s.deckVisit(r)
	return visit != "" && s.Turn.DeckOpen == visit
}

// PayDeckChange records one deck change (after the loadout layer has accepted
// the purchase). The first change of a visit spends the Influence that opens
// the deck; every further change there costs XP only.
func (s *State) PayDeckChange(r Rules) error {
	cost, where, err := s.DeckChangeCost(r)
	if err != nil {
		return err
	}
	if !s.DeckOpen(r) {
		if s.Turn.Influence < cost {
			return fmt.Errorf("opening your deck at the %s costs %d Influence; you have %d (play cards here for Influence first)", where, cost, s.Turn.Influence)
		}
		s.Turn.Influence -= cost
		s.Turn.DeckOpen = s.deckVisit(r)
		s.log("Deck opened at the %s for %d Influence: change it freely until you leave.", where, cost)
	}
	if s.Camp != nil {
		s.Camp.DeckChanges++
	}
	return nil
}

// ApplyDeckChange keeps the piles in step with the campaign sheet after a
// purchase: removed copies leave Spent first, then the draw pile, then the
// hand; replaced copies change in place (hand first); new copies go on top of
// the draw pile. before/after are the live-card multisets.
func (s *State) ApplyDeckChange(before, after map[string]int, replaced map[string]string) {
	for from, to := range replaced {
		n := before[from] - after[from]
		for i := 0; i < n && n > 0; i++ {
			done := false
			for _, pile := range []*[]string{&s.Hand, &s.Draw, &s.Spent} {
				for j, id := range *pile {
					if id == from {
						(*pile)[j] = to
						done = true
						break
					}
				}
				if done {
					break
				}
			}
		}
		before[from] -= n
		before[to] += n
	}
	for _, id := range sortedKeys(before) {
		for n := before[id] - after[id]; n > 0; n-- {
			var ok bool
			if s.Spent, ok = removeFirst(s.Spent, id); ok {
				continue
			}
			if s.Draw, ok = removeFirst(s.Draw, id); ok {
				continue
			}
			s.Hand, _ = removeFirst(s.Hand, id)
		}
	}
	for _, id := range sortedKeys(after) {
		for n := after[id] - before[id]; n > 0; n-- {
			s.Draw = append([]string{id}, s.Draw...)
		}
	}
}

// --- events -----------------------------------------------------------------

func (s *State) eventChoice(r Rules, a Action) (Rewards, error) {
	if err := s.requirePhase(PhaseEvent); err != nil {
		return Rewards{}, err
	}
	ev := r.Pack.Events[s.PendingEvent.Event]
	var choice *Choice
	for i := range ev.Choices {
		if ev.Choices[i].ID == a.Choice {
			choice = &ev.Choices[i]
		}
	}
	if choice == nil {
		return Rewards{}, fmt.Errorf("%s offers no choice %q", ev.Name, a.Choice)
	}
	st := s.site(s.PendingEvent.Site)
	if s.Turn.InfluenceSite != st.ID {
		s.Turn.Influence, s.Turn.InfluenceSite = 0, st.ID
	}
	// Cards offered become Influence at this site first.
	if len(a.Cards) > 0 {
		if _, err := s.checkPayment(r, a.Cards, 0); err != nil {
			return Rewards{}, err
		}
	}
	if s.Turn.Influence+len(a.Cards) < choice.Requires.Influence {
		return Rewards{}, fmt.Errorf("that choice needs %d Influence (cards played here); you have %d", choice.Requires.Influence, s.Turn.Influence+len(a.Cards))
	}
	if s.Gold < choice.Requires.Gold {
		return Rewards{}, fmt.Errorf("that choice needs %d gold; you have %d", choice.Requires.Gold, s.Gold)
	}
	if choice.Requires.Item != "" && s.Inventory[choice.Requires.Item] < 1 {
		return Rewards{}, fmt.Errorf("that choice needs a %s", r.Pack.Gear[choice.Requires.Item].Name)
	}
	for _, id := range a.Cards {
		s.Hand, _ = removeFirst(s.Hand, id)
		s.Spent = append(s.Spent, id)
		s.Turn.CardsUsed++
		s.Turn.Influence++
	}
	s.Turn.Influence -= choice.Requires.Influence
	s.Gold -= choice.Requires.Gold
	if choice.Requires.Item != "" {
		s.Inventory[choice.Requires.Item]--
	}
	result := &EventResult{Event: ev.ID, Site: s.PendingEvent.Site, Turn: s.Turn.Number, Round: s.Round, Choice: choice.ID}
	outcome := choice.Outcome
	if choice.Roll != nil {
		result.Roll, result.Needed = s.d6(), choice.Roll.Needed
		if result.Roll >= choice.Roll.Needed {
			outcome = &choice.Roll.Success
		} else {
			outcome = &choice.Roll.Failure
		}
	}
	result.Outcome, result.Text = *outcome, outcome.Text
	st.Resolved = true
	s.PendingEvent = nil
	s.Phase = PhaseTurn
	s.LastEvent = result
	rewards := s.applyOutcome(r, *outcome, ev.Name)
	s.log("%s: %s", ev.Name, outcome.Text)
	return rewards, nil
}

// applyOutcome grants gold, XP, items and statuses, inflicts wounds, mends,
// or starts a fight.
func (s *State) applyOutcome(r Rules, o Outcome, source string) Rewards {
	rewards := Rewards{}
	s.Gold += o.Gold
	if o.XP > 0 {
		s.XPEarned += o.XP
		rewards.XP = o.XP
	}
	if o.Item != "" {
		s.Inventory[o.Item]++
	}
	if o.Status != "" {
		s.addStatus(o.Status, 1)
	}
	if o.Mend > 0 {
		s.healPoints(o.Mend)
	}
	s.inflictWound(source, o.Wounds)
	if o.Fight != "" {
		def := r.Pack.Groups[o.Fight]
		g := GroupState{ID: "event/" + o.Fight + "/" + fmt.Sprint(s.Turn.Number), Group: o.Fight, Home: s.Player, Pos: s.Player}
		s.startBattle(r, battleStart{kind: "event", group: &g, groupDef: &def, origin: s.Turn.StartHex, position: s.Player, transient: true})
	}
	return rewards
}

func (s *State) addStatus(id string, stacks int) {
	for i := range s.Statuses {
		if s.Statuses[i].ID == id {
			s.Statuses[i].Stacks += stacks
			return
		}
	}
	s.Statuses = append(s.Statuses, StatusStack{ID: id, Stacks: stacks})
}

// inflictWound removes up to points live cards as one wound, discard first,
// then draw pile, then hand (the damage order), randomly within each pile;
// never the last live card. It reports the damage dealt.
func (s *State) inflictWound(source string, points int) int {
	var cards []string
	for len(cards) < points && s.LiveCards() > 1 {
		taken := false
		for _, pile := range []*[]string{&s.Spent, &s.Draw, &s.Hand} {
			if len(*pile) == 0 {
				continue
			}
			at := s.intn(len(*pile))
			card := (*pile)[at]
			*pile = append(append([]string(nil), (*pile)[:at]...), (*pile)[at+1:]...)
			cards = append(cards, card)
			taken = true
			break
		}
		if !taken {
			break
		}
	}
	s.addWound(cards, source, 0)
	return len(cards)
}

// --- fights -----------------------------------------------------------------

func (s *State) challenge(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if s.Turn.ActionUsed {
		return errors.New("you have already taken your action this turn")
	}
	if a.Group == "warlord" {
		if s.Warlord.Defeated {
			return errors.New("the Warlord is already beaten")
		}
		pos := s.Warlord.Pos()
		if _, _, placed := s.tileAt(r.Pack, pos); !placed || !Adjacent(s.Player, pos) {
			return errors.New("the Warlord is not next to you")
		}
		s.startBattle(r, battleStart{kind: "challenge", warlord: true, origin: s.Player, position: s.Player})
		return nil
	}
	g := s.group(a.Group)
	if g == nil || g.Defeated {
		return fmt.Errorf("there is no enemy group %q", a.Group)
	}
	if !Adjacent(s.Player, g.Pos) {
		return errors.New("that group is not next to you")
	}
	s.startBattle(r, battleStart{kind: "challenge", group: g, origin: s.Player, position: s.Player})
	return nil
}

func (s *State) enterSite(r Rules, a Action) error {
	if err := s.requirePhase(PhaseTurn); err != nil {
		return err
	}
	if s.Turn.ActionUsed {
		return errors.New("you have already taken your action this turn")
	}
	st := s.site(a.Site)
	if st == nil || !s.siteDefended(r.Pack, st) {
		return fmt.Errorf("%q is not a site with defenders", a.Site)
	}
	if !Adjacent(s.Player, st.Pos) {
		return errors.New("that site is not next to you")
	}
	def := r.Pack.Sites[st.ID]
	groupID := ""
	switch def.Type {
	case "keep", "tower":
		groupID = def.Garrison
	case "village", "town":
		groupID = def.OccupiedBy
	case "ruins", "grotto":
		groupID = def.Guardian.Day
		if s.Night {
			groupID = def.Guardian.Night
		}
	}
	gdef := r.Pack.Groups[groupID]
	g := GroupState{ID: st.ID + "/" + groupID, Group: groupID, Home: st.Pos, Pos: st.Pos}
	start := battleStart{kind: "guardian", group: &g, groupDef: &gdef, origin: s.Player, position: st.Pos, site: st.ID, transient: true}
	if def.Type != "ruins" && def.Type != "grotto" {
		start.kind, start.fortified = "garrison", true
		if a.Item != "" {
			gear, ok := r.Pack.Gear[a.Item]
			if !ok || gear.Effect != "remove_fortified" || s.Inventory[a.Item] < 1 {
				return fmt.Errorf("you have no %s to use", a.Item)
			}
			s.Inventory[a.Item]--
			start.fortified, start.usedSiege = false, true
		}
	}
	s.startBattle(r, start)
	return nil
}
