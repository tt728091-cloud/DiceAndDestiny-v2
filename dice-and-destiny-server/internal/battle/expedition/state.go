package expedition

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Phases of an expedition save.
const (
	PhaseTurn    = "turn"    // the player is taking a turn
	PhaseEvent   = "event"   // an event scene waits for a choice
	PhaseCamp    = "camp"    // resting: camp services, then break camp
	PhaseBattle  = "battle"  // a battle is pending or running
	PhaseVictory = "victory" // the Warlord is beaten; the region is won
	PhaseDefeat  = "defeat"  // the Warlord won; the run is over
)

// StatusStack is one map status (hand symmetry: it reaches battles too).
type StatusStack struct {
	ID     string `json:"id"`
	Stacks int    `json:"stacks"`
}

// PlacedTile is a tile in a slot.
type PlacedTile struct {
	Slot   string `json:"slot"`
	Tile   string `json:"tile"`
	Center Hex    `json:"center"`
}

// GroupState is one enemy group on the map.
type GroupState struct {
	ID       string `json:"id"`    // instance: "<tile>/<group>"
	Group    string `json:"group"` // definition
	Home     Hex    `json:"home"`
	Pos      Hex    `json:"pos"`
	Patrol   int    `json:"patrol"`
	Defeated bool   `json:"defeated,omitempty"`
	Seen     bool   `json:"seen,omitempty"`
	LastSeen *Hex   `json:"last_seen,omitempty"`
}

// SiteState is one site on the map.
type SiteState struct {
	ID        string `json:"id"` // the site definition id (unique per region)
	Tile      string `json:"tile"`
	Pos       Hex    `json:"pos"`
	Seen      bool   `json:"seen,omitempty"`
	Freed     bool   `json:"freed,omitempty"`     // garrison or occupiers beaten
	Cleared   bool   `json:"cleared,omitempty"`   // ruins/grotto looted
	Resolved  bool   `json:"resolved,omitempty"`  // event played
	Conquered bool   `json:"conquered,omitempty"` // the Warlord took it
}

// WarlordState is the Warlord's agenda.
type WarlordState struct {
	Group     string `json:"group"`
	Route     []Hex  `json:"route"`
	Index     int    `json:"index"`
	Goal      Hex    `json:"goal"`
	GoalSite  string `json:"goal_site"`
	GoalTaken bool   `json:"goal_taken,omitempty"`
	Defeated  bool   `json:"defeated,omitempty"`
	Seen      bool   `json:"seen,omitempty"`
	LastSeen  *Hex   `json:"last_seen,omitempty"`
	// Arrived: at the deadline he comes to the player's hex.
	Arrived bool `json:"arrived,omitempty"`
}

// Pos is where the Warlord stands now.
func (w WarlordState) Pos() Hex {
	if len(w.Route) == 0 {
		return w.Goal
	}
	i := w.Index
	if i >= len(w.Route) {
		i = len(w.Route) - 1
	}
	return w.Route[i]
}

