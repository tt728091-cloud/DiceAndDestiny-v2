package snapshot

import (
	"encoding/json"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Battle is the read-only view returned after events have been applied.
// It is safe for presentation or future network clients to render from.
type Battle struct {
	BattleID           string                          `json:"battle_id"`
	Status             state.BattleStatus              `json:"status,omitempty"`
	Segment            segment.Segment                 `json:"segment"`
	Round              int                             `json:"round"`
	ViewerActorID      string                          `json:"viewer_actor_id,omitempty"`
	Flow               *SegmentFlow                    `json:"flow,omitempty"`
	Resolution         *Resolution                     `json:"resolution,omitempty"`
	Damage             *DamageResolution               `json:"damage,omitempty"`
	Actors             map[string]Actor                `json:"actors,omitempty"`
	OffensiveProposals []state.PlanningProposal        `json:"offensive_proposals,omitempty"`
	DefensiveProposals []state.PlanningProposal        `json:"defensive_proposals,omitempty"`
	Origin             *state.BattleOrigin             `json:"origin,omitempty"`
	CompletedRounds    int                             `json:"completed_rounds,omitempty"`
	Stage              string                          `json:"stage,omitempty"`
	SettledSources     []state.SettledDamageSource     `json:"damage_sources,omitempty"`
	SettledDefenses    map[string]state.SettledDefense `json:"defense_selections,omitempty"`
	SettledDamage      *state.SettledDamageBatch       `json:"settled_damage,omitempty"`
	ContentCatalog     *ContentCatalog                 `json:"content_catalog,omitempty"`
}

// ContentCatalog publishes the same pinned immutable definitions used by the
// authority. It intentionally omits combatant AI policy and other hidden setup
// data while giving clients enough information to render and submit generic
// content without ID-specific code.
type ContentCatalog struct {
	Symbols   map[string]content.SymbolDefinition        `json:"symbols"`
	Dice      map[string]content.BattleDieDefinition     `json:"dice"`
	Cards     map[string]content.BattleCardDefinition    `json:"cards"`
	Abilities map[string]content.BattleAbilityDefinition `json:"abilities"`
	Statuses  map[string]content.BattleStatusDefinition  `json:"statuses"`
}

type Actor struct {
	DefinitionID       string                         `json:"definition_id,omitempty"`
	Controller         state.ControllerType           `json:"controller,omitempty"`
	Character          *CharacterMetadata             `json:"character,omitempty"`
	EnergyPoints       int                            `json:"energy_points"`
	MaxEnergyPoints    int                            `json:"max_energy_points,omitempty"`
	MaxHandSize        int                            `json:"max_hand_size,omitempty"`
	MaxHealth          int                            `json:"max_health,omitempty"`
	CurrentHealth      int                            `json:"current_health,omitempty"`
	HealthCardCount    *int                           `json:"health_card_count,omitempty"`
	Decklist           []state.DecklistEntry          `json:"decklist,omitempty"`
	Hand               []string                       `json:"hand,omitempty"`
	HandCount          int                            `json:"hand_count"`
	DeckCount          int                            `json:"deck_count"`
	DiscardCount       int                            `json:"discard_count"`
	RemovedCount       int                            `json:"removed_count"`
	DiceLoadout        []state.DiceLoadoutEntry       `json:"dice_loadout,omitempty"`
	DiceCount          int                            `json:"dice_count,omitempty"`
	AbilityIDs         []string                       `json:"abilities,omitempty"`
	AbilityCount       int                            `json:"ability_count,omitempty"`
	Statuses           []state.StatusState            `json:"statuses,omitempty"`
	Tokens             []state.TokenState             `json:"tokens,omitempty"`
	RollPreferences    *state.RollPreferences         `json:"roll_preferences,omitempty"`
	Dice               *DiceRollState                 `json:"dice,omitempty"`
	DefeatState        state.ActorDefeatState         `json:"defeat_state,omitempty"`
	CardInstances      map[string]state.CardInstance  `json:"card_instances,omitempty"`
	OffensiveAbilities []string                       `json:"offensive_abilities,omitempty"`
	DefensiveAbilities []string                       `json:"defensive_abilities,omitempty"`
	RollHistory        []state.RollBatch              `json:"roll_history,omitempty"`
	QualifiedAbilities []string                       `json:"qualified_abilities,omitempty"`
	SelectedAbility    string                         `json:"selected_ability,omitempty"`
	SelectedTier       string                         `json:"selected_tier,omitempty"`
	SelectedTargets    []string                       `json:"selected_targets,omitempty"`
	OffensiveOutcome   map[string]any                 `json:"offensive_outcome,omitempty"`
	AbilityModifiers   []state.RuntimeAbilityModifier `json:"ability_modifiers,omitempty"`
}

type CharacterMetadata struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type SegmentFlow struct {
	Segment      segment.Segment          `json:"segment"`
	Round        int                      `json:"round"`
	Entered      bool                     `json:"entered"`
	ExitStarted  bool                     `json:"exit_started,omitempty"`
	Stage        string                   `json:"stage,omitempty"`
	Iteration    int                      `json:"iteration"`
	Actors       map[string]ActorProgress `json:"actors,omitempty"`
	PendingInput map[string]PendingInput  `json:"pending_input,omitempty"`
}

type ActorProgress struct {
	Status     state.ActorProgressStatus `json:"status"`
	ReasonCode string                    `json:"reason_code,omitempty"`
}

type PendingInput struct {
	ID              string          `json:"id"`
	ActorID         string          `json:"actor_id"`
	Segment         segment.Segment `json:"segment"`
	Phase           state.FlowPhase `json:"phase,omitempty"`
	Stage           string          `json:"stage"`
	Iteration       int             `json:"iteration"`
	WindowID        string          `json:"window_id,omitempty"`
	ReactionRound   int             `json:"reaction_round,omitempty"`
	PlanningCycle   int             `json:"planning_cycle,omitempty"`
	InputType       string          `json:"input_type"`
	SourceType      string          `json:"source_type,omitempty"`
	SourceID        string          `json:"source_id,omitempty"`
	AllowedCommands []string        `json:"allowed_commands"`
}

type Resolution struct {
	ID           string                     `json:"id"`
	Origin       state.ResolutionCheckpoint `json:"origin"`
	Stage        state.ResolutionStage      `json:"stage"`
	Batch        *state.ProposalBatch       `json:"batch,omitempty"`
	ActiveWindow *InteractionWindow         `json:"active_window,omitempty"`
	Planning     *Planning                  `json:"planning,omitempty"`
}

type DamageResolution struct {
	ID                  string                            `json:"id"`
	Stage               state.DamageResolutionStage       `json:"stage"`
	Revision            int                               `json:"revision"`
	PendingTotals       []state.AccumulatedDamageProposal `json:"pending_totals,omitempty"`
	RevealedCards       []state.ProposedCardRemoval       `json:"revealed_cards,omitempty"`
	ActiveInteractionID string                            `json:"active_interaction_window_id,omitempty"`
}

type Planning struct {
	Segment segment.Segment          `json:"segment"`
	Cycle   int                      `json:"cycle"`
	Actors  map[string]PlanningActor `json:"actors"`
}

type PlanningActor struct {
	Participation     state.ActorProgressStatus     `json:"participation"`
	ReasonCode        string                        `json:"reason_code,omitempty"`
	LockedIn          bool                          `json:"locked_in"`
	Current           *state.PlanningCommitmentData `json:"current,omitempty"`
	Revealed          *state.PlanningCommitmentData `json:"revealed,omitempty"`
	EligibleTargetIDs []string                      `json:"eligible_target_ids,omitempty"`
}

type InteractionWindow struct {
	ID                  string                                  `json:"id"`
	Opened              bool                                    `json:"opened"`
	Purpose             state.InteractionPurpose                `json:"purpose"`
	Source              state.SourceReference                   `json:"source"`
	EligibleActors      []string                                `json:"eligible_actors"`
	RequiredActors      []string                                `json:"required_actors"`
	ActorProgress       map[string]state.InteractionActorStatus `json:"actor_progress"`
	AllowedCommands     []string                                `json:"allowed_commands"`
	HiddenCommitments   bool                                    `json:"hidden_commitments"`
	RevealStatus        state.RevealStatus                      `json:"reveal_status"`
	PassAllowed         bool                                    `json:"pass_allowed"`
	Commitments         map[string]state.InteractionCommitment  `json:"commitments,omitempty"`
	ReactionRound       int                                     `json:"reaction_round"`
	ChainDepth          int                                     `json:"chain_depth"`
	SuspendedCheckpoint state.ResolutionCheckpoint              `json:"suspended_checkpoint"`
}

type DiceRollState struct {
	RequestID      string               `json:"request_id"`
	Segment        segment.Segment      `json:"segment"`
	Pool           state.RollPool       `json:"pool"`
	SourceType     state.RollSourceType `json:"source_type"`
	SourceID       string               `json:"source_id"`
	Dice           []state.RolledDie    `json:"dice"`
	KeptIndices    []int                `json:"kept_indices,omitempty"`
	RollsUsed      int                  `json:"rolls_used"`
	MaxRolls       int                  `json:"max_rolls"`
	RollsRemaining int                  `json:"rolls_remaining"`
	Combinations   []string             `json:"combinations,omitempty"`
	SymbolCounts   map[string]int       `json:"symbol_counts,omitempty"`
	Complete       bool                 `json:"complete,omitempty"`
}

// FromBattle copies authoritative battle state into the public snapshot shape.
// Do not expose mutable state structs directly across the authority boundary.
func FromBattle(battle state.Battle) Battle {
	return FromBattleForViewer(battle, "")
}

// FromBattleForViewer copies authoritative battle state into a viewer-safe
// snapshot. Only the viewer actor receives hidden hand card IDs; all other
// actors expose counts only.
func FromBattleForViewer(battle state.Battle, viewerActorID string) Battle {
	actors := make(map[string]Actor, len(battle.Actors))
	for id, actor := range battle.Actors {
		cards := actor.Cards
		energyPoints := actor.Resources.EnergyPoints
		if energyPoints == 0 && actor.EnergyPoints != 0 {
			energyPoints = actor.EnergyPoints
		}
		snapshotActor := Actor{
			DefinitionID:    actor.DefinitionID,
			Controller:      actor.Controller,
			Character:       characterSnapshot(actor.Character),
			EnergyPoints:    energyPoints,
			MaxEnergyPoints: actor.Resources.MaxEnergyPoints,
			MaxHandSize:     actor.Resources.MaxHandSize,
			MaxHealth:       actor.Health.MaxHealth,
			HandCount:       len(cards.Hand),
			DeckCount:       len(cards.Deck),
			DiscardCount:    len(cards.Discard),
			RemovedCount:    len(cards.Removed),
			DiceCount:       diceCount(actor.DiceLoadout),
			AbilityCount:    len(actor.AbilityIDs),
			Statuses:        copyStatuses(actor.Statuses),
			Tokens:          copyTokens(actor.Tokens),
			DefeatState:     actor.DefeatState,
		}
		if actor.Health.Model != "" || actor.Health.MaxHealth != 0 {
			currentHealth := actor.CurrentHealth()
			snapshotActor.CurrentHealth = currentHealth
			snapshotActor.HealthCardCount = &currentHealth
		}
		if actor.Dice.CurrentRoll != nil && diceVisibleToViewer(battle, id, viewerActorID) {
			snapshotActor.Dice = diceRollStateSnapshot(actor.Dice.CurrentRoll)
		}
		if id == viewerActorID {
			snapshotActor.Hand = append([]string(nil), cards.Hand...)
			snapshotActor.Decklist = copyDecklist(actor.Decklist)
			snapshotActor.DiceLoadout = copyDiceLoadout(actor.DiceLoadout)
			snapshotActor.AbilityIDs = copyStrings(actor.AbilityIDs)
			if actor.RollPreferences != (state.RollPreferences{}) {
				preferences := actor.RollPreferences
				snapshotActor.RollPreferences = &preferences
			}
		}
		if battle.Settled != nil {
			runtime := battle.Settled.Actors[id]
			snapshotActor.OffensiveAbilities = copyStrings(runtime.OffensiveAbilityIDs)
			snapshotActor.DefensiveAbilities = copyStrings(runtime.DefensiveAbilityIDs)
			snapshotActor.AbilityModifiers = append([]state.RuntimeAbilityModifier(nil), runtime.AbilityModifiers...)
			if id == viewerActorID {
				snapshotActor.CardInstances = make(map[string]state.CardInstance, len(runtime.CardInstances))
				for instanceID, instance := range runtime.CardInstances {
					snapshotActor.CardInstances[instanceID] = instance
				}
				snapshotActor.RollHistory = append([]state.RollBatch(nil), runtime.RollHistory...)
				snapshotActor.QualifiedAbilities = copyStrings(runtime.QualifiedAbilityIDs)
				snapshotActor.SelectedAbility = runtime.SelectedAbilityID
				snapshotActor.SelectedTargets = copyStrings(runtime.SelectedTargetIDs)
			} else if battle.Settled.Stage != "planning" {
				snapshotActor.RollHistory = append([]state.RollBatch(nil), runtime.RollHistory...)
				snapshotActor.SelectedAbility = runtime.SelectedAbilityID
				snapshotActor.SelectedTargets = copyStrings(runtime.SelectedTargetIDs)
			}
		}
		actors[id] = snapshotActor
	}
	if len(actors) == 0 {
		actors = nil
	}

	result := Battle{
		BattleID:           battle.ID,
		Status:             battle.Status,
		Segment:            battle.Segment.Current,
		Round:              battle.Segment.Round,
		ViewerActorID:      viewerActorID,
		Flow:               flowSnapshot(battle, viewerActorID),
		Resolution:         resolutionSnapshot(battle, viewerActorID),
		Damage:             damageSnapshot(battle),
		Actors:             actors,
		ContentCatalog:     settledContentCatalog(battle),
		OffensiveProposals: planningProposalsForViewer(battle.OffensiveProposals, viewerActorID),
		DefensiveProposals: planningProposalsForViewer(battle.DefensiveProposals, viewerActorID),
		Origin:             originSnapshot(battle.Origin),
	}
	if battle.Settled != nil {
		result.CompletedRounds = battle.Settled.CompletedRounds
		result.Stage = battle.Settled.Stage
		result.SettledSources = append([]state.SettledDamageSource(nil), battle.Settled.OffensiveSources...)
		if battle.Segment.Current == segment.Defensive && battle.Settled.Stage == "defense_reaction" {
			result.SettledDefenses = make(map[string]state.SettledDefense, len(battle.Settled.DefenseSelections))
			for actorID, defense := range battle.Settled.DefenseSelections {
				result.SettledDefenses[actorID] = defense
			}
		}
		if battle.Settled.PendingDamage != nil {
			damage := *battle.Settled.PendingDamage
			damage.Sources = append([]state.SettledDamageSource(nil), battle.Settled.PendingDamage.Sources...)
			damage.Removals = append([]state.ProposedCardRemoval(nil), battle.Settled.PendingDamage.Removals...)
			result.SettledDamage = &damage
		}
	}
	return result
}

func settledContentCatalog(battle state.Battle) *ContentCatalog {
	if battle.Settled == nil || len(battle.SettledCatalog) == 0 {
		return nil
	}
	var library content.BattleLibrary
	if err := json.Unmarshal(battle.SettledCatalog, &library); err != nil {
		return nil
	}
	return &ContentCatalog{Symbols: library.Symbols, Dice: library.Dice, Cards: library.Cards, Abilities: library.Abilities, Statuses: library.Statuses}
}

func originSnapshot(origin state.BattleOrigin) *state.BattleOrigin {
	if origin.Kind != state.BattleOriginScenario {
		return nil
	}
	copied := origin
	return &copied
}

func damageSnapshot(battle state.Battle) *DamageResolution {
	if battle.DamageResolution == nil {
		return nil
	}
	damage := battle.DamageResolution
	snapshot := &DamageResolution{
		ID:       damage.ID,
		Stage:    damage.Stage,
		Revision: damage.Revision,
	}
	snapshot.PendingTotals = append(
		[]state.AccumulatedDamageProposal(nil),
		damage.AccumulatedProposals...,
	)
	for i := range snapshot.PendingTotals {
		snapshot.PendingTotals[i].SourceProposalIDs = copyStrings(
			damage.AccumulatedProposals[i].SourceProposalIDs,
		)
	}
	for _, card := range damage.CardProposals {
		if !card.Accepted || !card.Revealed {
			continue
		}
		card.DamageProposalIDs = copyStrings(card.DamageProposalIDs)
		card.SourceActorIDs = copyStrings(card.SourceActorIDs)
		snapshot.RevealedCards = append(snapshot.RevealedCards, card)
	}
	if battle.ActiveResolutionID == damage.ReactionResolutionID {
		if resolution, ok := battle.Resolutions[battle.ActiveResolutionID]; ok {
			snapshot.ActiveInteractionID = resolution.ActiveWindowID
		}
	}
	return snapshot
}

func characterSnapshot(value state.CharacterMetadata) *CharacterMetadata {
	if value == (state.CharacterMetadata{}) {
		return nil
	}
	return &CharacterMetadata{ID: value.ID, Name: value.Name, Class: value.Class}
}

func diceCount(loadout []state.DiceLoadoutEntry) int {
	total := 0
	for _, entry := range loadout {
		total += entry.Count
	}
	return total
}

func copyDecklist(values []state.DecklistEntry) []state.DecklistEntry {
	return append([]state.DecklistEntry(nil), values...)
}

func copyDiceLoadout(values []state.DiceLoadoutEntry) []state.DiceLoadoutEntry {
	return append([]state.DiceLoadoutEntry(nil), values...)
}

func copyStatuses(values []state.StatusState) []state.StatusState {
	return append([]state.StatusState(nil), values...)
}

func copyTokens(values []state.TokenState) []state.TokenState {
	return append([]state.TokenState(nil), values...)
}

func PendingInputForViewer(battle state.Battle, viewerActorID string) map[string]PendingInput {
	pending, ok := battle.Flow.PendingInput[viewerActorID]
	if !ok {
		return nil
	}
	return map[string]PendingInput{
		viewerActorID: pendingInputSnapshot(pending),
	}
}

func flowSnapshot(battle state.Battle, viewerActorID string) *SegmentFlow {
	if battle.Flow.Segment == "" {
		return nil
	}
	actors := make(map[string]ActorProgress, len(battle.Flow.Actors))
	for actorID, actor := range battle.Flow.Actors {
		actors[actorID] = ActorProgress{
			Status:     actor.Status,
			ReasonCode: actor.ReasonCode,
		}
	}
	if len(actors) == 0 {
		actors = nil
	}
	return &SegmentFlow{
		Segment:      battle.Flow.Segment,
		Round:        battle.Flow.Round,
		Entered:      battle.Flow.Entered,
		ExitStarted:  battle.Flow.ExitStarted,
		Stage:        battle.Flow.Stage,
		Iteration:    battle.Flow.Iteration,
		Actors:       actors,
		PendingInput: PendingInputForViewer(battle, viewerActorID),
	}
}

func pendingInputSnapshot(pending state.PendingInput) PendingInput {
	allowed := make([]string, len(pending.AllowedCommands))
	for i, commandType := range pending.AllowedCommands {
		allowed[i] = string(commandType)
	}
	return PendingInput{
		ID:              pending.ID,
		ActorID:         pending.ActorID,
		Segment:         pending.Segment,
		Phase:           pending.Phase,
		Stage:           pending.Stage,
		Iteration:       pending.Iteration,
		WindowID:        pending.WindowID,
		ReactionRound:   pending.ReactionRound,
		PlanningCycle:   pending.PlanningCycle,
		InputType:       pending.InputType,
		SourceType:      pending.SourceType,
		SourceID:        pending.SourceID,
		AllowedCommands: allowed,
	}
}

func resolutionSnapshot(battle state.Battle, viewerActorID string) *Resolution {
	if battle.ActiveResolutionID == "" {
		return nil
	}
	resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
	if !ok {
		return nil
	}
	snapshot := &Resolution{
		ID:     resolution.ID,
		Origin: resolution.Origin,
		Stage:  resolution.Stage,
	}
	if resolution.Batch.Revealed {
		batch := copyProposalBatch(resolution.Batch)
		snapshot.Batch = &batch
	}
	if window, ok := resolution.Windows[resolution.ActiveWindowID]; ok {
		snapshot.ActiveWindow = interactionWindowSnapshot(window, viewerActorID)
	}
	if resolution.Planning != nil {
		snapshot.Planning = planningSnapshot(*resolution.Planning, viewerActorID)
	}
	return snapshot
}

func planningSnapshot(planning state.PlanningState, viewerActorID string) *Planning {
	actors := make(map[string]PlanningActor, len(planning.Actors))
	for actorID, actor := range planning.Actors {
		view := PlanningActor{
			Participation: actor.Participation,
			ReasonCode:    actor.ReasonCode,
			LockedIn:      actor.LockedIn,
		}
		if actor.RevealedCommitment != nil {
			revealed := copyPlanningCommitment(*actor.RevealedCommitment)
			if actorID != viewerActorID {
				revealed.KeptIndices = nil
			}
			view.Revealed = &revealed
		}
		if actorID == viewerActorID {
			current := state.PlanningCommitmentData{
				Segment:         planning.Segment,
				Cycle:           planning.Cycle,
				FinalDice:       copyRolledDice(actor.FinalDice),
				KeptIndices:     append([]int(nil), actor.KeptIndices...),
				RollsUsed:       actor.RollsUsed,
				MaxRolls:        actor.MaxRolls,
				SelectedAbility: actor.SelectedAbility,
				CommittedCards:  copyStrings(actor.CommittedCards),
				SelectedTargets: copyStrings(actor.SelectedTargets),
				Passed:          actor.Passed,
				LockedIn:        actor.LockedIn,
			}
			view.Current = &current
			view.EligibleTargetIDs = copyStrings(actor.EligibleTargetIDs)
		}
		actors[actorID] = view
	}
	return &Planning{Segment: planning.Segment, Cycle: planning.Cycle, Actors: actors}
}

func interactionWindowSnapshot(
	window state.InteractionWindow,
	viewerActorID string,
) *InteractionWindow {
	allowed := make([]string, len(window.AllowedCommands))
	for i, commandType := range window.AllowedCommands {
		allowed[i] = string(commandType)
	}
	progress := make(map[string]state.InteractionActorStatus, len(window.ActorProgress))
	for actorID, status := range window.ActorProgress {
		progress[actorID] = status
	}
	commitments := make(map[string]state.InteractionCommitment)
	for actorID, commitment := range window.Commitments {
		if window.HiddenCommitments &&
			window.RevealStatus != state.RevealStatusRevealed &&
			actorID != viewerActorID {
			continue
		}
		commitments[actorID] = copyInteractionCommitment(commitment)
	}
	if len(commitments) == 0 {
		commitments = nil
	}
	return &InteractionWindow{
		ID:                  window.ID,
		Opened:              window.Opened,
		Purpose:             window.Purpose,
		Source:              window.Source,
		EligibleActors:      copyStrings(window.EligibleActors),
		RequiredActors:      copyStrings(window.RequiredActors),
		ActorProgress:       progress,
		AllowedCommands:     allowed,
		HiddenCommitments:   window.HiddenCommitments,
		RevealStatus:        window.RevealStatus,
		PassAllowed:         window.PassAllowed,
		Commitments:         commitments,
		ReactionRound:       window.ReactionRound,
		ChainDepth:          window.ChainDepth,
		SuspendedCheckpoint: window.SuspendedCheckpoint,
	}
}

func copyInteractionCommitment(
	commitment state.InteractionCommitment,
) state.InteractionCommitment {
	copied := commitment
	copied.Data.ProposalIDs = copyStrings(commitment.Data.ProposalIDs)
	copied.Data.CardIDs = copyStrings(commitment.Data.CardIDs)
	copied.Data.TargetIDs = copyStrings(commitment.Data.TargetIDs)
	if commitment.Data.Value != nil {
		value := *commitment.Data.Value
		copied.Data.Value = &value
	}
	if commitment.Data.Planning != nil {
		planning := copyPlanningCommitment(*commitment.Data.Planning)
		copied.Data.Planning = &planning
	}
	copied.Data.PlanningAdjustments = append(
		[]state.PlanningAdjustment(nil),
		commitment.Data.PlanningAdjustments...,
	)
	copied.Data.DamageReactions = append(
		[]state.DamageReaction(nil),
		commitment.Data.DamageReactions...,
	)
	return copied
}

func copyProposalBatch(batch state.ProposalBatch) state.ProposalBatch {
	copied := batch
	copied.Proposals = make([]state.Proposal, len(batch.Proposals))
	for i, proposal := range batch.Proposals {
		copied.Proposals[i] = proposal
		if proposal.Data.Amount != nil {
			amount := *proposal.Data.Amount
			copied.Proposals[i].Data.Amount = &amount
		}
		if proposal.Data.Selection != nil {
			selection := *proposal.Data.Selection
			selection.OptionIDs = copyStrings(proposal.Data.Selection.OptionIDs)
			copied.Proposals[i].Data.Selection = &selection
		}
		if proposal.Data.Roll != nil {
			roll := *proposal.Data.Roll
			roll.Dice = copyRolledDice(proposal.Data.Roll.Dice)
			copied.Proposals[i].Data.Roll = &roll
		}
		if proposal.Data.Planning != nil {
			planning := copyPlanningCommitment(*proposal.Data.Planning)
			copied.Proposals[i].Data.Planning = &planning
		}
	}
	return copied
}

func copyPlanningCommitment(value state.PlanningCommitmentData) state.PlanningCommitmentData {
	value.FinalDice = copyRolledDice(value.FinalDice)
	value.KeptIndices = append([]int(nil), value.KeptIndices...)
	value.CommittedCards = copyStrings(value.CommittedCards)
	value.SelectedTargets = copyStrings(value.SelectedTargets)
	return value
}

func planningProposalsForViewer(
	values []state.PlanningProposal,
	viewerActorID string,
) []state.PlanningProposal {
	if values == nil {
		return nil
	}
	copied := make([]state.PlanningProposal, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Commitment = copyPlanningCommitment(value.Commitment)
		if value.ActorID != viewerActorID {
			copied[i].Commitment.KeptIndices = nil
		}
	}
	return copied
}

func diceVisibleToViewer(battle state.Battle, actorID string, viewerActorID string) bool {
	if actorID == viewerActorID {
		return true
	}
	return battle.Flow.Segment != segment.Offensive && battle.Flow.Segment != segment.Defensive
}

func diceRollStateSnapshot(roll *state.RollState) *DiceRollState {
	if roll == nil {
		return nil
	}

	return &DiceRollState{
		RequestID:      roll.RequestID,
		Segment:        roll.Segment,
		Pool:           roll.Pool,
		SourceType:     roll.SourceType,
		SourceID:       roll.SourceID,
		Dice:           copyRolledDice(roll.Dice),
		KeptIndices:    append([]int(nil), roll.KeptIndices...),
		RollsUsed:      roll.RollsUsed,
		MaxRolls:       roll.MaxRolls,
		RollsRemaining: roll.MaxRolls - roll.RollsUsed,
		Combinations:   append([]string(nil), roll.Combinations...),
		SymbolCounts:   copySymbolCounts(roll.SymbolCounts),
		Complete:       roll.Complete,
	}
}

func copyRolledDice(values []state.RolledDie) []state.RolledDie {
	copied := make([]state.RolledDie, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Symbols = copyStrings(value.Symbols)
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func copySymbolCounts(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
