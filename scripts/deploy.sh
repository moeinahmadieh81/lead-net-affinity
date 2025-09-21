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

# Create namespace first
echo "📁 Creating namespace..."
kubectl apply -f k8s/namespace.yaml

# Apply RBAC
echo "🔐 Setting up RBAC..."
kubectl apply -f k8s/rbac.yaml

# Apply ConfigMap
echo "⚙️  Creating ConfigMap..."
kubectl apply -f k8s/configmap.yaml

# Deploy LEAD framework
echo "🎯 Deploying LEAD Framework..."
kubectl apply -f k8s/lead-deployment.yaml
kubectl apply -f k8s/lead-service.yaml

# Deploy microservices
echo "🔧 Deploying microservices..."
kubectl apply -f k8s/microservices/

# Wait for deployments to be ready
echo "⏳ Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/lead-framework -n lead-framework

echo "✅ LEAD Framework deployed successfully!"
echo ""
echo "📋 Useful commands:"
echo "  Check pod status:"
echo "    kubectl get pods -n lead-framework"
echo ""
echo "  View LEAD Framework logs:"
echo "    kubectl logs -f deployment/lead-framework -n lead-framework"
echo ""
echo "  Access LEAD Framework API:"
echo "    kubectl port-forward svc/lead-framework 8080:80 -n lead-framework"
echo ""
echo "  Check framework status:"
echo "    curl http://localhost:8080/status"
echo ""
echo "  View critical paths:"
echo "    curl http://localhost:8080/paths"
echo ""
echo "  Monitor cluster health:"
echo "    curl http://localhost:8080/health-summary"
