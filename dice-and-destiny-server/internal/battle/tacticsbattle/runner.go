// Package tacticsbattle runs Tactics Sandbox battles for the client
// (docs/game-design/tactics-sandbox-plan.md): it starts a battle through the
// authority with the pinned tactics setup, stores it on disk so it can be
// resumed, and after every human command drives the scripted enemies through
// the same legal actions, from their own viewer-safe view.
package tacticsbattle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/tactics"
	"diceanddestiny/server/internal/content"
)

// HeroSeat is the human's seat; enemies sit in EnemySeat(1..4).
const HeroSeat = "hero"

// EnemySeat is the seat of the n-th enemy (1-based).
func EnemySeat(n int) string { return fmt.Sprintf("enemy-%d", n) }

// Battle IDs of the sandbox all start with this prefix.
const BattlePrefix = "tactics-"

// BattleRootEnv is where the launcher keeps this workspace's battles.
const BattleRootEnv = "DICE_AND_DESTINY_BATTLE_STATE_ROOT"

// Roots locate the content, the published cards and the battle store.
type Roots struct {
	ContentRoot string
	LoadoutRoot string
	BattleRoot  string
}

func (r Roots) battleRoot() (string, error) {
	root := r.BattleRoot
	if root == "" {
		root = os.Getenv(BattleRootEnv)
	}
	if root == "" {
		return "", errors.New("no battle store: set battle_root or " + BattleRootEnv)
	}
	return filepath.Join(root, "tactics"), nil
}

// Setup is the sandbox setup screen's choice.
type Setup struct {
	Hero string `json:"hero"`
	// MapCards are map Move card definitions added to the hero's deck.
	MapCards []string `json:"map_cards,omitempty"`
	// Enemies are 1–4 enemy definitions, placed on the arena's enemy
	// starts in order.
	Enemies   []string        `json:"enemies"`
	Arena     string          `json:"arena"`
	Overrides json.RawMessage `json:"overrides,omitempty"`
	Seed      uint64          `json:"seed,omitempty"`
}

var mu sync.Mutex

// loaded is one content load: the battle catalog and the tactics pack.
type loaded struct {
	library  content.BattleLibrary
	manifest content.TacticsPackManifest
	pack     tactics.Pack
}

func load(roots Roots) (loaded, error) {
	library, manifest, err := battle.LoadTacticsLibrary(roots.ContentRoot, roots.LoadoutRoot)
	if err != nil {
		return loaded{}, err
	}
	pack, err := tactics.LoadPack(filepath.Join(roots.ContentRoot, battle.TacticsPack))
	if err != nil {
		return loaded{}, err
	}
	return loaded{library: library, manifest: manifest, pack: pack}, nil
}

// Catalog lists what the setup screen offers: heroes, enemies, arenas, the
// rules and the published map Move cards.
func Catalog(roots Roots) (map[string]any, error) {
	l, err := load(roots)
	if err != nil {
		return nil, err
	}
	fallback := content.AbilityDelivery{Type: l.pack.Rules.DefaultDelivery.Type, Range: l.pack.Rules.DefaultDelivery.Range, LineOfSight: l.pack.Rules.DefaultDelivery.LineOfSight}
	describe := func(id string) map[string]any {
		c := l.library.Combatants[id]
		var attacks, defenses []map[string]any
		for _, a := range c.AbilityBoard.Offensive {
			d := content.EffectiveDelivery(l.library.Abilities[a], fallback)
			attacks = append(attacks, map[string]any{"id": a, "name": l.library.Abilities[a].Name, "delivery": content.DeliveryText(d)})
		}
		for _, a := range c.AbilityBoard.Defensive {
			defenses = append(defenses, map[string]any{"id": a, "name": l.library.Abilities[a].Name, "on_the_move": l.library.Abilities[a].OnTheMove})
		}
		cards := 0
		for _, entry := range c.Decklist {
			cards += entry.Count
		}
		entry := map[string]any{"id": id, "name": c.Name, "health": cards, "attacks": attacks, "defenses": defenses, "base_speed": l.pack.Rules.BaseSpeed}
		if c.Tactics != nil {
			entry["base_speed"], entry["role"], entry["label"] = c.Tactics.BaseSpeed, c.Tactics.Role, c.Tactics.Label
		}
		return entry
	}
	heroes, enemies, arenas, mapCards := []map[string]any{}, []map[string]any{}, []map[string]any{}, []map[string]any{}
	for _, id := range l.manifest.Heroes {
		heroes = append(heroes, describe(id))
	}
	for _, id := range l.manifest.Enemies {
		enemies = append(enemies, describe(id))
	}
	for _, a := range l.pack.SortedArenas() {
		var rows [][]string
		for y := 0; y < a.Height(); y++ {
			row := make([]string, a.Width())
			for x := range row {
				row[x] = a.TerrainAt(tactics.Square{X: x, Y: y})
			}
			rows = append(rows, row)
		}
		arenas = append(arenas, map[string]any{"id": a.ID, "name": a.Name, "description": a.Description, "width": a.Width(), "height": a.Height(), "terrain": rows, "player_start": a.PlayerStart, "enemy_starts": a.EnemyStarts})
	}
	var ids []string
	for id, card := range l.library.Cards {
		if card.MapEffect != nil && card.MapEffect.Kind == "move" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		card := l.library.Cards[id]
		mapCards = append(mapCards, map[string]any{"id": id, "name": card.Name, "amount": card.MapEffect.Amount, "energy": card.Cost.Energy, "ignore_terrain": card.MapEffect.IgnoreTerrain, "choose_terrain": card.MapEffect.ChooseTerrain, "rules": card.Presentation.RulesText})
	}
	return map[string]any{"heroes": heroes, "enemies": enemies, "arenas": arenas, "rules": l.pack.Rules, "map_cards": mapCards}, nil
}

