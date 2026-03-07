package postgres

import (
	"context"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ store.ExecutionStore = (*ExecutionStore)(nil)

type ExecutionStore struct {
	pool *pgxpool.Pool
}

func NewExecutionStore(pool *pgxpool.Pool) *ExecutionStore {
	return &ExecutionStore{pool: pool}
}

func (s *ExecutionStore) Create(ctx context.Context, e *store.Execution) error {
	ensureID(&e.ID)
	setCreatedAt(&e.SubmittedAt)
	setDefaultStatus(&e.Status, "pending")

	_, err := s.pool.Exec(ctx,
		`INSERT INTO executions (id, job_step_id, executor_type, external_id, status, submitted_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, e.JobStepID, e.ExecutorType, e.ExternalID, e.Status, e.SubmittedAt, e.CompletedAt,
	)
	return err
}

func (s *ExecutionStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Execution, error) {
	var e store.Execution
	err := s.pool.QueryRow(ctx,
		`SELECT id, job_step_id, executor_type, external_id, status, submitted_at, completed_at
		 FROM executions WHERE id = $1`, id,
	).Scan(&e.ID, &e.JobStepID, &e.ExecutorType, &e.ExternalID, &e.Status, &e.SubmittedAt, &e.CompletedAt)
	if err != nil {
		return nil, wrapNoRowsByID("execution", id, err)
	}
	return &e, nil
}

func (s *ExecutionStore) ListByJobStep(ctx context.Context, jobStepID uuid.UUID) ([]store.Execution, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, job_step_id, executor_type, external_id, status, submitted_at, completed_at
		 FROM executions WHERE job_step_id = $1 ORDER BY submitted_at`, jobStepID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []store.Execution
	for rows.Next() {
		var e store.Execution
		if err := rows.Scan(&e.ID, &e.JobStepID, &e.ExecutorType, &e.ExternalID, &e.Status, &e.SubmittedAt, &e.CompletedAt); err != nil {
			return nil, err
		}
		executions = append(executions, e)
	}
	return executions, rows.Err()
}

func (s *ExecutionStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	completedAt := completedAtForStatus(status, now)

	result, err := s.pool.Exec(ctx,
		`UPDATE executions
		 SET status = $1,
		     completed_at = COALESCE($2, completed_at)
		 WHERE id = $3`,
		status, completedAt, id,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, "execution", id)
}
