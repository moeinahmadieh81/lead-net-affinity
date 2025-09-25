#!/bin/bash

# Deployment script for LEAD Framework
set -e

echo "🚀 Deploying LEAD Framework to Kubernetes..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster"
    echo "Please ensure kubectl is configured correctly"
    exit 1
fi

echo "✅ Kubernetes cluster is accessible"

# Apply RBAC
echo "🔐 Setting up RBAC..."
kubectl apply -f k8s/rbac.yaml

# Deploy LEAD scheduler
echo "🎯 Deploying LEAD Scheduler..."
kubectl apply -f k8s/scheduler-config.yaml
kubectl apply -f k8s/lead-deployment.yaml

# Deploy microservices
echo "🔧 Deploying microservices..."
kubectl apply -f k8s/microservices/

# Wait for deployments to be ready
echo "⏳ Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/lead-scheduler -n kube-system

echo "✅ LEAD Scheduler deployed successfully!"
echo ""
echo "📋 Useful commands:"
echo "  Check scheduler status:"
echo "    kubectl get pods -n kube-system | grep lead-scheduler"
echo ""
echo "  View scheduler logs:"
echo "    kubectl logs -f deployment/lead-scheduler -n kube-system"
echo ""
echo "  Access scheduler metrics:"
echo "    kubectl port-forward svc/lead-scheduler 10259:10259 -n kube-system"
echo ""
echo "  Check scheduler health:"
echo "    curl http://localhost:10259/healthz"
echo ""
echo "  View scheduler metrics:"
echo "    curl http://localhost:10259/metrics"
