package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

type AuthoredCards struct {
	Trees    map[string]CardTree             `json:"trees,omitempty"`
	Deleted  []string                        `json:"deleted,omitempty"`
	Version  int                             `json:"version"`
	Revision int                             `json:"revision"`
	Cards    map[string]BattleCardDefinition `json:"cards"`
}

var authoredMu sync.Mutex

func ReadAuthoredCards(root string) (AuthoredCards, error) {
	out := AuthoredCards{Version: 1, Cards: map[string]BattleCardDefinition{}}
	if root == "" {
		return out, nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "authored_cards.json"))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&out); err != nil {
		return out, err
	}
	if out.Version != 1 || out.Cards == nil {
		return out, fmt.Errorf("invalid authored-card store")
	}
	return out, nil
}
func OverlayAuthoredCards(lib BattleLibrary, root string) (BattleLibrary, error) {
	saved, err := ReadAuthoredCards(root)
	if err != nil {
		return lib, err
	}
	abilities, err := ReadAuthoredAbilities(root)
	if err != nil {
		return lib, err
	}
	lib, err = applyAuthoredAbilities(lib, abilities)
	if err != nil {
		return lib, err
	}
	return applyAuthoredCards(lib, saved)
}
func applyAuthoredCards(lib BattleLibrary, saved AuthoredCards) (BattleLibrary, error) {
	lib.CardTrees = saved.Trees
	lib.Cards = maps.Clone(lib.Cards)
	lib.Statuses = maps.Clone(lib.Statuses)
	for id, c := range saved.Cards {
		if id != c.ID || (c.Program == nil && c.Mechanic == nil) {
			return lib, fmt.Errorf("invalid authored card %s", id)
		}
		if c.Mechanic != nil {
			lib.Cards[id] = PrepareMechanicCard(c, &lib)
		} else {
			lib.Cards[id] = PrepareProgramCard(c, &lib)
		}
	}
	for _, id := range saved.Deleted {
		delete(lib.Cards, id)
	}
	if err := validateBattleLibrary(lib); err != nil {
		return lib, err
	}
	if err := ValidateCardTrees(saved.Trees, lib); err != nil {
		return lib, err
	}
	return lib, nil
}

// SaveAuthoredCard validates the complete resulting catalog and atomically
// publishes one revision. Existing battles retain their serialized catalog.
func SaveAuthoredCard(root string, lib BattleLibrary, card BattleCardDefinition, revision int) (AuthoredCards, error) {
	authoredMu.Lock()
	defer authoredMu.Unlock()
	saved, err := ReadAuthoredCards(root)
	if err != nil {
		return saved, err
	}
	if root == "" {
		return saved, fmt.Errorf("authoring root required")
	}
	if saved.Revision != revision {
		return saved, fmt.Errorf("card catalog changed; reload before publishing")
	}
	if ProgramContains(saved.Deleted, card.ID) {
		return saved, fmt.Errorf("card ID %q was deleted; use a new ID", card.ID)
	}
	if owner, _, _ := TreeCardOwner(saved.Trees, card.ID); owner != "" {
		return saved, fmt.Errorf("edit this card through its progression tree %s", owner)
	}
	if err = validateStableNamed("card", card.ID, card.Name); err != nil {
		return saved, err
	}
	saved.Cards[card.ID] = card
	validated, err := applyAuthoredCards(lib, saved)
	if err != nil {
		return saved, err
	}
	saved.Cards[card.ID] = validated.Cards[card.ID]
	saved.Revision++
	return saved, writeAuthoredCards(root, saved)
}

func writeAuthoredCards(root string, saved AuthoredCards) error {
	var err error
	dir := filepath.Clean(root)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".authored-cards-*.json")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temp, filepath.Join(dir, "authored_cards.json")); err != nil {
		return err
	}
	return nil
}

// WithoutCard checks structural references without touching the caller's catalog.
func WithoutCard(lib BattleLibrary, id string) (BattleLibrary, error) {
	if _, ok := lib.Cards[id]; !ok {
		return lib, fmt.Errorf("unknown card %q", id)
	}
	lib.Cards = maps.Clone(lib.Cards)
	delete(lib.Cards, id)
	return lib, validateBattleLibrary(lib)
}

// DeleteAuthoredCard publishes a tombstone as well as removing any custom
// override. A deleted built-in must not reappear when source packs reload.
// The admin service additionally checks economy and saved-deck references.
func DeleteAuthoredCard(root string, lib BattleLibrary, id string, revision int) (AuthoredCards, error) {
	authoredMu.Lock()
	defer authoredMu.Unlock()
	saved, err := ReadAuthoredCards(root)
	if err != nil {
		return saved, err
	}
	if root == "" {
		return saved, fmt.Errorf("authoring root required")
	}
	if saved.Revision != revision {
		return saved, fmt.Errorf("card catalog changed; reopen Admin settings")
	}
	if _, err = WithoutCard(lib, id); err != nil {
		return saved, err
	}
	delete(saved.Cards, id)
	saved.Deleted = append(saved.Deleted, id)
	saved.Revision++
	return saved, writeAuthoredCards(root, saved)
}

