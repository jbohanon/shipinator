# Shipinator

Shipinator is a self-hosted CI/CD system built as a learning project and portfolio artifact.

The goal is not to compete with existing CI/CD platforms, but to understand how they work by designing and implementing one with clear boundaries, explicit state, and minimal magic.

Shipinator is designed to be dogfooded by its own development process.

---

## Purpose

This project exists to:

* Build a CI/CD system from first principles
* Practice designing stateful orchestration systems
* Explore clean separation between control planes and execution
* Produce a serious, reviewable engineering artifact for job searches
* Create something that can actually be used for personal projects

The emphasis is on correctness, debuggability, and intentional tradeoffs rather than feature breadth.

---

## What Shipinator Does

Shipinator orchestrates build, test, and deploy workflows for Git repositories.

It provides:

* Build and test on pull requests
* Build and test on push to main
* Manual build and deploy via API, web UI, or CLI
* Artifact storage with a pluggable backend
* Remote execution via a strict executor contract

Shipinator coordinates work but never executes build or deploy commands itself.

---

## Execution Model

Shipinator runs as a long-lived control plane service.

All work is executed by remote workers called executors. Executors are responsible for:

* Cloning repositories
* Running build, test, or deploy commands
* Producing artifacts
* Reporting execution results

In v1, executors are implemented using Kubernetes Jobs. Kubernetes-specific logic is fully contained within executor implementations.

---

## Configuration

Each repository defines its pipelines in a `.shipinator.yaml` file.

The configuration describes what should happen, not how it happens. Execution details are owned by the executor.

---

## v1 Scope

### Build

* Clone repository
* Build Go programs
* Build OCI images
* Lint and package Helm charts
* Store build artifacts

### Test

* Run `go test`
* Run `helm lint`
* Support sequential and parallel test steps
* Store test output and optional coverage reports

### Deploy

* Deploy Helm charts
* Deploy raw Kubernetes manifests
* Target a local Kubernetes cluster initially
* Design for future deployment targets

---

## Explicit Non Goals

* Dynamic DAG scheduling
* Hosted multi-tenant CI
* Cross-project artifact sharing
* Control-plane level caching
* Replacing existing CI platforms

---

## Development Status

Shipinator is under active development. Expect breaking changes.

There is no guarantee of stability until v1 is complete.

---

## TODOs

High-level development milestones:

* [ ] Define `.shipinator.yaml` schema
* [ ] Implement relational data model and migrations
* [ ] Implement pipeline and job state machines
* [ ] Build Kubernetes executor
* [ ] Implement artifact storage abstraction (NFS first)
* [ ] Implement build executor steps
* [ ] Implement test executor steps
* [ ] Implement deploy executor steps
* [ ] Add minimal web UI
* [ ] Add CLI
* [ ] Dogfood Shipinator to build Shipinator

---

## License

Shipinator is licensed under the Apache License, Version 2.0.

