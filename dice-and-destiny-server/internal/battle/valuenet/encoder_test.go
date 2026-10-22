package valuenet_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/valuenet"
)

func serverRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// authoredCopy gives the test its own copy of the published cards; tests
// never read or write the tracked authored directory directly.
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

func mirror(t *testing.T, hero string) lookahead.Matchup {
	t.Helper()
	return lookahead.Matchup{ContentRoot: filepath.Join(serverRoot(t), "content"), AuthoredRoot: authoredCopy(t), RunStateRoot: t.TempDir(),
		Hero: hero, Opponent: hero, Unified: true, HeroPlansFirst: true}
}

// recorder keeps chosen decisions while the greedy player plays.
type recorder struct {
	inner lookahead.Policy
	keep  func(*lookahead.Decision) bool
	got   []*lookahead.Decision
}

func (r *recorder) Name() string { return "recorder" }

func (r *recorder) Choose(d *lookahead.Decision) (int, error) {
	if r.keep(d) {
		r.got = append(r.got, d)
	}
	return r.inner.Choose(d)
}

// planningDecisions plays a mirror and returns every offensive planning
// decision made before the acting seat rolled, from both seats.
func planningDecisions(t *testing.T, hero string, seed uint64) []*lookahead.Decision {
	t.Helper()
	runner, err := lookahead.NewRunner(mirror(t, hero))
	if err != nil {
		t.Fatal(err)
	}
	keep := func(d *lookahead.Decision) bool {
		return d.Battle.Segment.Current == "offensive" && d.Battle.Settled.Window != nil && d.Battle.Settled.Window.Stage == "planning" &&
			len(d.Battle.Settled.Actors[d.Seat].FinalDice) == 0
	}
	shared := []*lookahead.Decision{}
	a := &recorder{inner: lookahead.NewGreedyPolicy(), keep: keep}
	b := &recorder{inner: lookahead.NewGreedyPolicy(), keep: keep}
	if _, err := runner.Play(seed, a, b); err != nil {
		t.Fatal(err)
	}
	shared = append(append(shared, a.got...), b.got...)
	if len(shared) < 4 {
		t.Fatalf("only %d planning decisions", len(shared))
	}
	return shared
}

func encode(t *testing.T, enc *valuenet.Encoder, d *lookahead.Decision, b *state.Battle, seat string) []float32 {
	t.Helper()
	pos, err := valuenet.NewPosition(d.Sim, b, seat, d.Seat)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, valuenet.Width())
	if err := enc.Encode(pos, out); err != nil {
		t.Fatal(err)
	}
	return out
}

func diff(names []string, a, b []float32) string {
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return names[i]
		}
	}
	return ""
}

// The schema publishes one unique name per input and its hash, and the
// encoder writes exactly that many.
func TestSchemaWidth(t *testing.T) {
	names := valuenet.Features()
	if len(names) != valuenet.Width() || valuenet.Width() < 50 {
		t.Fatalf("%d names for width %d", len(names), valuenet.Width())
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("duplicate input %q", n)
		}
		seen[n] = true
	}
	if len(valuenet.SchemaHash()) != 64 {
		t.Fatalf("schema hash %q", valuenet.SchemaHash())
	}
	d := planningDecisions(t, "starter", 3)[0]
	pos, err := valuenet.NewPosition(d.Sim, d.Battle, d.Seat, d.Seat)
	if err != nil {
		t.Fatal(err)
	}
	if err := valuenet.NewEncoder().Encode(pos, make([]float32, valuenet.Width()-1)); err == nil {
		t.Fatal("encoded into a short buffer")
	}
}