// TurnState is what has happened this turn.
type TurnState struct {
	Number        int    `json:"number"`
	StartHex      Hex    `json:"start_hex"`
	Move          int    `json:"move"`
	CardsUsed     int    `json:"cards_used"`
	Fought        bool   `json:"fought,omitempty"`
	ActionUsed    bool   `json:"action_used,omitempty"`
	Influence     int    `json:"influence"`
	InfluenceSite string `json:"influence_site,omitempty"`
	SightBonus    int    `json:"sight_bonus,omitempty"`
	ScoutRange    int    `json:"scout_range,omitempty"`
	IgnoreTerrain bool   `json:"ignore_terrain,omitempty"`
	IgnoreNight   bool   `json:"ignore_night,omitempty"`
	// Map card effects that last for the rest of this turn.
	CheapTerrain    string `json:"cheap_terrain,omitempty"`    // this terrain costs 1 Move
	CrossCost       int    `json:"cross_cost,omitempty"`       // Move to enter one lake or mountain hex (0: none)
	Crossed         bool   `json:"crossed,omitempty"`          // that hex has been entered
	CrossFrom       *Hex   `json:"cross_from,omitempty"`       // where a turn ending on it sends the player back
	CarryOver       bool   `json:"carry_over,omitempty"`       // leftover Move carries to the next turn
	ExploreDiscount int    `json:"explore_discount,omitempty"` // exploring costs this much less
	ExploreReach    int    `json:"explore_reach,omitempty"`    // empty slots this many hexes away can be explored
	Sneak           bool   `json:"sneak,omitempty"`            // walking past enemies provokes no fight
	HighGround      bool   `json:"high_ground,omitempty"`      // the next fight counts as from cover
	AmbushBonus     int    `json:"ambush_bonus,omitempty"`     // extra Ambush dice for that fight
	RevealTiles     bool   `json:"reveal_tiles,omitempty"`     // enemies on tiles in sight are revealed
	HaggleAt        string `json:"haggle_at,omitempty"`        // the visit ("camp" or a site) Haggle covers
	Haggle          int    `json:"haggle,omitempty"`           // Influence off each payment there (never below 1)
	// DeckOpen is where this visit's deck-change fee was paid ("camp" or a
	// site id): further changes there cost XP only until the visit ends.
	DeckOpen string `json:"deck_open,omitempty"`
}

// PendingEvent is an open event scene.
type PendingEvent struct {
	Site  string `json:"site"`
	Event string `json:"event"`
}

// PendingBattle is a fight the map has started and the battle session must
// run: everything the bridge needs, persisted so a relaunch restarts it.
type PendingBattle struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // challenge, provoked, garrison, guardian, event, deadline
	Target   string `json:"target"`
	Group    string `json:"group"`
	Opponent string `json:"opponent"`
	Count    int    `json:"count"`
	Name     string `json:"name"`
	Site     string `json:"site,omitempty"`
	Ambush   bool   `json:"ambush,omitempty"`
	// EnemyStatuses are applied to every enemy seat at battle start
	// (Fortified, Conqueror, Lieutenant marks and the Warlord's buffs).
	EnemyStatuses  []StatusStack `json:"enemy_statuses,omitempty"`
	PlayerStatuses []StatusStack `json:"player_statuses,omitempty"`
	EnergyIn       int           `json:"energy_in"`
	Origin         Hex           `json:"origin"`
	Position       Hex           `json:"position"`
	Seed           uint64        `json:"seed"`
	Warlord        bool          `json:"warlord,omitempty"`
	// Piles as they entered the battle, so a defeat can restore them.
	HandBefore   []string  `json:"hand_before"`
	DrawBefore   []string  `json:"draw_before"`
	SpentBefore  []string  `json:"spent_before"`
	WoundsBefore WoundList `json:"wounds_before"`
	UsedSiege    bool      `json:"used_siege,omitempty"`
	// The turn's fight flags before the battle, so Fall back can undo it.
	FoughtBefore bool `json:"fought_before,omitempty"`
	ActionBefore bool `json:"action_before,omitempty"`
}

// Mark is an enemy a Hunter's Mark lets the player follow: they see where it
// is, but not the map around it, until RestsLeft rests have passed. Target is
// "warlord", an open group's instance id, or "site:<id>" for a Lieutenant
// holding a site.
type Mark struct {
	Target    string `json:"target"`
	Name      string `json:"name"`
	RestsLeft int    `json:"rests_left"`
}

// PendingLook is a look effect waiting for the player to arrange the cards:
// Cards are the top of the draw pile, top first.
type PendingLook struct {
	Card   string   `json:"card"`
	Cards  []string `json:"cards"`
	Draw   int      `json:"draw"`
	Bottom bool     `json:"bottom,omitempty"`
}

