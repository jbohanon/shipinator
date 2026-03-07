package postgres

import (
	"context"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.RepositoryStore = (*RepositoryStore)(nil)

type RepositoryStore struct {
	pool *pgxpool.Pool
}

func NewRepositoryStore(pool *pgxpool.Pool) *RepositoryStore {
	return &RepositoryStore{pool: pool}
}

func (s *RepositoryStore) Create(ctx context.Context, r *store.Repository) error {
	ensureID(&r.ID)
	setCreatedUpdated(&r.CreatedAt, &r.UpdatedAt)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO repositories (id, project_id, vcs_provider, clone_url, default_branch, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		r.ID, r.ProjectID, r.VCSProvider, r.CloneURL, r.DefaultBranch, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (s *RepositoryStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Repository, error) {
	var r store.Repository
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, vcs_provider, clone_url, default_branch, created_at, updated_at
		 FROM repositories WHERE id = $1`, id,
	).Scan(&r.ID, &r.ProjectID, &r.VCSProvider, &r.CloneURL, &r.DefaultBranch, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, wrapNoRowsByID("repository", id, err)
	}
	return &r, nil
}

func (s *RepositoryStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]store.Repository, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, vcs_provider, clone_url, default_branch, created_at, updated_at
		 FROM repositories WHERE project_id = $1 ORDER BY created_at`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []store.Repository
	for rows.Next() {
		var r store.Repository
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.VCSProvider, &r.CloneURL, &r.DefaultBranch, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

func (s *RepositoryStore) Update(ctx context.Context, r *store.Repository) error {
	setUpdatedAt(&r.UpdatedAt)
	result, err := s.pool.Exec(ctx,
		`UPDATE repositories SET project_id = $1, vcs_provider = $2, clone_url = $3, default_branch = $4, updated_at = $5
		 WHERE id = $6`,
		r.ProjectID, r.VCSProvider, r.CloneURL, r.DefaultBranch, r.UpdatedAt, r.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "repository", r.ID)
}

func (s *RepositoryStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "repository", id)
}
