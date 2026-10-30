// Package tactics is the pure grid rules of the Tactics Sandbox
// (docs/game-design/tactics-sandbox-plan.md): squares, distance, line of
// sight, pathing over terrain, movement intentions and the exact-time
// simultaneous movement simulation with its collisions and tiebreaks. It has
// no engine imports; the battle engine calls it. Every number comes from
// configuration (content/tactics_v1).
package tactics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrInvalidConfig marks a rules or arena file the strict loader refuses.
var ErrInvalidConfig = errors.New("invalid tactics configuration")

// Diagonal rules.
const (
	// DiagonalChebyshev counts every diagonal step as 1 (the D&D 5e default).
	DiagonalChebyshev = "chebyshev"
	// DiagonalAlternating counts diagonal steps 1, 2, 1, 2… (the 5-10-5 variant).
	DiagonalAlternating = "alternating"
)

// Tiebreak rules for two opponents entering the same square at the same
// moment, in their configured order.
const (
	TiebreakSquaresMoved = "squares_moved"
	TiebreakMoveRate     = "move_rate"
	TiebreakPaidMove     = "paid_move"
	TiebreakRandom       = "random"
)

// Square is one grid square; x is the column and y the row, from the top
// left corner.
type Square struct {
	X int `yaml:"x" json:"x"`
	Y int `yaml:"y" json:"y"`
}

func (s Square) String() string { return fmt.Sprintf("(%d,%d)", s.X, s.Y) }

// Terrain is one terrain type: what entering a square costs, whether it can
// be entered at all and whether it blocks line of sight.
type Terrain struct {
	Name              string `yaml:"name" json:"name"`
	Cost              int    `yaml:"cost" json:"cost"`
	Passable          bool   `yaml:"passable" json:"passable"`
	BlocksLineOfSight bool   `yaml:"blocks_line_of_sight" json:"blocks_line_of_sight"`
	Color             string `yaml:"color" json:"color"`
	Description       string `yaml:"description,omitempty" json:"description,omitempty"`
}

// DefaultDelivery is the delivery of an offensive ability that declares none.
type DefaultDelivery struct {
	Type        string `yaml:"type" json:"type"`
	Range       int    `yaml:"range" json:"range"`
	LineOfSight bool   `yaml:"line_of_sight" json:"line_of_sight"`
}

// AIRules tune the scripted enemies.
type AIRules struct {
	// MaxCardsPerRound is how many cards an enemy may commit to movement in
	// one round when that gets it into range next round.
	MaxCardsPerRound int `yaml:"max_cards_per_round" json:"max_cards_per_round"`
}

// Rules is content/tactics_v1/rules.yaml. A battle pins its rules (with any
// setup overrides) at start, so later edits never change a running battle.
type Rules struct {
	SchemaVersion         int                `yaml:"schema_version" json:"schema_version"`
	BaseSpeed             int                `yaml:"base_speed" json:"base_speed"`
	CardMove              int                `yaml:"card_move" json:"card_move"`
	EnergyPerMove         int                `yaml:"energy_per_move" json:"energy_per_move"`
	MapCardMoveMultiplier int                `yaml:"map_card_move_multiplier" json:"map_card_move_multiplier"`
	SprintBaseMultiplier  int                `yaml:"sprint_base_multiplier" json:"sprint_base_multiplier"`
	SprintCardBonus       int                `yaml:"sprint_card_bonus" json:"sprint_card_bonus"`
	DiagonalRule          string             `yaml:"diagonal_rule" json:"diagonal_rule"`
	CornerCutting         bool               `yaml:"corner_cutting" json:"corner_cutting"`
	LOSBlockedByCreatures bool               `yaml:"los_blocked_by_creatures" json:"los_blocked_by_creatures"`
	AreaHitsAllies        bool               `yaml:"area_hits_allies" json:"area_hits_allies"`
	ChargeCountsAsRunning bool               `yaml:"charge_counts_as_running" json:"charge_counts_as_running"`
	ChargeAllowsExtraMove bool               `yaml:"charge_allows_extra_move" json:"charge_allows_extra_move"`
	TiebreakOrder         []string           `yaml:"tiebreak_order" json:"tiebreak_order"`
	RunningStatus         string             `yaml:"running_status" json:"running_status"`
	DefaultDelivery       DefaultDelivery    `yaml:"default_delivery" json:"default_delivery"`
	Terrain               map[string]Terrain `yaml:"terrain" json:"terrain"`
	AI                    AIRules            `yaml:"ai" json:"ai"`
}

