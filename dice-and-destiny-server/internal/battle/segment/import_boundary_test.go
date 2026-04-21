package segment

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSegmentProductionCodeDoesNotImportGameplayOrPresentationPackages(t *testing.T) {
	forbiddenImportFragments := []string{
		"internal/battle/card",
		"internal/battle/dice",
		"internal/battle/damage",
		"internal/battle/engine",
		"internal/battle/enemy",
		"internal/battle/state",
		"dice-and-destiny-client",
		"gdextension",
		"godot",
		"/ui",
		"ui/",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("finding Go files: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing imports in %s: %v", file, err)
		}

		for _, imp := range parsed.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquoting import path %s in %s: %v", imp.Path.Value, file, err)
			}

			lowerImportPath := strings.ToLower(importPath)
			for _, forbidden := range forbiddenImportFragments {
				if strings.Contains(lowerImportPath, forbidden) {
					t.Fatalf("segment production file %s imports forbidden package %q", file, importPath)
				}
			}
		}
	}
}
