# Configurable card trees

Open **Character Creation → Card Trees** to manage a character's tree cards. Open **Admin settings → Manage card trees** to create or change trees. Publication uses the existing workspace-local admin capability; this is not remote account authentication.

## Admin player sheet

**Admin settings → Admin player sheet · All cards** opens a read-only view of the complete published catalog. It includes cards of every character type, tree bases, and all owned or unowned variants. Choose a character to see its deck counts, stored copies, health, and available XP without narrowing catalog visibility. Search names, IDs, rules, or tree names; filter by type, tree variants, base/standalone cards, or ownership. Select a row to inspect its rules, XP, energy, stable ID, and tree membership. Unpublished drafts remain in their editor until published. This sheet does not grant cards or enable direct purchases of variants.

## Authoring

The workshop draws each tree as a constellation: cards are medallions with their art, energy pip and ownership badges, joined by paths that show the XP step (`+3 XP`, `−2 XP`) and any deck rules. Upgrades sit above the base and cheaper variants below it. Hovering a card highlights its routes back to the base and shows full rules plus what changed from its predecessor.

Navigation: drag the background (or a card, in the player view) to pan; scroll or pinch to zoom; two-finger trackpad swipes pan. **Center** (or **F**) fits the tree. Right-click a card, connection or the background for its actions.

### Starting a tree

**New tree** opens the start panel. Either:

- **Create a new base card** — enter a name, pick a starting effect (or **Copy of** an existing card), energy, amount, XP value and artwork, then **Create base card**. The card exists only in this tree's draft until the tree is published, and existing cards are never changed. **Design in full editor…** opens the full card creator for anything more complex. The tree name and ID default from the card name.
- **Use an existing card** — the base is that actual catalog card; editing it changes its definition for future battles. Owned cards are listed first. A card belongs to one tree, and published trees keep their base identity.

### Growing it

Select a card and choose **Upgrade ↑** (U) or **Cheaper ↓** (D). The forge copies the card and offers only valid one-step adjustments to its own fields: energy, each effect's values (for example *Prevent damage 1 → 2*), saved-card destinations, targets, choice energy, uses per round, play destination, and one-click compatible additions such as *+ Draw 1 card*. Each change is validated immediately; the panel previews the rules text and the change list, suggests a name (for example *Steady Guard · Swift*) and suggests +3 XP (upgrade) or −2 XP (cheaper, minimum 1 XP). **Create upgrade** adds the card one row above or below and connects it. **Open full card editor…** covers every other setting and returns to the forge. **Quick edit** uses the same controls on an existing card. The editor does not infer balance from price.

A card is a complete definition, not a patch. Editing one card never changes its descendants. Tree card buy/sell values are equal and at least 1 XP; their prices are managed here rather than in the separate economy price overrides.

**To reuse an existing card such as Brace+:** select the card, choose one under **Use existing card as template**, then **Apply card template**. This replaces the card's settings (name, effects, artwork, energy, XP) while preserving its generated ID, position and connections. A unique name is generated from the source card and tree name. The source card remains unchanged.

### Connections and layout

Each branch can branch again. To combine branches, connect each chosen predecessor to the shared destination: drag the **⊕** knob on a card onto another card, use **Connect from here…** in a card's right-click menu, or pick a predecessor under **Connections**. Connections into the base, duplicates and loops are refused immediately. A shared destination has one complete definition, so both routes produce the same card. Connections optionally permit returning to their predecessor; the path shows a small reverse chevron when they do.

Drag cards to arrange them (positions snap to a 20-unit grid). **Auto-arrange** lays the tree out by XP: each upgrade one row up, each cheaper variant one row down, converging cards centred under their predecessors.

**Undo/Redo** (⌘Z / ⇧⌘Z, or Ctrl+Z / Ctrl+Y) covers every draft edit, including moves, rules and card settings. **Delete** removes the selected card or connection. Validation runs live: problems mark the affected card in red and appear beside the status line, and **Publish tree & cards** enables once the draft is valid.

### Steady Guard example

**Steady Guard example** creates an unpublished draft with its own new base card, so it never edits Brace:

