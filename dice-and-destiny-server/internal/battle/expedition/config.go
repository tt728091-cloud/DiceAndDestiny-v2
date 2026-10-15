package expedition

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"diceanddestiny/server/internal/content"
	"gopkg.in/yaml.v3"
)

// Range is a YAML value that is either a single integer or a [min, max] pair.
type Range struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

func (r *Range) UnmarshalYAML(value *yaml.Node) error {
	var single int
	if err := value.Decode(&single); err == nil {
		r.Min, r.Max = single, single
		return nil
	}
	var pair []int
	if err := value.Decode(&pair); err != nil || len(pair) != 2 {
		return fmt.Errorf("a range is one number or [min, max], not %q", value.Value)
	}
	r.Min, r.Max = pair[0], pair[1]
	return nil
}

func (r Range) Contains(v int) bool { return v >= r.Min && v <= r.Max }

// TerrainConfig is one terrain's cost by Day and Night. Impassable terrain has
// no cost; cover counts as "unseen" when the player attacks from it.
type TerrainConfig struct {
	Day        int  `yaml:"day" json:"day"`
	Night      int  `yaml:"night" json:"night"`
	Cover      bool `yaml:"cover,omitempty" json:"cover,omitempty"`
	Impassable bool `yaml:"impassable,omitempty" json:"impassable,omitempty"`
}

type HandConfig struct {
	Size  int `yaml:"size" json:"size"`
	Limit int `yaml:"limit" json:"limit"`
}

type MoveConfig struct {
	EnergyPerMove int `yaml:"energy_per_move" json:"energy_per_move"`
	ExploreCost   int `yaml:"explore_cost" json:"explore_cost"`
}

type DayNight struct {
	Day   int `yaml:"day" json:"day"`
	Night int `yaml:"night" json:"night"`
}

type AmbushConfig struct {
	BonusDice int `yaml:"bonus_dice" json:"bonus_dice"`
	MaxKept   int `yaml:"max_kept" json:"max_kept"`
}

type DefenseDiceConfig struct {
	ExtraDefenseDice int `yaml:"extra_defense_dice" json:"extra_defense_dice"`
}

type DefeatConfig struct {
	Wounds int `yaml:"wounds" json:"wounds"`
}

// CampHealingConfig: at rest every character's basic aid heals a
// basic_aid_die roll of one untreated wound (once per wound per camp); a
// bandage then heals a bandage_die roll of a treated wound (once per wound
// per camp); inn_bonus is added to each roll at an inn.
type CampHealingConfig struct {
	BasicAidDie int `yaml:"basic_aid_die" json:"basic_aid_die"`
	BandageDie  int `yaml:"bandage_die" json:"bandage_die"`
	InnBonus    int `yaml:"inn_bonus" json:"inn_bonus"`
}

type EarlyReward struct {
	XPPerRound        int `yaml:"xp_per_round" json:"xp_per_round"`
	ExtraRelicChoices int `yaml:"extra_relic_choices" json:"extra_relic_choices"`
}

type WarlordConfig struct {
	StartDistance int         `yaml:"start_distance" json:"start_distance"`
	MarchPerRest  int         `yaml:"march_per_rest" json:"march_per_rest"`
	EarlyReward   EarlyReward `yaml:"early_reward" json:"early_reward"`
}

type MendingConfig struct {
	VillageInfluencePerWound   int `yaml:"village_influence_per_wound" json:"village_influence_per_wound"`
	MonasteryInfluencePerWound int `yaml:"monastery_influence_per_wound" json:"monastery_influence_per_wound"`
	TownGoldPerWound           int `yaml:"town_gold_per_wound" json:"town_gold_per_wound"`
}

// GoldConfig holds gold drops and prices. Gold buys convenience and
// recovery, never XP.
type GoldConfig struct {
	Drops  map[string]Range `yaml:"drops" json:"drops"`
	Prices map[string]Range `yaml:"prices" json:"prices"`
}

// StatusIDs names the authored battle statuses the expedition applies.
type StatusIDs struct {
	Ambush    string `yaml:"ambush" json:"ambush"`
	Fortified string `yaml:"fortified" json:"fortified"`
	Conqueror string `yaml:"conqueror" json:"conqueror"`
	// Lieutenant is the Warlord's buff: one stack per living Lieutenant.
	Lieutenant string `yaml:"lieutenant" json:"lieutenant"`
}

