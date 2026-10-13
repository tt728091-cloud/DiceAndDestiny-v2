package expedition

import (
	"errors"
	"fmt"
	"strings"
)

type battleStart struct {
	kind      string
	group     *GroupState
	groupDef  *Group
	warlord   bool
	origin    Hex
	position  Hex
	site      string
	fortified bool
	usedSiege bool
	transient bool // a guardian or event enemy that is not a map group
}

// ambushAllowed: the player starts the fight, unseen (began the turn outside
// the target's sight, or attacks from cover), and the habit allows it now.
func (s *State) ambushAllowed(r Rules, kind string, habit string, target Hex) bool {
	if kind == "provoked" || kind == "deadline" {
		return false
	}
	unseen := Distance(s.Turn.StartHex, target) > r.Config.SightRange(s.Night)
	if _, th, ok := s.tileAt(r.Pack, s.Player); ok {
		if r.Config.Terrain[th.Terrain].Cover {
			unseen = true
		}
		if th.Site != "" && r.Pack.Sites[th.Site].Cover {
			unseen = true
		}
	}
	if !unseen {
		return false
	}
	switch habit {
	case "unwary":
		return true
	case "sleeper":
		return s.Night
	case "night_hunter":
		return !s.Night
	}
	return false
}

// startBattle records a pending battle: the fight runs in the battle session
// and ResolveBattle brings the result back. A fight ends the turn.
func (s *State) startBattle(r Rules, start battleStart) {
	cfg := r.Config
	var def Group
	switch {
	case start.warlord:
		def = r.Pack.Groups[s.Warlord.Group]
	case start.groupDef != nil:
		def = *start.groupDef
	default:
		def = r.Pack.Groups[start.group.Group]
	}
	pb := &PendingBattle{Kind: start.kind, Group: def.ID, Opponent: def.Combatant, Count: def.Count, Name: def.Name, Site: start.site, Origin: start.origin, Position: start.position, Warlord: start.warlord, UsedSiege: start.usedSiege}
	if start.group != nil {
		pb.Target = start.group.ID
	} else {
		pb.Target = "warlord"
	}
	pb.Seed = s.next()
	// Battle ids allow letters, digits, dots, dashes and underscores only.
	pb.ID = fmt.Sprintf("expedition-%s-r%d-t%d-n%d-%s", s.Region, s.Round, s.Turn.Number, s.RNGPosition, strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			return c
		}
		return '.'
	}, pb.Target))
	pb.EnergyIn = s.Energy
	pb.HandBefore = append([]string{}, s.Hand...)
	pb.DrawBefore = append([]string{}, s.Draw...)
	pb.SpentBefore = append([]string{}, s.Spent...)
	pb.WoundsBefore = s.Wounds.Clone()
	target := start.position
	if start.group != nil {
		target = start.group.Pos
	} else if start.warlord {
		target = s.Warlord.Pos()
	}
	pb.Ambush = s.ambushAllowed(r, start.kind, def.Habit, target)
	if pb.Ambush {
		pb.PlayerStatuses = append(pb.PlayerStatuses, StatusStack{ID: cfg.Statuses.Ambush, Stacks: cfg.Ambush.BonusDice})
	}
	for _, st := range s.Statuses {
		pb.PlayerStatuses = append(pb.PlayerStatuses, st)
	}
	fortified := start.fortified
	if start.warlord {
		if s.Warlord.GoalTaken {
			fortified = true
			pb.EnemyStatuses = append(pb.EnemyStatuses, StatusStack{ID: cfg.Statuses.Conqueror, Stacks: cfg.Conqueror.ExtraDefenseDice})
		} else if st := s.siteAt(s.Warlord.Pos()); st != nil {
			// On the road he is Fortified only when camped in a tower or keep.
			t := r.Pack.Sites[st.ID].Type
			fortified = t == "keep" || t == "tower"
		}
		if start.kind == "deadline" {
			// In the open field at the deadline he is never Fortified.
			fortified = false
		}
		if n := s.livingLieutenants(r.Pack); n > 0 {
			pb.EnemyStatuses = append(pb.EnemyStatuses, StatusStack{ID: cfg.Statuses.Lieutenant, Stacks: n})
		}
	}
	if fortified {
		pb.EnemyStatuses = append(pb.EnemyStatuses, StatusStack{ID: cfg.Statuses.Fortified, Stacks: cfg.Fortified.ExtraDefenseDice})
	}
	if def.Lieutenant {
		pb.EnemyStatuses = append(pb.EnemyStatuses, StatusStack{ID: "lieutenant", Stacks: 1})
	}
	s.Player = start.position
	s.PendingBattle = pb
	s.Phase = PhaseBattle
	s.Turn.Fought, s.Turn.ActionUsed = true, true
	s.refreshSight(r)
	s.log("Battle: %s (%s%s).", def.Name, start.kind, map[bool]string{true: ", Ambush", false: ""}[pb.Ambush])
}

