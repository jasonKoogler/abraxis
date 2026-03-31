package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jasonKoogler/prism/internal/ports"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesDiscovery provides service discovery using Kubernetes
type KubernetesDiscovery struct {
	Client        kubernetes.Interface
	config        ports.DiscoveryConfig
	namespace     string
	watchChannels map[string][]chan *ports.ServiceInstance
	mu            sync.RWMutex
	ctx           context.Context
	ctxCancel     context.CancelFunc
	podWatcher    watch.Interface
	watchCancels  map[string]context.CancelFunc
}

var _ ports.ServiceDiscoverer = &KubernetesDiscovery{}

// For testing only - allows tests to inject mock clients
// var SetKubernetesClient func(d *KubernetesDiscovery, client interface{})

// For testing only - allows tests to create KubernetesDiscovery with a fake client
func NewKubernetesDiscoveryWithClient(ctx context.Context, client kubernetes.Interface, namespace string) *KubernetesDiscovery {
	watchCtx, watchCtxCancel := context.WithCancel(context.Background())

	config := ports.DiscoveryConfig{
		Provider: "kubernetes",
		Kubernetes: &ports.KubernetesConfig{
			Namespace: namespace,
		},
	}

	discovery := &KubernetesDiscovery{
		Client:        client,
		config:        config,
		namespace:     namespace,
		watchChannels: make(map[string][]chan *ports.ServiceInstance),
		ctx:           watchCtx,
		ctxCancel:     watchCtxCancel,
	}

	return discovery
}

// NewKubernetesDiscovery creates a new Kubernetes-based service discovery
func NewKubernetesDiscovery(ctx context.Context, config ports.DiscoveryConfig) (*KubernetesDiscovery, error) {
	var kubeConfig *rest.Config
	var err error

	// Create Kubernetes client configuration
	if config.Kubernetes.InCluster {
		// Use in-cluster configuration
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		// Use kubeconfig file
		kubeConfigPath := config.Kubernetes.ConfigPath
		if kubeConfigPath == "" {
			kubeConfigPath = defaultKubeConfigPath()
		}

		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Use specified namespace or default
	namespace := config.Kubernetes.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Create context for watch operations
	watchCtx, watchCtxCancel := context.WithCancel(context.Background())

	discovery := &KubernetesDiscovery{
		Client:        client,
		config:        config,
		namespace:     namespace,
		watchChannels: make(map[string][]chan *ports.ServiceInstance),
		ctx:           watchCtx,
		ctxCancel:     watchCtxCancel,
	}

	// Start watching pods
	if err := discovery.startPodWatcher(); err != nil {
		return nil, err
	}

	return discovery, nil
}

func (d *KubernetesDiscovery) GetProviderName() string {
	return "kubernetes"
}

// RegisterInstance registers a service instance using pod annotations
func (d *KubernetesDiscovery) RegisterInstance(ctx context.Context, instance *ports.ServiceInstance) error {
	// In Kubernetes, we don't actually register services directly.
	// Instead, we update pod annotations to store service information.

	// Find the pod by hostname
	podName := getPodNameFromInstance(instance)
	if podName == "" {
		return fmt.Errorf("unable to determine pod name from instance ID: %s", instance.ID)
	}

	pod, err := d.Client.CoreV1().Pods(d.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s: %w", podName, err)
	}

	// Update pod annotations
	annotations := map[string]string{
		"service.discovery/service-id":   instance.ID,
		"service.discovery/service-name": instance.ServiceName,
		"service.discovery/version":      instance.Version,
		"service.discovery/address":      instance.Address,
		"service.discovery/port":         strconv.Itoa(instance.Port),
		"service.discovery/status":       instance.Status,
		"service.discovery/heartbeat":    time.Now().Format(time.RFC3339),
	}

	// Add tags as comma-separated string
	if len(instance.Tags) > 0 {
		annotations["service.discovery/tags"] = strings.Join(instance.Tags, ",")
	}

	// Add metadata
	for k, v := range instance.Metadata {
		annotations[fmt.Sprintf("service.discovery/meta-%s", k)] = v
	}

	// Update pod with new annotations
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	for k, v := range annotations {
		pod.Annotations[k] = v
	}

	_, err = d.Client.CoreV1().Pods(d.namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod annotations: %w", err)
	}

	return nil
}

// DeregisterInstance updates the pod annotation to mark service as deregistered
func (d *KubernetesDiscovery) DeregisterInstance(ctx context.Context, instanceID string) error {
	// In Kubernetes, we update the status annotation to mark as deregistered

	// Find the pod by instance ID
	pods, err := d.findPodsByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for instance ID: %s", instanceID)
	}

	// Update the first matching pod (should be only one)
	pod := pods[0]

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations["service.discovery/status"] = "DEREGISTERED"
	pod.Annotations["service.discovery/heartbeat"] = time.Now().Format(time.RFC3339)

	_, err = d.Client.CoreV1().Pods(d.namespace).Update(ctx, &pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod status: %w", err)
	}

	return nil
}

