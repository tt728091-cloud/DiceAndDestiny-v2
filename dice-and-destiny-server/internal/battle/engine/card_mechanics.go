package engine

import (
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"fmt"
	"sort"
	"strings"
)

func mechanicAvailable(b *state.Battle, actor string, c content.BattleCardDefinition) bool {
	if c.Mechanic == nil {
		return true
	}
	m := c.Mechanic
	w := programWindow(b)
	if b.Settled.Stage == stageOngoingDamage {
		w = "ongoing_damage"
	}
	if !content.ProgramContains(m.Windows, w) && !programWindowOpen(b, actor, m.Windows) {
		return false
	}
	uses := b.Settled.Actors[actor].CardUses
	return (m.UsesPerBattle == 0 || uses[c.ID] < m.UsesPerBattle) && (m.UsesPerRound == 0 || uses[fmt.Sprintf("%d:%s", b.Segment.Round, c.ID)] < m.UsesPerRound)
}
func mechanicCards(b *state.Battle, actor string) []string {
	a := b.Actors[actor]
	out := append([]string{}, a.Cards.Hand...)
	out = append(out, a.Cards.Deck...)
	return append(out, a.Cards.Discard...)
}
func mechanicPlayable(b *state.Battle, actor, instance string, c content.BattleCardDefinition) bool {
	return content.ProgramContains(c.Play.SourceZones, string(programCardZone(b, actor, instance)))
}
func payMechanic(b *state.Battle, actor, instance string, c content.BattleCardDefinition) {
	spendEnergy(b, actor, c.Cost.Energy)
	a := b.Actors[actor]
	zone := programCardZone(b, actor, instance)
	moveCard(&a.Cards, instance, zone, operation.CardZone(c.Play.Destination))
	b.Actors[actor] = a
	rt := b.Settled.Actors[actor]
	if rt.CardUses == nil {
		rt.CardUses = map[string]int{}
	}
	rt.CardUses[c.ID]++
	rt.CardUses[fmt.Sprintf("%d:%s", b.Segment.Round, c.ID)]++
	b.Settled.Actors[actor] = rt
}
func rememberMechanic(b *state.Battle, actor string, c content.BattleCardDefinition) {
	if c.Mechanic == nil {
		return
	}
	v := venomRuntime(b)
	if v.Cards == nil {
		v.Cards = map[string]string{}
	}
	v.Cards[actor+":"+content.MechanicKind(c)] = c.ID
}
func rememberedMechanic(b *state.Battle, lib content.BattleLibrary, actor, kind string) content.BattleCardDefinition {
	if b.Settled.Venom != nil {
		if id := b.Settled.Venom.Cards[actor+":"+kind]; id != "" {
			return lib.Cards[id]
		}
	}
	return lib.Cards[kind]
}
func mechanicStatus(b *state.Battle, actor, kind string) string {
	if b.Settled.Venom != nil {
		if id := b.Settled.Venom.Cards[actor+":"+kind]; id != "" {
			if id == kind {
				switch kind {
				case "three_knocks":
					return "three_knocks_status"
				case "grave_interest", "second_knell", "maledictions_refusal", "black_dividend", "curse_bloom":
					return kind
				}
			}
			return id + "_card_effect"
		}
	}
	switch kind {
	case "three_knocks":
		return "three_knocks_status"
	}
	return kind
}
func mechanicStacks(b *state.Battle, actor, kind string) int {
	return stacks(b, actor, mechanicStatus(b, actor, kind))
}
func removeMechanicStatus(b *state.Battle, actor, kind string) {
	removeStatus(b, actor, mechanicStatus(b, actor, kind), 0)
}
func applyMechanicStatus(b *state.Battle, lib content.BattleLibrary, actor string, c content.BattleCardDefinition) {
	rememberMechanic(b, actor, c)
	applyStatus(b, lib, actor, mechanicStatus(b, actor, content.MechanicKind(c)), 1)
	if c.Mechanic != nil {
		v := venomRuntime(b)
		if v.Expirations == nil {
			v.Expirations = map[string]state.MechanicExpiration{}
		}
		sid := mechanicStatus(b, actor, c.Mechanic.Kind)
		for _, status := range b.Actors[actor].Statuses {
			if status.DefinitionID == sid {
				v.Expirations[actor+":"+c.Mechanic.Kind] = state.MechanicExpiration{Checkpoint: c.Mechanic.Expiration, Round: b.Segment.Round + c.Mechanic.Rounds, StatusID: sid, ActorID: actor, InstanceID: status.InstanceID}
			}
		}
	}
}
func preparationKind(p state.CursePreparation) string {
	if p.Kind != "" {
		return p.Kind
	}
	return p.CardID
}
func activePreparation(b *state.Battle, p state.CursePreparation) bool {
	return p.StatusID == "" || stacks(b, p.Target, p.StatusID) > 0
}
func statusCap(lib content.BattleLibrary, id string) int {
	s := lib.Statuses[id]
	if s.Stacking.Uncapped {
		return 1000000
	}
	return s.Stacking.StackLimit
}

