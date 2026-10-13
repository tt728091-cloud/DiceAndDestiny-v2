package learned

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
)

// expeditionFixture drives a fresh expedition save to a challenge on the
// first explored tile, with a few cards in Spent, and returns the save id.
type expeditionFixture struct {
	contentRoot, loadoutRoot, saveID string
	x                                expeditionContext
}

func newExpeditionFixture(t *testing.T, seed uint64, name string) expeditionFixture {
	t.Helper()
	contentRoot := filepath.Join(testServerRoot(t), "content")
	loadoutRoot := campaignLoadoutRoot(t)
	x, err := loadExpeditionContext(contentRoot, loadoutRoot)
	if err != nil {
		t.Fatal(err)
	}
	save, err := loadout.NewExpeditionSave(loadoutRoot, "starter", name, "", seed, x.campaign, x.economy, x.catalogs, x.rules["starter"])
	if err != nil {
		t.Fatal(err)
	}
	return expeditionFixture{contentRoot: contentRoot, loadoutRoot: loadoutRoot, saveID: save.ID, x: x}
}

func (f expeditionFixture) save(t *testing.T) loadout.CampaignSave {
	t.Helper()
	save, err := loadout.ReadExpeditionSave(f.loadoutRoot, f.saveID, f.x.economy, f.x.catalogs)
	if err != nil {
		t.Fatal(err)
	}
	return save
}

func (f expeditionFixture) act(t *testing.T, a expedition.Action) loadout.CampaignSave {
	t.Helper()
	save, _, err := loadout.ExpeditionAct(f.loadoutRoot, f.saveID, f.x.economy, f.x.catalogs, f.x.rules["starter"], a, 0)
	if err != nil {
		t.Fatalf("%s: %v", a.Kind, err)
	}
	return save
}

// walkToChallenge moves east, explores the east slot and challenges the
// group at its centre from (2,0), leaving cards in Spent on the way.
func (f expeditionFixture) walkToChallenge(t *testing.T) loadout.CampaignSave {
	t.Helper()
	s := f.save(t).Expedition
	f.act(t, expedition.Action{Kind: "move", Target: &expedition.Hex{Q: 1, R: 0}, Cards: payWith(s.Hand, 1)})
	s = f.save(t).Expedition
	f.act(t, expedition.Action{Kind: "explore", Slot: "east", Cards: payWith(s.Hand, 2)})
	f.act(t, expedition.Action{Kind: "end_turn"})
	s = f.save(t).Expedition
	// (2,0) is the east tile's north-west hex; pay its terrain cost in cards.
	cost := 0
	for _, m := range s.View(f.x.rules["starter"], 0).Moves {
		if m.To == (expedition.Hex{Q: 2, R: 0}) {
			cost = m.Cost
		}
	}
	if cost < 1 || len(payWith(s.Hand, cost)) < cost {
		t.Fatalf("cannot pay %d Move for (2,0) with hand %v", cost, s.Hand)
	}
	f.act(t, expedition.Action{Kind: "move", Target: &expedition.Hex{Q: 2, R: 0}, Cards: payWith(s.Hand, cost)})
	s = f.save(t).Expedition
	var target string
	for _, g := range s.Groups {
		if g.Pos == (expedition.Hex{Q: 2, R: 1}) && !g.Defeated {
			target = g.ID
		}
	}
	if target == "" {
		t.Fatal("the east tile's centre holds a group")
	}
	return f.act(t, expedition.Action{Kind: "challenge", Group: target})
}

// payWith picks n hand cards to spend, never Trailblaze (kept for the battle).
func payWith(hand []string, n int) []string {
	var out []string
	for _, id := range hand {
		if id != "trailblaze" && len(out) < n {
			out = append(out, id)
		}
	}
	return out
}

func sortedCopy(ids []string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}

func pileDefinitions(b state.Battle, seat string, ids []string) []string {
	instances := b.Settled.Actors[seat].CardInstances
	var out []string
	for _, id := range ids {
		out = append(out, instances[id].DefinitionID)
	}
	return sortedCopy(out)
}

