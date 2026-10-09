extends "res://tests/presentation/verify_admin_item_preview.gd"

func _select(choice: OptionButton, index: int) -> void:
	choice.select(index); choice.item_selected.emit(index)
	for frame in 6: await process_frame

func _filter(screen, type_id: String) -> void:
	for index in screen._admin_type_filter.item_count:
		if str(screen._admin_type_filter.get_item_metadata(index)) == type_id:
			await _select(screen._admin_type_filter, index); return
	_expect(false, "missing type filter " + type_id)

func _visible_ids(screen) -> Array[String]:
	var ids: Array[String] = []
	for row in screen._admin_items.get_children():
		if row.visible and row.has_meta("entry_id"): ids.append(str(row.get_meta("entry_id")))
	return ids

func _ordered(screen, kind: String, grouped: bool) -> void:
	var previous := ""; var previous_type := ""
	for id in _visible_ids(screen):
		var caption: String = screen.catalogs[screen.character_id][kind][id].name
		var pool: String = screen._type_name(screen._draft_type(kind, id))
		if grouped: _expect(previous_type.naturalnocasecmp_to(pool) <= 0, "type groups are alphabetical")
		if not grouped or pool == previous_type: _expect(previous.naturalnocasecmp_to(caption) <= 0, "names alphabetical within sort category")
		previous = caption; previous_type = pool

func _run() -> void:
	root.gui_embed_subwindows = true; root.size = Vector2i(1920, 1080)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	await _click(screen._admin_button)
	var original := JSON.stringify(screen._admin_draft)
	for tab in [0, 1]:
		screen._admin_tabs.current_tab = tab
		for frame in 5: await process_frame
		var kind := "cards" if tab == 0 else "abilities"
		await _filter(screen, "")
		await _select(screen._admin_sort, 0)
		_ordered(screen, kind, false)
		_expect(_visible_ids(screen).size() == screen.catalogs[screen.character_id][kind].size(), "alphabetical mode includes every item")
		# Check pointer access, then activate the option through its selection signal.
		await _click(screen._admin_sort)
		_expect(screen._admin_sort.get_popup().visible, "sort dropdown opens on click")
		screen._admin_sort.get_popup().hide()
		await _select(screen._admin_sort, 1)
		_expect(screen._admin_sort_by_type, "combined sort selection applies")
		_ordered(screen, kind, true)
		for type_id in screen.catalogs[screen.character_id].access.types:
			await _filter(screen, str(type_id))
			var ids := _visible_ids(screen)
			var expected := 0
			for id in screen.catalogs[screen.character_id][kind]:
				if screen._draft_type(kind, id) == type_id: expected += 1
			_expect(ids.size() == expected, "filter contains exactly " + str(type_id) + " " + kind)
			_ordered(screen, kind, true)
			for id in ids: _expect(screen._draft_type(kind, id) == type_id, "filter excludes other types")
		await _filter(screen, "venom")
		await _search(screen, "zz-no-match")
		_expect(screen._admin_empty.visible and _visible_ids(screen).is_empty(), "search combines with type filter and has empty state")
		await _search(screen, "")
		var first: String = _visible_ids(screen)[0]
		await _search(screen, str(screen.catalogs[screen.character_id][kind][first].name))
		_expect(first in _visible_ids(screen), "search retains matching type/name")
		await _hover(screen._admin_item_rows[first].get_child(0))
		_expect(screen._admin_item_id == first, "preview works in filtered/grouped list")
		await _search(screen, "")
		for width in [1024, 1280, 1920]:
			root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
			for frame in 6: await process_frame
			_expect(root.get_visible_rect().encloses(screen._admin_sort.get_global_rect()), "sort fits viewport")
			_expect(root.get_visible_rect().encloses(screen._admin_type_filter.get_global_rect()), "type filter fits viewport")
			_expect(root.get_visible_rect().encloses(screen._admin_save.get_global_rect()), "save still fits")
			await _capture(screen, "admin-sort-%s-%d" % [kind, width])
	_expect(JSON.stringify(screen._admin_draft) == original, "browsing never changes admin draft")
	# Editing a draft type immediately repositions/refilters the same controls.
	screen._admin_tabs.current_tab = 0
	await _filter(screen, "general"); await _search(screen, "Steady Guard")
	await _set_price(screen._admin_prices.steady_guard, 12)
	var retained: SpinBox = screen._admin_prices.steady_guard
	var type_choice: OptionButton = screen._admin_types.cards.steady_guard
	for index in type_choice.item_count:
		if str(type_choice.get_item_metadata(index)) == "venom": await _select(type_choice, index); break
	_expect(not "steady_guard" in _visible_ids(screen), "retagged card leaves the General filter")
	await _filter(screen, "venom")
	_expect("steady_guard" in _visible_ids(screen), "retagged card enters the Venom filter")
	await _select(screen._admin_sort, 0)
	_expect(screen._admin_prices.steady_guard == retained and int(retained.value) == 12, "sorting retains edits and control identity")
	await _click(screen._admin_save)
	await _click(screen._admin_button)
	_expect(screen._admin_sort.selected == 0 and screen._admin_filter_type == "venom", "browse choices survive reopening")
	_expect(int(screen._admin_prices.steady_guard.value) == 12 and screen._draft_type("cards", "steady_guard") == "venom", "edits save correctly after sorting")
	await _click(_control(screen, "admin.close"))
	screen.queue_free(); await process_frame
	print("ADMIN SORTING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
