CREATE TABLE executions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_step_id   UUID NOT NULL REFERENCES job_steps(id),
    executor_type TEXT NOT NULL,
    external_id   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX idx_executions_job_step_id ON executions(job_step_id);
