package content

import (
	"fmt"
	"strings"
)

// CardMapEffect is a card's effect on the Expedition map. Every card is worth
// 1 Move or 1 Influence there; a map effect replaces that with something
// stronger when the card is played as a map card. Playing it for its map
// effect costs the card's printed energy, as in battle; map energy only
// refills at rest.
type CardMapEffect struct {
	// Kind is one of CardMapEffectKinds.
	Kind string `yaml:"kind" json:"kind"`
	// Amount is the kind's number (see CardMapEffectKinds): Move points,
	// cards drawn or looked at, energy, sight, scouting range, Influence,
	// hexes hidden, rests a mark lasts, rounds added, the Influence discount,
	// the healing bonus, bandages, or extra Ambush dice.
	Amount int `yaml:"amount" json:"amount"`
	// EnergyCost is a map-only extra paid from the map energy pool on top of
	// the card's printed energy cost when it is played for its effect (0 for
	// most map effects).
	EnergyCost int `yaml:"energy_cost,omitempty" json:"energy_cost,omitempty"`
	// IgnoreTerrain makes a move effect pay 1 per hex whatever the terrain
	// (impassable terrain stays impassable).
	IgnoreTerrain bool `yaml:"ignore_terrain,omitempty" json:"ignore_terrain,omitempty"`
	// IgnoreNight makes a move effect use Day terrain costs at Night.
	IgnoreNight bool `yaml:"ignore_night,omitempty" json:"ignore_night,omitempty"`
	// ChooseTerrain lets the player name one passable terrain when playing a
	// move effect; that terrain costs 1 Move for the rest of the turn.
	ChooseTerrain bool `yaml:"choose_terrain,omitempty" json:"choose_terrain,omitempty"`
	// CrossCost lets a move effect enter one impassable (lake or mountain) hex
	// this turn for this much Move. A turn that ends there sends the player
	// back to the hex they entered it from.
	CrossCost int `yaml:"cross_cost,omitempty" json:"cross_cost,omitempty"`
	// CarryOver keeps the turn's leftover Move for the next turn of the same
	// round (a visible status; a rest clears it).
	CarryOver bool `yaml:"carry_over,omitempty" json:"carry_over,omitempty"`
	// ExploreDiscount lowers this turn's exploring cost by this much.
	ExploreDiscount int `yaml:"explore_discount,omitempty" json:"explore_discount,omitempty"`
	// ExploreReach lets the player explore, this turn, an empty slot whose
	// nearest hex is up to this many hexes away (1 is the normal reach), as
	// long as it touches a placed tile.
	ExploreReach int `yaml:"explore_reach,omitempty" json:"explore_reach,omitempty"`
	// Sneak makes a move effect also stop provoked fights this turn.
	Sneak bool `yaml:"sneak,omitempty" json:"sneak,omitempty"`
	// RevealTiles makes a sight effect also reveal every enemy (open groups
	// and site defenders) on each tile the player can see into this turn.
	RevealTiles bool `yaml:"reveal_tiles,omitempty" json:"reveal_tiles,omitempty"`
	// Draw is how many of the looked-at cards a look effect puts into the
	// hand (0 or 1).
	Draw int `yaml:"draw,omitempty" json:"draw,omitempty"`
	// Bottom lets a look effect put any of the rest on the bottom of the draw
	// pile.
	Bottom bool `yaml:"bottom,omitempty" json:"bottom,omitempty"`
	// Text is the map rules text shown on the card and in hover.
	Text string `yaml:"text" json:"text"`
}

// CardMapEffectKinds lists the map effects the authority can resolve:
//   - move: gain Amount Move points this turn (with the move options);
//   - draw: draw Amount cards (available deck cards only, no reshuffle);
//   - energy: gain Amount map energy;
//   - sight: +Amount sight range for the rest of this turn;
//   - scout: reveal the contents of every placed hex within Amount of the
//     player for the rest of this turn;
//   - look: look at the top Amount cards of the draw pile, put Draw of them
//     into the hand and the rest back in any order (or on the bottom);
//   - influence: count as Amount Influence at the site or camp you are at;
//   - sneak: walking past enemies does not provoke a fight this turn;
//   - hide: until the end of your next turn enemies see you Amount hexes
//     less, so you can start that turn unseen;
//   - fall_back: played when you provoke a fight, step back to the hex you
//     came from instead;
//   - high_ground: your next fight this turn counts as from cover (an Ambush
//     if the enemy's habit allows one), with Amount extra Ambush dice;
//   - mark: mark the Warlord (in sight) or any Lieutenant; you see where it
//     is until your Amount-th rest;
//   - extend_time: with an enemy marked, the quest gains Amount more rounds
//     (once per quest);
//   - lure: bait the defenders of an adjacent keep, tower or occupied site
//     out to fight you in the open, so they are not Fortified;
//   - haggle: at this site or camp, every Influence payment this visit costs
//     Amount less, never less than 1;
//   - field_medic: at camp, +Amount to every basic-aid roll this camp;
//   - spare_bandage: gain Amount temporary bandages, used at camp and gone
//     when that camp breaks.
var CardMapEffectKinds = []string{"move", "draw", "energy", "sight", "scout", "look", "influence", "sneak", "hide", "fall_back", "high_ground", "mark", "extend_time", "lure", "haggle", "field_medic", "spare_bandage"}

