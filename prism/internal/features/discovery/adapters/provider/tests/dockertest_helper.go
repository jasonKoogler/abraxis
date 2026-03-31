// Package tests provides testing utilities for the discovery adapters
package tests

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// DockerResources holds references to dockertest resources
type DockerResources struct {
	Pool     *dockertest.Pool
	Resource *dockertest.Resource
}

// SetupContainerWithRetry attempts to set up a Docker container with retries
func SetupContainerWithRetry(setupFn func() (interface{}, func(), error)) (interface{}, func(), error) {
	const maxRetries = 3
	var result interface{}
	var cleanup func()
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempting to set up container (attempt %d/%d)", attempt, maxRetries)

		result, cleanup, err = setupFn()
		if err == nil {
			log.Printf("Container setup successful on attempt %d", attempt)
			return result, cleanup, nil
		}

		log.Printf("Container setup failed on attempt %d: %s. Retrying in 2 seconds...", attempt, err)
		if cleanup != nil {
			cleanup()
		}

		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}

	return nil, nil, fmt.Errorf("failed to set up container after %d attempts: %w", maxRetries, err)
}

// CheckHTTPEndpoint checks if an HTTP endpoint is accessible
func CheckHTTPEndpoint(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned non-200 status code: %d", resp.StatusCode)
	}
	return nil
}

// SetupConsulContainer creates a Consul container for testing
// returns the consul address, a cleanup function, and an error
func SetupConsulContainer(t *testing.T) (string, func(), error) {
	// Create a new pool
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", nil, fmt.Errorf("failed to connect to docker: %w", err)
	}

	// Set a timeout for docker operations
	pool.MaxWait = time.Minute * 2

	// Pull and run the Consul image
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "hashicorp/consul",
		Tag:        "1.15",
		Env: []string{
			"CONSUL_BIND_INTERFACE=eth0",
		},
		ExposedPorts: []string{"8500/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"8500/tcp": {
				{HostIP: "0.0.0.0", HostPort: "0"},
			},
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to start consul container: %w", err)
	}

	// Create cleanup function
	purgeFunc := func() {
		if err := pool.Purge(resource); err != nil {
			log.Printf("Failed to purge consul container: %v", err)
		}
	}

	// Get the assigned host port
	hostPort := resource.GetPort("8500/tcp")
	consulAddress := fmt.Sprintf("http://localhost:%s", hostPort)

	// Wait for Consul to be ready
	err = pool.Retry(func() error {
		return CheckHTTPEndpoint(fmt.Sprintf("%s/v1/status/leader", consulAddress))
	})
	if err != nil {
		purgeFunc()
		return "", nil, fmt.Errorf("consul container not ready: %w", err)
	}

	return consulAddress, purgeFunc, nil
}

// SetupEtcdContainer creates an etcd container for testing
func SetupEtcdContainer(t *testing.T) ([]string, func(), error) {
	// Create a new pool
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to docker: %w", err)
	}

	// Set a timeout for docker operations
	pool.MaxWait = time.Minute * 2

	// Pull and run the etcd image
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "bitnami/etcd",
		Tag:        "3.5.9",
		Env: []string{
			"ALLOW_NONE_AUTHENTICATION=yes",
			"ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:2379",
		},
		ExposedPorts: []string{"2379/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"2379/tcp": {
				{HostIP: "0.0.0.0", HostPort: "0"},
			},
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start etcd container: %w", err)
	}

	// Create cleanup function
	purgeFunc := func() {
		if err := pool.Purge(resource); err != nil {
			log.Printf("Failed to purge etcd container: %v", err)
		}
	}

	// Get the assigned host port
	hostPort := resource.GetPort("2379/tcp")
	etcdEndpoint := fmt.Sprintf("http://localhost:%s", hostPort)
	endpoints := []string{etcdEndpoint}

	// Wait for etcd to be ready
	err = pool.Retry(func() error {
		return CheckHTTPEndpoint(fmt.Sprintf("%s/health", etcdEndpoint))
	})
	if err != nil {
		purgeFunc()
		return nil, nil, fmt.Errorf("etcd container not ready: %w", err)
	}

	return endpoints, purgeFunc, nil
}

// SetupKubernetesContainer creates a k3s container for Kubernetes testing
func SetupKubernetesContainer(t *testing.T) (string, func(), error) {
	// Create a new pool
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", nil, fmt.Errorf("failed to connect to docker: %w", err)
	}

	// Set a timeout for docker operations
	pool.MaxWait = time.Minute * 5 // K8s takes longer to start

	// Pull and run the k3s image (lighter than full Kubernetes)
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "rancher/k3s",
		Tag:        "v1.26.4-k3s1",
		Env: []string{
			"K3S_KUBECONFIG_OUTPUT=/tmp/kubeconfig.yaml",
			"K3S_KUBECONFIG_MODE=666",
		},
		ExposedPorts: []string{"6443/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"6443/tcp": {
				{HostIP: "0.0.0.0", HostPort: "0"}, // Let Docker assign a random port
			},
		},
		Privileged: true, // k3s needs to run as privileged
		Cmd:        []string{"server", "--disable-agent", "--disable=traefik"},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to start k3s container: %w", err)
	}

	// Create cleanup function
	purgeFunc := func() {
		if err := pool.Purge(resource); err != nil {
			log.Printf("Failed to purge k3s container: %v", err)
		}
	}

	// Get the assigned host port
	hostPort := resource.GetPort("6443/tcp")

	// Wait for Kubernetes API to be ready - this takes longer
	err = pool.Retry(func() error {
		// For simplicity, we'll just check if the container is running
		// A more robust check would verify the API server is responding
		time.Sleep(5 * time.Second)

		// Check if container is running
		container, err := pool.Client.InspectContainer(resource.Container.ID)
		if err != nil {
			return err
		}
		if !container.State.Running {
			return fmt.Errorf("container is not running")
		}
		return nil
	})
	if err != nil {
		purgeFunc()
		return "", nil, fmt.Errorf("k3s container not ready: %w", err)
	}

	// Copy kubeconfig from container to host for use
	kubeconfigPath := fmt.Sprintf("/tmp/kubeconfig-%s.yaml", resource.Container.ID[:12])

	// Use docker cp command to copy the kubeconfig
	cmd := exec.Command("docker", "cp", resource.Container.ID+":/tmp/kubeconfig.yaml", kubeconfigPath)
	if err := cmd.Run(); err != nil {
		purgeFunc()
		return "", nil, fmt.Errorf("failed to copy kubeconfig from container: %w", err)
	}

	// Update the server URL in the kubeconfig to use the host's port
	serverURL := fmt.Sprintf("https://localhost:%s", hostPort)
	sedCmd := exec.Command("sed", "-i", fmt.Sprintf("s|https://127.0.0.1:6443|%s|g", serverURL), kubeconfigPath)
	if err := sedCmd.Run(); err != nil {
		purgeFunc()
		return "", nil, fmt.Errorf("failed to update kubeconfig server URL: %w", err)
	}

	return kubeconfigPath, purgeFunc, nil
}

// Helper functions to skip tests if Docker is not available
func skipIfNoDocker(t *testing.T) {
	_, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("Docker not available - skipping test")
	}
}
