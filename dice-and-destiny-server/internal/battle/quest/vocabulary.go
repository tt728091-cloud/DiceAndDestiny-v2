package quest

import "sort"

// Part is where a verb may stand in a quest.
type Part string

const (
	PartHook        Part = "hook"        // how the hero learns of the quest
	PartTrigger     Part = "trigger"     // world conditions that start it (all must hold)
	PartObjective   Part = "objective"   // what a stage asks
	PartWhen        Part = "when"        // what causes an outcome
	PartConsequence Part = "consequence" // what an outcome does
)

// Parts in the order the docs list them.
var Parts = []Part{PartHook, PartTrigger, PartObjective, PartWhen, PartConsequence}

// Engine statuses. Nothing runs from quest data in Phase 1; the status says
// whether today's Go scenario code runs the rule a verb describes.
const (
	EngineScenario       = "scenario"
	EngineNotImplemented = "not_implemented"
)

// EngineText is the badge the describer and the graph view show.
var EngineText = map[string]string{
	EngineScenario:       "Run by today's scenario code",
	EngineNotImplemented: "Not yet implemented",
}

// Kind is the type of a verb parameter.
type Kind string

const (
	KindText     Kind = "text"      // free text; {role} placeholders must name declared roles
	KindIdent    Kind = "ident"     // a snake_case name with no content behind it yet (paper only)
	KindNumber   Kind = "number"    // an integer, or {config: key} naming a number in expedition.yaml
	KindConfig   Kind = "config"    // a dotted key naming a number in expedition.yaml
	KindStatus   Kind = "status"    // {config: statuses.<name>}: a status id named in expedition.yaml
	KindEntity   Kind = "entity"    // a content id or {role: name}; Param.Entities lists what it may be
	KindRegion   Kind = "region"    // a region id
	KindEvent    Kind = "event"     // an event id
	KindChoice   Kind = "choice"    // a choice of the call's event
	KindItem     Kind = "item"      // a gear id; on a paper quest, a snake_case placeholder too
	KindSeasons  Kind = "seasons"   // a list of season ids
	KindMonths   Kind = "months"    // a list of month ids
	KindDate     Kind = "date"      // a season or month id
	KindUntil    Kind = "until"     // next_month, next_season, or a season or month id
	KindQuest    Kind = "quest"     // a quest id in the catalog
	KindOutcome  Kind = "outcome"   // success, failure or abandoned
	KindTier     Kind = "tier"      // a reputation tier
	KindEnum     Kind = "enum"      // one of Param.Enum
	KindAmount   Kind = "amount"    // a positive number, or all
	KindSiteFlag Kind = "site_flag" // a site state the scenario records
)

// Entities a KindEntity parameter may name. A content id is checked against
// the region pack (people have no content yet, so they are roles only); a
// role must be declared with one of these types.
const (
	EntityPerson = "person"
	EntityGroup  = "group"
	EntitySite   = "site"
	EntityTown   = "town" // a village or town site
)

// SiteFlags are the site states the scenario records.
var SiteFlags = []string{"seen", "freed", "cleared", "resolved", "conquered"}

