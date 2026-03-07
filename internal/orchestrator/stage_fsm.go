package orchestrator

import (
	"fmt"
	"slices"
)

// StageState represents the lifecycle state of an individual job (build, test,
// or deploy) within a pipeline run.
//
// State machine (TDD §7.2):
//
//	pending → running → succeeded
//	                  → failed
type StageState string

const (
	StageStatePending   StageState = statePending
	StageStateRunning   StageState = stateRunning
	StageStateSucceeded StageState = stateSucceeded // terminal
	StageStateFailed    StageState = stateFailed    // terminal
)

// stageTransitions maps each state to the set of states it may transition into.
// Terminal states have no outgoing transitions.
var stageTransitions = map[StageState][]StageState{
	StageStatePending:   {StageStateRunning},
	StageStateRunning:   {StageStateSucceeded, StageStateFailed},
	StageStateSucceeded: {},
	StageStateFailed:    {},
}

var stageTerminalStates = []StageState{
	StageStateSucceeded,
	StageStateFailed,
}

// ValidateTransition validates that transitioning from the current state to
// target is permitted. Transitioning to the same state is a no-op and always
// succeeds (idempotency for safe retries). Returns ErrInvalidTransition if the
// transition is not in the FSM.
func (current StageState) ValidateTransition(target StageState) error {
	if current == target {
		return nil
	}
	allowed, ok := stageTransitions[current]
	if !ok {
		return fmt.Errorf("stage state %q: %w", current, ErrInvalidTransition)
	}
	if slices.Contains(allowed, target) {
		return nil
	}
	return fmt.Errorf("stage: %q → %q: %w", current, target, ErrInvalidTransition)
}

// IsTerminal reports whether the state is terminal (no further transitions
// are possible).
func (s StageState) IsTerminal() bool {
	return slices.Contains(stageTerminalStates, s)
}
