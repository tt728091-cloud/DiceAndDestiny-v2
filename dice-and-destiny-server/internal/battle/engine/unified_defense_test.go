package engine

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func unifiedFixture(t *testing.T, amounts ...int) (state.Battle, content.BattleLibrary, Engine) {
	t.Helper()
	b, lib := adventurerFixture(t)
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 71}
	b.Settled.UnifiedDefense = true
	for id, runtime := range b.Settled.Actors {
		runtime.HandLimit = 99
		b.Settled.Actors[id] = runtime
	}
	b.Segment.Current = segment.Defensive
	b.Settled.Stage = ""
	closeSettledWindow(&b)
	a := b.Actors["player"]
	a.Cards = state.CardZones{}
	var ids []string
	for _, entry := range lib.Combatants["adventurer"].Decklist {
		for n := 0; n < entry.Count; n++ {
			for id, card := range b.Settled.Actors["player"].CardInstances {
				if card.DefinitionID == entry.CardID && !containsString(ids, id) {
					ids = append(ids, id)
					break
				}
			}
		}
	}
	sort.Strings(ids)
	a.Cards.Discard = append([]string{}, ids[:3]...)
	a.Cards.Deck = append([]string{}, ids[3:7]...)
	a.Cards.Hand = append([]string{}, ids[7:]...)
	b.Actors["player"] = a
	b.Settled.OffensiveSources = nil
	for i, amount := range amounts {
		b.Settled.OffensiveSources = append(b.Settled.OffensiveSources, state.SettledDamageSource{ID: string(rune('a' + i)), SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: amount})
	}
	e := NewEngine()
	if _, err := e.progressSettledDefensive(&b, lib); err != nil {
		t.Fatal(err)
	}
	return b, lib, e
}
func activeReservations(b state.Battle, source string) []string {
	var ids []string
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.Accepted && !r.Released && containsString(r.DamageProposalIDs, source) {
			ids = append(ids, r.CardID)
		}
	}
	return ids
}
func TestUnifiedSourcesRemainSeparateAndStable(t *testing.T) {
	b, _, e := unifiedFixture(t, 4, 6)
	a := activeReservations(b, "a")
	if len(a) != 4 || len(activeReservations(b, "b")) != 6 {
		t.Fatal("attack reservations missing")
	}
	seen := map[string]bool{}
	for _, r := range b.Settled.PendingDamage.Removals {
		if len(r.DamageProposalIDs) != 1 || seen[r.CardID] {
			t.Fatal("shared or duplicated reservation")
		}
		seen[r.CardID] = true
	}
	source := batchSourceByID(b.Settled.PendingDamage, "b")
	source.ScaleNumerator = 1
	source.ScaleDenominator = 2
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, activeReservations(b, "a")) || len(activeReservations(b, "b")) != 3 {
		t.Fatal("halving affected the wrong source")
	}
	before, _ := json.Marshal(b.Settled.PendingDamage)
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(b.Settled.PendingDamage)
	if string(before) != string(after) {
		t.Fatal("unchanged reconciliation rerolled cards")
	}
	cloned := b.Clone()
	if !reflect.DeepEqual(cloned.Settled.PendingDamage, b.Settled.PendingDamage) {
		t.Fatal("checkpoint clone lost reservations")
	}
}
func TestUnifiedDefenseRestoresHandThenDeckThenDiscard(t *testing.T) {
	b, _, e := unifiedFixture(t, 10)
	before := b.Actors["player"].Cards
	batchSourceByID(b.Settled.PendingDamage, "a").Prevention = 5
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	zones := map[operation.CardZone]int{}
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.Released {
			zones[r.ReleasedDestination]++
		}
	}
	if zones[operation.ZoneHand] != 3 || zones[operation.ZoneDeck] != 2 || zones[operation.ZoneDiscard] != 0 {
		t.Fatalf("restoration order: %v", zones)
	}
	if !reflect.DeepEqual(before, b.Actors["player"].Cards) {
		t.Fatal("rolled defense moved cards")
	}
}
func TestUnifiedCardPreventionRetainsAndFollowsPlayedCard(t *testing.T) {
	b, _, e := unifiedFixture(t, 10)
	// A threatened hand card is played before protection. Saving it must not
	// undo that play by returning it to hand.
	var played string
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.OriginalZone == operation.ZoneHand {
			played = r.CardID
			break
		}
	}
	a := b.Actors["player"]
	moveCard(&a.Cards, played, operation.ZoneHand, operation.ZoneDiscard)
	b.Actors["player"] = a
	batchSourceByID(b.Settled.PendingDamage, "a").Prevention = 10
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	if !containsString(b.Actors["player"].Cards.Discard, played) {
		t.Fatal("defense undid a played card")
	}
	b, _, e = unifiedFixture(t, 10)
	batchSourceByID(b.Settled.PendingDamage, "a").ReactionPrevention = 5
	hp := b.Actors["player"].CurrentHealth()
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.Released && (r.ReleasedDestination != r.OriginalZone || !containsString(zoneCards(b.Actors["player"].Cards, r.OriginalZone), r.CardID)) {
			t.Fatal("card prevention changed pile")
		}
	}
	if b.Actors["player"].CurrentHealth() != hp {
		t.Fatal("prevention reduced health")
	}
}
func TestUnifiedPassFinishesWithoutSelectingDefense(t *testing.T) {
	b, lib, e := unifiedFixture(t, 4, 6)
	actions := e.LegalActions(&b, "player")
	var pass command.Command
	for _, a := range actions {
		if a.Type == command.TypePlanningPass {
			pass = a
		}
	}
	if pass.Type == "" {
		t.Fatal("missing final pass")
	}
	events, err := e.handleSettledCommand(&b, pass)
	if err != nil {
		t.Fatal(err)
	}
	if b.Segment.Current == segment.DamageResolution || (b.Segment.Current == segment.Defensive && !state.IsTerminalBattleStatus(b.Status)) {
		t.Fatalf("pass did not finish combined segment: %s/%s", b.Segment.Current, b.Settled.Stage)
	}
	if len(b.Actors["player"].Cards.Removed) != 10 {
		t.Fatal("final damage not committed")
	}
	_ = events
	_ = lib
}
func TestUnifiedDefenseReturnsToHubAndBraceIsLegalBeforeRoll(t *testing.T) {
	b, lib, e := unifiedFixture(t, 4, 3)
	// Put one Brace in hand without changing total health.
	a := b.Actors["player"]
	for id, card := range b.Settled.Actors["player"].CardInstances {
		if card.DefinitionID == "brace" {
			for _, z := range []operation.CardZone{operation.ZoneDeck, operation.ZoneDiscard} {
				moveCard(&a.Cards, id, z, operation.ZoneHand)
			}
			break
		}
	}
	b.Actors["player"] = a
	var defense, brace command.Command
	for _, action := range e.LegalActions(&b, "player") {
		if action.Type == command.TypePlanningAbility {
			defense = action
		}
		var p command.CommitInteractionPayload
		_ = json.Unmarshal(action.Payload, &p)
		if len(p.Commitment.CardIDs) > 0 && settledCardDefinitionID(&b, "player", p.Commitment.CardIDs[0]) == "brace" {
			brace = action
		}
	}
	if brace.Type == "" || defense.Type == "" {
		t.Fatal("cards and defenses must be legal together")
	}
	if _, err := e.handleSettledCommand(&b, brace); err != nil {
		t.Fatal(err)
	}
	// Two viable attacks: Brace asks for its source after the card click.
	for _, action := range e.LegalActions(&b, "player") {
		_, key := programPayload(action)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "target" {
			if _, err := e.handleSettledCommand(&b, action); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if b.Settled.Actors["player"].CardExecution != nil {
		t.Fatal("Brace did not resolve against a chosen attack")
	}
	// Choose a current remaining source after prevention.
	defense = command.Command{}
	for _, action := range e.LegalActions(&b, "player") {
		if action.Type == command.TypePlanningAbility {
			defense = action
			break
		}
	}
	if _, err := e.handleSettledCommand(&b, defense); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage == stageDefenseRoll {
		actions := e.LegalActions(&b, "player")
		if _, err := e.handleSettledCommand(&b, actions[0]); err != nil {
			t.Fatal(err)
		}
	}
	if b.Settled.Stage != stageDefenseReact {
		t.Fatalf("missing roll reaction %s", b.Settled.Stage)
	}
	var apply command.Command
	for _, action := range e.LegalActions(&b, "player") {
		if action.Type == command.TypePlanningPass {
			apply = action
		}
	}
	if _, err := e.handleSettledCommand(&b, apply); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Stage != stageDefenseSelect || b.Settled.DefensePassed["player"] {
		t.Fatal("finishing one defense ended the entire segment")
	}
	_ = lib
}

func TestUnifiedQueuedStatusesAndProtect(t *testing.T) {
	b, lib, e := unifiedFixture(t, 4)
	applyStatus(&b, lib, "player", "protect", 2)
	var protect command.Command
	for _, a := range e.LegalActions(&b, "player") {
		var p command.CommitInteractionPayload
		_ = json.Unmarshal(a.Payload, &p)
		if p.Commitment.ChoiceID == "spend_round_prevention" {
			protect = a
		}
	}
	if protect.Type == "" {
		t.Fatal("Protect unavailable before rolling")
	}
	if _, err := e.handleSettledCommand(&b, protect); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "protect") != 0 || len(activeReservations(b, "a")) != 2 {
		t.Fatal("Protect failed to consume or save")
	}
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.Released && r.ReleasedDestination != operation.ZoneDiscard {
			t.Fatal("Protect did not discard")
		}
	}
	// An authored attack status remains pending despite full prevention.
	b.Settled.PendingDamage.Applications = []state.SettledStatusApplication{{SourceActorID: "enemy", TargetActorID: "player", StatusID: "blind", Stacks: 1}}
	batchSourceByID(b.Settled.PendingDamage, "a").Prevention = 99
	if err := e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "blind") != 0 {
		t.Fatal("attack status applied before commit")
	}
	if _, err := e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if stacks(&b, "player", "blind") != 1 {
		t.Fatal("blocked attack lost its unconditional status")
	}
}

