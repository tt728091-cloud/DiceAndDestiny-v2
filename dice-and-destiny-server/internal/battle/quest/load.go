package quest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/expedition"
	"gopkg.in/yaml.v3"
)

var (
	stableID    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	placeholder = regexp.MustCompile(`\{([^{}]*)\}`)
)

// Dir is the quest folder under the content root.
const Dir = "quests_v1"

// LoadQuests reads content/quests_v1 strictly: one folder per set (named by
// its set.yaml) holding one quest per file, each checked against the
// vocabulary, the expedition config (calendar and config keys) and the region
// pack (regions, sites, groups, events, choices, gear). Any problem refuses
// the whole catalog with an error naming the file and the field. It only
// reads.
func LoadQuests(root string, cfg expedition.Config, pack expedition.Pack) (Catalog, error) {
	cat := Catalog{Sets: []Set{}, Quests: map[string]Quest{}}
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cat, fmt.Errorf("quest content: read %s: %w", Dir, err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		set, quests, err := loadSet(root, entry.Name())
		if err != nil {
			return cat, err
		}
		for _, q := range quests {
			if other, dup := files[q.ID]; dup {
				return cat, fmt.Errorf("quest content: %s: id: duplicate quest id %q (also in %s)", q.File, q.ID, other)
			}
			files[q.ID] = q.File
			cat.Quests[q.ID] = q
			set.Quests = append(set.Quests, q.ID)
		}
		cat.Sets = append(cat.Sets, set)
	}
	if len(cat.Sets) == 0 {
		return cat, fmt.Errorf("quest content: %s holds no quest sets", Dir)
	}
	orders := map[int]string{}
	for _, set := range cat.Sets {
		if other, dup := orders[set.Order]; dup {
			return cat, fmt.Errorf("quest content: %s/%s/set.yaml: order: %d is also %s's", Dir, set.ID, set.Order, other)
		}
		orders[set.Order] = set.ID
	}
	sort.Slice(cat.Sets, func(i, j int) bool { return cat.Sets[i].Order < cat.Sets[j].Order })
	values, err := configValues(cfg)
	if err != nil {
		return cat, err
	}
	for _, q := range cat.Ordered() {
		c := &checker{quest: q, cfg: cfg, pack: pack, values: values, quests: cat.Quests, used: map[string]bool{}}
		if err := c.check(); err != nil {
			return cat, err
		}
	}
	return cat, nil
}

func decodeStrict(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

func loadSet(root, name string) (Set, []Quest, error) {
	var set Set
	rel := filepath.ToSlash(filepath.Join(Dir, name, "set.yaml"))
	if err := decodeStrict(filepath.Join(root, rel), &set); err != nil {
		return set, nil, fmt.Errorf("quest content: %s: %w", rel, err)
	}
	if set.SchemaVersion != 1 {
		return set, nil, fmt.Errorf("quest content: %s: schema_version must be 1", rel)
	}
	if set.ID != name || set.Name == "" || set.Text == "" || set.Order < 1 {
		return set, nil, fmt.Errorf("quest content: %s: a set needs its folder's id (%q), a name, text and an order of at least 1", rel, name)
	}
	set.Quests = []string{}
	entries, err := os.ReadDir(filepath.Join(root, Dir, name))
	if err != nil {
		return set, nil, err
	}
	names := []string{}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if !entry.IsDir() && (ext == ".yaml" || ext == ".yml") && entry.Name() != "set.yaml" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return set, nil, fmt.Errorf("quest content: set %q holds no quests", name)
	}
	quests := []Quest{}
	for _, file := range names {
		var q Quest
		rel := filepath.ToSlash(filepath.Join(Dir, name, file))
		if err := decodeStrict(filepath.Join(root, rel), &q); err != nil {
			return set, nil, fmt.Errorf("quest content: %s: %w", rel, err)
		}
		q.File = rel
		if q.SchemaVersion != 1 {
			return set, nil, fmt.Errorf("quest content: %s: schema_version must be 1", rel)
		}
		if !stableID.MatchString(q.ID) || strings.TrimSuffix(file, filepath.Ext(file)) != q.ID {
			return set, nil, fmt.Errorf("quest content: %s: id: %q must be snake_case and match the file name", rel, q.ID)
		}
		if q.Set != name {
			return set, nil, fmt.Errorf("quest content: %s: set: %q must be the folder's set %q", rel, q.Set, name)
		}
		quests = append(quests, q)
	}
	return set, quests, nil
}

