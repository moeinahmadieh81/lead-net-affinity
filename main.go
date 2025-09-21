package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lead-framework/internal/http"
	"lead-framework/internal/lead"
	"lead-framework/internal/models"
)

func main() {
	// Create LEAD framework instance with HotelReservation configuration
	config := &lead.FrameworkConfig{
		MonitoringInterval:     15 * time.Second,
		ResourceThreshold:      75.0,
		LatencyThreshold:       150 * time.Millisecond,
		PrometheusURL:          "http://prometheus.monitoring.svc.cluster.local:9090",
		KubernetesNamespace:    "default", // Change to your namespace
		OutputDirectory:        "/app/k8s-manifests",
		BandwidthWeight:        0.4,
		HopsWeight:             0.3,
		GeoDistanceWeight:      0.2,
		AvailabilityZoneWeight: 0.1,
	}

	leadFramework := lead.NewLEADFrameworkWithConfig(config)

	// Start the LEAD framework (it will discover services dynamically)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pass nil graph - LEAD will discover services from running pods
	if err := leadFramework.Start(ctx, nil); err != nil {
		log.Fatalf("Failed to start LEAD framework: %v", err)
	}

	// Start HTTP server
	httpServer := http.NewServer(leadFramework, 8080)
	go func() {
		if err := httpServer.Start(); err != nil && err != context.Canceled {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("LEAD Framework started successfully")
	fmt.Println("HTTP server running on port 8080")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /health - Health check")
	fmt.Println("  GET  /ready - Readiness check")
	fmt.Println("  GET  /status - Framework status")
	fmt.Println("  GET  /paths - Critical paths")
	fmt.Println("  GET  /health-summary - Cluster health")
	fmt.Println("  GET  /network-topology - Network analysis")
	fmt.Println("  POST /reanalyze - Trigger re-analysis")

	<-sigChan
	fmt.Println("\nShutting down LEAD Framework...")

	// Stop HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpServer.Stop(shutdownCtx)

	// Stop LEAD framework
	leadFramework.Stop()

	fmt.Println("LEAD Framework stopped")
}

func createExampleGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Add nodes (services)
	graph.AddNode(&models.ServiceNode{
		ID:       "fe",
		Name:     "frontend",
		Replicas: 3,
		RPS:      1000,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1a",
			Bandwidth:        1000, // Mbps
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
			GeoDistance:      100, // km
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

	graph.AddNode(&models.ServiceNode{
		ID:       "rsv",
		Name:     "reservation",
		Replicas: 2,
		RPS:      700,
		NetworkTopology: &models.NetworkTopology{
			AvailabilityZone: "us-west-1b",
			Bandwidth:        800,
			Hops:             1,
			GeoDistance:      100,
		},
	})

	// Add edges (dependencies)
	graph.AddEdge("fe", "src")
	graph.AddEdge("fe", "usr")
	graph.AddEdge("fe", "rcm")
	graph.AddEdge("fe", "rsv")
	graph.AddEdge("src", "prf")
	graph.AddEdge("src", "geo")
	graph.AddEdge("src", "rte")

	// Set gateway
	graph.SetGateway("fe")

	return graph
}
