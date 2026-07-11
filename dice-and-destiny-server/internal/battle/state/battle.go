package state

import (
	"errors"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
)

type Battle struct {
	ID                 string
	Status             BattleStatus
	EscapeRequested    bool
	Segment            segment.State
	Flow               SegmentFlowState
	ActiveResolutionID string
	Resolutions        map[string]ResolutionState
	Actors             map[string]ActorState
	DiceDefinitions    map[string]DiceDefinition
	RollRequests       map[string]RollRequest
	Commitments        map[string]OffensiveCommitment
	OffensiveProposals []PlanningProposal
	DefensiveProposals []PlanningProposal
	DamageResolution   *DamageResolutionState
	PendingOperations  []FinalizedOperationProposal
	Content            ContentCatalog
	Origin             BattleOrigin
	Random             RandomState
	SettledCatalog     []byte
	Settled            *SettledRuntime
}

type BattleOrigin struct {
	Kind                string `json:"kind"`
	ScenarioID          string `json:"scenario_id,omitempty"`
	ScenarioSchema      int    `json:"scenario_schema,omitempty"`
	ScenarioFingerprint string `json:"scenario_fingerprint,omitempty"`
	CreatedBy           string `json:"created_by,omitempty"`
}

const (
	BattleOriginNormal   = "normal"
	BattleOriginScenario = "scenario"
)

type RandomState struct {
	Mode      string `json:"mode"`
	Algorithm string `json:"algorithm"`
	Seed      uint64 `json:"seed,omitempty"`
	Cursor    uint64 `json:"cursor"`
}

const (
	RandomModeNormal       = "normal"
	RandomModeReproducible = "reproducible"
	RandomAlgorithmCrypto  = "crypto"
	RandomAlgorithmSHA256  = "sha256-counter-v1"
)

type ContentCatalog struct {
	Cards     map[string]RuntimeContentDefinition
	Abilities map[string]RuntimeContentDefinition
	Statuses  map[string]RuntimeStatusDefinition
}

type RuntimeContentDefinition struct {
	ID              string
	Segments        []segment.Segment
	EnergyCost      int
	RequiresTarget  bool
	DiceRequirement string
	Operations      []operation.Plan
}

type RuntimeStatusDefinition struct {
	ID                  string
	StackLimit          int
	StackOverflowPolicy string
	Triggers            []RuntimeStatusTrigger
}

type RuntimeStatusTrigger struct {
	ID         string
	Segment    segment.Segment
	Phase      FlowPhase
	Priority   int
	Operations []operation.Plan
}

type ActorState struct {
	DefinitionID string
	Controller   ControllerType
	Character    CharacterMetadata
	Resources    ResourceState
	// EnergyPoints is a compatibility alias kept synchronized with
	// Resources.EnergyPoints while older engine helpers migrate.
	EnergyPoints    int
	Health          HealthMetadata
	Decklist        []DecklistEntry
	Cards           CardZones
	DiceLoadout     []DiceLoadoutEntry
	Dice            DiceState
	AbilityIDs      []string
	Statuses        []StatusState
	Tokens          []TokenState
	RollPreferences RollPreferences
	DefeatState     ActorDefeatState
}

type CharacterMetadata struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type ResourceState struct {
	StartingHandSize     int
	MaxHandSize          int
	StartingEnergyPoints int
	MaxEnergyPoints      int
	EnergyPoints         int
}

type HealthMetadata struct {
	Model     string
	MaxHealth int
}

type DecklistEntry struct {
	CardID string `json:"card_id"`
	Count  int    `json:"count"`
}

type StatusState struct {
	InstanceID   string `json:"instance_id" yaml:"instance_id"`
	DefinitionID string `json:"definition_id" yaml:"definition_id"`
	Stacks       int    `json:"stacks" yaml:"stacks"`
}

type TokenState struct {
	ID    string `json:"id" yaml:"id"`
	Value int    `json:"value" yaml:"value"`
}

type RollMode string

