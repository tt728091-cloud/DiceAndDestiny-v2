package expedition

import (
	"fmt"
	"sort"
	"strings"
)

// The view is everything the client may know. It never contains the tile
// stack, the core deal, the RNG position, or the contents of hexes the
// player has not seen; the tests assert that by scanning the JSON.

type ViewCard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MapEffect   string `json:"map_effect,omitempty"`
	MapOnly     bool   `json:"map_only,omitempty"`
	MapPlayable bool   `json:"map_playable"`
	MapReason   string `json:"map_reason,omitempty"`
	EnergyCost  int    `json:"energy_cost,omitempty"`
	// MapKind is the map effect's kind; Targets what it must be played on
	// (terrains, enemies to mark or sites to lure from), chosen by the player.
	MapKind string      `json:"map_kind,omitempty"`
	Targets []MapTarget `json:"targets,omitempty"`
}

// ViewLook is the top of the draw pile a look effect shows the player.
type ViewLook struct {
	Card     string     `json:"card"`
	CardName string     `json:"card_name"`
	Cards    []ViewCard `json:"cards"` // top first
	Draw     int        `json:"draw"`
	Bottom   bool       `json:"bottom,omitempty"`
}

// ViewMark is an enemy a Hunter's Mark follows: where it is now.
type ViewMark struct {
	Target    string `json:"target"`
	Name      string `json:"name"`
	RestsLeft int    `json:"rests_left"`
	Hex       *Hex   `json:"hex,omitempty"`
	OffMap    bool   `json:"off_map,omitempty"` // on a part of the region not explored yet
}

type ViewStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Stacks int    `json:"stacks"`
	Rules  string `json:"rules"`
}

type ViewItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Description string `json:"description"`
}

type ViewService struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Item      string `json:"item,omitempty"`
	Influence int    `json:"influence,omitempty"`
	Gold      int    `json:"gold,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type ViewSite struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Description   string        `json:"description"`
	Defended      bool          `json:"defended,omitempty"`
	DefenderName  string        `json:"defender_name,omitempty"`
	Fortified     bool          `json:"fortified,omitempty"`
	Freed         bool          `json:"freed,omitempty"`
	Cleared       bool          `json:"cleared,omitempty"`
	Resolved      bool          `json:"resolved,omitempty"`
	Conquered     bool          `json:"conquered,omitempty"`
	Cover         bool          `json:"cover,omitempty"`
	Goal          bool          `json:"goal,omitempty"`
	Open          bool          `json:"open"`
	Closed        string        `json:"closed,omitempty"`
	Services      []ViewService `json:"services,omitempty"`
	DeckChange    int           `json:"deck_change,omitempty"` // Influence that opens the deck for this visit
	HasDeckChange bool          `json:"has_deck_change,omitempty"`
	DeckOpen      bool          `json:"deck_open,omitempty"` // the fee is paid; changes cost XP only until the turn ends
}

type ViewGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Combatant   string `json:"combatant"`
	Count       int    `json:"count"`
	Habit       string `json:"habit"`
	HabitText   string `json:"habit_text"`
	XP          int    `json:"xp"`
	Gold        int    `json:"gold"`
	Lieutenant  bool   `json:"lieutenant,omitempty"`
	Roams       bool   `json:"roams,omitempty"`
	InSight     bool   `json:"in_sight"`
	Remembered  bool   `json:"remembered,omitempty"`
	Ambushable  bool   `json:"ambushable"`
}

type ViewWarlord struct {
	Name         string `json:"name"`
	Known        bool   `json:"known"`
	InSight      bool   `json:"in_sight,omitempty"`
	LastSeen     *Hex   `json:"last_seen,omitempty"`
	NextStep     *Hex   `json:"next_step,omitempty"`
	GoalSite     string `json:"goal_site"`
	GoalName     string `json:"goal_name"`
	Goal         *Hex   `json:"goal,omitempty"`
	GoalTaken    bool   `json:"goal_taken,omitempty"`
	Fortified    bool   `json:"fortified,omitempty"`
	Conqueror    bool   `json:"conqueror,omitempty"`
	Lieutenants  int    `json:"lieutenants"`
	Habit        string `json:"habit"`
	HabitText    string `json:"habit_text"`
	XP           int    `json:"xp"`
	Gold         int    `json:"gold"`
	Defeated     bool   `json:"defeated,omitempty"`
	MarchPerRest int    `json:"march_per_rest"`
	// Marked: a Hunter's Mark shows where he is (Position), even off the map.
	Marked   bool `json:"marked,omitempty"`
	Position *Hex `json:"position,omitempty"`
}

type ViewHex struct {
	Q        int        `json:"q"`
	R        int        `json:"r"`
	Tile     string     `json:"tile"`
	TileName string     `json:"tile_name"`
	Terrain  string     `json:"terrain"`
	Cost     int        `json:"cost"`
	Passable bool       `json:"passable"`
	Cover    bool       `json:"cover,omitempty"`
	InSight  bool       `json:"in_sight"`
	Seen     bool       `json:"seen"`
	Player   bool       `json:"player,omitempty"`
	Site     *ViewSite  `json:"site,omitempty"`
	Group    *ViewGroup `json:"group,omitempty"`
	Warlord  bool       `json:"warlord,omitempty"`
}

type ViewSlot struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Center   Hex    `json:"center"`
	Placed   bool   `json:"placed"`
	Adjacent bool   `json:"adjacent,omitempty"`
	Ready    bool   `json:"ready,omitempty"`
	Why      string `json:"why,omitempty"`
}

type ViewMove struct {
	To       Hex    `json:"to"`
	Cost     int    `json:"cost"`
	Terrain  string `json:"terrain"`
	Provokes string `json:"provokes,omitempty"`
	Blocked  string `json:"blocked,omitempty"`
	Event    bool   `json:"event,omitempty"`
}

type ViewTarget struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Hex       Hex    `json:"hex"`
	Ambush    bool   `json:"ambush,omitempty"`
	Fortified bool   `json:"fortified,omitempty"`
	Siege     bool   `json:"siege,omitempty"`
	Warlord   bool   `json:"warlord,omitempty"`
}

type ViewChoice struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Influence int    `json:"influence,omitempty"`
	Gold      int    `json:"gold,omitempty"`
	Item      string `json:"item,omitempty"`
	Roll      int    `json:"roll,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type ViewEvent struct {
	Site    string       `json:"site"`
	Event   string       `json:"event"`
	Name    string       `json:"name"`
	Text    string       `json:"text"`
	Choices []ViewChoice `json:"choices"`
}

