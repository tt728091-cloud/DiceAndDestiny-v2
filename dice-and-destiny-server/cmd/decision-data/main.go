// Command decision-data turns recorded decisions (balance-lab -label) into
// training arrays for the network: each decision's value-v2 state inputs,
// each option's inputs and its measured value. The inputs are read from the
// deciding seat's viewer-safe view only.
//
// Output, in a new directory: states.f32 (decisions × state width),
// options.f32 (options × option width), targets.f32 (options × 5: value
// relative to the best option, its standard error, futures, mean value,
// greedy's pick), option_decision.i32 (each option's decision row) and
// header.json (feature names, schema hashes, per-decision metadata).
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"diceanddestiny/server/internal/battle/decisions"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/valuenet"
)

type decisionMeta struct {
	File     string  `json:"file"`
	Hero     string  `json:"hero"`
	Opponent string  `json:"opponent"`
	Seed     uint64  `json:"seed"`
	Seat     string  `json:"seat"`
	Step     int     `json:"step"`
	Round    int     `json:"round"`
	Kind     string  `json:"kind"`
	Result   float64 `json:"result"`
	Margin   float64 `json:"margin"`
	Options  int     `json:"options"`
	Best     int     `json:"best"`
	Anchor   int     `json:"anchor"`
	Played   int     `json:"played"`
}

type header struct {
	Format          string         `json:"format"`
	StateSchema     string         `json:"state_schema"`
	StateHash       string         `json:"state_schema_hash"`
	StateFeatures   []string       `json:"state_features"`
	OptionSchema    string         `json:"option_schema"`
	OptionHash      string         `json:"option_schema_hash"`
	OptionFeatures  []string       `json:"option_features"`
	Targets         []string       `json:"targets"`
	Sources         []string       `json:"sources"`
	Decisions       []decisionMeta `json:"decisions"`
	SkippedNoAnchor int            `json:"skipped_no_anchor"`
}

type encoded struct {
	meta    decisionMeta
	state   []float32
	options []float32
	targets []float32
	ok      bool
}

func main() {
	in := flag.String("in", "", "decision files (comma-separated, globs)")
	out := flag.String("out", "", "new output directory")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel encoders")
	flag.Parse()
	if *in == "" || *out == "" {
		fail(fmt.Errorf("-in and -out are required"))
	}
	var paths []string
	for _, part := range strings.Split(*in, ",") {
		matches, err := filepath.Glob(strings.TrimSpace(part))
		if err != nil {
			fail(err)
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		fail(fmt.Errorf("no files match %s", *in))
	}
	if err := os.Mkdir(*out, 0o755); err != nil {
		fail(err)
	}
	type job struct {
		index int
		file  string
		r     decisions.Record
		b     *state.Battle
	}
	var results []encoded
	var mu sync.Mutex
	jobs := make(chan job, 64)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enc := valuenet.NewEncoder()
			for j := range jobs {
				e := encode(enc, j.file, j.r, j.b)
				mu.Lock()
				results[j.index] = e
				mu.Unlock()
			}
		}()
	}
	count := 0
	for _, path := range paths {
		err := decisions.Read([]string{path}, func(r decisions.Record, b *state.Battle) error {
			mu.Lock()
			results = append(results, encoded{})
			mu.Unlock()
			jobs <- job{index: count, file: filepath.Base(path), r: r, b: b}
			count++
			return nil
		})
		if err != nil {
			fail(err)
		}
	}
	close(jobs)
	wg.Wait()
	h := header{Format: "dice-and-destiny-decisions-v1", StateSchema: valuenet.SchemaNameV2, StateHash: valuenet.SchemaHashV2(), StateFeatures: valuenet.FeaturesV2(),
		OptionSchema: valuenet.OptionSchemaName, OptionHash: valuenet.OptionSchemaHash(), OptionFeatures: valuenet.OptionFeatures(),
		Targets: []string{"value_vs_best", "standard_error", "futures", "mean", "greedy_pick"}, Sources: paths}
	files := map[string]*bufio.Writer{}
	var closers []*os.File
	open := func(name string) *bufio.Writer {
		f, err := os.Create(filepath.Join(*out, name))
		if err != nil {
			fail(err)
		}
		closers = append(closers, f)
		files[name] = bufio.NewWriter(f)
		return files[name]
	}
	states, options, targets, owners := open("states.f32"), open("options.f32"), open("targets.f32"), open("option_decision.i32")
	row := int32(0)
	for _, e := range results {
		if !e.ok {
			h.SkippedNoAnchor++
			continue
		}
		write(states, e.state)
		write(options, e.options)
		write(targets, e.targets)
		for k := 0; k < e.meta.Options; k++ {
			if err := binary.Write(owners, binary.LittleEndian, row); err != nil {
				fail(err)
			}
		}
		h.Decisions = append(h.Decisions, e.meta)
		row++
	}
	for _, w := range files {
		if err := w.Flush(); err != nil {
			fail(err)
		}
	}
	for _, f := range closers {
		if err := f.Close(); err != nil {
			fail(err)
		}
	}
	raw, err := json.Marshal(h)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "header.json"), raw, 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "%d decisions (%d skipped), state width %d, option width %d\n", len(h.Decisions), h.SkippedNoAnchor, valuenet.WidthV2(), valuenet.OptionWidth())
}

// encode reads one decision from its seat's view.
func encode(enc *valuenet.Encoder, file string, r decisions.Record, b *state.Battle) encoded {
	anchor := r.Option(b, r.Anchor)
	if anchor < 0 {
		return encoded{}
	}
	pos, err := valuenet.NewPosition(b, r.Seat, r.Seat)
	if err != nil {
		fail(err)
	}
	e := encoded{ok: true, meta: decisionMeta{File: file, Hero: r.Hero, Opponent: r.Opponent, Seed: r.Seed, Seat: r.Seat, Step: r.Step, Round: r.Round, Kind: r.Kind,
		Result: r.Result, Margin: r.Margin, Options: len(r.Options), Best: r.Best, Anchor: anchor, Played: -1}}
	if r.Played >= 0 {
		e.meta.Played = r.Option(b, r.Played)
	}
	e.state = make([]float32, valuenet.WidthV2())
	if err := enc.EncodeV2(pos, e.state); err != nil {
		fail(err)
	}
	e.options = make([]float32, valuenet.OptionWidth()*len(r.Options))
	if err := enc.EncodeOptions(pos, r.Actions, r.Options, r.Options[anchor], e.options); err != nil {
		fail(err)
	}
	for k := range r.Options {
		greedy := float32(0)
		if k == anchor {
			greedy = 1
		}
		e.targets = append(e.targets, float32(-r.Gap[k]), float32(r.GapSE[k]), float32(r.Futures[k]), float32(r.Mean[k]), greedy)
	}
	return e
}

func write(w *bufio.Writer, values []float32) {
	for _, v := range values {
		if math.IsNaN(float64(v)) {
			v = 0
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			fail(err)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
