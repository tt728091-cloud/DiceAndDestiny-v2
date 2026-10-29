# The quest format

**Status:** Phase 1C, description only (2026-10-18). Built from [`phase-1bc-handoff.md`](phase-1bc-handoff.md); the plan of record is [`open-world-plan.html`](open-world-plan.html) §6 "Towns, quests and prosperity".

Quests are data, not code, the same way card programs are: the engine implements a fixed vocabulary of verbs once, and every quest is a combination of them. Every quest is a small graph:

```
quest   → stages
stage   → objective · timer (none / seasonal / urgent) · outcomes
outcome (success / failure / abandoned) → next stage, or the end
          + consequences
```

**Nothing runs from quest data yet.** The loader, validator and describer exist so the format can be judged. No rule, timer, reward or view field of the running expedition reads quest data, and `internal/battle/expedition` never imports `internal/battle/quest` (`TestExpeditionDoesNotReadQuests`). The Salt Marches' Go rules still run the scenario; its quest files only describe it. Running quests (timers that tick, quest logs, branches that change the world) is Phase 5, and the quest editor (Phase 5E) is built from this document.

## Contents

- [Files](#files)
- [Fields](#fields)
- [Values: ids, roles, config references, text](#values-ids-roles-config-references-text)
- [The vocabulary](#the-vocabulary)
- [Refer, don't copy](#refer-dont-copy)
- [The validator](#the-validator)
- [The describer and the graph view](#the-describer-and-the-graph-view)
- [Examples](#examples)
- [Adding to the format](#adding-to-the-format)

## Files

```
dice-and-destiny-server/content/quests_v1/
  salt_marches/            one folder per set
    set.yaml               the set's id, name, text and order
    stop_the_drowned_regent.yaml   one quest per file, named after its id
    …
  paper/
    set.yaml
    kidnapped_daughter.yaml
    spring_farms.yaml
```

- **`salt_marches/`** (`status: scenario`) describes the opening scenario as seven quests: the main quest `stop_the_drowned_regent`; a side quest per Lieutenant (`hunt_the_bell_marshal`, `take_brinehold_keep`); `free_saltreed`; and a small quest per event site (`the_wayside_shrine`, `the_wrecked_barge`, `the_hermits_fire`).
- **`paper/`** (`status: paper`) holds design-only quests that test what the scenario lacks: template roles, a failure branch and a seasonal timer.
- **Loading.** `quest.LoadQuests(root, cfg, pack)` (`internal/battle/quest/load.go`) reads every set folder strictly, checks each quest against the vocabulary, the expedition config (`expedition.yaml`: the calendar and config keys) and the region pack (`content/expedition_v1/`), and refuses the whole catalog on the first problem with an error naming the file and the field, for example `quest content: quests_v1/paper/spring_farms.yaml: stages[0].timer.before: a seasonal timer needs a season of the calendar, not "monsoon"`. It only reads.
- **Viewing.** The read-only runtime op `quest_catalog` (`internal/battle/learned/quests.go`) loads the config, the pack and the catalog from the content root (never a save) and returns the described graphs. Character Creation's **Quests** tab opens the viewer (`app/screens/quests/quest_graph_viewer.gd`).

## Fields

### `set.yaml`

| Field | Required | Meaning |
| --- | --- | --- |
| `schema_version` | yes | `1` |
| `id` | yes | The folder's name. |
| `name`, `text` | yes | Shown in the viewer's list. |
| `order` | yes | Its place in the catalog (1 and up, unique); the Salt Marches are 1, the paper quests 2. |

### A quest

| Field | Required | Meaning |
| --- | --- | --- |
| `schema_version` | yes | `1` |
| `id` | yes | snake_case, unique across every set, and the file's name. |
| `name` | yes | The quest's name. |
| `set` | yes | The folder's set id. |
| `kind` | yes | `main`, `side`, `small` or `personal`. |
| `status` | yes | `scenario` (describes rules the Go code runs today; real content ids and no roles) or `paper` (design only; roles allowed; every number a placeholder, marked with a YAML comment). |
| `region` | no | A region id from the pack. |
| `roles` | no | Template roles: `name: {type, text}`, `type` one of `person`, `group`, `site`, `town`. Paper quests only. |
| `hook` | yes | One hook call: how the hero learns of it. |
| `trigger` | yes | A list of trigger calls: the world conditions that start it, all of which must hold. |
| `start` | yes | The first stage's id. |
| `stages` | yes | The stages (at least one). |

### A stage

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | yes | snake_case, unique within the quest, not `end`. |
| `title` | no | A short title; the objective's description when absent. |
| `text` | no | The stage's story text. |
| `objective` | yes | One objective call. |
| `timer` | no | `{type: none}` (or absent); `{type: urgent, rounds: N or {config: key}, from: quest_start or stage_start}`; `{type: seasonal, before: <season id>}`. Rounds are world-clock halves: one round is the Day or the Night half of a month (see [expedition.md §5](expedition.md#the-world-clock-and-the-calendar)). |
| `outcomes` | yes | The ways the stage ends (a list). |

### An outcome

| Field | Required | Meaning |
| --- | --- | --- |
| `outcome` | yes | `success`, `failure` or `abandoned`. |
| `when` | see below | One cause call: what makes this outcome happen. |
| `label` | no | The edge's short label in the graph; the cause or the outcome type when absent (an event choice labels its edge with the choice's text). |
| `text` | no | The outcome's story text (an event outcome shows the event file's text). |
| `consequences` | no | A list of consequence calls. |
| `next` | yes | The next stage's id, or `end`. A failure branch is just another stage. |

**Causes.** An outcome with no `when` has the default cause: a success happens when the objective is met, a failure when the stage's timer runs out. So a failure without a `when` needs a timer, an abandoned outcome always needs a `when`, and two outcomes of one type need different causes. The scenario's event quests have one success per event choice, each with its own `when: {verb: event_choice, …}`; the main quest has two failures (losing to the Regent, and the timer running out).

## Values: ids, roles, config references, text

A call is a YAML map: `{verb: <name>, <parameter>: <value>, …}`. A value is one of:

- **A content id.** A group, site, region, event, event choice or gear id from the region pack; a season or month id from `calendar` in `expedition.yaml`; a quest id in the catalog. The validator resolves every one. A `town` must be a village or town site. People have no content yet, so a person is always a role.
- **`{role: <name>}`.** A template role the quest declares; generated worlds fill it when they are built ("rescue {person} from {captor} at {site}"). The role's type must fit the parameter (a `group` parameter takes a group role). Hand-made scenario quests use real ids and no roles.
- **A number, or `{config: <key>}`.** `key` is a dotted path in `expedition.yaml` (`rounds_per_region`, `warlord.early_reward.xp_per_round`) and must name a number. A `status` parameter takes `{config: statuses.<name>}`, the status id the config names.
- **Text.** `{name}` placeholders in any text (hook text, stage text, role text, labels) must be declared roles; a scenario quest has none.
- **An item** is a gear id; a paper quest may also name a snake_case placeholder item (`moonpetal_draught`).
- **Calendar values.** `in: [autumn, winter]` (seasons or months); `before: spring` (a season); `until: next_season`, `next_month`, or a season or month id.

## The vocabulary

Every hook, trigger, objective, cause and consequence is a verb from this fixed list (`internal/battle/quest/vocabulary.go`). `?` marks an optional parameter. **Engine** is the verb's engine status: *scenario* means today's Go scenario code runs the rule the verb describes; *not yet* means nothing implements it yet. The describer and the graph view show it as a badge ("Run by today's scenario code" / "Not yet implemented"), so nobody mistakes a paper verb for a working one. Timer types have one too: `none` and `urgent` (the region's deadline) are the scenario's, `seasonal` is not yet.

Beyond the handoff's starting list the vocabulary adds the **causes** (`when`: `timer_expired`, `event_choice`, `lose_battle`, plus `defeat_group` and `quest_outcome` from the other parts), which say which edge a stage takes, and two consequences the Salt Marches need, `mend_wounds` (winning the region mends every wound) and `win_region` (today a won region ends in victory; `open_world` is the plan's later step). `prosperity` takes `at_least` as a trigger and `amount` as a consequence.

### Hooks: how the hero learns of the quest (`hook`, one call)

| Verb | Parameters | Engine | Meaning |
| --- | --- | --- | --- |
| `event` | `event` (event) | scenario | The hero comes upon an event scene (the hook shows the event's text). |
| `quest_failed` | `quest` (quest) | not yet | Another quest failed. |
| `raid` | `town` (town id or role) | not yet | An enemy raid on a town. |
| `request` | `from` (person/site/town id or role), `text?` (text) | not yet | Someone asks the hero for help. |
| `rumour` | `text` (text) | not yet | The hero hears a rumour. |
| `scenario_start` | `region?` (region) | scenario | Part of a scenario from its start (the hook shows the region's description). |

### Triggers: world conditions that start it (`trigger`, a list; all must hold)

| Verb | Parameters | Engine | Meaning |
| --- | --- | --- | --- |
| `month` | `in` (months) | not yet | The world clock is in one of these months. |
| `prosperity` | `town` (town id or role), `at_least` (number or config) | not yet | A town's prosperity has reached a level (trigger), or changes (consequence). |
| `quest_outcome` | `quest` (quest), `outcome?` (outcome) | not yet | Another quest has ended (with this outcome, if given). |
| `reputation_tier` | `with` (person/town id or role), `at_least` (tier) | not yet | A standing has reached a reputation tier. |
| `scenario_start` | `region?` (region) | scenario | Part of a scenario from its start (the hook shows the region's description). |
| `season` | `in` (seasons) | not yet | The world clock is in one of these seasons. |
| `site_state` | `site` (site/town id or role), `state` (site_flag) | scenario | A site is (trigger) or becomes (consequence) seen, freed, cleared, resolved or conquered. |

### Objectives: what a stage asks (`objective`)

| Verb | Parameters | Engine | Meaning |
| --- | --- | --- | --- |
| `be_at_by` | `site` (site/town id or role), `by` (date) | not yet | Be at a place by a date. |
| `clear_dungeon` | `site` (site id or role) | not yet | Clear a dungeon. |
| `defeat_group` | `group` (group id or role), `at?` (site/town id or role) | scenario | Beat an enemy group in battle (as a cause: the group is beaten). |
| `deliver_item` | `item` (item), `to` (person/group/town id or role) | not yet | Carry an item to someone (it sits in the deck as a card that counts as health). |
| `liberate_site` | `site` (site/town id or role) | scenario | Free a site held by enemies (its occupiers or garrison). |
| `reach_site` | `site` (site/town id or role) | not yet | Reach a site. |
| `resolve_event` | `site` (site id or role) | scenario | Play an event site's scene through one of its choices. |
| `spend_influence` | `amount` (number ≥ 1 or config), `at?` (person/site/town id or role) | not yet | Spend Influence with someone or somewhere. |
| `survey_tiles` | `count` (number ≥ 1 or config), `region?` (region) | not yet | Explore a number of tiles. |

### Causes: what makes an outcome happen (`when`)

| Verb | Parameters | Engine | Meaning |
| --- | --- | --- | --- |
| `defeat_group` | `group` (group id or role), `at?` (site/town id or role) | scenario | Beat an enemy group in battle (as a cause: the group is beaten). |
| `event_choice` | `event` (event), `choice` (choice) | scenario | One choice of an event scene is taken. |
| `lose_battle` | `group` (group id or role) | scenario | The hero loses a battle against a group. |
| `quest_outcome` | `quest` (quest), `outcome?` (outcome) | not yet | Another quest has ended (with this outcome, if given). |
| `timer_expired` | none | scenario | The stage's timer runs out (the default cause of a failure in a timed stage). |

### Consequences: what an outcome does (`consequences`, a list)

| Verb | Parameters | Engine | Meaning |
| --- | --- | --- | --- |
| `config_reward` | `key` (config), `reward` (xp \| relic_choice), `per` (unused_round \| early) | scenario | A reward whose size is a number in expedition.yaml. |
| `end_run` | none | scenario | The run is over. |
| `event_outcome` | `event` (event), `choice` (choice) | scenario | What an event choice does, as the event file says. |
| `gear` | `item` (item) | scenario | Grant an item. |
| `gold` | `amount` (number or config) | scenario | Grant gold. |
| `mend_wounds` | `amount` (amount) | scenario | Heal damage from the hero's wounds. |
| `open_world` | none | not yet | The open world begins. |
| `prosperity` | `town` (town id or role), `amount` (number or config) | not yet | A town's prosperity has reached a level (trigger), or changes (consequence). |
| `remove_enemy_status` | `group` (group id or role), `status` (status), `stacks` (number ≥ 1 or config) | scenario | Take stacks of a status from an enemy. |
| `reputation` | `with` (person/town id or role), `amount` (number or config) | not yet | Change a standing. |
| `site_state` | `site` (site/town id or role), `state` (site_flag) | scenario | A site is (trigger) or becomes (consequence) seen, freed, cleared, resolved or conquered. |
| `spawn` | `group` (group id or role), `from?` (person/group id or role), `at?` (site/town id or role) | not yet | A new group appears. |
| `start_quest` | `quest` (quest) | not yet | Start another quest. |
| `town_condition` | `town` (town id or role), `condition` (ident), `text` (text), `until` (until) | not yet | A town suffers (or enjoys) a condition until a point in time. |
| `win_region` | `region?` (region) | scenario | The region is won. |
| `xp` | `amount` (number or config) | scenario | Grant XP. |

## Refer, don't copy

When a number or text already lives in `expedition.yaml` or the region pack, a quest refers to it (a config key, a group, site or event id, an event choice id) instead of repeating it, and the validator resolves every reference. This keeps the scenario files from drifting when the scenario is tuned:

- the main quest's timer is `{config: rounds_per_region}` and its early-strike rewards are `config_reward` with `warlord.early_reward.xp_per_round` and `extra_relic_choices`;
- the Lieutenant quests remove `{config: statuses.lieutenant}`;
- the event quests' edges are the event's choices (`event_choice`) and their consequences the choices' outcomes (`event_outcome`); the choice texts, rolls, gold, XP and wounds stay in the event files;
- the hooks show the region's description and the event's text.

A fight's own XP and gold belong to the enemy group, not to the quest, so quest files don't repeat them. Paper quests have no existing source, so their numbers are written in place and marked `# placeholder`.

`TestSaltMarchesQuestsMatchScenario` ties the scenario quests to the live content: the main quest's timer resolves to `rounds_per_region`; the Lieutenant quests cover exactly the groups with `lieutenant: true`; the liberation quests cover exactly the sites with `occupied_by`; the event quests cover every event site and every choice of each event. Adding a Lieutenant or an event choice to the scenario fails the test until the quest files follow.

## The validator

`LoadQuests` refuses (one small invalid fixture per rule in `internal/battle/quest/testdata/invalid/`, each naming the error it must produce):

- unknown fields (the YAML is decoded strictly), unknown verbs, verbs used out of their part, unknown parameters, missing required fields and parameters, a `schema_version` other than 1;
- duplicate quest ids (across sets), duplicate stage ids, a file name or set that does not match the id or folder, a set without its `set.yaml` fields;
- a `start` or `next` that names no stage; a stage no path from the start reaches; a cycle (**no cycles in Phase 1; a retry is a new stage**);
- a stage with no `success` outcome; a stage with a timer but no `failure` outcome (a failure may simply lead to `end`); a failure without a timer or a `when`; an abandoned outcome without a `when`; two outcomes of one type with the same cause; `timer_expired` in a stage with no timer;
- a reference to an unknown region, site, group, event, event choice, gear (scenario quests), season, month, quest or config key, or a `config` reference whose value isn't a number;
- a role used but not declared, declared but never used, of the wrong type for its parameter, or declared on a `status: scenario` quest; a `{placeholder}` that is not a role;
- an `urgent` timer with no rounds or no `from`, a `seasonal` timer with an unknown season;
- calls the content contradicts: liberating a site no one holds, resolving a site that is not an event site, a group `at` a site that does not hold it, an event choice or outcome of another event than the stage resolves, a reward per unused round in a stage without an urgent timer.

## The describer and the graph view

`quest.Describe` turns the catalog into what the viewer shows; the client never interprets verbs. Each quest has its header (id, name, kind, status, roles, hook, triggers) and a graph:

- **nodes**: the stages (title, story text, objective text, timer chip and text, engine badges, depth = the longest path from the start), plus one end node per ending ("Complete", "Failed", "Abandoned"), one column after its stage, so branches stack instead of converging;
- **edges**: from a stage to the next stage or its end node, with the outcome type, the label, the cause's text and the consequences' texts, each with its engine status.

Texts name content from the pack and show config values with their keys: "Defeat the Bell Marshal (a Lieutenant)", "Urgent: 4 rounds (rounds_per_region) from the quest's start", "+30 XP per unused round (warlord.early_reward.xp_per_round)", "Remove 1 stack of Lieutenant's Favor (statuses.lieutenant) from the Drowned Regent". Every list is a list, never null.

The viewer lists the quests by set with kind and status chips; the selected quest's stages sit in columns by depth, left to right, its edges coloured by outcome (success green, failure red, abandoned grey) with their labels on the segment that enters the next card; the graph scrolls when it is larger than its area. Hovering or clicking a stage shows its full text, its outcomes and their consequences, with red notes on anything not yet implemented. It is read-only: no editing, no saving.

## Examples

The main quest of the Salt Marches. The urgent timer's failure branches into the deadline fight, as the Go rules run it; losing to the Regent at any time ends the run.

```yaml
schema_version: 1
id: stop_the_drowned_regent
name: Stop the Drowned Regent
set: salt_marches
kind: main
status: scenario
region: salt_marches
# The hook's text is the region's description.
hook: {verb: scenario_start, region: salt_marches}
trigger:
  - {verb: scenario_start, region: salt_marches}
start: stop_him
stages:
  - id: stop_him
    title: Stop him before the last night ends
    objective: {verb: defeat_group, group: drowned_regent}
    # Run Him Down (extend_time) can add a round to this deadline.
    timer: {type: urgent, rounds: {config: rounds_per_region}, from: quest_start}
    outcomes:
      - outcome: success
        label: Caught early
        text: "The Regent falls before his time runs out."
        consequences:
          - {verb: config_reward, key: warlord.early_reward.xp_per_round, reward: xp, per: unused_round}
          - {verb: config_reward, key: warlord.early_reward.extra_relic_choices, reward: relic_choice, per: early}
          - {verb: mend_wounds, amount: all}
          - {verb: win_region, region: salt_marches}
          - {verb: open_world}
        next: end
      - outcome: failure
        label: Lost to him
        when: {verb: lose_battle, group: drowned_regent}
        text: "Losing to the Warlord ends the run."
        consequences:
          - {verb: end_run}
        next: end
      - outcome: failure
        label: Time runs out
        when: {verb: timer_expired}
        text: "The last round is over: the Warlord comes for you."
        next: deadline
  - id: deadline
    title: The deadline fight
    text: "The Warlord comes to you, in the open: no Ambush, every buff he still has."
    objective: {verb: defeat_group, group: drowned_regent}
    outcomes:
      - outcome: success
        label: Beaten at the deadline
        text: "The Regent falls; no round was left unused."
        consequences:
          - {verb: mend_wounds, amount: all}
          - {verb: win_region, region: salt_marches}
          - {verb: open_world}
        next: end
      - outcome: failure
        label: Lost the deadline fight
        when: {verb: lose_battle, group: drowned_regent}
        consequences:
          - {verb: end_run}
        next: end
```

The kidnapped daughter: template roles, an urgent timer counted from the stage's start, and a failure branch. When the rescue runs out of time she turns, and the quest continues in a new stage with its own success (cure her) and failure (hunt the beast). The sixth role, `beast`, is the werewolf the failure branch spawns.

```yaml
schema_version: 1
id: kidnapped_daughter
name: The Kidnapped Daughter
set: paper
kind: personal
status: paper
# A template: the roles are filled when a world is built. Every number here
# is a placeholder.
roles:
  person:  {type: person, text: "Her father, who asks for help."}
  captive: {type: person, text: "The daughter who was taken."}
  captor:  {type: group, text: "The group that took her."}
  site:    {type: site, text: "The captor's lair."}
  town:    {type: town, text: "The father's town."}
  beast:   {type: group, text: "{captive} as a werewolf, once she has turned; the failure branch spawns it."}
hook: {verb: request, from: {role: person}, text: "{person} begs you to bring {captive} home from {site}."}
trigger:
  - {verb: reputation_tier, with: {role: person}, at_least: neutral}   # placeholder tier
start: rescue
stages:
  - id: rescue
    title: Rescue her
    text: "{captor} hold {captive} at {site}."
    objective: {verb: defeat_group, group: {role: captor}, at: {role: site}}
    timer: {type: urgent, rounds: 3, from: stage_start}   # placeholder: 3 rounds
    outcomes:
      - outcome: success
        label: Returned home
        text: "{captive} is home, and {town} hears of it."
        consequences:
          - {verb: reputation, with: {role: person}, amount: 2}   # placeholder
          - {verb: prosperity, town: {role: town}, amount: 1}     # placeholder
        next: end
      - outcome: failure
        label: Too late
        text: "You came too late: {captive} has turned."
        consequences:
          - {verb: spawn, group: {role: beast}, from: {role: captive}, at: {role: site}}
        next: turned
  - id: turned
    title: She has turned
    text: "{captive} roams near {site} as a werewolf. A rare reagent could still cure her."
    objective: {verb: deliver_item, item: moonpetal_draught, to: {role: beast}}   # placeholder item
    outcomes:
      - outcome: success
        label: Cure her
        text: "The draught takes hold and {captive} wakes as herself."
        consequences:
          - {verb: xp, amount: 40}                                 # placeholder
          - {verb: reputation, with: {role: person}, amount: 4}    # placeholder
          - {verb: prosperity, town: {role: town}, amount: 1}      # placeholder
        next: end
      - outcome: failure
        label: Hunt the beast
        when: {verb: defeat_group, group: {role: beast}}
        text: "The beast is dead. {town} mourns, and {person} will not forgive you."
        consequences:
          - {verb: prosperity, town: {role: town}, amount: -1}     # placeholder
          - {verb: reputation, with: {role: person}, amount: -4}   # placeholder: hostile
        next: end
```

The spring farms: a seasonal trigger, a seasonal timer and a plain failure with a town condition until the next season.

```yaml
schema_version: 1
id: spring_farms
name: Clear the Spring Farms
set: paper
kind: side
status: paper
# A template: the roles are filled when a world is built. Every number here
# is a placeholder.
roles:
  town:        {type: town, text: "The town the farms feed."}
  farms:       {type: site, text: "The farms outside {town}."}
  infestation: {type: group, text: "The vermin that overran {farms}."}
hook: {verb: request, from: {role: farms}, text: "The farmers of {farms} ask you to clear their fields before the spring sowing."}
trigger:
  - {verb: season, in: [autumn, winter]}
start: clear
stages:
  - id: clear
    title: Clear the farms
    objective: {verb: defeat_group, group: {role: infestation}, at: {role: farms}}
    timer: {type: seasonal, before: spring}
    outcomes:
      - outcome: success
        label: Fields sown
        text: "The fields are clear in time for the sowing."
        consequences:
          - {verb: prosperity, town: {role: town}, amount: 1}   # placeholder
        next: end
      - outcome: failure
        label: Spring comes
        text: "Spring comes and the fields lie bare."
        consequences:
          - {verb: town_condition, town: {role: town}, condition: food_shortage, text: "Food shortage: prices rise", until: next_season}
        next: end
```

## Adding to the format

- **A new verb** goes into `Vocabulary` with its parts, typed parameters and engine status; never special-case code for one quest. Document it in the table above.
- **A new parameter kind** needs its check in `checker.value` and its text in the describer.
- **Running quests** (Phase 5) will move a verb's engine status from *not yet* to implemented as the quest runtime learns it; timers will count the same world clock as the region's deadline.
