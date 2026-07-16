package battle

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/devhistory"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

type HistoryAuthorityConfig struct {
	BuildEnabled   bool
	RuntimeEnabled bool
	Gameplay       commandHandler
	Repository     repository.Repository
	Store          devhistory.Store
	IDGenerator    func(devhistory.Point) (string, error)
}

type HistoryAuthority struct{ config HistoryAuthorityConfig }

func NewHistoryAuthority(config HistoryAuthorityConfig) *HistoryAuthority {
	if config.IDGenerator == nil {
		config.IDGenerator = generateHistoryBattleID
	}
	return &HistoryAuthority{config: config}
}

func newDefaultHistoryAuthority(gameplay commandHandler, authority *Authority) *HistoryAuthority {
	_, sourceFile, _, ok := runtime.Caller(0)
	serverRoot := ""
	if ok {
		serverRoot = filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	}
	historyRoot := os.Getenv("DICE_AND_DESTINY_HISTORY_STATE_ROOT")
	if historyRoot == "" {
		historyRoot = filepath.Join(serverRoot, "save", "history")
	}
	var repo repository.Repository
	if authority != nil {
		repo = authority.repo
	}
	return NewHistoryAuthority(HistoryAuthorityConfig{
		BuildEnabled:   historyToolsBuildEnabled,
		RuntimeEnabled: os.Getenv("DICE_AND_DESTINY_ENABLE_HISTORY") == "1" || os.Getenv("DICE_AND_DESTINY_ENABLE_SNAPSHOTS") == "1",
		Gameplay:       gameplay, Repository: repo, Store: devhistory.Store{Root: historyRoot},
	})
}

func (authority *HistoryAuthority) HandleCommandJSON(commandJSON string) string {
	return handleCommand(commandJSON, authority)
}

func (authority *HistoryAuthority) HandleCommand(cmd command.Command) engine.Result {
	if authority == nil || authority.config.Gameplay == nil {
		return authorityRejected("history authority is not configured")
	}
	isHistory := cmd.Type == command.TypeListHistory || cmd.Type == command.TypeMarkHistory || cmd.Type == command.TypeJumpHistory || cmd.Type == command.TypeCommitHistory || cmd.Type == command.TypeReturnHistory || cmd.Type == command.TypeReplayHistory || cmd.Type == command.TypeDivergeHistory
	if isHistory && (!authority.config.BuildEnabled || !authority.config.RuntimeEnabled) {
		return authorityRejected("developer history tooling is disabled")
	}
	if authority.config.RuntimeEnabled {
		if branch, err := authority.config.Store.Branch(cmd.BattleID); err == nil && (branch.Status == devhistory.BranchReview || branch.Status == devhistory.BranchReplay || branch.Status == devhistory.BranchArchived) {
			allowed := isHistory || cmd.Type == command.TypeOpenBattle || cmd.Type == command.TypeListSnapshots || cmd.Type == command.TypeSaveSnapshot || cmd.Type == command.TypeLoadSnapshot
			if !allowed {
				if branch.Status == devhistory.BranchArchived {
					return authorityRejected("archived history future is read-only; jump to a history point to create a new branch")
				}
				if branch.Status == devhistory.BranchReplay {
					return authorityRejected("preserved history must be replayed through its recorded action or explicitly replaced")
				}
				return authorityRejected("history review is read-only; preserve or replace the future before continuing")
			}
		}
	}
	switch cmd.Type {
	case command.TypeListHistory:
		return authority.list(cmd)
	case command.TypeMarkHistory:
		return authority.mark(cmd)
	case command.TypeJumpHistory:
		return authority.jump(cmd)
	case command.TypeCommitHistory:
		return authority.commit(cmd)
	case command.TypeReturnHistory:
		return authority.returnLatest(cmd)
	case command.TypeReplayHistory:
		return authority.replay(cmd)
	case command.TypeDivergeHistory:
		return authority.diverge(cmd)
	default:
		return authority.config.Gameplay.HandleCommand(cmd)
	}
}