// Config holds every rule number of the Expedition. Nothing in this package
// hard-codes a value that appears here.
type Config struct {
	Version             int                      `yaml:"schema_version" json:"-"`
	RoundsPerRegion     int                      `yaml:"rounds_per_region" json:"rounds_per_region"`
	Hand                HandConfig               `yaml:"hand" json:"hand"`
	Move                MoveConfig               `yaml:"move" json:"move"`
	Terrain             map[string]TerrainConfig `yaml:"terrain" json:"terrain"`
	Sight               DayNight                 `yaml:"sight" json:"sight"`
	Ambush              AmbushConfig             `yaml:"ambush" json:"ambush"`
	Fortified           DefenseDiceConfig        `yaml:"fortified" json:"fortified"`
	Conqueror           DefenseDiceConfig        `yaml:"conqueror" json:"conqueror"`
	Defeat              DefeatConfig             `yaml:"defeat" json:"defeat"`
	CampHealing         CampHealingConfig        `yaml:"camp_healing" json:"camp_healing"`
	Warlord             WarlordConfig            `yaml:"warlord" json:"warlord"`
	DeckChangeInfluence map[string]int           `yaml:"deck_change_influence" json:"deck_change_influence"`
	Mending             MendingConfig            `yaml:"mending" json:"mending"`
	Gold                GoldConfig               `yaml:"gold" json:"gold"`
	Statuses            StatusIDs                `yaml:"statuses" json:"statuses"`
}

// Terrain names every terrain the config must define.
var requiredTerrain = []string{"plains", "hills", "forest", "marsh", "lake", "mountain"}

// Site tiers that price deck changes.
var deckChangeSites = []string{"camp", "village", "monastery", "town"}

var requiredDrops = []string{"minion_group", "lieutenant", "treasure", "event", "warlord"}
var requiredPrices = []string{"bandage", "consumable", "gear", "healer_grievous"}

const maxValue = 1000000

// LoadConfig reads content/progression_v1/expedition.yaml strictly: unknown
// fields, a wrong schema version and out-of-range numbers are errors. lib is
// the character library the statuses must exist in.
func LoadConfig(root string, lib content.BattleLibrary) (Config, error) {
	var c Config
	data, err := os.ReadFile(filepath.Join(root, "progression_v1", "expedition.yaml"))
	if err != nil {
		return c, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err = decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("expedition config: %w", err)
	}
	return c, c.validate(lib)
}

func inRange(name string, v, lo, hi int) error {
	if v < lo || v > hi {
		return fmt.Errorf("expedition config: %s must be %d–%d, not %d", name, lo, hi, v)
	}
	return nil
}

