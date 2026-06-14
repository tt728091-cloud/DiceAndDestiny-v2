package operation

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/segment"
)

type Type string

const (
	TypeRollDice            Type = "roll_dice"
	TypeEvaluateRollOutcome Type = "evaluate_roll_outcome"
	TypeModifyDie           Type = "modify_die"
	TypeRerollDie           Type = "reroll_die"
	TypeDealDamage          Type = "deal_damage"
	TypePreventDamage       Type = "prevent_damage"
	TypeApplyStatus         Type = "apply_status"
	TypeRemoveStatusStack   Type = "remove_status_stack"
	TypeMoveCards           Type = "move_cards"
	TypeDrawCards           Type = "draw_cards"
	TypeGainResource        Type = "gain_resource"
	TypeChangeTarget        Type = "change_target"
	TypeNoop                Type = "noop"
)

type TargetSelector string

const (
	TargetSelf             TargetSelector = "self"
	TargetSourceActor      TargetSelector = "source_actor"
	TargetSelectedTargets  TargetSelector = "selected_targets"
	TargetTargetActor      TargetSelector = "target_actor"
	TargetSelectedProposal TargetSelector = "selected_proposal"
	TargetSelectedDie      TargetSelector = "selected_die"
	TargetSelectedStatus   TargetSelector = "selected_status"
)

type CardZone string

const (
	ZoneDeck    CardZone = "deck"
	ZoneHand    CardZone = "hand"
	ZoneDiscard CardZone = "discard"
	ZoneRemoved CardZone = "removed"
)

type ResourceType string

const ResourceEnergyPoints ResourceType = "energy_points"

type ReactionPolicy string

const (
	ReactionNone     ReactionPolicy = "none"
	ReactionStandard ReactionPolicy = "standard"
)

type DieModification string

const (
	DieModificationSetFace     DieModification = "set_face"
	DieModificationAdjustValue DieModification = "adjust_value"
)

type Trigger struct {
	Segment  segment.Segment `yaml:"segment" json:"segment"`
	Phase    string          `yaml:"phase" json:"phase"`
	Priority int             `yaml:"priority" json:"priority"`
}

type ReactionMetadata struct {
	Eligible bool           `yaml:"eligible" json:"eligible"`
	Policy   ReactionPolicy `yaml:"policy" json:"policy"`
}

type Definition struct {
	ID                string              `yaml:"id"`
	Type              Type                `yaml:"type"`
	Source            TargetSelector      `yaml:"source,omitempty"`
	Target            TargetSelector      `yaml:"target,omitempty"`
	Amount            *int                `yaml:"amount,omitempty"`
	StatusID          string              `yaml:"status_id,omitempty"`
	StackCount        *int                `yaml:"stack_count,omitempty"`
	DiceCount         *int                `yaml:"dice_count,omitempty"`
	SideCount         *int                `yaml:"side_count,omitempty"`
	OnePerStatusStack bool                `yaml:"one_per_status_stack,omitempty"`
	DieIndex          *int                `yaml:"die_index,omitempty"`
	Face              *int                `yaml:"face,omitempty"`
	Modification      DieModification     `yaml:"modification,omitempty"`
	SourceZone        CardZone            `yaml:"source_zone,omitempty"`
	DestinationZone   CardZone            `yaml:"destination_zone,omitempty"`
	Resource          ResourceType        `yaml:"resource,omitempty"`
	Reaction          *ReactionMetadata   `yaml:"reaction,omitempty"`
	Outcomes          []OutcomeDefinition `yaml:"outcomes,omitempty"`
	Operations        []Definition        `yaml:"operations,omitempty"`
}

type OutcomeDefinition struct {
	ID         string       `yaml:"id"`
	Faces      []int        `yaml:"faces,omitempty"`
	MinFace    *int         `yaml:"min_face,omitempty"`
	MaxFace    *int         `yaml:"max_face,omitempty"`
	Operations []Definition `yaml:"operations"`
}

type Plan struct {
	ID                string            `json:"id"`
	Type              Type              `json:"type"`
	Source            TargetSelector    `json:"source,omitempty"`
	Target            TargetSelector    `json:"target,omitempty"`
	Amount            *int              `json:"amount,omitempty"`
	StatusID          string            `json:"status_id,omitempty"`
	StackCount        *int              `json:"stack_count,omitempty"`
	DiceCount         *int              `json:"dice_count,omitempty"`
	SideCount         *int              `json:"side_count,omitempty"`
	OnePerStatusStack bool              `json:"one_per_status_stack,omitempty"`
	DieIndex          *int              `json:"die_index,omitempty"`
	Face              *int              `json:"face,omitempty"`
	Modification      DieModification   `json:"modification,omitempty"`
	SourceZone        CardZone          `json:"source_zone,omitempty"`
	DestinationZone   CardZone          `json:"destination_zone,omitempty"`
	Resource          ResourceType      `json:"resource,omitempty"`
	Reaction          *ReactionMetadata `json:"reaction,omitempty"`
	Outcomes          []OutcomePlan     `json:"outcomes,omitempty"`
	Operations        []Plan            `json:"operations,omitempty"`
}

