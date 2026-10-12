package expedition

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHexGeometry(t *testing.T) {
	if Distance(Hex{0, 0}, Hex{2, 1}) != 3 || Distance(Hex{1, -1}, Hex{1, -1}) != 0 || Distance(Hex{0, 0}, Hex{1, 1}) != 2 {
		t.Fatal("hex distance is wrong")
	}
	// The six flower offsets tile the plane: adjacent flowers never share a
	// hex and every offset is on the lattice.
	center := TileHexes(Hex{})
	for _, o := range FlowerOffsets {
		if !IsFlowerCenter(o) {
			t.Fatalf("%v is not on the flower lattice", o)
		}
		for _, a := range center {
			for _, b := range TileHexes(o) {
				if a == b {
					t.Fatalf("flowers at the origin and %v overlap at %v", o, a)
				}
			}
		}
		touching := false
		for _, a := range center {
			for _, b := range TileHexes(o) {
				touching = touching || Adjacent(a, b)
			}
		}
		if !touching {
			t.Fatalf("flowers at the origin and %v do not touch", o)
		}
	}
	if IsFlowerCenter(Hex{1, 0}) || IsFlowerCenter(Hex{2, 0}) {
		t.Fatal("ordinary hexes are not flower centres")
	}
	line := Line(Hex{0, 0}, Hex{4, 2})
	if len(line) != 7 || line[0] != (Hex{0, 0}) || line[6] != (Hex{4, 2}) {
		t.Fatalf("line from the origin to (4,2) has 7 hexes, got %v", line)
	}
	for i := 1; i < len(line); i++ {
		if !Adjacent(line[i-1], line[i]) {
			t.Fatalf("line step %d is not adjacent: %v", i, line)
		}
	}
}

func TestLoadPackAcceptsShippedRegion(t *testing.T) {
	cfg, pack, _ := loadShipped(t)
	region, ok := pack.Regions["salt_marches"]
	if !ok {
		t.Fatal("the shipped region is salt_marches")
	}
	if len(region.CountrysideTiles) != 5 || len(region.CoreTiles) != 2 {
		t.Fatalf("the first region is start + 5 countryside + 2 core, got %d/%d", len(region.CountrysideTiles), len(region.CoreTiles))
	}
	tiles := append(append([]string{region.StartTile}, region.CountrysideTiles...), region.CoreTiles...)
	siteTypes := map[string]int{}
	terrains := map[string]bool{}
	var keepLieutenant, roamingLieutenant, occupiedVillage, townOnCore bool
	groupsSeen := map[string]bool{}
	for _, id := range tiles {
		tile := pack.Tiles[id]
		for _, h := range tile.Hexes {
			terrains[h.Terrain] = true
			if h.Group != "" {
				g := pack.Groups[h.Group]
				groupsSeen[g.Combatant+"x"+string(rune('0'+g.Count))] = true
				if g.Lieutenant && len(g.Patrol) > 0 {
					roamingLieutenant = true
				}
			}
			if h.Site != "" {
				site := pack.Sites[h.Site]
				siteTypes[site.Type]++
				if site.Type == "keep" && pack.Groups[site.Garrison].Lieutenant {
					keepLieutenant = true
				}
				if site.Type == "village" && site.OccupiedBy != "" {
					occupiedVillage = true
				}
				if site.Type == "town" && tile.Kind == "core" && region.Warlord.GoalSite == site.ID {
					townOnCore = true
				}
				if site.Guardian != nil {
					groupsSeen[pack.Groups[site.Guardian.Day].Combatant+"x"+string(rune('0'+pack.Groups[site.Guardian.Day].Count))] = true
				}
			}
		}
	}
	for site, n := range map[string]int{"village": 2, "monastery": 1, "town": 1, "ruins": 2, "grotto": 1, "event": 3, "keep": 1} {
		if siteTypes[site] < n {
			t.Fatalf("the region needs %d %s sites, has %d", n, site, siteTypes[site])
		}
	}
	for _, terrain := range []string{"lake", "mountain", "marsh", "forest", "hills", "plains"} {
		if !terrains[terrain] {
			t.Fatalf("the region needs %s terrain", terrain)
		}
	}
	for _, g := range []string{"drowned_oracle_brine_maskx1", "drowned_oracle_brine_maskx2", "drowned_oracle_bell_diverx1", "drowned_oracle_ribbon_eelx1"} {
		if !groupsSeen[g] {
			t.Fatalf("the region needs a %s group", g)
		}
	}
	habits := map[string]bool{}
	for _, g := range pack.Groups {
		habits[g.Habit] = true
	}
	for _, habit := range Habits {
		if !habits[habit] {
			t.Fatalf("every habit must appear; %s is missing", habit)
		}
	}
	if !keepLieutenant || !roamingLieutenant || !occupiedVillage || !townOnCore {
		t.Fatalf("keep lieutenant %v, roaming lieutenant %v, occupied village %v, goal town on core %v", keepLieutenant, roamingLieutenant, occupiedVillage, townOnCore)
	}
	if w := pack.Groups[region.Warlord.Group]; !w.Warlord || w.Combatant != "drowned_regent" {
		t.Fatalf("the placeholder Warlord is drowned_regent: %+v", w)
	}
	for _, id := range []string{"bandage", "lantern", "siege_hooks"} {
		if _, ok := pack.Gear[id]; !ok {
			t.Fatalf("gear %s is missing", id)
		}
	}
	// Events cover gold, XP, a wound or fight penalty, an Influence choice and a roll.
	var gold, xp, penalty, influence, roll bool
	for _, e := range pack.Events {
		for _, c := range e.Choices {
			influence = influence || c.Requires.Influence > 0
			roll = roll || c.Roll != nil
			outcomes := []Outcome{}
			if c.Outcome != nil {
				outcomes = append(outcomes, *c.Outcome)
			}
			if c.Roll != nil {
				outcomes = append(outcomes, c.Roll.Success, c.Roll.Failure)
			}
			for _, o := range outcomes {
				gold = gold || o.Gold > 0
				xp = xp || o.XP > 0
				penalty = penalty || o.Wounds > 0 || o.Fight != "" || o.Status != ""
			}
		}
	}
	if !gold || !xp || !penalty || !influence || !roll || len(pack.Events) < 3 {
		t.Fatalf("events must cover gold %v, xp %v, penalty %v, influence %v, roll %v with 3 scenes (%d)", gold, xp, penalty, influence, roll, len(pack.Events))
	}
	_ = cfg
}

