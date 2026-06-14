package engine_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestNestedResolutionPersistsReloadsRevealsChainsAndResumes(t *testing.T) {
	battle := nestedResolutionBattle(t)
	flow := &nestedResolutionFlow{}
	eng := nestedResolutionEngine(t, engine.Config{
		ProposalRules: []engine.ProposalRule{adjustTokenRule{}},
		InteractionAI: fakeInteractionAI{},
	}, flow)

	started, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if started.Status != engine.ProgressWaitingForInput {
		t.Fatalf("status = %q, want waiting_for_input", started.Status)
	}
	if flow.progressCalls != 0 {
		t.Fatalf("segment Progress calls = %d, want resolution to suspend flow", flow.progressCalls)
	}
	resolution := battle.Resolutions[battle.ActiveResolutionID]
	window := resolution.Windows[resolution.ActiveWindowID]
	if resolution.Origin != (state.ResolutionCheckpoint{
		Segment: segment.OngoingEffects, Phase: state.FlowPhaseOnEnter, Stage: "fake_nested", Iteration: 7,
	}) {
		t.Fatalf("origin checkpoint = %#v", resolution.Origin)
	}
	if window.ActorProgress["enemy"] != state.InteractionActorCommitted ||
		window.ActorProgress["player"] != state.InteractionActorAwaiting {
		t.Fatalf("actor progress = %#v, want AI committed and human awaiting", window.ActorProgress)
	}
	assertHiddenCommitmentSafe(t, battle, started.Events, "player", "ai-secret-choice")

	repo := repository.NewInMemory()
	if err := repo.Create(repository.Checkpoint{Battle: battle, Events: started.Events}); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	loaded, err := repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	loadedResolution := loaded.Battle.Resolutions[loaded.Battle.ActiveResolutionID]
	loadedWindow := loadedResolution.Windows[loadedResolution.ActiveWindowID]
	loadedCommitment := loadedWindow.Commitments["enemy"]
	loadedCommitment.Data.ChoiceID = "mutated-loaded-state"
	loadedWindow.Commitments["enemy"] = loadedCommitment
	loadedResolution.Windows[loadedResolution.ActiveWindowID] = loadedWindow
	loaded.Battle.Resolutions[loaded.Battle.ActiveResolutionID] = loadedResolution
	for i := range loaded.Events {
		if loaded.Events[i].Commitment != nil {
			loaded.Events[i].Commitment.Data.ChoiceID = "mutated-loaded-event"
		}
	}
	loaded, err = repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("second Load() returned error: %v", err)
	}
	persistedWindow := activeInteractionWindow(t, loaded.Battle)
	if persistedWindow.Commitments["enemy"].Data.ChoiceID != "ai-secret-choice" ||
		!eventsContainPrivateCommitment(loaded.Events, "ai-secret-choice") {
		t.Fatal("repository load aliased persisted resolution or event state")
	}
	pending := loaded.Battle.Flow.PendingInput["player"]
	initialCommit := interactionCommitCommand(
		loaded.Battle.ID,
		"player",
		pending,
		command.InteractionCommitmentData{ChoiceID: "player-choice"},
	)
	progressed, err := eng.ApplyBattleCommand(&loaded.Battle, initialCommit)
	if err != nil {
		t.Fatalf("initial interaction command returned error: %v", err)
	}
	loaded.Events = append(loaded.Events, progressed.Events...)
	if err := repo.Save(loaded); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if loaded.Battle.ActiveResolutionID == "" {
		t.Fatal("resolution completed before reaction chain")
	}
	reactionOne := activeInteractionWindow(t, loaded.Battle)
	if reactionOne.Purpose != state.InteractionPurposeReaction ||
		reactionOne.ReactionRound != 1 || reactionOne.ChainDepth != 1 {
		t.Fatalf("first reaction window = %#v", reactionOne)
	}
	if !loaded.Battle.Resolutions[loaded.Battle.ActiveResolutionID].Batch.Revealed {
		t.Fatal("proposal batch was not revealed before reactions")
	}
	if !eventsContainCommitment(progressed.Events, "ai-secret-choice") ||
		!eventsContainCommitment(progressed.Events, "player-choice") {
		t.Fatalf("reveal events did not contain simultaneous commitments: %#v", progressed.Events)
	}

	pending = loaded.Battle.Flow.PendingInput["player"]
	reactionCommit := interactionCommitCommand(
		loaded.Battle.ID,
		"player",
		pending,
		command.InteractionCommitmentData{
			ProposalIDs: []string{"proposal-1"},
			ChoiceID:    "fake-reaction",
		},
	)
	progressed, err = eng.ApplyBattleCommand(&loaded.Battle, reactionCommit)
	if err != nil {
		t.Fatalf("reaction commitment returned error: %v", err)
	}
	reactionTwo := activeInteractionWindow(t, loaded.Battle)
	if reactionTwo.ReactionRound != 2 || reactionTwo.ChainDepth != 2 {
		t.Fatalf("next reaction window = %#v, want round/depth 2", reactionTwo)
	}
	if flow.progressCalls != 0 {
		t.Fatalf("segment resumed during reaction chain: %d calls", flow.progressCalls)
	}

	pending = loaded.Battle.Flow.PendingInput["player"]
	progressed, err = eng.ApplyBattleCommand(
		&loaded.Battle,
		interactionPassCommand(loaded.Battle.ID, "player", pending),
	)
	if err != nil {
		t.Fatalf("reaction pass returned error: %v", err)
	}
	if loaded.Battle.ActiveResolutionID != "" {
		t.Fatalf("active resolution = %q, want complete", loaded.Battle.ActiveResolutionID)
	}
	completed := loaded.Battle.Resolutions["resolution-1"]
	if completed.Stage != state.ResolutionComplete || !completed.Batch.Committed {
		t.Fatalf("completed resolution = %#v", completed)
	}
	if tokenValue(loaded.Battle.Actors["player"], "fake-counter") != 3 {
		t.Fatalf("fake counter = %d, want proposal amount 3", tokenValue(loaded.Battle.Actors["player"], "fake-counter"))
	}
	if flow.progressCalls != 1 || flow.observedStage != "fake_nested" || flow.observedIteration != 7 {
		t.Fatalf(
			"flow resume = calls %d stage %q iteration %d",
			flow.progressCalls,
			flow.observedStage,
			flow.observedIteration,
		)
	}
	if progressed.Status != engine.ProgressWaitingForInput {
		t.Fatalf("final status = %q, want final flow wait", progressed.Status)
	}
}

