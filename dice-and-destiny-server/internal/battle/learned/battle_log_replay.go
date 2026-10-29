package learned

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

// LoggedBattle is one finished battle as a session's diagnostics log
// recorded it: its setup and every accepted command in order.
type LoggedBattle struct {
	BattleTelemetry
	StartedAtUTC string
}

// ReadBattleLog returns the finished battles in a session diagnostics log,
// oldest first.
func ReadBattleLog(path string) ([]LoggedBattle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	var out []LoggedBattle
	for scanner.Scan() {
		var record struct {
			Kind    string          `json:"kind"`
			Payload BattleTelemetry `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.Kind != "battle_completed" {
			continue
		}
		out = append(out, LoggedBattle{BattleTelemetry: record.Payload, StartedAtUTC: record.Payload.StartedAtUTC})
	}
	return out, scanner.Err()
}

// ReplayStep is one logged command replayed through the authority, with the
// full battle state on both sides of it (for after-the-game review only).
type ReplayStep struct {
	Decision DecisionRecord
	Legal    []command.Command
	Before   state.Battle
	After    state.Battle
	Events   []event.Event
}

// ReplayLoggedBattle rebuilds a logged battle from its seed and commands with
// the same session setup (sandbox loadouts read from config.LoadoutRoot, so
// decks must be unchanged since the game) and calls visit for every step.
// It fails if any logged command is not legal when it is replayed.
func ReplayLoggedBattle(config SessionConfig, battle LoggedBattle, visit func(ReplayStep) error) (state.Battle, error) {
	s, err := NewSession(config)
	if err != nil {
		return state.Battle{}, err
	}
	if _, err := s.ResetCharacterLoadout(battle.BattleID, battle.Seed, battle.HumanSeat, false, battle.Character, true, "sandbox"); err != nil {
		return state.Battle{}, err
	}
	env := s.environment
	for i, decision := range battle.Commands {
		action := decision.Command
		if env.Current().ActorID != action.ActorID {
			if _, _, err := env.PreferActor(action.ActorID); err != nil {
				return state.Battle{}, err
			}
		}
		before, err := env.InspectBattle()
		if err != nil {
			return state.Battle{}, err
		}
		legal := append([]command.Command(nil), env.Current().Result.LegalActions...)
		_, result, err := env.StepCommandForViewer(action, action.ActorID)
		if err != nil {
			return state.Battle{}, fmt.Errorf("step %d (%s %s) does not replay: %w", i+1, action.ActorID, action.Type, err)
		}
		after, err := env.InspectBattle()
		if err != nil {
			return state.Battle{}, err
		}
		if err := visit(ReplayStep{Decision: decision, Legal: legal, Before: before, After: after, Events: result.Events}); err != nil {
			return state.Battle{}, err
		}
	}
	final, err := env.InspectBattle()
	if err != nil {
		return state.Battle{}, err
	}
	if got := humanResult(final.WinnerActorID, final.Status, battle.HumanSeat); battle.Result != "" && got != battle.Result {
		return final, fmt.Errorf("replay ended %s, the log says %s", got, battle.Result)
	}
	return final, nil
}
