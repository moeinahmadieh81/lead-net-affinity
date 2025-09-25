#!/bin/bash

# Deploy LEAD Scheduler to Kubernetes
# This script builds and deploys the LEAD scheduler for production testing

set -e

echo "🚀 Deploying LEAD Scheduler to Kubernetes..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ docker is not installed or not in PATH"
    exit 1
fi

# Build the LEAD scheduler binary
echo "📦 Building LEAD scheduler binary..."
go build -o lead-scheduler ./cmd/scheduler
if [ $? -ne 0 ]; then
    echo "❌ Failed to build LEAD scheduler binary"
    exit 1
fi

# Build Docker image
echo "🐳 Building Docker image..."
docker build -t lead-scheduler:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build Docker image"
    exit 1
fi

# Deploy RBAC
echo "🔐 Deploying RBAC configuration..."
kubectl apply -f k8s/rbac.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy RBAC"
    exit 1
fi

# Deploy scheduler configuration
echo "⚙️ Deploying scheduler configuration..."
kubectl apply -f k8s/scheduler-config.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy scheduler configuration"
    exit 1
fi

# Deploy LEAD scheduler
echo "🚀 Deploying LEAD scheduler..."
kubectl apply -f k8s/lead-deployment.yaml
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy LEAD scheduler"
    exit 1
fi

# Wait for deployment to be ready
echo "⏳ Waiting for LEAD scheduler to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/lead-scheduler -n kube-system

# Ask if user wants to deploy monitoring stack
echo ""
read -p "Do you want to deploy Prometheus and Grafana for monitoring? (y/n): " deploy_monitoring

if [ "$deploy_monitoring" = "y" ] || [ "$deploy_monitoring" = "Y" ]; then
    echo "📊 Deploying monitoring stack..."
    ./scripts/deploy-monitoring.sh
fi

# Check deployment status
echo "📊 Checking deployment status..."
kubectl get pods -n kube-system -l app=lead-scheduler

echo "✅ LEAD Scheduler deployed successfully!"
echo ""
echo "🔍 To monitor the scheduler:"
echo "  kubectl logs -n kube-system -l app=lead-scheduler -f"
echo ""
echo "📈 To check metrics:"
echo "  kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259"
echo "  curl http://localhost:10259/metrics"
echo ""
echo "🧪 To deploy test workloads:"
echo "  ./scripts/deploy-test-workloads.sh"
