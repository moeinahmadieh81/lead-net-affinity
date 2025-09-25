#!/bin/bash

# Deploy Hotel Reservation test workloads
# This script deploys all microservices from the DeathStarBench HotelReservation benchmark

set -e

echo "🏨 Deploying Hotel Reservation test workloads..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Deploy all microservices
echo "📦 Deploying frontend service..."
kubectl apply -f k8s/microservices/frontend/

echo "👤 Deploying user service..."
kubectl apply -f k8s/microservices/user/

echo "👥 Deploying profile service..."
kubectl apply -f k8s/microservices/profile/

echo "🏠 Deploying reservation service..."
kubectl apply -f k8s/microservices/reservation/

echo "💡 Deploying recommendation service..."
kubectl apply -f k8s/microservices/recommendation/

echo "🌍 Deploying geo service..."
kubectl apply -f k8s/microservices/geo/

echo "💰 Deploying rate service..."
kubectl apply -f k8s/microservices/rate/

echo "🔍 Deploying search service..."
kubectl apply -f k8s/microservices/search/

# Wait for deployments to be ready
echo "⏳ Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/frontend
kubectl wait --for=condition=available --timeout=300s deployment/user
kubectl wait --for=condition=available --timeout=300s deployment/profile
kubectl wait --for=condition=available --timeout=300s deployment/reservation
kubectl wait --for=condition=available --timeout=300s deployment/recommendation
kubectl wait --for=condition=available --timeout=300s deployment/geo
kubectl wait --for=condition=available --timeout=300s deployment/rate
kubectl wait --for=condition=available --timeout=300s deployment/search

# Check deployment status
echo "📊 Checking deployment status..."
kubectl get pods -o wide

echo "✅ Hotel Reservation workloads deployed successfully!"
echo ""
echo "🔍 To monitor pod scheduling:"
echo "  kubectl get pods -o wide -w"
echo ""
echo "📈 To check scheduler decisions:"
echo "  kubectl get events --sort-by='.lastTimestamp' | grep -i scheduler"
echo ""
echo "🧪 To run validation tests:"
echo "  ./scripts/validate-scheduling.sh"
