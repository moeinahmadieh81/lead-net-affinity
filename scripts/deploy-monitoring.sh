#!/bin/bash

# Deploy Prometheus and Grafana for LEAD Scheduler monitoring
# This script sets up the complete monitoring stack

set -e

echo "📊 Deploying Prometheus and Grafana for LEAD Scheduler monitoring..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Deploy Prometheus
echo "🔍 Deploying Prometheus..."
kubectl apply -f k8s/prometheus-deployment.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy Prometheus"
    exit 1
fi

# Wait for Prometheus to be ready
echo "⏳ Waiting for Prometheus to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/prometheus -n monitoring

# Deploy Grafana
echo "📈 Deploying Grafana..."
kubectl apply -f k8s/grafana-deployment.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy Grafana"
    exit 1
fi

# Wait for Grafana to be ready
echo "⏳ Waiting for Grafana to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/grafana -n monitoring

# Check deployment status
echo "📊 Checking monitoring stack status..."
kubectl get pods -n monitoring

echo "✅ Monitoring stack deployed successfully!"
echo ""
echo "🔍 Access URLs:"
echo "  Prometheus: kubectl port-forward -n monitoring svc/prometheus 9090:9090"
echo "  Grafana:    kubectl port-forward -n monitoring svc/grafana 3000:3000"
echo ""
echo "📊 Grafana Login:"
echo "  Username: admin"
echo "  Password: admin"
echo ""
echo "📈 Key Metrics to Monitor:"
echo "  - lead_scheduler_decisions_total"
echo "  - lead_scheduler_discovery_duration_seconds"
echo "  - lead_scheduler_scoring_duration_seconds"
echo "  - lead_scheduler_services_discovered_total"
echo "  - lead_scheduler_network_analysis_total"
echo "  - lead_scheduler_affinity_rules_generated_total"
echo "  - lead_scheduler_errors_total"
echo ""
echo "🧪 To test Prometheus queries:"
echo "  kubectl port-forward -n monitoring svc/prometheus 9090:9090"
echo "  curl 'http://localhost:9090/api/v1/query?query=up{job=\"lead-scheduler\"}'"
