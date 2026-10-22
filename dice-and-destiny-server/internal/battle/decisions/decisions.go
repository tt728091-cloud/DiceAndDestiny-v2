// Package decisions records real battle decisions together with reference
// values for each option, measured by playing many futures of every option
// (lookahead.SearchPolicy.OptionValues). The records are the look-ahead's
// decision benchmark, which scores a player by the win chance it loses per
// decision, and the network's training labels.
//
// Files are gzip JSON lines. A battle's pinned catalog is large and the same
// for every decision of a matchup, so it is stored once per file and each
// decision names it by hash.
package decisions

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// Record is one decision of a real battle and its options' reference
// values.
type Record struct {
	Hero     string `json:"hero"`
	Opponent string `json:"opponent"`
	Seed     uint64 `json:"seed"`
	HeroSeat string `json:"hero_seat"`
	// Seat is the deciding seat; Step the decision's number in its battle.
	Seat    string `json:"seat"`
	Step    int    `json:"step"`
	Round   int    `json:"round"`
	Segment string `json:"segment"`
	Stage   string `json:"stage"`
	Kind    string `json:"kind"`
	// Catalog is the SHA-256 of the battle's settled catalog, which the file
	// stores once; Battle is the full state without it.
	Catalog string            `json:"catalog"`
	Battle  json.RawMessage   `json:"battle"`
	Actions []command.Command `json:"actions"`
	// Options are the evaluated actions (indices into Actions); Anchor is
	// the greedy player's choice and Played the move made in the battle.
	Options []int `json:"options"`
	Anchor  int   `json:"anchor"`
	Played  int   `json:"played"`
	// Futures, Mean, Gap and GapSE are per option: futures played, mean
	// value (on the search's scale: +1 win, -1 loss, plus the margin term),
	// and the paired difference from the best option on their common
	// futures with its standard error. Best has the highest mean among the
	// options with the most futures; its Gap is 0.
	Futures []int     `json:"futures"`
	Mean    []float64 `json:"mean"`
	Gap     []float64 `json:"gap"`
	GapSE   []float64 `json:"gap_se"`
	Best    int       `json:"best"`
	// Result is the seat's battle result (1 win, 0.5 draw, 0 loss) and
	// Margin its final health share minus the opponent's.
	Result    float64 `json:"result"`
	Margin    float64 `json:"margin"`
	Reference string  `json:"reference"`
}

// Option returns the position in Options of the listed action, or of an
// equivalent one (another copy of the same card), or -1.
func (r Record) Option(b *state.Battle, action int) int {
	for k, i := range r.Options {
		if i == action {
			return k
		}
	}
	key := lookahead.EquivalenceKey(b, r.Seat, r.Actions[action])
	for k, i := range r.Options {
		if lookahead.EquivalenceKey(b, r.Seat, r.Actions[i]) == key {
			return k
		}
	}
	return -1
}

// Kind names a decision for reports: the segment and stage, and what its
// options do (dice when one is a reroll, then ability, then card).
func Kind(b *state.Battle, actions []command.Command, options []int) string {
	stage := ""
	if b.Settled != nil && b.Settled.Window != nil {
		stage = b.Settled.Window.Stage
	}
	what := "other"
	rank := map[string]int{"dice": 3, "ability": 2, "card": 1, "other": 0}
	for _, i := range options {
		k := "other"
		switch {
		case actions[i].Type == command.TypePlanningReroll:
			k = "dice"
		case actions[i].Type == command.TypePlanningAbility:
			k = "ability"
		case len(lookahead.CardIDs(actions[i])) > 0:
			k = "card"
		}
		if rank[k] > rank[what] {
			what = k
		}
	}
	return fmt.Sprintf("%s/%s:%s", b.Segment.Current, stage, what)
}

