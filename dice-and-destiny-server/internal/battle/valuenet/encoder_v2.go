package valuenet

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"diceanddestiny/server/internal/battle/lookahead"
)

// SchemaNameV2 names the value-v2 inputs: value-v1 plus the decision point,
// so any decision (not just a round's start) can be judged.
const SchemaNameV2 = "value-v2"

var (
	segments = []string{"offensive", "defensive"}
	stages   = []string{"planning", "defense_selection", "defense_reaction", "damage_reaction", "discard_to_hand_limit"}
)

// point describes the moment of a decision from the seat's view: where in
// the round it is, the seat's own dice and the solver's view of them, the
// ability it has selected, and the attacks in flight.
type point struct {
	segment                                 [3]float64
	stage                                   [6]float64
	rolling, rollsLeft, rollsUsed           float64
	best, stop, miss, gain                  float64
	parts                                   [5]float64
	selected, selectedValue                 float64
	incoming, incomingBase, incomingPrevent float64
	incomingCount, incomingStatus           float64
	outgoing, outgoingCount, outgoingStatus float64
	threatened, defended                    float64
}

func (p *point) emit(put func(name string, v float64)) {
	for i, name := range append(append([]string(nil), segments...), "other") {
		put("point.segment."+name, p.segment[i])
	}
	for i, name := range append(append([]string(nil), stages...), "other") {
		put("point.stage."+name, p.stage[i])
	}
	for _, field := range []struct {
		name string
		v    float64
	}{
		{"rolling", p.rolling}, {"rolls_left", p.rollsLeft}, {"rolls_used", p.rollsUsed},
		{"dice_best", p.best}, {"dice_stop", p.stop}, {"dice_miss", p.miss}, {"dice_gain", p.gain},
		{"dice_parts.damage", p.parts[1]}, {"dice_parts.self_status", p.parts[2]}, {"dice_parts.enemy_status", p.parts[3]}, {"dice_parts.energy", p.parts[4]},
		{"selected", p.selected}, {"selected_value", p.selectedValue},
		{"incoming", p.incoming}, {"incoming_base", p.incomingBase}, {"incoming_prevented", p.incomingPrevent},
		{"incoming_count", p.incomingCount}, {"incoming_status", p.incomingStatus},
		{"outgoing", p.outgoing}, {"outgoing_count", p.outgoingCount}, {"outgoing_status", p.outgoingStatus},
		{"threatened", p.threatened}, {"defended", p.defended},
	} {
		put("point."+field.name, field.v)
	}
}

var schemaFeaturesV2, schemaHashV2 = func() ([]string, string) {
	names := Features()
	(&point{}).emit(func(name string, _ float64) { names = append(names, name) })
	h := sha256.New()
	fmt.Fprintf(h, "%s\nrates %s card-value %g\n%s\n", SchemaNameV2, encoderRates, encoderCardValue, strings.Join(names, "\n"))
	return names, hex.EncodeToString(h.Sum(nil))
}()

// FeaturesV2 lists the value-v2 inputs by name, in order.
func FeaturesV2() []string { return append([]string(nil), schemaFeaturesV2...) }

// WidthV2 is the number of value-v2 inputs.
func WidthV2() int { return len(schemaFeaturesV2) }

// SchemaHashV2 pins the value-v2 inputs.
func SchemaHashV2() string { return schemaHashV2 }

// EncodeV2 writes the position's WidthV2() inputs into out.
func (e *Encoder) EncodeV2(pos Position, out []float32) error {
	if len(out) != WidthV2() {
		return fmt.Errorf("value-v2 needs %d inputs, got room for %d", WidthV2(), len(out))
	}
	if err := e.Encode(pos, out[:Width()]); err != nil {
		return err
	}
	p := e.point(pos)
	i := Width()
	p.emit(func(_ string, v float64) {
		out[i] = float32(v)
		i++
	})
	return nil
}

