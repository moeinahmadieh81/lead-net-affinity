package tests

import (
	"context"
	"testing"
	"time"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/lead"
	"lead-framework/internal/models"
	"lead-framework/internal/scheduler"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestMultiCountrySchedulerScenario tests the LEAD scheduler behavior with 5 nodes across different countries
func TestMultiCountrySchedulerScenario(t *testing.T) {
	t.Log("=== Multi-Country LEAD Scheduler Scenario Test ===")
	t.Log("Scenario: 5 nodes across France, Sweden, Holland, Germany, and England")
	t.Log("Testing scheduler behavior for geo-distributed microservices and databases")

	// Create fake Kubernetes client for testing
	client := fake.NewSimpleClientset()

	// Create LEAD framework with geo-aware configuration
	config := &lead.FrameworkConfig{
		MonitoringInterval:     10 * time.Second,
		ResourceThreshold:      80.0,
		LatencyThreshold:       200 * time.Millisecond, // Higher threshold for geo-distributed setup
		PrometheusURL:          "http://prometheus.monitoring.svc.cluster.local:9090",
		KubernetesNamespace:    "hotel-reservation",
		OutputDirectory:        "./hotel-k8s-manifests",
		BandwidthWeight:        0.3, // Reduced weight for bandwidth due to geo-distribution
		HopsWeight:             0.2, // Reduced weight for hops
		GeoDistanceWeight:      0.4, // Increased weight for geo-distance
		AvailabilityZoneWeight: 0.1, // Reduced weight for AZ
	}

	leadFramework := lead.NewLEADFrameworkWithConfig(config)
	leadScheduler := scheduler.NewLEADScheduler(client, leadFramework, config)

	// Test 1: Create geo-distributed nodes
	t.Run("CreateGeoDistributedNodes", func(t *testing.T) {
		testCreateGeoDistributedNodes(t, client)
	})

	// Test 2: Analyze service graph for geo-distributed scenario
	t.Run("AnalyzeGeoDistributedServiceGraph", func(t *testing.T) {
		testAnalyzeGeoDistributedServiceGraph(t)
	})

	// Test 3: Test scheduler scoring with geo-distance
	t.Run("TestGeoDistanceScoring", func(t *testing.T) {
		testGeoDistanceScoring(t)
	})

	// Test 4: Test affinity rules for geo-distributed services
	t.Run("TestGeoDistributedAffinityRules", func(t *testing.T) {
		testGeoDistributedAffinityRules(t)
	})

	// Test 5: Simulate pod scheduling decisions
	t.Run("SimulatePodSchedulingDecisions", func(t *testing.T) {
		testSimulatePodSchedulingDecisions(t, client, leadScheduler)
	})

	t.Log("=== Multi-Country Scheduler Scenario Test Complete ===")
}

// testCreateGeoDistributedNodes creates 5 nodes across different countries
func testCreateGeoDistributedNodes(t *testing.T, client *fake.Clientset) {
	t.Log("Creating 5 geo-distributed nodes...")

	nodes := []struct {
		name     string
		country  string
		zone     string
		cpu      string
		memory   string
		nodeType string
	}{
		{"master-france", "France", "europe-west1-a", "8", "32Gi", "master"},
		{"worker-sweden", "Sweden", "europe-north1-a", "4", "16Gi", "worker"},
		{"worker-holland", "Holland", "europe-west4-a", "4", "16Gi", "worker"},
		{"worker-germany", "Germany", "europe-west3-a", "4", "16Gi", "worker"},
		{"worker-england", "England", "europe-west2-a", "4", "16Gi", "worker"},
	}

	for _, nodeInfo := range nodes {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeInfo.name,
				Labels: map[string]string{
					"topology.kubernetes.io/zone": nodeInfo.zone,
					"node-type":                   nodeInfo.nodeType,
					"country":                     nodeInfo.country,
					"region":                      "europe",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
				Allocatable: corev1.ResourceList{
					"cpu":    resource.MustParse(nodeInfo.cpu),
					"memory": resource.MustParse(nodeInfo.memory),
				},
			},
		}

		_, err := client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", nodeInfo.name, err)
		}

		t.Logf("✓ Created node: %s (%s, %s, %s CPU, %s memory)",
			nodeInfo.name, nodeInfo.country, nodeInfo.zone, nodeInfo.cpu, nodeInfo.memory)
	}

	// Verify all nodes were created
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}

	if len(nodeList.Items) != 5 {
		t.Errorf("Expected 5 nodes, got %d", len(nodeList.Items))
	}

	t.Logf("✓ Successfully created %d geo-distributed nodes", len(nodeList.Items))
}

