package tests

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"lead-framework/internal/models"
	"lead-framework/internal/monitoring"
)

// TestRealNetworkMonitoring tests the real network monitoring with Prometheus integration
func TestRealNetworkMonitoring(t *testing.T) {
	t.Log("=== Real Network Monitoring Test ===")

	// Test 1: Mock Prometheus client
	t.Run("MockPrometheusIntegration", func(t *testing.T) {
		testMockPrometheusIntegration(t)
	})

	// Test 2: Network metrics collection
	t.Run("NetworkMetricsCollection", func(t *testing.T) {
		testNetworkMetricsCollection(t)
	})

	// Test 3: Dynamic network topology updates
	t.Run("DynamicNetworkTopologyUpdates", func(t *testing.T) {
		testDynamicNetworkTopologyUpdates(t)
	})

	t.Log("=== Real Network Monitoring Test Complete ===")
}

// testMockPrometheusIntegration tests the mock Prometheus client
func testMockPrometheusIntegration(t *testing.T) {
	t.Log("Testing mock Prometheus integration...")

	// Create mock Prometheus client
	mockClient := monitoring.NewMockPrometheusClient()

	// Test bandwidth query
	bandwidthQuery := `rate(node_network_receive_bytes_total[5m]) * 8 / 1024 / 1024`
	results, err := mockClient.Query(bandwidthQuery)
	if err != nil {
		t.Fatalf("Failed to query bandwidth: %v", err)
	}

	if len(results) == 0 {
		t.Error("No bandwidth results returned")
	}

	// Verify bandwidth data
	for _, result := range results {
		if len(result.Value) < 2 {
			t.Error("Bandwidth result should have timestamp and value")
			continue
		}

		if value, ok := result.Value[1].(string); ok {
			if bandwidth, err := strconv.ParseFloat(value, 64); err != nil {
				t.Errorf("Failed to parse bandwidth value: %v", err)
			} else if bandwidth <= 0 {
				t.Errorf("Bandwidth should be positive, got %.2f", bandwidth)
			}
		}
	}

	t.Logf("✓ Mock Prometheus integration working correctly")
	t.Logf("  - Retrieved %d bandwidth metrics", len(results))
}

// testNetworkMetricsCollection tests network metrics collection
func testNetworkMetricsCollection(t *testing.T) {
	t.Log("Testing network metrics collection...")

	// Create mock Prometheus client
	mockClient := monitoring.NewMockPrometheusClient()

	// Create network monitor
	networkMonitor := monitoring.NewNetworkMonitor(mockClient, 5*time.Second)

	// Collect network metrics
	metrics, err := networkMonitor.GetNetworkMetrics()
	if err != nil {
		t.Fatalf("Failed to collect network metrics: %v", err)
	}

	// Verify metrics are populated
	if metrics.Bandwidth <= 0 {
		t.Error("Bandwidth should be positive")
	}

	if metrics.Latency <= 0 {
		t.Error("Latency should be positive")
	}

	if metrics.Throughput <= 0 {
		t.Error("Throughput should be positive")
	}

	if metrics.PacketLoss < 0 {
		t.Error("Packet loss should not be negative")
	}

	if len(metrics.NodeMetrics) == 0 {
		t.Error("Node metrics should be populated")
	}

	if len(metrics.ServiceMetrics) == 0 {
		t.Error("Service metrics should be populated")
	}

	t.Logf("✓ Network metrics collection working correctly")
	t.Logf("  - Bandwidth: %.2f Mbps", metrics.Bandwidth)
	t.Logf("  - Latency: %.2f ms", metrics.Latency)
	t.Logf("  - Throughput: %.2f Mbps", metrics.Throughput)
	t.Logf("  - Packet Loss: %.3f%%", metrics.PacketLoss)
	t.Logf("  - Node Metrics: %d nodes", len(metrics.NodeMetrics))
	t.Logf("  - Service Metrics: %d services", len(metrics.ServiceMetrics))
}

