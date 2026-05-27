package engine

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEngineProductionCodeDoesNotImportIncomeRuleDependencies(t *testing.T) {
	forbiddenImportFragments := []string{
		"internal/battle/card",
		"internal/battle/resource",
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
					t.Fatalf("engine production file %s imports income rule dependency %q", file, importPath)
				}
			}
		}
	}
}