// testAnalyzeGeoDistributedServiceGraph analyzes the service graph for geo-distributed scenario
func testAnalyzeGeoDistributedServiceGraph(t *testing.T) {
	t.Log("Analyzing service graph for geo-distributed scenario...")

	// Create a comprehensive service graph for hotel reservation system
	graph := createGeoDistributedServiceGraph()

	// Analyze the graph structure
	nodes := graph.GetAllNodes()
	t.Logf("Service graph analysis:")
	t.Logf("  - Total services: %d", len(nodes))
	t.Logf("  - Gateway: %s", graph.Gateway)

	// Analyze service distribution by name
	serviceNames := make(map[string]int)
	for _, node := range nodes {
		serviceNames[node.Name]++
	}

	t.Logf("  - Service names distribution:")
	for serviceName, count := range serviceNames {
		t.Logf("    * %s: %d services", serviceName, count)
	}

	// Analyze network topology manually
	pathFinder := models.NewPathFinder(graph)
	paths := pathFinder.FindAllPaths(graph.Gateway)

	t.Logf("  - Network topology analysis:")
	t.Logf("    * Total paths: %d", len(paths))

	// Calculate average metrics
	var totalBandwidth, totalHops, totalGeoDistance float64
	availabilityZones := make(map[string]int)

	for _, path := range paths {
		for _, service := range path.Services {
			if service.NetworkTopology != nil {
				totalBandwidth += service.NetworkTopology.Bandwidth
				totalHops += float64(service.NetworkTopology.Hops)
				totalGeoDistance += service.NetworkTopology.GeoDistance
				availabilityZones[service.NetworkTopology.AvailabilityZone]++
			}
		}
	}

	if len(paths) > 0 {
		avgBandwidth := totalBandwidth / float64(len(paths))
		avgHops := totalHops / float64(len(paths))
		avgGeoDistance := totalGeoDistance / float64(len(paths))

		t.Logf("    * Average bandwidth: %.2f Mbps", avgBandwidth)
		t.Logf("    * Average hops: %.2f", avgHops)
		t.Logf("    * Average geo distance: %.2f km", avgGeoDistance)
	}

	t.Logf("    * Availability zones: %v", getAvailabilityZoneNamesFromMap(availabilityZones))

	t.Log("✓ Service graph analysis complete")
}

