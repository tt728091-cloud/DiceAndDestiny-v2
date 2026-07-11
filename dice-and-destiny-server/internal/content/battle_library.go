package content

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// BattleLibrary is the settled, self-contained YAML content catalog.  It is
// intentionally separate from the legacy mock content structs: accepting both
// shapes in one decoder would make unknown-field validation ineffective.
type BattleLibrary struct {
	Symbols    map[string]SymbolDefinition
	Dice       map[string]BattleDieDefinition
	Cards      map[string]BattleCardDefinition
	Abilities  map[string]BattleAbilityDefinition
	Statuses   map[string]BattleStatusDefinition
	Combatants map[string]CombatantDefinition
}

type SymbolDefinition struct {
	ID              string `yaml:"id" json:"id"`
	Name            string `yaml:"name" json:"name"`
	PresentationKey string `yaml:"presentation_key" json:"presentation_key"`
}

type symbolCatalogFile struct {
	SchemaVersion int                `yaml:"schema_version"`
	Symbols       []SymbolDefinition `yaml:"symbols"`
}

type BattleDieDefinition struct {
	SchemaVersion int             `yaml:"schema_version" json:"schema_version"`
	ID            string          `yaml:"id" json:"id"`
	Name          string          `yaml:"name" json:"name"`
	DieType       string          `yaml:"die_type" json:"die_type"`
	SideCount     int             `yaml:"side_count" json:"side_count"`
	Faces         []BattleDieFace `yaml:"faces" json:"faces"`
}

type BattleDieFace struct {
	Number int    `yaml:"number" json:"number"`
	Symbol string `yaml:"symbol" json:"symbol"`
}

type BattleCost struct {
	Energy int `yaml:"energy" json:"energy"`
}

type Presentation struct {
	RulesText string `yaml:"rules_text" json:"rules_text"`
	ArtKey    string `yaml:"art_key" json:"art_key"`
}

type ReactionWindowDefinition struct {
	Opens        bool `yaml:"opens" json:"opens"`
	PassRequired bool `yaml:"pass_required" json:"pass_required"`
}

type TargetingDefinition struct {
	Selector string `yaml:"selector" json:"selector"`
	Minimum  int    `yaml:"minimum" json:"minimum"`
	Maximum  int    `yaml:"maximum" json:"maximum"`
}

type PlayTiming struct {
	Segment       string `yaml:"segment" json:"segment"`
	Phase         string `yaml:"phase" json:"phase"`
	Stage         string `yaml:"stage,omitempty" json:"stage,omitempty"`
	WindowPurpose string `yaml:"window_purpose" json:"window_purpose"`
}

type CardPlayDefinition struct {
	SourceZones    []string     `yaml:"source_zones" json:"source_zones"`
	Destination    string       `yaml:"destination" json:"destination"`
	PlayableDuring []PlayTiming `yaml:"playable_during" json:"playable_during"`
}

type BattleCardDefinition struct {
	SchemaVersion  int                      `yaml:"schema_version" json:"schema_version"`
	ID             string                   `yaml:"id" json:"id"`
	Name           string                   `yaml:"name" json:"name"`
	Type           string                   `yaml:"type" json:"type"`
	Presentation   Presentation             `yaml:"presentation" json:"presentation"`
	Cost           BattleCost               `yaml:"cost" json:"cost"`
	Play           CardPlayDefinition       `yaml:"play" json:"play"`
	Targeting      TargetingDefinition      `yaml:"targeting" json:"targeting"`
	ReactionWindow ReactionWindowDefinition `yaml:"reaction_window" json:"reaction_window"`
	Operations     []BattleOperation        `yaml:"operations" json:"operations"`
}

type BattleUsage struct {
	MaximumPerSegment int `yaml:"maximum_per_segment" json:"maximum_per_segment"`
}

type RequirementGroup struct {
	All []BattleRequirement `yaml:"all" json:"all"`
}

type BattleRequirement struct {
	Type     string `yaml:"type" json:"type"`
	SymbolID string `yaml:"symbol_id,omitempty" json:"symbol_id,omitempty"`
	Minimum  int    `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum  int    `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	Exact    *int   `yaml:"exact,omitempty" json:"exact,omitempty"`
	Pattern  string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Faces    []int  `yaml:"faces,omitempty" json:"faces,omitempty"`
}