// Summarize fills Futures, Mean, Gap, GapSE and Best from each option's
// values (option k's futures are values[k], all starting at the same seed).
func (r *Record) Summarize(values [][]float64) {
	n := len(values)
	r.Futures, r.Mean, r.Gap, r.GapSE = make([]int, n), make([]float64, n), make([]float64, n), make([]float64, n)
	most := 0
	for k, v := range values {
		r.Futures[k] = len(v)
		most = max(most, len(v))
		var sum float64
		for _, x := range v {
			sum += x
		}
		r.Mean[k] = sum / float64(max(len(v), 1))
	}
	r.Best = -1
	for k := range values {
		if r.Futures[k] == most && (r.Best < 0 || r.Mean[k] > r.Mean[r.Best]) {
			r.Best = k
		}
	}
	for k, v := range values {
		common := min(len(v), len(values[r.Best]))
		var sum, sq float64
		for j := 0; j < common; j++ {
			x := values[r.Best][j] - v[j]
			sum += x
			sq += x * x
		}
		if common == 0 {
			continue
		}
		mean := sum / float64(common)
		r.Gap[k] = mean
		if common > 1 {
			r.GapSE[k] = math.Sqrt(math.Max(sq/float64(common)-mean*mean, 0) / float64(common-1))
		}
	}
}

type catalogLine struct {
	Hash    string `json:"hash"`
	Settled []byte `json:"settled"`
}

type line struct {
	Catalog  *catalogLine `json:"catalog,omitempty"`
	Decision *Record      `json:"decision,omitempty"`
}

// Writer appends records to a file; it is safe for concurrent use.
type Writer struct {
	mu       sync.Mutex
	file     *os.File
	zip      *gzip.Writer
	enc      *json.Encoder
	catalogs map[string]bool
	Count    int
}

// Create starts a new file; it refuses to overwrite one.
func Create(path string) (*Writer, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	zip := gzip.NewWriter(file)
	return &Writer{file: file, zip: zip, enc: json.NewEncoder(zip), catalogs: map[string]bool{}}, nil
}

