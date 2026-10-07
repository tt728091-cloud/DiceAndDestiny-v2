package learned

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

type cardAdminSession struct {
	Root, Content string
	Expires       time.Time
}

// This desktop game has workspace-local Admin Settings, not remote accounts.
// Only that workflow requests this short-lived capability. Card publication,
// ordinary deck edits and battle commands cannot delete catalog definitions.
// Protected by catalogRuntimeMu in the native request dispatcher.
var cardAdmins = map[string]cardAdminSession{}

func handleCardAdmin(r runtimeRequest) string {
	if r.LoadoutRoot == "" || r.ContentRoot == "" {
		return runtimeError(fmt.Errorf("workspace roots required"))
	}
	root, _ := filepath.Abs(r.LoadoutRoot)
	contentRoot, _ := filepath.Abs(r.ContentRoot)
	for token, s := range cardAdmins {
		if time.Now().After(s.Expires) {
			delete(cardAdmins, token)
		}
	}
	if r.Op == "open_card_admin" {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return runtimeError(err)
		}
		token := hex.EncodeToString(random[:])
		cardAdmins[token] = cardAdminSession{root, contentRoot, time.Now().Add(30 * time.Minute)}
		return runtimeSuccess(map[string]any{"admin_token": token})
	}
	s, ok := cardAdmins[r.AdminToken]
	if !ok || s.Root != root || s.Content != contentRoot {
		return runtimeError(fmt.Errorf("card deletion requires an active Admin settings session; reopen Admin settings"))
	}
	if r.Op == "close_card_admin" {
		delete(cardAdmins, r.AdminToken)
		return runtimeSuccess(map[string]any{})
	}
	catalogs, err := CharacterCatalogs(r.ContentRoot, r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	lib := catalogs["adventurer"]
	card, ok := lib.Cards[r.CardID]
	if !ok {
		return runtimeError(fmt.Errorf("card no longer exists; reopen Admin settings"))
	}
	saved, err := content.ReadAuthoredCards(r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	blocker := cardDeletionBlocker(r, catalogs)
	if r.Op == "preview_delete_card" {
		return runtimeSuccess(map[string]any{"card_id": card.ID, "name": card.Name, "revision": saved.Revision, "can_delete": blocker == "", "reason": blocker})
	}
	if blocker != "" {
		return runtimeError(fmt.Errorf("cannot delete %s: %s", card.Name, blocker))
	}
	out, err := content.DeleteAuthoredCard(r.LoadoutRoot, lib, r.CardID, r.CatalogRevision)
	if err != nil {
		return runtimeError(err)
	}
	return runtimeSuccess(map[string]any{"deleted_card_id": card.ID, "revision": out.Revision})
}

func cardDeletionBlocker(r runtimeRequest, catalogs map[string]content.BattleLibrary) string {
	lib := catalogs["adventurer"]
	if owner, _, _ := content.TreeCardOwner(lib.CardTrees, r.CardID); owner != "" {
		return "Used by card tree " + owner + ". Edit its tree first."
	}
	for _, tree := range lib.CardTrees {
		for _, edge := range tree.Edges {
			for _, req := range edge.Requirements {
				if req.CardID == r.CardID {
					return "Required by card tree " + tree.Name
				}
			}
		}
	}
	// Describe common blockers before the full catalog validator's fallback.
	var reasons []string
	for _, actor := range lib.Combatants {
		for _, entry := range actor.Decklist {
			if entry.CardID == r.CardID {
				reasons = append(reasons, "Used by "+actor.Name+"'s starter deck.")
			}
		}
	}
	for id, c := range lib.Cards {
		if id == r.CardID || c.Economy == nil {
			continue
		}
		for _, upgrade := range c.Economy.Upgrades {
			if upgrade.To == r.CardID {
				reasons = append(reasons, "An upgrade from "+c.Name+" leads to this card. Change that upgrade first.")
			}
		}
	}
	economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
	if err != nil {
		return err.Error()
	}
	all, _, _, err := loadout.ProgressSnapshot(r.LoadoutRoot, economy, catalogs)
	if err != nil {
		return err.Error()
	}
	for id, progress := range all {
		for _, entry := range append(append([]loadout.Entry{}, progress.Deck...), progress.Collection...) {
			if entry.CardID == r.CardID {
				reasons = append(reasons, fmt.Sprintf("%s owns %d copies. Remove or sell them from its deck first.", lib.Combatants[id].Name, entry.Count))
			}
		}
	}
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return joinDeletionReasons(reasons)
	}
	for id, candidate := range catalogs {
		candidate, err = content.WithoutCard(candidate, r.CardID)
		if err != nil {
			return "Referenced by catalog content: " + err.Error()
		}
		catalogs[id] = candidate
	}
	if _, err = loadout.LoadEconomy(r.ContentRoot, catalogs); err != nil {
		return "Referenced by built-in progression settings: " + err.Error()
	}
	return ""
}

func joinDeletionReasons(reasons []string) string {
	result := ""
	for _, reason := range reasons {
		if result != "" {
			result += "\n"
		}
		result += reason
	}
	return result
}
