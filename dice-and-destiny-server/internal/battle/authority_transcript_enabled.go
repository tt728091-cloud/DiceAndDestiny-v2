//go:build transcript_tools

package battle

import (
	"fmt"
	"sync"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/transcript"
)

type authorityTranscriptBinding struct {
	mu       sync.Mutex
	recorder *transcript.Recorder
	context  TranscriptBattleContext
}

func newAuthorityTranscript() any {
	recorder := transcript.FromEnvironment()
	if recorder == nil {
		return nil
	}
	return &authorityTranscriptBinding{recorder: recorder}
}

func configureAuthorityTranscript(authority *Authority, context TranscriptBattleContext) {
	binding := transcriptBinding(authority)
	if binding == nil {
		return
	}
	binding.mu.Lock()
	binding.context = cloneTranscriptBattleContext(context)
	binding.mu.Unlock()
}

func captureAuthorityTranscriptBefore(authority *Authority, battle *state.Battle) any {
	if transcriptBinding(authority) == nil || battle == nil {
		return nil
	}
	cloned := battle.Clone()
	return &cloned
}

func beginAuthorityTranscriptCommand(authority *Authority, cmd command.Command, context TranscriptCommandContext, battle *state.Battle) {
	binding := transcriptBinding(authority)
	if binding == nil {
		return
	}
	binding.recorder.Begin(cmd, commandTranscriptContext(context), binding.battleContext(), battle)
}

func recordAuthorityTranscriptAccepted(
	authority *Authority,
	cmd command.Command,
	context TranscriptCommandContext,
	before any,
	after *state.Battle,
	events []event.Event,
) {
	binding := transcriptBinding(authority)
	if binding == nil {
		return
	}
	binding.recorder.Accepted(transcript.Transition{
		Command: cmd,
		Context: commandTranscriptContext(context),
		Battle:  binding.battleContext(),
		Before:  battlePointer(before),
		After:   after,
		Events:  events,
	})
}

func recordAuthorityTranscriptRejected(authority *Authority, cmd command.Command, context TranscriptCommandContext, before any, message string) {
	binding := transcriptBinding(authority)
	if binding == nil {
		return
	}
	binding.recorder.Rejected(transcript.Transition{
		Command: cmd,
		Context: commandTranscriptContext(context),
		Battle:  binding.battleContext(),
		Before:  battlePointer(before),
		Error:   message,
	})
}

func recordAuthorityTranscriptTransportRejection(raw string, err error) {
	if recorder := transcript.FromEnvironment(); recorder != nil {
		recorder.TransportRejected(raw, err)
	}
}

func RecordAuthorityTranscriptDiagnostic(battleID, actorID, kind string, details map[string]any) {
	recorder := transcript.FromEnvironment()
	if recorder == nil {
		return
	}
	summary := fmt.Sprintf("%s: %s", actorID, kind)
	if configured, ok := details["summary"].(string); ok && configured != "" {
		summary = configured
	}
	recorder.Diagnostic(transcript.Diagnostic{
		BattleID:   battleID,
		ActorID:    actorID,
		Kind:       kind,
		Summary:    summary,
		Details:    details,
		Controller: stringValueOr(details, "controller", "authority"),
	})
}

func transcriptBinding(authority *Authority) *authorityTranscriptBinding {
	if authority == nil {
		return nil
	}
	binding, _ := authority.transcript.(*authorityTranscriptBinding)
	return binding
}

func (binding *authorityTranscriptBinding) battleContext() transcript.BattleContext {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	return transcript.BattleContext{
		Mode:         binding.context.Mode,
		HumanActorID: binding.context.HumanActorID,
		ModelActorID: binding.context.ModelActorID,
		Controllers:  cloneStringMap(binding.context.Controllers),
		Metadata:     cloneAnyMap(binding.context.Metadata),
	}
}

func commandTranscriptContext(value TranscriptCommandContext) transcript.CommandContext {
	return transcript.CommandContext{
		Controller:     value.Controller,
		ActionIndex:    value.ActionIndex,
		CandidateCount: value.CandidateCount,
		InferenceMS:    value.InferenceMS,
		Metadata:       cloneAnyMap(value.Metadata),
	}
}

func battlePointer(value any) *state.Battle {
	battle, _ := value.(*state.Battle)
	return battle
}

func cloneTranscriptBattleContext(value TranscriptBattleContext) TranscriptBattleContext {
	value.Controllers = cloneStringMap(value.Controllers)
	value.Metadata = cloneAnyMap(value.Metadata)
	return value
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneAnyMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func stringValueOr(values map[string]any, key, fallback string) string {
	if value, ok := values[key].(string); ok && value != "" {
		return value
	}
	return fallback
}
