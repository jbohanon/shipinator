package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	shipinatorv1 "git.nonahob.net/jacob/shipinator/api/v1"
	"git.nonahob.net/jacob/shipinator/internal/artifact"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/encoding/protojson"
)

type fakePipelineStore struct {
	pipeline *store.Pipeline
	err      error
}

func (f *fakePipelineStore) GetByID(ctx context.Context, id store.PipelineID) (*store.Pipeline, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.pipeline == nil || f.pipeline.ID != id {
		return nil, store.ErrNotFound
	}
	return f.pipeline, nil
}

type fakePipelineRunStore struct {
	runs map[store.PipelineRunID]*store.PipelineRun
	err  error
}

func (f *fakePipelineRunStore) Create(ctx context.Context, r *store.PipelineRun) error {
	if f.err != nil {
		return f.err
	}
	now := time.Now().UTC()
	r.ID = store.PipelineRunID(uuid.New())
	r.CreatedAt = now
	if r.Status == "" {
		r.Status = "pending"
	}
	if f.runs == nil {
		f.runs = map[store.PipelineRunID]*store.PipelineRun{}
	}
	f.runs[r.ID] = r
	return nil
}

func (f *fakePipelineRunStore) GetByID(ctx context.Context, id store.PipelineRunID) (*store.PipelineRun, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.runs == nil {
		return nil, store.ErrNotFound
	}
	run, ok := f.runs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return run, nil
}

func (f *fakePipelineRunStore) UpdateStatus(ctx context.Context, id store.PipelineRunID, status string) error {
	if f.err != nil {
		return f.err
	}
	if f.runs == nil {
		return store.ErrNotFound
	}
	run, ok := f.runs[id]
	if !ok {
		return store.ErrNotFound
	}
	run.Status = status
	return nil
}

type fakeJobStore struct {
	jobs []store.Job
	err  error
}

func (f *fakeJobStore) ListByPipelineRun(ctx context.Context, pipelineRunID store.PipelineRunID) ([]store.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.jobs, nil
}

type fakeArtifactStore struct {
	artifact *store.Artifact
	err      error
}

func (f *fakeArtifactStore) GetByID(ctx context.Context, id store.ArtifactID) (*store.Artifact, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.artifact == nil || f.artifact.ID != id {
		return nil, store.ErrNotFound
	}
	return f.artifact, nil
}

type fakeExecutionStore struct {
	lastID     store.ExecutionID
	lastStatus string
	err        error
}

func (f *fakeExecutionStore) UpdateStatus(ctx context.Context, id store.ExecutionID, status string) error {
	if f.err != nil {
		return f.err
	}
	f.lastID = id
	f.lastStatus = status
	return nil
}

type fakeArtifactBytesStore struct {
	getFn func(ref artifact.Ref) (io.ReadCloser, error)
}

func (f *fakeArtifactBytesStore) Put(ctx context.Context, id uuid.UUID, content io.Reader) (artifact.Ref, error) {
	return artifact.Ref{}, nil
}

func (f *fakeArtifactBytesStore) Get(ctx context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	return f.getFn(ref)
}

