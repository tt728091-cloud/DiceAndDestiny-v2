package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/mlsim"
)

func main() {
	policyPath := flag.String("policy", "", "candidate policy-v3 export")
	policySHA := flag.String("policy-sha256", "", "required exact policy export hash")
	manifestPath := flag.String("observation-manifest", "", "frozen observation-v3 manifest")
	replayPath := flag.String("replay", "", "Python-generated v3 self-play authority replay")
	serverRoot := flag.String("server-root", ".", "server repository root")
	flag.Parse()
	if *policyPath == "" || *policySHA == "" || *manifestPath == "" || *replayPath == "" {
		fatalf("policy, policy-sha256, observation-manifest, and replay are required")
	}
	policy, err := learned.LoadCandidatePolicyV3(*policyPath, *policySHA)
	if err != nil {
		fatalf("load candidate: %v", err)
	}
	payload, err := os.ReadFile(*replayPath)
	if err != nil {
		fatalf("read replay: %v", err)
	}
	var replay mlsim.ReplayRecord
	if err := json.Unmarshal(payload, &replay); err != nil {
		fatalf("decode replay: %v", err)
	}
	environment, err := mlsim.New(mlsim.Config{
		ContentRoot: filepath.Join(*serverRoot, "content"), RunStateRoot: filepath.Join(*serverRoot, "save", "run_players"),
		AuthorityMode: mlsim.AuthorityModeEphemeral, TelemetryMode: mlsim.TelemetryModeTraining,
		TransportMode: mlsim.TransportModeFull, ObservationSchema: mlsim.ObservationSchemaV3,
		ObservationManifest: *manifestPath,
	})
	if err != nil {
		fatalf("new environment: %v", err)
	}
	transition, err := environment.Reset(mlsim.ResetRequest{
		Seed: replay.Seed, BattleID: replay.BattleID, SeatModels: replay.SeatModels,
		SeatDefinitions: replay.SeatDefinitions,
	})
	if err != nil {
		fatalf("reset: %v", err)
	}
	for sequence, expected := range replay.Actions {
		selected, _, selectErr := policy.Select(transition)
		if selectErr != nil {
			fatalf("select action %d: %v", sequence, selectErr)
		}
		if selected != expected.Index {
			fatalf("runtime parity mismatch at action %d: Go=%d Python=%d", sequence, selected, expected.Index)
		}
		transition, err = environment.Step(selected)
		if err != nil {
			fatalf("step action %d: %v", sequence, err)
		}
	}
	if !transition.Terminal || transition.Winner != replay.Winner {
		fatalf("replay result mismatch: terminal=%v winner=%q, want %q", transition.Terminal, transition.Winner, replay.Winner)
	}
	encoded, _ := json.Marshal(map[string]any{
		"passed": true, "actions": len(replay.Actions), "winner": transition.Winner,
		"model_id": policy.Metadata().ModelID, "policy_sha256": *policySHA,
	})
	fmt.Println(string(encoded))
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
