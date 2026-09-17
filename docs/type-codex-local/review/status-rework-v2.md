# Status review — revision 2, September 8, 2026

The user rejected the repeated spend-for-damage/prevention statuses and unexplained engine language in the fourteen pending type drafts. Revision 2 revisits all **56 signature entries across those 14 types**, then updates the affected cards, abilities, examples and pairings. Curse and Venom remain the reviewed reference panels, unchanged.

The editable `drafts/*.json` files and `shared-rules.json` are the current rules. `revisions/statuses-v1.json` and `revisions/shared-rules-v1.json` preserve superseded designs for comparison, not implementation. Each dossier is version 2, which asks for a fresh reviewed checkmark while retaining the browser's typed notes.

## Writing decisions to preserve

- Write the player action and result: “After you play your second card from your hand,” not “second accepted card play.”
- Describe when something happens using the game's segments, dice, cards, damage and stacks. Keep “batch commits,” source construction, proposals and other engine terms out of player rules.
- Explain a special effect where it is introduced. Echo repeats the stored damage number; it does not play a card or repeat the original status application. Pulse specifies who rolls, which die, the results and its use limit.
- Give each status a short purpose and a numerical or step-by-step example. State the holder, cap, payment, result, duration and any meaningful limit.
- Mechanical similarity needs a reason. A resource should have a distinct use or tradeoff; assigning a new name to +1 damage or one-for-one prevention is insufficient.
- When a rule changes, revise its cards, abilities, examples, counterplay and both sides of its pairings. Do not leave an old rule in the explanatory text.
- These are proposals awaiting user review. Do not implement new characters from them automatically.

## Main changes by type

| Type | Revision 2 distinction |
| --- | --- |
| Flame | Heat cools each Ongoing Effects and gives one attack bonus while at three or more stacks. Spending it can lose that threshold. Burn, Kindling and Ember have different jobs: delayed damage, status-damage amplification and card-linked Heat. |
| Tide | Flow pays for Tide techniques. Breakwater alone provides the stored one-for-one prevention. Soaked retries a status die; Undertow trades the next usual enemy Energy gain for Flow. |
| Storm | Charge comes from deliberate combat rerolls. Ground collects Charge when an enemy pays Energy for a card. Conductive punishes a final low effect roll; Overload adds a normal reroll attempt. |
| Frost | Chill charges Energy to start rerolling, leaving Entangle's roll reduction distinct. Frostbind locks one die; Rime builds matching numbers; Brittle punishes a block of at least two. |
| Stone | Armor blocks the first damaging Offensive ability each round according to its current stacks, then loses one stack. Resolve can buy Fortify. Fracture affects rolled defense; Fortify protects a number from an enemy edit. |
| Verdant | Growth reproduces only while a reserve survives. Spores rewards another affliction landing. Regrowth schedules paid recovery; Graft adds Leaf to a particular face without changing its number. |
| Blood | Ichor can pay for recovery conditional on a damaging attack. Blood Mark earns Ichor from health loss. Clot stops one Ongoing Effects worth of normal Poison or Bleed, without cleansing; it does not stop Volatile Poison. Existing Bleed is unchanged. |
| Beast | Quarry identifies the hunted enemy. Instinct rewards its repeated ability and can buy Pounce. Scent buys one extra single-die attempt. Pounce follows an attack that removes health. |
| Swarm | Brood grows from the first Offensive discard, rather than copying Aether's second-card reward. Carapace scales with Brood kept in reserve. Web joins two damaging hits across effects; Royal Order sends a third Brood. |
| Construct | Modules persist, with one selected program. Scrap builds and salvages them. Corrosion reduces the next positive status gain rather than duplicating Fracture. Calibration stores a chosen number for a chosen die. |
| Grave | Corpse records lost cards without restoring them. Remnant funds Haunt and Grave techniques. Haunt offers the enemy extra Energy payment or damage, or grants Remnant on card loss. Death Anchor alone provides the last-chance rescue. |
| Arcane | Aether discounts a card after earning stacks from hand plays. Echo stores damage only. Rune stores an already-rolled number for another die, replacing its old duplicate of Graft. Mirror reflects one new harmful stack. |
| Radiant | Grace supports cleansing and recovery; Ward blocks new stacks. Censure taxes the next card. Beacon chooses the next Income draw from discard rather than duplicating Death Anchor's rescue. |
| Fortune | Favor nudges a die number. Boon rewards real rolls. Streak tracks risky rerolls toward Jackpot. Insurance preserves Streak after a miss instead of providing another generic prevention token. |

