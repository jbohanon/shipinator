# Shipinator Implementation TODO

## Phase 1: Foundation & Configuration

1. ~~**Define the `.shipinator.yaml` schema as Go types** — Create a `config` package with structs for `BuildConfig`, `TestConfig`, `DeployConfig`, and their nested step/output types. Write a `Load()` function that parses YAML into these structs. Write tests with sample YAML files.~~ ✅

2. ~~**Set up server configuration** — Implement server-level config (listen address, DB connection string, artifact store path, k8s config). Use viper with YAML files and `SHIPINATOR_*` env var overrides. Nested `DBConfig` struct with individual fields (host, port, user, password, name, ssl_mode). Wired into `cmd/server/main.go`.~~ ✅

3. ~~**Choose and add dependencies** — Using `pgx` (via golibs/postgres), golibs migration runner, Echo v4 HTTP router, and `google/uuid`. All in `go.mod`/`go.sum`.~~ ✅

## Phase 2: Database & Data Model

4. ~~**Write SQL migrations** — Full relational schema from the DMD: `projects`, `repositories`, `pipelines`, `pipeline_runs`, `jobs`, `job_steps`, `artifacts`, `executions`. 8 migration files in `migrations/` with FK indexes, TIMESTAMPTZ, gen_random_uuid() PKs. Migration runner wired into server startup.~~ ✅

5. ~~**Implement repository layer** — Create a `store` or `db` package with Go interfaces and Postgres implementations for CRUD on each table. Start with `ProjectStore`, `RepositoryStore`, `PipelineStore`, etc. Write tests against a real or test database.~~ ✅

6. ~~**Implement the Artifact metadata store** — The `artifacts` table operations: create, get-by-id, list-by-job. This is metadata only — the actual bytes come later.~~ ✅

## Phase 3: Domain Core — State Machines

7. ~~**Implement the Stage FSM** — Create an `fsm` or `orchestrator` package. Model the stage states: `PENDING -> RUNNING -> SUCCEEDED | FAILED`. Enforce valid transitions. Write thorough tests for every valid and invalid transition.~~
   
8. ~~**Implement the Job/Pipeline Run FSM** — Model the pipeline run states: `PENDING -> RUNNING -> SUCCESS | FAILED | CANCELED`. Model how stage outcomes drive pipeline run transitions (e.g., stage FAILED -> pipeline run FAILED). Persist transitions via the store layer.~~
   
9. ~~**Implement the Orchestrator** — This is the core coordinator. Given a pipeline run, it should: determine which stages to execute, advance the FSM through `BUILDING -> BUILT -> TESTING -> TESTED -> DEPLOYING -> DEPLOYED`, and react to stage completion callbacks. This drives everything but executes nothing.~~

## Phase 4: Executor Abstraction

10. ~~**Define the Executor interface** — Implement `ExecutionSpec`, `ExecutionHandle`, `ExecutionStatus` types and the `Executor` interface (`Submit`, `Status`, `Cancel`) in an `executor` package.~~ ✅

11. ~~**Implement a mock/local executor** — Before touching Kubernetes, build a simple executor (runs a local command or just logs and returns success) so you can test the full orchestration loop end-to-end without a cluster.~~
    
12. ~~**Implement the Kubernetes executor** — Translate `ExecutionSpec` into a K8s `Job` resource. Handle submission, status polling/watching, and cancellation. Keep all K8s imports and logic contained in this one implementation.~~

## Phase 5: Artifact Storage

13. ~~**Define the ArtifactStore interface** — `Put(ctx, artifact) (ref, error)` and `Get(ctx, ref) (io.ReadCloser, error)` in an `artifact` package. Create a mock for this interface for use in tests using gomock, mirroring the existing mock generation.~~ ✅

14. ~~**Implement the filesystem-backed ArtifactStore** — Use the flat `/<artifact-id>/payload` + `metadata.json` layout from the DMD. Write to a configurable base path; backend label is a constructor parameter so NFS, S3, etc. are operational concerns. Write tests.~~ ✅

## Phase 6: Subsystem Implementations (Buildinator, Testinator, Deployinator)

15. ~~**Implement Buildinator** — Translate build steps from `.shipinator.yaml` into `ExecutionSpec`s. Submit them to the Executor. On completion, register produced artifacts in the store. Handle Go binary, OCI image, and Helm chart build outputs.~~ ✅

