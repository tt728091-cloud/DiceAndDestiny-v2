// Package lookahead holds battle players that decide by simulating the real
// battle engine forward ("look-ahead"), plus the baselines they are measured
// against. Nothing here is trained: every player is a fixed procedure, and the
// balance lab compares them by playing real authority battles.
package lookahead

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Simulator advances private copies of a battle with the authority's engine.
// It never touches a repository: look-ahead copies are thrown away.
type Simulator struct {
	engine engine.Engine
}

func NewSimulator() (*Simulator, error) {
	eng, err := engine.NewEngineWithConfig(engine.Config{OmitSnapshotContentCatalog: true}, engine.DefaultFlows()...)
	if err != nil {
		return nil, err
	}
	return &Simulator{engine: eng}, nil
}

// Terminal reports whether the battle has ended.
func Terminal(b *state.Battle) bool {
	return state.IsTerminalBattleStatus(b.Status)
}

// Next returns the actor the authority is waiting on and its legal commands
// in the engine's order. (Real battles list them in the environment's
// canonical order; look-ahead copies skip that sort, which only reorders
// equivalent commands.)
func (s *Simulator) Next(b *state.Battle) (string, []command.Command) {
	if Terminal(b) {
		return "", nil
	}
	for _, id := range seatOrder(b) {
		if actions := s.engine.LegalActions(b, id); len(actions) > 0 {
			return id, actions
		}
	}
	return "", nil
}

// Apply submits one listed command to the copy.
func (s *Simulator) Apply(b *state.Battle, cmd command.Command) error {
	_, err := s.engine.ApplyBattleCommand(b, cmd)
	return err
}

// View builds the acting seat's viewer-safe result, the input scripted
// opponents read in a real session.
func (s *Simulator) View(b *state.Battle, seat string, actions []command.Command) engine.Result {
	result := s.engine.OpenResult(b, seat)
	result.LegalActions = actions
	return result
}

// Snapshot is the viewer-safe snapshot View carries, without the legal
// actions: opponents' hands are counts, private planning is hidden.
func (s *Simulator) Snapshot(b *state.Battle, seat string) snapshot.Battle {
	return ViewSnapshot(b, seat)
}

// ViewSnapshot is the seat's viewer-safe snapshot, the one Simulator.View
// carries.
func ViewSnapshot(b *state.Battle, seat string) snapshot.Battle {
	return snapshot.FromBattleForViewerWithoutContentCatalog(*b, seat)
}

// Fork copies a battle for look-ahead with fresh randomness, so the copy
// cannot know the real future dice, draws or damage picks. Hidden opponent
// cards are re-dealt from what the viewer cannot see.
func Fork(b *state.Battle, viewer string, seed uint64) state.Battle {
	c := b.Clone()
	c.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: seed}
	determinize(&c, viewer, rand.New(rand.NewSource(int64(seed^0x6c6f6f6b))))
	return c
}

// determinize re-deals each opponent's hand from its hand plus draw pile, so
// a look-ahead copy only knows what the viewer can: counts, and the cards the
// battle has already shown (revealed for removal, being played, or otherwise
// named in pending effects), which stay where they are.
func determinize(b *state.Battle, viewer string, rng *rand.Rand) {
	if b.Settled == nil {
		return
	}
	var pinned string
	for id, actor := range b.Actors {
		if id == viewer || len(actor.Cards.Hand) == 0 || len(actor.Cards.Deck) == 0 {
			continue
		}
		if pinned == "" {
			pinned = referencedCards(b)
		}
		var free []int
		for i, card := range actor.Cards.Hand {
			if !strings.Contains(pinned, `"`+card+`"`) {
				free = append(free, i)
			}
		}
		if len(free) == 0 {
			continue
		}
		pool := append([]string(nil), actor.Cards.Deck...)
		for _, i := range free {
			pool = append(pool, actor.Cards.Hand[i])
		}
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		hand := append([]string(nil), actor.Cards.Hand...)
		for k, i := range free {
			hand[i] = pool[k]
		}
		actor.Cards.Hand = hand
		actor.Cards.Deck = append([]string(nil), pool[len(free):]...)
		b.Actors[id] = actor
	}
}

// referencedCards serializes the battle's pending bookkeeping (everything
// except the card lists themselves) so a card instance named anywhere in it
// can be found.
func referencedCards(b *state.Battle) string {
	settled := *b.Settled
	settled.Actors = map[string]state.SettledActorRuntime{}
	for id, rt := range b.Settled.Actors {
		rt.CardInstances = nil
		settled.Actors[id] = rt
	}
	raw, _ := json.Marshal([]any{settled, b.DamageResolution, b.PendingOperations, b.Resolutions, b.Commitments, b.OffensiveProposals, b.DefensiveProposals})
	return string(raw)
}

func seatOrder(b *state.Battle) []string {
	ids := make([]string, 0, len(b.Actors))
	for id := range b.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if b.Settled != nil && b.Settled.Window != nil && b.Settled.Window.RequiredActorID != "" {
		first := b.Settled.Window.RequiredActorID
		for i, id := range ids {
			if id == first {
				return append([]string{id}, append(ids[:i:i], ids[i+1:]...)...)
			}
		}
	}
	return ids
}

// MaxHealth counts every card the actor brought, removed ones included.
func MaxHealth(actor state.ActorState) int {
	return actor.CurrentHealth() + len(actor.Cards.Removed)
}

// Opponents lists the other team's actor IDs.
func Opponents(b *state.Battle, seat string) []string {
	team := b.Actors[seat].TeamID
	var out []string
	for id, actor := range b.Actors {
		if id != seat && (team == "" || actor.TeamID != team) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func library(b *state.Battle) (content.BattleLibrary, error) {
	lib, err := engine.SettledLibrary(b)
	if err != nil {
		return lib, fmt.Errorf("battle catalog: %w", err)
	}
	return lib, nil
}
