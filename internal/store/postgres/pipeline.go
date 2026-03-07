package postgres

import (
	"context"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.PipelineStore = (*PipelineStore)(nil)

type PipelineStore struct {
	pool *pgxpool.Pool
}

func NewPipelineStore(pool *pgxpool.Pool) *PipelineStore {
	return &PipelineStore{pool: pool}
}

func (s *PipelineStore) Create(ctx context.Context, p *store.Pipeline) error {
	ensureID(&p.ID)
	setCreatedUpdated(&p.CreatedAt, &p.UpdatedAt)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO pipelines (id, repository_id, name, trigger_type, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.RepositoryID, p.Name, p.TriggerType, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PipelineStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Pipeline, error) {
	var p store.Pipeline
	err := s.pool.QueryRow(ctx,
		`SELECT id, repository_id, name, trigger_type, created_at, updated_at
		 FROM pipelines WHERE id = $1`, id,
	).Scan(&p.ID, &p.RepositoryID, &p.Name, &p.TriggerType, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, wrapNoRowsByID("pipeline", id, err)
	}
	return &p, nil
}

func (s *PipelineStore) ListByRepository(ctx context.Context, repositoryID uuid.UUID) ([]store.Pipeline, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, repository_id, name, trigger_type, created_at, updated_at
		 FROM pipelines WHERE repository_id = $1 ORDER BY created_at`, repositoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pipelines []store.Pipeline
	for rows.Next() {
		var p store.Pipeline
		if err := rows.Scan(&p.ID, &p.RepositoryID, &p.Name, &p.TriggerType, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, rows.Err()
}

func (s *PipelineStore) Update(ctx context.Context, p *store.Pipeline) error {
	setUpdatedAt(&p.UpdatedAt)
	result, err := s.pool.Exec(ctx,
		`UPDATE pipelines SET repository_id = $1, name = $2, trigger_type = $3, updated_at = $4
		 WHERE id = $5`,
		p.RepositoryID, p.Name, p.TriggerType, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "pipeline", p.ID)
}

func (s *PipelineStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "pipeline", id)
}