Ice Seal and Anchor remain card-specific marks, fully explained on Icy Grip and Lode Bearing. Ice Seal temporarily forbids playing its card; it does not duplicate Censure's tax. Anchor exchanges the marked card for a random unmarked card from the same pile when damage would remove it, without reducing health loss.

## CSV research used

Read the local [Dice Throne reference CSV](../../game-design/dice-throne-reference/dice-throne-reference.csv), including its 122 StatusEffect entries. These patterns informed original proposals rather than importing external rules:

- Pyromancer's Fire Mastery: a cooling reserve and held-stack threshold, informing Heat.
- Artificer's Synth and bots: resources used to build distinct lasting pieces, informing Scrap and Module.
- Tactician's Constrict and Iceman's Dice Cube: concrete restrictions on rerolls, informing separate Chill and Frostbind roles.
- Tactician's Tactical Advantage and Vampire Lord's Blood Power: named choices with explicit costs and results, informing the resource-use differentiation.
- Doctor Strange's Premonition: choosing a future card without a free extra draw, informing Beacon.
- Existing target marks and reactive statuses: specify the watched event and its consequence instead of referring to engine processing.

Local rules take precedence: Poison and Volatile Poison use the reviewed Venom definitions, Bleed does not roll dice, Energy is the local currency, cards are health, and sacrifice is not recoverable damage.

## Manual consistency cases reviewed

These are design walkthroughs, **not executed gameplay tests**.

| Case | Expected result |
| --- | --- |
| Prevent damage, then cleanse in the same round | Only one earned Flow reward. Flow itself cannot be spent for prevention without a Tide card or ability. |
| Three Heat enters Ongoing, then enemy Poison deals damage | Heat cools to two, earns one back, and again meets its attack threshold. |
| Conductive on a Pulse roll of 2 versus 5 | At 2: one Pulse damage plus a separate one status damage; at 5: two Pulse damage and keep Conductive. Use the final corrected number. |
| Chill and Overload together | Initial roll stays free. Pay one Energy to begin rerolls; Overload still supplies its extra normal attempt. |
| Three Armor faces two attacks this round | The first qualifying attack can lose three damage and Armor loses one stack. The second receives no automatic Armor reduction. |
| Two Brood versus three Brood held with Carapace | One Carapace prevents two versus three attack damage, leaving those Brood in reserve. |
| Web's first hit lands; second is fully blocked | Keep Web with one hit counted; a later damaging hit before expiry can finish it. |
| Second hand card draws two and discards one in an Arcane/Swarm creature | Earn Aether for the second card and Brood for the first Offensive discard. Echo does neither. |
| A Rune changes a die to a Boon face | The number can qualify an ability, but changing it earns neither Favor nor Streak. |
| Regrowth recovers a card with Corpse, then Beacon marks it | The Corpse disappears; next Income takes that card instead of the normal draw. Beacon is not a rescue. |
| Clot with normal Poison and Bleed together | Choose one named status to block for this Ongoing Effects; the other still damages. Normal clearing/decay still happens. |
| Catalyst already retried one Poison die | Soaked may retry another revealed status die, never that same die again. |

## Verification and limits

Static validation covers all 14 version-2 dossiers, 56 rule examples, 84 abilities, 336 cards, 37 distinct pairings, all routes and local links, HTML structure, JavaScript syntax, feasible dice recipes and byte-identical Curse/Venom panels. The wording audit rejects the engine terms that caused this review in the new player-facing dossiers. The renderer must produce identical output on a second build.

No new game code or game content was changed by this revision. No gameplay balance claim is made. Fresh rendered-page and browser-interaction QA remain unavailable because the earlier browser request for the local codex URL was rejected by Browser URL policy; no alternate access was attempted.
