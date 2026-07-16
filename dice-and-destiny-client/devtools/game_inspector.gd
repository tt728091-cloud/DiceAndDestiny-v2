extends Node

const DEFAULT_PORT := 0
const LOOPBACK := "127.0.0.1"

var _server := TCPServer.new()
var _connections: Array = []
var _controls: Dictionary = {}
var _errors: Array = []
var _enabled := false
var _port := DEFAULT_PORT
var _token := ""
var _discovery_path := ""

func _ready() -> void:
	_enabled = OS.is_debug_build() and _requested()
	if not _enabled:
		set_process(false)
		return
	_port = int(OS.get_environment("DICE_AND_DESTINY_INSPECTOR_PORT")) if not OS.get_environment("DICE_AND_DESTINY_INSPECTOR_PORT").is_empty() else DEFAULT_PORT
	_token = OS.get_environment("DICE_AND_DESTINY_INSPECTOR_TOKEN")
	if _token.is_empty():
		_token = "%x%x%x" % [Time.get_ticks_usec(), OS.get_process_id(), randi()]
	var error := _server.listen(_port, LOOPBACK)
	if error != OK:
		push_error("[GameInspector] Could not listen on %s:%d: %s" % [LOOPBACK, _port, error_string(error)])
		set_process(false)
		return
	_port = _server.get_local_port()
	_write_discovery_record()
	print("[GameInspector] READY http://%s:%d token=%s" % [LOOPBACK, _port, _token])

func _write_discovery_record() -> void:
	var directory := WorkspacePaths.runtime_dir("inspectors")
	_discovery_path = directory.path_join("%d.json" % OS.get_process_id())
	var file := FileAccess.open(_discovery_path, FileAccess.WRITE)
	if file == null:
		push_error("[GameInspector] Could not write discovery record: %s" % error_string(FileAccess.get_open_error()))
		return
	file.store_string(JSON.stringify({
		"pid": OS.get_process_id(),
		"port": _port,
		"token": _token,
		"project_root": ProjectSettings.globalize_path("res://"),
		"started_unix": Time.get_unix_time_from_system(),
	}))

func _requested() -> bool:
	if OS.get_environment("DICE_AND_DESTINY_INSPECTOR") == "1": return true
	for argument in OS.get_cmdline_args() + OS.get_cmdline_user_args():
		if argument == "--enable-inspector": return true
	return false

func _process(_delta: float) -> void:
	while _server.is_connection_available():
		var peer := _server.take_connection()
		_connections.append({"peer": peer, "buffer": PackedByteArray(), "started": Time.get_ticks_msec()})
	for index in range(_connections.size() - 1, -1, -1):
		var connection: Dictionary = _connections[index]
		var peer: StreamPeerTCP = connection.peer
		if peer.get_status() != StreamPeerTCP.STATUS_CONNECTED:
			_connections.remove_at(index)
			continue
		var available := peer.get_available_bytes()
		if available > 0:
			connection.buffer.append_array(peer.get_data(available)[1])
			_connections[index] = connection
		var request := _parse_request(connection.buffer)
		if not request.is_empty():
			_connections.remove_at(index)
			_handle_request(peer, request)
		elif Time.get_ticks_msec() - int(connection.started) > 5000:
			_connections.remove_at(index)
			_send(peer, 408, {"error": "request timeout"})

func _parse_request(bytes: PackedByteArray) -> Dictionary:
	var text := bytes.get_string_from_utf8()
	var split_at := text.find("\r\n\r\n")
	if split_at < 0: return {}
	var head := text.substr(0, split_at)
	var body := text.substr(split_at + 4)
	var lines := head.split("\r\n")
	if lines.is_empty(): return {}
	var request_line := lines[0].split(" ")
	if request_line.size() < 2: return {}
	var headers := {}
	for line_index in range(1, lines.size()):
		var colon := lines[line_index].find(":")
		if colon > 0: headers[lines[line_index].substr(0, colon).strip_edges().to_lower()] = lines[line_index].substr(colon + 1).strip_edges()
	var content_length := int(headers.get("content-length", "0"))
	if body.to_utf8_buffer().size() < content_length: return {}
	return {"method": request_line[0], "path": request_line[1].split("?")[0].uri_decode(), "headers": headers, "body": body}

func _handle_request(peer: StreamPeerTCP, request: Dictionary) -> void:
	var path := str(request.path)
	if path != "/health" and str(request.headers.get("x-inspector-token", "")) != _token:
		_send(peer, 401, {"error": "unauthorized"})
		return
	match [str(request.method), path]:
		["GET", "/health"]: _send(peer, 200, {"ok": true, "enabled": _enabled, "port": _port, "scene": get_tree().current_scene.name if get_tree().current_scene else ""})
		["GET", "/state"]: _send(peer, 200, _state())
		["GET", "/controls"]: _send(peer, 200, {"controls": _control_list()})
		["GET", "/errors"]: _send(peer, 200, {"errors": _errors})
		["GET", "/screenshot"]: _screenshot_response(peer)
		["POST", "/wait"]: _wait_response(peer, _json_body(request.body))
		["POST", "/battle/new"]:
			var activated := _activate("battle.complete.play_again")
			_send(peer, 200 if activated.ok else 409, activated)
		_:
			if str(request.method) == "POST" and path.begins_with("/controls/") and path.ends_with("/activate"):
				var control_id := path.trim_prefix("/controls/").trim_suffix("/activate")
				var result := _activate(control_id)
				_send(peer, 200 if result.ok else 409, result)
			else: _send(peer, 404, {"error": "route not found", "method": request.method, "path": path})

