package discovery

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"lead-framework/internal/kubernetes"
	"lead-framework/internal/models"
)

// ServiceDiscovery provides dynamic service discovery for LEAD framework
type ServiceDiscovery struct {
	k8sClient      *kubernetes.KubernetesClient
	serviceGraph   *models.ServiceGraph
	graphMutex     sync.RWMutex
	updateCallback func(*models.ServiceGraph)
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
}

// NewServiceDiscovery creates a new service discovery instance
func NewServiceDiscovery(k8sClient *kubernetes.KubernetesClient) *ServiceDiscovery {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceDiscovery{
		k8sClient:    k8sClient,
		serviceGraph: models.NewServiceGraph(),
		ctx:          ctx,
		cancel:       cancel,
		isRunning:    false,
	}
}

// Start begins the service discovery process
func (sd *ServiceDiscovery) Start() error {
	if sd.isRunning {
		return fmt.Errorf("service discovery is already running")
	}

	sd.isRunning = true
	log.Println("Starting service discovery...")

	// Initial discovery
	if err := sd.discoverServices(); err != nil {
		return fmt.Errorf("failed to perform initial service discovery: %v", err)
	}

	// Start watching for pod events
	go sd.watchPodEvents()

	// Start periodic refresh
	go sd.periodicRefresh()

	log.Printf("Service discovery started, found %d services", len(sd.serviceGraph.GetAllNodes()))
	return nil
}

// Stop stops the service discovery
func (sd *ServiceDiscovery) Stop() {
	if !sd.isRunning {
		return
	}

	log.Println("Stopping service discovery...")
	sd.cancel()
	sd.isRunning = false
	log.Println("Service discovery stopped")
}

// GetServiceGraph returns the current service graph
func (sd *ServiceDiscovery) GetServiceGraph() *models.ServiceGraph {
	sd.graphMutex.RLock()
	defer sd.graphMutex.RUnlock()

	// Return a copy to avoid race conditions
	return sd.copyServiceGraph()
}

// SetUpdateCallback sets a callback function to be called when the graph is updated
func (sd *ServiceDiscovery) SetUpdateCallback(callback func(*models.ServiceGraph)) {
	sd.updateCallback = callback
}

// discoverServices performs initial service discovery
func (sd *ServiceDiscovery) discoverServices() error {
	pods, err := sd.k8sClient.GetCurrentPods()
	if err != nil {
		return fmt.Errorf("failed to get current pods: %v", err)
	}

	sd.graphMutex.Lock()
	defer sd.graphMutex.Unlock()

	// Clear existing graph
	sd.serviceGraph = models.NewServiceGraph()

	// Group pods by service
	servicePods := make(map[string][]*models.PodInfo)
	for _, pod := range pods {
		if pod.ServiceName != "" {
			servicePods[pod.ServiceName] = append(servicePods[pod.ServiceName], pod)
		}
	}

	// Create service nodes
	for serviceName, pods := range servicePods {
		serviceNode := sd.createServiceNodeFromPods(serviceName, pods)
		if serviceNode != nil {
			sd.serviceGraph.AddNode(serviceNode)
		}
	}

	// Determine gateway (frontend service)
	if _, exists := sd.serviceGraph.GetNode("frontend"); exists {
		sd.serviceGraph.SetGateway("frontend")
		log.Printf("Set gateway to: frontend")
	}

	// Add service dependencies based on HotelReservation benchmark
	sd.addHotelReservationDependencies()

	log.Printf("Discovered %d services: %v", len(sd.serviceGraph.GetAllNodes()), sd.getServiceNames())

	return nil
}

// createServiceNodeFromPods creates a service node from pod information
func (sd *ServiceDiscovery) createServiceNodeFromPods(serviceName string, pods []*models.PodInfo) *models.ServiceNode {
	if len(pods) == 0 {
		return nil
	}

	// Use first pod as reference for service properties
	referencePod := pods[0]

	// Count total replicas
	replicas := len(pods)

	// Calculate average resource usage
	var totalCPU, totalMemory float64
	for _, pod := range pods {
		// Parse resource requests (simplified)
		if pod.ResourceRequests.CPU != "" {
			// This is a simplified parsing - in production you'd want proper resource parsing
			totalCPU += 0.1 // Default assumption
		}
		if pod.ResourceRequests.Memory != "" {
			totalMemory += 128 // Default assumption in MiB
		}
	}

	// RPS will be gathered by monitoring system during runtime (as per LEAD paper)
	// Set initial RPS to 0 - it will be updated by monitoring system
	rps := 0.0

	// Create network topology based on node distribution
	networkTopology := sd.createNetworkTopology(pods)

	serviceNode := &models.ServiceNode{
		ID:              serviceName,
		Name:            serviceName,
		Replicas:        replicas,
		RPS:             rps,
		NetworkTopology: networkTopology,
		Labels: map[string]string{
			"service_type": referencePod.ServiceType,
			"namespace":    referencePod.Namespace,
		},
	}

	// Add pod-specific labels
	for key, value := range referencePod.Labels {
		serviceNode.Labels[key] = value
	}

	return serviceNode
}

