CREATE TABLE artifacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES jobs(id),
    artifact_type   TEXT NOT NULL,
    storage_backend TEXT NOT NULL,
    storage_path    TEXT NOT NULL,
    checksum        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_artifacts_job_id ON artifacts(job_id);
