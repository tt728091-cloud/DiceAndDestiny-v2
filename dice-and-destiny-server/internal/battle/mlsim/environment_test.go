package mlsim

import (
	"encoding/json"
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

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
	environment, err := New(testConfig())
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
