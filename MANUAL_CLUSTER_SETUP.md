# Manual Cluster Setup Guide for LEAD Framework

This guide explains how to configure your manual, multi-region server cluster for optimal LEAD framework performance using the Enhanced Prometheus Network Monitor.

## 🌍 **Enhanced Prometheus Network Monitoring**

The LEAD framework now uses the **Enhanced Prometheus Network Monitor** which provides comprehensive network topology monitoring using only Prometheus metrics, eliminating the need for static node label mappings.

### **1. Prometheus-Based Network Discovery**

The Enhanced Prometheus Network Monitor automatically discovers network topology through Prometheus queries:

#### **A. Automatic Network Metrics Collection**

The Enhanced Prometheus Network Monitor automatically collects:

- **Node Bandwidth**: `rate(node_network_receive_bytes_total[5m]) + rate(node_network_transmit_bytes_total[5m])`
- **Node Latency**: `histogram_quantile(0.95, rate(node_network_latency_seconds_bucket[5m])) * 1000`
- **Node Throughput**: `rate(node_network_transmit_bytes_total[5m]) + rate(node_network_receive_bytes_total[5m])`
- **Packet Loss**: `rate(node_network_drop_total[5m]) / rate(node_network_receive_bytes_total[5m]) * 100`

#### **B. Inter-Node Network Metrics**

Inter-node metrics are derived from node-level Prometheus data (no service mesh required):

- **Inter-Node Latency**: estimated from node P95 network latency
- **Inter-Node Bandwidth**: minimum of the two nodes' bandwidths
- **Network Hops**: not available without a service mesh (reported as 0)
- **Geographic Distance**: inferred only when nodes share the same region (0), otherwise unknown

#### **C. Instance Type Information**
```bash
# Label servers by their capabilities
kubectl label node worker-asia-1 node.kubernetes.io/instance-type=high-bandwidth-server
kubectl label node worker-americas-1 node.kubernetes.io/instance-type=medium-bandwidth-server
kubectl label node worker-africa-1 node.kubernetes.io/instance-type=low-bandwidth-server
```

### **2. Advanced Labeling Options**

#### **A. Distance-Based Zone Labels**
```bash
# Include distance information in zone labels
kubectl label node worker-asia-1 topology.kubernetes.io/zone=asia-east1-8000km
kubectl label node worker-americas-1 topology.kubernetes.io/zone=americas-west1-9000km
kubectl label node worker-africa-1 topology.kubernetes.io/zone=africa-south1-8000km
```

#### **B. Latency-Based Zone Labels**
```bash
# Include latency information in zone labels
kubectl label node worker-asia-1 topology.kubernetes.io/zone=asia-east1-50ms
kubectl label node worker-americas-1 topology.kubernetes.io/zone=americas-west1-100ms
kubectl label node worker-africa-1 topology.kubernetes.io/zone=africa-south1-80ms
```

## 📊 **Prometheus Configuration for Real-Time Metrics**

### **1. Network Metrics Collection**

Configure Prometheus to collect real-time network metrics between your servers:

```yaml
# prometheus-config.yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'node-network-metrics'
    static_configs:
      - targets: ['master-node:9100', 'worker-asia-1:9100', 'worker-americas-1:9100', 'worker-africa-1:9100']
    metrics_path: /metrics
    scrape_interval: 10s

  - job_name: 'inter-node-latency'
    static_configs:
      - targets: ['network-monitor:8080']
    metrics_path: /inter-node-metrics
    scrape_interval: 5s
```

### **2. Custom Network Metrics**

Create custom metrics for inter-node network performance:

```bash
# Example: Measure latency between nodes
curl -X POST http://prometheus:9090/api/v1/admin/tsdb/snapshot

# Example: Measure bandwidth utilization
curl -X POST http://prometheus:9090/api/v1/admin/tsdb/snapshot
```

## 🔧 **LEAD Framework Configuration**

### **1. Enable Dynamic Network Monitoring**

The LEAD framework now uses `EnhancedPrometheusNetworkMonitor` by default, which provides:
- **Label-based network topology discovery**
- **Real-time Prometheus integration**
- **Automatic geographic awareness**
- **No static configuration required**

```yaml
# lead-config.yaml
network:
  monitoring:
    enabled: true
    interval: 30s
    prometheus_url: "http://prometheus:9090"
  
  topology:
    discovery:
      enabled: true
      label_based: true
      real_time_metrics: true
      enhanced_prometheus_monitor: true  # Uses EnhancedPrometheusNetworkMonitor
```

### **2. Configure Service Priorities**

