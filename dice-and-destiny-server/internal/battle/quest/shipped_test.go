package quest

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
)

func TestShippedQuestsLoad(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	cat, err := LoadQuests(contentRoot(t), cfg, pack)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Sets[0].ID != "salt_marches" || cat.Sets[1].ID != "paper" {
		t.Fatalf("the scenario set comes first: %s, %s", cat.Sets[0].ID, cat.Sets[1].ID)
	}
	want := map[string][]string{
		"salt_marches": {"free_saltreed", "hunt_the_bell_marshal", "stop_the_drowned_regent", "take_brinehold_keep", "the_hermits_fire", "the_wayside_shrine", "the_wrecked_barge"},
		"paper":        {"kidnapped_daughter", "spring_farms"},
	}
	if len(cat.Sets) != len(want) || len(cat.Quests) != 9 {
		t.Fatalf("the shipped catalog holds two sets and nine quests: %d sets, %d quests", len(cat.Sets), len(cat.Quests))
	}
	for _, set := range cat.Sets {
		if got, ok := want[set.ID]; !ok || len(got) != len(set.Quests) {
			t.Fatalf("set %s: %v", set.ID, set.Quests)
		}
		for i, id := range set.Quests {
			q := cat.Quests[id]
			if id != want[set.ID][i] || q.Set != set.ID {
				t.Fatalf("set %s quest %d is %s (%s)", set.ID, i, id, q.Set)
			}
			if (set.ID == "paper") != (q.Status == StatusPaper) {
				t.Fatalf("%s: the paper set holds the paper quests, the Salt Marches the scenario ones (%s)", id, q.Status)
			}
		}
	}
}

func loadShippedQuests(t *testing.T) (Catalog, expedition.Config, expedition.Pack) {
	t.Helper()
	cfg, pack, _ := shippedRules(t)
	cat, err := LoadQuests(contentRoot(t), cfg, pack)
	if err != nil {
		t.Fatal(err)
	}
	return cat, cfg, pack
}