// mapKindsWithoutAmount are the kinds whose amount is unused (it must be 0);
// high_ground's amount (extra Ambush dice) may be 0.
var mapKindsWithoutAmount = []string{"sneak", "fall_back", "lure"}

// MapEffectHasAmount reports whether the kind uses its amount.
func MapEffectHasAmount(kind string) bool {
	return !ProgramContains(mapKindsWithoutAmount, kind)
}

// ValidateCardMapEffect checks a map effect's kind, ranges and text.
func ValidateCardMapEffect(effect *CardMapEffect) error {
	if effect == nil {
		return nil
	}
	if !ProgramContains(CardMapEffectKinds, effect.Kind) {
		return fmt.Errorf("map_effect kind %q is unknown", effect.Kind)
	}
	switch {
	case !MapEffectHasAmount(effect.Kind):
		if effect.Amount != 0 {
			return fmt.Errorf("map_effect %s has no amount; leave it 0", effect.Kind)
		}
	case effect.Kind == "high_ground":
		if effect.Amount < 0 || effect.Amount > 10 {
			return fmt.Errorf("map_effect high_ground extra Ambush dice must be 0–10")
		}
	case effect.Amount < 1 || effect.Amount > 100:
		return fmt.Errorf("map_effect amount must be 1–100")
	}
	if effect.EnergyCost < 0 || effect.EnergyCost > 100 {
		return fmt.Errorf("map_effect energy_cost must be 0–100")
	}
	move := effect.IgnoreTerrain || effect.IgnoreNight || effect.ChooseTerrain || effect.CrossCost != 0 || effect.CarryOver || effect.ExploreDiscount != 0 || effect.ExploreReach != 0 || effect.Sneak
	if move && effect.Kind != "move" {
		return fmt.Errorf("map_effect terrain options belong to move effects")
	}
	if effect.CrossCost < 0 || effect.CrossCost > 10 || effect.ExploreDiscount < 0 || effect.ExploreDiscount > 10 || effect.ExploreReach < 0 || effect.ExploreReach > 10 {
		return fmt.Errorf("map_effect cross_cost, explore_discount and explore_reach must be 0–10")
	}
	if effect.ExploreReach == 1 {
		return fmt.Errorf("map_effect explore_reach 1 is the normal reach; use 2 or more, or 0")
	}
	if effect.RevealTiles && effect.Kind != "sight" {
		return fmt.Errorf("map_effect reveal_tiles belongs to sight effects")
	}
	if (effect.Draw != 0 || effect.Bottom) && effect.Kind != "look" {
		return fmt.Errorf("map_effect draw and bottom belong to look effects")
	}
	if effect.Kind == "look" && (effect.Amount > 10 || effect.Draw < 0 || effect.Draw > 1 || effect.Draw > effect.Amount) {
		return fmt.Errorf("map_effect look sees 1–10 cards and draws 0 or 1 of them")
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
	n := effect.Amount
	switch effect.Kind {
	case "move":
		text = fmt.Sprintf("Move %d", n)
		var extras []string
		if effect.IgnoreTerrain {
			extras = append(extras, "any terrain")
		} else if effect.IgnoreNight {
			extras = append(extras, "day costs")
		}
		if effect.ChooseTerrain {
			extras = append(extras, "one terrain costs 1")
		}
		if effect.CrossCost > 0 {
			extras = append(extras, "cross lake or mountain")
		}
		if effect.CarryOver {
			extras = append(extras, "keep leftover Move")
		}
		if effect.ExploreDiscount > 0 {
			extras = append(extras, fmt.Sprintf("explore −%d", effect.ExploreDiscount))
		}
		if effect.ExploreReach > 0 {
			extras = append(extras, "explore farther")
		}
		if effect.Sneak {
			extras = append(extras, "no provoked fights")
		}
		if len(extras) > 0 {
			text += ", " + strings.Join(extras, ", ")
		}
	case "draw":
		text = fmt.Sprintf("Draw %d", n)
	case "energy":
		text = fmt.Sprintf("+%d Energy", n)
	case "sight":
		text = fmt.Sprintf("+%d sight", n)
		if effect.RevealTiles {
			text += ", read the enemy"
		}
	case "scout":
		text = fmt.Sprintf("Scout %d", n)
	case "look":
		text = fmt.Sprintf("Look at %d", n)
		if effect.Draw > 0 {
			text += fmt.Sprintf(", draw %d", effect.Draw)
		}
		if effect.Bottom {
			text += ", bottom any"
		}
	case "influence":
		text = fmt.Sprintf("%d Influence", n)
	case "sneak":
		text = "Sneak"
	case "hide":
		text = fmt.Sprintf("Hide %d", n)
	case "fall_back":
		text = "Fall back"
	case "high_ground":
		text = "High ground"
		if n > 0 {
			text += fmt.Sprintf(", Ambush +%d dice", n)
		}
	case "mark":
		text = fmt.Sprintf("Mark (%s)", counted(n, "rest", "rests"))
	case "extend_time":
		text = fmt.Sprintf("+%s", counted(n, "round", "rounds"))
	case "lure":
		text = "Lure"
	case "haggle":
		text = fmt.Sprintf("Haggle −%d", n)
	case "field_medic":
		text = fmt.Sprintf("Basic aid +%d", n)
	case "spare_bandage":
		text = fmt.Sprintf("+%s", counted(n, "spare bandage", "spare bandages"))
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
	n := effect.Amount
	switch effect.Kind {
	case "move":
		text = fmt.Sprintf("Gain %d Move this turn", n)
		if effect.IgnoreTerrain {
			text += "; while any of it remains, every passable hex costs 1 Move"
		} else if effect.IgnoreNight {
			text += "; while any of it remains, terrain costs its Day price even at Night"
		}
		if effect.ChooseTerrain {
			text += "; name a terrain: it costs 1 Move for the rest of this turn"
		}
		if effect.CrossCost > 0 {
			text += fmt.Sprintf("; this turn you may enter one lake or mountain hex for %d Move, but if your turn ends there you go back to the hex you entered it from (no Move spent, no fight provoked)", effect.CrossCost)
		}
		if effect.CarryOver {
			text += "; Move left at the end of this turn carries over to your next turn this round (a rest clears it)"
		}
		if effect.ExploreDiscount > 0 {
			text += fmt.Sprintf("; exploring costs %d Move less this turn", effect.ExploreDiscount)
		}
		if effect.ExploreReach > 0 {
			text += fmt.Sprintf("; this turn you may explore an empty slot up to %d hexes away, if it touches a placed tile", effect.ExploreReach)
		}
		if effect.Sneak {
			text += "; this turn, walking past enemies does not provoke a fight"
		}
	case "draw":
		text = fmt.Sprintf("Draw %s from your draw pile (only the cards still there; Spent cards are never reshuffled)", counted(n, "card", "cards"))
	case "energy":
		text = fmt.Sprintf("Gain %d energy on the map", n)
	case "sight":
		text = fmt.Sprintf("See %d hex further for the rest of this turn", n)
		if effect.RevealTiles {
			text += ", and learn every enemy on each tile you can see into, with the XP it is worth"
		}
	case "scout":
		text = fmt.Sprintf("Learn the contents of every revealed hex within %d of you for the rest of this turn", n)
	case "look":
		text = fmt.Sprintf("Look at the top %s of your draw pile", counted(n, "card", "cards"))
		if effect.Draw > 0 {
			text += fmt.Sprintf(", draw %d of them", effect.Draw)
		}
		if effect.Bottom {
			text += ", put any of the rest on the bottom"
		}
		text += " and put the rest back in any order"
	case "influence":
		text = fmt.Sprintf("Counts as %d Influence at the site or camp you are at", n)
	case "sneak":
		text = "This turn, walking past enemies does not provoke a fight"
	case "hide":
		text = fmt.Sprintf("Until the end of your next turn, enemies see you %s less, so you can start that turn unseen (a rest ends it)", counted(n, "hex", "hexes"))
	case "fall_back":
		text = "Play it when you provoke a fight: step back to the hex you came from instead (the Move stays spent; your turn goes on)"
	case "high_ground":
		text = "Your next fight this turn counts as from cover: an Ambush if the enemy's habit allows one"
		if n > 0 {
			text += fmt.Sprintf(", rolling %d extra Ambush dice", n)
		}
	case "mark":
		text = fmt.Sprintf("Mark the Warlord (while he is in sight) or any Lieutenant (even unseen): you see where it is, but not the map around it, until your %s", ordinalRest(n))
	case "extend_time":
		text = fmt.Sprintf("With an enemy marked, this quest gains %s before the deadline. Once per quest", counted(n, "more round", "more rounds"))
	case "lure":
		text = "Bait the defenders of an adjacent keep, tower or occupied site out to fight you in the open: they are not Fortified. This is your action"
	case "haggle":
		text = fmt.Sprintf("At this site or camp, every Influence payment this visit costs %d less, never less than 1", n)
	case "field_medic":
		text = fmt.Sprintf("At camp: +%d to every basic-aid roll this camp", n)
	case "spare_bandage":
		text = fmt.Sprintf("Gain %s: each heals a treated wound at camp like a bandage, and any left are gone when that camp breaks", counted(n, "spare bandage", "spare bandages"))
	}
	if effect.EnergyCost > 0 {
		text = fmt.Sprintf("Pay %d energy: %s", effect.EnergyCost, strings.ToLower(text[:1])+text[1:])
	}
	if flavour := strings.TrimRight(strings.TrimSpace(effect.Text), "."); flavour != "" {
		text += " — " + flavour
	}
	return "Map: " + text + ". Playing it for its effect counts as a card played this turn."
}

func ordinalRest(n int) string {
	switch n {
	case 1:
		return "next rest"
	case 2:
		return "second rest"
	case 3:
		return "third rest"
	}
	return fmt.Sprintf("%dth rest", n)
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
