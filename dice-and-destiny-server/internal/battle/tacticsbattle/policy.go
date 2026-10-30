package tacticsbattle

import (
	"errors"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
)

// enemyPolicy picks one of a seat's legal actions from its own view.
type enemyPolicy interface {
	choose(seat string, view engine.Result) (command.Command, error)
}

// scriptedPolicy drives the sandbox enemies. (M2: roll and pass; the
// behaviour of plan §8 follows in M3.)
type scriptedPolicy struct {
	enemies map[string]EnemyConfig
}

func (p scriptedPolicy) choose(seat string, view engine.Result) (command.Command, error) {
	if len(view.LegalActions) == 0 {
		return command.Command{}, errors.New("no legal action")
	}
	for _, kind := range []command.Type{command.TypePlanningRoll, command.TypeMovementPlan, command.TypePlanningPass, command.TypePass} {
		for _, action := range view.LegalActions {
			if action.Type == kind {
				return action, nil
			}
		}
	}
	return view.LegalActions[0], nil
}