// configValues is expedition.yaml as a tree of values keyed like the file, so
// a dotted key such as warlord.early_reward.xp_per_round resolves.
func configValues(cfg expedition.Config) (map[string]any, error) {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, yaml.Unmarshal(raw, &out)
}

func lookupConfig(values map[string]any, key string) (any, bool) {
	var at any = values
	for _, part := range strings.Split(key, ".") {
		m, ok := asMap(at)
		if !ok {
			return nil, false
		}
		if at, ok = m[part]; !ok {
			return nil, false
		}
	}
	return at, true
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

// asMap reads a nested map; yaml.v3 decodes maps inside a Call as Calls.
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case Call:
		return m, true
	case map[string]any:
		return m, true
	}
	return nil, false
}

// singleKey reads a one-key map such as {role: name} or {config: key}.
func singleKey(v any, key string) (string, bool) {
	m, ok := asMap(v)
	if !ok || len(m) != 1 {
		return "", false
	}
	s, ok := m[key].(string)
	return s, ok
}

type checker struct {
	quest  Quest
	cfg    expedition.Config
	pack   expedition.Pack
	values map[string]any
	quests map[string]Quest
	used   map[string]bool
}

func (c *checker) fail(field, format string, args ...any) error {
	return fmt.Errorf("quest content: %s: %s: %s", c.quest.File, field, fmt.Sprintf(format, args...))
}

func (c *checker) check() error {
	q := c.quest
	if q.Name == "" {
		return c.fail("name", "a quest needs a name")
	}
	if !contains(Kinds, q.Kind) {
		return c.fail("kind", "%q is not one of %s", q.Kind, strings.Join(Kinds, ", "))
	}
	if !contains(Statuses, q.Status) {
		return c.fail("status", "%q is not one of %s", q.Status, strings.Join(Statuses, ", "))
	}
	if q.Region != "" {
		if _, ok := c.pack.Regions[q.Region]; !ok {
			return c.fail("region", "unknown region %q", q.Region)
		}
	}
	if q.Status == StatusScenario && len(q.Roles) > 0 {
		return c.fail("roles", "a scenario quest uses real ids, not template roles")
	}
	for _, name := range sortedKeys(q.Roles) {
		role := q.Roles[name]
		if !stableID.MatchString(name) {
			return c.fail("roles."+name, "a role needs a snake_case name")
		}
		if !contains(RoleTypes, role.Type) {
			return c.fail("roles."+name+".type", "%q is not one of %s", role.Type, strings.Join(RoleTypes, ", "))
		}
		if err := c.text("roles."+name+".text", role.Text, true); err != nil {
			return err
		}
	}
	if q.Hook == nil {
		return c.fail("hook", "a quest needs a hook")
	}
	if err := c.call("hook", PartHook, q.Hook, nil); err != nil {
		return err
	}
	if len(q.Trigger) == 0 {
		return c.fail("trigger", "a quest needs at least one trigger")
	}
	for i, t := range q.Trigger {
		if err := c.call(fmt.Sprintf("trigger[%d]", i), PartTrigger, t, nil); err != nil {
			return err
		}
	}
	if len(q.Stages) == 0 {
		return c.fail("stages", "a quest needs at least one stage")
	}
	ids := map[string]bool{}
	for i, s := range q.Stages {
		if !stableID.MatchString(s.ID) || s.ID == End {
			return c.fail(fmt.Sprintf("stages[%d].id", i), "%q must be a snake_case id other than %q", s.ID, End)
		}
		if ids[s.ID] {
			return c.fail(fmt.Sprintf("stages[%d].id", i), "duplicate stage id %q", s.ID)
		}
		ids[s.ID] = true
	}
	if q.Start == "" {
		return c.fail("start", "a quest needs a start stage")
	}
	if !ids[q.Start] {
		return c.fail("start", "names no stage (%q)", q.Start)
	}
	for i := range q.Stages {
		if err := c.stage(fmt.Sprintf("stages[%d]", i), &q.Stages[i], ids); err != nil {
			return err
		}
	}
	if err := c.graph(); err != nil {
		return err
	}
	for _, name := range sortedKeys(q.Roles) {
		if !c.used[name] {
			return c.fail("roles."+name, "role %q is declared but never used", name)
		}
	}
	return nil
}

