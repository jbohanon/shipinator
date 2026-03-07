package store

import (
	"context"

	"github.com/google/uuid"
)

type ProjectStore interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, id uuid.UUID) (*Project, error)
	GetByName(ctx context.Context, name string) (*Project, error)
	List(ctx context.Context) ([]Project, error)
	Update(ctx context.Context, p *Project) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type RepositoryStore interface {
	Create(ctx context.Context, r *Repository) error
	GetByID(ctx context.Context, id uuid.UUID) (*Repository, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]Repository, error)
	Update(ctx context.Context, r *Repository) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type PipelineStore interface {
	Create(ctx context.Context, p *Pipeline) error
	GetByID(ctx context.Context, id uuid.UUID) (*Pipeline, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID) ([]Pipeline, error)
	Update(ctx context.Context, p *Pipeline) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type PipelineRunStore interface {
	Create(ctx context.Context, r *PipelineRun) error
	GetByID(ctx context.Context, id uuid.UUID) (*PipelineRun, error)
	ListByPipeline(ctx context.Context, pipelineID uuid.UUID) ([]PipelineRun, error)
	ListByStatus(ctx context.Context, status string) ([]PipelineRun, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

type JobStore interface {
	Create(ctx context.Context, j *Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*Job, error)
	ListByPipelineRun(ctx context.Context, pipelineRunID uuid.UUID) ([]Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

type JobStepStore interface {
	Create(ctx context.Context, s *JobStep) error
	GetByID(ctx context.Context, id uuid.UUID) (*JobStep, error)
	ListByJob(ctx context.Context, jobID uuid.UUID) ([]JobStep, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

type ArtifactStore interface {
	Create(ctx context.Context, a *Artifact) error
	GetByID(ctx context.Context, id uuid.UUID) (*Artifact, error)
	ListByJob(ctx context.Context, jobID uuid.UUID) ([]Artifact, error)
}

type ExecutionStore interface {
	Create(ctx context.Context, e *Execution) error
	GetByID(ctx context.Context, id uuid.UUID) (*Execution, error)
	ListByJobStep(ctx context.Context, jobStepID uuid.UUID) ([]Execution, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}