// Param is one parameter of a verb.
type Param struct {
	Name     string   `json:"name"`
	Kind     Kind     `json:"kind"`
	Required bool     `json:"required"`
	Entities []string `json:"entities,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	// Min bounds a number (MinSet false: any integer).
	Min    int    `json:"min,omitempty"`
	MinSet bool   `json:"-"`
	Text   string `json:"text"`
}

// Verb is one word of the vocabulary. PartParams gives a part its own
// parameters when the verb means something different there (prosperity is a
// level to reach as a trigger and a change as a consequence).
type Verb struct {
	Name       string           `json:"name"`
	Parts      []Part           `json:"parts"`
	Params     []Param          `json:"params"`
	PartParams map[Part][]Param `json:"part_params,omitempty"`
	Engine     string           `json:"engine"`
	Text       string           `json:"text"`
}

// ParamsFor lists the verb's parameters in a part.
func (v Verb) ParamsFor(p Part) []Param {
	if params, ok := v.PartParams[p]; ok {
		return params
	}
	return v.Params
}

func (v Verb) allows(p Part) bool {
	for _, part := range v.Parts {
		if part == p {
			return true
		}
	}
	return false
}

func (v Verb) param(part Part, name string) (Param, bool) {
	for _, p := range v.ParamsFor(part) {
		if p.Name == name {
			return p, true
		}
	}
	return Param{}, false
}

func req(name string, kind Kind, text string) Param {
	return Param{Name: name, Kind: kind, Required: true, Text: text}
}

func opt(name string, kind Kind, text string) Param {
	return Param{Name: name, Kind: kind, Text: text}
}

func entity(name string, required bool, text string, entities ...string) Param {
	return Param{Name: name, Kind: KindEntity, Required: required, Entities: entities, Text: text}
}

func enum(name string, text string, values ...string) Param {
	return Param{Name: name, Kind: KindEnum, Required: true, Enum: values, Text: text}
}

func atLeast(p Param, min int) Param {
	p.Min, p.MinSet = min, true
	return p
}

// Vocabulary is every verb the validator knows. A quest that needs something
// new gets a new verb here (with its engine status), never special-case code.
var Vocabulary = func() map[string]Verb {
	verbs := []Verb{
		// Hooks.
		{Name: "scenario_start", Parts: []Part{PartHook, PartTrigger}, Engine: EngineScenario, Text: "Part of a scenario from its start (the hook shows the region's description).",
			Params: []Param{opt("region", KindRegion, "the scenario's region")}},
		{Name: "rumour", Parts: []Part{PartHook}, Engine: EngineNotImplemented, Text: "The hero hears a rumour.",
			Params: []Param{req("text", KindText, "the rumour")}},
		{Name: "request", Parts: []Part{PartHook}, Engine: EngineNotImplemented, Text: "Someone asks the hero for help.",
			Params: []Param{entity("from", true, "who asks: a person, or the people of a site or town", EntityPerson, EntitySite, EntityTown), opt("text", KindText, "what they say")}},
		{Name: "event", Parts: []Part{PartHook}, Engine: EngineScenario, Text: "The hero comes upon an event scene (the hook shows the event's text).",
			Params: []Param{req("event", KindEvent, "the event scene")}},
		{Name: "raid", Parts: []Part{PartHook}, Engine: EngineNotImplemented, Text: "An enemy raid on a town.",
			Params: []Param{entity("town", true, "the town raided", EntityTown)}},
		{Name: "quest_failed", Parts: []Part{PartHook}, Engine: EngineNotImplemented, Text: "Another quest failed.",
			Params: []Param{req("quest", KindQuest, "the quest that failed")}},
		// Triggers.
		{Name: "season", Parts: []Part{PartTrigger}, Engine: EngineNotImplemented, Text: "The world clock is in one of these seasons.",
			Params: []Param{req("in", KindSeasons, "season ids from the calendar")}},
		{Name: "month", Parts: []Part{PartTrigger}, Engine: EngineNotImplemented, Text: "The world clock is in one of these months.",
			Params: []Param{req("in", KindMonths, "month ids from the calendar")}},
		{Name: "reputation_tier", Parts: []Part{PartTrigger}, Engine: EngineNotImplemented, Text: "A standing has reached a reputation tier.",
			Params: []Param{entity("with", true, "the person or town", EntityPerson, EntityTown), req("at_least", KindTier, "the lowest tier")}},
		{Name: "site_state", Parts: []Part{PartTrigger, PartConsequence}, Engine: EngineScenario, Text: "A site is (trigger) or becomes (consequence) seen, freed, cleared, resolved or conquered.",
			Params: []Param{entity("site", true, "the site", EntitySite, EntityTown), req("state", KindSiteFlag, "the state")}},
		{Name: "quest_outcome", Parts: []Part{PartTrigger, PartWhen}, Engine: EngineNotImplemented, Text: "Another quest has ended (with this outcome, if given).",
			Params: []Param{req("quest", KindQuest, "the quest"), opt("outcome", KindOutcome, "success, failure or abandoned; any if absent")}},
		// Objectives.
		{Name: "defeat_group", Parts: []Part{PartObjective, PartWhen}, Engine: EngineScenario, Text: "Beat an enemy group in battle (as a cause: the group is beaten).",
			Params: []Param{entity("group", true, "the group", EntityGroup), entity("at", false, "where it waits", EntitySite, EntityTown)}},
		{Name: "liberate_site", Parts: []Part{PartObjective}, Engine: EngineScenario, Text: "Free a site held by enemies (its occupiers or garrison).",
			Params: []Param{entity("site", true, "the site", EntitySite, EntityTown)}},
		{Name: "reach_site", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Reach a site.",
			Params: []Param{entity("site", true, "the site", EntitySite, EntityTown)}},
		{Name: "resolve_event", Parts: []Part{PartObjective}, Engine: EngineScenario, Text: "Play an event site's scene through one of its choices.",
			Params: []Param{entity("site", true, "the event site", EntitySite)}},
		{Name: "deliver_item", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Carry an item to someone (it sits in the deck as a card that counts as health).",
			Params: []Param{req("item", KindItem, "the item"), entity("to", true, "who receives it", EntityPerson, EntityGroup, EntityTown)}},
		{Name: "be_at_by", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Be at a place by a date.",
			Params: []Param{entity("site", true, "the place", EntitySite, EntityTown), req("by", KindDate, "a season or month id")}},
		{Name: "spend_influence", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Spend Influence with someone or somewhere.",
			Params: []Param{atLeast(req("amount", KindNumber, "the Influence"), 1), entity("at", false, "with whom or where", EntityPerson, EntitySite, EntityTown)}},
		{Name: "survey_tiles", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Explore a number of tiles.",
			Params: []Param{atLeast(req("count", KindNumber, "the tiles"), 1), opt("region", KindRegion, "the region")}},
		{Name: "clear_dungeon", Parts: []Part{PartObjective}, Engine: EngineNotImplemented, Text: "Clear a dungeon.",
			Params: []Param{entity("site", true, "the dungeon", EntitySite)}},
		// Outcome causes.
		{Name: "timer_expired", Parts: []Part{PartWhen}, Engine: EngineScenario, Text: "The stage's timer runs out (the default cause of a failure in a timed stage)."},
		{Name: "event_choice", Parts: []Part{PartWhen}, Engine: EngineScenario, Text: "One choice of an event scene is taken.",
			Params: []Param{req("event", KindEvent, "the event"), req("choice", KindChoice, "the choice")}},
		{Name: "lose_battle", Parts: []Part{PartWhen}, Engine: EngineScenario, Text: "The hero loses a battle against a group.",
			Params: []Param{entity("group", true, "the group", EntityGroup)}},
		// Consequences.
		{Name: "xp", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "Grant XP.",
			Params: []Param{req("amount", KindNumber, "the XP")}},
		{Name: "gold", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "Grant gold.",
			Params: []Param{req("amount", KindNumber, "the gold")}},
		{Name: "gear", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "Grant an item.",
			Params: []Param{req("item", KindItem, "the item")}},
		{Name: "reputation", Parts: []Part{PartConsequence}, Engine: EngineNotImplemented, Text: "Change a standing.",
			Params: []Param{entity("with", true, "the person or town", EntityPerson, EntityTown), req("amount", KindNumber, "the change, up or down")}},
		{Name: "prosperity", Parts: []Part{PartTrigger, PartConsequence}, Engine: EngineNotImplemented, Text: "A town's prosperity has reached a level (trigger), or changes (consequence).",
			PartParams: map[Part][]Param{
				PartTrigger:     {entity("town", true, "the town", EntityTown), req("at_least", KindNumber, "the lowest prosperity")},
				PartConsequence: {entity("town", true, "the town", EntityTown), req("amount", KindNumber, "the change, up or down")},
			}},
		{Name: "town_condition", Parts: []Part{PartConsequence}, Engine: EngineNotImplemented, Text: "A town suffers (or enjoys) a condition until a point in time.",
			Params: []Param{entity("town", true, "the town", EntityTown), req("condition", KindIdent, "the condition"), req("text", KindText, "what it does"), req("until", KindUntil, "when it ends")}},
		{Name: "remove_enemy_status", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "Take stacks of a status from an enemy.",
			Params: []Param{entity("group", true, "the enemy", EntityGroup), req("status", KindStatus, "the status, named in expedition.yaml"), atLeast(req("stacks", KindNumber, "the stacks"), 1)}},
		{Name: "spawn", Parts: []Part{PartConsequence}, Engine: EngineNotImplemented, Text: "A new group appears.",
			Params: []Param{entity("group", true, "the group", EntityGroup), entity("from", false, "who or what it comes from", EntityPerson, EntityGroup), entity("at", false, "where", EntitySite, EntityTown)}},
		{Name: "start_quest", Parts: []Part{PartConsequence}, Engine: EngineNotImplemented, Text: "Start another quest.",
			Params: []Param{req("quest", KindQuest, "the quest")}},
		{Name: "open_world", Parts: []Part{PartConsequence}, Engine: EngineNotImplemented, Text: "The open world begins."},
		{Name: "end_run", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "The run is over."},
		{Name: "config_reward", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "A reward whose size is a number in expedition.yaml.",
			Params: []Param{req("key", KindConfig, "the config key"), enum("reward", "what it grants", "xp", "relic_choice"), enum("per", "how often: per unused round of the stage's timer, or once if any round is unused", "unused_round", "early")}},
		{Name: "event_outcome", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "What an event choice does, as the event file says.",
			Params: []Param{req("event", KindEvent, "the event"), req("choice", KindChoice, "the choice")}},
		{Name: "mend_wounds", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "Heal damage from the hero's wounds.",
			Params: []Param{req("amount", KindAmount, "points of damage, or all")}},
		{Name: "win_region", Parts: []Part{PartConsequence}, Engine: EngineScenario, Text: "The region is won.",
			Params: []Param{opt("region", KindRegion, "the region")}},
	}
	out := map[string]Verb{}
	for _, v := range verbs {
		if _, dup := out[v.Name]; dup {
			panic("quest vocabulary: duplicate verb " + v.Name)
		}
		out[v.Name] = v
	}
	return out
}()

// VerbNames lists the vocabulary alphabetically.
func VerbNames() []string {
	names := make([]string, 0, len(Vocabulary))
	for name := range Vocabulary {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TimerEngine is the engine status of each timer type: the region's deadline
// is an urgent timer the scenario code runs; seasonal deadlines wait for the
// quest runtime.
var TimerEngine = map[string]string{"none": EngineScenario, "urgent": EngineScenario, "seasonal": EngineNotImplemented}
