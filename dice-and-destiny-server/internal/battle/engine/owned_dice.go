package engine

import (
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"fmt"
	"sort"
)

func curseRuntime(b *state.Battle) *state.CurseRuntime {
	if b.Settled.Curse == nil {
		b.Settled.Curse = &state.CurseRuntime{Dice: map[string][]state.OwnedDie{}, Bags: map[string][]int{}, Used: map[string]int{}, Bonus: map[string]int{}, TaxCards: map[string]int{}, TaxEnergy: map[string]int{}, LastStored: map[string]int{}}
	}
	c := b.Settled.Curse
	for _, id := range sortedSettledActorIDs(b) {
		if len(c.Dice[id]) > 0 {
			continue
		}
		for _, entry := range b.Actors[id].DiceLoadout {
			for n := 0; n < entry.Count; n++ {
				i := len(c.Dice[id])
				c.Dice[id] = append(c.Dice[id], state.OwnedDie{ID: fmt.Sprintf("%s/die/%d", id, i+1), DefinitionID: entry.DiceID, Index: i})
			}
		}
	}
	return c
}
func curseLog(b *state.Battle, actor, kind string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["kind"] = kind
	c := curseRuntime(b)
	c.Logs = append(c.Logs, state.CurseLog{Actor: actor, Round: b.Segment.Round, Segment: string(b.Segment.Current), Data: data})
}
func flushCurseEvents(b *state.Battle) []event.Event {
	if b.Settled.Curse == nil {
		return nil
	}
	// Private offensive retry faces become public only at the joint reveal.
	if !state.SettledPlanningPrivate(*b) {
		b.Settled.Curse.Logs = append(b.Settled.Curse.Logs, b.Settled.Curse.PendingKnell...)
		b.Settled.Curse.PendingKnell = nil
		b.Settled.Curse.Logs = append(b.Settled.Curse.Logs, b.Settled.Curse.PendingDividend...)
		b.Settled.Curse.PendingDividend = nil
	}
	var out []event.Event
	for _, l := range b.Settled.Curse.Logs {
		out = append(out, event.Event{Type: event.Type("curse_resolved"), ActorID: l.Actor, Round: l.Round, Segment: segment.Segment(l.Segment), Data: l.Data})
	}
	b.Settled.Curse.Logs = nil
	return out
}
func allOwned(b *state.Battle, actor string) []int {
	return allDieIndices(len(curseRuntime(b).Dice[actor]))
}
func ownedBagKey(b *state.Battle, actor, key string) string {
	if b.Segment.Current == segment.OngoingEffects {
		return fmt.Sprintf("effects:%d:%s", b.Segment.Round, actor)
	}
	return actor + ":" + key
}
func (e Engine) selectOwned(b *state.Battle, actor, key string, eligible []int) (int, error) {
	c := curseRuntime(b)
	if len(eligible) == 0 {
		return -1, fmt.Errorf("no eligible owned die for %s", actor)
	}
	// Bags contain unused identities, not future shuffled choices; priority is
	// evaluated when a check occurs and random sampling is without replacement.
	key = ownedBagKey(b, actor, key)
	remaining, exists := c.Bags[key]
	if !exists || len(remaining) == 0 {
		remaining = append([]int(nil), eligible...)
	}
	candidates := []int{}
	for _, i := range remaining {
		if containsInt(eligible, i) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		remaining = append([]int(nil), eligible...)
		candidates = append([]int(nil), eligible...)
	}
	priority := []int{}
	for _, i := range candidates {
		if c.Dice[actor][i].Entombed {
			priority = append(priority, i)
		}
	}
	instrument := -1
	if len(priority) > 0 {
		candidates = priority
	} else {
		for i, p := range c.Preparations {
			if p.CardID == "chosen_instrument" && p.Target == actor && containsInt(candidates, p.Die) {
				candidates = []int{p.Die}
				instrument = i
				break
			}
		}
	}
	n, err := e.namedIntn(b, "owned_die_selection", len(candidates))
	if err != nil {
		return -1, err
	}
	chosen := candidates[n]
	if instrument < 0 {
		for i, p := range c.Preparations {
			if p.CardID == "chosen_instrument" && p.Target == actor && p.Die == chosen {
				instrument = i
				break
			}
		}
	}
	if instrument >= 0 {
		c.Preparations = append(c.Preparations[:instrument], c.Preparations[instrument+1:]...)
	}
	next := []int{}
	for _, i := range remaining {
		if i != chosen {
			next = append(next, i)
		}
	}
	c.Bags[key] = next
	return chosen, nil
}
func (e Engine) ownedRoll(b *state.Battle, lib content.BattleLibrary, actor string, index int, stream string, allowRetry bool) (state.RolledDie, error) {
	c := curseRuntime(b)
	if index < 0 || index >= len(c.Dice[actor]) {
		return state.RolledDie{}, fmt.Errorf("invalid owned die %d", index)
	}
	d := c.Dice[actor][index]
	countBefore := stacks(b, actor, "curse_count")
	logStart := len(c.Logs)
	def := lib.Dice[d.DefinitionID]
	n, err := e.namedIntn(b, stream, def.SideCount)
	if err != nil {
		return state.RolledDie{}, err
	}
	face := def.Faces[n]
	result := state.RolledDie{Index: index, OwnedID: d.ID, DieID: d.DefinitionID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}
	cursed := containsInt(d.CursedFaces, face.Number)
	if cursed {
		applyStatus(b, lib, actor, "curse_count", 1)
		if d.Entombed {
			c.Dice[actor][index].Entombed = false
		}
		curseLog(b, actor, "count", map[string]any{"die_id": d.ID, "count": stacks(b, actor, "curse_count"), "released_entombment": d.Entombed})
		for i := range c.Preparations {
			p := &c.Preparations[i]
			if p.CardID == "black_dividend" && p.Target == actor && p.LastRewardRound != b.Segment.Round && p.Rewards < 2 {
				gainEnergy(b, p.Source, 1)
				p.LastRewardRound = b.Segment.Round
				p.Rewards++
				curseLog(b, p.Source, "dividend", map[string]any{"energy": b.Actors[p.Source].Resources.EnergyPoints})
			}
		}
	}
	// Never publish another participant's private offensive faces.
	if stream != "combat_dice" || !state.SettledPlanningPrivate(*b) {
		curseLog(b, actor, "owned_roll", map[string]any{"die": result, "cursed": cursed, "roll_context": stream})
	}
	if cursed {
		rewardBlackDividend(b, actor, result, stream)
	}
	if cursed && allowRetry {
		armed := stacks(b, actor, "second_knell") > 0
		if armed {
			removeStatus(b, actor, "second_knell", 0)
		}
		for i, p := range c.Preparations {
			if p.CardID == "second_knell" && p.Target == actor {
				c.Preparations = append(c.Preparations[:i], c.Preparations[i+1:]...)
				armed = true
				break
			}
		}
		if armed {
			retry, er := e.ownedRoll(b, lib, actor, index, stream, false)
			if er != nil {
				return retry, er
			}
			retry.EffectRetried = true
			// The trigger carries both physical results so playback can show the
			// first hit, status consumption, and retry without duplicate rolls.
			for _, log := range c.Logs[logStart:] {
				if log.Data["kind"] == "owned_roll" {
					log.Data["second_knell_part"] = true
				}
			}
			data := map[string]any{"kind": "second_knell_trigger", "die": result, "retry_die": retry, "retry_cursed": containsInt(d.CursedFaces, retry.Face), "count_before": countBefore, "count_after": stacks(b, actor, "curse_count"), "roll_context": stream}
			if stream == "combat_dice" && state.SettledPlanningPrivate(*b) {
				c.PendingKnell = append(c.PendingKnell, state.CurseLog{Actor: actor, Round: b.Segment.Round, Segment: string(b.Segment.Current), Data: data})
			} else {
				curseLog(b, actor, "second_knell_trigger", data)
			}
			return retry, nil
		}
	}
	return result, nil
}
func (e Engine) rollOwnedSubset(b *state.Battle, lib content.BattleLibrary, actor, key string, eligible []int, n int) ([]state.RolledDie, error) {
	var out []state.RolledDie
	for j := 0; j < n; j++ {
		i, err := e.selectOwned(b, actor, key, eligible)
		if err != nil {
			return nil, err
		}
		d, err := e.ownedRoll(b, lib, actor, i, "curse_dice", true)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
func (e Engine) rollOwnedEffect(b *state.Battle, lib content.BattleLibrary, roll *state.SettledEffectRoll, key, stream string, retry bool) error {
	c := curseRuntime(b)
	index := -1
	if retry {
		for i, d := range c.Dice[roll.ActorID] {
			if d.ID == roll.Die.OwnedID {
				index = i
				break
			}
		}
	}
	var err error
	if index < 0 {
		index, err = e.selectOwned(b, roll.ActorID, key, allOwned(b, roll.ActorID))
		if err != nil {
			return err
		}
	}
	ordinal := roll.Die.Index
	d, err := e.ownedRoll(b, lib, roll.ActorID, index, stream, !retry)
	if err != nil {
		return err
	}
	d.Index = ordinal
	roll.Die = d
	roll.Resolved = true
	roll.Rerolled = retry || d.EffectRetried
	return nil
}
func curseDice(b *state.Battle, actor, kind string) []int {
	var out []int
	for i, d := range curseRuntime(b).Dice[actor] {
		n := len(d.CursedFaces)
		if kind == "clean" && n == 0 || kind == "partly" && n > 0 && n < 6 || kind == "cursed" && n > 0 || kind == "unbound" && n > 0 && !d.Entombed || kind == "all" {
			out = append(out, i)
		}
	}
	return out
}
func markCurse(b *state.Battle, actor string, index, face int) {
	markCurseWithMethod(b, actor, index, face, "direct")
}

func markCurseWithMethod(b *state.Battle, actor string, index, face int, placement string) {
	c := curseRuntime(b)
	d := &c.Dice[actor][index]
	if containsInt(d.CursedFaces, face) {
		return
	}
	d.CursedFaces = append(d.CursedFaces, face)
	sort.Ints(d.CursedFaces)
	curseLog(b, actor, "face_marked", map[string]any{"die_id": d.ID, "face": face, "placement": placement, "cursed_faces": append([]int(nil), d.CursedFaces...)})
}
func (e Engine) lesserCurse(b *state.Battle, lib content.BattleLibrary, actor, key string, index int) error {
	if index < 0 {
		eligible := curseDice(b, actor, "partly")
		if len(eligible) == 0 {
			eligible = curseDice(b, actor, "cursed")
		}
		if len(eligible) == 0 {
			return nil
		}
		var err error
		index, err = e.selectOwned(b, actor, key, eligible)
		if err != nil {
			return err
		}
	}
	d, err := e.ownedRoll(b, lib, actor, index, "curse_dice", true)
	if err != nil {
		return err
	}
	markCurseWithMethod(b, actor, index, d.Face, "rolled")
	return nil
}
func (e Engine) applyCurse(b *state.Battle, lib content.BattleLibrary, actor, key string, n int) error {
	for j := 0; j < n; j++ {
		start := len(curseRuntime(b).Logs)
		clean := curseDice(b, actor, "clean")
		if len(clean) > 0 {
			i, err := e.namedIntn(b, "curse_target", len(clean))
			if err != nil {
				return err
			}
			markCurse(b, actor, clean[i], 1)
		} else if err := e.lesserCurse(b, lib, actor, key, -1); err != nil {
			return err
		}
		for _, log := range curseRuntime(b).Logs[start:] {
			log.Data["application_index"] = j + 1
			log.Data["application_count"] = n
			log.Data["ordinary_curse"] = true
		}
	}
	return nil
}
func entombedIndices(b *state.Battle, actor string) []int {
	var out []int
	for i, d := range curseRuntime(b).Dice[actor] {
		if d.Entombed {
			out = append(out, i)
		}
	}
	return out
}
func entomb(b *state.Battle, actor string, index int) {
	c := curseRuntime(b)
	if len(entombedIndices(b, actor)) >= 2 || index < 0 || index >= len(c.Dice[actor]) || len(c.Dice[actor][index].CursedFaces) == 0 {
		return
	}
	c.Dice[actor][index].Entombed = true
	rt := b.Settled.Actors[actor]
	kept := []int{}
	for _, i := range rt.KeptIndices {
		if i != index {
			kept = append(kept, i)
		}
	}
	rt.KeptIndices = kept
	b.Settled.Actors[actor] = rt
	curseLog(b, actor, "entombed", map[string]any{"die_id": c.Dice[actor][index].ID})
}

// The legacy chart synthesizes a final combination. Curse requires the actual
// roll history, so its battles use real owned dice even for chart opponents.
func (e Engine) planOwnedAI(b *state.Battle, lib content.BattleLibrary, actor string) error {
	rt := b.Settled.Actors[actor]
	for n := 0; n < rt.MaxRolls; n++ {
		dice, err := e.rollCombatDice(b, lib, actor, nil)
		if err != nil {
			return err
		}
		rt = b.Settled.Actors[actor]
		rt.FinalDice = dice
		rt.RollsUsed = n + 1
		rt.RollHistory = append(rt.RollHistory, state.RollBatch{Number: n + 1, Dice: cloneDice(dice), RolledIndices: allDieIndices(len(dice))})
		rt.QualifiedAbilityIDs = qualifiedAbilities(lib, rt.OffensiveAbilityIDs, dice, rt.AbilityModifiers)
		for _, id := range rt.QualifiedAbilityIDs {
			if b.Actors[actor].Resources.EnergyPoints >= lib.Abilities[id].Cost.Energy {
				rt.SelectedAbilityID = id
				tier, _ := qualifiedTier(lib.Abilities[id], dice)
				rt.SelectedTierID = tier.ID
				rt.SelectedTargetIDs = []string{enemyOf(b, actor)}
				break
			}
		}
		b.Settled.Actors[actor] = rt
		if rt.SelectedAbilityID != "" {
			break
		}
	}
	return nil
}