type ViewCamp struct {
	Reason        string     `json:"reason"`
	Turn          int        `json:"turn"`
	Round         int        `json:"round"`
	Inn           bool       `json:"inn,omitempty"`
	Site          string     `json:"site,omitempty"`
	Deadline      bool       `json:"deadline,omitempty"`
	Reshuffled    int        `json:"reshuffled"`
	EnergyBefore  int        `json:"energy_before"`
	Forfeited     int        `json:"forfeited"`
	Drew          int        `json:"drew"`
	Heals         []HealRoll `json:"heals"`
	Healed        int        `json:"healed"`
	BasicAidDie   int        `json:"basic_aid_die"`
	BandageDie    int        `json:"bandage_die"`
	InnBonus      int        `json:"inn_bonus"`
	AidBonus      int        `json:"aid_bonus,omitempty"` // Field Medic: added to basic-aid rolls
	SpareBandages int        `json:"spare_bandages,omitempty"`
	Bandages      int        `json:"bandages"`
	Untreated     int        `json:"untreated"`   // wounds basic aid can still treat
	Bandageable   int        `json:"bandageable"` // treated wounds a bandage can still heal
	CanBasicAid   bool       `json:"can_basic_aid"`
	CanBandage    bool       `json:"can_bandage"`
	HealReason    string     `json:"heal_reason,omitempty"`
	DeckChange    int        `json:"deck_change"` // Influence that opens the deck until camp breaks
	DeckOpen      bool       `json:"deck_open"`   // the fee is paid; changes cost XP only until camp breaks
}

// ViewWound is one grouping of damage on the stat strip and at camp.
type ViewWound struct {
	ID       string   `json:"id"`
	Damage   int      `json:"damage"` // cards still lost
	Size     int      `json:"size"`   // the damage when taken
	Cards    []string `json:"cards"`
	Source   string   `json:"source"`
	Taken    string   `json:"taken"`
	Treated  bool     `json:"treated"`
	Bandaged bool     `json:"bandaged"`
	BasicAid bool     `json:"basic_aid"` // basic aid can be used on it now
	Bandage  bool     `json:"bandage"`   // a bandage can be used on it now
	Reason   string   `json:"reason,omitempty"`
}

type ViewPendingBattle struct {
	ID       string       `json:"id"`
	Kind     string       `json:"kind"`
	Name     string       `json:"name"`
	Opponent string       `json:"opponent"`
	Count    int          `json:"count"`
	Ambush   bool         `json:"ambush,omitempty"`
	Warlord  bool         `json:"warlord,omitempty"`
	Enemy    []ViewStatus `json:"enemy_statuses"`
	Player   []ViewStatus `json:"player_statuses"`
	EnergyIn int          `json:"energy_in"`
}

type ViewTurn struct {
	Number        int    `json:"number"`
	Move          int    `json:"move"`
	CardsUsed     int    `json:"cards_used"`
	Fought        bool   `json:"fought,omitempty"`
	ActionUsed    bool   `json:"action_used,omitempty"`
	Influence     int    `json:"influence"`
	InfluenceSite string `json:"influence_site,omitempty"`
	Sight         int    `json:"sight"`
	CanEndTurn    bool   `json:"can_end_turn"`
	EndTurnReason string `json:"end_turn_reason,omitempty"`
	CanMakeCamp   bool   `json:"can_make_camp"`
	IgnoreTerrain bool   `json:"ignore_terrain,omitempty"`
	IgnoreNight   bool   `json:"ignore_night,omitempty"`
	ExploreCost   int    `json:"explore_cost"`
	// Effects lists this turn's map card effects for the HUD.
	Effects []string `json:"effects"`
}

