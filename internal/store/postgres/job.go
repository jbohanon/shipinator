package postgres

import (
	"context"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.JobStore = (*JobStore)(nil)

type JobStore struct {
	pool *pgxpool.Pool
}

func NewJobStore(pool *pgxpool.Pool) *JobStore {
	return &JobStore{pool: pool}
}

func (s *JobStore) Create(ctx context.Context, j *store.Job) error {
	ensureID(&j.ID)
	setCreatedAt(&j.CreatedAt)
	setDefaultStatus(&j.Status, "pending")

	_, err := s.pool.Exec(ctx,
		`INSERT INTO jobs (id, pipeline_run_id, job_type, name, status, created_at, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		j.ID, j.PipelineRunID, j.JobType, j.Name, j.Status, j.CreatedAt, j.StartedAt, j.FinishedAt,
	)
	return err
}

func (s *JobStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Job, error) {
	var j store.Job
	err := s.pool.QueryRow(ctx,
		`SELECT id, pipeline_run_id, job_type, name, status, created_at, started_at, finished_at
		 FROM jobs WHERE id = $1`, id,
	).Scan(&j.ID, &j.PipelineRunID, &j.JobType, &j.Name, &j.Status, &j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	if err != nil {
		return nil, wrapNoRowsByID("job", id, err)
	}
	return &j, nil
}

func (s *JobStore) ListByPipelineRun(ctx context.Context, pipelineRunID uuid.UUID) ([]store.Job, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, pipeline_run_id, job_type, name, status, created_at, started_at, finished_at
		 FROM jobs WHERE pipeline_run_id = $1 ORDER BY created_at`, pipelineRunID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []store.Job
	for rows.Next() {
		var j store.Job
		if err := rows.Scan(&j.ID, &j.PipelineRunID, &j.JobType, &j.Name, &j.Status, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *JobStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	startedAt, finishedAt := startedFinishedForStatus(status, now)

	result, err := s.pool.Exec(ctx,
		`UPDATE jobs
		 SET status = $1,
		     started_at = COALESCE($2, started_at),
		     finished_at = COALESCE($3, finished_at)
		 WHERE id = $4`,
		status, startedAt, finishedAt, id,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "job", id)
}
