CREATE TABLE job_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES jobs(id),
    name            TEXT NOT NULL,
    execution_order INTEGER,
    parallel_group  TEXT,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_job_steps_job_id ON job_steps(job_id);
