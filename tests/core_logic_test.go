package tests

import (
	"testing"

	"lead-framework/internal/algorithms"
)

// TestCoreLEADLogic tests the core LEAD framework logic without Kubernetes integration
func TestCoreLEADLogic(t *testing.T) {
	t.Log("=== Core LEAD Logic Test ===")

	// Test 1: Service Graph Creation and Validation
	t.Run("ServiceGraphCreation", func(t *testing.T) {
		testServiceGraphCreation(t)
	})

	// Test 2: Scoring Algorithm
	t.Run("ScoringAlgorithm", func(t *testing.T) {
		testScoringAlgorithm(t)
	})

	// Test 3: Affinity Rule Generation
	t.Run("AffinityRuleGeneration", func(t *testing.T) {
		testAffinityRuleGenerationCore(t)
	})

	// Test 4: Network Topology Analysis
	t.Run("NetworkTopologyAnalysis", func(t *testing.T) {
		testNetworkTopologyAnalysis(t)
	})

	t.Log("=== Core Logic Test Complete ===")
}

// testServiceGraphCreation tests service graph creation and validation
func testServiceGraphCreation(t *testing.T) {
	t.Log("Testing service graph creation...")

	graph := createHotelReservationTestGraph()

	// Verify graph structure
	nodes := graph.GetAllNodes()
	if len(nodes) != 8 {
		t.Errorf("Expected 8 nodes, got %d", len(nodes))
	}

	// Verify gateway is set
	if graph.Gateway != "frontend" {
		t.Errorf("Expected gateway 'frontend', got '%s'", graph.Gateway)
	}

	// Verify frontend has dependencies
	frontendDeps := graph.GetAdjacentNodes("frontend")
	if len(frontendDeps) < 4 {
		t.Errorf("Expected frontend to have at least 4 dependencies, got %d", len(frontendDeps))
	}

	// Verify search has dependencies
	searchDeps := graph.GetAdjacentNodes("search")
	if len(searchDeps) < 3 {
		t.Errorf("Expected search to have at least 3 dependencies, got %d", len(searchDeps))
	}

	// Verify network topology is set for all nodes
	for name, node := range nodes {
		if node.NetworkTopology == nil {
			t.Errorf("Node '%s' should have network topology", name)
		} else {
			if node.NetworkTopology.AvailabilityZone == "" {
				t.Errorf("Node '%s' should have availability zone", name)
			}
			if node.NetworkTopology.Bandwidth <= 0 {
				t.Errorf("Node '%s' should have positive bandwidth", name)
			}
		}
	}

	t.Logf("✓ Service graph creation working correctly")
	t.Logf("  - Nodes: %d", len(nodes))
	t.Logf("  - Gateway: %s", graph.Gateway)
	t.Logf("  - Frontend dependencies: %v", frontendDeps)
}

// testScoringAlgorithm tests the scoring algorithm
func testScoringAlgorithm(t *testing.T) {
	t.Log("Testing scoring algorithm...")

	graph := createHotelReservationTestGraph()
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	// Test path scoring
	paths, err := scoringAlg.ScorePaths("frontend")
	if err != nil {
		t.Fatalf("Failed to score paths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No paths found")
	}

	// Verify paths are sorted by score
	for i := 1; i < len(paths); i++ {
		if paths[i-1].Score < paths[i].Score {
			t.Errorf("Paths not sorted correctly: path %d has score %.2f, path %d has score %.2f",
				i-1, paths[i-1].Score, i, paths[i].Score)
		}
	}

	// Verify weights are assigned correctly
	for i, path := range paths {
		expectedWeight := 100 - i
		if path.Weight != expectedWeight {
			t.Errorf("Path %d has weight %d, expected %d", i, path.Weight, expectedWeight)
		}
	}

	// Verify network topology is considered
	hasNetworkTopology := false
	for _, path := range paths {
		if path.NetworkScore > 0 {
			hasNetworkTopology = true
			break
		}
	}

	if !hasNetworkTopology {
		t.Error("Network topology should be considered in scoring")
	}

	t.Logf("✓ Scoring algorithm working correctly")
	t.Logf("  - Found %d paths", len(paths))
	t.Logf("  - Top path: %v (Score: %.2f)", paths[0].GetServiceNames(), paths[0].Score)
}

