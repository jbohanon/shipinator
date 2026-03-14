package artifact

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_artifact.go -package=mocks git.nonahob.net/jacob/shipinator/internal/artifact ArtifactStore

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// Known backend labels. These are recorded in Ref.Backend and in the artifacts
// table (storage_backend column). The filesystem store does not interpret them;
// they are purely descriptive for the caller and the database.
const (
	BackendNFS = "nfs"
)

// Ref is a backend-agnostic reference to stored artifact bytes.
type Ref struct {
	Backend string // e.g. BackendNFS
	Locator string // opaque, backend-specific identifier (filesystem path, S3 key, etc.)
}

// ArtifactStore stores and retrieves artifact bytes.
type ArtifactStore interface {
	Put(ctx context.Context, id uuid.UUID, content io.Reader) (Ref, error)
	Get(ctx context.Context, ref Ref) (io.ReadCloser, error)
}
