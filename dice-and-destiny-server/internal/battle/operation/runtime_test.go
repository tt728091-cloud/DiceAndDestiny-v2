package operation_test

import (
	"testing"

	"diceanddestiny/server/internal/battle/operation"
)

func TestDamageRuntimeRegistryExecutesCompiledPlansWithoutMutation(t *testing.T) {
	amount := 2
	deal := operation.Plan{
		ID:     "deal",
		Type:   operation.TypeDealDamage,
		Target: operation.TargetSelectedTargets,
		Amount: &amount,
	}
	registry := operation.DefaultRuntimeRegistry()
	proposals, err := registry.Execute(operation.RuntimeContext{
		ProposalID:               "final-damage",
		SourcePlanningProposalID: "attack",
		SourceActorID:            "player",
		SourceContentType:        "ability",
		SourceContentID:          "strike",
		SelectedTargets:          []string{"enemy-one", "enemy-two"},
	}, deal)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(proposals) != 2 ||
		proposals[0].TargetActorID != "enemy-one" ||
		proposals[1].TargetActorID != "enemy-two" ||
		proposals[0].OriginatingOperation.Type != operation.TypeDealDamage {
		t.Fatalf("runtime proposals = %#v", proposals)
	}
	if deal.ID != "deal" || *deal.Amount != 2 {
		t.Fatalf("runtime execution mutated compiled plan: %#v", deal)
	}

	prevent := operation.Plan{
		ID:     "prevent",
		Type:   operation.TypePreventDamage,
		Target: operation.TargetSelectedProposal,
		Amount: &amount,
	}
	proposals, err = registry.Execute(operation.RuntimeContext{
		ProposalID:      "final-prevent",
		SourceActorID:   "enemy-one",
		SelectedTargets: []string{"attack"},
	}, prevent)
	if err != nil {
		t.Fatalf("prevent Execute() returned error: %v", err)
	}
	if len(proposals) != 1 || proposals[0].TargetProposalIDs[0] != "attack" {
		t.Fatalf("prevent runtime proposal = %#v", proposals)
	}
}
