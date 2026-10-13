package expedition

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"diceanddestiny/server/internal/content"
	"gopkg.in/yaml.v3"
)

// Pack is the expedition content: tiles, sites, enemy groups, event scenes,
// gear and regions, loaded from content/expedition_v1/ and validated against
// the battle library (combatants) and the expedition config (terrain, ranges).
type Pack struct {
	Tiles   map[string]Tile   `json:"tiles"`
	Sites   map[string]Site   `json:"sites"`
	Groups  map[string]Group  `json:"groups"`
	Events  map[string]Event  `json:"events"`
	Gear    map[string]Gear   `json:"gear"`
	Regions map[string]Region `json:"regions"`
}

// TileHex is one of a tile's seven hexes.
type TileHex struct {
	Offset  []int  `yaml:"offset" json:"offset"`
	Terrain string `yaml:"terrain" json:"terrain"`
	Site    string `yaml:"site,omitempty" json:"site,omitempty"`
	Group   string `yaml:"group,omitempty" json:"group,omitempty"`
}

// Tile is a seven-hex flower: a centre and its six neighbours.
type Tile struct {
	SchemaVersion int       `yaml:"schema_version" json:"-"`
	ID            string    `yaml:"id" json:"id"`
	Name          string    `yaml:"name" json:"name"`
	Kind          string    `yaml:"kind" json:"kind"` // start, countryside, core
	Hexes         []TileHex `yaml:"hexes" json:"hexes"`
}

// SiteType names: village, monastery, town, ruins, grotto, event, keep, tower.
var SiteTypes = []string{"village", "monastery", "town", "ruins", "grotto", "event", "keep", "tower"}

type DayNightGroups struct {
	Day   string `yaml:"day" json:"day"`
	Night string `yaml:"night" json:"night"`
}

type DayNightRange struct {
	Day   Range `yaml:"day" json:"day"`
	Night Range `yaml:"night" json:"night"`
}

// Site is a place on a tile hex.
type Site struct {
	SchemaVersion int    `yaml:"schema_version" json:"-"`
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Type          string `yaml:"type" json:"type"`
	Description   string `yaml:"description" json:"description"`
	// Shop lists the gear sold here (villages and towns).
	Shop []string `yaml:"shop,omitempty" json:"shop,omitempty"`
	// Event is the scene an event site opens.
	Event string `yaml:"event,omitempty" json:"event,omitempty"`
	// Garrison defends a keep or tower (Fortified inside the walls).
	Garrison string `yaml:"garrison,omitempty" json:"garrison,omitempty"`
	// OccupiedBy holds a village or town until beaten.
	OccupiedBy string `yaml:"occupied_by,omitempty" json:"occupied_by,omitempty"`
	// Guardian and Loot belong to ruins and grottos, by Day and Night.
	Guardian *DayNightGroups `yaml:"guardian,omitempty" json:"guardian,omitempty"`
	Loot     *DayNightRange  `yaml:"loot,omitempty" json:"loot,omitempty"`
	// Cover counts as "unseen" for an Ambush when attacking from this hex.
	Cover bool `yaml:"cover,omitempty" json:"cover,omitempty"`
}

// Habits: unwary (ambush day or night), sleeper (at night), night_hunter (by
// day; roams at night), always_alert (never).
var Habits = []string{"unwary", "sleeper", "night_hunter", "always_alert"}

// Group is one enemy group: one scripted combatant with one or two copies.
type Group struct {
	SchemaVersion int    `yaml:"schema_version" json:"-"`
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Description   string `yaml:"description" json:"description"`
	Combatant     string `yaml:"combatant" json:"combatant"`
	Count         int    `yaml:"count" json:"count"`
	Habit         string `yaml:"habit" json:"habit"`
	XP            int    `yaml:"xp" json:"xp"`
	Gold          int    `yaml:"gold" json:"gold"`
	Lieutenant    bool   `yaml:"lieutenant,omitempty" json:"lieutenant,omitempty"`
	Warlord       bool   `yaml:"warlord,omitempty" json:"warlord,omitempty"`
	// Patrol lists hex offsets from the group's home hex that it cycles
	// through: one step per rest, or per turn at night for night hunters.
	Patrol [][]int `yaml:"patrol,omitempty" json:"patrol,omitempty"`
}

