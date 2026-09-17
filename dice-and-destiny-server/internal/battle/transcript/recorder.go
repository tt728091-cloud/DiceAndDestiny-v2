package transcript

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func (r *Recorder) Begin(cmd command.Command, context CommandContext, battleContext BattleContext, battle *state.Battle) {
	if r == nil || cmd.Type == command.TypeOpenBattle {
		return
	}
	context = normalizedCommandContext(context, battleContext, battle, cmd.ActorID)
	visibility := commandVisibility(cmd, battleContext, battle)
	details := map[string]any{
		"command_type": string(cmd.Type),
		"payload":      decodeJSON(cmd.Payload),
	}
	if context.CandidateCount > 0 {
		details["action_index"] = context.ActionIndex
		details["candidate_count"] = context.CandidateCount
	}
	if context.InferenceMS > 0 {
		details["inference_ms"] = context.InferenceMS
	}
	mergeMap(details, context.Metadata)
	draftRecord := baseDraft(cmd.BattleID, battleContext, battle)
	draftRecord.Kind = "command_submitted"
	draftRecord.ActorID = cmd.ActorID
	draftRecord.Controller = context.Controller
	draftRecord.Visibility = visibility
	draftRecord.Summary = fmt.Sprintf("%s submitted %s", actorName(battle, cmd.ActorID), humanize(string(cmd.Type)))
	draftRecord.Details = details
	written := r.appendDrafts(0, []draft{{Record: draftRecord, privateKey: commandPrivateKey(cmd, battle)}})
	if len(written) == 1 {
		r.mu.Lock()
		r.activeCauses[cmd.BattleID] = written[0].Sequence
		r.mu.Unlock()
	}
}

func (r *Recorder) Accepted(transition Transition) {
	if r == nil || transition.Command.Type == command.TypeOpenBattle || transition.After == nil {
		return
	}
	cause := r.takeCause(transition.Command.BattleID)
	context := normalizedCommandContext(transition.Context, transition.Battle, transition.After, transition.Command.ActorID)
	commandState := transition.Before
	if commandState == nil {
		commandState = transition.After
	}
	eventContext := baseDraft(transition.Command.BattleID, transition.Battle, commandState)
	if transition.Before == nil {
		// A newly started battle may already be at Planning in After. Its
		// lifecycle records still begin at the first actual segment event.
		for _, ev := range transition.Events {
			if ev.Segment != "" || (ev.Type == event.TypeSegmentAdvanced && ev.To != "") {
				eventContext = advanceEventContext(eventContext, ev)
				break
			}
		}
	}
	accepted := eventContext
	accepted.Kind = "command_accepted"
	accepted.ActorID = transition.Command.ActorID
	accepted.Controller = context.Controller
	accepted.Visibility = VisibilityDebugSystem
	accepted.Summary = fmt.Sprintf("Authority accepted %s for %s", humanize(string(transition.Command.Type)), actorName(transition.After, transition.Command.ActorID))
	accepted.Details = map[string]any{"command_type": string(transition.Command.Type)}

	drafts := []draft{{Record: accepted}}
	if transition.Command.Type == command.TypeStartBattle {
		started := eventContext
		started.Kind = "battle_started"
		started.ActorID = transition.Command.ActorID
		started.Controller = context.Controller
		started.Visibility = VisibilityPublic
		started.Summary = battleStartedSummary(transition.After)
		started.Details = battleLifecycleDetails(transition.After, transition.Battle)
		drafts = append(drafts, draft{Record: started})
	}

	library := settledLibrary(transition.After)
	for _, authorityEvent := range transition.Events {
		eventContext = advanceEventContext(eventContext, authorityEvent)
		drafts = append(drafts, eventDraftsAt(authorityEvent, transition, library, eventContext)...)
	}
	drafts = append(drafts, defenseRevealDrafts(transition, library)...)
	drafts = append(drafts, automaticPlanningDrafts(transition, library)...)
	drafts = append(drafts, mutationDrafts(transition, library)...)
	drafts = append(drafts, pendingInputDrafts(transition.After, transition.Battle)...)
	if state.IsTerminalBattleStatus(transition.After.Status) && !containsKind(drafts, "battle_completed") {
		completed := baseDraft(transition.Command.BattleID, transition.Battle, transition.After)
		completed.Kind = "battle_completed"
		completed.Visibility = VisibilityPublic
		completed.Summary = battleCompletedSummary(transition.After)
		completed.Details = battleLifecycleDetails(transition.After, transition.Battle)
		drafts = append(drafts, draft{Record: completed})
	}
	r.appendDrafts(cause, dedupePrivateDrafts(drafts))
}

