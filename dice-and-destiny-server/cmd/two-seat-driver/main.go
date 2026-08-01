// Command two-seat-driver is a local developer console for manually driving
// both external seats through the portable battle authority. It does not open
// a socket or bypass normal command validation.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
)

func main() {
	battleID := flag.String("battle-id", "local-blade-mirror", "persistent local battle identifier")
	seed := flag.Uint64("seed", 1, "reproducible battle seed")
	resume := flag.Bool("resume", false, "open an existing battle instead of starting it")
	flag.Parse()

	if !*resume {
		seedValue := *seed
		payload := command.StartBattlePayload{
			Seats: []command.ParticipantDescriptor{
				{InstanceID: "seat-a", DefinitionID: "blade_warden"},
				{InstanceID: "seat-b", DefinitionID: "blade_warden"},
			},
			Seed: &seedValue,
		}
		result := submit(command.Command{BattleID: *battleID, ActorID: "seat-a", Type: command.TypeStartBattle, Payload: mustJSON(payload)})
		if !result.Accepted {
			fmt.Fprintf(os.Stderr, "start battle: %s\n", result.Error)
			os.Exit(1)
		}
		fmt.Printf("Started %s with seed %d.\n", *battleID, *seed)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		acted := false
		for _, actorID := range []string{"seat-a", "seat-b"} {
			view := submit(command.Command{BattleID: *battleID, ActorID: actorID, Type: command.TypeOpenBattle, Payload: json.RawMessage(`{}`)})
			if !view.Accepted {
				fmt.Fprintf(os.Stderr, "open %s: %s\n", actorID, view.Error)
				os.Exit(1)
			}
			if view.Status == engine.ProgressBattleComplete {
				fmt.Printf("Battle complete: %s", view.BattleResult)
				if view.Snapshot != nil && view.Snapshot.WinnerActorID != "" {
					fmt.Printf("; winner %s", view.Snapshot.WinnerActorID)
				}
				fmt.Println()
				return
			}
			if len(view.LegalActions) == 0 {
				continue
			}
			acted = true
			printView(actorID, view)
			for {
				fmt.Printf("%s action [0-%d, q]: ", actorID, len(view.LegalActions)-1)
				line, err := reader.ReadString('\n')
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				line = strings.TrimSpace(line)
				if line == "q" || line == "quit" {
					return
				}
				index, err := strconv.Atoi(line)
				if err != nil || index < 0 || index >= len(view.LegalActions) {
					fmt.Println("Enter one listed action index.")
					continue
				}
				result := submit(view.LegalActions[index])
				if !result.Accepted {
					fmt.Printf("Rejected: %s\n", result.Error)
				} else {
					fmt.Println("Accepted.")
				}
				break
			}
		}
		if !acted {
			fmt.Fprintln(os.Stderr, "battle is active but neither external seat has legal input")
			os.Exit(1)
		}
	}
}

func submit(cmd command.Command) engine.Result {
	var result engine.Result
	if err := json.Unmarshal([]byte(battle.HandleCommand(string(mustJSON(cmd)))), &result); err != nil {
		return engine.Result{Accepted: false, Error: err.Error()}
	}
	return result
}

func printView(actorID string, result engine.Result) {
	fmt.Printf("\n=== %s viewer-safe observation ===\n", actorID)
	pretty, _ := json.MarshalIndent(result.Snapshot, "", "  ")
	fmt.Println(string(pretty))
	fmt.Println("Legal actions:")
	for index, action := range result.LegalActions {
		fmt.Printf("  %d: %s %s\n", index, action.Type, action.Payload)
	}
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
