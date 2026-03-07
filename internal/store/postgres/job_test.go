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

func TestJobStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStore(pool)
	ctx := context.Background()

	j := &store.Job{
		PipelineRunID: chain.PipelineRunID,
		JobType:       "test",
		Name:          "unit-tests",
	}
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if j.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}
	if j.Status != "pending" {
		t.Errorf("Status = %q, want pending", j.Status)
	}

	got, err := s.GetByID(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.JobType != "test" {
		t.Errorf("JobType = %q, want test", got.JobType)
	}
	if got.Name != "unit-tests" {
		t.Errorf("Name = %q, want unit-tests", got.Name)
	}
}

func TestJobStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewJobStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestJobStore_ListByPipelineRun(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStore(pool)
	ctx := context.Background()

	// chain created one job; create another
	j2 := &store.Job{
		PipelineRunID: chain.PipelineRunID,
		JobType:       "deploy",
		Name:          "deploy-prod",
	}
	if err := s.Create(ctx, j2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	jobs, err := s.ListByPipelineRun(ctx, chain.PipelineRunID)
	if err != nil {
		t.Fatalf("ListByPipelineRun: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len = %d, want 2", len(jobs))
	}
}

func TestJobStore_UpdateStatus(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStore(pool)
	ctx := context.Background()

	if err := s.UpdateStatus(ctx, chain.JobID, "running"); err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	got, err := s.GetByID(ctx, chain.JobID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	if err := s.UpdateStatus(ctx, chain.JobID, "failed"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	got, err = s.GetByID(ctx, chain.JobID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to still be set")
	}
	if got.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}
