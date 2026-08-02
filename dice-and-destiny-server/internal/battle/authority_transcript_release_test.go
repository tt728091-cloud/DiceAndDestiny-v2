//go:build !transcript_tools

package battle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorityTranscriptCompileGateCannotBeEnabled(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	t.Setenv("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT", "1")
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PROJECT_ENABLED", "1")
	t.Setenv("DICE_AND_DESTINY_RUNTIME_ROOT", root)
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH", path)

	if recorder := newAuthorityTranscript(); recorder != nil {
		t.Fatalf("release authority unexpectedly constructed an omniscient transcript recorder: %#v", recorder)
	}
	RecordAuthorityTranscriptDiagnostic("release-gate", "actor", "private_fact", map[string]any{"secret": true})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("release authority created a transcript despite the compile gate: %v", err)
	}
}
