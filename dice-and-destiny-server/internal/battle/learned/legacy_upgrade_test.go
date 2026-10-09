package learned

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyUpgradeContent mirrors the shipped content root but authors one
// explicit economy card upgrade for the Adventurer. Shipped cards progress
// through card trees, so tests of the economy's direct card upgrades supply
// their own: Steady Guard upgrades into Emergency Ward (priced 20 XP) for 10 XP.
func legacyUpgradeContent(t *testing.T) string {
	t.Helper()
	source := filepath.Join(testServerRoot(t), "content")
	root := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "progression_v1" {
			continue
		}
		if err = os.Symlink(filepath.Join(source, entry.Name()), filepath.Join(root, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(source, "progression_v1", "economy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	anchor := "      defensive: [adventurer_guard]\n"
	if !strings.Contains(string(raw), anchor) {
		t.Fatal("Adventurer economy anchor not found")
	}
	economy := strings.Replace(string(raw), anchor, anchor+"    card_prices: {emergency_ward: 20}\n    card_upgrades:\n      steady_guard: {to: emergency_ward, xp: 10}\n", 1)
	if err = os.MkdirAll(filepath.Join(root, "progression_v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "progression_v1", "economy.yaml"), []byte(economy), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}