func TestInteractionCommandsRejectStaleBattleActorWindowStageIterationAndRound(t *testing.T) {
	base, eng := battleWaitingForInitialInteraction(t, engine.Config{
		ProposalRules: []engine.ProposalRule{adjustTokenRule{}},
		InteractionAI: fakeInteractionAI{},
	})
	pending := base.Flow.PendingInput["player"]

	tests := []struct {
		name   string
		mutate func(*command.Command)
		want   string
	}{
		{
			name: "battle",
			mutate: func(cmd *command.Command) {
				cmd.BattleID = "stale-battle"
			},
			want: "does not match",
		},
		{
			name: "actor",
			mutate: func(cmd *command.Command) {
				cmd.ActorID = "enemy"
			},
			want: "not human-controlled",
		},
		{
			name: "window",
			mutate: func(cmd *command.Command) {
				setInteractionCheckpoint(t, cmd, func(checkpoint *command.InteractionCheckpoint) {
					checkpoint.WindowID = "stale-window"
				})
			},
			want: "stale window checkpoint",
		},
		{
			name: "stage",
			mutate: func(cmd *command.Command) {
				setInteractionCheckpoint(t, cmd, func(checkpoint *command.InteractionCheckpoint) {
					checkpoint.Stage = "stale-stage"
				})
			},
			want: "stale window checkpoint",
		},
		{
			name: "iteration",
			mutate: func(cmd *command.Command) {
				setInteractionCheckpoint(t, cmd, func(checkpoint *command.InteractionCheckpoint) {
					checkpoint.Iteration++
				})
			},
			want: "stale window checkpoint",
		},
		{
			name: "reaction round",
			mutate: func(cmd *command.Command) {
				setInteractionCheckpoint(t, cmd, func(checkpoint *command.InteractionCheckpoint) {
					checkpoint.ReactionRound++
				})
			},
			want: "stale window checkpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			battle := base.Clone()
			cmd := interactionCommitCommand(
				battle.ID,
				"player",
				pending,
				command.InteractionCommitmentData{ChoiceID: "choice"},
			)
			tt.mutate(&cmd)
			before := battle.Clone()
			_, err := eng.ApplyBattleCommand(&battle, cmd)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ApplyBattleCommand() error = %v, want %q", err, tt.want)
			}
			if !reflect.DeepEqual(battle, before) {
				t.Fatal("rejected stale command mutated battle")
			}
		})
	}
}

