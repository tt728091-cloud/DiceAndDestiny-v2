//go:build !transcript_tools

package battle

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

func newAuthorityTranscript() any { return nil }

func configureAuthorityTranscript(*Authority, TranscriptBattleContext) {}

func captureAuthorityTranscriptBefore(*Authority, *state.Battle) any { return nil }

func beginAuthorityTranscriptCommand(*Authority, command.Command, TranscriptCommandContext, *state.Battle) {
}

func recordAuthorityTranscriptAccepted(*Authority, command.Command, TranscriptCommandContext, any, *state.Battle, []event.Event) {
}

func recordAuthorityTranscriptRejected(*Authority, command.Command, TranscriptCommandContext, any, string) {
}

func recordAuthorityTranscriptTransportRejection(string, error) {}

// RecordAuthorityTranscriptDiagnostic is compiled to a no-op unless the
// transcript_tools build tag is present.
func RecordAuthorityTranscriptDiagnostic(string, string, string, map[string]any) {}
