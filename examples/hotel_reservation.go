package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"lead-framework/internal/lead"
	"lead-framework/internal/models"
)

func main() {
	fmt.Println("=== LEAD Framework - Hotel Reservation Example ===")

	// Create a more comprehensive hotel reservation service graph
	graph := createHotelReservationGraph()

	// Create LEAD framework with custom configuration
	config := &lead.FrameworkConfig{
		MonitoringInterval:     15 * time.Second,
		ResourceThreshold:      75.0,
		LatencyThreshold:       150 * time.Millisecond,
		PrometheusURL:          "http://localhost:9090",
		KubernetesNamespace:    "hotel-reservation",
		OutputDirectory:        "./hotel-k8s-manifests",
		BandwidthWeight:        0.4,
		HopsWeight:             0.3,
		GeoDistanceWeight:      0.2,
		AvailabilityZoneWeight: 0.1,
	}

	leadFramework := lead.NewLEADFrameworkWithConfig(config)

	// Start the framework
	ctx := context.Background()
	if err := leadFramework.Start(ctx, graph); err != nil {
		log.Fatalf("Failed to start LEAD framework: %v", err)
	}

	// Demonstrate framework capabilities
	demonstrateFramework(leadFramework)

	// Keep running for demonstration
	fmt.Println("LEAD Framework is running. Press Ctrl+C to stop.")
	select {}
}

func createHotelReservationGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Frontend service (API Gateway)
	graph.AddNode(&models.ServiceNode{
		ID:       "fe",
		Name:     "frontend",
		Replicas: 3,
		RPS:      1500,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        1000, // Mbps
			Hops:             0,
			GeoDistance:      0,
		},
	})

	// Search service
	graph.AddNode(&models.ServiceNode{
		ID:       "src",
		Name:     "search",
		Replicas: 4,
		RPS:      1200,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      100, // km
		},
	})

	// User service
	graph.AddNode(&models.ServiceNode{
		ID:       "usr",
		Name:     "user",
		Replicas: 3,
		RPS:      800,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      0,
		},
	})

	// Recommendation service
	graph.AddNode(&models.ServiceNode{
		ID:       "rcm",
		Name:     "recommendation",
		Replicas: 2,
		RPS:      600,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1c",
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      200,
		},
	})

	// Reservation service
	graph.AddNode(&models.ServiceNode{
		ID:       "rsv",
		Name:     "reservation",
		Replicas: 3,
		RPS:      900,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      100,
		},
	})

	// Profile service
	graph.AddNode(&models.ServiceNode{
		ID:       "prf",
		Name:     "profile",
		Replicas: 2,
		RPS:      400,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        600,
			Hops:             2,
			GeoDistance:      0,
		},
	})

	// Geographic service
	graph.AddNode(&models.ServiceNode{
		ID:       "geo",
		Name:     "geographic",
		Replicas: 2,
		RPS:      300,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        500,
			Hops:             2,
			GeoDistance:      100,
		},
	})

	// Rate service
	graph.AddNode(&models.ServiceNode{
		ID:       "rte",
		Name:     "rate",
		Replicas: 2,
		RPS:      500,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1c",
			Bandwidth:        700,
			Hops:             2,
			GeoDistance:      200,
		},
	})

	// Define service dependencies
	// Frontend calls core services
	graph.AddEdge("fe", "src")
	graph.AddEdge("fe", "usr")
	graph.AddEdge("fe", "rcm")
	graph.AddEdge("fe", "rsv")

	// Search service calls supporting services
	graph.AddEdge("src", "prf")
	graph.AddEdge("src", "geo")
	graph.AddEdge("src", "rte")

	// Set gateway
	graph.SetGateway("fe")

	return graph
}

func demonstrateFramework(leadFramework *lead.LEADFramework) {
	fmt.Println("\n=== Demonstrating LEAD Framework Capabilities ===")

	// Wait a moment for initial analysis to complete
	time.Sleep(2 * time.Second)

	// Get framework status
	status := leadFramework.GetFrameworkStatus()
	fmt.Printf("Framework Status: Running=%t, Services=%d, Gateway=%s\n",
		status.IsRunning, status.TotalServices, status.Gateway)

	// Get critical paths
	paths, err := leadFramework.GetCriticalPaths(3)
	if err != nil {
		log.Printf("Failed to get critical paths: %v", err)
	} else {
		fmt.Println("\nTop 3 Critical Paths:")
		for i, path := range paths {
			fmt.Printf("%d. %v (Score: %.2f, Weight: %d)\n",
				i+1, path.GetServiceNames(), path.Score, path.Weight)
		}
	}

	// Get network topology analysis
	analysis, err := leadFramework.GetNetworkTopologyAnalysis()
	if err != nil {
		log.Printf("Failed to get network topology analysis: %v", err)
	} else {
		fmt.Printf("\nNetwork Topology Analysis:\n")
		fmt.Printf("- Total Paths: %d\n", analysis.TotalPaths)
		fmt.Printf("- Average Bandwidth: %.2f Mbps\n", analysis.AvgBandwidth)
		fmt.Printf("- Average Hops: %.2f\n", analysis.AvgHops)
		fmt.Printf("- Average Geo Distance: %.2f km\n", analysis.AvgGeoDistance)
		fmt.Println("- Availability Zones:")
		for az, count := range analysis.AvailabilityZones {
			fmt.Printf("  * %s: %d services\n", az, count)
		}
	}

	// Simulate some monitoring data
	fmt.Println("\nSimulating service monitoring...")
	time.Sleep(5 * time.Second)

	// Get cluster health
	health, err := leadFramework.GetClusterHealth()
	if err == nil && health != nil {
		fmt.Printf("\nCluster Health Summary:\n")
		fmt.Printf("- Total Services: %d\n", health.TotalServices)
		fmt.Printf("- Healthy Services: %d\n", health.HealthyServices)
		fmt.Printf("- Bottleneck Services: %d\n", health.BottleneckServices)
		fmt.Printf("- Average CPU Usage: %.2f%%\n", health.AvgCPUUsage)
		fmt.Printf("- Average Memory Usage: %.2f%%\n", health.AvgMemoryUsage)
		fmt.Printf("- Average Latency: %.2f ms\n", health.AvgLatency)
	}

	fmt.Println("\n=== Demonstration Complete ===")
}
