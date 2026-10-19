package quest

import (
	"fmt"
	"strings"

	"diceanddestiny/server/internal/battle/expedition"
)

// The describer turns a loaded catalog into display text and a graph per
// quest. The client never interprets verbs: it shows these strings, the
// engine-status badges and the graph's nodes and edges as given. Every list
// is a list, never null.

// Line is one described verb use with its engine status.
type Line struct {
	Text       string `json:"text"`
	Verb       string `json:"verb"`
	Engine     string `json:"engine"`
	EngineText string `json:"engine_text"`
}

// TimerView describes a stage's timer: the chip ("Urgent · 4 rounds") and
// the full text.
type TimerView struct {
	Type       string `json:"type"`
	Chip       string `json:"chip"`
	Text       string `json:"text"`
	Rounds     int    `json:"rounds,omitempty"`
	Engine     string `json:"engine"`
	EngineText string `json:"engine_text"`
}

// NodeView is a stage or an end of the quest graph. Depth is its column:
// the longest path from the start (ends sit one column after their stage).
type NodeView struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // stage, end
	Outcome   string    `json:"outcome,omitempty"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	Objective Line      `json:"objective"`
	Timer     TimerView `json:"timer"`
	Depth     int       `json:"depth"`
	Start     bool      `json:"start,omitempty"`
	Badges    []Line    `json:"badges"`
}

// EdgeView is one outcome: from its stage to the next stage or an end.
type EdgeView struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	To           string `json:"to"`
	Outcome      string `json:"outcome"`
	Label        string `json:"label"`
	When         Line   `json:"when"`
	Text         string `json:"text"`
	Consequences []Line `json:"consequences"`
}

// RoleView is a declared template role.
type RoleView struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// QuestView is one quest's header and graph.
type QuestView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Set        string     `json:"set"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	StatusText string     `json:"status_text"`
	Region     string     `json:"region,omitempty"`
	RegionName string     `json:"region_name,omitempty"`
	File       string     `json:"file"`
	Roles      []RoleView `json:"roles"`
	Hook       Line       `json:"hook"`
	Trigger    []Line     `json:"trigger"`
	Start      string     `json:"start"`
	Nodes      []NodeView `json:"nodes"`
	Edges      []EdgeView `json:"edges"`
}

// SetView is one quest set and its quests in file order.
type SetView struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Text   string   `json:"text"`
	Quests []string `json:"quests"`
}

// CatalogView is what the quest_catalog op returns.
type CatalogView struct {
	Sets       []SetView         `json:"sets"`
	Quests     []QuestView       `json:"quests"`
	EngineText map[string]string `json:"engine_text"`
}

var statusText = map[string]string{
	StatusScenario: "Scenario: describes rules the Go code runs today",
	StatusPaper:    "Paper: design only; nothing runs it and its numbers are placeholders",
}

var (
	endTitles     = map[string]string{"success": "Complete", "failure": "Failed", "abandoned": "Abandoned"}
	outcomeLabels = map[string]string{"success": "Success", "failure": "Failure", "abandoned": "Abandoned"}
)

type describer struct {
	cfg      expedition.Config
	pack     expedition.Pack
	quests   map[string]Quest
	statuses map[string]string
	values   map[string]any
	quest    Quest
}

// Describe builds the catalog's display view. statusNames maps status ids to
// their names (from the battle library); missing names show the id.
func Describe(cat Catalog, cfg expedition.Config, pack expedition.Pack, statusNames map[string]string) (CatalogView, error) {
	values, err := configValues(cfg)
	if err != nil {
		return CatalogView{}, err
	}
	view := CatalogView{Sets: []SetView{}, Quests: []QuestView{}, EngineText: map[string]string{}}
	for k, v := range EngineText {
		view.EngineText[k] = v
	}
	for _, set := range cat.Sets {
		view.Sets = append(view.Sets, SetView{ID: set.ID, Name: set.Name, Text: set.Text, Quests: append([]string{}, set.Quests...)})
	}
	for _, q := range cat.Ordered() {
		d := describer{cfg: cfg, pack: pack, quests: cat.Quests, statuses: statusNames, values: values, quest: q}
		view.Quests = append(view.Quests, d.questView())
	}
	return view, nil
}

// line describes one verb use; part is where it stands.
func (d describer) line(_ Part, call Call, text string) Line {
	verb := Vocabulary[call.Verb()]
	return Line{Text: text, Verb: verb.Name, Engine: verb.Engine, EngineText: EngineText[verb.Engine]}
}

