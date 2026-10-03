package learned

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle/loadout"
)

type runtimeRequest struct {
	LoadoutMode        string           `json:"loadout_mode,omitempty"`
	Purchase           loadout.Purchase `json:"purchase,omitempty"`
	LoadoutRoot        string           `json:"loadout_root,omitempty"`
	Decklist           []loadout.Entry  `json:"decklist,omitempty"`
	UnifiedDefense     bool             `json:"unified_defense"`
	OpponentDefinition string           `json:"opponent_definition,omitempty"`
	OpponentCount      int              `json:"opponent_count,omitempty"`
	Op                 string           `json:"op"`
	ReplaceSession     bool             `json:"replace_session,omitempty"`
	ModelPath          string           `json:"model_path,omitempty"`
	ModelSHA256        string           `json:"model_sha256,omitempty"`
	ContentRoot        string           `json:"content_root,omitempty"`
	RunStateRoot       string           `json:"run_state_root,omitempty"`
	DiagnosticsPath    string           `json:"diagnostics_path,omitempty"`
	TimeoutMS          int              `json:"timeout_ms,omitempty"`
	BattleID           string           `json:"battle_id,omitempty"`
	Seed               uint64           `json:"seed,omitempty"`
	HumanSeat          string           `json:"human_seat,omitempty"`
	Character          string           `json:"character,omitempty"`
	Rematch            bool             `json:"rematch,omitempty"`
	CommandJSON        string           `json:"command_json,omitempty"`
}

var learnedRuntime struct {
	sync.Mutex
	session *Session
	config  SessionConfig
}

func HandleRuntimeRequest(requestJSON string) string {
	var request runtimeRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return runtimeError(fmt.Errorf("decode learned runtime request: %w", err))
	}
	if request.Op == "character_catalogs" || request.Op == "save_character_deck" || request.Op == "progression_catalogs" || request.Op == "progression_purchase" {
		catalogs, err := CharacterCatalogs(request.ContentRoot)
		if err != nil {
			return runtimeError(err)
		}
		if request.Op == "progression_catalogs" || request.Op == "progression_purchase" {
			economy, err := loadout.LoadEconomy(request.ContentRoot, catalogs)
			if err != nil {
				return runtimeError(err)
			}
			if request.Op == "progression_purchase" {
				lib, ok := catalogs[request.Character]
				if !ok {
					return runtimeError(fmt.Errorf("unknown playable character"))
				}
				progress, err := loadout.Buy(request.LoadoutRoot, request.Character, economy, lib, request.Purchase)
				if err != nil {
					return runtimeError(err)
				}
				return runtimeSuccess(progress)
			}
			view := characterCatalogView(catalogs)
			for id, lib := range catalogs {
				progress, err := loadout.ReadProgress(request.LoadoutRoot, id, economy, lib)
				if err != nil {
					return runtimeError(fmt.Errorf("%s progression: %w", id, err))
				}
				entry := view[id].(map[string]any)
				entry["progression"] = progress
				entry["economy"] = economy.Offers(id)
				entry["owned_decklist"] = progress.Deck
				entry["combatants"].(map[string]any)[id].(map[string]any)["ability_board"] = progress.Abilities
				entry["deck_limits"] = map[string]int{"max_cards": loadout.MaxCards, "max_copies": loadout.MaxCopies}
			}
			return runtimeSuccess(view)
		}

		if request.Op == "save_character_deck" {
			catalog, ok := catalogs[request.Character]
			if !ok {
				return runtimeError(fmt.Errorf("unknown playable character %q", request.Character))
			}
			deck, err := loadout.Write(request.LoadoutRoot, request.Character, request.Decklist, catalog.Cards)
			if err != nil {
				return runtimeError(err)
			}
			return runtimeSuccess(deck)
		}
		view := characterCatalogView(catalogs)
		for id, catalog := range catalogs {
			deck, err := loadout.Read(request.LoadoutRoot, id, catalog.Cards)
			entry := view[id].(map[string]any)
			if err != nil {
				entry["loadout_error"] = err.Error()
			} else if deck != nil {
				entry["owned_decklist"] = deck
			}
			entry["deck_limits"] = map[string]int{"max_cards": loadout.MaxCards, "max_copies": loadout.MaxCopies}
		}
		return runtimeSuccess(view)
	}
	learnedRuntime.Lock()
	defer learnedRuntime.Unlock()
	if request.Op == "initialize" {
		config := SessionConfig{
			LoadoutRoot:        request.LoadoutRoot,
			OpponentDefinition: request.OpponentDefinition,
			OpponentCount:      request.OpponentCount,
			ContentRoot:        request.ContentRoot,
			RunStateRoot:       request.RunStateRoot,
			ModelPath:          request.ModelPath,
			ModelSHA256:        request.ModelSHA256,
			DiagnosticsPath:    request.DiagnosticsPath,
			InferenceTimeout:   time.Duration(request.TimeoutMS) * time.Millisecond,
		}
		differentConfig := learnedRuntime.session != nil && (config.LoadoutRoot != learnedRuntime.config.LoadoutRoot || config.ModelPath != learnedRuntime.config.ModelPath ||
			config.ModelSHA256 != learnedRuntime.config.ModelSHA256 ||
			config.ContentRoot != learnedRuntime.config.ContentRoot || config.OpponentDefinition != learnedRuntime.config.OpponentDefinition || config.OpponentCount != learnedRuntime.config.OpponentCount)
		if differentConfig && !request.ReplaceSession {
			return runtimeError(fmt.Errorf("learned runtime is already initialized with a different model or content root"))
		}
		if learnedRuntime.session == nil || differentConfig {
			session, err := NewSession(config)
			if err != nil {
				return runtimeError(err)
			}
			learnedRuntime.session = session
			learnedRuntime.config = config
		}
		_, lifetime := learnedRuntime.session.Telemetry()
		return runtimeSuccess(map[string]any{
			"model":    learnedRuntime.session.policy.Metadata(),
			"lifetime": lifetime,
		})
	}
	if learnedRuntime.session == nil {
		return runtimeError(fmt.Errorf("learned runtime is not initialized"))
	}
	var value any
	var err error
	switch request.Op {
	case "reset":
		value, err = learnedRuntime.session.ResetCharacterLoadout(request.BattleID, request.Seed, request.HumanSeat, request.Rematch, request.Character, request.UnifiedDefense, request.LoadoutMode)
	case "submit_human":
		value, err = learnedRuntime.session.SubmitHuman(request.CommandJSON)
	case "advance_model":
		value, err = learnedRuntime.session.AdvanceModel()
	case "telemetry":
		battle, lifetime := learnedRuntime.session.Telemetry()
		value = map[string]any{"battle": battle, "lifetime": lifetime}
	default:
		err = fmt.Errorf("unknown learned runtime operation %q", request.Op)
	}
	if err != nil {
		return runtimeError(err)
	}
	if presented, ok := value.(map[string]any); ok && request.Op != "telemetry" {
		encoded, _ := json.Marshal(presented)
		return string(encoded)
	}
	return runtimeSuccess(value)
}

func runtimeSuccess(value any) string {
	encoded, _ := json.Marshal(map[string]any{"ok": true, "result": value})
	return string(encoded)
}

func runtimeError(err error) string {
	encoded, _ := json.Marshal(map[string]any{"accepted": false, "ok": false, "error": err.Error()})
	return string(encoded)
}
