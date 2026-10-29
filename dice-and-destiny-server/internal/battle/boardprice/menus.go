package boardprice

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"diceanddestiny/server/internal/content"
	"gopkg.in/yaml.v3"
)

// UnlockOpen is the only unlock rule in the first pass: the entry is
// available from the start. Later menus will be limited the way gear is.
const UnlockOpen = "open"

// Menus are the Board Builder's choices, read from
// content/progression_v1/board_templates.yaml and riders.yaml.
type Menus struct {
	Pricing   PricingConfig         `yaml:"pricing"`
	Bands     map[string]Band       `yaml:"bands"`
	Jobs      map[string]Job        `yaml:"jobs"`
	Dice      []MenuDie             `yaml:"dice"`
	Defenses  []MenuDefense         `yaml:"defenses"`
	Families  map[string]FamilyMenu `yaml:"families"`
	Templates []Template            `yaml:"templates"`
	// From riders.yaml.
	Rates  Rates   `yaml:"-"`
	Riders []Rider `yaml:"-"`
}

// PricingConfig is the agreed pricing method's numbers.
type PricingConfig struct {
	Target        float64 `yaml:"target" json:"target"`
	Alpha         float64 `yaml:"alpha" json:"alpha"`
	Rolls         int     `yaml:"rolls" json:"rolls"`
	Dice          int     `yaml:"dice" json:"dice"`
	MinimumDamage int     `yaml:"minimum_damage" json:"minimum_damage"`
	// LadderStep is how much more damage each harder tier of one ability
	// deals than the tier below it.
	LadderStep int `yaml:"ladder_step" json:"ladder_step"`
	// HalfBand: a damage this close to a half rounds whichever way lands the
	// board nearer its target.
	HalfBand  float64         `yaml:"half_band" json:"half_band"`
	Staircase StaircaseConfig `yaml:"staircase" json:"staircase"`
}

// StaircaseConfig is the range of targets every board is priced over (the
// Board Builder's damage slider) and how finely it is scanned.
type StaircaseConfig struct {
	From float64 `yaml:"from" json:"from"`
	To   float64 `yaml:"to" json:"to"`
	Scan float64 `yaml:"scan" json:"scan"`
}

// Band is a difficulty band of chase odds: Min inclusive, Max exclusive.
type Band struct {
	Name string  `yaml:"name" json:"name"`
	Min  float64 `yaml:"min" json:"min"`
	Max  float64 `yaml:"max" json:"max"`
}

// Contains reports chase odds inside the band.
func (b Band) Contains(odds float64) bool { return odds >= b.Min && odds < b.Max }

// Job names what a slot does on a board.
type Job struct {
	Name string `yaml:"name" json:"name"`
}

type MenuDie struct {
	ID     string `yaml:"id" json:"id"`
	Unlock string `yaml:"unlock" json:"unlock"`
}

type MenuDefense struct {
	Ability string `yaml:"ability" json:"ability"`
	Unlock  string `yaml:"unlock" json:"unlock"`
}

type FamilyMenu struct {
	FlexCosts []MenuCost `yaml:"flex_costs" json:"flex_costs"`
}

type MenuCost struct {
	Cost   string `yaml:"cost" json:"cost"`
	Unlock string `yaml:"unlock" json:"unlock"`
}

// Template is a board skeleton for one die family.
type Template struct {
	ID       string         `yaml:"id" json:"id"`
	Name     string         `yaml:"name" json:"name"`
	Family   string         `yaml:"family" json:"family"`
	Defenses int            `yaml:"defenses" json:"defenses"`
	Unlock   string         `yaml:"unlock" json:"unlock"`
	Slots    []TemplateSlot `yaml:"slots" json:"slots"`
}

// TemplateSlot is one ability on a template. Flex slots (Bands set) take a
// cost from their family menu; core slots keep Cost and may take a rider
// when Rider is set. DefaultRider is the rider an unpicked slot takes (none
// when empty).
type TemplateSlot struct {
	ID           string   `yaml:"id" json:"id"`
	Job          string   `yaml:"job" json:"job"`
	Cost         string   `yaml:"cost" json:"cost"`
	Bands        []string `yaml:"bands,omitempty" json:"bands,omitempty"`
	Rider        bool     `yaml:"rider,omitempty" json:"rider,omitempty"`
	DefaultRider string   `yaml:"default_rider,omitempty" json:"default_rider,omitempty"`
}

