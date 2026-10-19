package quest

import "testing"

// The handoff's starting vocabulary is all present, each verb in its part.
func TestVocabularyHoldsTheStartingList(t *testing.T) {
	starting := map[Part][]string{
		PartHook:        {"scenario_start", "rumour", "request", "event", "raid", "quest_failed"},
		PartTrigger:     {"scenario_start", "season", "month", "reputation_tier", "prosperity", "site_state", "quest_outcome"},
		PartObjective:   {"defeat_group", "liberate_site", "reach_site", "resolve_event", "deliver_item", "be_at_by", "spend_influence", "survey_tiles", "clear_dungeon"},
		PartConsequence: {"xp", "gold", "gear", "reputation", "prosperity", "town_condition", "remove_enemy_status", "site_state", "spawn", "start_quest", "open_world", "end_run", "config_reward", "event_outcome"},
	}
	for part, names := range starting {
		for _, name := range names {
			verb, ok := Vocabulary[name]
			if !ok || !verb.allows(part) {
				t.Fatalf("%s is a %s verb", name, part)
			}
		}
	}
}

func TestEveryVerbIsWellFormed(t *testing.T) {
	kinds := map[Kind]bool{KindText: true, KindIdent: true, KindNumber: true, KindConfig: true, KindStatus: true, KindEntity: true, KindRegion: true, KindEvent: true, KindChoice: true, KindItem: true,
		KindSeasons: true, KindMonths: true, KindDate: true, KindUntil: true, KindQuest: true, KindOutcome: true, KindTier: true, KindEnum: true, KindAmount: true, KindSiteFlag: true}
	entities := map[string]bool{EntityPerson: true, EntityGroup: true, EntitySite: true, EntityTown: true}
	for _, name := range VerbNames() {
		verb := Vocabulary[name]
		if verb.Name != name || verb.Text == "" || len(verb.Parts) == 0 || EngineText[verb.Engine] == "" {
			t.Fatalf("%s needs a name, text, parts and an engine status: %+v", name, verb)
		}
		for _, part := range verb.Parts {
			seen := map[string]bool{}
			for _, p := range verb.ParamsFor(part) {
				if !kinds[p.Kind] || p.Name == "" || p.Name == "verb" || p.Text == "" || seen[p.Name] {
					t.Fatalf("%s %s: bad parameter %+v", name, part, p)
				}
				seen[p.Name] = true
				if p.Kind == KindEntity {
					if len(p.Entities) == 0 {
						t.Fatalf("%s.%s names no entities", name, p.Name)
					}
					for _, e := range p.Entities {
						if !entities[e] {
							t.Fatalf("%s.%s: unknown entity %q", name, p.Name, e)
						}
					}
				}
				if p.Kind == KindEnum && len(p.Enum) == 0 {
					t.Fatalf("%s.%s has no values", name, p.Name)
				}
			}
		}
	}
	for _, timer := range TimerTypes {
		if EngineText[TimerEngine[timer]] == "" {
			t.Fatalf("timer %s needs an engine status", timer)
		}
	}
}
