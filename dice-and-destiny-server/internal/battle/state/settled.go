package state

import (
	"diceanddestiny/server/internal/battle/command"
	"encoding/json"
)

// SettledRuntime contains only mutable state for the YAML-driven ruleset. The
// immutable compiled catalog is pinned separately on Battle as JSON so engine
// commands never need to reload files.
type SettledRuntime struct {
	UnifiedDefense bool
	// SimultaneousPlay pins the simultaneous rule: secret Before plays, a
	// lock-in per participant, one reveal, then simultaneous rolls.
	SimultaneousPlay bool                 `json:",omitempty"`
	Simultaneous     *SettledSimultaneous `json:",omitempty"`
	// Tactics pins the Tactics Sandbox grid rule (only with unified defense
	// and simultaneous play); TacticsRuntime is its grid state.
	Tactics            bool            `json:",omitempty"`
	TacticsRuntime     *TacticsRuntime `json:",omitempty"`
	DefensePassed      map[string]bool
	Curse              *CurseRuntime `json:",omitempty"`
	ReactionReplanning bool
	Initialized        bool
	CompletedRounds    int
	Stage              string
	Window             *SettledWindow
	Actors             map[string]SettledActorRuntime
	PlanningPublic     map[string]SettledPlanningPublicState
	OffensiveSources   []SettledDamageSource
	DefenseSelections  map[string]SettledDefense
	DefenseHistory     map[string]SettledDefense
	DefensePlans       map[string]SettledDefense
	PendingDamage      *SettledDamageBatch
	TriggerBatch       *SettledTriggerBatch
	PendingBlind       *SettledBlindResolution
	Sequence           int
	Venom              *VenomRuntime `json:",omitempty"`
}

// Simultaneous steps. Before steps are secret: plays that touch statuses,
// dice, damage or another participant queue until the reveal. Rolling keeps
// self-only plays hidden until the abilities are revealed. Offense After and
// Defense Before are one step: SimultaneousDefenseBefore, where each defense
// rolls and resolves when chosen, privately.
const (
	SimultaneousOffenseBefore  = "offense_before"
	SimultaneousOffenseRolling = "offense_rolling"
	SimultaneousDefenseBefore  = "defense_before"
	SimultaneousDefenseAfter   = "defense_after"
)

// SettledSimultaneous is the authority state of one simultaneous step.
type SettledSimultaneous struct {
	Step string `json:"step"`
	// Locked lists participants who locked in for the current step.
	Locked map[string]bool `json:"locked"`
	// Rolling lists Offense participants who locked in by rolling.
	Rolling map[string]bool `json:"rolling"`
	// Queue holds every secret play of the current Before step, in play order.
	Queue []SecretPlay `json:"queue,omitempty"`
	// Public is what other participants see of each actor during a secret step.
	Public map[string]SimultaneousPublic `json:"public,omitempty"`
	// PublicDamage is the damage batch as Defense Before began: what others see
	// of each participant's incoming attacks until the reveal.
	PublicDamage *SettledDamageBatch `json:"public_damage,omitempty"`
	// Revealed is the most recent reveal, kept for redraws after a reload.
	Revealed []RevealedPlay `json:"revealed,omitempty"`
	// SecretWounds are wounds recorded by secret plays (sacrifice costs).
	SecretWounds []string `json:"secret_wounds,omitempty"`
	Sequence     int      `json:"sequence,omitempty"`
	// Resolving is set while the reveal applies queued effects.
	Resolving bool `json:"resolving,omitempty"`
}

type SimultaneousPublic struct {
	EnergyPoints     int                      `json:"energy_points"`
	HandCount        int                      `json:"hand_count"`
	DeckCount        int                      `json:"deck_count"`
	DiscardCount     int                      `json:"discard_count"`
	RemovedCount     int                      `json:"removed_count"`
	Statuses         []StatusState            `json:"statuses,omitempty"`
	AbilityModifiers []RuntimeAbilityModifier `json:"ability_modifiers,omitempty"`
}

