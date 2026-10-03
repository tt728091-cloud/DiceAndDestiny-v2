extends RefCounted
# Small vector symbols share one palette and remain crisp at any board scale.
const SHAPES := {
	"strong_swing_ready": '<path d="M6 26L24 6 28 4 26 10 10 28Z"/><path d="M5 18L14 27M4 28L7 25"/>',
	"protect": '<path d="M16 2L28 7V16Q26 26 16 30Q6 26 4 16V7Z"/><path d="M10 16L14 20 23 11"/>',
	"dice": '<rect x="4" y="4" width="24" height="24" rx="4"/><circle cx="10" cy="10" r="1"/><circle cx="22" cy="10" r="1"/><circle cx="16" cy="16" r="1"/><circle cx="10" cy="22" r="1"/><circle cx="22" cy="22" r="1"/>',
	"entomb": '<rect x="6" y="7" width="20" height="21" rx="3"/><path d="M12 13V7C12 0 20 0 20 7V13M4 18H28M16 15V23"/>',
	"attack": '<path d="M6 27L25 3 29 3 29 7 10 27M5 18L16 27M3 29L7 25"/>',
	"block": '<path d="M16 2L28 7V16Q26 26 16 30Q6 26 4 16V7Z"/>',
	"energy": '<path d="M16 2L20 12 30 16 20 20 16 30 12 20 2 16 12 12Z"/>',
	"deck": '<path d="M7 4H23V26H7Z M4 8V29H21"/><path d="M12 10H18M12 15H18"/>',
	"hand": '<path d="M8 18V11Q8 8 11 10V17 5Q13 2 15 5V15 4Q17 1 19 4V16 7Q22 4 23 7V19L26 15Q30 14 28 19L23 28H12L4 19Q3 15 6 16Z"/>',
	"discard": '<path d="M9 9V28H23V9M6 8H26M12 4H20M13 13V24M19 13V24"/>',
	"removed": '<circle cx="16" cy="16" r="12"/><path d="M11 11L21 21M21 11L11 21"/>',
	"poison": '<path d="M12 3H20M14 3V12L6 25Q5 29 10 29H23Q27 29 25 25L18 12V3M9 22H23"/><circle cx="14" cy="19" r="1"/>',
	"volatile_poison": '<path d="M12 3H20M14 3V12L6 26Q5 29 10 29H23Q27 29 25 25L18 12V3"/><path d="M18 14L12 21H18L14 27"/>',
	"bleed": '<path d="M16 2Q12 10 7 16C-1 31 33 34 25 16Z"/><path d="M10 19Q8 24 14 26"/>',
	"blind": '<path d="M2 16Q16 0 30 16Q16 32 2 16M4 28L28 4"/><circle cx="16" cy="16" r="5"/>',
	"entangle": '<path d="M10 2Q25 10 12 17T21 30M22 2Q5 11 20 19T10 30M6 8L12 9M21 24L27 22"/>',
	"cursed_entangle": '<path d="M10 2Q25 10 12 17T21 30M22 2Q5 11 20 19T10 30"/><path d="M2 15L6 11 10 15 6 19Z"/>',
	"catalyst": '<path d="M16 2L27 8V22L16 30 5 22V8ZM5 8L16 16 27 8M16 16V30"/>',
	"incubation": '<path d="M16 3C2 8 1 28 16 29C31 28 30 8 16 3Z"/><path d="M17 9L12 16 20 19 15 25"/>',
	"curse_count": '<path d="M8 21C-3 5 10 1 16 2C31 1 34 17 24 21V28H8ZM12 23V29M20 23V29"/><circle cx="10" cy="14" r="3"/><circle cx="22" cy="14" r="3"/><path d="M14 20L16 17 18 20Z"/>',
	"grave_interest": '<path d="M6 3H26M6 29H26M9 4V9L23 23V28M23 4V9L9 23V28M10 25H22"/>',
	"grave_debt": '<path d="M8 28V10C8 0 24 0 24 10V28ZM12 13H20M16 9V20M5 29H27"/>',
	"second_knell": '<path d="M5 23H27L23 18V12C23 2 9 2 9 12V18ZM12 27Q16 32 20 27M16 2V5"/><path d="M26 5L29 2M28 11H31"/>',
	"three_knocks_status": '<path d="M6 4H26V29H6ZM17 12V22M13 17H21"/><circle cx="3" cy="9" r="1"/><circle cx="3" cy="16" r="1"/><circle cx="3" cy="23" r="1"/>',
	"maledictions_refusal": '<path d="M16 2L28 7V16Q26 26 16 30Q6 26 4 16V7ZM10 10L22 22M22 10L10 22"/>',
	"black_dividend": '<circle cx="16" cy="16" r="13"/><path d="M20 9H13Q7 15 16 16T19 23H11M16 5V27"/>',
	"chosen_instrument": '<path d="M7 3H25V29H7ZM11 8H21M11 14H21M11 20H21"/><circle cx="16" cy="25" r="1"/>',
	"curse_bloom": '<path d="M16 29V18M16 24L7 20M16 26L25 21M16 17C-2 20 1 8 11 11C4-3 28-3 21 11C34 8 31 22 16 17Z"/>',
	"injury": '<path d="M5 9L9 5 27 23 23 27ZM5 23L9 27 27 9 23 5Z"/>',
	"advanced_poison": '<path d="M12 3H20M14 3V12L6 26Q5 29 10 29H23Q27 29 25 25L18 12V3M10 22H23M16 14V25M12 19H20"/>'
}
static var cache: Dictionary = {}
static func texture(id: String) -> Texture2D:
	if cache.has(id): return cache[id]
	var color := "fff0cc"
	if id in ["poison", "volatile_poison", "catalyst", "incubation", "advanced_poison"]: color = "b9ef75"
	elif id in ["protect", "strong_swing_ready"]: color = "a5edce"
	elif id in ["bleed", "injury"]: color = "ff858a"
	elif id in ["curse_count", "second_knell", "three_knocks_status", "curse_bloom", "maledictions_refusal", "cursed_entangle"]: color = "d98aff"
	elif id in ["energy", "grave_interest", "black_dividend", "grave_debt"]: color = "ffdb78"
	var shape: String = SHAPES.get(id, '<path d="M16 2L30 16 16 30 2 16Z"/><circle cx="16" cy="16" r="5"/>')
	# Opaque outer ink follows the symbol itself, never a rectangular backplate.
	var svg := '<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="-2 -2 36 36"><g stroke-linecap="round" stroke-linejoin="round"><g fill="#15121b" stroke="#100e13" stroke-width="5.5">%s</g><g fill="#15121b" stroke="#%s" stroke-width="2.8">%s</g></g></svg>' % [shape, color, shape]
	var image := Image.new(); image.load_svg_from_string(svg)
	cache[id] = ImageTexture.create_from_image(image)
	return cache[id]
