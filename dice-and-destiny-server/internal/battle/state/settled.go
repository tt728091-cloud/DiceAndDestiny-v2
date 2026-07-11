package state

import "diceanddestiny/server/internal/battle/command"

// SettledRuntime contains only mutable state for the YAML-driven ruleset. The
// immutable compiled catalog is pinned separately on Battle as JSON so engine
// commands never need to reload files.
type SettledRuntime struct {
	Initialized       bool
	CompletedRounds   int
	Stage             string
	Window            *SettledWindow
	Actors            map[string]SettledActorRuntime
	OffensiveSources  []SettledDamageSource
	DefenseSelections map[string]SettledDefense
	PendingDamage     *SettledDamageBatch
	TriggerBatch      *SettledTriggerBatch
	PendingBlind      *SettledBlindResolution
	Sequence          int
}

type SettledActorRuntime struct {
	IncomeCards         int
	IncomeEnergy        int
	HandLimit           int
	OffensiveAbilityIDs []string
	DefensiveAbilityIDs []string
	CardInstances       map[string]CardInstance
	RollHistory         []RollBatch
	FinalDice           []RolledDie
	KeptIndices         []int
	RollsUsed           int
	MaxRolls            int
	QualifiedAbilityIDs []string
	SelectedAbilityID   string
	SelectedTierID      string
	SelectedTargetIDs   []string
	SelectedSourceID    string
	AID100              int
	AISimulatedRolls    int
	AbilityModifiers    []RuntimeAbilityModifier
	UsedAbilities       map[string]int
}

type CardInstance struct {
	InstanceID   string `json:"instance_id"`
	DefinitionID string `json:"definition_id"`
}

type RollBatch struct {
	Number        int         `json:"number"`
	RolledIndices []int       `json:"rolled_indices"`
	Dice          []RolledDie `json:"dice"`
	KeptIndices   []int       `json:"kept_indices,omitempty"`
}

type RuntimeAbilityModifier struct {
	SourceCardInstanceID string `json:"source_card_instance_id"`
	AbilityID            string `json:"ability_id"`
	BonusID              string `json:"bonus_id"`
}

type SettledWindow struct {
	ID              string
	PendingInputID  string
	Purpose         string
	Stage           string
	ReactionRound   int
	RequiredActorID string
	AllowedCommands []command.Type
	Passes          map[string]bool
	ResponsePlayed  bool
	SourceID        string
}

type SettledDamageSource struct {
	ID                 string                     `json:"id"`
	SourceActorID      string                     `json:"source_actor_id"`
	SourceContentID    string                     `json:"source_content_id"`
	TargetActorID      string                     `json:"target_actor_id"`
	BaseAmount         int                        `json:"base_amount"`
	Prevention         int                        `json:"prevention"`
	ReactionPrevention int                        `json:"reaction_prevention,omitempty"`
	ScaleNumerator     int                        `json:"scale_numerator,omitempty"`
	ScaleDenominator   int                        `json:"scale_denominator,omitempty"`
	FinalAmount        int                        `json:"final_amount"`
	StatusApplications []SettledStatusApplication `json:"status_applications,omitempty"`
}

type SettledStatusApplication struct {
	TargetActorID string `json:"target_actor_id"`
	StatusID      string `json:"status_id"`
	Stacks        int    `json:"stacks"`
}

type SettledDefense struct {
	ActorID    string `json:"actor_id"`
	AbilityID  string `json:"ability_id"`
	SourceID   string `json:"source_id"`
	RolledFace int    `json:"rolled_face,omitempty"`
	Finalized  bool   `json:"finalized"`
}

type SettledDamageBatch struct {
	ID           string                     `json:"id"`
	Sources      []SettledDamageSource      `json:"sources"`
	Removals     []ProposedCardRemoval      `json:"removals"`
	Overage      map[string]int             `json:"overage,omitempty"`
	Revealed     bool                       `json:"revealed"`
	Committed    bool                       `json:"committed"`
	Applications []SettledStatusApplication `json:"status_applications,omitempty"`
}

type SettledTriggerBatch struct {
	ID                string
	StatusInstanceIDs []string
	Rolls             []SettledEffectRoll
	Damage            []SettledDamageSource
	RemoveStacks      []SettledStatusRemoval
	Reactable         bool
	Finalized         bool
}

type SettledEffectRoll struct {
	ActorID          string    `json:"actor_id"`
	StatusInstanceID string    `json:"status_instance_id"`
	StatusID         string    `json:"status_id"`
	Die              RolledDie `json:"die"`
}