func TestReactionRoundAndDepthLimitsReturnExplicitErrors(t *testing.T) {
	tests := []struct {
		name   string
		config engine.Config
		target error
	}{
		{
			name: "round limit",
			config: engine.Config{
				MaxReactionRounds:     1,
				MaxReactionChainDepth: 4,
			},
			target: engine.ErrReactionRoundLimit,
		},
		{
			name: "chain depth limit",
			config: engine.Config{
				MaxReactionRounds:     4,
				MaxReactionChainDepth: 1,
			},
			target: engine.ErrReactionChainDepthLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.ProposalRules = []engine.ProposalRule{adjustTokenRule{}}
			tt.config.InteractionAI = fakeInteractionAI{}
			battle, eng := battleWaitingForInitialInteraction(t, tt.config)
			pending := battle.Flow.PendingInput["player"]
			if _, err := eng.ApplyBattleCommand(
				&battle,
				interactionCommitCommand(
					battle.ID,
					"player",
					pending,
					command.InteractionCommitmentData{ChoiceID: "initial"},
				),
			); err != nil {
				t.Fatalf("initial commitment returned error: %v", err)
			}
			pending = battle.Flow.PendingInput["player"]
			before := battle.Clone()
			_, err := eng.ApplyBattleCommand(
				&battle,
				interactionCommitCommand(
					battle.ID,
					"player",
					pending,
					command.InteractionCommitmentData{ChoiceID: "react"},
				),
			)
			if err == nil || !errors.Is(err, tt.target) {
				t.Fatalf("limit error = %v, want %v", err, tt.target)
			}
			if !reflect.DeepEqual(battle, before) {
				t.Fatal("limit failure committed a reaction or auto-passed")
			}
		})
	}
}

func TestAutomaticProgressionLimitDuringResolutionIsExplicit(t *testing.T) {
	battle := nestedResolutionBattle(t)
	flow := &automaticReactionFlow{}
	eng := nestedResolutionEngine(t, engine.Config{
		MaxAutomaticSteps:     3,
		MaxReactionRounds:     100,
		MaxReactionChainDepth: 100,
		ProposalRules:         []engine.ProposalRule{adjustTokenRule{}},
		InteractionAI:         alwaysReactAI{},
	}, flow)

	_, err := eng.ProgressUntilInput(&battle)
	if err == nil || !errors.Is(err, engine.ErrAutomaticStepLimit) {
		t.Fatalf("ProgressUntilInput() error = %v, want ErrAutomaticStepLimit", err)
	}
	if strings.Contains(err.Error(), "pass") {
		t.Fatalf("automatic limit silently passed: %v", err)
	}
}