func (c *checker) stage(field string, s *Stage, ids map[string]bool) error {
	if err := c.text(field+".title", s.Title, false); err != nil {
		return err
	}
	if err := c.text(field+".text", s.Text, false); err != nil {
		return err
	}
	if s.Objective == nil {
		return c.fail(field+".objective", "a stage needs an objective")
	}
	if err := c.call(field+".objective", PartObjective, s.Objective, s); err != nil {
		return err
	}
	timed, err := c.timer(field+".timer", s.Timer)
	if err != nil {
		return err
	}
	if len(s.Outcomes) == 0 {
		return c.fail(field+".outcomes", "a stage needs outcomes")
	}
	causes := map[string]bool{}
	success, failure := false, false
	for i, o := range s.Outcomes {
		of := fmt.Sprintf("%s.outcomes[%d]", field, i)
		if !contains(OutcomeTypes, o.Outcome) {
			return c.fail(of+".outcome", "%q is not one of %s", o.Outcome, strings.Join(OutcomeTypes, ", "))
		}
		success = success || o.Outcome == "success"
		failure = failure || o.Outcome == "failure"
		switch {
		case o.When != nil:
			if err := c.call(of+".when", PartWhen, o.When, s); err != nil {
				return err
			}
			if o.When.Verb() == "timer_expired" && !timed {
				return c.fail(of+".when", "timer_expired needs a stage with a timer")
			}
		case o.Outcome == "abandoned":
			return c.fail(of+".when", "an abandoned outcome needs a when")
		case o.Outcome == "failure" && !timed:
			return c.fail(of+".when", "a failure without a when is the timer running out, but the stage has no timer")
		}
		key := o.Outcome + "|" + causeKey(o.When)
		if causes[key] {
			return c.fail(of+".when", "two %s outcomes have the same cause; give each its own when", o.Outcome)
		}
		causes[key] = true
		for _, f := range []struct{ name, value string }{{"label", o.Label}, {"text", o.Text}} {
			if err := c.text(of+"."+f.name, f.value, false); err != nil {
				return err
			}
		}
		for j, cons := range o.Consequences {
			if err := c.call(fmt.Sprintf("%s.consequences[%d]", of, j), PartConsequence, cons, s); err != nil {
				return err
			}
		}
		if o.Next == "" {
			return c.fail(of+".next", "an outcome needs a next stage or %q", End)
		}
		if o.Next != End && !ids[o.Next] {
			return c.fail(of+".next", "names no stage (%q)", o.Next)
		}
	}
	if !success {
		return c.fail(field+".outcomes", "stage %q has no success outcome", s.ID)
	}
	if timed && !failure {
		return c.fail(field+".outcomes", "stage %q has a timer but no failure outcome (a failure may simply lead to %q)", s.ID, End)
	}
	return nil
}

func causeKey(when Call) string {
	if when == nil {
		return "default"
	}
	raw, _ := json.Marshal(when)
	return string(raw)
}