const (
	RollModeAutomatic RollMode = "automatic"
	RollModeManual    RollMode = "manual"
)

type RollPreferences struct {
	StatusEffects RollMode `json:"status_effects"`
	Offensive     RollMode `json:"offensive"`
	Defensive     RollMode `json:"defensive"`
}

type BattleStatus string

const (
	BattleActive  BattleStatus = "active"
	BattleVictory BattleStatus = "victory"
	BattleDefeat  BattleStatus = "defeat"
	BattleDraw    BattleStatus = "draw"
	BattleEscaped BattleStatus = "escaped"
)

type ControllerType string

const (
	ControllerHuman  ControllerType = "human"
	ControllerAI     ControllerType = "ai"
	ControllerSystem ControllerType = "system"
)

type ActorProgressStatus string

const (
	ActorResolvingAutomatic ActorProgressStatus = "resolving_automatic"
	ActorNeedsInput         ActorProgressStatus = "needs_input"
	ActorLockedIn           ActorProgressStatus = "locked_in"
	ActorResolved           ActorProgressStatus = "resolved"
	ActorNotParticipating   ActorProgressStatus = "not_participating"
)

type SegmentFlowState struct {
	Segment      segment.Segment
	Round        int
	Entered      bool
	ExitStarted  bool
	Stage        string
	Iteration    int
	Actors       map[string]ActorFlowState
	PendingInput map[string]PendingInput
}

type ActorFlowState struct {
	Status       ActorProgressStatus
	ReasonCode   string
	CommitmentID string
}

type PendingInput struct {
	ID              string
	ActorID         string
	Segment         segment.Segment
	Phase           FlowPhase
	Stage           string
	Iteration       int
	WindowID        string
	ReactionRound   int
	PlanningCycle   int
	InputType       string
	SourceType      string
	SourceID        string
	AllowedCommands []command.Type
}

// OffensiveCommitment is authoritative private planning state. Story 3 only
// persists it; reveal and reaction mechanics are intentionally deferred.
type OffensiveCommitment struct {
	ID              string
	ActorID         string
	FinalDice       []RolledDie
	RollsUsed       int
	SelectedAbility string
	SelectedCards   []string
	SelectedTargets []string
	RollHistory     [][]RolledDie
}

type CardZones struct {
	Deck    []string `json:"deck" yaml:"deck"`
	Hand    []string `json:"hand" yaml:"hand"`
	Discard []string `json:"discard" yaml:"discard"`
	Removed []string `json:"removed" yaml:"removed"`
}

type BattleSetup struct {
	Actors          []ActorSetup
	DiceDefinitions []DiceDefinition
	Content         ContentCatalog
	SettledCatalog  []byte
	SettledActors   map[string]SettledActorRuntime
}

type ActorSetup struct {
	ID              string
	DefinitionID    string
	ControllerType  ControllerType
	Character       CharacterMetadata
	Resources       ResourceState
	Health          HealthMetadata
	Decklist        []DecklistEntry
	Deck            []string
	Hand            []string
	Discard         []string
	Removed         []string
	DiceLoadout     []DiceLoadoutEntry
	AbilityIDs      []string
	Statuses        []StatusState
	Tokens          []TokenState
	RollPreferences RollPreferences
}

type DiceDefinition struct {
	ID        string
	Name      string
	DieType   string
	SideCount int
	Faces     []DiceFace
}

type DiceFace struct {
	Face    int
	Value   int
	Symbols []string
}

type DiceLoadoutEntry struct {
	DiceID string `json:"dice_id"`
	Count  int    `json:"count"`
}

type DiceState struct {
	CurrentRoll *RollState
}

type RollPool string

const (
	RollPoolOffensive RollPool = "offensive"
	RollPoolDefensive RollPool = "defensive"
	RollPoolEffect    RollPool = "effect"
	RollPoolCard      RollPool = "card"
)

type RollSourceType string