type runner struct {
	// authority answers the hero with the content catalog; quiet answers the
	// enemies without it (they read only their view and legal actions).
	authority *battle.Authority
	quiet     *battle.Authority
	repo      *writeBehind
	battleID  string
	policy    enemyPolicy
	stored    stored
}

// stored is what the runner keeps beside a battle: the setup, and each
// enemy seat's scripted behaviour, pinned at start like the rules.
type stored struct {
	Setup   Setup                  `json:"setup"`
	Enemies map[string]EnemyConfig `json:"enemies"`
}

// EnemyConfig is one enemy seat's pinned behaviour.
type EnemyConfig struct {
	Definition string                      `json:"definition"`
	Keep       content.SingleAbilityPolicy `json:"keep"`
	Role       string                      `json:"role"`
}

func newRunner(roots Roots, battleID string, assembler battle.ParticipantAssembler) (*runner, error) {
	root, err := roots.battleRoot()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(battleID, BattlePrefix) {
		return nil, fmt.Errorf("tactics battle IDs start with %q", BattlePrefix)
	}
	if assembler == nil {
		assembler = battle.ParticipantAssemblerFunc(func([]battle.Participant) (state.BattleSetup, error) {
			return state.BattleSetup{}, errors.New("a stored sandbox battle cannot start again")
		})
	}
	repo := &writeBehind{disk: repository.NewDisk(root)}
	quiet, err := engine.NewEngineWithConfig(engine.Config{OmitSnapshotContentCatalog: true}, engine.DefaultFlows()...)
	if err != nil {
		return nil, err
	}
	r := &runner{authority: battle.NewAuthority(engine.NewEngine(), repo, assembler), quiet: battle.NewAuthority(quiet, repo, assembler), repo: repo, battleID: battleID}
	return r, nil
}

