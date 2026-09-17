# Cinematic battle UI implementation

Implemented in the live Godot battle screen using `cinematic-duel-compact-left-abilities-v1.png` as the visual reference. The screenshot `cinematic-battle-implemented-2026-09-15.png` is a rendered game scene, not a painted UI mockup.

- Cathedral battlefield, Venom/Blade Warden scene art, faction banners, brass frames, serif labels, red health bars, and illustrated cards. Other character matchups use the empty cathedral and their own existing portraits.
- Compact character panels retain health, energy, all four card-zone counts, status stacks, and rule tooltips.
- Four offensive abilities occupy the lower-left rail. Needlefang's qualified tiers remain individually selectable; unavailable tiers remain disabled. The information button opens the current rules, including Venom Lens upgrades.
- The bottom hand remains scrollable, with independent costs, legal states, card targeting, and hand-limit selection. Both offensive dice trays remain interactive under the existing authority rules. Kept dice have gold highlights.
- Action controls stay separate from the scrollable results area. Enemy abilities, combat log, inspection tools, settings, and opt-in developer history are available through the right-side utilities. **Disable auto-pass is in Settings.** Existing transcript and snapshot controls are in Inspect.
- Defense results retain their side of the battlefield even when only one actor defends. The three-second result review remains intact. A defense checkpoint without any results no longer adds an empty review after a card animation.
- Existing automatic Effects, card cleanse, conversion, damage prevention, income, and Catalyst animations remain connected to the actual battle data. Status trails now correctly transform their coordinates when the cinematic stage is scaled.
- Damage removals are grouped side by side with compact cards; Effects can use the full central height. Long content scrolls without moving the action controls or hand.

The scene uses a 1920×1080 design space scaled uniformly to the available window; other aspect ratios are letterboxed. No battle rules, card costs, RNG, authority commands, model policy, or saved-battle format were changed.

## Validation

All Godot commands ran through `./scripts/godot.sh`, with disposable launcher-managed test state.

- New `verify_cinematic_layout.gd`: 1920×1080, 1280×720, and 1600×1000; live pointer input for Settings, debug toggle, and Needlefang tier selection; all four abilities visible; long content cannot displace actions.
- `verify_battle_scene.gd`: authority-driven card targets, dice keeping/rerolling, damage cards, defense display, income, snapshots, and history controls.
- Passed defense result animation, enemy-only defense review, defense review/debug toggle, automatic pass, automatic defense roll, single-choice defense, quiet offensive reaction, card cleanse, damage response flow, and automatic Effects tests.
- Passed native Venom Lens preview, Steady Hand dice update, authority start/roll smoke test, and Venom character/full-game test.
- Inspected graphical captures of planning, defense, Catalyst flight, Effects dice/card losses/closeout, and damage prevention.

Generated environment and card illustrations are stored in `dice-and-destiny-client/assets/battle/cinematic/`. Banners and every interactive control are native/vector UI. The six illustration panels are reused by card category; card identity, rules, costs, and legality always come from the authority catalog.