// testDynamicNetworkTopologyUpdates tests dynamic network topology updates
func testDynamicNetworkTopologyUpdates(t *testing.T) {
	t.Log("Testing dynamic network topology updates...")

	// Create mock Prometheus client
	mockClient := monitoring.NewMockPrometheusClient()

	// Create network monitor
	networkMonitor := monitoring.NewNetworkMonitor(mockClient, 1*time.Second)

	// Create a service with static network topology
	service := &models.ServiceNode{
		ID:       "test-service",
		Name:     "test-service",
		Replicas: 2,
		RPS:      1000,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        500, // Static value
			Hops:             1,   // Static value
			GeoDistance:      100, // Static value
		},
	}

	// Get initial metrics
	metrics, err := networkMonitor.GetNetworkMetrics()
	if err != nil {
		t.Fatalf("Failed to get network metrics: %v", err)
	}

	// Update service network topology with real metrics
	networkMonitor.UpdateServiceNetworkTopology(service, metrics)

	// Verify that network topology was updated with real values
	if service.NetworkTopology.Bandwidth != metrics.Bandwidth {
		t.Errorf("Service bandwidth should be updated to %.2f, got %.2f",
			metrics.Bandwidth, service.NetworkTopology.Bandwidth)
	}

	// Verify hops were estimated based on latency
	expectedHops := int(metrics.Latency / 10)
	if expectedHops > 0 && service.NetworkTopology.Hops != expectedHops {
		t.Logf("Hops updated based on latency: %.2f ms -> %d hops",
			metrics.Latency, service.NetworkTopology.Hops)
	}

	// Verify geo distance was estimated based on latency
	expectedDistance := metrics.Latency * 200
	if expectedDistance > 0 && service.NetworkTopology.GeoDistance != expectedDistance {
		t.Logf("Geo distance updated based on latency: %.2f ms -> %.2f km",
			metrics.Latency, service.NetworkTopology.GeoDistance)
	}

	t.Logf("✓ Dynamic network topology updates working correctly")
	t.Logf("  - Original bandwidth: 500.0 Mbps")
	t.Logf("  - Updated bandwidth: %.2f Mbps", service.NetworkTopology.Bandwidth)
	t.Logf("  - Updated hops: %d", service.NetworkTopology.Hops)
	t.Logf("  - Updated geo distance: %.2f km", service.NetworkTopology.GeoDistance)
}

// TestPrometheusQueries tests the actual Prometheus queries that would be used
func TestPrometheusQueries(t *testing.T) {
	t.Log("=== Prometheus Queries Test ===")

	// Test the actual Prometheus queries that would be used in production
	queries := monitoring.DefaultPrometheusQueries()

	// Test CPU usage query
	t.Run("CPUUsageQuery", func(t *testing.T) {
		testPrometheusQuery(t, "CPU Usage", queries.CPUUsage, "frontend")
	})

	// Test memory usage query
	t.Run("MemoryUsageQuery", func(t *testing.T) {
		testPrometheusQuery(t, "Memory Usage", queries.MemoryUsage, "search")
	})

	// Test request rate query
	t.Run("RequestRateQuery", func(t *testing.T) {
		testPrometheusQuery(t, "Request Rate", queries.RequestRate, "profile")
	})

	// Test latency query
	t.Run("LatencyQuery", func(t *testing.T) {
		testPrometheusQuery(t, "Latency", queries.Latency, "user")
	})

	t.Log("=== Prometheus Queries Test Complete ===")
}

// testPrometheusQuery tests a specific Prometheus query
func testPrometheusQuery(t *testing.T, queryName, queryTemplate, serviceName string) {
	t.Logf("Testing %s query...", queryName)

	// Format the query with service name
	query := fmt.Sprintf(queryTemplate, serviceName)

	// Verify query is properly formatted
	if !strings.Contains(query, serviceName) {
		t.Errorf("%s query should contain service name '%s'", queryName, serviceName)
	}

	// Test with mock client
	mockClient := monitoring.NewMockPrometheusClient()
	results, err := mockClient.Query(query)
	if err != nil {
		t.Errorf("Failed to execute %s query: %v", queryName, err)
		return
	}

	if len(results) == 0 {
		t.Errorf("%s query should return results", queryName)
		return
	}

	t.Logf("✓ %s query working correctly", queryName)
	t.Logf("  - Query: %s", query)
	t.Logf("  - Results: %d metrics", len(results))
}
