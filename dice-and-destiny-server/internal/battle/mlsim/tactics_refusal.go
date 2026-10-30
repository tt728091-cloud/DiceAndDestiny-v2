package mlsim

import "errors"

// ErrTacticsBattle refuses a Tactics Sandbox battle: the ML environment, its
// encoders and the learned and search players do not model the grid.
var ErrTacticsBattle = errors.New("tactics battles are not supported by the ML environment or its policies")

// RefuseTactics reports ErrTacticsBattle for a tactics battle's transition.
func RefuseTactics(transition Transition) error {
	if transition.Result.Snapshot != nil && transition.Result.Snapshot.Tactics != nil {
		return ErrTacticsBattle
	}
	return nil
}