func TestInteractionPurposesUseOneWindowContract(t *testing.T) {
	purposes := []state.InteractionPurpose{
		state.InteractionPurposeRequiredRoll,
		state.InteractionPurposePlanning,
		state.InteractionPurposeReaction,
		state.InteractionPurposeChooseCard,
		state.InteractionPurposeSelectTarget,
	}
	for _, purpose := range purposes {
		t.Run(string(purpose), func(t *testing.T) {
			battle := nestedResolutionBattle(t)
			battle.Flow.Stage = "purpose"
			battle.Flow.Iteration = 1
			allowed := []command.Type{command.TypeCommitInteraction}
			passAllowed := false
			reactionRound := 0
			chainDepth := 0
			if purpose == state.InteractionPurposeReaction {
				allowed = append(allowed, command.TypePass)
				passAllowed = true
				reactionRound = 1
				chainDepth = 1
			}
			err := engine.BeginResolution(&engine.Context{
				Battle: &battle,
				Phase:  state.FlowPhaseInProgress,
			}, engine.ResolutionSpec{
				ID:    "purpose-resolution",
				Batch: fakeProposalBatch("purpose-resolution"),
				InitialWindow: engine.WindowSpec{
					ID:              "purpose-window",
					Purpose:         purpose,
					Source:          fakeSource(),
					EligibleActors:  []string{"player"},
					RequiredActors:  []string{"player"},
					AllowedCommands: allowed,
					PassAllowed:     passAllowed,
					ReactionRound:   reactionRound,
					ChainDepth:      chainDepth,
				},
			})
			if err != nil {
				t.Fatalf("BeginResolution() returned error: %v", err)
			}
			window := activeInteractionWindow(t, battle)
			if window.Purpose != purpose {
				t.Fatalf("purpose = %q, want %q", window.Purpose, purpose)
			}
		})
	}
}

func TestOnExitResolutionSuspendsAndAdvancesWithoutRepeatingExit(t *testing.T) {
	battle := nestedResolutionBattle(t)
	exitFlow := &exitResolutionFlow{}
	incomeFlow := &incomeWaitingFlow{}
	eng, err := engine.NewEngineWithConfig(
		engine.Config{
			ProposalRules: []engine.ProposalRule{adjustTokenRule{}},
			InteractionAI: fakeInteractionAI{},
		},
		exitFlow,
		incomeFlow,
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfig() returned error: %v", err)
	}

	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	resolution := battle.Resolutions[battle.ActiveResolutionID]
	if resolution.Origin.Phase != state.FlowPhaseOnExit ||
		battle.Segment.Current != segment.OngoingEffects ||
		!battle.Flow.ExitStarted {
		t.Fatalf("exit suspension state = resolution %#v flow %#v", resolution, battle.Flow)
	}
	if exitFlow.exitCalls != 1 {
		t.Fatalf("OnExit calls = %d, want 1", exitFlow.exitCalls)
	}

	pending := battle.Flow.PendingInput["player"]
	if _, err := eng.ApplyBattleCommand(
		&battle,
		interactionCommitCommand(
			battle.ID,
			"player",
			pending,
			command.InteractionCommitmentData{ChoiceID: "exit-choice"},
		),
	); err != nil {
		t.Fatalf("exit interaction command returned error: %v", err)
	}
	if exitFlow.exitCalls != 1 {
		t.Fatalf("OnExit repeated %d times", exitFlow.exitCalls)
	}
	if battle.Segment.Current != segment.Income || !incomeFlow.entered {
		t.Fatalf("segment = %q, income entered = %v", battle.Segment.Current, incomeFlow.entered)
	}
}

func nestedResolutionEngine(
	t *testing.T,
	config engine.Config,
	flow engine.SegmentFlow,
) engine.Engine {
	t.Helper()
	eng, err := engine.NewEngineWithConfig(config, flow)
	if err != nil {
		t.Fatalf("NewEngineWithConfig() returned error: %v", err)
	}
	return eng
}

func battleWaitingForInitialInteraction(
	t *testing.T,
	config engine.Config,
) (state.Battle, engine.Engine) {
	t.Helper()
	battle := nestedResolutionBattle(t)
	flow := &nestedResolutionFlow{}
	eng := nestedResolutionEngine(t, config, flow)
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	return battle, eng
}