// Start creates a sandbox battle and returns the hero's view.
func Start(roots Roots, battleID string, setup Setup) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	l, err := load(roots)
	if err != nil {
		return nil, err
	}
	if !contains(l.manifest.Heroes, setup.Hero) {
		return nil, fmt.Errorf("unknown sandbox hero %q", setup.Hero)
	}
	arena, ok := l.pack.Arenas[setup.Arena]
	if !ok {
		return nil, fmt.Errorf("unknown arena %q", setup.Arena)
	}
	if len(setup.Enemies) < 1 || len(setup.Enemies) > len(arena.EnemyStarts) || len(setup.Enemies) > 4 {
		return nil, fmt.Errorf("choose 1–%d enemies", min(4, len(arena.EnemyStarts)))
	}
	for _, id := range setup.Enemies {
		if !contains(l.manifest.Enemies, id) {
			return nil, fmt.Errorf("unknown sandbox enemy %q", id)
		}
	}
	rules, err := tactics.ApplyOverrides(l.pack.Rules, setup.Overrides)
	if err != nil {
		return nil, err
	}
	if err := tactics.ValidateArena(arena, rules); err != nil {
		return nil, err
	}
	deck := map[string]int{}
	var order []string
	for _, entry := range l.library.Combatants[setup.Hero].Decklist {
		if deck[entry.CardID] == 0 {
			order = append(order, entry.CardID)
		}
		deck[entry.CardID] += entry.Count
	}
	for _, id := range setup.MapCards {
		card, ok := l.library.Cards[id]
		if !ok || card.MapEffect == nil || card.MapEffect.Kind != "move" {
			return nil, fmt.Errorf("%q is not a published map Move card", id)
		}
		if deck[id] == 0 {
			order = append(order, id)
		}
		deck[id]++
	}
	var heroDeck []loadout.Entry
	for _, id := range order {
		heroDeck = append(heroDeck, loadout.Entry{CardID: id, Count: deck[id]})
	}
	seed := setup.Seed
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}
	setup.Seed = seed
	placement := map[string]tactics.Square{HeroSeat: arena.PlayerStart}
	seats := []command.ParticipantDescriptor{{InstanceID: HeroSeat, DefinitionID: setup.Hero, TeamID: "heroes"}}
	for i, id := range setup.Enemies {
		seat := EnemySeat(i + 1)
		placement[seat] = arena.EnemyStarts[i]
		seats = append(seats, command.ParticipantDescriptor{InstanceID: seat, DefinitionID: id, TeamID: "monsters"})
	}
	catalog, err := json.Marshal(l.library)
	if err != nil {
		return nil, err
	}
	assembler := battle.WithDeckLoadouts(battle.ParticipantAssemblerFunc(func(participants []battle.Participant) (state.BattleSetup, error) {
		return battle.AssembleTacticsParticipants(participants, l.library, catalog)
	}), map[string][]loadout.Entry{HeroSeat: heroDeck})
	r, err := newRunner(roots, battleID, assembler)
	if err != nil {
		return nil, err
	}
	r.stored = stored{Setup: setup, Enemies: map[string]EnemyConfig{}}
	for i, id := range setup.Enemies {
		c := l.library.Combatants[id]
		r.stored.Enemies[EnemySeat(i+1)] = EnemyConfig{Definition: id, Keep: *c.SingleAbilityPolicy, Role: c.Tactics.Role}
	}
	r.policy = scriptedPolicy{enemies: r.stored.Enemies}
	payload, err := json.Marshal(command.StartBattlePayload{UnifiedDefense: true, SimultaneousPlay: true, Seats: seats, Seed: &seed, Tactics: &command.TacticsSetup{Arena: arena, Rules: rules, Placement: placement}})
	if err != nil {
		return nil, err
	}
	started := r.authority.HandleCommand(command.Command{BattleID: battleID, ActorID: HeroSeat, Type: command.TypeStartBattle, Payload: payload})
	if !started.Accepted {
		return nil, fmt.Errorf("the sandbox battle could not start: %s", started.Error)
	}
	if err := r.save(roots); err != nil {
		return nil, err
	}
	events := started.Events
	driven, err := r.driveEnemies()
	if err != nil {
		return nil, err
	}
	return r.present(append(events, driven...))
}

// Open resumes a stored sandbox battle: it lets any enemy that still owes a
// decision act, then returns the hero's view.
func Open(roots Roots, battleID string) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	r, err := reopen(roots, battleID)
	if err != nil {
		return nil, err
	}
	if _, err := r.authority.InspectBattleState(battleID); err != nil {
		return nil, fmt.Errorf("sandbox battle %s: %w", battleID, err)
	}
	driven, err := r.driveEnemies()
	if err != nil {
		return nil, err
	}
	return r.present(driven)
}

// History returns every resolved round of a sandbox battle as the hero may
// see it (movement, attacks, hits and health), oldest first, so the client
// can replay the whole battle.
func History(roots Roots, battleID string) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	r, err := reopen(roots, battleID)
	if err != nil {
		return nil, err
	}
	stored, err := r.authority.InspectBattleState(battleID)
	if err != nil {
		return nil, fmt.Errorf("sandbox battle %s: %w", battleID, err)
	}
	raw, err := json.Marshal(map[string]any{"battle_id": battleID, "rounds": snapshot.TacticsHistory(stored, HeroSeat)})
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func reopen(roots Roots, battleID string) (*runner, error) {
	r, err := newRunner(roots, battleID, nil)
	if err != nil {
		return nil, err
	}
	if err := r.load(roots); err != nil {
		return nil, err
	}
	r.policy = scriptedPolicy{enemies: r.stored.Enemies}
	return r, nil
}

// Submit applies one hero command, lets the enemies answer, and returns the
// hero's view with every event the hero may see.
func Submit(roots Roots, battleID, commandJSON string) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	r, err := reopen(roots, battleID)
	if err != nil {
		return nil, err
	}
	normalized, err := integralNumbers(commandJSON)
	if err != nil {
		return nil, err
	}
	cmd, err := command.ParseJSON(normalized)
	if err != nil {
		return nil, err
	}
	if cmd.BattleID != battleID || cmd.ActorID != HeroSeat || cmd.Type == command.TypeStartBattle {
		return nil, errors.New("only the hero's commands for this battle are accepted")
	}
	result := r.authority.HandleCommandForViewer(cmd, HeroSeat)
	if !result.Accepted {
		return nil, fmt.Errorf("the authority refused the command: %s", result.Error)
	}
	driven, err := r.driveEnemies()
	if err != nil {
		return nil, err
	}
	return r.present(append(result.Events, driven...))
}