func (d describer) questView() QuestView {
	q := d.quest
	v := QuestView{ID: q.ID, Name: q.Name, Set: q.Set, Kind: q.Kind, Status: q.Status, StatusText: statusText[q.Status], Region: q.Region, File: q.File,
		Roles: []RoleView{}, Trigger: []Line{}, Start: q.Start, Nodes: []NodeView{}, Edges: []EdgeView{}}
	if q.Region != "" {
		v.RegionName = d.pack.Regions[q.Region].Name
	}
	for _, name := range sortedKeys(q.Roles) {
		v.Roles = append(v.Roles, RoleView{Name: name, Type: q.Roles[name].Type, Text: q.Roles[name].Text})
	}
	v.Hook = d.line(PartHook, q.Hook, d.hookText(q.Hook))
	for _, t := range q.Trigger {
		v.Trigger = append(v.Trigger, d.line(PartTrigger, t, d.triggerText(t)))
	}
	depth := d.depths()
	for _, s := range q.Stages {
		objective := d.line(PartObjective, s.Objective, d.objectiveText(s.Objective))
		node := NodeView{ID: s.ID, Kind: "stage", Title: s.Title, Text: s.Text, Objective: objective, Timer: d.timerView(s.Timer), Depth: depth[s.ID], Start: s.ID == q.Start, Badges: []Line{objective}}
		if node.Title == "" {
			node.Title = objective.Text
		}
		node.Badges = append(node.Badges, Line{Text: node.Timer.Chip, Verb: "timer:" + node.Timer.Type, Engine: node.Timer.Engine, EngineText: node.Timer.EngineText})
		v.Nodes = append(v.Nodes, node)
	}
	for _, s := range q.Stages {
		for i, o := range s.Outcomes {
			edge := EdgeView{ID: fmt.Sprintf("%s#%d", s.ID, i), From: s.ID, To: o.Next, Outcome: o.Outcome, Text: o.Text, Consequences: []Line{}}
			edge.When = d.whenLine(o)
			switch {
			case o.Label != "":
				edge.Label = o.Label
			case o.When.Verb() == "event_choice":
				edge.Label = edge.When.Text
			default:
				edge.Label = outcomeLabels[o.Outcome]
			}
			for _, c := range o.Consequences {
				edge.Consequences = append(edge.Consequences, d.line(PartConsequence, c, d.consequenceText(c)))
				if edge.Text == "" && c.Verb() == "event_outcome" {
					edge.Text = d.eventOutcomeStory(c)
				}
			}
			if o.Next == End {
				// Each ending is its own node, one column after its stage, so
				// branches stack instead of converging on one end.
				end := NodeView{ID: fmt.Sprintf("end:%s#%d", s.ID, i), Kind: "end", Outcome: o.Outcome, Title: endTitles[o.Outcome], Text: "", Depth: depth[s.ID] + 1,
					Objective: Line{}, Timer: TimerView{}, Badges: []Line{}}
				edge.To = end.ID
				v.Nodes = append(v.Nodes, end)
			}
			v.Edges = append(v.Edges, edge)
		}
	}
	return v
}

// depths is each stage's column: the longest path from the start.
func (d describer) depths() map[string]int {
	depth := map[string]int{d.quest.Start: 0}
	order := []string{}
	seen := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		s, _ := d.quest.Stage(id)
		for _, o := range s.Outcomes {
			if o.Next != End {
				visit(o.Next)
			}
		}
		order = append(order, id)
	}
	visit(d.quest.Start)
	for i := len(order) - 1; i >= 0; i-- {
		s, _ := d.quest.Stage(order[i])
		for _, o := range s.Outcomes {
			if o.Next != End && depth[o.Next] < depth[s.ID]+1 {
				depth[o.Next] = depth[s.ID] + 1
			}
		}
	}
	return depth
}

// --- names ------------------------------------------------------------------

// entity names a content id or a role ("{captor}").
func (d describer) entity(v any) string {
	if role, ok := singleKey(v, "role"); ok {
		return "{" + role + "}"
	}
	id, _ := v.(string)
	if g, ok := d.pack.Groups[id]; ok {
		return theName(g.Name)
	}
	if s, ok := d.pack.Sites[id]; ok {
		return s.Name
	}
	return id
}

