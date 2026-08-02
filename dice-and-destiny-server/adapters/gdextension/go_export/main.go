package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/learned"
)

//export HandleCommandJSON
func HandleCommandJSON(command *C.char) *C.char {
	if command == nil {
		return C.CString(`{"accepted":false,"error":"nil command JSON"}`)
	}

	result := battle.HandleCommand(C.GoString(command))
	return C.CString(result)
}

//export HandleLearnedBattleJSON
func HandleLearnedBattleJSON(request *C.char) *C.char {
	if request == nil {
		return C.CString(`{"accepted":false,"ok":false,"error":"nil learned battle request"}`)
	}
	return C.CString(learned.HandleRuntimeRequest(C.GoString(request)))
}

//export FreeCString
func FreeCString(value *C.char) {
	if value == nil {
		return
	}
	C.free(unsafe.Pointer(value))
}

func main() {}
