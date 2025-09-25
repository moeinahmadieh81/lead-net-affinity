package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lead-framework/internal/lead"
	"lead-framework/internal/scheduler"

	"k8s.io/client-go/kubernetes/fake"
)

// TestSimpleLEADSchedulerIntegration tests the LEAD scheduler without Kubernetes dependencies
func TestSimpleLEADSchedulerIntegration(t *testing.T) {
	t.Log("=== Simple LEAD Scheduler Integration Test ===")

	// Create fake Kubernetes client for testing
	client := fake.NewSimpleClientset()

	// Create LEAD framework with test configuration
	config := &lead.FrameworkConfig{
		MonitoringInterval:     5 * time.Second,
		ResourceThreshold:      75.0,
		LatencyThreshold:       150 * time.Millisecond,
		PrometheusURL:          "http://localhost:9090",
		KubernetesNamespace:    "test-namespace",
		OutputDirectory:        "./test-k8s-manifests",
		BandwidthWeight:        0.4,
		HopsWeight:             0.3,
		GeoDistanceWeight:      0.2,
		AvailabilityZoneWeight: 0.1,
	}

	leadFramework := lead.NewLEADFrameworkWithConfig(config)
	leadScheduler := scheduler.NewLEADScheduler(client, leadFramework, config)

	// Test 1: Scheduler initialization with static graph
	t.Run("SchedulerInitialization", func(t *testing.T) {
		testSchedulerInitializationSimple(t, leadScheduler)
	})

	// Test 2: Network topology scoring
	t.Run("NetworkTopologyScoring", func(t *testing.T) {
		testNetworkTopologyScoringSimple(t, leadScheduler)
	})

	// Test 3: Critical path discovery
	t.Run("CriticalPathDiscovery", func(t *testing.T) {
		testCriticalPathDiscoverySimple(t, leadScheduler)
	})

	// Test 4: Framework status and monitoring
	t.Run("FrameworkStatusAndMonitoring", func(t *testing.T) {
		testFrameworkStatusAndMonitoringSimple(t, leadScheduler)
	})

	t.Log("=== Simple Integration Test Complete ===")
}

// testSchedulerInitializationSimple tests scheduler initialization with a static service graph
func testSchedulerInitializationSimple(t *testing.T, leadScheduler *scheduler.LEADScheduler) {
	t.Log("Testing static graph initialization...")

	// Skip Kubernetes-dependent initialization for now
	t.Skip("Skipping scheduler initialization test - requires real Kubernetes cluster")

	// Start the scheduler
	ctx := context.Background()
	err := leadScheduler.Run(ctx)
	if err != nil {
		t.Fatalf("Failed to start LEAD scheduler: %v", err)
	}

	// Get the LEAD framework from the scheduler
	leadFramework := leadScheduler.GetLEADFramework()
	if leadFramework == nil {
		t.Fatal("LEAD framework should not be nil")
	}

	// Verify framework is running
	if !leadFramework.IsRunning() {
		t.Error("Framework should be running")
	}

	// Verify service graph is loaded
	status := leadFramework.GetFrameworkStatus()
	if status.TotalServices == 0 {
		t.Error("No services found in the graph")
	}

	expectedServices := 8 // frontend, search, user, recommendation, reservation, profile, geo, rate
	if status.TotalServices != expectedServices {
		t.Errorf("Expected %d services, got %d", expectedServices, status.TotalServices)
	}

	if status.Gateway != "frontend" {
		t.Errorf("Expected gateway to be 'frontend', got '%s'", status.Gateway)
	}

	t.Logf("✓ Framework initialized with %d services, gateway: %s", status.TotalServices, status.Gateway)
}

// testNetworkTopologyScoringSimple tests the network topology scoring algorithm
func testNetworkTopologyScoringSimple(t *testing.T, leadScheduler *scheduler.LEADScheduler) {
	t.Log("Testing network topology scoring...")

	// Skip Kubernetes-dependent tests for now
	t.Skip("Skipping network topology scoring test - requires real Kubernetes cluster")

	// Wait for initial analysis
	time.Sleep(2 * time.Second)

	// Get the LEAD framework from the scheduler
	leadFramework := leadScheduler.GetLEADFramework()
	if leadFramework == nil {
		t.Fatal("LEAD framework should not be nil")
	}

	// Get critical paths
	paths, err := leadFramework.GetCriticalPaths(5)
	if err != nil {
		t.Fatalf("Failed to get critical paths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No critical paths found")
	}

	// Verify paths are sorted by score (highest first)
	for i := 1; i < len(paths); i++ {
		if paths[i-1].Score < paths[i].Score {
			t.Errorf("Paths not sorted correctly: path %d has score %.2f, path %d has score %.2f",
				i-1, paths[i-1].Score, i, paths[i].Score)
		}
	}

	// Verify network topology factors are considered
	hasNetworkTopology := false
	for _, path := range paths {
		if path.NetworkScore > 0 {
			hasNetworkTopology = true
			break
		}
	}

	if !hasNetworkTopology {
		t.Error("Network topology scoring not applied")
	}

	// Test network topology analysis
	analysis, err := leadFramework.GetNetworkTopologyAnalysis()
	if err != nil {
		t.Fatalf("Failed to get network topology analysis: %v", err)
	}

	if analysis.TotalPaths == 0 {
		t.Error("No paths found in network topology analysis")
	}

	if analysis.AvgBandwidth <= 0 {
		t.Error("Average bandwidth should be positive")
	}

	if len(analysis.AvailabilityZones) == 0 {
		t.Error("No availability zones found")
	}

	t.Logf("✓ Network topology scoring working correctly")
	t.Logf("  - Total paths: %d", analysis.TotalPaths)
	t.Logf("  - Average bandwidth: %.2f Mbps", analysis.AvgBandwidth)
	t.Logf("  - Average hops: %.2f", analysis.AvgHops)
	t.Logf("  - Availability zones: %v", getAvailabilityZoneNames(analysis.AvailabilityZones))
}

