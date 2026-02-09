# Shipinator — Technical Design Document

## 1. Project Overview & Purpose

### 1.1 Motivation

Shipinator is a self-hosted CI/CD control plane designed as a learning-focused but production-grade system. While functionally overlapping with existing CI/CD platforms, Shipinator is intentionally built to:

* Demonstrate deep understanding of CI/CD internals (state machines, artifact management, orchestration)
* Provide a flexible system that can be adapted to bespoke workflows
* Serve as a portfolio-quality project illustrating systems design maturity
* Act as its own first customer (dogfooding)

The goal is not novelty, but correctness, clarity, and extensibility.

### 1.2 Core Principles

* **Triggers are not execution**: External systems (Gitea Actions, CLI, UI) only submit intent.
* **Artifacts are first-class**: Builds produce immutable artifacts; deploys consume them.
* **Explicit state**: All work progresses through well-defined, persisted states.
* **Modular monolith**: Logical subsystems with hard boundaries, deployed as a single binary (v1).
* **Extensibility over completeness**: Clear seams for future storage backends, deploy targets, and schedulers.

---

## 2. Scope (v1)

### In Scope

* Build and test on PRs
* Build and test on push to `main`
* Manual build + deploy via CLI or Web UI
* Builders for:

  * Go binaries
  * OCI images
  * Helm charts (lint + package)
* Testers for:

  * `go test`
  * `helm lint`
* Deployment to a local Kubernetes cluster
* Artifact storage via mounted NFS

### Out of Scope (Explicit Non-Goals)

* Multi-environment promotion
* Approval workflows
* Secret management beyond basic environment injection
* Advanced deployment strategies (canary, blue/green)
* DAG-based pipelines

---

## 3. High-Level Architecture

### 3.0 Execution Model Overview

Shipinator adopts a **remote worker execution model** from v1. All build, test, and deploy work is executed outside the Shipinator process itself by ephemeral workers. In the initial implementation, Kubernetes Jobs are used as the worker substrate.

Shipinator remains a pure control plane:

* It decides *what* should run and *when*
* It never shells out to local executables
* It never embeds Kubernetes-specific logic outside executor implementations

This design avoids "break-glass" execution paths (e.g., `os/exec`) and ensures that platform-specific concerns remain tightly scoped.

### 3.1 System Overview

```
+-------------------+
| Clients           |
|-------------------|
| - Gitea Actions   |
| - CLI             |
| - Web UI          |
+---------+---------+
          |
          v
+-----------------------------+
| Shipinator API              |
|-----------------------------|
| - AuthN/AuthZ               |
| - Job Submission            |
| - Job Inspection            |
+-------------+---------------+
              |
              v
+---------------------------------------------+
| Shipinator Core (Control Plane)              |
|---------------------------------------------|
| Orchestrator / Scheduler                    |
|   - Job FSM                                  |
|   - Stage FSMs                               |
|                                             |
| Buildinator | Testinator | Deployinator     |
|   (logical subsystems)                       |
|                                             |
| Executor Interface                           |
| Artifact Registry                            |
+------------------+--------------------------+
                   |
                   v
+-----------------------------+
| Execution Substrate         |
|-----------------------------|
| - Kubernetes Jobs (v1)      |
|   - Ephemeral Pods          |
| - Future Executors         |
|   (VMs, ECS, SSH, etc.)     |
+-----------------------------+
```

```
+-------------------+
| Clients           |
|-------------------|
| - Gitea Actions   |
| - CLI             |
| - Web UI          |
+---------+---------+
          |
          v
+-----------------------------+
| Shipinator API              |
|-----------------------------|
| - AuthN/AuthZ               |
| - Job Submission            |
| - Job Inspection            |
+-------------+---------------+
              |
              v
+---------------------------------------------+
| Shipinator Core (Modular Monolith)           |
|---------------------------------------------|
| Orchestrator / Scheduler                    |
|   - Job FSM                                  |
|   - Stage FSMs                               |
|                                             |
| Buildinator | Testinator | Deployinator     |
|                                             |
| Artifact Management                          |
|   - Artifact Registry                        |
|   - Artifact Store Interface                 |
+------------------+--------------------------+
                   |
                   v
+-----------------------------+
| Execution Environment       |
|-----------------------------|
| - Local workers             |
| - Kubernetes (deploy only)  |
+-----------------------------+
```

### 3.2 Architectural Style

* Single deployable binary
* Internal subsystems communicate via Go interfaces
* One persistent datastore
* One artifact store abstraction

---

## 4. Major Design Decisions & Tradeoffs

### 4.1 Modular Monolith vs Microservices

**Decision:** Modular monolith

**Rationale:**

* Reduces operational complexity
* Enables focus on state modeling and correctness
* Internal APIs are designed to be externally viable

**Tradeoff:**

* Less realistic operational overhead compared to microservices
* Mitigated by strong internal boundaries

---

### 4.2 FSM-Based Orchestration vs DAG Pipelines

**Decision:** Finite State Machines

**Rationale:**

* Aligns with proven order-processing models
* Easier to reason about, debug, and persist
* Sufficient for v1 requirements

**Tradeoff:**

* Less expressive than DAGs
* DAG support can be layered later if needed

---

### 4.3 Artifact-Centric Design

**Decision:** Deployers only accept artifacts, never source

**Rationale:**

* Guarantees reproducibility
* Enables promotion and rollback semantics
* Decouples build from deploy

**Tradeoff:**

* Requires artifact storage infrastructure

---

## 5. Configuration: `.shipinator.yaml`

The repository-local configuration file defines build, test, and deploy intent.

Example:

