package segment

import "testing"

func TestInitialStateIsDeterministic(t *testing.T) {
	got := NewManager().InitialState()

	want := State{
		Current: OngoingEffects,
		Round:   1,
	}

	if got != want {
		t.Fatalf("InitialState() = %#v, want %#v", got, want)
	}
}

func TestOrderIsDeterministic(t *testing.T) {
	got := Order()
	want := []Segment{
		OngoingEffects,
		Income,
		Offensive,
		Defensive,
		DamageResolution,
	}

	if len(got) != len(want) {
		t.Fatalf("Order() length = %d, want %d: %#v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Order()[%d] = %q, want %q; full order: %#v", i, got[i], want[i], got)
		}
	}

	got[0] = DamageResolution
	if Order()[0] != OngoingEffects {
		t.Fatalf("Order() returned mutable package order")
	}
}

func TestAdvanceFromEachSegment(t *testing.T) {
	tests := []struct {
		name          string
		state         State
		wantState     State
		wantCompleted bool
	}{
		{
			name:          "ongoing effects to income",
			state:         State{Current: OngoingEffects, Round: 1},
			wantState:     State{Current: Income, Round: 1},
			wantCompleted: false,
		},
		{
			name:          "income to offensive",
			state:         State{Current: Income, Round: 1},
			wantState:     State{Current: Offensive, Round: 1},
			wantCompleted: false,
		},
		{
			name:          "offensive to defensive",
			state:         State{Current: Offensive, Round: 1},
			wantState:     State{Current: Defensive, Round: 1},
			wantCompleted: false,
		},
		{
			name:          "defensive to damage resolution",
			state:         State{Current: Defensive, Round: 1},
			wantState:     State{Current: DamageResolution, Round: 1},
			wantCompleted: false,
		},
		{
			name:          "damage resolution starts next turn",
			state:         State{Current: DamageResolution, Round: 1},
			wantState:     State{Current: OngoingEffects, Round: 2},
			wantCompleted: true,
		},
	}

	manager := NewManager()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotAdvance, err := manager.Advance(tt.state)
			if err != nil {
				t.Fatalf("Advance(%#v) returned error: %v", tt.state, err)
			}

			if gotState != tt.wantState {
				t.Fatalf("Advance(%#v) state = %#v, want %#v", tt.state, gotState, tt.wantState)
			}

			if gotAdvance.From != tt.state.Current {
				t.Fatalf("Advance(%#v) From = %q, want %q", tt.state, gotAdvance.From, tt.state.Current)
			}

			if gotAdvance.To != tt.wantState.Current {
				t.Fatalf("Advance(%#v) To = %q, want %q", tt.state, gotAdvance.To, tt.wantState.Current)
			}

			if gotAdvance.Round != tt.wantState.Round {
				t.Fatalf("Advance(%#v) Round = %d, want %d", tt.state, gotAdvance.Round, tt.wantState.Round)
			}

			if gotAdvance.CompletedTurn != tt.wantCompleted {
				t.Fatalf("Advance(%#v) CompletedTurn = %v, want %v", tt.state, gotAdvance.CompletedTurn, tt.wantCompleted)
			}
		})
	}
}

func TestRoundIncrementsOnlyWhenNextTurnStarts(t *testing.T) {
	manager := NewManager()
	state := manager.InitialState()

	for _, want := range []State{
		{Current: Income, Round: 1},
		{Current: Offensive, Round: 1},
		{Current: Defensive, Round: 1},
		{Current: DamageResolution, Round: 1},
		{Current: OngoingEffects, Round: 2},
	} {
		var err error
		state, _, err = manager.Advance(state)
		if err != nil {
			t.Fatalf("Advance() returned error: %v", err)
		}

		if state != want {
			t.Fatalf("advanced state = %#v, want %#v", state, want)
		}
	}
}

func TestRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name  string
		state State
	}{
		{
			name:  "unknown segment",
			state: State{Current: Segment("planning"), Round: 1},
		},
		{
			name:  "empty segment",
			state: State{Current: "", Round: 1},
		},
		{
			name:  "zero round",
			state: State{Current: OngoingEffects, Round: 0},
		},
		{
			name:  "negative round",
			state: State{Current: OngoingEffects, Round: -1},
		},
	}

	manager := NewManager()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotAdvance, err := manager.Advance(tt.state)
			if err == nil {
				t.Fatalf("Advance(%#v) succeeded with state %#v and advance %#v", tt.state, gotState, gotAdvance)
			}
		})
	}
}

func TestManagerDrivesBattleStateThroughCompleteTurnWithoutTransport(t *testing.T) {
	type battleState struct {
		BattleID string
		Segment  State
	}

	manager := NewManager()
	battle := battleState{
		BattleID: "battle-1",
		Segment:  manager.InitialState(),
	}

	var completedTurns int
	for i := 0; i < len(Order()); i++ {
		next, advance, err := manager.Advance(battle.Segment)
		if err != nil {
			t.Fatalf("Advance(%#v) returned error: %v", battle.Segment, err)
		}

		battle.Segment = next
		if advance.CompletedTurn {
			completedTurns++
		}
	}

	if completedTurns != 1 {
		t.Fatalf("completed turns = %d, want 1", completedTurns)
	}

	want := battleState{
		BattleID: "battle-1",
		Segment: State{
			Current: OngoingEffects,
			Round:   2,
		},
	}

	if battle != want {
		t.Fatalf("battle state = %#v, want %#v", battle, want)
	}
}
