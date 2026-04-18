#ifndef NATIVE_BATTLE_AUTHORITY_H
#define NATIVE_BATTLE_AUTHORITY_H

#include <godot_cpp/classes/object.hpp>
#include <godot_cpp/core/class_db.hpp>
#include <godot_cpp/variant/string.hpp>

namespace godot {

class NativeBattleAuthority : public Object {
	GDCLASS(NativeBattleAuthority, Object)

	using HandleCommandJSONFn = char *(*)(const char *);
	using FreeCStringFn = void (*)(char *);

	void *go_library_handle = nullptr;
	HandleCommandJSONFn handle_command_json = nullptr;
	FreeCStringFn free_c_string = nullptr;
	String last_load_error;

	bool ensure_go_library_loaded();

protected:
	static void _bind_methods();

public:
	NativeBattleAuthority() = default;
	~NativeBattleAuthority();

	String submit_command(const String &command_json);
};

} // namespace godot

#endif
