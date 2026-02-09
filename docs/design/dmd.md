# Shipinator — Data Modeling & Interfaces

This document supplements the Technical Design Document by specifying **persistent storage models**, **artifact storage layout**, **API surface**, **executor contracts**, and **caching considerations** for Shipinator v1.

The goal is to be concrete enough to implement directly, while preserving extensibility and avoiding premature complexity.

---

## 1. Persistent Storage Overview

Shipinator uses a **relational database** as the source of truth for:

* Projects and repositories
* Pipeline/job definitions
* Job executions and state transitions
* Artifact metadata
* Executor interactions

The database represents **intent, state, and history** — not large binary data.

Artifacts themselves are stored in an external artifact store (initially NFS).

---

## 2. Relational Schema

### 2.1 Projects

Represents a logical unit of ownership and isolation.

```sql
projects (
  id                UUID PRIMARY KEY,
  name              TEXT NOT NULL UNIQUE,
  description       TEXT,
  created_at        TIMESTAMP NOT NULL,
  updated_at        TIMESTAMP NOT NULL
)
```

---

### 2.2 Repositories

Maps a VCS repository into Shipinator.

```sql
repositories (
  id                UUID PRIMARY KEY,
  project_id        UUID NOT NULL REFERENCES projects(id),
  vcs_provider      TEXT NOT NULL,           -- e.g. gitea, github
  clone_url         TEXT NOT NULL,
  default_branch    TEXT NOT NULL,
  created_at        TIMESTAMP NOT NULL,
  updated_at        TIMESTAMP NOT NULL
)
```

---

### 2.3 Pipelines

A pipeline is a **logical workflow template** derived from `.shipinator.yaml`.

```sql
pipelines (
  id                UUID PRIMARY KEY,
  repository_id     UUID NOT NULL REFERENCES repositories(id),
  name              TEXT NOT NULL,
  trigger_type      TEXT NOT NULL,           -- pr, push, manual
  created_at        TIMESTAMP NOT NULL,
  updated_at        TIMESTAMP NOT NULL
)
```

---

### 2.4 Pipeline Runs

A concrete execution of a pipeline.

```sql
pipeline_runs (
  id                UUID PRIMARY KEY,
  pipeline_id       UUID NOT NULL REFERENCES pipelines(id),
  git_ref           TEXT NOT NULL,
  git_sha           TEXT NOT NULL,
  status            TEXT NOT NULL,           -- pending, running, success, failed, canceled
  created_at        TIMESTAMP NOT NULL,
  started_at        TIMESTAMP,
  finished_at       TIMESTAMP
)
```

---

### 2.5 Jobs

A pipeline run consists of multiple jobs (build, test, deploy).

```sql
jobs (
  id                UUID PRIMARY KEY,
  pipeline_run_id   UUID NOT NULL REFERENCES pipeline_runs(id),
  job_type          TEXT NOT NULL,           -- build, test, deploy
  name              TEXT NOT NULL,
  status            TEXT NOT NULL,
  created_at        TIMESTAMP NOT NULL,
  started_at        TIMESTAMP,
  finished_at       TIMESTAMP
)
```

---

### 2.6 Job Steps

Represents ordered or parallelized execution units within a job.

```sql
job_steps (
  id                UUID PRIMARY KEY,
  job_id            UUID NOT NULL REFERENCES jobs(id),
  name              TEXT NOT NULL,
  execution_order   INTEGER,
  parallel_group    TEXT,                    -- NULL if sequential
  status            TEXT NOT NULL,
  created_at        TIMESTAMP NOT NULL,
  started_at        TIMESTAMP,
  finished_at       TIMESTAMP
)
```

---

### 2.7 Artifacts (Metadata Only)

```sql
artifacts (
  id                UUID PRIMARY KEY,
  job_id            UUID NOT NULL REFERENCES jobs(id),
  artifact_type     TEXT NOT NULL,            -- binary, image, chart, test-report, coverage
  storage_backend   TEXT NOT NULL,            -- nfs, s3
  storage_path      TEXT NOT NULL,
  checksum          TEXT,
  created_at        TIMESTAMP NOT NULL
)
```

