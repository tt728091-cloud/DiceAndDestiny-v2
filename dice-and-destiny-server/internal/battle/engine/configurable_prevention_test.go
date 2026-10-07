package engine

import (
	"encoding/json"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Run real card plays and finalized defensive rolls, then flip only the pinned
// configuration. No identity-specific handler may determine the destination.
func TestConfiguredPreventionDestinations(t *testing.T) {
	for _, id := range []string{"brace", "brace_plus", "adventurer_guard", "adventurer_guard_plus"} {
		for _, flip := range []bool{false, true} {
			for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneHand, operation.ZoneDiscard} {
				name := id + "/" + string(zone)
				if flip {
					name += "/config_flipped"
				}
				t.Run(name, func(t *testing.T) {
					b, lib := adventurerFixture(t)
					card, isCard := lib.Cards[id]
					ability := lib.Abilities[id]
					destination := ability.SavedCardDestination
					if isCard {
						// Program prevention authors its destination on the effect.
						destination = content.ProgramString(card.Program.Steps[0], "destination")
					}
					if destination == "" {
						t.Fatal("test versions must explicitly author their destination")
					}
					if flip {
						if destination == content.SavedCardsDiscard {
							destination = content.SavedCardsOriginal
						} else {
							destination = content.SavedCardsDiscard
						}
						if isCard {
							card, _ = content.EditableGeneralCard(card)
							card.Program.Steps[0].Params["destination"] = destination
							lib.Cards[id] = card
						} else {
							ability.SavedCardDestination = destination
							lib.Abilities[id] = ability
						}
					}
					// Saves/catalog pinning must retain the authored option.
					raw, err := json.Marshal(lib)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &lib); err != nil {
						t.Fatal(err)
					}
					b.Settled.UnifiedDefense = true
					b.Segment.Current = segment.Defensive
					b.Settled.Stage = stageDefenseSelect
					a := b.Actors["player"]
					a.Cards = state.CardZones{Hand: []string{"payment"}}
					ids := []string{"nudge-0", "nudge-1", "strong_swing-0"}
					setZone(&a.Cards, zone, append(zoneCards(a.Cards, zone), ids...))
					b.Actors["player"] = a
					rt := b.Settled.Actors["player"]
					rt.CardInstances["payment"] = state.CardInstance{InstanceID: "payment", DefinitionID: id}
					b.Settled.Actors["player"] = rt
					source := state.SettledDamageSource{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "sword_cut", BaseAmount: 3, FinalAmount: 3}
					b.Settled.OffensiveSources = []state.SettledDamageSource{source}
					batch := &state.SettledDamageBatch{ID: "config", Sources: []state.SettledDamageSource{source}}
					for _, cardID := range ids {
						batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: cardID, CardID: cardID, TargetActorID: "player", OriginalZone: zone, Accepted: true, Revealed: true, DamageProposalIDs: []string{"hit"}})
					}
					b.Settled.PendingDamage = batch
					openSettledWindow(&b, "config", stageDefenseSelect, "defense_selection", []command.Type{command.TypeCommitInteraction, command.TypePlanningPass})
					e := NewEngine()
					if isCard {
						b.SettledCatalog, _ = json.Marshal(lib)
						if _, err := e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "start")); err != nil {
							t.Fatal(err)
						}
						if !containsString(b.Actors["player"].Cards.Discard, "payment") {
							t.Fatal("played card did not pay discard")
						}
					} else {
						b.Settled.DefenseSelections = map[string]state.SettledDefense{"player": {ActorID: "player", AbilityID: id, SourceID: "hit", RolledFaces: []int{1, 2, 3}}}
						if _, err := e.finalizeDefenses(&b, lib); err != nil {
							t.Fatal(err)
						}
					}
					want := zone
					if destination == content.SavedCardsDiscard {
						want = operation.ZoneDiscard
					}
					for _, r := range batch.Removals {
						if !r.Released || r.Accepted || r.ReleasedDestination != want || !containsString(zoneCards(b.Actors["player"].Cards, want), r.CardID) {
							t.Fatalf("configuration %s not respected: %+v", destination, r)
						}
					}
					if b.Actors["player"].CurrentHealth() != 4 || batch.Sources[0].FinalAmount != 0 {
						t.Fatal("prevention lost health or left damage")
					}
					// A later effect's different destination cannot relocate earlier saves.
					before, _ := json.Marshal(b.Actors["player"].Cards)
					if err := e.reconcilePreventionDestination(&b, content.SavedCardsDiscard); err != nil {
						t.Fatal(err)
					}
					after, _ := json.Marshal(b.Actors["player"].Cards)
					if string(before) != string(after) {
						t.Fatal("reconciliation moved earlier saved cards")
					}
				})
			}
		}
	}
}
