#!/bin/bash

# Test script for LEAD Framework
set -e

echo "🧪 Testing LEAD Framework..."

# Check if LEAD Framework is running
echo "🔍 Checking if LEAD Framework is running..."
if ! kubectl get deployment lead-framework -n lead-framework &> /dev/null; then
    echo "❌ LEAD Framework deployment not found"
    echo "Please run ./scripts/deploy.sh first"
    exit 1
fi

# Wait for LEAD Framework to be ready
echo "⏳ Waiting for LEAD Framework to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/lead-framework -n lead-framework

# Set up port forwarding
echo "🌐 Setting up port forwarding..."
kubectl port-forward svc/lead-framework 8080:80 -n lead-framework &
PORT_FORWARD_PID=$!

# Wait for port forwarding to be ready
sleep 5

# Function to test endpoint
test_endpoint() {
    local endpoint=$1
    local description=$2
    
    echo "🔍 Testing $description..."
    response=$(curl -s -w "%{http_code}" -o /tmp/response.json "http://localhost:8080$endpoint")
    http_code="${response: -3}"
    
    if [ "$http_code" -eq 200 ]; then
        echo "✅ $description: OK"
        cat /tmp/response.json | jq . 2>/dev/null || cat /tmp/response.json
        echo ""
    else
        echo "❌ $description: Failed (HTTP $http_code)"
        cat /tmp/response.json
        echo ""
    fi
}

# Test endpoints
echo "🚀 Running API tests..."
echo ""

test_endpoint "/health" "Health Check"
test_endpoint "/ready" "Readiness Check"
test_endpoint "/status" "Framework Status"
test_endpoint "/paths" "Critical Paths"
test_endpoint "/health-summary" "Cluster Health Summary"
test_endpoint "/network-topology" "Network Topology Analysis"

# Test re-analysis endpoint
echo "🔄 Testing re-analysis endpoint..."
response=$(curl -s -w "%{http_code}" -X POST -o /tmp/response.json "http://localhost:8080/reanalyze")
http_code="${response: -3}"

if [ "$http_code" -eq 200 ]; then
    echo "✅ Re-analysis: OK"
    cat /tmp/response.json | jq . 2>/dev/null || cat /tmp/response.json
    echo ""
else
    echo "❌ Re-analysis: Failed (HTTP $http_code)"
    cat /tmp/response.json
    echo ""
fi

# Check microservices
echo "🔍 Checking microservices..."
kubectl get pods -n lead-framework -l managed-by=lead-framework

echo ""
echo "📊 Deployment Summary:"
kubectl get all -n lead-framework

# Clean up
kill $PORT_FORWARD_PID 2>/dev/null || true
rm -f /tmp/response.json

echo ""
echo "✅ Testing completed!"
echo ""
echo "📋 To manually test:"
echo "  kubectl port-forward svc/lead-framework 8080:80 -n lead-framework"
echo "  curl http://localhost:8080/status"
