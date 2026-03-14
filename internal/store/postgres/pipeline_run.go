package postgres

import (
	"context"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.PipelineRunStore = (*PipelineRunStore)(nil)

type PipelineRunStore struct {
	pool *pgxpool.Pool
}

func NewPipelineRunStore(pool *pgxpool.Pool) *PipelineRunStore {
	return &PipelineRunStore{pool: pool}
}

func (s *PipelineRunStore) Create(ctx context.Context, r *store.PipelineRun) error {
	ensureID(&r.ID)
	setCreatedAt(&r.CreatedAt)
	setDefaultStatus(&r.Status, "pending")

	_, err := s.pool.Exec(ctx,
		`INSERT INTO pipeline_runs (id, pipeline_id, git_ref, git_sha, status, created_at, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		toUUID(r.ID), toUUID(r.PipelineID), r.GitRef, r.GitSHA, r.Status, r.CreatedAt, r.StartedAt, r.FinishedAt,
	)
	return err
}

func (s *PipelineRunStore) GetByID(ctx context.Context, id store.PipelineRunID) (*store.PipelineRun, error) {
	var r store.PipelineRun
	var idRaw, pipelineIDRaw uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, pipeline_id, git_ref, git_sha, status, created_at, started_at, finished_at
		 FROM pipeline_runs WHERE id = $1`, toUUID(id),
	).Scan(&idRaw, &pipelineIDRaw, &r.GitRef, &r.GitSHA, &r.Status, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if err != nil {
		return nil, wrapNoRowsByID("pipeline run", id, err)
	}
	r.ID = store.PipelineRunID(idRaw)
	r.PipelineID = store.PipelineID(pipelineIDRaw)
	return &r, nil
}

func (s *PipelineRunStore) ListByPipeline(ctx context.Context, pipelineID store.PipelineID) ([]store.PipelineRun, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, pipeline_id, git_ref, git_sha, status, created_at, started_at, finished_at
		 FROM pipeline_runs WHERE pipeline_id = $1 ORDER BY created_at`, toUUID(pipelineID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []store.PipelineRun
	for rows.Next() {
		var r store.PipelineRun
		var idRaw, pipelineIDRaw uuid.UUID
		if err := rows.Scan(&idRaw, &pipelineIDRaw, &r.GitRef, &r.GitSHA, &r.Status, &r.CreatedAt, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		r.ID = store.PipelineRunID(idRaw)
		r.PipelineID = store.PipelineID(pipelineIDRaw)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

func (s *PipelineRunStore) ListByStatus(ctx context.Context, status string) ([]store.PipelineRun, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, pipeline_id, git_ref, git_sha, status, created_at, started_at, finished_at
		 FROM pipeline_runs WHERE status = $1 ORDER BY created_at`, status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []store.PipelineRun
	for rows.Next() {
		var r store.PipelineRun
		var idRaw, pipelineIDRaw uuid.UUID
		if err := rows.Scan(&idRaw, &pipelineIDRaw, &r.GitRef, &r.GitSHA, &r.Status, &r.CreatedAt, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		r.ID = store.PipelineRunID(idRaw)
		r.PipelineID = store.PipelineID(pipelineIDRaw)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

func (s *PipelineRunStore) UpdateStatus(ctx context.Context, id store.PipelineRunID, status string) error {
	now := time.Now()
	startedAt, finishedAt := startedFinishedForStatus(status, now)

	result, err := s.pool.Exec(ctx,
		`UPDATE pipeline_runs
		 SET status = $1,
		     started_at = COALESCE($2, started_at),
		     finished_at = COALESCE($3, finished_at)
		 WHERE id = $4`,
		status, startedAt, finishedAt, toUUID(id),
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "pipeline run", id)
}
