package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// guardCard authors a prevent-1 program card with the given play windows.
func guardCard(t *testing.T, lib *content.BattleLibrary, id string, windows ...string) content.BattleCardDefinition {
	t.Helper()
	c, err := content.EditableGeneralCard(lib.Cards["steady_guard"])
	if err != nil {
		t.Fatal(err)
	}
	c.ID, c.Name = id, id
	c.Program.Windows = windows
	c.Program.Steps[0].Params["amount"] = 1
	c = content.PrepareProgramCard(c, lib)
	if err = content.ValidateCardProgram(c, *lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[id] = c
	return c
}

func giveCardCopy(b *state.Battle, instance, definition string) {
	a := b.Actors["player"]
	a.Cards.Hand = append(a.Cards.Hand, instance)
	b.Actors["player"] = a
	rt := b.Settled.Actors["player"]
	rt.CardInstances[instance] = state.CardInstance{InstanceID: instance, DefinitionID: definition}
	b.Settled.Actors["player"] = rt
}

// startActions maps each legally startable card instance to its command.
func startActions(e Engine, b *state.Battle) map[string]command.Command {
	out := map[string]command.Command{}
	for _, a := range e.LegalActions(b, "player") {
		id, key := programPayload(a)
		var c programChoice
		if id != "" && json.Unmarshal([]byte(key), &c) == nil && c.Verb == "start" && c.Then == "" {
			out[id] = a
		}
	}
	return out
}

func applyFirst(t *testing.T, e Engine, b *state.Battle, kind command.Type) {
	t.Helper()
	for _, a := range e.LegalActions(b, "player") {
		if a.Type == kind {
			if _, err := e.ApplyBattleCommand(b, a); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no %s action at %s", kind, b.Settled.Stage)
}

func remainingDamage(b *state.Battle) int {
	total := 0
	for _, s := range b.Settled.PendingDamage.Sources {
		total += settledSourceAmount(s)
	}
	return total
}

func TestDefenseBeforeRollWindowClosesAfterFirstDefenseRoll(t *testing.T) {
	b, lib, e := unifiedFixture(t, 4, 3)
	early := guardCard(t, &lib, "early_guard", "defense_before_roll")
	guardCard(t, &lib, "any_guard", "defense_selection")
	guardCard(t, &lib, "late_guard", content.CardTimingChoices["defense"]["after"]...)
	guardCard(t, &lib, "review_guard", append(append([]string{}, content.CardTimingChoices["defense"]["after"]...), content.CardReactionWindows["defense"])...)
	if !strings.Contains(early.Presentation.RulesText, "Play: Defense, before your first defense roll.") {
		t.Fatalf("rules text omits timing: %q", early.Presentation.RulesText)
	}
	giveCardCopy(&b, "early-1", "early_guard")
	giveCardCopy(&b, "early-2", "early_guard")
	giveCardCopy(&b, "any-1", "any_guard")
	giveCardCopy(&b, "late-1", "late_guard")
	giveCardCopy(&b, "review-1", "review_guard")
	b.SettledCatalog, _ = json.Marshal(lib)

	// Before any defense roll both timings are offered.
	starts := startActions(e, &b)
	for _, id := range []string{"early-1", "early-2", "any-1"} {
		if _, ok := starts[id]; !ok {
			t.Fatalf("%s not playable before the first defense roll", id)
		}
	}
	if _, ok := starts["late-1"]; ok {
		t.Fatal("after card playable before the first defense roll")
	}
	staleEarly := starts["early-2"]

	// Play one early copy against a chosen attack through the real commands.
	before := remainingDamage(&b)
	if _, err := e.ApplyBattleCommand(&b, starts["early-1"]); err != nil {
		t.Fatal(err)
	}
	target := command.Command{}
	for _, a := range e.LegalActions(&b, "player") {
		_, key := programPayload(a)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "target" && c.Source == "a" {
			target = a
		}
	}
	if target.Type == "" {
		t.Fatal("attack a not offered as a target")
	}
	if _, err := e.ApplyBattleCommand(&b, target); err != nil {
		t.Fatal(err)
	}
	if b.Settled.Actors["player"].CardExecution != nil || remainingDamage(&b) != before-1 || settledSourceAmount(*batchSourceByID(b.Settled.PendingDamage, "a")) != 3 {
		t.Fatalf("early guard prevented %d damage, want 1", before-remainingDamage(&b))
	}

	// Roll one defense and apply it; the hub reopens for the second attack.
	applyFirst(t, e, &b, command.TypePlanningAbility)
	if b.Settled.Stage == stageDefenseRoll {
		if _, err := e.ApplyBattleCommand(&b, e.LegalActions(&b, "player")[0]); err != nil {
			t.Fatal(err)
		}
	}
	if b.Settled.Stage != stageDefenseReact {
		t.Fatalf("missing roll review, stage %s", b.Settled.Stage)
	}
	if _, ok := startActions(e, &b)["early-2"]; ok {
		t.Fatal("before-roll card offered during the roll review")
	}
	// The roll review is an opt-in reaction moment: plain After waits it out.
	if _, ok := startActions(e, &b)["late-1"]; ok {
		t.Fatal("after card without the review opt-in paused the roll review")
	}
	if _, ok := startActions(e, &b)["review-1"]; !ok {
		t.Fatal("review opt-in card not playable during the roll review")
	}
	applyFirst(t, e, &b, command.TypePlanningPass)
	if b.Settled.Stage != stageDefenseSelect || b.Settled.DefensePassed["player"] {
		t.Fatalf("defense did not return to the hub: %s", b.Settled.Stage)
	}

	starts = startActions(e, &b)
	if _, ok := starts["early-2"]; ok {
		t.Fatal("before-roll card still offered after a defense roll")
	}
	if _, ok := starts["any-1"]; !ok {
		t.Fatal("any-time defense card unavailable between defenses")
	}
	if _, ok := starts["late-1"]; !ok {
		t.Fatal("after prevention card unavailable on the Defense screen after a roll")
	}
	if _, err := e.ApplyBattleCommand(&b, staleEarly); err == nil {
		t.Fatal("stale before-roll card command accepted after a defense roll")
	}
}

func TestDefenseBeforeRollWindowRules(t *testing.T) {
	b, lib, _ := unifiedFixture(t, 4)
	windows := []string{"defense_before_roll"}
	if b.Settled.Stage != stageDefenseSelect || !programWindowOpen(&b, "player", windows) {
		t.Fatal("window closed on a fresh Defense screen")
	}
	b.Settled.DefenseHistory = map[string]state.SettledDefense{"skipped": {ActorID: "player", SourceID: "skipped", Finalized: true}}
	if !programWindowOpen(&b, "player", windows) {
		t.Fatal("skipping an attack without rolling closed the window")
	}
	b.Settled.DefenseHistory["other"] = state.SettledDefense{ActorID: "enemy", AbilityID: "guard", SourceID: "other", Finalized: true}
	if !programWindowOpen(&b, "player", windows) {
		t.Fatal("another participant's defense roll closed the window")
	}
	b.Settled.DefenseHistory["rolled"] = state.SettledDefense{ActorID: "player", AbilityID: "adventurer_guard", SourceID: "rolled", Finalized: true}
	if programWindowOpen(&b, "player", windows) {
		t.Fatal("window open after the player's defense roll")
	}
	if !programWindowOpen(&b, "player", []string{"defense_selection"}) {
		t.Fatal("any-time defense window closed after a roll")
	}

	// Offensive-only effects cannot use a defense window.
	c := guardCard(t, &lib, "both_guard", "defense_before_roll", "defense_selection")
	if !strings.Contains(c.Presentation.RulesText, "Play: Defense, any time.") {
		t.Fatalf("any-time card described wrongly: %q", c.Presentation.RulesText)
	}
	c.Program.Steps = []content.CardStep{selfStep("set_die", map[string]any{"faces": []int{6}})}
	c.Program.Windows = []string{"defense_before_roll"}
	if err := content.ValidateCardProgram(c, lib); err == nil {
		t.Fatal("offensive die effect accepted in the before-roll defense window")
	}
}

// cleanseCard authors a remove-all-stacks program card for one timing choice.
func cleanseCard(t *testing.T, lib *content.BattleLibrary, id, segment, timing string) {
	t.Helper()
	c := content.BattleCardDefinition{SchemaVersion: 1, ID: id, Name: id, Cost: content.BattleCost{Energy: 0}, Play: content.CardPlayDefinition{SourceZones: []string{"hand"}, Destination: "discard"}, Targeting: content.TargetingDefinition{Selector: "card_program", Minimum: 1, Maximum: 1}, Program: &content.CardProgram{Version: 1, Windows: content.CardTimingChoices[segment][timing], RollRequirement: "any", Steps: []content.CardStep{selfStep("remove_status", map[string]any{"stacks": 0})}}}
	c = content.PrepareProgramCard(c, lib)
	if err := content.ValidateCardProgram(c, *lib); err != nil {
		t.Fatal(err)
	}
	lib.Cards[id] = c
}

// pendingBleedFixture is one 4-damage attack carrying 2 pending Bleed onto a
// player who already has 1 Bleed.
func pendingBleedFixture(t *testing.T) (state.Battle, content.BattleLibrary, Engine) {
	t.Helper()
	b, lib, e := unifiedFixture(t, 4)
	application := state.SettledStatusApplication{SourceActorID: "enemy", TargetActorID: "player", StatusID: "bleed", Stacks: 2}
	b.Settled.OffensiveSources[0].StatusApplications = []state.SettledStatusApplication{application}
	b.Settled.PendingDamage.Sources[0].StatusApplications = []state.SettledStatusApplication{application}
	b.Settled.PendingDamage.Applications = []state.SettledStatusApplication{application}
	applyStatus(&b, lib, "player", "bleed", 1)
	return b, lib, e
}

func playCleanse(t *testing.T, e Engine, b *state.Battle, instance string) {
	t.Helper()
	start, ok := startActions(e, b)[instance]
	if !ok {
		t.Fatalf("%s not playable at %s", instance, b.Settled.Stage)
	}
	if _, err := e.ApplyBattleCommand(b, start); err != nil {
		t.Fatal(err)
	}
	// A single eligible status resolves immediately.
	if b.Settled.Actors["player"].CardExecution == nil {
		return
	}
	for _, a := range e.LegalActions(b, "player") {
		_, key := programPayload(a)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "target" && c.Status == "bleed" {
			if _, err := e.ApplyBattleCommand(b, a); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("bleed not offered as a target")
}

func rollAndApplyDefense(t *testing.T, e Engine, b *state.Battle) {
	t.Helper()
	applyFirst(t, e, b, command.TypePlanningAbility)
	if b.Settled.Stage == stageDefenseRoll {
		if _, err := e.ApplyBattleCommand(b, e.LegalActions(b, "player")[0]); err != nil {
			t.Fatal(err)
		}
	}
	applyFirst(t, e, b, command.TypePlanningPass)
	if b.Settled.Stage != stageDefenseSelect {
		t.Fatalf("defense did not return to the hub: %s", b.Settled.Stage)
	}
}

func passDefense(t *testing.T, e Engine, b *state.Battle) {
	t.Helper()
	for b.Segment.Current == segment.Defensive && b.Settled.PendingDamage != nil && !b.Settled.PendingDamage.Committed {
		applyFirst(t, e, b, command.TypePlanningPass)
	}
}

// A defended attack's pending statuses land when its defense resolves, still
// in Defense; an "after" cleanse then sees all 3 Bleed and nothing re-applies.
func TestDefendedAttackStatusesApplyBeforeAfterCards(t *testing.T) {
	b, lib, e := pendingBleedFixture(t)
	cleanseCard(t, &lib, "late_cleanse", "defense", "after")
	giveCardCopy(&b, "late-1", "late_cleanse")
	b.SettledCatalog, _ = json.Marshal(lib)
	if _, ok := startActions(e, &b)["late-1"]; ok {
		t.Fatal("after card offered before any defense roll")
	}
	rollAndApplyDefense(t, e, &b)
	if got := stacks(&b, "player", "bleed"); got != 3 {
		t.Fatalf("defended attack statuses not applied in Defense: %d Bleed", got)
	}
	if !b.Settled.PendingDamage.Applications[0].Applied || !b.Settled.PendingDamage.Sources[0].StatusApplications[0].Applied || !b.Settled.OffensiveSources[0].StatusApplications[0].Applied {
		t.Fatal("applied statuses still marked pending")
	}
	playCleanse(t, e, &b, "late-1")
	if got := stacks(&b, "player", "bleed"); got != 0 {
		t.Fatalf("after cleanse left %d Bleed", got)
	}
	passDefense(t, e, &b)
	if got := stacks(&b, "player", "bleed"); got != 0 {
		t.Fatalf("final commit re-applied statuses: %d Bleed", got)
	}
}

// A "before" cleanse only reaches the existing stack; the pending 2 land after.
func TestBeforeCardsCannotReachPendingStatuses(t *testing.T) {
	b, lib, e := pendingBleedFixture(t)
	cleanseCard(t, &lib, "early_cleanse", "defense", "before")
	giveCardCopy(&b, "early-1", "early_cleanse")
	b.SettledCatalog, _ = json.Marshal(lib)
	playCleanse(t, e, &b, "early-1")
	if got := stacks(&b, "player", "bleed"); got != 0 {
		t.Fatalf("before cleanse left %d Bleed", got)
	}
	rollAndApplyDefense(t, e, &b)
	passDefense(t, e, &b)
	if got := stacks(&b, "player", "bleed"); got != 2 {
		t.Fatalf("want 2 Bleed after Defense, got %d", got)
	}
}

// Passing without rolling never opens "after"; the statuses land at the end.
func TestPassingWithoutRollingSkipsAfterCards(t *testing.T) {
	b, lib, e := pendingBleedFixture(t)
	cleanseCard(t, &lib, "late_cleanse", "defense", "after")
	giveCardCopy(&b, "late-1", "late_cleanse")
	b.SettledCatalog, _ = json.Marshal(lib)
	if _, ok := startActions(e, &b)["late-1"]; ok {
		t.Fatal("after card offered without a defense roll")
	}
	passDefense(t, e, &b)
	if got := stacks(&b, "player", "bleed"); got != 3 {
		t.Fatalf("undefended statuses not applied at the end: %d Bleed", got)
	}
}

func TestCardTimingChoicesMapToWindows(t *testing.T) {
	b, _, _ := unifiedFixture(t, 4)
	for segmentName, choices := range content.CardTimingChoices {
		for timing, windows := range choices {
			if got := content.CardTiming(segmentName, windows, "any", false); got != timing {
				t.Fatalf("%s %s classified as %q", segmentName, timing, got)
			}
		}
	}
	if content.CardTiming("offense", []string{"offensive_planning"}, "any", true) != "after" || content.CardTiming("offense", []string{"offensive_planning"}, "before_first", false) != "before" {
		t.Fatal("prior-roll effects or legacy roll requirements misclassified")
	}
	after := content.CardTimingChoices["defense"]["after"]
	if programWindowOpen(&b, "player", after) {
		t.Fatal("defense after open before a roll")
	}
	b.Settled.DefenseHistory = map[string]state.SettledDefense{"a": {ActorID: "player", AbilityID: "adventurer_guard", SourceID: "a", Finalized: true}}
	if !programWindowOpen(&b, "player", after) || programWindowOpen(&b, "player", content.CardTimingChoices["defense"]["before"]) {
		t.Fatal("defense before/after did not switch at the first roll")
	}
	b.Settled.Stage = stageOffensivePlan
	b.Segment.Current = segment.Offensive
	rt := b.Settled.Actors["player"]
	rt.RollsUsed = 0
	b.Settled.Actors["player"] = rt
	if !programWindowOpen(&b, "player", content.CardTimingChoices["offense"]["before"]) || programWindowOpen(&b, "player", content.CardTimingChoices["offense"]["after"][:1]) {
		t.Fatal("offense before/after wrong before the first roll")
	}
	rt.RollsUsed = 1
	b.Settled.Actors["player"] = rt
	if programWindowOpen(&b, "player", content.CardTimingChoices["offense"]["before"]) || !programWindowOpen(&b, "player", content.CardTimingChoices["offense"]["after"]) {
		t.Fatal("offense before/after wrong after the first roll")
	}
}

// "Any time" is the Defense screen only: built-in and specialized prevention
// cards never pause a defense roll's review, then play once it applies.
func TestAnyTimePreventionWaitsOutRollReview(t *testing.T) {
	b, _, e := unifiedFixture(t, 4)
	addCardInstance(&b, "ward-1", "emergency_ward")
	a := b.Actors["player"]
	a.Cards.Hand = append(a.Cards.Hand, "ward-1")
	b.Actors["player"] = a
	wardActions := func() []command.Command {
		var out []command.Command
		for _, action := range e.LegalActions(&b, "player") {
			var p command.CommitInteractionPayload
			if json.Unmarshal(action.Payload, &p) == nil && len(p.Commitment.CardIDs) == 1 && p.Commitment.CardIDs[0] == "ward-1" {
				out = append(out, action)
			}
		}
		return out
	}
	applyFirst(t, e, &b, command.TypePlanningAbility)
	if b.Settled.Stage == stageDefenseRoll {
		if _, err := e.ApplyBattleCommand(&b, e.LegalActions(&b, "player")[0]); err != nil {
			t.Fatal(err)
		}
	}
	if b.Settled.Stage != stageDefenseReact {
		t.Fatalf("missing roll review: %s", b.Settled.Stage)
	}
	if len(wardActions()) != 0 {
		t.Fatal("Emergency Ward offered during the roll review")
	}
	applyFirst(t, e, &b, command.TypePlanningPass)
	if b.Settled.Stage != stageDefenseSelect {
		t.Fatalf("defense did not return to the hub: %s", b.Settled.Stage)
	}
	if amount := settledSourceAmount(b.Settled.PendingDamage.Sources[0]); amount > 0 && len(wardActions()) == 0 {
		t.Fatal("Emergency Ward unavailable on the Defense screen after the roll")
	}

	for _, id := range []string{"coagulate", "emergency_molt", "antivenom_draught", "spiteful_ward"} {
		for _, stage := range []string{stageDefenseReact, stageDefenseSelect} {
			b, lib := curseFixture(t)
			c := configuredClone(t, &lib, id)
			putMechanic(&b, c)
			applyStatus(&b, lib, "enemy", "poison", 2)
			applyStatus(&b, lib, "player", "catalyst", 2)
			b.Segment.Current = segment.Defensive
			b.Settled.UnifiedDefense = true
			b.Settled.Stage = stage
			source := state.SettledDamageSource{ID: "incoming", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "sword_cut", BaseAmount: 5, FinalAmount: 5}
			b.Settled.OffensiveSources = []state.SettledDamageSource{source}
			b.Settled.PendingDamage = &state.SettledDamageBatch{ID: "damage", Sources: []state.SettledDamageSource{source}}
			var choices int
			if lib.Cards[c.ID].AccessType == "curse" {
				choices = len(curseCardChoices(&b, lib, "player", c))
			} else {
				choices = len(venomCardChoices(&b, lib, "player", c))
			}
			if (choices > 0) != (stage == stageDefenseSelect) {
				t.Fatalf("%s playable=%v at %s", id, choices > 0, stage)
			}
		}
	}
}

// The offensive reaction is opt-in: turn cards wait it out, reaction cards
// (enemy dice) play there, and the rules text states each timing.
func TestOffenseReactionIsOptIn(t *testing.T) {
	b, lib := adventurerFixture(t)
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 6})
	b.Settled.Stage = stageOffensiveReact
	openSettledWindow(&b, "react", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	for _, id := range []string{"take_stock-0", "second_wind-0", "nudge-0", "try_again-0"} {
		if programOffers(&b, lib, id) {
			t.Fatalf("%s paused the offensive reaction", id)
		}
	}
	for windows, want := range map[string]string{
		"offensive_planning":                      "Play: Offense, any time.",
		"offensive_after_roll,offensive_reaction": "Play: Offense, after your first roll, or as a reaction to revealed attack dice.",
		"offensive_reaction":                      "Play: Offense, only as a reaction to revealed attack dice.",
		"defense_selection,defense_reaction":      "Play: Defense, any time, or while a defense roll's result is showing.",
	} {
		if got := content.CardTimingRules(strings.Split(windows, ","), "any", false); got != want {
			t.Fatalf("%s: %q want %q", windows, got, want)
		}
	}
}
