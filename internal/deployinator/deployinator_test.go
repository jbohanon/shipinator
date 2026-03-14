package deployinator_test

import (
	"context"
	"errors"
	"testing"

	shipcfg "git.nonahob.net/jacob/shipinator/internal/config"
	"git.nonahob.net/jacob/shipinator/internal/deployinator"
	"git.nonahob.net/jacob/shipinator/internal/executor"
	"git.nonahob.net/jacob/shipinator/internal/store"
)

type fakeArtifactStore struct {
	arts []store.Artifact
}

func (f *fakeArtifactStore) Create(_ context.Context, _ *store.Artifact) error { return nil }
func (f *fakeArtifactStore) GetByID(_ context.Context, _ store.ArtifactID) (*store.Artifact, error) {
	return nil, nil
}
func (f *fakeArtifactStore) ListByJob(_ context.Context, _ store.JobID) ([]store.Artifact, error) {
	return f.arts, nil
}

type stepCapture struct {
	spec executor.ExecutionSpec
	err  error
}

func (c *stepCapture) run(_ context.Context, _ store.JobID, _ string, _ int, _ *string, spec executor.ExecutionSpec) error {
	c.spec = spec
	return c.err
}

func TestDeployinator_Run_HelmChart(t *testing.T) {
	buildJobID := store.NewJobID()
	artID := store.NewArtifactID()

	arts := &fakeArtifactStore{arts: []store.Artifact{
		{ID: artID, ArtifactType: "helm_chart", StoragePath: "/artifacts/" + artID.String()},
	}}
	cap := &stepCapture{}
	d := deployinator.New(cap.run, arts, "builder:latest")

	cfg := &shipcfg.DeployConfig{
		Artifact:  "helm_chart",
		Target:    "myapp",
		Namespace: "production",
	}

	if err := d.Run(context.Background(), store.NewJobID(), buildJobID, cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "helm upgrade --install myapp /artifacts/" + artID.String() + " --namespace production"
	if len(cap.spec.Command) < 3 || cap.spec.Command[2] != want {
		t.Errorf("command = %q, want %q", cap.spec.Command, want)
	}
}

func TestDeployinator_Run_Binary(t *testing.T) {
	buildJobID := store.NewJobID()
	artID := store.NewArtifactID()

	arts := &fakeArtifactStore{arts: []store.Artifact{
		{ID: artID, ArtifactType: "binary", StoragePath: "/artifacts/" + artID.String()},
	}}
	cap := &stepCapture{}
	d := deployinator.New(cap.run, arts, "builder:latest")

	cfg := &shipcfg.DeployConfig{
		Artifact:  "binary",
		Target:    "myapp",
		Namespace: "staging",
	}

	if err := d.Run(context.Background(), store.NewJobID(), buildJobID, cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "kubectl apply -f /artifacts/" + artID.String() + " --namespace staging"
	if len(cap.spec.Command) < 3 || cap.spec.Command[2] != want {
		t.Errorf("command = %q, want %q", cap.spec.Command, want)
	}
}

func TestDeployinator_Run_NoMatchingArtifact(t *testing.T) {
	arts := &fakeArtifactStore{arts: []store.Artifact{
		{ArtifactType: "binary"},
	}}
	cap := &stepCapture{}
	d := deployinator.New(cap.run, arts, "builder:latest")

	cfg := &shipcfg.DeployConfig{Artifact: "helm_chart", Target: "x", Namespace: "default"}

	err := d.Run(context.Background(), store.NewJobID(), store.NewJobID(), cfg)
	if err == nil {
		t.Fatal("expected error for missing artifact, got nil")
	}
}

func TestDeployinator_Run_EmptyBuildJob(t *testing.T) {
	arts := &fakeArtifactStore{arts: nil}
	cap := &stepCapture{}
	d := deployinator.New(cap.run, arts, "builder:latest")

	cfg := &shipcfg.DeployConfig{Artifact: "helm_chart", Target: "x", Namespace: "default"}

	err := d.Run(context.Background(), store.NewJobID(), store.JobID{}, cfg)
	if err == nil {
		t.Fatal("expected error for empty build job, got nil")
	}
}

func TestDeployinator_Run_StepError(t *testing.T) {
	buildJobID := store.NewJobID()
	arts := &fakeArtifactStore{arts: []store.Artifact{
		{ArtifactType: "helm_chart", StoragePath: "/artifacts/abc"},
	}}
	cap := &stepCapture{err: errors.New("execution failed")}
	d := deployinator.New(cap.run, arts, "builder:latest")

	cfg := &shipcfg.DeployConfig{Artifact: "helm_chart", Target: "x", Namespace: "default"}

	if err := d.Run(context.Background(), store.NewJobID(), buildJobID, cfg); err == nil {
		t.Fatal("expected error from step failure, got nil")
	}
}
