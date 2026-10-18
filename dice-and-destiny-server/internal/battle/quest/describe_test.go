package quest

import (
	"encoding/json"
	"strings"
	"testing"
)

func describeShipped(t *testing.T) CatalogView {
	t.Helper()
	cat, cfg, pack := loadShippedQuests(t)
	_, _, lib := shippedRules(t)
	names := map[string]string{}
	for id, s := range lib.Statuses {
		names[id] = s.Name
	}
	view, err := Describe(cat, cfg, pack, names)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func questView(t *testing.T, view CatalogView, id string) QuestView {
	t.Helper()
	for _, q := range view.Quests {
		if q.ID == id {
			return q
		}
	}
	t.Fatalf("no quest %s in the view", id)
	return QuestView{}
}

func node(q QuestView, id string) (NodeView, bool) {
	for _, n := range q.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return NodeView{}, false
}

func TestDescribeTheMainQuest(t *testing.T) {
	q := questView(t, describeShipped(t), "stop_the_drowned_regent")
	stop, ok := node(q, "stop_him")
	if !ok || !stop.Start || stop.Depth != 0 || stop.Objective.Text != "Defeat the Drowned Regent (the Warlord)" || stop.Objective.EngineText != "Run by today's scenario code" {
		t.Fatalf("the main quest's first stage: %+v", stop)
	}
	if stop.Timer.Type != "urgent" || stop.Timer.Rounds != 4 || stop.Timer.Chip != "Urgent · 4 rounds" || !strings.Contains(stop.Timer.Text, "(rounds_per_region)") {
		t.Fatalf("the timer resolves rounds_per_region: %+v", stop.Timer)
	}
	if !strings.HasPrefix(q.Hook.Text, "From the start of The Salt Marches: A drowned coast.") || q.StatusText == "" || q.RegionName != "The Salt Marches" {
		t.Fatalf("the hook refers to the region's description: %q", q.Hook.Text)
	}
	var early EdgeView
	for _, e := range q.Edges {
		if e.From == "stop_him" && e.Outcome == "success" {
			early = e
		}
	}
	texts := []string{}
	engines := map[string]string{}
	for _, c := range early.Consequences {
		texts = append(texts, c.Text)
		engines[c.Verb] = c.Engine
	}
	joined := strings.Join(texts, " | ")
	for _, want := range []string{"+30 XP per unused round (warlord.early_reward.xp_per_round)", "+1 extra relic choice if any round is unused (warlord.early_reward.extra_relic_choices)", "Mend every wound", "The world opens"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the early strike's consequences say %q: %s", want, joined)
		}
	}
	if engines["open_world"] != EngineNotImplemented || engines["config_reward"] != EngineScenario {
		t.Fatalf("open_world is not yet implemented; the early reward is the scenario's: %v", engines)
	}
	if !strings.HasPrefix(early.To, "end:") {
		t.Fatalf("the early strike ends the quest: %+v", early)
	}
	if deadline, ok := node(q, "deadline"); !ok || deadline.Depth != 1 || deadline.Timer.Chip != "No timer" {
		t.Fatalf("the deadline fight is the timer's failure branch: %+v", deadline)
	}
}

func TestDescribeTheFailureBranchAndEvents(t *testing.T) {
	view := describeShipped(t)
	q := questView(t, view, "kidnapped_daughter")
	found := false
	for _, e := range q.Edges {
		if e.From == "rescue" && e.To == "turned" && e.Outcome == "failure" && e.Label == "Too late" && e.When.Text == "The timer runs out" {
			found = true
			if len(e.Consequences) != 1 || e.Consequences[0].Text != "{beast} appears near {site}, from {captive}" || e.Consequences[0].EngineText != "Not yet implemented" {
				t.Fatalf("the failure spawns the werewolf: %+v", e.Consequences)
			}
		}
	}
	if !found {
		t.Fatalf("the kidnapped daughter's rescue fails into the werewolf stage: %+v", q.Edges)
	}
	if turned, _ := node(q, "turned"); turned.Depth != 1 || len(q.Roles) != 6 || q.Status != StatusPaper {
		t.Fatalf("the werewolf stage sits one column on: %+v", turned)
	}
	shrine := questView(t, view, "the_wayside_shrine")
	labels := []string{}
	for _, e := range shrine.Edges {
		labels = append(labels, e.Label+" → "+strings.Join(func() []string {
			out := []string{}
			for _, c := range e.Consequences {
				out = append(out, c.Text)
			}
			return out
		}(), ", "))
	}
	joined := strings.Join(labels, " | ")
	for _, want := range []string{"Leave an offering (1 Influence) → +8 XP, mend 1 damage", "Take the silver → +15 gold, a wound of 1 damage", "Abandoned"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the shrine's edges read the event file: %s", joined)
		}
	}
	barge := questView(t, view, "the_wrecked_barge")
	if !strings.Contains(barge.Edges[0].Consequences[0].Text, "Roll 4+: +25 gold; otherwise fight the Brine Mask") {
		t.Fatalf("a rolled choice shows both sides: %+v", barge.Edges[0])
	}
	if keep := questView(t, view, "take_brinehold_keep"); !strings.Contains(keep.Edges[0].Consequences[0].Text, "Remove 1 stack of Lieutenant's Favor (statuses.lieutenant) from the Drowned Regent") {
		t.Fatalf("the Lieutenant quest names the status: %+v", keep.Edges[0].Consequences)
	}
}

// Every list in the described catalog is a list, never null.
func TestDescribedListsAreNeverNull(t *testing.T) {
	raw, err := json.Marshal(describeShipped(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		i := strings.Index(string(raw), "null")
		t.Fatalf("the view holds a null near %s", string(raw)[max(0, i-120):i+4])
	}
}