func TestCreatePipelineRun(t *testing.T) {
	t.Parallel()

	pipelineID := store.PipelineID(uuid.New())
	pipelines := &fakePipelineStore{pipeline: &store.Pipeline{ID: pipelineID}}
	runs := &fakePipelineRunStore{}
	s := New(
		pipelines,
		runs,
		&fakeJobStore{},
		&fakeArtifactStore{},
		&fakeExecutionStore{},
		&fakeArtifactBytesStore{},
	)

	e := echo.New()
	s.RegisterRoutes(e)

	body := `{"git_ref":"refs/heads/main","git_sha":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipelines/"+pipelineID.String()+"/runs", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(runs.runs) == 0 {
		t.Fatalf("run was not created")
	}
	created := runs.runs[runs.runIDByGitRef("refs/heads/main")]
	if created == nil {
		t.Fatalf("created run not found")
	}
	if created.PipelineID != pipelineID {
		t.Fatalf("run.PipelineID = %s, want %s", created.PipelineID, pipelineID)
	}

	var resp shipinatorv1.CreatePipelineRunResponse
	if err := protojson.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := resp.GetPipelineRun().GetGitRef(); got != "refs/heads/main" {
		t.Fatalf("git_ref = %q, want refs/heads/main", got)
	}
}

func TestDownloadArtifact(t *testing.T) {
	t.Parallel()

	artifactID := store.ArtifactID(uuid.New())
	meta := &fakeArtifactStore{
		artifact: &store.Artifact{
			ID:             artifactID,
			StorageBackend: artifact.BackendNFS,
			StoragePath:    "/artifacts/" + artifactID.String(),
		},
	}
	s := New(
		&fakePipelineStore{},
		&fakePipelineRunStore{},
		&fakeJobStore{},
		meta,
		&fakeExecutionStore{},
		&fakeArtifactBytesStore{
			getFn: func(ref artifact.Ref) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewBufferString("hello")), nil
			},
		},
	)

	e := echo.New()
	s.RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodGet, "/v1/artifacts/"+artifactID.String()+"/download", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "hello" {
		t.Fatalf("body = %q, want hello", got)
	}
}

func TestReportExecutionStatus(t *testing.T) {
	t.Parallel()

	execs := &fakeExecutionStore{}
	s := New(
		&fakePipelineStore{},
		&fakePipelineRunStore{},
		&fakeJobStore{},
		&fakeArtifactStore{},
		execs,
		&fakeArtifactBytesStore{},
	)

	e := echo.New()
	s.RegisterRoutes(e)

	execID := store.ExecutionID(uuid.New())
	body := `{"status":"CALLBACK_STATUS_SUCCESS"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/executions/"+execID.String()+"/status", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if execs.lastID != execID {
		t.Fatalf("updated exec id = %s, want %s", execs.lastID, execID)
	}
	if execs.lastStatus != "succeeded" {
		t.Fatalf("updated status = %q, want succeeded", execs.lastStatus)
	}
}

func TestCancelPipelineRun(t *testing.T) {
	t.Parallel()

	runID := store.PipelineRunID(uuid.New())
	runs := &fakePipelineRunStore{
		runs: map[store.PipelineRunID]*store.PipelineRun{
			runID: {
				ID:         runID,
				PipelineID: store.PipelineID(uuid.New()),
				GitRef:     "refs/heads/main",
				GitSHA:     "abc123",
				Status:     "running",
				CreatedAt:  time.Now().UTC(),
			},
		},
	}
	s := New(
		&fakePipelineStore{},
		runs,
		&fakeJobStore{},
		&fakeArtifactStore{},
		&fakeExecutionStore{},
		&fakeArtifactBytesStore{},
	)

	e := echo.New()
	s.RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline-runs/"+runID.String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := runs.runs[runID].Status; got != "canceled" {
		t.Fatalf("status = %q, want canceled", got)
	}
}

func TestRetryPipelineRun(t *testing.T) {
	t.Parallel()

	origID := store.PipelineRunID(uuid.New())
	orig := &store.PipelineRun{
		ID:         origID,
		PipelineID: store.PipelineID(uuid.New()),
		GitRef:     "refs/heads/main",
		GitSHA:     "abc123",
		Status:     "failed",
		CreatedAt:  time.Now().UTC(),
	}
	runs := &fakePipelineRunStore{
		runs: map[store.PipelineRunID]*store.PipelineRun{
			origID: orig,
		},
	}
	s := New(
		&fakePipelineStore{},
		runs,
		&fakeJobStore{},
		&fakeArtifactStore{},
		&fakeExecutionStore{},
		&fakeArtifactBytesStore{},
	)

	e := echo.New()
	s.RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline-runs/"+origID.String()+"/retry", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(runs.runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs.runs))
	}
	foundRetry := false
	for id, run := range runs.runs {
		if id == origID {
			continue
		}
		foundRetry = true
		if run.PipelineID != orig.PipelineID || run.GitRef != orig.GitRef || run.GitSHA != orig.GitSHA {
			t.Fatalf("retry run did not copy source run fields")
		}
	}
	if !foundRetry {
		t.Fatalf("retry run not created")
	}
}

func (f *fakePipelineRunStore) runIDByGitRef(gitRef string) store.PipelineRunID {
	for id, run := range f.runs {
		if run.GitRef == gitRef {
			return id
		}
	}
	return store.PipelineRunID{}
}
