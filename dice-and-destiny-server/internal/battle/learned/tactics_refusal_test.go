package learned

import (
	"errors"
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// §11.15: the learned, scripted-session and look-ahead players refuse
// tactics battles rather than mis-handle them.
func TestPoliciesRefuseTacticsBattles(t *testing.T) {
	transition := mlsim.Transition{ActorID: "seat-b", Result: engine.Result{Snapshot: &snapshot.Battle{Tactics: &snapshot.TacticsView{}}}}
	if _, err := EncodeDecision(transition); !errors.Is(err, mlsim.ErrTacticsBattle) {
		t.Fatalf("v1 encoder = %v", err)
	}
	if _, _, err := (&Policy{}).Select(transition); !errors.Is(err, mlsim.ErrTacticsBattle) {
		t.Fatalf("v1 policy = %v", err)
	}
	if _, _, err := (&PolicyV2{}).Select(transition); !errors.Is(err, mlsim.ErrTacticsBattle) {
		t.Fatalf("v2 policy = %v", err)
	}
	minion := &singleAbilityPolicy{definition: content.CombatantDefinition{SingleAbilityPolicy: &content.SingleAbilityPolicy{TargetFace: 3}}}
	if _, _, err := minion.Select(transition); !errors.Is(err, mlsim.ErrTacticsBattle) {
		t.Fatalf("scripted minion = %v", err)
	}
	battle := state.Battle{Settled: &state.SettledRuntime{Tactics: true, TacticsRuntime: &state.TacticsRuntime{}}}
	if _, _, err := (&lookaheadPolicy{}).SelectWithState(transition, battle); !errors.Is(err, mlsim.ErrTacticsBattle) {
		t.Fatalf("look-ahead = %v", err)
	}
}