func TestExpeditionBattleBridgeBothWays(t *testing.T) {
	victories, defeats := 0, 0
	sawDiscardFirst := false
	for seed := uint64(1); seed <= 60 && (victories == 0 || defeats == 0 || !sawDiscardFirst); seed++ {
		f := newExpeditionFixture(t, seed, fmt.Sprintf("Bridge %d", seed))
		before := f.walkToChallenge(t)
		pb := before.Expedition.PendingBattle
		if pb == nil || pb.Kind != "challenge" {
			t.Fatalf("seed %d: no pending challenge", seed)
		}
		mapHand, mapDraw, mapSpent, mapWounds := sortedCopy(before.Expedition.Hand), sortedCopy(before.Expedition.Draw), sortedCopy(before.Expedition.Spent), sortedCopy(before.Expedition.Wounds.Cards())
		if len(mapSpent) < 4 {
			t.Fatalf("seed %d: the walk should leave 4 cards in Spent, has %d", seed, len(mapSpent))
		}
		energyIn := before.Expedition.Energy
		s, err := NewSession(SessionConfig{ContentRoot: f.contentRoot, LoadoutRoot: f.loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: pb.Opponent, OpponentCount: pb.Count})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResetExpeditionBattle(f.saveID); err != nil {
			t.Fatal(err)
		}
		if s.telemetry.BattleID != pb.ID || s.telemetry.Seed != pb.Seed {
			t.Fatalf("the battle uses the pending battle's id and seed: %s %d", s.telemetry.BattleID, s.telemetry.Seed)
		}
		// At the first decision (round 1 Offensive) Income has drawn exactly one
		// card from the map draw pile; there was no opening draw.
		start, err := s.environment.InspectBattle()
		if err != nil {
			t.Fatal(err)
		}
		actor := start.Actors[s.humanSeat]
		if actor.Resources.StartingHandSize != 0 || actor.Resources.MaxHandSize != 6 || start.Settled.Actors[s.humanSeat].HandLimit != 6 {
			t.Fatalf("preset resources: %+v hand limit %d", actor.Resources, start.Settled.Actors[s.humanSeat].HandLimit)
		}
		hand, deck, discard, removed := pileDefinitions(start, s.humanSeat, actor.Cards.Hand), pileDefinitions(start, s.humanSeat, actor.Cards.Deck), pileDefinitions(start, s.humanSeat, actor.Cards.Discard), pileDefinitions(start, s.humanSeat, actor.Cards.Removed)
		if len(hand) != len(mapHand)+1 || len(deck) != len(mapDraw)-1 || !reflect.DeepEqual(discard, mapSpent) || !reflect.DeepEqual(removed, mapWounds) {
			t.Fatalf("seed %d: battle piles hand %v deck %v discard %v removed %v; map hand %v draw %v spent %v wounds %v", seed, hand, deck, discard, removed, mapHand, mapDraw, mapSpent, mapWounds)
		}
		// Every map hand card is in the battle hand; the drawn card came from the map draw pile.
		if !containsAll(hand, mapHand) || !containsAll(mapDraw, deck) {
			t.Fatalf("seed %d: hand/draw mismatch: %v ⊇ %v, %v ⊇ %v", seed, hand, mapHand, mapDraw, deck)
		}
		if actor.Resources.EnergyPoints != energyIn+5 {
			t.Fatalf("seed %d: the battle starts with the map's %d energy plus Income 5, has %d", seed, energyIn, actor.Resources.EnergyPoints)
		}
		if pb.Ambush != (stacksOf(actor.Statuses, "ambush") == 3) {
			t.Fatalf("seed %d: Ambush status %v vs pending %v", seed, actor.Statuses, pb.Ambush)
		}
		// hand = map hand + exactly one card and deck = map draw − 1 at round
		// 1's first decision: Income drew once and there was no opening draw
		// (an opening draw would have moved four more cards).
		view := playCampaignBattle(t, s)
		final, err := s.environment.InspectBattle()
		if err != nil {
			t.Fatal(err)
		}
		actor = final.Actors[s.humanSeat]
		after := f.save(t)
		outcome := s.expedition.outcome
		if outcome == nil || s.expedition.err != "" || object(view["campaign"])["outcome"] == nil {
			t.Fatalf("seed %d: the battle end did not record: %+v", seed, s.expedition)
		}
		if outcome.Result != view["battle_result"] || after.Expedition.PendingBattle != nil {
			t.Fatalf("seed %d: outcome %+v vs %v, pending %v", seed, outcome, view["battle_result"], after.Expedition.PendingBattle)
		}
		endHand, endDeck, endDiscard, endRemoved := pileDefinitions(final, s.humanSeat, actor.Cards.Hand), pileDefinitions(final, s.humanSeat, actor.Cards.Deck), pileDefinitions(final, s.humanSeat, actor.Cards.Discard), pileDefinitions(final, s.humanSeat, actor.Cards.Removed)
		// Damage takes discard first: the first wound of the battle came
		// entirely from the discard pile while Spent covered it.
		for _, w := range final.Wounds {
			if w.TargetActorID != s.humanSeat {
				continue
			}
			if len(w.Cards) <= len(mapSpent) {
				for _, c := range w.Cards {
					if c.OriginalZone != operation.ZoneDiscard {
						t.Fatalf("seed %d: the first hit took %s from %s, not discard", seed, c.CardDefinitionID, c.OriginalZone)
					}
				}
				sawDiscardFirst = true
			}
			break
		}
		x := after.Expedition
		// Never better off than before: worst to best, each tier with the
		// better ones holds at most what it held going in.
		if outcome.Result == "victory" {
			hand, draw, spent := len(endHand), len(endDeck), len(endDiscard)
			held := outcome.Report.HeldBack
			capped := hand <= len(mapHand) && hand+draw <= len(mapHand)+len(mapDraw) && hand+draw+spent <= len(mapHand)+len(mapDraw)+len(mapSpent)
			if capped != (held == 0) {
				t.Fatalf("seed %d: held back %d, but the battle piles %d/%d/%d vs %d/%d/%d were %v within the caps", seed, held, hand, draw, spent, len(mapHand), len(mapDraw), len(mapSpent), capped)
			}
			if x.Wounds.Damage() < len(mapWounds) {
				t.Fatalf("seed %d: a battle never leaves fewer wounds than it began with: %d < %d", seed, x.Wounds.Damage(), len(mapWounds))
			}
		}
		switch outcome.Result {
		case "victory":
			victories++
			if !reflect.DeepEqual(sortedCopy(x.Wounds.Cards()), endRemoved) {
				t.Fatalf("seed %d: wounds must come back exactly: %v vs %v", seed, x.Wounds, endRemoved)
			}
			// Each hit of the battle is one wound on the map, with its cards,
			// its source's name and the battle round; never a count of cards.
			ledger := 0
			for _, w := range final.Wounds {
				if w.TargetActorID != s.humanSeat || len(w.Cards) == 0 {
					continue
				}
				if ledger >= len(x.Wounds) {
					t.Fatalf("seed %d: the map lost a battle wound: %+v", seed, x.Wounds)
				}
				got := x.Wounds[ledger]
				want := make([]string, 0, len(w.Cards))
				for _, c := range w.Cards {
					want = append(want, c.CardDefinitionID)
				}
				if !reflect.DeepEqual(got.Cards, want) || got.Source == "" || got.BattleRound != w.Round || got.Size != len(want) || got.Treated || got.Bandaged {
					t.Fatalf("seed %d: battle wound %d (%s) came back as %+v, want cards %v", seed, ledger, w.SourceContentID, got, want)
				}
				ledger++
			}
			if ledger != len(x.Wounds) || outcome.Report.Wounds != len(x.Wounds) || outcome.Report.WoundCards != len(endRemoved) || outcome.Report.NewWounds != ledger {
				t.Fatalf("seed %d: wound groupings %d vs ledger %d; report %+v", seed, len(x.Wounds), ledger, outcome.Report)
			}
			// The fight ended the turn: the end-of-turn draw moved cards from
			// the battle's draw pile into the hand; if that emptied the draw
			// pile, the rest reshuffled Spent into it.
			live := append(append([]string{}, x.Hand...), x.Draw...)
			if x.Phase == expedition.PhaseTurn {
				if !reflect.DeepEqual(sortedCopy(x.Spent), endDiscard) || !reflect.DeepEqual(sortedCopy(live), sortedCopy(append(append([]string{}, endHand...), endDeck...))) {
					t.Fatalf("seed %d: Spent %v vs %v; hand+draw %v vs hand+deck %v", seed, x.Spent, endDiscard, live, append(endHand, endDeck...))
				}
				if x.Turn.Number != 3 {
					t.Fatalf("seed %d: the turn moved on: %d", seed, x.Turn.Number)
				}
			} else {
				if x.Phase != expedition.PhaseCamp || x.Camp.Reason != "deck_empty" || len(x.Spent) != 0 || !reflect.DeepEqual(sortedCopy(live), sortedCopy(append(append(append([]string{}, endHand...), endDeck...), endDiscard...))) {
					t.Fatalf("seed %d: after the draw pile ran out the rest reshuffled: phase %s spent %v live %v", seed, x.Phase, x.Spent, live)
				}
			}
			if !containsAll(x.Hand, endHand) {
				t.Fatalf("seed %d: the battle hand stays: %v ⊇ %v", seed, x.Hand, endHand)
			}
			if x.Energy != min(actor.Resources.EnergyPoints, energyIn) || outcome.Report.EnergyKept != x.Energy {
				t.Fatalf("seed %d: energy kept is min(end %d, in %d), got %d", seed, actor.Resources.EnergyPoints, energyIn, x.Energy)
			}
			if outcome.XP != f.x.rules["starter"].Pack.Groups[pb.Group].XP || after.Sheet.XP != 100+outcome.XP || x.Gold != f.x.rules["starter"].Pack.Groups[pb.Group].Gold {
				t.Fatalf("seed %d: victory rewards: %+v xp %d gold %d", seed, outcome, after.Sheet.XP, x.Gold)
			}
			if deckHealth(after.Sheet.Deck) != len(endHand)+len(endDeck)+len(endDiscard) || deckHealth(after.Sheet.Wounds) != len(endRemoved) {
				t.Fatalf("seed %d: the sheet's deck and wounds follow the piles", seed)
			}
		case "defeat":
			defeats++
			cards := x.Wounds.Cards()
			if !reflect.DeepEqual(sortedCopy(cards[:len(mapWounds)]), mapWounds) || len(cards) != len(mapWounds)+2 || outcome.Report.DefeatWounds != 2 {
				t.Fatalf("seed %d: defeat wounds are the pre-battle wounds plus 2: %v", seed, x.Wounds)
			}
			if last := x.Wounds[len(x.Wounds)-1]; last.Damage() != 2 || last.Source != "defeat by "+pb.Name || outcome.Report.NewWounds != 1 {
				t.Fatalf("seed %d: a defeat is one wound of 2 damage: %+v", seed, x.Wounds)
			}
			if x.Player != pb.Origin || x.Phase != expedition.PhaseCamp || x.Camp.Reason != "defeat" || after.Sheet.XP != 100 || x.Gold != 0 {
				t.Fatalf("seed %d: defeat pushes back and rests without reward: player %v phase %s", seed, x.Player, x.Phase)
			}
			if g := after.Expedition.Groups; !groupAlive(g, pb.Target) {
				t.Fatalf("seed %d: the enemy stays after a defeat", seed)
			}
		}
		// Recording twice is refused; the save is unchanged.
		s.mu.Lock()
		s.recordExpeditionBattle()
		s.mu.Unlock()
		again := f.save(t)
		if again.Sheet.Revision != after.Sheet.Revision {
			t.Fatal("re-recording changed the save")
		}
	}
	if victories == 0 || defeats == 0 || !sawDiscardFirst {
		t.Fatalf("missing coverage: %d victories, %d defeats, discard-first %v", victories, defeats, sawDiscardFirst)
	}
}