type OutcomePlan struct {
	ID         string `json:"id"`
	Faces      []int  `json:"faces,omitempty"`
	MinFace    *int   `json:"min_face,omitempty"`
	MaxFace    *int   `json:"max_face,omitempty"`
	Operations []Plan `json:"operations"`
}

type CompileContext struct {
	Path string
}

type Handler interface {
	Type() Type
	Validate(def Definition) error
	Compile(ctx CompileContext, def Definition, compileNested func(string, []Definition) ([]Plan, error)) (Plan, error)
}

type Registry struct {
	handlers map[Type]Handler
}

func NewRegistry(handlers ...Handler) (*Registry, error) {
	registry := &Registry{handlers: make(map[Type]Handler, len(handlers))}
	for _, handler := range handlers {
		if handler == nil {
			return nil, errors.New("operation handler is required")
		}
		operationType := handler.Type()
		if operationType == "" {
			return nil, errors.New("operation handler type is required")
		}
		if _, exists := registry.handlers[operationType]; exists {
			return nil, fmt.Errorf("duplicate operation handler %q", operationType)
		}
		registry.handlers[operationType] = handler
	}
	return registry, nil
}

func DefaultRegistry() *Registry {
	handlers := make([]Handler, 0, len(allTypes()))
	for _, operationType := range allTypes() {
		handlers = append(handlers, standardHandler{operationType: operationType})
	}
	registry, err := NewRegistry(handlers...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (registry *Registry) Compile(path string, definitions []Definition) ([]Plan, error) {
	if registry == nil {
		return nil, errors.New("operation registry is required")
	}
	return registry.compileList(path, definitions)
}

func (registry *Registry) compileList(path string, definitions []Definition) ([]Plan, error) {
	plans := make([]Plan, len(definitions))
	seen := make(map[string]struct{}, len(definitions))
	for i, definition := range definitions {
		operationPath := fmt.Sprintf("%s.operations[%d]", path, i)
		if definition.ID == "" {
			definition.ID = operationPath
		}
		if _, exists := seen[definition.ID]; exists {
			return nil, fmt.Errorf("%s: duplicate operation id %q", operationPath, definition.ID)
		}
		seen[definition.ID] = struct{}{}
		handler, ok := registry.handlers[definition.Type]
		if !ok {
			return nil, fmt.Errorf("%s: no registered handler for operation type %q", operationPath, definition.Type)
		}
		if err := handler.Validate(definition); err != nil {
			return nil, fmt.Errorf("%s (%s): %w", operationPath, definition.ID, err)
		}
		plan, err := handler.Compile(
			CompileContext{Path: operationPath},
			definition,
			registry.compileList,
		)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", operationPath, definition.ID, err)
		}
		plans[i] = plan
	}
	return plans, nil
}

func ValidateTrigger(trigger Trigger) error {
	if !segment.IsValid(trigger.Segment) {
		return fmt.Errorf("unknown trigger segment %q", trigger.Segment)
	}
	switch trigger.Phase {
	case "on_enter", "in_progress", "on_exit":
		return nil
	default:
		return fmt.Errorf("unknown trigger phase %q", trigger.Phase)
	}
}

type EffectInstanceTrigger struct {
	Trigger
	CreationOrder int64
	InstanceID    string
}

func SortTriggers(values []EffectInstanceTrigger) {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Priority != values[j].Priority {
			return values[i].Priority > values[j].Priority
		}
		if values[i].CreationOrder != values[j].CreationOrder {
			return values[i].CreationOrder < values[j].CreationOrder
		}
		return values[i].InstanceID < values[j].InstanceID
	})
}

func MatchingTriggers(
	values []EffectInstanceTrigger,
	currentSegment segment.Segment,
	currentPhase string,
) []EffectInstanceTrigger {
	matches := make([]EffectInstanceTrigger, 0, len(values))
	for _, value := range values {
		if value.Segment == currentSegment && value.Phase == currentPhase {
			matches = append(matches, value)
		}
	}
	SortTriggers(matches)
	return matches
}

