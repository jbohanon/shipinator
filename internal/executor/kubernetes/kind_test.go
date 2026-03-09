//go:build integration

package kubernetes_test

import (
	"fmt"
	"os/exec"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kindClusterName = "shipinator-test"
	testNamespace   = "default"
)

// testClient is initialised by TestMain and used by all integration tests.
var testClient kubernetes.Interface

func TestMain(m *testing.M) {
	teardown, err := setupKindCluster()
	if err != nil {
		panic(fmt.Sprintf("kind setup: %v", err))
	}
	defer teardown()
	m.Run()
}

// setupKindCluster creates a kind cluster, wires up testClient, and returns a
// teardown function that deletes the cluster.
func setupKindCluster() (func(), error) {
	out, err := exec.Command("kind", "create", "cluster", "--name", kindClusterName, "--wait", "60s").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kind create cluster: %w\n%s", err, out)
	}

	teardown := func() {
		exec.Command("kind", "delete", "cluster", "--name", kindClusterName).Run() //nolint:errcheck
	}

	kubeconfig, err := exec.Command("kind", "get", "kubeconfig", "--name", kindClusterName).Output()
	if err != nil {
		teardown()
		return nil, fmt.Errorf("kind get kubeconfig: %w", err)
	}

	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		teardown()
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		teardown()
		return nil, fmt.Errorf("create k8s client: %w", err)
	}

	testClient = client
	return teardown, nil
}