const (
	RollSourceSegment RollSourceType = "segment"
	RollSourceAbility RollSourceType = "ability"
	RollSourceCard    RollSourceType = "card"
	RollSourceStatus  RollSourceType = "status"
	RollSourceSystem  RollSourceType = "system"
)

type RollRequest struct {
	ID          string
	ActorID     string
	Segment     segment.Segment
	Pool        RollPool
	SourceType  RollSourceType
	SourceID    string
	DiceLoadout []DiceLoadoutEntry
	MaxRolls    int
	Required    bool
	Complete    bool
}

type RollState struct {
	RequestID    string
	ActorID      string
	Segment      segment.Segment
	Pool         RollPool
	SourceType   RollSourceType
	SourceID     string
	Dice         []RolledDie
	KeptIndices  []int
	RollsUsed    int
	MaxRolls     int
	Combinations []string
	SymbolCounts map[string]int
	Complete     bool
}

type RolledDie struct {
	Index   int      `json:"index"`
	DieID   string   `json:"die_id"`
	Face    int      `json:"face"`
	Value   int      `json:"value"`
	Symbols []string `json:"symbols"`
}

func NewBattle(id string) (Battle, error) {
	return NewBattleFromSetup(id, BattleSetup{})
}

func NewBattleFromSetup(id string, setup BattleSetup) (Battle, error) {
	if id == "" {
		return Battle{}, errors.New("battle id is required")
	}

	actors := make(map[string]ActorState, len(setup.Actors))
	for _, actor := range setup.Actors {
		if actor.ID == "" {
			return Battle{}, errors.New("actor id is required")
		}
		if _, exists := actors[actor.ID]; exists {
			return Battle{}, errors.New("duplicate actor id")
		}
		controller := actor.ControllerType
		if controller == "" {
			controller = ControllerHuman
		}
		if !IsValidControllerType(controller) {
			return Battle{}, errors.New("invalid actor controller type")
		}

		actors[actor.ID] = ActorState{
			DefinitionID:    actor.DefinitionID,
			Controller:      controller,
			Character:       actor.Character,
			Resources:       actor.Resources,
			EnergyPoints:    actor.Resources.EnergyPoints,
			Health:          actor.Health,
			Decklist:        copyDecklist(actor.Decklist),
			DiceLoadout:     copyDiceLoadout(actor.DiceLoadout),
			AbilityIDs:      copyStrings(actor.AbilityIDs),
			Statuses:        copyStatuses(actor.Statuses),
			Tokens:          copyTokens(actor.Tokens),
			RollPreferences: actor.RollPreferences,
			Cards: CardZones{
				Deck:    append([]string(nil), actor.Deck...),
				Hand:    append([]string(nil), actor.Hand...),
				Discard: append([]string(nil), actor.Discard...),
				Removed: append([]string(nil), actor.Removed...),
			},
		}
	}

	initial := segment.NewManager().InitialState()
	return Battle{
		ID:              id,
		Status:          BattleActive,
		Segment:         initial,
		Flow:            NewSegmentFlowState(initial),
		Resolutions:     make(map[string]ResolutionState),
		Actors:          actors,
		DiceDefinitions: copyDiceDefinitions(setup.DiceDefinitions),
		RollRequests:    make(map[string]RollRequest),
		Commitments:     make(map[string]OffensiveCommitment),
		Content:         cloneContentCatalog(setup.Content),
		SettledCatalog:  append([]byte(nil), setup.SettledCatalog...),
		Origin:          BattleOrigin{Kind: BattleOriginNormal},
		Random:          RandomState{Mode: RandomModeNormal, Algorithm: RandomAlgorithmCrypto},
		Settled:         settledRuntimeFromSetup(setup),
	}, nil
}

func IsTerminalBattleStatus(status BattleStatus) bool {
	switch status {
	case BattleVictory, BattleDefeat, BattleDraw, BattleEscaped:
		return true
	default:
		return false
	}
}

