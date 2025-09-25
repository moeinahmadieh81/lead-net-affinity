package monitoring

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"lead-framework/internal/kubernetes"
	"lead-framework/internal/models"
)

// InterNodeMetrics represents network metrics between two nodes
type InterNodeMetrics struct {
	Node1       string    `json:"node1"`
	Node2       string    `json:"node2"`
	Latency     float64   `json:"latency"`      // ms
	Bandwidth   float64   `json:"bandwidth"`    // Mbps
	Throughput  float64   `json:"throughput"`   // Mbps
	PacketLoss  float64   `json:"packet_loss"`  // percentage
	GeoDistance float64   `json:"geo_distance"` // km
	LastUpdated time.Time `json:"last_updated"`
}

// DynamicNetworkMonitor provides real-time network topology monitoring without static mappings
type DynamicNetworkMonitor struct {
	k8sClient        *kubernetes.KubernetesClient
	prometheusClient PrometheusClient
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc

	// Dynamic network topology cache
	nodeNetworkMap   map[string]*kubernetes.NodeNetworkInfo
	interNodeMetrics map[string]*InterNodeMetrics // Key: "node1-node2"
	serviceGraph     *models.ServiceGraph
	mu               sync.RWMutex
}

// NewDynamicNetworkMonitor creates a new dynamic network monitor
func NewDynamicNetworkMonitor(
	k8sClient *kubernetes.KubernetesClient,
	prometheusClient PrometheusClient,
	serviceGraph *models.ServiceGraph,
	interval time.Duration,
) *DynamicNetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &DynamicNetworkMonitor{
		k8sClient:        k8sClient,
		prometheusClient: prometheusClient,
		serviceGraph:     serviceGraph,
		interval:         interval,
		ctx:              ctx,
		cancel:           cancel,
		nodeNetworkMap:   make(map[string]*kubernetes.NodeNetworkInfo),
		interNodeMetrics: make(map[string]*InterNodeMetrics),
	}
}

// Start begins dynamic network topology monitoring
func (dnm *DynamicNetworkMonitor) Start() error {
	log.Println("Starting dynamic network topology monitoring...")

	go dnm.monitorNetworkTopology()
	go dnm.updateServiceNetworkTopology()

	return nil
}

// Stop stops the dynamic network monitor
func (dnm *DynamicNetworkMonitor) Stop() {
	dnm.cancel()
}

// monitorNetworkTopology continuously monitors network topology from node labels and real-time metrics
func (dnm *DynamicNetworkMonitor) monitorNetworkTopology() {
	ticker := time.NewTicker(dnm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-dnm.ctx.Done():
			return
		case <-ticker.C:
			dnm.updateNodeNetworkTopology()
			dnm.updateInterNodeMetrics()
		}
	}
}

// updateNodeNetworkTopology updates network topology from node labels and real-time measurements
func (dnm *DynamicNetworkMonitor) updateNodeNetworkTopology() {
	nodes, err := dnm.k8sClient.GetNodes()
	if err != nil {
		log.Printf("Failed to get nodes for network topology update: %v", err)
		return
	}

	dnm.mu.Lock()
	defer dnm.mu.Unlock()

	for _, node := range nodes {
		// Extract network information from node labels
		networkInfo := dnm.extractNetworkInfoFromLabels(node)

		// Update with real-time measurements if available
		if dnm.prometheusClient != nil {
			dnm.updateWithRealTimeMetrics(node.Name, networkInfo)
		}

		dnm.nodeNetworkMap[node.Name] = networkInfo
	}
}

