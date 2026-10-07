package state

import "encoding/json"

// Venom work is part of authoritative state, including interrupted windows, so
// a rejected command, save/load, or history fork cannot lose a delayed effect.
type MechanicExpiration struct {
	Checkpoint string
	Round      int
	StatusID   string
	ActorID    string
	InstanceID string
}
type VenomRuntime struct {
	Expirations    map[string]MechanicExpiration `json:",omitempty"`
	Cards          map[string]string             `json:",omitempty"`
	Round          int
	Provoked       map[string]int
	Used           map[string]bool
	Maturing       []string
	MaturationDone bool
	CatalystUsed   map[string]bool
	Queue          []VenomWork
	Active         *VenomWork
	Resume         *VenomResume
	AttackChecks   map[string][]SettledEffectRoll
	MoltRewards    []VenomMoltReward
}
type VenomMoltReward struct {
	CardID          string `json:",omitempty"`
	ActorID         string
	SourceID        string
	PriorPrevention int
}
type VenomWork struct {
	Checks          int `json:",omitempty"`
	SourceContentID string
	Damage          int
	RequirePoison   bool
	Kind            string
	SourceActorID   string
	TargetActorID   string
	StatusID        string
	InstanceID      string
	Stacks          int
	Rolls           []SettledEffectRoll
	Accelerant      bool
}
type VenomResume struct {
	Stage   string
	Window  *SettledWindow
	Flow    SegmentFlowState
	Trigger *SettledTriggerBatch
	Damage  *SettledDamageBatch
	Advance bool
}

func CloneVenom(v *VenomRuntime) *VenomRuntime {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var result VenomRuntime
	if err = json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return &result
}

// Planning remains private while a public child effect temporarily interrupts
// that input. The child window does not reveal either uncommitted dice pool.
func SettledPlanningPrivate(b Battle) bool {
	if b.Settled == nil || b.Settled.ReactionReplanning {
		return false
	}
	return b.Settled.Stage == "planning" || (b.Settled.Curse != nil && b.Settled.Curse.Resume != nil && b.Settled.Curse.Resume.Stage == "planning") || (b.Settled.Venom != nil && b.Settled.Venom.Resume != nil && b.Settled.Venom.Resume.Stage == "planning")
}