// Write stores a record of the battle b (its state goes in without the
// catalog).
func (w *Writer) Write(r Record, b *state.Battle) error {
	sum := sha256.Sum256(b.SettledCatalog)
	r.Catalog = hex.EncodeToString(sum[:])
	stripped := b.Clone()
	stripped.SettledCatalog = nil
	raw, err := json.Marshal(stripped)
	if err != nil {
		return err
	}
	r.Battle = raw
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.catalogs[r.Catalog] {
		if err := w.enc.Encode(line{Catalog: &catalogLine{Hash: r.Catalog, Settled: b.SettledCatalog}}); err != nil {
			return err
		}
		w.catalogs[r.Catalog] = true
	}
	w.Count++
	return w.enc.Encode(line{Decision: &r})
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.zip.Close(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

// Read calls each for every record in the files, in order, with its battle
// restored.
func Read(paths []string, each func(r Record, b *state.Battle) error) error {
	for _, path := range paths {
		if err := readFile(path, each); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func readFile(path string, each func(r Record, b *state.Battle) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zip, err := gzip.NewReader(bufio.NewReader(file))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(zip)
	catalogs := map[string][]byte{}
	for {
		var l line
		if err := dec.Decode(&l); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if l.Catalog != nil {
			catalogs[l.Catalog.Hash] = l.Catalog.Settled
			continue
		}
		if l.Decision == nil {
			continue
		}
		r := *l.Decision
		settled, ok := catalogs[r.Catalog]
		if !ok {
			return fmt.Errorf("decision %d of seed %d names an unknown catalog", r.Step, r.Seed)
		}
		var b state.Battle
		if err := json.Unmarshal(r.Battle, &b); err != nil {
			return err
		}
		b.SettledCatalog = settled
		if err := each(r, &b); err != nil {
			return err
		}
	}
}

// Recorder is a lab observer that records sampled decisions of the hero
// seat with reference values. Each runner needs its own (the reference
// player is not shared).
type Recorder struct {
	Hero, Opponent string
	// Reference measures the options: its playout settings play the
	// futures and its option breadth (TopRerolls, TopAbilities) picks them.
	Reference *lookahead.SearchPolicy
	Describe  string
	// Futures is the number of futures for the two best options. Every
	// option first gets a quarter of them, the better half half of them,
	// and the best two all of them (successive halving).
	Futures int
	// Sample is the share of decisions with two or more options to record.
	Sample float64
	Writer *Writer

	seed     uint64
	heroSeat string
	step     int
	pending  []pendingRecord
}

type pendingRecord struct {
	record Record
	battle *state.Battle
}

// Begin starts a battle.
func (rc *Recorder) Begin(seed uint64, heroSeat string) {
	rc.seed, rc.heroSeat, rc.step, rc.pending = seed, heroSeat, 0, nil
}

// Decision measures a sampled decision of the hero seat. It works on a copy
// of the decision with its own randomness, so the battle plays exactly as
// it would unrecorded. Only the hero's decisions are recorded: the hero
// plans first, so the other seat's planning would see the hero's private
// dice in the battle state.
func (rc *Recorder) Decision(d *lookahead.Decision) {
	rc.step++
	seed := mix(rc.seed, uint64(rc.step))
	if d.Seat != rc.heroSeat || float64(seed%1_000_000)/1_000_000 >= rc.Sample {
		return
	}
	copy := *d
	copy.Rand = rand.New(rand.NewSource(int64(seed >> 1)))
	options, anchor, err := rc.Reference.Options(&copy)
	if err != nil || len(options) < 2 {
		return
	}
	values, err := rc.measure(&copy, options, seed)
	if err != nil {
		return
	}
	r := Record{Hero: rc.Hero, Opponent: rc.Opponent, HeroSeat: rc.heroSeat, Seat: d.Seat, Step: rc.step, Round: d.Battle.Segment.Round,
		Segment: string(d.Battle.Segment.Current), Kind: Kind(d.Battle, d.Actions, options), Actions: d.Actions, Options: options,
		Anchor: anchor, Played: -1, Reference: rc.Describe}
	if d.Battle.Settled != nil && d.Battle.Settled.Window != nil {
		r.Stage = d.Battle.Settled.Window.Stage
	}
	r.Summarize(values)
	clone := d.Battle.Clone()
	rc.pending = append(rc.pending, pendingRecord{record: r, battle: &clone})
}

// measure plays the options by successive halving.
func (rc *Recorder) measure(d *lookahead.Decision, options []int, seed uint64) ([][]float64, error) {
	values := make([][]float64, len(options))
	alive := make([]int, len(options))
	for k := range alive {
		alive[k] = k
	}
	have := 0
	for _, target := range []int{rc.Futures / 4, rc.Futures / 2, rc.Futures} {
		if target <= have {
			continue
		}
		list := make([]int, len(alive))
		for i, k := range alive {
			list[i] = options[k]
		}
		more, err := rc.Reference.OptionValues(d, list, target-have, seed+uint64(have)*0x9e3779b97f4a7c15)
		if err != nil {
			return nil, err
		}
		for i, k := range alive {
			values[k] = append(values[k], more[i]...)
		}
		have = target
		sort.SliceStable(alive, func(a, b int) bool { return mean(values[alive[a]]) > mean(values[alive[b]]) })
		if len(alive) <= 2 {
			continue
		}
		keep := max(2, (len(alive)+1)/2)
		if target == rc.Futures/2 {
			keep = 2
		}
		alive = alive[:keep]
	}
	return values, nil
}

// Chose notes the move the battle's player made.
func (rc *Recorder) Chose(d *lookahead.Decision, index int) {
	if n := len(rc.pending); n > 0 && rc.pending[n-1].record.Step == rc.step {
		rc.pending[n-1].record.Played = index
	}
}

// Finish writes the battle's records with its result.
func (rc *Recorder) Finish(final *state.Battle, record lookahead.BattleRecord, _, _ lookahead.Policy) {
	for _, p := range rc.pending {
		r := p.record
		r.Seed = record.Seed
		switch final.WinnerActorID {
		case r.Seat:
			r.Result = 1
		case "":
			r.Result = 0.5
		}
		r.Margin = Margin(final, r.Seat)
		if err := rc.Writer.Write(r, p.battle); err != nil {
			fmt.Fprintln(os.Stderr, "decisions:", err)
		}
	}
	rc.pending, rc.step = nil, 0
}

// Margin is the seat's health share minus its strongest opponent's.
func Margin(b *state.Battle, seat string) float64 {
	h := b.Actors[seat]
	own := float64(h.CurrentHealth()) / float64(max(lookahead.MaxHealth(h), 1))
	foe := 0.0
	for _, id := range lookahead.Opponents(b, seat) {
		a := b.Actors[id]
		foe = math.Max(foe, float64(a.CurrentHealth())/float64(max(lookahead.MaxHealth(a), 1)))
	}
	return own - foe
}

// mix hashes two numbers into a seed (splitmix64).
func mix(a, b uint64) uint64 {
	z := a*0x9e3779b97f4a7c15 + b + 0x632be59bd9b4e019
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func mean(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(max(len(v), 1))
}
