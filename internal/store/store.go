package store

import (
	"context"
)

type ProjectStore interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, id ProjectID) (*Project, error)
	GetByName(ctx context.Context, name string) (*Project, error)
	List(ctx context.Context) ([]Project, error)
	Update(ctx context.Context, p *Project) error
	Delete(ctx context.Context, id ProjectID) error
}

type RepositoryStore interface {
	Create(ctx context.Context, r *Repository) error
	GetByID(ctx context.Context, id RepositoryID) (*Repository, error)
	ListByProject(ctx context.Context, projectID ProjectID) ([]Repository, error)
	Update(ctx context.Context, r *Repository) error
	Delete(ctx context.Context, id RepositoryID) error
}

type PipelineStore interface {
	Create(ctx context.Context, p *Pipeline) error
	GetByID(ctx context.Context, id PipelineID) (*Pipeline, error)
	ListByRepository(ctx context.Context, repositoryID RepositoryID) ([]Pipeline, error)
	Update(ctx context.Context, p *Pipeline) error
	Delete(ctx context.Context, id PipelineID) error
}

type PipelineRunStore interface {
	Create(ctx context.Context, r *PipelineRun) error
	GetByID(ctx context.Context, id PipelineRunID) (*PipelineRun, error)
	ListByPipeline(ctx context.Context, pipelineID PipelineID) ([]PipelineRun, error)
	ListByStatus(ctx context.Context, status string) ([]PipelineRun, error)
	UpdateStatus(ctx context.Context, id PipelineRunID, status string) error
}

type JobStore interface {
	Create(ctx context.Context, j *Job) error
	GetByID(ctx context.Context, id JobID) (*Job, error)
	ListByPipelineRun(ctx context.Context, pipelineRunID PipelineRunID) ([]Job, error)
	UpdateStatus(ctx context.Context, id JobID, status string) error
}

type JobStepStore interface {
	Create(ctx context.Context, s *JobStep) error
	GetByID(ctx context.Context, id JobStepID) (*JobStep, error)
	ListByJob(ctx context.Context, jobID JobID) ([]JobStep, error)
	UpdateStatus(ctx context.Context, id JobStepID, status string) error
}

type ArtifactStore interface {
	Create(ctx context.Context, a *Artifact) error
	GetByID(ctx context.Context, id ArtifactID) (*Artifact, error)
	ListByJob(ctx context.Context, jobID JobID) ([]Artifact, error)
}

type ExecutionStore interface {
	Create(ctx context.Context, e *Execution) error
	GetByID(ctx context.Context, id ExecutionID) (*Execution, error)
	ListByJobStep(ctx context.Context, jobStepID JobStepID) ([]Execution, error)
	UpdateStatus(ctx context.Context, id ExecutionID, status string) error
}
