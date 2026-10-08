package content

import (
	"path/filepath"
	"testing"
)

func TestValidateCardColors(t *testing.T) {
	for _, ok := range []Presentation{
		{},
		{FrameColor: "red", BorderColor: "white"},
		{FrameColor: "#12abEF", BorderColor: "#000000"},
	} {
		if err := ValidateCardColors(ok); err != nil {
			t.Fatalf("%+v rejected: %v", ok, err)
		}
	}
	for _, bad := range []Presentation{
		{FrameColor: "purple"},
		{BorderColor: "red"},
		{FrameColor: "#12345"},
		{BorderColor: "000000"},
	} {
		if err := ValidateCardColors(bad); err == nil {
			t.Fatalf("%+v accepted", bad)
		}
	}
}

func TestCardColorsSurviveTheLibrary(t *testing.T) {
	lib, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for id, card := range lib.Cards {
		card.Presentation.FrameColor = "blue"
		card.Presentation.BorderColor = "#336699"
		lib.Cards[id] = card
		if err := validateBattleLibrary(lib); err != nil {
			t.Fatalf("configured colours rejected: %v", err)
		}
		card.Presentation.FrameColor = "purple"
		lib.Cards[id] = card
		if err := validateBattleLibrary(lib); err == nil {
			t.Fatal("unknown frame colour accepted")
		}
		break
	}
}
