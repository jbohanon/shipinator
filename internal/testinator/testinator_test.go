package testinator_test

import (
	"context"
	"sync"
	"testing"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"git.nonahob.net/jacob/shipinator/internal/testinator"
)

type stepRecorder struct {
	mu     sync.Mutex
	names  []string
	groups []*string
	err    error
}

func (r *stepRecorder) run(_ context.Context, _ store.JobID, name string, _ int, group *string, _ executor.ExecutionSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
	r.groups = append(r.groups, group)
	return r.err
}

func TestTestinator_Sequential(t *testing.T) {
	rec := &stepRecorder{}
	tn := testinator.New(rec.run, "builder:latest")

	steps := []shipcfg.TestStep{
		{Name: "lint", Run: "golangci-lint run"},
		{Name: "unit", Run: "go test ./..."},
	}

	if err := tn.Run(context.Background(), store.NewJobID(), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.names) != 2 {
		t.Fatalf("step called %d times, want 2", len(rec.names))
	}
	if rec.names[0] != "lint" || rec.names[1] != "unit" {
		t.Errorf("step order: got %v, want [lint unit]", rec.names)
	}
	for _, g := range rec.groups {
		if g != nil {
			t.Errorf("sequential steps should have nil group, got %q", *g)
		}
	}
}

func TestTestinator_Parallel(t *testing.T) {
	rec := &stepRecorder{}
	tn := testinator.New(rec.run, "builder:latest")

	steps := []shipcfg.TestStep{
		{Name: "lint", Run: "golangci-lint run", Parallel: true},
		{Name: "unit", Run: "go test ./...", Parallel: true},
	}

	if err := tn.Run(context.Background(), store.NewJobID(), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.names) != 2 {
		t.Fatalf("step called %d times, want 2", len(rec.names))
	}
	for _, g := range rec.groups {
		if g == nil {
			t.Error("parallel steps should have a non-nil group")
		}
	}
	// Both parallel steps should share the same group name.
	if *rec.groups[0] != *rec.groups[1] {
		t.Errorf("parallel steps have different groups: %q vs %q", *rec.groups[0], *rec.groups[1])
	}
}

func TestTestinator_MixedSequentialParallel(t *testing.T) {
	rec := &stepRecorder{}
	tn := testinator.New(rec.run, "builder:latest")

	steps := []shipcfg.TestStep{
		{Name: "seq1", Run: "cmd1"},
		{Name: "par1", Run: "cmd2", Parallel: true},
		{Name: "par2", Run: "cmd3", Parallel: true},
		{Name: "seq2", Run: "cmd4"},
	}

	if err := tn.Run(context.Background(), store.NewJobID(), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.names) != 4 {
		t.Fatalf("step called %d times, want 4", len(rec.names))
	}
}

func TestTestinator_StepFailure(t *testing.T) {
	rec := &stepRecorder{err: context.DeadlineExceeded}
	tn := testinator.New(rec.run, "builder:latest")

	steps := []shipcfg.TestStep{
		{Name: "unit", Run: "go test ./..."},
	}

	if err := tn.Run(context.Background(), store.NewJobID(), steps); err == nil {
		t.Fatal("expected error, got nil")
	}
}
