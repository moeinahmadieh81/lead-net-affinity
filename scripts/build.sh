#!/bin/bash

# Build script for LEAD Framework
set -e

echo "🏗️  Building LEAD Framework..."

# Build the scheduler binary
echo "📦 Building LEAD Scheduler..."
go build -o lead-scheduler ./cmd/scheduler

# Build Docker image
echo "🐳 Building Docker image..."
docker build -t lead-scheduler:latest .

# Tag for registry (optional)
if [ ! -z "$REGISTRY" ]; then
    echo "🏷️  Tagging image for registry: $REGISTRY"
    docker tag lead-scheduler:latest $REGISTRY/lead-scheduler:latest
fi

echo "✅ Build completed successfully!"
echo ""
echo "📋 Next steps:"
echo "  1. Push image to registry (if needed):"
echo "     docker push $REGISTRY/lead-scheduler:latest"
echo ""
echo "  2. Deploy to Kubernetes:"
echo "     ./scripts/deploy.sh"
echo ""
echo "  3. Check deployment status:"
echo "     kubectl get pods -n kube-system | grep lead-scheduler"
