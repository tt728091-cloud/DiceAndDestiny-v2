package learned

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type runtimeRequest struct {
	Op              string `json:"op"`
	ReplaceSession  bool   `json:"replace_session,omitempty"`
	ModelPath       string `json:"model_path,omitempty"`
	ModelSHA256     string `json:"model_sha256,omitempty"`
	ContentRoot     string `json:"content_root,omitempty"`
	RunStateRoot    string `json:"run_state_root,omitempty"`
	DiagnosticsPath string `json:"diagnostics_path,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	BattleID        string `json:"battle_id,omitempty"`
	Seed            uint64 `json:"seed,omitempty"`
	HumanSeat       string `json:"human_seat,omitempty"`
	Character       string `json:"character,omitempty"`
	Rematch         bool   `json:"rematch,omitempty"`
	CommandJSON     string `json:"command_json,omitempty"`
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
	learnedRuntime.Lock()
	defer learnedRuntime.Unlock()
	if request.Op == "initialize" {
		config := SessionConfig{
			ContentRoot:      request.ContentRoot,
			RunStateRoot:     request.RunStateRoot,
			ModelPath:        request.ModelPath,
			ModelSHA256:      request.ModelSHA256,
			DiagnosticsPath:  request.DiagnosticsPath,
			InferenceTimeout: time.Duration(request.TimeoutMS) * time.Millisecond,
		}
		differentConfig := learnedRuntime.session != nil && (config.ModelPath != learnedRuntime.config.ModelPath ||
			config.ModelSHA256 != learnedRuntime.config.ModelSHA256 ||
			config.ContentRoot != learnedRuntime.config.ContentRoot)
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
		value, err = learnedRuntime.session.ResetCharacter(request.BattleID, request.Seed, request.HumanSeat, request.Rematch, request.Character)
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
