extends "res://tests/presentation/verify_card_trees.gd"

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen); await _frames()
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		await create_timer(0.25).timeout
	await _click(screen._admin_button); await _frames()
	await _click(_control(screen, "admin.trees")); await _frames()
	if not screen.has_node("CardTreeWorkspace"): quit(1); return
	var ui = screen.get_node("CardTreeWorkspace")
	_tree_line(ui, "tree.id", "template_tree"); _tree_line(ui, "tree.name", "Template Tree")
	_choose(ui._base_pick, "brace"); await _click(_tree_control(ui, "base")); await _frames()
	var base: Dictionary = ui._node("base").duplicate(true)
	var source: Dictionary = ui.catalog.templates.brace_plus.duplicate(true)
	await _click(_tree_control(ui, "add_up")); await _frames()
	await _click(_tree_control(ui, "forge.commit")); await _frames()
	var node_id: String = ui._selected
	var original: Dictionary = ui._node(node_id).duplicate(true)
	var edges: Array = ui.draft.edges.duplicate(true)
	_expect(_tree_control(ui, "node.apply_template").disabled, "applying requires an explicit template selection")
	# Switching across different programs must replace all settings, not merge stale fields.
	for template_id in ["strong_swing", "brace_plus"]:
		_choose(_tree_control(ui, "node.template", "tree_field"), template_id)
		await _click(_tree_control(ui, "node.apply_template")); await _frames()
		var expected: Dictionary = ui.catalog.templates[template_id].duplicate(true)
		expected.id = original.card.id; expected.name = ui._node(node_id).card.name; expected.economy.buy = maxi(1, int(expected.economy.buy)); expected.economy.sell = expected.economy.buy; expected.economy.upgrades = []
		_expect(JSON.stringify(ui._node(node_id).card) == JSON.stringify(expected), "complete independent settings copied from " + template_id)
		_expect(ui._node(node_id).x == original.x and ui._node(node_id).y == original.y and ui.draft.edges == edges, "identity, position and graph preserved")
	_expect(ui._node("base") == base and ui.catalog.templates.brace_plus == source, "base and source definitions unchanged")
	_expect(ui._dirty and ui._publish.disabled, "template changes require validation and publication")
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9.0 / 16.0)); await _frames()
		var apply := _tree_control(ui, "node.apply_template")
		var scroll: ScrollContainer = ui._details.get_parent(); scroll.ensure_control_visible(apply); await _frames()
		_expect(root.get_visible_rect().encloses(apply.get_global_rect()), "template control fits viewport")
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(); DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
			root.get_texture().get_image().save_png("res://.godot/layout-review/card-tree-template-%d.png" % width)
	await _click(_tree_control(ui, "validate")); await _frames()
	_expect(not ui._publish.disabled, "copied template validates: " + ui._message.text)
	if ui._publish.disabled: quit(1); return
	await _click(ui._publish); await _frames()
	_expect(ui._message.text.begins_with("Published"), "template tree publishes: " + ui._message.text)
	await _click(_tree_control(ui, "refresh")); await _frames()
	_expect(ui._node(node_id).card.name == source.name + " · Template Tree" and ui._node(node_id).card.program == source.program, "published variant survives reload")
	_expect(ui.catalog.templates.brace_plus == source, "published source stays unchanged")
	await _click(_tree_control(ui, "back")); await _frames()
	screen._tabs.current_tab = 5; await _frames(); ui = screen.get_node("CardTreeWorkspace")
	_expect(not ui._admin(), "player view remains non-admin")
	for c in ui.find_children("*", "Control", true, false):
		_expect(c.get_meta("tree_action", "") != "node.apply_template", "template control is admin-only")
	var xp: int = int(ui.state.progression.xp)
	var difference: int = int(source.economy.buy) - int(base.card.economy.buy)
	await _confirm_trade(ui, "edge_1")
	_expect(ui._collection_counts().get(original.card.id, 0) == 1 and int(ui.state.progression.xp) == xp - difference, "upgrade obtains copied variant at its configured XP difference")
	await _click(_tree_control(ui, "equip." + str(original.card.id))); await _frames()
	_expect(ui._counts().get(original.card.id, 0) == 1, "copied upgrade can be equipped")
	screen.queue_free(); await process_frame
	print("CARD TREE TEMPLATES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
