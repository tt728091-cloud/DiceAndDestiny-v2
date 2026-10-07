package learned

import (
	"diceanddestiny/server/internal/content"
	"path/filepath"
	"strings"
	"testing"
)

// Status filters must agree with the target polarity and read by name.
func TestStatusFiltersMatchPolarityAndUseNames(t *testing.T) {
	catalogs, err := CharacterCatalogs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	lib := catalogs["adventurer"]
	c, err := content.EditableGeneralCard(lib.Cards["antidote"])
	if err != nil {
		t.Fatal(err)
	}
	if c.Program.Steps[0].Effect != "remove_status" {
		t.Fatalf("antidote program = %+v", c.Program.Steps)
	}
	target := &c.Program.Steps[0].Target
	target.Owner, target.Mode, target.Selection = "self", "one", "choose"
	target.Polarity = "negative"
	target.StatusIDs = []string{"bleed", "poison"}
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatalf("negative filters rejected: %v", err)
	}
	rules := content.PrepareProgramCard(c, &lib).Presentation.RulesText
	if !strings.Contains(rules, "matching Bleed, Poison") || strings.Contains(rules, "bleed") {
		t.Fatalf("rules should name filtered statuses: %q", rules)
	}
	target.StatusIDs = []string{"bleed", "protect"}
	if err := content.ValidateCardProgram(c, lib); err == nil || !strings.Contains(err.Error(), "Protect") {
		t.Fatalf("positive filter on a negative-only effect accepted: %v", err)
	}
	target.Polarity = "any"
	if err := content.ValidateCardProgram(c, lib); err != nil {
		t.Fatalf("any polarity accepts mixed filters: %v", err)
	}
}
