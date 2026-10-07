package engine

import (
	"encoding/json"
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

type programChoice struct {
	Verb    string `json:"verb"`
	Label   string `json:"label"`
	Actor   string `json:"actor,omitempty"`
	Card    string `json:"card,omitempty"`
	Source  string `json:"source,omitempty"`
	Status  string `json:"status,omitempty"`
	Ability string `json:"ability,omitempty"`
	Die     int    `json:"die"`
	Face    int    `json:"face,omitempty"`
	Option  int    `json:"option,omitempty"`
}

func (c programChoice) key() string { b, _ := json.Marshal(c); return string(b) }

// A die may offer several face choices, but remains one target.
func programTargetKey(c programChoice) string {
	c.Face = 0
	c.Label = ""
	return c.key()
}
func programUniqueCount(cs []programChoice) int {
	seen := map[string]bool{}
	for _, c := range cs {
		seen[programTargetKey(c)] = true
	}
	return len(seen)
}
func programUnselected(cs []programChoice, selected []string) []programChoice {
	seen := map[string]bool{}
	for _, k := range selected {
		var c programChoice
		_ = json.Unmarshal([]byte(k), &c)
		seen[programTargetKey(c)] = true
	}
	out := []programChoice{}
	for _, c := range cs {
		if !seen[programTargetKey(c)] {
			out = append(out, c)
		}
	}
	return out
}
func programQueue(x *state.CardExecution) []content.CardStep {
	var steps []content.CardStep
	_ = json.Unmarshal(x.Steps, &steps)
	return steps
}
func programSetQueue(x *state.CardExecution, s []content.CardStep) {
	x.Steps, _ = json.Marshal(s)
	x.Selected = nil
}

// programWindowOpen also resolves the before/after-roll windows. Offensive
// planning splits on the actor's offensive rolls; the Defense screen splits on
// whether the actor has rolled a defense this segment. Skipped attacks record
// no ability and do not count as a roll.
func programWindowOpen(b *state.Battle, actor string, windows []string) bool {
	w := programWindow(b)
	if content.ProgramContains(windows, w) {
		return true
	}
	switch w {
	case "offensive_planning":
		rolled := b.Settled.Actors[actor].RollsUsed > 0
		return content.ProgramContains(windows, "offensive_after_roll") && rolled || content.ProgramContains(windows, "offensive_before_roll") && !rolled
	case "defense_selection":
		rolled := defenseRolled(b, actor)
		return content.ProgramContains(windows, "defense_after_roll") && rolled || content.ProgramContains(windows, "defense_before_roll") && !rolled
	}
	return false
}
func defenseRolled(b *state.Battle, actor string) bool {
	for _, d := range b.Settled.DefenseHistory {
		if d.ActorID == actor && d.AbilityID != "" {
			return true
		}
	}
	return false
}
func programWindow(b *state.Battle) string {
	switch b.Settled.Stage {
	case stageOffensivePlan:
		return "offensive_planning"
	case stageOffensiveReact:
		return "offensive_reaction"
	case stageDefenseSelect:
		return "defense_selection"
	case stageDefenseReact:
		return "defense_reaction"
	case stageDamageReact:
		return "damage_reaction"
	}
	return ""
}
func programCardZone(b *state.Battle, actor, card string) operation.CardZone {
	for _, z := range []operation.CardZone{operation.ZoneHand, operation.ZoneDeck, operation.ZoneDiscard, operation.ZoneRemoved} {
		if containsString(zoneCards(b.Actors[actor].Cards, z), card) {
			return z
		}
	}
	return ""
}
func programCards(b *state.Battle, lib content.BattleLibrary, actor string) []string {
	rt := b.Settled.Actors[actor]
	if rt.CardExecution != nil {
		return []string{rt.CardExecution.CardID}
	}
	var ids []string
	for _, z := range []operation.CardZone{operation.ZoneHand, operation.ZoneDiscard, operation.ZoneDeck} {
		for _, id := range zoneCards(b.Actors[actor].Cards, z) {
			d := lib.Cards[rt.CardInstances[id].DefinitionID]
			p := d.Program
			if p == nil || !content.ProgramContains(d.Play.SourceZones, string(z)) || !programWindowOpen(b, actor, p.Windows) || b.Actors[actor].Resources.EnergyPoints < d.Cost.Energy {
				continue
			}
			// Legacy roll requirements count offensive rolls, so they only gate planning.
			if programWindow(b) == "offensive_planning" && (p.RollRequirement == "before_first" && rt.RollsUsed != 0 || p.RollRequirement == "after_first" && rt.RollsUsed == 0) {
				continue
			}
			if p.UsesPerBattle > 0 && rt.CardUses[d.ID] >= p.UsesPerBattle || p.UsesPerRound > 0 && rt.CardUses[fmt.Sprintf("%d:%s", b.Segment.Round, d.ID)] >= p.UsesPerRound {
				continue
			}
			var first *content.CardStep
			for i := range p.Steps {
				step := &p.Steps[i]
				if step.Condition == nil || step.Effect == "ability_bonus" || requirementsMet(*step.Condition, rt.FinalDice) {
					first = step
					break
				}
			}
			if first == nil {
				continue
			}
			count := programUniqueCount(programCandidates(b, lib, actor, id, *first, nil))
			if count == 0 || first.Target.Mode == "exact" && count < first.Target.Count {
				continue
			}
			ids = append(ids, id)
		}
	}
	return ids
}
func programCommand(b *state.Battle, actor, id string, p state.PendingInput, c programChoice) command.Command {
	if b.Settled.Stage == stageOffensivePlan {
		return legalCommand(b.ID, actor, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: p.ID, Checkpoint: planningCheckpoint(p), CardIDs: []string{id}, StatusID: c.key()})
	}
	return legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: p.ID, Checkpoint: interactionCheckpoint(p), Commitment: command.InteractionCommitmentData{CardIDs: []string{id}, ChoiceID: c.key()}})
}
func programActions(b *state.Battle, lib content.BattleLibrary, actor string, p state.PendingInput) []command.Command {
	var actions []command.Command
	for _, id := range programCards(b, lib, actor) {
		x := b.Settled.Actors[actor].CardExecution
		if x == nil {
			actions = append(actions, programCommand(b, actor, id, p, programChoice{Verb: "start", Label: "Play " + lib.Cards[b.Settled.Actors[actor].CardInstances[id].DefinitionID].Name}))
			continue
		}
		if !x.Paid {
			actions = append(actions, programCommand(b, actor, id, p, programChoice{Verb: "cancel", Label: "Cancel card"}))
		}
		queue := programQueue(x)
		if len(queue) == 0 {
			continue
		}
		s := queue[0]
		choices := programUnselected(programCandidates(b, lib, actor, id, s, x), x.Selected)
		for _, c := range choices {
			if !containsString(x.Selected, c.key()) {
				actions = append(actions, programCommand(b, actor, id, p, c))
			}
		}
		if s.Target.Mode == "up_to" || len(x.Selected) > 0 {
			n := 1
			if s.Target.Mode == "exact" || s.Target.Mode == "up_to" {
				n = s.Target.Count
			}
			if s.Target.Mode == "up_to" || (s.Target.Mode != "all" && len(x.Selected) == n) {
				actions = append(actions, programCommand(b, actor, id, p, programChoice{Verb: "confirm", Label: "Apply selected targets"}))
			}
		}
	}
	return actions
}
func programCandidates(b *state.Battle, lib content.BattleLibrary, actor, card string, s content.CardStep, x *state.CardExecution) []programChoice {
	if s.Effect == "choice" {
		var out []programChoice
		for i, o := range s.Choices {
			baseCost := 0
			if x == nil || !x.Paid {
				baseCost = lib.Cards[b.Settled.Actors[actor].CardInstances[card].DefinitionID].Cost.Energy
			}
			if len(o.Steps) > 0 && o.Steps[0].Effect == "sacrifice" && programUniqueCount(programCandidates(b, lib, actor, card, o.Steps[0], x)) < o.Steps[0].Target.Count {
				continue
			}
			if b.Actors[actor].Resources.EnergyPoints >= o.Energy+baseCost {
				out = append(out, programChoice{Verb: "option", Option: i, Label: fmt.Sprintf("%s · %d energy", o.Name, o.Energy)})
			}
		}
		return out
	}
	var out []programChoice
	kind := content.CardCapabilities()[s.Effect].Target
	for _, a := range sortedSettledActorIDs(b) {
		if s.Target.Owner == "self" && a != actor || s.Target.Owner == "enemy" && !containsString(otherActorIDs(b, actor), a) || b.Actors[a].DefeatState == state.ActorDefeated {
			continue
		}
		rt := b.Settled.Actors[a]
		name := lib.Combatants[b.Actors[a].DefinitionID].Name
		if name == "" {
			name = a
		}
		base := programChoice{Verb: "target", Actor: a, Label: name}
		switch kind {
		case "actor":
			out = append(out, base)
		case "ability":
			for _, id := range rt.OffensiveAbilityIDs {
				if s.Target.Qualified && !containsString(rt.QualifiedAbilityIDs, id) {
					continue
				}
				c := base
				c.Ability = id
				c.Label = lib.Abilities[id].Name
				out = append(out, c)
			}
		case "offensive_die", "defensive_die":
			dice := rt.FinalDice
			if kind == "offensive_die" && (rt.RollsUsed == 0 || (a != actor && b.Settled.Stage != stageOffensiveReact)) {
				continue
			}
			if kind == "defensive_die" {
				def, ok := b.Settled.DefenseSelections[a]
				if !ok || def.Finalized {
					continue
				}
				dice = def.RolledDice
			}
			for i, d := range dice {
				if len(s.Target.Faces) > 0 && !containsInt(s.Target.Faces, d.Face) {
					continue
				}
				c := base
				c.Die = i
				c.Label = fmt.Sprintf("%s · die %d (%d)", name, i+1, d.Face)
				faces := []int{0}
				switch s.Effect {
				case "set_die":
					faces = content.ProgramInts(s, "faces")
				case "adjust_die":
					faces = nil
					lo, hi := content.ProgramInt(s, "minimum"), content.ProgramInt(s, "maximum")
					for _, delta := range content.ProgramInts(s, "deltas") {
						f := d.Face + delta
						if content.ProgramBool(s, "wrap") {
							f = lo + ((f-lo)%(hi-lo+1)+(hi-lo+1))%(hi-lo+1)
						}
						if f >= lo && f <= hi {
							faces = append(faces, f)
						}
					}
				case "flip_die":
					faces = []int{content.ProgramInt(s, "sum") - d.Face}
				case "copy_die":
					faces = nil
					for j, from := range dice {
						if j != i && from.Face != d.Face && !containsInt(faces, from.Face) {
							faces = append(faces, from.Face)
						}
					}
				}
				if s.Effect == "reroll" && content.ProgramBool(s, "consume_roll") && rt.RollsUsed >= rt.MaxRolls {
					continue
				}
				for _, f := range faces {
					if s.Effect != "reroll" && s.Effect != "reroll_defense" && (f <= 0 || f == d.Face || dieFace(lib, d.DieID, f).Number == 0) {
						continue
					}
					c.Face = f
					if f > 0 {
						c.Label = fmt.Sprintf("%s · die %d: %d → %d", name, i+1, d.Face, f)
					}
					out = append(out, c)
				}
			}
		case "status":
			for _, st := range b.Actors[a].Statuses {
				def := lib.Statuses[st.DefinitionID]
				if def.DispelImmune || st.Stacks <= 0 || s.Target.Polarity != "" && s.Target.Polarity != "any" && def.Polarity != s.Target.Polarity || len(s.Target.StatusIDs) > 0 && !containsString(s.Target.StatusIDs, st.DefinitionID) {
					continue
				}
				c := base
				c.Status = st.DefinitionID
				c.Label = fmt.Sprintf("%s · %s (%d)", name, def.Name, st.Stacks)
				out = append(out, c)
			}
		case "source":
			for _, src := range reactionDamageSources(b) {
				if src.TargetActorID != a || settledSourceAmount(src) <= 0 {
					continue
				}
				c := base
				c.Source = src.ID
				c.Label = fmt.Sprintf("%s · %s · %d damage", src.SourceActorID, src.SourceContentID, settledSourceAmount(src))
				out = append(out, c)
			}
		case "threatened_card":
			if b.Settled.PendingDamage != nil {
				for _, r := range b.Settled.PendingDamage.Removals {
					if r.TargetActorID != a || !r.Accepted || r.Released || !r.Revealed {
						continue
					}
					for _, source := range r.DamageProposalIDs {
						c := base
						c.Card = r.CardID
						c.Source = source
						c.Label = lib.Cards[rt.CardInstances[r.CardID].DefinitionID].Name + " · " + source
						out = append(out, c)
					}
				}
			}
		case "card":
			for _, z := range s.Target.Zones {
				for _, id := range zoneCards(b.Actors[a].Cards, operation.CardZone(z)) {
					if id == card {
						continue
					}
					def := lib.Cards[rt.CardInstances[id].DefinitionID]
					if containsString(s.Target.ExcludeCards, def.ID) {
						continue
					}
					if s.Target.DrawnThisPlay && (x == nil || !containsString(x.Drawn, id)) {
						continue
					}
					if s.Target.ExcludeRecovery && programRecovery(def) {
						continue
					}
					c := base
					c.Card = id
					c.Label = def.Name + " · " + z
					out = append(out, c)
				}
			}
		}
	}
	return out
}
func programRecovery(d content.BattleCardDefinition) bool {
	if op, ok := generalOperation(d); ok && op.Modification == "recover_discard" {
		return true
	}
	if d.Program != nil {
		var recoverSteps func([]content.CardStep) bool
		recoverSteps = func(steps []content.CardStep) bool {
			for _, s := range steps {
				if s.Effect == "move_cards" && content.ProgramString(s, "destination") != "removed" {
					return true
				}
				for _, o := range s.Choices {
					if recoverSteps(o.Steps) {
						return true
					}
				}
			}
			return false
		}
		return recoverSteps(d.Program.Steps)
	}
	return false
}
func programPayload(cmd command.Command) (string, string) {
	if cmd.Type == command.TypePlanningCards {
		var p command.PlanningCardsPayload
		if command.DecodePayload(cmd, &p) == nil && len(p.CardIDs) == 1 {
			return p.CardIDs[0], p.StatusID
		}
	}
	if cmd.Type == command.TypeCommitInteraction {
		var p command.CommitInteractionPayload
		if command.DecodePayload(cmd, &p) == nil && len(p.Commitment.CardIDs) == 1 {
			return p.Commitment.CardIDs[0], p.Commitment.ChoiceID
		}
	}
	return "", ""
}
func (e Engine) handleProgramCommand(b *state.Battle, lib content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	actor := cmd.ActorID
	liveBefore := map[string]map[string]operation.CardZone{}
	for actorID, a := range b.Actors {
		liveBefore[actorID] = map[string]operation.CardZone{}
		for _, z := range []operation.CardZone{operation.ZoneHand, operation.ZoneDeck, operation.ZoneDiscard} {
			for _, cardID := range zoneCards(a.Cards, z) {
				liveBefore[actorID][cardID] = z
			}
		}
	}
	id, key := programPayload(cmd)
	pending := b.Flow.PendingInput[actor]
	legal := false
	for _, action := range programActions(b, lib, actor, pending) {
		a, k := programPayload(action)
		if a == id && k == key {
			legal = true
			break
		}
	}
	if !legal {
		return nil, fmt.Errorf("card choice is no longer legal")
	}
	var c programChoice
	_ = json.Unmarshal([]byte(key), &c)
	rt := b.Settled.Actors[actor]
	def := lib.Cards[rt.CardInstances[id].DefinitionID]
	x := rt.CardExecution
	if x != nil {
		x.Feedback = nil
	}
	if c.Verb == "cancel" {
		rt.CardExecution = nil
		b.Settled.Actors[actor] = rt
		rotateSettledPending(b, actor)
		return nil, nil
	}
	if c.Verb == "start" {
		x = &state.CardExecution{CardID: id}
		programSetQueue(x, def.Program.Steps)
		rt.CardExecution = x
		b.Settled.Actors[actor] = rt
	} else {
		queue := programQueue(x)
		s := queue[0]
		if c.Verb == "option" {
			o := s.Choices[c.Option]
			if err := payProgramCard(b, actor, id, x); err != nil {
				return nil, err
			}
			spendEnergy(b, actor, o.Energy)
			programSetQueue(x, append(append([]content.CardStep{}, o.Steps...), queue[1:]...))
		} else {
			if c.Verb == "target" {
				x.Selected = append(x.Selected, key)
			}
			n := 1
			if s.Target.Mode == "exact" || s.Target.Mode == "up_to" {
				n = s.Target.Count
			}
			if s.Target.Mode == "all" {
				n = programUniqueCount(programCandidates(b, lib, actor, id, s, x))
			}
			if c.Verb == "confirm" || len(x.Selected) >= n {
				var targets []programChoice
				for _, k := range x.Selected {
					var t programChoice
					_ = json.Unmarshal([]byte(k), &t)
					targets = append(targets, t)
				}
				if err := e.applyProgramStep(b, lib, actor, id, s, targets, x); err != nil {
					return nil, err
				}
				programSetQueue(x, queue[1:])
			}
		}
	}
	// Automatic steps run in authored order. New choices are regenerated after
	// each mutation, so a subsequent step can choose a newly drawn card.
	for limit := 0; limit < 256; limit++ {
		queue := programQueue(x)
		if len(queue) == 0 {
			break
		}
		s := queue[0]
		if s.Condition != nil && s.Effect != "ability_bonus" && !requirementsMet(*s.Condition, b.Settled.Actors[actor].FinalDice) {
			programSetQueue(x, queue[1:])
			continue
		}
		candidates := programCandidates(b, lib, actor, id, s, x)
		if len(candidates) == 0 {
			if s.Effect == "sacrifice" {
				return nil, fmt.Errorf("cannot pay sacrifice cost")
			}
			programSetQueue(x, queue[1:])
			continue
		}
		n := 1
		if s.Target.Mode == "exact" || s.Target.Mode == "up_to" {
			n = s.Target.Count
		}
		unique := programUniqueCount(candidates)
		if s.Target.Mode == "all" {
			n = unique
		}
		if n > unique {
			if s.Effect == "sacrifice" {
				return nil, fmt.Errorf("cannot pay sacrifice cost")
			}
			if s.Target.Mode == "exact" {
				programSetQueue(x, queue[1:])
				continue
			}
			n = unique
		}
		if s.Effect == "choice" || s.Target.Selection == "choose" && (len(x.Selected) > 0 || (s.Target.Mode == "all" && len(candidates) > unique) || (s.Target.Mode != "all" && (len(candidates) > 1 || s.Target.Mode == "up_to"))) {
			break
		}
		selected := []programChoice{}
		for len(selected) < n {
			i := 0
			if s.Target.Selection == "random" {
				var err error
				i, err = e.namedIntn(b, "card_program", len(candidates))
				if err != nil {
					return nil, err
				}
			}
			selected = append(selected, candidates[i])
			candidates = programUnselected(candidates, []string{candidates[i].key()})
		}
		if err := e.applyProgramStep(b, lib, actor, id, s, selected, x); err != nil {
			return nil, err
		}
		programSetQueue(x, queue[1:])
	}
	rt = b.Settled.Actors[actor]
	done := len(programQueue(x)) == 0
	if done {
		a := b.Actors[actor]
		zone := programCardZone(b, actor, id)
		if x.Paid && zone != "" && zone != operation.ZoneRemoved {
			moveCard(&a.Cards, id, zone, operation.CardZone(def.Play.Destination))
			b.Actors[actor] = a
		}
		if batch := b.Settled.PendingDamage; batch != nil {
			for i := range batch.Removals {
				r := &batch.Removals[i]
				if r.TargetActorID == actor && r.CardID == id && r.Released {
					damage.RetainPreventedCard(b, r)
				}
			}
		}
		rt.CardExecution = nil
	} else {
		rt.CardExecution = x
	}
	b.Settled.Actors[actor] = rt
	refreshPlanningPublicCounts(b)
	rotateSettledPending(b, actor)
	kind := event.Type("card_program_choice")
	if done {
		kind = event.TypeCardPlayed
	}
	events := []event.Event{settledEvent(kind, b, actor, map[string]any{"card_instance_id": id, "card_definition_id": def.ID, "program": true, "complete": done, "program_dice_changes": x.Feedback})}
	defeated := false
	for actorID, a := range b.Actors {
		cards := []state.WoundCard{}
		for _, cardID := range a.Cards.Removed {
			if zone, ok := liveBefore[actorID][cardID]; ok {
				cards = append(cards, state.WoundCard{CardID: cardID, CardDefinitionID: b.Settled.Actors[actorID].CardInstances[cardID].DefinitionID, OriginalZone: zone})
			}
		}
		if len(cards) > 0 {
			b.Settled.Sequence++
			woundID := fmt.Sprintf("card-removal-%d", b.Settled.Sequence)
			b.Wounds = append(b.Wounds, state.Wound{ID: woundID, BatchID: woundID, Round: b.Segment.Round, Segment: b.Segment.Current, SourceActorID: actor, SourceContentID: def.ID, TargetActorID: actorID, Cards: cards})
		}
		if len(cards) > 0 && a.CurrentHealth() == 0 && a.DefeatState != state.ActorDefeated {
			defeated = true
			a.DefeatState = state.ActorPendingDefeat
			b.Actors[actorID] = a
		}
	}
	if defeated {
		completion, err := evaluateBattleCompletion(b)
		cancelDefeatedOffense(b)
		if state.IsTerminalBattleStatus(b.Status) {
			clearCompletedProgramBonuses(b)
		}
		return append(events, completion...), err
	}
	return events, nil
}
func (e Engine) applyProgramStep(b *state.Battle, lib content.BattleLibrary, actor, id string, s content.CardStep, targets []programChoice, x *state.CardExecution) error {
	if len(targets) > 0 {
		if err := payProgramCard(b, actor, id, x); err != nil {
			return err
		}
	}
	for _, t := range targets {
		faceBefore := 0
		if content.CardCapabilities()[s.Effect].Target == "offensive_die" {
			faceBefore = b.Settled.Actors[t.Actor].FinalDice[t.Die].Face
		}
		result := effectResult{}
		switch s.Effect {
		case "curse", "roll_cursed", "status_threshold", "conditional_status":
			special := &content.SharedSpecialEffect{Kind: s.Effect, Limit: content.ProgramInt(s, "limit"), FallbackStatusID: content.ProgramString(s, "fallback_status_id"), FallbackStacks: content.ProgramInt(s, "fallback_stacks"), Amount: content.ProgramInt(s, "amount"), StatusID: content.ProgramString(s, "status_id"), Threshold: content.ProgramInt(s, "threshold"), ResultStatusID: content.ProgramString(s, "result_status_id"), Stacks: content.ProgramInt(s, "stacks")}
			ctx := effectContext{SourceActorID: actor, SourceContentID: b.Settled.Actors[actor].CardInstances[id].DefinitionID, SourceContentType: "card"}
			if err := e.executeSharedSpecial(b, lib, ctx, []string{t.Actor}, special); err != nil {
				return err
			}
		case "draw":
			for i := 0; i < content.ProgramInt(s, "amount"); i++ {
				card, err := e.drawSettledCard(b, t.Actor, "card_draw")
				if err != nil {
					return err
				}
				if card != "" {
					x.Drawn = append(x.Drawn, card)
				}
			}
		case "energy":
			gainEnergy(b, t.Actor, content.ProgramInt(s, "amount"))
		case "move_cards", "sacrifice":
			dest := content.ProgramString(s, "destination")
			if s.Effect == "sacrifice" {
				dest = "removed"
			}
			a := b.Actors[t.Actor]
			zone := programCardZone(b, t.Actor, t.Card)
			if zone != "" && zone != operation.ZoneRemoved {
				moveCard(&a.Cards, t.Card, zone, operation.CardZone(dest))
				b.Actors[t.Actor] = a
			}
		case "prevent":
			result.Preventions = []effectPrevention{{ProposalID: t.Source, Amount: content.ProgramInt(s, "amount")}}
			result.SavedCardDestination = content.ProgramString(s, "destination")
		case "save_cards":
			if b.Settled.PendingDamage != nil {
				for i := range b.Settled.PendingDamage.Removals {
					r := &b.Settled.PendingDamage.Removals[i]
					if r.CardID == t.Card && r.TargetActorID == t.Actor && r.Accepted && containsString(r.DamageProposalIDs, t.Source) {
						r.Accepted = false
						r.Released = true
						r.ProtectedFromSource = true
						damage.MovePreventedCard(b, r, content.ProgramString(s, "destination"))
						break
					}
				}
			}
			result.Preventions = []effectPrevention{{ProposalID: t.Source, Amount: 1}}
			result.SavedCardDestination = content.ProgramString(s, "destination")
		case "remove_status":
			result.StatusRemovals = []state.SettledStatusRemoval{{ActorID: t.Actor, StatusID: t.Status, Stacks: content.ProgramInt(s, "stacks")}}
		case "apply_status":
			result.StatusApplications = []state.SettledStatusApplication{{SourceActorID: actor, TargetActorID: t.Actor, StatusID: content.ProgramString(s, "status_id"), Stacks: content.ProgramInt(s, "stacks")}}
		case "set_die", "adjust_die", "flip_die", "copy_die":
			result.DieChanges = []effectDieChange{{ActorID: t.Actor, Index: t.Die, Face: t.Face}}
		case "reroll":
			old := b.Settled.Actors[t.Actor].FinalDice[t.Die].Face
			dice, err := e.rollCombatDice(b, lib, t.Actor, []int{t.Die})
			if err != nil {
				return err
			}
			f := programRerollFace(s, old, dice[t.Die].Face)
			result.DieChanges = []effectDieChange{{ActorID: t.Actor, Index: t.Die, Face: f}}
		case "reroll_defense":
			sel := b.Settled.DefenseSelections[t.Actor]
			old := sel.RolledDice[t.Die]
			d, err := e.ownedRoll(b, lib, t.Actor, old.Index, "defense_dice", true)
			if err != nil {
				return err
			}
			if programRerollFace(s, old.Face, d.Face) == old.Face {
				d = old
			}
			sel.RolledDice[t.Die] = d
			sel.RolledFaces[t.Die] = d.Face
			sel.RolledFace = sel.RolledFaces[0]
			b.Settled.DefenseSelections[t.Actor] = sel
		case "ability_bonus":
			if err := applyProgramBonus(b, lib, actor, id, t, s); err != nil {
				return err
			}
		}
		if err := e.applyEffectMutations(b, lib, id, result); err != nil {
			return err
		}
		if content.CardCapabilities()[s.Effect].Target == "offensive_die" {
			x.Feedback = append(x.Feedback, state.ProgramDieFeedback{ActorID: t.Actor, Index: t.Die, FaceBefore: faceBefore, Face: b.Settled.Actors[t.Actor].FinalDice[t.Die].Face, Rolled: s.Effect == "reroll"})
		}
		if content.CardCapabilities()[s.Effect].Target == "offensive_die" && b.Settled.Stage == stageOffensiveReact {
			if err := e.revalidateOffensiveSelection(b, lib, t.Actor); err != nil {
				return err
			}
		}
	}
	if s.Effect == "reroll" && content.ProgramBool(s, "consume_roll") {
		seen := map[string]bool{}
		for _, t := range targets {
			if !seen[t.Actor] {
				rt := b.Settled.Actors[t.Actor]
				rt.RollsUsed++
				b.Settled.Actors[t.Actor] = rt
				seen[t.Actor] = true
			}
		}
	}
	return nil
}
func programRerollFace(s content.CardStep, old, next int) int {
	switch content.ProgramString(s, "result") {
	case "higher":
		return max(old, next)
	case "lower":
		return min(old, next)
	}
	return next
}
func applyProgramBonus(b *state.Battle, lib content.BattleLibrary, actor, id string, t programChoice, s content.CardStep) error {
	rt := b.Settled.Actors[t.Actor]
	def := lib.Cards[b.Settled.Actors[actor].CardInstances[id].DefinitionID]
	statusID := content.ProgramStatusID(def.ID, s)
	duration := content.ProgramString(s, "duration")
	if content.ProgramString(s, "stacking") == "refresh" {
		found := false
		for i := range rt.AbilityModifiers {
			m := &rt.AbilityModifiers[i]
			if m.StatusID != statusID || m.AbilityID != t.Ability {
				continue
			}
			found = true
			if duration == "round" {
				m.ExpiresAfterRound = b.Segment.Round
			}
			if duration == "rounds" {
				m.ExpiresAfterRound = b.Segment.Round + content.ProgramInt(s, "rounds") - 1
			}
		}
		if found {
			b.Settled.Actors[t.Actor] = rt
			return nil
		}
	}
	if content.ProgramString(s, "stacking") == "replace" {
		kept := rt.AbilityModifiers[:0]
		for _, m := range rt.AbilityModifiers {
			if m.StatusID != statusID || m.AbilityID != t.Ability {
				kept = append(kept, m)
			} else {
				removeStatus(b, t.Actor, statusID, 1)
			}
		}
		rt.AbilityModifiers = kept
	}
	n := 0
	for _, m := range rt.AbilityModifiers {
		if m.StatusID == statusID && m.AbilityID == t.Ability {
			n++
		}
	}
	if n >= content.ProgramInt(s, "stack_limit") {
		return nil
	}
	raw, _ := json.Marshal(s)
	m := state.RuntimeAbilityModifier{SourceCardInstanceID: id, AbilityID: t.Ability, BonusID: statusID, StatusID: statusID, ProgramBonus: raw, ConsumeOnUse: duration == "next_use", ExpiresAfterOffensive: duration == "offensive"}
	if duration == "round" {
		m.ExpiresAfterRound = b.Segment.Round
	}
	if duration == "rounds" {
		m.ExpiresAfterRound = b.Segment.Round + content.ProgramInt(s, "rounds") - 1
	}
	rt.AbilityModifiers = append(rt.AbilityModifiers, m)
	b.Settled.Actors[t.Actor] = rt
	applyVenomStatus(b, lib, actor, state.SettledStatusApplication{SourceActorID: actor, TargetActorID: t.Actor, StatusID: statusID, Stacks: 1})
	return nil
}