// plan is the solver's plan of the seat's own dice showing, or nil.
func (e *Encoder) plan(pos Position) *planView {
	a := pos.View.Actors[pos.Seat]
	if a.Dice == nil || a.Dice.Pool != "offensive" || len(a.Dice.Dice) == 0 || a.Dice.Complete {
		return nil
	}
	lib := pos.Library
	cards := handCards(pos)
	plan := e.solver.PlanTurn(lib, pos.turn(pos.Seat, cards), a.Dice.Dice, a.Dice.RollsRemaining, nil)
	if plan == nil {
		return nil
	}
	return &planView{plan: plan, best: plan.Best(), miss: plan.Miss(), parts: plan.Parts()}
}

func (e *Encoder) point(pos Position) point {
	var p point
	view := pos.View
	switch k := indexOf(segments, string(view.Segment)); {
	case k >= 0:
		p.segment[k] = 1
	default:
		p.segment[len(segments)] = 1
	}
	switch k := indexOf(stages, view.Stage); {
	case k >= 0:
		p.stage[k] = 1
	default:
		p.stage[len(stages)] = 1
	}
	a := view.Actors[pos.Seat]
	if a.Dice != nil && a.Dice.Pool == "offensive" && len(a.Dice.Dice) > 0 {
		p.rolling = 1
		p.rollsLeft, p.rollsUsed = float64(a.Dice.RollsRemaining)/3, float64(a.Dice.RollsUsed)/3
	}
	if plan := e.plan(pos); plan != nil {
		p.best, p.stop, p.miss = plan.best/10, plan.plan.Stop/10, plan.miss
		p.gain = (plan.best - plan.plan.Stop) / 5
		p.parts = [5]float64{plan.parts[0], plan.parts[1] / 10, plan.parts[2] / 3, plan.parts[3] / 3, plan.parts[4] / 10}
	}
	if a.SelectedAbility != "" {
		p.selected = 1
		if def, ok := pos.Library.Abilities[a.SelectedAbility]; ok && def.Qualification != nil {
			for i, tier := range def.Qualification.ActivationTiers {
				if tier.ID == a.SelectedTier || (a.SelectedTier == "" && i == 0) {
					p.selectedValue = (encoderRates.TierValue(tier.Operations) + damageBonus(a.AbilityModifiers, a.SelectedAbility)) / 10
				}
			}
		}
	}
	for _, source := range view.SettledSources {
		stacks := 0
		for _, s := range source.StatusApplications {
			if !s.Applied {
				stacks += s.Stacks
			}
		}
		switch pos.Seat {
		case source.TargetActorID:
			p.incoming += float64(source.FinalAmount)
			p.incomingBase += float64(source.BaseAmount)
			p.incomingPrevent += float64(source.Prevention + source.ReactionPrevention)
			p.incomingCount++
			p.incomingStatus += float64(stacks)
		case source.SourceActorID:
			p.outgoing += float64(source.FinalAmount)
			p.outgoingCount++
			p.outgoingStatus += float64(stacks)
		}
	}
	p.incoming, p.incomingBase, p.incomingPrevent = p.incoming/10, p.incomingBase/10, p.incomingPrevent/10
	p.incomingCount, p.incomingStatus = p.incomingCount/3, p.incomingStatus/5
	p.outgoing, p.outgoingCount, p.outgoingStatus = p.outgoing/10, p.outgoingCount/3, p.outgoingStatus/5
	if view.Damage != nil {
		for _, card := range view.Damage.RevealedCards {
			if card.TargetActorID == pos.Seat && !card.Released && !card.ProtectedFromSource {
				p.threatened++
			}
		}
	}
	p.threatened /= 10
	for _, defense := range view.SettledDefenses {
		if defense.ActorID == pos.Seat {
			p.defended = 1
		}
	}
	return p
}

// planView is the solver's plan of the seat's dice showing, with its value
// and miss chance from here.
type planView struct {
	plan       *lookahead.DicePlan
	best, miss float64
	parts      lookahead.Parts
}

// handCards counts the seat's own cards in hand that can still be played
// this round.
func handCards(pos Position) map[string]int {
	own := pos.View.Actors[pos.Seat]
	return lookahead.HandCards(pos.Library, own.Hand, own.CardInstances, own.CardUses, pos.View.Round)
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}
