package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	shipinatorv1 "git.nonahob.net/jacob/shipinator/api/v1"
	"git.nonahob.net/jacob/shipinator/internal/artifact"
	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type pipelineStore interface {
	GetByID(ctx context.Context, id store.PipelineID) (*store.Pipeline, error)
}

type pipelineRunStore interface {
	Create(ctx context.Context, r *store.PipelineRun) error
	GetByID(ctx context.Context, id store.PipelineRunID) (*store.PipelineRun, error)
	UpdateStatus(ctx context.Context, id store.PipelineRunID, status string) error
}

type jobStore interface {
	ListByPipelineRun(ctx context.Context, pipelineRunID store.PipelineRunID) ([]store.Job, error)
}

type artifactStore interface {
	GetByID(ctx context.Context, id store.ArtifactID) (*store.Artifact, error)
}

type executionStore interface {
	UpdateStatus(ctx context.Context, id store.ExecutionID, status string) error
}

// Server serves Shipinator v1 API endpoints over HTTP.
type Server struct {
	pipelines  pipelineStore
	runs       pipelineRunStore
	jobs       jobStore
	artifacts  artifactStore
	executions executionStore

	artifactBytes artifact.ArtifactStore
}

func New(
	pipelines pipelineStore,
	runs pipelineRunStore,
	jobs jobStore,
	artifacts artifactStore,
	executions executionStore,
	artifactBytes artifact.ArtifactStore,
) *Server {
	return &Server{
		pipelines:     pipelines,
		runs:          runs,
		jobs:          jobs,
		artifacts:     artifacts,
		executions:    executions,
		artifactBytes: artifactBytes,
	}
}

// RegisterRoutes wires all v1 API routes into Echo.
func (s *Server) RegisterRoutes(e *echo.Echo) {
	e.POST("/v1/pipelines/:id/runs", s.createPipelineRun)
	e.POST("/v1/pipeline-runs/:id/cancel", s.cancelPipelineRun)
	e.POST("/v1/pipeline-runs/:id/retry", s.retryPipelineRun)
	e.GET("/v1/pipeline-runs/:id", s.getPipelineRun)
	e.GET("/v1/pipeline-runs/:id/jobs", s.listPipelineRunJobs)
	e.GET("/v1/artifacts/:id/metadata", s.getArtifactMetadata)
	e.GET("/v1/artifacts/:id/download", s.downloadArtifact)
	e.POST("/v1/executions/:id/status", s.reportExecutionStatus)
}

func (s *Server) createPipelineRun(c echo.Context) error {
	pipelineIDRaw := c.Param("id")
	pipelineID, err := parsePipelineID(pipelineIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	req := &shipinatorv1.CreatePipelineRunRequest{}
	if err := decodeProtoJSON(c, req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	if req.GetPipelineId() != "" && req.GetPipelineId() != pipelineIDRaw {
		return echo.NewHTTPError(http.StatusBadRequest, "path pipeline id does not match request body")
	}
	if req.GetGitRef() == "" || req.GetGitSha() == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "git_ref and git_sha are required")
	}

	if _, err := s.pipelines.GetByID(c.Request().Context(), pipelineID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load pipeline: %v", err))
	}

	run := &store.PipelineRun{
		PipelineID: pipelineID,
		GitRef:     req.GetGitRef(),
		GitSHA:     req.GetGitSha(),
	}
	if err := s.runs.Create(c.Request().Context(), run); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("create pipeline run: %v", err))
	}

	return writeProtoJSON(c, http.StatusCreated, &shipinatorv1.CreatePipelineRunResponse{
		PipelineRun: toProtoPipelineRun(run),
	})
}

func (s *Server) getPipelineRun(c echo.Context) error {
	runID, err := parsePipelineRunID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	run, err := s.runs.GetByID(c.Request().Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline run not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load pipeline run: %v", err))
	}

	return writeProtoJSON(c, http.StatusOK, &shipinatorv1.CancelPipelineRunResponse{
		PipelineRun: toProtoPipelineRun(run),
	})
}

func (s *Server) cancelPipelineRun(c echo.Context) error {
	runID, err := parsePipelineRunID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	run, err := s.runs.GetByID(c.Request().Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline run not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load pipeline run: %v", err))
	}
	if isTerminalRunStatus(run.Status) {
		return echo.NewHTTPError(http.StatusConflict, "pipeline run is already terminal")
	}
	if err := s.runs.UpdateStatus(c.Request().Context(), runID, "canceled"); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline run not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("cancel pipeline run: %v", err))
	}
	run.Status = "canceled"

	return writeProtoJSON(c, http.StatusOK, &shipinatorv1.GetPipelineRunResponse{
		PipelineRun: toProtoPipelineRun(run),
	})
}