type standardHandler struct {
	operationType Type
}

func (handler standardHandler) Type() Type {
	return handler.operationType
}

func (handler standardHandler) Validate(def Definition) error {
	if def.Type != handler.operationType {
		return fmt.Errorf("handler %q cannot validate %q", handler.operationType, def.Type)
	}
	if err := validateSelectors(def); err != nil {
		return err
	}
	if def.Reaction != nil {
		switch def.Reaction.Policy {
		case ReactionNone, ReactionStandard:
		default:
			return fmt.Errorf("unknown reaction policy %q", def.Reaction.Policy)
		}
		if def.Reaction.Eligible && def.Reaction.Policy != ReactionStandard {
			return errors.New("reaction-eligible operations require standard reaction policy")
		}
		if !def.Reaction.Eligible && def.Reaction.Policy != ReactionNone {
			return errors.New("reaction-ineligible operations require none reaction policy")
		}
	}

	switch def.Type {
	case TypeRollDice:
		if err := rejectFields(def, fieldSet("source", "target", "dice_count", "side_count", "one_per_status_stack", "reaction", "operations")); err != nil {
			return err
		}
		if def.SideCount == nil || *def.SideCount < 2 {
			return errors.New("side_count must be at least 2")
		}
		if def.OnePerStatusStack == (def.DiceCount != nil) {
			return errors.New("exactly one of dice_count or one_per_status_stack is required")
		}
		if def.DiceCount != nil && *def.DiceCount < 1 {
			return errors.New("dice_count must be positive")
		}
	case TypeEvaluateRollOutcome:
		if err := rejectFields(def, fieldSet("source", "target", "outcomes", "reaction")); err != nil {
			return err
		}
		if len(def.Outcomes) == 0 {
			return errors.New("outcomes are required")
		}
		outcomeIDs := make(map[string]struct{}, len(def.Outcomes))
		coveredFaces := make(map[int]string)
		for i, outcome := range def.Outcomes {
			if outcome.ID == "" {
				return fmt.Errorf("outcomes[%d].id is required", i)
			}
			if _, exists := outcomeIDs[outcome.ID]; exists {
				return fmt.Errorf("duplicate outcome id %q", outcome.ID)
			}
			outcomeIDs[outcome.ID] = struct{}{}
			hasFaces := len(outcome.Faces) > 0
			hasRange := outcome.MinFace != nil || outcome.MaxFace != nil
			if hasFaces == hasRange {
				return fmt.Errorf("outcome %q requires either faces or min_face/max_face", outcome.ID)
			}
			if hasRange && (outcome.MinFace == nil || outcome.MaxFace == nil || *outcome.MinFace > *outcome.MaxFace) {
				return fmt.Errorf("outcome %q requires a valid min_face/max_face range", outcome.ID)
			}
			if len(outcome.Operations) == 0 {
				return fmt.Errorf("outcome %q operations are required", outcome.ID)
			}
			for _, face := range outcome.Faces {
				if face < 1 {
					return fmt.Errorf("outcome %q faces must be positive", outcome.ID)
				}
				if previous, exists := coveredFaces[face]; exists {
					return fmt.Errorf("outcomes %q and %q both match face %d", previous, outcome.ID, face)
				}
				coveredFaces[face] = outcome.ID
			}
		}
	case TypeModifyDie:
		if err := rejectFields(def, fieldSet("source", "target", "die_index", "face", "amount", "modification", "reaction")); err != nil {
			return err
		}
		if def.Target != TargetSelectedDie {
			return errors.New("target must be selected_die")
		}
		switch def.Modification {
		case DieModificationSetFace:
			if def.Face == nil || *def.Face < 1 || def.Amount != nil {
				return errors.New("set_face requires a positive face and no amount")
			}
		case DieModificationAdjustValue:
			if def.Amount == nil || def.Face != nil {
				return errors.New("adjust_value requires amount and no face")
			}
		default:
			return fmt.Errorf("unknown die modification %q", def.Modification)
		}
	case TypeRerollDie:
		if err := rejectFields(def, fieldSet("source", "target", "die_index", "reaction")); err != nil {
			return err
		}
		if def.Target != TargetSelectedDie {
			return errors.New("target must be selected_die")
		}
	case TypeDealDamage, TypePreventDamage:
		if err := rejectFields(def, fieldSet("source", "target", "amount", "reaction")); err != nil {
			return err
		}
		if err := requirePositiveAmount(def); err != nil {
			return err
		}
		if def.Target == "" {
			return errors.New("target is required")
		}
	case TypeApplyStatus:
		if err := rejectFields(def, fieldSet("source", "target", "status_id", "stack_count", "reaction")); err != nil {
			return err
		}
		if def.StatusID == "" {
			return errors.New("status_id is required")
		}
		if def.StackCount == nil || *def.StackCount < 1 {
			return errors.New("stack_count must be positive")
		}
		if def.Target == "" {
			return errors.New("target is required")
		}
	case TypeRemoveStatusStack:
		if err := rejectFields(def, fieldSet("source", "target", "status_id", "stack_count", "reaction")); err != nil {
			return err
		}
		if def.StackCount == nil || *def.StackCount < 1 {
			return errors.New("stack_count must be positive")
		}
		if def.Target != TargetSelectedStatus && def.StatusID == "" {
			return errors.New("status_id is required unless target is selected_status")
		}
	case TypeMoveCards:
		if err := rejectFields(def, fieldSet("source", "target", "amount", "source_zone", "destination_zone", "reaction")); err != nil {
			return err
		}
		if err := requirePositiveAmount(def); err != nil {
			return err
		}
		if !validZone(def.SourceZone) || !validZone(def.DestinationZone) {
			return errors.New("source_zone and destination_zone must be valid card zones")
		}
		if def.SourceZone == def.DestinationZone {
			return errors.New("source_zone and destination_zone must differ")
		}
		if def.Target == "" {
			return errors.New("target is required")
		}
	case TypeDrawCards:
		if err := rejectFields(def, fieldSet("source", "target", "amount", "source_zone", "reaction")); err != nil {
			return err
		}
		if err := requirePositiveAmount(def); err != nil {
			return err
		}
		if def.Target == "" {
			return errors.New("target is required")
		}
		if def.SourceZone != "" && def.SourceZone != ZoneDeck {
			return errors.New("draw_cards source_zone must be deck")
		}
	case TypeGainResource:
		if err := rejectFields(def, fieldSet("source", "target", "amount", "resource", "reaction")); err != nil {
			return err
		}
		if err := requirePositiveAmount(def); err != nil {
			return err
		}
		if def.Resource != ResourceEnergyPoints {
			return fmt.Errorf("unknown resource %q", def.Resource)
		}
		if def.Target == "" {
			return errors.New("target is required")
		}
	case TypeChangeTarget:
		if err := rejectFields(def, fieldSet("source", "target", "reaction")); err != nil {
			return err
		}
		if def.Source == "" || def.Target == "" {
			return errors.New("source and target are required")
		}
	case TypeNoop:
		if hasParameters(def) {
			return errors.New("noop does not accept parameters")
		}
	default:
		return fmt.Errorf("unknown operation type %q", def.Type)
	}
	return nil
}