// theName puts "the" before a group's name unless it has one.
func theName(name string) string {
	if strings.HasPrefix(name, "The ") {
		return "the " + strings.TrimPrefix(name, "The ")
	}
	return "the " + name
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (d describer) item(v any) string {
	id, _ := v.(string)
	if g, ok := d.pack.Gear[id]; ok {
		return "a " + g.Name
	}
	return strings.ReplaceAll(id, "_", " ") + " (a placeholder item)"
}

func (d describer) date(id string) string {
	if s, ok := d.cfg.Calendar.Season(id); ok {
		return s.Name
	}
	if i, ok := d.cfg.Calendar.MonthIndex(id); ok {
		return d.cfg.Calendar.Months[i].Name
	}
	switch id {
	case "next_month":
		return "the next month"
	case "next_season":
		return "the next season"
	}
	return id
}

func (d describer) dates(v any) string {
	list, _ := v.([]any)
	names := []string{}
	for _, item := range list {
		id, _ := item.(string)
		names = append(names, d.date(id))
	}
	return strings.Join(names, " or ")
}

// number shows a literal number, or a config value with its key.
func (d describer) number(v any) (int, string) {
	if n, ok := asInt(v); ok {
		return n, ""
	}
	key, _ := singleKey(v, "config")
	value, _ := lookupConfig(d.values, key)
	n, _ := asInt(value)
	return n, key
}

func (d describer) numberText(v any) string {
	n, key := d.number(v)
	if key != "" {
		return fmt.Sprintf("%d (%s)", n, key)
	}
	return fmt.Sprintf("%d", n)
}

func signed(n int) string {
	if n < 0 {
		return fmt.Sprintf("−%d", -n)
	}
	return fmt.Sprintf("+%d", n)
}

func (d describer) questName(v any) string {
	id, _ := v.(string)
	if q, ok := d.quests[id]; ok {
		return q.Name
	}
	return id
}

func (d describer) choice(event, choice string) expedition.Choice {
	for _, c := range d.pack.Events[event].Choices {
		if c.ID == choice {
			return c
		}
	}
	return expedition.Choice{}
}

// --- parts --------------------------------------------------------------------

func (d describer) hookText(c Call) string {
	switch c.Verb() {
	case "scenario_start":
		region, _ := c["region"].(string)
		if r, ok := d.pack.Regions[region]; ok {
			return fmt.Sprintf("From the start of %s: %s", r.Name, r.Description)
		}
		return "From the start of the scenario"
	case "rumour":
		return "A rumour: " + fmt.Sprint(c["text"])
	case "request":
		text := "A request from " + d.entity(c["from"])
		if t, ok := c["text"].(string); ok {
			text += ": " + t
		}
		return text
	case "event":
		id, _ := c["event"].(string)
		ev := d.pack.Events[id]
		return fmt.Sprintf("The hero comes upon %s: %s", ev.Name, ev.Text)
	case "raid":
		return "A raid on " + d.entity(c["town"])
	case "quest_failed":
		return d.questName(c["quest"]) + " has failed"
	}
	return c.Verb()
}

func (d describer) triggerText(c Call) string {
	switch c.Verb() {
	case "scenario_start":
		region, _ := c["region"].(string)
		if r, ok := d.pack.Regions[region]; ok {
			return r.Name + " begin"
		}
		return "The scenario begins"
	case "season", "month":
		return "In " + d.dates(c["in"])
	case "reputation_tier":
		tier, _ := c["at_least"].(string)
		return fmt.Sprintf("%s regards the hero as at least %s", capitalize(d.entity(c["with"])), capitalize(tier))
	case "prosperity":
		return fmt.Sprintf("%s has at least %s prosperity", capitalize(d.entity(c["town"])), d.numberText(c["at_least"]))
	case "site_state":
		return fmt.Sprintf("%s is %s", capitalize(d.entity(c["site"])), c["state"])
	case "quest_outcome":
		return d.questOutcomeText(c)
	}
	return c.Verb()
}

func (d describer) questOutcomeText(c Call) string {
	if outcome, ok := c["outcome"].(string); ok {
		return fmt.Sprintf("%s ends in %s", d.questName(c["quest"]), outcome)
	}
	return d.questName(c["quest"]) + " ends"
}

func (d describer) objectiveText(c Call) string {
	switch c.Verb() {
	case "defeat_group":
		text := "Defeat " + d.entity(c["group"])
		if at, ok := c["at"]; ok {
			text += " at " + d.entity(at)
		}
		if id, ok := c["group"].(string); ok {
			switch g := d.pack.Groups[id]; {
			case g.Warlord:
				text += " (the Warlord)"
			case g.Lieutenant:
				text += " (a Lieutenant)"
			}
		}
		return text
	case "liberate_site":
		text := "Liberate " + d.entity(c["site"])
		if id, ok := c["site"].(string); ok {
			s := d.pack.Sites[id]
			holder := s.OccupiedBy
			if holder == "" {
				holder = s.Garrison
			}
			text += " from " + d.entity(holder)
		}
		return text
	case "reach_site":
		return "Reach " + d.entity(c["site"])
	case "resolve_event":
		return "Resolve the scene at " + d.entity(c["site"])
	case "deliver_item":
		return fmt.Sprintf("Deliver %s to %s", d.item(c["item"]), d.entity(c["to"]))
	case "be_at_by":
		by, _ := c["by"].(string)
		return fmt.Sprintf("Be at %s by %s", d.entity(c["site"]), d.date(by))
	case "spend_influence":
		text := "Spend " + d.numberText(c["amount"]) + " Influence"
		if at, ok := c["at"]; ok {
			text += " at " + d.entity(at)
		}
		return text
	case "survey_tiles":
		return "Explore " + d.numberText(c["count"]) + " tiles"
	case "clear_dungeon":
		return "Clear " + d.entity(c["site"])
	}
	return c.Verb()
}

func (d describer) timerView(t *Timer) TimerView {
	if t == nil || t.Type == "none" {
		return TimerView{Type: "none", Chip: "No timer", Text: "No timer: it waits.", Engine: TimerEngine["none"], EngineText: EngineText[TimerEngine["none"]]}
	}
	v := TimerView{Type: t.Type, Engine: TimerEngine[t.Type], EngineText: EngineText[TimerEngine[t.Type]]}
	switch t.Type {
	case "urgent":
		n, key := d.number(t.Rounds)
		v.Rounds = n
		v.Chip = fmt.Sprintf("Urgent · %d rounds", n)
		from := "the quest's start"
		if t.From == "stage_start" {
			from = "the stage's start"
		}
		v.Text = fmt.Sprintf("Urgent: %d rounds from %s", n, from)
		if key != "" {
			v.Text = fmt.Sprintf("Urgent: %d rounds (%s) from %s", n, key, from)
		}
	case "seasonal":
		name := d.date(t.Before)
		v.Chip = "Before " + name
		v.Text = "Seasonal: before " + name + " comes"
	}
	return v
}

func (d describer) whenLine(o Outcome) Line {
	if o.When == nil {
		if o.Outcome == "failure" {
			verb := Vocabulary["timer_expired"]
			return Line{Text: "The timer runs out", Verb: verb.Name, Engine: verb.Engine, EngineText: EngineText[verb.Engine]}
		}
		return Line{Text: "The objective is met", Verb: "objective_met", Engine: EngineScenario, EngineText: EngineText[EngineScenario]}
	}
	c := o.When
	text := c.Verb()
	switch c.Verb() {
	case "timer_expired":
		text = "The timer runs out"
	case "event_choice":
		event, _ := c["event"].(string)
		choice, _ := c["choice"].(string)
		text = d.choice(event, choice).Text
	case "lose_battle":
		text = "Lose to " + d.entity(c["group"])
	case "defeat_group":
		text = d.objectiveText(c)
	case "quest_outcome":
		text = d.questOutcomeText(c)
	}
	return d.line(PartWhen, c, text)
}

func (d describer) consequenceText(c Call) string {
	switch c.Verb() {
	case "xp":
		return signed(mustNumber(d, c["amount"])) + " XP" + d.keyNote(c["amount"])
	case "gold":
		return signed(mustNumber(d, c["amount"])) + " gold" + d.keyNote(c["amount"])
	case "gear":
		return "Gain " + d.item(c["item"])
	case "reputation":
		return fmt.Sprintf("%s reputation with %s%s", signed(mustNumber(d, c["amount"])), d.entity(c["with"]), d.keyNote(c["amount"]))
	case "prosperity":
		return fmt.Sprintf("%s prosperity for %s%s", signed(mustNumber(d, c["amount"])), d.entity(c["town"]), d.keyNote(c["amount"]))
	case "town_condition":
		until, _ := c["until"].(string)
		return fmt.Sprintf("%s: %s, until %s", capitalize(d.entity(c["town"])), c["text"], d.date(until))
	case "remove_enemy_status":
		key, _ := singleKey(c["status"], "config")
		value, _ := lookupConfig(d.values, key)
		id, _ := value.(string)
		name := id
		if n, ok := d.statuses[id]; ok && n != "" {
			name = n
		}
		stacks, _ := d.number(c["stacks"])
		plural := "s"
		if stacks == 1 {
			plural = ""
		}
		return fmt.Sprintf("Remove %d stack%s of %s (%s) from %s", stacks, plural, name, key, d.entity(c["group"]))
	case "site_state":
		return fmt.Sprintf("%s is %s", capitalize(d.entity(c["site"])), c["state"])
	case "spawn":
		text := capitalize(d.entity(c["group"])) + " appears"
		if at, ok := c["at"]; ok {
			text += " near " + d.entity(at)
		}
		if from, ok := c["from"]; ok {
			text += ", from " + d.entity(from)
		}
		return text
	case "start_quest":
		return "Start " + d.questName(c["quest"])
	case "open_world":
		return "The world opens"
	case "end_run":
		return "The run is over"
	case "config_reward":
		key, _ := c["key"].(string)
		value, _ := lookupConfig(d.values, key)
		n, _ := asInt(value)
		what := fmt.Sprintf("%s XP", signed(n))
		if c["reward"] == "relic_choice" {
			what = fmt.Sprintf("%s extra relic choice", signed(n))
			if n != 1 {
				what += "s"
			}
		}
		if c["per"] == "unused_round" {
			return fmt.Sprintf("%s per unused round (%s)", what, key)
		}
		return fmt.Sprintf("%s if any round is unused (%s)", what, key)
	case "event_outcome":
		return d.eventOutcomeText(c)
	case "mend_wounds":
		if c["amount"] == "all" {
			return "Mend every wound"
		}
		return fmt.Sprintf("Mend %s damage", d.numberText(c["amount"]))
	case "win_region":
		region, _ := c["region"].(string)
		if r, ok := d.pack.Regions[region]; ok {
			return "Win the region: " + r.Name
		}
		return "Win the region"
	}
	return c.Verb()
}

func mustNumber(d describer, v any) int {
	n, _ := d.number(v)
	return n
}

func (d describer) keyNote(v any) string {
	if _, key := d.number(v); key != "" {
		return " (" + key + ")"
	}
	return ""
}

// effects lists what one event outcome does, from the event file.
func (d describer) effects(o expedition.Outcome) string {
	parts := []string{}
	if o.XP > 0 {
		parts = append(parts, fmt.Sprintf("+%d XP", o.XP))
	}
	if o.Gold > 0 {
		parts = append(parts, fmt.Sprintf("+%d gold", o.Gold))
	}
	if o.Mend > 0 {
		parts = append(parts, fmt.Sprintf("mend %d damage", o.Mend))
	}
	if o.Wounds > 0 {
		parts = append(parts, fmt.Sprintf("a wound of %d damage", o.Wounds))
	}
	if o.Item != "" {
		parts = append(parts, "gain "+d.item(o.Item))
	}
	if o.Status != "" {
		name := o.Status
		if n, ok := d.statuses[o.Status]; ok && n != "" {
			name = n
		}
		parts = append(parts, "the "+name+" status")
	}
	if o.Fight != "" {
		parts = append(parts, "fight "+d.entity(o.Fight))
	}
	if len(parts) == 0 {
		return "no effect"
	}
	return strings.Join(parts, ", ")
}

func (d describer) eventOutcomeText(c Call) string {
	event, _ := c["event"].(string)
	choice := d.choice(event, fmt.Sprint(c["choice"]))
	if choice.Roll != nil {
		return fmt.Sprintf("Roll %d+: %s; otherwise %s", choice.Roll.Needed, d.effects(choice.Roll.Success), d.effects(choice.Roll.Failure))
	}
	if choice.Outcome != nil {
		return capitalize(d.effects(*choice.Outcome))
	}
	return ""
}

// eventOutcomeStory is the event file's narrative for a choice.
func (d describer) eventOutcomeStory(c Call) string {
	event, _ := c["event"].(string)
	choice := d.choice(event, fmt.Sprint(c["choice"]))
	if choice.Roll != nil {
		return fmt.Sprintf("Roll %d+: %s Otherwise: %s", choice.Roll.Needed, choice.Roll.Success.Text, choice.Roll.Failure.Text)
	}
	if choice.Outcome != nil {
		return choice.Outcome.Text
	}
	return ""
}
