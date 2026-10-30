package snapshot

import (
	"crypto/sha256"
	"sort"
	"sync"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

// TacticsView is a tactics battle as one viewer may see it: the grid, where
// every creature stands, the revealed attacks and the last resolved
// movement. A participant's movement plan, and an area attack's square while
// planning, are only ever in their own view until the reveal.
type TacticsView struct {
	Arena           TacticsArenaView               `json:"arena"`
	Rules           tactics.Rules                  `json:"rules"`
	Creatures       map[string]TacticsCreatureView `json:"creatures"`
	Step            string                         `json:"step,omitempty"`
	OffenseRevealed bool                           `json:"offense_revealed,omitempty"`
	Sprinting       []string                       `json:"sprinting,omitempty"`
	Attacks         []state.TacticsAttack          `json:"attacks,omitempty"`
	// OwnSquare is the viewer's own area target square while planning.
	OwnSquare *tactics.Square `json:"own_square,omitempty"`
	// Plan is the viewer's own movement plan (draft or locked) with its Move
	// budget and preview path.
	Plan *state.TacticsPlan `json:"plan,omitempty"`
	// HandMove is, during the movement step, what each card in the viewer's
	// own hand would add to their plan's Move.
	HandMove map[string]TacticsCardMove `json:"hand_move,omitempty"`
	LockedIn []string                   `json:"locked_in,omitempty"`
	Movement *TacticsMovementView       `json:"movement,omitempty"`
	// DisabledDefenses are the viewer's defenses unavailable now, with why.
	DisabledDefenses map[string]string `json:"disabled_defenses,omitempty"`
}

// TacticsCardMove is one hand card's Move: discarded, and (for a map Move
// card) played for its amount.
type TacticsCardMove struct {
	Discard int `json:"discard"`
	Map     int `json:"map,omitempty"`
}

type TacticsArenaView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	// Terrain lists each row's terrain IDs, top row first.
	Terrain [][]string `json:"terrain"`
}

type TacticsCreatureView struct {
	Position   tactics.Square          `json:"position"`
	RoundStart *tactics.Square         `json:"round_start,omitempty"`
	BaseSpeed  int                     `json:"base_speed"`
	Role       string                  `json:"role,omitempty"`
	Label      string                  `json:"label,omitempty"`
	Defeated   bool                    `json:"defeated,omitempty"`
	Deliveries map[string]DeliveryView `json:"deliveries,omitempty"`
}

// DeliveryView is an offensive ability's effective delivery with its text.
type DeliveryView struct {
	content.AbilityDelivery
	Text string `json:"text"`
}

// TacticsMovementView is the public record of a resolved movement. Other
// participants' discarded cards show only as a count; played map cards are
// shown by their definition.
type TacticsMovementView struct {
	Round    int                           `json:"round"`
	Start    map[string]tactics.Square     `json:"start"`
	Final    map[string]tactics.Square     `json:"final"`
	Paths    map[string][]tactics.Waypoint `json:"paths"`
	Timeline []tactics.Entry               `json:"timeline"`
	Budgets  map[string]tactics.MoveBudget `json:"budgets"`
	Running  []string                      `json:"running,omitempty"`
	Plans    map[string]TacticsPublicPlan  `json:"plans"`
	Contact  map[string]tactics.Frac       `json:"contact,omitempty"`
	Attacks  []state.TacticsAttack         `json:"attacks,omitempty"`
	Hits     []state.TacticsHitResult      `json:"hits,omitempty"`
}

type TacticsPublicPlan struct {
	Intention tactics.Intention   `json:"intention"`
	Cards     []string            `json:"cards,omitempty"`
	CardCount int                 `json:"card_count"`
	MapCards  []TacticsPlayedCard `json:"map_cards,omitempty"`
	Energy    int                 `json:"energy,omitempty"`
	Sprint    bool                `json:"sprint,omitempty"`
	Charge    string              `json:"charge,omitempty"`
	Budget    tactics.MoveBudget  `json:"budget"`
}

type TacticsPlayedCard struct {
	CardDefinitionID string `json:"card_definition_id"`
	Terrain          string `json:"terrain,omitempty"`
}

// tacticsCatalogs caches decoded pinned catalogs by their bytes' hash; the
// tactics view only reads them.
var tacticsCatalogs sync.Map

func tacticsCatalog(battle state.Battle) *ContentCatalog {
	key := sha256.Sum256(battle.SettledCatalog)
	if cached, ok := tacticsCatalogs.Load(key); ok {
		return cached.(*ContentCatalog)
	}
	catalog := settledContentCatalog(battle)
	if catalog != nil {
		tacticsCatalogs.Store(key, catalog)
	}
	return catalog
}