func ClonePlans(values []Plan) []Plan {
	if values == nil {
		return nil
	}
	cloned := make([]Plan, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].Amount = copyInt(value.Amount)
		cloned[i].StackCount = copyInt(value.StackCount)
		cloned[i].DiceCount = copyInt(value.DiceCount)
		cloned[i].SideCount = copyInt(value.SideCount)
		cloned[i].DieIndex = copyInt(value.DieIndex)
		cloned[i].Face = copyInt(value.Face)
		cloned[i].Reaction = copyReaction(value.Reaction)
		cloned[i].Operations = ClonePlans(value.Operations)
		if value.Outcomes != nil {
			cloned[i].Outcomes = make([]OutcomePlan, len(value.Outcomes))
			for j, outcome := range value.Outcomes {
				cloned[i].Outcomes[j] = outcome
				cloned[i].Outcomes[j].Faces = append([]int(nil), outcome.Faces...)
				cloned[i].Outcomes[j].MinFace = copyInt(outcome.MinFace)
				cloned[i].Outcomes[j].MaxFace = copyInt(outcome.MaxFace)
				cloned[i].Outcomes[j].Operations = ClonePlans(outcome.Operations)
			}
		}
	}
	return cloned
}

func (handler standardHandler) Compile(
	ctx CompileContext,
	def Definition,
	compileNested func(string, []Definition) ([]Plan, error),
) (Plan, error) {
	plan := Plan{
		ID:                def.ID,
		Type:              def.Type,
		Source:            def.Source,
		Target:            def.Target,
		Amount:            copyInt(def.Amount),
		StatusID:          def.StatusID,
		StackCount:        copyInt(def.StackCount),
		DiceCount:         copyInt(def.DiceCount),
		SideCount:         copyInt(def.SideCount),
		OnePerStatusStack: def.OnePerStatusStack,
		DieIndex:          copyInt(def.DieIndex),
		Face:              copyInt(def.Face),
		Modification:      def.Modification,
		SourceZone:        def.SourceZone,
		DestinationZone:   def.DestinationZone,
		Resource:          def.Resource,
		Reaction:          copyReaction(def.Reaction),
	}
	var err error
	if len(def.Operations) > 0 {
		plan.Operations, err = compileNested(ctx.Path, def.Operations)
		if err != nil {
			return Plan{}, err
		}
	}
	for i, outcome := range def.Outcomes {
		operations, err := compileNested(
			fmt.Sprintf("%s.outcomes[%d]", ctx.Path, i),
			outcome.Operations,
		)
		if err != nil {
			return Plan{}, err
		}
		plan.Outcomes = append(plan.Outcomes, OutcomePlan{
			ID:         outcome.ID,
			Faces:      append([]int(nil), outcome.Faces...),
			MinFace:    copyInt(outcome.MinFace),
			MaxFace:    copyInt(outcome.MaxFace),
			Operations: operations,
		})
	}
	return plan, nil
}

