package tests

import (
	"context"
	"testing"

	"github.com/jasonKoogler/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// TestKubernetesDiscoveryWithFakeClient tests the Kubernetes discovery with a fake client
func TestKubernetesDiscoveryWithFakeClient(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create test namespace
	namespace := "default"

	// Create test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
			Annotations: map[string]string{
				"service.discovery/service-id":   "test-service-1",
				"service.discovery/service-name": "test-service",
				"service.discovery/version":      "1.0.0",
				"service.discovery/address":      "localhost",
				"service.discovery/port":         "8080",
				"service.discovery/status":       "ACTIVE",
				"service.discovery/tags":         "api,v1",
				"service.discovery/meta-region":  "us-east-1",
			},
			Labels: map[string]string{
				"service.discovery": "true",
				"app":               "test-service",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	// Add pod to fake client
	ctx := context.Background()
	_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create a Kubernetes discovery service with the fake client
	kubeDiscovery := provider.NewKubernetesDiscoveryWithClient(ctx, clientset, namespace)
	require.NotNil(t, kubeDiscovery)

	t.Run("verify_provider_name", func(t *testing.T) {
		assert.Equal(t, "kubernetes", kubeDiscovery.GetProviderName())
	})

	t.Run("list_instances", func(t *testing.T) {
		instances, err := kubeDiscovery.ListInstances(ctx, "test-service")
		require.NoError(t, err)

		assert.Len(t, instances, 1)
		assert.Equal(t, "test-service-1", instances[0].ID)
		assert.Equal(t, "test-service", instances[0].ServiceName)
	})

	t.Run("get_instance", func(t *testing.T) {
		instance, err := kubeDiscovery.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.NotNil(t, instance)
		assert.Equal(t, "test-service-1", instance.ID)
		assert.Equal(t, "test-service", instance.ServiceName)
		assert.Equal(t, "1.0.0", instance.Version)
	})

	t.Run("register_instance", func(t *testing.T) {
		instance := &ports.ServiceInstance{
			ID:          "test-service-2",
			ServiceName: "test-service",
			Version:     "1.0.1",
			Address:     "localhost",
			Port:        8081,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v2"},
			Metadata: map[string]string{
				"region": "us-west-1",
			},
		}

		// This will attempt to find and update the pod with the name test-service-2
		// Since it doesn't exist in our fake client, we'll create it
		pod2 := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-2",
				Namespace: namespace,
				Labels: map[string]string{
					"service.discovery": "true",
					"app":               "test-service",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.2",
			},
		}

		_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod2, metav1.CreateOptions{})
		require.NoError(t, err)

		err = kubeDiscovery.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Get updated pod
		updatedPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, "test-service-2", metav1.GetOptions{})
		require.NoError(t, err)

		// Verify annotations were added
		assert.Equal(t, "test-service-2", updatedPod.Annotations["service.discovery/service-id"])
		assert.Equal(t, "test-service", updatedPod.Annotations["service.discovery/service-name"])
		assert.Equal(t, "1.0.1", updatedPod.Annotations["service.discovery/version"])
	})

	t.Run("deregister_instance", func(t *testing.T) {
		err := kubeDiscovery.DeregisterInstance(ctx, "test-service-1")
		require.NoError(t, err)

		// Get updated pod
		updatedPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, "test-pod", metav1.GetOptions{})
		require.NoError(t, err)

		// Verify status was updated
		assert.Equal(t, "DEREGISTERED", updatedPod.Annotations["service.discovery/status"])
	})
}

