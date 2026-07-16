package devhistory

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle/repository"
)

const SchemaVersion = 1

const (
	BranchActive     = "active"
	BranchReview     = "review"
	BranchReplay     = "replay"
	BranchArchived   = "archived"
	KindDecision     = "decision"
	KindPresentation = "presentation"
)

var (
	ErrNotFound      = errors.New("developer history was not found")
	ErrInvalidPoint  = errors.New("invalid developer history point")
	ErrInvalidBranch = errors.New("invalid developer history branch")
	ErrCorrupt       = errors.New("corrupt developer history")
	validPointID     = regexp.MustCompile(`^history-[a-f0-9]{16}$`)
	storeMu          sync.Mutex
)

type PointMetadata struct {
	ID                string    `json:"id"`
	ParentPointID     string    `json:"parent_point_id,omitempty"`
	BattleID          string    `json:"battle_id"`
	ActorID           string    `json:"actor_id"`
	Label             string    `json:"label"`
	Kind              string    `json:"kind"`
	CreatedAt         time.Time `json:"created_at"`
	Round             int       `json:"round"`
	Segment           string    `json:"segment"`
	Stage             string    `json:"stage,omitempty"`
	EventCount        int       `json:"event_count"`
	PresentedSequence uint64    `json:"presented_sequence"`
	ActionKey         string    `json:"action_key,omitempty"`
	ActionType        string    `json:"action_type,omitempty"`
	ActionLabel       string    `json:"action_label,omitempty"`
}

type Point struct {
	SchemaVersion int                   `json:"schema_version"`
	Metadata      PointMetadata         `json:"metadata"`
	ClientState   map[string]any        `json:"client_state,omitempty"`
	Checkpoint    repository.Checkpoint `json:"checkpoint"`
}

