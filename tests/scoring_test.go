package tests

import (
	"testing"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/models"
)

func TestScoringAlgorithm(t *testing.T) {
	// Create test service graph
	graph := createTestGraph()

	// Create scoring algorithm
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	// Test scoring paths
	paths, err := scoringAlg.ScorePaths("fe")
	if err != nil {
		t.Fatalf("Failed to score paths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No paths found")
	}

	// Verify paths are sorted by score (highest first)
	for i := 1; i < len(paths); i++ {
		if paths[i-1].Score < paths[i].Score {
			t.Errorf("Paths not sorted correctly: path %d has score %.2f, path %d has score %.2f",
				i-1, paths[i-1].Score, i, paths[i].Score)
		}
	}

	// Verify weights are assigned correctly (decreasing from 100)
	for i := 0; i < len(paths); i++ {
		expectedWeight := 100 - i
		if paths[i].Weight != expectedWeight {
			t.Errorf("Path %d has weight %d, expected %d", i, paths[i].Weight, expectedWeight)
		}
	}

	// Test network topology analysis
	analysis, err := scoringAlg.AnalyzeNetworkTopology("fe")
	if err != nil {
		t.Fatalf("Failed to analyze network topology: %v", err)
	}

	if analysis.TotalPaths == 0 {
		t.Error("No paths found in network topology analysis")
	}

	t.Logf("Found %d paths with network analysis", analysis.TotalPaths)
	t.Logf("Average bandwidth: %.2f Mbps", analysis.AvgBandwidth)
	t.Logf("Average hops: %.2f", analysis.AvgHops)
}

func TestPathFinding(t *testing.T) {
	graph := createTestGraph()
	pathFinder := models.NewPathFinder(graph)

	// Test finding all paths
	paths := pathFinder.FindAllPaths("fe")

	if len(paths) == 0 {
		t.Fatal("No paths found from gateway")
	}

	// Verify all paths start with gateway
	for _, path := range paths {
		if path.Services[0].ID != "fe" {
			t.Errorf("Path does not start with gateway: %v", path.GetServiceNames())
		}
	}

	t.Logf("Found %d paths from gateway", len(paths))
	for i, path := range paths {
		t.Logf("Path %d: %v", i+1, path.GetServiceNames())
	}
}

func TestNetworkTopologyScoring(t *testing.T) {
	t.Log("Testing network topology scoring with HotelReservation benchmark...")

	// Create a comprehensive graph with different network topologies
	graph := createHotelReservationTestGraph()
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	// Test scoring with mixed network topologies
	paths, err := scoringAlg.ScorePaths("frontend")
	if err != nil {
		t.Fatalf("Failed to score paths: %v", err)
	}

	if len(paths) < 3 {
		t.Fatalf("Expected at least 3 paths for HotelReservation, got %d", len(paths))
	}

	t.Logf("Found %d paths with different network topologies", len(paths))
	for i, path := range paths {
		t.Logf("Path %d: %v (Score: %.2f, Network Score: %.2f)",
			i+1, path.GetServiceNames(), path.Score, path.NetworkScore)
	}

	// Verify that paths are sorted by score (highest first)
	for i := 1; i < len(paths); i++ {
		if paths[i-1].Score < paths[i].Score {
			t.Errorf("Paths not sorted correctly: path %d has score %.2f, path %d has score %.2f",
				i-1, paths[i-1].Score, i, paths[i].Score)
		}
	}

	// Verify that network scores are different for different topologies
	networkScores := make(map[float64]bool)
	for _, path := range paths {
		networkScores[path.NetworkScore] = true
	}

	if len(networkScores) == 1 {
		t.Error("All paths have the same network score, expected different network topologies to produce different scores")
	}

	// Verify network topology factors are properly weighted
	hasGoodNetworkPath := false
	hasPoorNetworkPath := false

	for _, path := range paths {
		if path.NetworkScore > 1.5 {
			hasGoodNetworkPath = true
		}
		if path.NetworkScore < 1.3 {
			hasPoorNetworkPath = true
		}
	}

	if !hasGoodNetworkPath {
		t.Error("Should have paths with good network topology scores")
	}

	if !hasPoorNetworkPath {
		t.Error("Should have paths with poor network topology scores")
	}

	t.Logf("✓ Network topology scoring working correctly with %d distinct network scores", len(networkScores))
}

