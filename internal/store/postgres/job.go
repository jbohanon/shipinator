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
		toUUID(j.ID), toUUID(j.PipelineRunID), j.JobType, j.Name, j.Status, j.CreatedAt, j.StartedAt, j.FinishedAt,
	)
	return err
}

func (s *JobStore) GetByID(ctx context.Context, id store.JobID) (*store.Job, error) {
	var j store.Job
	var idRaw, runIDRaw uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, pipeline_run_id, job_type, name, status, created_at, started_at, finished_at
		 FROM jobs WHERE id = $1`, toUUID(id),
	).Scan(&idRaw, &runIDRaw, &j.JobType, &j.Name, &j.Status, &j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	if err != nil {
		return nil, wrapNoRowsByID("job", id, err)
	}
	j.ID = store.JobID(idRaw)
	j.PipelineRunID = store.PipelineRunID(runIDRaw)
	return &j, nil
}

func (s *JobStore) ListByPipelineRun(ctx context.Context, pipelineRunID store.PipelineRunID) ([]store.Job, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, pipeline_run_id, job_type, name, status, created_at, started_at, finished_at
		 FROM jobs WHERE pipeline_run_id = $1 ORDER BY created_at`, toUUID(pipelineRunID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []store.Job
	for rows.Next() {
		var j store.Job
		var idRaw, runIDRaw uuid.UUID
		if err := rows.Scan(&idRaw, &runIDRaw, &j.JobType, &j.Name, &j.Status, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		j.ID = store.JobID(idRaw)
		j.PipelineRunID = store.PipelineRunID(runIDRaw)
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *JobStore) UpdateStatus(ctx context.Context, id store.JobID, status string) error {
	now := time.Now()
	startedAt, finishedAt := startedFinishedForStatus(status, now)

	result, err := s.pool.Exec(ctx,
		`UPDATE jobs
		 SET status = $1,
		     started_at = COALESCE($2, started_at),
		     finished_at = COALESCE($3, finished_at)
		 WHERE id = $4`,
		status, startedAt, finishedAt, toUUID(id),
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "job", id)
}
