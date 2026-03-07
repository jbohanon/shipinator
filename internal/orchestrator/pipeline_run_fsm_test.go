package orchestrator_test

import (
	"errors"
	"testing"

	"git.nonahob.net/jacob/shipinator/internal/orchestrator"
)

func TestTransitionPipelineRun_ValidTransitions(t *testing.T) {
	cases := []struct {
		from, to orchestrator.PipelineRunState
	}{
		{orchestrator.PipelineRunStatePending, orchestrator.PipelineRunStateRunning},
		{orchestrator.PipelineRunStateRunning, orchestrator.PipelineRunStateSuccess},
		{orchestrator.PipelineRunStateRunning, orchestrator.PipelineRunStateFailed},
		{orchestrator.PipelineRunStateRunning, orchestrator.PipelineRunStateCanceled},
		// pending may fail or cancel before ever running
		{orchestrator.PipelineRunStatePending, orchestrator.PipelineRunStateFailed},
		{orchestrator.PipelineRunStatePending, orchestrator.PipelineRunStateCanceled},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if err := tc.from.ValidateTransition(tc.to); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTransitionPipelineRun_TerminalStatesHaveNoTransitions(t *testing.T) {
	terminal := []orchestrator.PipelineRunState{
		orchestrator.PipelineRunStateSuccess,
		orchestrator.PipelineRunStateFailed,
		orchestrator.PipelineRunStateCanceled,
	}
	targets := []orchestrator.PipelineRunState{
		orchestrator.PipelineRunStateRunning,
		orchestrator.PipelineRunStateFailed,
		orchestrator.PipelineRunStateCanceled,
	}

	for _, from := range terminal {
		for _, to := range targets {
			if from == to {
				continue // same-state is always valid (idempotent)
			}
			t.Run(string(from)+"→"+string(to), func(t *testing.T) {
				err := from.ValidateTransition(to)
				if !errors.Is(err, orchestrator.ErrInvalidTransition) {
					t.Errorf("expected ErrInvalidTransition, got %v", err)
				}
			})
		}
	}
}

func TestTransitionPipelineRun_InvalidTransitions(t *testing.T) {
	cases := []struct {
		from, to orchestrator.PipelineRunState
	}{
		// backwards
		{orchestrator.PipelineRunStateRunning, orchestrator.PipelineRunStatePending},
		// pending cannot jump straight to success
		{orchestrator.PipelineRunStatePending, orchestrator.PipelineRunStateSuccess},
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

func TestTransitionPipelineRun_SameStateIsIdempotent(t *testing.T) {
	all := []orchestrator.PipelineRunState{
		orchestrator.PipelineRunStatePending,
		orchestrator.PipelineRunStateRunning,
		orchestrator.PipelineRunStateSuccess,
		orchestrator.PipelineRunStateFailed,
		orchestrator.PipelineRunStateCanceled,
	}

	for _, s := range all {
		t.Run(string(s)+"→"+string(s), func(t *testing.T) {
			if err := s.ValidateTransition(s); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTransitionPipelineRun_UnknownState(t *testing.T) {
	err := orchestrator.PipelineRunState("bogus").ValidateTransition(orchestrator.PipelineRunStateRunning)
	if !errors.Is(err, orchestrator.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestPipelineRunStateIsTerminal(t *testing.T) {
	terminal := []orchestrator.PipelineRunState{
		orchestrator.PipelineRunStateSuccess,
		orchestrator.PipelineRunStateFailed,
		orchestrator.PipelineRunStateCanceled,
	}
	nonTerminal := []orchestrator.PipelineRunState{
		orchestrator.PipelineRunStatePending,
		orchestrator.PipelineRunStateRunning,
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