// SecretPlay is one card played in a secret step. Immediate private effects
// (draws, energy, own piles) already happened; Steps and Legacy are applied at
// the reveal against the targets chosen when the card was played.
type SecretPlay struct {
	Sequence         int                 `json:"sequence"`
	Step             string              `json:"step"`
	ActorID          string              `json:"actor_id"`
	CardID           string              `json:"card_instance_id"`
	CardDefinitionID string              `json:"card_definition_id"`
	Steps            []SecretProgramStep `json:"steps,omitempty"`
	Legacy           *SecretLegacyPlay   `json:"legacy,omitempty"`
	// ProtectSourceID spends the actor's Protect status on this attack.
	ProtectSourceID string `json:"protect_source_id,omitempty"`
	// Immediate plays resolved in full when played; the reveal only shows them.
	Immediate bool     `json:"immediate,omitempty"`
	Targets   []string `json:"targets,omitempty"`
}

type SecretProgramStep struct {
	Step    json.RawMessage `json:"step"`
	Targets []string        `json:"targets"`
}

type SecretLegacyPlay struct {
	TargetIDs []string `json:"target_ids,omitempty"`
	AbilityID string   `json:"ability_id,omitempty"`
	DieIndex  int      `json:"die_index"`
	StatusID  string   `json:"status_id,omitempty"`
}

// RevealedPlay is the public record of one secret play at its reveal.
type RevealedPlay struct {
	Sequence         int      `json:"sequence"`
	Step             string   `json:"step"`
	ActorID          string   `json:"actor_id"`
	CardID           string   `json:"card_instance_id"`
	CardDefinitionID string   `json:"card_definition_id"`
	Targets          []string `json:"targets,omitempty"`
	// Outcome is "resolved", or "no_effect" when every target was gone.
	Outcome string `json:"outcome"`
}

func cloneSimultaneous(value *SettledSimultaneous) *SettledSimultaneous {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var result SettledSimultaneous
	if err = json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return &result
}

type SettledPlanningPublicState struct {
	EnergyPoints     int
	HandCount        int
	DeckCount        int
	DiscardCount     int
	RemovedCount     int
	AbilityModifiers []RuntimeAbilityModifier
}

type SettledActorRuntime struct {
	PaidOffensiveEnergy int            `json:",omitempty"`
	CardExecution       *CardExecution `json:",omitempty"`
	CardUses            map[string]int `json:",omitempty"`
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
	// AmbushChoose is how many dice the actor must keep from a first roll
	// that rolled bonus dice (an Ambush); zero when no choice is pending.
	AmbushChoose int `json:",omitempty"`
	// DeckTop and DeckBottom are draw-pile cards whose place the owner
	// knows (looked at and put back): draws take DeckTop first, in order,
	// and DeckBottom only once nothing else is left. The rest of the draw
	// pile is drawn at random. Entries that left the draw pile are ignored.
	DeckTop    []string `json:",omitempty"`
	DeckBottom []string `json:",omitempty"`
	// RestoreStatuses holds the stacks of restore-each-round statuses the
	// actor started the battle with; Ongoing Effects tops them back up.
	RestoreStatuses     map[string]int `json:",omitempty"`
	QualifiedAbilityIDs []string
	SelectedAbilityID   string
	SelectedTierID      string
	SelectedToxins      []string
	SelectedTargetIDs   []string
	SelectedSourceID    string
	PlanningCommitted   bool
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
	ProgramBonus          json.RawMessage `json:"program_bonus,omitempty"`
	ConsumeOnUse          bool            `json:"consume_on_use,omitempty"`
	ExpiresAfterOffensive bool            `json:"expires_after_offensive,omitempty"`
	StatusID              string          `json:"status_id,omitempty"`
	ExpiresAfterRound     int             `json:"expires_after_round,omitempty"`
	SourceCardInstanceID  string          `json:"source_card_instance_id"`
	AbilityID             string          `json:"ability_id"`
	BonusID               string          `json:"bonus_id"`
}

type SettledWindow struct {
	ID               string
	PendingInputID   string
	Purpose          string
	Stage            string
	ReactionRound    int
	RequiredActorID  string
	RequiredActorIDs []string
	PriorityActorIDs []string
	PriorityIndex    int
	AllowedCommands  []command.Type
	Passes           map[string]bool
	ResponsePlayed   bool
	SourceID         string
}

