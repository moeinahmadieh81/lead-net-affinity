package discovery

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"lead-framework/internal/kubernetes"
	"lead-framework/internal/models"
	"lead-framework/internal/monitoring"
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
	} else if _, exists := sd.serviceGraph.GetNode("fe"); exists {
		sd.serviceGraph.SetGateway("fe")
		log.Printf("Set gateway to: fe")
	}

	// Add service dependencies based on static dependency graph
	// Note: Consul is used for service registration but is not part of the dependency graph
	// and its placement does not affect affinity rules
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

// createNetworkTopology creates network topology using Prometheus queries only
func (sd *ServiceDiscovery) createNetworkTopology(pods []*models.PodInfo) *models.NetworkTopology {
	if len(pods) == 0 {
		return nil
	}

	// Get network topology from Prometheus for the service
	serviceName := pods[0].ServiceName
	networkTopology, err := sd.getServiceNetworkTopologyFromPrometheus(serviceName)
	if err != nil {
		log.Printf("Failed to get network topology from Prometheus for service %s: %v", serviceName, err)
		return nil
	}

	return networkTopology
}

// getServiceNetworkTopologyFromPrometheus gets network topology for a service from Prometheus only
func (sd *ServiceDiscovery) getServiceNetworkTopologyFromPrometheus(serviceName string) (*models.NetworkTopology, error) {
	// Create a temporary Prometheus client for this operation
	// In a real implementation, this would be injected as a dependency
	prometheusClient := monitoring.NewRealPrometheusClient("http://prometheus.monitoring.svc.cluster.local:9090")

	// Get service latency - must have real data
	latency, err := sd.getServiceLatencyFromPrometheus(prometheusClient, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service latency: %v", err)
	}

	// Get service bandwidth - must have real data
	bandwidth, err := sd.getServiceBandwidthFromPrometheus(prometheusClient, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service bandwidth: %v", err)
	}

	// Get service throughput - must have real data
	throughput, err := sd.getServiceThroughputFromPrometheus(prometheusClient, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service throughput: %v", err)
	}

	// Get service packet loss - must have real data
	packetLoss, err := sd.getServicePacketLossFromPrometheus(prometheusClient, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service packet loss: %v", err)
	}

	// Get availability zone - must have real data
	availabilityZone, err := sd.getServiceAvailabilityZoneFromPrometheus(prometheusClient, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability zone: %v", err)
	}

	// Calculate hops (simplified)
	hops := sd.calculateServiceHops(serviceName)

	// Calculate geo distance (simplified)
	geoDistance := sd.calculateServiceGeoDistance(availabilityZone)

	return &models.NetworkTopology{
		AvailabilityZone: availabilityZone,
		Bandwidth:        bandwidth,
		Hops:             hops,
		GeoDistance:      geoDistance,
		Throughput:       throughput,
		Latency:          latency,
		PacketLoss:       packetLoss,
	}, nil
}

// getServiceLatencyFromPrometheus gets latency for a service from Prometheus
func (sd *ServiceDiscovery) getServiceLatencyFromPrometheus(client monitoring.PrometheusClient, serviceName string) (float64, error) {
	query := fmt.Sprintf(`histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{service="%s"}[5m])) * 1000`, serviceName)
	results, err := client.Query(query)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no latency data for service %s", serviceName)
	}

	value, err := sd.parseMetricValue(results[0].Value[1])
	if err != nil {
		return 0, err
	}

	return value, nil
}

// getServiceBandwidthFromPrometheus gets bandwidth for a service from Prometheus
func (sd *ServiceDiscovery) getServiceBandwidthFromPrometheus(client monitoring.PrometheusClient, serviceName string) (float64, error) {
	query := fmt.Sprintf(`rate(http_request_bytes_total{service="%s"}[5m])`, serviceName)
	results, err := client.Query(query)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no bandwidth data for service %s", serviceName)
	}

	value, err := sd.parseMetricValue(results[0].Value[1])
	if err != nil {
		return 0, err
	}

	// Convert bytes to Mbps
	return value / (1024 * 1024), nil
}

// getServiceThroughputFromPrometheus gets throughput for a service from Prometheus
func (sd *ServiceDiscovery) getServiceThroughputFromPrometheus(client monitoring.PrometheusClient, serviceName string) (float64, error) {
	query := fmt.Sprintf(`rate(http_requests_total{service="%s"}[5m])`, serviceName)
	results, err := client.Query(query)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no throughput data for service %s", serviceName)
	}

	value, err := sd.parseMetricValue(results[0].Value[1])
	if err != nil {
		return 0, err
	}

	// Convert requests per second to Mbps (simplified)
	return value * 0.001, nil
}

