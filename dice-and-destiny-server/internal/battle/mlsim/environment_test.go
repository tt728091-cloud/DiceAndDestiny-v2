package mlsim

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

func TestCommittedDamageMetricsUseFinalAuthorityBatch(t *testing.T) {
	environment := &Environment{}
	environment.recordCommittedDamage([]event.Event{{
		Type: event.TypeDamageCommitted,
		Data: map[string]any{
			"sources": []state.SettledDamageSource{
				{TargetActorID: "seat-b", SourceContentID: "sword_cut", BaseAmount: 6, FinalAmount: 2},
				{TargetActorID: "seat-b", SourceContentID: "bleed", BaseAmount: 1, FinalAmount: 1},
				{TargetActorID: "seat-a", SourceContentID: "poison", BaseAmount: 1, FinalAmount: 1},
			},
			"removals": []state.ProposedCardRemoval{
				{TargetActorID: "seat-b", Accepted: true},
				{TargetActorID: "seat-b", Accepted: true},
				{TargetActorID: "seat-b", Accepted: true},
				{TargetActorID: "seat-b", Accepted: true, Released: true},
				{TargetActorID: "seat-a", Accepted: true},
			},
		},
	}})

	if got := environment.metrics.DamageBySeat.SeatB; got != (DamageMetrics{
		RawAttack: 6, RawBleed: 1, RawTotal: 7, ResolvedTotal: 3, ActualTotal: 3,
	}) {
		t.Fatalf("seat-b damage metrics = %#v", got)
	}
	if got := environment.metrics.DamageBySeat.SeatA; got != (DamageMetrics{
		RawPoison: 1, RawTotal: 1, ResolvedTotal: 1, ActualTotal: 1,
	}) {
		t.Fatalf("seat-a damage metrics = %#v", got)
	}
}

func TestEncodedTransportStripsRawDecisionAndParityModeRetainsIt(t *testing.T) {
	encodedConfig := testConfig()
	encodedConfig.AuthorityMode = AuthorityModeEphemeral
	encodedConfig.TelemetryMode = TelemetryModeTraining
	encodedConfig.TransportMode = TransportModeEncoded
	encoded, err := New(encodedConfig)
	if err != nil {
		t.Fatal(err)
	}
	request := ResetRequest{Seed: 41, BattleID: "encoded-transport"}
	encodedTransition, err := encoded.Reset(request)
	if err != nil {
		t.Fatal(err)
	}
	if encodedTransition.Result.Snapshot != nil || len(encodedTransition.Result.LegalActions) != 0 {
		t.Fatal("encoded transport retained raw snapshot or legal commands")
	}
	decision := encodedTransition.EncodedDecision
	if decision == nil || decision.CandidateCount == 0 || len(decision.CandidateTypes) != decision.CandidateCount {
		t.Fatalf("encoded decision metadata is incomplete: %#v", decision)
	}
	observation, err := base64.StdEncoding.DecodeString(decision.ObservationBase64)
	if err != nil {
		t.Fatal(err)
	}
	mask, err := base64.StdEncoding.DecodeString(decision.ActionMaskBase64)
	if err != nil {
		t.Fatal(err)
	}
	if len(observation) != EncodedObservationSize*4 || len(mask) != EncodedMaxActions/8 {
		t.Fatalf("encoded payload sizes = %d/%d", len(observation), len(mask))
	}

	parityConfig := testConfig()
	parityConfig.TransportMode = TransportModeParity
	parity, err := New(parityConfig)
	if err != nil {
		t.Fatal(err)
	}
	request.BattleID = "parity-transport"
	parityTransition, err := parity.Reset(request)
	if err != nil {
		t.Fatal(err)
	}
	if parityTransition.Result.Snapshot == nil || parityTransition.EncodedDecision == nil {
		t.Fatal("parity transport must retain raw and encoded decisions")
	}
	if len(parityTransition.Result.LegalActions) != parityTransition.EncodedDecision.CandidateCount {
		t.Fatal("raw and encoded candidate counts differ")
	}
}

