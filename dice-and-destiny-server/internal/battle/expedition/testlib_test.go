package expedition

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"diceanddestiny/server/internal/content"
)

// serverRoot resolves dice-and-destiny-server/ from this file.
func serverRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve the server root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func contentRoot(t *testing.T) string { return filepath.Join(serverRoot(t), "content") }

// testLibrary builds the Starter's battle library: the base pack plus every
// character pack and the minions, like learned.CharacterCatalogs, without the
// authored overlay (tests must never touch the tracked authored content).
func testLibrary(t *testing.T, root string) content.BattleLibrary {
	t.Helper()
	lib, err := content.LoadBattleLibrary(filepath.Join(root, "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"venom_v1", "curse_v1", "adventurer_v1", "general_v1", "minions_v1"} {
		if lib, err = content.LoadBattleExtension(lib, filepath.Join(root, pack)); err != nil {
			t.Fatal(err)
		}
	}
	return lib
}

func loadShipped(t *testing.T) (Config, Pack, content.BattleLibrary) {
	t.Helper()
	root := contentRoot(t)
	lib := testLibrary(t, root)
	cfg, err := LoadConfig(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := LoadPack(root, cfg, lib)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, pack, lib
}

// copyTree copies a directory tree of small text files into dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// scratchContent copies progression_v1 and expedition_v1 into a temp content
// root so a test can edit one file. Other packs are read from the real root
// through the library, which is loaded separately.
func scratchContent(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join(contentRoot(t), "progression_v1"), filepath.Join(root, "progression_v1"))
	copyTree(t, filepath.Join(contentRoot(t), "expedition_v1"), filepath.Join(root, "expedition_v1"))
	return root
}

func rewrite(t *testing.T, path string, edit func(string) string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next := edit(string(data))
	if next == string(data) {
		t.Fatalf("edit did not change %s", path)
	}
	if err = os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceOnce(old, new string) func(string) string {
	return func(s string) string {
		if !strings.Contains(s, old) {
			return s
		}
		return strings.Replace(s, old, new, 1)
	}
}

// setNight forces Day or Night while keeping the round, as tests did when
// Night was a stored flag: the clock and the region start move together by
// one half, so only the Day/Night of the clock changes.
func setNight(s *State, night bool) {
	if s.Night() != night {
		s.Clock.Half++
		s.RegionStart++
	}
}
