package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"diceanddestiny/server/internal/battle/contentpin"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

const CheckpointSchemaVersion = 1

var (
	ErrBattleNotFound        = errors.New("battle not found")
	ErrBattleExists          = errors.New("battle already exists")
	ErrInvalidBattleID       = errors.New("invalid battle id")
	ErrCorruptCheckpoint     = errors.New("corrupt battle checkpoint")
	ErrUnsupportedCheckpoint = errors.New("unsupported battle checkpoint")
)

var validBattleID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Checkpoint struct {
	SchemaVersion      int            `json:"schema_version"`
	EventSchemaVersion int            `json:"event_schema_version"`
	BattleID           string         `json:"battle_id"`
	ContentPin         contentpin.Pin `json:"content_pin"`
	NextEventSequence  uint64         `json:"next_event_sequence"`
	Battle             state.Battle   `json:"battle"`
	Events             []event.Event  `json:"events"`
}

type Repository interface {
	Create(checkpoint Checkpoint) error
	Load(battleID string) (Checkpoint, error)
	Save(checkpoint Checkpoint) error
}

type Router struct {
	Normal   Repository
	Scenario Repository
}

func (router Router) Create(checkpoint Checkpoint) error {
	repo, err := router.repositoryFor(checkpoint.BattleID)
	if err != nil {
		return err
	}
	if err := validateOriginRoute(checkpoint); err != nil {
		return err
	}
	return repo.Create(checkpoint)
}

