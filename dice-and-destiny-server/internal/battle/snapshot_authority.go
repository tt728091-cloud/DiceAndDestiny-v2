package battle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/devhistory"
	"diceanddestiny/server/internal/battle/devsnapshot"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

type SnapshotAuthorityConfig struct {
	BuildEnabled   bool
	RuntimeEnabled bool
	Gameplay       commandHandler
	Repository     repository.Repository
	Store          devsnapshot.Store
	HistoryStore   devhistory.Store
	IDGenerator    func(devsnapshot.Record) (string, error)
}

type SnapshotAuthority struct {
	config SnapshotAuthorityConfig
}

func NewSnapshotAuthority(config SnapshotAuthorityConfig) *SnapshotAuthority {
	if config.IDGenerator == nil {
		config.IDGenerator = generateSnapshotBattleID
	}
	return &SnapshotAuthority{config: config}
}

func newDefaultSnapshotAuthority(gameplay commandHandler, authority *Authority) *SnapshotAuthority {
	_, sourceFile, _, ok := runtime.Caller(0)
	serverRoot := ""
	if ok {
		serverRoot = filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	}
	snapshotRoot := os.Getenv("DICE_AND_DESTINY_SNAPSHOT_STATE_ROOT")
	if snapshotRoot == "" {
		snapshotRoot = filepath.Join(serverRoot, "save", "snapshots")
	}
	historyRoot := os.Getenv("DICE_AND_DESTINY_HISTORY_STATE_ROOT")
	if historyRoot == "" {
		historyRoot = filepath.Join(serverRoot, "save", "history")
	}
	var repo repository.Repository
	if authority != nil {
		repo = authority.repo
	}
	return NewSnapshotAuthority(SnapshotAuthorityConfig{
		BuildEnabled:   snapshotToolsBuildEnabled,
		RuntimeEnabled: os.Getenv("DICE_AND_DESTINY_ENABLE_SNAPSHOTS") == "1",
		Gameplay:       gameplay,
		Repository:     repo,
		Store:          devsnapshot.Store{Root: snapshotRoot},
		HistoryStore:   devhistory.Store{Root: historyRoot},
	})
}

func (authority *SnapshotAuthority) HandleCommandJSON(commandJSON string) string {
	return handleCommand(commandJSON, authority)
}

func (authority *SnapshotAuthority) HandleCommand(cmd command.Command) engine.Result {
	if authority == nil || authority.config.Gameplay == nil {
		return authorityRejected("snapshot authority is not configured")
	}
	switch cmd.Type {
	case command.TypeListSnapshots, command.TypeSaveSnapshot, command.TypeLoadSnapshot:
		if !authority.config.BuildEnabled || !authority.config.RuntimeEnabled {
			return authorityRejected("developer snapshot tooling is disabled")
		}
	}
	switch cmd.Type {
	case command.TypeListSnapshots:
		return authority.listSnapshots()
	case command.TypeSaveSnapshot:
		return authority.saveSnapshot(cmd)
	case command.TypeLoadSnapshot:
		return authority.loadSnapshot(cmd)
	default:
		return authority.config.Gameplay.HandleCommand(cmd)
	}
}

func (authority *SnapshotAuthority) listSnapshots() engine.Result {
	values, err := authority.config.Store.List()
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"snapshots": values}}
}

func (authority *SnapshotAuthority) saveSnapshot(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("snapshot battle repository is not configured")
	}
	var payload command.SaveSnapshotPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid save_dev_snapshot payload")
	}
	checkpoint, err := authority.config.Repository.Load(cmd.BattleID)
	if err != nil {
		return authorityRejected(fmt.Sprintf("load snapshot source battle: %v", err))
	}
	var history *devsnapshot.HistoryRecord
	if authority.config.HistoryStore.Root != "" {
		bundle, exportErr := authority.config.HistoryStore.Export(cmd.BattleID)
		if exportErr != nil {
			return authorityRejected(fmt.Sprintf("export snapshot history: %v", exportErr))
		}
		if bundle != nil {
			history = &devsnapshot.HistoryRecord{Bundle: *bundle}
			if bundle.Branch.Status != devhistory.BranchActive {
				latestBattleID, resolveErr := authority.config.HistoryStore.ResolveLatestBattleID(cmd.BattleID)
				if resolveErr != nil {
					return authorityRejected(fmt.Sprintf("resolve snapshot history future: %v", resolveErr))
				}
				latest, loadErr := authority.config.Repository.Load(latestBattleID)
				if loadErr != nil {
					return authorityRejected(fmt.Sprintf("load snapshot history future: %v", loadErr))
				}
				history.LatestCheckpoint = &latest
			}
		}
	}
	metadata, err := authority.config.Store.SaveWithHistory(payload.Name, cmd.ActorID, checkpoint, history, payload.Overwrite)
	if err != nil {
		if errors.Is(err, devsnapshot.ErrAlreadyExists) {
			return authorityRejected("developer snapshot already exists; enable overwrite to replace it")
		}
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"snapshot": metadata}}
}