func (s *Server) retryPipelineRun(c echo.Context) error {
	runID, err := parsePipelineRunID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	previous, err := s.runs.GetByID(c.Request().Context(), runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline run not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load pipeline run: %v", err))
	}
	if !isRetryEligibleRunStatus(previous.Status) {
		return echo.NewHTTPError(http.StatusConflict, "pipeline run is not eligible for retry")
	}

	retry := &store.PipelineRun{
		PipelineID: previous.PipelineID,
		GitRef:     previous.GitRef,
		GitSHA:     previous.GitSHA,
	}
	if err := s.runs.Create(c.Request().Context(), retry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("create retry pipeline run: %v", err))
	}

	return writeProtoJSON(c, http.StatusCreated, &shipinatorv1.RetryPipelineRunResponse{
		PipelineRun: toProtoPipelineRun(retry),
	})
}

func (s *Server) listPipelineRunJobs(c echo.Context) error {
	runID, err := parsePipelineRunID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if _, err := s.runs.GetByID(c.Request().Context(), runID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "pipeline run not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load pipeline run: %v", err))
	}

	jobs, err := s.jobs.ListByPipelineRun(c.Request().Context(), runID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list jobs: %v", err))
	}

	resp := &shipinatorv1.ListPipelineRunJobsResponse{
		Jobs: make([]*shipinatorv1.Job, 0, len(jobs)),
	}
	for i := range jobs {
		resp.Jobs = append(resp.Jobs, toProtoJob(&jobs[i]))
	}
	return writeProtoJSON(c, http.StatusOK, resp)
}

func (s *Server) getArtifactMetadata(c echo.Context) error {
	artifactID, err := parseArtifactID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	a, err := s.artifacts.GetByID(c.Request().Context(), artifactID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "artifact not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load artifact: %v", err))
	}

	return writeProtoJSON(c, http.StatusOK, &shipinatorv1.GetArtifactMetadataResponse{
		Artifact: toProtoArtifact(a),
	})
}

func (s *Server) downloadArtifact(c echo.Context) error {
	artifactID, err := parseArtifactID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	a, err := s.artifacts.GetByID(c.Request().Context(), artifactID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "artifact not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load artifact: %v", err))
	}

	r, err := s.artifactBytes.Get(c.Request().Context(), artifact.Ref{
		Backend: a.StorageBackend,
		Locator: a.StoragePath,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("load artifact bytes: %v", err))
	}
	defer r.Close()

	filename := artifactID.String()
	if base := filepath.Base(a.StoragePath); base != "" && base != "." && base != "/" {
		filename = base
	}

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "application/octet-stream")
	resp.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	resp.WriteHeader(http.StatusOK)
	if _, err := io.Copy(resp, r); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("stream artifact bytes: %v", err))
	}
	return nil
}

func (s *Server) reportExecutionStatus(c echo.Context) error {
	execIDRaw := c.Param("id")
	execID, err := parseExecutionID(execIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	req := &shipinatorv1.ReportExecutionStatusRequest{}
	if err := decodeProtoJSON(c, req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
	}
	if req.GetExecutionId() != "" && req.GetExecutionId() != execIDRaw {
		return echo.NewHTTPError(http.StatusBadRequest, "path execution id does not match request body")
	}

	status, phase, err := callbackStatusToExecutionStatus(req.GetStatus())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := s.executions.UpdateStatus(c.Request().Context(), execID, status); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "execution not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("update execution status: %v", err))
	}

	return writeProtoJSON(c, http.StatusOK, &shipinatorv1.ReportExecutionStatusResponse{
		ExecutionId: execID.String(),
		Phase:       phase,
	})
}

func writeProtoJSON(c echo.Context, status int, msg proto.Message) error {
	b, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("marshal response: %v", err))
	}
	return c.Blob(status, echo.MIMEApplicationJSONCharsetUTF8, b)
}

func decodeProtoJSON(c echo.Context, msg proto.Message) error {
	body := c.Request().Body
	if body == nil {
		return nil
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, msg)
}

func parsePipelineID(v string) (store.PipelineID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return store.PipelineID{}, fmt.Errorf("invalid pipeline id %q", v)
	}
	return store.PipelineID(id), nil
}

func parsePipelineRunID(v string) (store.PipelineRunID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return store.PipelineRunID{}, fmt.Errorf("invalid pipeline run id %q", v)
	}
	return store.PipelineRunID(id), nil
}

func parseArtifactID(v string) (store.ArtifactID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return store.ArtifactID{}, fmt.Errorf("invalid artifact id %q", v)
	}
	return store.ArtifactID(id), nil
}

