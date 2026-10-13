package learned

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// --- a competent human proxy for the battle ---------------------------------
//
// The proxy only ever submits one of the authority's legal actions. It plays
// like an attentive player: roll, re-roll the dice that do not serve the
// strongest reachable attack, pick the highest-damage qualified tier, keep the
// best five after an Ambush, defend every attack with a defensive ability and
// play prevention cards whenever they are legal.

func decodePayload(action command.Command, into any) {
	_ = json.Unmarshal(action.Payload, into)
}

func symbolCount(dice []state.RolledDie, symbol string) int {
	n := 0
	for _, d := range dice {
		for _, s := range d.Symbols {
			if s == symbol {
				n++
				break
			}
		}
	}
	return n
}

func hasSymbol(d state.RolledDie, symbol string) bool {
	for _, s := range d.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

func goalSymbol(dice []state.RolledDie) string {
	if symbolCount(dice, "gold_coin") >= 3 && symbolCount(dice, "sword") < 3 {
		return "gold_coin"
	}
	return "sword"
}

func operationAmount(op content.BattleOperation) int {
	switch v := op.Amount.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func tierDamage(lib content.BattleLibrary, abilityID, tierID string) int {
	def, ok := lib.Abilities[abilityID]
	if !ok || def.Qualification == nil {
		return 1
	}
	best := 0
	for i, tier := range def.Qualification.ActivationTiers {
		if tierID != "" && tier.ID != tierID {
			continue
		}
		for _, op := range tier.Operations {
			if op.Type == "deal_damage" && operationAmount(op) > best {
				best = operationAmount(op)
			}
		}
		if tierID == "" && i == 0 {
			break
		}
	}
	if best == 0 {
		return 1
	}
	return best
}

func sortedInts(in []int) []int {
	out := append([]int{}, in...)
	sort.Ints(out)
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func smartHumanAction(b state.Battle, lib content.BattleLibrary, seat string, actions []command.Command) command.Command {
	byType := map[command.Type][]command.Command{}
	for _, a := range actions {
		byType[a.Type] = append(byType[a.Type], a)
	}
	// Prevention and reaction cards against incoming damage: a played card
	// stays health, so play them whenever they are legal.
	for _, a := range byType[command.TypeCommitInteraction] {
		var p command.CommitInteractionPayload
		decodePayload(a, &p)
		if len(p.Commitment.ProposalIDs) > 0 && len(p.Commitment.CardIDs) > 0 {
			return a
		}
	}
	stage := ""
	var runtime state.SettledActorRuntime
	if b.Settled != nil {
		if b.Settled.Window != nil {
			stage = b.Settled.Window.Stage
		}
		runtime = b.Settled.Actors[seat]
	}
	switch stage {
	case "planning":
		if roll := byType[command.TypePlanningRoll]; len(roll) > 0 {
			return roll[0]
		}
		dice := runtime.FinalDice
		if runtime.AmbushChoose > 0 && len(byType[command.TypePlanningKeep]) > 0 {
			goal := goalSymbol(dice)
			order := make([]int, len(dice))
			for i := range order {
				order[i] = i
			}
			sort.SliceStable(order, func(i, j int) bool {
				return hasSymbol(dice[order[i]], goal) && !hasSymbol(dice[order[j]], goal)
			})
			wanted := sortedInts(order[:runtime.AmbushChoose])
			for _, a := range byType[command.TypePlanningKeep] {
				var p command.PlanningKeepPayload
				decodePayload(a, &p)
				if equalInts(sortedInts(p.KeptIndices), wanted) {
					return a
				}
			}
			return byType[command.TypePlanningKeep][0]
		}
		var best *command.Command
		bestDamage := -1
		for i, a := range byType[command.TypePlanningAbility] {
			var p command.PlanningAbilityPayload
			decodePayload(a, &p)
			if d := tierDamage(lib, p.AbilityID, p.TierID); d > bestDamage {
				bestDamage = d
				best = &byType[command.TypePlanningAbility][i]
			}
		}
		rerolls := byType[command.TypePlanningReroll]
		if runtime.SelectedAbilityID == "" && len(rerolls) > 0 && bestDamage < 6 {
			goal := goalSymbol(dice)
			var indices []int
			for i, d := range dice {
				if !hasSymbol(d, goal) {
					indices = append(indices, i)
				}
			}
			if len(indices) > 0 {
				for _, a := range rerolls {
					var p command.PlanningRerollPayload
					decodePayload(a, &p)
					if equalInts(sortedInts(p.RerollIndices), indices) {
						return a
					}
				}
			}
		}
		if runtime.SelectedAbilityID == "" && best != nil {
			return *best
		}
		if pass := byType[command.TypePlanningPass]; len(pass) > 0 {
			return pass[0]
		}
	case "defense_selection":
		for _, a := range byType[command.TypePlanningAbility] {
			var p command.PlanningAbilityPayload
			decodePayload(a, &p)
			if !p.SpendCatalyst {
				return a
			}
		}
		if pass := byType[command.TypePlanningPass]; len(pass) > 0 {
			return pass[0]
		}
	case "defense_roll":
		if roll := byType[command.TypeRollDice]; len(roll) > 0 {
			return roll[0]
		}
	}
	for _, kind := range []command.Type{command.TypeRollDice, command.TypePlanningPass, command.TypePass, command.TypeCommitInteraction} {
		if list := byType[kind]; len(list) > 0 {
			return list[0]
		}
	}
	return actions[0]
}

// playExpeditionBattle plays the session's current battle to the end with
// the competent proxy on the human seat and returns the final view.
func playExpeditionBattle(t *testing.T, s *Session, lib content.BattleLibrary) map[string]any {
	t.Helper()
	var view map[string]any
	var err error
	for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal && s.current.TruncationReason == ""; step++ {
		if s.isModelSeat(s.current.ActorID) {
			view, err = s.AdvanceModel()
		} else {
			b, inspectErr := s.environment.InspectBattle()
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			chosen := smartHumanAction(b, lib, s.humanSeat, s.current.Result.LegalActions)
			alias := aliasValue(commandMap(t, chosen), s.aliases(false))
			encoded, _ := json.Marshal(alias)
			view, err = s.SubmitHuman(string(encoded))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !s.current.Terminal {
		t.Fatalf("expedition battle did not finish: %s", s.current.TruncationReason)
	}
	return view
}

// --- a map driver over the save API ----------------------------------------

type regionDriver struct {
	t       *testing.T
	f       expeditionFixture
	rules   expedition.Rules
	lib     content.BattleLibrary
	fights  []string // results in order
	actions int
	// afterAct runs after every successful action (save/reload checks).
	afterAct func(save loadout.CampaignSave)
}

func newRegionDriver(t *testing.T, seed uint64, name string) *regionDriver {
	t.Helper()
	f := newExpeditionFixture(t, seed, name)
	return &regionDriver{t: t, f: f, rules: f.x.rules["starter"], lib: f.x.catalogs["starter"]}
}

func (d *regionDriver) save() loadout.CampaignSave { return d.f.save(d.t) }

func (d *regionDriver) st() *expedition.State { return d.save().Expedition }

func (d *regionDriver) view() expedition.View {
	save := d.save()
	return save.Expedition.View(d.rules, save.Sheet.Revision)
}

// try submits an action and returns the authority's error, if any.
func (d *regionDriver) try(a expedition.Action) error {
	save, _, err := loadout.ExpeditionAct(d.f.loadoutRoot, d.f.saveID, d.f.x.economy, d.f.x.catalogs, d.rules, a, 0)
	if err == nil {
		d.actions++
		if d.afterAct != nil {
			d.afterAct(save)
		}
	}
	return err
}

func (d *regionDriver) act(a expedition.Action) {
	d.t.Helper()
	if err := d.try(a); err != nil {
		d.t.Fatalf("%s %+v: %v", a.Kind, a, err)
	}
}

func (d *regionDriver) moveCost(to expedition.Hex) (int, bool) {
	for _, m := range d.view().Moves {
		if m.To == to && m.Blocked == "" {
			return m.Cost, true
		}
	}
	return 0, false
}

// pay converts hand cards first, then energy, into the Move still needed.
func (d *regionDriver) payment(cost int) ([]string, int, bool) {
	s := d.st()
	need := cost - s.Turn.Move
	if need <= 0 {
		return nil, 0, true
	}
	cards := payWith(s.Hand, need)
	if len(cards) < need && len(cards) < len(s.Hand) {
		cards = append([]string{}, s.Hand[:min(need, len(s.Hand))]...)
	}
	need -= len(cards)
	energy := 0
	per := d.rules.Config.Move.EnergyPerMove
	for need > 0 && energy+per <= s.Energy {
		energy += per
		need--
	}
	return cards, energy, need == 0
}

// step moves one hex, paying as needed. Returns false if it is not offered
// or cannot be afforded this turn.
func (d *regionDriver) step(to expedition.Hex) bool {
	d.t.Helper()
	cost, ok := d.moveCost(to)
	if !ok {
		return false
	}
	cards, energy, affordable := d.payment(cost)
	if !affordable {
		return false
	}
	d.act(expedition.Action{Kind: "move", Target: &to, Cards: cards, Energy: energy})
	return true
}

// path is the shortest route over placed, passable, enemy-free hexes; hexes
// next to a living group are avoided when another route exists.
func (d *regionDriver) path(to expedition.Hex, quiet bool) []expedition.Hex {
	v := d.view()
	s := d.st()
	free := map[expedition.Hex]bool{}
	threatened := map[expedition.Hex]bool{}
	for _, g := range s.Groups {
		if !g.Defeated {
			for _, n := range expedition.Neighbors(g.Pos) {
				threatened[n] = true
			}
		}
	}
	for _, h := range v.Hexes {
		at := expedition.Hex{Q: h.Q, R: h.R}
		ok := h.Passable && h.Group == nil && !h.Warlord && !(h.Site != nil && h.Site.Defended)
		if quiet && threatened[at] && at != to {
			ok = false
		}
		if ok || at == to {
			free[at] = true
		}
	}
	start := s.Player
	prev := map[expedition.Hex]expedition.Hex{start: start}
	queue := []expedition.Hex{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == to {
			break
		}
		for _, n := range expedition.Neighbors(cur) {
			if free[n] {
				if _, seen := prev[n]; !seen {
					prev[n] = cur
					queue = append(queue, n)
				}
			}
		}
	}
	if _, ok := prev[to]; !ok {
		if quiet {
			return d.path(to, false)
		}
		return nil
	}
	var out []expedition.Hex
	for h := to; h != start; h = prev[h] {
		out = append([]expedition.Hex{h}, out...)
	}
	return out
}

// placed reports whether a hex lies on a placed tile.
func (d *regionDriver) placed(h expedition.Hex) bool {
	for _, x := range d.view().Hexes {
		if x.Q == h.Q && x.R == h.R {
			return true
		}
	}
	return false
}

// settle resolves whatever is not a plain turn: a pending fight is played,
// a camp is broken, an event takes its first free choice.
func (d *regionDriver) settle() {
	d.t.Helper()
	for guard := 0; guard < 20; guard++ {
		s := d.st()
		switch {
		case s.Result != "":
			return
		case s.PendingBattle != nil:
			d.fight()
		case s.Phase == expedition.PhaseCamp:
			d.act(expedition.Action{Kind: "break_camp"})
		case s.Phase == expedition.PhaseEvent:
			chosen := ""
			for _, c := range d.view().Event.Choices {
				if c.Available && c.Influence == 0 && c.Gold == 0 && c.Item == "" && chosen == "" {
					chosen = c.ID
				}
			}
			if chosen == "" {
				d.t.Fatalf("event %s offers no free choice", d.view().Event.Event)
			}
			d.act(expedition.Action{Kind: "event_choice", Choice: chosen})
		default:
			return
		}
	}
}

// endTurn discards a card if nothing was used yet, then ends the turn.
func (d *regionDriver) endTurn() {
	d.t.Helper()
	d.settle()
	s := d.st()
	if s.Result != "" || s.Phase != expedition.PhaseTurn {
		return
	}
	if s.Turn.CardsUsed == 0 && !s.Turn.Fought {
		if len(s.Hand) == 0 {
			d.t.Fatal("stuck: no card to discard and the turn cannot end")
		}
		d.act(expedition.Action{Kind: "discard", Cards: s.Hand[:1]})
	}
	d.act(expedition.Action{Kind: "end_turn"})
	d.settle()
}

// walk brings the player to a hex, ending turns on the way; it stops early
// when a fight, event or rest interrupts and returns whether it arrived.
func (d *regionDriver) walk(to expedition.Hex) bool {
	d.t.Helper()
	for guard := 0; guard < 60; guard++ {
		d.settle()
		s := d.st()
		if s.Result != "" {
			return false
		}
		if s.Player == to {
			return true
		}
		if s.Phase != expedition.PhaseTurn {
			return false
		}
		route := d.path(to, true)
		if len(route) == 0 {
			return false
		}
		if !d.step(route[0]) {
			d.endTurn()
		}
	}
	return false
}

// fight plays the pending battle through a real session and records it.
func (d *regionDriver) fight() string {
	d.t.Helper()
	save := d.save()
	pb := save.Expedition.PendingBattle
	if pb == nil {
		d.t.Fatal("no pending battle")
	}
	s, err := NewSession(SessionConfig{ContentRoot: d.f.contentRoot, LoadoutRoot: d.f.loadoutRoot, RunStateRoot: d.t.TempDir(), OpponentDefinition: pb.Opponent, OpponentCount: pb.Count})
	if err != nil {
		d.t.Fatal(err)
	}
	if _, err = s.ResetExpeditionBattle(d.f.saveID); err != nil {
		x := save.Expedition
		d.t.Fatalf("%v (hand %d draw %d spent %d wounds %d, fights so far %v, last battle %+v)", err, len(x.Hand), len(x.Draw), len(x.Spent), len(x.Wounds), d.fights, x.LastBattle)
	}
	view := playExpeditionBattle(d.t, s, d.lib)
	result := str(view["battle_result"])
	if s.expedition == nil || s.expedition.outcome == nil || s.expedition.err != "" {
		d.t.Fatalf("the battle %s did not record: %+v", pb.ID, s.expedition)
	}
	d.fights = append(d.fights, fmt.Sprintf("%s:%s", pb.Name, result))
	after := d.save()
	if after.Expedition.PendingBattle != nil {
		d.t.Fatalf("the pending battle %s is still there after %s", pb.ID, result)
	}
	if d.afterAct != nil {
		d.afterAct(after)
	}
	return result
}

// slotOf names the region slot whose flower holds a hex ("" for the start tile).
func (d *regionDriver) slotOf(h expedition.Hex) (string, expedition.Hex) {
	for _, slot := range d.view().Slots {
		if slot.ID == "start" {
			continue
		}
		for _, x := range expedition.TileHexes(slot.Center) {
			if x == h {
				return slot.ID, slot.Center
			}
		}
	}
	return "", expedition.Hex{}
}

// standNextTo walks to the reachable placed hex nearest to any hex of a
// slot's flower, so the slot can be explored. Returns false if none.
func (d *regionDriver) standNextTo(center expedition.Hex) bool {
	d.t.Helper()
	v := d.view()
	placed := map[expedition.Hex]expedition.ViewHex{}
	for _, h := range v.Hexes {
		placed[expedition.Hex{Q: h.Q, R: h.R}] = h
	}
	var best []expedition.Hex
	var target expedition.Hex
	player := d.st().Player
	for _, fh := range expedition.TileHexes(center) {
		for _, n := range expedition.Neighbors(fh) {
			h, ok := placed[n]
			if !ok || !h.Passable || h.Group != nil || h.Warlord {
				continue
			}
			if n == player {
				return true
			}
			route := d.path(n, true)
			if len(route) > 0 && (best == nil || len(route) < len(best)) {
				best, target = route, n
			}
		}
	}
	if best == nil {
		return false
	}
	return d.walk(target)
}

// huntWarlord brings the player next to the Warlord (exploring the slot he
// stands on when it is still face down) and challenges him; returns the
// fight's result, or "" when the region ended first.
func (d *regionDriver) huntWarlord() string {
	d.t.Helper()
	for guard := 0; guard < 80; guard++ {
		d.settle()
		s := d.st()
		if s.Result != "" {
			return ""
		}
		if s.Phase != expedition.PhaseTurn {
			continue
		}
		w := s.Warlord.Pos()
		if !d.placed(w) {
			slot, center := d.slotOf(w)
			if slot == "" || !d.standNextTo(center) {
				d.endTurn()
				continue
			}
			if d.st().Phase != expedition.PhaseTurn {
				continue
			}
			cards, energy, ok := d.payment(d.rules.Config.Move.ExploreCost)
			if !ok {
				d.endTurn()
				continue
			}
			if err := d.try(expedition.Action{Kind: "explore", Slot: slot, Cards: cards, Energy: energy}); err != nil {
				d.endTurn()
			}
			continue
		}
		if !expedition.Adjacent(s.Player, w) {
			moved := false
			for _, n := range expedition.Neighbors(w) {
				if len(d.path(n, true)) > 0 && d.walk(n) {
					moved = true
					break
				}
				if d.st().Result != "" || d.st().Phase != expedition.PhaseTurn {
					moved = true // something else happened; settle on the next pass
					break
				}
			}
			if !moved {
				d.endTurn()
			}
			continue
		}
		if s.Turn.ActionUsed {
			d.endTurn()
			continue
		}
		if err := d.try(expedition.Action{Kind: "challenge", Group: "warlord"}); err != nil {
			d.endTurn()
			continue
		}
		return d.fight()
	}
	return ""
}

// --- the seeded full-region play-throughs -----------------------------------

// reloadCheck asserts, after every action, that the state written to disk
// reads back identical, survives a JSON round trip unchanged, and offers the
// same legal actions (the whole viewer-safe view) as the in-memory state.
func (d *regionDriver) reloadCheck(save loadout.CampaignSave) {
	d.t.Helper()
	raw, err := json.Marshal(save.Expedition)
	if err != nil {
		d.t.Fatal(err)
	}
	var again expedition.State
	if err := json.Unmarshal(raw, &again); err != nil {
		d.t.Fatal(err)
	}
	if !reflect.DeepEqual(*save.Expedition, again) {
		d.t.Fatalf("action %d: the state changes through a JSON round trip", d.actions)
	}
	disk := d.save()
	if !reflect.DeepEqual(disk.Expedition, save.Expedition) {
		d.t.Fatalf("action %d: the reloaded save differs from the one returned", d.actions)
	}
	if disk.Sheet.Revision != save.Sheet.Revision || !reflect.DeepEqual(disk.Sheet.Deck, save.Sheet.Deck) || !reflect.DeepEqual(disk.Sheet.Wounds, save.Sheet.Wounds) || disk.Sheet.XP != save.Sheet.XP {
		d.t.Fatalf("action %d: the reloaded sheet differs", d.actions)
	}
	v1, v2 := save.Expedition.View(d.rules, save.Sheet.Revision), disk.Expedition.View(d.rules, disk.Sheet.Revision)
	if !reflect.DeepEqual(v1, v2) {
		d.t.Fatalf("action %d: the reloaded state offers different legal actions", d.actions)
	}
}

// The seeds were found with TestProbeWarlordOdds (EXPEDITION_PROBE=1): with
// the competent proxy, seed 8 catches the Warlord on the road in round 1 and
// wins; seed 5 never reaches him and loses the forced deadline fight. Both
// play real battles through the session; every map action is checked for
// save/reload equivalence.
const (
	regionVictorySeed = 8
	regionDefeatSeed  = 5
)

func TestFullRegionSeededPlayThroughs(t *testing.T) {
	t.Run("victory", func(t *testing.T) {
		d := newRegionDriver(t, regionVictorySeed, "Region victory")
		d.afterAct = d.reloadCheck
		s := d.st()
		v := d.view()
		if s.Round != 1 || s.Night || s.Phase != expedition.PhaseTurn || len(s.Hand) != 4 || s.Energy != 10 || len(s.Placed) != 1 || len(v.Hexes) != 7 {
			t.Fatalf("a new expedition starts on the start tile with four cards and base energy: round %d hand %d energy %d placed %d hexes %d", s.Round, len(s.Hand), s.Energy, len(s.Placed), len(v.Hexes))
		}
		// Turn 1: the village next door. A card pays the step; two more are
		// Influence; a deck change through the site spends it and 10 XP.
		if !d.step(expedition.Hex{Q: -1, R: 1}) {
			t.Fatal("the village is one plains step away")
		}
		s = d.st()
		if s.Player != (expedition.Hex{Q: -1, R: 1}) || len(s.Spent) != 1 || s.Turn.CardsUsed != 1 {
			t.Fatalf("the paid card went to Spent: %+v", s.Turn)
		}
		save := d.save()
		if _, err := loadout.ExpeditionDeckChange(d.f.loadoutRoot, d.f.saveID, d.f.x.economy, d.f.x.catalogs, d.rules, loadout.Purchase{Kind: "buy_card", ID: "trailblaze", Revision: save.Sheet.Revision, ExpectedCost: 10}); err == nil || !strings.Contains(err.Error(), "Influence") {
			t.Fatalf("a deck change without Influence is refused: %v", err)
		}
		d.act(expedition.Action{Kind: "use_site", Service: "influence", Cards: s.Hand[:2]})
		s = d.st()
		if s.Turn.Influence != 2 || s.Turn.InfluenceSite != "hollow_ford" || len(s.Hand) != 1 {
			t.Fatalf("two cards played at the village are 2 Influence there: %+v", s.Turn)
		}
		save = d.save()
		bought, err := loadout.ExpeditionDeckChange(d.f.loadoutRoot, d.f.saveID, d.f.x.economy, d.f.x.catalogs, d.rules, loadout.Purchase{Kind: "buy_card", ID: "trailblaze", Revision: save.Sheet.Revision, ExpectedCost: 10})
		if err != nil {
			t.Fatal(err)
		}
		d.reloadCheck(bought)
		s = d.st()
		if s.Draw[0] != "trailblaze" || s.Turn.Influence != 0 || bought.Sheet.XP != 90 || s.LiveCards() != 13 {
			t.Fatalf("the bought card tops the draw pile, the Influence and 10 XP are spent: draw %v xp %d", s.Draw, bought.Sheet.XP)
		}
		d.act(expedition.Action{Kind: "end_turn"})
		s = d.st()
		if s.Turn.Number != 2 || len(s.Hand) != 4 {
			t.Fatalf("the hand refills to four: %+v hand %d", s.Turn, len(s.Hand))
		}
		// Catch the Warlord on the road and win the region early.
		result := d.huntWarlord()
		s = d.st()
		if result != "victory" || s.Result != "victory" || s.Phase != expedition.PhaseVictory || !s.Warlord.Defeated {
			t.Fatalf("seed %d: the Warlord hunt ends in victory: %q, fights %v, state %s", regionVictorySeed, result, d.fights, s.Result)
		}
		report := s.LastBattle
		unused := d.rules.Config.RoundsPerRegion - s.Round
		if !report.RegionWon || report.EarlyXP != unused*d.rules.Config.Warlord.EarlyReward.XPPerRound || report.RelicChoices != d.rules.Config.Warlord.EarlyReward.ExtraRelicChoices || s.PendingRelicChoices != report.RelicChoices {
			t.Fatalf("an early victory pays per unused round (%d) and a relic choice: %+v", unused, report)
		}
		if len(s.Wounds) != 0 || report.WoundsMended < 0 {
			t.Fatalf("winning the region mends every wound: %v", s.Wounds)
		}
		save = d.save()
		warlordXP := d.rules.Pack.Groups[s.Warlord.Group].XP
		if save.Sheet.XP != 90+warlordXP+report.EarlyXP || s.XPEarned != warlordXP+report.EarlyXP {
			t.Fatalf("XP: sheet %d earned %d (warlord %d early %d)", save.Sheet.XP, s.XPEarned, warlordXP, report.EarlyXP)
		}
		if err := d.try(expedition.Action{Kind: "end_turn"}); err == nil || !strings.Contains(err.Error(), "over") {
			t.Fatalf("a won region accepts no more actions: %v", err)
		}
		t.Logf("victory seed %d: fights %v, actions %d, XP %d", regionVictorySeed, d.fights, d.actions, save.Sheet.XP)
	})
	t.Run("deadline defeat", func(t *testing.T) {
		d := newRegionDriver(t, regionDefeatSeed, "Region defeat")
		d.afterAct = d.reloadCheck
		// Explore east, then rest through every round: each rest marches the
		// Warlord two hexes along his route and flips Day and Night.
		if !d.step(expedition.Hex{Q: 1, R: 0}) {
			t.Fatal("one step east")
		}
		cards, energy, ok := d.payment(d.rules.Config.Move.ExploreCost)
		if !ok {
			t.Fatal("explore east is affordable")
		}
		d.act(expedition.Action{Kind: "explore", Slot: "east", Cards: cards, Energy: energy})
		s := d.st()
		if len(s.Placed) != 2 || len(d.view().Hexes) != 14 {
			t.Fatalf("the east tile is placed: %d tiles, %d hexes", len(s.Placed), len(d.view().Hexes))
		}
		start := s.Warlord.Pos()
		for round := 1; round <= d.rules.Config.RoundsPerRegion; round++ {
			s = d.st()
			if s.Round != round || s.Night != (round%2 == 0) {
				t.Fatalf("round %d: state round %d night %v", round, s.Round, s.Night)
			}
			d.act(expedition.Action{Kind: "make_camp"})
			s = d.st()
			if s.Phase != expedition.PhaseCamp || len(s.Spent) != 0 || s.Energy != s.BaseEnergy || len(s.Hand) != min(4, s.LiveCards()) {
				t.Fatalf("make camp: Spent reshuffled, energy reset, hand forfeited and a fresh hand drawn: %+v hand %d spent %d energy %d", s.Camp, len(s.Hand), len(s.Spent), s.Energy)
			}
			before := s.Warlord.Index
			d.act(expedition.Action{Kind: "break_camp"})
			s = d.st()
			if s.Round != round+1 || s.Night != (round%2 == 1) || s.LastRest == nil {
				t.Fatalf("break camp advances the round and flips Day/Night: round %d night %v", s.Round, s.Night)
			}
			if !s.Warlord.GoalTaken && s.Warlord.Index != before+d.rules.Config.Warlord.MarchPerRest && !s.Warlord.Arrived {
				t.Fatalf("the Warlord marches %d per rest: %d → %d", d.rules.Config.Warlord.MarchPerRest, before, s.Warlord.Index)
			}
		}
		if s.PendingBattle == nil || s.PendingBattle.Kind != "deadline" || !s.PendingBattle.Warlord || s.PendingBattle.Ambush || !s.Warlord.Arrived || !s.LastRest.Deadline {
			t.Fatalf("after the last round the Warlord comes to the player: %+v", s.PendingBattle)
		}
		// His route was laid out at build with start_distance steps to his
		// goal; the deadline appended the player's hex as his last step.
		route := s.Warlord.Route
		if !s.Warlord.GoalTaken || route[0] != start || route[len(route)-2] != s.Warlord.Goal || len(route)-2 != d.rules.Config.Warlord.StartDistance {
			t.Fatalf("three rests took him from %v to his goal: %+v", start, s.Warlord)
		}
		conqueror := 0
		for _, st := range s.PendingBattle.EnemyStatuses {
			if st.ID == d.rules.Config.Statuses.Conqueror {
				conqueror = st.Stacks
			}
		}
		if conqueror != d.rules.Config.Conqueror.ExtraDefenseDice {
			t.Fatalf("the Warlord at his goal has Conqueror: %+v", s.PendingBattle.EnemyStatuses)
		}
		result := d.fight()
		s = d.st()
		if result != "defeat" || s.Result != "defeat" || s.Phase != expedition.PhaseDefeat || !s.LastBattle.RunOver {
			t.Fatalf("seed %d: the deadline fight is lost and the run is over: %q %s", regionDefeatSeed, result, s.Result)
		}
		if err := d.try(expedition.Action{Kind: "make_camp"}); err == nil || !strings.Contains(err.Error(), "over") {
			t.Fatalf("an ended run accepts no more actions: %v", err)
		}
		if _, err := loadout.ExpeditionDeckChange(d.f.loadoutRoot, d.f.saveID, d.f.x.economy, d.f.x.catalogs, d.rules, loadout.Purchase{Kind: "buy_card", ID: "trailblaze", Revision: d.save().Sheet.Revision, ExpectedCost: 10}); err == nil {
			t.Fatal("an ended run accepts no deck changes")
		}
		t.Logf("defeat seed %d: fights %v, actions %d", regionDefeatSeed, d.fights, d.actions)
	})
}

// --- an exploratory probe (not part of the suite) --------------------------
//
// EXPEDITION_PROBE=1 go test ./internal/battle/learned/ -run TestProbeWarlord -v
// reports, per seed, how a straight run at the Warlord with the competent
// proxy ends. It guided the choice of the seeds used by the region tests.
func TestProbeWarlordOdds(t *testing.T) {
	if os.Getenv("EXPEDITION_PROBE") == "" {
		t.Skip("set EXPEDITION_PROBE=1 to run the Warlord odds probe")
	}
	from, to := 1, 40
	if v := os.Getenv("EXPEDITION_PROBE_FROM"); v != "" {
		from, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("EXPEDITION_PROBE_TO"); v != "" {
		to, _ = strconv.Atoi(v)
	}
	wins := 0
	for seed := from; seed <= to; seed++ {
		d := newRegionDriver(t, uint64(seed), fmt.Sprintf("Probe %d", seed))
		result := d.huntWarlord()
		s := d.st()
		t.Logf("seed %d: %s · round %d · region %s · fights %v", seed, result, s.Round, s.Result, d.fights)
		if result == "victory" {
			wins++
		}
	}
	t.Logf("wins: %d of %d", wins, to-from+1)
}

// EXPEDITION_PROBE=1 go test ./internal/battle/learned/ -run TestProbeRegentMatchup -v
// plays the fresh Starter (sandbox loadout) straight against the Warlord
// placeholder for many seeds and reports the win rate of the competent proxy.
func TestProbeRegentMatchup(t *testing.T) {
	if os.Getenv("EXPEDITION_PROBE") == "" {
		t.Skip("set EXPEDITION_PROBE=1 to run the matchup probe")
	}
	opponent := os.Getenv("EXPEDITION_PROBE_OPPONENT")
	if opponent == "" {
		opponent = "drowned_regent"
	}
	n := 40
	if v := os.Getenv("EXPEDITION_PROBE_TO"); v != "" {
		n, _ = strconv.Atoi(v)
	}
	f := newExpeditionFixture(t, 1, "Matchup")
	lib := f.x.catalogs["starter"]
	wins, draws := 0, 0
	for seed := 1; seed <= n; seed++ {
		s, err := NewSession(SessionConfig{ContentRoot: f.contentRoot, LoadoutRoot: f.loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: opponent, OpponentCount: 1})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ResetCharacterLoadout(fmt.Sprintf("matchup-%d", seed), uint64(seed), "seat-a", false, "starter", true, "sandbox"); err != nil {
			t.Fatal(err)
		}
		view := playExpeditionBattle(t, s, lib)
		final, _ := s.environment.InspectBattle()
		you, foe := final.Actors["seat-a"], final.Actors["seat-b"]
		t.Logf("seed %d: %s · you %d/%d · %s %d/%d · round %d", seed, str(view["battle_result"]), you.CurrentHealth(), len(you.Cards.Hand)+len(you.Cards.Deck)+len(you.Cards.Discard)+len(you.Cards.Removed), opponent, foe.CurrentHealth(), len(foe.Cards.Hand)+len(foe.Cards.Deck)+len(foe.Cards.Discard)+len(foe.Cards.Removed), final.Segment.Round)
		switch str(view["battle_result"]) {
		case "victory":
			wins++
		case "draw":
			draws++
		}
	}
	t.Logf("%s: wins %d, draws %d of %d", opponent, wins, draws, n)
}
