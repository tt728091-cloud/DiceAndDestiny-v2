package boardprice

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"sync"

	"diceanddestiny/server/internal/content"
)

// Choice is a player's board: a die, a template, a pick for each slot and the
// defenses. It is what the sheet stores and what board_price prices.
type Choice struct {
	Die      string     `json:"die"`
	Template string     `json:"template"`
	Slots    []SlotPick `json:"slots,omitempty"`
	Defenses []string   `json:"defenses,omitempty"`
}

// SlotPick is the cost (flex slots) and rider chosen for one template slot.
// Empty fields take the template's default.
type SlotPick struct {
	Slot  string `json:"slot"`
	Cost  string `json:"cost,omitempty"`
	Rider string `json:"rider,omitempty"`
}

// Builder prices and builds boards from the menus over a catalog holding the
// menu dice, defenses and rider statuses (the custom adventurer's).
type Builder struct {
	Menus Menus
	Lib   content.BattleLibrary

	mu    sync.Mutex
	chase map[string]float64
}

// NewBuilder loads and validates the menus against the catalog.
func NewBuilder(contentRoot string, lib content.BattleLibrary) (*Builder, error) {
	menus, err := LoadMenus(contentRoot, lib)
	if err != nil {
		return nil, err
	}
	return &Builder{Menus: menus, Lib: lib, chase: map[string]float64{}}, nil
}

// Die returns a menu die ready to price.
func (b *Builder) Die(id string) (Die, error) {
	if !b.Menus.OffersDie(id) {
		return Die{}, fmt.Errorf("%q is not on the die menu", id)
	}
	def, ok := b.Lib.Dice[id]
	if !ok {
		return Die{}, fmt.Errorf("unknown die %q", id)
	}
	return NewDie(def)
}

// Chase is a cost's chance on a die when every keep chases it (its first
// tier for a ladder), cached per die.
func (b *Builder) Chase(die Die, cost Cost) float64 {
	key := die.ID + "|" + cost[0].String()
	b.mu.Lock()
	if odds, ok := b.chase[key]; ok {
		b.mu.Unlock()
		return odds
	}
	b.mu.Unlock()
	odds := newSolver(die, []Tier{cost[0]}, b.Menus.Pricing.Dice).chase(0, 1, b.Menus.Pricing.Rolls)
	b.mu.Lock()
	b.chase[key] = odds
	b.mu.Unlock()
	return odds
}

// Band names the band a chance falls in.
func (b *Builder) Band(odds float64) string {
	ids := make([]string, 0, len(b.Menus.Bands))
	for id := range b.Menus.Bands {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if b.Menus.Bands[id].Contains(odds) {
			return id
		}
	}
	return ""
}

// FlexCosts lists the costs a flex slot may take on a die: its family menu
// inside the slot's bands.
func (b *Builder) FlexCosts(die Die, family string, slot TemplateSlot) []Cost {
	var out []Cost
	for _, entry := range b.Menus.Families[family].FlexCosts {
		cost, err := ParseCost(entry.Cost)
		if err != nil || entry.Unlock != UnlockOpen {
			continue
		}
		if slices.Contains(slot.Bands, b.Band(b.Chase(die, cost))) {
			out = append(out, cost)
		}
	}
	return out
}

// Default is a template at its default picks on a die: default costs, no
// riders and the first defenses on the menu.
func (b *Builder) Default(templateID, dieID string) Choice {
	return Choice{Die: dieID, Template: templateID}
}

