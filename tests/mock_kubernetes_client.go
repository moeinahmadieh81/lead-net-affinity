package tests

import (
	"context"
	"time"

	"lead-framework/internal/models"
)

// MockKubernetesClient simulates Kubernetes client for testing
type MockKubernetesClient struct {
	pods      []*models.PodInfo
	nodes     []*MockNodeInfo
	eventChan chan PodEvent
	ctx       context.Context
	cancel    context.CancelFunc
}

// MockNodeInfo represents mock node information
type MockNodeInfo struct {
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	CreationTime     time.Time         `json:"creation_time"`
	InternalIP       string            `json:"internal_ip"`
	AvailabilityZone string            `json:"availability_zone"`
}

// PodEvent represents a pod lifecycle event
type PodEvent struct {
	Type      string          `json:"type"` // ADDED, MODIFIED, DELETED
	Pod       *models.PodInfo `json:"pod"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewMockKubernetesClient creates a new mock Kubernetes client
func NewMockKubernetesClient(namespace string) *MockKubernetesClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &MockKubernetesClient{
		pods:      make([]*models.PodInfo, 0),
		nodes:     createMockNodes(),
		eventChan: make(chan PodEvent, 100),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// GetCurrentPods returns all current pods
func (mkc *MockKubernetesClient) GetCurrentPods() ([]*models.PodInfo, error) {
	return mkc.pods, nil
}

// GetPodsByService returns pods for a specific service
func (mkc *MockKubernetesClient) GetPodsByService(serviceName string) ([]*models.PodInfo, error) {
	var servicePods []*models.PodInfo
	for _, pod := range mkc.pods {
		if pod.ServiceName == serviceName {
			servicePods = append(servicePods, pod)
		}
	}
	return servicePods, nil
}

// GetPodEvents returns a channel for receiving pod events
func (mkc *MockKubernetesClient) GetPodEvents() <-chan PodEvent {
	return mkc.eventChan
}

// GetNodes returns cluster node information
func (mkc *MockKubernetesClient) GetNodes() ([]*MockNodeInfo, error) {
	return mkc.nodes, nil
}

// AddPod simulates adding a pod
func (mkc *MockKubernetesClient) AddPod(pod *models.PodInfo) {
	mkc.pods = append(mkc.pods, pod)

	select {
	case mkc.eventChan <- PodEvent{
		Type:      "ADDED",
		Pod:       pod,
		Timestamp: time.Now(),
	}:
	default:
		// Channel full, skip event
	}
}

// RemovePod simulates removing a pod
func (mkc *MockKubernetesClient) RemovePod(podName string) {
	for i, pod := range mkc.pods {
		if pod.Name == podName {
			// Remove from slice
			mkc.pods = append(mkc.pods[:i], mkc.pods[i+1:]...)

			select {
			case mkc.eventChan <- PodEvent{
				Type:      "DELETED",
				Pod:       pod,
				Timestamp: time.Now(),
			}:
			default:
				// Channel full, skip event
			}
			break
		}
	}
}

// UpdatePod simulates updating a pod
func (mkc *MockKubernetesClient) UpdatePod(updatedPod *models.PodInfo) {
	for i, pod := range mkc.pods {
		if pod.Name == updatedPod.Name {
			mkc.pods[i] = updatedPod

			select {
			case mkc.eventChan <- PodEvent{
				Type:      "MODIFIED",
				Pod:       updatedPod,
				Timestamp: time.Now(),
			}:
			default:
				// Channel full, skip event
			}
			break
		}
	}
}

// Stop stops the mock client
func (mkc *MockKubernetesClient) Stop() {
	mkc.cancel()
	close(mkc.eventChan)
}

// createMockNodes creates mock cluster nodes
func createMockNodes() []*MockNodeInfo {
	return []*MockNodeInfo{
		{
			Name:             "node-1",
			Labels:           map[string]string{"topology.kubernetes.io/zone": "us-west-1a"},
			Annotations:      map[string]string{},
			CreationTime:     time.Now(),
			InternalIP:       "10.0.1.10",
			AvailabilityZone: "us-west-1a",
		},
		{
			Name:             "node-2",
			Labels:           map[string]string{"topology.kubernetes.io/zone": "us-west-1b"},
			Annotations:      map[string]string{},
			CreationTime:     time.Now(),
			InternalIP:       "10.0.1.11",
			AvailabilityZone: "us-west-1b",
		},
		{
			Name:             "node-3",
			Labels:           map[string]string{"topology.kubernetes.io/zone": "us-west-1c"},
			Annotations:      map[string]string{},
			CreationTime:     time.Now(),
			InternalIP:       "10.0.1.12",
			AvailabilityZone: "us-west-1c",
		},
	}
}

// createMockHotelReservationPods creates mock pods for HotelReservation benchmark
func createMockHotelReservationPods() []*models.PodInfo {
	now := time.Now()

	return []*models.PodInfo{
		// Frontend pods
		{
			Name:             "frontend-deployment-abc123",
			Namespace:        "hotel-reservation",
			ServiceName:      "frontend",
			ServiceType:      "microservice",
			NodeName:         "node-1",
			PodIP:            "10.244.1.10",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "frontend"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "100m", Memory: "128Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "500m", Memory: "512Mi"},
			CreationTime:     now,
		},
		{
			Name:             "frontend-deployment-def456",
			Namespace:        "hotel-reservation",
			ServiceName:      "frontend",
			ServiceType:      "microservice",
			NodeName:         "node-1",
			PodIP:            "10.244.1.11",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "frontend"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "100m", Memory: "128Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "500m", Memory: "512Mi"},
			CreationTime:     now,
		},

		// Search service pods
		{
			Name:             "search-deployment-ghi789",
			Namespace:        "hotel-reservation",
			ServiceName:      "search",
			ServiceType:      "microservice",
			NodeName:         "node-2",
			PodIP:            "10.244.2.10",
			HostIP:           "10.0.1.11",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "search"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "150m", Memory: "256Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "800m", Memory: "1Gi"},
			CreationTime:     now,
		},

		// User service pods
		{
			Name:             "user-deployment-jkl012",
			Namespace:        "hotel-reservation",
			ServiceName:      "user",
			ServiceType:      "microservice",
			NodeName:         "node-1",
			PodIP:            "10.244.1.12",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "user"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "100m", Memory: "128Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "500m", Memory: "512Mi"},
			CreationTime:     now,
		},

		// Profile service pods
		{
			Name:             "profile-deployment-mno345",
			Namespace:        "hotel-reservation",
			ServiceName:      "profile",
			ServiceType:      "microservice",
			NodeName:         "node-1",
			PodIP:            "10.244.1.13",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "profile"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "100m", Memory: "128Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "400m", Memory: "512Mi"},
			CreationTime:     now,
		},

		// MongoDB pods
		{
			Name:             "mongodb-profile-deployment-pqr678",
			Namespace:        "hotel-reservation",
			ServiceName:      "mongodb-profile",
			ServiceType:      "mongodb",
			NodeName:         "node-1",
			PodIP:            "10.244.1.14",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "mongodb-profile"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "200m", Memory: "512Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "1000m", Memory: "2Gi"},
			CreationTime:     now,
		},

		// Memcached pods
		{
			Name:             "memcached-profile-deployment-stu901",
			Namespace:        "hotel-reservation",
			ServiceName:      "memcached-profile",
			ServiceType:      "memcached",
			NodeName:         "node-1",
			PodIP:            "10.244.1.15",
			HostIP:           "10.0.1.10",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "memcached-profile"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "50m", Memory: "64Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "200m", Memory: "256Mi"},
			CreationTime:     now,
		},

		// Recommendation service
		{
			Name:             "recommendation-deployment-vwx234",
			Namespace:        "hotel-reservation",
			ServiceName:      "recommendation",
			ServiceType:      "microservice",
			NodeName:         "node-3",
			PodIP:            "10.244.3.10",
			HostIP:           "10.0.1.12",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "recommendation"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "100m", Memory: "128Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "400m", Memory: "512Mi"},
			CreationTime:     now,
		},

		// Reservation service
		{
			Name:             "reservation-deployment-yza567",
			Namespace:        "hotel-reservation",
			ServiceName:      "reservation",
			ServiceType:      "microservice",
			NodeName:         "node-2",
			PodIP:            "10.244.2.11",
			HostIP:           "10.0.1.11",
			Status:           "Running",
			Labels:           map[string]string{"io.kompose.service": "reservation"},
			Annotations:      map[string]string{},
			ResourceRequests: models.ResourceInfo{CPU: "150m", Memory: "256Mi"},
			ResourceLimits:   models.ResourceInfo{CPU: "800m", Memory: "1Gi"},
			CreationTime:     now,
		},
	}
}
