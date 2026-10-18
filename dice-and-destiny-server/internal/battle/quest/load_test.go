package quest

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSet = "schema_version: 1\nid: fixture\nname: Fixtures\ntext: \"Test quests.\"\n"

// fixtureRoot is a content root holding one quest set with the given files
// (name → YAML).
func fixtureRoot(t *testing.T, sets map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for set, files := range sets {
		dir := filepath.Join(root, Dir, set)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		header := strings.ReplaceAll(fixtureSet, "fixture", set)
		if err := os.WriteFile(filepath.Join(dir, "set.yaml"), []byte(header), 0o644); err != nil {
			t.Fatal(err)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.ReplaceAll(body, "set: fixture", "set: "+set)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestValidFixturesLoad(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	files := map[string]string{}
	for _, name := range []string{"base_quest.yaml", "base_paper.yaml"} {
		files[name] = readFixture(t, filepath.Join("testdata", "valid", name))
	}
	cat, err := LoadQuests(fixtureRoot(t, map[string]map[string]string{"fixture": files}), cfg, pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Quests) != 2 || cat.Sets[0].Name != "Fixtures" || len(cat.Sets[0].Quests) != 2 {
		t.Fatalf("the valid fixtures load: %+v", cat.Sets)
	}
}

// One small invalid fixture per validator rule; its first line names the
// error it must produce.
func TestInvalidFixturesAreRefused(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	base := readFixture(t, filepath.Join("testdata", "valid", "base_quest.yaml"))
	paths, err := filepath.Glob(filepath.Join("testdata", "invalid", "*.yaml"))
	if err != nil || len(paths) < 30 {
		t.Fatalf("the invalid fixtures are missing (%d, %v)", len(paths), err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(strings.TrimSuffix(name, ".yaml"), func(t *testing.T) {
			body := readFixture(t, path)
			first, _, _ := strings.Cut(body, "\n")
			want, ok := strings.CutPrefix(first, "# want: ")
			if !ok {
				t.Fatalf("%s must start with a # want: line", name)
			}
			root := fixtureRoot(t, map[string]map[string]string{"fixture": {"base_quest.yaml": base, name: body}})
			_, err := LoadQuests(root, cfg, pack)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected an error containing %q, got %v", want, err)
			}
			if !strings.Contains(err.Error(), "quests_v1/fixture/"+name) {
				t.Fatalf("the error names the file: %v", err)
			}
		})
	}
}

func TestLoaderRefusesBadSetsAndFiles(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	base := readFixture(t, filepath.Join("testdata", "valid", "base_quest.yaml"))
	cases := []struct {
		name string
		sets map[string]map[string]string
		edit func(root string)
		want string
	}{
		{"duplicate quest id across sets", map[string]map[string]string{"fixture": {"base_quest.yaml": base}, "other": {"base_quest.yaml": base}}, nil, `duplicate quest id "base_quest"`},
		{"file name differs from the id", map[string]map[string]string{"fixture": {"renamed.yaml": base}}, nil, `"base_quest" must be snake_case and match the file name`},
		{"set differs from the folder", map[string]map[string]string{"fixture": {"base_quest.yaml": strings.Replace(base, "set: fixture", "set: elsewhere", 1)}}, nil, `"elsewhere" must be the folder's set "fixture"`},
		{"unknown field in set.yaml", map[string]map[string]string{"fixture": {"base_quest.yaml": base}}, func(root string) {
			path := filepath.Join(root, Dir, "fixture", "set.yaml")
			raw, _ := os.ReadFile(path)
			_ = os.WriteFile(path, append(raw, []byte("order: 1\n")...), 0o644)
		}, "field order not found"},
		{"empty set", map[string]map[string]string{"fixture": {}}, nil, `set "fixture" holds no quests`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := fixtureRoot(t, c.sets)
			if c.edit != nil {
				c.edit(root)
			}
			_, err := LoadQuests(root, cfg, pack)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
	if _, err := LoadQuests(t.TempDir(), cfg, pack); err == nil || !strings.Contains(err.Error(), "read quests_v1") {
		t.Fatalf("a content root without quests is refused: %v", err)
	}
}

// snapshot lists every file and directory under a root with its mode, size,
// modification time and content hash.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(&b, "%s %v %d %d", rel, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if !d.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, " %x", sha256.Sum256(raw))
		}
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestLoaderNeverWrites(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	files := map[string]string{}
	for _, name := range []string{"base_quest.yaml", "base_paper.yaml"} {
		files[name] = readFixture(t, filepath.Join("testdata", "valid", name))
	}
	root := fixtureRoot(t, map[string]map[string]string{"fixture": files})
	// A read-only quest folder still loads: the loader only reads.
	dir := filepath.Join(root, Dir)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, root)
	must(filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(path, 0o555)
		}
		return os.Chmod(path, 0o444)
	}))
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o755)
			}
			return nil
		})
	})
	readOnly := snapshot(t, root)
	if _, err := LoadQuests(root, cfg, pack); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, root); after != readOnly {
		t.Fatalf("loading changed the content root:\n%s\nvs\n%s", readOnly, after)
	}
	if before == readOnly {
		t.Fatal("the snapshot sees file modes")
	}
}