// A single command can resolve several segments before returning its snapshot.
// Unstamped events belong to the current position in that ordered event stream,
// never automatically to the final snapshot's segment or reaction window.
func advanceEventContext(current Record, ev event.Event) Record {
	nextSegment := string(ev.Segment)
	if ev.Type == event.TypeSegmentAdvanced && ev.To != "" {
		nextSegment = string(ev.To)
	}
	if (nextSegment != "" && nextSegment != current.Segment) || (ev.Round != 0 && ev.Round != current.Round) {
		current.Stage = ""
		current.WindowID = ""
		current.PendingInputID = ""
	}
	if nextSegment != "" {
		current.Segment = nextSegment
	}
	if ev.Round != 0 {
		current.Round = ev.Round
	}
	if ev.WindowID != "" {
		current.WindowID = ev.WindowID
	}
	if ev.PendingInputID != "" {
		current.PendingInputID = ev.PendingInputID
	}
	return current
}

func (r *Recorder) Rejected(transition Transition) {
	if r == nil || transition.Command.Type == command.TypeOpenBattle {
		return
	}
	cause := r.takeCause(transition.Command.BattleID)
	battle := transition.Before
	context := normalizedCommandContext(transition.Context, transition.Battle, battle, transition.Command.ActorID)
	rejected := baseDraft(transition.Command.BattleID, transition.Battle, battle)
	rejected.Kind = rejectionKind(transition.Error)
	rejected.ActorID = transition.Command.ActorID
	rejected.Controller = context.Controller
	rejected.Visibility = VisibilityDebugSystem
	rejected.Summary = fmt.Sprintf("Authority rejected %s for %s: %s", humanize(string(transition.Command.Type)), actorName(battle, transition.Command.ActorID), transition.Error)
	rejected.Details = map[string]any{
		"command_type": string(transition.Command.Type),
		"payload":      decodeJSON(transition.Command.Payload),
		"error":        transition.Error,
	}
	r.appendDrafts(cause, []draft{{Record: rejected}})
}

func (r *Recorder) TransportRejected(raw string, err error) {
	if r == nil {
		return
	}
	record := Record{
		Kind:       "command_rejected_invalid_envelope",
		Visibility: VisibilityDebugSystem,
		Summary:    "Authority rejected an invalid command envelope",
		Details:    map[string]any{"error": err.Error(), "raw_length": len(raw)},
	}
	r.appendDrafts(0, []draft{{Record: record}})
}

func (r *Recorder) Diagnostic(value Diagnostic) {
	if r == nil {
		return
	}
	record := baseDraft(value.BattleID, value.Context, value.Battle)
	record.Kind = value.Kind
	record.ActorID = value.ActorID
	record.Controller = value.Controller
	record.Visibility = VisibilityDebugSystem
	record.Summary = value.Summary
	record.Details = cloneMap(value.Details)
	r.appendDrafts(0, []draft{{Record: record}})
}

func (r *Recorder) takeCause(battleID string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	cause := r.activeCauses[battleID]
	delete(r.activeCauses, battleID)
	return cause
}

