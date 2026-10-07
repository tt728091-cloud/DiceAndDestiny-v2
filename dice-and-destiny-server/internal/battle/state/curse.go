package state

import "encoding/json"

// OwnedDie is a physical die, independent of the last result in any roll pool.
type OwnedDie struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definition_id"`
	Index        int    `json:"index"`
	CursedFaces  []int  `json:"cursed_faces"`
	Entombed     bool   `json:"entombed"`
}
type CursePreparation struct {
	Kind            string `json:"kind,omitempty"`
	StatusID        string `json:"status_id,omitempty"`
	SourceID        string `json:"source_id,omitempty"`
	CardID          string `json:"card_id"`
	Source          string `json:"source"`
	Target          string `json:"target"`
	Die             int    `json:"die"`
	Round           int    `json:"round"`
	ExpiresEffects  int    `json:"expires_effects"`
	LastRewardRound int    `json:"last_reward_round"`
	Rewards         int    `json:"rewards"`
}

// DividendState stores ownership and limits; the matching status enables rewards.
type DividendState struct {
	Energy          int
	Limit           int
	StatusID        string
	Source          string
	StatusInstance  string
	ExpiresEffects  int
	LastRewardRound int
	Rewards         int
}
type CurseWork struct {
	Configured      bool     `json:"configured,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	DefenseSourceID string   `json:"defense_source_id,omitempty"`
	Kind            string   `json:"kind"`
	Source          string   `json:"source"`
	Target          string   `json:"target"`
	CardID          string   `json:"card_id"`
	Die             int      `json:"die"`
	Face            int      `json:"face"`
	Amount          int      `json:"amount"`
	Options         []string `json:"options,omitempty"`
}
type CurseLog struct {
	Actor   string
	Round   int
	Segment string
	Data    map[string]any
}
type CurseRuntime struct {
	Dice              map[string][]OwnedDie
	Bags              map[string][]int
	Preparations      []CursePreparation
	Used              map[string]int
	Bonus             map[string]int
	TaxCards          map[string]int
	TaxEnergy         map[string]int
	LastStored        map[string]int
	Queue             []CurseWork
	Active            *CurseWork
	Resume            *VenomResume
	Logs              []CurseLog
	PendingKnell      []CurseLog
	PendingDividend   []CurseLog
	Dividends         map[string]DividendState
	EffectsRound      int
	ConversionRound   int
	AfterAttacksRound int
}

func CloneCurse(v *CurseRuntime) *CurseRuntime {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out CurseRuntime
	if err = json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return &out
}
