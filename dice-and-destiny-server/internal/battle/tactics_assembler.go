package battle

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// TacticsPack is the content folder of the Tactics Sandbox.
const TacticsPack = "tactics_v1"

// LoadTacticsLibrary builds the catalog a tactics battle pins: battle_v1 with
// the venom, general, minion and adventurer packs, the published (authored)
// cards from the loadout root, then the tactics pack, whose ability copies
// replace the shipped definitions in this catalog only.
func LoadTacticsLibrary(contentRoot, loadoutRoot string) (content.BattleLibrary, content.TacticsPackManifest, error) {
	var manifest content.TacticsPackManifest
	library, err := content.LoadBattleLibrary(filepath.Join(contentRoot, "battle_v1"))
	if err != nil {
		return library, manifest, err
	}
	for _, pack := range []string{"venom_v1", "general_v1", "minions_v1", "adventurer_v1"} {
		if library, err = content.LoadBattleExtension(library, filepath.Join(contentRoot, pack)); err != nil {
			return library, manifest, fmt.Errorf("%s: %w", pack, err)
		}
	}
	if loadoutRoot != "" {
		if library, err = content.OverlayAuthoredCards(library, loadoutRoot); err != nil {
			return library, manifest, err
		}
	}
	return content.ApplyTacticsPack(library, filepath.Join(contentRoot, TacticsPack))
}

// NewTacticsParticipantAssembler assembles tactics battles from that catalog.
func NewTacticsParticipantAssembler(contentRoot, loadoutRoot string) (ParticipantAssembler, content.TacticsPackManifest, error) {
	library, manifest, err := LoadTacticsLibrary(contentRoot, loadoutRoot)
	if err != nil {
		return nil, manifest, err
	}
	catalog, err := json.Marshal(library)
	if err != nil {
		return nil, manifest, err
	}
	return ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		return AssembleTacticsParticipants(participants, library, catalog)
	}), manifest, nil
}

// AssembleTacticsParticipants assembles seats from an already loaded tactics
// catalog (and its JSON, which the battle pins).
func AssembleTacticsParticipants(participants []participant.Participant, library content.BattleLibrary, catalog []byte) (state.BattleSetup, error) {
	for _, requested := range participants {
		if _, ok := library.Combatants[requested.DefinitionID]; !ok {
			return state.BattleSetup{}, fmt.Errorf("unknown tactics combatant %q", requested.DefinitionID)
		}
	}
	return assembleSettledParticipantsWithCatalog(participants, library, catalog)
}
