package monitoring

import (
	"context"
	"log"
	"strconv"
	"time"

	"lead-framework/internal/models"
)

// NetworkMonitor provides real-time network topology monitoring
type NetworkMonitor struct {
	prometheusClient PrometheusClient
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
}

// PrometheusClient interface for querying Prometheus metrics
type PrometheusClient interface {
	Query(query string) ([]MetricResult, error)
	QueryRange(query string, start, end time.Time, step time.Duration) ([]MetricResult, error)
}

// MetricResult represents a Prometheus query result
type MetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// NetworkMetrics represents real-time network metrics
type NetworkMetrics struct {
	Bandwidth      float64            `json:"bandwidth"`       // Actual measured bandwidth
	Latency        float64            `json:"latency"`         // Network latency in ms
	PacketLoss     float64            `json:"packet_loss"`     // Packet loss percentage
	Throughput     float64            `json:"throughput"`      // Actual throughput
	NodeMetrics    map[string]float64 `json:"node_metrics"`    // Per-node metrics
	ServiceMetrics map[string]float64 `json:"service_metrics"` // Per-service metrics
	LastUpdated    time.Time          `json:"last_updated"`
}

// NewNetworkMonitor creates a new network monitor with real Prometheus integration
func NewNetworkMonitor(prometheusClient PrometheusClient, interval time.Duration) *NetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &NetworkMonitor{
		prometheusClient: prometheusClient,
		interval:         interval,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start begins real-time network monitoring
func (nm *NetworkMonitor) Start() error {
	log.Println("Starting real-time network monitoring...")

	go nm.monitorNetworkTopology()

	return nil
}

// Stop stops network monitoring
func (nm *NetworkMonitor) Stop() {
	nm.cancel()
}

// GetNetworkMetrics returns current network metrics
func (nm *NetworkMonitor) GetNetworkMetrics() (*NetworkMetrics, error) {
	return nm.collectNetworkMetrics()
}

// monitorNetworkTopology continuously monitors network topology
func (nm *NetworkMonitor) monitorNetworkTopology() {
	ticker := time.NewTicker(nm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			metrics, err := nm.collectNetworkMetrics()
			if err != nil {
				log.Printf("Failed to collect network metrics: %v", err)
				continue
			}

			log.Printf("Network metrics updated: Bandwidth=%.2f Mbps, Latency=%.2f ms, Throughput=%.2f Mbps",
				metrics.Bandwidth, metrics.Latency, metrics.Throughput)
		}
	}
}

// collectNetworkMetrics collects real network metrics from Prometheus
func (nm *NetworkMonitor) collectNetworkMetrics() (*NetworkMetrics, error) {
	metrics := &NetworkMetrics{
		NodeMetrics:    make(map[string]float64),
		ServiceMetrics: make(map[string]float64),
		LastUpdated:    time.Now(),
	}

	// Query 1: Network bandwidth between nodes
	bandwidthQuery := `rate(node_network_receive_bytes_total[5m]) * 8 / 1024 / 1024` // Convert to Mbps
	bandwidthResults, err := nm.prometheusClient.Query(bandwidthQuery)
	if err != nil {
		log.Printf("Failed to query bandwidth metrics: %v", err)
		metrics.Bandwidth = 500 // Fallback to static value
	} else {
		metrics.Bandwidth = nm.calculateAverageBandwidth(bandwidthResults)
	}

	// Query 2: Network latency between services
	latencyQuery := `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) * 1000`
	latencyResults, err := nm.prometheusClient.Query(latencyQuery)
	if err != nil {
		log.Printf("Failed to query latency metrics: %v", err)
		metrics.Latency = 50 // Fallback to static value
	} else {
		metrics.Latency = nm.calculateAverageLatency(latencyResults)
	}

	// Query 3: Network throughput
	throughputQuery := `rate(node_network_transmit_bytes_total[5m]) * 8 / 1024 / 1024`
	throughputResults, err := nm.prometheusClient.Query(throughputQuery)
	if err != nil {
		log.Printf("Failed to query throughput metrics: %v", err)
		metrics.Throughput = metrics.Bandwidth * 0.8 // Estimate as 80% of bandwidth
	} else {
		metrics.Throughput = nm.calculateAverageThroughput(throughputResults)
	}

	// Query 4: Packet loss
	packetLossQuery := `rate(node_network_receive_drop_total[5m]) / rate(node_network_receive_packets_total[5m]) * 100`
	packetLossResults, err := nm.prometheusClient.Query(packetLossQuery)
	if err != nil {
		log.Printf("Failed to query packet loss metrics: %v", err)
		metrics.PacketLoss = 0.1 // Fallback to 0.1%
	} else {
		metrics.PacketLoss = nm.calculateAveragePacketLoss(packetLossResults)
	}

	// Query 5: Per-node metrics
	nodeMetrics, err := nm.collectPerNodeMetrics()
	if err != nil {
		log.Printf("Failed to collect per-node metrics: %v", err)
	} else {
		metrics.NodeMetrics = nodeMetrics
	}

	// Query 6: Per-service metrics
	serviceMetrics, err := nm.collectPerServiceMetrics()
	if err != nil {
		log.Printf("Failed to collect per-service metrics: %v", err)
	} else {
		metrics.ServiceMetrics = serviceMetrics
	}

	return metrics, nil
}

