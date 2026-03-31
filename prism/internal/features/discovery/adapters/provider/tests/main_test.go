package tests

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

var (
	// skipIntegration is a flag that can be set to skip integration tests
	skipIntegration = flag.Bool("skip-integration", false, "Skip integration tests that require Docker")

	// requiredDockerImages is a list of Docker images required for tests
	requiredDockerImages = []string{
		"hashicorp/consul:1.15",
		"bitnami/etcd:3.5.9",
		"rancher/k3s:v1.26.4-k3s1",
	}
)

// TestMain is the entry point for all discovery tests
func TestMain(m *testing.M) {
	flag.Parse()

	// Check if we're running in CI or with environment variables set
	if os.Getenv("CI") != "" || os.Getenv("SKIP_INTEGRATION_TESTS") != "" || *skipIntegration {
		// In CI, we'll skip integration tests unless explicitly enabled
		os.Setenv("SKIP_INTEGRATION_TESTS", "true")
		fmt.Println("Skipping integration tests due to CI environment or explicit skip flag")
	} else if !checkDockerAvailable() {
		// If Docker is not available, skip integration tests
		os.Setenv("SKIP_INTEGRATION_TESTS", "true")
		fmt.Println("Docker is not available. Integration tests will be skipped.")
	} else {
		// Docker is available, check if required images are available or can be pulled
		imagesAvailable := checkRequiredDockerImages()
		if !imagesAvailable {
			os.Setenv("SKIP_INTEGRATION_TESTS", "true")
			fmt.Println("Not all required Docker images are available. Integration tests will be skipped.")
		} else {
			fmt.Println("Docker and all required images are available. Integration tests will run.")
		}
	}

	// Run the tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

// checkDockerAvailable returns true if Docker is available, false otherwise
func checkDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// checkRequiredDockerImages checks if all required Docker images are available
// or can be pulled, and returns true if all are available
func checkRequiredDockerImages() bool {
	for _, image := range requiredDockerImages {
		fmt.Printf("Checking for Docker image: %s\n", image)

		// Check if image exists locally
		checkCmd := exec.Command("docker", "image", "inspect", image)
		if err := checkCmd.Run(); err == nil {
			fmt.Printf("  - Image %s found locally\n", image)
			continue
		}

		// Try to pull the image
		fmt.Printf("  - Image %s not found locally, attempting to pull...\n", image)
		pullCmd := exec.Command("docker", "pull", image)
		if err := pullCmd.Run(); err != nil {
			fmt.Printf("  - Failed to pull image %s: %v\n", image, err)
			return false
		}
		fmt.Printf("  - Successfully pulled image %s\n", image)
	}

	return true
}
