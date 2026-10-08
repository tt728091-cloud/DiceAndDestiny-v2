extends RefCounted

## Card face colours. Each card names a frame colour (its colour identity) and
## a border colour in presentation.frame_color / border_color, or gives a
## "#rrggbb" hex value. Every tint on the face is derived from these two, so a
## new colour needs no art. Keep the names in step with content.CardFrameColors
## and content.CardBorderColors on the server, which validate them.

const FRAMES := {
	"white": Color("e8e2cf"),
	"blue": Color("2368a8"),
	"black": Color("2f2b2c"),
	"red": Color("c4392c"),
	"green": Color("2e7a47"),
	"artifact": Color("8f7558"),
	"gold": Color("c9a24a"),
	"colorless": Color("a5a7a6"),
}
const BORDERS := {
	"black": Color("0e0d0d"),
	"white": Color("f1eee6"),
	"silver": Color("b6bac0"),
	"gold": Color("e2bd3f"),
}
const DEFAULT_FRAME := "colorless"
const DEFAULT_BORDER := "black"

static func frame(value: String) -> Color:
	return _resolve(value, FRAMES, DEFAULT_FRAME)

static func border(value: String) -> Color:
	return _resolve(value, BORDERS, DEFAULT_BORDER)

static func _resolve(value: String, named: Dictionary, fallback: String) -> Color:
	var key := value.strip_edges()
	if named.has(key): return named[key]
	if key.length() == 7 and key.begins_with("#") and Color.html_is_valid(key): return Color.html(key)
	return named[fallback]