// extractNetworkInfoFromLabels extracts network information from node labels
func (dnm *DynamicNetworkMonitor) extractNetworkInfoFromLabels(node *kubernetes.NodeInfo) *kubernetes.NodeNetworkInfo {
	networkInfo := &kubernetes.NodeNetworkInfo{
		LastUpdated: time.Now(),
	}

	// Extract bandwidth from labels
	if bandwidthStr, exists := node.Labels["network.bandwidth.mbps"]; exists {
		if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
			networkInfo.Bandwidth = bandwidth
		}
	} else if bandwidthStr, exists := node.Labels["bandwidth"]; exists {
		if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
			networkInfo.Bandwidth = bandwidth
		}
	} else {
		// Try to extract from instance type
		if instanceType, exists := node.Labels["node.kubernetes.io/instance-type"]; exists {
			networkInfo.Bandwidth = dnm.extractBandwidthFromInstanceType(instanceType)
		} else {
			networkInfo.Bandwidth = 1000.0 // Default
		}
	}

	// Extract latency from labels
	if latencyStr, exists := node.Labels["network.latency.ms"]; exists {
		if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
			networkInfo.Latency = latency
		}
	} else if latencyStr, exists := node.Labels["latency"]; exists {
		if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
			networkInfo.Latency = latency
		}
	} else {
		networkInfo.Latency = 5.0 // Default
	}

	// Extract throughput from labels
	if throughputStr, exists := node.Labels["network.throughput.mbps"]; exists {
		if throughput, err := strconv.ParseFloat(throughputStr, 64); err == nil {
			networkInfo.Throughput = throughput
		}
	} else {
		// Estimate throughput as 85% of bandwidth
		networkInfo.Throughput = networkInfo.Bandwidth * 0.85
	}

	// Extract packet loss from labels
	if packetLossStr, exists := node.Labels["network.packetloss.percent"]; exists {
		if packetLoss, err := strconv.ParseFloat(packetLossStr, 64); err == nil {
			networkInfo.PacketLoss = packetLoss
		}
	} else {
		networkInfo.PacketLoss = 0.1 // Default 0.1%
	}

	// Extract instance type and region
	if instanceType, exists := node.Labels["node.kubernetes.io/instance-type"]; exists {
		networkInfo.InstanceType = instanceType
	}
	if region, exists := node.Labels["topology.kubernetes.io/region"]; exists {
		networkInfo.Region = region
	}

	// Determine network interface type
	networkInfo.NetworkInterface = dnm.determineNetworkInterface(networkInfo.InstanceType)

	return networkInfo
}

// extractBandwidthFromInstanceType extracts bandwidth from instance type label
func (dnm *DynamicNetworkMonitor) extractBandwidthFromInstanceType(instanceType string) float64 {
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
	if strings.Contains(strings.ToLower(instanceType), "ultra") || strings.Contains(strings.ToLower(instanceType), "10000") {
		return 10000.0
	}

	// Default bandwidth for unknown instance types
	return 1000.0
}

// determineNetworkInterface determines network interface type from instance type
func (dnm *DynamicNetworkMonitor) determineNetworkInterface(instanceType string) string {
	if strings.Contains(strings.ToLower(instanceType), "ultra") || strings.Contains(strings.ToLower(instanceType), "10000") {
		return "10Gbps"
	}
	if strings.Contains(strings.ToLower(instanceType), "high") || strings.Contains(strings.ToLower(instanceType), "1000") {
		return "1Gbps"
	}
	if strings.Contains(strings.ToLower(instanceType), "medium") || strings.Contains(strings.ToLower(instanceType), "500") {
		return "500Mbps"
	}
	return "1Gbps" // Default
}

