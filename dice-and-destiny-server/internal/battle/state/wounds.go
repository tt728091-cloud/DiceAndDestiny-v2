package state

import (
	"fmt"
	"slices"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
)

// Wound is one committed damage source, not an attack preview or a removed-pile
// summary. Card instance IDs are retained so future recovery can address exactly
// these losses without conflating repeated hits or copies of the same card.
type Wound struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batch_id"`
	Round           int             `json:"round"`
	Segment         segment.Segment `json:"segment"`
	SourceID        string          `json:"source_id,omitempty"`
	SourceActorID   string          `json:"source_actor_id,omitempty"`
	SourceContentID string          `json:"source_content_id,omitempty"`
	TargetActorID   string          `json:"target_actor_id"`
	Cards           []WoundCard     `json:"cards"`
}

type WoundCard struct {
	CardID           string             `json:"card_id"`
	CardDefinitionID string             `json:"card_definition_id,omitempty"`
	OriginalZone     operation.CardZone `json:"original_zone"`
}

func CloneWounds(wounds []Wound) []Wound {
	result := slices.Clone(wounds)
	for i := range result {
		result[i].Cards = slices.Clone(wounds[i].Cards)
	}
	return result
}

// RecordDamageWounds must only run after successful damage commitment. Unified
// reservations retain their exact source. Older pooled damage (including Effects)
// is apportioned in source order, up to each source's final damage, using only
// the cards actually removed. Overkill and prevented/released cards never count.
func (battle *Battle) RecordDamageWounds(batchID string, sources []SettledDamageSource, removals []ProposedCardRemoval) {
	for _, wound := range battle.Wounds {
		if wound.BatchID == batchID {
			return
		}
	}
	wounds := make([]Wound, len(sources))
	for i, source := range sources {
		wounds[i] = Wound{BatchID: batchID, Round: battle.Segment.Round, Segment: battle.Segment.Current,
			SourceID: source.ID, SourceActorID: source.SourceActorID, SourceContentID: source.SourceContentID, TargetActorID: source.TargetActorID}
	}
	used := make([]bool, len(removals))
	add := func(i, j int) {
		r := removals[j]
		definition := r.CardDefinitionID
		if definition == "" && battle.Settled != nil {
			definition = battle.Settled.Actors[r.TargetActorID].CardInstances[r.CardID].DefinitionID
		}
		wounds[i].Cards = append(wounds[i].Cards, WoundCard{CardID: r.CardID, CardDefinitionID: definition, OriginalZone: r.OriginalZone})
		used[j] = true
	}
	for j, r := range removals {
		if !r.Accepted || r.Released || len(r.DamageProposalIDs) != 1 {
			continue
		}
		for i, s := range sources {
			if s.ID == r.DamageProposalIDs[0] && s.TargetActorID == r.TargetActorID {
				add(i, j)
				break
			}
		}
	}
	for i, s := range sources {
		for j, r := range removals {
			if len(wounds[i].Cards) >= s.FinalAmount {
				break
			}
			if used[j] || !r.Accepted || r.Released || r.TargetActorID != s.TargetActorID {
				continue
			}
			if len(r.DamageProposalIDs) > 0 && !slices.Contains(r.DamageProposalIDs, s.ID) {
				continue
			}
			add(i, j)
		}
	}
	for _, wound := range wounds {
		if len(wound.Cards) == 0 {
			continue
		}
		wound.ID = fmt.Sprintf("%s:%s:wound-%d", battle.ID, batchID, len(battle.Wounds)+1)
		battle.Wounds = append(battle.Wounds, wound)
	}
}

// HealWoundCard records that a removed card returned to play. The card leaves
// the wound that removed it, so that wound shrinks by one health; a wound with
// no remaining cards is fully healed and disappears. Removals recorded before
// wound tracking have no entry, so nothing changes for them.
func (battle *Battle) HealWoundCard(actorID, cardID string) {
	for i := range battle.Wounds {
		w := &battle.Wounds[i]
		if w.TargetActorID != actorID {
			continue
		}
		for j, c := range w.Cards {
			if c.CardID != cardID {
				continue
			}
			w.Cards = slices.Delete(slices.Clone(w.Cards), j, j+1)
			if len(w.Cards) == 0 {
				battle.Wounds = slices.Delete(slices.Clone(battle.Wounds), i, i+1)
			}
			return
		}
	}
}
