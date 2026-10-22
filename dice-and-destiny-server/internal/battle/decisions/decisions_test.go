package decisions_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/decisions"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

func serverRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func authoredCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"authored_cards.json", "economy_admin.json"} {
		raw, err := os.ReadFile(filepath.Join(serverRoot(t), "content", "authored", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Recording a battle's decisions never changes the battle, and the records
// read back with their battles: the same legal actions, a catalog the
// engine can use, the options' values and the battle's result.
func TestRecorderLeavesBattlesUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl.gz")
	writer, err := decisions.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	matchup := lookahead.Matchup{ContentRoot: filepath.Join(serverRoot(t), "content"), AuthoredRoot: authoredCopy(t), RunStateRoot: t.TempDir(),
		Hero: "starter", Opponent: "starter", Unified: true, HeroPlansFirst: true}
	plain, err := lookahead.NewRunner(matchup)
	if err != nil {
		t.Fatal(err)
	}
	matchup.Observe = func() lookahead.Observer {
		reference := lookahead.NewSearchPolicy(func() lookahead.Policy { return lookahead.NewGreedyPolicy() }, 0)
		reference.Rounds, reference.TopRerolls, reference.TopAbilities = 1, 8, 4
		return &decisions.Recorder{Hero: "starter", Opponent: "starter", Reference: reference, Futures: 8, Sample: 0.5, Writer: writer}
	}
	recorded, err := lookahead.NewRunner(matchup)
	if err != nil {
		t.Fatal(err)
	}
	results := map[uint64]float64{}
	for seed := uint64(1); seed <= 2; seed++ {
		want, err := plain.Play(seed, lookahead.NewGreedyPolicy(), lookahead.NewGreedyPolicy())
		if err != nil {
			t.Fatal(err)
		}
		got, err := recorded.Play(seed, lookahead.NewGreedyPolicy(), lookahead.NewGreedyPolicy())
		if err != nil {
			t.Fatal(err)
		}
		want.HeroThinkMS, got.HeroThinkMS = 0, 0
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("seed %d: recording changed the battle\nplain:    %+v\nrecorded: %+v", seed, want, got)
		}
		results[seed] = map[string]float64{"win": 1, "draw": 0.5, "loss": 0}[want.Result]
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	sim, err := lookahead.NewSimulator()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	err = decisions.Read([]string{path}, func(r decisions.Record, b *state.Battle) error {
		count++
		if r.Seat != r.HeroSeat || r.Result != results[r.Seed] {
			t.Fatalf("record %d of seed %d: seat %s (hero %s), result %v (battle %v)", r.Step, r.Seed, r.Seat, r.HeroSeat, r.Result, results[r.Seed])
		}
		if actions := sim.Legal(b, r.Seat); len(actions) != len(r.Actions) {
			t.Fatalf("record %d of seed %d: the restored battle lists %d actions, recorded %d", r.Step, r.Seed, len(actions), len(r.Actions))
		}
		if len(r.Options) < 2 || len(r.Mean) != len(r.Options) || r.Gap[r.Best] != 0 || r.Futures[r.Best] != 8 {
			t.Fatalf("record %d of seed %d: options %v, futures %v, gap %v, best %d", r.Step, r.Seed, r.Options, r.Futures, r.Gap, r.Best)
		}
		if r.Option(b, r.Anchor) < 0 {
			t.Fatalf("record %d of seed %d: the greedy choice %d is not among the options %v", r.Step, r.Seed, r.Anchor, r.Options)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 || count != writer.Count {
		t.Fatalf("read %d records, wrote %d", count, writer.Count)
	}
}
