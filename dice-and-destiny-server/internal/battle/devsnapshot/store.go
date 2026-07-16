package devsnapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"diceanddestiny/server/internal/battle/devhistory"
	"diceanddestiny/server/internal/battle/repository"
)

const SchemaVersion = 2
const legacySchemaVersion = 1

var (
	ErrInvalidName   = errors.New("invalid developer snapshot name")
	ErrNotFound      = errors.New("developer snapshot not found")
	ErrAlreadyExists = errors.New("developer snapshot already exists")
	ErrCorrupt       = errors.New("corrupt developer snapshot")
	validName        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

type Metadata struct {
	Name              string    `json:"name"`
	CreatedAt         time.Time `json:"created_at"`
	SourceBattleID    string    `json:"source_battle_id"`
	ActorID           string    `json:"actor_id"`
	Round             int       `json:"round"`
	Segment           string    `json:"segment"`
	Stage             string    `json:"stage,omitempty"`
	Status            string    `json:"status"`
	EventCount        int       `json:"event_count"`
	OriginKind        string    `json:"origin_kind"`
	HistoryIncluded   bool      `json:"history_included,omitempty"`
	HistoryPointCount int       `json:"history_point_count,omitempty"`
}

type HistoryRecord struct {
	Bundle           devhistory.Bundle      `json:"bundle"`
	LatestCheckpoint *repository.Checkpoint `json:"latest_checkpoint,omitempty"`
}

type Record struct {
	SchemaVersion int                   `json:"schema_version"`
	Metadata      Metadata              `json:"metadata"`
	Checkpoint    repository.Checkpoint `json:"checkpoint"`
	History       *HistoryRecord        `json:"history,omitempty"`
}

type Store struct {
	Root string
	Now  func() time.Time
}

func ValidateName(name string) error {
	if !validName.MatchString(name) || name == "." || name == ".." {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}

func (store Store) Save(name, actorID string, checkpoint repository.Checkpoint, overwrite bool) (Metadata, error) {
	return store.SaveWithHistory(name, actorID, checkpoint, nil, overwrite)
}

func (store Store) SaveWithHistory(name, actorID string, checkpoint repository.Checkpoint, history *HistoryRecord, overwrite bool) (Metadata, error) {
	if err := ValidateName(name); err != nil {
		return Metadata{}, err
	}
	if err := repository.ValidateCheckpoint(checkpoint); err != nil {
		return Metadata{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if _, ok := checkpoint.Battle.Actors[actorID]; !ok {
		return Metadata{}, errors.New("snapshot actor is not a battle participant")
	}
	if err := validateHistory(history); err != nil {
		return Metadata{}, err
	}
	path, err := store.path(name)
	if err != nil {
		return Metadata{}, err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return Metadata{}, ErrAlreadyExists
		} else if !errors.Is(err, os.ErrNotExist) {
			return Metadata{}, fmt.Errorf("inspect developer snapshot: %w", err)
		}
	}
	now := time.Now().UTC()
	if store.Now != nil {
		now = store.Now().UTC()
	}
	metadata := Metadata{
		Name:            name,
		CreatedAt:       now,
		SourceBattleID:  checkpoint.BattleID,
		ActorID:         actorID,
		Round:           checkpoint.Battle.Segment.Round,
		Segment:         string(checkpoint.Battle.Segment.Current),
		Stage:           checkpoint.Battle.Flow.Stage,
		Status:          string(checkpoint.Battle.Status),
		EventCount:      len(checkpoint.Events),
		OriginKind:      checkpoint.Battle.Origin.Kind,
		HistoryIncluded: history != nil,
	}
	if history != nil {
		metadata.HistoryPointCount = len(history.Bundle.Points)
	}
	record := Record{SchemaVersion: SchemaVersion, Metadata: metadata, Checkpoint: checkpoint, History: history}
	if err := store.writeAtomic(path, record); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func (store Store) Load(name string) (Record, error) {
	path, err := store.path(name)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read developer snapshot: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("%w: decode: %v", ErrCorrupt, err)
	}
	if (record.SchemaVersion != legacySchemaVersion && record.SchemaVersion != SchemaVersion) || record.Metadata.Name != name {
		return Record{}, fmt.Errorf("%w: unsupported schema or mismatched name", ErrCorrupt)
	}
	if err := repository.ValidateCheckpoint(record.Checkpoint); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if record.SchemaVersion == legacySchemaVersion {
		record.History = nil
		record.Metadata.HistoryIncluded = false
		record.Metadata.HistoryPointCount = 0
	} else if err := validateHistory(record.History); err != nil {
		return Record{}, err
	}
	return record, nil
}

func validateHistory(history *HistoryRecord) error {
	if history == nil {
		return nil
	}
	if err := devhistory.ValidateBundle(history.Bundle); err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if history.Bundle.Branch.Status != devhistory.BranchActive {
		if history.LatestCheckpoint == nil {
			return fmt.Errorf("%w: non-active history is missing its latest checkpoint", ErrCorrupt)
		}
		if err := repository.ValidateCheckpoint(*history.LatestCheckpoint); err != nil {
			return fmt.Errorf("%w: invalid latest history checkpoint: %v", ErrCorrupt, err)
		}
	}
	return nil
}

func (store Store) List() ([]Metadata, error) {
	if store.Root == "" {
		return nil, errors.New("developer snapshot root is required")
	}
	entries, err := os.ReadDir(filepath.Clean(store.Root))
	if errors.Is(err, os.ErrNotExist) {
		return []Metadata{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read developer snapshot directory: %w", err)
	}
	result := make([]Metadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".json")]
		record, err := store.Load(name)
		if err != nil {
			return nil, err
		}
		result = append(result, record.Metadata)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].Name < result[j].Name
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (store Store) path(name string) (string, error) {
	if store.Root == "" {
		return "", errors.New("developer snapshot root is required")
	}
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Clean(store.Root), name+".json"), nil
}

func (store Store) writeAtomic(path string, record Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create developer snapshot directory: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode developer snapshot: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create developer snapshot temporary file: %w", err)
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
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace developer snapshot: %w", err)
	}
	return nil
}