// BattleEnd is what the battle session reads from the terminal battle.
type BattleEnd struct {
	BattleID string   `json:"battle_id"`
	Result   string   `json:"result"` // victory, defeat, draw
	Hand     []string `json:"hand"`
	Draw     []string `json:"draw"`
	Spent    []string `json:"spent"`
	Removed  []string `json:"removed"`
	Energy   int      `json:"energy"`
	// Wounds are the battle's wound ledger for the player, in order: each
	// damage source that removed cards, with the cards it took.
	Wounds []BattleWound `json:"wounds,omitempty"`
}

// BattleWound is one grouping of damage the battle recorded.
type BattleWound struct {
	Round  int      `json:"round"`
	Source string   `json:"source"`
	Cards  []string `json:"cards"`
}

// woundsAfterBattle regroups the removed pile the battle returned: the
// pre-battle wounds keep the cards still removed, each battle wound becomes a
// grouping of the cards it took, and anything the ledger does not explain
// (battles before the ledger, or a healed-and-rewounded copy) is one more
// grouping. Health never changes: the groupings hold exactly end.Removed.
func (s *State) woundsAfterBattle(pb *PendingBattle, end BattleEnd, name string) int {
	remaining := map[string]int{}
	for _, id := range end.Removed {
		remaining[id]++
	}
	take := func(ids []string) []string {
		kept := []string{}
		for _, id := range ids {
			if remaining[id] > 0 {
				remaining[id]--
				kept = append(kept, id)
			}
		}
		return kept
	}
	wounds := WoundList{}
	for _, w := range pb.WoundsBefore {
		w.Cards = take(w.Cards)
		if len(w.Cards) > 0 {
			wounds = append(wounds, w)
		}
	}
	s.Wounds = wounds
	added := 0
	for _, bw := range end.Wounds {
		cards := take(bw.Cards)
		source := bw.Source
		if source == "" {
			source = name
		}
		if s.addWound(cards, source, bw.Round) != nil {
			added++
		}
	}
	var rest []string
	for _, id := range end.Removed {
		if remaining[id] > 0 {
			remaining[id]--
			rest = append(rest, id)
		}
	}
	if s.addWound(rest, name, 0) != nil {
		added++
	}
	return added
}