type AbilityTier struct {
	ID           string            `yaml:"id" json:"id"`
	Requirements RequirementGroup  `yaml:"requirements" json:"requirements"`
	Operations   []BattleOperation `yaml:"operations" json:"operations"`
}

type AbilityQualification struct {
	ActivationTiers    []AbilityTier `yaml:"activation_tiers" json:"activation_tiers"`
	ConditionalBonuses []AbilityTier `yaml:"conditional_bonuses" json:"conditional_bonuses"`
}

type DefenseSelection struct {
	RequiresIncomingProposal bool     `yaml:"requires_incoming_proposal" json:"requires_incoming_proposal"`
	AllowedProposalTypes     []string `yaml:"allowed_proposal_types" json:"allowed_proposal_types"`
	TargetCount              int      `yaml:"target_count" json:"target_count"`
}

type RollDefinition struct {
	DiceID    string `yaml:"dice_id" json:"dice_id"`
	DiceCount int    `yaml:"dice_count" json:"dice_count"`
}

type DefenseResolution struct {
	Roll           *RollDefinition          `yaml:"roll,omitempty" json:"roll,omitempty"`
	ReactionWindow ReactionWindowDefinition `yaml:"reaction_window" json:"reaction_window"`
	Operations     []BattleOperation        `yaml:"operations" json:"operations"`
}

type BattleAbilityDefinition struct {
	SchemaVersion int                   `yaml:"schema_version" json:"schema_version"`
	ID            string                `yaml:"id" json:"id"`
	Name          string                `yaml:"name" json:"name"`
	Type          string                `yaml:"type" json:"type"`
	Presentation  Presentation          `yaml:"presentation" json:"presentation"`
	Cost          BattleCost            `yaml:"cost" json:"cost"`
	Usage         BattleUsage           `yaml:"usage" json:"usage"`
	Targeting     *TargetingDefinition  `yaml:"targeting,omitempty" json:"targeting,omitempty"`
	Qualification *AbilityQualification `yaml:"qualification,omitempty" json:"qualification,omitempty"`
	Selection     *DefenseSelection     `yaml:"selection,omitempty" json:"selection,omitempty"`
	Resolution    *DefenseResolution    `yaml:"resolution,omitempty" json:"resolution,omitempty"`
}

type StatusStacking struct {
	StackLimit     int    `yaml:"stack_limit" json:"stack_limit"`
	OverflowPolicy string `yaml:"overflow_policy" json:"overflow_policy"`
}

type StatusLifecycle struct {
	Persistent                 bool `yaml:"persistent" json:"persistent"`
	ConsumeOnTriggerCheckpoint bool `yaml:"consume_on_trigger_checkpoint" json:"consume_on_trigger_checkpoint"`
	ConsumeOnPlay              bool `yaml:"consume_on_play" json:"consume_on_play"`
	RemoveAfterResolution      bool `yaml:"remove_after_resolution" json:"remove_after_resolution"`
	RemoveOnDurationZero       bool `yaml:"remove_on_duration_zero" json:"remove_on_duration_zero"`
}

type StatusTriggerDefinition struct {
	ID             string                    `yaml:"id" json:"id"`
	Segment        string                    `yaml:"segment" json:"segment"`
	Phase          string                    `yaml:"phase" json:"phase"`
	Stage          string                    `yaml:"stage" json:"stage"`
	Priority       int                       `yaml:"priority" json:"priority"`
	ReactionWindow *ReactionWindowDefinition `yaml:"reaction_window,omitempty" json:"reaction_window,omitempty"`
	Operations     []BattleOperation         `yaml:"operations" json:"operations"`
}