func (authority *HistoryAuthority) list(cmd command.Command) engine.Result {
	timeline, err := authority.config.Store.List(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"timeline": timeline}}
}

func (authority *HistoryAuthority) mark(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("history battle repository is not configured")
	}
	var payload command.MarkHistoryPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid mark_dev_history payload")
	}
	checkpoint, err := authority.config.Repository.Load(cmd.BattleID)
	if err != nil {
		return authorityRejected(fmt.Sprintf("load history battle: %v", err))
	}
	point, err := authority.config.Store.Mark(checkpoint, cmd.ActorID, payload.Label, payload.Kind, payload.PresentedSequence, payload.ClientState, payload.Action)
	if err != nil {
		return authorityRejected(err.Error())
	}
	timeline, err := authority.config.Store.List(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"point": point, "timeline": timeline}}
}

func (authority *HistoryAuthority) jump(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("history battle repository is not configured")
	}
	var payload command.JumpHistoryPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid jump_dev_history payload")
	}
	point, sourceBranch, err := authority.config.Store.LoadPointForBranch(cmd.BattleID, payload.PointID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if _, ok := point.Checkpoint.Battle.Actors[cmd.ActorID]; !ok {
		return authorityRejected("history actor is not a battle participant")
	}
	var battleID string
	var cloned repository.Checkpoint
	for attempt := 0; attempt < 4; attempt++ {
		battleID, err = authority.config.IDGenerator(point)
		if err != nil {
			return authorityRejected(fmt.Sprintf("generate history battle id: %v", err))
		}
		cloned, err = repository.CloneCheckpoint(point.Checkpoint, battleID)
		if err != nil {
			return authorityRejected(fmt.Sprintf("clone history point: %v", err))
		}
		err = authority.config.Repository.Create(cloned)
		if !errors.Is(err, repository.ErrBattleExists) {
			break
		}
	}
	if err != nil {
		return authorityRejected(fmt.Sprintf("create history review battle: %v", err))
	}
	branch, err := authority.config.Store.CreateReviewBranch(sourceBranch, battleID, point.Metadata.ID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened := authority.config.Gameplay.HandleCommand(command.Command{BattleID: battleID, ActorID: cmd.ActorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
	if !opened.Accepted {
		return opened
	}
	opened.Events = event.ForViewer(cloned.Events, cmd.ActorID)
	opened.Data = map[string]any{"history": map[string]any{
		"review": true, "point": point.Metadata, "branch": branch,
		"origin_battle_id": sourceBranch.BattleID, "presented_sequence": point.Metadata.PresentedSequence,
		"client_state": point.ClientState,
	}}
	return opened
}

func (authority *HistoryAuthority) commit(cmd command.Command) engine.Result {
	var payload command.CommitHistoryPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid commit_dev_history payload")
	}
	before, err := authority.config.Store.Branch(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	cursor := before.CursorPointID
	if cursor == "" {
		cursor = before.BasePointID
	}
	point, _, err := authority.config.Store.LoadPointForBranch(cmd.BattleID, cursor)
	if err != nil {
		return authorityRejected(err.Error())
	}
	branch, err := authority.config.Store.CommitReview(cmd.BattleID, payload.Mode)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened := authority.config.Gameplay.HandleCommand(command.Command{BattleID: cmd.BattleID, ActorID: cmd.ActorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
	if !opened.Accepted {
		return opened
	}
	// Opening a battle returns only the normal live delta. A history resume must
	// also return the checkpoint's complete visible event stream so the client
	// can reconstruct any presentation beat recorded at this cursor (for
	// example, Continue Presentation in ongoing effects or income).
	opened.Events = event.ForViewer(point.Checkpoint.Events, cmd.ActorID)
	timeline, err := authority.config.Store.List(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened.Data = map[string]any{"history": map[string]any{
		"review": false, "replay": branch.Status == devhistory.BranchReplay, "branch": branch,
		"timeline": timeline, "mode": payload.Mode, "point": point.Metadata,
		"presented_sequence": point.Metadata.PresentedSequence, "client_state": point.ClientState,
	}}
	return opened
}

func (authority *HistoryAuthority) returnLatest(cmd command.Command) engine.Result {
	branch, err := authority.config.Store.Branch(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if branch.Status != devhistory.BranchReview || branch.ParentBattleID == "" {
		return authorityRejected("history branch is not reviewing an earlier point")
	}
	latestBattleID, err := authority.config.Store.ResolveLatestBattleID(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened := authority.config.Gameplay.HandleCommand(command.Command{BattleID: latestBattleID, ActorID: cmd.ActorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
	if !opened.Accepted {
		return opened
	}
	checkpoint, err := authority.config.Repository.Load(latestBattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened.Data = map[string]any{"history": map[string]any{"review": false, "returned_to_latest": true, "presented_sequence": len(checkpoint.Events)}}
	return opened
}

func (authority *HistoryAuthority) replay(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("history battle repository is not configured")
	}
	var payload command.ReplayHistoryPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid replay_dev_history_action payload")
	}
	step, err := authority.config.Store.PrepareReplay(cmd.BattleID, payload.Action)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if !step.Matches {
		expectedLabel := step.Current.Metadata.ActionLabel
		if expectedLabel == "" {
			expectedLabel = step.Current.Metadata.Label
		}
		return engine.Result{Accepted: false, Error: "recorded history action differs; confirmation is required before replacing the future", Data: map[string]any{"history": map[string]any{
			"divergence": true, "expected_label": expectedLabel,
			"future_point_count": step.FutureCount, "point": step.Current.Metadata,
		}}}
	}
	var target repository.Checkpoint
	var presentedSequence uint64
	var clientState map[string]any
	var pointMetadata any
	if step.Next != nil {
		target = step.Next.Checkpoint
		presentedSequence = step.Next.Metadata.PresentedSequence
		clientState = step.Next.ClientState
		pointMetadata = step.Next.Metadata
	} else {
		latestBattleID, resolveErr := authority.config.Store.ResolveLatestBattleID(cmd.BattleID)
		if resolveErr != nil {
			return authorityRejected(resolveErr.Error())
		}
		target, err = authority.config.Repository.Load(latestBattleID)
		if err != nil {
			return authorityRejected(fmt.Sprintf("load latest history future: %v", err))
		}
		presentedSequence = uint64(len(target.Events))
	}
	cloned, err := repository.CloneCheckpoint(target, cmd.BattleID)
	if err != nil {
		return authorityRejected(fmt.Sprintf("clone replay history checkpoint: %v", err))
	}
	if err := authority.config.Repository.Save(cloned); err != nil {
		return authorityRejected(fmt.Sprintf("save replay history checkpoint: %v", err))
	}
	branch, err := authority.config.Store.AdvanceReplay(cmd.BattleID, step.Current.Metadata.ID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened := authority.config.Gameplay.HandleCommand(command.Command{BattleID: cmd.BattleID, ActorID: cmd.ActorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
	if !opened.Accepted {
		return opened
	}
	opened.Events = event.ForViewer(cloned.Events, cmd.ActorID)
	timeline, err := authority.config.Store.List(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	opened.Data = map[string]any{"history": map[string]any{
		"review": false, "replay": branch.Status == devhistory.BranchReplay, "branch": branch,
		"timeline": timeline, "point": pointMetadata, "presented_sequence": presentedSequence,
		"client_state": clientState,
	}}
	return opened
}

func (authority *HistoryAuthority) diverge(cmd command.Command) engine.Result {
	branch, err := authority.config.Store.TruncateReplay(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	timeline, err := authority.config.Store.List(cmd.BattleID)
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"history": map[string]any{
		"review": false, "replay": false, "branch": branch, "timeline": timeline,
	}}}
}

func generateHistoryBattleID(point devhistory.Point) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	prefix := "battle-history-"
	if point.Checkpoint.Battle.Origin.Kind == state.BattleOriginScenario {
		prefix = "scenario-history-"
	}
	label := strings.ToLower(point.Metadata.Kind)
	return prefix + label + "-" + hex.EncodeToString(suffix[:]), nil
}
