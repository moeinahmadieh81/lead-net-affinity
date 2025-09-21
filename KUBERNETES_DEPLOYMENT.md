# LEAD Framework - Kubernetes Deployment Guide

This guide explains how to deploy the LEAD framework with the HotelReservation benchmark to test the scheduler with real microservices, MongoDB, and Memcached components.

## Overview

The LEAD framework now supports **dynamic service discovery** from running Kubernetes pods. It automatically:

1. **Discovers services** from deployed pods
2. **Builds dependency graphs** based on the HotelReservation benchmark
3. **Monitors network topology** and pod placement
4. **Generates optimized affinity rules** for service co-location
5. **Continuously adapts** as pods are added/removed

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────────┤
│  LEAD Framework Pod                                        │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  • Service Discovery (Watches pods)                │   │
│  │  • Scoring Algorithm (Network topology aware)      │   │
│  │  • Monitoring Algorithm (Real-time scaling)        │   │
│  │  • Affinity Generator (K8s affinity rules)         │   │
│  │  • HTTP API (Status, paths, health)                │   │
│  └─────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│  HotelReservation Microservices                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ Frontend    │ │ Search      │ │ User        │          │
│  │ (Gateway)   │ │             │ │             │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ Profile     │ │ Rate        │ │ Geo         │          │
│  │ + MongoDB   │ │ + MongoDB   │ │ + MongoDB   │          │
│  │ + Memcached │ │ + Memcached │ │             │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
│  ┌─────────────┐ ┌─────────────┐                          │
│  │ Reservation │ │ Recommendation│                        │
│  │ + MongoDB   │ │ + MongoDB   │                          │
│  │ + Memcached │ │             │                          │
│  └─────────────┘ └─────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

## Prerequisites

- Kubernetes cluster (v1.20+)
- kubectl configured
- Docker (for building images)
- RBAC enabled cluster

## Quick Start

### 1. Build and Deploy LEAD Framework

```bash
# Build the LEAD framework
./scripts/build.sh

# Deploy to Kubernetes
./scripts/deploy.sh
```

### 2. Deploy HotelReservation Benchmark

```bash
# Deploy all microservices with MongoDB and Memcached
kubectl apply -f k8s/microservices/

# Check deployment status
kubectl get pods -l managed-by=lead-framework
```

### 3. Monitor LEAD Framework

```bash
# Check LEAD framework status
kubectl get pods -n lead-framework

# View logs
kubectl logs -f deployment/lead-framework -n lead-framework

# Access API
kubectl port-forward svc/lead-framework 8080:80 -n lead-framework
```

### 4. Test the System

```bash
# Run comprehensive tests
./scripts/test.sh

# Manual API testing
curl http://localhost:8080/status
curl http://localhost:8080/paths
curl http://localhost:8080/network-topology
```

## Detailed Deployment Steps

### Step 1: Prepare the Environment

```bash
# Clone and navigate to the project
cd lead-framework

# Make scripts executable
chmod +x scripts/*.sh
```

### Step 2: Configure LEAD Framework

Edit `k8s/configmap.yaml` to match your cluster:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lead-config
  namespace: lead-framework
data:
  prometheus_url: "http://prometheus.monitoring.svc.cluster.local:9090"
  kubernetes_namespace: "default"  # Change to your target namespace
  monitoring_interval: "30s"
  resource_threshold: "80.0"
  latency_threshold: "100ms"
```

### Step 3: Deploy LEAD Framework

```bash
# Create namespace and RBAC
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml

# Create configuration
kubectl apply -f k8s/configmap.yaml

# Deploy LEAD framework
kubectl apply -f k8s/lead-deployment.yaml
kubectl apply -f k8s/lead-service.yaml

# Wait for deployment
kubectl wait --for=condition=available --timeout=300s deployment/lead-framework -n lead-framework
```

### Step 4: Deploy HotelReservation Services

Deploy services in the correct order to establish dependencies:

```bash
# 1. Deploy databases and caches first
kubectl apply -f k8s/microservices/profile/mongodb-profile-deployment.yaml
kubectl apply -f k8s/microservices/profile/memcached-profile-deployment.yaml
kubectl apply -f k8s/microservices/rate/mongodb-rate-deployment.yaml
kubectl apply -f k8s/microservices/rate/memcached-rate-deployment.yaml
kubectl apply -f k8s/microservices/user/mongodb-user-deployment.yaml
kubectl apply -f k8s/microservices/geo/mongodb-geo-deployment.yaml
kubectl apply -f k8s/microservices/recommendation/mongodb-recommendation-deployment.yaml
kubectl apply -f k8s/microservices/reservation/mongodb-reservation-deployment.yaml
kubectl apply -f k8s/microservices/reservation/memcached-reservation-deployment.yaml