// Arena is one hand-made battlefield: a terrain grid (one legend glyph per
// square), the player's start square and up to four enemy start squares,
// used in order.
type Arena struct {
	SchemaVersion int               `yaml:"schema_version" json:"schema_version"`
	ID            string            `yaml:"id" json:"id"`
	Name          string            `yaml:"name" json:"name"`
	Order         int               `yaml:"order" json:"order"`
	Description   string            `yaml:"description" json:"description"`
	Legend        map[string]string `yaml:"legend" json:"legend"`
	Grid          []string          `yaml:"grid" json:"grid"`
	PlayerStart   Square            `yaml:"player_start" json:"player_start"`
	EnemyStarts   []Square          `yaml:"enemy_starts" json:"enemy_starts"`
}

// Width and Height are the arena's size in squares.
func (a Arena) Width() int {
	if len(a.Grid) == 0 {
		return 0
	}
	return len([]rune(a.Grid[0]))
}
func (a Arena) Height() int { return len(a.Grid) }

// TerrainAt is the terrain ID of a square (empty outside the arena).
func (a Arena) TerrainAt(s Square) string {
	if s.Y < 0 || s.Y >= len(a.Grid) {
		return ""
	}
	row := []rune(a.Grid[s.Y])
	if s.X < 0 || s.X >= len(row) {
		return ""
	}
	return a.Legend[string(row[s.X])]
}

// Pack is the loaded content/tactics_v1 configuration.
type Pack struct {
	Rules  Rules
	Arenas map[string]Arena
}