// ViewClock is the world clock as the map shows it: the month, the season
// with its tooltip rules and effects, Day or Night, and the label.
type ViewClock struct {
	Year          int      `json:"year"`
	Month         string   `json:"month"`
	MonthName     string   `json:"month_name"`
	Season        string   `json:"season"`
	SeasonName    string   `json:"season_name"`
	SeasonRules   string   `json:"season_rules"`
	SeasonEffects []string `json:"season_effects"`
	Night         bool     `json:"night"`
	Label         string   `json:"label"`
}

func viewClock(cfg Config, c WorldClock) ViewClock {
	cal := cfg.Calendar
	month, season := c.Month(cal), c.Season(cal)
	// Seasonal effects arrive in Phase 4; the loader refuses any until then.
	return ViewClock{Year: c.Year(cal), Month: month.ID, MonthName: month.Name, Season: season.ID, SeasonName: season.Name, SeasonRules: season.RulesText,
		SeasonEffects: []string{}, Night: c.Night(), Label: c.Label(cal)}
}

type View struct {
	Region      string    `json:"region"`
	RegionName  string    `json:"region_name"`
	Description string    `json:"description"`
	Round       int       `json:"round"`
	Rounds      int       `json:"rounds"`
	Night       bool      `json:"night"`
	DayLabel    string    `json:"day_label"`
	Clock       ViewClock `json:"clock"`
	Phase       string    `json:"phase"`
	Result      string    `json:"result,omitempty"`
	Revision    int       `json:"revision"`
	Seed        uint64    `json:"seed"`

	Player       Hex               `json:"player"`
	Energy       int               `json:"energy"`
	BaseEnergy   int               `json:"base_energy"`
	Gold         int               `json:"gold"`
	Hand         []ViewCard        `json:"hand"`
	HandSize     int               `json:"hand_size"`
	HandLimit    int               `json:"hand_limit"`
	Draw         []string          `json:"draw"`
	Spent        []string          `json:"spent"`
	Wounds       []ViewWound       `json:"wounds"`      // groupings of damage
	WoundCards   []string          `json:"wound_cards"` // every wounded card (the removed pile)
	CardNames    map[string]string `json:"card_names"`
	Inventory    []ViewItem        `json:"inventory"`
	Statuses     []ViewStatus      `json:"statuses"`
	Health       int               `json:"health"`
	MaxHealth    int               `json:"max_health"`
	XPEarned     int               `json:"xp_earned"`
	RelicChoices int               `json:"pending_relic_choices,omitempty"`

	Turn       ViewTurn           `json:"turn"`
	Hexes      []ViewHex          `json:"hexes"`
	Slots      []ViewSlot         `json:"slots"`
	Moves      []ViewMove         `json:"moves"`
	Explores   []ViewSlot         `json:"explores"`
	Challenges []ViewTarget       `json:"challenges"`
	Enters     []ViewTarget       `json:"enters"`
	SiteHere   *ViewSite          `json:"site_here,omitempty"`
	Warlord    ViewWarlord        `json:"warlord"`
	Event      *ViewEvent         `json:"event,omitempty"`
	Camp       *ViewCamp          `json:"camp,omitempty"`
	Battle     *ViewPendingBattle `json:"pending_battle,omitempty"`
	LastBattle *BattleReport      `json:"last_battle,omitempty"`
	LastRest   *RestSummary       `json:"last_rest,omitempty"`
	LastEvent  *EventResult       `json:"last_event,omitempty"`
	Look       *ViewLook          `json:"look,omitempty"`
	Marks      []ViewMark         `json:"marks"`
	Log        []string           `json:"log"`
	Config     ViewConfig         `json:"config"`
}

// ViewConfig repeats the rule numbers the HUD explains.
type ViewConfig struct {
	EnergyPerMove       int                      `json:"energy_per_move"`
	EnergyPerInfluence  int                      `json:"energy_per_influence"`
	ExploreCost         int                      `json:"explore_cost"`
	SightDay            int                      `json:"sight_day"`
	SightNight          int                      `json:"sight_night"`
	Terrain             map[string]TerrainConfig `json:"terrain"`
	DefeatWounds        int                      `json:"defeat_wounds"`
	DeckChangeInfluence map[string]int           `json:"deck_change_influence"`
}

var habitText = map[string]string{
	"unwary":       "Unwary: can be ambushed by day or night.",
	"sleeper":      "Sleeper: can be ambushed at night.",
	"night_hunter": "Night hunter: can be ambushed by day; roams at night.",
	"always_alert": "Always alert: never ambushed.",
}

func (s *State) viewStatus(r Rules, st StatusStack) ViewStatus {
	v := ViewStatus{ID: st.ID, Name: st.ID, Stacks: st.Stacks}
	if def, ok := r.Statuses[st.ID]; ok {
		v.Name, v.Rules = def.Name, def.Presentation.RulesText
	}
	return v
}