func register_control(control_id: String, control: Control, description: String = "") -> void:
	if control_id.is_empty() or control == null: return
	control.set_meta("inspection_id", control_id)
	control.set_meta("inspection_description", description)
	_controls[control_id] = weakref(control)

func record_error(message: String, context: Dictionary = {}) -> void:
	_errors.append({"time_msec": Time.get_ticks_msec(), "message": message, "context": context})
	if _errors.size() > 100: _errors.pop_front()

func _battle_screen():
	var screens := get_tree().get_nodes_in_group("inspectable_battle_screen")
	return screens[-1] if not screens.is_empty() else null

func _state() -> Dictionary:
	var screen = _battle_screen()
	if screen != null and screen.has_method("inspection_state"): return screen.inspection_state()
	return {"ready": false, "scene": get_tree().current_scene.name if get_tree().current_scene else "", "controls": _control_list().size(), "errors": _errors.size()}

func _control_list() -> Array:
	var result: Array = []
	var stale: Array = []
	for control_id in _controls:
		var reference: WeakRef = _controls[control_id]
		var control = reference.get_ref()
		if not is_instance_valid(control): stale.append(control_id); continue
		if not control is Control: continue
		var entry := {"id": control_id, "type": control.get_class(), "visible": control.is_visible_in_tree(), "enabled": _control_enabled(control), "focused": control.has_focus(), "bounds": {"x": control.global_position.x, "y": control.global_position.y, "width": control.size.x, "height": control.size.y}, "tooltip": control.tooltip_text, "description": str(control.get_meta("inspection_description", ""))}
		if control is Button: entry["text"] = control.text; entry["pressed"] = control.button_pressed
		result.append(entry)
	for control_id in stale: _controls.erase(control_id)
	result.sort_custom(func(a, b): return str(a.id) < str(b.id))
	return result

func _control_enabled(control: Control) -> bool:
	if not control.is_visible_in_tree(): return false
	if control is BaseButton: return not control.disabled
	return control.mouse_filter != Control.MOUSE_FILTER_IGNORE

func _activate(control_id: String) -> Dictionary:
	if not _controls.has(control_id): return {"ok": false, "error": "control not found", "control_id": control_id}
	var control = (_controls[control_id] as WeakRef).get_ref()
	if not is_instance_valid(control):
		_controls.erase(control_id)
		return {"ok": false, "error": "control is stale", "control_id": control_id}
	if not control is Button: return {"ok": false, "error": "control is not activatable", "control_id": control_id}
	if not _control_enabled(control): return {"ok": false, "error": "control is hidden or disabled", "control_id": control_id}
	if control.toggle_mode:
		control.button_pressed = not control.button_pressed
		control.toggled.emit(control.button_pressed)
	control.pressed.emit()
	return {"ok": true, "control_id": control_id}

func _screenshot_response(peer: StreamPeerTCP) -> void:
	await RenderingServer.frame_post_draw
	var texture := get_viewport().get_texture()
	if texture == null: _send(peer, 503, {"error": "viewport texture unavailable"}); return
	var image := texture.get_image()
	if image == null: _send(peer, 503, {"error": "viewport image unavailable"}); return
	if image.get_format() != Image.FORMAT_RGBA8: image.convert(Image.FORMAT_RGBA8)
	var png := image.save_png_to_buffer()
	_send(peer, 200, {"width": image.get_width(), "height": image.get_height(), "png_base64": Marshalls.raw_to_base64(png)})

func _wait_response(peer: StreamPeerTCP, condition: Dictionary) -> void:
	var timeout_ms := clampi(int(condition.get("timeout_ms", 5000)), 1, 60000)
	var started := Time.get_ticks_msec()
	while Time.get_ticks_msec() - started < timeout_ms:
		var state := _state()
		if _condition_matches(condition, state): _send(peer, 200, {"ok": true, "state": state}); return
		await get_tree().create_timer(0.05).timeout
	_send(peer, 408, {"ok": false, "error": "condition timeout", "condition": condition, "state": _state()})

func _condition_matches(condition: Dictionary, state: Dictionary) -> bool:
	for key in ["segment", "stage", "battle_result", "presentation_type"]:
		if condition.has(key) and state.get(key) != condition[key]: return false
	if condition.has("ready") and bool(state.get("ready", false)) != bool(condition.ready): return false
	if condition.has("control_id"):
		for control in _control_list():
			if control.id == condition.control_id and (not condition.get("enabled", false) or control.enabled): return true
		return false
	return true

func _json_body(body: String) -> Dictionary:
	var parsed = JSON.parse_string(body)
	return parsed if parsed is Dictionary else {}

func _send(peer: StreamPeerTCP, status: int, body) -> void:
	if peer == null or peer.get_status() != StreamPeerTCP.STATUS_CONNECTED: return
	var bytes := JSON.stringify(body).to_utf8_buffer()
	var reason := "OK" if status < 400 else "Error"
	var header := "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n" % [status, reason, bytes.size()]
	peer.put_data(header.to_utf8_buffer())
	peer.put_data(bytes)
	peer.disconnect_from_host()

func _exit_tree() -> void:
	if _server.is_listening(): _server.stop()
	if not _discovery_path.is_empty() and FileAccess.file_exists(_discovery_path):
		DirAccess.remove_absolute(_discovery_path)
