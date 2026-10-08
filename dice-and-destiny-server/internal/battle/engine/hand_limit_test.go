package engine

import (
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
)

// A single hand-limit discard names one card, the same shape as a card-program
// play. Every shipped card is a program card, so the discard must still reach
// the hand-limit handler rather than be rejected as a stale card choice.
func TestHandLimitDiscardsOneProgramCard(t *testing.T) {
	for _, discard := range []int{1, 2} {
		b, lib := adventurerFixture(t)
		b.Settled.Initialized = true
		b.Segment.Current = segment.DamageResolution
		hand := append([]string(nil), b.Actors["player"].Cards.Hand...)
		r := b.Settled.Actors["player"]
		r.HandLimit = len(hand) - discard
		b.Settled.Actors["player"] = r
		if lib.Cards[r.CardInstances[hand[0]].DefinitionID].Program == nil {
			t.Fatal("fixture hand must hold program cards")
		}
		b.Settled.Stage = stageHandLimit
		openSettledWindowForActors(&b, "hand-limit", stageHandLimit, "choose_card", []command.Type{command.TypeCommitInteraction}, []string{"player"}, false)

		eng := NewEngine()
		var chosen *command.Command
		for _, action := range eng.LegalActions(&b, "player") {
			var p command.CommitInteractionPayload
			if action.Type != command.TypeCommitInteraction || command.DecodePayload(action, &p) != nil || p.Commitment.ChoiceID != "" || len(p.Commitment.CardIDs) != discard {
				t.Fatalf("discard %d: hand limit offered a non-discard action: %s %s", discard, action.Type, action.Payload)
			}
			if p.Commitment.CardIDs[0] == hand[0] && chosen == nil {
				picked := action
				chosen = &picked
			}
		}
		if chosen == nil {
			t.Fatalf("discard %d: no discard offered for %s", discard, hand[0])
		}
		// Dispatch only: this bare fixture's enemy has no AI to plan the next round.
		if _, err := eng.handleSettledCommand(&b, *chosen); err != nil {
			t.Fatalf("discard %d: %v", discard, err)
		}
		player := b.Actors["player"]
		if len(player.Cards.Hand) != r.HandLimit || !containsString(player.Cards.Discard, hand[0]) || containsString(player.Cards.Hand, hand[0]) {
			t.Fatalf("discard %d: selected card was not discarded: %+v", discard, player.Cards)
		}
		if b.Settled.Stage == stageHandLimit {
			t.Fatalf("discard %d: hand-limit window stayed open", discard)
		}
	}
}
