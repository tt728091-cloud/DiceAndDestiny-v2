package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
)

// Effects is a rules-driven sequence, never a participant decision. Keep the
// existing resolution handlers (including Catalyst priority and damage order),
// but consume their internal checkpoints before exposing the next decision.
func (e Engine) progressAutomaticEffects(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	before := effectsPublicActors(b)
	round := b.Segment.Round
	var events []event.Event
	var steps []event.Event
	for n := 0; n < DefaultMaxAutomaticSteps; n++ {
		if b.Segment.Current != segment.OngoingEffects || state.IsTerminalBattleStatus(b.Status) {
			summary := event.Event{Type: event.Type("effects_resolved"), Segment: segment.OngoingEffects, Round: round, Data: map[string]any{"actors_before": before, "actors_after": effectsPublicActors(b), "steps": steps}}
			// Present the completed Effects sequence before the Income entry or defeat.
			at := len(events)
			for i, ev := range events {
				if ev.Type == event.TypeBattleCompleted || ev.Type == event.TypeSegmentAdvanced || (ev.Type == event.TypeSegmentEntered && ev.To != segment.OngoingEffects && ev.Segment != segment.OngoingEffects) {
					at = i
					break
				}
			}
			events = append(events, event.Event{})
			copy(events[at+1:], events[at:])
			events[at] = summary
			return events, nil
		}
		var next []event.Event
		var err error
		if b.Settled.Window == nil {
			next, err = e.progressSettledOngoing(b, lib)
		} else {
			ids := sortedSettledActorIDs(b)
			actor := ""
			for _, id := range ids {
				if _, ok := b.Flow.PendingInput[id]; ok {
					actor = id
					break
				}
			}
			if actor == "" {
				return nil, fmt.Errorf("automatic effects checkpoint has no actor: %s", b.Settled.Stage)
			}
			pending := b.Flow.PendingInput[actor]
			cmd := command.Command{BattleID: b.ID, ActorID: actor, Type: command.TypePass}
			cmd.Payload, _ = json.Marshal(command.PassPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending)})
			switch b.Settled.Window.Stage {
			case stageOngoingRoll:
				cmd.Type = command.TypeRollDice
				cmd.Payload, _ = json.Marshal(command.RollDicePayload{PendingInputID: pending.ID})
				next, err = e.handleStatusRollCommand(b, lib, cmd)
			case stageOngoingReact:
				next, err = e.handleStatusReactionCommand(b, lib, cmd)
			case stageOngoingDamage:
				next, err = e.handleDamageReactionCommand(b, lib, cmd)
			case stageVenomStatus:
				next, err = e.handleVenomStatus(b, lib, cmd)
			default:
				return nil, fmt.Errorf("unsupported automatic effects checkpoint %q", b.Settled.Window.Stage)
			}
			if err == nil && b.Settled.Venom != nil && b.Settled.Venom.Active == nil && len(b.Settled.Venom.Queue) > 0 && b.Segment.Current == segment.OngoingEffects {
				child, childErr := e.startVenomWork(b, lib, false)
				next = append(next, child...)
				err = childErr
			}
		}
		if err != nil {
			return nil, err
		}
		// Freeze event payloads now: collected roll slices can be changed by a later
		// Catalyst reroll. Animation must retain the original and final faces.
		frozen, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		var copied []event.Event
		if err = json.Unmarshal(frozen, &copied); err != nil {
			return nil, err
		}
		events = append(events, copied...)
		for _, ev := range copied {
			switch ev.Type {
			case event.TypeDiceRolled, event.TypeInteractionWindowOpened, event.TypeProposalBatchCommitted, event.TypeDamageCommitted:
				steps = append(steps, ev)
			}
		}
	}
	return nil, ErrAutomaticStepLimit
}

// Public profile values only; no hand/deck identities are included. Lost cards
// are already publicly revealed by the committed damage event.
func effectsPublicActors(b *state.Battle) map[string]any {
	out := map[string]any{}
	for _, id := range sortedSettledActorIDs(b) {
		a := b.Actors[id]
		out[id] = map[string]any{"statuses": append([]state.StatusState{}, a.Statuses...), "health": a.CurrentHealth(), "energy": a.Resources.EnergyPoints, "deck_count": len(a.Cards.Deck), "hand_count": len(a.Cards.Hand), "discard_count": len(a.Cards.Discard), "removed_count": len(a.Cards.Removed)}
	}
	return out
}
