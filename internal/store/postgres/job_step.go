package postgres

import (
	"context"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.JobStepStore = (*JobStepStore)(nil)

type JobStepStore struct {
	pool *pgxpool.Pool
}

func NewJobStepStore(pool *pgxpool.Pool) *JobStepStore {
	return &JobStepStore{pool: pool}
}

func (s *JobStepStore) Create(ctx context.Context, js *store.JobStep) error {
	ensureID(&js.ID)
	setCreatedAt(&js.CreatedAt)
	setDefaultStatus(&js.Status, "pending")

	_, err := s.pool.Exec(ctx,
		`INSERT INTO job_steps (id, job_id, name, execution_order, parallel_group, status, created_at, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		toUUID(js.ID), toUUID(js.JobID), js.Name, js.ExecutionOrder, js.ParallelGroup, js.Status, js.CreatedAt, js.StartedAt, js.FinishedAt,
	)
	return err
}

func (s *JobStepStore) GetByID(ctx context.Context, id store.JobStepID) (*store.JobStep, error) {
	var js store.JobStep
	var idRaw, jobIDRaw uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, job_id, name, execution_order, parallel_group, status, created_at, started_at, finished_at
		 FROM job_steps WHERE id = $1`, toUUID(id),
	).Scan(&idRaw, &jobIDRaw, &js.Name, &js.ExecutionOrder, &js.ParallelGroup, &js.Status, &js.CreatedAt, &js.StartedAt, &js.FinishedAt)
	if err != nil {
		return nil, wrapNoRowsByID("job step", id, err)
	}
	js.ID = store.JobStepID(idRaw)
	js.JobID = store.JobID(jobIDRaw)
	return &js, nil
}

func (s *JobStepStore) ListByJob(ctx context.Context, jobID store.JobID) ([]store.JobStep, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, job_id, name, execution_order, parallel_group, status, created_at, started_at, finished_at
		 FROM job_steps WHERE job_id = $1 ORDER BY created_at`, toUUID(jobID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []store.JobStep
	for rows.Next() {
		var js store.JobStep
		var idRaw, jobIDRaw uuid.UUID
		if err := rows.Scan(&idRaw, &jobIDRaw, &js.Name, &js.ExecutionOrder, &js.ParallelGroup, &js.Status, &js.CreatedAt, &js.StartedAt, &js.FinishedAt); err != nil {
			return nil, err
		}
		js.ID = store.JobStepID(idRaw)
		js.JobID = store.JobID(jobIDRaw)
		steps = append(steps, js)
	}
	return steps, rows.Err()
}

func (s *JobStepStore) UpdateStatus(ctx context.Context, id store.JobStepID, status string) error {
	now := time.Now()
	startedAt, finishedAt := startedFinishedForStatus(status, now)

	result, err := s.pool.Exec(ctx,
		`UPDATE job_steps
		 SET status = $1,
		     started_at = COALESCE($2, started_at),
		     finished_at = COALESCE($3, finished_at)
		 WHERE id = $4`,
		status, startedAt, finishedAt, toUUID(id),
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "job step", id)
}