func TestEphemeralAuthorityMatchesNormalAuthorityCorpus(t *testing.T) {
	seeds := []uint64{1, 9, 41, 20260801, 30000000}
	generated := rand.New(rand.NewSource(20260802))
	for len(seeds) < 25 {
		seeds = append(seeds, generated.Uint64())
	}
	for _, seed := range seeds {
		seed := seed
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			normalConfig := testConfig()
			normalConfig.AuthorityMode = AuthorityModeNormal
			ephemeralConfig := testConfig()
			ephemeralConfig.AuthorityMode = AuthorityModeEphemeral
			normal, err := New(normalConfig)
			if err != nil {
				t.Fatal(err)
			}
			ephemeral, err := New(ephemeralConfig)
			if err != nil {
				t.Fatal(err)
			}
			request := ResetRequest{
				Seed:       seed,
				BattleID:   fmt.Sprintf("parity-%d", seed),
				SeatModels: map[string]string{"seat-a": "parity-a", "seat-b": "parity-b"},
			}
			normalTransition, err := normal.Reset(request)
			if err != nil {
				t.Fatal(err)
			}
			ephemeralTransition, err := ephemeral.Reset(request)
			if err != nil {
				t.Fatal(err)
			}
			assertParityTransition(t, normalTransition, ephemeralTransition)

			selector := rand.New(rand.NewSource(int64(seed)))
			for !normalTransition.Terminal && normalTransition.TruncationReason == "" {
				if normalTransition.ActorID != ephemeralTransition.ActorID {
					t.Fatalf("actor order mismatch: normal=%q ephemeral=%q", normalTransition.ActorID, ephemeralTransition.ActorID)
				}
				if !reflect.DeepEqual(normalTransition.Result.LegalActions, ephemeralTransition.Result.LegalActions) {
					t.Fatal("canonical legal candidates differ")
				}
				index := parityActionIndex(selector, normalTransition.Result.LegalActions, normalTransition.Metrics.Actions)
				viewer := normalTransition.ActorID
				nextNormal, normalApplied, err := normal.StepForViewer(index, viewer)
				if err != nil {
					t.Fatal(err)
				}
				nextEphemeral, ephemeralApplied, err := ephemeral.StepForViewer(index, viewer)
				if err != nil {
					t.Fatal(err)
				}
				normalTransition = nextNormal
				ephemeralTransition = nextEphemeral
				normalJSON, _ := json.Marshal(normalApplied)
				ephemeralJSON, _ := json.Marshal(ephemeralApplied)
				if string(normalJSON) != string(ephemeralJSON) {
					t.Fatalf("accepted viewer result/events differ at action %d: %s", normalTransition.Metrics.Actions, firstJSONDifference(normalJSON, ephemeralJSON))
				}
				normalState, err := normal.authority.InspectBattleState(request.BattleID)
				if err != nil {
					t.Fatal(err)
				}
				ephemeralState, err := ephemeral.authority.InspectBattleState(request.BattleID)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(normalState, ephemeralState) {
					t.Fatalf("authority state or random cursor differs at action %d", normalTransition.Metrics.Actions)
				}
				assertParityTransition(t, normalTransition, ephemeralTransition)
			}
			if !normalTransition.Terminal || normalTransition.TruncationReason != "" {
				t.Fatalf("parity corpus battle did not terminate: %#v", normalTransition.Metrics)
			}
		})
	}
}