func (c Config) validate(lib content.BattleLibrary) error {
	if c.Version != 1 {
		return fmt.Errorf("expedition config: schema_version must be 1")
	}
	checks := []struct {
		name   string
		v      int
		lo, hi int
	}{
		{"rounds_per_region", c.RoundsPerRegion, 1, 100},
		{"hand.size", c.Hand.Size, 1, 20},
		{"hand.limit", c.Hand.Limit, c.Hand.Size, 20},
		{"move.energy_per_move", c.Move.EnergyPerMove, 1, maxValue},
		{"move.explore_cost", c.Move.ExploreCost, 0, 100},
		{"sight.day", c.Sight.Day, 1, 10},
		{"sight.night", c.Sight.Night, 1, 10},
		{"ambush.bonus_dice", c.Ambush.BonusDice, 1, 10},
		{"ambush.max_kept", c.Ambush.MaxKept, content.LoadoutDiceCount, content.LoadoutDiceCount},
		{"fortified.extra_defense_dice", c.Fortified.ExtraDefenseDice, 1, 10},
		{"conqueror.extra_defense_dice", c.Conqueror.ExtraDefenseDice, 1, 10},
		{"defeat.wounds", c.Defeat.Wounds, 0, 100},
		{"camp_healing.basic_aid_die", c.CampHealing.BasicAidDie, 1, 20},
		{"camp_healing.bandage_die", c.CampHealing.BandageDie, 1, 20},
		{"camp_healing.inn_bonus", c.CampHealing.InnBonus, 0, 5},
		{"warlord.start_distance", c.Warlord.StartDistance, 1, 50},
		{"warlord.march_per_rest", c.Warlord.MarchPerRest, 1, 50},
		{"warlord.early_reward.xp_per_round", c.Warlord.EarlyReward.XPPerRound, 0, maxValue},
		{"warlord.early_reward.extra_relic_choices", c.Warlord.EarlyReward.ExtraRelicChoices, 0, 10},
		{"mending.village_influence_per_wound", c.Mending.VillageInfluencePerWound, 1, 20},
		{"mending.monastery_influence_per_wound", c.Mending.MonasteryInfluencePerWound, 1, 20},
		{"mending.town_gold_per_wound", c.Mending.TownGoldPerWound, 1, maxValue},
	}
	for _, check := range checks {
		if err := inRange(check.name, check.v, check.lo, check.hi); err != nil {
			return err
		}
	}
	for _, name := range requiredTerrain {
		t, ok := c.Terrain[name]
		if !ok {
			return fmt.Errorf("expedition config: terrain %q is missing", name)
		}
		if t.Impassable {
			if t.Day != 0 || t.Night != 0 {
				return fmt.Errorf("expedition config: impassable terrain %q has no cost", name)
			}
			continue
		}
		if err := inRange("terrain."+name+".day", t.Day, 1, 20); err != nil {
			return err
		}
		if err := inRange("terrain."+name+".night", t.Night, 1, 20); err != nil {
			return err
		}
	}
	for _, site := range deckChangeSites {
		v, ok := c.DeckChangeInfluence[site]
		if !ok {
			return fmt.Errorf("expedition config: deck_change_influence.%s is missing", site)
		}
		if err := inRange("deck_change_influence."+site, v, 0, 20); err != nil {
			return err
		}
	}
	for site := range c.DeckChangeInfluence {
		if !containsString(deckChangeSites, site) {
			return fmt.Errorf("expedition config: unknown deck-change site %q", site)
		}
	}
	for _, name := range requiredDrops {
		r, ok := c.Gold.Drops[name]
		if !ok {
			return fmt.Errorf("expedition config: gold.drops.%s is missing", name)
		}
		if r.Min < 0 || r.Max < r.Min || r.Max > maxValue {
			return fmt.Errorf("expedition config: gold.drops.%s is not a valid range", name)
		}
	}
	for _, name := range requiredPrices {
		r, ok := c.Gold.Prices[name]
		if !ok {
			return fmt.Errorf("expedition config: gold.prices.%s is missing", name)
		}
		if r.Min < 1 || r.Max < r.Min || r.Max > maxValue {
			return fmt.Errorf("expedition config: gold.prices.%s is not a valid range", name)
		}
	}
	statuses := map[string]string{"ambush": c.Statuses.Ambush, "fortified": c.Statuses.Fortified, "conqueror": c.Statuses.Conqueror, "lieutenant": c.Statuses.Lieutenant}
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		id := statuses[name]
		def, ok := lib.Statuses[id]
		if id == "" || !ok {
			return fmt.Errorf("expedition config: statuses.%s names unknown status %q", name, id)
		}
		m := def.Modifiers
		switch name {
		case "ambush":
			if m == nil || m.FirstRollBonusDice < 1 {
				return fmt.Errorf("expedition config: status %q must add first-roll bonus dice per stack", id)
			}
			if !def.Stacking.Uncapped && def.Stacking.StackLimit < c.Ambush.BonusDice {
				return fmt.Errorf("expedition config: status %q cannot hold %d stacks", id, c.Ambush.BonusDice)
			}
		case "fortified", "conqueror":
			if m == nil || m.DefenseDiceBonus < 1 {
				return fmt.Errorf("expedition config: status %q must add defensive dice per stack", id)
			}
		case "lieutenant":
			if m == nil || m.AttackDamageBonus < 1 {
				return fmt.Errorf("expedition config: status %q must add attack damage per stack", id)
			}
		}
	}
	return nil
}

// TerrainCost returns the Move cost of entering a terrain now, and whether it
// can be entered at all.
func (c Config) TerrainCost(terrain string, night bool) (int, bool) {
	t, ok := c.Terrain[terrain]
	if !ok || t.Impassable {
		return 0, false
	}
	if night {
		return t.Night, true
	}
	return t.Day, true
}

// SightRange is how far the player and enemies see now.
func (c Config) SightRange(night bool) int {
	if night {
		return c.Sight.Night
	}
	return c.Sight.Day
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
