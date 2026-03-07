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

func TestArtifactStore_CreateAndGetByID(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewArtifactStore(pool)
	ctx := context.Background()

	checksum := "sha256:abc123"
	a := &store.Artifact{
		JobID:          chain.JobID,
		ArtifactType:   "binary",
		StorageBackend: "nfs",
		StoragePath:    "/artifacts/test/binary",
		Checksum:       &checksum,
	}
	if err := s.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == uuid.Nil {
		t.Fatal("expected ID to be set")
	}

	got, err := s.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ArtifactType != "binary" {
		t.Errorf("ArtifactType = %q, want binary", got.ArtifactType)
	}
	if got.StorageBackend != "nfs" {
		t.Errorf("StorageBackend = %q, want nfs", got.StorageBackend)
	}
	if got.Checksum == nil || *got.Checksum != checksum {
		t.Errorf("Checksum = %v, want %q", got.Checksum, checksum)
	}
}

func TestArtifactStore_GetByID_NotFound(t *testing.T) {
	pool := storetest.NewTestPool(t)
	s := postgres.NewArtifactStore(pool)

	_, err := s.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestArtifactStore_ListByJob(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewArtifactStore(pool)
	ctx := context.Background()

	for i, aType := range []string{"binary", "oci_image", "coverage"} {
		a := &store.Artifact{
			JobID:          chain.JobID,
			ArtifactType:   aType,
			StorageBackend: "nfs",
			StoragePath:    "/artifacts/test/" + aType,
		}
		if err := s.Create(ctx, a); err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	artifacts, err := s.ListByJob(ctx, chain.JobID)
	if err != nil {
		t.Fatalf("ListByJob: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("len = %d, want 3", len(artifacts))
	}
}

func TestArtifactStore_NullChecksum(t *testing.T) {
	pool := storetest.NewTestPool(t)
	chain := storetest.CreateEntityChain(t, pool)
	s := postgres.NewArtifactStore(pool)
	ctx := context.Background()

	a := &store.Artifact{
		JobID:          chain.JobID,
		ArtifactType:   "test_report",
		StorageBackend: "s3",
		StoragePath:    "/artifacts/test/report",
	}
	if err := s.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Checksum != nil {
		t.Errorf("Checksum = %v, want nil", got.Checksum)
	}
}
