package lookahead

import (
	"encoding/json"
	"sort"
	"strconv"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// ProxyPolicy is the scripted "competent human" the expedition tests use
// (smartHumanAction in internal/battle/learned/expedition_region_test.go),
// ported so it can be measured in the same harness. It rolls, rerolls toward
// Swords (or Coins after three), takes the highest-damage tier, defends every
// attack and plays prevention cards whenever they are legal. It never plays
// offensive cards, and its symbols are tuned to the Starter/Adventurer board.
type ProxyPolicy struct{}

func (ProxyPolicy) Name() string { return "proxy" }

func (ProxyPolicy) Choose(d *Decision) (int, error) {
	lib, err := library(d.Battle)
	if err != nil {
		return 0, err
	}
	return proxyChoice(*d.Battle, lib, d.Seat, d.Actions), nil
}

func proxyChoice(b state.Battle, lib content.BattleLibrary, seat string, actions []command.Command) int {
	byType := map[command.Type][]int{}
	for i, a := range actions {
		byType[a.Type] = append(byType[a.Type], i)
	}
	for _, i := range byType[command.TypeCommitInteraction] {
		var p command.CommitInteractionPayload
		_ = json.Unmarshal(actions[i].Payload, &p)
		if len(p.Commitment.ProposalIDs) > 0 && len(p.Commitment.CardIDs) > 0 {
			return i
		}
	}
	stage := ""
	var runtime state.SettledActorRuntime
	if b.Settled != nil {
		if b.Settled.Window != nil {
			stage = b.Settled.Window.Stage
		}
		runtime = b.Settled.Actors[seat]
	}
	switch stage {
	case "planning":
		if roll := byType[command.TypePlanningRoll]; len(roll) > 0 {
			return roll[0]
		}
		dice := runtime.FinalDice
		if runtime.AmbushChoose > 0 && len(byType[command.TypePlanningKeep]) > 0 {
			goal := proxyGoalSymbol(dice)
			order := make([]int, len(dice))
			for i := range order {
				order[i] = i
			}
			sort.SliceStable(order, func(i, j int) bool {
				return hasSymbol(dice[order[i]], goal) && !hasSymbol(dice[order[j]], goal)
			})
			wanted := sortedInts(order[:runtime.AmbushChoose])
			for _, i := range byType[command.TypePlanningKeep] {
				var p command.PlanningKeepPayload
				_ = json.Unmarshal(actions[i].Payload, &p)
				if equalInts(sortedInts(p.KeptIndices), wanted) {
					return i
				}
			}
			return byType[command.TypePlanningKeep][0]
		}
		best, bestDamage := -1, -1
		for _, i := range byType[command.TypePlanningAbility] {
			var p command.PlanningAbilityPayload
			_ = json.Unmarshal(actions[i].Payload, &p)
			if d := proxyTierDamage(lib, p.AbilityID, p.TierID); d > bestDamage {
				bestDamage, best = d, i
			}
		}
		if rerolls := byType[command.TypePlanningReroll]; runtime.SelectedAbilityID == "" && len(rerolls) > 0 && bestDamage < 6 {
			goal := proxyGoalSymbol(dice)
			var indices []int
			for i, die := range dice {
				if !hasSymbol(die, goal) {
					indices = append(indices, i)
				}
			}
			if len(indices) > 0 {
				for _, i := range rerolls {
					var p command.PlanningRerollPayload
					_ = json.Unmarshal(actions[i].Payload, &p)
					if equalInts(sortedInts(p.RerollIndices), indices) {
						return i
					}
				}
			}
		}
		if runtime.SelectedAbilityID == "" && best >= 0 {
			return best
		}
		if pass := byType[command.TypePlanningPass]; len(pass) > 0 {
			return pass[0]
		}
	case "defense_selection":
		for _, i := range byType[command.TypePlanningAbility] {
			var p command.PlanningAbilityPayload
			_ = json.Unmarshal(actions[i].Payload, &p)
			if !p.SpendCatalyst {
				return i
			}
		}
		if pass := byType[command.TypePlanningPass]; len(pass) > 0 {
			return pass[0]
		}
	case "defense_roll":
		if roll := byType[command.TypeRollDice]; len(roll) > 0 {
			return roll[0]
		}
	}
	for _, kind := range []command.Type{command.TypeRollDice, command.TypePlanningPass, command.TypePass, command.TypeCommitInteraction} {
		if list := byType[kind]; len(list) > 0 {
			return list[0]
		}
	}
	return 0
}

func proxyGoalSymbol(dice []state.RolledDie) string {
	if symbolCount(dice, "gold_coin") >= 3 && symbolCount(dice, "sword") < 3 {
		return "gold_coin"
	}
	return "sword"
}

func proxyTierDamage(lib content.BattleLibrary, abilityID, tierID string) int {
	def, ok := lib.Abilities[abilityID]
	if !ok || def.Qualification == nil {
		return 1
	}
	best := 0
	for i, tier := range def.Qualification.ActivationTiers {
		if tierID != "" && tier.ID != tierID {
			continue
		}
		for _, op := range tier.Operations {
			if op.Type == "deal_damage" && amount(op.Amount) > best {
				best = amount(op.Amount)
			}
		}
		if tierID == "" && i == 0 {
			break
		}
	}
	if best == 0 {
		return 1
	}
	return best
}

func symbolCount(dice []state.RolledDie, symbol string) int {
	n := 0
	for _, d := range dice {
		if hasSymbol(d, symbol) {
			n++
		}
	}
	return n
}

func hasSymbol(d state.RolledDie, symbol string) bool {
	for _, s := range d.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

func sortedInts(in []int) []int {
	out := append([]int(nil), in...)
	sort.Ints(out)
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// amount reads an authored integer amount; formulas such as "rolled_face"
// count as zero.
func amount(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
