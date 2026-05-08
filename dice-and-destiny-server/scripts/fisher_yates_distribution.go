package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

const (
	trials = 100000
	seed   = int64(20260507)
)

func main() {
	rng := rand.New(rand.NewSource(seed))

	permutations := map[string]int{}
	dPositions := [4]int{}
	stayedInStartingSpot := map[string]int{
		"A": 0,
		"B": 0,
		"C": 0,
		"D": 0,
	}
	startingPosition := map[string]int{
		"A": 0,
		"B": 1,
		"C": 2,
		"D": 3,
	}

	for trial := 0; trial < trials; trial++ {
		deck := []string{"A", "B", "C", "D"}
		shuffle(deck, rng)

		permutations[strings.Join(deck, "")]++
		for position, card := range deck {
			if startingPosition[card] == position {
				stayedInStartingSpot[card]++
			}

			if card == "D" {
				dPositions[position]++
			}
		}
	}

	keys := make([]string, 0, len(permutations))
	for key := range permutations {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Printf("Fisher-Yates distribution for [A B C D]\n")
	fmt.Printf("trials: %d\n", trials)
	fmt.Printf("seed: %d\n", seed)
	fmt.Printf("possible permutations seen: %d of 24\n\n", len(keys))

	fmt.Println("Permutation breakdown:")
	for _, key := range keys {
		count := permutations[key]
		fmt.Printf("%s: %4d  %6.2f%%\n", key, count, percent(count))
	}

	fmt.Println("\nD position breakdown:")
	for position, count := range dPositions {
		fmt.Printf("position %d: %4d  %6.2f%%\n", position, count, percent(count))
	}

	fmt.Println("\nStarting spot retention:")
	for _, card := range []string{"A", "B", "C", "D"} {
		count := stayedInStartingSpot[card]
		fmt.Printf("%s stayed at position %d: %4d  %6.2f%%\n", card, startingPosition[card], count, percent(count))
	}
}

func shuffle(deck []string, rng *rand.Rand) {
	for i := len(deck) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

func percent(count int) float64 {
	return float64(count) * 100 / float64(trials)
}
