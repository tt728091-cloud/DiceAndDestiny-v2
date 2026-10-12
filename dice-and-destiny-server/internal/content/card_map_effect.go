package content

import (
	"fmt"
	"strings"
)

// CardMapEffect is a card's effect on the Expedition map. Every card is worth
// 1 Move or 1 Influence there; a map effect replaces that with something
// stronger when the card is played as a map card. Map effects that cost
// energy are rare and big: map energy only refills at rest.
type CardMapEffect struct {
	// Kind is one of CardMapEffectKinds.
	Kind string `yaml:"kind" json:"kind"`
	// Amount is the Move points, cards drawn, energy gained, sight bonus or
	// scouting range, by kind.
	Amount int `yaml:"amount" json:"amount"`
	// EnergyCost is paid from the map energy pool when the card is played
	// for its effect (0 for most map effects).
	EnergyCost int `yaml:"energy_cost,omitempty" json:"energy_cost,omitempty"`
	// IgnoreTerrain makes a move effect pay 1 per hex whatever the terrain
	// (impassable terrain stays impassable).
	IgnoreTerrain bool `yaml:"ignore_terrain,omitempty" json:"ignore_terrain,omitempty"`
	// IgnoreNight makes a move effect use Day terrain costs at Night.
	IgnoreNight bool `yaml:"ignore_night,omitempty" json:"ignore_night,omitempty"`
	// Text is the map rules text shown on the card and in hover.
	Text string `yaml:"text" json:"text"`
}

// CardMapEffectKinds lists the map effects the authority can resolve:
//   - move: gain Amount Move points this turn;
//   - draw: draw Amount cards (available deck cards only, no reshuffle);
//   - energy: gain Amount map energy;
//   - sight: +Amount sight range for the rest of this turn;
//   - scout: reveal the contents of every placed hex within Amount of the
//     player for the rest of this turn.
var CardMapEffectKinds = []string{"move", "draw", "energy", "sight", "scout"}

// ValidateCardMapEffect checks a map effect's kind, ranges and text.
func ValidateCardMapEffect(effect *CardMapEffect) error {
	if effect == nil {
		return nil
	}
	known := false
	for _, kind := range CardMapEffectKinds {
		known = known || kind == effect.Kind
	}
	if !known {
		return fmt.Errorf("map_effect kind %q is unknown", effect.Kind)
	}
	if effect.Amount < 1 || effect.Amount > 100 {
		return fmt.Errorf("map_effect amount must be 1–100")
	}
	if effect.EnergyCost < 0 || effect.EnergyCost > 100 {
		return fmt.Errorf("map_effect energy_cost must be 0–100")
	}
	if (effect.IgnoreTerrain || effect.IgnoreNight) && effect.Kind != "move" {
		return fmt.Errorf("map_effect terrain options belong to move effects")
	}
	if effect.Text == "" {
		return fmt.Errorf("map_effect text is required")
	}
	return nil
}

// IsMapOnlyCard reports a card that works on the map but never in battle.
func IsMapOnlyCard(card BattleCardDefinition) bool {
	return card.MapEffect != nil && IsBlankCard(card)
}

// CardMapEffectFace is the short map line on a card face ("Move 2", "Draw 2",
// "+10 Energy", "+1 sight", "Scout 3").
func CardMapEffectFace(effect *CardMapEffect) string {
	if effect == nil {
		return ""
	}
	var text string
	switch effect.Kind {
	case "move":
		text = fmt.Sprintf("Move %d", effect.Amount)
		if effect.IgnoreTerrain {
			text += ", any terrain"
		} else if effect.IgnoreNight {
			text += ", day costs"
		}
	case "draw":
		text = fmt.Sprintf("Draw %d", effect.Amount)
	case "energy":
		text = fmt.Sprintf("+%d Energy", effect.Amount)
	case "sight":
		text = fmt.Sprintf("+%d sight", effect.Amount)
	case "scout":
		text = fmt.Sprintf("Scout %d", effect.Amount)
	}
	if effect.EnergyCost > 0 {
		text = fmt.Sprintf("%d Energy: %s", effect.EnergyCost, text)
	}
	return text
}

// CardMapEffectRules is the full map rules line shown on hover.
func CardMapEffectRules(effect *CardMapEffect) string {
	if effect == nil {
		return ""
	}
	var text string
	switch effect.Kind {
	case "move":
		text = fmt.Sprintf("Gain %d Move this turn", effect.Amount)
		if effect.IgnoreTerrain {
			text += "; while any of it remains, every passable hex costs 1 Move"
		} else if effect.IgnoreNight {
			text += "; while any of it remains, terrain costs its Day price even at Night"
		}
	case "draw":
		text = fmt.Sprintf("Draw %d cards from your draw pile (only the cards still there; Spent cards are never reshuffled)", effect.Amount)
	case "energy":
		text = fmt.Sprintf("Gain %d energy on the map", effect.Amount)
	case "sight":
		text = fmt.Sprintf("See %d hex further for the rest of this turn", effect.Amount)
	case "scout":
		text = fmt.Sprintf("Learn the contents of every revealed hex within %d of you for the rest of this turn", effect.Amount)
	}
	if effect.EnergyCost > 0 {
		text = fmt.Sprintf("Pay %d energy: %s", effect.EnergyCost, strings.ToLower(text[:1])+text[1:])
	}
	if effect.Text != "" {
		text += " — " + effect.Text
	}
	return "Map: " + text + ". Playing it for its effect counts as a card played this turn."
}

// PrepareMapCard fills a map-only card's generated text: it has no battle
// effect, so the face says so and shows the map line.
func PrepareMapCard(card BattleCardDefinition) BattleCardDefinition {
	if card.MapEffect == nil || !IsBlankCard(card) {
		return card
	}
	if card.Operations == nil {
		card.Operations = []BattleOperation{}
	}
	if card.Play.PlayableDuring == nil {
		card.Play.PlayableDuring = []PlayTiming{}
	}
	card.Presentation.EffectSummary = "Map only\n" + CardMapEffectFace(card.MapEffect)
	card.Presentation.RulesText = "Map card. It cannot be played in battle; it still counts as health and can be lost to damage.\n" + CardMapEffectRules(card.MapEffect)
	card.Presentation.Timing = nil
	card.Presentation.PlayLimit = ""
	card.Presentation.Target = ""
	return card
}