func baseDraft(battleID string, context BattleContext, battle *state.Battle) Record {
	record := Record{
		BattleID:     battleID,
		Mode:         context.Mode,
		HumanActorID: context.HumanActorID,
		ModelActorID: context.ModelActorID,
	}
	if battle == nil {
		return record
	}
	record.BattleID = battle.ID
	record.Seed = battle.Random.Seed
	record.Round = battle.Segment.Round
	record.Segment = string(battle.Segment.Current)
	record.Stage = battle.Flow.Stage
	if battle.Settled != nil {
		if battle.Settled.Stage != "" {
			record.Stage = battle.Settled.Stage
		}
		if battle.Settled.Window != nil {
			record.WindowID = battle.Settled.Window.ID
			record.PendingInputID = battle.Settled.Window.PendingInputID
		}
	}
	if record.Mode == "" {
		record.Mode = inferMode(battle)
	}
	if record.HumanActorID == "" {
		record.HumanActorID = firstActorWithController(battle, state.ControllerHuman)
	}
	return record
}

func normalizedCommandContext(value CommandContext, battleContext BattleContext, battle *state.Battle, actorID string) CommandContext {
	if value.Controller == "" {
		value.Controller = controllerForActor(battleContext, battle, actorID)
	}
	return value
}

func controllerForActor(context BattleContext, battle *state.Battle, actorID string) string {
	if configured := context.Controllers[actorID]; configured != "" {
		return configured
	}
	if actorID == context.HumanActorID && actorID != "" {
		return "human"
	}
	if actorID == context.ModelActorID && actorID != "" {
		return "learned_policy"
	}
	if battle != nil {
		switch battle.Actors[actorID].Controller {
		case state.ControllerHuman:
			return "human"
		case state.ControllerAI:
			return "d100"
		case state.ControllerExternal:
			return "external"
		case state.ControllerSystem:
			return "authority"
		}
	}
	return "authority"
}

func commandVisibility(cmd command.Command, context BattleContext, battle *state.Battle) string {
	if cmd.Type == command.TypeStartBattle {
		return VisibilityDebugSystem
	}
	return privateVisibility(cmd.ActorID, context, battle)
}

func commandPrivateKey(cmd command.Command, battle *state.Battle) string {
	stage := ""
	segmentID := ""
	if battle != nil {
		stage = battle.Flow.Stage
		segmentID = string(battle.Segment.Current)
		if battle.Settled != nil && battle.Settled.Stage != "" {
			stage = battle.Settled.Stage
		}
	}
	switch cmd.Type {
	case command.TypePlanningAbility:
		if stage == "defense_selection" {
			return "defense_choice:" + cmd.ActorID
		}
		return "ability:" + cmd.ActorID
	case command.TypePlanningRoll, command.TypePlanningReroll:
		return diePrivateKey(cmd.ActorID, "offensive", "")
	case command.TypeRollDice:
		if segmentID == "defensive" {
			return "defense_die:" + cmd.ActorID
		}
	case command.TypeCommitInteraction, command.TypePlanningCards:
		payload := jsonMap(decodeJSON(cmd.Payload))
		commitment := mapValue(payload, "commitment")
		if len(commitment) == 0 {
			commitment = payload
		}
		for _, cardID := range stringSlice(commitment["card_ids"]) {
			return "card:" + cmd.ActorID + ":" + cardID
		}
	}
	return ""
}

func privateVisibility(actorID string, context BattleContext, battle *state.Battle) string {
	human := context.HumanActorID
	if human == "" {
		human = firstActorWithController(battle, state.ControllerHuman)
		if human == "" && battle != nil {
			ids := sortedActorIDs(battle)
			if len(ids) > 0 {
				human = ids[0]
			}
		}
	}
	if actorID != "" && actorID == human {
		return VisibilityPrivateSelf
	}
	return VisibilityPrivateOpponent
}

func inferMode(battle *state.Battle) string {
	if battle == nil {
		return ""
	}
	if battle.Origin.Kind == state.BattleOriginScenario {
		return "scenario"
	}
	for _, actor := range battle.Actors {
		if actor.Controller == state.ControllerExternal {
			return "learned_mirror"
		}
	}
	return "d100"
}

