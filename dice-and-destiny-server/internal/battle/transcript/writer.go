package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	enableEnv      = "DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT"
	projectGateEnv = "DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PROJECT_ENABLED"
	pathEnv        = "DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH"
	runtimeRootEnv = "DICE_AND_DESTINY_RUNTIME_ROOT"
)

var registry = struct {
	sync.Mutex
	writers map[string]*Recorder
}{writers: map[string]*Recorder{}}

type Recorder struct {
	mu             sync.Mutex
	path           string
	sequence       uint64
	activeCauses   map[string]uint64
	privateRecords map[string]uint64
	lastError      error
}

func FromEnvironment() *Recorder {
	if os.Getenv(enableEnv) != "1" || os.Getenv(projectGateEnv) != "1" {
		return nil
	}
	path := filepath.Clean(strings.TrimSpace(os.Getenv(pathEnv)))
	runtimeRoot := filepath.Clean(strings.TrimSpace(os.Getenv(runtimeRootEnv)))
	if path == "." || runtimeRoot == "." || !filepath.IsAbs(path) || !filepath.IsAbs(runtimeRoot) {
		reportError(errors.New("authority transcript requires absolute workspace runtime and output paths"))
		return nil
	}
	relative, err := filepath.Rel(runtimeRoot, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		reportError(fmt.Errorf("authority transcript path %q is outside workspace runtime %q", path, runtimeRoot))
		return nil
	}
	want := filepath.Join("user", "debug", "authority-transcript.jsonl")
	if filepath.Clean(relative) != want {
		reportError(fmt.Errorf("authority transcript path must be workspace-local %q, got %q", want, relative))
		return nil
	}

	registry.Lock()
	defer registry.Unlock()
	if existing := registry.writers[path]; existing != nil {
		return existing
	}
	recorder := &Recorder{
		path:           path,
		activeCauses:   map[string]uint64{},
		privateRecords: map[string]uint64{},
	}
	if err := recorder.loadSequence(); err != nil {
		recorder.lastError = err
		reportError(err)
		return nil
	}
	registry.writers[path] = recorder
	return recorder
}

func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

func (r *Recorder) loadSequence() error {
	file, err := os.Open(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open existing authority transcript: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			continue
		}
		var envelope struct {
			SchemaVersion int    `json:"schema_version"`
			Sequence      uint64 `json:"sequence"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			return fmt.Errorf("authority transcript line %d is not valid JSON: %w", lineNumber, err)
		}
		if envelope.SchemaVersion != SchemaVersion {
			return fmt.Errorf("authority transcript line %d has unsupported schema version %d", lineNumber, envelope.SchemaVersion)
		}
		if envelope.Sequence <= r.sequence {
			return fmt.Errorf("authority transcript line %d has non-monotonic sequence %d after %d", lineNumber, envelope.Sequence, r.sequence)
		}
		r.sequence = envelope.Sequence
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan existing authority transcript: %w", err)
	}
	return nil
}

func (r *Recorder) appendDrafts(cause uint64, drafts []draft) []Record {
	if r == nil || len(drafts) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		r.fail(fmt.Errorf("create authority transcript directory: %w", err))
		return nil
	}
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		r.fail(fmt.Errorf("open authority transcript: %w", err))
		return nil
	}
	defer file.Close()

	written := make([]Record, 0, len(drafts))
	for _, item := range drafts {
		r.sequence++
		item.SchemaVersion = SchemaVersion
		item.RecorderVersion = RecorderVersion
		item.RecordedAtUTC = time.Now().UTC().Format(time.RFC3339Nano)
		item.Sequence = r.sequence
		if item.CausedBySequence == 0 {
			item.CausedBySequence = cause
		}
		var reveals []uint64
		for _, key := range item.revealKeys {
			if sequence := r.privateRecords[item.BattleID+"|"+key]; sequence != 0 {
				reveals = append(reveals, sequence)
				if item.RevealOfSequence == 0 {
					item.RevealOfSequence = sequence
				}
			}
		}
		if len(reveals) > 0 {
			if item.Details == nil {
				item.Details = map[string]any{}
			}
			item.Details["reveals_sequences"] = reveals
		}
		encoded, encodeErr := json.Marshal(item.Record)
		if encodeErr != nil {
			r.fail(fmt.Errorf("encode authority transcript record: %w", encodeErr))
			return written
		}
		line := append(encoded, '\n')
		n, writeErr := file.Write(line)
		if writeErr != nil || n != len(line) {
			if writeErr == nil {
				writeErr = ioErrShortWrite
			}
			r.fail(fmt.Errorf("append authority transcript record: %w", writeErr))
			return written
		}
		if item.privateKey != "" {
			r.privateRecords[item.BattleID+"|"+item.privateKey] = item.Sequence
		}
		written = append(written, item.Record)
	}
	if err := file.Sync(); err != nil {
		r.fail(fmt.Errorf("sync authority transcript: %w", err))
	}
	return written
}

var ioErrShortWrite = errors.New("short write")

func (r *Recorder) fail(err error) {
	r.lastError = err
	reportError(err)
}

func reportError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[AuthorityTranscript] ERROR: %v\n", err)
}

// ResetRegistryForTests prevents a prior environment path from retaining a
// cached sequence during tagged tests. It is intentionally unavailable to
// gameplay callers outside this internal package.
func ResetRegistryForTests() {
	registry.Lock()
	defer registry.Unlock()
	registry.writers = map[string]*Recorder{}
}
