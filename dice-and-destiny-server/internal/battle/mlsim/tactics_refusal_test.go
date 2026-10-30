package mlsim

import (
	"errors"
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/snapshot"
)

func tacticsTransition() Transition {
	return Transition{ActorID: "seat-b", Result: engine.Result{Snapshot: &snapshot.Battle{Tactics: &snapshot.TacticsView{}}}}
}

// §11.15: the ML environment and its encoders refuse tactics battles.
func TestMLEnvironmentRefusesTacticsBattles(t *testing.T) {
	transition := tacticsTransition()
	if err := RefuseTactics(transition); !errors.Is(err, ErrTacticsBattle) {
		t.Fatalf("RefuseTactics = %v", err)
	}
	if _, _, err := EncodeDecisionV2ForRuntime(transition); !errors.Is(err, ErrTacticsBattle) {
		t.Fatalf("v2 encoder = %v", err)
	}
	if _, _, err := (&V3RuntimeEncoder{}).Encode(transition); !errors.Is(err, ErrTacticsBattle) {
		t.Fatalf("v3 encoder = %v", err)
	}
	env := &Environment{}
	if _, err := env.selectNext(transition.Result, ""); !errors.Is(err, ErrTacticsBattle) {
		t.Fatalf("selectNext = %v", err)
	}
	if err := RefuseTactics(Transition{Result: engine.Result{Snapshot: &snapshot.Battle{}}}); err != nil {
		t.Fatalf("an ordinary battle was refused: %v", err)
	}
}
