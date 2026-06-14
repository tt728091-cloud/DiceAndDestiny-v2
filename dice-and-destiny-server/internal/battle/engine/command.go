package engine

import (
	"errors"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

type Result struct {
	Accepted     bool                             `json:"accepted"`
	Status       ProgressStatus                   `json:"status,omitempty"`
	BattleResult state.BattleStatus               `json:"battle_result,omitempty"`
	Events       []event.Event                    `json:"events,omitempty"`
	PendingInput map[string]snapshot.PendingInput `json:"pending_input,omitempty"`
	Snapshot     *snapshot.Battle                 `json:"snapshot,omitempty"`
	Error        string                           `json:"error,omitempty"`
}

func (e Engine) HandleCommand(cmd command.Command) Result {
	return rejected("battle repository is required")
}

func (e Engine) AdvanceUntilInput(battle *state.Battle, viewerActorID string) Result {
	progressed, err := e.ProgressUntilInput(battle)
	if err != nil {
		return rejected(err.Error())
	}
	return e.ResultForViewer(battle, viewerActorID, progressed)
}

func (e Engine) HandleBattleCommand(battle *state.Battle, cmd command.Command) Result {
	progressed, err := e.ApplyBattleCommand(battle, cmd)
	if err != nil {
		return rejected(err.Error())
	}
	return e.ResultForViewer(battle, cmd.ActorID, progressed)
}

func (e Engine) ApplyBattleCommand(battle *state.Battle, cmd command.Command) (ProgressionResult, error) {
	if battle == nil {
		return ProgressionResult{}, errors.New("battle is nil")
	}
	if cmd.BattleID != battle.ID {
		return ProgressionResult{}, errors.New("command battle does not match current battle")
	}
	if state.IsTerminalBattleStatus(battle.Status) {
		return ProgressionResult{}, errors.New("battle is complete")
	}

	flow, err := e.FlowFor(battle.Segment.Current)
	if err != nil {
		return ProgressionResult{}, err
	}

	working := battle.Clone()
	var commandEvents []event.Event
	if working.ActiveResolutionID != "" {
		resolution, window, activeErr := activeWindow(&working)
		if activeErr != nil {
			return ProgressionResult{}, activeErr
		}
		if resolution.Planning != nil && window.Purpose == state.InteractionPurposePlanning {
			commandEvents, err = e.handlePlanningCommand(&working, cmd)
		} else {
			commandEvents, err = e.handleInteractionCommand(&working, cmd)
		}
	} else {
		commandEvents, err = flow.HandleCommand(&Context{Battle: &working, Phase: state.FlowPhaseInProgress}, cmd)
	}
	if err != nil {
		return ProgressionResult{}, err
	}

	progressed, err := e.ProgressUntilInput(&working)
	if err != nil {
		return ProgressionResult{}, err
	}
	*battle = working

	events := append([]event.Event(nil), commandEvents...)
	events = append(events, progressed.Events...)
	return ProgressionResult{
		Status: progressed.Status,
		Events: events,
	}, nil
}

func (e Engine) ResultForViewer(
	battle *state.Battle,
	viewerActorID string,
	progressed ProgressionResult,
) Result {
	viewerActorID = viewerActorIDForBattle(battle, viewerActorID)
	result := Result{
		Accepted:     true,
		Status:       progressed.Status,
		Events:       event.ForViewer(progressed.Events, viewerActorID),
		PendingInput: snapshot.PendingInputForViewer(*battle, viewerActorID),
		Snapshot:     battleSnapshotForViewer(battle, viewerActorID),
	}
	if state.IsTerminalBattleStatus(battle.Status) {
		result.Status = ProgressBattleComplete
		result.BattleResult = battle.Status
	}
	return result
}

func battleSnapshotForViewer(battle *state.Battle, viewerActorID string) *snapshot.Battle {
	if battle == nil {
		return nil
	}
	snap := snapshot.FromBattleForViewer(*battle, viewerActorIDForBattle(battle, viewerActorID))
	return &snap
}

func viewerActorIDForBattle(battle *state.Battle, viewerActorID string) string {
	if battle == nil {
		return ""
	}
	if _, ok := battle.Actors[viewerActorID]; ok {
		return viewerActorID
	}
	return ""
}

func unsupportedCommand() error {
	return errors.New("unsupported command type")
}

func rejected(message string) Result {
	return Result{
		Accepted: false,
		Error:    message,
	}
}