// ResolveBattle applies the bridge rules to a finished battle: the piles come
// back as the battle left them, energy is capped at what was brought in, the
// turn ends; victory pays the group's XP and gold; a normal defeat restores
// the pre-battle wounds plus the defeat wounds, pushes the player back and
// forces a rest; losing to the Warlord ends the run. It returns the XP the
// campaign sheet must credit and the mended wounds on a region win.
func (s *State) ResolveBattle(r Rules, end BattleEnd) (Rewards, *BattleReport, error) {
	pb := s.PendingBattle
	if pb == nil {
		return Rewards{}, nil, errors.New("no battle is pending")
	}
	cfg := r.Config
	def := r.Pack.Groups[pb.Group]
	report := &BattleReport{BattleID: end.BattleID, Kind: pb.Kind, Name: pb.Name, Result: end.Result, EnergyIn: pb.EnergyIn, EnergyEnd: end.Energy, WoundsBefore: len(pb.WoundsBefore), Lieutenant: def.Lieutenant}
	rewards := Rewards{}
	s.PendingBattle = nil
	switch end.Result {
	case "victory":
		s.Hand, s.Draw, s.Spent = append([]string{}, end.Hand...), append([]string{}, end.Draw...), append([]string{}, end.Spent...)
		report.NewWounds = s.woundsAfterBattle(pb, end, pb.Name)
		s.Energy = min(end.Energy, pb.EnergyIn)
		rewards.XP = def.XP
		s.XPEarned += def.XP
		s.Gold += def.Gold
		report.XP, report.Gold = def.XP, def.Gold
		if pb.Warlord {
			s.Warlord.Defeated = true
			s.Result = "victory"
			s.Phase = PhaseVictory
			unused := max(0, cfg.RoundsPerRegion-s.Round)
			if pb.Kind == "deadline" {
				unused = 0
			}
			report.EarlyXP = unused * cfg.Warlord.EarlyReward.XPPerRound
			rewards.XP += report.EarlyXP
			s.XPEarned += report.EarlyXP
			if unused > 0 {
				report.RelicChoices = cfg.Warlord.EarlyReward.ExtraRelicChoices
				s.PendingRelicChoices += report.RelicChoices
			}
			// Winning the region mends every wound, as clearing a campaign does.
			report.WoundsMended = len(s.healPoints(s.Wounds.Damage()))
			report.RegionWon = true
			s.log("The Warlord is beaten. The region is won.")
		} else {
			if g := s.group(pb.Target); g != nil {
				g.Defeated = true
			}
			if pb.Site != "" {
				st := s.site(pb.Site)
				sdef := r.Pack.Sites[pb.Site]
				switch sdef.Type {
				case "ruins", "grotto":
					st.Cleared = true
					loot := sdef.Loot.Day
					if s.Night {
						loot = sdef.Loot.Night
					}
					report.Loot = s.rangeIn(loot)
					s.Gold += report.Loot
				default:
					st.Freed = true
					report.SiteFreed = sdef.Name
				}
			}
			s.log("Victory over %s: +%d XP, +%d gold.", pb.Name, def.XP, def.Gold)
		}
	case "defeat":
		if pb.Warlord {
			s.Hand, s.Draw, s.Spent = append([]string{}, end.Hand...), append([]string{}, end.Draw...), append([]string{}, end.Spent...)
			report.NewWounds = s.woundsAfterBattle(pb, end, pb.Name)
			s.Energy = min(end.Energy, pb.EnergyIn)
			s.Result = "defeat"
			s.Phase = PhaseDefeat
			report.RunOver = true
			s.log("The Warlord has won. The run is over.")
		} else {
			// The battle's damage does not stick: pre-battle piles and wounds,
			// plus one defeat wound, never the last live card.
			s.Hand, s.Draw, s.Spent = append([]string{}, pb.HandBefore...), append([]string{}, pb.DrawBefore...), append([]string{}, pb.SpentBefore...)
			s.Wounds = pb.WoundsBefore.Clone()
			s.Energy = pb.EnergyIn
			report.DefeatWounds = s.inflictWound("defeat by "+pb.Name, cfg.Defeat.Wounds)
			if report.DefeatWounds > 0 {
				report.NewWounds = 1
			}
			s.Player = pb.Origin
			origin := pb.Origin
			report.PushedBack, report.ForcedRest = &origin, true
			s.log("Defeated by %s: %d defeat wounds, driven back to %s.", pb.Name, report.DefeatWounds, pb.Origin)
		}
	default:
		// A draw (both sides fell in the same round) records nothing, as in
		// the linear campaign: the pre-battle piles, wounds and energy come
		// back, nobody wins, the enemy stays, and the turn ends.
		s.Hand, s.Draw, s.Spent = append([]string{}, pb.HandBefore...), append([]string{}, pb.DrawBefore...), append([]string{}, pb.SpentBefore...)
		s.Wounds = pb.WoundsBefore.Clone()
		s.Energy = pb.EnergyIn
		if pb.Warlord {
			s.Player = pb.Origin
		}
		s.log("The battle with %s ended in a draw: both sides fall back.", pb.Name)
	}
	report.EnergyKept = s.Energy
	report.HandAfter = append([]string{}, s.Hand...)
	report.DrawAfter, report.SpentAfter, report.Wounds, report.WoundCards = len(s.Draw), len(s.Spent), len(s.Wounds), s.Wounds.Damage()
	s.LastBattle = report
	if s.Result != "" {
		s.refreshSight(r)
		return rewards, report, nil
	}
	if end.Result == "draw" && pb.Kind == "deadline" {
		// The deadline fight is forced: after a draw the Warlord attacks again.
		s.Phase = PhaseTurn
		s.log("The Warlord attacks again.")
		s.startBattle(r, battleStart{kind: "deadline", warlord: true, origin: s.Player, position: s.Player})
		return rewards, report, nil
	}
	// A fight ends the turn.
	if report.ForcedRest {
		s.Phase = PhaseTurn
		s.Turn.Fought = true
		s.beginRest(r, "defeat", 0)
		return rewards, report, nil
	}
	s.Phase = PhaseTurn
	if err := s.endTurn(r); err != nil {
		return rewards, report, err
	}
	return rewards, report, nil
}

// BattleSetup is what the battle bridge needs to start the pending battle.
type BattleSetup struct {
	BattleID       string
	Opponent       string
	Count          int
	Seed           uint64
	Hand           []string
	Draw           []string
	Spent          []string
	Wounds         []string
	Energy         int
	PlayerStatuses []StatusStack
	EnemyStatuses  []StatusStack
	HandSize       int
	HandLimit      int
}

// Setup returns the pending battle's starting piles and statuses.
func (s *State) Setup(r Rules) (BattleSetup, error) {
	pb := s.PendingBattle
	if pb == nil {
		return BattleSetup{}, errors.New("no battle is pending")
	}
	return BattleSetup{BattleID: pb.ID, Opponent: pb.Opponent, Count: pb.Count, Seed: pb.Seed,
		Hand: append([]string{}, s.Hand...), Draw: append([]string{}, s.Draw...), Spent: append([]string{}, s.Spent...), Wounds: s.Wounds.Cards(),
		Energy: s.Energy, PlayerStatuses: append([]StatusStack{}, pb.PlayerStatuses...), EnemyStatuses: append([]StatusStack{}, pb.EnemyStatuses...),
		HandSize: s.effectiveHandSize(r), HandLimit: s.effectiveHandLimit(r)}, nil
}