// Flex reports a slot whose cost the player picks.
func (s TemplateSlot) Flex() bool { return len(s.Bands) > 0 }

// TakesRider reports a slot the player may give a rider.
func (s TemplateSlot) TakesRider() bool { return s.Flex() || s.Rider }

// Rates are the damage equivalents of rider effects, per unit.
type Rates struct {
	Status map[string]float64 `yaml:"status" json:"status"`
	Energy float64            `yaml:"energy" json:"energy"`
	Draw   float64            `yaml:"draw" json:"draw"`
	Heal   float64            `yaml:"heal" json:"heal"`
}

// Rider is a side effect a slot may carry.
type Rider struct {
	ID         string                    `yaml:"id" json:"id"`
	Name       string                    `yaml:"name" json:"name"`
	Operations []content.BattleOperation `yaml:"operations" json:"operations"`
	Jobs       []string                  `yaml:"jobs" json:"jobs"`
	Unlock     string                    `yaml:"unlock" json:"unlock"`
}

// RiderNone is the empty rider every slot defaults to.
const RiderNone = "none"

type ridersFile struct {
	SchemaVersion int     `yaml:"schema_version"`
	Rates         Rates   `yaml:"rates"`
	Riders        []Rider `yaml:"riders"`
}

type menusFile struct {
	SchemaVersion int `yaml:"schema_version"`
	Menus         `yaml:",inline"`
}

var stableID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

// LoadMenus reads and strictly validates the Board Builder menus against a
// catalog that holds the custom adventurer's dice, defenses and statuses.
func LoadMenus(contentRoot string, lib content.BattleLibrary) (Menus, error) {
	var templates menusFile
	if err := decodeStrict(filepath.Join(contentRoot, "progression_v1", "board_templates.yaml"), &templates); err != nil {
		return Menus{}, err
	}
	if templates.SchemaVersion != 1 {
		return Menus{}, fmt.Errorf("board_templates.yaml: schema_version must be 1")
	}
	var riders ridersFile
	if err := decodeStrict(filepath.Join(contentRoot, "progression_v1", "riders.yaml"), &riders); err != nil {
		return Menus{}, err
	}
	if riders.SchemaVersion != 1 {
		return Menus{}, fmt.Errorf("riders.yaml: schema_version must be 1")
	}
	m := templates.Menus
	m.Rates, m.Riders = riders.Rates, riders.Riders
	if err := m.validate(lib); err != nil {
		return Menus{}, err
	}
	return m, nil
}

