package lookahead

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Matchup is one hero against one opponent, both by content definition ID.
type Matchup struct {
	ContentRoot  string
	AuthoredRoot string
	RunStateRoot string
	Hero         string
	Opponent     string
	Unified      bool
	// HeroPlansFirst gives the hero the first move in simultaneous private
	// windows (offensive planning), so a look-ahead hero's copies of the
	// battle never contain the opponent's private planning.
	HeroPlansFirst bool
	// HeroInSeatB seats the hero in seat B (and the opponent in seat A).
	HeroInSeatB bool
	// HeroDeck and FoeDeck replace a side's content decklist when set.
	HeroDeck, FoeDeck []loadout.Entry
	// Observe, when set, builds an observer for each runner.
	Observe func() Observer
}

// Observer watches a runner's battles: every decision before the player
// makes it, and each battle's end. One runner uses it at a time. An
// observer may also have Begin(seed, heroSeat), called as each battle
// starts, and Chose(d, index), called with each move made.
type Observer interface {
	Decision(d *Decision)
	Finish(final *state.Battle, record BattleRecord, hero, foe Policy)
}

// battleStarter and moveObserver are the optional parts of an Observer.
type battleStarter interface {
	Begin(seed uint64, heroSeat string)
}

type moveObserver interface {
	Chose(d *Decision, index int)
}

// heroSeat is the hero's seat in the runner's next battle.
func (r *Runner) heroSeat() string {
	hero, _ := r.matchup.seats()
	return hero
}

// seats returns the hero's and the opponent's seats.
func (m Matchup) seats() (string, string) {
	if m.HeroInSeatB {
		return FoeSeat, HeroSeat
	}
	return HeroSeat, FoeSeat
}

const (
	HeroSeat = "seat-a"
	FoeSeat  = "seat-b"
)

// BattleRecord is the outcome of one real authority battle.
type BattleRecord struct {
	Seed          uint64         `json:"seed"`
	Seat          string         `json:"hero_seat"`
	Result        string         `json:"result"`
	Rounds        int            `json:"rounds"`
	HeroHealth    int            `json:"hero_health"`
	HeroMax       int            `json:"hero_max"`
	FoeHealth     int            `json:"foe_health"`
	FoeMax        int            `json:"foe_max"`
	HeroDecisions int            `json:"hero_decisions"`
	HeroThinkMS   float64        `json:"hero_think_ms"`
	Error         string         `json:"error,omitempty"`
	Searched      int            `json:"searched,omitempty"`
	Overrides     int            `json:"overrides,omitempty"`
	Futures       int            `json:"futures,omitempty"`
	Advised       int            `json:"advised,omitempty"`
	Actions       map[string]int `json:"hero_actions"`
	CardsPlayed   map[string]int `json:"hero_cards_played"`
	Abilities     map[string]int `json:"hero_abilities"`
	// HeroTurns and FoeTurns count each side's offensive turns that
	// resolved: an ability was selected, or none was and the battle went
	// on (a miss). A turn rolled but cut short by the battle's end is not
	// counted. Dealt and Taken are the committed damage against the
	// opponent and the hero by source (an ability, card or status): before
	// defense, prevented by the defense roll, prevented by reactions (cards
	// and Protect) and after both.
	HeroTurns int                           `json:"hero_turns"`
	FoeTurns  int                           `json:"foe_turns"`
	Dealt     map[string]mlsim.SourceDamage `json:"dealt,omitempty"`
	Taken     map[string]mlsim.SourceDamage `json:"taken,omitempty"`
}

// Runner plays real battles through the ML environment, the same authority
// path game sessions use; only the choices come from the policies.
type Runner struct {
	matchup  Matchup
	env      *mlsim.Environment
	sim      *Simulator
	observer Observer
}

func NewRunner(m Matchup) (*Runner, error) {
	env, err := mlsim.New(mlsim.Config{
		ContentRoot:       m.ContentRoot,
		AuthoredCardsRoot: m.AuthoredRoot,
		RunStateRoot:      m.RunStateRoot,
		AuthorityMode:     mlsim.AuthorityModeEphemeral,
		TelemetryMode:     mlsim.TelemetryModeTraining,
		SessionID:         "balance-lab",
	})
	if err != nil {
		return nil, err
	}
	sim, err := NewSimulator()
	if err != nil {
		return nil, err
	}
	runner := &Runner{matchup: m, env: env, sim: sim}
	if m.Observe != nil {
		runner.observer = m.Observe()
	}
	return runner, nil
}

