package learned

import (
	"bytes"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

func handleCardTrees(r runtimeRequest) string {
	catalogs, err := CharacterCatalogs(r.ContentRoot, r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	lib := catalogs["adventurer"]
	saved, err := content.ReadAuthoredCards(r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	if r.Op == "card_trees" {
		if r.Character == "" {
			r.Character = "adventurer"
		}
		if _, ok := catalogs[r.Character]; !ok {
			return runtimeError(fmt.Errorf("unknown character"))
		}
		economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		progress, economy, _, err := loadout.ProgressSnapshot(r.LoadoutRoot, economy, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		p := progress[r.Character]
		offers := map[string]any{}
		equipment := map[string]any{}
		for id, tree := range saved.Trees {
			choices := map[string]any{}
			for _, n := range tree.Nodes {
				_, why := loadout.MoveCollectionCard(p, n.Card.ID, true, economy, catalogs[r.Character], r.Character)
				reason := ""
				if why != nil {
					reason = why.Error()
				}
				equipment[n.Card.ID] = map[string]any{"available": why == nil, "reason": reason}
			}
			for _, edge := range tree.Edges {
				a, _ := tree.Node(edge.From)
				b, _ := tree.Node(edge.To)
				for reverse := 0; reverse < 2; reverse++ {
					if reverse == 1 {
						if !edge.Reversible {
							continue
						}
						a, b = b, a
					}
					_, cost, why := loadout.UpgradeTreeCard(p, a.Card.ID, b.Card.ID, economy, catalogs[r.Character], r.Character)
					if why == nil && cost > p.XP {
						why = fmt.Errorf("not enough XP")
					}
					reason := ""
					if why != nil {
						reason = why.Error()
					}
					key := edge.ID
					if reverse == 1 {
						key += ":back"
					}
					choices[key] = map[string]any{"from": a.ID, "to": b.ID, "from_card": a.Card.ID, "to_card": b.Card.ID, "cost": cost, "available": why == nil, "reason": reason}
				}
			}
			offers[id] = choices
		}
		trees := saved.Trees
		if trees == nil {
			trees = map[string]content.CardTree{}
		}
		return runtimeSuccess(map[string]any{"trees": trees, "revision": saved.Revision, "progression": p, "offers": offers, "equipment": equipment})
	}
	var raw any
	if err = json.Unmarshal(r.CardTree, &raw); err != nil {
		return runtimeError(err)
	}
	data, _ := json.Marshal(raw)
	var tree content.CardTree
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&tree); err != nil {
		return runtimeError(err)
	}
	if r.CatalogRevision != saved.Revision {
		return runtimeError(fmt.Errorf("catalog changed; reload before publishing this tree"))
	}
	next, _, err := content.PreviewCardTree(saved, lib, tree)
	if err != nil {
		return runtimeError(err)
	}
	for id, candidate := range catalogs {
		_, updated, previewErr := content.PreviewCardTree(saved, candidate, tree)
		if previewErr != nil {
			return runtimeError(previewErr)
		}
		catalogs[id] = updated
	}
	economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
	if err != nil {
		return runtimeError(err)
	}
	if err = loadout.ValidateCatalogUpdate(r.LoadoutRoot, economy, catalogs); err != nil {
		return runtimeError(err)
	}
	if r.Op == "publish_card_tree" {
		root, _ := filepath.Abs(r.LoadoutRoot)
		source, _ := filepath.Abs(r.ContentRoot)
		admin, ok := cardAdmins[r.AdminToken]
		if !ok || admin.Root != root || admin.Content != source || time.Now().After(admin.Expires) {
			return runtimeError(fmt.Errorf("publishing a tree requires an active Admin settings session"))
		}
		next, err = content.SaveCardTree(r.LoadoutRoot, lib, tree, r.CatalogRevision)
		if err != nil {
			return runtimeError(err)
		}
	}
	return runtimeSuccess(map[string]any{"tree": next.Trees[tree.ID], "revision": next.Revision})
}