func TestTrainingTelemetryMatchesFullAuthorityCorpus(t *testing.T) {
	seeds := []uint64{1, 9, 41, 20260801, 30000000}
	generated := rand.New(rand.NewSource(20260803))
	for len(seeds) < 12 {
		seeds = append(seeds, generated.Uint64())
	}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			fullConfig := testConfig()
			fullConfig.AuthorityMode = AuthorityModeNormal
			fullConfig.TelemetryMode = TelemetryModeFull
			optimizedConfig := testConfig()
			optimizedConfig.AuthorityMode = AuthorityModeEphemeral
			optimizedConfig.TelemetryMode = TelemetryModeTraining
			full, err := New(fullConfig)
			if err != nil {
				t.Fatal(err)
			}
			optimized, err := New(optimizedConfig)
			if err != nil {
				t.Fatal(err)
			}
			request := ResetRequest{
				Seed:       seed,
				BattleID:   fmt.Sprintf("training-parity-%d", seed),
				SeatModels: map[string]string{"seat-a": "parity-a", "seat-b": "parity-b"},
			}
			fullTransition, err := full.Reset(request)
			if err != nil {
				t.Fatal(err)
			}
			optimizedTransition, err := optimized.Reset(request)
			if err != nil {
				t.Fatal(err)
			}
			selector := rand.New(rand.NewSource(int64(seed)))
			for !fullTransition.Terminal && fullTransition.TruncationReason == "" {
				// Committed-damage diagnostics intentionally require the full
				// authority event stream. Training telemetry omits that stream;
				// normalize this evaluation-only metric before state/decision parity.
				fullTransition.Metrics.DamageBySeat = DamageBySeatMetrics{}
				optimizedTransition.Metrics.DamageBySeat = DamageBySeatMetrics{}
				assertParityTransition(t, fullTransition, optimizedTransition)
				if !reflect.DeepEqual(fullTransition.Result.LegalActions, optimizedTransition.Result.LegalActions) {
					t.Fatal("optimized candidate order or content differs")
				}
				index := parityActionIndex(selector, fullTransition.Result.LegalActions, fullTransition.Metrics.Actions)
				fullTransition, err = full.Step(index)
				if err != nil {
					t.Fatal(err)
				}
				optimizedTransition, err = optimized.Step(index)
				if err != nil {
					t.Fatal(err)
				}
				fullState, _ := full.authority.InspectBattleState(request.BattleID)
				optimizedState, _ := optimized.authority.InspectBattleState(request.BattleID)
				if !reflect.DeepEqual(fullState, optimizedState) {
					t.Fatalf("optimized hidden state differs at action %d", fullTransition.Metrics.Actions)
				}
			}
			fullTransition.Metrics.DamageBySeat = DamageBySeatMetrics{}
			optimizedTransition.Metrics.DamageBySeat = DamageBySeatMetrics{}
			assertParityTransition(t, fullTransition, optimizedTransition)
			fullEvents, err := full.authority.InspectBattleEventsJSON(request.BattleID)
			if err != nil {
				t.Fatal(err)
			}
			optimizedEvents, err := optimized.authority.InspectBattleEventsJSON(request.BattleID)
			if err != nil {
				t.Fatal(err)
			}
			if string(fullEvents) != string(optimizedEvents) {
				t.Fatalf("accepted event stream differs: %s", firstJSONDifference(fullEvents, optimizedEvents))
			}
		})
	}
}

func assertParityTransition(t *testing.T, normal, ephemeral Transition) {
	t.Helper()
	normal.Metrics.DurationMillis = 0
	ephemeral.Metrics.DurationMillis = 0
	if !reflect.DeepEqual(normal, ephemeral) {
		normalJSON, _ := json.Marshal(normal)
		ephemeralJSON, _ := json.Marshal(ephemeral)
		t.Fatalf("transition mismatch: %s", firstJSONDifference(normalJSON, ephemeralJSON))
	}
}

func firstJSONDifference(left, right []byte) string {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	index := 0
	for index < limit && left[index] == right[index] {
		index++
	}
	start := index - 120
	if start < 0 {
		start = 0
	}
	leftEnd := index + 240
	if leftEnd > len(left) {
		leftEnd = len(left)
	}
	rightEnd := index + 240
	if rightEnd > len(right) {
		rightEnd = len(right)
	}
	return fmt.Sprintf("first byte %d (lengths %d/%d): left=%q right=%q", index, len(left), len(right), left[start:leftEnd], right[start:rightEnd])
}