// BattleReport is what the last battle changed, for the result panel.
type BattleReport struct {
	BattleID   string   `json:"battle_id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Result     string   `json:"result"`
	XP         int      `json:"xp"`
	Gold       int      `json:"gold"`
	EnergyIn   int      `json:"energy_in"`
	EnergyEnd  int      `json:"energy_end"`
	EnergyKept int      `json:"energy_kept"`
	HandAfter  []string `json:"hand_after"`
	DrawAfter  int      `json:"draw_after"`
	SpentAfter int      `json:"spent_after"`
	// Wounds counts wound groupings; WoundCards the cards they hold (health
	// lost). NewWounds are the groupings taken in this battle.
	Wounds       int `json:"wounds"`
	WoundCards   int `json:"wound_cards"`
	WoundsBefore int `json:"wounds_before"`
	NewWounds    int `json:"new_wounds,omitempty"`
	DefeatWounds int `json:"defeat_wounds,omitempty"`
	// HeldBack counts the cards the bridge moved to a worse pile because the
	// battle may not leave the player better off than they went in.
	HeldBack     int    `json:"held_back,omitempty"`
	PushedBack   *Hex   `json:"pushed_back,omitempty"`
	ForcedRest   bool   `json:"forced_rest,omitempty"`
	RegionWon    bool   `json:"region_won,omitempty"`
	RunOver      bool   `json:"run_over,omitempty"`
	EarlyXP      int    `json:"early_xp,omitempty"`
	RelicChoices int    `json:"relic_choices,omitempty"`
	WoundsMended int    `json:"wounds_mended,omitempty"`
	Lieutenant   bool   `json:"lieutenant,omitempty"`
	Loot         int    `json:"loot,omitempty"`
	SiteFreed    string `json:"site_freed,omitempty"`
}

// HealRoll is one camp healing roll on one wound: basic aid (every
// character's, once per untreated wound per camp) or a bandage (once per
// treated wound per camp). Healed cards leave the wound and return to the
// draw pile; a wound healed to zero closes.
type HealRoll struct {
	Wound     string   `json:"wound"`
	Source    string   `json:"source"`
	Method    string   `json:"method"` // basic_aid, bandage
	Item      string   `json:"item,omitempty"`
	Die       int      `json:"die"`
	Roll      int      `json:"roll"`
	Bonus     int      `json:"bonus"`
	Healed    int      `json:"healed"`
	Cards     []string `json:"cards"`
	Remaining int      `json:"remaining"`
	Closed    bool     `json:"closed,omitempty"`
}

// CampState is an open rest.
type CampState struct {
	Reason       string     `json:"reason"` // deck_empty, make_camp, defeat
	Turn         int        `json:"turn"`   // the turn that ended with this rest
	Round        int        `json:"round"`  // the round this rest ends
	Inn          bool       `json:"inn,omitempty"`
	Site         string     `json:"site,omitempty"`
	Deadline     bool       `json:"deadline,omitempty"`
	Reshuffled   int        `json:"reshuffled"`
	EnergyBefore int        `json:"energy_before"`
	Forfeited    int        `json:"forfeited"`
	Drew         int        `json:"drew"`       // the fresh hand drawn at the rest
	HandDrawn    bool       `json:"hand_drawn"` // false only in camps saved before the hand was drawn at the rest
	HandSize     int        `json:"hand_size"`  // the hand size the rest drew to (map statuses still counted)
	Heals        []HealRoll `json:"heals"`
	Healed       int        `json:"healed"`
	DeckChanges  int        `json:"deck_changes"`
	AidBonus     int        `json:"aid_bonus,omitempty"` // Field Medic: added to every basic-aid roll
}

// RestSummary is shown after breaking camp.
type RestSummary struct {
	Camp         CampState `json:"camp"`
	WarlordFrom  Hex       `json:"warlord_from"`
	WarlordTo    Hex       `json:"warlord_to"`
	WarlordSteps int       `json:"warlord_steps"`
	GoalTaken    bool      `json:"goal_taken,omitempty"`
	Patrols      []string  `json:"patrols"`
	RoundAfter   int       `json:"round_after"`
	NightAfter   bool      `json:"night_after"`
	Drew         int       `json:"drew"`
	Healed       int       `json:"healed"`
	Deadline     bool      `json:"deadline,omitempty"`
	Statuses     []string  `json:"statuses_expired,omitempty"`
}

// EventResult is what the last event choice did.
type EventResult struct {
	Event   string  `json:"event"`
	Site    string  `json:"site"`
	Turn    int     `json:"turn"`
	Round   int     `json:"round"`
	Choice  string  `json:"choice"`
	Text    string  `json:"text"`
	Roll    int     `json:"roll,omitempty"`
	Needed  int     `json:"needed,omitempty"`
	Outcome Outcome `json:"outcome"`
}

// State is the whole expedition, stored inside the campaign save. Everything
// the viewer must not see (the tile stack, the core deal, unseen contents,
// the RNG position) lives here and is filtered out by View.
type State struct {
	Version     int    `json:"version"`
	Region      string `json:"region"`
	Seed        uint64 `json:"seed"`
	RNGPosition uint64 `json:"rng_position"`
	Phase       string `json:"phase"`

	// Clock is world time, kept apart from the hero's piles; RegionStart is
	// the clock half at which this region's first round began. The round and
	// Day/Night are derived from them (clock.go), never stored.
	Clock       WorldClock `json:"clock"`
	RegionStart int        `json:"region_start"`
	// LegacyRound and LegacyNight are the round and Day/Night that saves
	// written before the world clock stored. They are only read: the first
	// rules call migrates them onto the clock and clears them.
	LegacyRound int  `json:"round,omitempty"`
	LegacyNight bool `json:"night,omitempty"`

	Placed   []PlacedTile      `json:"placed"`
	Stack    []string          `json:"stack"`
	CoreDeal map[string]string `json:"core_deal"`
	Seen     map[string]bool   `json:"seen"`

	Player     Hex            `json:"player"`
	Hand       []string       `json:"hand"`
	Draw       []string       `json:"draw"`
	Spent      []string       `json:"spent"`
	Wounds     WoundList      `json:"wounds"`
	WoundSeq   int            `json:"wound_seq,omitempty"`
	Energy     int            `json:"energy"`
	BaseEnergy int            `json:"base_energy"`
	Gold       int            `json:"gold"`
	Inventory  map[string]int `json:"inventory"`
	Statuses   []StatusStack  `json:"statuses"`

	Groups  []GroupState `json:"groups"`
	Sites   []SiteState  `json:"sites"`
	Warlord WarlordState `json:"warlord"`

	Turn          TurnState      `json:"turn"`
	Camp          *CampState     `json:"camp,omitempty"`
	PendingEvent  *PendingEvent  `json:"pending_event,omitempty"`
	PendingBattle *PendingBattle `json:"pending_battle,omitempty"`
	LastBattle    *BattleReport  `json:"last_battle,omitempty"`
	LastRest      *RestSummary   `json:"last_rest,omitempty"`
	LastEvent     *EventResult   `json:"last_event,omitempty"`
	PendingLook   *PendingLook   `json:"pending_look,omitempty"`

	// Marks are the enemies a Hunter's Mark follows; ExtraRounds the rounds
	// added to the deadline (TimeExtended once one has been added this quest);
	// HiddenUntil the last turn the Hidden status lasts.
	Marks        []Mark `json:"marks,omitempty"`
	ExtraRounds  int    `json:"extra_rounds,omitempty"`
	TimeExtended bool   `json:"time_extended,omitempty"`
	HiddenUntil  int    `json:"hidden_until,omitempty"`

	Result              string   `json:"result,omitempty"`
	PendingRelicChoices int      `json:"pending_relic_choices,omitempty"`
	XPEarned            int      `json:"xp_earned"`
	Log                 []string `json:"log"`
}

const maxLog = 40

func (s *State) log(format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
	if len(s.Log) > maxLog {
		s.Log = s.Log[len(s.Log)-maxLog:]
	}
}

// Clone deep-copies the state through JSON.
func (s *State) Clone() *State {
	raw, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(raw, &out)
	return &out
}

// LiveCards is the player's health: hand, draw pile and Spent.
func (s *State) LiveCards() int { return len(s.Hand) + len(s.Draw) + len(s.Spent) }

func hexKey(h Hex) string { return h.String() }

// --- wounds -----------------------------------------------------------------

// Wound is one grouping of damage: the cards one damage source removed
// together (a battle hit, a defeat, an event). It holds as many cards as
// health it still costs; healing takes cards out of it and a wound with no
// cards left is gone. Treated and Bandaged are this camp's healing attempts
// and reset at the next rest.
type Wound struct {
	ID          string   `json:"id"`
	Cards       []string `json:"cards"`
	Size        int      `json:"size"` // the damage when taken
	Source      string   `json:"source"`
	Round       int      `json:"round"`
	Turn        int      `json:"turn"`
	BattleRound int      `json:"battle_round,omitempty"`
	Treated     bool     `json:"treated,omitempty"`
	Bandaged    bool     `json:"bandaged,omitempty"`
}

// Damage is the health the wound still costs.
func (w Wound) Damage() int { return len(w.Cards) }

// WoundList is the player's wounds, oldest first. Saves written before wounds
// were grouped hold a flat list of card ids; it loads as one wound.
type WoundList []Wound

func (l *WoundList) UnmarshalJSON(data []byte) error {
	var groups []Wound
	if err := json.Unmarshal(data, &groups); err == nil {
		*l = groups
		return nil
	}
	var flat []string
	if err := json.Unmarshal(data, &flat); err != nil {
		return fmt.Errorf("wounds must be a list of wounds or of card ids: %w", err)
	}
	if len(flat) == 0 {
		*l = nil
		return nil
	}
	*l = WoundList{{ID: "w0", Cards: flat, Size: len(flat), Source: "earlier wounds"}}
	return nil
}

// Cards lists every wounded card, oldest wound first.
func (l WoundList) Cards() []string {
	out := []string{}
	for _, w := range l {
		out = append(out, w.Cards...)
	}
	return out
}

// Damage is the health all wounds cost together.
func (l WoundList) Damage() int { return len(l.Cards()) }

// Clone deep-copies the list.
func (l WoundList) Clone() WoundList {
	out := make(WoundList, 0, len(l))
	for _, w := range l {
		w.Cards = append([]string{}, w.Cards...)
		out = append(out, w)
	}
	return out
}

func (l WoundList) find(id string) *Wound {
	for i := range l {
		if l[i].ID == id {
			return &l[i]
		}
	}
	return nil
}

// addWound records one grouping of damage holding cards (already taken out
// of the live piles) and returns it.
func (s *State) addWound(cards []string, source string, battleRound int) *Wound {
	if len(cards) == 0 {
		return nil
	}
	s.WoundSeq++
	s.Wounds = append(s.Wounds, Wound{ID: fmt.Sprintf("w%d", s.WoundSeq), Cards: append([]string{}, cards...), Size: len(cards), Source: source, Round: s.Round(), Turn: s.Turn.Number, BattleRound: battleRound})
	return &s.Wounds[len(s.Wounds)-1]
}

// dropClosedWounds forgets wounds with no cards left.
func (s *State) dropClosedWounds() {
	kept := s.Wounds[:0]
	for _, w := range s.Wounds {
		if len(w.Cards) > 0 {
			kept = append(kept, w)
		}
	}
	if len(kept) == 0 {
		s.Wounds = nil
		return
	}
	s.Wounds = kept
}

// --- lookups --------------------------------------------------------------

func (s *State) tileAt(pack Pack, h Hex) (PlacedTile, TileHex, bool) {
	for _, p := range s.Placed {
		for _, th := range pack.Tiles[p.Tile].Hexes {
			off, _ := hexFromPair(th.Offset)
			if p.Center.Add(off) == h {
				return p, th, true
			}
		}
	}
	return PlacedTile{}, TileHex{}, false
}

func (s *State) placedHexes(pack Pack) []Hex {
	var out []Hex
	for _, p := range s.Placed {
		for _, h := range TileHexes(p.Center) {
			out = append(out, h)
		}
	}
	return out
}

func (s *State) slotPlaced(slot string) bool {
	for _, p := range s.Placed {
		if p.Slot == slot {
			return true
		}
	}
	return false
}

func (s *State) siteAt(h Hex) *SiteState {
	for i := range s.Sites {
		if s.Sites[i].Pos == h {
			return &s.Sites[i]
		}
	}
	return nil
}

func (s *State) site(id string) *SiteState {
	for i := range s.Sites {
		if s.Sites[i].ID == id {
			return &s.Sites[i]
		}
	}
	return nil
}

func (s *State) group(id string) *GroupState {
	for i := range s.Groups {
		if s.Groups[i].ID == id {
			return &s.Groups[i]
		}
	}
	return nil
}

// openGroupAt returns the living open group standing on h.
func (s *State) openGroupAt(h Hex) *GroupState {
	for i := range s.Groups {
		if !s.Groups[i].Defeated && s.Groups[i].Pos == h {
			return &s.Groups[i]
		}
	}
	return nil
}

func (s *State) warlordAt(h Hex) bool {
	return !s.Warlord.Defeated && s.Warlord.Pos() == h
}

// siteDefended reports a site whose defenders must be fought to enter.
func (s *State) siteDefended(pack Pack, st *SiteState) bool {
	if st == nil || st.Conquered {
		return false
	}
	def := pack.Sites[st.ID]
	switch def.Type {
	case "keep", "tower":
		return def.Garrison != "" && !st.Freed
	case "village", "town":
		return def.OccupiedBy != "" && !st.Freed
	case "ruins", "grotto":
		return def.Guardian != nil && !st.Cleared
	}
	return false
}

// hexBlocked reports a hex the player cannot simply walk onto.
func (s *State) hexBlocked(pack Pack, h Hex) (string, bool) {
	if g := s.openGroupAt(h); g != nil {
		return "an enemy group stands there; challenge it instead", true
	}
	if s.warlordAt(h) && !s.Warlord.Arrived {
		return "the Warlord stands there; challenge him instead", true
	}
	if st := s.siteAt(h); st != nil && s.siteDefended(pack, st) {
		return "its defenders must be fought first; enter the site instead", true
	}
	return "", false
}

// livingLieutenants counts the Lieutenants still alive: roaming ones on the
// map and ones holding a keep, tower or occupied site that is not yet freed.
func (s *State) livingLieutenants(pack Pack) int {
	n := 0
	for _, g := range s.Groups {
		if !g.Defeated && pack.Groups[g.Group].Lieutenant {
			n++
		}
	}
	for _, st := range s.Sites {
		def := pack.Sites[st.ID]
		holder := def.Garrison
		if holder == "" {
			holder = def.OccupiedBy
		}
		if holder != "" && !st.Freed && !st.Conquered && pack.Groups[holder].Lieutenant {
			n++
		}
	}
	return n
}

// handSize and handLimit apply map statuses (hand symmetry).
func (s *State) handDelta(cards map[string]int) int {
	delta := 0
	for _, st := range s.Statuses {
		delta += cards[st.ID] * st.Stacks
	}
	return delta
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func removeFirst(list []string, value string) ([]string, bool) {
	for i, v := range list {
		if v == value {
			return append(append([]string(nil), list[:i]...), list[i+1:]...), true
		}
	}
	return list, false
}

func countOf(list []string, value string) int {
	n := 0
	for _, v := range list {
		if v == value {
			n++
		}
	}
	return n
}