// The encoder only knows what the seat sees: forking the battle (fresh
// randomness and a re-dealt hidden hand) or changing the opponent's hidden
// hand and private dice leaves every input unchanged. Visible changes do
// show, so the check is not vacuous.
func TestEncoderIsViewerSafe(t *testing.T) {
	enc := valuenet.NewEncoder()
	names := valuenet.Features()
	checked, private := 0, 0
	for _, hero := range []string{"starter", "venom", "curse"} {
		for _, d := range planningDecisions(t, hero, 5) {
			for _, seat := range []string{"seat-a", "seat-b"} {
				want := encode(t, enc, d, d.Battle, seat)
				for seed := uint64(1); seed <= 4; seed++ {
					fork := lookahead.Fork(d.Battle, seat, seed*7919)
					if got := encode(t, valuenet.NewEncoder(), d, &fork, seat); diff(names, want, got) != "" {
						t.Fatalf("%s %s round %d: fork %d changed %s", hero, seat, d.Battle.Segment.Round, seed, diff(names, want, got))
					}
				}
				// Swap the opponent's hidden hand with its draw pile, and change
				// its private dice if it has rolled.
				other := "seat-a"
				if seat == "seat-a" {
					other = "seat-b"
				}
				tampered := d.Battle.Clone()
				foe := tampered.Actors[other]
				n := min(len(foe.Cards.Hand), len(foe.Cards.Deck))
				for i := 0; i < n; i++ {
					foe.Cards.Hand[i], foe.Cards.Deck[i] = foe.Cards.Deck[i], foe.Cards.Hand[i]
				}
				tampered.Actors[other] = foe
				if rt := tampered.Settled.Actors[other]; len(rt.FinalDice) > 0 && state.SettledPlanningPrivate(tampered) {
					private++
					for i := range rt.FinalDice {
						rt.FinalDice[i].Face = 7 - rt.FinalDice[i].Face
					}
					tampered.Settled.Actors[other] = rt
				}
				if got := encode(t, enc, d, &tampered, seat); diff(names, want, got) != "" {
					t.Fatalf("%s %s: hidden change moved %s", hero, seat, diff(names, want, got))
				}
				checked++
			}
		}
		// A visible change (own energy) moves the inputs.
		d := planningDecisions(t, hero, 5)[0]
		visible := d.Battle.Clone()
		actor := visible.Actors[d.Seat]
		actor.Resources.EnergyPoints += 5
		visible.Actors[d.Seat] = actor
		if diff(names, encode(t, enc, d, d.Battle, d.Seat), encode(t, enc, d, &visible, d.Seat)) == "" {
			t.Fatalf("%s: +5 energy changed nothing", hero)
		}
	}
	if checked < 12 || private == 0 {
		t.Fatalf("checked %d positions, %d with the opponent's private dice", checked, private)
	}
}

// The same battle encodes to the same bytes, from fresh encoders and from a
// second recording of the same seeds.
func TestEncodingIsDeterministic(t *testing.T) {
	d := planningDecisions(t, "starter", 7)[1]
	names := valuenet.Features()
	a := encode(t, valuenet.NewEncoder(), d, d.Battle, d.Seat)
	b := encode(t, valuenet.NewEncoder(), d, d.Battle, d.Seat)
	if diff(names, a, b) != "" {
		t.Fatalf("fresh encoders differ at %s", diff(names, a, b))
	}
	record := func() ([]byte, valuenet.DatasetHeader) {
		dir := filepath.Join(t.TempDir(), "data")
		data, err := valuenet.NewDataset(dir)
		if err != nil {
			t.Fatal(err)
		}
		m := mirror(t, "starter")
		m.Observe = data.Observer
		greedy := func() (lookahead.Policy, error) { return lookahead.NewGreedyPolicy(), nil }
		if _, err := lookahead.RunInterleaved(m, []lookahead.Entrant{{Name: "greedy", Hero: greedy}}, greedy, []bool{false, true}, 1, 4, 3, nil); err != nil {
			t.Fatal(err)
		}
		header, err := data.Close(valuenet.DatasetHeader{Hero: "starter", Opponent: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, header.Shards[0].File))
		if err != nil {
			t.Fatal(err)
		}
		return raw, header
	}
	first, header := record()
	second, _ := record()
	if !bytes.Equal(first, second) {
		t.Fatal("recording the same battles twice gave different bytes")
	}
	width := valuenet.Width() + len(valuenet.MetaColumns)
	if header.Battles != 8 || header.Rows == 0 || len(first) != header.Rows*width*4 {
		t.Fatalf("header %d battles %d rows for %d bytes", header.Battles, header.Rows, len(first))
	}
	// Every battle has both seats' views at each round start, labelled with
	// opposite results.
	row := func(i, col int) float32 {
		return math.Float32frombits(binary.LittleEndian.Uint32(first[(i*width+col)*4:]))
	}
	result := valuenet.Width() + 6
	for i := 0; i+1 < header.Rows; i += 2 {
		if row(i, valuenet.Width()) != row(i+1, valuenet.Width()) || row(i, result)+row(i+1, result) != 1 {
			t.Fatalf("rows %d and %d are not the two seats of one position", i, i+1)
		}
	}
}