| Card | Energy | Prevention | Saved cards | Total XP |
| --- | ---: | ---: | --- | ---: |
| Steady Guard (base) | 1 | 1 | Discard | 10 |
| Deeper guard | 1 | 2 | Discard | 13 |
| Original piles | 1 | 1 | Original live piles | 14 |
| Complete guard | 1 | 2 | Original live piles | 17 |
| Heavy guard | 2 | 1 | Discard | 8 |

Complete guard connects from both upper branches. Nothing changes until an admin publishes it.

## Deck restrictions, not upgrade restrictions

Select a connection to add **Require card** or **Forbid card** rules. Required rules support a minimum copy count; **Include descendant variants** treats that card and its tree descendants as a family. The connection displays the referenced card, with a crossed-out illustration for exclusions. Select it to read every rule.

These rules do not prevent creating, owning, or upgrading a card. They apply when adding it to the deck. A deck must satisfy all rules along at least one complete path from the base to each equipped variant. A converging node may qualify through either route. Requirements remain enforced on subsequent deck edits, purchases, sales, and moves out of the deck, so adding a conflicting card or removing a prerequisite cannot bypass them. Only deck contents count; conflicting cards stored in the collection do not matter.

For deck-wide exclusive branches, add reciprocal forbidden-card rules to the two branch entries, including descendants when appropriate. For separate paths on individual copies, separate connections suffice; different copies can take different paths. Be careful with descendant exclusions at a shared destination: a destination descending from both excluded branches will exclude itself. Such a graph expresses an unavailable deck configuration, rather than an automatic exception to its rules.

A shield prerequisite currently means a specified shield card in the deck. This feature does not introduce a separate equipment-slot system.

## Collection, XP, and health

In the player view, owned cards and the paths between them glow gold; cards you can obtain right now pulse, with animated paths from the owned card; cards blocked by XP or rules show a lock. Select a card to see **Get this card** (routes into it) and **Paths from here** (routes out of it), each with the change summary and price. Hovering a trade compares both cards' rules. Every trade asks for confirmation and says whether it uses a stored copy or takes one out of the deck (lowering health).

Upgrading moves one owned copy along an adjacent connection and stores the resulting variant in the character's collection. It uses a stored source copy first; otherwise it takes one equipped source copy out of the deck. If removing that equipped source would invalidate another equipped card's prerequisite, move the dependent card out first or use a separate source copy.

The transaction spends or refunds the difference between total card values. For example, 10 → 13 costs 3 XP; 10 → 8 refunds 2 XP. Reversing the same path reverses the same difference. Stored cards retain their invested XP value but contribute **no health**. **Add collected copy to deck** costs no additional XP and checks deck requirements, character access, and copy/deck limits. **Move deck copy to collection** is also free. **Sell collected copy** refunds its value. **Buy base for collection** acquires a new source without equipping it. The regular Deck & Library inspector also shows stored quantities and controls for moving, equipping, and selling collected copies, including cards outside trees.

The budget ledger is: available XP + equipped card value + stored card value + ability upgrade spending. There is no additional permanent tree-point currency. Direct progression purchases of non-base variants are disabled; obtain them through the tree. Sandbox deck editing remains free as before but must satisfy the same deck restrictions.

## Persistence and validation

Tree metadata and every generated card are stored in one atomic revision of the tracked `dice-and-destiny-server/content/authored/authored_cards.json`; commit it to share published trees. Collection and equipped cards share the existing per-character progression ledger across modes. Battle catalogs remain pinned, so publishing a tree does not change an active battle's rules or cards.

Validation rejects stale publications/trades, unknown fields, invalid card effects, duplicate or colliding IDs, missing referenced cards, cycles, disconnected nodes, invalid XP, and edits that invalidate saved decks, collection contents, or budgets. Graph limits are 100 nodes, 300 connections, and 20 rules per connection. Graph definitions are acyclic; an explicitly reversible connection supports refunds without introducing a definition cycle.

Regression coverage lives in `internal/battle/learned/card_tree_test.go` and `tests/presentation/verify_card_trees.gd`. It includes restricted upgrades, equip-time rejection, prerequisites, deck-wide exclusions, alternative paths, XP conservation, stored-card sales, stale revisions, admin publication, complete battles with tree variants, pinned definitions, and pointer-driven authoring/equipping.