// updateWithRealTimeMetrics updates network info with real-time Prometheus metrics
func (dnm *DynamicNetworkMonitor) updateWithRealTimeMetrics(nodeName string, networkInfo *kubernetes.NodeNetworkInfo) {
	// Query real-time latency
	if latency, err := dnm.prometheusClient.Query(fmt.Sprintf("node_network_latency{node=\"%s\"}", nodeName)); err == nil && len(latency) > 0 {
		if latencyValue, ok := latency[0].Value[1].(string); ok {
			if parsedLatency, err := strconv.ParseFloat(latencyValue, 64); err == nil {
				networkInfo.Latency = parsedLatency
			}
		}
	}

	// Query real-time bandwidth utilization
	if bandwidth, err := dnm.prometheusClient.Query(fmt.Sprintf("node_network_bandwidth_utilization{node=\"%s\"}", nodeName)); err == nil && len(bandwidth) > 0 {
		if bandwidthValue, ok := bandwidth[0].Value[1].(string); ok {
			if parsedBandwidth, err := strconv.ParseFloat(bandwidthValue, 64); err == nil {
				// Update throughput based on actual utilization
				networkInfo.Throughput = networkInfo.Bandwidth * (parsedBandwidth / 100.0)
			}
		}
	}

	// Query real-time packet loss
	if packetLoss, err := dnm.prometheusClient.Query(fmt.Sprintf("node_network_packet_loss{node=\"%s\"}", nodeName)); err == nil && len(packetLoss) > 0 {
		if packetLossValue, ok := packetLoss[0].Value[1].(string); ok {
			if parsedPacketLoss, err := strconv.ParseFloat(packetLossValue, 64); err == nil {
				networkInfo.PacketLoss = parsedPacketLoss
			}
		}
	}
}

// updateInterNodeMetrics updates inter-node network metrics
func (dnm *DynamicNetworkMonitor) updateInterNodeMetrics() {
	dnm.mu.Lock()
	defer dnm.mu.Unlock()

	// Get all nodes
	nodes, err := dnm.k8sClient.GetNodes()
	if err != nil {
		log.Printf("Failed to get nodes for inter-node metrics: %v", err)
		return
	}

	// Calculate inter-node metrics for all pairs
	for i, node1 := range nodes {
		for j, node2 := range nodes {
			if i >= j {
				continue // Skip same node and avoid duplicates
			}

			key := fmt.Sprintf("%s-%s", node1.Name, node2.Name)

			// Query real-time inter-node latency
			latency := 5.0 // Default
			if dnm.prometheusClient != nil {
				if latencyQuery, err := dnm.prometheusClient.Query(fmt.Sprintf("inter_node_latency{node1=\"%s\",node2=\"%s\"}", node1.Name, node2.Name)); err == nil && len(latencyQuery) > 0 {
					if latencyValue, ok := latencyQuery[0].Value[1].(string); ok {
						if parsedLatency, err := strconv.ParseFloat(latencyValue, 64); err == nil {
							latency = parsedLatency
						}
					}
				}
			}

			// Calculate geographic distance from zone labels
			geoDistance := dnm.calculateGeoDistanceFromZones(node1.AvailabilityZone, node2.AvailabilityZone)

			dnm.interNodeMetrics[key] = &InterNodeMetrics{
				Node1:       node1.Name,
				Node2:       node2.Name,
				Latency:     latency,
				Bandwidth:   math.Min(dnm.nodeNetworkMap[node1.Name].Bandwidth, dnm.nodeNetworkMap[node2.Name].Bandwidth),
				Throughput:  math.Min(dnm.nodeNetworkMap[node1.Name].Throughput, dnm.nodeNetworkMap[node2.Name].Throughput),
				PacketLoss:  math.Max(dnm.nodeNetworkMap[node1.Name].PacketLoss, dnm.nodeNetworkMap[node2.Name].PacketLoss),
				GeoDistance: geoDistance,
				LastUpdated: time.Now(),
			}
		}
	}
}

// calculateGeoDistanceFromZones calculates geographic distance from zone labels
func (dnm *DynamicNetworkMonitor) calculateGeoDistanceFromZones(zone1, zone2 string) float64 {
	// If same zone, distance is 0
	if zone1 == zone2 {
		return 0.0
	}

	// Try to extract distance from zone labels
	// Expected format: "region-zone-distance" or "region-zone-100km"
	distance1 := dnm.extractDistanceFromZone(zone1)
	distance2 := dnm.extractDistanceFromZone(zone2)

	if distance1 > 0 && distance2 > 0 {
		// Calculate distance between two points
		return math.Abs(distance1 - distance2)
	}

	// If no distance info in labels, use a default based on zone characteristics
	// This will be updated by real-time monitoring
	return 100.0 // Default distance
}

