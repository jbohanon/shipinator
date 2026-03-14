package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/artifact"
	"github.com/google/uuid"
)

// metadata is written alongside the payload to capture basic artifact info.
type metadata struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// Store writes artifact bytes to a local filesystem path. The underlying
// filesystem (local disk, NFS mount, etc.) is an operational concern; this
// package does not distinguish between them.
type Store struct {
	backend  string
	basePath string
}

// New creates a Store rooted at basePath. backend is recorded in the returned
// Ref and should match the storage_backend value used in the artifacts table
// (e.g. artifact.BackendNFS).
func New(backend, basePath string) *Store {
	return &Store{backend: backend, basePath: basePath}
}

// Put writes content to <basePath>/<id>/payload and writes metadata.json
// alongside it. The returned Ref records the backend label and directory path.
func (s *Store) Put(ctx context.Context, id uuid.UUID, content io.Reader) (artifact.Ref, error) {
	dir := filepath.Join(s.basePath, id.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return artifact.Ref{}, fmt.Errorf("create artifact dir: %w", err)
	}

	payloadPath := filepath.Join(dir, "payload")
	f, err := os.Create(payloadPath)
	if err != nil {
		return artifact.Ref{}, fmt.Errorf("create payload file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return artifact.Ref{}, fmt.Errorf("write payload: %w", err)
	}

	meta := metadata{
		ID:        id.String(),
		CreatedAt: time.Now().UTC(),
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return artifact.Ref{}, fmt.Errorf("marshal metadata: %w", err)
	}
	metaPath := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(metaPath, metaBytes, 0o644); err != nil {
		return artifact.Ref{}, fmt.Errorf("write metadata: %w", err)
	}

	return artifact.Ref{
		Backend: s.backend,
		Locator: dir,
	}, nil
}

// Get opens and returns the payload for the artifact at ref.
func (s *Store) Get(ctx context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	if ref.Backend != s.backend {
		return nil, fmt.Errorf("backend mismatch: store is %q, ref is %q", s.backend, ref.Backend)
	}

	payloadPath := filepath.Join(ref.Locator, "payload")
	f, err := os.Open(payloadPath)
	if err != nil {
		return nil, fmt.Errorf("open payload: %w", err)
	}
	return f, nil
}