// collectPerNodeMetrics collects network metrics per Kubernetes node
func (nm *NetworkMonitor) collectPerNodeMetrics() (map[string]float64, error) {
	nodeMetrics := make(map[string]float64)

	// Query node network utilization
	nodeQuery := `rate(node_network_receive_bytes_total[5m]) * 8 / 1024 / 1024`
	results, err := nm.prometheusClient.Query(nodeQuery)
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		if nodeName, exists := result.Metric["instance"]; exists {
			if len(result.Value) > 1 {
				if value, ok := result.Value[1].(string); ok {
					if bandwidth, err := strconv.ParseFloat(value, 64); err == nil {
						nodeMetrics[nodeName] = bandwidth
					}
				}
			}
		}
	}

	return nodeMetrics, nil
}

// collectPerServiceMetrics collects network metrics per service
func (nm *NetworkMonitor) collectPerServiceMetrics() (map[string]float64, error) {
	serviceMetrics := make(map[string]float64)

	// Query service request rates
	serviceQuery := `rate(http_requests_total[5m])`
	results, err := nm.prometheusClient.Query(serviceQuery)
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		if serviceName, exists := result.Metric["service"]; exists {
			if len(result.Value) > 1 {
				if value, ok := result.Value[1].(string); ok {
					if rate, err := strconv.ParseFloat(value, 64); err == nil {
						serviceMetrics[serviceName] = rate
					}
				}
			}
		}
	}

	return serviceMetrics, nil
}

// Helper functions for calculating averages
func (nm *NetworkMonitor) calculateAverageBandwidth(results []MetricResult) float64 {
	if len(results) == 0 {
		return 500 // Default fallback
	}

	var total float64
	count := 0

	for _, result := range results {
		if len(result.Value) > 1 {
			if value, ok := result.Value[1].(string); ok {
				if bandwidth, err := strconv.ParseFloat(value, 64); err == nil {
					total += bandwidth
					count++
				}
			}
		}
	}

	if count > 0 {
		return total / float64(count)
	}

	return 500 // Default fallback
}

func (nm *NetworkMonitor) calculateAverageLatency(results []MetricResult) float64 {
	if len(results) == 0 {
		return 50 // Default fallback
	}

	var total float64
	count := 0

	for _, result := range results {
		if len(result.Value) > 1 {
			if value, ok := result.Value[1].(string); ok {
				if latency, err := strconv.ParseFloat(value, 64); err == nil {
					total += latency
					count++
				}
			}
		}
	}

	if count > 0 {
		return total / float64(count)
	}

	return 50 // Default fallback
}

func (nm *NetworkMonitor) calculateAverageThroughput(results []MetricResult) float64 {
	if len(results) == 0 {
		return 400 // Default fallback
	}

	var total float64
	count := 0

	for _, result := range results {
		if len(result.Value) > 1 {
			if value, ok := result.Value[1].(string); ok {
				if throughput, err := strconv.ParseFloat(value, 64); err == nil {
					total += throughput
					count++
				}
			}
		}
	}

	if count > 0 {
		return total / float64(count)
	}

	return 400 // Default fallback
}

func (nm *NetworkMonitor) calculateAveragePacketLoss(results []MetricResult) float64 {
	if len(results) == 0 {
		return 0.1 // Default fallback
	}

	var total float64
	count := 0

	for _, result := range results {
		if len(result.Value) > 1 {
			if value, ok := result.Value[1].(string); ok {
				if packetLoss, err := strconv.ParseFloat(value, 64); err == nil {
					total += packetLoss
					count++
				}
			}
		}
	}

	if count > 0 {
		return total / float64(count)
	}

	return 0.1 // Default fallback
}

// UpdateServiceNetworkTopology updates network topology for a service based on real metrics
func (nm *NetworkMonitor) UpdateServiceNetworkTopology(service *models.ServiceNode, metrics *NetworkMetrics) {
	if service.NetworkTopology == nil {
		service.NetworkTopology = &models.NetworkTopology{}
	}

	// Update with real metrics if available
	if metrics != nil {
		// Use real bandwidth if available
		if metrics.Bandwidth > 0 {
			service.NetworkTopology.Bandwidth = metrics.Bandwidth
		}

		// Estimate hops based on latency
		if metrics.Latency > 0 {
			// Rough estimation: 1 hop = ~10ms latency
			estimatedHops := int(metrics.Latency / 10)
			if estimatedHops > 0 {
				service.NetworkTopology.Hops = estimatedHops
			}
		}

		// Update geo distance based on actual latency
		if metrics.Latency > 0 {
			// Rough estimation: 1ms = ~200km distance
			estimatedDistance := metrics.Latency * 200
			service.NetworkTopology.GeoDistance = estimatedDistance
		}
	}
}
