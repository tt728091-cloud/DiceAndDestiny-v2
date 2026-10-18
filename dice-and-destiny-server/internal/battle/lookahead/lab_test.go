package lookahead_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
)

func serverRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// authoredCopy gives the test its own copy of the published cards; tests
// never read or write the tracked authored directory directly.
func authoredCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"authored_cards.json", "economy_admin.json"} {
		raw, err := os.ReadFile(filepath.Join(serverRoot(t), "content", "authored", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func brineMaskRunner(t *testing.T) (*lookahead.Runner, lookahead.Policy) {
	t.Helper()
	contentRoot := filepath.Join(serverRoot(t), "content")
	runner, err := lookahead.NewRunner(lookahead.Matchup{
		ContentRoot:  contentRoot,
		AuthoredRoot: authoredCopy(t),
		RunStateRoot: t.TempDir(),
		Hero:         "starter",
		Opponent:     "drowned_oracle_brine_mask",
		Unified:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	minion, err := learned.LoadScriptedMinion(contentRoot, "drowned_oracle_brine_mask")
	if err != nil {
		t.Fatal(err)
	}
	return runner, lookahead.Scripted{Label: "brine_mask", Policy: minion}
}
