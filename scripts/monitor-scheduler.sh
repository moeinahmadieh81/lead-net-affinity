#!/bin/bash

# Monitor LEAD Scheduler in real-time
# This script provides real-time monitoring of the LEAD scheduler behavior

set -e

echo "📊 LEAD Scheduler Real-time Monitor"
echo "=================================="

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Function to show scheduler status
show_scheduler_status() {
    echo "🚀 Scheduler Status:"
    kubectl get pods -n kube-system -l app=lead-scheduler
    echo ""
}

# Function to show recent scheduler logs
show_recent_logs() {
    echo "📝 Recent Scheduler Logs (last 20 lines):"
    kubectl logs -n kube-system -l app=lead-scheduler --tail=20
    echo ""
}

# Function to show pod distribution
show_pod_distribution() {
    echo "📊 Current Pod Distribution:"
    kubectl get pods -o wide --no-headers | awk '{print $1, $7}' | sort -k2
    echo ""
}

# Function to show scheduler events
show_scheduler_events() {
    echo "📅 Recent Scheduler Events:"
    kubectl get events --sort-by='.lastTimestamp' | grep -i lead | tail -5
    echo ""
}

# Function to show service discovery status
show_service_discovery() {
    echo "🔍 Service Discovery Status:"
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=50)
    if echo "$logs" | grep -q "Discovered.*services"; then
        echo "$logs" | grep "Discovered.*services" | tail -1
    else
        echo "No service discovery messages found"
    fi
    echo ""
}

# Function to show network topology analysis
show_network_topology() {
    echo "🌐 Network Topology Analysis:"
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=50)
    if echo "$logs" | grep -q -E "(Network topology|Geo distance|Bandwidth)"; then
        echo "$logs" | grep -E "(Network topology|Geo distance|Bandwidth)" | tail -3
    else
        echo "No network topology analysis found"
    fi
    echo ""
}

# Function to show affinity rules
show_affinity_rules() {
    echo "🔗 Affinity Rules Generation:"
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=50)
    if echo "$logs" | grep -q -E "(Affinity rules|Co-location)"; then
        echo "$logs" | grep -E "(Affinity rules|Co-location)" | tail -3
    else
        echo "No affinity rules generation found"
    fi
    echo ""
}

# Function to show scoring decisions
show_scoring_decisions() {
    echo "🎯 Scoring Decisions:"
    local logs=$(kubectl logs -n kube-system -l app=lead-scheduler --tail=50)
    if echo "$logs" | grep -q -E "(Score|Path|Critical path)"; then
        echo "$logs" | grep -E "(Score|Path|Critical path)" | tail -3
    else
        echo "No scoring decisions found"
    fi
    echo ""
}

# Function to show resource utilization
show_resource_utilization() {
    echo "💾 Resource Utilization:"
    if kubectl top nodes &> /dev/null; then
        kubectl top nodes
        echo ""
        kubectl top pods
    else
        echo "Metrics server not available"
    fi
    echo ""
}

# Main monitoring loop
monitor_loop() {
    while true; do
        clear
        echo "📊 LEAD Scheduler Real-time Monitor - $(date)"
        echo "=============================================="
        echo ""
        
        show_scheduler_status
        show_pod_distribution
        show_service_discovery
        show_network_topology
        show_affinity_rules
        show_scoring_decisions
        show_scheduler_events
        show_resource_utilization
        
        echo "Press Ctrl+C to exit"
        echo "Refreshing in 10 seconds..."
        sleep 10
    done
}

# Check if we should run in continuous mode
if [ "$1" = "--continuous" ] || [ "$1" = "-c" ]; then
    monitor_loop
else
    # Show current status once
    show_scheduler_status
    show_pod_distribution
    show_service_discovery
    show_network_topology
    show_affinity_rules
    show_scoring_decisions
    show_scheduler_events
    show_resource_utilization
    
    echo "💡 To run in continuous mode:"
    echo "  ./scripts/monitor-scheduler.sh --continuous"
    echo ""
    echo "📝 To follow scheduler logs:"
    echo "  kubectl logs -n kube-system -l app=lead-scheduler -f"
    echo ""
    echo "📈 To check scheduler metrics:"
    echo "  kubectl port-forward -n kube-system svc/lead-scheduler 10259:10259"
    echo "  curl http://localhost:10259/metrics"
fi