func sortedSet(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The scenario quests describe the live scenario content, so a Lieutenant,
// an occupied site, an event site or an event choice added to the scenario
// fails this test until the quest files follow.
func TestSaltMarchesQuestsMatchScenario(t *testing.T) {
	cat, cfg, pack := loadShippedQuests(t)
	region := pack.Regions["salt_marches"]
	var set Set
	for _, s := range cat.Sets {
		if s.ID == "salt_marches" {
			set = s
		}
	}
	if len(set.Quests) == 0 {
		t.Fatal("the salt_marches set is missing")
	}
	lieutenants, occupied, events := map[string]bool{}, map[string]bool{}, map[string]bool{}
	mains := 0
	for _, id := range set.Quests {
		q := cat.Quests[id]
		if q.Status != StatusScenario || q.Region != "salt_marches" || len(q.Roles) > 0 {
			t.Fatalf("%s is a scenario quest of the Salt Marches with no roles", id)
		}
		first, _ := q.Stage(q.Start)
		obj := first.Objective
		group, _ := obj["group"].(string)
		switch {
		case q.Kind == "main":
			mains++
			if obj.Verb() != "defeat_group" || group != region.Warlord.Group {
				t.Fatalf("the main quest is to beat the region's Warlord %s: %v", region.Warlord.Group, obj)
			}
			key, _ := singleKey(first.Timer.Rounds, "config")
			value, _ := lookupConfig(mustValues(t, cfg), key)
			if first.Timer.Type != "urgent" || key != "rounds_per_region" || value != cfg.RoundsPerRegion {
				t.Fatalf("the main quest's timer is urgent for rounds_per_region rounds: %+v (%v)", first.Timer, value)
			}
		case obj.Verb() == "defeat_group" && pack.Groups[group].Lieutenant:
			lieutenants[group] = true
			removes := false
			for _, o := range first.Outcomes {
				for _, c := range o.Consequences {
					status, _ := singleKey(c["status"], "config")
					removes = removes || o.Outcome == "success" && c.Verb() == "remove_enemy_status" && c["group"] == region.Warlord.Group && status == "statuses.lieutenant"
				}
			}
			if !removes || q.Kind != "side" {
				t.Fatalf("%s: beating a Lieutenant removes a stack of statuses.lieutenant from the Warlord", id)
			}
		case obj.Verb() == "liberate_site":
			occupied[obj["site"].(string)] = true
		case obj.Verb() == "resolve_event":
			site := obj["site"].(string)
			events[site] = true
			event := pack.Events[pack.Sites[site].Event]
			want, got := map[string]bool{}, map[string]bool{}
			for _, c := range event.Choices {
				want[c.ID] = true
			}
			for _, o := range first.Outcomes {
				if o.When.Verb() != "event_choice" {
					continue
				}
				choice := o.When["choice"].(string)
				if o.Outcome != "success" || len(o.Consequences) != 1 || o.Consequences[0].Verb() != "event_outcome" || o.Consequences[0]["choice"] != choice {
					t.Fatalf("%s: choice %s is a success edge whose consequence is its event_outcome", id, choice)
				}
				got[choice] = true
			}
			if !reflect.DeepEqual(sortedSet(want), sortedSet(got)) {
				t.Fatalf("%s covers the choices %v of %s, not %v", id, sortedSet(got), event.ID, sortedSet(want))
			}
		default:
			t.Fatalf("%s describes nothing the drift test knows", id)
		}
	}
	if mains != 1 {
		t.Fatalf("the Salt Marches have one main quest, not %d", mains)
	}
	wantLieutenants, wantOccupied, wantEvents := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for id, g := range pack.Groups {
		if g.Lieutenant {
			wantLieutenants[id] = true
		}
	}
	for id, s := range pack.Sites {
		if s.OccupiedBy != "" {
			wantOccupied[id] = true
		}
		if s.Type == "event" {
			wantEvents[id] = true
		}
	}
	for _, c := range []struct {
		what      string
		got, want map[string]bool
	}{{"Lieutenant quests", lieutenants, wantLieutenants}, {"liberation quests", occupied, wantOccupied}, {"event quests", events, wantEvents}} {
		if !reflect.DeepEqual(sortedSet(c.got), sortedSet(c.want)) {
			t.Fatalf("the %s cover %v, but the scenario has %v", c.what, sortedSet(c.got), sortedSet(c.want))
		}
	}
}

func mustValues(t *testing.T, cfg expedition.Config) map[string]any {
	t.Helper()
	values, err := configValues(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// The kidnapped daughter is the format's failure branch: the rescue's
// failure leads to the werewolf stage, which has its own outcomes.
func TestKidnappedDaughterHasAFailureBranch(t *testing.T) {
	cat, _, _ := loadShippedQuests(t)
	q := cat.Quests["kidnapped_daughter"]
	rescue, _ := q.Stage("rescue")
	branch := ""
	for _, o := range rescue.Outcomes {
		if o.Outcome == "failure" {
			branch = o.Next
		}
	}
	turned, ok := q.Stage(branch)
	if branch != "turned" || !ok || rescue.Timer == nil || rescue.Timer.Type != "urgent" || rescue.Timer.From != "stage_start" {
		t.Fatalf("the rescue's timed failure leads to the werewolf stage: %q", branch)
	}
	types := map[string]bool{}
	for _, o := range turned.Outcomes {
		types[o.Outcome] = true
	}
	if !types["success"] || !types["failure"] || len(q.Roles) != 6 {
		t.Fatalf("the werewolf stage has its own success and failure: %v (roles %v)", types, q.Roles)
	}
	farms := cat.Quests["spring_farms"]
	clear, _ := farms.Stage("clear")
	if clear.Timer == nil || clear.Timer.Type != "seasonal" || clear.Timer.Before != "spring" || farms.Trigger[0].Verb() != "season" {
		t.Fatalf("the spring farms have a seasonal timer before spring and an autumn/winter trigger: %+v", clear.Timer)
	}
}

func TestShippedQuestFolderIsNeverWritten(t *testing.T) {
	cfg, pack, _ := shippedRules(t)
	shipped := filepath.Join(contentRoot(t), Dir)
	was := snapshot(t, shipped)
	if _, err := LoadQuests(contentRoot(t), cfg, pack); err != nil {
		t.Fatal(err)
	}
	if now := snapshot(t, shipped); now != was {
		t.Fatal("loading the shipped quests changed content/quests_v1")
	}
}

// Phase 1 is description only: the expedition rules never read quest data.
func TestExpeditionDoesNotReadQuests(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "expedition", "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("expedition sources: %v", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "internal/battle/quest\"") || strings.Contains(string(raw), "quests_v1") {
			t.Fatalf("%s reads quest data", filepath.Base(path))
		}
	}
}