// getServicePacketLossFromPrometheus gets packet loss for a service from Prometheus
func (sd *ServiceDiscovery) getServicePacketLossFromPrometheus(client monitoring.PrometheusClient, serviceName string) (float64, error) {
	query := fmt.Sprintf(`rate(http_requests_total{service="%s",status=~"5.."}[5m]) / rate(http_requests_total{service="%s"}[5m]) * 100`, serviceName, serviceName)
	results, err := client.Query(query)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no packet loss data for service %s", serviceName)
	}

	return sd.parseMetricValue(results[0].Value[1])
}

// getServiceAvailabilityZoneFromPrometheus gets availability zone for a service from Prometheus
func (sd *ServiceDiscovery) getServiceAvailabilityZoneFromPrometheus(client monitoring.PrometheusClient, serviceName string) (string, error) {
	// Get the node where the service is running
	query := fmt.Sprintf(`kube_pod_info{pod=~"%s-.*"}`, serviceName)
	results, err := client.Query(query)
	if err != nil {
		return "unknown", err
	}

	if len(results) == 0 {
		return "unknown", fmt.Errorf("no availability zone data for service %s", serviceName)
	}

	// Extract availability zone from node labels
	if zone, exists := results[0].Metric["zone"]; exists {
		return zone, nil
	}

	return "unknown", nil
}

// calculateServiceHops calculates network hops for a service from Prometheus only
func (sd *ServiceDiscovery) calculateServiceHops(serviceName string) int {
	// In a real implementation, this would query Prometheus for actual hop data
	// For now, return 0 to indicate no hop data available
	// This should be replaced with actual Prometheus queries when hop metrics are available
	return 0 // No hop data available
}

// calculateServiceGeoDistance calculates geographic distance for a service
func (sd *ServiceDiscovery) calculateServiceGeoDistance(availabilityZone string) float64 {
	// In production, we don't know which region is the "primary" region
	// Geographic distance calculation requires a reference point which we don't have
	// Return 0 to indicate no distance information is available
	return 0.0 // No distance information available without knowing the reference point
}

// parseMetricValue parses a metric value from Prometheus response
func (sd *ServiceDiscovery) parseMetricValue(value interface{}) (float64, error) {
	valueStr, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("invalid value type")
	}

	return strconv.ParseFloat(valueStr, 64)
}

// extractBandwidthFromNode extracts bandwidth from node labels (dynamic data only)
func (sd *ServiceDiscovery) extractBandwidthFromNode(pod *models.PodInfo, nodes []*kubernetes.NodeInfo) float64 {
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
		}
	}

	return 0 // No dynamic data available
}

// extractHopsFromNode extracts network hops from node labels (dynamic data only)
func (sd *ServiceDiscovery) extractHopsFromNode(pod *models.PodInfo, nodes []*kubernetes.NodeInfo) int {
	// Find the node where this pod is running
	for _, node := range nodes {
		if node.Name == pod.NodeName {
			// Try to extract hops from node labels
			if hopsStr, exists := node.Labels["network.hops"]; exists {
				if hops, err := strconv.Atoi(hopsStr); err == nil {
					return hops
				}
			}

			// Try alternative label formats
			if hopsStr, exists := node.Labels["hops"]; exists {
				if hops, err := strconv.Atoi(hopsStr); err == nil {
					return hops
				}
			}
		}
	}

	return 0 // No dynamic data available
}

// extractGeoDistanceFromZone extracts geographic distance from zone labels (dynamic data only)
func (sd *ServiceDiscovery) extractGeoDistanceFromZone(zone string) float64 {
	// In production, we don't know which region is the "primary" region
	// Geographic distance calculation requires a reference point which we don't have
	return -1 // No dynamic data available
}

// extractThroughputFromNode extracts throughput from node labels (dynamic data only)
func (sd *ServiceDiscovery) extractThroughputFromNode(pod *models.PodInfo, nodes []*kubernetes.NodeInfo) float64 {
	// Find the node where this pod is running
	for _, node := range nodes {
		if node.Name == pod.NodeName {
			// Try to extract throughput from node labels
			if throughputStr, exists := node.Labels["network.throughput.mbps"]; exists {
				if throughput, err := strconv.ParseFloat(throughputStr, 64); err == nil {
					return throughput
				}
			}

			// Try alternative label formats
			if throughputStr, exists := node.Labels["throughput"]; exists {
				if throughput, err := strconv.ParseFloat(throughputStr, 64); err == nil {
					return throughput
				}
			}
		}
	}

	return 0 // No dynamic data available
}