// Play runs one battle to the end with the hero in seat A.
func (r *Runner) Play(seed uint64, hero, foe Policy) (BattleRecord, error) {
	record := BattleRecord{Seed: seed, Actions: map[string]int{}, CardsPlayed: map[string]int{}, Abilities: map[string]int{}}
	heroSeat, foeSeat := r.matchup.seats()
	record.Seat = heroSeat
	decks := map[string][]loadout.Entry{}
	if r.matchup.HeroDeck != nil {
		decks[heroSeat] = r.matchup.HeroDeck
	}
	if r.matchup.FoeDeck != nil {
		decks[foeSeat] = r.matchup.FoeDeck
	}
	if len(decks) == 0 {
		decks = nil
	}
	tr, err := r.env.Reset(mlsim.ResetRequest{
		SeatDecklists:   decks,
		Seed:            seed,
		BattleID:        fmt.Sprintf("lab-%d", seed),
		UnifiedDefense:  r.matchup.Unified,
		SeatDefinitions: map[string]string{heroSeat: r.matchup.Hero, foeSeat: r.matchup.Opponent},
		SeatModels:      map[string]string{heroSeat: "human-lab-" + hero.Name(), foeSeat: foe.Name()},
	})
	if err != nil {
		return record, err
	}
	if b, ok := r.observer.(battleStarter); ok {
		b.Begin(seed, heroSeat)
	}
	rng := rand.New(rand.NewSource(int64(seed)*7919 + 17))
	turns := turnCounter{}
	for !tr.Terminal && tr.TruncationReason == "" {
		if r.matchup.HeroPlansFirst && tr.ActorID != heroSeat && tr.Result.Snapshot != nil && tr.Result.Snapshot.Stage == "planning" {
			if tr, _, err = r.env.PreferActor(heroSeat); err != nil {
				return record, err
			}
		}
		full, err := r.env.InspectBattle()
		if err != nil {
			return record, err
		}
		view := tr.Result
		d := &Decision{Sim: r.sim, Battle: &full, Seat: tr.ActorID, Actions: tr.Result.LegalActions, Rand: rng, view: &view}
		policy := foe
		if tr.ActorID == heroSeat {
			policy = hero
		}
		if r.observer != nil {
			r.observer.Decision(d)
		}
		started := time.Now()
		index, err := policy.Choose(d)
		if err != nil {
			return record, fmt.Errorf("%s at round %d: %w", policy.Name(), full.Segment.Round, err)
		}
		if index < 0 || index >= len(d.Actions) {
			return record, fmt.Errorf("%s chose index %d of %d", policy.Name(), index, len(d.Actions))
		}
		if c, ok := r.observer.(moveObserver); ok {
			c.Chose(d, index)
		}
		turns.note(tr.ActorID, full.Segment.Current, d.Actions[index].Type)
		if tr.ActorID == heroSeat {
			record.HeroDecisions++
			record.HeroThinkMS += float64(time.Since(started).Microseconds()) / 1000
			noteAction(&record, &full, heroSeat, d.Actions[index])
		}
		if tr, err = r.env.Step(index); err != nil {
			return record, err
		}
	}
	final, err := r.env.InspectBattle()
	if err != nil {
		return record, err
	}
	record.HeroTurns, record.FoeTurns = turns.resolved(heroSeat), turns.resolved(foeSeat)
	damage, err := r.env.CommittedDamage()
	if err != nil {
		return record, err
	}
	record.Dealt, record.Taken = damage[foeSeat], damage[heroSeat]
	hero0, foe0 := final.Actors[heroSeat], final.Actors[foeSeat]
	record.Rounds = final.Segment.Round
	record.HeroHealth, record.HeroMax = hero0.CurrentHealth(), MaxHealth(hero0)
	record.FoeHealth, record.FoeMax = foe0.CurrentHealth(), MaxHealth(foe0)
	switch {
	case tr.TruncationReason != "":
		record.Result = "truncated"
	case final.WinnerActorID == heroSeat:
		record.Result = "win"
	case final.WinnerActorID == "":
		record.Result = "draw"
	default:
		record.Result = "loss"
	}
	if r.observer != nil {
		r.observer.Finish(&final, record, hero, foe)
	}
	return record, nil
}

// turnCounter counts each seat's offensive turns that resolved. A turn
// starts with the seat's first offensive roll; it resolved once the seat
// selects an offensive ability, or once its next turn starts (it missed).
type turnCounter map[string]*seatTurns

type seatTurns struct {
	done           int
	open, selected bool
}

func (c turnCounter) note(seat string, seg segment.Segment, action command.Type) {
	t := c[seat]
	if t == nil {
		t = &seatTurns{}
		c[seat] = t
	}
	switch {
	case action == command.TypePlanningRoll:
		if t.open {
			t.done++
		}
		t.open, t.selected = true, false
	case action == command.TypePlanningAbility && seg == segment.Offensive && t.open && !t.selected:
		t.selected = true
	}
}

func (c turnCounter) resolved(seat string) int {
	t := c[seat]
	if t == nil {
		return 0
	}
	if t.open && t.selected {
		return t.done + 1
	}
	return t.done
}

func noteAction(record *BattleRecord, b *state.Battle, seat string, action command.Command) {
	record.Actions[string(action.Type)]++
	if b.Settled.Window != nil && b.Settled.Window.Stage == "discard_to_hand_limit" {
		record.Actions["discard_to_hand_limit"]++
		return
	}
	instances := b.Settled.Actors[seat].CardInstances
	var cards []string
	switch action.Type {
	case command.TypePlanningCards:
		var p command.PlanningCardsPayload
		if json.Unmarshal(action.Payload, &p) == nil && !isProgramContinuation(p.StatusID) {
			cards = p.CardIDs
		}
	case command.TypeCommitInteraction:
		var p command.CommitInteractionPayload
		if json.Unmarshal(action.Payload, &p) == nil && !isProgramContinuation(p.Commitment.ChoiceID) {
			cards = p.Commitment.CardIDs
		}
	case command.TypePlanningAbility:
		var p command.PlanningAbilityPayload
		if json.Unmarshal(action.Payload, &p) == nil {
			record.Abilities[p.AbilityID]++
		}
	}
	for _, id := range cards {
		record.CardsPlayed[instances[id].DefinitionID]++
	}
}

// isProgramContinuation reports a later step of a card already being played
// (its die or target choice), which must not count as a second play.
func isProgramContinuation(statusID string) bool {
	var choice map[string]any
	if json.Unmarshal([]byte(statusID), &choice) != nil {
		return false
	}
	verb, _ := choice["verb"].(string)
	return verb != "" && verb != "start"
}
