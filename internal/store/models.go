package store

import (
	"time"

	"github.com/google/uuid"
)

// Typed entity ID types. Using distinct types (not aliases) gives compile-time
// enforcement that IDs from different entities cannot be accidentally swapped.
type (
	ProjectID     uuid.UUID
	RepositoryID  uuid.UUID
	PipelineID    uuid.UUID
	PipelineRunID uuid.UUID
	JobID         uuid.UUID
	JobStepID     uuid.UUID
	ArtifactID    uuid.UUID
	ExecutionID   uuid.UUID
)

// Constructors
func NewProjectID() ProjectID         { return ProjectID(uuid.New()) }
func NewRepositoryID() RepositoryID   { return RepositoryID(uuid.New()) }
func NewPipelineID() PipelineID       { return PipelineID(uuid.New()) }
func NewPipelineRunID() PipelineRunID { return PipelineRunID(uuid.New()) }
func NewJobID() JobID                 { return JobID(uuid.New()) }
func NewJobStepID() JobStepID         { return JobStepID(uuid.New()) }
func NewArtifactID() ArtifactID       { return ArtifactID(uuid.New()) }
func NewExecutionID() ExecutionID     { return ExecutionID(uuid.New()) }

// String methods (delegates to uuid.UUID)
func (id ProjectID) String() string     { return uuid.UUID(id).String() }
func (id RepositoryID) String() string  { return uuid.UUID(id).String() }
func (id PipelineID) String() string    { return uuid.UUID(id).String() }
func (id PipelineRunID) String() string { return uuid.UUID(id).String() }
func (id JobID) String() string         { return uuid.UUID(id).String() }
func (id JobStepID) String() string     { return uuid.UUID(id).String() }
func (id ArtifactID) String() string    { return uuid.UUID(id).String() }
func (id ExecutionID) String() string   { return uuid.UUID(id).String() }

// Job type constants. These are stored as job_type in the jobs table.
const (
	JobTypeBuild  = "build"
	JobTypeTest   = "test"
	JobTypeDeploy = "deploy"
)

type Project struct {
	ID          ProjectID
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository struct {
	ID            RepositoryID
	ProjectID     ProjectID
	VCSProvider   string
	CloneURL      string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Pipeline struct {
	ID           PipelineID
	RepositoryID RepositoryID
	Name         string
	TriggerType  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PipelineRun struct {
	ID         PipelineRunID
	PipelineID PipelineID
	GitRef     string
	GitSHA     string
	Status     string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type Job struct {
	ID            JobID
	PipelineRunID PipelineRunID
	JobType       string
	Name          string
	Status        string
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

type JobStep struct {
	ID             JobStepID
	JobID          JobID
	Name           string
	ExecutionOrder *int
	ParallelGroup  *string
	Status         string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type Artifact struct {
	ID             ArtifactID
	JobID          JobID
	ArtifactType   string
	StorageBackend string
	StoragePath    string
	Checksum       *string
	CreatedAt      time.Time
}

type Execution struct {
	ID           ExecutionID
	JobStepID    JobStepID
	ExecutorType string
	ExternalID   string
	Status       string
	SubmittedAt  time.Time
	CompletedAt  *time.Time
}