# 2. Deploy microservices
kubectl apply -f k8s/microservices/profile/profile-deployment.yaml
kubectl apply -f k8s/microservices/rate/rate-deployment.yaml
kubectl apply -f k8s/microservices/user/user-deployment.yaml
kubectl apply -f k8s/microservices/geo/geo-deployment.yaml
kubectl apply -f k8s/microservices/search/search-deployment.yaml
kubectl apply -f k8s/microservices/recommendation/recommendation-deployment.yaml
kubectl apply -f k8s/microservices/reservation/reservation-deployment.yaml

# 3. Deploy frontend (gateway) last
kubectl apply -f k8s/microservices/frontend/frontend-deployment.yaml
```

### Step 5: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -o wide

# Check LEAD framework discovered services
kubectl logs deployment/lead-framework -n lead-framework | grep "Discovered"

# Check service graph
curl http://localhost:8080/status
```

## API Endpoints

Once deployed, LEAD framework provides these endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check |
| `/status` | GET | Framework status and discovered services |
| `/paths` | GET | Critical paths with scores |
| `/health-summary` | GET | Cluster health metrics |
| `/network-topology` | GET | Network topology analysis |
| `/reanalyze` | POST | Trigger re-analysis |

### Example API Usage

```bash
# Get framework status
curl http://localhost:8080/status | jq

# Get critical paths
curl http://localhost:8080/paths | jq

# Get network topology analysis
curl http://localhost:8080/network-topology | jq

# Trigger re-analysis
curl -X POST http://localhost:8080/reanalyze
```

## Dynamic Service Discovery

LEAD framework automatically discovers services by:

1. **Watching pod events** in the target namespace
2. **Grouping pods** by service name
3. **Determining service types** (microservice, MongoDB, Memcached)
4. **Building dependency graphs** based on HotelReservation architecture
5. **Updating network topology** as pods are scheduled

### Service Type Detection

- **Microservices**: `frontend`, `search`, `user`, etc.
- **MongoDB**: Pods with `mongodb-*` in name or labels
- **Memcached**: Pods with `memcached-*` in name or labels

### Network Topology Estimation

- **Availability Zone**: Extracted from node labels
- **Bandwidth**: Estimated based on service type and resources
- **Hops**: Calculated based on service dependencies
- **Geo Distance**: Estimated based on availability zones

## Monitoring and Scaling

### Real-time Monitoring

LEAD framework continuously monitors:

- **Pod resource usage** (CPU, memory)
- **Network latency** between services
- **Service health** and error rates
- **Bottleneck detection** and automatic scaling

### Scaling Events

When bottlenecks are detected:

1. **Identifies bottleneck services**
2. **Triggers re-scoring** of critical paths
3. **Scales out services** automatically
4. **Updates affinity rules** for optimal placement

## Troubleshooting

### Common Issues

1. **Services not discovered**
   ```bash
   # Check if pods are running
   kubectl get pods
   
   # Check LEAD framework logs
   kubectl logs deployment/lead-framework -n lead-framework
   ```

2. **Permission denied**
   ```bash
   # Ensure RBAC is applied
   kubectl apply -f k8s/rbac.yaml
   
   # Check service account
   kubectl get serviceaccount lead-framework -n lead-framework
   ```

3. **Prometheus connection failed**
   ```bash
   # Check if Prometheus is running
   kubectl get pods -n monitoring
   
   # Update ConfigMap with correct URL
   kubectl edit configmap lead-config -n lead-framework
   ```

### Debug Commands

```bash
# Check LEAD framework events
kubectl get events -n lead-framework

# Check pod events
kubectl get events --field-selector involvedObject.kind=Pod

# Check service discovery logs
kubectl logs deployment/lead-framework -n lead-framework | grep "Pod event"

# Check network topology
kubectl logs deployment/lead-framework -n lead-framework | grep "Network topology"
```

## Cleanup

```bash
# Remove all resources
./scripts/cleanup.sh

# Or manual cleanup
kubectl delete -f k8s/microservices/
kubectl delete -f k8s/lead-deployment.yaml
kubectl delete -f k8s/lead-service.yaml
kubectl delete -f k8s/configmap.yaml
kubectl delete -f k8s/rbac.yaml
kubectl delete -f k8s/namespace.yaml
```

## Performance Testing

### Load Testing

```bash
# Generate load on frontend
kubectl run load-test --image=busybox --rm -it --restart=Never -- /bin/sh
# Inside the pod:
wget -qO- http://frontend.default.svc.cluster.local:5000/

# Monitor LEAD framework response
curl http://localhost:8080/health-summary
```

### Scaling Test

```bash
# Scale a service manually
kubectl scale deployment profile --replicas=3

# Check LEAD framework response
kubectl logs deployment/lead-framework -n lead-framework | grep "scaling"
```

## Next Steps

1. **Integrate with Prometheus** for real metrics
2. **Add custom metrics** for your services
3. **Configure alerting** for bottleneck detection
4. **Tune network topology** weights for your environment
5. **Monitor performance** improvements

## Support

For issues or questions:

1. Check the logs: `kubectl logs deployment/lead-framework -n lead-framework`
2. Verify pod discovery: `kubectl get pods -o wide`
3. Test API endpoints: `curl http://localhost:8080/status`
4. Review service dependencies in the logs
