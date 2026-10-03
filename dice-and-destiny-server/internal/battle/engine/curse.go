package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"errors"
	"fmt"
	"strconv"
)

const stageCurseChoice = "curse_choice"

func queueCurse(b *state.Battle, w state.CurseWork) {
	c := curseRuntime(b)
	c.Queue = append(c.Queue, w)
}
func (e Engine) startCurseWork(b *state.Battle, lib content.BattleLibrary, advance bool) ([]event.Event, error) {
	c := curseRuntime(b)
	if c.Active != nil || len(c.Queue) == 0 {
		return nil, nil
	}
	c.Resume = &state.VenomResume{Stage: b.Settled.Stage, Window: b.Settled.Window, Flow: b.Flow, Trigger: b.Settled.TriggerBatch, Damage: b.Settled.PendingDamage, Advance: advance}
	b.Settled.Window = nil
	b.Flow = state.NewSegmentFlowState(b.Segment)
	b.Flow.Entered = true
	return e.nextCurseWork(b, lib)
}
func (e Engine) nextCurseWork(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	c := curseRuntime(b)
	for len(c.Queue) > 0 {
		w := c.Queue[0]
		c.Queue = c.Queue[1:]
		if b.Actors[w.Target].DefeatState == state.ActorDefeated || b.Actors[w.Target].CurrentHealth() == 0 {
			continue
		}
		c.Active = &w
		switch w.Kind {
		case "grasp":
			if err := e.applyCurse(b, lib, w.Target, w.CardID, 2); err != nil {
				return nil, err
			}
			if len(entombedIndices(b, w.Target)) < 2 {
				for _, i := range curseDice(b, w.Target, "unbound") {
					w.Options = append(w.Options, fmt.Sprint(i))
				}
			}
		case "eclipse":
			for face := 1; face <= 6; face++ {
				for _, d := range c.Dice[w.Target] {
					if !containsInt(d.CursedFaces, face) {
						w.Options = append(w.Options, fmt.Sprint(face))
						break
					}
				}
			}
			if len(w.Options) == 0 {
				if _, err := e.rollOwnedSubset(b, lib, w.Target, w.CardID, allOwned(b, w.Target), 5); err != nil {
					return nil, err
				}
			}
		case "adjacent":
			d, err := e.ownedRoll(b, lib, w.Target, w.Die, "curse_dice", true)
			if err != nil {
				return nil, err
			}
			w.Face = d.Face
			for _, face := range []int{d.Face - 1, d.Face + 1} {
				if face >= 1 && face <= 6 && !containsInt(c.Dice[w.Target][w.Die].CursedFaces, face) {
					w.Options = append(w.Options, fmt.Sprint(face))
				}
			}
		case "repaid":
			for _, i := range allOwned(b, w.Target) {
				w.Options = append(w.Options, fmt.Sprint(i))
			}
		case "tomb":
			w.Options = []string{"count", "roll"}
		case "misfortune":
			w.Options = []string{"damage"}
			if stacks(b, w.Target, "cursed_entangle") == 0 {
				w.Options = append(w.Options, "entangle")
			}
		default:
			return nil, fmt.Errorf("unknown Curse work %q", w.Kind)
		}
		if len(w.Options) == 0 {
			continue
		}
		c.Active = &w
		chooser := w.Source
		if w.Kind == "tomb" || w.Kind == "misfortune" {
			chooser = w.Target
		}
		if b.Actors[chooser].Controller == state.ControllerAI {
			if err := e.resolveCurseChoice(b, lib, w.Options[0]); err != nil {
				return nil, err
			}
			continue
		}
		openSettledWindowForActors(b, "curse-choice", stageCurseChoice, "choose_option", []command.Type{command.TypeCommitInteraction}, []string{chooser}, false)
		return []event.Event{settledEvent(event.TypeInteractionWindowOpened, b, chooser, map[string]any{"curse_choice": w})}, nil
	}
	r := c.Resume
	c.Active = nil
	c.Resume = nil
	if r == nil {
		return nil, nil
	}
	b.Settled.Stage = r.Stage
	b.Settled.Window = r.Window
	b.Flow = r.Flow
	b.Settled.TriggerBatch = r.Trigger
	b.Settled.PendingDamage = r.Damage
	if r.Advance {
		return e.advanceSettledSegment(b)
	}
	return nil, nil
}
func (e Engine) handleCurseChoice(b *state.Battle, lib content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	var p command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &p); err != nil {
		return nil, err
	}
	w := curseRuntime(b).Active
	if w == nil || !containsString(w.Options, p.Commitment.ChoiceID) || len(p.Commitment.CardIDs) > 0 {
		return nil, errors.New("invalid Curse choice")
	}
	if err := e.resolveCurseChoice(b, lib, p.Commitment.ChoiceID); err != nil {
		return nil, err
	}
	closeSettledWindow(b)
	return e.nextCurseWork(b, lib)
}
func (e Engine) resolveCurseChoice(b *state.Battle, lib content.BattleLibrary, key string) error {
	w := *curseRuntime(b).Active
	if w.Kind == "repaid" {
		defer tagDefenseCurseEvents(b, len(curseRuntime(b).Logs), w.Source, w.CardID, w.DefenseSourceID)
	}
	n, _ := strconv.Atoi(key)
	curseLog(b, w.Source, "choice", map[string]any{"card_id": w.CardID, "choice": key, "target": w.Target})
	switch w.Kind {
	case "grasp":
		entomb(b, w.Target, n)
	case "eclipse":
		for _, i := range allOwned(b, w.Target) {
			markCurse(b, w.Target, i, n)
		}
	case "adjacent":
		markCurse(b, w.Target, w.Die, n)
	case "repaid":
		if len(curseRuntime(b).Dice[w.Target][n].CursedFaces) == 0 {
			markCurse(b, w.Target, n, 1)
		} else {
			return e.lesserCurse(b, lib, w.Target, w.CardID, n)
		}
	case "tomb":
		if key == "count" {
			applyStatus(b, lib, w.Target, "curse_count", 3)
		} else {
			_, err := e.rollOwnedSubset(b, lib, w.Target, w.CardID, allOwned(b, w.Target), 5)
			return err
		}
	case "misfortune":
		if key == "entangle" {
			applyStatus(b, lib, w.Target, "cursed_entangle", 1)
		} else {
			v := venomRuntime(b)
			v.Queue = append(v.Queue, state.VenomWork{Kind: "damage", SourceActorID: w.Source, TargetActorID: w.Target, SourceContentID: w.CardID, Damage: 2})
		}
	}
	return nil
}
func (e Engine) queueCurseAbilities(b *state.Battle, lib content.BattleLibrary) error {
	c := curseRuntime(b)
	if c.AfterAttacksRound == b.Segment.Round {
		return nil
	}
	c.AfterAttacksRound = b.Segment.Round
	for _, id := range sortedSettledActorIDs(b) {
		rt := b.Settled.Actors[id]
		for _, target := range rt.SelectedTargetIDs {
			switch rt.SelectedAbilityID {
			case "grasp_of_the_sarcophagus":
				queueCurse(b, state.CurseWork{Kind: "grasp", Source: id, Target: target, CardID: rt.SelectedAbilityID})
			case "eclipse_of_the_black_star":
				queueCurse(b, state.CurseWork{Kind: "eclipse", Source: id, Target: target, CardID: rt.SelectedAbilityID})
			}
		}
	}
	return nil
}
func (e Engine) curseDefenseCompleted(b *state.Battle, lib content.BattleLibrary, d state.SettledDefense) error {
	defer tagDefenseCurseEvents(b, len(curseRuntime(b).Logs), d.ActorID, d.AbilityID, d.SourceID)
	s := sourceBySettledID(b, d.SourceID)
	if s == nil {
		return nil
	}
	switch d.AbilityID {
	case "hexward_rebuttal":
		return e.applyCurse(b, lib, s.SourceActorID, d.SourceID, 1)
	case "misfortune_repaid":
		prevention := 0
		omen := false
		for _, f := range defenseFaces(d) {
			if f <= 3 {
				prevention++
			} else if f <= 5 {
				prevention += 2
			} else {
				omen = true
			}
		}
		if omen {
			applyStatus(b, lib, s.SourceActorID, "curse_count", 2)
		}
		if prevention > 0 && s.BaseAmount > max(0, s.Prevention-prevention) {
			queueCurse(b, state.CurseWork{Kind: "repaid", Source: d.ActorID, Target: s.SourceActorID, CardID: d.AbilityID, DefenseSourceID: d.SourceID})
		}
	}
	return nil
}
func actualCurseDamage(batch *state.SettledDamageBatch, source string) int {
	n := 0
	for _, r := range batch.Removals {
		if r.Accepted && !r.Released && containsString(r.DamageProposalIDs, source) {
			n++
		}
	}
	return n
}
func (e Engine) curseDamageCompleted(b *state.Battle, lib content.BattleLibrary, batch *state.SettledDamageBatch) error {
	if b.Settled.Curse == nil {
		return nil
	}
	c := curseRuntime(b)
	for _, s := range batch.Sources {
		if b.Actors[s.TargetActorID].CurrentHealth() == 0 {
			continue
		}
		switch s.SourceContentID {
		case "hexbrand":
			n := 1
			switch b.Settled.Actors[s.SourceActorID].SelectedTierID {
			case "skull_4":
				n = 2
			case "skull_5":
				n = 3
			}
			start := len(c.Logs)
			if err := e.applyCurse(b, lib, s.TargetActorID, s.ID, n); err != nil {
				return err
			}
			for _, log := range c.Logs[start:] {
				log.Data["source_actor_id"] = s.SourceActorID
				log.Data["source_ability_id"] = s.SourceContentID
				log.Data["attack_source_id"] = s.ID
			}
		case "funeral_rattle":
			eligible := curseDice(b, s.TargetActorID, "cursed")
			if len(eligible) > 0 {
				if _, err := e.rollOwnedSubset(b, lib, s.TargetActorID, s.ID, eligible, min(3, len(eligible))); err != nil {
					return err
				}
			}
			if stacks(b, s.TargetActorID, "curse_count") >= 6 {
				applyStatus(b, lib, s.TargetActorID, "blind", 1)
			}
		}
		if s.SourceContentID == "curse_count" && stacks(b, s.TargetActorID, "curse_bloom") > 0 {
			removeStatus(b, s.TargetActorID, "curse_bloom", 0)
			damage := actualCurseDamage(batch, s.ID)
			attempts := 0
			if damage > 0 && len(curseDice(b, s.TargetActorID, "cursed")) > 0 {
				attempts = 3
			}
			start := len(c.Logs)
			curseLog(b, s.TargetActorID, "bloom_trigger", map[string]any{"damage": damage, "attempts": attempts})
			for n := 0; n < attempts; n++ {
				if err := e.lesserCurse(b, lib, s.TargetActorID, s.ID, -1); err != nil {
					return err
				}
			}
			for _, log := range c.Logs[start:] {
				log.Data["source_card_id"] = "curse_bloom"
				log.Data["source_actor_id"] = s.SourceActorID
				log.Data["attack_source_id"] = s.ID
			}
		}
		// Consume before rolling: a secondary Curse result cannot reuse this preparation.
		pending := append([]state.CursePreparation(nil), c.Preparations...)
		for _, p := range pending {
			switch p.CardID {
			case "black_fingerprint":
				if p.Source != s.SourceActorID || p.Target != s.TargetActorID || lib.Abilities[s.SourceContentID].Type != "offensive" {
					continue
				}
				removeCursePreparation(b, p)
				n := actualCurseDamage(batch, s.ID)
				if n >= 1 && n <= 6 {
					eligible := missingCurseFace(b, p.Target, n, false)
					if len(eligible) > 0 {
						i, err := e.namedIntn(b, "curse_target", len(eligible))
						if err != nil {
							return err
						}
						markCurse(b, p.Target, eligible[i], n)
					} else {
						if _, err := e.rollOwnedSubset(b, lib, p.Target, s.ID, allOwned(b, p.Target), 1); err != nil {
							return err
						}
					}
				}
			case "curse_bloom":
				if s.SourceContentID != "curse_count" || p.Target != s.TargetActorID {
					continue
				}
				removeCursePreparation(b, p)
				if actualCurseDamage(batch, s.ID) > 0 {
					for n := 0; n < 3; n++ {
						if err := e.lesserCurse(b, lib, p.Target, s.ID, -1); err != nil {
							return err
						}
					}
				}
			case "spiteful_ward":
				if p.SourceID != s.ID {
					continue
				}
				removeCursePreparation(b, p)
				base := max(0, s.BaseAmount-s.Prevention)
				if s.ScaleDenominator > 0 {
					base = base * s.ScaleNumerator / s.ScaleDenominator
				}
				if base > p.Die {
					start := len(c.Logs)
					if err := e.applyCurse(b, lib, p.Target, s.ID, 1); err != nil {
						return err
					}
					for _, log := range c.Logs[start:] {
						log.Data["source_actor_id"] = p.Source
						log.Data["source_card_id"] = p.CardID
						log.Data["attack_source_id"] = s.ID
					}
				}
			}
		}
	}
	return nil
}
func (e Engine) convertCurse(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	c := curseRuntime(b)
	completion, err := evaluateBattleCompletion(b)
	if err != nil {
		return nil, err
	}
	if state.IsTerminalBattleStatus(b.Status) {
		return completion, nil
	}
	c.ConversionRound = b.Segment.Round
	var sources []state.SettledDamageSource
	// Snapshot both totals before any conversion damage or Bloom rolls occur.
	for _, actor := range sortedSettledActorIDs(b) {
		if b.Actors[actor].DefeatState == state.ActorDefeated {
			continue
		}
		count := stacks(b, actor, "curse_count")
		groups := count / 3
		mult := 1
		if stacks(b, actor, "three_knocks_status") > 0 {
			mult = 2
		}
		removeStatus(b, actor, "three_knocks_status", 0)
		mode := ""
		if stacks(b, actor, "grave_interest") > 0 {
			mode = "grave_interest"
			removeStatus(b, actor, "grave_interest", 0)
		}
		for _, p := range append([]state.CursePreparation(nil), c.Preparations...) {
			if p.Target == actor && isCurseMode(p.CardID) {
				mode = p.CardID
				removeCursePreparation(b, p)
			}
		}
		if mode == "stored_calamity" {
			c.LastStored[actor] = b.Segment.Round
			continue
		}
		if groups > 0 {
			removeStatus(b, actor, "curse_count", groups*3)
		}
		if groups > 0 && (mode == "grave_interest" || mode == "black_tax") {
			groups--
			if mode == "grave_interest" {
				if _, ok := lib.Statuses["grave_debt"]; ok {
					applyStatus(b, lib, actor, "grave_debt", 1)
				} else {
					c.TaxEnergy[actor]++
				}
			} else {
				c.TaxCards[actor]++
			}
		}
		if mode == "grave_interest" {
			curseLog(b, actor, "grave_interest_trigger", map[string]any{"triggered": count >= 3, "count_spent": min(count/3, 1) * 3, "count_before": count, "debt": stacks(b, actor, "grave_debt")})
		}
		curseLog(b, actor, "conversion", map[string]any{"count_before": count, "damage": groups * mult, "mode": mode, "count_after": stacks(b, actor, "curse_count")})
		if groups > 0 {
			sources = append(sources, newSettledDamageSource(b, enemyOf(b, actor), actor, "curse_count", groups*mult))
		}
	}
	if len(sources) == 0 {
		return e.advanceSettledSegment(b)
	}
	batch, err := e.buildDamageBatch(b, sources)
	if err != nil {
		return nil, err
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(b, "curse-conversion", stageOngoingDamage, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	return []event.Event{damageRevealEvent(b, batch)}, nil
}
func (e Engine) cursedEntangleEntry(b *state.Battle, lib content.BattleLibrary) error {
	for _, actor := range sortedSettledActorIDs(b) {
		if stacks(b, actor, "cursed_entangle") == 0 {
			continue
		}
		removeStatus(b, actor, "cursed_entangle", 0)
		removeStatus(b, actor, "entangle", 0)
		rt := b.Settled.Actors[actor]
		rt.MaxRolls = max(1, rt.MaxRolls-1)
		b.Settled.Actors[actor] = rt
		if _, err := e.rollOwnedSubset(b, lib, actor, fmt.Sprintf("entangle:%d", b.Segment.Round), allOwned(b, actor), 5); err != nil {
			return err
		}
	}
	return nil
}
func isCurseMode(id string) bool {
	return id == "grave_interest" || id == "black_tax" || id == "stored_calamity"
}
func removeCursePreparation(b *state.Battle, p state.CursePreparation) {
	c := curseRuntime(b)
	for i, v := range c.Preparations {
		if v.CardID == p.CardID && v.Source == p.Source && v.Target == p.Target && v.Round == p.Round && v.SourceID == p.SourceID {
			c.Preparations = append(c.Preparations[:i], c.Preparations[i+1:]...)
			return
		}
	}
}
func expireCursePreparations(b *state.Battle) {
	c := curseRuntime(b)
	damageExit := b.Segment.Current == segment.DamageResolution || (b.Settled.UnifiedDefense && b.Segment.Current == segment.Defensive && b.Settled.Stage == "complete")
	if b.Segment.Current == segment.OngoingEffects {
		expireBlackDividends(b)
		for _, actor := range sortedSettledActorIDs(b) {
			if stacks(b, actor, "curse_bloom") > 0 {
				removeStatus(b, actor, "curse_bloom", 0)
				curseLog(b, actor, "bloom_expired", nil)
			}
			if stacks(b, actor, "second_knell") > 0 {
				removeStatus(b, actor, "second_knell", 0)
				curseLog(b, actor, "second_knell_expired", nil)
			}
		}
	}
	if b.Segment.Current == segment.Income {
		for _, actor := range sortedSettledActorIDs(b) {
			if stacks(b, actor, "maledictions_refusal") > 0 {
				removeStatus(b, actor, "maledictions_refusal", 0)
				curseLog(b, actor, "refusal_expired", nil)
			}
		}
	}
	for _, p := range append([]state.CursePreparation(nil), c.Preparations...) {
		if b.Segment.Current == segment.OngoingEffects && p.ExpiresEffects <= b.Segment.Round || damageExit && p.CardID == "black_fingerprint" && p.Round <= b.Segment.Round || b.Segment.Current == segment.Income && p.CardID == "maledictions_refusal" {
			removeCursePreparation(b, p)
		}
	}
	if damageExit {
		c.Bags = map[string][]int{}
	}
}
func (e Engine) cleanseCurseCount(b *state.Battle, lib content.BattleLibrary, actor string, n int) error {
	armed := stacks(b, actor, "maledictions_refusal") > 0
	// Existing battles retain their pinned catalog and legacy guard.
	for _, p := range append([]state.CursePreparation(nil), curseRuntime(b).Preparations...) {
		if p.CardID == "maledictions_refusal" && p.Target == actor {
			removeCursePreparation(b, p)
			armed = true
		}
	}
	if !armed {
		removeStatus(b, actor, "curse_count", n)
		return nil
	}
	removeStatus(b, actor, "maledictions_refusal", 0)
	before := stacks(b, actor, "curse_count")
	start := len(curseRuntime(b).Logs)
	dice, err := e.rollOwnedSubset(b, lib, actor, "maledictions_refusal", allOwned(b, actor), 1)
	if err != nil {
		return err
	}
	d := dice[0]
	blocked := containsInt(curseRuntime(b).Dice[actor][d.Index].CursedFaces, d.Face)
	// A Knell retry has its own detailed animation; otherwise Refusal owns the roll.
	retried := false
	for _, log := range curseRuntime(b).Logs[start:] {
		if log.Data["kind"] == "second_knell_trigger" {
			retried = true
		}
		if log.Data["kind"] == "owned_roll" {
			log.Data["refusal_part"] = true
		}
	}
	rolledCount := stacks(b, actor, "curse_count")
	if !blocked {
		removeStatus(b, actor, "curse_count", n)
	}
	curseLog(b, actor, "refusal_trigger", map[string]any{"die": d, "blocked": blocked, "count_before": before, "count_rolled": rolledCount, "count_after": stacks(b, actor, "curse_count"), "already_rolled": retried})
	return nil
}

func curseBonusKey(b *state.Battle, actor string) string {
	r := b.Settled.Actors[actor]
	return fmt.Sprintf("%d:%s:%s:%s", b.Segment.Round, actor, r.SelectedAbilityID, first(r.SelectedTargetIDs))
}

// Preserve the cause across the transition from defense review to damage.
// These fields only describe public events; they never change resolution.
func tagDefenseCurseEvents(b *state.Battle, start int, actor, ability, source string) {
	for _, log := range curseRuntime(b).Logs[start:] {
		log.Data["source_actor_id"] = actor
		log.Data["source_ability_id"] = ability
		log.Data["defense_source_id"] = source
	}
}
