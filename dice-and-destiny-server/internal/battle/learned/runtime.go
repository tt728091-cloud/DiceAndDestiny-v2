package learned

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type runtimeRequest struct {
	Op              string `json:"op"`
	ModelPath       string `json:"model_path,omitempty"`
	ContentRoot     string `json:"content_root,omitempty"`
	RunStateRoot    string `json:"run_state_root,omitempty"`
	DiagnosticsPath string `json:"diagnostics_path,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	BattleID        string `json:"battle_id,omitempty"`
	Seed            uint64 `json:"seed,omitempty"`
	HumanSeat       string `json:"human_seat,omitempty"`
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
			DiagnosticsPath:  request.DiagnosticsPath,
			InferenceTimeout: time.Duration(request.TimeoutMS) * time.Millisecond,
		}
		if learnedRuntime.session == nil {
			session, err := NewSession(config)
			if err != nil {
				return runtimeError(err)
			}
			learnedRuntime.session = session
			learnedRuntime.config = config
		} else if config.ModelPath != learnedRuntime.config.ModelPath || config.ContentRoot != learnedRuntime.config.ContentRoot {
			return runtimeError(fmt.Errorf("learned runtime is already initialized with a different model or content root"))
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
		value, err = learnedRuntime.session.Reset(request.BattleID, request.Seed, request.HumanSeat, request.Rematch)
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