type BattleStatusDefinition struct {
	SchemaVersion  int                       `yaml:"schema_version" json:"schema_version"`
	ID             string                    `yaml:"id" json:"id"`
	Name           string                    `yaml:"name" json:"name"`
	ActivationMode string                    `yaml:"activation_mode" json:"activation_mode"`
	Polarity       string                    `yaml:"polarity" json:"polarity"`
	Stacking       StatusStacking            `yaml:"stacking" json:"stacking"`
	Lifecycle      StatusLifecycle           `yaml:"lifecycle" json:"lifecycle"`
	Triggers       []StatusTriggerDefinition `yaml:"triggers" json:"triggers"`
	PlayableDuring []PlayTiming              `yaml:"playable_during,omitempty" json:"playable_during,omitempty"`
	Targeting      *TargetingDefinition      `yaml:"targeting,omitempty" json:"targeting,omitempty"`
	ReactionWindow *ReactionWindowDefinition `yaml:"reaction_window,omitempty" json:"reaction_window,omitempty"`
	Operations     []BattleOperation         `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// BattleOperation is a closed, validated data language.  Fields are shared by
// operation kinds so nested outcomes and ability modifiers remain declarative.
type BattleOperation struct {
	ID                string                    `yaml:"id,omitempty" json:"id,omitempty"`
	Type              string                    `yaml:"type" json:"type"`
	Target            string                    `yaml:"target,omitempty" json:"target,omitempty"`
	Amount            any                       `yaml:"amount,omitempty" json:"amount,omitempty"`
	Resource          string                    `yaml:"resource,omitempty" json:"resource,omitempty"`
	StatusID          string                    `yaml:"status_id,omitempty" json:"status_id,omitempty"`
	StackCount        int                       `yaml:"stack_count,omitempty" json:"stack_count,omitempty"`
	Repeat            string                    `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	DiceID            string                    `yaml:"dice_id,omitempty" json:"dice_id,omitempty"`
	DiceCount         int                       `yaml:"dice_count,omitempty" json:"dice_count,omitempty"`
	OnePerStatusStack bool                      `yaml:"one_per_status_stack,omitempty" json:"one_per_status_stack,omitempty"`
	ReactionWindow    *ReactionWindowDefinition `yaml:"reaction_window,omitempty" json:"reaction_window,omitempty"`
	Outcomes          []BattleOutcome           `yaml:"outcomes,omitempty" json:"outcomes,omitempty"`
	Modification      string                    `yaml:"modification,omitempty" json:"modification,omitempty"`
	Face              int                       `yaml:"face,omitempty" json:"face,omitempty"`
	Numerator         int                       `yaml:"numerator,omitempty" json:"numerator,omitempty"`
	Denominator       int                       `yaml:"denominator,omitempty" json:"denominator,omitempty"`
	Rounding          string                    `yaml:"rounding,omitempty" json:"rounding,omitempty"`
	Duration          string                    `yaml:"duration,omitempty" json:"duration,omitempty"`
	Modifier          *AbilityModifier          `yaml:"modifier,omitempty" json:"modifier,omitempty"`
}

type BattleOutcome struct {
	ID         string            `yaml:"id,omitempty" json:"id,omitempty"`
	Faces      []int             `yaml:"faces" json:"faces"`
	Operations []BattleOperation `yaml:"operations" json:"operations"`
}

type AbilityModifier struct {
	AddConditionalBonus *AbilityTier `yaml:"add_conditional_bonus,omitempty" json:"add_conditional_bonus,omitempty"`
}

type CombatantResources struct {
	StartingHandSize int `yaml:"starting_hand_size" json:"starting_hand_size"`
	StartingEnergy   int `yaml:"starting_energy" json:"starting_energy"`
	HandLimit        int `yaml:"hand_limit" json:"hand_limit"`
}

type CombatantIncome struct {
	Cards  int `yaml:"cards" json:"cards"`
	Energy int `yaml:"energy" json:"energy"`
}

type AbilityBoard struct {
	Offensive []string `yaml:"offensive" json:"offensive"`
	Defensive []string `yaml:"defensive" json:"defensive"`
}

type ControllerDefaults struct {
	Type string `yaml:"type" json:"type"`
}
type CombatantRollPreferences struct {
	StatusEffects  string `yaml:"status_effects" json:"status_effects"`
	Offensive      string `yaml:"offensive" json:"offensive"`
	AbilityEffects string `yaml:"ability_effects" json:"ability_effects"`
}

type D100Range struct {
	Start int
	End   int
}

