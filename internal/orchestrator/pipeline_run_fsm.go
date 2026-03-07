package orchestrator

import (
	"fmt"
	"slices"
)

// PipelineRunState represents the coarse lifecycle state of a pipeline run.
// The orchestrator advances runs through this FSM as stage outcomes arrive.
//
// State machine
//
//	pending → running → success
//	                  → failed
//	                  → canceled
//
// pending may also transition directly to failed or canceled if the run
// cannot start (e.g., validation failure, pre-run cancellation).
type PipelineRunState string

const (
	PipelineRunStatePending  PipelineRunState = statePending
	PipelineRunStateRunning  PipelineRunState = stateRunning
	PipelineRunStateSuccess  PipelineRunState = stateSuccess  // terminal
	PipelineRunStateFailed   PipelineRunState = stateFailed   // terminal
	PipelineRunStateCanceled PipelineRunState = stateCanceled // terminal
)

// pipelineRunTransitions maps each state to the set of states it may transition into.
// Terminal states have no outgoing transitions.
var pipelineRunTransitions = map[PipelineRunState][]PipelineRunState{
	PipelineRunStatePending:  {PipelineRunStateRunning, PipelineRunStateFailed, PipelineRunStateCanceled},
	PipelineRunStateRunning:  {PipelineRunStateSuccess, PipelineRunStateFailed, PipelineRunStateCanceled},
	PipelineRunStateSuccess:  {},
	PipelineRunStateFailed:   {},
	PipelineRunStateCanceled: {},
}

var terminalStates = []PipelineRunState{
	PipelineRunStateSuccess,
	PipelineRunStateFailed,
	PipelineRunStateCanceled,
}

// ValidateTransition validates that transitioning from the current state to
// target is permitted. Transitioning to the same state is a no-op and always
// succeeds (idempotency for safe retries). Returns ErrInvalidTransition if the
// transition is not in the FSM.
func (current PipelineRunState) ValidateTransition(target PipelineRunState) error {
	if current == target {
		return nil
	}
	allowed, ok := pipelineRunTransitions[current]
	if !ok {
		return fmt.Errorf("pipeline run state %q: %w", current, ErrInvalidTransition)
	}
	if slices.Contains(allowed, target) {
		return nil
	}
	return fmt.Errorf("pipeline run: %q → %q: %w", current, target, ErrInvalidTransition)
}

// IsTerminal reports whether the state is terminal (no further transitions
// are possible).
func (s PipelineRunState) IsTerminal() bool {
	return slices.Contains(terminalStates, s)
}