func TestUnifiedOverkillAndCheckpointReplay(t *testing.T) {
	b, _, e := unifiedFixture(t, 7, 7)
	before := activeReservations(b, "b")
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var restored state.Battle
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, battle := range []*state.Battle{&b, &restored} {
		batchSourceByID(battle.Settled.PendingDamage, "a").Prevention = 3
		if err = e.reconcileUnifiedDamage(battle, true); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(b.Settled.PendingDamage, restored.Settled.PendingDamage) {
		t.Fatal("saved checkpoint changed random restoration")
	}
	seen := map[string]bool{}
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.Accepted && !r.Released {
			if seen[r.CardID] {
				t.Fatal("overkill duplicated a reservation")
			}
			seen[r.CardID] = true
		}
	}
	if len(seen) != 11 {
		t.Fatalf("overkill dropped damage: %d", len(seen))
	}
	for _, id := range before {
		if !containsString(activeReservations(b, "b"), id) {
			t.Fatal("other source's existing card changed")
		}
	}
}

func TestUnifiedSpecializedPreventionChoices(t *testing.T) {
	for _, id := range []string{"coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal", "spiteful_ward"} {
		t.Run(id, func(t *testing.T) {
			b, lib := curseFixture(t)
			b.Settled.UnifiedDefense = true
			b.Segment.Current = segment.Defensive
			b.Settled.Stage = ""
			closeSettledWindow(&b)
			a := b.Actors["player"]
			a.Cards.Hand = append(a.Cards.Hand, "test-card")
			b.Actors["player"] = a
			r := b.Settled.Actors["player"]
			r.CardInstances["test-card"] = state.CardInstance{InstanceID: "test-card", DefinitionID: id}
			b.Settled.Actors["player"] = r
			applyStatus(&b, lib, "enemy", "poison", 2)
			b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "hexbrand", BaseAmount: 6}}
			e := NewEngine()
			if _, err := e.progressSettledDefensive(&b, lib); err != nil {
				t.Fatal(err)
			}
			// Fixture actor ordering can put enemy first; open the player's hub input.
			openSettledWindowForActors(&b, "test", stageDefenseSelect, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePlanningPass}, []string{"player"}, false)
			var chosen command.Command
			for _, a := range e.LegalActions(&b, "player") {
				var p command.CommitInteractionPayload
				_ = json.Unmarshal(a.Payload, &p)
				if containsString(p.Commitment.CardIDs, "test-card") {
					chosen = a
					break
				}
			}
			if chosen.Type == "" {
				t.Fatal("specialized prevention lost in unified hub")
			}
			if _, err := e.handleSettledCommand(&b, chosen); err != nil {
				t.Fatal(err)
			}
			batch := b.Settled.PendingDamage
			if batch == nil && b.Settled.Venom != nil && b.Settled.Venom.Resume != nil {
				batch = b.Settled.Venom.Resume.Damage
			}
			if batch == nil || settledSourceAmount(batch.Sources[0]) >= 6 {
				t.Fatal("specialized card did not prevent source damage")
			}
		})
	}
}

