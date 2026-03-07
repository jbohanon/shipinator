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

func TestRepositoryStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewRepositoryStore(pool)
	ctx := context.Background()

	r := &store.Repository{
		ProjectID:     chain.ProjectID,
		VCSProvider:   "github",
		CloneURL:      "https://github.com/test/repo.git",
		DefaultBranch: "develop",
	}
	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}

	got, err := s.GetByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.VCSProvider != "github" {
		t.Errorf("VCSProvider = %q, want github", got.VCSProvider)
	}
	if got.CloneURL != r.CloneURL {
		t.Errorf("CloneURL = %q, want %q", got.CloneURL, r.CloneURL)
	}
	if got.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want develop", got.DefaultBranch)
	}
}

func TestRepositoryStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewRepositoryStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryStore_ListByProject(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewRepositoryStore(pool)
	ctx := context.Background()

	// chain already created one repo; create another
	r2 := &store.Repository{
		ProjectID:     chain.ProjectID,
		VCSProvider:   "gitlab",
		CloneURL:      "https://gitlab.com/test/repo2.git",
		DefaultBranch: "main",
	}
	if err := s.Create(ctx, r2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	repos, err := s.ListByProject(ctx, chain.ProjectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len = %d, want 2", len(repos))
	}
}

func TestRepositoryStore_Update(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewRepositoryStore(pool)
	ctx := context.Background()

	got, err := s.GetByID(ctx, chain.RepositoryID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	got.DefaultBranch = "develop"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := s.GetByID(ctx, chain.RepositoryID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want develop", updated.DefaultBranch)
	}
}

func TestRepositoryStore_Delete(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewRepositoryStore(pool)
	ps := postgres.NewProjectStore(pool)
	ctx := context.Background()

	// Create a standalone project+repo to avoid FK issues from chain
	p := &store.Project{Name: "repo-delete-test-" + uuid.New().String()[:8]}
	if err := ps.Create(ctx, p); err != nil {
		t.Fatalf("Create project: %v", err)
	}
	r := &store.Repository{
		ProjectID:     p.ID,
		VCSProvider:   "git",
		CloneURL:      "https://example.com/del.git",
		DefaultBranch: "main",
	}
	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(ctx, r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.GetByID(ctx, r.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