func decodeStrict(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func (m Menus) validate(lib content.BattleLibrary) error {
	p := m.Pricing
	st := p.Staircase
	if p.Target <= 0 || p.Target > 100 || p.Alpha < 0 || p.Alpha > 2 || p.Rolls < 1 || p.Rolls > 5 || p.Dice != content.LoadoutDiceCount || p.MinimumDamage < 1 || p.LadderStep < 0 || p.LadderStep > 10 ||
		p.HalfBand < 0 || p.HalfBand >= 0.5 || st.From <= 0 || st.To <= st.From || st.To > 100 || p.Target < st.From || p.Target > st.To || st.Scan <= 0 || st.Scan > (st.To-st.From)/10 {
		return fmt.Errorf("board pricing values are out of range")
	}
	for id, band := range m.Bands {
		if !stableID.MatchString(id) || band.Name == "" || band.Min < 0 || band.Max <= band.Min {
			return fmt.Errorf("invalid band %q", id)
		}
	}
	for id, job := range m.Jobs {
		if !stableID.MatchString(id) || job.Name == "" {
			return fmt.Errorf("invalid job %q", id)
		}
	}
	// Dice: catalog dice with roles; remember each family's dice.
	familyDice := map[string][]content.DieLayout{}
	seen := map[string]bool{}
	for _, d := range m.Dice {
		if err := checkUnlock("die "+d.ID, d.Unlock); err != nil {
			return err
		}
		die, ok := lib.Dice[d.ID]
		if !ok || seen[d.ID] {
			return fmt.Errorf("die menu: unknown or repeated die %q", d.ID)
		}
		seen[d.ID] = true
		layout, err := content.DieRoles(die)
		if err != nil {
			return fmt.Errorf("die menu: %w", err)
		}
		familyDice[layout.Family] = append(familyDice[layout.Family], layout)
	}
	if len(m.Defenses) == 0 {
		return fmt.Errorf("the defense menu is empty")
	}
	for _, d := range m.Defenses {
		if err := checkUnlock("defense "+d.Ability, d.Unlock); err != nil {
			return err
		}
		if err := checkRoleDefense(lib, d.Ability); err != nil {
			return err
		}
	}
	// Families: every menu cost parses and is reachable on the family's dice.
	for family, menu := range m.Families {
		if len(familyDice[family]) == 0 {
			return fmt.Errorf("family %s has no dice in the die menu", family)
		}
		costs := map[string]bool{}
		for _, entry := range menu.FlexCosts {
			if err := checkUnlock("cost "+entry.Cost, entry.Unlock); err != nil {
				return err
			}
			cost, err := ParseCost(entry.Cost)
			if err != nil {
				return fmt.Errorf("family %s: %w", family, err)
			}
			if len(cost) != 1 || cost[0].Straight != "" {
				return fmt.Errorf("family %s: flex cost %s must be one symbol tier", family, entry.Cost)
			}
			if costs[cost.String()] {
				return fmt.Errorf("family %s repeats cost %s", family, cost)
			}
			costs[cost.String()] = true
			for _, layout := range familyDice[family] {
				if err := cost.Reachable(layout, p.Dice); err != nil {
					return fmt.Errorf("family %s: %w", family, err)
				}
			}
		}
	}
	// Riders.
	riderIDs := map[string]bool{}
	for _, r := range m.Riders {
		if err := checkUnlock("rider "+r.ID, r.Unlock); err != nil {
			return err
		}
		if !stableID.MatchString(r.ID) || r.Name == "" || riderIDs[r.ID] {
			return fmt.Errorf("invalid or repeated rider %q", r.ID)
		}
		riderIDs[r.ID] = true
		if len(r.Jobs) == 0 {
			return fmt.Errorf("rider %s takes no jobs", r.ID)
		}
		for _, job := range r.Jobs {
			if _, ok := m.Jobs[job]; !ok {
				return fmt.Errorf("rider %s names unknown job %q", r.ID, job)
			}
		}
		if err := m.Rates.checkRider(lib, r); err != nil {
			return err
		}
	}
	if !riderIDs[RiderNone] {
		return fmt.Errorf("the rider menu needs %q", RiderNone)
	}
	// Templates.
	templateIDs := map[string]bool{}
	familyTemplates := map[string]int{}
	for _, t := range m.Templates {
		if err := checkUnlock("template "+t.ID, t.Unlock); err != nil {
			return err
		}
		if !stableID.MatchString(t.ID) || t.Name == "" || templateIDs[t.ID] {
			return fmt.Errorf("invalid or repeated template %q", t.ID)
		}
		templateIDs[t.ID] = true
		menu, ok := m.Families[t.Family]
		if !ok {
			return fmt.Errorf("template %s: unknown family %q", t.ID, t.Family)
		}
		familyTemplates[t.Family]++
		if t.Defenses < 1 || t.Defenses > len(m.Defenses) {
			return fmt.Errorf("template %s: defenses must be 1–%d", t.ID, len(m.Defenses))
		}
		if len(t.Slots) == 0 || len(t.Slots) > 20 {
			return fmt.Errorf("template %s needs 1–20 slots", t.ID)
		}
		slotIDs, costs := map[string]bool{}, map[string]bool{}
		for _, s := range t.Slots {
			if !stableID.MatchString(s.ID) || slotIDs[s.ID] {
				return fmt.Errorf("template %s: invalid or repeated slot %q", t.ID, s.ID)
			}
			slotIDs[s.ID] = true
			if _, ok := m.Jobs[s.Job]; !ok {
				return fmt.Errorf("template %s slot %s: unknown job %q", t.ID, s.ID, s.Job)
			}
			cost, err := ParseCost(s.Cost)
			if err != nil {
				return fmt.Errorf("template %s slot %s: %w", t.ID, s.ID, err)
			}
			if costs[cost.String()] {
				return fmt.Errorf("template %s repeats cost %s", t.ID, cost)
			}
			costs[cost.String()] = true
			for _, band := range s.Bands {
				if _, ok := m.Bands[band]; !ok {
					return fmt.Errorf("template %s slot %s: unknown band %q", t.ID, s.ID, band)
				}
			}
			if s.Flex() {
				if s.Job != "flex" || s.Rider {
					return fmt.Errorf("template %s slot %s: flex slots have the flex job and always take riders", t.ID, s.ID)
				}
				if !slices.ContainsFunc(menu.FlexCosts, func(c MenuCost) bool { parsed, _ := ParseCost(c.Cost); return parsed.String() == cost.String() }) {
					return fmt.Errorf("template %s slot %s: default %s is not in the %s menu", t.ID, s.ID, cost, t.Family)
				}
			} else if s.Job == "flex" {
				return fmt.Errorf("template %s slot %s: a flex slot needs bands", t.ID, s.ID)
			}
			for _, layout := range familyDice[t.Family] {
				if err := cost.Reachable(layout, p.Dice); err != nil {
					return fmt.Errorf("template %s slot %s: %w", t.ID, s.ID, err)
				}
			}
			if s.DefaultRider != "" {
				rider, ok := m.Rider(s.DefaultRider)
				if !ok || rider.Unlock != UnlockOpen || (s.DefaultRider != RiderNone && (!s.TakesRider() || !rider.RiderFits(s.Job))) {
					return fmt.Errorf("template %s slot %s cannot take default rider %q", t.ID, s.ID, s.DefaultRider)
				}
			}
		}
	}
	for family := range m.Families {
		if familyTemplates[family] == 0 {
			return fmt.Errorf("family %s has no template", family)
		}
	}
	for family := range familyDice {
		if _, ok := m.Families[family]; !ok {
			return fmt.Errorf("the die menu offers family %s with no menu", family)
		}
	}
	return nil
}

func checkUnlock(what, unlock string) error {
	if unlock != UnlockOpen {
		return fmt.Errorf("%s: unlock must be %q in the first pass, got %q", what, UnlockOpen, unlock)
	}
	return nil
}

// checkRoleDefense accepts defenses that roll the owner's die with role
// outcomes, so they work on any die the player picks.
func checkRoleDefense(lib content.BattleLibrary, id string) error {
	a, ok := lib.Abilities[id]
	if !ok || a.Type != "defensive" || a.Resolution == nil {
		return fmt.Errorf("defense menu: %q is not a defensive ability", id)
	}
	for _, op := range a.Resolution.Operations {
		if op.Type == "roll_dice" && op.Dice == content.OwnerDice {
			return nil
		}
	}
	return fmt.Errorf("defense menu: %s must roll the owner's die by roles", id)
}

// checkRider accepts only effects pricing has a rate for, applied to the
// rider's owner: Protect and other existing positive self-statuses, energy,
// draws and heals.
func (r Rates) checkRider(lib content.BattleLibrary, rider Rider) error {
	for _, op := range rider.Operations {
		if op.Target != "self" {
			return fmt.Errorf("rider %s: %s must target self", rider.ID, op.Type)
		}
		if _, isText := op.Amount.(string); isText {
			return fmt.Errorf("rider %s: amounts are fixed", rider.ID)
		}
		switch op.Type {
		case "apply_status":
			status, ok := lib.Statuses[op.StatusID]
			if !ok {
				return fmt.Errorf("rider %s: status %q is not in the catalog", rider.ID, op.StatusID)
			}
			if status.Polarity != "positive" || status.Lifecycle.Persistent || status.Modifiers.MapOnly() || op.ApplicationTiming != "immediate" {
				return fmt.Errorf("rider %s: %s must be a positive, non-persistent battle status applied immediately", rider.ID, op.StatusID)
			}
			if op.StackCount < 1 || (!status.Stacking.Uncapped && op.StackCount > status.Stacking.StackLimit) {
				return fmt.Errorf("rider %s: %d stacks of %s exceed its cap", rider.ID, op.StackCount, op.StatusID)
			}
			if _, ok := r.Status[op.StatusID]; !ok {
				return fmt.Errorf("rider %s: no exchange rate for %s", rider.ID, op.StatusID)
			}
		case "gain_resource":
			if op.Resource != "energy" {
				return fmt.Errorf("rider %s: only energy can be gained", rider.ID)
			}
		case "draw_cards", "heal_cards":
		default:
			return fmt.Errorf("rider %s: %s has no exchange rate", rider.ID, op.Type)
		}
		if _, err := r.Value([]content.BattleOperation{op}); err != nil {
			return fmt.Errorf("rider %s: %w", rider.ID, err)
		}
	}
	// The catalog's own rules (immediate statuses, amounts) on a probe ability.
	probe := content.BattleAbilityDefinition{SchemaVersion: 1, ConfigurationVersion: 1, ID: "board_rider_probe", Name: "Board Rider Probe " + rider.ID, Type: "offensive",
		Presentation: content.Presentation{RulesText: "probe"}, Usage: content.BattleUsage{MaximumPerSegment: 1},
		Targeting:     &content.TargetingDefinition{Selector: "one_enemy", Minimum: 1, Maximum: 1},
		Qualification: &content.AbilityQualification{ActivationTiers: []content.AbilityTier{{ID: "probe", Requirements: content.RequirementGroup{All: []content.BattleRequirement{{Type: "number_pattern", Pattern: "small_straight"}}}, Operations: append([]content.BattleOperation{{Type: "deal_damage", Target: "selected_targets", Amount: 1}}, rider.Operations...)}}}}
	if err := content.ValidateAuthoredAbility(probe, lib); err != nil {
		return fmt.Errorf("rider %s: %w", rider.ID, err)
	}
	return nil
}

// Value is what rider operations are worth in damage at these rates.
func (r Rates) Value(ops []content.BattleOperation) (float64, error) {
	value := 0.0
	for _, op := range ops {
		amount := 0
		switch v := op.Amount.(type) {
		case int:
			amount = v
		case float64:
			amount = int(v)
		}
		switch op.Type {
		case "apply_status":
			rate, ok := r.Status[op.StatusID]
			if !ok {
				return 0, fmt.Errorf("no exchange rate for %s", op.StatusID)
			}
			value += rate * float64(op.StackCount)
		case "gain_resource":
			value += r.Energy * float64(amount)
		case "draw_cards":
			value += r.Draw * float64(amount)
		case "heal_cards":
			value += r.Heal * float64(amount)
		default:
			return 0, fmt.Errorf("no exchange rate for %s", op.Type)
		}
	}
	return value, nil
}

// Template returns a template by ID.
func (m Menus) Template(id string) (Template, bool) {
	for _, t := range m.Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Rider returns a rider by ID.
func (m Menus) Rider(id string) (Rider, bool) {
	for _, r := range m.Riders {
		if r.ID == id {
			return r, true
		}
	}
	return Rider{}, false
}

// OffersDie reports a die on the menu.
func (m Menus) OffersDie(id string) bool {
	return slices.ContainsFunc(m.Dice, func(d MenuDie) bool { return d.ID == id })
}

// OffersDefense reports a defense on the menu.
func (m Menus) OffersDefense(id string) bool {
	return slices.ContainsFunc(m.Defenses, func(d MenuDefense) bool { return d.Ability == id })
}

// RiderFits reports whether a slot's job may take a rider.
func (r Rider) RiderFits(job string) bool { return slices.Contains(r.Jobs, job) }