func nestedResolutionBattle(t *testing.T) state.Battle {
	t.Helper()
	battle, err := state.NewBattleFromSetup("nested-battle", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "player",
				ControllerType: state.ControllerHuman,
				Tokens:         []state.TokenState{{ID: "fake-counter"}},
			},
			{
				ID:             "enemy",
				ControllerType: state.ControllerAI,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	return battle
}

type nestedResolutionFlow struct {
	progressCalls     int
	observedStage     string
	observedIteration int
}

func (*nestedResolutionFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*nestedResolutionFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "fake_nested"
	ctx.Battle.Flow.Iteration = 7
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorResolved}
	ctx.Battle.Flow.Actors["enemy"] = state.ActorFlowState{Status: state.ActorResolved}
	return nil, engine.BeginResolution(ctx, fakeResolutionSpec())
}

func (flow *nestedResolutionFlow) Progress(ctx *engine.Context) (engine.ProgressResult, error) {
	flow.progressCalls++
	flow.observedStage = ctx.Battle.Flow.Stage
	flow.observedIteration = ctx.Battle.Flow.Iteration
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "after-resolution",
		ActorID:         "player",
		Segment:         segment.OngoingEffects,
		Phase:           state.FlowPhaseInProgress,
		Stage:           ctx.Battle.Flow.Stage,
		Iteration:       ctx.Battle.Flow.Iteration,
		InputType:       "test_complete",
		AllowedCommands: []command.Type{command.TypeCommitInteraction},
	}
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (*nestedResolutionFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (*nestedResolutionFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

type automaticReactionFlow struct{}

func (*automaticReactionFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*automaticReactionFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "automatic_reaction"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["enemy"] = state.ActorFlowState{Status: state.ActorResolved}
	return nil, engine.BeginResolution(ctx, engine.ResolutionSpec{
		ID:    "automatic-resolution",
		Batch: fakeProposalBatch("automatic-resolution"),
		InitialWindow: engine.WindowSpec{
			ID:                "automatic-window",
			Purpose:           state.InteractionPurposeReaction,
			Source:            fakeSource(),
			EligibleActors:    []string{"enemy"},
			RequiredActors:    []string{"enemy"},
			AllowedCommands:   []command.Type{command.TypeCommitInteraction, command.TypePass},
			HiddenCommitments: true,
			PassAllowed:       true,
			ReactionRound:     1,
			ChainDepth:        1,
		},
		ReactionPolicy: fakeReactionPolicy([]string{"enemy"}),
	})
}

func (*automaticReactionFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressSegmentComplete}, nil
}

func (*automaticReactionFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (*automaticReactionFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

type exitResolutionFlow struct {
	exitCalls int
}

func (*exitResolutionFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*exitResolutionFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "exit_nested"
	ctx.Battle.Flow.Iteration = 5
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorResolved}
	ctx.Battle.Flow.Actors["enemy"] = state.ActorFlowState{Status: state.ActorResolved}
	return nil, nil
}

func (*exitResolutionFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressSegmentComplete}, nil
}

func (*exitResolutionFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (flow *exitResolutionFlow) OnExit(ctx *engine.Context) ([]event.Event, error) {
	flow.exitCalls++
	return nil, engine.BeginResolution(ctx, engine.ResolutionSpec{
		ID:    "exit-resolution",
		Batch: fakeProposalBatch("exit-resolution"),
		InitialWindow: engine.WindowSpec{
			ID:                "exit-window",
			Purpose:           state.InteractionPurposeChooseCard,
			Source:            fakeSource(),
			EligibleActors:    []string{"player"},
			RequiredActors:    []string{"player"},
			AllowedCommands:   []command.Type{command.TypeCommitInteraction},
			HiddenCommitments: true,
		},
	})
}

type incomeWaitingFlow struct {
	entered bool
}

func (*incomeWaitingFlow) ID() segment.Segment {
	return segment.Income
}

func (flow *incomeWaitingFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	flow.entered = true
	ctx.Battle.Flow.Stage = "income_wait"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "income-wait",
		ActorID:         "player",
		Segment:         segment.Income,
		Phase:           state.FlowPhaseInProgress,
		Stage:           "income_wait",
		Iteration:       1,
		InputType:       "test_wait",
		AllowedCommands: []command.Type{command.TypeCommitInteraction},
	}
	return nil, nil
}

func (*incomeWaitingFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (*incomeWaitingFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, errors.New("not implemented")
}

func (*incomeWaitingFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

func fakeResolutionSpec() engine.ResolutionSpec {
	actors := []string{"player", "enemy"}
	return engine.ResolutionSpec{
		ID:    "resolution-1",
		Batch: fakeProposalBatch("resolution-1"),
		InitialWindow: engine.WindowSpec{
			ID:                "resolution-1-window-1",
			Purpose:           state.InteractionPurposeChooseCard,
			Source:            fakeSource(),
			EligibleActors:    actors,
			RequiredActors:    actors,
			AllowedCommands:   []command.Type{command.TypeCommitInteraction},
			HiddenCommitments: true,
		},
		ReactionPolicy: fakeReactionPolicy(actors),
	}
}

func fakeReactionPolicy(actors []string) *state.ReactionWindowPolicy {
	return &state.ReactionWindowPolicy{
		Source:            fakeSource(),
		EligibleActors:    append([]string(nil), actors...),
		RequiredActors:    append([]string(nil), actors...),
		AllowedCommands:   []command.Type{command.TypeCommitInteraction, command.TypePass},
		HiddenCommitments: true,
		PassAllowed:       true,
	}
}

func fakeProposalBatch(resolutionID string) state.ProposalBatch {
	return state.ProposalBatch{
		ID:           "batch-1",
		ResolutionID: resolutionID,
		Proposals: []state.Proposal{
			{
				ID:     "proposal-1",
				Source: fakeSource(),
				Target: state.TargetReference{
					Type:    "token",
					ID:      "fake-counter",
					ActorID: "player",
				},
				Operation: state.ProposalOperationAdjustValue,
				Data: state.ProposalData{
					Amount: &state.AmountData{Value: 3},
				},
			},
		},
	}
}

func fakeSource() state.SourceReference {
	return state.SourceReference{
		Type:         "test_rule",
		ID:           "fake-source",
		ActorID:      "enemy",
		DefinitionID: "fake-definition",
	}
}

type fakeInteractionAI struct{}

func (fakeInteractionAI) Commit(
	_ *engine.Context,
	window state.InteractionWindow,
	actorID string,
) (state.InteractionCommitment, error) {
	if window.Purpose == state.InteractionPurposeReaction {
		return state.InteractionCommitment{
			ID:      "commit-" + window.ID + "-" + actorID,
			ActorID: actorID,
			Command: command.TypePass,
			Passed:  true,
		}, nil
	}
	return state.InteractionCommitment{
		ID:      "commit-" + window.ID + "-" + actorID,
		ActorID: actorID,
		Command: command.TypeCommitInteraction,
		Data: state.InteractionCommitmentData{
			ChoiceID: "ai-secret-choice",
		},
	}, nil
}

type alwaysReactAI struct{}

func (alwaysReactAI) Commit(
	_ *engine.Context,
	window state.InteractionWindow,
	actorID string,
) (state.InteractionCommitment, error) {
	return state.InteractionCommitment{
		ID:      "commit-" + window.ID + "-" + actorID,
		ActorID: actorID,
		Command: command.TypeCommitInteraction,
		Data: state.InteractionCommitmentData{
			ChoiceID: "automatic-reaction",
		},
	}, nil
}

type adjustTokenRule struct{}

func (adjustTokenRule) Operation() state.ProposalOperation {
	return state.ProposalOperationAdjustValue
}

func (adjustTokenRule) Validate(_ *engine.Context, proposal state.Proposal) error {
	if proposal.Target.ActorID == "" || proposal.Data.Amount == nil {
		return errors.New("adjust value requires actor target and amount")
	}
	return nil
}

func (adjustTokenRule) Apply(ctx *engine.Context, proposal state.Proposal) ([]event.Event, error) {
	actor, ok := ctx.Battle.Actors[proposal.Target.ActorID]
	if !ok {
		return nil, errors.New("target actor is missing")
	}
	for i := range actor.Tokens {
		if actor.Tokens[i].ID == proposal.Target.ID {
			actor.Tokens[i].Value += proposal.Data.Amount.Value
			ctx.Battle.Actors[proposal.Target.ActorID] = actor
			return nil, nil
		}
	}
	return nil, errors.New("target token is missing")
}

func activeInteractionWindow(t *testing.T, battle state.Battle) state.InteractionWindow {
	t.Helper()
	resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
	if !ok {
		t.Fatalf("active resolution %q is missing", battle.ActiveResolutionID)
	}
	window, ok := resolution.Windows[resolution.ActiveWindowID]
	if !ok {
		t.Fatalf("active window %q is missing", resolution.ActiveWindowID)
	}
	return window
}

func interactionCommitCommand(
	battleID string,
	actorID string,
	pending state.PendingInput,
	data command.InteractionCommitmentData,
) command.Command {
	payload, err := json.Marshal(command.CommitInteractionPayload{
		PendingInputID: pending.ID,
		Checkpoint: command.InteractionCheckpoint{
			WindowID:      pending.WindowID,
			Stage:         pending.Stage,
			Iteration:     pending.Iteration,
			ReactionRound: pending.ReactionRound,
		},
		Commitment: data,
	})
	if err != nil {
		panic(err)
	}
	return command.Command{
		BattleID: battleID,
		ActorID:  actorID,
		Type:     command.TypeCommitInteraction,
		Payload:  payload,
	}
}

func interactionPassCommand(
	battleID string,
	actorID string,
	pending state.PendingInput,
) command.Command {
	payload, err := json.Marshal(command.PassPayload{
		PendingInputID: pending.ID,
		Checkpoint: command.InteractionCheckpoint{
			WindowID:      pending.WindowID,
			Stage:         pending.Stage,
			Iteration:     pending.Iteration,
			ReactionRound: pending.ReactionRound,
		},
	})
	if err != nil {
		panic(err)
	}
	return command.Command{
		BattleID: battleID,
		ActorID:  actorID,
		Type:     command.TypePass,
		Payload:  payload,
	}
}

func setInteractionCheckpoint(
	t *testing.T,
	cmd *command.Command,
	mutate func(*command.InteractionCheckpoint),
) {
	t.Helper()
	var payload command.CommitInteractionPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}
	mutate(&payload.Checkpoint)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	cmd.Payload = encoded
}

func assertHiddenCommitmentSafe(
	t *testing.T,
	battle state.Battle,
	events []event.Event,
	viewerActorID string,
	secret string,
) {
	t.Helper()
	view := snapshot.FromBattleForViewer(battle, viewerActorID)
	viewPayload, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("Marshal(snapshot) returned error: %v", err)
	}
	if strings.Contains(string(viewPayload), secret) {
		t.Fatalf("snapshot leaked hidden commitment: %s", viewPayload)
	}
	filteredPayload, err := json.Marshal(event.ForViewer(events, viewerActorID))
	if err != nil {
		t.Fatalf("Marshal(events) returned error: %v", err)
	}
	if strings.Contains(string(filteredPayload), secret) {
		t.Fatalf("events leaked hidden commitment: %s", filteredPayload)
	}
	pendingPayload, err := json.Marshal(snapshot.PendingInputForViewer(battle, viewerActorID))
	if err != nil {
		t.Fatalf("Marshal(pending input) returned error: %v", err)
	}
	if strings.Contains(string(pendingPayload), secret) {
		t.Fatalf("pending input leaked hidden commitment: %s", pendingPayload)
	}
}

func eventsContainCommitment(events []event.Event, choiceID string) bool {
	for _, battleEvent := range events {
		for _, commitment := range battleEvent.Commitments {
			if commitment.Data.ChoiceID == choiceID {
				return true
			}
		}
	}
	return false
}

func eventsContainPrivateCommitment(events []event.Event, choiceID string) bool {
	for _, battleEvent := range events {
		if battleEvent.Commitment != nil && battleEvent.Commitment.Data.ChoiceID == choiceID {
			return true
		}
	}
	return false
}

func tokenValue(actor state.ActorState, tokenID string) int {
	for _, token := range actor.Tokens {
		if token.ID == tokenID {
			return token.Value
		}
	}
	return 0
}
