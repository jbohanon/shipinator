package executor

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_executor.go -package=mocks git.nonahob.net/jacob/shipinator/internal/executor Executor

import (
	"context"
	"slices"
	"time"
)

// ArtifactSpec describes an artifact produced by an execution.
type ArtifactSpec struct {
	Type string
	Path string
}

// ExecutionSpec defines a unit of work submitted to an Executor.
type ExecutionSpec struct {
	Image     string
	Command   []string
	Env       map[string]string
	Artifacts []ArtifactSpec
	Timeout   time.Duration
}

// ExecutionHandle is an executor-specific reference to a submitted execution.
type ExecutionHandle struct {
	ID string
}

// ExecutionPhase is the lifecycle phase for an execution.
type ExecutionPhase string

const (
	ExecutionPhasePending   ExecutionPhase = "pending"
	ExecutionPhaseRunning   ExecutionPhase = "running"
	ExecutionPhaseSucceeded ExecutionPhase = "succeeded"
	ExecutionPhaseFailed    ExecutionPhase = "failed"
	ExecutionPhaseCanceled  ExecutionPhase = "canceled"
)

var terminalExecutionPhases = []ExecutionPhase{
	ExecutionPhaseSucceeded,
	ExecutionPhaseFailed,
	ExecutionPhaseCanceled,
}

// ExecutionStatus contains the current state and optional result metadata.
type ExecutionStatus struct {
	Phase      ExecutionPhase
	StartedAt  *time.Time
	FinishedAt *time.Time
	ExitCode   *int
	Message    string
}

// IsTerminal reports whether the execution has reached a terminal phase.
func (s ExecutionStatus) IsTerminal() bool {
	return slices.Contains(terminalExecutionPhases, s.Phase)
}

// Executor is the abstraction used by Shipinator to run remote work.
// Implementations are responsible for translating platform-specific behavior.
type Executor interface {
	Submit(ctx context.Context, spec ExecutionSpec) (ExecutionHandle, error)
	Status(ctx context.Context, handle ExecutionHandle) (ExecutionStatus, error)
	Cancel(ctx context.Context, handle ExecutionHandle) error
}
