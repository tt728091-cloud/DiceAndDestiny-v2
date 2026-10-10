package learned

import (
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle/loadout"
)

type runtimeRequest struct {
	CardTree           json.RawMessage       `json:"card_tree,omitempty"`
	AdminToken         string                `json:"admin_token,omitempty"`
	CardID             string                `json:"card_id,omitempty"`
	Ability            json.RawMessage       `json:"ability,omitempty"`
	AbilityBoard       content.AbilityBoard  `json:"ability_board,omitempty"`
	Card               json.RawMessage       `json:"card,omitempty"`
	CatalogRevision    int                   `json:"catalog_revision,omitempty"`
	AdminSettings      loadout.AdminSettings `json:"admin_settings,omitempty"`
	LoadoutMode        string                `json:"loadout_mode,omitempty"`
	Purchase           loadout.Purchase      `json:"purchase,omitempty"`
	LoadoutRoot        string                `json:"loadout_root,omitempty"`
	Decklist           []loadout.Entry       `json:"decklist,omitempty"`
	UnifiedDefense     bool                  `json:"unified_defense"`
	OpponentDefinition string                `json:"opponent_definition,omitempty"`
	OpponentCount      int                   `json:"opponent_count,omitempty"`
	Op                 string                `json:"op"`
	ReplaceSession     bool                  `json:"replace_session,omitempty"`
	ModelPath          string                `json:"model_path,omitempty"`
	ModelSHA256        string                `json:"model_sha256,omitempty"`
	ContentRoot        string                `json:"content_root,omitempty"`
	RunStateRoot       string                `json:"run_state_root,omitempty"`
	DiagnosticsPath    string                `json:"diagnostics_path,omitempty"`
	TimeoutMS          int                   `json:"timeout_ms,omitempty"`
	BattleID           string                `json:"battle_id,omitempty"`
	Seed               uint64                `json:"seed,omitempty"`
	HumanSeat          string                `json:"human_seat,omitempty"`
	Character          string                `json:"character,omitempty"`
	Rematch            bool                  `json:"rematch,omitempty"`
	CommandJSON        string                `json:"command_json,omitempty"`
	Encounter          string                `json:"encounter,omitempty"`
}

var learnedRuntime struct {
	sync.Mutex
	session *Session
	config  SessionConfig
}

// Keep catalog edits and deck transactions ordered, including delete's final
// dependency check. Battle commands keep their existing session lock.
var catalogRuntimeMu sync.Mutex

func HandleRuntimeRequest(requestJSON string) string {
	var request runtimeRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return runtimeError(fmt.Errorf("decode learned runtime request: %w", err))
	}
	switch request.Op {
	case "card_trees", "validate_card_tree", "publish_card_tree", "open_card_admin", "close_card_admin", "preview_delete_card", "admin_delete_card",
		"ability_authoring", "validate_ability", "publish_ability", "assign_abilities",
		"card_authoring", "validate_card", "publish_card", "character_catalogs",
		"save_character_deck", "progression_catalogs", "progression_purchase", "save_economy_admin", "campaign_status", "campaign_loadout":
		catalogRuntimeMu.Lock()
		defer catalogRuntimeMu.Unlock()
	}
	if request.Op == "campaign_loadout" {
		view, err := campaignLoadout(request.ContentRoot, request.LoadoutRoot, request.Character)
		if err != nil {
			return runtimeError(err)
		}
		return runtimeSuccess(view)
	}
	if request.Op == "campaign_status" {
		status, err := campaignStatus(request.ContentRoot, request.LoadoutRoot)
		if err != nil {
			return runtimeError(err)
		}
		return runtimeSuccess(status)
	}
	if request.Op == "card_trees" || request.Op == "validate_card_tree" || request.Op == "publish_card_tree" {
		return handleCardTrees(request)
	}
	if request.Op == "open_card_admin" || request.Op == "close_card_admin" || request.Op == "preview_delete_card" || request.Op == "admin_delete_card" {
		return handleCardAdmin(request)
	}
	if request.Op == "ability_authoring" || request.Op == "validate_ability" || request.Op == "publish_ability" || request.Op == "assign_abilities" {
		return handleAbilityAuthoring(request)
	}
	if request.Op == "card_authoring" || request.Op == "validate_card" || request.Op == "publish_card" {
		return handleCardAuthoring(request)
	}
	if request.Op == "character_catalogs" || request.Op == "save_character_deck" || request.Op == "progression_catalogs" || request.Op == "progression_purchase" || request.Op == "save_economy_admin" {
		catalogs, err := CharacterCatalogs(request.ContentRoot, request.LoadoutRoot)
		if err != nil {
			return runtimeError(err)
		}
		baseEconomy, err := loadout.LoadEconomy(request.ContentRoot, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		access, err := loadout.ReadAccess(request.LoadoutRoot, baseEconomy)
		if err != nil {
			return runtimeError(err)
		}
		if request.Op == "progression_catalogs" || request.Op == "progression_purchase" || request.Op == "save_economy_admin" {
			economy := baseEconomy
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
			if request.Op == "save_economy_admin" {
				if err := loadout.SaveAdmin(request.LoadoutRoot, economy, catalogs, request.AdminSettings); err != nil {
					return runtimeError(err)
				}
			}
			all, economy, admin, err := loadout.ProgressSnapshot(request.LoadoutRoot, economy, catalogs)
			if err != nil {
				return runtimeError(err)
			}
			// Allocation offsets are authority bookkeeping, never editable client input.
			admin.BudgetAllocations = nil
			view := characterCatalogView(catalogs)
			for id := range catalogs {
				progress := all[id]
				entry := view[id].(map[string]any)
				entry["access"] = economy.Access
				entry["type_conflicts"] = economy.Access.Problems(id, progress.Deck, progress.Abilities)
				entry["progression"] = progress
				entry["economy"] = economy.Offers(id)
				entry["admin_settings"] = admin
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
			if err := access.ValidateDeck(request.Character, request.Decklist); err != nil {
				return runtimeError(err)
			}
			deck, err := loadout.WriteSharedDeck(request.LoadoutRoot, request.Character, request.Decklist, baseEconomy, catalog)
			if err != nil {
				return runtimeError(err)
			}
			return runtimeSuccess(deck)
		}
		view := characterCatalogView(catalogs)
		for id, catalog := range catalogs {
			progress, err := loadout.ReadProgress(request.LoadoutRoot, id, baseEconomy, catalog)
			deck := progress.Deck
			entry := view[id].(map[string]any)
			if err != nil {
				entry["loadout_error"] = err.Error()
			} else if deck != nil {
				entry["owned_decklist"] = deck
			}
			if progress.AuthoredAbilityRevision > 0 {
				entry["combatants"].(map[string]any)[id].(map[string]any)["ability_board"] = progress.Abilities
			}
			effectiveDeck := deck
			if effectiveDeck == nil {
				for _, card := range catalog.Combatants[id].Decklist {
					effectiveDeck = append(effectiveDeck, loadout.Entry{CardID: card.CardID, Count: card.Count})
				}
			}
			entry["economy"] = baseEconomy.Offers(id)
			entry["access"] = access
			entry["type_conflicts"] = access.Problems(id, effectiveDeck, catalog.Combatants[id].AbilityBoard)
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
		if request.Encounter != "" {
			value, err = learnedRuntime.session.ResetCampaignEncounter(request.BattleID, request.Seed, request.Character, request.Encounter)
			break
		}
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