type SettledStatusRemoval struct {
	ActorID  string `json:"actor_id"`
	StatusID string `json:"status_id"`
	Stacks   int    `json:"stacks"`
}

type SettledBlindResolution struct {
	ActorID  string
	StatusID string
	Face     int
}

func settledRuntimeFromSetup(setup BattleSetup) *SettledRuntime {
	if len(setup.SettledCatalog) == 0 {
		return nil
	}
	actors := make(map[string]SettledActorRuntime, len(setup.SettledActors))
	for id, actor := range setup.SettledActors {
		actors[id] = cloneSettledActor(actor)
	}
	return &SettledRuntime{
		Actors:            actors,
		DefenseSelections: make(map[string]SettledDefense),
	}
}

func cloneSettledRuntime(value *SettledRuntime) *SettledRuntime {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Actors = make(map[string]SettledActorRuntime, len(value.Actors))
	for id, actor := range value.Actors {
		cloned.Actors[id] = cloneSettledActor(actor)
	}
	cloned.OffensiveSources = append([]SettledDamageSource(nil), value.OffensiveSources...)
	for i := range cloned.OffensiveSources {
		cloned.OffensiveSources[i].StatusApplications = append([]SettledStatusApplication(nil), value.OffensiveSources[i].StatusApplications...)
	}
	cloned.DefenseSelections = make(map[string]SettledDefense, len(value.DefenseSelections))
	for id, defense := range value.DefenseSelections {
		cloned.DefenseSelections[id] = defense
	}
	if value.Window != nil {
		window := *value.Window
		window.AllowedCommands = append([]command.Type(nil), value.Window.AllowedCommands...)
		window.Passes = copyBoolMap(value.Window.Passes)
		cloned.Window = &window
	}
	if value.PendingDamage != nil {
		batch := *value.PendingDamage
		batch.Sources = append([]SettledDamageSource(nil), value.PendingDamage.Sources...)
		batch.Removals = append([]ProposedCardRemoval(nil), value.PendingDamage.Removals...)
		batch.Overage = copyIntMap(value.PendingDamage.Overage)
		batch.Applications = append([]SettledStatusApplication(nil), value.PendingDamage.Applications...)
		cloned.PendingDamage = &batch
	}
	if value.TriggerBatch != nil {
		batch := *value.TriggerBatch
		batch.StatusInstanceIDs = copyStrings(value.TriggerBatch.StatusInstanceIDs)
		batch.Rolls = append([]SettledEffectRoll(nil), value.TriggerBatch.Rolls...)
		batch.Damage = append([]SettledDamageSource(nil), value.TriggerBatch.Damage...)
		batch.RemoveStacks = append([]SettledStatusRemoval(nil), value.TriggerBatch.RemoveStacks...)
		cloned.TriggerBatch = &batch
	}
	if value.PendingBlind != nil {
		blind := *value.PendingBlind
		cloned.PendingBlind = &blind
	}
	return &cloned
}

func cloneSettledActor(value SettledActorRuntime) SettledActorRuntime {
	value.OffensiveAbilityIDs = copyStrings(value.OffensiveAbilityIDs)
	value.DefensiveAbilityIDs = copyStrings(value.DefensiveAbilityIDs)
	cards := make(map[string]CardInstance, len(value.CardInstances))
	for id, card := range value.CardInstances {
		cards[id] = card
	}
	value.CardInstances = cards
	value.RollHistory = append([]RollBatch(nil), value.RollHistory...)
	for i := range value.RollHistory {
		value.RollHistory[i].RolledIndices = append([]int(nil), value.RollHistory[i].RolledIndices...)
		value.RollHistory[i].Dice = copyRolledDice(value.RollHistory[i].Dice)
		value.RollHistory[i].KeptIndices = append([]int(nil), value.RollHistory[i].KeptIndices...)
	}
	value.FinalDice = copyRolledDice(value.FinalDice)
	value.KeptIndices = append([]int(nil), value.KeptIndices...)
	value.QualifiedAbilityIDs = copyStrings(value.QualifiedAbilityIDs)
	value.SelectedTargetIDs = copyStrings(value.SelectedTargetIDs)
	value.AbilityModifiers = append([]RuntimeAbilityModifier(nil), value.AbilityModifiers...)
	uses := make(map[string]int, len(value.UsedAbilities))
	for id, count := range value.UsedAbilities {
		uses[id] = count
	}
	value.UsedAbilities = uses
	return value
}

func copyBoolMap(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	result := make(map[string]bool, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