func TestUnifiedPreventionUsesCurrentDamageInPlayOrder(t *testing.T) {
	b, lib, e := unifiedFixture(t, 7)
	card := effectResult{Preventions: []effectPrevention{{ProposalID: "a", Amount: 3}}}
	half := effectResult{Scales: []effectScale{{ProposalID: "a", Numerator: 1, Denominator: 2}}}
	if err := e.applyEffectMutations(&b, lib, "brace", card); err != nil {
		t.Fatal(err)
	}
	if err := e.applyEffectMutations(&b, lib, "", half); err != nil {
		t.Fatal(err)
	}
	if n := settledSourceAmount(b.Settled.PendingDamage.Sources[0]); n != 2 {
		t.Fatalf("7 minus 3, then half must be 2, got %d", n)
	}
	if len(activeReservations(b, "a")) != 2 {
		t.Fatal("ordered arithmetic and cards disagree")
	}
	encoded, _ := json.Marshal(b)
	var restored state.Battle
	_ = json.Unmarshal(encoded, &restored)
	if err := e.applyEffectMutations(&restored, lib, "", effectResult{Preventions: []effectPrevention{{ProposalID: "a", Amount: 1}}}); err != nil {
		t.Fatal(err)
	}
	if n := settledSourceAmount(restored.Settled.PendingDamage.Sources[0]); n != 1 {
		t.Fatalf("later defense must subtract from current damage: %d", n)
	}
	b, lib, e = unifiedFixture(t, 7)
	if err := e.applyEffectMutations(&b, lib, "", half); err != nil {
		t.Fatal(err)
	}
	if err := e.applyEffectMutations(&b, lib, "brace", card); err != nil {
		t.Fatal(err)
	}
	if settledSourceAmount(b.Settled.PendingDamage.Sources[0]) != 0 {
		t.Fatal("half of 7 rounded down, then prevent 3 must be zero")
	}
}