func (r *D100Range) UnmarshalYAML(node *yaml.Node) error {
	var values []int
	if err := node.Decode(&values); err != nil {
		return err
	}
	if len(values) != 2 {
		return fmt.Errorf("D100 range must contain exactly two values")
	}
	r.Start, r.End = values[0], values[1]
	return nil
}

type AbilityActivationRanges struct {
	FirstRoll  *D100Range `yaml:"first_roll,omitempty" json:"first_roll,omitempty"`
	SecondRoll *D100Range `yaml:"second_roll,omitempty" json:"second_roll,omitempty"`
	ThirdRoll  *D100Range `yaml:"third_roll,omitempty" json:"third_roll,omitempty"`
}
type D100AbilityEntry struct {
	AbilityID        string                  `yaml:"ability_id" json:"ability_id"`
	ActivationRanges AbilityActivationRanges `yaml:"activation_ranges" json:"activation_ranges"`
}
type D100Chart struct {
	Abilities       []D100AbilityEntry `yaml:"abilities" json:"abilities"`
	NoAbilityRanges []D100Range        `yaml:"no_ability_ranges" json:"no_ability_ranges"`
}
type OffensivePlanningAI struct {
	Charts map[string]D100Chart `yaml:"charts" json:"charts"`
}
type DefensiveSelectionAI struct {
	Controller      string   `yaml:"controller" json:"controller"`
	PreferenceOrder []string `yaml:"preference_order" json:"preference_order"`
}
type CombatantAI struct {
	RevealProfiles     map[string][]int     `yaml:"reveal_profiles" json:"reveal_profiles"`
	OffensivePlanning  OffensivePlanningAI  `yaml:"offensive_planning" json:"offensive_planning"`
	DefensiveSelection DefensiveSelectionAI `yaml:"defensive_selection" json:"defensive_selection"`
}

type CombatantDefinition struct {
	SchemaVersion      int                      `yaml:"schema_version" json:"schema_version"`
	ID                 string                   `yaml:"id" json:"id"`
	Name               string                   `yaml:"name" json:"name"`
	Class              string                   `yaml:"class" json:"class"`
	ControllerDefaults ControllerDefaults       `yaml:"controller_defaults" json:"controller_defaults"`
	Resources          CombatantResources       `yaml:"resources" json:"resources"`
	Income             CombatantIncome          `yaml:"income" json:"income"`
	Decklist           []DecklistEntry          `yaml:"decklist" json:"decklist"`
	DiceLoadout        []DiceLoadoutEntry       `yaml:"dice_loadout" json:"dice_loadout"`
	AbilityBoard       AbilityBoard             `yaml:"ability_board" json:"ability_board"`
	StartingStatuses   []StartingStatus         `yaml:"starting_statuses" json:"starting_statuses"`
	StartingTokens     []StartingToken          `yaml:"starting_tokens" json:"starting_tokens"`
	RollPreferences    CombatantRollPreferences `yaml:"roll_preferences" json:"roll_preferences"`
	AI                 *CombatantAI             `yaml:"ai,omitempty" json:"ai,omitempty"`
}

var stableID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