type SettledDamageSource struct {
	// Keeps sequential reductions equivalent to the published final amount
	// when card prevention can precede a multiplicative defensive ability.
	ResolutionAdjustment int                        `json:"resolution_adjustment,omitempty"`
	ID                   string                     `json:"id"`
	SourceActorID        string                     `json:"source_actor_id"`
	SourceContentID      string                     `json:"source_content_id"`
	TargetActorID        string                     `json:"target_actor_id"`
	BaseAmount           int                        `json:"base_amount"`
	Prevention           int                        `json:"prevention"`
	ReactionPrevention   int                        `json:"reaction_prevention,omitempty"`
	ScaleNumerator       int                        `json:"scale_numerator,omitempty"`
	ScaleDenominator     int                        `json:"scale_denominator,omitempty"`
	FinalAmount          int                        `json:"final_amount"`
	StatusApplications   []SettledStatusApplication `json:"status_applications,omitempty"`
}

type SettledStatusApplication struct {
	RequirePoison bool   `json:"require_poison,omitempty"`
	SourceActorID string `json:"source_actor_id,omitempty"`
	TargetActorID string `json:"target_actor_id"`
	StatusID      string `json:"status_id"`
	Stacks        int    `json:"stacks"`
	// Applied marks a unified-defense status that already landed when the
	// defense against its attack resolved; the final commit skips it.
	Applied bool `json:"applied,omitempty"`
}

type SettledDefense struct {
	RolledDice   []RolledDie `json:"rolled_dice,omitempty"`
	ActorID      string      `json:"actor_id"`
	AbilityID    string      `json:"ability_id"`
	SourceID     string      `json:"source_id"`
	RolledFace   int         `json:"rolled_face,omitempty"`
	RolledFaces  []int       `json:"rolled_faces,omitempty"`
	CatalystPaid bool        `json:"catalyst_paid,omitempty"`
	Finalized    bool        `json:"finalized"`
	// Prevented and EnergyGained record what a defense resolved during
	// Defense Before did, so a later reroll of its dice re-resolves it.
	Prevented    int `json:"prevented,omitempty"`
	EnergyGained int `json:"energy_gained,omitempty"`
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
	ActorID           string `json:"actor_id"`
	SourceContentType string `json:"source_content_type"`
	SourceContentID   string `json:"source_content_id"`
	StatusInstanceID  string `json:"status_instance_id,omitempty"`
	// StatusID is retained in snapshots for backwards compatibility. New
	// consumers should use SourceContentID for every activation source.
	StatusID       string    `json:"status_id,omitempty"`
	TriggerID      string    `json:"trigger_id,omitempty"`
	OperationID    string    `json:"operation_id,omitempty"`
	OperationIndex int       `json:"operation_index"`
	Die            RolledDie `json:"die"`
	Resolved       bool      `json:"resolved,omitempty"`
	Rerolled       bool      `json:"rerolled,omitempty"`
	FaceSet        bool      `json:"face_set,omitempty"`
}

type SettledStatusRemoval struct {
	ActorID  string `json:"actor_id"`
	StatusID string `json:"status_id"`
	Stacks   int    `json:"stacks"`
	// ForBattle keeps the removed stacks of a restore-each-round status from
	// coming back at later Ongoing Effects.
	ForBattle bool `json:"for_battle,omitempty"`
}