// createHotelReservationTestGraph creates a comprehensive HotelReservation test graph
func createHotelReservationTestGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Frontend service (API Gateway) - Excellent network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "frontend",
		Name:     "frontend",
		Replicas: 3,
		RPS:      1500,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        1000, // High bandwidth
			Hops:             0,    // No hops (gateway)
			GeoDistance:      0,    // No distance
		},
	})

	// Search service - Good network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "search",
		Name:     "search",
		Replicas: 4,
		RPS:      1200,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        800, // Good bandwidth
			Hops:             1,   // 1 hop from gateway
			GeoDistance:      100, // Close distance
		},
	})

	// User service - Good network topology (same AZ as frontend)
	graph.AddNode(&models.ServiceNode{
		ID:       "user",
		Name:     "user",
		Replicas: 3,
		RPS:      800,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a", // Same AZ as frontend
			Bandwidth:        800,          // Good bandwidth
			Hops:             1,            // 1 hop from gateway
			GeoDistance:      0,            // Same AZ
		},
	})

	// Recommendation service - Poor network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "recommendation",
		Name:     "recommendation",
		Replicas: 2,
		RPS:      600,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-east-1a", // Different region
			Bandwidth:        300,          // Low bandwidth
			Hops:             3,            // Many hops
			GeoDistance:      3000,         // Far distance
		},
	})

	// Reservation service - Good network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "reservation",
		Name:     "reservation",
		Replicas: 3,
		RPS:      900,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b", // Same AZ as search
			Bandwidth:        800,          // Good bandwidth
			Hops:             1,            // 1 hop from gateway
			GeoDistance:      100,          // Close distance
		},
	})

	// Profile service - Moderate network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "profile",
		Name:     "profile",
		Replicas: 2,
		RPS:      400,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1c", // Different AZ
			Bandwidth:        500,          // Moderate bandwidth
			Hops:             2,            // 2 hops (through search)
			GeoDistance:      200,          // Moderate distance
		},
	})

	// Geographic service - Poor network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "geo",
		Name:     "geographic",
		Replicas: 2,
		RPS:      300,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1c", // Different AZ
			Bandwidth:        300,          // Low bandwidth
			Hops:             2,            // 2 hops (through search)
			GeoDistance:      200,          // Moderate distance
		},
	})

	// Rate service - Moderate network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "rate",
		Name:     "rate",
		Replicas: 2,
		RPS:      500,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b", // Same AZ as search
			Bandwidth:        600,          // Moderate bandwidth
			Hops:             2,            // 2 hops (through search)
			GeoDistance:      100,          // Close distance
		},
	})

	// Define service dependencies (HotelReservation benchmark)
	// Frontend calls core services
	graph.AddEdge("frontend", "search")
	graph.AddEdge("frontend", "user")
	graph.AddEdge("frontend", "recommendation")
	graph.AddEdge("frontend", "reservation")

	// Search service calls supporting services
	graph.AddEdge("search", "profile")
	graph.AddEdge("search", "geo")
	graph.AddEdge("search", "rate")

	// Set gateway
	graph.SetGateway("frontend")

	return graph
}

func createTestGraphWithMultipleTopologies() *models.ServiceGraph {
	// Use the HotelReservation graph for consistency
	return createHotelReservationTestGraph()
}

func createTestGraphWithTopology(bandwidth float64, hops int, geoDistance float64) *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Add nodes with specific network topology
	graph.AddNode(&models.ServiceNode{
		ID:       "fe",
		Name:     "frontend",
		Replicas: 3,
		RPS:      1000,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        bandwidth,
			Hops:             hops,
			GeoDistance:      geoDistance,
		},
	})

	graph.AddNode(&models.ServiceNode{
		ID:       "src",
		Name:     "search",
		Replicas: 2,
		RPS:      800,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        bandwidth * 0.8,
			Hops:             hops + 1,
			GeoDistance:      geoDistance + 50,
		},
	})

	// Add edges
	graph.AddEdge("fe", "src")

	// Set gateway
	graph.SetGateway("fe")

	return graph
}

func createTestGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Add nodes
	graph.AddNode(&models.ServiceNode{
		ID:       "fe",
		Name:     "frontend",
		Replicas: 3,
		RPS:      1000,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        1000,
			Hops:             0,
			GeoDistance:      0,
		},
	})

	graph.AddNode(&models.ServiceNode{
		ID:       "src",
		Name:     "search",
		Replicas: 2,
		RPS:      800,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      100,
		},
	})

	graph.AddNode(&models.ServiceNode{
		ID:       "usr",
		Name:     "user",
		Replicas: 2,
		RPS:      600,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      0,
		},
	})

	graph.AddNode(&models.ServiceNode{
		ID:       "rcm",
		Name:     "recommendation",
		Replicas: 1,
		RPS:      400,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1c",
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      200,
		},
	})

	// Add edges
	graph.AddEdge("fe", "src")
	graph.AddEdge("fe", "usr")
	graph.AddEdge("fe", "rcm")

	// Set gateway
	graph.SetGateway("fe")

	return graph
}

func BenchmarkScoringAlgorithm(b *testing.B) {
	graph := createTestGraph()
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scoringAlg.ScorePaths("fe")
		if err != nil {
			b.Fatalf("Scoring failed: %v", err)
		}
	}
}

func BenchmarkPathFinding(b *testing.B) {
	graph := createTestGraph()
	pathFinder := models.NewPathFinder(graph)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pathFinder.FindAllPaths("fe")
	}
}
