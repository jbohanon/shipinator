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

func TestExecutionStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewExecutionStore(pool)
	ctx := context.Background()

	e := &store.Execution{
		JobStepID:    chain.JobStepID,
		ExecutorType: "kubernetes",
		ExternalID:   "k8s-job-abc123",
	}
	if err := s.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}
	if e.Status != "pending" {
		t.Errorf("Status = %q, want pending", e.Status)
	}

	got, err := s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ExecutorType != "kubernetes" {
		t.Errorf("ExecutorType = %q, want kubernetes", got.ExecutorType)
	}
	if got.ExternalID != "k8s-job-abc123" {
		t.Errorf("ExternalID = %q, want k8s-job-abc123", got.ExternalID)
	}
	if got.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil")
	}
}

func TestExecutionStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewExecutionStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExecutionStore_ListByJobStep(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewExecutionStore(pool)
	ctx := context.Background()

	for i := range 3 {
		e := &store.Execution{
			JobStepID:    chain.JobStepID,
			ExecutorType: "kubernetes",
			ExternalID:   uuid.New().String()[:8],
		}
		if err := s.Create(ctx, e); err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	executions, err := s.ListByJobStep(ctx, chain.JobStepID)
	if err != nil {
		t.Fatalf("ListByJobStep: %v", err)
	}
	if len(executions) != 3 {
		t.Fatalf("len = %d, want 3", len(executions))
	}
}

func TestExecutionStore_UpdateStatus(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewExecutionStore(pool)
	ctx := context.Background()

	e := &store.Execution{
		JobStepID:    chain.JobStepID,
		ExecutorType: "kubernetes",
		ExternalID:   "k8s-job-status-test",
	}
	if err := s.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Transition to running (no completed_at)
	if err := s.UpdateStatus(ctx, e.ID, "running"); err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	got, err := s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil while running")
	}

	// Transition to success
	if err := s.UpdateStatus(ctx, e.ID, "success"); err != nil {
		t.Fatalf("UpdateStatus success: %v", err)
	}
	got, err = s.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "success" {
		t.Errorf("Status = %q, want success", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}