func parityActionIndex(rng *rand.Rand, actions []command.Command, completed int) int {
	if completed >= 700 {
		return preferredActionIndex(actions)
	}
	progressing := make([]int, 0, len(actions))
	for index, action := range actions {
		if action.Type != command.TypePlanningKeep {
			progressing = append(progressing, index)
		}
	}
	if len(progressing) == 0 {
		return preferredActionIndex(actions)
	}
	return progressing[rng.Intn(len(progressing))]
}

func TestResetIsDeterministicFreshAndViewerSafe(t *testing.T) {
	environment := newTestEnvironment(t)
	first, err := environment.Reset(ResetRequest{Seed: 41, BattleID: "deterministic-reset", SeatModels: map[string]string{"seat-a": "one", "seat-b": "two"}})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first.Result)
	if err != nil {
		t.Fatal(err)
	}
	seatB, err := environment.Observe("seat-b")
	if err != nil {
		t.Fatal(err)
	}
	if got := seatB.Snapshot.Actors["seat-a"]; len(got.Hand) != 0 || len(got.CardInstances) != 0 || len(got.RollHistory) != 0 {
		t.Fatalf("seat A private planning state leaked to seat B: %#v", got)
	}

	second, err := environment.Reset(ResetRequest{Seed: 41, BattleID: "deterministic-reset", SeatModels: map[string]string{"seat-a": "one", "seat-b": "two"}})
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second.Result)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("same reset seed and battle ID produced a different initial viewer trajectory")
	}
	if second.Metrics.Actions != 0 || len(second.Result.LegalActions) == 0 {
		t.Fatalf("episode state leaked across reset: %#v", second)
	}
}

