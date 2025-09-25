#!/bin/bash

# Validate LEAD Scheduler behavior
# This script runs validation tests to ensure the scheduler is working correctly

set -e

echo "🧪 Validating LEAD Scheduler behavior..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Function to check service discovery
check_service_discovery() {
    echo "🔍 Checking service discovery..."
    
    # Get scheduler logs
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=100)
    
    # Check for service discovery messages
    if echo "$logs" | grep -q "Discovered.*services"; then
        echo "✅ Service discovery working"
        echo "$logs" | grep "Discovered.*services"
    else
        echo "❌ Service discovery not working"
        return 1
    fi
}

# Function to check network topology analysis
check_network_topology() {
    echo "🌐 Checking network topology analysis..."
    
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=100)
    
    if echo "$logs" | grep -q -E "(Network topology|Geo distance|Bandwidth)"; then
        echo "✅ Network topology analysis working"
        echo "$logs" | grep -E "(Network topology|Geo distance|Bandwidth)" | head -5
    else
        echo "❌ Network topology analysis not working"
        return 1
    fi
}

# Function to check affinity rules generation
check_affinity_rules() {
    echo "🔗 Checking affinity rules generation..."
    
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=100)
    
    if echo "$logs" | grep -q -E "(Affinity rules|Co-location)"; then
        echo "✅ Affinity rules generation working"
        echo "$logs" | grep -E "(Affinity rules|Co-location)" | head -3
    else
        echo "❌ Affinity rules generation not working"
        return 1
    fi
}

# Function to check pod distribution
check_pod_distribution() {
    echo "📊 Checking pod distribution..."
    
    # Get all pods with their nodes
    local pods=$(kubectl get pods -o wide --no-headers)
    local node_count=$(echo "$pods" | awk '{print $7}' | sort -u | wc -l)
    local total_pods=$(echo "$pods" | wc -l)
    
    echo "Total pods: $total_pods"
    echo "Nodes used: $node_count"
    
    if [ "$node_count" -gt 1 ]; then
        echo "✅ Pods distributed across multiple nodes"
    else
        echo "⚠️ All pods on single node (may be expected in small clusters)"
    fi
}

# Function to check database co-location
check_database_colocation() {
    echo "🗄️ Checking database co-location..."
    
    # Check user and mongodb-user co-location
    local user_pods=$(kubectl get pods -l io.kompose.service=user -o wide --no-headers | awk '{print $7}')
    local mongodb_user_pods=$(kubectl get pods -l io.kompose.service=mongodb-user -o wide --no-headers | awk '{print $7}')
    
    if [ "$user_pods" = "$mongodb_user_pods" ]; then
        echo "✅ User service co-located with mongodb-user"
    else
        echo "⚠️ User service not co-located with mongodb-user"
        echo "  User pods on: $user_pods"
        echo "  MongoDB pods on: $mongodb_user_pods"
    fi
}

# Function to check scheduler events
check_scheduler_events() {
    echo "📅 Checking scheduler events..."
    
    local events=$(kubectl get events --sort-by='.lastTimestamp' | grep -i lead | tail -5)
    
    if [ -n "$events" ]; then
        echo "✅ Scheduler events found"
        echo "$events"
    else
        echo "⚠️ No recent scheduler events found"
    fi
}

# Function to check resource utilization
check_resource_utilization() {
    echo "💾 Checking resource utilization..."
    
    # Check if metrics-server is available
    if kubectl top nodes &> /dev/null; then
        echo "📊 Node resource utilization:"
        kubectl top nodes
        echo ""
        echo "📊 Pod resource utilization:"
        kubectl top pods
    else
        echo "⚠️ Metrics server not available, skipping resource check"
    fi
}

# Run all validation checks
echo "Starting validation checks..."
echo "================================"

check_service_discovery
echo ""

check_network_topology
echo ""

check_affinity_rules
echo ""

check_pod_distribution
echo ""

check_database_colocation
echo ""

check_scheduler_events
echo ""

check_resource_utilization
echo ""

echo "================================"
echo "✅ Validation complete!"
echo ""
echo "🔍 To monitor scheduler in real-time:"
echo "  kubectl logs -n kube-system -l app=lead-scheduler -f"
echo ""
echo "📈 To check scheduler metrics:"
echo "  kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259"
echo "  curl http://localhost:10259/metrics"