// timer checks a stage's timer and reports whether it can run out.
func (c *checker) timer(field string, t *Timer) (bool, error) {
	if t == nil {
		return false, nil
	}
	switch t.Type {
	case "none":
		if t.Rounds != nil || t.From != "" || t.Before != "" {
			return false, c.fail(field, "a none timer has no rounds, from or before")
		}
		return false, nil
	case "urgent":
		if t.Rounds == nil {
			return false, c.fail(field+".rounds", "an urgent timer needs rounds (a number or {config: key})")
		}
		if err := c.number(field+".rounds", t.Rounds, 1, true); err != nil {
			return false, err
		}
		if !contains(TimerStarts, t.From) {
			return false, c.fail(field+".from", "an urgent timer counts from %s, not %q", strings.Join(TimerStarts, " or "), t.From)
		}
		if t.Before != "" {
			return false, c.fail(field+".before", "an urgent timer has no before")
		}
	case "seasonal":
		if _, ok := c.cfg.Calendar.Season(t.Before); !ok {
			return false, c.fail(field+".before", "a seasonal timer needs a season of the calendar, not %q", t.Before)
		}
		if t.Rounds != nil || t.From != "" {
			return false, c.fail(field, "a seasonal timer has no rounds or from")
		}
	default:
		return false, c.fail(field+".type", "%q is not one of %s", t.Type, strings.Join(TimerTypes, ", "))
	}
	return true, nil
}

