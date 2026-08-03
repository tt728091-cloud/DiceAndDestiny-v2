package mlsim

import (
	"encoding/json"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/snapshot"
)

func TestV2ActionProgressMaskRejectsAllProvisionalKeepCommands(t *testing.T) {
	action := command.Command{Type: command.TypePlanningKeep}
	action.Payload, _ = json.Marshal(command.PlanningKeepPayload{KeptIndices: []int{0, 2}})
	kept := map[int]bool{0: true, 2: true}
	if v2ActionProgresses(action, kept, snapshot.Actor{}, snapshot.ContentCatalog{}) {
		t.Fatal("idempotent keep must be masked")
	}
	action.Payload, _ = json.Marshal(command.PlanningKeepPayload{KeptIndices: []int{0}})
	if v2ActionProgresses(action, kept, snapshot.Actor{}, snapshot.ContentCatalog{}) {
		t.Fatal("non-idempotent keep must also be masked because it does not advance planning")
	}
	if !v2ActionProgresses(command.Command{Type: command.TypePlanningPass}, kept, snapshot.Actor{}, snapshot.ContentCatalog{}) {
		t.Fatal("progressing non-keep action must remain selectable")
	}
}
