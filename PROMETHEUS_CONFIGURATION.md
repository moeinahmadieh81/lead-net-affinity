# Prometheus Configuration for LEAD Scheduler

This document explains the Prometheus configuration needed for monitoring the LEAD scheduler in production.

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   LEAD Scheduler │    │   Prometheus    │    │     Grafana     │
│                 │    │                 │    │                 │
│  Port: 10259    │───▶│  Port: 9090     │───▶│  Port: 3000     │
│  /metrics       │    │  /api/v1/query  │    │  /dashboards    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 📊 LEAD Scheduler Metrics

The LEAD scheduler exposes the following metrics on port 10259:

### Core Scheduler Metrics
- `lead_scheduler_decisions_total` - Total scheduling decisions made
- `lead_scheduler_discovery_duration_seconds` - Service discovery duration histogram
- `lead_scheduler_scoring_duration_seconds` - Scoring algorithm duration histogram
- `lead_scheduler_services_discovered_total` - Total services discovered
- `lead_scheduler_network_analysis_total` - Network topology analysis count
- `lead_scheduler_affinity_rules_generated_total` - Affinity rules generated count
- `lead_scheduler_errors_total` - Total errors encountered

### Performance Metrics
- `lead_scheduler_scheduling_duration_seconds` - Overall scheduling duration
- `lead_scheduler_queue_size` - Current scheduling queue size
- `lead_scheduler_active_goroutines` - Number of active goroutines

## 🔧 Configuration Files

### 1. Prometheus Configuration (`k8s/prometheus-deployment.yaml`)

```yaml
scrape_configs:
  # LEAD Scheduler metrics
  - job_name: 'lead-scheduler'
    static_configs:
      - targets: ['lead-scheduler.kube-system.svc.cluster.local:10259']
    metrics_path: '/metrics'
    scrape_interval: 15s
    scrape_timeout: 10s
```

### 2. LEAD Scheduler Environment Variables

```yaml
env:
- name: PROMETHEUS_URL
  value: "http://prometheus.monitoring.svc.cluster.local:9090"
- name: MONITORING_INTERVAL
  value: "15s"
- name: RESOURCE_THRESHOLD
  value: "80.0"
- name: LATENCY_THRESHOLD
  value: "150ms"
```

## 🌐 **Service Addresses vs Localhost**

### **When to Use Service Addresses (Inside Cluster)**
- **Application configuration** (environment variables)
- **Inter-service communication** within the cluster
- **Prometheus scraping configuration**

```yaml
# ✅ CORRECT - Use Kubernetes service DNS
PROMETHEUS_URL: "http://prometheus.monitoring.svc.cluster.local:9090"
```

### **When to Use Localhost (Outside Cluster)**
- **Port-forwarding** for external access
- **Testing from your local machine**
- **Development and debugging**

```bash
# ✅ CORRECT - Use localhost with port-forward
kubectl port-forward -n monitoring svc/prometheus 9090:9090
curl http://localhost:9090/api/v1/query?query=up
```

## 🚀 Deployment Steps

### Step 1: Deploy Monitoring Stack
```bash
# Deploy Prometheus and Grafana
./scripts/deploy-monitoring.sh
```

### Step 2: Deploy LEAD Scheduler
```bash
# Deploy LEAD scheduler (will ask about monitoring)
./scripts/deploy-lead-scheduler.sh
```

### Step 3: Verify Metrics Collection
```bash
# Check if Prometheus is scraping LEAD scheduler
# First port-forward to access Prometheus from outside the cluster
kubectl port-forward -n monitoring svc/prometheus 9090:9090
curl 'http://localhost:9090/api/v1/query?query=up{job="lead-scheduler"}'
```

## 📈 Key Prometheus Queries

### Scheduler Health
```promql
# Scheduler up/down status
up{job="lead-scheduler"}

# Scheduler error rate
rate(lead_scheduler_errors_total[5m])
```

### Performance Metrics
```promql
# Scheduling decisions per second
rate(lead_scheduler_decisions_total[5m])

# 95th percentile discovery duration
histogram_quantile(0.95, rate(lead_scheduler_discovery_duration_seconds_bucket[5m]))

# 95th percentile scoring duration
histogram_quantile(0.95, rate(lead_scheduler_scoring_duration_seconds_bucket[5m]))
```

