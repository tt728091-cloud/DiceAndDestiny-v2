package lookahead

import (
	"errors"
	"testing"

	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
)

// §11.15: the balance lab's players refuse tactics battles.
func TestLabPlayersRefuseTacticsBattles(t *testing.T) {
	d := &Decision{Battle: &state.Battle{Settled: &state.SettledRuntime{Tactics: true, TacticsRuntime: &state.TacticsRuntime{}}}}
	for _, p := range []Policy{RandomPolicy{}, NewGreedyPolicy(), NewSearchPolicy(func() Policy { return RandomPolicy{} }, 1), ProxyPolicy{}, Scripted{Label: "minion"}} {
		if _, err := p.Choose(d); !errors.Is(err, mlsim.ErrTacticsBattle) {
			t.Fatalf("%s: %v", p.Name(), err)
		}
	}
}
