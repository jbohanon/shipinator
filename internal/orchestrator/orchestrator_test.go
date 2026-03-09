package orchestrator_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/executor/mocks"
	"git.nonahob.net/jacob/shipinator/internal/orchestrator"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
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
	ctrl := gomock.NewController(t)
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}

	handle := executor.ExecutionHandle{ID: "test-handle"}
	exec := mocks.NewMockExecutor(ctrl)
	exec.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(handle, nil).Times(2)
	exec.EXPECT().Status(gomock.Any(), gomock.Any()).Return(executor.ExecutionStatus{Phase: executor.ExecutionPhaseSucceeded}, nil).AnyTimes()

	o := newOrchestrator(exec, runs, jobs, steps)
	t.Log("running pipeline: build(compile) + test(unit), executor returns succeeded")
	if err := o.Run(context.Background(), uuid.New(), buildTestCfg()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("pipeline run statuses: %v", runs.statuses)
	t.Logf("job statuses:          %v", jobs.statuses)
	t.Logf("step statuses:         %v", steps.statuses)

	// Pipeline run: running → success.
	assertStatuses(t, "pipeline run", runs.statuses, "running", "success")
	// Two stages (build + test): each gets running + succeeded.
	assertStatuses(t, "jobs", jobs.statuses, "running", "succeeded", "running", "succeeded")
	// Two steps: each gets running + succeeded.
	assertStatuses(t, "steps", steps.statuses, "running", "succeeded", "running", "succeeded")
}

func TestRun_ExecutorFailure_FailsRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}

	handle := executor.ExecutionHandle{ID: "test-handle"}
	exec := mocks.NewMockExecutor(ctrl)
	exec.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(handle, nil).Times(1)
	exec.EXPECT().Status(gomock.Any(), gomock.Any()).Return(executor.ExecutionStatus{Phase: executor.ExecutionPhaseFailed}, nil).AnyTimes()

	o := newOrchestrator(exec, runs, jobs, steps)
	t.Log("running pipeline: executor returns failed, expecting build stage to fail and run to abort")
	err := o.Run(context.Background(), uuid.New(), buildTestCfg())
	t.Logf("Run error: %v", err)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	t.Logf("pipeline run statuses: %v", runs.statuses)
	t.Logf("job statuses:          %v", jobs.statuses)
	t.Logf("step statuses:         %v", steps.statuses)

	assertFinalStatus(t, "pipeline run", runs.statuses, "failed")
	assertFinalStatus(t, "jobs", jobs.statuses, "failed")
	assertFinalStatus(t, "steps", steps.statuses, "failed")
}

func TestRun_ContextCanceled_CancelsRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}

	handle := executor.ExecutionHandle{ID: "test-handle"}
	exec := mocks.NewMockExecutor(ctrl)
	exec.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(handle, nil).Times(1)
	exec.EXPECT().Status(gomock.Any(), gomock.Any()).Return(executor.ExecutionStatus{Phase: executor.ExecutionPhaseRunning}, nil).AnyTimes()
	exec.EXPECT().Cancel(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	o := newOrchestrator(exec, runs, jobs, steps)
	t.Log("running pipeline: context canceled after 5ms while executor blocks on running")
	err := o.Run(ctx, uuid.New(), buildTestCfg())
	t.Logf("Run error: %v", err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	t.Logf("pipeline run statuses: %v", runs.statuses)
	assertFinalStatus(t, "pipeline run", runs.statuses, "canceled")
}

func TestRun_BuildOnly_NoTestStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}

	handle := executor.ExecutionHandle{ID: "test-handle"}
	exec := mocks.NewMockExecutor(ctrl)
	exec.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(handle, nil).Times(1)
	exec.EXPECT().Status(gomock.Any(), gomock.Any()).Return(executor.ExecutionStatus{Phase: executor.ExecutionPhaseSucceeded}, nil).AnyTimes()

	cfg := &shipcfg.ShipinatorConfig{
		Build: &shipcfg.BuildConfig{
			Steps: []shipcfg.BuildStep{
				{Name: "compile", Run: "make build"},
			},
		},
	}

	o := newOrchestrator(exec, runs, jobs, steps)
	t.Log("running pipeline: build only (no test stage)")
	if err := o.Run(context.Background(), uuid.New(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("job statuses: %v", jobs.statuses)
	// Only one stage → two job status updates.
	assertStatuses(t, "jobs", jobs.statuses, "running", "succeeded")
}

func TestRun_ParallelTestSteps_AllSucceed(t *testing.T) {
	ctrl := gomock.NewController(t)
	runs := &fakeRunStore{}
	jobs := &fakeJobStore{}
	steps := &fakeJobStepStore{}

	handle := executor.ExecutionHandle{ID: "test-handle"}
	exec := mocks.NewMockExecutor(ctrl)
	exec.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(handle, nil).Times(2)
	exec.EXPECT().Status(gomock.Any(), gomock.Any()).Return(executor.ExecutionStatus{Phase: executor.ExecutionPhaseSucceeded}, nil).AnyTimes()

	cfg := &shipcfg.ShipinatorConfig{
		Test: &shipcfg.TestConfig{
			Steps: []shipcfg.TestStep{
				{Name: "lint", Run: "golangci-lint run", Parallel: true},
				{Name: "unit", Run: "go test ./...", Parallel: true},
			},
		},
	}

	o := newOrchestrator(exec, runs, jobs, steps)
	t.Log("running pipeline: test stage with 2 parallel steps (lint + unit)")
	if err := o.Run(context.Background(), uuid.New(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	steps.mu.Lock()
	defer steps.mu.Unlock()
	t.Logf("step statuses: %v", steps.statuses)
	// Two parallel steps → 4 step status updates (running+succeeded each).
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
