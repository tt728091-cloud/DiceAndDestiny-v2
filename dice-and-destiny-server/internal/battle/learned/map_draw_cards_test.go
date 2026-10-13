package learned

import (
	"os"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/expedition"
	"diceanddestiny/server/internal/content"
)

// copyAuthored copies the tracked authored content into a disposable root so
// a test reads the published cards without ever writing to them.
func copyAuthored(t *testing.T, root string) string {
	t.Helper()
	t.Setenv(content.AuthoredRootEnv, "")
	dir := t.TempDir()
	entries, err := os.ReadDir(filepath.Join(root, "authored"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "authored", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Every card that draws for its own side in battle can draw on the
// Expedition map too: its map effect is a draw (the published Take Stock
// tree's upgrades were authored before map effects and had none).
func TestEveryBattleDrawCardDrawsOnTheMap(t *testing.T) {
	root := filepath.Join("..", "..", "..", "content")
	catalogs, err := CharacterCatalogs(root, copyAuthored(t, root))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for id, card := range catalogs["starter"].Cards {
		if card.Program == nil {
			continue
		}
		draws := 0
		for _, s := range card.Program.Steps {
			if s.Effect != "draw" || s.Target.Owner == "enemy" {
				continue
			}
			switch v := s.Params["amount"].(type) {
			case float64:
				draws += int(v)
			case int:
				draws += v
			}
		}
		if draws == 0 {
			continue
		}
		checked++
		if card.MapEffect == nil || card.MapEffect.Kind != "draw" || card.MapEffect.Amount < 1 {
			t.Errorf("%s (%s) draws %d in battle but has no map draw: %+v", card.Name, id, draws, card.MapEffect)
		}
	}
	if checked < 5 {
		t.Fatalf("expected Take Stock and its upgrades among the drawing cards, checked %d", checked)
	}
}

// Plenty (Take Stock's upgrade) plays on the map: it costs its printed 5
// energy and draws 3.
func TestPlentyDrawsThreeOnTheMap(t *testing.T) {
	root := filepath.Join("..", "..", "..", "content")
	x, err := loadExpeditionContext(root, copyAuthored(t, root))
	if err != nil {
		t.Fatal(err)
	}
	r := x.rules["starter"]
	plenty := "take_stock_paths_variant_node_1"
	if def := r.Cards[plenty]; def.Name != "Plenty" || def.MapEffect == nil || def.MapEffect.Amount != 3 {
		t.Fatalf("Plenty's map effect: %+v", def.MapEffect)
	}
	deck := []string{plenty, "nudge", "nudge", "try_again", "try_again", "strong_swing", "steady_guard", "steady_guard", "steady_guard", "take_stock", "take_stock", "second_wind"}
	var region string
	for id := range r.Pack.Regions {
		region = id
	}
	s, err := expedition.New(r, region, 3, deck, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	s.Hand = []string{plenty}
	s.Draw = deck[1:]
	if _, err := s.Act(r, expedition.Action{Kind: "play_map_card", Card: plenty}); err != nil {
		t.Fatalf("Plenty is playable on the map: %v", err)
	}
	if s.Energy != 5 || len(s.Hand) != 3 || len(s.Draw) != len(deck)-1-3 {
		t.Fatalf("Plenty costs 5 energy and draws 3: energy %d hand %d draw %d", s.Energy, len(s.Hand), len(s.Draw))
	}
}