func NewSegmentFlowState(current segment.State) SegmentFlowState {
	return SegmentFlowState{
		Segment:      current.Current,
		Round:        current.Round,
		Actors:       make(map[string]ActorFlowState),
		PendingInput: make(map[string]PendingInput),
	}
}

func IsValidControllerType(controller ControllerType) bool {
	switch controller {
	case ControllerHuman, ControllerAI, ControllerSystem:
		return true
	default:
		return false
	}
}

func IsValidActorProgressStatus(status ActorProgressStatus) bool {
	switch status {
	case ActorResolvingAutomatic, ActorNeedsInput, ActorLockedIn, ActorResolved, ActorNotParticipating:
		return true
	default:
		return false
	}
}

func (battle Battle) Clone() Battle {
	cloned := battle
	cloned.Actors = cloneActors(battle.Actors)
	cloned.Resolutions = cloneResolutions(battle.Resolutions)
	cloned.DiceDefinitions = copyDiceDefinitionMap(battle.DiceDefinitions)
	cloned.RollRequests = cloneRollRequests(battle.RollRequests)
	cloned.Commitments = cloneCommitments(battle.Commitments)
	cloned.OffensiveProposals = clonePlanningProposals(battle.OffensiveProposals)
	cloned.DefensiveProposals = clonePlanningProposals(battle.DefensiveProposals)
	cloned.DamageResolution = cloneDamageResolution(battle.DamageResolution)
	cloned.PendingOperations = append([]FinalizedOperationProposal(nil), battle.PendingOperations...)
	for i := range cloned.PendingOperations {
		cloned.PendingOperations[i] = cloneFinalizedOperation(battle.PendingOperations[i])
	}
	cloned.Content = cloneContentCatalog(battle.Content)
	cloned.SettledCatalog = append([]byte(nil), battle.SettledCatalog...)
	cloned.Settled = cloneSettledRuntime(battle.Settled)
	cloned.Flow = cloneFlowState(battle.Flow)
	return cloned
}

func cloneActors(values map[string]ActorState) map[string]ActorState {
	if values == nil {
		return nil
	}
	copied := make(map[string]ActorState, len(values))
	for id, actor := range values {
		actor.Cards = CardZones{
			Deck:    copyStrings(actor.Cards.Deck),
			Hand:    copyStrings(actor.Cards.Hand),
			Discard: copyStrings(actor.Cards.Discard),
			Removed: copyStrings(actor.Cards.Removed),
		}
		actor.DiceLoadout = copyDiceLoadout(actor.DiceLoadout)
		actor.Decklist = copyDecklist(actor.Decklist)
		actor.AbilityIDs = copyStrings(actor.AbilityIDs)
		actor.Statuses = copyStatuses(actor.Statuses)
		actor.Tokens = copyTokens(actor.Tokens)
		if actor.Dice.CurrentRoll != nil {
			roll := *actor.Dice.CurrentRoll
			roll.Dice = copyRolledDice(actor.Dice.CurrentRoll.Dice)
			roll.KeptIndices = append([]int(nil), actor.Dice.CurrentRoll.KeptIndices...)
			roll.Combinations = copyStrings(actor.Dice.CurrentRoll.Combinations)
			roll.SymbolCounts = copyIntMap(actor.Dice.CurrentRoll.SymbolCounts)
			actor.Dice.CurrentRoll = &roll
		}
		copied[id] = actor
	}
	return copied
}

func copyDiceDefinitionMap(values map[string]DiceDefinition) map[string]DiceDefinition {
	if values == nil {
		return nil
	}
	definitions := make([]DiceDefinition, 0, len(values))
	for _, definition := range values {
		definitions = append(definitions, definition)
	}
	return copyDiceDefinitions(definitions)
}

func cloneRollRequests(values map[string]RollRequest) map[string]RollRequest {
	if values == nil {
		return nil
	}
	copied := make(map[string]RollRequest, len(values))
	for id, request := range values {
		request.DiceLoadout = copyDiceLoadout(request.DiceLoadout)
		copied[id] = request
	}
	return copied
}

