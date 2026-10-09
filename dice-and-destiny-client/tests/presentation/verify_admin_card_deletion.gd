extends "res://tests/presentation/verify_admin_item_preview.gd"

func _click_dialog(dialog: Window, button: Button) -> void:
	var point := Vector2(dialog.position) + button.get_global_rect().get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed
		root.push_input(event, true); await process_frame
	for frame in 6: await process_frame

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var authoring: Dictionary = runtime.card_authoring()
	var card: Dictionary = authoring.result.templates.steady_guard.duplicate(true)
	card.id = "admin_delete_example"; card.name = "Admin Delete Example"; card.economy.upgrades = []
	var published: Dictionary = runtime.card_authoring("publish_card", card, int(authoring.result.revision))
	_expect(published.get("ok", false), "test card publishes")
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen.inspect_entry("cards", card.id)
	_expect(_control(screen, "admin.delete_card") == null, "ordinary inspector has no deletion control")
	await _click(screen._admin_button)
	await _search(screen, "Admin Delete Example")
	await _hover(screen._admin_prices[card.id].get_parent().get_parent().get_child(0))
	var delete_button: Button = _control(screen, "admin.delete_card")
	_expect(delete_button != null and not delete_button.disabled, "unused custom card can be deleted in admin")
	_expect(screen._admin_item_details.get_parent().get_global_rect().encloses(delete_button.get_global_rect()), "delete action is visible in preview")
	await _capture(screen, "admin-card-delete")
	await _click(delete_button)
	_expect(is_instance_valid(screen._delete_card_dialog) and screen._delete_card_dialog.visible, "pointer opens confirmation")
	_expect(card.name in screen._delete_card_dialog.dialog_text and card.id in screen._delete_card_dialog.dialog_text, "confirmation identifies exact card")
	await _click_dialog(screen._delete_card_dialog, screen._delete_card_dialog.get_cancel_button())
	_expect(runtime.card_authoring().result.templates.has(card.id), "cancel preserves card")
	await _click(delete_button)
	await _click_dialog(screen._delete_card_dialog, screen._delete_card_dialog.get_ok_button())
	_expect(not screen.catalogs.adventurer.cards.has(card.id), "confirmation deletes from live character library")
	_expect(not screen._admin_item_rows.has(card.id), "admin list refreshes after deletion")
	_expect(not runtime.card_authoring().result.templates.has(card.id), "deleted card absent from creator templates")
	await _search(screen, "Steady Guard")
	await _hover(screen._admin_prices.steady_guard.get_parent().get_parent().get_child(0))
	_expect(_control(screen, "admin.delete_card").disabled, "starter/owned card deletion disabled")
	_expect("starter deck" in _text(screen._admin_item_details), "dependency reason visible")
	var token: String = screen._card_admin_token
	await _click(_control(screen, "admin.close"))
	_expect(not runtime.card_admin("admin_delete_card", token, "steady_guard", 0).get("ok", false), "closing admin revokes deletion session")
	await _click(screen._admin_button)
	_expect(not screen._admin_item_rows.has(card.id), "deletion persists after reopening")
	await _click(screen._admin_save)
	_expect(not screen._admin_overlay.visible, "economy save still works after deletion")
	screen.queue_free(); await process_frame
	print("ADMIN CARD DELETION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
