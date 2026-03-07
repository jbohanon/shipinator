package orchestrator_test

import (
	"errors"
	"testing"

	"git.nonahob.net/jacob/shipinator/internal/orchestrator"
)

func TestTransitionStage_ValidTransitions(t *testing.T) {
	cases := []struct {
		from, to orchestrator.StageState
	}{
		{orchestrator.StageStatePending, orchestrator.StageStateRunning},
		{orchestrator.StageStateRunning, orchestrator.StageStateSucceeded},
		{orchestrator.StageStateRunning, orchestrator.StageStateFailed},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if err := tc.from.ValidateTransition(tc.to); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTransitionStage_InvalidTransitions(t *testing.T) {
	cases := []struct {
		from, to orchestrator.StageState
	}{
		// skip running
		{orchestrator.StageStatePending, orchestrator.StageStateSucceeded},
		{orchestrator.StageStatePending, orchestrator.StageStateFailed},
		// backwards
		{orchestrator.StageStateRunning, orchestrator.StageStatePending},
		{orchestrator.StageStateSucceeded, orchestrator.StageStateRunning},
		{orchestrator.StageStateFailed, orchestrator.StageStateRunning},
		// terminal → terminal
		{orchestrator.StageStateSucceeded, orchestrator.StageStateFailed},
		{orchestrator.StageStateFailed, orchestrator.StageStateSucceeded},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			err := tc.from.ValidateTransition(tc.to)
			if !errors.Is(err, orchestrator.ErrInvalidTransition) {
				t.Errorf("expected ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestTransitionStage_SameStateIsIdempotent(t *testing.T) {
	all := []orchestrator.StageState{
		orchestrator.StageStatePending,
		orchestrator.StageStateRunning,
		orchestrator.StageStateSucceeded,
		orchestrator.StageStateFailed,
	}

	for _, s := range all {
		t.Run(string(s)+"→"+string(s), func(t *testing.T) {
			if err := s.ValidateTransition(s); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTransitionStage_UnknownState(t *testing.T) {
	err := orchestrator.StageState("bogus").ValidateTransition(orchestrator.StageStateRunning)
	if !errors.Is(err, orchestrator.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestStageStateIsTerminal(t *testing.T) {
	terminal := []orchestrator.StageState{
		orchestrator.StageStateSucceeded,
		orchestrator.StageStateFailed,
	}
	nonTerminal := []orchestrator.StageState{
		orchestrator.StageStatePending,
		orchestrator.StageStateRunning,
	}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%q: expected terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%q: expected non-terminal", s)
		}
	}
}