```yaml
# service-priorities.yaml
services:
  frontend:
    priority: 100
    network_requirements:
      min_bandwidth: 800
      max_latency: 50
  
  database:
    priority: 90
    network_requirements:
      min_bandwidth: 600
      max_latency: 20
  
  cache:
    priority: 80
    network_requirements:
      min_bandwidth: 500
      max_latency: 10
```

## 🚀 **Deployment Script**

### **1. Automated Node Labeling**

```bash
#!/bin/bash
# setup-manual-cluster.sh

# Function to label nodes with network information
label_node() {
    local node_name=$1
    local region=$2
    local zone=$3
    local bandwidth=$4
    local latency=$5
    local instance_type=$6
    
    echo "Labeling node: $node_name"
    
    # Geographic labels
    kubectl label node $node_name topology.kubernetes.io/region=$region
    kubectl label node $node_name topology.kubernetes.io/zone=$zone
    
    # Network capability labels
    kubectl label node $node_name network.bandwidth.mbps=$bandwidth
    kubectl label node $node_name network.latency.ms=$latency
    kubectl label node $node_name network.throughput.mbps=$((bandwidth * 85 / 100))
    kubectl label node $node_name network.packetloss.percent=0.1
    
    # Instance type label
    kubectl label node $node_name node.kubernetes.io/instance-type=$instance_type
}

# Label your nodes
label_node "master-node" "europe" "europe-west1" 1000 5 "high-bandwidth-server"
label_node "worker-asia-1" "asia" "asia-east1" 1000 5 "high-bandwidth-server"
label_node "worker-americas-1" "americas" "americas-west1" 500 10 "medium-bandwidth-server"
label_node "worker-africa-1" "africa" "africa-south1" 100 20 "low-bandwidth-server"

echo "Node labeling completed!"
```

### **2. Verification Script**

```bash
#!/bin/bash
# verify-cluster-setup.sh

echo "Verifying LEAD framework cluster setup..."

# Check node labels
echo "Node labels:"
kubectl get nodes --show-labels

# Check network topology discovery
echo "Network topology information:"
kubectl get nodes -o custom-columns=NAME:.metadata.name,REGION:.metadata.labels.topology\.kubernetes\.io/region,ZONE:.metadata.labels.topology\.kubernetes\.io/zone,BANDWIDTH:.metadata.labels.network\.bandwidth\.mbps,LATENCY:.metadata.labels.network\.latency\.ms

# Check LEAD scheduler status
echo "LEAD scheduler status:"
kubectl get pods -n kube-system | grep lead-scheduler

# Check Prometheus metrics
echo "Prometheus metrics endpoint:"
curl -s http://prometheus:9090/api/v1/query?query=up | jq '.data.result'
```

## 📈 **Expected Benefits**

### **1. Dynamic Network Topology Discovery**
- **Automatic detection** of server capabilities from labels
- **Real-time updates** of network performance
- **No static configuration** required

### **2. Intelligent Service Placement**
- **Geographic optimization** based on zone labels
- **Bandwidth-aware placement** based on server capabilities
- **Latency optimization** based on real-time measurements

### **3. Adaptive Performance**
- **Real-time network monitoring** via Prometheus
- **Dynamic re-scheduling** based on network conditions
- **Automatic optimization** without manual intervention

## 🔍 **Monitoring and Debugging**

### **1. Check Network Topology Discovery**
```bash
# View discovered network topology
kubectl logs deployment/lead-scheduler -n kube-system | grep "Network topology"

# Check inter-node metrics
curl http://lead-scheduler:10259/lead/network-topology
```

### **2. Monitor Service Placement Decisions**
```bash
# View scheduling decisions
kubectl logs deployment/lead-scheduler -n kube-system | grep "Scheduling pod"

# Check affinity rules
kubectl get pods -o wide
```

### **3. Verify Network Performance**
```bash
# Check real-time network metrics
curl http://prometheus:9090/api/v1/query?query=node_network_latency

# Monitor inter-node communication
curl http://prometheus:9090/api/v1/query?query=inter_node_latency
```

## 🎯 **Summary**

This setup provides:

1. **🌍 Dynamic Geographic Discovery**: Automatically detects server locations from labels
2. **📊 Real-Time Network Monitoring**: Uses Prometheus for live network metrics
3. **🔧 Label-Based Configuration**: No static mappings required
4. **⚡ Intelligent Placement**: Optimizes pod placement based on network topology
5. **🔄 Adaptive Performance**: Continuously adapts to changing network conditions

The LEAD framework will automatically discover your server capabilities and optimize service placement across your global infrastructure! 🚀
