# LEAD Scheduler Production Testing Guide

This guide provides step-by-step instructions for testing the LEAD scheduler in a production Kubernetes environment using your existing YAML files.

## 🏗️ Current Setup

You have:
- ✅ **LEAD Scheduler**: `k8s/lead-deployment.yaml` + `k8s/rbac.yaml` + `k8s/scheduler-config.yaml`
- ✅ **Hotel Reservation Microservices**: Complete DeathStarBench setup in `k8s/microservices/`
- ✅ **RBAC Configuration**: Proper permissions for the scheduler
- ✅ **Scheduler Configuration**: LEAD plugin configuration

## 🚀 Production Testing Steps

### Step 1: Build and Deploy LEAD Scheduler

```bash
# 1. Build the LEAD scheduler binary
cd /Users/moein/Desktop/Lead_Framework_Extended
go build -o lead-scheduler ./cmd/scheduler

# 2. Build Docker image
docker build -t lead-scheduler:latest .

# 3. Deploy LEAD scheduler to Kubernetes (includes monitoring option)
./scripts/deploy-lead-scheduler.sh
```

### Step 1.5: Deploy Monitoring Stack (Optional)

```bash
# Deploy Prometheus and Grafana for monitoring
./scripts/deploy-monitoring.sh
```

### Step 2: Verify Scheduler Deployment

```bash
# Check if scheduler is running
kubectl get pods -n kube-system -l app=lead-scheduler

# Check scheduler logs
kubectl logs -n kube-system -l app=lead-scheduler -f

# Check scheduler metrics
kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259
curl http://localhost:10259/metrics

# Check Prometheus metrics (if monitoring deployed)
kubectl port-forward -n monitoring svc/prometheus 9090:9090
curl 'http://localhost:9090/api/v1/query?query=up{job="lead-scheduler"}'
```

### Step 3: Deploy Hotel Reservation Microservices

```bash
# Deploy all microservices
kubectl apply -f k8s/microservices/frontend/
kubectl apply -f k8s/microservices/user/
kubectl apply -f k8s/microservices/profile/
kubectl apply -f k8s/microservices/reservation/
kubectl apply -f k8s/microservices/recommendation/
kubectl apply -f k8s/microservices/geo/
kubectl apply -f k8s/microservices/rate/
kubectl apply -f k8s/microservices/search/
```

### Step 4: Monitor Scheduler Behavior

```bash
# Watch pod scheduling
kubectl get pods -o wide -w

# Check scheduler decisions
kubectl get events --sort-by='.lastTimestamp' | grep -i scheduler

# Monitor LEAD scheduler logs
kubectl logs -n kube-system -l app=lead-scheduler -f
```

## 🧪 Testing Scenarios

### Scenario 1: Basic Service Discovery
**Goal**: Verify scheduler discovers all microservices

```bash
# Check if all services are discovered
kubectl logs -n kube-system -l app=lead-scheduler | grep "Discovered.*services"

# Expected output: Should show all 8+ services from HotelReservation
```

### Scenario 2: Geo-Distributed Scheduling
**Goal**: Test scheduling across multiple nodes/zones

```bash
# Add node labels for geo-distribution
kubectl label nodes <node1> topology.kubernetes.io/zone=europe-west1-a
kubectl label nodes <node2> topology.kubernetes.io/zone=europe-west1-b
kubectl label nodes <node3> topology.kubernetes.io/zone=europe-west1-c

# Scale up services to see distribution
kubectl scale deployment frontend --replicas=3
kubectl scale deployment user --replicas=2
```

### Scenario 3: Database Co-location
**Goal**: Verify databases are co-located with their microservices

```bash
# Check pod placement
kubectl get pods -o wide | grep -E "(user|mongodb-user)"
kubectl get pods -o wide | grep -E "(profile|mongodb-profile)"

# Should show pods on same nodes
```

### Scenario 4: Dynamic Scaling
**Goal**: Test scheduler behavior during scaling

```bash
# Scale up frontend
kubectl scale deployment frontend --replicas=5

# Watch scheduler logs for new pod scheduling decisions
kubectl logs -n kube-system -l app=lead-scheduler -f

# Scale down
kubectl scale deployment frontend --replicas=1
```

### Scenario 5: Node Failure Simulation
**Goal**: Test scheduler behavior during node failures

