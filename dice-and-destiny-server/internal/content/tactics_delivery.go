package content

import "fmt"

// EffectiveDelivery is an offensive ability's delivery in a tactics battle:
// its own, or the rules' default (direct melee) when it declares none.
func EffectiveDelivery(a BattleAbilityDefinition, fallback AbilityDelivery) AbilityDelivery {
	if a.Delivery != nil {
		d := *a.Delivery
		if d.Area != nil {
			area := *d.Area
			d.Area = &area
		}
		if d.Charge != nil {
			charge := *d.Charge
			d.Charge = &charge
		}
		return d
	}
	return fallback
}

// DeliveryText describes a delivery for rules text and tooltips: "Melee
// (range 1)", "Range 6, line of sight", "Area 3×3, range 5, line of sight",
// "Charge 4, line of sight".
func DeliveryText(d AbilityDelivery) string {
	sight := ""
	if d.LineOfSight {
		sight = ", line of sight"
	}
	switch {
	case d.Type == DeliveryTypeArea && d.Area != nil:
		side := 2*d.Area.Radius + 1
		return fmt.Sprintf("Area %d×%d, range %d%s", side, side, d.Range, sight)
	case d.Charge != nil:
		return fmt.Sprintf("Charge %d (target within %d)%s", d.Charge.Range, d.Charge.Range+1, sight)
	case d.Range <= 1:
		return "Melee (range 1)"
	}
	return fmt.Sprintf("Range %d%s", d.Range, sight)
}
