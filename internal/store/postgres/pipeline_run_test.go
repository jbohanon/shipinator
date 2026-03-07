//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"git.nonahob.net/jacob/shipinator/internal/store/postgres"
	"git.nonahob.net/jacob/shipinator/internal/store/storetest"
	"github.com/google/uuid"
)

func TestPipelineRunStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineRunStore(pool)
	ctx := context.Background()

	r := &store.PipelineRun{
		PipelineID: chain.PipelineID,
		GitRef:     "refs/heads/feature",
		GitSHA:     "deadbeef",
	}
	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}
	if r.Status != "pending" {
		t.Errorf("Status = %q, want pending", r.Status)
	}

	got, err := s.GetByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.GitRef != "refs/heads/feature" {
		t.Errorf("GitRef = %q, want refs/heads/feature", got.GitRef)
	}
	if got.StartedAt != nil {
		t.Error("expected StartedAt to be nil")
	}
}

func TestPipelineRunStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewPipelineRunStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipelineRunStore_ListByPipeline(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineRunStore(pool)
	ctx := context.Background()

	// chain created one run; create another
	r2 := &store.PipelineRun{
		PipelineID: chain.PipelineID,
		GitRef:     "refs/heads/other",
		GitSHA:     "cafebabe",
	}
	if err := s.Create(ctx, r2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	runs, err := s.ListByPipeline(ctx, chain.PipelineID)
	if err != nil {
		t.Fatalf("ListByPipeline: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len = %d, want 2", len(runs))
	}
}

func TestPipelineRunStore_ListByStatus(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineRunStore(pool)
	ctx := context.Background()

	// The chain run is "pending"; query for it
	runs, err := s.ListByStatus(ctx, "pending")
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.ID == chain.PipelineRunID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find chain pipeline run in pending list")
	}
}

func TestPipelineRunStore_UpdateStatus(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineRunStore(pool)
	ctx := context.Background()

	// Transition to running
	if err := s.UpdateStatus(ctx, chain.PipelineRunID, "running"); err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	got, err := s.GetByID(ctx, chain.PipelineRunID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if got.FinishedAt != nil {
		t.Error("expected FinishedAt to be nil")
	}

	// Transition to success
	if err := s.UpdateStatus(ctx, chain.PipelineRunID, "success"); err != nil {
		t.Fatalf("UpdateStatus success: %v", err)
	}
	got, err = s.GetByID(ctx, chain.PipelineRunID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "success" {
		t.Errorf("Status = %q, want success", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to still be set")
	}
	if got.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}
