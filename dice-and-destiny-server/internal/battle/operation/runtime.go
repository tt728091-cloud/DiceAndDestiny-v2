package operation

import (
	"errors"
	"fmt"
)

type RuntimeContext struct {
	ProposalID               string
	SourcePlanningProposalID string
	SourceActorID            string
	SourceContentType        string
	SourceContentID          string
	SelectedTargets          []string
}

type RuntimeProposal struct {
	ID                       string
	Type                     Type
	SourcePlanningProposalID string
	SourceActorID            string
	SourceContentType        string
	SourceContentID          string
	TargetActorID            string
	TargetProposalIDs        []string
	Amount                   int
	OriginatingOperation     Plan
}

type RuntimeHandler interface {
	Type() Type
	Execute(ctx RuntimeContext, plan Plan) ([]RuntimeProposal, error)
}

type RuntimeRegistry struct {
	handlers map[Type]RuntimeHandler
}

func NewRuntimeRegistry(handlers ...RuntimeHandler) (*RuntimeRegistry, error) {
	registry := &RuntimeRegistry{handlers: make(map[Type]RuntimeHandler, len(handlers))}
	for _, handler := range handlers {
		if handler == nil {
			return nil, errors.New("operation runtime handler is required")
		}
		operationType := handler.Type()
		if operationType == "" {
			return nil, errors.New("operation runtime handler type is required")
		}
		if _, exists := registry.handlers[operationType]; exists {
			return nil, fmt.Errorf("duplicate operation runtime handler %q", operationType)
		}
		registry.handlers[operationType] = handler
	}
	return registry, nil
}

func DefaultRuntimeRegistry() *RuntimeRegistry {
	registry, err := NewRuntimeRegistry(
		damageRuntimeHandler{operationType: TypeDealDamage},
		damageRuntimeHandler{operationType: TypePreventDamage},
	)
	if err != nil {
		panic(err)
	}
	return registry
}

func (registry *RuntimeRegistry) Execute(ctx RuntimeContext, plan Plan) ([]RuntimeProposal, error) {
	if registry == nil {
		return nil, errors.New("operation runtime registry is required")
	}
	handler, ok := registry.handlers[plan.Type]
	if !ok {
		return nil, fmt.Errorf("no runtime handler registered for operation type %q", plan.Type)
	}
	return handler.Execute(ctx, plan)
}

func (registry *RuntimeRegistry) Supports(operationType Type) bool {
	if registry == nil {
		return false
	}
	_, ok := registry.handlers[operationType]
	return ok
}

type damageRuntimeHandler struct {
	operationType Type
}

func (handler damageRuntimeHandler) Type() Type {
	return handler.operationType
}

func (handler damageRuntimeHandler) Execute(ctx RuntimeContext, plan Plan) ([]RuntimeProposal, error) {
	if plan.Type != handler.operationType {
		return nil, fmt.Errorf("runtime handler %q cannot execute %q", handler.operationType, plan.Type)
	}
	if plan.Amount == nil || *plan.Amount < 1 {
		return nil, errors.New("damage operation amount must be positive")
	}

	base := RuntimeProposal{
		ID:                       ctx.ProposalID,
		Type:                     plan.Type,
		SourcePlanningProposalID: ctx.SourcePlanningProposalID,
		SourceActorID:            ctx.SourceActorID,
		SourceContentType:        ctx.SourceContentType,
		SourceContentID:          ctx.SourceContentID,
		Amount:                   *plan.Amount,
		OriginatingOperation:     ClonePlans([]Plan{plan})[0],
	}

	switch plan.Type {
	case TypeDealDamage:
		targets, err := runtimeActorTargets(ctx, plan.Target)
		if err != nil {
			return nil, err
		}
		proposals := make([]RuntimeProposal, len(targets))
		for i, targetID := range targets {
			proposals[i] = base
			proposals[i].ID = fmt.Sprintf("%s-target-%s", ctx.ProposalID, targetID)
			proposals[i].TargetActorID = targetID
		}
		return proposals, nil
	case TypePreventDamage:
		if plan.Target != TargetSelectedProposal {
			return nil, fmt.Errorf("prevent_damage runtime target %q is not supported", plan.Target)
		}
		if len(ctx.SelectedTargets) == 0 {
			return nil, errors.New("prevent_damage requires selected proposal targets")
		}
		base.TargetProposalIDs = append([]string(nil), ctx.SelectedTargets...)
		return []RuntimeProposal{base}, nil
	default:
		return nil, fmt.Errorf("unsupported damage runtime operation %q", plan.Type)
	}
}

func runtimeActorTargets(ctx RuntimeContext, selector TargetSelector) ([]string, error) {
	switch selector {
	case TargetSelf, TargetSourceActor:
		if ctx.SourceActorID == "" {
			return nil, errors.New("source actor is required")
		}
		return []string{ctx.SourceActorID}, nil
	case TargetSelectedTargets, TargetTargetActor:
		if len(ctx.SelectedTargets) == 0 {
			return nil, errors.New("damage operation requires selected actor targets")
		}
		return append([]string(nil), ctx.SelectedTargets...), nil
	default:
		return nil, fmt.Errorf("damage runtime target %q is not supported", selector)
	}
}