func (authority *SnapshotAuthority) loadSnapshot(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("snapshot battle repository is not configured")
	}
	var payload command.LoadSnapshotPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid load_dev_snapshot payload")
	}
	record, err := authority.config.Store.Load(payload.Name)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if _, ok := record.Checkpoint.Battle.Actors[cmd.ActorID]; !ok {
		return authorityRejected("snapshot actor is not a battle participant")
	}
	var battleID string
	for attempt := 0; attempt < 4; attempt++ {
		battleID, err = authority.config.IDGenerator(record)
		if err != nil {
			return authorityRejected(fmt.Sprintf("generate snapshot battle id: %v", err))
		}
		cloned, cloneErr := repository.CloneCheckpoint(record.Checkpoint, battleID)
		if cloneErr != nil {
			return authorityRejected(fmt.Sprintf("clone developer snapshot: %v", cloneErr))
		}
		err = authority.config.Repository.Create(cloned)
		if !errors.Is(err, repository.ErrBattleExists) {
			break
		}
	}
	if err != nil {
		return authorityRejected(fmt.Sprintf("create battle from developer snapshot: %v", err))
	}
	var restoredHistory *devhistory.ImportResult
	if record.History != nil && authority.config.HistoryStore.Root != "" {
		latestBattleID := battleID
		if record.History.Bundle.Branch.Status != devhistory.BranchActive {
			latestBattleID = restoredLatestBattleID(battleID)
			latest, cloneErr := repository.CloneCheckpoint(*record.History.LatestCheckpoint, latestBattleID)
			if cloneErr != nil {
				return authorityRejected(fmt.Sprintf("clone snapshot history future: %v", cloneErr))
			}
			if createErr := authority.config.Repository.Create(latest); createErr != nil {
				return authorityRejected(fmt.Sprintf("create snapshot history future: %v", createErr))
			}
		}
		imported, importErr := authority.config.HistoryStore.Import(record.History.Bundle, battleID, latestBattleID)
		if importErr != nil {
			return authorityRejected(fmt.Sprintf("restore snapshot history: %v", importErr))
		}
		restoredHistory = &imported
	}
	opened := authority.config.Gameplay.HandleCommand(command.Command{
		BattleID: battleID,
		ActorID:  cmd.ActorID,
		Type:     command.TypeOpenBattle,
		Payload:  []byte(`{}`),
	})
	if !opened.Accepted {
		return opened
	}
	data := map[string]any{
		"loaded_snapshot":  record.Metadata,
		"source_battle_id": record.Checkpoint.BattleID,
		"new_battle_id":    battleID,
	}
	if restoredHistory != nil {
		timeline, listErr := authority.config.HistoryStore.List(battleID)
		if listErr != nil {
			return authorityRejected(fmt.Sprintf("list restored snapshot history: %v", listErr))
		}
		presentedSequence := uint64(len(record.Checkpoint.Events))
		var point any
		var clientState map[string]any
		if restoredHistory.Current.Metadata.ID != "" {
			presentedSequence = restoredHistory.Current.Metadata.PresentedSequence
			point = restoredHistory.Current.Metadata
			clientState = restoredHistory.Current.ClientState
		}
		data["history"] = map[string]any{
			"restored": true, "review": restoredHistory.Branch.Status == devhistory.BranchReview,
			"replay": restoredHistory.Branch.Status == devhistory.BranchReplay,
			"branch": restoredHistory.Branch, "timeline": timeline, "point": point,
			"presented_sequence": presentedSequence, "client_state": clientState,
			"origin_battle_id": restoredHistory.Branch.ParentBattleID,
		}
	}
	opened.Data = data
	return opened
}

func restoredLatestBattleID(battleID string) string {
	const suffix = "-latest"
	maxBase := 128 - len(suffix)
	if len(battleID) > maxBase {
		battleID = strings.TrimRight(battleID[:maxBase], ".-_")
	}
	return battleID + suffix
}

func generateSnapshotBattleID(record devsnapshot.Record) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	prefix := "battle-snapshot-"
	if record.Checkpoint.Battle.Origin.Kind == state.BattleOriginScenario {
		prefix = "scenario-snapshot-"
	}
	name := strings.Trim(record.Metadata.Name, ".-_")
	if len(name) > 48 {
		name = name[:48]
	}
	return prefix + name + "-" + hex.EncodeToString(suffix[:]), nil
}