// createNetworkTopology creates network topology based on pod distribution
func (sd *ServiceDiscovery) createNetworkTopology(pods []*models.PodInfo) *models.NetworkTopology {
	if len(pods) == 0 {
		return nil
	}

	// Get node information
	nodes, err := sd.k8sClient.GetNodes()
	if err != nil {
		log.Printf("Failed to get node information: %v", err)
		return &models.NetworkTopology{
			AvailabilityZone: "unknown",
			Bandwidth:        500,
			Hops:             1,
			GeoDistance:      100,
			Throughput:       425, // 85% of bandwidth
			Latency:          5.0, // Default latency
			PacketLoss:       0.1, // Default packet loss
		}
	}

	// Find the most common availability zone for this service
	zoneCount := make(map[string]int)
	for _, pod := range pods {
		for _, node := range nodes {
			if node.Name == pod.NodeName && node.AvailabilityZone != "" {
				zoneCount[node.AvailabilityZone]++
			}
		}
	}

	// Determine primary availability zone
	var primaryZone string
	maxCount := 0
	for zone, count := range zoneCount {
		if count > maxCount {
			maxCount = count
			primaryZone = zone
		}
	}

	if primaryZone == "" {
		primaryZone = "unknown"
	}

	// Estimate bandwidth based on node type (simplified)
	bandwidth := sd.estimateBandwidth(pods[0])

	// Estimate hops based on service type
	hops := sd.estimateHops(pods[0].ServiceType)

	// Estimate geo distance based on zone
	geoDistance := sd.estimateGeoDistance(primaryZone)

	return &models.NetworkTopology{
		AvailabilityZone: primaryZone,
		Bandwidth:        bandwidth,
		Hops:             hops,
		GeoDistance:      geoDistance,
		Throughput:       bandwidth * 0.85, // Estimate throughput as 85% of bandwidth
		Latency:          1.0,              // Default latency, will be updated by monitoring
		PacketLoss:       0.1,              // Default packet loss, will be updated by monitoring
	}
}

// estimateBandwidth estimates network bandwidth dynamically from node labels
func (sd *ServiceDiscovery) estimateBandwidth(pod *models.PodInfo) float64 {
	// Get node information to extract bandwidth from labels
	nodes, err := sd.k8sClient.GetNodes()
	if err != nil {
		log.Printf("Failed to get nodes for bandwidth estimation: %v", err)
		return 1000.0 // Default fallback
	}

	// Find the node where this pod is running
	for _, node := range nodes {
		if node.Name == pod.NodeName {
			// Try to extract bandwidth from node labels
			if bandwidthStr, exists := node.Labels["network.bandwidth.mbps"]; exists {
				if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
					return bandwidth
				}
			}

			// Try alternative label formats
			if bandwidthStr, exists := node.Labels["bandwidth"]; exists {
				if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
					return bandwidth
				}
			}

			// Try to extract from instance type if available
			if instanceType, exists := node.Labels["node.kubernetes.io/instance-type"]; exists {
				return sd.estimateBandwidthFromInstanceType(instanceType)
			}

			// Default based on service type if no labels available
			return sd.getDefaultBandwidthForServiceType(pod.ServiceType)
		}
	}

	return 1000.0 // Default fallback
}

// estimateHops estimates network hops dynamically from service characteristics
func (sd *ServiceDiscovery) estimateHops(serviceType string) int {
	// Dynamic hop estimation based on service type characteristics
	// Database services typically have more hops due to additional network layers
	switch serviceType {
	case "mongodb", "database", "postgresql", "mysql":
		return 2 // Database services typically have 2 hops
	case "memcached", "redis", "cache":
		return 2 // Cache services typically have 2 hops
	case "frontend", "gateway":
		return 1 // Frontend services typically have 1 hop
	default:
		return 1 // Default for microservices
	}
}

// estimateGeoDistance estimates geographic distance dynamically from zone labels
func (sd *ServiceDiscovery) estimateGeoDistance(zone string) float64 {
	// Try to extract distance from zone label if it contains distance information
	// Expected format: "region-zone-distance" or "region-zone" with distance in label
	if distanceStr, exists := sd.extractDistanceFromZone(zone); exists {
		if distance, err := strconv.ParseFloat(distanceStr, 64); err == nil {
			return distance
		}
	}

	// If no distance information in zone, use a default based on zone characteristics
	// This will be updated by real-time monitoring
	return 100.0 // Default distance, will be updated by network monitoring
}

// addHotelReservationDependencies adds the known dependencies for HotelReservation benchmark
func (sd *ServiceDiscovery) addHotelReservationDependencies() {
	// Frontend dependencies (based on the dependency graph)
	dependencies := map[string][]string{
		"frontend": {"search", "user", "recommendation", "reservation"},
		"search":   {"profile", "geo", "rate"},
		"user":     {"rate"},
	}

	for service, deps := range dependencies {
		if _, exists := sd.serviceGraph.GetNode(service); exists {
			for _, dep := range deps {
				if _, depExists := sd.serviceGraph.GetNode(dep); depExists {
					sd.serviceGraph.AddEdge(service, dep)
					log.Printf("Added dependency: %s -> %s", service, dep)
				}
			}
		}
	}
}