// Outcome is what an event choice (or one side of its roll) does.
type Outcome struct {
	Text   string `yaml:"text" json:"text"`
	Gold   int    `yaml:"gold,omitempty" json:"gold,omitempty"`
	XP     int    `yaml:"xp,omitempty" json:"xp,omitempty"`
	Wounds int    `yaml:"wounds,omitempty" json:"wounds,omitempty"`
	Mend   int    `yaml:"mend,omitempty" json:"mend,omitempty"`
	Item   string `yaml:"item,omitempty" json:"item,omitempty"`
	Fight  string `yaml:"fight,omitempty" json:"fight,omitempty"`
	// Status applies a map status (hand symmetry: it reaches battles too).
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
}

type Requirement struct {
	Influence int    `yaml:"influence,omitempty" json:"influence,omitempty"`
	Gold      int    `yaml:"gold,omitempty" json:"gold,omitempty"`
	Item      string `yaml:"item,omitempty" json:"item,omitempty"`
}

type EventRoll struct {
	Needed  int     `yaml:"needed" json:"needed"`
	Success Outcome `yaml:"success" json:"success"`
	Failure Outcome `yaml:"failure" json:"failure"`
}

type Choice struct {
	ID       string      `yaml:"id" json:"id"`
	Text     string      `yaml:"text" json:"text"`
	Requires Requirement `yaml:"requires,omitempty" json:"requires"`
	Outcome  *Outcome    `yaml:"outcome,omitempty" json:"outcome,omitempty"`
	Roll     *EventRoll  `yaml:"roll,omitempty" json:"roll,omitempty"`
}

// Event is a short scene with two or three choices.
type Event struct {
	SchemaVersion int      `yaml:"schema_version" json:"-"`
	ID            string   `yaml:"id" json:"id"`
	Name          string   `yaml:"name" json:"name"`
	Text          string   `yaml:"text" json:"text"`
	Choices       []Choice `yaml:"choices" json:"choices"`
}

// GearEffects: mend_roll (one camp healing roll: a bandage heals 1d6 from a
// treated wound), night_sight (+Amount sight at night), remove_fortified (the
// next garrison fight is not Fortified).
var GearEffects = []string{"mend_roll", "night_sight", "remove_fortified"}

type Gear struct {
	SchemaVersion int    `yaml:"schema_version" json:"-"`
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Kind          string `yaml:"kind" json:"kind"` // consumable, gear
	Effect        string `yaml:"effect" json:"effect"`
	Amount        int    `yaml:"amount" json:"amount"`
	Price         int    `yaml:"price" json:"price"`
	Description   string `yaml:"description" json:"description"`
}

type Slot struct {
	ID     string `yaml:"id" json:"id"`
	Center []int  `yaml:"center" json:"center"`
	Kind   string `yaml:"kind" json:"kind"` // countryside, core
}

type WarlordDefinition struct {
	Group    string `yaml:"group" json:"group"`
	GoalTile string `yaml:"goal_tile" json:"goal_tile"`
	GoalSite string `yaml:"goal_site" json:"goal_site"`
}

// Region is a scenario: a start tile, a shuffled countryside stack, core tiles
// dealt to the core slots, and the Warlord's goal.
type Region struct {
	SchemaVersion    int               `yaml:"schema_version" json:"-"`
	ID               string            `yaml:"id" json:"id"`
	Name             string            `yaml:"name" json:"name"`
	Description      string            `yaml:"description" json:"description"`
	StartTile        string            `yaml:"start_tile" json:"start_tile"`
	CountrysideTiles []string          `yaml:"countryside_tiles" json:"countryside_tiles"`
	CoreTiles        []string          `yaml:"core_tiles" json:"core_tiles"`
	Slots            []Slot            `yaml:"slots" json:"slots"`
	Warlord          WarlordDefinition `yaml:"warlord" json:"warlord"`
}