// EditableGeneralCard translates existing effect semantics, not card names.
// Legacy definitions stay untouched until an author publishes a revision.
func EditableGeneralCard(c BattleCardDefinition) (BattleCardDefinition, error) {
	if c.Program != nil {
		return c, nil
	}
	p := &CardProgram{Version: 1, RollRequirement: "any"}
	for _, timing := range c.Play.PlayableDuring {
		w := ""
		switch timing.Segment {
		case "offensive":
			if timing.WindowPurpose == "planning" {
				w = "offensive_planning"
			} else {
				w = "offensive_reaction"
			}
		case "defensive":
			w = "defense_reaction"
		case "damage_resolution":
			w = "defense_selection"
		}
		if w != "" && !ProgramContains(p.Windows, w) {
			p.Windows = append(p.Windows, w)
		}
	}
	if c.Play.BeforeFirstRoll {
		p.RollRequirement = "before_first"
	}
	target := CardTarget{Owner: "self", Mode: "one", Count: 1, Selection: "choose"}
	destination := c.SavedCardDestination
	if destination == "" {
		destination = "original"
	}
	for _, op := range c.Operations {
		s := CardStep{Target: target, Params: map[string]any{}}
		switch op.Type {
		case "draw_cards":
			s.Effect = "draw"
			s.Params["amount"] = op.Amount
		case "gain_resource":
			s.Effect = "energy"
			s.Params["amount"] = op.Amount
		case "prevent_damage":
			s.Effect = "prevent"
			s.Params = map[string]any{"amount": op.Amount, "destination": destination}
			p.Windows = []string{"defense_selection", "damage_reaction"}
		case "modify_die":
			if op.Modification == "adjacent_non_six" {
				s.Effect = "adjust_die"
				s.Params = map[string]any{"deltas": []int{-1, 1}, "minimum": 1, "maximum": 5, "wrap": false}
				p.RollRequirement = "after_first"
			} else {
				s.Effect = "set_die"
				s.Params["faces"] = []int{op.Face}
			}
			if c.Targeting.Selector == "selected_die" {
				s.Target.Owner = "any"
				if c.Targeting.RequiredFace > 0 {
					s.Target.Faces = []int{c.Targeting.RequiredFace}
				}
			}
		case "reroll_die":
			s.Effect = "reroll"
			p.RollRequirement = "after_first"
		case "remove_status", "remove_status_stack":
			s.Effect = "remove_status"
			s.Target.Polarity = "negative"
			s.Params["stacks"] = op.StackCount
			p.Windows = []string{"offensive_planning", "defense_selection", "defense_reaction"}
		case "apply_ability_modifier":
			s.Effect = "ability_bonus"
			s.Target.Qualified = c.Targeting.RequiresQualified
			s.Params["damage"] = 0
			s.Params["duration"] = op.Duration
			s.Params["stack_limit"] = 100
			if op.Modifier == nil || op.Modifier.AddConditionalBonus == nil {
				return c, fmt.Errorf("unsupported modifier")
			}
			bonus := op.Modifier.AddConditionalBonus
			s.Condition = &bonus.Requirements
			for _, effect := range bonus.Operations {
				switch effect.Type {
				case "deal_damage":
					s.Params["damage"] = effect.Amount
				case "apply_status":
					s.Params["status_id"] = effect.StatusID
					s.Params["stacks"] = effect.StackCount
				default:
					return c, fmt.Errorf("unsupported modifier effect")
				}
			}
		case "general_card":
			switch op.Modification {
			case "copy_die":
				s.Effect = "copy_die"
				p.RollRequirement = "after_first"
			case "flip_die":
				s.Effect = "flip_die"
				p.RollRequirement = "after_first"
			case "reroll_enemy_die":
				s.Effect = "reroll"
				s.Target.Owner = "enemy"
				p.Windows = []string{"offensive_reaction"}
			case "reroll_defense_dice":
				s.Effect = "reroll_defense"
				s.Target.Mode = "up_to"
				s.Target.Count = 100
				p.Windows = []string{"defense_reaction"}
			case "recover_discard":
				s.Effect = "move_cards"
				s.Target.Zones = []string{"discard"}
				s.Target.ExcludeRecovery = true
				s.Params["destination"] = "hand"
			case "dispel_positive":
				s.Effect = "remove_status"
				s.Target.Owner = "enemy"
				s.Target.Polarity = "positive"
				s.Params["stacks"] = 1
				p.Windows = []string{"offensive_planning", "defense_selection", "defense_reaction"}
			case "save_threatened_card":
				s.Effect = "save_cards"
				s.Params["destination"] = destination
				p.Windows = []string{"defense_selection", "defense_reaction"}
			case "boost_prevention":
				s.Effect = "choice"
				amount := 0
				raw, _ := json.Marshal(op.Amount)
				_ = json.Unmarshal(raw, &amount)
				for i := 0; i < 2; i++ {
					s.Choices = append(s.Choices, CardOption{Name: fmt.Sprintf("Prevent %d", amount+i*op.BonusAmount), Energy: i * op.ExtraEnergy, Steps: []CardStep{{Effect: "prevent", Target: target, Params: map[string]any{"amount": amount + i*op.BonusAmount, "destination": destination}}}})
				}
				p.Windows = []string{"defense_selection", "defense_reaction", "damage_reaction"}
			default:
				return c, fmt.Errorf("unsupported general operation %s", op.Modification)
			}
		default:
			return c, fmt.Errorf("card is not in the supported General effect vocabulary: %s", op.Type)
		}
		p.Steps = append(p.Steps, s)
	}
	c.Program = p
	c.Operations = nil
	c.Play.BeforeFirstRoll = false
	c.Play.PlayableDuring = nil
	c.ReactionWindow = ReactionWindowDefinition{}
	c.Targeting = TargetingDefinition{Selector: "card_program", Minimum: 1, Maximum: 1}
	c.SavedCardDestination = ""
	c.Presentation.RulesText = CardProgramRules(p)
	c.Presentation.EffectSummary = c.Presentation.RulesText
	return c, nil
}

func ValidateAuthoredCard(c BattleCardDefinition, lib BattleLibrary) error {
	_, err := applyAuthoredCards(lib, AuthoredCards{Version: 1, Cards: map[string]BattleCardDefinition{c.ID: c}})
	return err
}