```yaml
build:
  steps:
    - name: go-build
      run: make build
      outputs:
        - type: binary
          path: bin/app

    - name: image
      run: make image
      outputs:
        - type: oci_image
          ref: myapp:{{ .commit }}

test:
  steps:
    - name: unit
      run: go test ./... -coverprofile=coverage.out
      artifacts:
        - type: coverage
          path: coverage.out

    - name: helm-lint
      run: helm lint charts/*

deploy:
  artifact: helm_chart
  target: local
  namespace: default
```

---

## 6. Data Model

### 6.0 Executor Abstraction

The Executor interface defines the contract between Shipinator's control plane and any execution substrate.

```go
type Executor interface {
    Execute(ctx context.Context, spec ExecutionSpec) (ExecutionHandle, error)
}
```

Executor implementations translate platform-specific execution signals into generic completion results. Kubernetes-specific concepts (Jobs, Pods, log streaming) are fully encapsulated within the KubernetesExecutor and do not leak into the core system.

### 6.1 Job

Represents user intent.

| Field            | Description           |
| ---------------- | --------------------- |
| id               | Unique job identifier |
| repo             | Repository URL        |
| ref              | Commit SHA            |
| trigger          | pr / push / manual    |
| requested_stages | build, test, deploy   |
| status           | Overall job status    |
| created_at       | Timestamp             |

---

### 6.2 StageExecution

Represents execution of a stage.

| Field       | Description           |
| ----------- | --------------------- |
| id          | Stage execution ID    |
| job_id      | Parent job            |
| stage_type  | build / test / deploy |
| state       | FSM state             |
| logs_ref    | Log storage reference |
| started_at  | Timestamp             |
| finished_at | Timestamp             |

---

### 6.3 Artifact

Represents immutable build or test output.

| Field       | Description                              |
| ----------- | ---------------------------------------- |
| id          | Artifact ID                              |
| job_id      | Producing job                            |
| type        | binary / oci_image / helm_chart / report |
| digest      | Content hash                             |
| storage_ref | Artifact store reference                 |
| metadata    | JSON metadata                            |

---

## 7. Finite State Machine Design

### 7.0 FSM and Execution Decoupling

FSM state transitions are driven by executor-reported outcomes, not by platform-native states. Executor implementations are responsible for mapping execution results (success, failure, cancellation) into FSM transitions.

This ensures that Kubernetes concepts do not leak into orchestration logic and preserves executor interchangeability.

### 7.1 Job FSM

```
SUBMITTED
  → BUILDING
    → BUILT
      → TESTING
        → TESTED
          → DEPLOYING
            → DEPLOYED

Any state → FAILED
Any state → CANCELLED
```

Transitions are explicit, persisted, and idempotent.

---

### 7.2 Stage FSM (Generic)

```
PENDING → RUNNING → SUCCEEDED
                   → FAILED
```

---

## 8. Builder Design (Buildinator)

### Responsibilities

* Define build execution intent
* Translate build configuration into execution specifications
* Register produced artifacts

### Execution Model

Buildinator does not execute build commands directly. Instead, it constructs an execution specification and submits it to the Executor. The default Executor schedules a Kubernetes Job to perform the build.

This avoids embedding build tooling or Kubernetes logic into the control plane.

### Interfaces

```go
type Builder interface {
    Build(ctx context.Context, job Job) ([]Artifact, error)
}
```

### Supported Outputs (v1)

* Go binaries
* OCI images
* Helm chart packages

---

## 9. Tester Design (Testinator)

### Responsibilities

* Define test execution intent
* Support serial and parallel test steps
* Collect test result artifacts

### Execution Model

Testinator submits test execution specifications to the Executor. Test execution is performed by ephemeral workers, ensuring isolation and consistency across environments.

### Parallelism Model

* Steps may declare `parallel: true`
* Parallel steps are executed within a single stage
* Stage completes only when all steps finish

---

## 10. Deployer Design (Deployinator)

### Responsibilities

* Accept artifact references
* Define deployment intent
* Trigger artifact deployment to a target environment

### Execution Model

Deployinator never performs deployments in-process. All deployments are executed by remote workers via the Executor interface. Deployment tooling such as `helm` and `kubectl` runs exclusively inside worker environments.

This approach eliminates the need for the Shipinator process to shell out or link Kubernetes client libraries directly.

### Interfaces

```go
type Deployer interface {
    Deploy(ctx context.Context, artifact Artifact, target DeployTarget) error
}
```

### Supported Targets (v1)

* Local Kubernetes cluster (via Kubernetes Job executor)

Future targets (design-ready):

* EKS
* GKE
* ECS

---

## 11. Artifact Storage

### Abstraction

```go
type ArtifactStore interface {
    Put(ctx context.Context, artifact Artifact) (string, error)
    Get(ctx context.Context, ref string) (io.ReadCloser, error)
}
```

### Implementations

* NFS-backed store (v1)
* S3-backed store (future)

---

## 12. Failure Handling & Idempotency

* All state transitions are persisted
* Jobs can be resumed after process crash
* External retries are safe
* Cancellation propagates to active stages

---

## 13. Observability (v1)

* Structured logs per stage
* Logs linked via `logs_ref`
* Basic metrics (job duration, failure rate)

---

## 14. Future Extensions

* Multi-environment promotion
* Approval gates
* DAG execution model
* Remote build runners
* Artifact promotion and rollback

---

## 15. Summary

Shipinator v1 is a deliberately scoped CI/CD control plane emphasizing explicit state, artifact-centric workflows, and extensible architecture. It prioritizes correctness and learning over feature completeness, while remaining realistic, defensible, and evolvable into a more distributed system if desired.