func LoadBattleLibrary(root string) (BattleLibrary, error) {
	var symbols symbolCatalogFile
	if err := loadYAMLFile(filepath.Join(root, "symbols.yaml"), &symbols); err != nil {
		return BattleLibrary{}, err
	}
	if symbols.SchemaVersion != 1 {
		return BattleLibrary{}, fmt.Errorf("%w: symbols schema_version must be 1", ErrInvalidContent)
	}
	lib := BattleLibrary{Symbols: map[string]SymbolDefinition{}, Dice: map[string]BattleDieDefinition{}, Cards: map[string]BattleCardDefinition{}, Abilities: map[string]BattleAbilityDefinition{}, Statuses: map[string]BattleStatusDefinition{}, Combatants: map[string]CombatantDefinition{}}
	for _, symbol := range symbols.Symbols {
		if err := validateStableNamed("symbol", symbol.ID, symbol.Name); err != nil {
			return BattleLibrary{}, err
		}
		if symbol.PresentationKey == "" {
			return BattleLibrary{}, fmt.Errorf("%w: symbol %q presentation_key is required", ErrInvalidContent, symbol.ID)
		}
		if _, ok := lib.Symbols[symbol.ID]; ok {
			return BattleLibrary{}, fmt.Errorf("%w: duplicate symbol id %q", ErrInvalidContent, symbol.ID)
		}
		lib.Symbols[symbol.ID] = symbol
	}
	if err := loadBattleItems(filepath.Join(root, "dice"), &lib.Dice); err != nil {
		return BattleLibrary{}, err
	}
	if err := loadBattleItems(filepath.Join(root, "cards"), &lib.Cards); err != nil {
		return BattleLibrary{}, err
	}
	if err := loadBattleItems(filepath.Join(root, "abilities"), &lib.Abilities); err != nil {
		return BattleLibrary{}, err
	}
	if err := loadBattleItems(filepath.Join(root, "statuses"), &lib.Statuses); err != nil {
		return BattleLibrary{}, err
	}
	if err := loadBattleItems(filepath.Join(root, "combatants"), &lib.Combatants); err != nil {
		return BattleLibrary{}, err
	}
	if err := validateBattleLibrary(lib); err != nil {
		return BattleLibrary{}, err
	}
	return lib, nil
}

func loadBattleItems[T any](dir string, target *map[string]T) error {
	paths, err := yamlFiles(dir)
	if err != nil {
		return err
	}
	for _, path := range paths {
		var item T
		if err := loadYAMLFile(path, &item); err != nil {
			return err
		}
		id := battleItemID(any(item))
		if id == "" {
			return fmt.Errorf("%w: %s id is required", ErrInvalidContent, filepath.Base(path))
		}
		if _, exists := (*target)[id]; exists {
			return fmt.Errorf("%w: duplicate id %q in %s", ErrInvalidContent, id, dir)
		}
		(*target)[id] = item
	}
	return nil
}

func battleItemID(item any) string {
	switch value := item.(type) {
	case BattleDieDefinition:
		return value.ID
	case BattleCardDefinition:
		return value.ID
	case BattleAbilityDefinition:
		return value.ID
	case BattleStatusDefinition:
		return value.ID
	case CombatantDefinition:
		return value.ID
	default:
		return ""
	}
}