// SortedArenas lists the arenas in their menu order.
func (p Pack) SortedArenas() []Arena {
	out := make([]Arena, 0, len(p.Arenas))
	for _, a := range p.Arenas {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// LoadPack reads rules.yaml and arenas/*.yaml strictly: unknown fields,
// out-of-range numbers and inconsistent arenas are refused.
func LoadPack(root string) (Pack, error) {
	rules, err := LoadRules(filepath.Join(root, "rules.yaml"))
	if err != nil {
		return Pack{}, err
	}
	paths, err := filepath.Glob(filepath.Join(root, "arenas", "*.yaml"))
	if err != nil {
		return Pack{}, err
	}
	sort.Strings(paths)
	pack := Pack{Rules: rules, Arenas: map[string]Arena{}}
	for _, path := range paths {
		var arena Arena
		if err := decodeStrictFile(path, &arena); err != nil {
			return Pack{}, err
		}
		if err := ValidateArena(arena, rules); err != nil {
			return Pack{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if _, dup := pack.Arenas[arena.ID]; dup {
			return Pack{}, fmt.Errorf("%w: duplicate arena %q", ErrInvalidConfig, arena.ID)
		}
		pack.Arenas[arena.ID] = arena
	}
	if len(pack.Arenas) == 0 {
		return Pack{}, fmt.Errorf("%w: no arenas in %s", ErrInvalidConfig, root)
	}
	return pack, nil
}

// LoadRules reads one rules file strictly.
func LoadRules(path string) (Rules, error) {
	var rules Rules
	if err := decodeStrictFile(path, &rules); err != nil {
		return Rules{}, err
	}
	if err := ValidateRules(rules); err != nil {
		return Rules{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return rules, nil
}

func decodeStrictFile(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%w: parse %s: %v", ErrInvalidConfig, filepath.Base(path), err)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidConfig, fmt.Sprintf(format, args...))
}

func between(name string, value, low, high int) error {
	if value < low || value > high {
		return invalid("%s must be %d–%d, got %d", name, low, high, value)
	}
	return nil
}

// ValidateRules checks every number and choice of a rules file.
func ValidateRules(r Rules) error {
	if r.SchemaVersion != 1 {
		return invalid("schema_version must be 1")
	}
	for _, check := range []error{
		between("base_speed", r.BaseSpeed, 1, 20),
		between("card_move", r.CardMove, 0, 10),
		between("energy_per_move", r.EnergyPerMove, 1, 100),
		between("map_card_move_multiplier", r.MapCardMoveMultiplier, 0, 10),
		between("sprint_base_multiplier", r.SprintBaseMultiplier, 1, 10),
		between("sprint_card_bonus", r.SprintCardBonus, 0, 10),
		between("ai.max_cards_per_round", r.AI.MaxCardsPerRound, 0, 10),
	} {
		if check != nil {
			return check
		}
	}
	if r.DiagonalRule != DiagonalChebyshev && r.DiagonalRule != DiagonalAlternating {
		return invalid("diagonal_rule must be chebyshev or alternating")
	}
	if len(r.TiebreakOrder) == 0 || r.TiebreakOrder[len(r.TiebreakOrder)-1] != TiebreakRandom {
		return invalid("tiebreak_order must end with random")
	}
	seen := map[string]bool{}
	for _, rule := range r.TiebreakOrder {
		switch rule {
		case TiebreakSquaresMoved, TiebreakMoveRate, TiebreakPaidMove, TiebreakRandom:
		default:
			return invalid("unknown tiebreak %q", rule)
		}
		if seen[rule] {
			return invalid("tiebreak %q listed twice", rule)
		}
		seen[rule] = true
	}
	if strings.TrimSpace(r.RunningStatus) == "" {
		return invalid("running_status is required")
	}
	if r.DefaultDelivery.Type != "direct" || r.DefaultDelivery.Range < 1 || r.DefaultDelivery.Range > 100 {
		return invalid("default_delivery must be direct with range 1–100")
	}
	if len(r.Terrain) == 0 {
		return invalid("terrain table is empty")
	}
	passable := false
	for id, t := range r.Terrain {
		if id == "" || strings.TrimSpace(t.Name) == "" {
			return invalid("terrain %q needs a name", id)
		}
		if !strings.HasPrefix(t.Color, "#") || (len(t.Color) != 7 && len(t.Color) != 9) {
			return invalid("terrain %q color must be #rrggbb", id)
		}
		if t.Passable {
			passable = true
			if err := between("terrain "+id+" cost", t.Cost, 1, 10); err != nil {
				return err
			}
		} else if t.Cost != 0 {
			return invalid("impassable terrain %q must have cost 0", id)
		}
	}
	if !passable {
		return invalid("at least one terrain must be passable")
	}
	return nil
}

// ValidateArena checks an arena against the rules' terrain table.
func ValidateArena(a Arena, rules Rules) error {
	if a.SchemaVersion != 1 {
		return invalid("arena schema_version must be 1")
	}
	if a.ID == "" || strings.TrimSpace(a.Name) == "" {
		return invalid("arena id and name are required")
	}
	if a.Height() < 3 || a.Height() > 40 {
		return invalid("arena %q must have 3–40 rows", a.ID)
	}
	width := a.Width()
	if width < 3 || width > 40 {
		return invalid("arena %q must have 3–40 columns", a.ID)
	}
	for glyph, terrain := range a.Legend {
		if len([]rune(glyph)) != 1 {
			return invalid("arena %q legend keys are single characters", a.ID)
		}
		if _, ok := rules.Terrain[terrain]; !ok {
			return invalid("arena %q legend names unknown terrain %q", a.ID, terrain)
		}
	}
	for y, row := range a.Grid {
		runes := []rune(row)
		if len(runes) != width {
			return invalid("arena %q row %d has %d squares, want %d", a.ID, y, len(runes), width)
		}
		for x, r := range runes {
			if _, ok := a.Legend[string(r)]; !ok {
				return invalid("arena %q square (%d,%d) uses glyph %q missing from the legend", a.ID, x, y, string(r))
			}
		}
	}
	if len(a.EnemyStarts) < 1 || len(a.EnemyStarts) > 4 {
		return invalid("arena %q needs 1–4 enemy start squares", a.ID)
	}
	starts := append([]Square{a.PlayerStart}, a.EnemyStarts...)
	seen := map[Square]bool{}
	for _, s := range starts {
		terrain, ok := rules.Terrain[a.TerrainAt(s)]
		if !ok || !terrain.Passable {
			return invalid("arena %q start %v is outside the grid or impassable", a.ID, s)
		}
		if seen[s] {
			return invalid("arena %q uses start %v twice", a.ID, s)
		}
		seen[s] = true
	}
	return nil
}

// ApplyOverrides returns the rules with a partial override applied (the
// setup screen's per-battle changes). Unknown keys and invalid results are
// refused; nested objects (terrain, ai, default_delivery) merge by key.
func ApplyOverrides(rules Rules, overrides json.RawMessage) (Rules, error) {
	if len(bytes.TrimSpace(overrides)) == 0 || string(bytes.TrimSpace(overrides)) == "null" {
		return rules, nil
	}
	base, err := json.Marshal(rules)
	if err != nil {
		return Rules{}, err
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return Rules{}, err
	}
	var patch map[string]any
	if err := json.Unmarshal(overrides, &patch); err != nil {
		return Rules{}, invalid("rule overrides must be an object: %v", err)
	}
	if err := mergeOverride(merged, patch, ""); err != nil {
		return Rules{}, err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return Rules{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var out Rules
	if err := decoder.Decode(&out); err != nil {
		return Rules{}, invalid("rule overrides: %v", err)
	}
	if err := ValidateRules(out); err != nil {
		return Rules{}, err
	}
	return out, nil
}

func mergeOverride(target, patch map[string]any, path string) error {
	for key, value := range patch {
		existing, ok := target[key]
		if !ok {
			// New terrain types cannot be invented by an override.
			return invalid("unknown rule %q", path+key)
		}
		if child, isMap := value.(map[string]any); isMap {
			if into, intoMap := existing.(map[string]any); intoMap {
				if err := mergeOverride(into, child, path+key+"."); err != nil {
					return err
				}
				continue
			}
		}
		target[key] = value
	}
	return nil
}