// Helper function to set up test pods in the fake clientset
func setupTestPods(t *testing.T, ctx context.Context, clientset kubernetes.Interface) {
	pod1 := createTestPod(t, "test-pod-1", "test-service-1", "test-service")
	pod2 := createTestPod(t, "test-pod-2", "test-service-2", "test-service")

	// Add pods to the fake clientset
	_, err := clientset.CoreV1().Pods("default").Create(ctx, pod1, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().Pods("default").Create(ctx, pod2, metav1.CreateOptions{})
	require.NoError(t, err)
}

// Helper function to create a test pod
func createTestPod(t *testing.T, podName, serviceID, serviceName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Annotations: map[string]string{
				"service.discovery/service-id":   serviceID,
				"service.discovery/service-name": serviceName,
				"service.discovery/version":      "1.0.0",
				"service.discovery/port":         "8080",
			},
			Labels: map[string]string{
				"app": serviceName,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

// Helper function to create another test pod with a different service
func createOtherServicePod(t *testing.T, ctx context.Context, clientset kubernetes.Interface) {
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod-1",
			Namespace: "default",
			Annotations: map[string]string{
				"service.discovery/service-id":   "other-service-1",
				"service.discovery/service-name": "other-service",
				"service.discovery/version":      "2.0.0",
				"service.discovery/port":         "9090",
			},
			Labels: map[string]string{
				"app": "other-service",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.3",
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	_, err := clientset.CoreV1().Pods("default").Create(ctx, pod3, metav1.CreateOptions{})
	require.NoError(t, err)
}

// TestKubernetesDiscoveryIntegration is temporarily disabled while we implement the K3s docker container
// func TestKubernetesDiscoveryIntegration(t *testing.T) {
// 	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
// 		t.Skip("Skipping integration test")
// 	}

// 	// Create K3s container
// 	kubeconfigPath, purgeFunc, err := SetupKubernetesContainer(t)
// 	if err != nil {
// 		t.Skipf("Failed to start K3s container: %v", err)
// 		return
// 	}
// 	defer purgeFunc()

// 	t.Logf("Started K3s container with kubeconfig at: %s", kubeconfigPath)

// 	// Create discovery service config
// 	ctx := context.Background()
// 	kubeConfig := ports.DiscoveryConfig{
// 		Provider: "kubernetes",
// 		Kubernetes: &ports.KubernetesConfig{
// 			InCluster:  false,
// 			ConfigPath: kubeconfigPath,
// 			Namespace:  "default",
// 		},
// 		HeartbeatInterval: 1 * time.Second,
// 		HeartbeatTimeout:  5 * time.Second,
// 		DeregisterTimeout: 10 * time.Second,
// 	}

// 	// Create discovery service
// 	kubeDiscovery, err := discovery.NewKubernetesDiscovery(ctx, kubeConfig)
// 	if err != nil {
// 		t.Fatalf("Failed to create Kubernetes discovery service: %v", err)
// 	}

// 	t.Run("verify_provider_name", func(t *testing.T) {
// 		assert.Equal(t, "kubernetes", kubeDiscovery.GetProviderName())
// 	})

// 	t.Run("create_and_register_pod", func(t *testing.T) {
// 		// Create a test pod
// 		pod := &corev1.Pod{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      "integration-test-pod",
// 				Namespace: "default",
// 				Labels: map[string]string{
// 					"app": "integration-test",
// 				},
// 			},
// 			Spec: corev1.PodSpec{
// 				Containers: []corev1.Container{
// 					{
// 						Name:  "nginx",
// 						Image: "nginx:latest",
// 						Ports: []corev1.ContainerPort{
// 							{
// 								ContainerPort: 80,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}

// 		// Create the pod
// 		_, err := kubeDiscovery.Client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
// 		require.NoError(t, err)

// 		// Register it as a service instance
// 		instance := &ports.ServiceInstance{
// 			ID:          "integration-test-service",
// 			ServiceName: "integration-service",
// 			Version:     "1.0.0",
// 			Address:     "localhost",
// 			Port:        80,
// 			Status:      "ACTIVE",
// 			Tags:        []string{"integration", "test"},
// 			Metadata: map[string]string{
// 				"region": "test-region",
// 			},
// 		}

// 		// Register the instance
// 		err = kubeDiscovery.RegisterInstance(ctx, instance)
// 		require.NoError(t, err)

// 		// Get the instance back
// 		retrieved, err := kubeDiscovery.GetInstance(ctx, "integration-test-service")
// 		require.NoError(t, err)
// 		assert.Equal(t, "integration-test-service", retrieved.ID)
// 		assert.Equal(t, "integration-service", retrieved.ServiceName)
// 		assert.Equal(t, "1.0.0", retrieved.Version)

// 		// Cleanup
// 		err = kubeDiscovery.DeregisterInstance(ctx, "integration-test-service")
// 		require.NoError(t, err)
// 	})
// }
