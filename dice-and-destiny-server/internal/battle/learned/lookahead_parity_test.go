package learned

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/lookahead"
)

// The balance lab's runner must play the same battle a game session does:
// the session's competent proxy and the lab's port of it reach the same
// result and health on the same seeds.
func TestLookaheadRunnerMatchesSession(t *testing.T) {
	f := newExpeditionFixture(t, 1, "Parity")
	lib := f.x.catalogs["starter"]
	const opponent = "drowned_oracle_brine_mask"
	runner, err := lookahead.NewRunner(lookahead.Matchup{
		ContentRoot:  f.contentRoot,
		AuthoredRoot: f.loadoutRoot,
		RunStateRoot: t.TempDir(),
		Hero:         "starter",
		Opponent:     opponent,
		Unified:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	minion, err := LoadScriptedMinion(f.contentRoot, opponent)
	if err != nil {
		t.Fatal(err)
	}
	foe := lookahead.Scripted{Label: opponent, Policy: minion}
	for seed := uint64(1); seed <= 6; seed++ {
		s, err := NewSession(SessionConfig{ContentRoot: f.contentRoot, LoadoutRoot: f.loadoutRoot, RunStateRoot: t.TempDir(), OpponentDefinition: opponent, OpponentCount: 1})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ResetCharacterLoadout(fmt.Sprintf("parity-%d", seed), seed, "seat-a", false, "starter", true, "sandbox"); err != nil {
			t.Fatal(err)
		}
		view := playExpeditionBattle(t, s, lib)
		final, err := s.environment.InspectBattle()
		if err != nil {
			t.Fatal(err)
		}
		record, err := runner.Play(seed, lookahead.ProxyPolicy{}, foe)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"victory": "win", "defeat": "loss", "draw": "draw"}[str(view["battle_result"])]
		if record.Result != want || record.HeroHealth != final.Actors["seat-a"].CurrentHealth() ||
			record.FoeHealth != final.Actors["seat-b"].CurrentHealth() || record.Rounds != final.Segment.Round {
			t.Fatalf("seed %d: lab %s %d/%d round %d, session %s %d/%d round %d", seed,
				record.Result, record.HeroHealth, record.FoeHealth, record.Rounds,
				want, final.Actors["seat-a"].CurrentHealth(), final.Actors["seat-b"].CurrentHealth(), final.Segment.Round)
		}
	}
}

// The look-ahead opponent plays a mirror of the human's character and deck,
// finishes the battle, and always makes its private offensive plan before
// the human makes theirs, so its copies of the battle never hold the human's
// private planning.
func TestLookaheadSessionMirror(t *testing.T) {
	f := newExpeditionFixture(t, 1, "Mirror")
	lib := f.x.catalogs["starter"]
	diagnostics := filepath.Join(t.TempDir(), "learned-battles.jsonl")
	s, err := NewSession(SessionConfig{ContentRoot: f.contentRoot, LoadoutRoot: f.loadoutRoot, RunStateRoot: t.TempDir(), OpponentPolicy: OpponentPolicyLookahead, InferenceTimeout: time.Minute, DiagnosticsPath: diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{"seat-a", "seat-b"} {
		if _, err := s.ResetCharacterLoadout("mirror-"+seat, 7, seat, false, "starter", true, "sandbox"); err != nil {
			t.Fatal(err)
		}
		start, _ := s.environment.InspectBattle()
		if start.Actors[s.modelSeat].DefinitionID != "starter" || len(start.Actors[s.modelSeat].Cards.Deck)+len(start.Actors[s.modelSeat].Cards.Hand) != len(start.Actors[s.humanSeat].Cards.Deck)+len(start.Actors[s.humanSeat].Cards.Hand) {
			t.Fatalf("%s: opponent is not a Starter mirror", seat)
		}
		playExpeditionBattle(t, s, lib)
		telemetry, _ := s.Telemetry()
		if telemetry.ModelDecisions == 0 || len(telemetry.Errors) > 0 || telemetry.Timeouts > 0 {
			t.Fatalf("%s: model decisions %d, errors %v, timeouts %d", seat, telemetry.ModelDecisions, telemetry.Errors, telemetry.Timeouts)
		}
		first := map[string]string{}
		for _, decision := range telemetry.Commands {
			if !strings.HasPrefix(string(decision.Command.Type), "planning_") {
				continue
			}
			var payload struct {
				Checkpoint struct {
					WindowID string `json:"window_id"`
					Stage    string `json:"stage"`
				} `json:"checkpoint"`
			}
			_ = json.Unmarshal(decision.Command.Payload, &payload)
			if payload.Checkpoint.Stage != "planning" {
				continue
			}
			if _, seen := first[payload.Checkpoint.WindowID]; !seen {
				first[payload.Checkpoint.WindowID] = decision.ActorSeat
			}
		}
		if len(first) == 0 {
			t.Fatalf("%s: no offensive planning recorded", seat)
		}
		for window, actor := range first {
			if actor != s.modelSeat {
				t.Fatalf("%s: the human planned first in %s", seat, window)
			}
		}
	}
	// The balance lab's -tally reads finished mirror battles from this log.
	raw, err := os.ReadFile(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, line := range strings.Split(string(raw), "\n") {
		var record struct {
			Kind    string          `json:"kind"`
			Payload BattleTelemetry `json:"payload"`
		}
		if json.Unmarshal([]byte(line), &record) == nil && record.Kind == "battle_completed" {
			completed++
			if record.Payload.Model.ModelID != LookaheadModelID || record.Payload.Character != "starter" || record.Payload.Result == "" {
				t.Fatalf("completed record lacks tally fields: %+v", record.Payload)
			}
		}
	}
	if completed != 2 {
		t.Fatalf("%d completed battles logged, want 2", completed)
	}
}
