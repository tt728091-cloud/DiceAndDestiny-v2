package boardprice

import (
	"fmt"
	"strings"
)

// SymbolName is a role's symbol name on a die ("Cinder").
func (b *Builder) SymbolName(die Die, role string) string {
	id := die.Layout.Symbols[role]
	if symbol, ok := b.Lib.Symbols[id]; ok {
		return symbol.Name
	}
	return id
}

// Label writes a cost in the die's symbol names: "Cinder ×3/4/5",
// "Cinder ×2 + Spark ×3", "small straight", "small or large straight".
func (b *Builder) Label(die Die, cost Cost) string {
	if straights(cost) {
		var names []string
		for _, t := range cost {
			names = append(names, t.Straight)
		}
		return strings.Join(names, " or ") + " straight"
	}
	if ladder, role := ladderRole(cost); ladder {
		counts := make([]string, len(cost))
		for i, t := range cost {
			counts[i] = fmt.Sprint(t.Counts[0].Count)
		}
		return fmt.Sprintf("%s ×%s", b.SymbolName(die, role), strings.Join(counts, "/"))
	}
	tiers := make([]string, len(cost))
	for i, t := range cost {
		tiers[i] = b.tierLabel(die, t)
	}
	return strings.Join(tiers, " / ")
}

func (b *Builder) tierLabel(die Die, t Tier) string {
	if t.Straight != "" {
		return t.Straight + " straight"
	}
	parts := make([]string, len(t.Counts))
	for i, c := range t.Counts {
		parts[i] = fmt.Sprintf("%s ×%d", b.SymbolName(die, c.Role), c.Count)
	}
	return strings.Join(parts, " + ")
}

// straights reports a cost made only of straights.
func straights(cost Cost) bool {
	for _, t := range cost {
		if t.Straight == "" {
			return false
		}
	}
	return len(cost) > 0
}

// ladderRole reports a cost whose tiers each count the same single role.
func ladderRole(cost Cost) (bool, string) {
	if len(cost) < 2 {
		return false, ""
	}
	role := ""
	for _, t := range cost {
		if t.Straight != "" || len(t.Counts) != 1 || (role != "" && t.Counts[0].Role != role) {
			return false, ""
		}
		role = t.Counts[0].Role
	}
	return true, role
}

// faceNumbers writes a role's faces: "faces 1–3", "face 6".
func faceNumbers(die Die, role string) string {
	faces := die.Layout.Faces[role]
	switch {
	case len(faces) == 1:
		return fmt.Sprintf("face %d", faces[0])
	case len(faces) > 1 && faces[len(faces)-1]-faces[0] == len(faces)-1:
		return fmt.Sprintf("faces %d–%d", faces[0], faces[len(faces)-1])
	}
	parts := make([]string, len(faces))
	for i, f := range faces {
		parts[i] = fmt.Sprint(f)
	}
	return "faces " + strings.Join(parts, ", ")
}

// FaceView is one die face for viewers: number, symbol and role.
type FaceView struct {
	Number int    `json:"number"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Glyph  string `json:"glyph"`
	Role   string `json:"role"`
}

// Faces lists a die's faces with their roles.
func (b *Builder) Faces(die Die) []FaceView {
	out := make([]FaceView, len(die.Faces))
	for i, f := range die.Faces {
		symbol := b.Lib.Symbols[f.Symbol]
		out[i] = FaceView{Number: f.Number, Symbol: f.Symbol, Name: symbol.Name, Glyph: symbol.Glyph, Role: die.Layout.Roles[f.Symbol]}
	}
	return out
}
