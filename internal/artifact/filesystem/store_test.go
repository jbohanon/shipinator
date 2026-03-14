package filesystem_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"git.nonahob.net/jacob/shipinator/internal/artifact"
	"git.nonahob.net/jacob/shipinator/internal/artifact/filesystem"
	"github.com/google/uuid"
)

func TestStore_Put(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	store := filesystem.New(artifact.BackendNFS, base)

	id := uuid.New()
	content := []byte("hello, artifact")

	ref, err := store.Put(ctx, id, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if ref.Backend != artifact.BackendNFS {
		t.Errorf("ref.Backend = %q, want %q", ref.Backend, artifact.BackendNFS)
	}

	expectedDir := filepath.Join(base, id.String())
	if ref.Locator != expectedDir {
		t.Errorf("ref.Locator = %q, want %q", ref.Locator, expectedDir)
	}

	// Verify payload was written correctly.
	payloadBytes, err := os.ReadFile(filepath.Join(expectedDir, "payload"))
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if !bytes.Equal(payloadBytes, content) {
		t.Errorf("payload = %q, want %q", payloadBytes, content)
	}

	// Verify metadata.json was written and is valid JSON with the right ID.
	metaBytes, err := os.ReadFile(filepath.Join(expectedDir, "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata.json: %v", err)
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ID != id.String() {
		t.Errorf("metadata.id = %q, want %q", meta.ID, id.String())
	}
}

func TestStore_Get(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	store := filesystem.New(artifact.BackendNFS, base)

	id := uuid.New()
	content := []byte("artifact content")

	ref, err := store.Put(ctx, id, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("Get content = %q, want %q", got, content)
	}
}

func TestStore_Get_BackendMismatch(t *testing.T) {
	ctx := context.Background()
	store := filesystem.New(artifact.BackendNFS, t.TempDir())

	_, err := store.Get(ctx, artifact.Ref{Backend: "s3", Locator: "some/path"})
	if err == nil {
		t.Fatal("expected error for backend mismatch, got nil")
	}
}

func TestStore_Get_MissingArtifact(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	store := filesystem.New(artifact.BackendNFS, base)

	ref := artifact.Ref{
		Backend: artifact.BackendNFS,
		Locator:    filepath.Join(base, uuid.New().String()),
	}
	_, err := store.Get(ctx, ref)
	if err == nil {
		t.Fatal("expected error for missing artifact, got nil")
	}
}

func TestStore_Put_EmptyContent(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	store := filesystem.New(artifact.BackendNFS, base)

	id := uuid.New()
	ref, err := store.Put(ctx, id, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("Put with empty content: %v", err)
	}

	rc, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get after empty Put: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty payload, got %q", got)
	}
}
