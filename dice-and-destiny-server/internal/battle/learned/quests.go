package learned

import (
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/quest"
)

// questCatalog is the read-only quest_catalog op: it loads the expedition
// config, the region pack and the quest files from the content root and
// returns the described sets and quest graphs. It reads only content, never a
// save or the loadout root, and nothing it returns feeds an expedition rule.
func questCatalog(contentRoot string) (quest.CatalogView, error) {
	catalogs, err := CharacterCatalogs(contentRoot)
	if err != nil {
		return quest.CatalogView{}, err
	}
	// Every character library holds the same content packs (statuses,
	// combatants); any one validates the config and the pack.
	ids := make([]string, 0, len(catalogs))
	for id := range catalogs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return quest.CatalogView{}, fmt.Errorf("no character library to check the quests against")
	}
	lib := catalogs[ids[0]]
	rules, err := loadout.ExpeditionRules(contentRoot, lib)
	if err != nil {
		return quest.CatalogView{}, err
	}
	cat, err := quest.LoadQuests(contentRoot, rules.Config, rules.Pack)
	if err != nil {
		return quest.CatalogView{}, err
	}
	names := map[string]string{}
	for id, s := range lib.Statuses {
		names[id] = s.Name
	}
	return quest.Describe(cat, rules.Config, rules.Pack, names)
}
