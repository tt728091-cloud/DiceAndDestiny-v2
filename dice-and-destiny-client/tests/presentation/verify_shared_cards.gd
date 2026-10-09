extends "res://tests/presentation/verify_card_trees.gd"
## Shared cards: one definition lent to several trees, each tree owning its XP.
## Drives the real tree editor, player tree view, deck screen and admin sheet
## against the native authority with pointer input where the UI supports it.
const NAMES = preload("res://app/screens/character/card_name_suggester.gd")
const SHARED := "linked_ward"

func _shared_node(ui) -> Dictionary:
	for n in ui.draft.nodes:
		if n.card.id == SHARED: return n
	return {}

func _published_node(ui, tree_id: String) -> Dictionary:
	for n in ui.state.trees.get(tree_id, {}).get("nodes", []):
		if n.card.id == SHARED: return n
	return {}

func _publish(ui, what: String) -> void:
	await _click(_tree_control(ui, "validate")); await _frames()
	_expect(not ui._publish.disabled, what + " validates: " + ui._message.text)
	await _click(ui._publish); await _frames()
	_expect(ui._message.text.begins_with("Published"), what + " publishes: " + ui._message.text)

func _deck_row(screen, key: String) -> Button:
	for b in screen._deck_buttons:
		if b.get_meta("entry_id", "") == key: return b
	return null

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1920, 1080)
	var runtime = root.get_node("LearnedBattleRuntime")
	# The suggester never treats a card's own name as taken by itself.
	var taken := NAMES.taken({SHARED: "Linked Ward", "other": "Other Ward"}, SHARED)
	_expect(not taken.has("linked ward") and taken.has("other ward"), "a shared card's own name is free for itself")

	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen); await _frames()
	await _click(screen._admin_button); await _frames()
	await _click(_control(screen, "admin.trees")); await _frames()
	var ui = screen.get_node("CardTreeWorkspace")

	# Tree A grows from Steady Guard; its upgrade becomes a shared card.
	_tree_line(ui, "tree.id", "guard_tree"); _tree_line(ui, "tree.name", "Guard paths")
	_choose(ui._base_pick, "steady_guard"); await _click(_tree_control(ui, "base")); await _frames()
	_expect(int(ui._node("base").get("xp", 0)) >= 1, "base node carries the tree's XP")
	var guard_base: int = ui._value(ui._node("base"))
	await _click(_tree_control(ui, "add_up")); await _frames()
	_tree_line(ui, "forge.name", "Linked Ward")
	await _click(_tree_control(ui, "forge.commit")); await _frames()
	var guard_node: String = ui._selected
	_expect(int(ui._node(guard_node).xp) == guard_base + 3 and int(ui._node(guard_node).card.economy.buy) == guard_base + 3, "upgrade node XP set like the suggested price (+3)")
	_tree_line(ui, "share.id", SHARED)
	await _click(_tree_control(ui, "share_card")); await _frames()
	var shared: Dictionary = ui._node(guard_node)
	_expect(shared.get("shared", false) and shared.card.id == SHARED, "Share this card turns the variant into a shared card with its own ID")
	_expect(int(shared.xp) == guard_base + 3 and int(shared.card.economy.buy) == 0 and int(shared.card.economy.sell) == 0, "shared card keeps the tree's XP on the node and no price of its own")
	_expect(ui._canvas._nodes[guard_node].shared, "shared node draws the chain-link badge")
	# Undo/redo and auto-arrange preserve the node fields.
	await _key(KEY_Z, true); _expect(not ui._node(guard_node).get("shared", false), "undo restores the exclusive variant")
	await _key(KEY_Z, true, true); _expect(ui._node(guard_node).get("shared", false) and int(ui._node(guard_node).xp) == guard_base + 3, "redo restores shared and xp")
	await _click(_tree_control(ui, "arrange")); await _frames()
	_expect(ui._node(guard_node).get("shared", false) and int(ui._node(guard_node).xp) == guard_base + 3, "auto-arrange keeps shared and xp")
	await _publish(ui, "tree A")
	_expect(ui.catalog.templates.has(SHARED) and int(ui.catalog.templates[SHARED].economy.buy) == 0, "published shared card has no price of its own")
	_expect(_published_node(ui, "guard_tree").get("shared", false) and int(_published_node(ui, "guard_tree").xp) == guard_base + 3, "published tree keeps xp and shared")

	# Tree B grows from Nudge and lends the same card at a different XP.
	await _click(_tree_control(ui, "new")); await _frames()
	_tree_line(ui, "tree.id", "nudge_tree"); _tree_line(ui, "tree.name", "Nudge paths")
	_choose(ui._base_pick, "nudge"); await _click(_tree_control(ui, "base")); await _frames()
	var nudge_base: int = ui._value(ui._node("base"))
	ui._context_menu("canvas", "", Vector2(400, 400)); await _frames()
	var menu_items: Array = []
	for i in ui._menu.item_count: menu_items.append(ui._menu.get_item_text(i))
	_expect("Add an existing card…" in menu_items, "background right-click offers Add an existing card: " + str(menu_items))
	ui._menu.hide()
	await _click(_tree_control(ui, "add_existing")); await _frames()
	var picker: OptionButton = _tree_control(ui, "share.card", "tree_field")
	var offered := {}
	for i in picker.item_count: offered[str(picker.get_item_metadata(i))] = true
	_expect(offered.has(SHARED), "existing shared card is offered")
	_expect(not offered.has("steady_guard") and not offered.has("nudge"), "bases are never offered as shared cards")
	_expect(offered.keys().all(func(id): return not str(id).contains("_variant_")), "exclusive variants are never offered")
	_expect(int(_tree_control(ui, "share.xp", "tree_field").value) == nudge_base + 10, "default XP is 10 more than the node it connects from")
	# Another tree's base is refused even when requested directly.
	var before_nodes: int = ui.draft.nodes.size()
	ui._add_existing("steady_guard", "base", 12)
	_expect(ui.draft.nodes.size() == before_nodes and ui._message.text.contains("base"), "another tree's base cannot be added: " + ui._message.text)
	_choose(_tree_control(ui, "share.card", "tree_field"), SHARED); await _frames()
	_tree_control(ui, "share.xp", "tree_field").value = nudge_base + 15
	await _click(_tree_control(ui, "share.add")); await _frames()
	var nudge_node: String = ui._selected
	_expect(ui._node(nudge_node).get("shared", false) and ui._node(nudge_node).card.id == SHARED and int(ui._node(nudge_node).xp) == nudge_base + 15, "shared node added with its own XP")
	_expect(ui.draft.edges.any(func(e): return e.from == "base" and e.to == nudge_node), "shared node connects like other nodes")
	_expect(_tree_control(ui, "preview.shared", "tree_field").text.contains("Guard paths"), "details name the other tree: " + _tree_control(ui, "preview.shared", "tree_field").text)
	_expect(str(ui._canvas.tooltips[nudge_node]).contains("also used by: Guard paths"), "hover names the other tree")
	_expect(ui._canvas._nodes[nudge_node].shared, "badge shows in the second tree")
	_expect(not ui._issues.has(nudge_node), "same name in two trees is fine for one shared card ID")
	# Node details edit the tree's XP; a shared card stays priceless.
	await _click(_tree_control(ui, "node.xp.inc")); await _frames()
	_expect(int(ui._node(nudge_node).xp) == nudge_base + 16 and int(ui._node(nudge_node).card.economy.buy) == 0 and _value(ui, "node.xp") == str(nudge_base + 16), "node details XP stepper edits the node XP")
	await _click(_tree_control(ui, "node.xp.dec")); await _frames()
	_expect(int(ui._node(nudge_node).xp) == nudge_base + 15, "node details XP stepper lowers the node XP")
	# Renaming the tree keeps the shared card's own ID.
	_tree_line(ui, "tree.id", "nudge_tree_tmp"); _tree_line(ui, "tree.id", "nudge_tree")
	_expect(ui._node(nudge_node).card.id == SHARED, "renaming the tree keeps a shared card ID")
	# A shared base is rejected locally and by the authority.
	var base_node: Dictionary = ui._node("base")
	base_node.shared = true
	_expect(str(ui._local_issues().get("base", "")).contains("base cannot be a shared"), "local validation rejects a shared base")
	var rejected: Dictionary = runtime.card_trees("validate_card_tree", ui.draft, ui._revision, "adventurer", ui.admin_token)
	_expect(not rejected.get("ok", false) and str(rejected.get("error", "")).contains("base"), "authority rejects a shared base: " + str(rejected.get("error", "")))
	base_node.erase("shared")
	# The same card twice in one tree, or another tree's base as a shared node, is rejected.
	var twice: Dictionary = ui.draft.duplicate(true)
	twice.nodes.append({"id": "node_twice", "card": _shared_node(ui).card.duplicate(true), "x": 0.0, "y": 700.0, "xp": 9, "shared": true})
	twice.edges.append({"id": "edge_twice", "from": "base", "to": "node_twice", "reversible": true, "requirements": []})
	rejected = runtime.card_trees("validate_card_tree", twice, ui._revision, "adventurer", ui.admin_token)
	_expect(not rejected.get("ok", false), "authority rejects the same card twice in one tree")
	var stolen: Dictionary = ui.draft.duplicate(true)
	stolen.nodes.append({"id": "node_stolen", "card": ui.catalog.templates.steady_guard.duplicate(true), "x": 0.0, "y": 700.0, "xp": 9, "shared": true})
	stolen.nodes[-1].card.economy = {"buy": 0, "sell": 0, "copy_limit": 20, "upgrades": []}
	stolen.edges.append({"id": "edge_stolen", "from": "base", "to": "node_stolen", "reversible": true, "requirements": []})
	rejected = runtime.card_trees("validate_card_tree", stolen, ui._revision, "adventurer", ui.admin_token)
	_expect(not rejected.get("ok", false), "authority rejects another tree's base as a shared node")
	await _publish(ui, "tree B")

	# Edit the shared definition from tree B: the notice names both trees.
	var energy := int(ui.catalog.templates[SHARED].cost.energy)
	await _canvas_click(ui, nudge_node)
	await _click(_tree_control(ui, "quick_edit")); await _frames()
	var notice: String = _tree_control(ui, "forge.shared_notice", "tree_field").text
	_expect(notice.contains("updates 2 trees") and notice.contains("Guard paths") and notice.contains("Nudge paths"), "quick edit warns which trees change: " + notice)
	_expect(_value(ui, "forge.xp") == str(nudge_base + 15), "XP stepper shows this tree's XP")
	_expect(not _tree_control(ui, "forge.commit").disabled, "a shared card's own name is not taken by itself")
	await _click(_tree_control(ui, "forge.energy.inc")); await _frames()
	await _click(_tree_control(ui, "forge.commit")); await _frames()
	_expect(int(ui._node(nudge_node).card.economy.buy) == 0 and int(ui._node(nudge_node).xp) == nudge_base + 15, "quick edit keeps a shared card priceless and its tree XP")
	await _click(_tree_control(ui, "edit_card")); await _frames()
	var editor = ui.get_children().filter(func(c): return c.get_script() == ui.CARD_EDITOR)[0]
	_expect(_node(editor, "embedded_notice").text.contains("updates 2 trees"), "full editor warns about every tree")
	await _click(editor._publish_button); await _frames()
	_expect(int(ui._node(nudge_node).card.economy.buy) == 0 and int(ui._node(nudge_node).xp) == nudge_base + 15, "full editor round trip keeps the tree XP and no card price")
	await _publish(ui, "shared edit")
	_expect(ui._message.text.contains("Shared card updates") and ui._message.text.contains("Guard paths"), "publish reports the other trees updated: " + ui._message.text)
	_expect(int(ui.catalog.templates[SHARED].cost.energy) == energy + 1, "catalog shows the new energy")
	_expect(int(_published_node(ui, "guard_tree").card.cost.energy) == energy + 1, "the other tree shows the new energy")
	_expect(int(_published_node(ui, "guard_tree").xp) == guard_base + 3 and int(_published_node(ui, "nudge_tree").xp) == nudge_base + 15, "each tree keeps its own XP")
	_choose(ui._tree_pick, "guard_tree"); await _frames()
	_expect(int(_shared_node(ui).card.cost.energy) == energy + 1 and ui._canvas._nodes[guard_node].xp == guard_base + 3, "reloaded tree A shows the new definition at its XP")
	await _canvas_click(ui, guard_node)
	_expect(_tree_control(ui, "preview.shared", "tree_field").text.contains("Nudge paths"), "badge details list the other tree from tree A")
	await _click(_tree_control(ui, "back")); await _frames()

	# Player trades: copies belong to the tree they came through.
	screen._tabs.current_tab = 5; await _frames(); ui = screen.get_node("CardTreeWorkspace")
	_choose(ui._tree_pick, "guard_tree"); await _frames()
	var xp: int = int(ui.state.progression.xp)
	_expect(ui.state.offers.guard_tree.edge_1.get("tree", "") == "guard_tree", "offers name their tree")
	await _confirm_trade(ui, "edge_1")
	_expect(int(ui._collection_counts().get("guard_tree/" + SHARED, 0)) == 1 and int(ui.state.progression.xp) == xp - 3, "trade into the shared card in tree A: " + ui._message.text)
	_expect(ui._canvas.node_states.get(guard_node) == "owned", "tree A counts its copy as owned")
	_choose(ui._tree_pick, "nudge_tree"); await _frames()
	_expect(ui._canvas.node_states.get(nudge_node) != "owned" and ui._canvas._nodes[nudge_node].stored_count == 0, "tree B does not count tree A's copy")
	xp = int(ui.state.progression.xp)
	await _confirm_trade(ui, "edge_1")
	_expect(int(ui.state.progression.xp) == xp - 15 and int(ui._collection_counts().get("nudge_tree/" + SHARED, 0)) == 1, "trading into it in tree B costs tree B's XP")
	await _canvas_click(ui, nudge_node)
	await _click(_tree_control(ui, "equip." + SHARED)); await _frames()
	_expect(int(ui._counts().get("nudge_tree/" + SHARED, 0)) == 1 and int(ui._counts().get("guard_tree/" + SHARED, 0)) == 0, "equipping in tree B moves only tree B's copy: " + ui._message.text)
	_choose(ui._tree_pick, "guard_tree"); await _frames()
	await _canvas_click(ui, guard_node)
	await _click(_tree_control(ui, "equip." + SHARED)); await _frames()
	_expect(int(ui._counts().get("guard_tree/" + SHARED, 0)) == 1, "equipping in tree A uses tree A's copy: " + ui._message.text)
	await _click(_tree_control(ui, "back")); await _frames()

	# Deck screen: one line per tree, each at its tree's XP; selling refunds that XP.
	screen._tabs.current_tab = 1; await _frames()
	var guard_row := _deck_row(screen, "guard_tree/" + SHARED); var nudge_row := _deck_row(screen, "nudge_tree/" + SHARED)
	_expect(guard_row != null and nudge_row != null, "deck lists the two trees' copies separately")
	if guard_row == null or nudge_row == null: quit(1); return
	_expect(_text(guard_row).contains("via Guard paths") and _text(nudge_row).contains("via Nudge paths"), "deck lines name their trees")
	_expect(_text(guard_row).contains("%d XP" % (guard_base + 3)) and _text(nudge_row).contains("%d XP" % (nudge_base + 15)), "each line has its tree's XP value")
	await _click(nudge_row); await _frames()
	xp = int(screen.catalogs.adventurer.progression.xp)
	var sell := _control(screen, "sell.nudge_tree/" + SHARED)
	_expect(sell != null and sell.text.contains("+%d XP" % (nudge_base + 15)), "sell quotes tree B's XP")
	await _click(sell); await _frames()
	if screen._purchase_overlay.visible: await _click(screen._purchase_confirm); await _frames()
	_expect(int(screen.catalogs.adventurer.progression.xp) == xp + nudge_base + 15, "selling the tree-B copy refunds tree B's XP: " + screen._error.text)
	_expect(screen._card_count("nudge_tree/" + SHARED) == 0 and screen._card_count("guard_tree/" + SHARED) == 1, "the tree-A copy stays")
	# The plain card inspector lists copies by tree.
	screen.inspect_entry("cards", SHARED); await _frames()
	_expect(_control(screen, "copies.guard_tree/" + SHARED) != null and _control(screen, "copies.nudge_tree/" + SHARED) != null, "shared card inspector lists every tree")

	# Admin player sheet: per-tree breakdown.
	await _click(screen._admin_button); await _frames()
	await _click(_control(screen, "admin.player_sheet")); await _frames()
	var sheet = screen.get_node("AdminPlayerSheet")
	_expect(sheet._rows.has("guard_tree/" + SHARED) and sheet._rows.has("nudge_tree/" + SHARED), "admin sheet has a row per tree")
	if sheet._rows.has("guard_tree/" + SHARED):
		_expect(sheet._rows["guard_tree/" + SHARED].get_text(4) == str(guard_base + 3) and sheet._rows["guard_tree/" + SHARED].get_text(5) == "1", "admin row shows tree A's XP and copies")
		_expect(sheet._rows["nudge_tree/" + SHARED].get_text(4) == str(nudge_base + 15) and sheet._rows["nudge_tree/" + SHARED].get_text(5) == "0", "admin row shows tree B's XP and copies")
		sheet._search.text = SHARED; sheet._search.text_changed.emit(SHARED); await _frames()
		var item: TreeItem = sheet._rows["nudge_tree/" + SHARED]
		sheet._table.scroll_to_item(item); await _frames()
		var point: Vector2 = sheet._table.global_position + sheet._table.get_item_area_rect(item).get_center()
		var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true)
		for pressed in [true, false]:
			var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
		await _frames()
		_expect(sheet._selected == "nudge_tree/" + SHARED, "pointer selects one tree's row: " + sheet._selected)
		_expect(_text(sheet._details).contains("COPIES BY TREE") and _text(sheet._details).contains("Guard paths") and _text(sheet._details).contains("Nudge paths"), "admin details break down copies by tree")
	await _click(sheet._back); await _frames()

	# Sharing an owned exclusive variant retires its ID; the refusal is explained.
	await _click(_control(screen, "admin.trees")); await _frames()
	ui = screen.get_node("CardTreeWorkspace")
	_choose(ui._tree_pick, "guard_tree"); await _frames()
	await _canvas_click(ui, "base")
	await _click(_tree_control(ui, "add_down")); await _frames()
	await _click(_tree_control(ui, "forge.commit")); await _frames()
	var cheap: String = ui._selected; var cheap_id: String = ui._node(cheap).card.id
	_expect(int(ui._node(cheap).xp) == guard_base - 2 and not ui._node(cheap).get("shared", false), "cheaper copy gets -2 XP and stays exclusive")
	await _publish(ui, "cheaper variant")
	var progress: Dictionary = runtime.card_trees().result.progression
	var traded: Dictionary = runtime.purchase_progression("adventurer", "tree_card", "steady_guard", int(progress.revision), -2, cheap_id, "guard_tree")
	_expect(traded.get("ok", false), "player owns the exclusive variant: " + str(traded.get("error", "")))
	await _click(_tree_control(ui, "refresh")); await _frames()
	await _canvas_click(ui, cheap)
	_tree_line(ui, "share.id", "cheap_link")
	await _click(_tree_control(ui, "share_card")); await _frames()
	await _click(_tree_control(ui, "validate")); await _frames()
	_expect(ui._publish.disabled and ui._message.text.contains("Could not share") and ui._message.text.contains(cheap_id), "owned copies block sharing with a clear reason: " + ui._message.text)
	ui._dirty = false; await _click(_tree_control(ui, "back")); await _frames()
	screen._close_admin(); screen.queue_free(); await process_frame

	# Sandbox free edits keep tree tags and never merge two trees' copies.
	var sandbox = SCREEN.new(); sandbox.loadout_mode = "sandbox"; root.add_child(sandbox); await _frames()
	sandbox.inspect_entry("cards", SHARED); await _frames()
	await _click(_control(sandbox, "copies.nudge_tree/" + SHARED)); await _frames()
	await _click(_control(sandbox, "add.nudge_tree/" + SHARED)); await _frames()
	_expect(sandbox._card_count("nudge_tree/" + SHARED) == 1 and sandbox._card_count("guard_tree/" + SHARED) == 1, "sandbox adds a copy to one tree's line")
	await _click(sandbox._apply); await _frames()
	var saved: Array = runtime.character_catalogs("sandbox").result.adventurer.get("owned_decklist", [])
	var lines := saved.filter(func(e): return e.card_id == SHARED)
	_expect(lines.size() == 2 and lines.all(func(e): return not str(e.get("tree", "")).is_empty()), "saved deck keeps separate tree-tagged entries: %s %s" % [str(lines), sandbox._error.text])
	sandbox.queue_free(); await process_frame
	print("SHARED CARDS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