func ownQualifiedDamage(b *state.Battle, lib content.BattleLibrary, actor string) bool {
	ops, ok := resolvedOffensiveOperations(b, lib, actor)
	if !ok {
		return false
	}
	for _, op := range ops {
		if op.Type == "deal_damage" {
			return true
		}
	}
	return false
}
func spendMechanicStatus(b *state.Battle, actor, status string, n int) {
	if n > 0 {
		removeStatus(b, actor, status, n)
	}
}

func repeatStatus(id string, n int) []string {
	out := []string{}
	for i := 0; i < n; i++ {
		out = append(out, id)
	}
	return out
}

func configuredExpiration(b *state.Battle, actor, kind string) bool {
	if b.Settled.Venom == nil {
		return false
	}
	_, ok := b.Settled.Venom.Expirations[actor+":"+kind]
	return ok
}
func expireMechanicStatuses(b *state.Battle) {
	if b.Settled.Venom == nil {
		return
	}
	keys := make([]string, 0, len(b.Settled.Venom.Expirations))
	for key := range b.Settled.Venom.Expirations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		x := b.Settled.Venom.Expirations[key]
		due := state.IsTerminalBattleStatus(b.Status)
		switch x.Checkpoint {
		case "offensive_exit":
			due = due || b.Segment.Current == segment.Offensive && b.Segment.Round >= x.Round-1
		case "damage_exit":
			due = due || b.Segment.Round >= x.Round || b.Segment.Round >= x.Round-1 && (b.Segment.Current == segment.DamageResolution || (b.Settled.UnifiedDefense && b.Segment.Current == segment.Defensive && b.Settled.Stage == "complete"))
		case "next_income":
			due = due || b.Segment.Current == segment.Income && b.Segment.Round >= x.Round
		case "next_ongoing":
			due = due || b.Segment.Current == segment.OngoingEffects && b.Segment.Round >= x.Round
		}
		if due {
			for _, status := range b.Actors[x.ActorID].Statuses {
				if status.InstanceID == x.InstanceID {
					removeStatus(b, x.ActorID, x.StatusID, 0)
					kind := strings.TrimPrefix(key, x.ActorID+":")
					events := map[string]string{"curse_bloom": "bloom_expired", "black_dividend": "dividend_expired", "second_knell": "second_knell_expired", "maledictions_refusal": "refusal_expired"}
					if event := events[kind]; event != "" {
						data := map[string]any{"source_card_id": rememberedMechanicID(b, x.ActorID, kind)}
						if kind == "black_dividend" {
							reward := curseRuntime(b).Dividends[x.ActorID]
							data["rewards"], data["limit"], data["source_actor_id"] = reward.Rewards, dividendLimit(reward), reward.Source
						}
						curseLog(b, x.ActorID, event, data)
					}
				}
			}
			delete(b.Settled.Venom.Expirations, key)
		}
	}
}

func mechanicLiveCards(b *state.Battle) map[string]map[string]operation.CardZone {
	out := map[string]map[string]operation.CardZone{}
	for actor := range b.Actors {
		out[actor] = map[string]operation.CardZone{}
		for _, id := range mechanicCards(b, actor) {
			out[actor][id] = programCardZone(b, actor, id)
		}
	}
	return out
}
func recordMechanicRemovals(b *state.Battle, actor string, c content.BattleCardDefinition, before map[string]map[string]operation.CardZone) {
	for target, a := range b.Actors {
		cards := []state.WoundCard{}
		for _, id := range a.Cards.Removed {
			if zone, ok := before[target][id]; ok {
				cards = append(cards, state.WoundCard{CardID: id, CardDefinitionID: b.Settled.Actors[target].CardInstances[id].DefinitionID, OriginalZone: zone})
			}
		}
		if len(cards) > 0 {
			b.Settled.Sequence++
			id := fmt.Sprintf("card-removal-%d", b.Settled.Sequence)
			b.Wounds = append(b.Wounds, state.Wound{ID: id, BatchID: id, Round: b.Segment.Round, Segment: b.Segment.Current, SourceActorID: actor, SourceContentID: c.ID, TargetActorID: target, Cards: cards})
			if a.CurrentHealth() == 0 {
				a.DefeatState = state.ActorPendingDefeat
				b.Actors[target] = a
			}
		}
	}
}

func canReceiveMechanicStatus(b *state.Battle, lib content.BattleLibrary, actor, status string) bool {
	return stacks(b, actor, status) < statusCap(lib, status) || status == "poison" && stacks(b, actor, "incubation") == 0
}

func rememberedMechanicID(b *state.Battle, actor, kind string) string {
	if b.Settled.Venom != nil {
		if id := b.Settled.Venom.Cards[actor+":"+kind]; id != "" {
			return id
		}
	}
	return kind
}
