# LEAD Framework with Network Topology

This implementation extends the LEAD (Latency-aware Enhanced Affinity-based Deployment) framework with comprehensive network topology support for Kubernetes-based microservices deployment optimization.

## Overview

LEAD is a framework integrated into Kubernetes that utilizes three main algorithms to analyze service interaction graphs for critical path discovery, considering resource requirements, dynamic performance metrics, and **network topology parameters**.

### Key Features

- **Algorithm 1: Scoring** - Evaluates critical paths considering network topology (bandwidth, hops, geo distance, availability zones)
- **Algorithm 2: Real-time Monitoring** - Continuously monitors services and dynamically scales bottleneck services
- **Algorithm 3: Affinity Rule Generator** - Generates Kubernetes affinity rules for optimal service co-location
- **Network Topology Integration** - Analyzes and optimizes based on network characteristics
- **Prometheus Integration** - Real-time metrics collection and monitoring
- **Kubernetes Config Generation** - Automatic generation of deployment and service manifests

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │  Business Logic  │    │ Caching & DB    │
│   (Gateway)     │    │   Microservices  │    │   Services      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
    ┌────▼────┐              ┌───▼────┐              ┌───▼────┐
    │ Network │              │Network │              │Network │
    │ Topology│              │Topology│              │Topology│
    └─────────┘              └────────┘              └────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │    LEAD Framework       │
                    │  - Scoring Algorithm    │
                    │  - Monitoring Algorithm │
                    │  - Affinity Generator   │
                    └─────────────────────────┘
```

## Network Topology Parameters

The framework now considers the following network parameters for each worker node:

- **Bandwidth**: Network bandwidth capacity (Mbps)
- **Hops**: Number of network hops from the gateway
- **Geo Distance**: Geographic distance from the gateway (km)
- **Availability Zone**: Cloud availability zone or data center location

## Installation

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster
- Prometheus (optional, for monitoring)

### Build and Run

```bash
# Clone the repository
git clone <repository-url>
cd lead-framework

# Install dependencies
go mod tidy

# Build the application
go build -o lead-framework main.go

# Run the example
go run main.go
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "lead-framework/internal/lead"
    "lead-framework/internal/models"
)

func main() {
    // Create service graph with network topology
    graph := models.NewServiceGraph()
    
    // Add services with network topology information
    graph.AddNode(&models.ServiceNode{
        ID:       "frontend",
        Name:     "api-gateway",
        Replicas: 3,
        RPS:      1000,
        NetworkTopology: &models.NetworkTopology{
            AvailabilityZone: "us-west-1a",
            Bandwidth:        1000, // Mbps
            Hops:             0,
            GeoDistance:      0,
        },
    })
    
    // Add more services...
    graph.AddEdge("frontend", "search")
    graph.SetGateway("frontend")
    
    // Create and start LEAD framework
    leadFramework := lead.NewLEADFramework()
    ctx := context.Background()
    
    if err := leadFramework.Start(ctx, graph); err != nil {
        log.Fatal(err)
    }
}
```

### Advanced Configuration

```go
config := &lead.FrameworkConfig{
    MonitoringInterval:    15 * time.Second,
    ResourceThreshold:     75.0,
    LatencyThreshold:      150 * time.Millisecond,
    PrometheusURL:        "http://localhost:9090",
    KubernetesNamespace:  "my-namespace",
    OutputDirectory:      "./k8s-manifests",
    BandwidthWeight:      0.4,
    HopsWeight:           0.3,
    GeoDistanceWeight:    0.2,
    AvailabilityZoneWeight: 0.1,
}

leadFramework := lead.NewLEADFrameworkWithConfig(config)
```

## Algorithms

### Algorithm 1: Scoring with Network Topology

The scoring algorithm now considers network topology parameters:

```go
score = (pathLengthScore * 0.3 + podCountScore * 0.25 + 
         edgeCountScore * 0.15 + rpsScore * 0.3) * 0.7 +
        networkScore * 0.3
```

Where `networkScore` is calculated as:
```go
networkScore = bandwidthScore * 0.4 + hopScore * 0.3 + 
               distanceScore * 0.2 + azScore * 0.1