// driveEnemies lets every enemy with a pending decision act, one command at
// a time, until only the hero (or nobody) has a decision left. Each enemy
// decides from its own viewer-safe view and legal actions.
func (r *runner) driveEnemies() ([]event.Event, error) {
	var events []event.Event
	for steps := 0; steps < 2000; steps++ {
		hero := r.quiet.HandleCommand(command.Command{BattleID: r.battleID, ActorID: HeroSeat, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
		if !hero.Accepted {
			return nil, errors.New(hero.Error)
		}
		if hero.Status == engine.ProgressBattleComplete || hero.Snapshot == nil || state.IsTerminalBattleStatus(hero.Snapshot.Status) {
			return events, nil
		}
		acted := false
		var seats []string
		for id := range hero.Snapshot.Actors {
			if id != HeroSeat {
				seats = append(seats, id)
			}
		}
		sort.Strings(seats)
		for _, seat := range seats {
			view := r.quiet.HandleCommand(command.Command{BattleID: r.battleID, ActorID: seat, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
			if !view.Accepted || len(view.LegalActions) == 0 {
				continue
			}
			action, err := r.policy.choose(seat, view)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", seat, err)
			}
			result := r.quiet.HandleCommandForViewer(action, HeroSeat)
			if !result.Accepted {
				return nil, fmt.Errorf("%s's %s was refused: %s", seat, action.Type, result.Error)
			}
			events = append(events, result.Events...)
			acted = true
			break
		}
		if !acted {
			return events, nil
		}
	}
	return nil, errors.New("the enemies did not finish their decisions")
}

func (r *runner) present(events []event.Event) (map[string]any, error) {
	if err := r.repo.flush(); err != nil {
		return nil, err
	}
	setup := r.stored.Setup
	result := r.authority.HandleCommand(command.Command{BattleID: r.battleID, ActorID: HeroSeat, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
	if !result.Accepted {
		return nil, errors.New(result.Error)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var view map[string]any
	if err := decoder.Decode(&view); err != nil {
		return nil, err
	}
	encodedEvents, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	var plain []any
	decoder = json.NewDecoder(bytes.NewReader(encodedEvents))
	decoder.UseNumber()
	if err := decoder.Decode(&plain); err != nil {
		return nil, err
	}
	if plain == nil {
		plain = []any{}
	}
	view["events"] = plain
	enemies := []string{}
	for i := range setup.Enemies {
		enemies = append(enemies, EnemySeat(i+1))
	}
	view["tactics_session"] = map[string]any{"battle_id": r.battleID, "hero_seat": HeroSeat, "enemy_seats": enemies, "setup": setup}
	if result.Snapshot != nil && state.IsTerminalBattleStatus(result.Snapshot.Status) {
		outcome := "draw"
		switch {
		case result.Snapshot.Status == state.BattleDraw || result.Snapshot.WinnerActorID == "":
		case result.Snapshot.WinnerActorID == HeroSeat:
			outcome = "victory"
		default:
			outcome = "defeat"
		}
		view["battle_result"] = outcome
	}
	return view, nil
}

// The setup and pinned enemy behaviour are stored beside the battle.
func (r *runner) storePath(roots Roots) (string, error) {
	root, err := roots.battleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, r.battleID+".setup.json"), nil
}

func (r *runner) save(roots Roots) error {
	path, err := r.storePath(roots)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(r.stored)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func (r *runner) load(roots Roots) error {
	path, err := r.storePath(roots)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("sandbox battle %s is not stored here: %w", r.battleID, err)
	}
	return json.Unmarshal(raw, &r.stored)
}

// integralNumbers rewrites whole numbers written as floats (Godot's JSON
// writes 1 as 1.0) as integers, so they decode into integer fields.
func integralNumbers(raw string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode command: %w", err)
	}
	var walk func(any) any
	walk = func(v any) any {
		switch typed := v.(type) {
		case map[string]any:
			for k, inner := range typed {
				typed[k] = walk(inner)
			}
		case []any:
			for i, inner := range typed {
				typed[i] = walk(inner)
			}
		case json.Number:
			if f, err := typed.Float64(); err == nil && f == float64(int64(f)) {
				return int64(f)
			}
		}
		return v
	}
	out, err := json.Marshal(walk(value))
	return string(out), err
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