func (s *State) viewGroup(r Rules, g GroupState, inSight bool) *ViewGroup {
	def := r.Pack.Groups[g.Group]
	return &ViewGroup{ID: g.ID, Name: def.Name, Description: def.Description, Combatant: def.Combatant, Count: def.Count, Habit: def.Habit, HabitText: habitText[def.Habit], XP: def.XP, Gold: def.Gold, Lieutenant: def.Lieutenant, Roams: len(def.Patrol) > 0, InSight: inSight, Remembered: !inSight, Ambushable: s.ambushAllowed(r, "challenge", def.Habit, g.Pos)}
}

func (s *State) viewSite(r Rules, st SiteState) *ViewSite {
	def := r.Pack.Sites[st.ID]
	v := &ViewSite{ID: st.ID, Name: def.Name, Type: def.Type, Description: def.Description, Defended: s.siteDefended(r.Pack, &st), Freed: st.Freed, Cleared: st.Cleared, Resolved: st.Resolved, Conquered: st.Conquered, Cover: def.Cover, Goal: st.ID == s.Warlord.GoalSite}
	if v.Defended {
		gid := def.Garrison
		switch def.Type {
		case "village", "town":
			gid = def.OccupiedBy
		case "ruins", "grotto":
			gid = def.Guardian.Day
			if s.Night() {
				gid = def.Guardian.Night
			}
		}
		v.DefenderName = r.Pack.Groups[gid].Name
		v.Fortified = def.Type == "keep" || def.Type == "tower" || def.OccupiedBy != ""
	}
	here := s.Player == st.Pos
	if here {
		_, _, closed := s.siteHere(r)
		v.Open, v.Closed = closed == "", closed
		if v.Open {
			v.Services = s.siteServices(r, st, def)
			if tier := def.DeckChangeTier(); tier != "" {
				v.HasDeckChange, v.DeckChange, v.DeckOpen = true, s.influencePrice(r, r.Config.DeckChangeInfluence[tier]), s.Turn.DeckOpen == st.ID
			}
		}
	}
	return v
}

func (s *State) siteServices(r Rules, st SiteState, def Site) []ViewService {
	cfg := r.Config
	influence := s.Turn.Influence
	if s.Turn.InfluenceSite != st.ID {
		influence = 0
	}
	out := []ViewService{}
	switch def.Type {
	case "village", "monastery":
		per := cfg.Mending.VillageInfluencePerWound
		if def.Type == "monastery" {
			per = cfg.Mending.MonasteryInfluencePerWound
		}
		per = s.influencePrice(r, per)
		sv := ViewService{ID: "mend", Label: fmt.Sprintf("Mend 1 damage (%d Influence)", per), Influence: per, Available: len(s.Wounds) > 0 && influence >= per}
		if len(s.Wounds) == 0 {
			sv.Reason = "no wounds"
		} else if influence < per {
			sv.Reason = fmt.Sprintf("play %d more card(s) here for Influence", per-influence)
		}
		out = append(out, sv)
	case "town":
		price := cfg.Mending.TownGoldPerWound
		sv := ViewService{ID: "mend", Label: fmt.Sprintf("Healer: mend 1 damage (%d gold)", price), Gold: price, Available: len(s.Wounds) > 0 && s.Gold >= price}
		if len(s.Wounds) == 0 {
			sv.Reason = "no wounds"
		} else if s.Gold < price {
			sv.Reason = "not enough gold"
		}
		out = append(out, sv)
	}
	for _, item := range def.Shop {
		g := r.Pack.Gear[item]
		sv := ViewService{ID: "shop", Item: item, Label: fmt.Sprintf("Buy %s (%d gold)", g.Name, g.Price), Gold: g.Price, Available: s.Gold >= g.Price}
		if !sv.Available {
			sv.Reason = "not enough gold"
		}
		out = append(out, sv)
	}
	return out
}