16. ~~**Implement Testinator** — Translate test steps into `ExecutionSpec`s. Support sequential and parallel step groups. Parallel logic moved from orchestrator into testinator.~~ ✅

17. ~~**Implement Deployinator** — Accept artifact references (never source). Looks up artifact by type from the build job's store records and translates deploy config into `ExecutionSpec`s that run `helm upgrade` or `kubectl apply`.~~ ✅

## Phase 7: API Layer

18. ~~**Define the protobuf API** — Flesh out `api/v1/shipinator.proto` with messages and services for pipeline control, artifact access, and executor callbacks. Generate Go code.~~ ✅

19. ~~**Implement API handlers** — Wire up the HTTP/gRPC endpoints from the DMD:~~ ✅
    - `POST /v1/pipelines/{id}/runs` — trigger a pipeline run
    - `POST /v1/pipeline-runs/{id}/cancel` — cancel an in-flight run
    - `POST /v1/pipeline-runs/{id}/retry` — retry a failed/canceled run by creating a new run from the same pipeline/ref
    - `GET /v1/pipeline-runs/{id}` — get run status
    - `GET /v1/pipeline-runs/{id}/jobs` — list jobs in a run
    - `GET /v1/artifacts/{id}/metadata` and `/download`
    - `POST /v1/executions/{id}/status` — executor callback
    - Architecture decision: use grpc-gateway style HTTP mappings in `api/v1/shipinator.proto` for control/metadata APIs, and implement `/v1/artifacts/{id}/download` as native streaming HTTP in Echo (not base64 JSON payloads)
    - Retry policy: retry creates a new pipeline run (never mutates a prior run in place)

20. **Wire everything together in `cmd/server/main.go`** — Initialize DB, create stores, create executor, create orchestrator, create subsystems, start HTTP server. This is where the modular monolith comes together.
    - Remaining work: trigger/retry endpoints should dispatch orchestration (initially in-process async), and cancel should propagate to in-flight execution via orchestrator/reconciler integration

## Phase 8: Server Lifecycle & Reliability

21. **Implement graceful shutdown** — Handle `SIGTERM`/`SIGINT`, drain in-flight requests, and cleanly close DB connections.

22. **Implement crash recovery / reconciler loop** — On startup, query for pipeline runs stuck in `RUNNING` state. Re-check executor status and resume or fail them. Continue with a background reconciliation loop that picks up `PENDING` runs and advances them. This is the idempotency guarantee from the TDD.
    - Define deterministic convergence rules for ambiguous states (control-plane crash vs executor completion)
    - Add cancellation propagation from run/job state into active executor handles
    - Enforce retry eligibility and idempotency-key handling for trigger/retry endpoints

23. **Add structured logging** — Use `slog` or similar. Attach `job_id`, `pipeline_run_id`, `stage_type` to log entries. Link logs via `logs_ref`.

## Phase 9: Builder Docker Image

24. **Flesh out `builders/builder.Dockerfile`** — Build an executor worker image containing Go toolchain, Docker (or buildah/kaniko for OCI builds), Helm, and kubectl. This is what K8s Jobs will run.

## Phase 10: CLI & Web UI

25. **Build the CLI** — A `shipinator` command (or separate binary) that talks to the API. Core commands: `trigger`, `status`, `logs`, `list`. Use `cobra` or similar.

26. **Build a minimal web UI** — Show pipeline runs, job status, logs. Keep it simple — server-rendered HTML with Go templates, or a lightweight SPA. This is last because the API must be stable first.

## Phase 11: Dogfooding

27. **Write Shipinator's own `.shipinator.yaml`** — Define build (Go binary + OCI image), test (`go test`), and deploy (to your local cluster) for Shipinator itself.

28. **Deploy Shipinator using Shipinator** — The final milestone. Trigger a pipeline run that builds, tests, and deploys the Shipinator server to your cluster.

---

## Dependency Graph

```
Config parsing (1)
       |
   DB schema (4) + Server config (2-3)
       |
   Store layer (5-6)
       |
   FSMs (7-8) + Executor interface (10)
       |
   Orchestrator (9) + Mock executor (11)
       |
   Subsystems (15-17) + Artifact store (13-14)
       |
   API handlers (18-19) + Wire-up (20)
       |
   K8s executor (12) + Builder image (24)
       |
   Lifecycle (21-23)
       |
   CLI (25) + Web UI (26)
       |
   Dogfood (27-28)
```