func requirePositiveAmount(def Definition) error {
	if def.Amount == nil || *def.Amount < 1 {
		return errors.New("amount must be positive")
	}
	return nil
}

func validateSelectors(def Definition) error {
	if def.Source != "" && !validTarget(def.Source) {
		return fmt.Errorf("unknown source selector %q", def.Source)
	}
	if def.Target != "" && !validTarget(def.Target) {
		return fmt.Errorf("unknown target selector %q", def.Target)
	}
	if def.SourceZone != "" && !validZone(def.SourceZone) {
		return fmt.Errorf("unknown source zone %q", def.SourceZone)
	}
	if def.DestinationZone != "" && !validZone(def.DestinationZone) {
		return fmt.Errorf("unknown destination zone %q", def.DestinationZone)
	}
	if def.Resource != "" && def.Resource != ResourceEnergyPoints {
		return fmt.Errorf("unknown resource %q", def.Resource)
	}
	return nil
}

func validTarget(value TargetSelector) bool {
	switch value {
	case TargetSelf, TargetSourceActor, TargetSelectedTargets, TargetTargetActor,
		TargetSelectedProposal, TargetSelectedDie, TargetSelectedStatus:
		return true
	default:
		return false
	}
}

func validZone(value CardZone) bool {
	switch value {
	case ZoneDeck, ZoneHand, ZoneDiscard, ZoneRemoved:
		return true
	default:
		return false
	}
}

func hasParameters(def Definition) bool {
	return def.Source != "" || def.Target != "" || def.Amount != nil ||
		def.StatusID != "" || def.StackCount != nil || def.DiceCount != nil ||
		def.SideCount != nil || def.OnePerStatusStack || def.DieIndex != nil ||
		def.Face != nil || def.Modification != "" || def.SourceZone != "" ||
		def.DestinationZone != "" || def.Resource != "" || def.Reaction != nil ||
		len(def.Outcomes) > 0 || len(def.Operations) > 0
}

func fieldSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func rejectFields(def Definition, allowed map[string]bool) error {
	present := map[string]bool{
		"source": def.Source != "", "target": def.Target != "", "amount": def.Amount != nil,
		"status_id": def.StatusID != "", "stack_count": def.StackCount != nil,
		"dice_count": def.DiceCount != nil, "side_count": def.SideCount != nil,
		"one_per_status_stack": def.OnePerStatusStack, "die_index": def.DieIndex != nil,
		"face": def.Face != nil, "modification": def.Modification != "",
		"source_zone": def.SourceZone != "", "destination_zone": def.DestinationZone != "",
		"resource": def.Resource != "", "reaction": def.Reaction != nil,
		"outcomes": len(def.Outcomes) > 0, "operations": len(def.Operations) > 0,
	}
	for field, exists := range present {
		if exists && !allowed[field] {
			return fmt.Errorf("%s is not valid for %s", field, def.Type)
		}
	}
	return nil
}

func allTypes() []Type {
	return []Type{
		TypeRollDice,
		TypeEvaluateRollOutcome,
		TypeModifyDie,
		TypeRerollDie,
		TypeDealDamage,
		TypePreventDamage,
		TypeApplyStatus,
		TypeRemoveStatusStack,
		TypeMoveCards,
		TypeDrawCards,
		TypeGainResource,
		TypeChangeTarget,
		TypeNoop,
	}
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func copyReaction(value *ReactionMetadata) *ReactionMetadata {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