```bash
# Drain a node (simulate failure)
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data

# Watch pods being rescheduled
kubectl get pods -o wide -w

# Uncordon the node
kubectl uncordon <node-name>
```

## 📊 Validation Commands

### Check Service Graph Discovery
```bash
# Look for service discovery logs
kubectl logs -n kube-system -l app=lead-scheduler | grep -E "(Discovered|Service graph|Gateway)"
```

### Check Network Topology Analysis
```bash
# Look for topology analysis
kubectl logs -n kube-system -l app=lead-scheduler | grep -E "(Network topology|Geo distance|Bandwidth)"
```

### Check Affinity Rules Generation
```bash
# Look for affinity rule generation
kubectl logs -n kube-system -l app=lead-scheduler | grep -E "(Affinity rules|Co-location)"
```

### Check Scoring Decisions
```bash
# Look for scoring decisions
kubectl logs -n kube-system -l app=lead-scheduler | grep -E "(Score|Path|Critical path)"
```

## 🔍 Expected Behavior

### 1. Service Discovery
- Should discover: frontend, user, profile, reservation, recommendation, geo, rate, search
- Should discover databases: mongodb-user, mongodb-profile, mongodb-reservation, etc.
- Should discover memcached: memcached-profile, memcached-reservation, memcached-rate

### 2. Network Topology
- Should calculate geo-distances between services
- Should estimate bandwidth and hops
- Should identify availability zones

### 3. Scheduling Decisions
- Should prioritize geo-distance (40% weight)
- Should co-locate databases with microservices
- Should distribute frontend across multiple nodes
- Should consider resource constraints

### 4. Affinity Rules
- Should generate pod affinity for database co-location
- Should generate node affinity for geo-distribution
- Should generate anti-affinity for high availability

## 🚨 Troubleshooting

### Scheduler Not Starting
```bash
# Check RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:kube-system:lead-scheduler

# Check scheduler logs
kubectl logs -n kube-system -l app=lead-scheduler
```

### Services Not Discovered
```bash
# Check if services have proper labels
kubectl get pods --show-labels | grep io.kompose.service

# Check scheduler configuration
kubectl get configmap scheduler-config -n kube-system -o yaml
```

### Pods Not Scheduling
```bash
# Check scheduler events
kubectl get events --sort-by='.lastTimestamp' | grep -i lead

# Check node resources
kubectl describe nodes
```

## 📈 Performance Monitoring

### Scheduler Metrics
```bash
# Access metrics endpoint
kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259

# Key metrics to monitor:
# - lead_scheduler_decisions_total
# - lead_scheduler_discovery_duration_seconds
# - lead_scheduler_scoring_duration_seconds
# - lead_scheduler_services_discovered_total
# - lead_scheduler_network_analysis_total
# - lead_scheduler_affinity_rules_generated_total
# - lead_scheduler_errors_total
```

### Prometheus Monitoring
```bash
# Access Prometheus UI
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Key queries:
# - up{job="lead-scheduler"} - Scheduler health
# - rate(lead_scheduler_decisions_total[5m]) - Decisions per second
# - histogram_quantile(0.95, rate(lead_scheduler_discovery_duration_seconds_bucket[5m])) - 95th percentile discovery time
```

### Grafana Dashboards
```bash
# Access Grafana UI
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Login: admin/admin
# Pre-configured dashboard: "LEAD Scheduler Dashboard"
```

### Application Metrics
```bash
# Monitor application performance
kubectl top pods
kubectl top nodes

# Check resource utilization
kubectl describe nodes
```

## 🎯 Success Criteria

✅ **Scheduler Discovery**: All microservices and databases discovered  
✅ **Geo-Distribution**: Services distributed across multiple nodes/zones  
✅ **Database Co-location**: Databases scheduled with their microservices  
✅ **Dynamic Updates**: Scheduler responds to scaling events  
✅ **Performance**: Scheduling decisions made within acceptable time  
✅ **Reliability**: Scheduler handles node failures gracefully  

## 🔄 Next Steps

1. **Run the tests** following the scenarios above
2. **Monitor the logs** for expected behavior
3. **Validate scheduling decisions** match LEAD algorithm
4. **Test edge cases** like resource constraints and failures
5. **Measure performance** and compare with default scheduler

This setup will give you a complete production testing environment for the LEAD scheduler! 🚀
