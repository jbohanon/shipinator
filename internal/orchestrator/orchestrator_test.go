package orchestrator_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/orchestrator"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
)

// --- fake stores ---

type fakeRunStore struct {
	mu       sync.Mutex
	statuses []string
}

func (f *fakeRunStore) UpdateStatus(_ context.Context, _ uuid.UUID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeRunStore) Create(_ context.Context, _ *store.PipelineRun) error        { return nil }
func (f *fakeRunStore) GetByID(_ context.Context, _ uuid.UUID) (*store.PipelineRun, error) {
	return &store.PipelineRun{ID: uuid.New()}, nil
}
func (f *fakeRunStore) ListByPipeline(_ context.Context, _ uuid.UUID) ([]store.PipelineRun, error) {
	return nil, nil
}
func (f *fakeRunStore) ListByStatus(_ context.Context, _ string) ([]store.PipelineRun, error) {
	return nil, nil
}

type fakeJobStore struct {
	mu       sync.Mutex
	statuses []string
}

func (f *fakeJobStore) UpdateStatus(_ context.Context, _ uuid.UUID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeJobStore) Create(_ context.Context, j *store.Job) error {
	j.ID = uuid.New()
	return nil
}
func (f *fakeJobStore) GetByID(_ context.Context, _ uuid.UUID) (*store.Job, error) {
	return &store.Job{ID: uuid.New()}, nil
}
func (f *fakeJobStore) ListByPipelineRun(_ context.Context, _ uuid.UUID) ([]store.Job, error) {
	return nil, nil
}

type fakeJobStepStore struct {
	mu       sync.Mutex
	statuses []string
}

func (f *fakeJobStepStore) UpdateStatus(_ context.Context, _ uuid.UUID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeJobStepStore) Create(_ context.Context, s *store.JobStep) error {
	s.ID = uuid.New()
	return nil
}
func (f *fakeJobStepStore) GetByID(_ context.Context, _ uuid.UUID) (*store.JobStep, error) {
	return &store.JobStep{ID: uuid.New()}, nil
}
func (f *fakeJobStepStore) ListByJob(_ context.Context, _ uuid.UUID) ([]store.JobStep, error) {
	return nil, nil
}

type fakeExecutionStore struct{}

func (f *fakeExecutionStore) Create(_ context.Context, e *store.Execution) error {
	e.ID = uuid.New()
	return nil
}
func (f *fakeExecutionStore) UpdateStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeExecutionStore) GetByID(_ context.Context, _ uuid.UUID) (*store.Execution, error) {
	return &store.Execution{ID: uuid.New()}, nil
}
func (f *fakeExecutionStore) ListByJobStep(_ context.Context, _ uuid.UUID) ([]store.Execution, error) {
	return nil, nil
}

// --- fake executor ---

// fakeExecutor returns the configured phase on the first Status call after
// Submit. It tracks Cancel calls.
type fakeExecutor struct {
	phase       executor.ExecutionPhase
	cancelCalls int
	mu          sync.Mutex

	// blockUntil is closed when Status should return a terminal result.
	// If nil, Status returns terminal immediately.
	blockUntil chan struct{}
}

func (f *fakeExecutor) Submit(_ context.Context, _ executor.ExecutionSpec) (executor.ExecutionHandle, error) {
	return executor.ExecutionHandle{ID: uuid.New().String()}, nil
}

func (f *fakeExecutor) Status(_ context.Context, _ executor.ExecutionHandle) (executor.ExecutionStatus, error) {
	if f.blockUntil != nil {
		select {
		case <-f.blockUntil:
		default:
			return executor.ExecutionStatus{Phase: executor.ExecutionPhaseRunning}, nil
		}
	}
	return executor.ExecutionStatus{Phase: f.phase}, nil
}

func (f *fakeExecutor) Cancel(_ context.Context, _ executor.ExecutionHandle) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls++
	if f.blockUntil != nil {
		select {
		case <-f.blockUntil:
		default:
			close(f.blockUntil)
		}
	}
	return nil
}

// --- helpers ---

func newOrchestrator(exec executor.Executor, runs *fakeRunStore, jobs *fakeJobStore, steps *fakeJobStepStore) *orchestrator.Orchestrator {
	return orchestrator.New(
		runs,
		jobs,
		steps,
		&fakeExecutionStore{},
		exec,
		orchestrator.Config{
			BuilderImage: "shipinator-builder:latest",
			PollInterval: time.Millisecond, // fast polling for tests
		},
	)
}