// estimateBandwidthFromInstanceType estimates bandwidth from instance type label
func (sd *ServiceDiscovery) estimateBandwidthFromInstanceType(instanceType string) float64 {
	// Try to extract bandwidth from instance type if it contains bandwidth info
	// Expected format: "server-1000mbps" or "high-bandwidth-server"
	if strings.Contains(strings.ToLower(instanceType), "high") || strings.Contains(strings.ToLower(instanceType), "1000") {
		return 1000.0
	}
	if strings.Contains(strings.ToLower(instanceType), "medium") || strings.Contains(strings.ToLower(instanceType), "500") {
		return 500.0
	}
	if strings.Contains(strings.ToLower(instanceType), "low") || strings.Contains(strings.ToLower(instanceType), "100") {
		return 100.0
	}

	// Default bandwidth for unknown instance types
	return 800.0
}

// getDefaultBandwidthForServiceType returns default bandwidth based on service type
func (sd *ServiceDiscovery) getDefaultBandwidthForServiceType(serviceType string) float64 {
	// Default bandwidth based on service type characteristics
	switch serviceType {
	case "frontend", "gateway":
		return 1000.0 // Frontend services need higher bandwidth
	case "microservice":
		return 800.0 // Standard microservice bandwidth
	case "mongodb", "database":
		return 600.0 // Database services
	case "memcached", "cache":
		return 500.0 // Cache services
	default:
		return 800.0 // Default bandwidth
	}
}

// extractDistanceFromZone tries to extract distance information from zone label
func (sd *ServiceDiscovery) extractDistanceFromZone(zone string) (string, bool) {
	// Try to extract distance from zone label
	// Expected formats: "region-zone-distance", "region-zone-100km", etc.
	parts := strings.Split(zone, "-")
	if len(parts) >= 3 {
		// Check if last part is a number (distance)
		lastPart := parts[len(parts)-1]
		if _, err := strconv.ParseFloat(lastPart, 64); err == nil {
			return lastPart, true
		}
		// Check if last part contains "km" or distance indicator
		if strings.Contains(strings.ToLower(lastPart), "km") {
			distanceStr := strings.TrimSuffix(strings.ToLower(lastPart), "km")
			if _, err := strconv.ParseFloat(distanceStr, 64); err == nil {
				return distanceStr, true
			}
		}
	}

	// Try to extract from zone label if it contains distance info
	if strings.Contains(strings.ToLower(zone), "distance") {
		// Look for distance pattern in the zone string
		// This is a simple implementation - could be enhanced with regex
		return "", false
	}

	return "", false
}

// watchPodEvents watches for pod events and updates the service graph
func (sd *ServiceDiscovery) watchPodEvents() {
	podEvents := sd.k8sClient.GetPodEvents()

	for {
		select {
		case <-sd.ctx.Done():
			return
		case event := <-podEvents:
			sd.handlePodEvent(event)
		}
	}
}

// handlePodEvent handles individual pod events
func (sd *ServiceDiscovery) handlePodEvent(event kubernetes.PodEvent) {
	log.Printf("Pod event: %s %s (%s)", event.Type, event.Pod.Name, event.Pod.ServiceType)

	// Trigger service discovery refresh
	go func() {
		if err := sd.discoverServices(); err != nil {
			log.Printf("Failed to refresh service discovery: %v", err)
		} else {
			// Notify callback if set
			if sd.updateCallback != nil {
				sd.updateCallback(sd.copyServiceGraph())
			}
		}
	}()
}

// periodicRefresh performs periodic service discovery refresh
func (sd *ServiceDiscovery) periodicRefresh() {
	ticker := time.NewTicker(5 * time.Minute) // Refresh every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-sd.ctx.Done():
			return
		case <-ticker.C:
			if err := sd.discoverServices(); err != nil {
				log.Printf("Failed to refresh service discovery: %v", err)
			} else {
				log.Printf("Periodic service discovery refresh completed")
			}
		}
	}
}

// copyServiceGraph creates a deep copy of the service graph
func (sd *ServiceDiscovery) copyServiceGraph() *models.ServiceGraph {
	// This is a simplified copy - in production you'd want a proper deep copy
	graph := models.NewServiceGraph()

	nodes := sd.serviceGraph.GetAllNodes()
	for _, node := range nodes {
		graph.AddNode(node)
	}

	// Copy edges (simplified - you'd need to implement edge copying in the graph)
	graph.SetGateway(sd.serviceGraph.Gateway)

	return graph
}

// getServiceNames returns a list of service names
func (sd *ServiceDiscovery) getServiceNames() []string {
	var names []string
	nodes := sd.serviceGraph.GetAllNodes()
	for name := range nodes {
		names = append(names, name)
	}
	return names
}
