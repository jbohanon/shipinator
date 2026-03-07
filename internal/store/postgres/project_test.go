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

func TestProjectStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)
	ctx := context.Background()

	desc := "a test project"
	p := &store.Project{Name: "my-project", Description: &desc}

	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	got, err := s.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("Description = %v, want %q", got.Description, desc)
	}
}

func TestProjectStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectStore_GetByName(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)
	ctx := context.Background()

	p := &store.Project{Name: "find-me"}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetByName(ctx, "find-me")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID = %s, want %s", got.ID, p.ID)
	}
}

func TestProjectStore_GetByName_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)

	_, err := s.GetByName(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectStore_List(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)
	ctx := context.Background()

	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if err := s.Create(ctx, &store.Project{Name: name}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	projects, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("len = %d, want 3", len(projects))
	}
	if projects[0].Name != "alpha" {
		t.Errorf("first project = %q, want alpha", projects[0].Name)
	}
}

func TestProjectStore_Update(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)
	ctx := context.Background()

	p := &store.Project{Name: "original"}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	p.Name = "updated"
	desc := "now with description"
	p.Description = &desc
	if err := s.Update(ctx, p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "updated" {
		t.Errorf("Name = %q, want updated", got.Name)
	}
	if got.UpdatedAt.Before(got.CreatedAt) {
		t.Error("UpdatedAt should be >= CreatedAt")
	}
}

func TestProjectStore_Delete(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewProjectStore(pool)
	ctx := context.Background()

	p := &store.Project{Name: "to-delete"}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.GetByID(ctx, p.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
