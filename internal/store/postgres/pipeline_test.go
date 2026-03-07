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

func TestPipelineStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineStore(pool)
	ctx := context.Background()

	p := &store.Pipeline{
		RepositoryID: chain.RepositoryID,
		Name:         "ci-pipeline",
		TriggerType:  "pr",
	}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}

	got, err := s.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "ci-pipeline" {
		t.Errorf("Name = %q, want ci-pipeline", got.Name)
	}
	if got.TriggerType != "pr" {
		t.Errorf("TriggerType = %q, want pr", got.TriggerType)
	}
}

func TestPipelineStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewPipelineStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipelineStore_ListByRepository(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineStore(pool)
	ctx := context.Background()

	// chain already created one pipeline; create another
	p2 := &store.Pipeline{
		RepositoryID: chain.RepositoryID,
		Name:         "deploy-pipeline",
		TriggerType:  "manual",
	}
	if err := s.Create(ctx, p2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	pipelines, err := s.ListByRepository(ctx, chain.RepositoryID)
	if err != nil {
		t.Fatalf("ListByRepository: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("len = %d, want 2", len(pipelines))
	}
}

func TestPipelineStore_Update(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewPipelineStore(pool)
	ctx := context.Background()

	got, err := s.GetByID(ctx, chain.PipelineID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	got.TriggerType = "manual"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := s.GetByID(ctx, chain.PipelineID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.TriggerType != "manual" {
		t.Errorf("TriggerType = %q, want manual", updated.TriggerType)
	}
}

func TestPipelineStore_Delete(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewPipelineStore(pool)
	rs := postgres.NewRepositoryStore(pool)
	ps := postgres.NewProjectStore(pool)
	ctx := context.Background()

	// Create standalone chain to avoid FK issues
	p := &store.Project{Name: "pipe-del-" + uuid.New().String()[:8]}
	if err := ps.Create(ctx, p); err != nil {
		t.Fatalf("Create project: %v", err)
	}
	r := &store.Repository{ProjectID: p.ID, VCSProvider: "git", CloneURL: "https://example.com/d.git", DefaultBranch: "main"}
	if err := rs.Create(ctx, r); err != nil {
		t.Fatalf("Create repo: %v", err)
	}
	pl := &store.Pipeline{RepositoryID: r.ID, Name: "del-me", TriggerType: "push"}
	if err := s.Create(ctx, pl); err != nil {
		t.Fatalf("Create pipeline: %v", err)
	}

	if err := s.Delete(ctx, pl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.GetByID(ctx, pl.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