func validateBattleLibrary(lib BattleLibrary) error {
	for id, die := range lib.Dice {
		if die.SchemaVersion != 1 || id != die.ID {
			return fmt.Errorf("%w: dice %q has invalid schema or id", ErrInvalidContent, id)
		}
		if err := validateStableNamed("dice", id, die.Name); err != nil {
			return err
		}
		if die.SideCount < 1 || len(die.Faces) != die.SideCount {
			return fmt.Errorf("%w: dice %q faces must exactly match side_count", ErrInvalidContent, id)
		}
		seen := map[int]bool{}
		for _, face := range die.Faces {
			if face.Number < 1 || face.Number > die.SideCount || seen[face.Number] {
				return fmt.Errorf("%w: dice %q faces must cover 1..%d exactly once", ErrInvalidContent, id, die.SideCount)
			}
			seen[face.Number] = true
			if _, ok := lib.Symbols[face.Symbol]; !ok {
				return fmt.Errorf("%w: dice %q references unknown symbol %q", ErrInvalidContent, id, face.Symbol)
			}
		}
	}
	for id, status := range lib.Statuses {
		if status.SchemaVersion != 1 {
			return fmt.Errorf("%w: status %q schema_version must be 1", ErrInvalidContent, id)
		}
		if err := validateStableNamed("status", id, status.Name); err != nil {
			return err
		}
		if status.Stacking.StackLimit < 1 || status.Stacking.OverflowPolicy != "reject_additional_stacks" {
			return fmt.Errorf("%w: status %q has invalid stacking", ErrInvalidContent, id)
		}
		if status.ActivationMode != "automatic" && status.ActivationMode != "player_activated" && status.ActivationMode != "hybrid" {
			return fmt.Errorf("%w: status %q has invalid activation_mode", ErrInvalidContent, id)
		}
		if status.Lifecycle.Persistent && status.Lifecycle.RemoveAfterResolution {
			return fmt.Errorf("%w: status %q lifecycle is contradictory", ErrInvalidContent, id)
		}
		for _, trigger := range status.Triggers {
			if trigger.ID == "" || !validTiming(trigger.Segment, trigger.Phase) {
				return fmt.Errorf("%w: status %q has invalid trigger timing", ErrInvalidContent, id)
			}
			if err := validateBattleOperations(trigger.Operations, lib); err != nil {
				return fmt.Errorf("%w: status %q: %v", ErrInvalidContent, id, err)
			}
		}
		if err := validateBattleOperations(status.Operations, lib); err != nil {
			return err
		}
	}
	for id, card := range lib.Cards {
		if card.SchemaVersion != 1 {
			return fmt.Errorf("%w: card %q schema_version must be 1", ErrInvalidContent, id)
		}
		if err := validateStableNamed("card", id, card.Name); err != nil {
			return err
		}
		if card.Cost.Energy < 0 || len(card.Play.SourceZones) == 0 || card.Play.Destination == "" || len(card.Play.PlayableDuring) == 0 {
			return fmt.Errorf("%w: card %q has invalid cost or play rules", ErrInvalidContent, id)
		}
		for _, timing := range card.Play.PlayableDuring {
			if !validTiming(timing.Segment, timing.Phase) {
				return fmt.Errorf("%w: card %q has invalid timing", ErrInvalidContent, id)
			}
		}
		if err := validateBattleOperations(card.Operations, lib); err != nil {
			return fmt.Errorf("%w: card %q: %v", ErrInvalidContent, id, err)
		}
	}
	for id, ability := range lib.Abilities {
		if ability.SchemaVersion != 1 {
			return fmt.Errorf("%w: ability %q schema_version must be 1", ErrInvalidContent, id)
		}
		if err := validateStableNamed("ability", id, ability.Name); err != nil {
			return err
		}
		if ability.Cost.Energy < 0 {
			return fmt.Errorf("%w: ability %q cost must be non-negative", ErrInvalidContent, id)
		}
		if ability.Type == "offensive" {
			if ability.Qualification == nil || ability.Selection != nil || ability.Resolution != nil {
				return fmt.Errorf("%w: offensive ability %q needs qualification only", ErrInvalidContent, id)
			}
			for _, tier := range append(append([]AbilityTier{}, ability.Qualification.ActivationTiers...), ability.Qualification.ConditionalBonuses...) {
				if err := validateTier(tier, lib); err != nil {
					return fmt.Errorf("%w: ability %q: %v", ErrInvalidContent, id, err)
				}
			}
		} else if ability.Type == "defensive" {
			if ability.Selection == nil || ability.Resolution == nil || ability.Qualification != nil {
				return fmt.Errorf("%w: defensive ability %q needs selection and resolution", ErrInvalidContent, id)
			}
			if ability.Resolution.Roll != nil {
				if _, ok := lib.Dice[ability.Resolution.Roll.DiceID]; !ok {
					return fmt.Errorf("%w: ability %q references unknown dice %q", ErrInvalidContent, id, ability.Resolution.Roll.DiceID)
				}
			}
			if err := validateBattleOperations(ability.Resolution.Operations, lib); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%w: ability %q type must be offensive or defensive", ErrInvalidContent, id)
		}
	}
	for id, combatant := range lib.Combatants {
		if combatant.SchemaVersion != 1 {
			return fmt.Errorf("%w: combatant %q schema_version must be 1", ErrInvalidContent, id)
		}
		if err := validateStableNamed("combatant", id, combatant.Name); err != nil {
			return err
		}
		if combatant.Resources.HandLimit < 1 || combatant.Resources.StartingHandSize < 0 || combatant.Resources.StartingEnergy < 0 || len(combatant.Decklist) == 0 {
			return fmt.Errorf("%w: combatant %q has invalid resources or deck", ErrInvalidContent, id)
		}
		totalDice := 0
		for _, entry := range combatant.Decklist {
			if entry.Count < 1 {
				return fmt.Errorf("%w: combatant %q has invalid deck count", ErrInvalidContent, id)
			}
			if _, ok := lib.Cards[entry.CardID]; !ok {
				return fmt.Errorf("%w: combatant %q references unknown card %q", ErrInvalidContent, id, entry.CardID)
			}
		}
		for _, entry := range combatant.DiceLoadout {
			if _, ok := lib.Dice[entry.DiceID]; !ok {
				return fmt.Errorf("%w: combatant %q references unknown dice %q", ErrInvalidContent, id, entry.DiceID)
			}
			totalDice += entry.Count
		}
		if totalDice != 5 {
			return fmt.Errorf("%w: combatant %q dice loadout must contain five dice", ErrInvalidContent, id)
		}
		for _, aid := range combatant.AbilityBoard.Offensive {
			a, ok := lib.Abilities[aid]
			if !ok || a.Type != "offensive" {
				return fmt.Errorf("%w: combatant %q has invalid offensive ability %q", ErrInvalidContent, id, aid)
			}
		}
		for _, aid := range combatant.AbilityBoard.Defensive {
			a, ok := lib.Abilities[aid]
			if !ok || a.Type != "defensive" {
				return fmt.Errorf("%w: combatant %q has invalid defensive ability %q", ErrInvalidContent, id, aid)
			}
		}
		for _, s := range combatant.StartingStatuses {
			d, ok := lib.Statuses[s.DefinitionID]
			if !ok || s.Stacks < 1 || s.Stacks > d.Stacking.StackLimit {
				return fmt.Errorf("%w: combatant %q has invalid status %q", ErrInvalidContent, id, s.DefinitionID)
			}
		}
		if combatant.ControllerDefaults.Type == "ai" {
			if combatant.AI == nil {
				return fmt.Errorf("%w: AI combatant %q needs ai data", ErrInvalidContent, id)
			}
			if err := validateAI(*combatant.AI, combatant, lib); err != nil {
				return fmt.Errorf("%w: combatant %q: %v", ErrInvalidContent, id, err)
			}
		} else if combatant.AI != nil {
			return fmt.Errorf("%w: human combatant %q cannot define ai", ErrInvalidContent, id)
		}
	}
	return nil
}