// extractLatencyFromNode extracts latency from node labels (dynamic data only)
func (sd *ServiceDiscovery) extractLatencyFromNode(pod *models.PodInfo, nodes []*kubernetes.NodeInfo) float64 {
	// Find the node where this pod is running
	for _, node := range nodes {
		if node.Name == pod.NodeName {
			// Try to extract latency from node labels
			if latencyStr, exists := node.Labels["network.latency.ms"]; exists {
				if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
					return latency
				}
			}

			// Try alternative label formats
			if latencyStr, exists := node.Labels["latency"]; exists {
				if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
					return latency
				}
			}
		}
	}

	return 0 // No dynamic data available
}

// extractPacketLossFromNode extracts packet loss from node labels (dynamic data only)
func (sd *ServiceDiscovery) extractPacketLossFromNode(pod *models.PodInfo, nodes []*kubernetes.NodeInfo) float64 {
	// Find the node where this pod is running
	for _, node := range nodes {
		if node.Name == pod.NodeName {
			// Try to extract packet loss from node labels
			if packetLossStr, exists := node.Labels["network.packetloss.percent"]; exists {
				if packetLoss, err := strconv.ParseFloat(packetLossStr, 64); err == nil {
					return packetLoss
				}
			}

			// Try alternative label formats
			if packetLossStr, exists := node.Labels["packetloss"]; exists {
				if packetLoss, err := strconv.ParseFloat(packetLossStr, 64); err == nil {
					return packetLoss
				}
			}
		}
	}

	return -1 // No dynamic data available
}

// addHotelReservationDependencies adds the complete static dependency graph
// This matches the static architecture diagram:
// - Frontend (fe) -> search (src), user (usr), recommendation (rcm), reservation (rsv)
// - Search (src) -> profile (prf), geo, rate (rte)
// - Profile (prf) -> prf-mc (memcached), prf-db (mongodb)
// - Geo -> geo-db (mongodb)
// - User (usr) -> usr-db (mongodb)
// - Rate (rte) -> rte-mc (memcached), rte-db (mongodb)
// - Recommendation (rcm) -> rcm-db (mongodb)
// - Reservation (rsv) -> rsv-mc (memcached), rsv-db (mongodb)
func (sd *ServiceDiscovery) addHotelReservationDependencies() {
	// Static service dependency graph matching the architecture diagram
	dependencies := map[string][]string{
		// Frontend (Gateway) - depends on all business services
		"frontend": {"search", "user", "recommendation", "reservation"},
		"fe":       {"search", "user", "recommendation", "reservation"}, // Alias

		// Search service - depends on profile, geo, and rate
		"search": {"profile", "geo", "rate"},
		"src":    {"profile", "geo", "rate"}, // Alias

		// Profile service - depends on its cache and database
		"profile": {"memcached-profile", "mongodb-profile"},
		"prf":     {"memcached-profile", "mongodb-profile"}, // Alias

		// Geo service - depends on its database
		"geo": {"mongodb-geo"},

		// User service - depends on its database
		"user": {"mongodb-user"},
		"usr":  {"mongodb-user"}, // Alias

		// Rate service - depends on its cache and database
		"rate": {"memcached-rate", "mongodb-rate"},
		"rte":  {"memcached-rate", "mongodb-rate"}, // Alias

		// Recommendation service - depends on its database
		"recommendation": {"mongodb-recommendation"},
		"rcm":            {"mongodb-recommendation"}, // Alias

		// Reservation service - depends on its cache and database
		"reservation": {"memcached-reservation", "mongodb-reservation"},
		"rsv":         {"memcached-reservation", "mongodb-reservation"}, // Alias

		// Database and Cache Services (leaf nodes - no dependencies)
		"mongodb-profile":        {},
		"memcached-profile":      {},
		"mongodb-rate":           {},
		"memcached-rate":         {},
		"mongodb-user":           {},
		"mongodb-geo":            {},
		"mongodb-recommendation": {},
		"mongodb-reservation":    {},
		"memcached-reservation":  {},
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