type SettledBlindResolution struct {
	ActorID  string
	StatusID string
	DieID    string
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
	cloned.DefensePassed = make(map[string]bool, len(value.DefensePassed))
	for id, passed := range value.DefensePassed {
		cloned.DefensePassed[id] = passed
	}
	if value.Curse != nil {
		cloned.Curse = CloneCurse(value.Curse)
	}
	if value.Venom != nil {
		cloned.Venom = CloneVenom(value.Venom)
	}
	cloned.Simultaneous = cloneSimultaneous(value.Simultaneous)
	cloned.TacticsRuntime = CloneTactics(value.TacticsRuntime)
	cloned.Actors = make(map[string]SettledActorRuntime, len(value.Actors))
	for id, actor := range value.Actors {
		cloned.Actors[id] = cloneSettledActor(actor)
	}
	cloned.PlanningPublic = make(map[string]SettledPlanningPublicState, len(value.PlanningPublic))
	for actorID, public := range value.PlanningPublic {
		public.AbilityModifiers = append([]RuntimeAbilityModifier(nil), public.AbilityModifiers...)
		cloned.PlanningPublic[actorID] = public
	}
	cloned.OffensiveSources = append([]SettledDamageSource(nil), value.OffensiveSources...)
	for i := range cloned.OffensiveSources {
		cloned.OffensiveSources[i].StatusApplications = append([]SettledStatusApplication(nil), value.OffensiveSources[i].StatusApplications...)
	}
	cloned.DefensePlans = make(map[string]SettledDefense, len(value.DefensePlans))
	for id, defense := range value.DefensePlans {
		defense.RolledFaces = append([]int(nil), defense.RolledFaces...)
		defense.RolledDice = copyRolledDice(defense.RolledDice)
		cloned.DefensePlans[id] = defense
	}
	cloned.DefenseHistory = make(map[string]SettledDefense, len(value.DefenseHistory))
	for id, defense := range value.DefenseHistory {
		defense.RolledFaces = append([]int(nil), defense.RolledFaces...)
		defense.RolledDice = copyRolledDice(defense.RolledDice)
		cloned.DefenseHistory[id] = defense
	}
	cloned.DefenseSelections = make(map[string]SettledDefense, len(value.DefenseSelections))
	for id, defense := range value.DefenseSelections {
		defense.RolledFaces = append([]int(nil), defense.RolledFaces...)
		defense.RolledDice = copyRolledDice(defense.RolledDice)
		cloned.DefenseSelections[id] = defense
	}
	if value.Window != nil {
		window := *value.Window
		window.AllowedCommands = append([]command.Type(nil), value.Window.AllowedCommands...)
		window.RequiredActorIDs = copyStrings(value.Window.RequiredActorIDs)
		window.PriorityActorIDs = copyStrings(value.Window.PriorityActorIDs)
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
	if value.CardExecution != nil {
		c := *value.CardExecution
		c.Steps = append([]byte(nil), c.Steps...)
		c.Feedback = append([]ProgramDieFeedback(nil), c.Feedback...)
		c.Selected = copyStrings(c.Selected)
		c.Drawn = copyStrings(c.Drawn)
		c.Looked = copyStrings(c.Looked)
		c.Deferred = append([]SecretProgramStep(nil), c.Deferred...)
		for i := range c.Deferred {
			c.Deferred[i].Step = append([]byte(nil), c.Deferred[i].Step...)
			c.Deferred[i].Targets = copyStrings(c.Deferred[i].Targets)
		}
		value.CardExecution = &c
	}
	uses2 := map[string]int{}
	for k, v := range value.CardUses {
		uses2[k] = v
	}
	value.CardUses = uses2
	value.DeckTop = copyStrings(value.DeckTop)
	value.DeckBottom = copyStrings(value.DeckBottom)
	if value.RestoreStatuses != nil {
		restore := make(map[string]int, len(value.RestoreStatuses))
		for k, v := range value.RestoreStatuses {
			restore[k] = v
		}
		value.RestoreStatuses = restore
	}
	value.SelectedToxins = copyStrings(value.SelectedToxins)
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

// CardExecution persists ordered effect choices between authority commands.
type CardExecution struct {
	Feedback []ProgramDieFeedback
	CardID   string
	Steps    []byte
	Selected []string
	Drawn    []string
	// Removed lists cards this play sacrificed, so a later step cannot revive them.
	Removed []string `json:",omitempty"`
	// Looked lists the draw-pile cards this play looked at, top first.
	Looked []string `json:",omitempty"`
	Paid   bool
	// Deferred holds steps a secret play queues for the reveal.
	Deferred []SecretProgramStep `json:",omitempty"`
}

type ProgramDieFeedback struct {
	ActorID    string `json:"actor_id"`
	Index      int    `json:"index"`
	FaceBefore int    `json:"face_before"`
	Face       int    `json:"face"`
	Rolled     bool   `json:"rolled"`
}