func buildTestCfg() *shipcfg.ShipinatorConfig {
	return &shipcfg.ShipinatorConfig{
		Build: &shipcfg.BuildConfig{
			Steps: []shipcfg.BuildStep{
				{Name: "compile", Run: "make build"},
			},
		},
		Test: &shipcfg.TestConfig{
			Steps: []shipcfg.TestStep{
				{Name: "unit", Run: "go test ./..."},
			},
		},
	}
}

// --- tests ---

func TestRun_HappyPath(t *testing.T) {
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}
	exec := &fakeExecutor{phase: executor.ExecutionPhaseSucceeded}

	o := newOrchestrator(exec, runs, jobs, steps)
	err := o.Run(context.Background(), uuid.New(), buildTestCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pipeline run: running, success.
	assertStatuses(t, "pipeline run", runs.statuses, "running", "success")
	// Two stages (build + test): each gets running + succeeded.
	assertStatuses(t, "jobs", jobs.statuses, "running", "succeeded", "running", "succeeded")
	// Two steps: each gets running + succeeded.
	assertStatuses(t, "steps", steps.statuses, "running", "succeeded", "running", "succeeded")
}

func TestRun_ExecutorFailure_FailsRun(t *testing.T) {
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}
	exec := &fakeExecutor{phase: executor.ExecutionPhaseFailed}

	o := newOrchestrator(exec, runs, jobs, steps)
	err := o.Run(context.Background(), uuid.New(), buildTestCfg())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	assertFinalStatus(t, "pipeline run", runs.statuses, "failed")
	assertFinalStatus(t, "jobs", jobs.statuses, "failed")
	assertFinalStatus(t, "steps", steps.statuses, "failed")
}

func TestRun_ContextCanceled_CancelsRun(t *testing.T) {
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}
	exec := &fakeExecutor{
		phase:      executor.ExecutionPhaseSucceeded,
		blockUntil: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	o := newOrchestrator(exec, runs, jobs, steps)
	err := o.Run(ctx, uuid.New(), buildTestCfg())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	assertFinalStatus(t, "pipeline run", runs.statuses, "canceled")
}

func TestRun_BuildOnly_NoTestStage(t *testing.T) {
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}
	exec := &fakeExecutor{phase: executor.ExecutionPhaseSucceeded}

	cfg := &shipcfg.ShipinatorConfig{
		Build: &shipcfg.BuildConfig{
			Steps: []shipcfg.BuildStep{
				{Name: "compile", Run: "make build"},
			},
		},
	}

	o := newOrchestrator(exec, runs, jobs, steps)
	if err := o.Run(context.Background(), uuid.New(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one stage → two job status updates.
	assertStatuses(t, "jobs", jobs.statuses, "running", "succeeded")
}

func TestRun_ParallelTestSteps_AllSucceed(t *testing.T) {
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}
	exec := &fakeExecutor{phase: executor.ExecutionPhaseSucceeded}

	cfg := &shipcfg.ShipinatorConfig{
		Test: &shipcfg.TestConfig{
			Steps: []shipcfg.TestStep{
				{Name: "lint", Run: "golangci-lint run", Parallel: true},
				{Name: "unit", Run: "go test ./...", Parallel: true},
			},
		},
	}

	o := newOrchestrator(exec, runs, jobs, steps)
	if err := o.Run(context.Background(), uuid.New(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two parallel steps → 4 step status updates (running+succeeded each).
	steps.mu.Lock()
	defer steps.mu.Unlock()
	if len(steps.statuses) != 4 {
		t.Errorf("expected 4 step status updates, got %d: %v", len(steps.statuses), steps.statuses)
	}
}

// --- assertion helpers ---

func assertStatuses(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got statuses %v, want %v", label, got, want)
		return
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], w)
		}
	}
}

func assertFinalStatus(t *testing.T, label string, statuses []string, want string) {
	t.Helper()
	if len(statuses) == 0 {
		t.Errorf("%s: no status updates recorded", label)
		return
	}
	if got := statuses[len(statuses)-1]; got != want {
		t.Errorf("%s final status: got %q, want %q", label, got, want)
	}
}