func TestUnifiedDrawnThreatenedCardStaysInHandWhenSaved(t *testing.T) {
	b, _, e := unifiedFixture(t, 10)
	id, err := e.drawSettledCard(&b, "player", "card_draw")
	if err != nil {
		t.Fatal(err)
	}
	batchSourceByID(b.Settled.PendingDamage, "a").Prevention = 4
	if err = e.reconcileUnifiedDamage(&b, true); err != nil {
		t.Fatal(err)
	}
	if !containsString(b.Actors["player"].Cards.Hand, id) {
		t.Fatal("defense undid the draw")
	}
	for _, r := range b.Settled.PendingDamage.Removals {
		if r.CardID == id && (!r.Released || r.OriginalZone != operation.ZoneDeck || r.ReleasedDestination != operation.ZoneHand) {
			t.Fatalf("live pile restoration: %+v", r)
		}
	}
}

func TestUnifiedPreservesDamageExitCleanup(t *testing.T) {
	b, _ := curseFixture(t)
	b.Settled.UnifiedDefense = true
	b.Segment.Current = segment.Defensive
	c := curseRuntime(&b)
	c.Bags = map[string][]int{"enemy": {1, 2}}
	c.Preparations = []state.CursePreparation{{CardID: "black_fingerprint", Source: "player", Target: "enemy", Round: b.Segment.Round, ExpiresEffects: 999}}
	b.Settled.Stage = stageDefenseSelect
	expireCursePreparations(&b)
	if len(c.Preparations) != 1 || len(c.Bags) != 1 {
		t.Fatal("cleanup ran before combined damage finished")
	}
	b.Settled.Stage = "complete"
	expireCursePreparations(&b)
	if len(c.Preparations) != 0 || len(c.Bags) != 0 {
		t.Fatal("skipping Damage skipped its cleanup")
	}
}