// View builds the viewer-safe status.
func (s *State) View(r Rules, revision int) View {
	if s.LegacyRound > 0 {
		// A save from before the world clock: show it as it will be once the
		// next action migrates it, without changing the save here.
		migrated := s.Clone()
		migrated.migrateClock(r)
		return migrated.View(r, revision)
	}
	cfg, pack := r.Config, r.Pack
	region := pack.Regions[s.Region]
	v := View{Region: s.Region, RegionName: region.Name, Description: region.Description, Round: s.Round(), Rounds: s.rounds(r), Night: s.Night(), Phase: s.Phase, Result: s.Result, Revision: revision, Seed: s.Seed,
		Player: s.Player, Energy: s.Energy, BaseEnergy: s.BaseEnergy, Gold: s.Gold, HandSize: s.effectiveHandSize(r), HandLimit: s.effectiveHandLimit(r),
		Draw: sortedStrings(s.Draw), Spent: append([]string{}, s.Spent...), Wounds: s.viewWounds(r), WoundCards: s.Wounds.Cards(), CardNames: map[string]string{},
		Hand: []ViewCard{}, Inventory: []ViewItem{}, Statuses: []ViewStatus{}, Hexes: []ViewHex{}, Slots: []ViewSlot{}, Moves: []ViewMove{}, Explores: []ViewSlot{}, Challenges: []ViewTarget{}, Enters: []ViewTarget{}, Marks: []ViewMark{},
		Health: s.LiveCards(), MaxHealth: s.LiveCards() + s.Wounds.Damage(), XPEarned: s.XPEarned, RelicChoices: s.PendingRelicChoices,
		LastBattle: s.LastBattle, LastRest: s.viewLastRest(r), LastEvent: s.LastEvent, Log: append([]string{}, s.Log...),
		Config: ViewConfig{EnergyPerMove: cfg.Move.EnergyPerMove, EnergyPerInfluence: cfg.Influence.EnergyPerInfluence, ExploreCost: cfg.Move.ExploreCost, SightDay: cfg.Sight.Day, SightNight: cfg.Sight.Night, Terrain: cfg.Terrain, DefeatWounds: cfg.Defeat.Wounds, DeckChangeInfluence: cfg.DeckChangeInfluence}}
	v.Clock = viewClock(cfg, s.Clock)
	if s.Round() > s.rounds(r) {
		v.DayLabel = "The deadline"
	} else {
		v.DayLabel = fmt.Sprintf("%s · round %d of %d", v.Clock.Label, s.Round(), s.rounds(r))
	}
	for _, id := range append(append(append([]string{}, s.Hand...), s.Draw...), append(append([]string{}, s.Spent...), s.Wounds.Cards()...)...) {
		v.CardNames[id] = r.cardName(id)
	}
	for _, id := range s.Hand {
		card := ViewCard{ID: id, Name: r.cardName(id)}
		if def, ok := r.Cards[id]; ok && def.MapEffect != nil {
			card.MapEffect = def.MapEffect.Text
			card.MapOnly = def.Program == nil && def.Mechanic == nil
			card.EnergyCost = mapEnergyCost(def)
			card.MapKind = def.MapEffect.Kind
			ready, why := s.mapCardReady(r, def.MapEffect)
			card.MapPlayable = ready && s.Energy >= card.EnergyCost
			if !ready {
				card.MapReason = why
			} else if !card.MapPlayable {
				card.MapReason = fmt.Sprintf("needs %d energy; you have %d", card.EnergyCost, s.Energy)
			}
			if card.MapPlayable {
				card.Targets = s.mapCardTargets(r, def.MapEffect)
			}
		}
		v.Hand = append(v.Hand, card)
	}
	for _, id := range sortedKeys(s.Inventory) {
		if s.Inventory[id] > 0 {
			g := pack.Gear[id]
			v.Inventory = append(v.Inventory, ViewItem{ID: id, Name: g.Name, Count: s.Inventory[id], Description: g.Description})
		}
	}
	for _, st := range s.Statuses {
		v.Statuses = append(v.Statuses, s.viewStatus(r, st))
	}
	// Turn.
	canEnd, why := s.canEndTurn()
	v.Turn = ViewTurn{Number: s.Turn.Number, Move: s.Turn.Move, CardsUsed: s.Turn.CardsUsed, Fought: s.Turn.Fought, ActionUsed: s.Turn.ActionUsed, Influence: s.Turn.Influence, InfluenceSite: s.Turn.InfluenceSite, Sight: s.sightRange(r), CanEndTurn: s.Phase == PhaseTurn && canEnd && s.PendingLook == nil, EndTurnReason: why, CanMakeCamp: s.Phase == PhaseTurn && s.PendingLook == nil, IgnoreTerrain: s.Turn.IgnoreTerrain, IgnoreNight: s.Turn.IgnoreNight, ExploreCost: s.exploreCost(r), Effects: s.turnEffects(r)}
	// Hexes.
	seenWarlord := s.Warlord.Seen && !s.Warlord.Defeated
	warlordPos := s.Warlord.Pos()
	warlordInSight := seenWarlord && s.inSight(r, warlordPos)
	if _, _, placed := s.tileAt(pack, warlordPos); !placed && !s.Warlord.Arrived {
		warlordInSight = false
	}
	for _, p := range s.Placed {
		tile := pack.Tiles[p.Tile]
		for _, th := range tile.Hexes {
			off, _ := hexFromPair(th.Offset)
			h := p.Center.Add(off)
			cost, _, passable := s.moveCost(r, h)
			vh := ViewHex{Q: h.Q, R: h.R, Tile: p.Tile, TileName: tile.Name, Terrain: th.Terrain, Cost: cost, Passable: passable, Cover: cfg.Terrain[th.Terrain].Cover, InSight: s.inSight(r, h), Seen: s.Seen[hexKey(h)], Player: h == s.Player}
			if vh.Seen {
				if st := s.siteAt(h); st != nil && st.Seen {
					vh.Site = s.viewSite(r, *st)
				}
				for _, g := range s.Groups {
					if g.Defeated || !g.Seen {
						continue
					}
					if vh.InSight && g.Pos == h {
						vh.Group = s.viewGroup(r, g, true)
					} else if !vh.InSight && g.LastSeen != nil && *g.LastSeen == h {
						vh.Group = s.viewGroup(r, g, false)
					}
				}
				if warlordInSight && warlordPos == h {
					vh.Warlord = true
				} else if seenWarlord && !warlordInSight && s.Warlord.LastSeen != nil && *s.Warlord.LastSeen == h && !s.inSight(r, h) {
					vh.Warlord = true
				}
			}
			v.Hexes = append(v.Hexes, vh)
		}
	}
	sort.Slice(v.Hexes, func(i, j int) bool {
		if v.Hexes[i].R != v.Hexes[j].R {
			return v.Hexes[i].R < v.Hexes[j].R
		}
		return v.Hexes[i].Q < v.Hexes[j].Q
	})
	// Slots and explores.
	options := map[string]exploreOption{}
	for _, o := range s.exploreOptions(r) {
		options[o.Slot] = o
	}
	v.Slots = append(v.Slots, ViewSlot{ID: "start", Kind: "start", Center: Hex{}, Placed: true})
	for _, slot := range region.Slots {
		center, _ := hexFromPair(slot.Center)
		vs := ViewSlot{ID: slot.ID, Kind: slot.Kind, Center: center, Placed: s.slotPlaced(slot.ID)}
		if o, ok := options[slot.ID]; ok {
			vs.Adjacent, vs.Ready, vs.Why = true, o.Ready && s.Phase == PhaseTurn, o.Why
			v.Explores = append(v.Explores, vs)
		}
		v.Slots = append(v.Slots, vs)
	}
	// Moves.
	if s.Phase == PhaseTurn {
		for _, n := range Neighbors(s.Player) {
			_, th, placed := s.tileAt(pack, n)
			if !placed {
				continue
			}
			cost, why, ok := s.moveCost(r, n)
			m := ViewMove{To: n, Cost: cost, Terrain: th.Terrain}
			if !ok {
				m.Blocked = why
			} else if reason, blocked := s.hexBlocked(pack, n); blocked {
				m.Blocked = reason
			}
			if g := s.provokedBy(s.Player, n); g != nil && m.Blocked == "" {
				m.Provokes = pack.Groups[g.Group].Name
			}
			if st := s.siteAt(n); st != nil && pack.Sites[st.ID].Type == "event" && !st.Resolved {
				m.Event = true
			}
			v.Moves = append(v.Moves, m)
		}
		if !s.Turn.ActionUsed {
			for _, g := range s.Groups {
				if !g.Defeated && Adjacent(s.Player, g.Pos) {
					def := pack.Groups[g.Group]
					v.Challenges = append(v.Challenges, ViewTarget{ID: g.ID, Name: def.Name, Hex: g.Pos, Ambush: s.ambushAllowed(r, "challenge", def.Habit, g.Pos)})
				}
			}
			if !s.Warlord.Defeated && Adjacent(s.Player, warlordPos) {
				if _, _, placed := s.tileAt(pack, warlordPos); placed {
					def := pack.Groups[s.Warlord.Group]
					fortified := s.Warlord.GoalTaken
					if st := s.siteAt(warlordPos); st != nil && (pack.Sites[st.ID].Type == "keep" || pack.Sites[st.ID].Type == "tower") {
						fortified = true
					}
					v.Challenges = append(v.Challenges, ViewTarget{ID: "warlord", Name: def.Name, Hex: warlordPos, Warlord: true, Ambush: s.ambushAllowed(r, "challenge", def.Habit, warlordPos), Fortified: fortified})
				}
			}
			siege := false
			for id, n := range s.Inventory {
				if g, ok := pack.Gear[id]; ok && g.Effect == "remove_fortified" && n > 0 {
					siege = true
				}
			}
			for _, st := range s.Sites {
				if s.siteDefended(pack, &st) && Adjacent(s.Player, st.Pos) && st.Seen {
					def := pack.Sites[st.ID]
					vs := s.viewSite(r, st)
					t := ViewTarget{ID: st.ID, Name: def.Name, Hex: st.Pos, Fortified: vs.Fortified, Siege: vs.Fortified && siege}
					gid := def.Garrison
					switch def.Type {
					case "village", "town":
						gid = def.OccupiedBy
					case "ruins", "grotto":
						gid = def.Guardian.Day
						if s.Night() {
							gid = def.Guardian.Night
						}
					}
					t.Ambush = s.ambushAllowed(r, "garrison", pack.Groups[gid].Habit, st.Pos)
					v.Enters = append(v.Enters, t)
				}
			}
		}
		if st := s.siteAt(s.Player); st != nil {
			v.SiteHere = s.viewSite(r, *st)
		}
	}
	// Marks.
	for _, m := range s.Marks {
		vm := ViewMark{Target: m.Target, Name: m.Name, RestsLeft: m.RestsLeft}
		var at Hex
		switch {
		case m.Target == "warlord":
			at = s.Warlord.Pos()
		case strings.HasPrefix(m.Target, "site:"):
			if st := s.site(strings.TrimPrefix(m.Target, "site:")); st != nil {
				at = st.Pos
			}
		default:
			if g := s.group(m.Target); g != nil {
				at = g.Pos
			}
		}
		vm.Hex = &at
		if _, _, placed := s.tileAt(pack, at); !placed {
			vm.OffMap = true
		}
		v.Marks = append(v.Marks, vm)
	}
	// The Warlord.
	wdef := pack.Groups[s.Warlord.Group]
	v.Warlord = ViewWarlord{Name: wdef.Name, Known: seenWarlord, InSight: warlordInSight, GoalSite: s.Warlord.GoalSite, GoalName: pack.Sites[s.Warlord.GoalSite].Name, GoalTaken: s.Warlord.GoalTaken, Lieutenants: s.livingLieutenants(pack), Habit: wdef.Habit, HabitText: habitText[wdef.Habit], XP: wdef.XP, Gold: wdef.Gold, Defeated: s.Warlord.Defeated, MarchPerRest: cfg.Warlord.MarchPerRest, Conqueror: s.Warlord.GoalTaken}
	if s.marked("warlord") && !s.Warlord.Defeated {
		pos := warlordPos
		v.Warlord.Marked, v.Warlord.Position = true, &pos
	}
	if seenWarlord {
		v.Warlord.LastSeen = s.Warlord.LastSeen
		if warlordInSight && s.Warlord.Index < len(s.Warlord.Route)-1 && !s.Warlord.GoalTaken {
			next := s.Warlord.Route[s.Warlord.Index+1]
			v.Warlord.NextStep = &next
		}
		if warlordInSight {
			v.Warlord.Fortified = s.Warlord.GoalTaken
			if st := s.siteAt(warlordPos); st != nil && (pack.Sites[st.ID].Type == "keep" || pack.Sites[st.ID].Type == "tower") {
				v.Warlord.Fortified = true
			}
		}
	}
	if gs := s.site(s.Warlord.GoalSite); gs != nil && gs.Seen {
		goal := gs.Pos
		v.Warlord.Goal = &goal
	}
	// Event, camp, battle.
	if s.PendingEvent != nil {
		ev := pack.Events[s.PendingEvent.Event]
		ve := &ViewEvent{Site: s.PendingEvent.Site, Event: ev.ID, Name: ev.Name, Text: ev.Text, Choices: []ViewChoice{}}
		influence := s.Turn.Influence
		if s.Turn.InfluenceSite != s.PendingEvent.Site {
			influence = 0
		}
		influence += s.Energy / cfg.Influence.EnergyPerInfluence
		for _, c := range ev.Choices {
			need := s.influencePrice(r, c.Requires.Influence)
			vc := ViewChoice{ID: c.ID, Text: c.Text, Influence: need, Gold: c.Requires.Gold, Item: c.Requires.Item, Available: true}
			if c.Roll != nil {
				vc.Roll = c.Roll.Needed
			}
			switch {
			case influence+len(s.Hand) < need:
				vc.Available, vc.Reason = false, fmt.Sprintf("needs %d Influence (cards or energy played here)", need)
			case s.Gold < c.Requires.Gold:
				vc.Available, vc.Reason = false, fmt.Sprintf("needs %d gold", c.Requires.Gold)
			case c.Requires.Item != "" && s.Inventory[c.Requires.Item] < 1:
				vc.Available, vc.Reason = false, "needs a "+pack.Gear[c.Requires.Item].Name
			}
			ve.Choices = append(ve.Choices, vc)
		}
		v.Event = ve
	}
	if s.Camp != nil {
		c := s.Camp
		vc := &ViewCamp{Reason: c.Reason, Turn: c.Turn, Round: c.Round, Inn: c.Inn, Site: c.Site, Deadline: c.Deadline, Reshuffled: c.Reshuffled, EnergyBefore: c.EnergyBefore, Forfeited: c.Forfeited, Drew: c.Drew, Heals: append([]HealRoll{}, c.Heals...), Healed: c.Healed, BasicAidDie: cfg.CampHealing.BasicAidDie, BandageDie: cfg.CampHealing.BandageDie, InnBonus: s.innBonus(r), AidBonus: c.AidBonus, SpareBandages: s.statusStacks(cfg.Statuses.SpareBandage), Bandages: s.bandages(r), DeckChange: s.influencePrice(r, cfg.DeckChangeInfluence["camp"]), DeckOpen: s.Turn.DeckOpen == "camp"}
		for _, w := range v.Wounds {
			if w.BasicAid {
				vc.Untreated++
			}
			if w.Treated && !w.Bandaged {
				vc.Bandageable++
			}
		}
		vc.CanBasicAid = vc.Untreated > 0
		vc.CanBandage = vc.Bandageable > 0 && vc.Bandages > 0
		switch {
		case len(s.Wounds) == 0:
			vc.HealReason = "no wounds"
		case vc.Untreated == 0 && vc.Bandageable == 0:
			vc.HealReason = "every wound has been treated and bandaged at this camp"
		case vc.Untreated == 0 && vc.Bandages == 0:
			vc.HealReason = "every wound has been treated at this camp; a bandage would heal more"
		}
		v.Camp = vc
	}
	if pb := s.PendingBattle; pb != nil {
		vb := &ViewPendingBattle{ID: pb.ID, Kind: pb.Kind, Name: pb.Name, Opponent: pb.Opponent, Count: pb.Count, Ambush: pb.Ambush, Warlord: pb.Warlord, EnergyIn: pb.EnergyIn, Enemy: []ViewStatus{}, Player: []ViewStatus{}}
		for _, st := range pb.EnemyStatuses {
			vb.Enemy = append(vb.Enemy, s.viewStatus(r, st))
		}
		for _, st := range pb.PlayerStatuses {
			vb.Player = append(vb.Player, s.viewStatus(r, st))
		}
		v.Battle = vb
	}
	if look := s.PendingLook; look != nil {
		vl := &ViewLook{Card: look.Card, CardName: r.cardName(look.Card), Cards: []ViewCard{}, Draw: look.Draw, Bottom: look.Bottom}
		for _, id := range look.Cards {
			vl.Cards = append(vl.Cards, ViewCard{ID: id, Name: r.cardName(id)})
			v.CardNames[id] = r.cardName(id)
		}
		v.Look = vl
	}
	return v
}