func tacticsView(battle state.Battle, viewer string) *TacticsView {
	if battle.Settled == nil || !battle.Settled.Tactics || battle.Settled.TacticsRuntime == nil {
		return nil
	}
	catalog := tacticsCatalog(battle)
	rt := battle.Settled.TacticsRuntime
	view := &TacticsView{Rules: rt.Rules, Creatures: map[string]TacticsCreatureView{}, OffenseRevealed: rt.OffenseRevealed}
	view.Arena = TacticsArenaView{ID: rt.Arena.ID, Name: rt.Arena.Name, Description: rt.Arena.Description, Width: rt.Arena.Width(), Height: rt.Arena.Height()}
	for y := 0; y < rt.Arena.Height(); y++ {
		row := make([]string, rt.Arena.Width())
		for x := range row {
			row[x] = rt.Arena.TerrainAt(tactics.Square{X: x, Y: y})
		}
		view.Arena.Terrain = append(view.Arena.Terrain, row)
	}
	fallback := content.AbilityDelivery{Type: rt.Rules.DefaultDelivery.Type, Range: rt.Rules.DefaultDelivery.Range, LineOfSight: rt.Rules.DefaultDelivery.LineOfSight}
	for id, actor := range battle.Actors {
		creature := rt.Creatures[id]
		c := TacticsCreatureView{Position: rt.Positions[id], BaseSpeed: creature.BaseSpeed, Role: creature.Role, Label: creature.Label, Defeated: actor.DefeatState == state.ActorDefeated}
		if start, ok := rt.RoundStart[id]; ok {
			s := start
			c.RoundStart = &s
		}
		if catalog != nil {
			c.Deliveries = map[string]DeliveryView{}
			for _, abilityID := range battle.Settled.Actors[id].OffensiveAbilityIDs {
				d := content.EffectiveDelivery(catalog.Abilities[abilityID], fallback)
				c.Deliveries[abilityID] = DeliveryView{AbilityDelivery: d, Text: content.DeliveryText(d)}
			}
		}
		view.Creatures[id] = c
	}
	if rt.OffenseRevealed {
		view.Attacks = append([]state.TacticsAttack(nil), rt.Attacks...)
		for id, sprint := range rt.Sprinting {
			if sprint {
				view.Sprinting = append(view.Sprinting, id)
			}
		}
		sort.Strings(view.Sprinting)
	} else if square, ok := rt.Squares[viewer]; ok {
		s := square
		view.OwnSquare = &s
	}
	if sim := battle.Settled.Simultaneous; sim != nil && sim.Step == state.SimultaneousMovement {
		view.Step = state.SimultaneousMovement
		for id, locked := range sim.Locked {
			if locked {
				view.LockedIn = append(view.LockedIn, id)
			}
		}
		sort.Strings(view.LockedIn)
	}
	if plan, ok := rt.Plans[viewer]; ok {
		p := plan
		view.Plan = &p
		if view.Step == state.SimultaneousMovement && catalog != nil && !plan.Locked && (plan.Charge == "" || rt.Rules.ChargeAllowsExtraMove) {
			view.HandMove = map[string]TacticsCardMove{}
			run := battle.Settled.Actors[viewer]
			for _, card := range battle.Actors[viewer].Cards.Hand {
				move := TacticsCardMove{Discard: tactics.CardMove(rt.Rules, plan.Sprint)}
				if effect := catalog.Cards[run.CardInstances[card].DefinitionID].MapEffect; effect != nil && effect.Kind == "move" {
					move.Map = tactics.MapCardMove(rt.Rules, effect.Amount, plan.Sprint)
				}
				view.HandMove[card] = move
			}
		}
	}
	if m := rt.Movement; m != nil {
		mv := &TacticsMovementView{Round: m.Round, Start: m.Start, Final: m.Result.Final, Paths: m.Result.Paths, Timeline: m.Result.Timeline, Budgets: m.Budgets, Running: m.Running, Contact: m.Result.Contact, Plans: map[string]TacticsPublicPlan{}, Attacks: m.Attacks, Hits: m.Hits}
		for id, plan := range m.Plans {
			public := TacticsPublicPlan{Intention: plan.Intention, CardCount: len(plan.Cards), Energy: plan.Energy, Sprint: plan.Sprint, Charge: plan.Charge, Budget: plan.Budget}
			if id == viewer {
				public.Cards = append([]string(nil), plan.Cards...)
			}
			for _, played := range plan.MapCards {
				public.MapCards = append(public.MapCards, TacticsPlayedCard{CardDefinitionID: battle.Settled.Actors[id].CardInstances[played.CardID].DefinitionID, Terrain: played.Terrain})
			}
			mv.Plans[id] = public
		}
		view.Movement = mv
	}
	if catalog != nil {
		limited := false
		for _, status := range battle.Actors[viewer].Statuses {
			if m := catalog.Statuses[status.DefinitionID].Modifiers; m != nil && m.OnTheMoveDefensesOnly && status.Stacks > 0 {
				limited = true
			}
		}
		if limited {
			view.DisabledDefenses = map[string]string{}
			for _, id := range battle.Settled.Actors[viewer].DefensiveAbilityIDs {
				if !catalog.Abilities[id].OnTheMove {
					view.DisabledDefenses[id] = "Running: only on-the-move defenses this round"
				}
			}
		}
	}
	return view
}
