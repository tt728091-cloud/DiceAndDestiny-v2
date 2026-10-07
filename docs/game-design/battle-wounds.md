# Battle wound review

The completion screen's **Review · Wounds** button opens a read-only ledger for
all combatants. Character tabs show wound and card-loss totals. Each wound lists
its round, segment, source actor, attack/card/status, and exact lost card copies.
Pile icons show where cards were removed; hovering or focusing a card previews it
beside the list at normal hand-card size. Close or Escape returns to the result.

During combat, player and enemy health bars show one muted gray segment per
committed wound in the missing-health area. Segment width represents cards
actually lost; adjacent segments use different gray tones. Oldest wounds stay
at the right, with later wounds extending leftward. Hover shows the wound’s
damage, round, cause, and lost cards (including their original piles). Segments
appear only once displayed health reflects the hit, so queued damage animations
do not expose future wounds. Unknown historical losses remain dark.

## Recording contract

- Record only successfully committed damage removals, after prevention. One
  damage source in one batch is one wound. Repeated hits, multiple enemies,
  and later rounds remain separate. Zero-loss hits do not create wounds.
- Wound damage is the number of recorded cards, not raw attack damage. Released
  cards and overkill beyond available health never count. Non-damage removals,
  such as playing a card with a remove destination, do not create wounds.
- Unified Defense preserves exact source reservations. Effects and legacy pooled
  batches allocate their actual removed cards in source order, capped by each
  source's final damage. This assigns the previously unassigned pooled cards
  without changing card selection, randomness, prevention, or health.
- `state.Battle.Wounds` retains stable wound, batch, source, actor, and card
  instance IDs; card definitions; round/segment; and the live removal zone.
  The ledger deep-clones and persists with the battle checkpoint. Recording the
  same batch again cannot add duplicate wounds.
- The viewer snapshot exposes the committed ledger throughout combat and at completion.
  Presentation does not derive wounds from animations, transient events, or the
  aggregate removed pile. Old saves cannot reconstruct exact historical hits;
  any removed cards without records are explicitly identified in the review.
- Records begin with damage committed by this version. Existing battles retain
  their rules and can record subsequent damage, but earlier losses are not
  retroactively invented.

This phase does not change character decks between battles and does not implement
healing, bandages, permanent wounds, or recovery rolls. Those systems can later
reference individual wound and card-instance IDs without pooling distinct hits.

## Validation

- `internal/battle/engine/wounds_test.go`: prevention, separate hits/rounds,
  ongoing effects, overkill, played-card zones, failed commits, deep clone,
  JSON persistence, snapshot visibility, duplicate recording.
- `internal/battle/damage/damage_test.go`: legacy commitment and atomic retries.
- `tests/presentation/verify_wound_health_bar.gd`: proportional segments, muted
  colors, player/enemy pointer hovers, one/four enemies, 720p/1080p/4:3 layouts,
  playback visibility, rebuild stability, and unrecorded historical losses.
- `tests/presentation/verify_wound_review.gd`: real native battle against two
  Brine Masks, loss totals, pointer-open/close/reopen, keyboard close, actor tabs,
  card previews, viewport sizes, scrolling, four-enemy layout, and missing records.