var stableID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func loadItems[T any](dir string, id func(T) string, out map[string]T) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("expedition content: read %q: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("expedition content: read %q: %w", path, err)
		}
		var item T
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err = decoder.Decode(&item); err != nil {
			return fmt.Errorf("expedition content: %s: %w", path, err)
		}
		key := id(item)
		if !stableID.MatchString(key) {
			return fmt.Errorf("expedition content: %s: id %q must be lowercase snake_case", path, key)
		}
		if _, exists := out[key]; exists {
			return fmt.Errorf("expedition content: duplicate id %q in %s", key, dir)
		}
		out[key] = item
	}
	return nil
}

// LoadPack reads content/expedition_v1 and validates every reference.
func LoadPack(root string, cfg Config, lib content.BattleLibrary) (Pack, error) {
	p := Pack{Tiles: map[string]Tile{}, Sites: map[string]Site{}, Groups: map[string]Group{}, Events: map[string]Event{}, Gear: map[string]Gear{}, Regions: map[string]Region{}}
	dir := filepath.Join(root, "expedition_v1")
	if err := loadItems(filepath.Join(dir, "tiles"), func(t Tile) string { return t.ID }, p.Tiles); err != nil {
		return p, err
	}
	if err := loadItems(filepath.Join(dir, "sites"), func(s Site) string { return s.ID }, p.Sites); err != nil {
		return p, err
	}
	if err := loadItems(filepath.Join(dir, "groups"), func(g Group) string { return g.ID }, p.Groups); err != nil {
		return p, err
	}
	if err := loadItems(filepath.Join(dir, "events"), func(e Event) string { return e.ID }, p.Events); err != nil {
		return p, err
	}
	if err := loadItems(filepath.Join(dir, "gear"), func(g Gear) string { return g.ID }, p.Gear); err != nil {
		return p, err
	}
	if err := loadItems(filepath.Join(dir, "regions"), func(r Region) string { return r.ID }, p.Regions); err != nil {
		return p, err
	}
	return p, p.Validate(cfg, lib)
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Validate checks every definition and every cross reference.
func (p Pack) Validate(cfg Config, lib content.BattleLibrary) error {
	for _, id := range sortedKeys(p.Gear) {
		if err := p.validateGear(cfg, p.Gear[id]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(p.Groups) {
		if err := p.validateGroup(cfg, lib, p.Groups[id]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(p.Events) {
		if err := p.validateEvent(cfg, lib, p.Events[id]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(p.Sites) {
		if err := p.validateSite(cfg, p.Sites[id]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(p.Tiles) {
		if err := p.validateTile(cfg, p.Tiles[id]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(p.Regions) {
		if err := p.validateRegion(cfg, p.Regions[id]); err != nil {
			return err
		}
	}
	return nil
}

func (p Pack) validateGear(cfg Config, g Gear) error {
	if g.SchemaVersion != 1 || g.Name == "" || g.Description == "" {
		return fmt.Errorf("expedition content: gear %q needs schema_version 1, a name and a description", g.ID)
	}
	if g.Kind != "consumable" && g.Kind != "gear" {
		return fmt.Errorf("expedition content: gear %q kind must be consumable or gear", g.ID)
	}
	if !containsString(GearEffects, g.Effect) {
		return fmt.Errorf("expedition content: gear %q has unknown effect %q", g.ID, g.Effect)
	}
	if g.Amount < 1 || g.Amount > 10 {
		return fmt.Errorf("expedition content: gear %q amount must be 1–10", g.ID)
	}
	priceKey := g.Kind
	if g.Effect == "mend_roll" {
		priceKey = "bandage"
	}
	if r := cfg.Gold.Prices[priceKey]; !r.Contains(g.Price) {
		return fmt.Errorf("expedition content: gear %q price %d is outside gold.prices.%s (%d–%d)", g.ID, g.Price, priceKey, r.Min, r.Max)
	}
	return nil
}

func (p Pack) validateGroup(cfg Config, lib content.BattleLibrary, g Group) error {
	if g.SchemaVersion != 1 || g.Name == "" {
		return fmt.Errorf("expedition content: group %q needs schema_version 1 and a name", g.ID)
	}
	def, ok := lib.Combatants[g.Combatant]
	if !ok || def.SingleAbilityPolicy == nil {
		return fmt.Errorf("expedition content: group %q needs a scripted minion combatant, not %q", g.ID, g.Combatant)
	}
	if g.Count < 1 || g.Count > 2 {
		return fmt.Errorf("expedition content: group %q count must be 1 or 2 (one combatant definition per battle)", g.ID)
	}
	if !containsString(Habits, g.Habit) {
		return fmt.Errorf("expedition content: group %q has unknown habit %q", g.ID, g.Habit)
	}
	if g.XP < 1 || g.XP > maxValue {
		return fmt.Errorf("expedition content: group %q xp must be 1–%d", g.ID, maxValue)
	}
	if g.Warlord && g.Lieutenant {
		return fmt.Errorf("expedition content: group %q cannot be both the Warlord and a Lieutenant", g.ID)
	}
	if g.Warlord && g.Count != 1 {
		return fmt.Errorf("expedition content: the Warlord %q is one combatant", g.ID)
	}
	dropKey := "minion_group"
	if g.Lieutenant {
		dropKey = "lieutenant"
	}
	if g.Warlord {
		dropKey = "warlord"
	}
	if r := cfg.Gold.Drops[dropKey]; !r.Contains(g.Gold) {
		return fmt.Errorf("expedition content: group %q gold %d is outside gold.drops.%s (%d–%d)", g.ID, g.Gold, dropKey, r.Min, r.Max)
	}
	for i, step := range g.Patrol {
		if _, err := hexFromPair(step); err != nil {
			return fmt.Errorf("expedition content: group %q patrol step %d: %v", g.ID, i, err)
		}
	}
	if len(g.Patrol) > 0 && len(g.Patrol) < 2 {
		return fmt.Errorf("expedition content: group %q patrol needs at least two steps", g.ID)
	}
	return nil
}

func (p Pack) validateOutcome(cfg Config, lib content.BattleLibrary, where string, o Outcome) error {
	if o.Text == "" {
		return fmt.Errorf("expedition content: %s outcome needs text", where)
	}
	if r := cfg.Gold.Drops["event"]; o.Gold != 0 && !r.Contains(o.Gold) {
		return fmt.Errorf("expedition content: %s gold %d is outside gold.drops.event (%d–%d)", where, o.Gold, r.Min, r.Max)
	}
	if o.XP < 0 || o.XP > maxValue || o.Wounds < 0 || o.Wounds > 10 || o.Mend < 0 || o.Mend > 10 {
		return fmt.Errorf("expedition content: %s has numbers out of range", where)
	}
	if o.Item != "" {
		if _, ok := p.Gear[o.Item]; !ok {
			return fmt.Errorf("expedition content: %s names unknown gear %q", where, o.Item)
		}
	}
	if o.Fight != "" {
		g, ok := p.Groups[o.Fight]
		if !ok || g.Warlord {
			return fmt.Errorf("expedition content: %s names unknown fight group %q", where, o.Fight)
		}
	}
	if o.Status != "" {
		if _, ok := lib.Statuses[o.Status]; !ok {
			return fmt.Errorf("expedition content: %s names unknown status %q", where, o.Status)
		}
	}
	return nil
}

func (p Pack) validateEvent(cfg Config, lib content.BattleLibrary, e Event) error {
	if e.SchemaVersion != 1 || e.Name == "" || e.Text == "" {
		return fmt.Errorf("expedition content: event %q needs schema_version 1, a name and text", e.ID)
	}
	if len(e.Choices) < 2 || len(e.Choices) > 3 {
		return fmt.Errorf("expedition content: event %q needs 2–3 choices", e.ID)
	}
	seen := map[string]bool{}
	for _, c := range e.Choices {
		where := fmt.Sprintf("event %q choice %q", e.ID, c.ID)
		if !stableID.MatchString(c.ID) || c.Text == "" || seen[c.ID] {
			return fmt.Errorf("expedition content: %s needs a unique snake_case id and text", where)
		}
		seen[c.ID] = true
		if c.Requires.Influence < 0 || c.Requires.Influence > 20 || c.Requires.Gold < 0 || c.Requires.Gold > maxValue {
			return fmt.Errorf("expedition content: %s has requirements out of range", where)
		}
		if c.Requires.Item != "" {
			if _, ok := p.Gear[c.Requires.Item]; !ok {
				return fmt.Errorf("expedition content: %s requires unknown gear %q", where, c.Requires.Item)
			}
		}
		if (c.Outcome == nil) == (c.Roll == nil) {
			return fmt.Errorf("expedition content: %s needs exactly one of outcome or roll", where)
		}
		if c.Outcome != nil {
			if err := p.validateOutcome(cfg, lib, where, *c.Outcome); err != nil {
				return err
			}
		}
		if c.Roll != nil {
			if c.Roll.Needed < 1 || c.Roll.Needed > 6 {
				return fmt.Errorf("expedition content: %s roll needs a target of 1–6", where)
			}
			if err := p.validateOutcome(cfg, lib, where+" success", c.Roll.Success); err != nil {
				return err
			}
			if err := p.validateOutcome(cfg, lib, where+" failure", c.Roll.Failure); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Pack) validateSite(cfg Config, s Site) error {
	if s.SchemaVersion != 1 || s.Name == "" || s.Description == "" {
		return fmt.Errorf("expedition content: site %q needs schema_version 1, a name and a description", s.ID)
	}
	if !containsString(SiteTypes, s.Type) {
		return fmt.Errorf("expedition content: site %q has unknown type %q", s.ID, s.Type)
	}
	for _, item := range s.Shop {
		if _, ok := p.Gear[item]; !ok {
			return fmt.Errorf("expedition content: site %q sells unknown gear %q", s.ID, item)
		}
	}
	groupRef := func(field, id string) error {
		if id == "" {
			return nil
		}
		g, ok := p.Groups[id]
		if !ok || g.Warlord {
			return fmt.Errorf("expedition content: site %q %s names unknown group %q", s.ID, field, id)
		}
		return nil
	}
	for _, ref := range [][2]string{{"garrison", s.Garrison}, {"occupied_by", s.OccupiedBy}} {
		if err := groupRef(ref[0], ref[1]); err != nil {
			return err
		}
	}
	if s.Guardian != nil {
		for _, ref := range [][2]string{{"guardian.day", s.Guardian.Day}, {"guardian.night", s.Guardian.Night}} {
			if ref[1] == "" {
				return fmt.Errorf("expedition content: site %q needs both a day and a night guardian", s.ID)
			}
			if err := groupRef(ref[0], ref[1]); err != nil {
				return err
			}
		}
	}
	if s.Loot != nil {
		treasure := cfg.Gold.Drops["treasure"]
		for _, r := range []Range{s.Loot.Day, s.Loot.Night} {
			if r.Min > r.Max || !treasure.Contains(r.Min) || !treasure.Contains(r.Max) {
				return fmt.Errorf("expedition content: site %q loot must lie within gold.drops.treasure (%d–%d)", s.ID, treasure.Min, treasure.Max)
			}
		}
	}
	switch s.Type {
	case "village", "town":
		if s.Garrison != "" || s.Guardian != nil || s.Loot != nil || s.Event != "" {
			return fmt.Errorf("expedition content: %s %q only has a shop and may be occupied", s.Type, s.ID)
		}
	case "monastery":
		if s.Garrison != "" || s.OccupiedBy != "" || s.Guardian != nil || s.Loot != nil || s.Event != "" || len(s.Shop) > 0 {
			return fmt.Errorf("expedition content: monastery %q offers only its services", s.ID)
		}
	case "ruins", "grotto":
		if s.Guardian == nil || s.Loot == nil || s.Garrison != "" || s.OccupiedBy != "" || s.Event != "" || len(s.Shop) > 0 {
			return fmt.Errorf("expedition content: %s %q needs a guardian and loot and nothing else", s.Type, s.ID)
		}
	case "event":
		if _, ok := p.Events[s.Event]; !ok || s.Garrison != "" || s.OccupiedBy != "" || s.Guardian != nil || s.Loot != nil || len(s.Shop) > 0 {
			return fmt.Errorf("expedition content: event site %q needs a known event and nothing else", s.ID)
		}
	case "keep", "tower":
		if s.Garrison == "" || s.OccupiedBy != "" || s.Guardian != nil || s.Loot != nil || s.Event != "" || len(s.Shop) > 0 {
			return fmt.Errorf("expedition content: %s %q needs a garrison and nothing else", s.Type, s.ID)
		}
	}
	return nil
}

// groupHexes lists the tile offsets that hold an open enemy group: one that
// stands on the map and provokes. Garrisons, occupiers and guardians wait
// inside their sites and fight only when the player enters, so they never pull
// a second group into a battle; patrol steps are guarded at run time instead.
func (p Pack) groupHexes(t Tile) ([]Hex, error) {
	var out []Hex
	for _, h := range t.Hexes {
		if h.Group == "" {
			continue
		}
		offset, err := hexFromPair(h.Offset)
		if err != nil {
			return nil, err
		}
		out = append(out, offset)
	}
	return out, nil
}

func (p Pack) validateTile(cfg Config, t Tile) error {
	if t.SchemaVersion != 1 || t.Name == "" {
		return fmt.Errorf("expedition content: tile %q needs schema_version 1 and a name", t.ID)
	}
	if t.Kind != "start" && t.Kind != "countryside" && t.Kind != "core" {
		return fmt.Errorf("expedition content: tile %q kind must be start, countryside or core", t.ID)
	}
	if len(t.Hexes) != len(FlowerHexes) {
		return fmt.Errorf("expedition content: tile %q needs exactly %d hexes", t.ID, len(FlowerHexes))
	}
	seen := map[Hex]bool{}
	for _, h := range t.Hexes {
		offset, err := hexFromPair(h.Offset)
		if err != nil {
			return fmt.Errorf("expedition content: tile %q: %v", t.ID, err)
		}
		known := false
		for _, f := range FlowerHexes {
			known = known || f == offset
		}
		if !known || seen[offset] {
			return fmt.Errorf("expedition content: tile %q hex %v must be the centre or one of its six neighbours, once", t.ID, h.Offset)
		}
		seen[offset] = true
		if _, ok := cfg.Terrain[h.Terrain]; !ok {
			return fmt.Errorf("expedition content: tile %q uses unknown terrain %q", t.ID, h.Terrain)
		}
		if h.Site != "" {
			site, ok := p.Sites[h.Site]
			if !ok {
				return fmt.Errorf("expedition content: tile %q references unknown site %q", t.ID, h.Site)
			}
			if cfg.Terrain[h.Terrain].Impassable {
				return fmt.Errorf("expedition content: tile %q puts site %q on impassable terrain", t.ID, h.Site)
			}
			_ = site
		}
		if h.Group != "" {
			if _, ok := p.Groups[h.Group]; !ok {
				return fmt.Errorf("expedition content: tile %q references unknown group %q", t.ID, h.Group)
			}
			if cfg.Terrain[h.Terrain].Impassable {
				return fmt.Errorf("expedition content: tile %q puts group %q on impassable terrain", t.ID, h.Group)
			}
			if p.Groups[h.Group].Warlord {
				return fmt.Errorf("expedition content: tile %q places the Warlord; his route is laid out by the region", t.ID)
			}
		}
		if h.Site != "" && h.Group != "" {
			return fmt.Errorf("expedition content: tile %q hex %v holds both a site and a group; garrisons belong to the site", t.ID, h.Offset)
		}
		if t.Kind == "start" && (h.Group != "" || (h.Site != "" && (p.Sites[h.Site].Garrison != "" || p.Sites[h.Site].OccupiedBy != "" || p.Sites[h.Site].Guardian != nil))) && offset == (Hex{}) {
			return fmt.Errorf("expedition content: start tile %q cannot put an enemy on the player's starting hex", t.ID)
		}
	}
	// Patrol steps must stay on the tile and off its sites.
	for _, h := range t.Hexes {
		if h.Group == "" {
			continue
		}
		home, _ := hexFromPair(h.Offset)
		for _, step := range p.Groups[h.Group].Patrol {
			offset, _ := hexFromPair(step)
			target := home.Add(offset)
			if !seen[target] {
				return fmt.Errorf("expedition content: tile %q group %q patrols off the tile (%v)", t.ID, h.Group, target)
			}
			for _, other := range t.Hexes {
				o, _ := hexFromPair(other.Offset)
				if o == target && (other.Site != "" || cfg.Terrain[other.Terrain].Impassable) {
					return fmt.Errorf("expedition content: tile %q group %q patrols onto a site or impassable hex (%v)", t.ID, h.Group, target)
				}
			}
		}
	}
	// Within one tile, no hex may touch two different open groups.
	groups, err := p.groupHexes(t)
	if err != nil {
		return fmt.Errorf("expedition content: tile %q: %v", t.ID, err)
	}
	for i := range groups {
		for j := i + 1; j < len(groups); j++ {
			if Distance(groups[i], groups[j]) <= 2 {
				return fmt.Errorf("expedition content: tile %q places groups at %v and %v so that one hex touches both", t.ID, groups[i], groups[j])
			}
		}
	}
	return nil
}

// validateAdjacency is the region-build guarantee: for every pair of tiles in
// the region and every relative placement two flowers can have, no hex touches
// group hexes of both tiles. Because it holds for every placement, no seeded
// deal or exploration order can violate it.
func (p Pack) validateAdjacency(tiles []string) error {
	hexes := map[string][]Hex{}
	for _, id := range tiles {
		g, err := p.groupHexes(p.Tiles[id])
		if err != nil {
			return err
		}
		hexes[id] = g
	}
	for i, a := range tiles {
		for _, b := range tiles[i+1:] {
			for _, offset := range FlowerOffsets {
				for _, ga := range hexes[a] {
					for _, gb := range hexes[b] {
						if Distance(ga, gb.Add(offset)) <= 2 {
							return fmt.Errorf("expedition content: tiles %q and %q placed at offset %v put groups at %v and %v so that one hex touches both", a, b, offset, ga, gb.Add(offset))
						}
					}
				}
			}
		}
	}
	return nil
}

func (p Pack) validateRegion(cfg Config, r Region) error {
	if r.SchemaVersion != 1 || r.Name == "" || r.Description == "" {
		return fmt.Errorf("expedition content: region %q needs schema_version 1, a name and a description", r.ID)
	}
	start, ok := p.Tiles[r.StartTile]
	if !ok || start.Kind != "start" {
		return fmt.Errorf("expedition content: region %q start_tile %q must be a start tile", r.ID, r.StartTile)
	}
	all := []string{r.StartTile}
	seen := map[string]bool{r.StartTile: true}
	for _, kind := range []struct {
		kind string
		ids  []string
	}{{"countryside", r.CountrysideTiles}, {"core", r.CoreTiles}} {
		if len(kind.ids) == 0 {
			return fmt.Errorf("expedition content: region %q needs %s tiles", r.ID, kind.kind)
		}
		for _, id := range kind.ids {
			t, ok := p.Tiles[id]
			if !ok || t.Kind != kind.kind || seen[id] {
				return fmt.Errorf("expedition content: region %q tile %q must be a %s tile used once", r.ID, id, kind.kind)
			}
			seen[id] = true
			all = append(all, id)
		}
	}
	centers := map[Hex]string{{}: "start"}
	slotIDs := map[string]bool{"start": true}
	counts := map[string]int{}
	for _, s := range r.Slots {
		center, err := hexFromPair(s.Center)
		if err != nil || !IsFlowerCenter(center) || center == (Hex{}) {
			return fmt.Errorf("expedition content: region %q slot %q centre %v is not on the tile lattice", r.ID, s.ID, s.Center)
		}
		if !stableID.MatchString(s.ID) || slotIDs[s.ID] {
			return fmt.Errorf("expedition content: region %q slot %q needs a unique snake_case id", r.ID, s.ID)
		}
		if _, taken := centers[center]; taken {
			return fmt.Errorf("expedition content: region %q slots overlap at %v", r.ID, center)
		}
		if s.Kind != "countryside" && s.Kind != "core" {
			return fmt.Errorf("expedition content: region %q slot %q kind must be countryside or core", r.ID, s.ID)
		}
		centers[center] = s.ID
		slotIDs[s.ID] = true
		counts[s.Kind]++
	}
	if counts["countryside"] != len(r.CountrysideTiles) || counts["core"] != len(r.CoreTiles) {
		return fmt.Errorf("expedition content: region %q needs one slot per tile (%d countryside, %d core)", r.ID, len(r.CountrysideTiles), len(r.CoreTiles))
	}
	// Every slot must be reachable from the start through flower-adjacent slots.
	reached := map[Hex]bool{{}: true}
	frontier := []Hex{{}}
	for len(frontier) > 0 {
		c := frontier[0]
		frontier = frontier[1:]
		for _, o := range FlowerOffsets {
			n := c.Add(o)
			if _, ok := centers[n]; ok && !reached[n] {
				reached[n] = true
				frontier = append(frontier, n)
			}
		}
	}
	if len(reached) != len(centers) {
		return fmt.Errorf("expedition content: region %q has slots no path of tiles reaches", r.ID)
	}
	// The Warlord.
	w, ok := p.Groups[r.Warlord.Group]
	if !ok || !w.Warlord {
		return fmt.Errorf("expedition content: region %q warlord.group %q must be a Warlord group", r.ID, r.Warlord.Group)
	}
	if !containsString(r.CoreTiles, r.Warlord.GoalTile) {
		return fmt.Errorf("expedition content: region %q warlord.goal_tile %q must be one of its core tiles", r.ID, r.Warlord.GoalTile)
	}
	goalFound := false
	for _, h := range p.Tiles[r.Warlord.GoalTile].Hexes {
		goalFound = goalFound || h.Site == r.Warlord.GoalSite
	}
	if site, ok := p.Sites[r.Warlord.GoalSite]; !goalFound || !ok || (site.Type != "town" && site.Type != "village" && site.Type != "keep") {
		return fmt.Errorf("expedition content: region %q warlord.goal_site %q must be a town, village or keep on the goal tile", r.ID, r.Warlord.GoalSite)
	}
	// Each site appears on at most one tile of the region, so site instances
	// are unambiguous.
	sitesSeen := map[string]string{}
	for _, id := range all {
		for _, h := range p.Tiles[id].Hexes {
			if h.Site == "" {
				continue
			}
			if other, dup := sitesSeen[h.Site]; dup {
				return fmt.Errorf("expedition content: region %q places site %q on both %q and %q", r.ID, h.Site, other, id)
			}
			sitesSeen[h.Site] = id
		}
	}
	return p.validateAdjacency(all)
}

// SiteByType helpers used by rules and tests.
func (s Site) OpenAtNight() bool { return s.Type != "village" && s.Type != "town" }

// HasInn: villages and towns keep an inn open at night (a safe rest, with a
// bonus to camp mending rolls).
func (s Site) HasInn() bool { return s.Type == "village" || s.Type == "town" }

// DeckChangeTier maps a site type to the deck_change_influence key.
func (s Site) DeckChangeTier() string {
	switch s.Type {
	case "village", "monastery", "town":
		return s.Type
	}
	return ""
}
