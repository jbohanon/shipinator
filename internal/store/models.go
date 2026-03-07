package store

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	VCSProvider   string
	CloneURL      string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Pipeline struct {
	ID           uuid.UUID
	RepositoryID uuid.UUID
	Name         string
	TriggerType  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PipelineRun struct {
	ID         uuid.UUID
	PipelineID uuid.UUID
	GitRef     string
	GitSHA     string
	Status     string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type Job struct {
	ID            uuid.UUID
	PipelineRunID uuid.UUID
	JobType       string
	Name          string
	Status        string
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

type JobStep struct {
	ID             uuid.UUID
	JobID          uuid.UUID
	Name           string
	ExecutionOrder *int
	ParallelGroup  *string
	Status         string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type Artifact struct {
	ID             uuid.UUID
	JobID          uuid.UUID
	ArtifactType   string
	StorageBackend string
	StoragePath    string
	Checksum       *string
	CreatedAt      time.Time
}

type Execution struct {
	ID           uuid.UUID
	JobStepID    uuid.UUID
	ExecutorType string
	ExternalID   string
	Status       string
	SubmittedAt  time.Time
	CompletedAt  *time.Time
}
