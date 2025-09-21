#!/bin/bash

# Build script for LEAD Framework
set -e

echo "🏗️  Building LEAD Framework..."

# Build the Go application
echo "📦 Building Go application..."
go build -o lead-framework main.go

# Build Docker image
echo "🐳 Building Docker image..."
docker build -t lead-framework:latest .

# Tag for registry (optional)
if [ ! -z "$REGISTRY" ]; then
    echo "🏷️  Tagging image for registry: $REGISTRY"
    docker tag lead-framework:latest $REGISTRY/lead-framework:latest
fi

echo "✅ Build completed successfully!"
echo ""
echo "📋 Next steps:"
echo "  1. Push image to registry (if needed):"
echo "     docker push $REGISTRY/lead-framework:latest"
echo ""
echo "  2. Deploy to Kubernetes:"
echo "     ./scripts/deploy.sh"
echo ""
echo "  3. Check deployment status:"
echo "     kubectl get pods -n lead-framework"
