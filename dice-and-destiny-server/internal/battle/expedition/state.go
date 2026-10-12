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
	HandBefore   []string `json:"hand_before"`
	DrawBefore   []string `json:"draw_before"`
	SpentBefore  []string `json:"spent_before"`
	WoundsBefore []string `json:"wounds_before"`
	UsedSiege    bool     `json:"used_siege,omitempty"`
}

// BattleReport is what the last battle changed, for the result panel.
type BattleReport struct {
	BattleID     string   `json:"battle_id"`
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Result       string   `json:"result"`
	XP           int      `json:"xp"`
	Gold         int      `json:"gold"`
	EnergyIn     int      `json:"energy_in"`
	EnergyEnd    int      `json:"energy_end"`
	EnergyKept   int      `json:"energy_kept"`
	HandAfter    []string `json:"hand_after"`
	DrawAfter    int      `json:"draw_after"`
	SpentAfter   int      `json:"spent_after"`
	Wounds       int      `json:"wounds"`
	WoundsBefore int      `json:"wounds_before"`
	DefeatWounds int      `json:"defeat_wounds,omitempty"`
	PushedBack   *Hex     `json:"pushed_back,omitempty"`
	ForcedRest   bool     `json:"forced_rest,omitempty"`
	RegionWon    bool     `json:"region_won,omitempty"`
	RunOver      bool     `json:"run_over,omitempty"`
	EarlyXP      int      `json:"early_xp,omitempty"`
	RelicChoices int      `json:"relic_choices,omitempty"`
	WoundsMended int      `json:"wounds_mended,omitempty"`
	Lieutenant   bool     `json:"lieutenant,omitempty"`
	Loot         int      `json:"loot,omitempty"`
	SiteFreed    string   `json:"site_freed,omitempty"`
}

// MendRoll is one camp mending roll.
type MendRoll struct {
	Card    string `json:"card"`
	Roll    int    `json:"roll"`
	Bonus   int    `json:"bonus"`
	Needed  int    `json:"needed"`
	Success bool   `json:"success"`
	Paid    string `json:"paid"` // bandage or influence
}

// CampState is an open rest.
type CampState struct {
	Reason       string     `json:"reason"` // deck_empty, make_camp, defeat
	Inn          bool       `json:"inn,omitempty"`
	Site         string     `json:"site,omitempty"`
	Deadline     bool       `json:"deadline,omitempty"`
	Reshuffled   int        `json:"reshuffled"`
	EnergyBefore int        `json:"energy_before"`
	Forfeited    int        `json:"forfeited"`
	MendRolls    []MendRoll `json:"mend_rolls"`
	DeckChanges  int        `json:"deck_changes"`
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
	Deadline     bool      `json:"deadline,omitempty"`
	Statuses     []string  `json:"statuses_expired,omitempty"`
}

// EventResult is what the last event choice did.
type EventResult struct {
	Event   string  `json:"event"`
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
	Round       int    `json:"round"`
	Night       bool   `json:"night"`
	Phase       string `json:"phase"`

	Placed   []PlacedTile      `json:"placed"`
	Stack    []string          `json:"stack"`
	CoreDeal map[string]string `json:"core_deal"`
	Seen     map[string]bool   `json:"seen"`

	Player     Hex            `json:"player"`
	Hand       []string       `json:"hand"`
	Draw       []string       `json:"draw"`
	Spent      []string       `json:"spent"`
	Wounds     []string       `json:"wounds"`
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
	out := append([]string(nil), values...)
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
