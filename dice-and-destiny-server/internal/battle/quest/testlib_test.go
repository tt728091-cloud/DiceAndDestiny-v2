package quest

import (
	"path/filepath"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/content"
)

// contentRoot resolves dice-and-destiny-server/content from this file.
func contentRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve the content root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "content"))
}

// shippedRules loads the expedition config and region pack the quests refer
// to, against the base battle library and every character pack (never the
// tracked authored overlay).
func shippedRules(t *testing.T) (expedition.Config, expedition.Pack, content.BattleLibrary) {
	t.Helper()
	root := contentRoot(t)
	lib, err := content.LoadBattleLibrary(filepath.Join(root, "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"venom_v1", "curse_v1", "adventurer_v1", "general_v1", "minions_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join(root, pack)); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := expedition.LoadConfig(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := expedition.LoadPack(root, cfg, lib)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, pack, lib
}
