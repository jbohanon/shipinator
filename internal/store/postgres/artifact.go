package postgres

import (
	"context"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.ArtifactStore = (*ArtifactStore)(nil)

type ArtifactStore struct {
	pool *pgxpool.Pool
}

func NewArtifactStore(pool *pgxpool.Pool) *ArtifactStore {
	return &ArtifactStore{pool: pool}
}

func (s *ArtifactStore) Create(ctx context.Context, a *store.Artifact) error {
	ensureID(&a.ID)
	setCreatedAt(&a.CreatedAt)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO artifacts (id, job_id, artifact_type, storage_backend, storage_path, checksum, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.JobID, a.ArtifactType, a.StorageBackend, a.StoragePath, a.Checksum, a.CreatedAt,
	)
	return err
}

func (s *ArtifactStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Artifact, error) {
	var a store.Artifact
	err := s.pool.QueryRow(ctx,
		`SELECT id, job_id, artifact_type, storage_backend, storage_path, checksum, created_at
		 FROM artifacts WHERE id = $1`, id,
	).Scan(&a.ID, &a.JobID, &a.ArtifactType, &a.StorageBackend, &a.StoragePath, &a.Checksum, &a.CreatedAt)
	if err != nil {
		return nil, wrapNoRowsByID("artifact", id, err)
	}
	return &a, nil
}

func (s *ArtifactStore) ListByJob(ctx context.Context, jobID uuid.UUID) ([]store.Artifact, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, job_id, artifact_type, storage_backend, storage_path, checksum, created_at
		 FROM artifacts WHERE job_id = $1 ORDER BY created_at`, jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []store.Artifact
	for rows.Next() {
		var a store.Artifact
		if err := rows.Scan(&a.ID, &a.JobID, &a.ArtifactType, &a.StorageBackend, &a.StoragePath, &a.Checksum, &a.CreatedAt); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}
