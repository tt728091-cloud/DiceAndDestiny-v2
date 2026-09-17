package transcript

import (
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestCatalystSameFaceRetryIsPublicAndExplicit(t *testing.T) {
	battle := &state.Battle{Actors: map[string]state.ActorState{
		"seat-a": {Character: state.CharacterMetadata{Name: "Venom"}},
		"seat-b": {Character: state.CharacterMetadata{Name: "Blade Warden"}},
	}}
	lib := content.BattleLibrary{Statuses: map[string]content.BattleStatusDefinition{"poison": {ID: "poison", Name: "Poison"}}}
	roll := state.SettledEffectRoll{ActorID: "seat-b", SourceContentID: "poison", Rerolled: true, Resolved: true, Die: state.RolledDie{Index: 1, Face: 5}}
	result := eventDrafts(event.Event{Type: event.TypeDiceRolled, ActorID: "seat-b", Data: map[string]any{
		"source_type": "catalyst", "holder": "seat-a", "forced": true,
		"die_index": 1, "face_before": 5, "status_id": "poison", "rolls": []state.SettledEffectRoll{roll},
	}}, Transition{Command: command.Command{BattleID: "test"}, After: battle}, lib)
	if len(result) != 1 {
		t.Fatalf("got %d records", len(result))
	}
	r := result[0]
	if r.Visibility != VisibilityPublic || r.SourceID != "catalyst" || r.StatusID != "poison" {
		t.Fatalf("forced public reroll misclassified: %+v", r)
	}
	if !strings.Contains(r.Summary, "Venom's Catalyst automatically rerolled Blade Warden's Poison die 2: 5 → 5") || !strings.Contains(r.Summary, "passing only declines a card response") {
		t.Fatalf("ambiguous retry summary: %s", r.Summary)
	}
	if dice := r.Details["dice"].([]state.RolledDie); len(dice) != 1 || dice[0].Index != 1 {
		t.Fatalf("reported untouched dice as rerolls: %+v", dice)
	}
}
