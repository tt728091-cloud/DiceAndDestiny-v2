package tacticsbattle

import (
	"errors"

	"diceanddestiny/server/internal/battle/repository"
)

// writeBehind keeps one battle's checkpoint in memory for the length of a
// request: the hero's command and every enemy answer load and save it here,
// and the disk is written once, by flush. Engine.ApplyBattleCommand works on
// a clone, so a refused command never touches the held checkpoint.
type writeBehind struct {
	disk  *repository.Disk
	held  *repository.Checkpoint
	dirty bool
}

func (r *writeBehind) Create(checkpoint repository.Checkpoint) error {
	return r.disk.Create(checkpoint)
}

func (r *writeBehind) Load(battleID string) (repository.Checkpoint, error) {
	if r.held == nil || r.held.BattleID != battleID {
		checkpoint, err := r.disk.Load(battleID)
		if err != nil {
			return repository.Checkpoint{}, err
		}
		r.held, r.dirty = &checkpoint, false
	}
	return *r.held, nil
}

func (r *writeBehind) Save(checkpoint repository.Checkpoint) error {
	if r.held == nil || r.held.BattleID != checkpoint.BattleID {
		return errors.New("save before load")
	}
	r.held, r.dirty = &checkpoint, true
	return nil
}

// flush writes the held checkpoint to disk if a command changed it.
func (r *writeBehind) flush() error {
	if r.held == nil || !r.dirty {
		return nil
	}
	if err := r.disk.Save(*r.held); err != nil {
		return err
	}
	r.dirty = false
	return nil
}