func parseExecutionID(v string) (store.ExecutionID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return store.ExecutionID{}, fmt.Errorf("invalid execution id %q", v)
	}
	return store.ExecutionID(id), nil
}

func toProtoPipelineRun(r *store.PipelineRun) *shipinatorv1.PipelineRun {
	out := &shipinatorv1.PipelineRun{
		Id:         r.ID.String(),
		PipelineId: r.PipelineID.String(),
		GitRef:     r.GitRef,
		GitSha:     r.GitSHA,
		Status:     pipelineRunStatusToProto(r.Status),
		CreatedAt:  timestamppb.New(r.CreatedAt),
	}
	if r.StartedAt != nil {
		out.StartedAt = timestamppb.New(*r.StartedAt)
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return out
}

func toProtoJob(j *store.Job) *shipinatorv1.Job {
	out := &shipinatorv1.Job{
		Id:            j.ID.String(),
		PipelineRunId: j.PipelineRunID.String(),
		JobType:       jobTypeToProto(j.JobType),
		Name:          j.Name,
		Status:        jobStatusToProto(j.Status),
		CreatedAt:     timestamppb.New(j.CreatedAt),
	}
	if j.StartedAt != nil {
		out.StartedAt = timestamppb.New(*j.StartedAt)
	}
	if j.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*j.FinishedAt)
	}
	return out
}

func toProtoArtifact(a *store.Artifact) *shipinatorv1.Artifact {
	out := &shipinatorv1.Artifact{
		Id:            a.ID.String(),
		JobId:         a.JobID.String(),
		ArtifactType:  a.ArtifactType,
		StorageBackend: a.StorageBackend,
		StorageLocator: a.StoragePath,
		CreatedAt:      timestamppb.New(a.CreatedAt),
	}
	if a.Checksum != nil {
		out.Checksum = *a.Checksum
	}
	return out
}

func pipelineRunStatusToProto(status string) shipinatorv1.PipelineRunStatus {
	switch status {
	case "pending":
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_PENDING
	case "running":
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING
	case "success", "succeeded":
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_SUCCESS
	case "failed":
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_FAILED
	case "canceled":
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_CANCELED
	default:
		return shipinatorv1.PipelineRunStatus_PIPELINE_RUN_STATUS_UNSPECIFIED
	}
}

func jobTypeToProto(jobType string) shipinatorv1.JobType {
	switch jobType {
	case store.JobTypeBuild:
		return shipinatorv1.JobType_JOB_TYPE_BUILD
	case store.JobTypeTest:
		return shipinatorv1.JobType_JOB_TYPE_TEST
	case store.JobTypeDeploy:
		return shipinatorv1.JobType_JOB_TYPE_DEPLOY
	default:
		return shipinatorv1.JobType_JOB_TYPE_UNSPECIFIED
	}
}

func jobStatusToProto(status string) shipinatorv1.JobStatus {
	switch status {
	case "pending":
		return shipinatorv1.JobStatus_JOB_STATUS_PENDING
	case "running":
		return shipinatorv1.JobStatus_JOB_STATUS_RUNNING
	case "success", "succeeded":
		return shipinatorv1.JobStatus_JOB_STATUS_SUCCESS
	case "failed":
		return shipinatorv1.JobStatus_JOB_STATUS_FAILED
	case "canceled":
		return shipinatorv1.JobStatus_JOB_STATUS_CANCELED
	default:
		return shipinatorv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func callbackStatusToExecutionStatus(st shipinatorv1.CallbackStatus) (string, shipinatorv1.ExecutionPhase, error) {
	switch st {
	case shipinatorv1.CallbackStatus_CALLBACK_STATUS_RUNNING:
		return "running", shipinatorv1.ExecutionPhase_EXECUTION_PHASE_RUNNING, nil
	case shipinatorv1.CallbackStatus_CALLBACK_STATUS_SUCCESS:
		return "succeeded", shipinatorv1.ExecutionPhase_EXECUTION_PHASE_SUCCEEDED, nil
	case shipinatorv1.CallbackStatus_CALLBACK_STATUS_FAILED:
		return "failed", shipinatorv1.ExecutionPhase_EXECUTION_PHASE_FAILED, nil
	case shipinatorv1.CallbackStatus_CALLBACK_STATUS_CANCELED:
		return "canceled", shipinatorv1.ExecutionPhase_EXECUTION_PHASE_CANCELED, nil
	default:
		return "", shipinatorv1.ExecutionPhase_EXECUTION_PHASE_UNSPECIFIED, fmt.Errorf("invalid callback status")
	}
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case "success", "succeeded", "failed", "canceled":
		return true
	default:
		return false
	}
}

func isRetryEligibleRunStatus(status string) bool {
	switch status {
	case "failed", "canceled":
		return true
	default:
		return false
	}
}