func (router Router) Load(battleID string) (Checkpoint, error) {
	repo, err := router.repositoryFor(battleID)
	if err != nil {
		return Checkpoint{}, err
	}
	checkpoint, err := repo.Load(battleID)
	if err != nil {
		return Checkpoint{}, err
	}
	if err := validateOriginRoute(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

func (router Router) Save(checkpoint Checkpoint) error {
	repo, err := router.repositoryFor(checkpoint.BattleID)
	if err != nil {
		return err
	}
	if err := validateOriginRoute(checkpoint); err != nil {
		return err
	}
	return repo.Save(checkpoint)
}

func (router Router) repositoryFor(battleID string) (Repository, error) {
	if strings.HasPrefix(battleID, "scenario-") {
		if router.Scenario == nil {
			return nil, errors.New("scenario battle repository is not configured")
		}
		return router.Scenario, nil
	}
	if router.Normal == nil {
		return nil, errors.New("normal battle repository is not configured")
	}
	return router.Normal, nil
}

func validateOriginRoute(checkpoint Checkpoint) error {
	scenarioID := strings.HasPrefix(checkpoint.BattleID, "scenario-")
	scenarioOrigin := checkpoint.Battle.Origin.Kind == state.BattleOriginScenario
	if scenarioID != scenarioOrigin {
		return fmt.Errorf("%w: battle origin does not match repository route", ErrCorruptCheckpoint)
	}
	return nil
}

func NewCheckpoint(battle state.Battle) (Checkpoint, error) {
	pin, err := contentpin.Compute(battle)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("compute content pin: %w", err)
	}
	return Checkpoint{
		SchemaVersion:      CheckpointSchemaVersion,
		EventSchemaVersion: event.SchemaVersion,
		BattleID:           battle.ID,
		ContentPin:         pin,
		NextEventSequence:  1,
		Battle:             battle,
	}, nil
}

func AppendEvents(checkpoint *Checkpoint, values []event.Event) ([]event.Event, error) {
	if checkpoint == nil {
		return nil, errors.New("checkpoint is required")
	}
	if err := validateBattleID(checkpoint.Battle.ID); err != nil {
		return nil, err
	}
	if checkpoint.BattleID == "" {
		checkpoint.BattleID = checkpoint.Battle.ID
	}
	if checkpoint.NextEventSequence == 0 {
		checkpoint.NextEventSequence = uint64(len(checkpoint.Events)) + 1
	}

	assigned := cloneEvents(values)
	for i := range assigned {
		sequence := checkpoint.NextEventSequence
		assigned[i].BattleID = checkpoint.Battle.ID
		assigned[i].SchemaVersion = event.SchemaVersion
		assigned[i].Sequence = sequence
		assigned[i].ID = eventID(checkpoint.Battle.ID, sequence)
		checkpoint.NextEventSequence++
	}
	checkpoint.Events = append(checkpoint.Events, cloneEvents(assigned)...)
	return assigned, nil
}

func ValidateCheckpoint(checkpoint Checkpoint) error {
	if checkpoint.SchemaVersion != CheckpointSchemaVersion {
		return fmt.Errorf("%w: schema version %d", ErrUnsupportedCheckpoint, checkpoint.SchemaVersion)
	}
	if checkpoint.EventSchemaVersion != event.SchemaVersion {
		return fmt.Errorf("%w: event schema version %d", ErrUnsupportedCheckpoint, checkpoint.EventSchemaVersion)
	}
	if err := validateBattleID(checkpoint.BattleID); err != nil {
		return fmt.Errorf("%w: %v", ErrCorruptCheckpoint, err)
	}
	if checkpoint.Battle.ID != checkpoint.BattleID {
		return fmt.Errorf("%w: checkpoint battle id does not match battle state", ErrCorruptCheckpoint)
	}
	if checkpoint.Battle.Origin.Kind != state.BattleOriginNormal &&
		checkpoint.Battle.Origin.Kind != state.BattleOriginScenario {
		return fmt.Errorf("%w: invalid battle origin %q", ErrCorruptCheckpoint, checkpoint.Battle.Origin.Kind)
	}
	switch checkpoint.Battle.Random.Mode {
	case state.RandomModeNormal:
		if checkpoint.Battle.Random.Algorithm != state.RandomAlgorithmCrypto {
			return fmt.Errorf("%w: invalid normal random algorithm", ErrCorruptCheckpoint)
		}
	case state.RandomModeReproducible:
		if checkpoint.Battle.Random.Algorithm != state.RandomAlgorithmSHA256 {
			return fmt.Errorf("%w: invalid reproducible random algorithm", ErrCorruptCheckpoint)
		}
	default:
		return fmt.Errorf("%w: invalid random mode %q", ErrCorruptCheckpoint, checkpoint.Battle.Random.Mode)
	}
	if checkpoint.ContentPin.SchemaVersion != contentpin.SchemaVersion ||
		checkpoint.ContentPin.CompiledContentVersion != contentpin.CompiledContentVersion ||
		checkpoint.ContentPin.Algorithm != contentpin.Algorithm {
		return fmt.Errorf(
			"%w: content pin schema=%d compiled_content=%d algorithm=%q",
			ErrUnsupportedCheckpoint,
			checkpoint.ContentPin.SchemaVersion,
			checkpoint.ContentPin.CompiledContentVersion,
			checkpoint.ContentPin.Algorithm,
		)
	}
	if err := contentpin.Validate(checkpoint.ContentPin, checkpoint.Battle); err != nil {
		return fmt.Errorf("%w: %v", ErrCorruptCheckpoint, err)
	}
	for i, battleEvent := range checkpoint.Events {
		sequence := uint64(i + 1)
		if battleEvent.BattleID != checkpoint.BattleID ||
			battleEvent.SchemaVersion != event.SchemaVersion ||
			battleEvent.Sequence != sequence ||
			battleEvent.ID != eventID(checkpoint.BattleID, sequence) {
			return fmt.Errorf("%w: invalid event metadata at sequence %d", ErrCorruptCheckpoint, sequence)
		}
	}
	wantNext := uint64(len(checkpoint.Events)) + 1
	if checkpoint.NextEventSequence != wantNext {
		return fmt.Errorf(
			"%w: next event sequence %d, want %d",
			ErrCorruptCheckpoint,
			checkpoint.NextEventSequence,
			wantNext,
		)
	}
	return nil
}

func prepareNewCheckpoint(checkpoint Checkpoint) (Checkpoint, error) {
	if checkpoint.SchemaVersion == 0 {
		initialized, err := NewCheckpoint(checkpoint.Battle)
		if err != nil {
			return Checkpoint{}, err
		}
		events := checkpoint.Events
		checkpoint = initialized
		if _, err := AppendEvents(&checkpoint, events); err != nil {
			return Checkpoint{}, err
		}
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

func validateHistoryExtension(existing, updated Checkpoint) error {
	if existing.ContentPin != updated.ContentPin {
		return fmt.Errorf("%w: content pin changed during save", ErrCorruptCheckpoint)
	}
	if len(updated.Events) < len(existing.Events) {
		return fmt.Errorf("%w: saved event history removed persisted events", ErrCorruptCheckpoint)
	}
	if !reflect.DeepEqual(updated.Events[:len(existing.Events)], existing.Events) {
		return fmt.Errorf("%w: saved event history changed persisted events", ErrCorruptCheckpoint)
	}
	return nil
}

func eventID(battleID string, sequence uint64) string {
	return fmt.Sprintf("%s:event:%020d", battleID, sequence)
}

func validateBattleID(battleID string) error {
	if !validBattleID.MatchString(battleID) || battleID == "." || battleID == ".." {
		return fmt.Errorf("%w: %q", ErrInvalidBattleID, battleID)
	}
	return nil
}

func ValidateBattleID(battleID string) error {
	return validateBattleID(battleID)
}

// CloneCheckpoint re-keys a validated checkpoint as a new independent battle.
// Gameplay state, random state, pending inputs, content pins, and event history
// are preserved. Only authority-owned battle and event identity changes.
func CloneCheckpoint(checkpoint Checkpoint, battleID string) (Checkpoint, error) {
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	if err := validateBattleID(battleID); err != nil {
		return Checkpoint{}, err
	}
	cloned := cloneCheckpoint(checkpoint)
	cloned.BattleID = battleID
	cloned.Battle.ID = battleID
	for i := range cloned.Events {
		sequence := uint64(i + 1)
		cloned.Events[i].BattleID = battleID
		cloned.Events[i].ID = eventID(battleID, sequence)
	}
	if err := ValidateCheckpoint(cloned); err != nil {
		return Checkpoint{}, err
	}
	return cloned, nil
}

type InMemory struct {
	mu          sync.RWMutex
	checkpoints map[string]Checkpoint
}

func NewInMemory() *InMemory {
	return &InMemory{checkpoints: make(map[string]Checkpoint)}
}

func (repo *InMemory) Create(checkpoint Checkpoint) error {
	prepared, err := prepareNewCheckpoint(checkpoint)
	if err != nil {
		return err
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if _, exists := repo.checkpoints[prepared.BattleID]; exists {
		return ErrBattleExists
	}
	repo.checkpoints[prepared.BattleID] = cloneCheckpoint(prepared)
	return nil
}

func (repo *InMemory) Load(battleID string) (Checkpoint, error) {
	if err := validateBattleID(battleID); err != nil {
		return Checkpoint{}, err
	}
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	checkpoint, ok := repo.checkpoints[battleID]
	if !ok {
		return Checkpoint{}, ErrBattleNotFound
	}
	return cloneCheckpoint(checkpoint), nil
}

func (repo *InMemory) Save(checkpoint Checkpoint) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	battleID := checkpoint.BattleID
	if battleID == "" {
		battleID = checkpoint.Battle.ID
	}
	existing, exists := repo.checkpoints[battleID]
	if !exists {
		return ErrBattleNotFound
	}
	prepared, err := prepareSavedCheckpoint(existing, checkpoint)
	if err != nil {
		return err
	}
	repo.checkpoints[prepared.BattleID] = cloneCheckpoint(prepared)
	return nil
}

func prepareSavedCheckpoint(existing, checkpoint Checkpoint) (Checkpoint, error) {
	if checkpoint.SchemaVersion == 0 {
		prepared, err := NewCheckpoint(checkpoint.Battle)
		if err != nil {
			return Checkpoint{}, err
		}
		if _, err := AppendEvents(&prepared, checkpoint.Events); err != nil {
			return Checkpoint{}, err
		}
		return prepared, nil
	}

	firstUnassigned := len(checkpoint.Events)
	for i, battleEvent := range checkpoint.Events {
		if battleEvent.Sequence == 0 && battleEvent.ID == "" {
			firstUnassigned = i
			break
		}
	}
	checkpoint.NextEventSequence = uint64(firstUnassigned) + 1
	unassigned := append([]event.Event(nil), checkpoint.Events[firstUnassigned:]...)
	checkpoint.Events = checkpoint.Events[:firstUnassigned]
	if _, err := AppendEvents(&checkpoint, unassigned); err != nil {
		return Checkpoint{}, err
	}
	if firstUnassigned < len(existing.Events) {
		return Checkpoint{}, fmt.Errorf("%w: saved event history removed persisted events", ErrCorruptCheckpoint)
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

type Disk struct {
	root   string
	mu     sync.Mutex
	rename func(string, string) error
}

func NewDisk(root string) *Disk {
	return &Disk{root: filepath.Clean(root), rename: os.Rename}
}

func (repo *Disk) Create(checkpoint Checkpoint) error {
	prepared, err := prepareNewCheckpoint(checkpoint)
	if err != nil {
		return err
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	path, err := repo.path(prepared.BattleID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return ErrBattleExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect battle checkpoint: %w", err)
	}
	return repo.writeAtomic(path, prepared)
}

func (repo *Disk) Load(battleID string) (Checkpoint, error) {
	path, err := repo.path(battleID)
	if err != nil {
		return Checkpoint{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, ErrBattleNotFound
	}
	if err != nil {
		return Checkpoint{}, fmt.Errorf("read battle checkpoint: %w", err)
	}

	var header struct {
		SchemaVersion      int `json:"schema_version"`
		EventSchemaVersion int `json:"event_schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Checkpoint{}, fmt.Errorf("%w: decode header: %v", ErrCorruptCheckpoint, err)
	}
	if header.SchemaVersion != CheckpointSchemaVersion ||
		header.EventSchemaVersion != event.SchemaVersion {
		return Checkpoint{}, fmt.Errorf(
			"%w: schema versions checkpoint=%d event=%d",
			ErrUnsupportedCheckpoint,
			header.SchemaVersion,
			header.EventSchemaVersion,
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var checkpoint Checkpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return Checkpoint{}, fmt.Errorf("%w: decode checkpoint: %v", ErrCorruptCheckpoint, err)
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return cloneCheckpoint(checkpoint), nil
}

func (repo *Disk) Save(checkpoint Checkpoint) error {
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return err
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	path, err := repo.path(checkpoint.BattleID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return ErrBattleNotFound
	} else if err != nil {
		return fmt.Errorf("inspect battle checkpoint: %w", err)
	}
	existing, err := repo.Load(checkpoint.BattleID)
	if err != nil {
		return err
	}
	if err := validateHistoryExtension(existing, checkpoint); err != nil {
		return err
	}
	return repo.writeAtomic(path, checkpoint)
}

func (repo *Disk) path(battleID string) (string, error) {
	if repo == nil || repo.root == "" || repo.root == "." {
		return "", errors.New("disk repository root is required")
	}
	if err := validateBattleID(battleID); err != nil {
		return "", err
	}
	return filepath.Join(repo.root, battleID+".json"), nil
}

func (repo *Disk) writeAtomic(path string, checkpoint Checkpoint) error {
	if err := os.MkdirAll(repo.root, 0o700); err != nil {
		return fmt.Errorf("create battle checkpoint directory: %w", err)
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode battle checkpoint: %w", err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(repo.root, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary battle checkpoint: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary battle checkpoint: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary battle checkpoint: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary battle checkpoint: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary battle checkpoint: %w", err)
	}
	if err := repo.rename(tempPath, path); err != nil {
		return fmt.Errorf("replace battle checkpoint: %w", err)
	}
	return nil
}

func cloneCheckpoint(checkpoint Checkpoint) Checkpoint {
	return Checkpoint{
		SchemaVersion:      checkpoint.SchemaVersion,
		EventSchemaVersion: checkpoint.EventSchemaVersion,
		BattleID:           checkpoint.BattleID,
		ContentPin:         checkpoint.ContentPin,
		NextEventSequence:  checkpoint.NextEventSequence,
		Battle:             checkpoint.Battle.Clone(),
		Events:             cloneEvents(checkpoint.Events),
	}
}

func cloneEvents(events []event.Event) []event.Event {
	if events == nil {
		return nil
	}
	cloned := make([]event.Event, len(events))
	for i, source := range events {
		cloned[i] = source
		cloned[i].Cards = append([]string(nil), source.Cards...)
		cloned[i].Dice = cloneRolledDice(source.Dice)
		cloned[i].RolledIndices = append([]int(nil), source.RolledIndices...)
		cloned[i].RollsRemaining = cloneInt(source.RollsRemaining)
		cloned[i].Combinations = append([]string(nil), source.Combinations...)
		cloned[i].SymbolCounts = cloneIntMap(source.SymbolCounts)
		cloned[i].Commitment = cloneInteractionCommitment(source.Commitment)
		cloned[i].Commitments = cloneInteractionCommitments(source.Commitments)
		cloned[i].ProposalBatch = cloneProposalBatch(source.ProposalBatch)
		cloned[i].DamageCards = cloneDamageCards(source.DamageCards)
	}
	return cloned
}

func cloneDamageCards(values []state.ProposedCardRemoval) []state.ProposedCardRemoval {
	if values == nil {
		return nil
	}
	cloned := make([]state.ProposedCardRemoval, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].DamageProposalIDs = append([]string(nil), value.DamageProposalIDs...)
		cloned[i].SourceActorIDs = append([]string(nil), value.SourceActorIDs...)
	}
	return cloned
}

func cloneInteractionCommitment(value *state.InteractionCommitment) *state.InteractionCommitment {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Data.ProposalIDs = append([]string(nil), value.Data.ProposalIDs...)
	cloned.Data.CardIDs = append([]string(nil), value.Data.CardIDs...)
	cloned.Data.TargetIDs = append([]string(nil), value.Data.TargetIDs...)
	cloned.Data.Value = cloneInt(value.Data.Value)
	if value.Data.Planning != nil {
		planning := clonePlanningCommitment(*value.Data.Planning)
		cloned.Data.Planning = &planning
	}
	cloned.Data.PlanningAdjustments = append([]state.PlanningAdjustment(nil), value.Data.PlanningAdjustments...)
	cloned.Data.DamageReactions = append([]state.DamageReaction(nil), value.Data.DamageReactions...)
	return &cloned
}

func cloneInteractionCommitments(values []state.InteractionCommitment) []state.InteractionCommitment {
	if values == nil {
		return nil
	}
	cloned := make([]state.InteractionCommitment, len(values))
	for i := range values {
		cloned[i] = *cloneInteractionCommitment(&values[i])
	}
	return cloned
}

func cloneProposalBatch(value *state.ProposalBatch) *state.ProposalBatch {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Proposals = make([]state.Proposal, len(value.Proposals))
	for i, proposal := range value.Proposals {
		cloned.Proposals[i] = proposal
		if proposal.Data.Amount != nil {
			amount := *proposal.Data.Amount
			cloned.Proposals[i].Data.Amount = &amount
		}
		if proposal.Data.Selection != nil {
			selection := *proposal.Data.Selection
			selection.OptionIDs = append([]string(nil), proposal.Data.Selection.OptionIDs...)
			cloned.Proposals[i].Data.Selection = &selection
		}
		if proposal.Data.Roll != nil {
			roll := *proposal.Data.Roll
			roll.Dice = cloneRolledDice(proposal.Data.Roll.Dice)
			cloned.Proposals[i].Data.Roll = &roll
		}
		if proposal.Data.Planning != nil {
			planning := clonePlanningCommitment(*proposal.Data.Planning)
			cloned.Proposals[i].Data.Planning = &planning
		}
	}
	return &cloned
}

func clonePlanningCommitment(value state.PlanningCommitmentData) state.PlanningCommitmentData {
	value.FinalDice = cloneRolledDice(value.FinalDice)
	value.KeptIndices = append([]int(nil), value.KeptIndices...)
	value.CommittedCards = append([]string(nil), value.CommittedCards...)
	value.SelectedTargets = append([]string(nil), value.SelectedTargets...)
	return value
}

func cloneRolledDice(values []state.RolledDie) []state.RolledDie {
	if values == nil {
		return nil
	}
	cloned := make([]state.RolledDie, len(values))
	for i, die := range values {
		cloned[i] = die
		cloned[i].Symbols = append([]string(nil), die.Symbols...)
	}
	return cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntMap(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	cloned := make(map[string]int, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