// turnEffects names this turn's map card effects for the HUD.
func (s *State) turnEffects(r Rules) []string {
	t := s.Turn
	out := []string{}
	if t.IgnoreTerrain {
		out = append(out, "every hex costs 1 Move")
	} else if t.IgnoreNight && s.Night() {
		out = append(out, "Day terrain costs")
	}
	if t.CheapTerrain != "" {
		out = append(out, t.CheapTerrain+" costs 1 Move")
	}
	if t.CrossCost > 0 && !t.Crossed {
		out = append(out, fmt.Sprintf("enter one lake or mountain hex for %d Move", t.CrossCost))
	}
	if t.CarryOver {
		out = append(out, "leftover Move carries over")
	}
	if t.ExploreDiscount > 0 {
		out = append(out, fmt.Sprintf("exploring costs %d", s.exploreCost(r)))
	}
	if t.ExploreReach > 1 {
		out = append(out, fmt.Sprintf("explore up to %d hexes away", t.ExploreReach))
	}
	if t.Sneak {
		out = append(out, "no provoked fights")
	}
	if t.HighGround {
		text := "next fight from cover"
		if t.AmbushBonus > 0 {
			text += fmt.Sprintf(" (Ambush +%d dice)", t.AmbushBonus)
		}
		out = append(out, text)
	}
	if t.RevealTiles {
		out = append(out, "enemies on tiles in sight revealed")
	}
	if t.Haggle > 0 && t.HaggleAt == s.visitHere(r) {
		out = append(out, fmt.Sprintf("Influence payments here cost %d less", t.Haggle))
	}
	return out
}