func TestLoadPackRejectsBadContent(t *testing.T) {
	root := contentRoot(t)
	lib := testLibrary(t, root)
	cases := []struct {
		name string
		file string
		old  string
		new  string
		want string
	}{
		{"unknown tile field", "tiles/reed_shallows.yaml", "kind: countryside", "kind: countryside\nweather: rain", "field weather not found"},
		{"tile schema", "tiles/reed_shallows.yaml", "schema_version: 1", "schema_version: 3", "needs schema_version 1"},
		{"tile with six hexes", "tiles/reed_shallows.yaml", "  - {offset: [0, 1], terrain: hills}\n", "", "needs exactly 7 hexes"},
		{"tile duplicate offset", "tiles/reed_shallows.yaml", "offset: [0, 1]", "offset: [1, 0]", "once"},
		{"tile unknown terrain", "tiles/reed_shallows.yaml", "terrain: hills", "terrain: tundra", "unknown terrain \"tundra\""},
		{"tile unknown site", "tiles/reed_shallows.yaml", "site: shrine_site", "site: lost_shrine", "unknown site \"lost_shrine\""},
		{"tile unknown group", "tiles/reed_shallows.yaml", "group: brine_lone", "group: kraken", "unknown group \"kraken\""},
		{"group on a lake", "tiles/reed_shallows.yaml", "terrain: marsh, group: brine_lone", "terrain: lake, group: brine_lone", "impassable terrain"},
		{"warlord on a tile", "tiles/reed_shallows.yaml", "group: brine_lone", "group: drowned_regent", "places the Warlord"},
		{"site unknown gear", "sites/hollow_ford.yaml", "shop: [\"bandage\"]", "shop: [\"elixir\"]", "unknown gear \"elixir\""},
		{"site unknown event", "sites/shrine_site.yaml", "event: wayside_shrine", "event: lost_event", "needs a known event"},
		{"keep without garrison", "sites/brinehold_keep.yaml", "garrison: eel_warden", "", "needs a garrison"},
		{"ruins without guardian", "sites/drowned_cairn.yaml", "guardian: {\"day\": \"brine_lone\", \"night\": \"brine_pair\"}\n", "", "needs a guardian and loot"},
		{"loot outside treasure range", "sites/drowned_cairn.yaml", "\"night\": [30, 40]", "\"night\": [30, 90]", "within gold.drops.treasure"},
		{"group unknown combatant", "groups/brine_lone.yaml", "combatant: drowned_oracle_brine_mask", "combatant: adventurer", "needs a scripted minion combatant"},
		{"group count three", "groups/brine_lone.yaml", "count: 1", "count: 3", "count must be 1 or 2"},
		{"group bad habit", "groups/brine_lone.yaml", "habit: unwary", "habit: cautious", "unknown habit"},
		{"group gold outside range", "groups/brine_lone.yaml", "gold: 8", "gold: 50", "outside gold.drops.minion_group"},
		{"lieutenant gold", "groups/eel_warden.yaml", "gold: 30", "gold: 10", "outside gold.drops.lieutenant"},
		{"gear price outside range", "gear/lantern.yaml", "price: 40", "price: 5", "outside gold.prices.gear"},
		{"gear unknown effect", "gear/lantern.yaml", "effect: night_sight", "effect: fly", "unknown effect"},
		{"event unknown fight", "events/wrecked_barge.yaml", "fight: brine_lone", "fight: leviathan", "unknown fight group"},
		{"event duplicate choice id", "events/wayside_shrine.yaml", "  - id: take_silver", "  - id: offering", "unique snake_case id"},
		{"event gold outside range", "events/wayside_shrine.yaml", "gold: 15", "gold: 150", "outside gold.drops.event"},
		{"event roll target", "events/wrecked_barge.yaml", "needed: 4", "needed: 9", "target of 1–6"},
		{"region unknown tile", "regions/salt_marches.yaml", "reed_shallows, cairn_hills", "reed_shallows, salt_flats", "must be a countryside tile used once"},
		{"region slot off lattice", "regions/salt_marches.yaml", "center: [2, 1]", "center: [2, 0]", "not on the tile lattice"},
		{"region slot count", "regions/salt_marches.yaml", "  - {id: west, center: [-3, 2], kind: countryside}\n", "", "one slot per tile"},
		{"region disconnected slot", "regions/salt_marches.yaml", "center: [7, 0]", "center: [-6, -3]", "no path of tiles reaches"},
		{"region goal not core", "regions/salt_marches.yaml", "goal_tile: greywharf_strand", "goal_tile: reed_shallows", "must be one of its core tiles"},
		{"region goal site elsewhere", "regions/salt_marches.yaml", "goal_site: greywharf", "goal_site: hollow_ford", "on the goal tile"},
		{"region warlord not warlord", "regions/salt_marches.yaml", "group: drowned_regent", "group: brine_lone", "must be a Warlord group"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			scratch := scratchContent(t)
			rewrite(t, filepath.Join(scratch, "expedition_v1", c.file), replaceOnce(c.old, c.new))
			cfg, err := LoadConfig(scratch, lib)
			if err != nil {
				t.Fatal(err)
			}
			_, err = LoadPack(scratch, cfg, lib)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
}

// Two open groups must never share a touching hex, whatever tiles land next
// to each other. A group off a tile's centre can touch a neighbouring tile's
// centre group in some placement, so the loader refuses it.
func TestLoadPackRejectsTouchingGroups(t *testing.T) {
	lib := testLibrary(t, contentRoot(t))
	scratch := scratchContent(t)
	// Move Reed Shallows' Brine Mask from the centre to its east hex.
	rewrite(t, filepath.Join(scratch, "expedition_v1", "tiles", "reed_shallows.yaml"), func(s string) string {
		s = strings.Replace(s, "terrain: marsh, group: brine_lone}", "terrain: marsh}", 1)
		return strings.Replace(s, "{offset: [1, 0], terrain: plains}", "{offset: [1, 0], terrain: plains, group: brine_lone}", 1)
	})
	cfg, err := LoadConfig(scratch, lib)
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadPack(scratch, cfg, lib)
	if err == nil || !strings.Contains(err.Error(), "one hex touches both") {
		t.Fatalf("a ring-hex group was accepted: %v", err)
	}
	// Within one tile the same rule applies: a patrol that comes within two
	// hexes of another group is refused.
	scratch = scratchContent(t)
	rewrite(t, filepath.Join(scratch, "expedition_v1", "tiles", "saltreed_fen.yaml"), replaceOnce("{offset: [1, -1], terrain: marsh}", "{offset: [1, -1], terrain: marsh, group: brine_lone}"))
	_, err = LoadPack(scratch, cfg, lib)
	if err == nil || !strings.Contains(err.Error(), "one hex touches both") {
		t.Fatalf("two groups on one tile were accepted: %v", err)
	}
}