// testAffinityRuleGenerationCore tests affinity rule generation
func testAffinityRuleGenerationCore(t *testing.T) {
	t.Log("Testing affinity rule generation...")

	graph := createHotelReservationTestGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	// Test affinity rule generation for critical paths
	paths, err := algorithms.NewScoringAlgorithm(graph).ScorePaths("frontend")
	if err != nil {
		t.Fatalf("Failed to get paths for affinity generation: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No paths found for affinity generation")
	}

	// Generate affinity rules for top 3 paths
	for i := 0; i < 3 && i < len(paths); i++ {
		path := paths[i]
		rules, err := affinityGen.GenerateAffinityRules(path, path.Weight)
		if err != nil {
			t.Errorf("Failed to generate affinity rules for path %d: %v", i, err)
			continue
		}

		if rules == nil || len(rules) == 0 {
			t.Errorf("No affinity rules generated for path %d", i)
			continue
		}

		// Verify rules have required fields
		for j, rule := range rules {
			if rule == nil {
				t.Errorf("Rule %d for path %d is nil", j, i)
				continue
			}

			if rule.ServiceID == "" {
				t.Errorf("Affinity rule service ID should not be empty for path %d, rule %d", i, j)
			}

			// Verify that at least one affinity type is configured
			hasAffinity := rule.PodAffinity != nil || rule.PodAntiAffinity != nil || rule.NodeAffinity != nil
			if !hasAffinity {
				t.Errorf("Affinity rule should have at least one affinity type configured for path %d, rule %d", i, j)
			}
		}
	}

	t.Logf("✓ Affinity rule generation working correctly")
}

// testNetworkTopologyAnalysis tests network topology analysis
func testNetworkTopologyAnalysis(t *testing.T) {
	t.Log("Testing network topology analysis...")

	graph := createHotelReservationTestGraph()
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	// Test network topology analysis
	analysis, err := scoringAlg.AnalyzeNetworkTopology("frontend")
	if err != nil {
		t.Fatalf("Failed to analyze network topology: %v", err)
	}

	if analysis.TotalPaths == 0 {
		t.Error("No paths found in network topology analysis")
	}

	if analysis.AvgBandwidth <= 0 {
		t.Error("Average bandwidth should be positive")
	}

	if analysis.AvgHops < 0 {
		t.Error("Average hops should not be negative")
	}

	if analysis.AvgGeoDistance < 0 {
		t.Error("Average geo distance should not be negative")
	}

	if len(analysis.AvailabilityZones) == 0 {
		t.Error("Should have availability zones")
	}

	// Verify availability zones are from our test data
	expectedZones := []string{"us-west-1a", "us-west-1b", "us-west-1c", "us-east-1a"}
	foundZones := 0
	for _, expectedZone := range expectedZones {
		if analysis.AvailabilityZones[expectedZone] > 0 {
			foundZones++
		}
	}

	if foundZones < 2 {
		t.Errorf("Should have services in at least 2 availability zones, found %d", foundZones)
	}

	t.Logf("✓ Network topology analysis working correctly")
	t.Logf("  - Total paths: %d", analysis.TotalPaths)
	t.Logf("  - Average bandwidth: %.2f Mbps", analysis.AvgBandwidth)
	t.Logf("  - Average hops: %.2f", analysis.AvgHops)
	t.Logf("  - Average geo distance: %.2f km", analysis.AvgGeoDistance)
	t.Logf("  - Availability zones: %v", getAvailabilityZoneNamesCore(analysis.AvailabilityZones))
}

// Helper function to get availability zone names from the analysis
func getAvailabilityZoneNamesCore(zones map[string]int) []string {
	var names []string
	for zone := range zones {
		names = append(names, zone)
	}
	return names
}
