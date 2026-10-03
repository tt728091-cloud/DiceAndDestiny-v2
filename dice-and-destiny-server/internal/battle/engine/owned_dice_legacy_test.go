package engine

import battlerandom "diceanddestiny/server/internal/battle/random"

// Existing tests script faces and effects. Physical identity selection is
// orthogonal to those assertions; Curse tests exercise it with strict scripts.
type ownedSelectionScript battlerandom.Scripted

func (s *ownedSelectionScript) IntnNamed(stream string, n int) (int, error) {
	if stream == "owned_die_selection" {
		return 0, nil
	}
	return (*battlerandom.Scripted)(s).IntnNamed(stream, n)
}
func (s *ownedSelectionScript) AssertExhausted() error {
	return (*battlerandom.Scripted)(s).AssertExhausted()
}
