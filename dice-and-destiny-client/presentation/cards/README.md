# Cards Presentation

Owns card visual components and card interaction presentation.

Card rule hovers use `card_rules_tooltip.gd` with the shared wrapped text presenter.
The fan owns pointer selection and supplies the whole hand as the exclusion
rectangle; individual card buttons are mouse-ignored inside the fan. Keep the
popup above that rectangle, or beside it when space requires, never anchored to
the mouse over the art. Standalone cards use their own transformed bounds.
Recompute placement during hover to follow scaling, fan movement, and resizing.
Godot still owns tooltip delay, dismissal, and input pass-through.

Validate with `verify_card_tooltip_placement.gd` (actual hovers, no overlap,
scaled viewports, changing cards, clicks, and dismissal) and
`verify_fanned_hand.gd` (fan animation and keyboard/pointer routing).

Battle Settings → **Keep hand visible** disables automatic hand collapse. It is
off by default and persists in the workspace's `battle_preferences.cfg` under
`presentation.keep_hand_visible`. The fan's `keep_visible` preference is separate
from its gameplay-driven `held_open` flag, so targeting, new-card presentation,
and discard prompts can still raise the hand in either mode. Card hover lift,
selection, and tooltips remain active. Verify the checkbox, immediate behavior,
redraws, and disk reload with `verify_hand_visibility_setting.gd`.
