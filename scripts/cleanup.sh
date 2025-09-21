#!/bin/bash

# Cleanup script for LEAD Framework
set -e

echo "🧹 Cleaning up LEAD Framework deployment..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Confirm deletion
read -p "⚠️  This will delete all LEAD Framework resources. Are you sure? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cleanup cancelled"
    exit 1
fi

echo "🗑️  Deleting LEAD Framework resources..."

# Delete microservices
echo "🔧 Deleting microservices..."
kubectl delete -f k8s/microservices/ --ignore-not-found=true

# Delete LEAD framework
echo "🎯 Deleting LEAD Framework..."
kubectl delete -f k8s/lead-deployment.yaml --ignore-not-found=true
kubectl delete -f k8s/lead-service.yaml --ignore-not-found=true

# Delete ConfigMap
echo "⚙️  Deleting ConfigMap..."
kubectl delete -f k8s/configmap.yaml --ignore-not-found=true

# Delete RBAC
echo "🔐 Deleting RBAC..."
kubectl delete -f k8s/rbac.yaml --ignore-not-found=true

# Delete namespace (this will delete everything in the namespace)
echo "📁 Deleting namespace..."
kubectl delete -f k8s/namespace.yaml --ignore-not-found=true

# Wait for namespace to be deleted
echo "⏳ Waiting for namespace deletion..."
kubectl wait --for=delete namespace/lead-framework --timeout=60s || true

echo "✅ Cleanup completed successfully!"
echo ""
echo "📋 To verify cleanup:"
echo "  kubectl get all -n lead-framework"
echo "  kubectl get namespace lead-framework"