// Resolve fills a choice's defaults and checks it against the menus: the die
// and template are on the menus and of one family; core slots keep their
// cost; flex costs come from the family menu inside the slot's bands on this
// die; riders exist and fit the slot's job; no tier repeats; and the board
// carries the template's number of distinct menu defenses. It returns the
// complete choice, the die and the board to price.
func (b *Builder) Resolve(c Choice) (Choice, Die, Board, error) {
	tpl, ok := b.Menus.Template(c.Template)
	if !ok {
		return c, Die{}, Board{}, fmt.Errorf("unknown board template %q", c.Template)
	}
	die, err := b.Die(c.Die)
	if err != nil {
		return c, Die{}, Board{}, err
	}
	if die.Layout.Family != tpl.Family {
		return c, Die{}, Board{}, fmt.Errorf("%s is a %s die; the %s template is for %s dice", die.Name, die.Layout.Order, tpl.Name, familyOrders(tpl.Family))
	}
	picks := map[string]SlotPick{}
	for _, p := range c.Slots {
		if _, dup := picks[p.Slot]; dup {
			return c, Die{}, Board{}, fmt.Errorf("slot %s is picked twice", p.Slot)
		}
		if !slices.ContainsFunc(tpl.Slots, func(s TemplateSlot) bool { return s.ID == p.Slot }) {
			return c, Die{}, Board{}, fmt.Errorf("the %s template has no slot %q", tpl.Name, p.Slot)
		}
		picks[p.Slot] = p
	}
	out := Choice{Die: c.Die, Template: c.Template}
	var board Board
	used := map[string]string{}
	for _, slot := range tpl.Slots {
		pick := picks[slot.ID]
		cost, _ := ParseCost(slot.Cost)
		if pick.Cost != "" {
			chosen, err := ParseCost(pick.Cost)
			if err != nil {
				return c, Die{}, Board{}, fmt.Errorf("slot %s: %w", slot.ID, err)
			}
			if !slot.Flex() && chosen.String() != cost.String() {
				return c, Die{}, Board{}, fmt.Errorf("slot %s is a core slot; its cost stays %s", slot.ID, cost)
			}
			cost = chosen
		}
		if slot.Flex() {
			offered := false
			for _, entry := range b.Menus.Families[tpl.Family].FlexCosts {
				if parsed, _ := ParseCost(entry.Cost); parsed.String() == cost.String() && entry.Unlock == UnlockOpen {
					offered = true
				}
			}
			if !offered {
				return c, Die{}, Board{}, fmt.Errorf("slot %s: %s is not on the %s menu", slot.ID, cost, familyOrders(tpl.Family))
			}
			odds := b.Chase(die, cost)
			if band := b.Band(odds); !slices.Contains(slot.Bands, band) {
				return c, Die{}, Board{}, fmt.Errorf("slot %s takes %v costs; %s is %s (%.0f%%) on the %s", slot.ID, slot.Bands, cost, band, odds*100, die.Name)
			}
		}
		for _, tier := range cost {
			if other, dup := used[tier.String()]; dup {
				return c, Die{}, Board{}, fmt.Errorf("slots %s and %s both need %s", other, slot.ID, tier)
			}
			used[tier.String()] = slot.ID
		}
		riderID := pick.Rider
		if riderID == "" {
			riderID = RiderNone
		}
		rider, ok := b.Menus.Rider(riderID)
		if !ok || rider.Unlock != UnlockOpen {
			return c, Die{}, Board{}, fmt.Errorf("slot %s: unknown rider %q", slot.ID, riderID)
		}
		if riderID != RiderNone && (!slot.TakesRider() || !rider.RiderFits(slot.Job)) {
			return c, Die{}, Board{}, fmt.Errorf("slot %s (%s) cannot take %s", slot.ID, b.Menus.Jobs[slot.Job].Name, rider.Name)
		}
		out.Slots = append(out.Slots, SlotPick{Slot: slot.ID, Cost: cost.String(), Rider: riderID})
		board.Slots = append(board.Slots, Slot{ID: slot.ID, Job: slot.Job, Cost: cost, RiderID: riderID, Rider: rider.Operations})
	}
	defenses := c.Defenses
	if len(defenses) == 0 {
		for _, d := range b.Menus.Defenses[:tpl.Defenses] {
			defenses = append(defenses, d.Ability)
		}
	}
	if len(defenses) != tpl.Defenses {
		return c, Die{}, Board{}, fmt.Errorf("the %s template carries %d defense(s)", tpl.Name, tpl.Defenses)
	}
	for i, id := range defenses {
		if !b.Menus.OffersDefense(id) || slices.Contains(defenses[:i], id) {
			return c, Die{}, Board{}, fmt.Errorf("defense %q is not on the menu or is picked twice", id)
		}
	}
	out.Defenses = append([]string(nil), defenses...)
	return out, die, board, nil
}

// Price resolves and prices a choice at the menus' agreed pricing.
func (b *Builder) Price(c Choice) (Choice, Report, error) {
	resolved, die, board, err := b.Resolve(c)
	if err != nil {
		return c, Report{}, err
	}
	report, err := Price(die, board, b.Menus.Options())
	return resolved, report, err
}

// RandomBoard draws a seeded random board of a family (any family when
// empty): a menu die and template, a cost for each flex slot from its band,
// a rider for every slot that takes one and the defenses. The AI work trains
// on boards drawn this way.
func (b *Builder) RandomBoard(rng *rand.Rand, family string) (Choice, error) {
	var dice []Die
	for _, entry := range b.Menus.Dice {
		die, err := b.Die(entry.ID)
		if err != nil {
			return Choice{}, err
		}
		if family == "" || die.Layout.Family == family {
			dice = append(dice, die)
		}
	}
	if len(dice) == 0 {
		return Choice{}, fmt.Errorf("no menu die of family %q", family)
	}
	die := dice[rng.Intn(len(dice))]
	var templates []Template
	for _, t := range b.Menus.Templates {
		if t.Family == die.Layout.Family {
			templates = append(templates, t)
		}
	}
	tpl := templates[rng.Intn(len(templates))]
	choice := Choice{Die: die.ID, Template: tpl.ID}
	// A tier may appear only once on a board: flex costs avoid the core
	// slots' tiers and each other.
	used := map[string]bool{}
	for _, slot := range tpl.Slots {
		if !slot.Flex() {
			cost, _ := ParseCost(slot.Cost)
			for _, tier := range cost {
				used[tier.String()] = true
			}
		}
	}
	for _, slot := range tpl.Slots {
		pick := SlotPick{Slot: slot.ID}
		if slot.Flex() {
			var open []Cost
			for _, cost := range b.FlexCosts(die, tpl.Family, slot) {
				if !used[cost[0].String()] {
					open = append(open, cost)
				}
			}
			if len(open) == 0 {
				return Choice{}, fmt.Errorf("slot %s of %s has no free cost on %s", slot.ID, tpl.ID, die.ID)
			}
			cost := open[rng.Intn(len(open))]
			used[cost[0].String()] = true
			pick.Cost = cost.String()
		}
		if slot.TakesRider() {
			var riders []string
			for _, r := range b.Menus.Riders {
				if r.Unlock == UnlockOpen && r.RiderFits(slot.Job) {
					riders = append(riders, r.ID)
				}
			}
			pick.Rider = riders[rng.Intn(len(riders))]
		}
		choice.Slots = append(choice.Slots, pick)
	}
	for _, i := range rng.Perm(len(b.Menus.Defenses))[:tpl.Defenses] {
		choice.Defenses = append(choice.Defenses, b.Menus.Defenses[i].Ability)
	}
	resolved, _, _, err := b.Resolve(choice)
	return resolved, err
}

func familyOrders(family string) string {
	switch family {
	case content.FamilyThreeTwoOne:
		return "3-2-1 and 2-3-1"
	case content.FamilyThreeOneOneOne:
		return "3-1-1-1"
	case content.FamilyTwoTwoOneOne:
		return "2-2-1-1 and 2-1-2-1"
	}
	return family
}
