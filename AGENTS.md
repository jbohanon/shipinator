# AGENTS.md — Shipinator

A guide for agents working on this codebase. Read this before touching anything.

---

## What it is

Shipinator is a self-hosted CI/CD system written in Go. It reads a `.shipinator.yaml` from a repo, runs build → test → deploy stages by submitting Kubernetes Jobs, and persists all state to Postgres. The ultimate goal (phase 11) is for Shipinator to build and deploy itself.

---

## Architecture

The system is a modular monolith. Everything lives in one binary (`cmd/server`), but internal packages have hard dependency boundaries enforced by import paths. There is no shared global state.

### Data flow for a pipeline run

```
PipelineRun (store) ──► Orchestrator.Run()
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         Buildinator     Testinator      Deployinator
              │               │               │
              └───────────────┴───────────────┘
                              │
                         runStep() ◄── owned by Orchestrator
                              │
                         Executor.Submit() ──► Kubernetes Job
                              │
                         poll() until terminal
                              │
                         store.Execution updated
```

Each subsystem receives a `StepFn` from the orchestrator — a closure over `o.runStep`. This inversion of control keeps subsystems free of store/executor dependencies.

### Key packages

| Package | Responsibility |
|---|---|
| `internal/config` | Parse and validate `.shipinator.yaml` into typed structs |
| `internal/store` | Interfaces + typed ID types for all DB entities |
| `internal/store/postgres` | pgx v5 implementations of every store interface |
| `internal/store/storetest` | Integration test helpers: `NewTestPool`, `CreateEntityChain` |
| `internal/executor` | `Executor` interface + `ExecutionSpec/Handle/Status` types |
| `internal/executor/kubernetes` | K8s Job translation; all k8s imports live here only |
| `internal/executor/mocks` | gomock for `Executor` (go:generate in executor.go) |
| `internal/artifact` | `ArtifactStore` interface + `Ref` type; `BackendNFS` constant |
| `internal/artifact/filesystem` | Filesystem-backed `ArtifactStore`; NFS/local is operational config |
| `internal/artifact/mocks` | gomock for `ArtifactStore` |
| `internal/orchestrator` | Pipeline run lifecycle, FSMs, step polling; delegates stage logic |
| `internal/buildinator` | Build steps → executor submissions → artifact DB registration |
| `internal/testinator` | Test steps → sequential/parallel executor submissions |
| `internal/deployinator` | Resolves build artifact from store → deploy executor submission |
| `internal/server` | Echo HTTP server wiring |
| `internal/server/config` | Server-level config (DB, listen addr, artifact path, k8s) |

---

## Design principles — the ones that matter

**1. Abstraction layers must not leak operational details.**
The `artifact/filesystem` package does not know or care whether it is writing to local disk or NFS. NFS is an operational concern (mount path configuration). The backend label (`artifact.BackendNFS = "nfs"`) is passed in at construction time, not hardcoded. Same principle applies everywhere: K8s job names, DB connection strings, storage paths — all injected, never embedded.

**2. Distinct typed IDs, not raw UUIDs.**
All entity IDs are distinct types (`type JobID uuid.UUID`, not `= uuid.UUID`). The compiler rejects passing a `JobID` where a `PipelineRunID` is expected. Constructors: `store.NewJobID()`. String conversion: `id.String()`. In postgres implementations, `toUUID(id)` at bind sites; intermediate `uuid.UUID` vars at scan sites (pgx v5 cannot encode custom types).

**3. Subsystems own their logic, orchestrator owns lifecycle.**
Sequential/parallel step batching lives in `testinator`, not the orchestrator. Artifact registration after build steps lives in `buildinator`. Deploy artifact resolution lives in `deployinator`. The orchestrator only manages job creation, FSM transitions, and step polling.

**4. Locator, not path.**
`artifact.Ref.Locator` is an opaque string. For the filesystem backend it happens to be a directory path; for S3 it would be a key. Never use "path" to mean a generic artifact reference — that ties the abstraction to filesystems.

**5. Interfaces at boundaries, concrete at injection.**
The orchestrator accepts `store.JobStore`, `store.ExecutionStore`, etc. — interfaces. It injects concrete `*buildinator.Buildinator` because there is one implementation. Mock everything at test time via the `mocks` sub-packages.

---

## Store layer conventions

**Typed IDs** — defined in `internal/store/models.go`. Use constructors (`store.NewJobID()`), not `uuid.New()`.

**Postgres helpers** (in `internal/store/postgres/helpers.go`):
- `ensureID[T ~[16]byte](&id)` — assigns a new UUID if zero; works on all typed ID types
- `toUUID[T ~[16]byte](id)` — cast to `uuid.UUID` for pgx bind sites
- `wrapNoRowsByID[T ~[16]byte](entity, id, err)` — wraps `pgx.ErrNoRows` → `store.ErrNotFound`
- `ensureRowsAffected[T ~[16]byte](result, entity, id)` — errors if no rows affected

**Scan pattern** for UUID fields — pgx v5 cannot decode into custom ID types:
```go
var idRaw, foreignKeyRaw uuid.UUID
row.Scan(&idRaw, &foreignKeyRaw, ...)
entity.ID = store.SomeID(idRaw)
entity.ForeignKeyID = store.OtherID(foreignKeyRaw)
```

**Integration tests** — tagged `//go:build integration`. Use `storetest.NewTestPool(t)` which creates a fresh Postgres database per test and drops it on cleanup. Run with `-tags integration`. Connection config via `SHIPINATOR_TEST_DB_*` env vars.

---

## Executor conventions

The `Executor` interface has three methods: `Submit`, `Status`, `Cancel`. The orchestrator polls on a ticker (default 5s). Status polling is fire-and-forget error-tolerant (logs warn, continues). Cancellation on context cancellation uses `context.Background()` for the cancel call so it succeeds even if the original ctx is done.

`ExecutionSpec.Artifacts` carries artifact output specifications from the executor's perspective. The buildinator pre-allocates `ArtifactID`s and tells the executor where to write them.

---

## API conventions (Phase 7 decisions)

**Proto-first API contract.**
`api/v1/shipinator.proto` is the transport contract source of truth for backend and frontend type generation. Keep transport DTOs in proto and map explicitly to domain/store models in handlers.

**HTTP mapping annotations in proto.**
Use `google.api.http` options on RPCs so grpc-gateway-compatible routing is defined in one place. The documented v1 routes are already represented there.

**Streaming downloads stay native HTTP.**
`GET /v1/artifacts/{id}/download` should be implemented as a streaming Echo handler (`io.Copy` from artifact store reader to response writer). Avoid JSON/base64 artifact payloads for large content.

---

## What's done and what's next

Phases 1–6 are complete (see `docs/TODO.md` for the full list). In Phase 7:
- Protobuf API contract is implemented in `api/v1/shipinator.proto` and generated for Go (`api/v1/shipinator.pb.go`, `api/v1/shipinator_grpc.pb.go`)
- Next: implement HTTP handlers (trigger run, get status, list jobs, artifact metadata/download, executor callback)
- Then wire handlers, stores, executor, subsystems, and orchestrator in `cmd/server/main.go`

Current `cmd/server/main.go` is a stub. The server package and server config exist but are not yet wired to stores or the orchestrator.

---

## Running tests

```bash
# Unit tests (no DB required)
go test ./...

# Integration tests (requires Postgres)
go test -tags integration ./internal/store/postgres/...
```

Generate mocks (after changing an interface):
```bash
go generate ./internal/executor/...
go generate ./internal/artifact/...
```
