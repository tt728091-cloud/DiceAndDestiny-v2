package valuenet

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// DatasetFormat names the recorded layout: little-endian float32 shards, one
// row per position (the features, then MetaColumns), and header.json.
const DatasetFormat = "dice-and-destiny-value-dataset-v1"

// MetaColumns follow the features in every row. battle numbers the dataset's
// battles; viewer_seat_b and viewer_is_hero are 0 or 1; player indexes the
// header's players (the hero's player); result is the viewer's result (win
// 1, draw 0.5, loss 0) and margin its final health fraction minus the
// strongest opponent's.
var MetaColumns = []string{"battle", "seed", "round", "viewer_seat_b", "viewer_is_hero", "player", "result", "margin"}

// shardRows caps one shard file.
const shardRows = 50000

// DatasetHeader describes a recorded dataset.
type DatasetHeader struct {
	Format        string   `json:"format"`
	Schema        string   `json:"schema"`
	SchemaHash    string   `json:"schema_hash"`
	Features      []string `json:"features"`
	MetaColumns   []string `json:"meta_columns"`
	Hero          string   `json:"hero"`
	Opponent      string   `json:"opponent"`
	Foe           string   `json:"foe_player"`
	Players       []string `json:"players"`
	Settings      []string `json:"settings,omitempty"`
	Command       string   `json:"command,omitempty"`
	Battles       int      `json:"battles"`
	Truncated     int      `json:"truncated_battles_skipped"`
	Rows          int      `json:"rows"`
	ContentHashes []string `json:"content_hashes"`
	Shards        []Shard  `json:"shards"`
	CreatedUTC    string   `json:"created_utc"`
}

// Shard is one data file and its SHA-256.
type Shard struct {
	File   string `json:"file"`
	Rows   int    `json:"rows"`
	SHA256 string `json:"sha256"`
}

// Dataset collects recorded positions from every runner of a lab run and
// writes them, sorted, when closed.
type Dataset struct {
	dir     string
	mu      sync.Mutex
	rows    []recordedRow
	players []string
	hashes  map[string]bool
	skipped int
	err     error
}

type recordedRow struct {
	player, seed      int
	heroSeatB, viewer int
	round             int
	values            []float32 // features then meta
}

// NewDataset prepares an empty directory for a recording.
func NewDataset(dir string) (*Dataset, error) {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("record directory %s is not empty", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Dataset{dir: dir, hashes: map[string]bool{}}, nil
}

// Observer builds a recorder for one runner.
func (d *Dataset) Observer() lookahead.Observer {
	return &recorder{data: d, enc: NewEncoder()}
}

// Close sorts the rows (by player, seed, hero seat, round and viewer seat,
// so the same battles always give the same bytes), writes the shards and
// the header, and returns the header.
func (d *Dataset) Close(header DatasetHeader) (DatasetHeader, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return header, d.err
	}
	sort.SliceStable(d.rows, func(i, j int) bool {
		a, b := d.rows[i], d.rows[j]
		if a.player != b.player {
			return a.player < b.player
		}
		if a.seed != b.seed {
			return a.seed < b.seed
		}
		if a.heroSeatB != b.heroSeatB {
			return a.heroSeatB < b.heroSeatB
		}
		if a.round != b.round {
			return a.round < b.round
		}
		return a.viewer < b.viewer
	})
	battle, last := -1, [3]int{-1, -1, -1}
	for _, r := range d.rows {
		if key := [3]int{r.player, r.seed, r.heroSeatB}; key != last {
			battle, last = battle+1, key
		}
		r.values[Width()] = float32(battle)
	}
	header.Format, header.Schema, header.SchemaHash = DatasetFormat, SchemaName, SchemaHash()
	header.Features, header.MetaColumns = Features(), append([]string(nil), MetaColumns...)
	header.Players, header.Battles, header.Truncated, header.Rows = d.players, battle+1, d.skipped, len(d.rows)
	header.ContentHashes = nil
	for h := range d.hashes {
		header.ContentHashes = append(header.ContentHashes, h)
	}
	sort.Strings(header.ContentHashes)
	header.Shards = nil
	for start := 0; start < len(d.rows) || start == 0; start += shardRows {
		end := min(start+shardRows, len(d.rows))
		buf := make([]byte, 0, (end-start)*(Width()+len(MetaColumns))*4)
		for _, r := range d.rows[start:end] {
			for _, v := range r.values {
				buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(v))
			}
		}
		name := fmt.Sprintf("shard-%04d.f32", len(header.Shards))
		if err := os.WriteFile(filepath.Join(d.dir, name), buf, 0o644); err != nil {
			return header, err
		}
		sum := sha256.Sum256(buf)
		header.Shards = append(header.Shards, Shard{File: name, Rows: end - start, SHA256: hex.EncodeToString(sum[:])})
		if end == len(d.rows) {
			break
		}
	}
	header.CreatedUTC = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(header, "", "  ")
	if err != nil {
		return header, err
	}
	return header, os.WriteFile(filepath.Join(d.dir, "header.json"), raw, 0o644)
}