```

### Algorithm 2: Real-time Monitoring

Continuously monitors services and scales out bottleneck services:

```go
if detectLatencyViolation() {
    bottleneckServices := analyzeAndIdentifyBottleneck()
    if len(bottleneckServices) > 0 {
        call Algorithm 1  // Re-score paths
        scaleOut(bottleneckServices)
    }
}
```

### Algorithm 3: Affinity Rule Generator

Generates Kubernetes affinity rules considering network topology:

```go
for i := 0; i < len(services)-1; i++ {
    if i % 2 == 0 {  // Only for even indices
        generateAffinityRule(services[i], services[i+1], weight)
        applyNetworkTopologyAffinity(services[i], services[i+1])
    }
}
```

## Network Topology Analysis

The framework provides comprehensive network topology analysis:

```go
analysis, err := leadFramework.GetNetworkTopologyAnalysis()
if err == nil {
    fmt.Printf("Total Paths: %d\n", analysis.TotalPaths)
    fmt.Printf("Average Bandwidth: %.2f Mbps\n", analysis.AvgBandwidth)
    fmt.Printf("Average Hops: %.2f\n", analysis.AvgHops)
    fmt.Printf("Average Geo Distance: %.2f km\n", analysis.AvgGeoDistance)
    
    for az, count := range analysis.AvailabilityZones {
        fmt.Printf("Availability Zone %s: %d services\n", az, count)
    }
}
```

## Generated Kubernetes Manifests

The framework generates optimized Kubernetes deployment and service manifests:

### Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: hotel-reservation
spec:
  replicas: 3
  template:
    spec:
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: search
              topologyKey: kubernetes.io/hostname
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            preference:
              matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values: ["us-west-1a"]
      nodeSelector:
        topology.kubernetes.io/zone: us-west-1a
```

## Monitoring Integration

### Prometheus Queries

The framework includes predefined Prometheus queries for monitoring:

```go
queries := &monitoring.PrometheusQueries{
    CPUUsage:     `rate(container_cpu_usage_seconds_total{pod=~"%s-.*"}[5m]) * 100`,
    MemoryUsage:  `(container_memory_usage_bytes{pod=~"%s-.*"} / container_spec_memory_limit_bytes{pod=~"%s-.*"}) * 100`,
    RequestRate:  `rate(http_requests_total{pod=~"%s-.*"}[5m])`,
    ErrorRate:    `rate(http_requests_total{pod=~"%s-.*",status=~"5.."}[5m]) / rate(http_requests_total{pod=~"%s-.*"}[5m]) * 100`,
    ResponseTime: `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{pod=~"%s-.*"}[5m])) * 1000`,
    Latency:      `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{pod=~"%s-.*"}[5m])) * 1000`,
}
```

## Testing

Run the test suite:

```bash
go test ./tests/...
```

Run benchmarks:

```bash
go test -bench=. ./tests/...
```

## Examples

See the `examples/` directory for comprehensive examples:

- `hotel_reservation.go` - Complete hotel reservation service mesh example
- `tests/` - Comprehensive test suite with benchmarks

## Configuration

### Environment Variables

- `PROMETHEUS_URL`: Prometheus server URL (default: http://localhost:9090)
- `KUBERNETES_NAMESPACE`: Target Kubernetes namespace (default: default)
- `OUTPUT_DIRECTORY`: Directory for generated manifests (default: ./k8s-manifests)

### Configuration File

```json
{
  "monitoring_interval": "30s",
  "resource_threshold": 80.0,
  "latency_threshold": "100ms",
  "prometheus_url": "http://localhost:9090",
  "kubernetes_namespace": "default",
  "output_directory": "./k8s-manifests",
  "bandwidth_weight": 0.4,
  "hops_weight": 0.3,
  "geo_distance_weight": 0.2,
  "availability_zone_weight": 0.1
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## References

- LEAD Framework Paper: [Original LEAD Framework Research]
- Kubernetes Affinity Rules: [Kubernetes Documentation]
- Prometheus Monitoring: [Prometheus Documentation]