func cloneCommitments(values map[string]OffensiveCommitment) map[string]OffensiveCommitment {
	if values == nil {
		return nil
	}
	copied := make(map[string]OffensiveCommitment, len(values))
	for id, commitment := range values {
		commitment.FinalDice = copyRolledDice(commitment.FinalDice)
		commitment.SelectedCards = copyStrings(commitment.SelectedCards)
		commitment.SelectedTargets = copyStrings(commitment.SelectedTargets)
		if commitment.RollHistory != nil {
			commitment.RollHistory = make([][]RolledDie, len(commitment.RollHistory))
			for i, roll := range values[id].RollHistory {
				commitment.RollHistory[i] = copyRolledDice(roll)
			}
		}
		copied[id] = commitment
	}
	return copied
}

func cloneFlowState(flow SegmentFlowState) SegmentFlowState {
	cloned := flow
	if flow.Actors != nil {
		cloned.Actors = make(map[string]ActorFlowState, len(flow.Actors))
		for id, actor := range flow.Actors {
			cloned.Actors[id] = actor
		}
	}
	if flow.PendingInput != nil {
		cloned.PendingInput = clonePendingInputs(flow.PendingInput)
	}
	return cloned
}

func cloneContentCatalog(catalog ContentCatalog) ContentCatalog {
	cloned := ContentCatalog{
		Cards:     make(map[string]RuntimeContentDefinition, len(catalog.Cards)),
		Abilities: make(map[string]RuntimeContentDefinition, len(catalog.Abilities)),
		Statuses:  make(map[string]RuntimeStatusDefinition, len(catalog.Statuses)),
	}
	for id, definition := range catalog.Cards {
		definition.Segments = append([]segment.Segment(nil), definition.Segments...)
		definition.Operations = operation.ClonePlans(definition.Operations)
		cloned.Cards[id] = definition
	}
	for id, definition := range catalog.Abilities {
		definition.Segments = append([]segment.Segment(nil), definition.Segments...)
		definition.Operations = operation.ClonePlans(definition.Operations)
		cloned.Abilities[id] = definition
	}
	for id, definition := range catalog.Statuses {
		if definition.Triggers != nil {
			definition.Triggers = append([]RuntimeStatusTrigger(nil), definition.Triggers...)
			for i := range definition.Triggers {
				definition.Triggers[i].Operations = operation.ClonePlans(definition.Triggers[i].Operations)
			}
		}
		cloned.Statuses[id] = definition
	}
	if catalog.Cards == nil {
		cloned.Cards = nil
	}
	if catalog.Abilities == nil {
		cloned.Abilities = nil
	}
	if catalog.Statuses == nil {
		cloned.Statuses = nil
	}
	return cloned
}

func copyRolledDice(values []RolledDie) []RolledDie {
	if values == nil {
		return nil
	}
	copied := make([]RolledDie, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Symbols = copyStrings(value.Symbols)
	}
	return copied
}

func copyIntMap(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func copyDiceLoadout(values []DiceLoadoutEntry) []DiceLoadoutEntry {
	return append([]DiceLoadoutEntry(nil), values...)
}

func copyDecklist(values []DecklistEntry) []DecklistEntry {
	return append([]DecklistEntry(nil), values...)
}

func copyStatuses(values []StatusState) []StatusState {
	return append([]StatusState(nil), values...)
}

func copyTokens(values []TokenState) []TokenState {
	return append([]TokenState(nil), values...)
}

func copyDiceDefinitions(values []DiceDefinition) map[string]DiceDefinition {
	if len(values) == 0 {
		return nil
	}

	copied := make(map[string]DiceDefinition, len(values))
	for _, definition := range values {
		faces := make([]DiceFace, len(definition.Faces))
		for i, face := range definition.Faces {
			faces[i] = DiceFace{
				Face:    face.Face,
				Value:   face.Value,
				Symbols: copyStrings(face.Symbols),
			}
		}
		definition.Faces = faces
		copied[definition.ID] = definition
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