// UpdateInstanceStatus updates the status of a service instance
func (d *KubernetesDiscovery) UpdateInstanceStatus(ctx context.Context, instanceID, status string) error {
	// Find the pod by instance ID
	pods, err := d.findPodsByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for instance ID: %s", instanceID)
	}

	// Update the first matching pod (should be only one)
	pod := pods[0]

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations["service.discovery/status"] = status
	pod.Annotations["service.discovery/heartbeat"] = time.Now().Format(time.RFC3339)

	_, err = d.Client.CoreV1().Pods(d.namespace).Update(ctx, &pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod status: %w", err)
	}

	return nil
}

// ListInstances returns service instances based on pod annotations
func (d *KubernetesDiscovery) ListInstances(ctx context.Context, serviceName ...string) ([]*ports.ServiceInstance, error) {
	var instances []*ports.ServiceInstance

	// Create label selector for service pods
	req, err := labels.NewRequirement("service.discovery", selection.Exists, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to create label selector: %w", err)
	}

	selector := labels.NewSelector().Add(*req)

	// Get pods with service discovery annotations
	pods, err := d.Client.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Convert pods to service instances
	for _, pod := range pods.Items {
		instance, err := d.podToServiceInstance(&pod)
		if err != nil {
			continue
		}

		// Filter by service name if specified
		if len(serviceName) > 0 && serviceName[0] != "" && instance.ServiceName != serviceName[0] {
			continue
		}

		// Skip non-active instances unless they're specifically requested
		if instance.Status != "ACTIVE" && (len(serviceName) == 0 || serviceName[0] == "") {
			continue
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

// GetInstance returns a specific instance by ID
func (d *KubernetesDiscovery) GetInstance(ctx context.Context, instanceID string) (*ports.ServiceInstance, error) {
	// Find pods with matching instance ID
	pods, err := d.findPodsByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}

	// Convert the first matching pod to a service instance
	return d.podToServiceInstance(&pods[0])
}

// ListServices returns information about all registered services
func (d *KubernetesDiscovery) ListServices(ctx context.Context) ([]*ports.ServiceInfo, error) {
	// Get all service instances
	pods, err := d.Client.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Count services by name and version
	serviceVersions := make(map[string]map[string]int) // serviceName -> (version -> count)

	for _, pod := range pods.Items {
		if pod.Annotations == nil {
			continue
		}

		serviceName, ok := pod.Annotations["service.discovery/service-name"]
		if !ok {
			continue
		}

		version := pod.Annotations["service.discovery/version"]
		status := pod.Annotations["service.discovery/status"]

		// Skip non-active services
		if status != "ACTIVE" {
			continue
		}

		// Initialize version map if needed
		if _, exists := serviceVersions[serviceName]; !exists {
			serviceVersions[serviceName] = make(map[string]int)
		}

		// Increment count
		serviceVersions[serviceName][version]++
	}

	// Create service info objects
	var services []*ports.ServiceInfo
	for serviceName, versions := range serviceVersions {
		for version, count := range versions {
			services = append(services, &ports.ServiceInfo{
				Name:    serviceName,
				Version: version,
				Count:   count,
			})
		}
	}

	return services, nil
}

// WatchServices returns a channel that receives instance updates
func (d *KubernetesDiscovery) WatchServices(ctx context.Context, serviceName ...string) (<-chan *ports.ServiceInstance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create channel for updates
	ch := make(chan *ports.ServiceInstance, 10)

	// Determine service filter
	serviceFilter := ""
	if len(serviceName) > 0 && serviceName[0] != "" {
		serviceFilter = serviceName[0]
	}

	// Register channel
	d.watchChannels[serviceFilter] = append(d.watchChannels[serviceFilter], ch)

	// Send initial state
	go func() {
		instances, err := d.ListInstances(ctx, serviceName...)
		if err != nil {
			return
		}

		for _, instance := range instances {
			select {
			case ch <- instance:
				// Successfully sent
			case <-ctx.Done():
				return
			}
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		d.removeWatchChannel(ch, serviceFilter)
	}()

	return ch, nil
}

// Heartbeat sends a heartbeat for a service instance
func (d *KubernetesDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	// Find the pod by instance ID
	pods, err := d.findPodsByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for instance ID: %s", instanceID)
	}

	// Update the heartbeat timestamp
	pod := pods[0]

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations["service.discovery/heartbeat"] = time.Now().Format(time.RFC3339)

	// Update status to ACTIVE if it was INACTIVE
	if pod.Annotations["service.discovery/status"] == "INACTIVE" {
		pod.Annotations["service.discovery/status"] = "ACTIVE"
	}

	_, err = d.Client.CoreV1().Pods(d.namespace).Update(ctx, &pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod heartbeat: %w", err)
	}

	return nil
}

// Close shuts down the Kubernetes service discovery
func (d *KubernetesDiscovery) Close() error {
	d.ctxCancel()

	// Stop pod watcher
	if d.podWatcher != nil {
		d.podWatcher.Stop()
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Close all watch channels
	for service, channels := range d.watchChannels {
		for _, ch := range channels {
			close(ch)
		}
		delete(d.watchChannels, service)
	}

	return nil
}

// Helper methods

// startPodWatcher starts watching pods for changes
func (d *KubernetesDiscovery) startPodWatcher() error {
	var err error

	// Set up pod watcher
	d.podWatcher, err = d.Client.CoreV1().Pods(d.namespace).Watch(d.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start pod watcher: %w", err)
	}

	// Handle pod events
	go func() {
		for event := range d.podWatcher.ResultChan() {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Check if this pod has service discovery annotations
			if pod.Annotations == nil || pod.Annotations["service.discovery/service-name"] == "" {
				continue
			}

			// Convert to service instance
			instance, err := d.podToServiceInstance(pod)
			if err != nil {
				continue
			}

			// Notify watchers
			d.notifyWatchers(instance)
		}
	}()

	return nil
}

// podToServiceInstance converts a Kubernetes pod to a ServiceInstance
func (d *KubernetesDiscovery) podToServiceInstance(pod *corev1.Pod) (*ports.ServiceInstance, error) {
	if pod.Annotations == nil {
		return nil, fmt.Errorf("pod has no annotations")
	}

	// Get required fields
	serviceID := pod.Annotations["service.discovery/service-id"]
	serviceName := pod.Annotations["service.discovery/service-name"]

	if serviceName == "" {
		return nil, fmt.Errorf("missing service name annotation")
	}

	// If service ID is missing, use pod name
	if serviceID == "" {
		serviceID = pod.Name
	}

	// Get port
	portStr := pod.Annotations["service.discovery/port"]
	port := 0
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	// Use pod IP as address if not specified
	address := pod.Annotations["service.discovery/address"]
	if address == "" {
		address = pod.Status.PodIP
	}

	// Get status (default to pod phase if not specified)
	status := pod.Annotations["service.discovery/status"]
	if status == "" {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			status = "ACTIVE"
		case corev1.PodSucceeded:
			status = "DEREGISTERED"
		case corev1.PodFailed:
			status = "INACTIVE"
		default:
			status = "INACTIVE"
		}
	}

	// Get tags
	var tags []string
	tagsStr := pod.Annotations["service.discovery/tags"]
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	// Get metadata
	metadata := make(map[string]string)
	for key, value := range pod.Annotations {
		if strings.HasPrefix(key, "service.discovery/meta-") {
			metaKey := strings.TrimPrefix(key, "service.discovery/meta-")
			metadata[metaKey] = value
		}
	}

	// Parse timestamps
	var registeredAt, lastHeartbeat time.Time

	if ts := pod.Annotations["service.discovery/registered-at"]; ts != "" {
		registeredAt, _ = time.Parse(time.RFC3339, ts)
	} else {
		// Use pod creation time if not specified
		registeredAt = pod.CreationTimestamp.Time
	}

	if ts := pod.Annotations["service.discovery/heartbeat"]; ts != "" {
		lastHeartbeat, _ = time.Parse(time.RFC3339, ts)
	}

	// Create service instance
	instance := &ports.ServiceInstance{
		ID:            serviceID,
		ServiceName:   serviceName,
		Version:       pod.Annotations["service.discovery/version"],
		Address:       address,
		Port:          port,
		Status:        status,
		Tags:          tags,
		Metadata:      metadata,
		RegisteredAt:  registeredAt,
		LastHeartbeat: lastHeartbeat,
	}

	return instance, nil
}

// findPodsByInstanceID finds pods with a specific instance ID
func (d *KubernetesDiscovery) findPodsByInstanceID(ctx context.Context, instanceID string) ([]corev1.Pod, error) {
	// Try to find by service ID annotation
	fieldSelector := fields.OneTermEqualSelector("metadata.annotations.service.discovery/service-id", instanceID)
	pods, err := d.Client.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) > 0 {
		return pods.Items, nil
	}

	// If not found by annotation, try pod name
	pod, err := d.Client.CoreV1().Pods(d.namespace).Get(ctx, instanceID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("pod not found: %w", err)
	}

	return []corev1.Pod{*pod}, nil
}

// notifyWatchers sends instance updates to registered watchers
func (d *KubernetesDiscovery) notifyWatchers(instance *ports.ServiceInstance) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Notify subscribers to this specific service
	if channels, exists := d.watchChannels[instance.ServiceName]; exists {
		for _, ch := range channels {
			go func(c chan *ports.ServiceInstance, inst *ports.ServiceInstance) {
				select {
				case c <- inst:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					// Channel might be blocked, skip this update
				case <-d.ctx.Done():
					return
				}
			}(ch, instance)
		}
	}

	// Also notify subscribers to all services
	if channels, exists := d.watchChannels[""]; exists {
		for _, ch := range channels {
			go func(c chan *ports.ServiceInstance, inst *ports.ServiceInstance) {
				select {
				case c <- inst:
					// Successfully sent
				case <-time.After(100 * time.Millisecond):
					// Channel might be blocked, skip this update
				case <-d.ctx.Done():
					return
				}
			}(ch, instance)
		}
	}
}

// removeWatchChannel removes a watch channel from the specified service
func (d *KubernetesDiscovery) removeWatchChannel(ch chan *ports.ServiceInstance, serviceFilter string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	channels, exists := d.watchChannels[serviceFilter]
	if !exists {
		close(ch)
		return
	}

	for i, c := range channels {
		if c == ch {
			// Remove channel
			d.watchChannels[serviceFilter] = append(channels[:i], channels[i+1:]...)

			// Clean up empty service watch list
			if len(d.watchChannels[serviceFilter]) == 0 {
				delete(d.watchChannels, serviceFilter)

				// Stop watching this service if no more channels
				if cancel, exists := d.watchCancels[serviceFilter]; exists {
					cancel()
					delete(d.watchCancels, serviceFilter)
				}
			}
		}
	}
}

func defaultKubeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

func getPodNameFromInstance(instance *ports.ServiceInstance) string {
	// TODO: Implement, this isn't the pod name but the instance id
	return instance.ID
}
