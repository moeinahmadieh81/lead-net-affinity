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

# Delete LEAD scheduler
echo "🎯 Deleting LEAD Scheduler..."
kubectl delete -f k8s/lead-deployment.yaml --ignore-not-found=true

# Delete scheduler config
echo "⚙️  Deleting scheduler config..."
kubectl delete -f k8s/scheduler-config.yaml --ignore-not-found=true

# Delete RBAC
echo "🔐 Deleting RBAC..."
kubectl delete -f k8s/rbac.yaml --ignore-not-found=true

echo "✅ Cleanup completed successfully!"
echo ""
echo "📋 To verify cleanup:"
echo "  kubectl get pods -n kube-system | grep lead-scheduler"