func validateStableNamed(kind, id, name string) error {
	if !stableID.MatchString(id) {
		return fmt.Errorf("%w: %s id %q must be lowercase snake_case", ErrInvalidContent, kind, id)
	}
	if name == "" {
		return fmt.Errorf("%w: %s %q name is required", ErrInvalidContent, kind, id)
	}
	return nil
}
func validTiming(segment, phase string) bool {
	switch segment {
	case "ongoing_effects", "income", "offensive", "defensive", "damage_resolution":
	default:
		return false
	}
	switch phase {
	case "entry", "main", "exit":
		return true
	}
	return false
}
func validateTier(tier AbilityTier, lib BattleLibrary) error {
	if tier.ID == "" || len(tier.Requirements.All) == 0 || len(tier.Operations) == 0 {
		return fmt.Errorf("tier requires id, requirements, and operations")
	}
	for _, r := range tier.Requirements.All {
		switch r.Type {
		case "symbol_count":
			if _, ok := lib.Symbols[r.SymbolID]; !ok {
				return fmt.Errorf("unknown symbol %q", r.SymbolID)
			}
			if r.Exact == nil && r.Minimum == 0 && r.Maximum == 0 {
				return fmt.Errorf("symbol_count needs a bound")
			}
		case "number_pattern":
			if r.Pattern != "three_of_a_kind" && r.Pattern != "exact_pair" {
				return fmt.Errorf("unknown number pattern %q", r.Pattern)
			}
		case "exact_faces":
			if len(r.Faces) == 0 {
				return fmt.Errorf("exact_faces needs faces")
			}
		default:
			return fmt.Errorf("unknown requirement type %q", r.Type)
		}
	}
	return validateBattleOperations(tier.Operations, lib)
}
func validateBattleOperations(ops []BattleOperation, lib BattleLibrary) error {
	supported := map[string]bool{"noop": true, "deal_damage": true, "prevent_damage": true, "scale_damage": true, "apply_status": true, "remove_status": true, "remove_status_stack": true, "gain_resource": true, "draw_cards": true, "modify_die": true, "apply_ability_modifier": true, "adjust_max_rolls": true, "cancel_source": true, "roll_dice": true}
	for _, op := range ops {
		if !supported[op.Type] {
			return fmt.Errorf("unsupported operation type %q", op.Type)
		}
		if op.StatusID != "" {
			if _, ok := lib.Statuses[op.StatusID]; !ok {
				return fmt.Errorf("operation references unknown status %q", op.StatusID)
			}
		}
		if op.DiceID != "" {
			if _, ok := lib.Dice[op.DiceID]; !ok {
				return fmt.Errorf("operation references unknown dice %q", op.DiceID)
			}
		}
		if op.Type == "apply_ability_modifier" && (op.Duration != "battle" || op.Modifier == nil || op.Modifier.AddConditionalBonus == nil) {
			return fmt.Errorf("ability modifier must be a battle-duration conditional bonus")
		}
		if op.Modifier != nil && op.Modifier.AddConditionalBonus != nil {
			if err := validateTier(*op.Modifier.AddConditionalBonus, lib); err != nil {
				return err
			}
		}
		covered := map[int]bool{}
		for _, outcome := range op.Outcomes {
			for _, face := range outcome.Faces {
				if covered[face] {
					return fmt.Errorf("operation outcomes overlap on face %d", face)
				}
				covered[face] = true
			}
			if err := validateBattleOperations(outcome.Operations, lib); err != nil {
				return err
			}
		}
	}
	return nil
}
func validateAI(ai CombatantAI, c CombatantDefinition, lib BattleLibrary) error {
	for aid, faces := range ai.RevealProfiles {
		if _, ok := lib.Abilities[aid]; !ok {
			return fmt.Errorf("reveal profile references unknown ability %q", aid)
		}
		if len(faces) != 5 {
			return fmt.Errorf("reveal profile %q must contain five faces", aid)
		}
		for _, face := range faces {
			if face < 1 || face > 6 {
				return fmt.Errorf("reveal profile %q has invalid face", aid)
			}
		}
	}
	keys := []string{"1_roll", "2_rolls", "3_rolls"}
	for _, key := range keys {
		chart, ok := ai.OffensivePlanning.Charts[key]
		if !ok {
			return fmt.Errorf("missing D100 chart %q", key)
		}
		coverage := make([]int, 101)
		for _, entry := range chart.Abilities {
			if _, ok := ai.RevealProfiles[entry.AbilityID]; !ok {
				return fmt.Errorf("chart references missing reveal profile %q", entry.AbilityID)
			}
			for _, r := range []*D100Range{entry.ActivationRanges.FirstRoll, entry.ActivationRanges.SecondRoll, entry.ActivationRanges.ThirdRoll} {
				if r != nil {
					if err := markRange(coverage, *r); err != nil {
						return err
					}
				}
			}
		}
		for _, r := range chart.NoAbilityRanges {
			if err := markRange(coverage, r); err != nil {
				return err
			}
		}
		for i := 1; i <= 100; i++ {
			if coverage[i] != 1 {
				return fmt.Errorf("D100 chart %q does not cover %d exactly once", key, i)
			}
		}
	}
	for _, aid := range ai.DefensiveSelection.PreferenceOrder {
		a, ok := lib.Abilities[aid]
		if !ok || a.Type != "defensive" {
			return fmt.Errorf("invalid defensive preference %q", aid)
		}
	}
	return nil
}
func markRange(coverage []int, r D100Range) error {
	if r.Start < 1 || r.End > 100 || r.Start > r.End {
		return fmt.Errorf("invalid D100 range %d-%d", r.Start, r.End)
	}
	for i := r.Start; i <= r.End; i++ {
		coverage[i]++
		if coverage[i] > 1 {
			return fmt.Errorf("overlapping D100 value %d", i)
		}
	}
	return nil
}

// BattleLibraryExists is used by participant assembly without turning a
// missing optional settled catalog into an error for legacy content roots.
func BattleLibraryExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "symbols.yaml"))
	return err == nil
}

func SortedCombatantIDs(lib BattleLibrary) []string {
	ids := make([]string, 0, len(lib.Combatants))
	for id := range lib.Combatants {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
