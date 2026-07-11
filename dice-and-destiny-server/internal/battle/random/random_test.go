package random

import "testing"

func TestScriptedNamedRandomRejectsWrongDomainBoundAndExhaustion(t *testing.T) {
	source := &Scripted{Values: []ScriptedValue{{Stream: "combat_dice", Bound: 6, Value: 5}}}
	if _, err := source.IntnNamed("damage_selection", 6); err == nil {
		t.Fatal("wrong stream accepted")
	}
	if source.Cursor != 0 {
		t.Fatal("rejected call consumed script")
	}
	if _, err := source.IntnNamed("combat_dice", 5); err == nil {
		t.Fatal("wrong bound accepted")
	}
	value, err := source.IntnNamed("combat_dice", 6)
	if err != nil || value != 5 {
		t.Fatalf("value=%d err=%v", value, err)
	}
	if err := source.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.IntnNamed("combat_dice", 6); err == nil {
		t.Fatal("exhausted source accepted a call")
	}
}