func consumeProgramBonuses(b *state.Battle, actor, ability string) {
	rt := b.Settled.Actors[actor]
	kept := rt.AbilityModifiers[:0]
	for _, m := range rt.AbilityModifiers {
		var s content.CardStep
		_ = json.Unmarshal(m.ProgramBonus, &s)
		if m.ConsumeOnUse && m.AbilityID == ability && (s.Condition == nil || requirementsMet(*s.Condition, rt.FinalDice)) {
			removeStatus(b, actor, m.StatusID, 1)
		} else {
			kept = append(kept, m)
		}
	}
	rt.AbilityModifiers = kept
	b.Settled.Actors[actor] = rt
}
func expireProgramRoundBonuses(b *state.Battle) {
	for actor, rt := range b.Settled.Actors {
		kept := rt.AbilityModifiers[:0]
		for _, m := range rt.AbilityModifiers {
			if len(m.ProgramBonus) > 0 && m.ExpiresAfterRound > 0 && m.ExpiresAfterRound <= b.Segment.Round {
				removeStatus(b, actor, m.StatusID, 1)
			} else {
				kept = append(kept, m)
			}
		}
		rt.AbilityModifiers = kept
		b.Settled.Actors[actor] = rt
	}
}

func payProgramCard(b *state.Battle, actor, id string, x *state.CardExecution) error {
	if x.Paid {
		return nil
	}
	lib, err := settledLibrary(b)
	if err != nil {
		return err
	}
	rt := b.Settled.Actors[actor]
	d := lib.Cards[rt.CardInstances[id].DefinitionID]
	if b.Actors[actor].Resources.EnergyPoints < d.Cost.Energy {
		return fmt.Errorf("not enough energy")
	}
	spendEnergy(b, actor, d.Cost.Energy)
	if rt.CardUses == nil {
		rt.CardUses = map[string]int{}
	}
	rt.CardUses[d.ID]++
	rt.CardUses[fmt.Sprintf("%d:%s", b.Segment.Round, d.ID)]++
	rt.CardExecution = x
	b.Settled.Actors[actor] = rt
	x.Paid = true
	return nil
}

// Battle-scoped preparations end with the battle, including an immediate defeat
// caused by an explicit removal cost rather than a normal phase transition.
func clearCompletedProgramBonuses(b *state.Battle) {
	for actor, rt := range b.Settled.Actors {
		for _, m := range rt.AbilityModifiers {
			if len(m.ProgramBonus) > 0 {
				removeStatus(b, actor, m.StatusID, 0)
			}
		}
	}
}
