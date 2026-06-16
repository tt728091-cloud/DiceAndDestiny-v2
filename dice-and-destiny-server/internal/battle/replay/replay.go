package replay

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/contentpin"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
)

var ErrContentChanged = errors.New("replay content fingerprint changed")

type Reader struct {
	repository repository.Repository
}

type Request struct {
	BattleID                   string
	ViewerActorID              string
	ExpectedContentFingerprint string
}

type Replay struct {
	BattleID         string         `json:"battle_id"`
	CheckpointSchema int            `json:"checkpoint_schema_version"`
	EventSchema      int            `json:"event_schema_version"`
	ContentPin       contentpin.Pin `json:"content_pin"`
	Events           []event.Event  `json:"events"`
}

func NewReader(repo repository.Repository) Reader {
	return Reader{repository: repo}
}

// Read returns validated recorded facts in sequence order. It deliberately
// does not run the engine, random sources, or content operations.
func (reader Reader) Read(request Request) (Replay, error) {
	if reader.repository == nil {
		return Replay{}, errors.New("replay repository is required")
	}
	checkpoint, err := reader.repository.Load(request.BattleID)
	if err != nil {
		return Replay{}, err
	}
	if err := repository.ValidateCheckpoint(checkpoint); err != nil {
		return Replay{}, err
	}
	if request.ExpectedContentFingerprint != "" &&
		request.ExpectedContentFingerprint != checkpoint.ContentPin.Fingerprint {
		return Replay{}, fmt.Errorf(
			"%w: recorded=%s expected=%s",
			ErrContentChanged,
			checkpoint.ContentPin.Fingerprint,
			request.ExpectedContentFingerprint,
		)
	}
	events := checkpoint.Events
	if request.ViewerActorID != "" {
		events = event.ForViewer(events, request.ViewerActorID)
	}
	return Replay{
		BattleID:         checkpoint.BattleID,
		CheckpointSchema: checkpoint.SchemaVersion,
		EventSchema:      checkpoint.EventSchemaVersion,
		ContentPin:       checkpoint.ContentPin,
		Events:           events,
	}, nil
}