func TestCompleteReplayUsesOriginalAuthorityCommands(t *testing.T) {
	environment := newTestEnvironment(t)
	transition, err := environment.Reset(ResetRequest{Seed: 20260801, BattleID: "ml-replay", SeatModels: map[string]string{"seat-a": "heuristic-a", "seat-b": "heuristic-b"}})
	if err != nil {
		t.Fatal(err)
	}
	for !transition.Terminal && transition.TruncationReason == "" {
		transition, err = environment.Step(preferredActionIndex(transition.Result.LegalActions))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !transition.Terminal || transition.Replay == nil || transition.Winner == "" {
		t.Fatalf("battle did not reach a recorded terminal result: %#v", transition)
	}
	if transition.Metrics.AuthorityRejects != 0 || transition.Metrics.InvalidActionIDs != 0 {
		t.Fatalf("unexpected invalid action metrics: %#v", transition.Metrics)
	}
	// Round-trip through JSON exactly as the portable replay runner does. Raw
	// payload whitespace and object-key order are not command semantics.
	replayJSON, err := json.MarshalIndent(transition.Replay, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var portable ReplayRecord
	if err := json.Unmarshal(replayJSON, &portable); err != nil {
		t.Fatal(err)
	}
	replayed, err := environment.Replay(portable)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Terminal || replayed.Winner != transition.Winner || replayed.Metrics.Actions != transition.Metrics.Actions {
		t.Fatalf("replay mismatch: original=%#v replayed=%#v", transition.Metrics, replayed.Metrics)
	}
}

func TestCanonicalActionsAndInvalidIndexProtection(t *testing.T) {
	environment := newTestEnvironment(t)
	transition, err := environment.Reset(ResetRequest{Seed: 9, BattleID: "canonical-actions"})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(transition.Result.LegalActions)
	view, err := environment.Observe(transition.ActorID)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := json.Marshal(view.LegalActions)
	if string(first) != string(second) {
		t.Fatal("unchanged authority state produced a different candidate order")
	}
	if _, err := environment.Step(len(transition.Result.LegalActions)); err == nil {
		t.Fatal("out-of-range action index was accepted")
	}
	if environment.metrics.InvalidActionIDs != 1 || environment.metrics.Actions != 0 {
		t.Fatalf("invalid action mutated episode history: %#v", environment.metrics)
	}
}

func TestRepeatedEpisodesDoNotRequireProcessRestart(t *testing.T) {
	environment := newTestEnvironment(t)
	rng := rand.New(rand.NewSource(77))
	seen := map[string]bool{}
	for episode := 0; episode < 12; episode++ {
		transition, err := environment.Reset(ResetRequest{Seed: uint64(1000 + episode)})
		if err != nil {
			t.Fatal(err)
		}
		if seen[transition.Metrics.BattleID] {
			t.Fatalf("duplicate generated battle ID %q", transition.Metrics.BattleID)
		}
		seen[transition.Metrics.BattleID] = true
		for !transition.Terminal && transition.TruncationReason == "" {
			// Mix random legal choices with a terminal-seeking fallback. This
			// exercises candidate coverage without making the unit test flaky.
			index := rng.Intn(len(transition.Result.LegalActions))
			if transition.Metrics.Actions > 700 {
				index = preferredActionIndex(transition.Result.LegalActions)
			}
			transition, err = environment.Step(index)
			if err != nil {
				t.Fatal(err)
			}
		}
		if transition.TruncationReason != "" {
			t.Fatalf("episode %d truncated: %#v", episode, transition.Metrics)
		}
		if !state.IsTerminalBattleStatus(transition.Metrics.Status) || transition.Metrics.AuthorityRejects != 0 {
			t.Fatalf("episode %d invalid terminal metrics: %#v", episode, transition.Metrics)
		}
	}
}

func BenchmarkAuthorityEnvironmentCompleteBattle(b *testing.B) {
	benchmarkAuthorityEnvironmentCompleteBattle(b, AuthorityModeNormal)
}

func BenchmarkAuthorityEnvironmentCompleteBattleEphemeral(b *testing.B) {
	benchmarkAuthorityEnvironmentCompleteBattle(b, AuthorityModeEphemeral)
}

func BenchmarkAuthorityEnvironmentCompleteBattleEphemeralTrainingTelemetry(b *testing.B) {
	config := testConfig()
	config.AuthorityMode = AuthorityModeEphemeral
	config.TelemetryMode = TelemetryModeTraining
	benchmarkAuthorityEnvironmentCompleteBattleWithConfig(b, config)
}

func benchmarkAuthorityEnvironmentCompleteBattle(b *testing.B, authorityMode string) {
	config := testConfig()
	config.AuthorityMode = authorityMode
	benchmarkAuthorityEnvironmentCompleteBattleWithConfig(b, config)
}

func benchmarkAuthorityEnvironmentCompleteBattleWithConfig(b *testing.B, config Config) {
	environment, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for episode := 0; episode < b.N; episode++ {
		transition, err := environment.Reset(ResetRequest{Seed: uint64(episode + 1)})
		if err != nil {
			b.Fatal(err)
		}
		for !transition.Terminal && transition.TruncationReason == "" {
			transition, err = environment.Step(preferredActionIndex(transition.Result.LegalActions))
			if err != nil {
				b.Fatal(err)
			}
		}
		if !transition.Terminal {
			b.Fatalf("episode truncated: %#v", transition.Metrics)
		}
	}
}

func preferredActionIndex(actions []command.Command) int {
	for _, kind := range []command.Type{
		command.TypePlanningRoll,
		command.TypePlanningAbility,
		command.TypePlanningReroll,
		command.TypeRollDice,
		command.TypePlanningPass,
		command.TypePass,
		command.TypeCommitInteraction,
	} {
		match := -1
		for index, action := range actions {
			if action.Type == kind {
				match = index
			}
		}
		if match >= 0 {
			return match
		}
	}
	return 0
}

func newTestEnvironment(t *testing.T) *Environment {
	t.Helper()
	environment, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func testConfig() Config {
	_, source, _, _ := runtime.Caller(0)
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	return Config{
		ContentRoot:  filepath.Join(serverRoot, "content"),
		RunStateRoot: filepath.Join(serverRoot, "save", "run_players"),
		MaxActions:   DefaultMaxActions,
		SessionID:    "test",
	}
}
