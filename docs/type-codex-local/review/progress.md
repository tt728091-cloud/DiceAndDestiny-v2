# All-types design progress

Requested September 7, 2026: preserve the completed Curse and Venom dossiers; author the other 14 types as webpage-only drafts, each with 5-die face map, 4 signature statuses, 6 abilities, 24 cards, explicit timing, counterplay and at least three cross-type pairings. No new playable characters authorized in this phase.

Venom memory: `docs/game-design/venom-development-handoff.md` and `venom-playable-character.md`.

Editable new dossiers: `docs/type-codex-local/drafts/*.json`. Render to static HTML so the original local-file URL remains usable without a server or fetch. Existing Curse/Venom panel baselines and hashes are in this directory. Do not regenerate or change their reviewed mechanics.

Completed: all 14 dossiers authored and rendered, 84 abilities, 336 cards, 56 signature state entries, plus explicit extra card overlays. Shared contracts, numeric qualification checks, 37-pair interaction index (36 new pairs plus the existing Curse/Venom reference), per-type review notes/export/search and durable source/audit documentation are present. Static structural/content/link/JavaScript validation passes; existing Curse/Venom panels remain byte-for-byte unchanged. Visual/browser interaction verification remains blocked as described below. Next action belongs to the user: review type drafts and request revisions; do not implement new characters automatically.

Screenshot constraint: CUA selected the existing local Chrome codex tab but the Browser URL policy rejected access. Do not use alternate browser surfaces or indirect browser execution to bypass it. The supplied type-index screenshot is preserved as `type-index-user-reference.png`. No fresh browser screenshots have been captured.

References read: local type identities, both existing dossiers, Battle Hooks Field Guide, current turn-structure rules (batch collection/commit/children, accepted costs, cancellation scopes, cards-as-health), active status definitions, and selected patterns from the 1,260-row / 40-character Dice Throne reference CSV. External reference names/rules are inspiration only; author original content.

September 8 revision: all 56 signature entries across the 14 pending types were reviewed and reworded, with distinct resource jobs and 56 purpose/example pairs. Dependent cards, abilities, combinations, counterplay and pairings now use those rules. Player-facing technical terminology was removed; developer hook names remain in a collapsed reference. Current dossiers have version 2. Read `status-rework-v2.md` for durable decisions. The v1 JSON and shared rules are archived in `revisions/` and must not be used as current rules. No gameplay implementation was authorized or performed by this revision.