// recorder encodes both seats' views at the start of each round's offensive
// planning, before anyone rolls, and labels them when the battle ends.
type recorder struct {
	data    *Dataset
	enc     *Encoder
	round   int
	pending []recordedRow
	seats   []string
	err     error
}

// PlanningStart reports the first moment of a round's offensive planning:
// nobody has rolled yet.
func PlanningStart(b *state.Battle) bool {
	if b.Settled == nil || b.Settled.Window == nil || b.Segment.Current != "offensive" || b.Settled.Window.Stage != "planning" {
		return false
	}
	for _, rt := range b.Settled.Actors {
		if len(rt.FinalDice) > 0 || rt.RollsUsed > 0 {
			return false
		}
	}
	return true
}

func (r *recorder) Decision(d *lookahead.Decision) {
	b := d.Battle
	if r.err != nil || b.Segment.Round == r.round || !PlanningStart(b) {
		return
	}
	r.round = b.Segment.Round
	for _, seat := range sortedActors(b) {
		pos, err := NewPosition(d.Sim, b, seat, d.Seat)
		if err != nil {
			r.err = err
			return
		}
		values := make([]float32, Width()+len(MetaColumns))
		if err := r.enc.Encode(pos, values[:Width()]); err != nil {
			r.err = err
			return
		}
		viewer := 0
		if seat != sortedActors(b)[0] {
			viewer = 1
		}
		r.pending = append(r.pending, recordedRow{viewer: viewer, round: b.Segment.Round, values: values})
		r.seats = append(r.seats, seat)
	}
}

func (r *recorder) Finish(final *state.Battle, record lookahead.BattleRecord, hero, foe lookahead.Policy) {
	defer func() { r.round, r.pending, r.seats = 0, nil, nil }()
	d := r.data
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.err != nil {
		if d.err == nil {
			d.err = r.err
		}
		r.err = nil
		return
	}
	if record.Result == "truncated" {
		d.skipped++
		return
	}
	if _, err := engine.SettledLibrary(final); err == nil {
		d.hashes[hex.EncodeToString(final.SettledCatalogHash[:])] = true
	}
	player := -1
	for i, name := range d.players {
		if name == hero.Name() {
			player = i
		}
	}
	if player < 0 {
		player = len(d.players)
		d.players = append(d.players, hero.Name())
	}
	heroSeatB := 0
	if record.Seat != sortedActors(final)[0] {
		heroSeatB = 1
	}
	for i, row := range r.pending {
		seat := r.seats[i]
		meta := row.values[Width():]
		meta[1], meta[2], meta[3] = float32(record.Seed), float32(row.round), float32(row.viewer)
		if seat == record.Seat {
			meta[4] = 1
		}
		meta[5], meta[6], meta[7] = float32(player), float32(Result(final, seat)), float32(Margin(final, seat))
		row.player, row.seed, row.heroSeatB = player, int(record.Seed), heroSeatB
		d.rows = append(d.rows, row)
	}
}

// Result scores a finished battle for one seat: win 1, draw 0.5, loss 0.
func Result(b *state.Battle, seat string) float64 {
	switch b.WinnerActorID {
	case seat:
		return 1
	case "":
		return 0.5
	}
	return 0
}

// Margin is the seat's health as a share of its maximum minus the strongest
// opponent's, the health race the look-ahead's evaluator scores.
func Margin(b *state.Battle, seat string) float64 {
	own := b.Actors[seat]
	frac := float64(own.CurrentHealth()) / float64(max(lookahead.MaxHealth(own), 1))
	foe := 0.0
	for _, id := range lookahead.Opponents(b, seat) {
		a := b.Actors[id]
		foe = math.Max(foe, float64(a.CurrentHealth())/float64(max(lookahead.MaxHealth(a), 1)))
	}
	return frac - foe
}

func sortedActors(b *state.Battle) []string {
	ids := make([]string, 0, len(b.Actors))
	for id := range b.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
