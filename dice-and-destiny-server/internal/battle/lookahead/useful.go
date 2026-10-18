package lookahead

import (
	"encoding/json"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

// Useful returns the indices of the listed commands that can change the
// battle. It drops pure bookkeeping moves: marking dice as kept (a reroll
// names its dice anyway), re-selecting the ability, tier and targets that are
// already selected, and cancelling a card just started (start-then-cancel
// changes nothing and can loop). Copies of the same card count once. The
// engine already refuses prevention against an attack with no damage left,
// so that waste never reaches a player. If nothing would remain, every
// command is returned.
func Useful(b *state.Battle, seat string, actions []command.Command) []int {
	var runtime state.SettledActorRuntime
	if b.Settled != nil {
		runtime = b.Settled.Actors[seat]
	}
	var out []int
	seen := map[string]bool{}
	for i, action := range actions {
		switch action.Type {
		case command.TypePlanningKeep:
			if runtime.AmbushChoose == 0 {
				continue
			}
		case command.TypePlanningAbility:
			var p command.PlanningAbilityPayload
			if json.Unmarshal(action.Payload, &p) == nil && p.AbilityID == runtime.SelectedAbilityID &&
				p.TierID == runtime.SelectedTierID && sameStrings(p.TargetIDs, runtime.SelectedTargetIDs) && !p.SpendCatalyst && len(p.ToxinChoices) == 0 {
				continue
			}
		case command.TypePlanningTargets:
			var p command.PlanningTargetsPayload
			if json.Unmarshal(action.Payload, &p) == nil && sameStrings(p.TargetIDs, runtime.SelectedTargetIDs) {
				continue
			}
		}
		if CardChoice(action).Verb == "cancel" {
			continue
		}
		key := distinctKey(action, runtime.CardInstances)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, i)
	}
	if len(out) == 0 {
		for i := range actions {
			out = append(out, i)
		}
	}
	return out
}

// Choice is a card program step as the authority lists it: start a card,
// pick its target, or cancel it.
type Choice struct {
	Verb    string `json:"verb"`
	Label   string `json:"label"`
	Ability string `json:"ability"`
	Die     int    `json:"die"`
	Face    *int   `json:"face"`
	Kind    string `json:"kind"`
}

// cardPayload holds the card fields of planning_commit_cards and
// commit_interaction payloads. Decoding only these keeps commands logged by
// the client (whose numbers arrive as 1.0) readable.
type cardPayload struct {
	CardIDs    []string `json:"card_ids"`
	StatusID   string   `json:"status_id"`
	Commitment struct {
		CardIDs  []string `json:"card_ids"`
		ChoiceID string   `json:"choice_id"`
	} `json:"commitment"`
}

func decodeCardPayload(action command.Command) (cardPayload, bool) {
	var p cardPayload
	if action.Type != command.TypePlanningCards && action.Type != command.TypeCommitInteraction {
		return p, false
	}
	return p, json.Unmarshal(action.Payload, &p) == nil
}

// CardChoice decodes a card command's program step, if it has one.
func CardChoice(action command.Command) Choice {
	var choice Choice
	p, ok := decodeCardPayload(action)
	if !ok {
		return choice
	}
	raw := p.StatusID
	if action.Type == command.TypeCommitInteraction {
		raw = p.Commitment.ChoiceID
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &choice)
	}
	return choice
}

// CardIDs lists the card instances a command plays or commits.
func CardIDs(action command.Command) []string {
	p, ok := decodeCardPayload(action)
	if !ok {
		return nil
	}
	if action.Type == command.TypeCommitInteraction {
		return p.Commitment.CardIDs
	}
	return p.CardIDs
}

// distinctKey treats copies of the same card as one option: the command
// with every card instance replaced by its definition.
func distinctKey(action command.Command, instances map[string]state.CardInstance) string {
	key := string(action.Type) + string(action.Payload)
	for _, id := range CardIDs(action) {
		if definition := instances[id].DefinitionID; definition != "" {
			key = strings.ReplaceAll(key, `"`+id+`"`, `"card:`+definition+`"`)
		}
	}
	return key
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