---

### 2.8 Executor Records

Tracks executor submissions and callbacks.

```sql
executions (
  id                UUID PRIMARY KEY,
  job_step_id       UUID NOT NULL REFERENCES job_steps(id),
  executor_type     TEXT NOT NULL,            -- kubernetes
  external_id       TEXT NOT NULL,            -- e.g. k8s job name
  status            TEXT NOT NULL,
  submitted_at      TIMESTAMP NOT NULL,
  completed_at      TIMESTAMP
)
```

---

## 3. Artifact Storage Design

### 3.1 Guiding Principles

* Artifact storage is **content-addressed by UUID**, not by project hierarchy
* The database is the authoritative index
* Storage backends are pluggable

---

### 3.2 Initial NFS Layout

Flat namespace with UUID-based directories:

```
/artifacts/
  <artifact-id>/
    payload
    metadata.json
```

Rationale:

* Avoids deep directory hierarchies
* Simplifies backend transitions
* Prevents path-based coupling to project structure

---

### 3.3 Future S3 Layout (Conceptual)

```
s3://shipinator-artifacts/<artifact-id>/payload
```

The database `storage_path` abstracts this entirely.

---

## 4. API Surface (Internal)

Shipinator exposes a **versioned HTTP API** used by:

* Web UI
* CLI
* Executors (callbacks)

### 4.1 Pipeline Control

* `POST /v1/pipelines/{id}/runs`
* `GET  /v1/pipeline-runs/{id}`
* `GET  /v1/pipeline-runs/{id}/jobs`

---

### 4.2 Artifact Access

* `GET /v1/artifacts/{id}/metadata`
* `GET /v1/artifacts/{id}/download`

---

### 4.3 Executor Callbacks

* `POST /v1/executions/{id}/status`

Payload includes:

```json
{
  "status": "success | failed",
  "exit_code": 0,
  "logs_uri": "...",
  "artifacts": ["artifact-id"]
}
```

---

## 5. Executor Contract

Executors implement a strict interface and are responsible for all platform-specific behavior.

### 5.1 Execution Spec

```go
type ExecutionSpec struct {
  Image       string
  Command     []string
  Env         map[string]string
  Artifacts   []ArtifactSpec
  Timeout     Duration
}
```

---

### 5.2 Executor Interface

```go
type Executor interface {
  Submit(spec ExecutionSpec) (ExecutionHandle, error)
  Status(handle ExecutionHandle) (ExecutionStatus, error)
  Cancel(handle ExecutionHandle) error
}
```

The Kubernetes executor translates this into a `Job` resource internally.

---

### 5.3 Executor Responsibilities

* Clone repository
* Execute declared commands
* Upload artifacts
* Report terminal status

Shipinator **never** interprets Kubernetes concepts directly.

---

## 6. Caching Considerations

### 6.1 Clone & Dependency Caching

* Executors may mount a shared cache volume
* Cache keys derived from:

  * Repository ID
  * Git SHA
  * Toolchain version

Caching is an executor optimization, not a control-plane concern.

---

### 6.2 Artifact Reuse

* Artifacts are immutable
* Reuse is achieved by referencing artifact IDs across pipeline runs

---

### 6.3 Database Caching

* Read-heavy endpoints (pipeline run status) may be cached
* FSM transitions always hit the primary database

---

## 7. Non-Goals (v1)

* Cross-project artifact sharing
* Content-addressable deduplication
* Dynamic DAG scheduling
* Executor autoscaling policies

---

## 8. Summary

This data model reinforces Shipinator’s core philosophy:

* **Relational DB for truth and coordination**
* **Artifact store for bytes**
* **Executors for platform-specific execution**
* **FSM-driven orchestration without DAG complexity**

The design is intentionally conservative, explicit, and evolvable.

