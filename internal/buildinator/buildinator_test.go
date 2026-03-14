package buildinator_test

import (
	"context"
	"sync"
	"testing"

	"git.nonahob.net/jacob/shipinator/internal/buildinator"
	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
)

// --- fakes ---

type fakeArtifactStore struct {
	mu      sync.Mutex
	created []*store.Artifact
}

func (f *fakeArtifactStore) Create(_ context.Context, a *store.Artifact) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, a)
	return nil
}
func (f *fakeArtifactStore) GetByID(_ context.Context, _ store.ArtifactID) (*store.Artifact, error) {
	return nil, nil
}
func (f *fakeArtifactStore) ListByJob(_ context.Context, _ store.JobID) ([]store.Artifact, error) {
	return nil, nil
}

type stepRecorder struct {
	mu    sync.Mutex
	calls []executor.ExecutionSpec
	err   error
}

func (r *stepRecorder) run(_ context.Context, _ store.JobID, _ string, _ int, _ *string, spec executor.ExecutionSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, spec)
	return r.err
}

// --- tests ---

func TestBuildinator_Run_NoOutputs(t *testing.T) {
	rec := &stepRecorder{}
	arts := &fakeArtifactStore{}
	b := buildinator.New(rec.run, arts, "nfs", "/artifacts", "builder:latest")

	steps := []shipcfg.BuildStep{
		{Name: "compile", Run: "make build"},
	}

	if err := b.Run(context.Background(), store.NewJobID(), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Errorf("step called %d times, want 1", len(rec.calls))
	}
	if len(arts.created) != 0 {
		t.Errorf("artifacts created %d, want 0 (no outputs declared)", len(arts.created))
	}
}

func TestBuildinator_Run_RegistersArtifacts(t *testing.T) {
	rec := &stepRecorder{}
	arts := &fakeArtifactStore{}
	jobID := store.NewJobID()
	b := buildinator.New(rec.run, arts, "nfs", "/artifacts", "builder:latest")

	steps := []shipcfg.BuildStep{
		{
			Name: "build",
			Run:  "make build",
			Outputs: []shipcfg.BuildOutput{
				{Type: "binary", Path: "./bin/server"},
				{Type: "oci_image", Ref: "registry/server:latest"},
			},
		},
	}

	if err := b.Run(context.Background(), jobID, steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(arts.created) != 2 {
		t.Fatalf("artifacts created %d, want 2", len(arts.created))
	}

	for _, a := range arts.created {
		if a.JobID != jobID {
			t.Errorf("artifact JobID = %v, want %v", a.JobID, jobID)
		}
		if a.StorageBackend != "nfs" {
			t.Errorf("StorageBackend = %q, want %q", a.StorageBackend, "nfs")
		}
		if a.ID == (store.ArtifactID{}) {
			t.Error("artifact ID is zero")
		}
		if a.StoragePath == "" {
			t.Error("StoragePath is empty")
		}
	}

	types := map[string]bool{}
	for _, a := range arts.created {
		types[a.ArtifactType] = true
	}
	if !types["binary"] || !types["oci_image"] {
		t.Errorf("artifact types = %v, want binary and oci_image", types)
	}
}

func TestBuildinator_Run_MultipleSteps(t *testing.T) {
	rec := &stepRecorder{}
	arts := &fakeArtifactStore{}
	b := buildinator.New(rec.run, arts, "nfs", "/artifacts", "builder:latest")

	steps := []shipcfg.BuildStep{
		{Name: "step1", Run: "cmd1", Outputs: []shipcfg.BuildOutput{{Type: "binary", Path: "./a"}}},
		{Name: "step2", Run: "cmd2", Outputs: []shipcfg.BuildOutput{{Type: "helm_chart", Path: "./chart"}}},
	}

	if err := b.Run(context.Background(), store.NewJobID(), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(rec.calls) != 2 {
		t.Errorf("step called %d times, want 2", len(rec.calls))
	}
	if len(arts.created) != 2 {
		t.Errorf("artifacts created %d, want 2", len(arts.created))
	}
}

func TestBuildinator_Run_StepFailure_NoArtifacts(t *testing.T) {
	rec := &stepRecorder{err: context.DeadlineExceeded}
	arts := &fakeArtifactStore{}
	b := buildinator.New(rec.run, arts, "nfs", "/artifacts", "builder:latest")

	steps := []shipcfg.BuildStep{
		{Name: "build", Run: "make", Outputs: []shipcfg.BuildOutput{{Type: "binary", Path: "./bin"}}},
	}

	if err := b.Run(context.Background(), store.NewJobID(), steps); err == nil {
		t.Fatal("expected error from failed step, got nil")
	}

	if len(arts.created) != 0 {
		t.Errorf("artifacts created %d, want 0 after step failure", len(arts.created))
	}
}
