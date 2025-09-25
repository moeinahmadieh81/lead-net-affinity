#!/bin/bash

# Quick Start Production Testing for LEAD Scheduler
# This script provides a complete production testing workflow

set -e

echo "🚀 LEAD Scheduler Production Testing - Quick Start"
echo "=================================================="
echo ""

# Check prerequisites
echo "🔍 Checking prerequisites..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    echo "Please install kubectl and configure it to access your Kubernetes cluster"
    exit 1
fi

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ docker is not installed or not in PATH"
    echo "Please install docker to build the scheduler image"
    exit 1
fi

# Check if go is available
if ! command -v go &> /dev/null; then
    echo "❌ go is not installed or not in PATH"
    echo "Please install Go to build the scheduler binary"
    exit 1
fi

# Check kubectl connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster"
    echo "Please configure kubectl to access your cluster"
    exit 1
fi

echo "✅ All prerequisites met!"
echo ""

# Show current cluster info
echo "📊 Current cluster information:"
kubectl cluster-info
echo ""
kubectl get nodes
echo ""

# Ask user what they want to do
echo "What would you like to do?"
echo "1. Deploy LEAD Scheduler only"
echo "2. Deploy LEAD Scheduler + Test Workloads"
echo "3. Deploy Test Workloads only (scheduler already deployed)"
echo "4. Deploy Monitoring Stack (Prometheus + Grafana)"
echo "5. Monitor existing deployment"
echo "6. Run validation tests"
echo "7. Full production test (deploy + test + monitor)"
echo ""

read -p "Enter your choice (1-7): " choice

case $choice in
    1)
        echo "🚀 Deploying LEAD Scheduler..."
        ./scripts/deploy-lead-scheduler.sh
        ;;
    2)
        echo "🚀 Deploying LEAD Scheduler and Test Workloads..."
        ./scripts/deploy-lead-scheduler.sh
        echo ""
        echo "⏳ Waiting 30 seconds for scheduler to initialize..."
        sleep 30
        ./scripts/deploy-test-workloads.sh
        ;;
    3)
        echo "🏨 Deploying Test Workloads..."
        ./scripts/deploy-test-workloads.sh
        ;;
    4)
        echo "📊 Deploying Monitoring Stack..."
        ./scripts/deploy-monitoring.sh
        ;;
    5)
        echo "📊 Monitoring LEAD Scheduler..."
        ./scripts/monitor-scheduler.sh --continuous
        ;;
    6)
        echo "🧪 Running validation tests..."
        ./scripts/validate-scheduling.sh
        ;;
    7)
        echo "🚀 Running full production test..."
        echo ""
        echo "Step 1: Deploying LEAD Scheduler..."
        ./scripts/deploy-lead-scheduler.sh
        echo ""
        echo "Step 2: Deploying Monitoring Stack..."
        ./scripts/deploy-monitoring.sh
        echo ""
        echo "Step 3: Waiting for scheduler to initialize..."
        sleep 30
        echo ""
        echo "Step 4: Deploying Test Workloads..."
        ./scripts/deploy-test-workloads.sh
        echo ""
        echo "Step 5: Waiting for workloads to be ready..."
        sleep 60
        echo ""
        echo "Step 6: Running validation tests..."
        ./scripts/validate-scheduling.sh
        echo ""
        echo "Step 7: Starting continuous monitoring..."
        echo "Press Ctrl+C to stop monitoring"
        ./scripts/monitor-scheduler.sh --continuous
        ;;
    *)
        echo "❌ Invalid choice. Please run the script again and select 1-7."
        exit 1
        ;;
esac

echo ""
echo "✅ Production testing workflow completed!"
echo ""
echo "📚 For more information, see:"
echo "  - PRODUCTION_TESTING_GUIDE.md"
echo "  - Individual scripts in scripts/ directory"
echo ""
echo "🔍 Useful commands:"
echo "  kubectl logs -n kube-system -l app=lead-scheduler -f"
echo "  kubectl get pods -o wide -w"
echo "  kubectl get events --sort-by='.lastTimestamp' | grep -i lead"