### Service Discovery
```promql
# Total services discovered
lead_scheduler_services_discovered_total

# Network analysis rate
rate(lead_scheduler_network_analysis_total[5m])

# Affinity rules generation rate
rate(lead_scheduler_affinity_rules_generated_total[5m])
```

### Hotel Reservation Services
```promql
# Service availability
up{job="hotel-reservation-services"}

# Service response time
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{job="hotel-reservation-services"}[5m]))

# Service error rate
rate(http_requests_total{job="hotel-reservation-services",code=~"5.."}[5m]) / rate(http_requests_total{job="hotel-reservation-services"}[5m])
```

## 🚨 Alerting Rules

### Critical Alerts
```yaml
- alert: LeadSchedulerDown
  expr: up{job="lead-scheduler"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "LEAD Scheduler is down"

- alert: LeadSchedulerHighErrorRate
  expr: rate(lead_scheduler_errors_total[5m]) > 0.1
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "LEAD Scheduler high error rate"
```

### Performance Alerts
```yaml
- alert: LeadSchedulerHighLatency
  expr: histogram_quantile(0.95, rate(lead_scheduler_scheduling_duration_seconds_bucket[5m])) > 1
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "LEAD Scheduler high latency"
```

## 📊 Grafana Dashboard

### Pre-configured Dashboard
The Grafana deployment includes a pre-configured dashboard with:

1. **Scheduler Status** - Up/down status and health
2. **Performance Metrics** - Decisions/sec, discovery duration, scoring duration
3. **Service Discovery** - Services discovered, network analysis, affinity rules
4. **Error Monitoring** - Error rates and types
5. **Hotel Reservation Services** - Service status and performance
6. **Pod Distribution** - Pod placement across nodes

### Access Grafana
```bash
# Port-forward to access Grafana from outside the cluster
kubectl port-forward -n monitoring svc/grafana 3000:3000
# Open browser to http://localhost:3000
# Username: admin
# Password: admin
```

## 🔍 Troubleshooting

### Metrics Not Appearing
```bash
# Check if LEAD scheduler is exposing metrics
# Port-forward to access LEAD scheduler metrics from outside the cluster
kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259
curl http://localhost:10259/metrics

# Check Prometheus targets
# Port-forward to access Prometheus from outside the cluster
kubectl port-forward -n monitoring svc/prometheus 9090:9090
curl 'http://localhost:9090/api/v1/targets'
```

### Prometheus Connection Issues
```bash
# Check Prometheus logs
kubectl logs -n monitoring -l app=prometheus

# Check LEAD scheduler logs
kubectl logs -n kube-system -l app=lead-scheduler

# Verify network connectivity
kubectl exec -n kube-system deployment/lead-scheduler -- nslookup prometheus.monitoring.svc.cluster.local
```

### Grafana Dashboard Issues
```bash
# Check Grafana logs
kubectl logs -n monitoring -l app=grafana

# Verify datasource configuration
kubectl get configmap grafana-datasources -n monitoring -o yaml
```

## 📚 Advanced Configuration

### Custom Metrics
To add custom metrics to the LEAD scheduler:

1. **Add metric definition** in the scheduler code
2. **Update Prometheus queries** in the configuration
3. **Add Grafana panels** for visualization
4. **Create alerting rules** if needed

### Scaling Prometheus
For production environments with high metric volume:

```yaml
# Increase retention time
- '--storage.tsdb.retention.time=30d'

# Increase storage size
resources:
  requests:
    memory: 2Gi
  limits:
    memory: 4Gi
```

### External Prometheus
To use an external Prometheus instance:

1. **Update PROMETHEUS_URL** in LEAD scheduler deployment
2. **Configure scrape config** in external Prometheus
3. **Update Grafana datasource** to point to external Prometheus

## 🎯 Best Practices

1. **Monitor Key Metrics**: Focus on scheduler health, performance, and error rates
2. **Set Appropriate Alerts**: Configure alerts for critical issues
3. **Regular Dashboard Review**: Check dashboards regularly for anomalies
4. **Retention Policy**: Set appropriate retention for metrics storage
5. **Resource Limits**: Set proper resource limits for Prometheus and Grafana

## 🔗 Useful Links

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Kubernetes Monitoring](https://kubernetes.io/docs/tasks/debug-application-cluster/resource-usage-monitoring/)
- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