func containsAll(have, want []string) bool {
	counts := map[string]int{}
	for _, id := range have {
		counts[id]++
	}
	for _, id := range want {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

func groupAlive(groups []expedition.GroupState, id string) bool {
	for _, g := range groups {
		if g.ID == id {
			return !g.Defeated
		}
	}
	return false
}

func stacksOf(statuses []state.StatusState, id string) int {
	n := 0
	for _, st := range statuses {
		if st.DefinitionID == id {
			n += st.Stacks
		}
	}
	return n
}

// A map card in the battle: never legal to play, still health, removable.
func TestMapCardsAreNeverLegalInBattle(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		f := newExpeditionFixture(t, seed, "Trailblaze")
		// Put Trailblaze into the sheet's deck and the map hand directly, as a
		// deck change at a site would, then challenge the east group.
		if err := editExpeditionSave(f, func(save *loadout.CampaignSave) {
			save.Sheet.Deck = append(save.Sheet.Deck, loadout.Entry{CardID: "trailblaze", Count: 1})
			save.Sheet.DeckValue += 10
			save.Expedition.Hand = append(save.Expedition.Hand, "trailblaze")
		}); err != nil {
			t.Fatal(err)
		}
		before := f.walkToChallenge(t)
		if !strings.Contains(strings.Join(before.Expedition.Hand, ","), "trailblaze") {
			// The walk spent other cards; Trailblaze must still be in hand.
			t.Fatalf("seed %d: Trailblaze left the hand: %v", seed, before.Expedition.Hand)
		}
		pb := before.Expedition.PendingBattle
		s, err := NewSession(SessionConfig{ContentRoot: f.contentRoot, LoadoutRoot: f.loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: pb.Opponent, OpponentCount: pb.Count})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResetExpeditionBattle(f.saveID); err != nil {
			t.Fatal(err)
		}
		start, _ := s.environment.InspectBattle()
		instance := ""
		for id, card := range start.Settled.Actors[s.humanSeat].CardInstances {
			if card.DefinitionID == "trailblaze" {
				instance = id
			}
		}
		if instance == "" || !containsString(start.Actors[s.humanSeat].Cards.Hand, instance) || start.Actors[s.humanSeat].Health.MaxHealth != 13 {
			t.Fatalf("seed %d: Trailblaze must be in the battle hand and count as health", seed)
		}
		decisions := 0
		for step := 0; step < mlsim.DefaultMaxActions && !s.current.Terminal && s.current.TruncationReason == ""; step++ {
			if s.isModelSeat(s.current.ActorID) {
				if _, err = s.AdvanceModel(); err != nil {
					t.Fatal(err)
				}
				continue
			}
			decisions++
			for _, action := range s.current.Result.LegalActions {
				if action.Type == command.TypePlanningCards && strings.Contains(string(action.Payload), instance) {
					t.Fatalf("seed %d: a map card was offered as a legal play: %s", seed, action.Payload)
				}
			}
			alias := aliasValue(commandMap(t, venomTestAction(s.current.Result.LegalActions)), s.aliases(false))
			encoded, _ := json.Marshal(alias)
			if _, err = s.SubmitHuman(string(encoded)); err != nil {
				t.Fatal(err)
			}
		}
		if decisions == 0 {
			t.Fatalf("seed %d: no human decisions", seed)
		}
		final, _ := s.environment.InspectBattle()
		if containsString(final.Actors[s.humanSeat].Cards.Removed, instance) {
			return // removed by damage: it counted as health and could be lost
		}
	}
	t.Log("Trailblaze was never removed by damage in 40 seeds; legality and health were still verified")
}

func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func editExpeditionSave(f expeditionFixture, edit func(*loadout.CampaignSave)) error {
	path := filepath.Join(f.loadoutRoot, "campaigns", f.saveID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var save loadout.CampaignSave
	if err = json.Unmarshal(raw, &save); err != nil {
		return err
	}
	edit(&save)
	raw, _ = json.Marshal(save)
	return os.WriteFile(path, raw, 0o600)
}