// extractDistanceFromZone tries to extract distance information from zone label
func (dnm *DynamicNetworkMonitor) extractDistanceFromZone(zone string) float64 {
	// Try to extract distance from zone label
	// Expected formats: "region-zone-distance", "region-zone-100km", etc.
	parts := strings.Split(zone, "-")
	if len(parts) >= 3 {
		// Check if last part is a number (distance)
		lastPart := parts[len(parts)-1]
		if distance, err := strconv.ParseFloat(lastPart, 64); err == nil {
			return distance
		}
		// Check if last part contains "km" or distance indicator
		if strings.Contains(strings.ToLower(lastPart), "km") {
			distanceStr := strings.TrimSuffix(strings.ToLower(lastPart), "km")
			if distance, err := strconv.ParseFloat(distanceStr, 64); err == nil {
				return distance
			}
		}
	}

	return 0.0 // No distance information found
}

// updateServiceNetworkTopology updates service network topology based on current node information
func (dnm *DynamicNetworkMonitor) updateServiceNetworkTopology() {
	ticker := time.NewTicker(dnm.interval * 2) // Update less frequently
	defer ticker.Stop()

	for {
		select {
		case <-dnm.ctx.Done():
			return
		case <-ticker.C:
			dnm.updateServiceGraphNetworkTopology()
		}
	}
}

// updateServiceGraphNetworkTopology updates the service graph with current network topology
func (dnm *DynamicNetworkMonitor) updateServiceGraphNetworkTopology() {
	dnm.mu.RLock()
	defer dnm.mu.RUnlock()

	// Update each service's network topology based on current node information
	allNodes := dnm.serviceGraph.GetAllNodes()
	for _, serviceNode := range allNodes {
		// Find the node where this service is running
		// This is a simplified approach - in practice, you'd track which pods are on which nodes
		for nodeName, nodeNetworkInfo := range dnm.nodeNetworkMap {
			// Update service network topology with current node information
			if serviceNode.NetworkTopology != nil {
				serviceNode.NetworkTopology.Bandwidth = nodeNetworkInfo.Bandwidth
				serviceNode.NetworkTopology.Latency = nodeNetworkInfo.Latency
				serviceNode.NetworkTopology.Throughput = nodeNetworkInfo.Throughput
				serviceNode.NetworkTopology.PacketLoss = nodeNetworkInfo.PacketLoss
				serviceNode.NetworkTopology.AvailabilityZone = nodeName // Simplified mapping
			}
		}
	}
}

// GetNodeNetworkInfo returns network information for a specific node
func (dnm *DynamicNetworkMonitor) GetNodeNetworkInfo(nodeName string) (*kubernetes.NodeNetworkInfo, bool) {
	dnm.mu.RLock()
	defer dnm.mu.RUnlock()

	info, exists := dnm.nodeNetworkMap[nodeName]
	return info, exists
}

// GetInterNodeMetrics returns network metrics between two nodes
func (dnm *DynamicNetworkMonitor) GetInterNodeMetrics(node1, node2 string) (*InterNodeMetrics, bool) {
	dnm.mu.RLock()
	defer dnm.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", node1, node2)
	metrics, exists := dnm.interNodeMetrics[key]
	return metrics, exists
}

// GetAllNodeNetworkInfo returns network information for all nodes
func (dnm *DynamicNetworkMonitor) GetAllNodeNetworkInfo() map[string]*kubernetes.NodeNetworkInfo {
	dnm.mu.RLock()
	defer dnm.mu.RUnlock()

	result := make(map[string]*kubernetes.NodeNetworkInfo)
	for nodeName, info := range dnm.nodeNetworkMap {
		result[nodeName] = info
	}
	return result
}