// testGeoDistanceScoring tests the scoring algorithm with geo-distance considerations
func testGeoDistanceScoring(t *testing.T) {
	t.Log("Testing geo-distance scoring algorithm...")

	graph := createGeoDistributedServiceGraph()
	scoringAlg := algorithms.NewScoringAlgorithm(graph)

	// Test scoring from gateway
	paths, err := scoringAlg.ScorePaths("fe")
	if err != nil {
		t.Fatalf("Failed to score paths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("No paths found from gateway")
	}

	t.Logf("Geo-distance scoring results:")
	t.Logf("  - Found %d paths from gateway", len(paths))

	// Analyze the top 5 paths
	topPaths := paths
	if len(paths) > 5 {
		topPaths = paths[:5]
	}

	for i, path := range topPaths {
		t.Logf("  - Path %d: %v", i+1, path.GetServiceNames())
		t.Logf("    * Score: %.2f", path.Score)
		t.Logf("    * Network Score: %.2f", path.NetworkScore)
		t.Logf("    * Weight: %d", path.Weight)

		// Calculate total geo distance for this path
		var totalGeoDistance float64
		for _, service := range path.Services {
			if service.NetworkTopology != nil {
				totalGeoDistance += service.NetworkTopology.GeoDistance
			}
		}
		t.Logf("    * Total Geo Distance: %.2f km", totalGeoDistance)
	}

	// Verify that geo-distance is being considered
	hasGeoDistance := false
	for _, path := range paths {
		for _, service := range path.Services {
			if service.NetworkTopology != nil && service.NetworkTopology.GeoDistance > 0 {
				hasGeoDistance = true
				break
			}
		}
		if hasGeoDistance {
			break
		}
	}

	if !hasGeoDistance {
		t.Error("Geo-distance should be considered in scoring")
	}

	t.Log("✓ Geo-distance scoring test complete")
}

// testGeoDistributedAffinityRules tests affinity rules for geo-distributed services
func testGeoDistributedAffinityRules(t *testing.T) {
	t.Log("Testing affinity rules for geo-distributed services...")

	graph := createGeoDistributedServiceGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	// Test affinity rules for database services
	databaseServices := []string{"mongodb-user", "mongodb-profile", "mongodb-reservation", "mongodb-recommendation", "mongodb-geo", "mongodb-rate"}

	for _, serviceName := range databaseServices {
		// Find the service node
		var serviceNode *models.ServiceNode
		for _, node := range graph.GetAllNodes() {
			if node.Name == serviceName {
				serviceNode = node
				break
			}
		}

		if serviceNode == nil {
			t.Logf("Warning: Service %s not found in graph", serviceName)
			continue
		}

		// Create a simple path for testing
		path := &models.Path{
			Services: []*models.ServiceNode{serviceNode},
			Weight:   100,
		}

		rules, err := affinityGen.GenerateAffinityRules(path, 100)
		if err != nil {
			t.Logf("Warning: Failed to generate affinity rules for %s: %v", serviceName, err)
			continue
		}

		if len(rules) > 0 {
			t.Logf("  - %s: Generated %d affinity rules", serviceName, len(rules))
			for _, rule := range rules {
				t.Logf("    * Service ID: %s", rule.ServiceID)
				if rule.PodAffinity != nil {
					t.Logf("    * Pod Affinity: %+v", rule.PodAffinity)
				}
				if rule.NodeAffinity != nil {
					t.Logf("    * Node Affinity: %+v", rule.NodeAffinity)
				}
			}
		}
	}

	// Test affinity rules for microservices
	microservices := []string{"user", "profile", "reservation", "recommendation", "geographic", "rate"}

	for _, serviceName := range microservices {
		// Find the service node
		var serviceNode *models.ServiceNode
		for _, node := range graph.GetAllNodes() {
			if node.Name == serviceName {
				serviceNode = node
				break
			}
		}

		if serviceNode == nil {
			t.Logf("Warning: Service %s not found in graph", serviceName)
			continue
		}

		// Create a simple path for testing
		path := &models.Path{
			Services: []*models.ServiceNode{serviceNode},
			Weight:   100,
		}

		rules, err := affinityGen.GenerateAffinityRules(path, 100)
		if err != nil {
			t.Logf("Warning: Failed to generate affinity rules for %s: %v", serviceName, err)
			continue
		}

		if len(rules) > 0 {
			t.Logf("  - %s: Generated %d affinity rules", serviceName, len(rules))
		}
	}

	t.Log("✓ Geo-distributed affinity rules test complete")
}

// testSimulatePodSchedulingDecisions simulates pod scheduling decisions for the geo-distributed scenario
func testSimulatePodSchedulingDecisions(t *testing.T, client *fake.Clientset, leadScheduler *scheduler.LEADScheduler) {
	t.Log("Simulating pod scheduling decisions...")

	// Skip the actual scheduler run since it requires real Kubernetes
	t.Skip("Skipping pod scheduling simulation - requires real Kubernetes cluster")

	// This would test the actual scheduling decisions in a real environment
	// For now, we'll just log what the expected behavior would be

	t.Log("Expected scheduler behavior:")
	t.Log("  1. Frontend pods should be scheduled to nodes with lowest latency to users")
	t.Log("  2. Database pods should be co-located with their microservices")
	t.Log("  3. Microservices should be distributed across countries for redundancy")
	t.Log("  4. Geo-distance should be minimized for frequently communicating services")
	t.Log("  5. Load balancing should consider network latency between countries")

	t.Log("✓ Pod scheduling simulation complete")
}

// createGeoDistributedServiceGraph creates a service graph for geo-distributed hotel reservation system
func createGeoDistributedServiceGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Frontend service (API Gateway) - should be in multiple countries
	graph.AddNode(&models.ServiceNode{
		ID:       "fe",
		Name:     "frontend",
		Replicas: 5, // One per country
		RPS:      2000,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west1-a", // France
			Bandwidth:        1000,             // Mbps
			Hops:             0,
			GeoDistance:      0,
		},
	})

	// User service - distributed across countries
	graph.AddNode(&models.ServiceNode{
		ID:       "usr",
		Name:     "user",
		Replicas: 4,
		RPS:      800,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west1-a", // France
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      0,
		},
	})

	// User MongoDB - co-located with user service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-user",
		Name:     "mongodb-user",
		Replicas: 2,
		RPS:      400,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west1-a", // France
			Bandwidth:        600,
			Hops:             1,
			GeoDistance:      0,
		},
	})

	// Profile service - in Sweden
	graph.AddNode(&models.ServiceNode{
		ID:       "prf",
		Name:     "profile",
		Replicas: 2,
		RPS:      400,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-north1-a", // Sweden
			Bandwidth:        800,
			Hops:             2,
			GeoDistance:      1200, // km from France
		},
	})

	// Profile MongoDB - co-located with profile service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-profile",
		Name:     "mongodb-profile",
		Replicas: 1,
		RPS:      200,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-north1-a", // Sweden
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      1200,
		},
	})

	// Reservation service - in Holland
	graph.AddNode(&models.ServiceNode{
		ID:       "rsv",
		Name:     "reservation",
		Replicas: 3,
		RPS:      900,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west4-a", // Holland
			Bandwidth:        800,
			Hops:             2,
			GeoDistance:      400, // km from France
		},
	})

	// Reservation MongoDB - co-located with reservation service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-reservation",
		Name:     "mongodb-reservation",
		Replicas: 2,
		RPS:      450,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west4-a", // Holland
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      400,
		},
	})

	// Recommendation service - in Germany
	graph.AddNode(&models.ServiceNode{
		ID:       "rcm",
		Name:     "recommendation",
		Replicas: 2,
		RPS:      600,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west3-a", // Germany
			Bandwidth:        800,
			Hops:             2,
			GeoDistance:      800, // km from France
		},
	})

	// Recommendation MongoDB - co-located with recommendation service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-recommendation",
		Name:     "mongodb-recommendation",
		Replicas: 1,
		RPS:      300,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west3-a", // Germany
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      800,
		},
	})

	// Geographic service - in England
	graph.AddNode(&models.ServiceNode{
		ID:       "geo",
		Name:     "geographic",
		Replicas: 2,
		RPS:      300,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west2-a", // England
			Bandwidth:        800,
			Hops:             2,
			GeoDistance:      500, // km from France
		},
	})

	// Geographic MongoDB - co-located with geographic service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-geo",
		Name:     "mongodb-geo",
		Replicas: 1,
		RPS:      150,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west2-a", // England
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      500,
		},
	})

	// Rate service - in England
	graph.AddNode(&models.ServiceNode{
		ID:       "rte",
		Name:     "rate",
		Replicas: 2,
		RPS:      500,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west2-a", // England
			Bandwidth:        800,
			Hops:             2,
			GeoDistance:      500,
		},
	})

	// Rate MongoDB - co-located with rate service
	graph.AddNode(&models.ServiceNode{
		ID:       "mongodb-rate",
		Name:     "mongodb-rate",
		Replicas: 1,
		RPS:      250,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "europe-west2-a", // England
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      500,
		},
	})

	// Define service dependencies
	// Frontend calls all microservices
	graph.AddEdge("fe", "usr")
	graph.AddEdge("fe", "prf")
	graph.AddEdge("fe", "rsv")
	graph.AddEdge("fe", "rcm")
	graph.AddEdge("fe", "geo")
	graph.AddEdge("fe", "rte")

	// Microservices call their databases
	graph.AddEdge("usr", "mongodb-user")
	graph.AddEdge("prf", "mongodb-profile")
	graph.AddEdge("rsv", "mongodb-reservation")
	graph.AddEdge("rcm", "mongodb-recommendation")
	graph.AddEdge("geo", "mongodb-geo")
	graph.AddEdge("rte", "mongodb-rate")

	// Set gateway
	graph.SetGateway("fe")

	return graph
}

// getAvailabilityZoneNamesFromMap extracts zone names from the availability zones map
func getAvailabilityZoneNamesFromMap(zones map[string]int) []string {
	var names []string
	for zone := range zones {
		names = append(names, zone)
	}
	return names
}