type Branch struct {
	SchemaVersion        int       `json:"schema_version"`
	BattleID             string    `json:"battle_id"`
	RootBattleID         string    `json:"root_battle_id"`
	HeadPointID          string    `json:"head_point_id,omitempty"`
	CursorPointID        string    `json:"cursor_point_id,omitempty"`
	ParentBattleID       string    `json:"parent_battle_id,omitempty"`
	LatestBattleID       string    `json:"latest_battle_id,omitempty"`
	BasePointID          string    `json:"base_point_id,omitempty"`
	DiscardedHeadPointID string    `json:"discarded_head_point_id,omitempty"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
}

type Timeline struct {
	Branch Branch          `json:"branch"`
	Points []PointMetadata `json:"points"`
}

// Bundle is the self-contained, portable portion of a developer history
// branch. Point checkpoints and client state are included so a snapshot can
// restore a fully jumpable timeline without depending on the source store.
type Bundle struct {
	SchemaVersion int     `json:"schema_version"`
	Branch        Branch  `json:"branch"`
	Points        []Point `json:"points"`
}

type ImportResult struct {
	Branch  Branch
	Current Point
}

type ReplayStep struct {
	Branch      Branch
	Current     Point
	Next        *Point
	Matches     bool
	FutureCount int
}

type Store struct {
	Root        string
	Now         func() time.Time
	IDGenerator func() (string, error)
}

func (store Store) Mark(checkpoint repository.Checkpoint, actorID, label, kind string, presentedSequence uint64, clientState, action map[string]any) (PointMetadata, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := repository.ValidateCheckpoint(checkpoint); err != nil {
		return PointMetadata{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if _, ok := checkpoint.Battle.Actors[actorID]; !ok {
		return PointMetadata{}, errors.New("history actor is not a battle participant")
	}
	if kind != KindDecision && kind != KindPresentation {
		return PointMetadata{}, errors.New("history kind must be decision or presentation")
	}
	if label == "" || len(label) > 160 {
		return PointMetadata{}, errors.New("history label must contain 1 to 160 characters")
	}
	if presentedSequence > uint64(len(checkpoint.Events)) {
		return PointMetadata{}, errors.New("presented sequence cannot exceed the authoritative event count")
	}
	branch, err := store.loadBranchUnlocked(checkpoint.BattleID)
	if errors.Is(err, ErrNotFound) {
		branch = Branch{SchemaVersion: SchemaVersion, BattleID: checkpoint.BattleID, RootBattleID: checkpoint.BattleID, Status: BranchActive, CreatedAt: store.now()}
	} else if err != nil {
		return PointMetadata{}, err
	}
	if branch.Status != BranchActive {
		return PointMetadata{}, errors.New("history points can only be added to an active branch")
	}
	actionKey, actionType, err := canonicalAction(action)
	if err != nil {
		return PointMetadata{}, err
	}
	actionLabel := ""
	if actionKey != "" {
		actionLabel = label
	}
	// A newly reached state is recorded immediately with no action. When the
	// player later acts from that state, enrich the existing point instead of
	// adding a duplicate checkpoint. This keeps the newest state jumpable while
	// preserving action identity on the outgoing replay edge.
	if branch.HeadPointID != "" {
		head, loadErr := store.loadPointUnlocked(branch.HeadPointID)
		if loadErr != nil {
			return PointMetadata{}, loadErr
		}
		if head.Metadata.ActionKey == "" && reflect.DeepEqual(head.Checkpoint, checkpoint) {
			if head.Metadata.Label == "" {
				head.Metadata.Label = label
			}
			head.Metadata.Kind = kind
			head.Metadata.PresentedSequence = presentedSequence
			head.Metadata.ActionKey = actionKey
			head.Metadata.ActionType = actionType
			head.Metadata.ActionLabel = actionLabel
			head.ClientState = cloneMap(clientState)
			if err := store.writeAtomic(store.pointPath(head.Metadata.ID), head); err != nil {
				return PointMetadata{}, err
			}
			return head.Metadata, nil
		}
	}
	pointID, err := store.uniquePointIDUnlocked()
	if err != nil {
		return PointMetadata{}, err
	}
	metadata := PointMetadata{
		ID: pointID, ParentPointID: branch.HeadPointID, BattleID: checkpoint.BattleID,
		ActorID: actorID, Label: label, Kind: kind, CreatedAt: store.now(),
		Round: checkpoint.Battle.Segment.Round, Segment: string(checkpoint.Battle.Segment.Current),
		Stage: checkpoint.Battle.Flow.Stage, EventCount: len(checkpoint.Events), PresentedSequence: presentedSequence,
		ActionKey: actionKey, ActionType: actionType, ActionLabel: actionLabel,
	}
	point := Point{SchemaVersion: SchemaVersion, Metadata: metadata, ClientState: cloneMap(clientState), Checkpoint: checkpoint}
	if err := store.writeAtomic(store.pointPath(pointID), point); err != nil {
		return PointMetadata{}, err
	}
	branch.HeadPointID = pointID
	if err := store.writeAtomic(store.branchPath(branch.BattleID), branch); err != nil {
		return PointMetadata{}, err
	}
	return metadata, nil
}

func (store Store) List(battleID string) (Timeline, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if errors.Is(err, ErrNotFound) {
		return Timeline{Branch: Branch{BattleID: battleID, RootBattleID: battleID, Status: BranchActive}, Points: []PointMetadata{}}, nil
	}
	if err != nil {
		return Timeline{}, err
	}
	points, err := store.ancestryUnlocked(branch.HeadPointID)
	if err != nil {
		return Timeline{}, err
	}
	return Timeline{Branch: branch, Points: points}, nil
}

// Export returns the complete visible ancestry for a branch. A missing branch
// is represented by a nil bundle so snapshots made before history begins stay
// valid and backward compatible.
func (store Store) Export(battleID string) (*Bundle, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	points, err := store.ancestryPointsUnlocked(branch.HeadPointID)
	if err != nil {
		return nil, err
	}
	bundle := Bundle{SchemaVersion: SchemaVersion, Branch: branch, Points: points}
	return cloneBundle(bundle), nil
}

// ValidateBundle validates a history bundle without consulting a history
// store. Snapshot loading uses this before any files or battles are created.
func ValidateBundle(bundle Bundle) error {
	if bundle.SchemaVersion != SchemaVersion || bundle.Branch.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: unsupported history bundle schema", ErrCorrupt)
	}
	if bundle.Branch.BattleID == "" || bundle.Branch.RootBattleID == "" ||
		(bundle.Branch.Status != BranchActive && bundle.Branch.Status != BranchReview && bundle.Branch.Status != BranchReplay && bundle.Branch.Status != BranchArchived) {
		return fmt.Errorf("%w: invalid bundled branch", ErrCorrupt)
	}
	if len(bundle.Points) == 0 {
		if bundle.Branch.HeadPointID != "" || bundle.Branch.CursorPointID != "" || bundle.Branch.BasePointID != "" {
			return fmt.Errorf("%w: empty history bundle references points", ErrCorrupt)
		}
		return nil
	}
	seen := make(map[string]bool, len(bundle.Points))
	previous := ""
	for index := range bundle.Points {
		point := bundle.Points[index]
		if point.SchemaVersion != SchemaVersion || !validPointID.MatchString(point.Metadata.ID) || seen[point.Metadata.ID] {
			return fmt.Errorf("%w: invalid bundled point identity", ErrCorrupt)
		}
		seen[point.Metadata.ID] = true
		if point.Metadata.ParentPointID != previous {
			return fmt.Errorf("%w: bundled history is not a linear ancestry", ErrCorrupt)
		}
		if point.Metadata.BattleID == "" || point.Metadata.ActorID == "" || point.Metadata.Label == "" ||
			(point.Metadata.Kind != KindDecision && point.Metadata.Kind != KindPresentation) {
			return fmt.Errorf("%w: invalid bundled point metadata", ErrCorrupt)
		}
		if err := repository.ValidateCheckpoint(point.Checkpoint); err != nil {
			return fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
		if _, ok := point.Checkpoint.Battle.Actors[point.Metadata.ActorID]; !ok || point.Metadata.PresentedSequence > uint64(len(point.Checkpoint.Events)) {
			return fmt.Errorf("%w: bundled point actor or presentation watermark is invalid", ErrCorrupt)
		}
		previous = point.Metadata.ID
	}
	if bundle.Branch.HeadPointID != previous {
		return fmt.Errorf("%w: bundled branch head does not match its timeline", ErrCorrupt)
	}
	for _, referenced := range []string{bundle.Branch.CursorPointID, bundle.Branch.BasePointID} {
		if referenced != "" && !seen[referenced] {
			return fmt.Errorf("%w: bundled branch cursor is outside its timeline", ErrCorrupt)
		}
	}
	return nil
}

// Import creates an independent copy of a bundled timeline. Every point ID and
// checkpoint battle ID is remapped so loading the same snapshot repeatedly can
// never mutate another loaded copy or the original history.
func (store Store) Import(bundle Bundle, battleID, latestBattleID string) (ImportResult, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := ValidateBundle(bundle); err != nil {
		return ImportResult{}, err
	}
	if err := repository.ValidateBattleID(battleID); err != nil {
		return ImportResult{}, err
	}
	if _, err := os.Stat(store.branchPath(battleID)); err == nil {
		return ImportResult{}, errors.New("history branch already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return ImportResult{}, err
	}
	nonActive := bundle.Branch.Status != BranchActive
	if nonActive {
		if err := repository.ValidateBattleID(latestBattleID); err != nil || latestBattleID == battleID {
			return ImportResult{}, errors.New("restored non-active history requires an independent latest battle")
		}
		if _, err := os.Stat(store.branchPath(latestBattleID)); err == nil {
			return ImportResult{}, errors.New("restored latest history branch already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return ImportResult{}, err
		}
	} else {
		latestBattleID = battleID
	}

	idMap := make(map[string]string, len(bundle.Points))
	reserved := make(map[string]bool, len(bundle.Points))
	for index := range bundle.Points {
		pointID := ""
		for attempt := 0; attempt < 4; attempt++ {
			candidate, err := store.uniquePointIDUnlocked()
			if err != nil {
				return ImportResult{}, err
			}
			if !reserved[candidate] {
				pointID = candidate
				break
			}
		}
		if pointID == "" {
			return ImportResult{}, errors.New("could not generate unique imported history point ids")
		}
		reserved[pointID] = true
		idMap[bundle.Points[index].Metadata.ID] = pointID
	}

	checkpointBattleID := latestBattleID
	importedPoints := make([]Point, 0, len(bundle.Points))
	for index := range bundle.Points {
		source := bundle.Points[index]
		checkpoint, err := repository.CloneCheckpoint(source.Checkpoint, checkpointBattleID)
		if err != nil {
			return ImportResult{}, fmt.Errorf("clone bundled history point: %w", err)
		}
		metadata := source.Metadata
		metadata.ID = idMap[source.Metadata.ID]
		metadata.ParentPointID = idMap[source.Metadata.ParentPointID]
		metadata.BattleID = checkpointBattleID
		point := Point{SchemaVersion: SchemaVersion, Metadata: metadata, ClientState: cloneMap(source.ClientState), Checkpoint: checkpoint}
		if err := store.writeAtomic(store.pointPath(metadata.ID), point); err != nil {
			return ImportResult{}, err
		}
		importedPoints = append(importedPoints, point)
	}

	remap := func(pointID string) string { return idMap[pointID] }
	branch := bundle.Branch
	branch.SchemaVersion = SchemaVersion
	branch.BattleID = battleID
	branch.RootBattleID = battleID
	branch.HeadPointID = remap(bundle.Branch.HeadPointID)
	branch.CursorPointID = remap(bundle.Branch.CursorPointID)
	branch.BasePointID = remap(bundle.Branch.BasePointID)
	branch.DiscardedHeadPointID = remap(bundle.Branch.DiscardedHeadPointID)
	branch.ParentBattleID = ""
	branch.LatestBattleID = ""
	if nonActive {
		branch.RootBattleID = latestBattleID
		branch.ParentBattleID = latestBattleID
		branch.LatestBattleID = latestBattleID
		latest := Branch{SchemaVersion: SchemaVersion, BattleID: latestBattleID, RootBattleID: latestBattleID, HeadPointID: branch.HeadPointID, Status: BranchActive, CreatedAt: store.now()}
		if err := store.writeAtomic(store.branchPath(latestBattleID), latest); err != nil {
			return ImportResult{}, err
		}
	}
	if err := store.writeAtomic(store.branchPath(battleID), branch); err != nil {
		return ImportResult{}, err
	}
	currentID := branch.HeadPointID
	if branch.CursorPointID != "" {
		currentID = branch.CursorPointID
	} else if branch.BasePointID != "" && branch.Status != BranchActive {
		currentID = branch.BasePointID
	}
	var current Point
	for index := range importedPoints {
		if importedPoints[index].Metadata.ID == currentID {
			current = importedPoints[index]
			break
		}
	}
	return ImportResult{Branch: branch, Current: current}, nil
}

func (store Store) LoadPointForBranch(battleID, pointID string) (Point, Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return Point{}, Branch{}, err
	}
	contains, err := store.containsPointUnlocked(branch.HeadPointID, pointID)
	if err != nil {
		return Point{}, Branch{}, err
	}
	if !contains {
		return Point{}, Branch{}, ErrInvalidPoint
	}
	point, err := store.loadPointUnlocked(pointID)
	return point, branch, err
}

func (store Store) CreateReviewBranch(source Branch, battleID, pointID string) (Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := repository.ValidateBattleID(battleID); err != nil {
		return Branch{}, err
	}
	if _, err := os.Stat(store.branchPath(battleID)); err == nil {
		return Branch{}, errors.New("history branch already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Branch{}, err
	}
	parentBattleID := source.BattleID
	if source.Status == BranchReview && source.ParentBattleID != "" {
		parentBattleID = source.ParentBattleID
	}
	latestBattleID, err := store.latestBattleIDUnlocked(source)
	if err != nil {
		return Branch{}, err
	}
	branch := Branch{
		SchemaVersion: SchemaVersion, BattleID: battleID, RootBattleID: source.RootBattleID,
		HeadPointID: source.HeadPointID, CursorPointID: pointID, ParentBattleID: parentBattleID,
		LatestBattleID: latestBattleID, BasePointID: pointID,
		Status: BranchReview, CreatedAt: store.now(),
	}
	if branch.HeadPointID == "" {
		branch.HeadPointID = pointID
	}
	if err := store.writeAtomic(store.branchPath(battleID), branch); err != nil {
		return Branch{}, err
	}
	return branch, nil
}

func (store Store) CommitReview(battleID, mode string) (Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return Branch{}, err
	}
	if branch.Status != BranchReview {
		return Branch{}, errors.New("history branch is not in review mode")
	}
	if mode != "preserve" && mode != "replace" {
		return Branch{}, errors.New("history branch mode must be preserve or replace")
	}
	latestBattleID, err := store.latestBattleIDUnlocked(branch)
	if err != nil {
		return Branch{}, err
	}
	branch.LatestBattleID = latestBattleID
	if mode == "replace" && branch.ParentBattleID != "" {
		parent, err := store.loadBranchUnlocked(branch.ParentBattleID)
		if err != nil {
			return Branch{}, err
		}
		parent.Status = BranchArchived
		if err := store.writeAtomic(store.branchPath(parent.BattleID), parent); err != nil {
			return Branch{}, err
		}
	}
	point, err := store.loadPointUnlocked(store.cursorPointID(branch))
	if err != nil {
		return Branch{}, err
	}
	atTerminalState := point.Metadata.ID == branch.HeadPointID && point.Metadata.ActionKey == ""
	if atTerminalState {
		branch.CursorPointID = ""
		branch.Status = BranchActive
	} else if mode == "preserve" {
		branch.Status = BranchReplay
	} else {
		branch.DiscardedHeadPointID = branch.HeadPointID
		branch.HeadPointID = point.Metadata.ParentPointID
		branch.CursorPointID = ""
		branch.Status = BranchActive
	}
	if err := store.writeAtomic(store.branchPath(branch.BattleID), branch); err != nil {
		return Branch{}, err
	}
	return branch, nil
}

func (store Store) PrepareReplay(battleID string, action map[string]any) (ReplayStep, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return ReplayStep{}, err
	}
	if branch.Status != BranchReplay {
		return ReplayStep{}, errors.New("history branch is not replaying a preserved future")
	}
	cursor := store.cursorPointID(branch)
	current, err := store.loadPointUnlocked(cursor)
	if err != nil {
		return ReplayStep{}, err
	}
	actionKey, _, err := canonicalAction(action)
	if err != nil {
		return ReplayStep{}, err
	}
	points, err := store.ancestryPointsUnlocked(branch.HeadPointID)
	if err != nil {
		return ReplayStep{}, err
	}
	step := ReplayStep{Branch: branch, Current: current, Matches: current.Metadata.ActionKey != "" && current.Metadata.ActionKey == actionKey}
	for index := range points {
		if points[index].Metadata.ID != cursor {
			continue
		}
		step.FutureCount = len(points) - index
		if index+1 < len(points) {
			next := points[index+1]
			step.Next = &next
		}
		return step, nil
	}
	return ReplayStep{}, fmt.Errorf("%w: replay cursor is not in the branch timeline", ErrCorrupt)
}

func (store Store) AdvanceReplay(battleID, expectedCursor string) (Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return Branch{}, err
	}
	if branch.Status != BranchReplay || store.cursorPointID(branch) != expectedCursor {
		return Branch{}, errors.New("history replay cursor changed")
	}
	points, err := store.ancestryPointsUnlocked(branch.HeadPointID)
	if err != nil {
		return Branch{}, err
	}
	for index := range points {
		if points[index].Metadata.ID != expectedCursor {
			continue
		}
		if index+1 < len(points) {
			next := points[index+1]
			if index+1 == len(points)-1 && next.Metadata.ActionKey == "" {
				branch.CursorPointID = ""
				branch.Status = BranchActive
			} else {
				branch.CursorPointID = next.Metadata.ID
			}
		} else {
			branch.CursorPointID = ""
			branch.Status = BranchActive
		}
		if err := store.writeAtomic(store.branchPath(branch.BattleID), branch); err != nil {
			return Branch{}, err
		}
		return branch, nil
	}
	return Branch{}, fmt.Errorf("%w: replay cursor is not in the branch timeline", ErrCorrupt)
}

func (store Store) TruncateReplay(battleID string) (Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return Branch{}, err
	}
	if branch.Status != BranchReplay {
		return Branch{}, errors.New("history branch is not replaying a preserved future")
	}
	current, err := store.loadPointUnlocked(store.cursorPointID(branch))
	if err != nil {
		return Branch{}, err
	}
	branch.DiscardedHeadPointID = branch.HeadPointID
	branch.HeadPointID = current.Metadata.ParentPointID
	branch.CursorPointID = ""
	branch.Status = BranchActive
	if err := store.writeAtomic(store.branchPath(branch.BattleID), branch); err != nil {
		return Branch{}, err
	}
	return branch, nil
}

func (store Store) Branch(battleID string) (Branch, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return store.loadBranchUnlocked(battleID)
}

// ResolveLatestBattleID returns the authoritative end checkpoint for a
// retained future. Walking to the nearest active parent also repairs branches
// written by the original history implementation, which could retain a stale
// pre-divergence LatestBattleID after rewinding an already replaced future.
func (store Store) ResolveLatestBattleID(battleID string) (string, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	branch, err := store.loadBranchUnlocked(battleID)
	if err != nil {
		return "", err
	}
	return store.latestBattleIDUnlocked(branch)
}

func (store Store) latestBattleIDUnlocked(branch Branch) (string, error) {
	if branch.Status == BranchActive {
		return branch.BattleID, nil
	}
	seen := map[string]bool{branch.BattleID: true}
	parentID := branch.ParentBattleID
	for parentID != "" {
		if seen[parentID] {
			return "", fmt.Errorf("%w: history branch parent cycle", ErrCorrupt)
		}
		seen[parentID] = true
		parent, err := store.loadBranchUnlocked(parentID)
		if errors.Is(err, ErrNotFound) {
			break
		}
		if err != nil {
			return "", err
		}
		if parent.Status == BranchActive {
			return parent.BattleID, nil
		}
		parentID = parent.ParentBattleID
	}
	if branch.LatestBattleID != "" {
		return branch.LatestBattleID, nil
	}
	if branch.ParentBattleID != "" {
		return branch.ParentBattleID, nil
	}
	return branch.BattleID, nil
}

func (store Store) ancestryUnlocked(head string) ([]PointMetadata, error) {
	points, err := store.ancestryPointsUnlocked(head)
	if err != nil {
		return nil, err
	}
	values := make([]PointMetadata, 0, len(points))
	for _, point := range points {
		values = append(values, point.Metadata)
	}
	return values, nil
}

func (store Store) ancestryPointsUnlocked(head string) ([]Point, error) {
	var values []Point
	seen := map[string]bool{}
	for head != "" {
		if seen[head] {
			return nil, fmt.Errorf("%w: history parent cycle", ErrCorrupt)
		}
		seen[head] = true
		point, err := store.loadPointUnlocked(head)
		if err != nil {
			return nil, err
		}
		values = append(values, point)
		head = point.Metadata.ParentPointID
	}
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
	return values, nil
}

func (store Store) containsPointUnlocked(head, wanted string) (bool, error) {
	seen := map[string]bool{}
	for head != "" {
		if seen[head] {
			return false, fmt.Errorf("%w: history parent cycle", ErrCorrupt)
		}
		seen[head] = true
		if head == wanted {
			return true, nil
		}
		point, err := store.loadPointUnlocked(head)
		if err != nil {
			return false, err
		}
		head = point.Metadata.ParentPointID
	}
	return false, nil
}

func (store Store) loadPointUnlocked(pointID string) (Point, error) {
	if !validPointID.MatchString(pointID) {
		return Point{}, ErrInvalidPoint
	}
	var point Point
	if err := store.readJSON(store.pointPath(pointID), &point); err != nil {
		return Point{}, err
	}
	if point.SchemaVersion != SchemaVersion || point.Metadata.ID != pointID {
		return Point{}, fmt.Errorf("%w: point schema or identity mismatch", ErrCorrupt)
	}
	if err := repository.ValidateCheckpoint(point.Checkpoint); err != nil {
		return Point{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return point, nil
}

func (store Store) loadBranchUnlocked(battleID string) (Branch, error) {
	if err := repository.ValidateBattleID(battleID); err != nil {
		return Branch{}, ErrInvalidBranch
	}
	var branch Branch
	if err := store.readJSON(store.branchPath(battleID), &branch); err != nil {
		return Branch{}, err
	}
	if branch.SchemaVersion != SchemaVersion || branch.BattleID != battleID || branch.RootBattleID == "" ||
		(branch.Status != BranchActive && branch.Status != BranchReview && branch.Status != BranchReplay && branch.Status != BranchArchived) {
		return Branch{}, fmt.Errorf("%w: branch schema or identity mismatch", ErrCorrupt)
	}
	return branch, nil
}

func (store Store) cursorPointID(branch Branch) string {
	if branch.CursorPointID != "" {
		return branch.CursorPointID
	}
	return branch.BasePointID
}

func canonicalAction(action map[string]any) (string, string, error) {
	if len(action) == 0 {
		return "", "", nil
	}
	data, err := json.Marshal(action)
	if err != nil {
		return "", "", fmt.Errorf("encode history action: %w", err)
	}
	if len(data) > 64*1024 {
		return "", "", errors.New("history action exceeds 64 KiB")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), fmt.Sprint(action["type"]), nil
}

func (store Store) readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return nil
}

func (store Store) uniquePointIDUnlocked() (string, error) {
	for attempt := 0; attempt < 4; attempt++ {
		pointID, err := store.pointID()
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(store.pointPath(pointID)); errors.Is(err, os.ErrNotExist) {
			return pointID, nil
		}
	}
	return "", errors.New("could not generate a unique history point id")
}

func (store Store) pointID() (string, error) {
	if store.IDGenerator != nil {
		return store.IDGenerator()
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "history-" + hex.EncodeToString(suffix[:]), nil
}

func (store Store) now() time.Time {
	if store.Now != nil {
		return store.Now().UTC()
	}
	return time.Now().UTC()
}

func (store Store) pointPath(pointID string) string {
	return filepath.Join(filepath.Clean(store.Root), "points", pointID+".json")
}
func (store Store) branchPath(battleID string) string {
	return filepath.Join(filepath.Clean(store.Root), "branches", battleID+".json")
}

func (store Store) writeAtomic(path string, value any) error {
	if store.Root == "" {
		return errors.New("developer history root is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	data, _ := json.Marshal(value)
	var cloned map[string]any
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func cloneBundle(value Bundle) *Bundle {
	data, _ := json.Marshal(value)
	var cloned Bundle
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}