// viewWounds lists the wounds with what each camp healing step can still do.
func (s *State) viewWounds(r Rules) []ViewWound {
	out := []ViewWound{}
	bandages := s.bandages(r)
	for _, w := range s.Wounds {
		vw := ViewWound{ID: w.ID, Damage: w.Damage(), Size: w.Size, Cards: append([]string{}, w.Cards...), Source: w.Source, Treated: w.Treated, Bandaged: w.Bandaged}
		if w.Round > 0 {
			// Wounds migrated from a save written before wounds were grouped
			// have no record of when they were taken.
			vw.Taken = fmt.Sprintf("%s %d, turn %d", dayName(w.Round%2 == 0), (w.Round+1)/2, w.Turn)
			if w.BattleRound > 0 {
				vw.Taken += fmt.Sprintf(" · battle round %d", w.BattleRound)
			}
		}
		if s.Phase == PhaseCamp {
			vw.BasicAid = !w.Treated
			vw.Bandage = w.Treated && !w.Bandaged && bandages > 0
			switch {
			case !w.Treated:
			case w.Bandaged:
				vw.Reason = "treated and bandaged at this camp"
			case bandages == 0:
				vw.Reason = "treated at this camp; a bandage would heal more"
			}
		}
		out = append(out, vw)
	}
	return out
}

// bandages counts the gear that grants a camp healing roll, with the spare
// (temporary) bandages.
func (s *State) bandages(r Rules) int {
	n := s.statusStacks(r.Config.Statuses.SpareBandage)
	for id, count := range s.Inventory {
		if g, ok := r.Pack.Gear[id]; ok && g.Effect == "mend_roll" {
			n += count
		}
	}
	return n
}

// innBonus is the healing bonus at this camp.
func (s *State) innBonus(r Rules) int {
	if s.Camp != nil && s.Camp.Inn {
		return r.Config.CampHealing.InnBonus
	}
	return 0
}

// viewLastRest copies the rest summary with its lists never null (summaries
// saved before camp healing have no heals list) and the clock of the half
// the rest began, read from the round it records.
func (s *State) viewLastRest(r Rules) *RestSummary {
	if s.LastRest == nil {
		return nil
	}
	rest := *s.LastRest
	rest.Camp.Heals = append([]HealRoll{}, rest.Camp.Heals...)
	rest.Patrols = append([]string{}, rest.Patrols...)
	after := viewClock(r.Config, s.roundClock(rest.RoundAfter))
	rest.ClockAfter = &after
	return &rest
}
