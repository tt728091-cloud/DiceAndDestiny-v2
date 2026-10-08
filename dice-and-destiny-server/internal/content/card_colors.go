package content

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Card faces have two configurable colours. The frame colour is the card's
// colour identity, like a trading card's colour; the border is the outer
// edge, usually black or white. Content names a colour below or gives a
// "#rrggbb" hex value. The client draws every tint from these two colours,
// so a new colour needs no art. Keep these tables in step with
// presentation/cards/card_colors.gd in the client.
var CardFrameColors = map[string]string{
	"white":     "#e8e2cf",
	"blue":      "#2368a8",
	"black":     "#2f2b2c",
	"red":       "#c4392c",
	"green":     "#2e7a47",
	"artifact":  "#8f7558",
	"gold":      "#c9a24a",
	"colorless": "#a5a7a6",
}

var CardBorderColors = map[string]string{
	"black":  "#0e0d0d",
	"white":  "#f1eee6",
	"silver": "#b6bac0",
	"gold":   "#e2bd3f",
}

var hexCardColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ValidateCardColors accepts an empty value (the default), a palette name,
// or a "#rrggbb" hex colour for the frame and border.
func ValidateCardColors(p Presentation) error {
	for _, field := range []struct {
		name, value string
		palette     map[string]string
	}{
		{"frame_color", p.FrameColor, CardFrameColors},
		{"border_color", p.BorderColor, CardBorderColors},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" || hexCardColor.MatchString(value) {
			continue
		}
		if _, ok := field.palette[value]; !ok {
			return fmt.Errorf("presentation %s %q must be one of %s or a #rrggbb colour", field.name, value, strings.Join(colorNames(field.palette), ", "))
		}
	}
	return nil
}

func colorNames(palette map[string]string) []string {
	names := make([]string, 0, len(palette))
	for name := range palette {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
