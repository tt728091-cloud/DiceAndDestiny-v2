package boardprice

import (
	"fmt"
	"math"

	"diceanddestiny/server/internal/content"
)

// XP pricing of a board (docs/game-design/power-pricing-research.md): the
// damage level and each defense cost XP at one rate per point of average
// damage, the same rate a point of average prevention costs. Block defenses
// (faces that prevent a whole attack) stop a share of every hit, so they are
// worth more as damage grows and cost a share of the sheet's total XP.

// NoDefense leaves a template's defense slot empty.
const NoDefense = "none"

// BlockAmount is the prevention that marks a face as a block: it prevents a
// whole attack (the configured-ability maximum).
const BlockAmount = 100

// XPRates are the XP prices of a board, read from economy.yaml board_xp.
type XPRates struct {
	// DamagePoint is the XP per point of the board's average damage per turn
	// above the standard level.
	DamagePoint float64 `yaml:"damage_point" json:"damage_point"`
	// PreventionPoint is the XP per point of a flat defense's average
	// prevention per roll.
	PreventionPoint float64 `yaml:"prevention_point" json:"prevention_point"`
	// Block prices block defenses: chance × Share × (total XP + Offset).
	Block BlockRate `yaml:"block" json:"block"`
	// DefenseExtra adds a flat XP amount to a defense (Holdfast+ keeps saved
	// cards in their pile, the Guard+ upgrade).
	DefenseExtra map[string]int `yaml:"defense_extra,omitempty" json:"defense_extra,omitempty"`
}

// BlockRate prices a block defense from its block chance and the sheet's total XP.
type BlockRate struct {
	Share  float64 `yaml:"share" json:"share"`
	Offset int     `yaml:"offset" json:"offset"`
}

// Validate checks the rates.
func (r XPRates) Validate() error {
	if r.DamagePoint <= 0 || r.DamagePoint > 1000 || r.PreventionPoint <= 0 || r.PreventionPoint > 1000 {
		return fmt.Errorf("board_xp: damage_point and prevention_point must be in (0, 1000]")
	}
	if r.Block.Share <= 0 || r.Block.Share > 1 || r.Block.Offset < 0 || r.Block.Offset > 100000 {
		return fmt.Errorf("board_xp: block share must be in (0, 1] and offset in [0, 100000]")
	}
	for id, xp := range r.DefenseExtra {
		if xp < 0 || xp > 100000 {
			return fmt.Errorf("board_xp: defense_extra %s must be in [0, 100000]", id)
		}
	}
	return nil
}

// DefenseValue is what a defense does on one die.
type DefenseValue struct {
	// Prevention is a flat defense's average prevention per roll.
	Prevention float64 `json:"prevention"`
	// Block is a block defense's chance to prevent a whole attack per roll.
	Block float64 `json:"block"`
	// Blocks reports a block defense.
	Blocks bool `json:"blocks"`
}

// ValueDefense reads a defense's average prevention, or its block chance,
// on the die its owner rolls (the engine rolls the defender's own dice and
// matches outcomes by face number or by the face's role). A defense either
// blocks (some faces prevent BlockAmount, the rest nothing) or prevents flat
// amounts; it cannot mix the two.
func ValueDefense(ability content.BattleAbilityDefinition, die content.BattleDieDefinition) (DefenseValue, error) {
	if ability.Type != "defensive" || ability.Resolution == nil {
		return DefenseValue{}, fmt.Errorf("%q is not a defensive ability", ability.ID)
	}
	var roll *content.BattleOperation
	for i, op := range ability.Resolution.Operations {
		if op.Type == "roll_dice" {
			roll = &ability.Resolution.Operations[i]
		}
	}
	if roll == nil || roll.DiceCount < 1 {
		return DefenseValue{}, fmt.Errorf("%s must roll dice to be priced", ability.Name)
	}
	faces := map[int]int{} // face number → prevention
	for _, outcome := range roll.Outcomes {
		amount := 0
		for _, op := range outcome.Operations {
			if op.Type != "prevent_damage" {
				continue
			}
			n, ok := wholeAmount(op.Amount)
			if !ok {
				return DefenseValue{}, fmt.Errorf("%s: prevention amounts must be fixed to be priced", ability.Name)
			}
			amount += n
		}
		if len(outcome.Roles) > 0 {
			layout, err := content.DieRoles(die)
			if err != nil {
				return DefenseValue{}, err
			}
			for _, role := range outcome.Roles {
				for _, face := range layout.Faces[role] {
					faces[face] += amount
				}
			}
		}
		for _, face := range outcome.Faces {
			faces[face] += amount
		}
	}
	blockFaces, flat := 0, 0
	for face := 1; face <= sides; face++ {
		switch amount := faces[face]; {
		case amount >= BlockAmount:
			blockFaces++
		case amount > 0:
			flat += amount
		}
	}
	if blockFaces > 0 && flat > 0 {
		return DefenseValue{}, fmt.Errorf("%s mixes block faces with flat prevention; price it as one or the other", ability.Name)
	}
	if blockFaces > 0 {
		miss := 1 - float64(blockFaces)/sides
		return DefenseValue{Block: 1 - math.Pow(miss, float64(roll.DiceCount)), Blocks: true}, nil
	}
	return DefenseValue{Prevention: float64(flat) / sides * float64(roll.DiceCount)}, nil
}

func wholeAmount(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == math.Trunc(n) {
			return int(n), true
		}
	}
	return 0, false
}

// DefenseXP is a defense's price on a sheet with this total XP: flat
// defenses cost PreventionPoint per point of average prevention; blocks cost
// chance × Share × (total + Offset), so their price rises as the character
// earns XP. Prices round to the nearest XP.
func (r XPRates) DefenseXP(id string, v DefenseValue, total int) int {
	var xp float64
	if v.Blocks {
		xp = v.Block * r.Block.Share * float64(total+r.Block.Offset)
	} else {
		xp = v.Prevention * r.PreventionPoint
	}
	return int(math.Round(xp)) + r.DefenseExtra[id]
}

// BlockShare is the share of total XP a block defense's price grows by per
// XP earned (0 for flat defenses).
func (r XPRates) BlockShare(v DefenseValue) float64 {
	if !v.Blocks {
		return 0
	}
	return v.Block * r.Block.Share
}

// LevelXP is the XP a board's damage level holds: DamagePoint per point of
// average damage its step adds over the standard step, rounded to the
// nearest XP. The standard level is free, and so is every level below it.
func (r XPRates) LevelXP(sc Staircase, standard, level float64) int {
	if len(sc.Steps) == 0 {
		return 0
	}
	base := sc.Steps[sc.At(standard)].Average
	added := sc.Steps[sc.At(level)].Average - base
	return max(0, int(math.Round(added*r.DamagePoint)))
}
