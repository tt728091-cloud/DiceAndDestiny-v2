#include "native_battle_authority.h"

#include <dlfcn.h>

#include <godot_cpp/classes/project_settings.hpp>

using namespace godot;

void NativeBattleAuthority::_bind_methods() {
	ClassDB::bind_method(D_METHOD("submit_command", "command_json"), &NativeBattleAuthority::submit_command);
}

NativeBattleAuthority::~NativeBattleAuthority() {
	// Do not dlclose the Go runtime during the spike; unloading Go c-shared libraries can be unsafe.
	go_library_handle = nullptr;
	handle_command_json = nullptr;
	free_c_string = nullptr;
}

String NativeBattleAuthority::submit_command(const String &command_json) {
	if (!ensure_go_library_loaded()) {
		return String("{\"accepted\":false,\"error\":\"") + last_load_error + String("\"}");
	}

	CharString command_utf8 = command_json.utf8();
	char *result = handle_command_json(command_utf8.get_data());
	if (result == nullptr) {
		return "{\"accepted\":false,\"error\":\"Go authority returned null\"}";
	}

	String result_json = String::utf8(result);
	free_c_string(result);
	return result_json;
}

bool NativeBattleAuthority::ensure_go_library_loaded() {
	if (handle_command_json != nullptr && free_c_string != nullptr) {
		return true;
	}

	ProjectSettings *project_settings = ProjectSettings::get_singleton();
	String dylib_path = project_settings->globalize_path("res://native/libbattle_go_authority.dylib");
	CharString dylib_path_utf8 = dylib_path.utf8();

	go_library_handle = dlopen(dylib_path_utf8.get_data(), RTLD_NOW | RTLD_LOCAL);
	if (go_library_handle == nullptr) {
		const char *error = dlerror();
		last_load_error = String("failed to load Go authority dylib: ") + (error == nullptr ? "unknown error" : error);
		return false;
	}

	handle_command_json = reinterpret_cast<HandleCommandJSONFn>(dlsym(go_library_handle, "HandleCommandJSON"));
	free_c_string = reinterpret_cast<FreeCStringFn>(dlsym(go_library_handle, "FreeCString"));

	if (handle_command_json == nullptr || free_c_string == nullptr) {
		const char *error = dlerror();
		last_load_error = String("failed to bind Go authority symbols: ") + (error == nullptr ? "unknown error" : error);
		return false;
	}

	return true;
}
