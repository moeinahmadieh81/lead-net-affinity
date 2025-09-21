package discovery

import (
	"context"
	"fmt"
	"log"
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

	// Estimate RPS based on service type and resources
	rps := sd.estimateRPS(serviceName, replicas, totalCPU)

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
	}
}

// estimateRPS estimates requests per second based on service characteristics
func (sd *ServiceDiscovery) estimateRPS(serviceName string, replicas int, cpu float64) float64 {
	// Base RPS estimates for HotelReservation services
	baseRPS := map[string]float64{
		"frontend":       1000,
		"search":         800,
		"user":           600,
		"recommendation": 400,
		"reservation":    700,
		"profile":        500,
		"rate":           300,
		"geo":            200,
	}

	rps, exists := baseRPS[serviceName]
	if !exists {
		rps = 100 // Default for unknown services
	}

	// Scale by replicas and CPU
	rps = rps * float64(replicas) * (1 + cpu)

	return rps
}

// estimateBandwidth estimates network bandwidth
func (sd *ServiceDiscovery) estimateBandwidth(pod *models.PodInfo) float64 {
	// Simplified bandwidth estimation based on service type
	bandwidthMap := map[string]float64{
		"microservice": 800,
		"mongodb":      600,
		"memcached":    500,
	}

	bandwidth, exists := bandwidthMap[pod.ServiceType]
	if !exists {
		bandwidth = 500 // Default
	}

	return bandwidth
}

// estimateHops estimates network hops
func (sd *ServiceDiscovery) estimateHops(serviceType string) int {
	// Simplified hop estimation
	hopMap := map[string]int{
		"microservice": 1,
		"mongodb":      2,
		"memcached":    2,
	}

	hops, exists := hopMap[serviceType]
	if !exists {
		hops = 1 // Default
	}

	return hops
}

// estimateGeoDistance estimates geographic distance
func (sd *ServiceDiscovery) estimateGeoDistance(zone string) float64 {
	// Simplified geo distance estimation
	distanceMap := map[string]float64{
		"us-west-1a": 0,
		"us-west-1b": 100,
		"us-west-1c": 200,
		"us-east-1a": 3000,
		"us-east-1b": 3100,
	}

	distance, exists := distanceMap[zone]
	if !exists {
		distance = 100 // Default
	}

	return distance
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
