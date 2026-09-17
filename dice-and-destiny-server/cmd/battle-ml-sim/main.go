// Command battle-ml-sim is a persistent JSON-lines bridge between local ML
// tooling and the real Go battle authority. It never opens a network listener.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"diceanddestiny/server/internal/battle/mlsim"
)

type request struct {
	Op              string              `json:"op"`
	Seed            uint64              `json:"seed,omitempty"`
	BattleID        string              `json:"battle_id,omitempty"`
	SeatModels      map[string]string   `json:"seat_models,omitempty"`
	SeatDefinitions map[string]string   `json:"seat_definitions,omitempty"`
	SeatID          string              `json:"seat_id,omitempty"`
	ActionIndex     int                 `json:"action_index,omitempty"`
	Replay          *mlsim.ReplayRecord `json:"replay,omitempty"`
}

type response struct {
	OK                bool              `json:"ok"`
	Error             string            `json:"error,omitempty"`
	EnvironmentSchema string            `json:"environment_schema"`
	ObservationSchema string            `json:"observation_schema"`
	ActionSchema      string            `json:"action_schema"`
	Transition        *mlsim.Transition `json:"transition,omitempty"`
	Result            any               `json:"result,omitempty"`
}

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", serverRoot+"/content", "battle content root")
	runStateRoot := flag.String("run-state-root", serverRoot+"/save/run_players", "run-player state root")
	maxActions := flag.Int("max-actions", mlsim.DefaultMaxActions, "episode action safety limit")
	sessionID := flag.String("session-id", "", "optional stable process/session identifier")
	authorityMode := flag.String("authority-mode", mlsim.AuthorityModeNormal, "authority repository mode: normal or ephemeral")
	telemetryMode := flag.String("telemetry-mode", mlsim.TelemetryModeFull, "result telemetry mode: full or training")
	transportMode := flag.String("transport-mode", mlsim.TransportModeFull, "Go-Python transport mode: full, encoded, or parity")
	observationSchema := flag.String("observation-schema", mlsim.ObservationSchemaVersion, "observation schema version")
	observationManifest := flag.String("observation-manifest", "", "frozen observation v3 manifest path")
	flag.Parse()

	environment, err := mlsim.New(mlsim.Config{
		ContentRoot:         *contentRoot,
		RunStateRoot:        *runStateRoot,
		MaxActions:          *maxActions,
		SessionID:           *sessionID,
		AuthorityMode:       *authorityMode,
		TelemetryMode:       *telemetryMode,
		TransportMode:       *transportMode,
		ObservationSchema:   *observationSchema,
		ObservationManifest: *observationManifest,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	for scanner.Scan() {
		var input request
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			writeResponse(encoder, environment, nil, fmt.Errorf("decode request: %w", err))
			continue
		}
		var value any
		switch input.Op {
		case "reset":
			value, err = environment.Reset(mlsim.ResetRequest{Seed: input.Seed, BattleID: input.BattleID, SeatModels: input.SeatModels, SeatDefinitions: input.SeatDefinitions})
		case "observe":
			value, err = environment.Observe(input.SeatID)
		case "legal_actions":
			value, err = environment.LegalActions(input.SeatID)
		case "step":
			value, err = environment.Step(input.ActionIndex)
		case "current":
			value = environment.Current()
		case "replay":
			if input.Replay == nil {
				err = fmt.Errorf("replay record is required")
			} else {
				value, err = environment.Replay(*input.Replay)
			}
		case "close":
			writeResponse(encoder, environment, map[string]bool{"closed": true}, nil)
			return
		default:
			err = fmt.Errorf("unknown operation %q", input.Op)
		}
		writeResponse(encoder, environment, value, err)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeResponse(encoder *json.Encoder, environment *mlsim.Environment, value any, err error) {
	environmentSchema, observationSchema, actionSchema := environment.SchemaVersions()
	result := response{
		OK:                err == nil,
		EnvironmentSchema: environmentSchema,
		ObservationSchema: observationSchema,
		ActionSchema:      actionSchema,
	}
	if err != nil {
		result.Error = err.Error()
	} else if transition, ok := value.(mlsim.Transition); ok {
		result.Transition = &transition
	} else {
		result.Result = value
	}
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		fmt.Fprintln(os.Stderr, encodeErr)
		os.Exit(1)
	}
}

func defaultServerRoot() string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	const suffix = "/cmd/battle-ml-sim/main.go"
	if strings.HasSuffix(source, suffix) {
		return strings.TrimSuffix(source, suffix)
	}
	return "."
}
