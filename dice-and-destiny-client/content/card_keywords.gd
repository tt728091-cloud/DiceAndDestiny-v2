extends RefCounted

# Card-face keywords. A card's hover adds the definition of each keyword its
# face uses, so faces can say "Prevent 3" instead of explaining prevention.
# Patterns match the face text; statuses are defined from the catalog instead.
const TERMS := [
	{"pattern": "\\bPrevent \\d", "text": "Prevent N: stop N damage from one incoming attack. The cards it would have removed are saved."},
	{"pattern": "Saved cards →", "text": "Saved cards → pile: cards this saves move to that pile instead of staying where they were. They still count as health."},
	{"pattern": "Saved cards are removed", "text": "Saved cards are removed: cards this saves are permanently removed instead, and each still costs one health."},
	{"pattern": "\\bSave\\b", "text": "Save: protect a card that an attack would remove."},
	{"pattern": "\\bReroll\\b", "text": "Reroll: roll the die again. A card's reroll does not use one of your normal rolls unless it says so."},
	{"pattern": "\\bFlip\\b", "text": "Flip: turn a die to its opposite face."},
	{"pattern": "\\bClear\\b", "text": "Clear: remove every stack of a status."},
	{"pattern": "\\b(buff|debuff)s?\\b", "text": "Buff / debuff: a positive / negative status."},
]