// graph refuses stages no path from the start reaches and cycles (a retry is
// a new stage).
func (c *checker) graph() error {
	q := c.quest
	next := map[string][]string{}
	for _, s := range q.Stages {
		for _, o := range s.Outcomes {
			if o.Next != End {
				next[s.ID] = append(next[s.ID], o.Next)
			}
		}
	}
	state := map[string]int{} // 1 visiting, 2 done
	var visit func(id string, path []string) error
	visit = func(id string, path []string) error {
		switch state[id] {
		case 1:
			return c.fail("stages", "the stages form a cycle (%s); a retry is a new stage", strings.Join(append(path, id), " → "))
		case 2:
			return nil
		}
		state[id] = 1
		for _, n := range next[id] {
			if err := visit(n, append(path, id)); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	if err := visit(q.Start, nil); err != nil {
		return err
	}
	for i, s := range q.Stages {
		if state[s.ID] == 0 {
			return c.fail(fmt.Sprintf("stages[%d]", i), "no path from the start (%q) reaches stage %q", q.Start, s.ID)
		}
	}
	return nil
}

// text checks free text: present when required, and every {placeholder}
// names a declared role (scenario quests have none).
func (c *checker) text(field, value string, required bool) error {
	if required && strings.TrimSpace(value) == "" {
		return c.fail(field, "needs text")
	}
	for _, m := range placeholder.FindAllStringSubmatch(value, -1) {
		if _, ok := c.quest.Roles[m[1]]; !ok {
			return c.fail(field, "{%s} is not a declared role", m[1])
		}
		c.used[m[1]] = true
	}
	return nil
}

// call checks one verb use in a part. stage is the stage it stands in (nil
// for the hook and triggers).
func (c *checker) call(field string, part Part, call Call, stage *Stage) error {
	name := call.Verb()
	if name == "" {
		return c.fail(field+".verb", "needs a verb")
	}
	verb, ok := Vocabulary[name]
	if !ok {
		return c.fail(field+".verb", "unknown verb %q", name)
	}
	if !verb.allows(part) {
		return c.fail(field+".verb", "%s cannot be used as %s", name, partNames[part])
	}
	for _, key := range sortedKeys(call) {
		if key == "verb" {
			continue
		}
		param, ok := verb.param(part, key)
		if !ok {
			return c.fail(field+"."+key, "%s has no parameter %q", name, key)
		}
		if err := c.value(field+"."+key, param, call[key], call); err != nil {
			return err
		}
	}
	for _, param := range verb.ParamsFor(part) {
		if _, ok := call[param.Name]; param.Required && !ok {
			return c.fail(field, "%s needs %s (%s)", name, param.Name, param.Text)
		}
	}
	return c.semantics(field, part, call, stage)
}

var partNames = map[Part]string{PartHook: "a hook", PartTrigger: "a trigger", PartObjective: "an objective", PartWhen: "an outcome's when", PartConsequence: "a consequence"}

func (c *checker) value(field string, p Param, v any, call Call) error {
	str, isString := v.(string)
	switch p.Kind {
	case KindText:
		if !isString {
			return c.fail(field, "must be text")
		}
		return c.text(field, str, true)
	case KindIdent:
		if !isString || !stableID.MatchString(str) {
			return c.fail(field, "must be a snake_case name")
		}
	case KindNumber:
		min := 0
		if p.MinSet {
			min = p.Min
		}
		return c.number(field, v, min, p.MinSet)
	case KindConfig:
		if !isString {
			return c.fail(field, "must be a config key")
		}
		return c.configNumber(field, str)
	case KindStatus:
		key, ok := singleKey(v, "config")
		if !ok || !strings.HasPrefix(key, "statuses.") {
			return c.fail(field, "must be {config: statuses.<name>}, the status named in expedition.yaml")
		}
		value, found := lookupConfig(c.values, key)
		if id, isID := value.(string); !found || !isID || id == "" {
			return c.fail(field, "unknown config key %q", key)
		}
	case KindEntity:
		return c.entity(field, p, v)
	case KindRegion:
		if _, ok := c.pack.Regions[str]; !isString || !ok {
			return c.fail(field, "unknown region %v", quoted(v))
		}
	case KindEvent:
		if _, ok := c.pack.Events[str]; !isString || !ok {
			return c.fail(field, "unknown event %v", quoted(v))
		}
	case KindChoice:
		eventID, _ := call["event"].(string)
		event, ok := c.pack.Events[eventID]
		if !ok {
			return c.fail(field, "a choice needs a known event")
		}
		for _, choice := range event.Choices {
			if isString && choice.ID == str {
				return nil
			}
		}
		return c.fail(field, "unknown choice %v of event %q", quoted(v), eventID)
	case KindItem:
		if !isString {
			return c.fail(field, "must be an item id")
		}
		if _, ok := c.pack.Gear[str]; !ok && !(c.quest.Status == StatusPaper && stableID.MatchString(str)) {
			return c.fail(field, "unknown item %q", str)
		}
	case KindSeasons, KindMonths:
		list, ok := v.([]any)
		if !ok || len(list) == 0 {
			return c.fail(field, "must be a list of ids")
		}
		for _, item := range list {
			id, _ := item.(string)
			if p.Kind == KindSeasons {
				if _, known := c.cfg.Calendar.Season(id); !known {
					return c.fail(field, "unknown season %v", quoted(item))
				}
			} else if _, known := c.cfg.Calendar.MonthIndex(id); !known {
				return c.fail(field, "unknown month %v", quoted(item))
			}
		}
	case KindDate, KindUntil:
		if p.Kind == KindUntil && (str == "next_month" || str == "next_season") {
			return nil
		}
		_, season := c.cfg.Calendar.Season(str)
		_, month := c.cfg.Calendar.MonthIndex(str)
		if !isString || !season && !month {
			return c.fail(field, "unknown season or month %v", quoted(v))
		}
	case KindQuest:
		if _, ok := c.quests[str]; !isString || !ok {
			return c.fail(field, "unknown quest %v", quoted(v))
		}
		if str == c.quest.ID {
			return c.fail(field, "a quest cannot refer to itself")
		}
	case KindOutcome:
		if !contains(OutcomeTypes, str) {
			return c.fail(field, "%v is not one of %s", quoted(v), strings.Join(OutcomeTypes, ", "))
		}
	case KindTier:
		if !contains(ReputationTiers, str) {
			return c.fail(field, "%v is not one of %s", quoted(v), strings.Join(ReputationTiers, ", "))
		}
	case KindEnum:
		if !contains(p.Enum, str) {
			return c.fail(field, "%v is not one of %s", quoted(v), strings.Join(p.Enum, ", "))
		}
	case KindAmount:
		if str == "all" {
			return nil
		}
		return c.number(field, v, 1, true)
	case KindSiteFlag:
		if !contains(SiteFlags, str) {
			return c.fail(field, "%v is not one of %s", quoted(v), strings.Join(SiteFlags, ", "))
		}
	}
	return nil
}

func quoted(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%v", v)
}

// number checks an integer or {config: key} naming a number.
func (c *checker) number(field string, v any, min int, bounded bool) error {
	n, ok := asInt(v)
	if !ok {
		key, isRef := singleKey(v, "config")
		if !isRef {
			return c.fail(field, "must be a number or {config: key}")
		}
		if err := c.configNumber(field, key); err != nil {
			return err
		}
		value, _ := lookupConfig(c.values, key)
		n, _ = asInt(value)
	}
	if bounded && n < min {
		return c.fail(field, "must be at least %d, not %d", min, n)
	}
	return nil
}

func (c *checker) configNumber(field, key string) error {
	value, ok := lookupConfig(c.values, key)
	if !ok {
		return c.fail(field, "unknown config key %q", key)
	}
	if _, isNumber := asInt(value); !isNumber {
		return c.fail(field, "config key %q is not a number", key)
	}
	return nil
}

// entity checks a content id or {role: name} against what the parameter may
// name.
func (c *checker) entity(field string, p Param, v any) error {
	if role, ok := singleKey(v, "role"); ok {
		decl, declared := c.quest.Roles[role]
		if !declared {
			return c.fail(field, "role %q is not declared", role)
		}
		if !contains(p.Entities, decl.Type) {
			return c.fail(field, "role %q is a %s; this takes a %s", role, decl.Type, strings.Join(p.Entities, " or "))
		}
		c.used[role] = true
		return nil
	}
	id, ok := v.(string)
	if !ok {
		return c.fail(field, "must be an id or {role: name}")
	}
	for _, kind := range p.Entities {
		switch kind {
		case EntityGroup:
			if _, ok := c.pack.Groups[id]; ok {
				return nil
			}
		case EntitySite:
			if _, ok := c.pack.Sites[id]; ok {
				return nil
			}
		case EntityTown:
			if s, ok := c.pack.Sites[id]; ok && (s.Type == "village" || s.Type == "town") {
				return nil
			}
		}
	}
	if len(p.Entities) == 1 && p.Entities[0] == EntityPerson {
		return c.fail(field, "people have no content yet; name a person role")
	}
	return c.fail(field, "unknown %s %q", strings.Join(p.Entities, " or "), id)
}

// semantics are the checks that tie a call to the content beyond its ids.
func (c *checker) semantics(field string, part Part, call Call, stage *Stage) error {
	plain := func(key string) (string, bool) {
		s, ok := call[key].(string)
		return s, ok
	}
	switch call.Verb() {
	case "liberate_site":
		if id, ok := plain("site"); ok {
			if s := c.pack.Sites[id]; s.OccupiedBy == "" && s.Garrison == "" {
				return c.fail(field+".site", "site %q is held by no one (no occupied_by or garrison)", id)
			}
		}
	case "resolve_event":
		if id, ok := plain("site"); ok && c.pack.Sites[id].Type != "event" {
			return c.fail(field+".site", "site %q is not an event site", id)
		}
	case "defeat_group":
		group, groupPlain := plain("group")
		site, sitePlain := plain("at")
		if groupPlain && sitePlain {
			s := c.pack.Sites[site]
			holds := s.Garrison == group || s.OccupiedBy == group || s.Guardian != nil && (s.Guardian.Day == group || s.Guardian.Night == group)
			if !holds {
				return c.fail(field+".at", "site %q does not hold group %q", site, group)
			}
		}
	case "event_choice", "event_outcome":
		if stage != nil && stage.Objective.Verb() == "resolve_event" {
			if site, ok := stage.Objective["site"].(string); ok {
				if want := c.pack.Sites[site].Event; call["event"] != want {
					return c.fail(field+".event", "this stage resolves event %q, not %v", want, quoted(call["event"]))
				}
			}
		}
	case "config_reward":
		if stage == nil || stage.Timer == nil || stage.Timer.Type != "urgent" {
			return c.fail(field+".per", "a reward per unused round needs a stage with an urgent timer")
		}
	}
	return nil
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
