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

func TestJobStepStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStepStore(pool)
	ctx := context.Background()

	order := 1
	group := "compile"
	js := &store.JobStep{
		JobID:          chain.JobID,
		Name:           "compile-step",
		ExecutionOrder: &order,
		ParallelGroup:  &group,
	}
	if err := s.Create(ctx, js); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if js.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}

	got, err := s.GetByID(ctx, js.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "compile-step" {
		t.Errorf("Name = %q, want compile-step", got.Name)
	}
	if got.ExecutionOrder == nil || *got.ExecutionOrder != 1 {
		t.Errorf("ExecutionOrder = %v, want 1", got.ExecutionOrder)
	}
	if got.ParallelGroup == nil || *got.ParallelGroup != "compile" {
		t.Errorf("ParallelGroup = %v, want compile", got.ParallelGroup)
	}
}

func TestJobStepStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewJobStepStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestJobStepStore_ListByJob(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStepStore(pool)
	ctx := context.Background()

	// chain created one step; create another
	js2 := &store.JobStep{
		JobID: chain.JobID,
		Name:  "lint-step",
	}
	if err := s.Create(ctx, js2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	steps, err := s.ListByJob(ctx, chain.JobID)
	if err != nil {
		t.Fatalf("ListByJob: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("len = %d, want 2", len(steps))
	}
}

func TestJobStepStore_UpdateStatus(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewJobStepStore(pool)
	ctx := context.Background()

	if err := s.UpdateStatus(ctx, chain.JobStepID, "running"); err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	got, err := s.GetByID(ctx, chain.JobStepID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	if err := s.UpdateStatus(ctx, chain.JobStepID, "success"); err != nil {
		t.Fatalf("UpdateStatus success: %v", err)
	}
	got, err = s.GetByID(ctx, chain.JobStepID)
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
