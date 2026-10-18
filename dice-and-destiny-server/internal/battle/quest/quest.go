// Package quest describes quests as data: every quest is a small graph of
// stages, and every hook, trigger, objective, outcome cause and consequence is
// a verb from a fixed vocabulary (vocabulary.go), like card programs.
//
// Phase 1C is description only. The loader, validator and describer exist so
// the format can be judged; no rule, timer, reward or view field of the
// running expedition reads quest data, and the expedition package never
// imports this one. Quest files live in content/quests_v1/<set>/, one quest
// per file; see docs/game-design/quest-format.md.
package quest

// Quest kinds, statuses, outcomes and role types.
var (
	Kinds        = []string{"main", "side", "small", "personal"}
	Statuses     = []string{StatusScenario, StatusPaper}
	OutcomeTypes = []string{"success", "failure", "abandoned"}
	RoleTypes    = []string{"person", "group", "site", "town"}
	TimerTypes   = []string{"none", "seasonal", "urgent"}
	TimerStarts  = []string{"quest_start", "stage_start"}
	// ReputationTiers are the standings of the open-world plan, worst first.
	ReputationTiers = []string{"hostile", "wary", "neutral", "friendly", "devoted"}
)

const (
	// StatusScenario: the quest describes rules the Go code runs today. It
	// uses real content ids and no roles.
	StatusScenario = "scenario"
	// StatusPaper: design only. It may declare template roles, and its
	// numbers are placeholders.
	StatusPaper = "paper"
	// End is the next of an outcome that ends the quest.
	End = "end"
)

// Call is one use of a verb: {verb: name, param: value, ...}. A value is a
// number, text, an id, a list of ids, {role: name} for a template role, or
// {config: key} for a value in expedition.yaml.
type Call map[string]any

// Verb is the call's verb name.
func (c Call) Verb() string {
	v, _ := c["verb"].(string)
	return v
}

// Role is one template role of a paper quest: what the world builder must
// fill it with (Type) and what it stands for.
type Role struct {
	Type string `yaml:"type" json:"type"`
	Text string `yaml:"text" json:"text"`
}

// Timer limits a stage. Rounds are world-clock halves (one round is the Day or
// Night half of a month); Rounds is a number or {config: key}.
type Timer struct {
	Type   string `yaml:"type" json:"type"`
	Rounds any    `yaml:"rounds,omitempty" json:"rounds,omitempty"`
	From   string `yaml:"from,omitempty" json:"from,omitempty"`
	Before string `yaml:"before,omitempty" json:"before,omitempty"`
}

// Outcome is one way a stage ends: its type, what causes it (When; empty means
// the default: the objective is met for success, the timer runs out for
// failure), what it does, and the next stage or the end.
type Outcome struct {
	Outcome      string `yaml:"outcome" json:"outcome"`
	Label        string `yaml:"label,omitempty" json:"label,omitempty"`
	When         Call   `yaml:"when,omitempty" json:"when,omitempty"`
	Text         string `yaml:"text,omitempty" json:"text,omitempty"`
	Consequences []Call `yaml:"consequences,omitempty" json:"consequences,omitempty"`
	Next         string `yaml:"next" json:"next"`
}

// Stage is one step of a quest: an objective, an optional timer and its
// outcomes. Title defaults to the objective's description.
type Stage struct {
	ID        string    `yaml:"id" json:"id"`
	Title     string    `yaml:"title,omitempty" json:"title,omitempty"`
	Text      string    `yaml:"text,omitempty" json:"text,omitempty"`
	Objective Call      `yaml:"objective" json:"objective"`
	Timer     *Timer    `yaml:"timer,omitempty" json:"timer,omitempty"`
	Outcomes  []Outcome `yaml:"outcomes" json:"outcomes"`
}

// Quest is one quest file.
type Quest struct {
	SchemaVersion int             `yaml:"schema_version" json:"-"`
	ID            string          `yaml:"id" json:"id"`
	Name          string          `yaml:"name" json:"name"`
	Set           string          `yaml:"set" json:"set"`
	Kind          string          `yaml:"kind" json:"kind"`
	Status        string          `yaml:"status" json:"status"`
	Region        string          `yaml:"region,omitempty" json:"region,omitempty"`
	Roles         map[string]Role `yaml:"roles,omitempty" json:"roles,omitempty"`
	Hook          Call            `yaml:"hook" json:"hook"`
	Trigger       []Call          `yaml:"trigger" json:"trigger"`
	Start         string          `yaml:"start" json:"start"`
	Stages        []Stage         `yaml:"stages" json:"stages"`

	// File is the quest's path under the content root, for messages.
	File string `yaml:"-" json:"-"`
}

// Stage returns a stage by id.
func (q Quest) Stage(id string) (Stage, bool) {
	for _, s := range q.Stages {
		if s.ID == id {
			return s, true
		}
	}
	return Stage{}, false
}

// Set is one folder of quests (set.yaml names it). Order places it in the
// catalog (scenario sets before paper ones).
type Set struct {
	SchemaVersion int    `yaml:"schema_version" json:"-"`
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Text          string `yaml:"text" json:"text"`
	Order         int    `yaml:"order" json:"order"`
	// Quests lists the set's quest ids in file order.
	Quests []string `yaml:"-" json:"quests"`
}

// Catalog is every quest, loaded and validated.
type Catalog struct {
	Sets   []Set            `json:"sets"`
	Quests map[string]Quest `json:"quests"`
}

// Ordered lists the quests set by set, in file order.
func (c Catalog) Ordered() []Quest {
	out := []Quest{}
	for _, set := range c.Sets {
		for _, id := range set.Quests {
			out = append(out, c.Quests[id])
		}
	}
	return out
}
