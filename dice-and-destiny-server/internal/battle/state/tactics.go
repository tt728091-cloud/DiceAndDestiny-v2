package state

import (
	"encoding/json"

	"diceanddestiny/server/internal/battle/tactics"
)

// SimultaneousMovement is the tactics movement step: every participant locks
// in one secret movement plan after the attacks are revealed.
const SimultaneousMovement = "movement"

// TacticsRuntime is a tactics battle's grid state
// (docs/game-design/tactics-sandbox-plan.md): the arena and rules pinned at
// start, where every creature stands, this round's attacks and movement
// plans, and the most recent resolved movement.
type TacticsRuntime struct {
	Arena     tactics.Arena              `json:"arena"`
	Rules     tactics.Rules              `json:"rules"`
	Creatures map[string]TacticsCreature `json:"creatures"`
	// Positions are where creatures stand now; RoundStart is where they
	// stood when this round's Offense began (range and line of sight are
	// measured from there).
	Positions  map[string]tactics.Square `json:"positions"`
	RoundStart map[string]tactics.Square `json:"round_start,omitempty"`
	// Squares are area attacks' target squares chosen while planning,
	// private until the offense reveal.
	Squares map[string]tactics.Square `json:"squares,omitempty"`
	// Sprinting lists participants who skipped this round's offense; it is
	// public once the offense is revealed.
	Sprinting       map[string]bool `json:"sprinting,omitempty"`
	OffenseRevealed bool            `json:"offense_revealed,omitempty"`
	// Attacks are this round's revealed attacks with their delivery; the
	// movement reveal records whether each connected.
	Attacks []TacticsAttack `json:"attacks,omitempty"`
	// Plans are this round's movement plans: drafts and lock-ins, private to
	// their owner until the movement reveal.
	Plans map[string]TacticsPlan `json:"plans,omitempty"`
	// Movement is the last resolved movement, public from its reveal until
	// the next one.
	Movement *TacticsMovement `json:"movement,omitempty"`
	// History holds every earlier resolved round, oldest first, so a
	// battle can be replayed whole; Movement is the latest.
	History []TacticsMovement `json:"history,omitempty"`
}

// TacticsCreature is a combatant's grid profile.
type TacticsCreature struct {
	BaseSpeed int    `json:"base_speed"`
	Role      string `json:"role,omitempty"`
	Label     string `json:"label,omitempty"`
}

// Attack outcomes on the grid.
const (
	TacticsHit        = "hit"
	TacticsDodged     = "dodged"
	TacticsOutOfReach = "out_of_reach"
)

// TacticsAttack is one revealed attack: who attacks with what, how it is
// delivered and what it aims at, then whether it connected.
type TacticsAttack struct {
	Attacker    string           `json:"attacker"`
	AbilityID   string           `json:"ability_id"`
	TierID      string           `json:"tier_id,omitempty"`
	Delivery    string           `json:"delivery"`
	Range       int              `json:"range"`
	LineOfSight bool             `json:"line_of_sight"`
	Target      string           `json:"target,omitempty"`
	Square      *tactics.Square  `json:"square,omitempty"`
	Area        []tactics.Square `json:"area,omitempty"`
	ChargeRange int              `json:"charge_range,omitempty"`
	Resolved    bool             `json:"resolved,omitempty"`
	Outcome     string           `json:"outcome,omitempty"`
	Hits        []string         `json:"hits,omitempty"`
	ContactAt   *tactics.Frac    `json:"contact_at,omitempty"`
}

// Attack deliveries as revealed (a charge is a direct ability with a reach).
const (
	DeliveryDirect = "direct"
	DeliveryArea   = "area"
	DeliveryCharge = "charge"
)

// TacticsPlan is one participant's movement plan: an intention and the exact
// cards (discarded for Move), map Move cards (played for their amount) and
// energy it commits. The authority keeps the plan's Move budget and a preview
// path that assumes everyone else holds.
type TacticsPlan struct {
	Intention tactics.Intention  `json:"intention"`
	Cards     []string           `json:"cards,omitempty"`
	MapCards  []TacticsMapCard   `json:"map_cards,omitempty"`
	Energy    int                `json:"energy,omitempty"`
	Sprint    bool               `json:"sprint,omitempty"`
	Charge    string             `json:"charge,omitempty"`
	Locked    bool               `json:"locked,omitempty"`
	Budget    tactics.MoveBudget `json:"budget"`
	Preview   *TacticsPreview    `json:"preview,omitempty"`
}

// TacticsMapCard is a map Move card played in a plan; Terrain is the terrain
// a choose_terrain card names.
type TacticsMapCard struct {
	CardID  string `json:"card_id"`
	Terrain string `json:"terrain,omitempty"`
}

// TacticsPreview is the projected path of a plan if everyone else holds.
type TacticsPreview struct {
	Path   []tactics.Waypoint `json:"path"`
	Final  tactics.Square     `json:"final"`
	Stop   string             `json:"stop,omitempty"`
	Spent  int                `json:"spent"`
	Arrive *tactics.Frac      `json:"arrive,omitempty"`
}

// TacticsMovement is one resolved round of movement.
type TacticsMovement struct {
	Round   int                           `json:"round"`
	Start   map[string]tactics.Square     `json:"start"`
	Result  tactics.Result                `json:"result"`
	Plans   map[string]TacticsPlan        `json:"plans"`
	Budgets map[string]tactics.MoveBudget `json:"budgets"`
	Running []string                      `json:"running,omitempty"`
	Seed    uint64                        `json:"seed"`
	// Attacks are the round's attacks with whether each connected; Hits
	// are filled in when the round's damage commits (for the playback).
	Attacks []TacticsAttack    `json:"attacks,omitempty"`
	Hits    []TacticsHitResult `json:"hits,omitempty"`
	// HealthBefore is each creature's health when the movement resolved;
	// HealthAfter its health once the round's damage committed.
	HealthBefore map[string]int `json:"health_before,omitempty"`
	HealthAfter  map[string]int `json:"health_after,omitempty"`
}

// TacticsHitResult is one connected attack's committed result on one creature.
type TacticsHitResult struct {
	SourceID  string `json:"source_id"`
	Attacker  string `json:"attacker"`
	AbilityID string `json:"ability_id"`
	Target    string `json:"target"`
	Base      int    `json:"base"`
	Final     int    `json:"final"`
	Removed   int    `json:"removed"`
	Defense   string `json:"defense,omitempty"`
	Lethal    bool   `json:"lethal,omitempty"`
}

// CloneTactics deep-copies the tactics runtime.
func CloneTactics(value *TacticsRuntime) *TacticsRuntime {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var out TacticsRuntime
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return &out
}