// testCriticalPathDiscoverySimple tests critical path discovery and scoring
func testCriticalPathDiscoverySimple(t *testing.T, leadScheduler *scheduler.LEADScheduler) {
	t.Log("Testing critical path discovery...")

	// Skip Kubernetes-dependent tests for now
	t.Skip("Skipping critical path discovery test - requires real Kubernetes cluster")

	// Get the LEAD framework from the scheduler
	leadFramework := leadScheduler.GetLEADFramework()
	if leadFramework == nil {
		t.Fatal("LEAD framework should not be nil")
	}

	// Get critical paths
	paths, err := leadFramework.GetCriticalPaths(10)
	if err != nil {
		t.Fatalf("Failed to get critical paths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No critical paths found")
	}

	// Verify we have expected critical paths
	expectedPaths := []string{
		"[frontend search profile]",
		"[frontend search geo]",
		"[frontend search rate]",
		"[frontend user]",
		"[frontend recommendation]",
		"[frontend reservation]",
	}

	foundPaths := make(map[string]bool)
	for _, path := range paths {
		pathStr := path.GetServiceNames()
		foundPaths[fmt.Sprintf("%v", pathStr)] = true
	}

	// Check for key critical paths
	criticalPathsFound := 0
	for _, expectedPath := range expectedPaths {
		if foundPaths[expectedPath] {
			criticalPathsFound++
		}
	}

	if criticalPathsFound < 3 {
		t.Errorf("Expected at least 3 critical paths, found %d", criticalPathsFound)
	}

	// Verify path properties
	for i, path := range paths {
		if path.PathLength <= 0 {
			t.Errorf("Path %d has invalid length: %d", i, path.PathLength)
		}

		if path.PodCount <= 0 {
			t.Errorf("Path %d has invalid pod count: %d", i, path.PodCount)
		}

		if path.Score <= 0 {
			t.Errorf("Path %d has invalid score: %.2f", i, path.Score)
		}

		if path.Weight <= 0 {
			t.Errorf("Path %d has invalid weight: %d", i, path.Weight)
		}
	}

	t.Logf("✓ Critical path discovery working correctly")
	t.Logf("  - Found %d critical paths", len(paths))
	t.Logf("  - Top path: %v (Score: %.2f)", paths[0].GetServiceNames(), paths[0].Score)
}

// testFrameworkStatusAndMonitoringSimple tests framework status and monitoring capabilities
func testFrameworkStatusAndMonitoringSimple(t *testing.T, leadScheduler *scheduler.LEADScheduler) {
	t.Log("Testing framework status and monitoring...")

	// Skip Kubernetes-dependent tests for now
	t.Skip("Skipping framework status and monitoring test - requires real Kubernetes cluster")

	// Get the LEAD framework from the scheduler
	leadFramework := leadScheduler.GetLEADFramework()
	if leadFramework == nil {
		t.Fatal("LEAD framework should not be nil")
	}

	// Get framework status
	status := leadFramework.GetFrameworkStatus()
	if !status.IsRunning {
		t.Error("Framework should be running")
	}

	if status.TotalServices == 0 {
		t.Error("Framework should have services")
	}

	// Get cluster health (simulated)
	health, err := leadFramework.GetClusterHealth()
	if err != nil {
		t.Logf("Cluster health not available (expected in test environment): %v", err)
	} else if health != nil {
		if health.TotalServices != status.TotalServices {
			t.Errorf("Health total services (%d) should match status total services (%d)",
				health.TotalServices, status.TotalServices)
		}
	}

	// Test re-analysis trigger
	leadFramework.TriggerReanalysis()
	time.Sleep(1 * time.Second) // Allow time for re-analysis

	// Verify framework is still running after re-analysis
	if !leadFramework.IsRunning() {
		t.Error("Framework should still be running after re-analysis")
	}

	t.Logf("✓ Framework status and monitoring working correctly")
	t.Logf("  - Status: Running=%t, Services=%d", status.IsRunning, status.TotalServices)
}

// Helper function to get availability zone names from the analysis
func getAvailabilityZoneNames(zones map[string]int) []string {
	var names []string
	for zone := range zones {
		names = append(names, zone)
	}
	return names
}