func firstActorWithController(battle *state.Battle, controller state.ControllerType) string {
	for _, actorID := range sortedActorIDs(battle) {
		if battle.Actors[actorID].Controller == controller {
			return actorID
		}
	}
	return ""
}

func sortedActorIDs(battle *state.Battle) []string {
	if battle == nil {
		return nil
	}
	ids := make([]string, 0, len(battle.Actors))
	for actorID := range battle.Actors {
		ids = append(ids, actorID)
	}
	sort.Strings(ids)
	return ids
}

func actorName(battle *state.Battle, actorID string) string {
	if battle != nil {
		if name := strings.TrimSpace(battle.Actors[actorID].Character.Name); name != "" {
			return name
		}
	}
	if actorID == "" {
		return "Authority"
	}
	return humanize(actorID)
}

func settledLibrary(battle *state.Battle) content.BattleLibrary {
	var library content.BattleLibrary
	if battle != nil && len(battle.SettledCatalog) > 0 {
		_ = json.Unmarshal(battle.SettledCatalog, &library)
	}
	return library
}

func contentName(library content.BattleLibrary, kind, id string) string {
	switch kind {
	case "card":
		if definition, ok := library.Cards[id]; ok && definition.Name != "" {
			return definition.Name
		}
	case "ability":
		if definition, ok := library.Abilities[id]; ok && definition.Name != "" {
			return definition.Name
		}
	case "status":
		if definition, ok := library.Statuses[id]; ok && definition.Name != "" {
			return definition.Name
		}
	}
	return humanize(id)
}

func humanize(value string) string {
	if value == "" {
		return "Unknown"
	}
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(value))
	for index := range words {
		words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
	}
	return strings.Join(words, " ")
}

func decodeJSON(payload []byte) any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return string(payload)
	}
	return value
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func mergeMap(target map[string]any, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}

func rejectionKind(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "stale"):
		return "command_rejected_stale"
	case strings.Contains(lower, "does not own"), strings.Contains(lower, "wrong seat"):
		return "command_rejected_wrong_seat"
	case strings.Contains(lower, "fabricated"), strings.Contains(lower, "does not exist"):
		return "command_rejected_fabricated"
	case strings.Contains(lower, "timeout"):
		return "command_rejected_timeout"
	default:
		return "command_rejected"
	}
}

func containsKind(drafts []draft, kind string) bool {
	for _, item := range drafts {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func dedupePrivateDrafts(values []draft) []draft {
	seen := map[string]bool{}
	result := make([]draft, 0, len(values))
	for _, value := range values {
		if value.privateKey != "" {
			key := value.BattleID + "|" + value.privateKey
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		result = append(result, value)
	}
	return result
}

func battleStartedSummary(battle *state.Battle) string {
	names := make([]string, 0, len(battle.Actors))
	for _, actorID := range sortedActorIDs(battle) {
		names = append(names, actorName(battle, actorID))
	}
	return fmt.Sprintf("Battle started: %s", strings.Join(names, " vs "))
}

func battleCompletedSummary(battle *state.Battle) string {
	if battle.WinnerActorID != "" {
		return fmt.Sprintf("Battle completed: %s won", actorName(battle, battle.WinnerActorID))
	}
	return fmt.Sprintf("Battle completed: %s", humanize(string(battle.Status)))
}

func battleLifecycleDetails(battle *state.Battle, context BattleContext) map[string]any {
	actors := map[string]any{}
	for _, actorID := range sortedActorIDs(battle) {
		actor := battle.Actors[actorID]
		actors[actorID] = map[string]any{
			"name":          actorName(battle, actorID),
			"definition_id": actor.DefinitionID,
			"controller":    controllerForActor(context, battle, actorID),
		}
	}
	return map[string]any{
		"status":          battle.Status,
		"winner_actor_id": battle.WinnerActorID,
		"actors":          actors,
		"random":          battle.Random,
		"origin":          battle.Origin,
	}
}

// Keep the event import explicit here so record.go remains the single schema
// definition while translation helpers can use the typed event family.
var _ event.Type
