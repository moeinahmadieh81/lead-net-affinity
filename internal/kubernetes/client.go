package kubernetes

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"lead-framework/internal/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesClient provides integration with Kubernetes API
type KubernetesClient struct {
	clientset   *kubernetes.Clientset
	namespace   string
	ctx         context.Context
	cancel      context.CancelFunc
	podStore    cache.Store
	podInformer cache.SharedIndexInformer
	eventChan   chan PodEvent
}

// PodEvent represents a pod lifecycle event
type PodEvent struct {
	Type      string          `json:"type"` // ADDED, MODIFIED, DELETED
	Pod       *models.PodInfo `json:"pod"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewKubernetesClient creates a new Kubernetes client
func NewKubernetesClient(namespace string) (*KubernetesClient, error) {
	// Try in-cluster config first, then fall back to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	kc := &KubernetesClient{
		clientset: clientset,
		namespace: namespace,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan PodEvent, 100),
	}

	// Set up informer for pod events
	if err := kc.setupPodInformer(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup pod informer: %v", err)
	}

	return kc, nil
}

// setupPodInformer sets up a pod informer to watch pod events
func (kc *KubernetesClient) setupPodInformer() error {
	// Create informer factory
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		kc.clientset,
		time.Minute*10, // resync period
		informers.WithNamespace(kc.namespace),
	)

	// Create pod informer
	kc.podInformer = informerFactory.Core().V1().Pods().Informer()
	kc.podStore = kc.podInformer.GetStore()

	// Add event handlers
	kc.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			kc.handlePodEvent("ADDED", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			kc.handlePodEvent("MODIFIED", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			kc.handlePodEvent("DELETED", obj)
		},
	})

	// Start the informer
	go kc.podInformer.Run(kc.ctx.Done())

	// Wait for cache to sync
	if !cache.WaitForCacheSync(kc.ctx.Done(), kc.podInformer.HasSynced) {
		return fmt.Errorf("failed to sync pod informer cache")
	}

	log.Printf("Pod informer started for namespace: %s", kc.namespace)
	return nil
}

// handlePodEvent processes pod events
func (kc *KubernetesClient) handlePodEvent(eventType string, obj interface{}) {
	pod := obj.(*corev1.Pod)

	// Convert to our PodInfo format
	podInfo := kc.convertToPodInfo(pod)

	// Determine service type based on pod name and labels
	podInfo.ServiceType = kc.determineServiceType(pod)

	// Send event
	select {
	case kc.eventChan <- PodEvent{
		Type:      eventType,
		Pod:       podInfo,
		Timestamp: time.Now(),
	}:
	default:
		log.Printf("Warning: pod event channel is full, dropping event for pod %s", podInfo.Name)
	}
}

// convertToPodInfo converts Kubernetes pod to our PodInfo format
func (kc *KubernetesClient) convertToPodInfo(pod *corev1.Pod) *models.PodInfo {
	podInfo := &models.PodInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		NodeName:     pod.Spec.NodeName,
		PodIP:        pod.Status.PodIP,
		HostIP:       pod.Status.HostIP,
		Status:       string(pod.Status.Phase),
		Labels:       pod.Labels,
		Annotations:  pod.Annotations,
		CreationTime: pod.CreationTimestamp.Time,
	}

	// Extract resource information from first container
	if len(pod.Spec.Containers) > 0 {
		container := pod.Spec.Containers[0]
		if container.Resources.Requests != nil {
			if cpu, exists := container.Resources.Requests["cpu"]; exists {
				podInfo.ResourceRequests.CPU = cpu.String()
			}
			if memory, exists := container.Resources.Requests["memory"]; exists {
				podInfo.ResourceRequests.Memory = memory.String()
			}
		}
		if container.Resources.Limits != nil {
			if cpu, exists := container.Resources.Limits["cpu"]; exists {
				podInfo.ResourceLimits.CPU = cpu.String()
			}
			if memory, exists := container.Resources.Limits["memory"]; exists {
				podInfo.ResourceLimits.Memory = memory.String()
			}
		}
	}

	// Determine service name from labels or pod name
	if serviceName := pod.Labels["io.kompose.service"]; serviceName != "" {
		podInfo.ServiceName = serviceName
	} else {
		// Extract service name from pod name (remove deployment suffix)
		podInfo.ServiceName = kc.extractServiceNameFromPodName(pod.Name)
	}

	return podInfo
}

// determineServiceType determines if a pod is a microservice, MongoDB, or Memcached
func (kc *KubernetesClient) determineServiceType(pod *corev1.Pod) string {
	podName := pod.Name
	serviceName := pod.Labels["io.kompose.service"]

	// Check for MongoDB pods
	if serviceName != "" && (serviceName == "mongodb-profile" || serviceName == "mongodb-rate" ||
		serviceName == "mongodb-user" || serviceName == "mongodb-geo" || serviceName == "mongodb-recommendation" ||
		serviceName == "mongodb-reservation") {
		return "mongodb"
	}

	// Check for Memcached pods
	if serviceName != "" && (serviceName == "memcached-profile" || serviceName == "memcached-rate" ||
		serviceName == "memcached-reservation") {
		return "memcached"
	}

	// Check pod name for MongoDB/Memcached patterns
	if contains(podName, "mongodb") || contains(podName, "mongo") {
		return "mongodb"
	}
	if contains(podName, "memcached") || contains(podName, "memcache") {
		return "memcached"
	}

	// Default to microservice
	return "microservice"
}

// extractServiceNameFromPodName extracts service name from pod name
func (kc *KubernetesClient) extractServiceNameFromPodName(podName string) string {
	// Remove common deployment suffixes
	suffixes := []string{"-deployment-", "-pod-", "-sts-"}
	for _, suffix := range suffixes {
		if idx := findLastIndex(podName, suffix); idx != -1 {
			return podName[:idx]
		}
	}

	// Remove replica set suffixes (e.g., "-abc123")
	for i := len(podName) - 1; i >= 0; i-- {
		if podName[i] == '-' && i < len(podName)-1 && isAlphanumeric(podName[i+1:]) {
			return podName[:i]
		}
	}

	return podName
}

// GetPodEvents returns a channel for receiving pod events
func (kc *KubernetesClient) GetPodEvents() <-chan PodEvent {
	return kc.eventChan
}

// GetCurrentPods returns all current pods in the namespace
func (kc *KubernetesClient) GetCurrentPods() ([]*models.PodInfo, error) {
	pods := kc.podStore.List()
	var podInfos []*models.PodInfo

	for _, obj := range pods {
		pod := obj.(*corev1.Pod)
		podInfo := kc.convertToPodInfo(pod)
		podInfo.ServiceType = kc.determineServiceType(pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// GetPodsByService returns pods for a specific service
func (kc *KubernetesClient) GetPodsByService(serviceName string) ([]*models.PodInfo, error) {
	allPods, err := kc.GetCurrentPods()
	if err != nil {
		return nil, err
	}

	var servicePods []*models.PodInfo
	for _, pod := range allPods {
		if pod.ServiceName == serviceName {
			servicePods = append(servicePods, pod)
		}
	}

	return servicePods, nil
}

// GetNodes returns cluster node information
func (kc *KubernetesClient) GetNodes() ([]*NodeInfo, error) {
	nodes, err := kc.clientset.CoreV1().Nodes().List(kc.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodeInfos []*NodeInfo
	for _, node := range nodes.Items {
		nodeInfo := &NodeInfo{
			Name:         node.Name,
			Labels:       node.Labels,
			Annotations:  node.Annotations,
			CreationTime: node.CreationTimestamp.Time,
		}

		// Extract internal IP
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				nodeInfo.InternalIP = address.Address
				break
			}
		}

		// Extract availability zone
		if zone := node.Labels["topology.kubernetes.io/zone"]; zone != "" {
			nodeInfo.AvailabilityZone = zone
		}

		// Extract region
		region := node.Labels["topology.kubernetes.io/region"]
		if region == "" {
			region = node.Labels["failure-domain.beta.kubernetes.io/region"]
		}

		// Create network topology information from dynamic data only
		networkInfo := kc.extractNetworkInfoFromNode(nodeInfo, region)
		if networkInfo != nil {
			nodeInfo.NetworkTopology = networkInfo
		}

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// NodeInfo represents cluster node information
type NodeInfo struct {
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	CreationTime     time.Time         `json:"creation_time"`
	InternalIP       string            `json:"internal_ip"`
	AvailabilityZone string            `json:"availability_zone"`
	NetworkTopology  *NodeNetworkInfo  `json:"network_topology,omitempty"`
}

// NodeNetworkInfo represents network characteristics of a Kubernetes node
type NodeNetworkInfo struct {
	Bandwidth   float64   `json:"bandwidth"`   // Mbps
	Latency     float64   `json:"latency"`     // ms
	Throughput  float64   `json:"throughput"`  // Mbps
	PacketLoss  float64   `json:"packet_loss"` // percentage
	Region      string    `json:"region"`      // Cloud region
	LastUpdated time.Time `json:"last_updated"`
}

// Stop stops the Kubernetes client
func (kc *KubernetesClient) Stop() {
	kc.cancel()
	close(kc.eventChan)
}

// extractNetworkInfoFromNode extracts network information from node labels (dynamic data only)
func (kc *KubernetesClient) extractNetworkInfoFromNode(node *NodeInfo, region string) *NodeNetworkInfo {
	// Extract bandwidth from node labels
	bandwidth := kc.extractBandwidthFromNode(node)
	if bandwidth <= 0 {
		log.Printf("No bandwidth data available for node %s, skipping network topology", node.Name)
		return nil
	}

	// Extract latency from node labels
	latency := kc.extractLatencyFromNode(node)
	if latency <= 0 {
		log.Printf("No latency data available for node %s, skipping network topology", node.Name)
		return nil
	}

	// Extract throughput from node labels
	throughput := kc.extractThroughputFromNode(node)
	if throughput <= 0 {
		log.Printf("No throughput data available for node %s, skipping network topology", node.Name)
		return nil
	}

	// Extract packet loss from node labels
	packetLoss := kc.extractPacketLossFromNode(node)
	if packetLoss < 0 {
		log.Printf("No packet loss data available for node %s, skipping network topology", node.Name)
		return nil
	}

	return &NodeNetworkInfo{
		Bandwidth:   bandwidth,
		Latency:     latency,
		Throughput:  throughput,
		PacketLoss:  packetLoss,
		Region:      region,
		LastUpdated: time.Now(),
	}
}

// extractBandwidthFromNode extracts bandwidth from node labels (dynamic data only)
func (kc *KubernetesClient) extractBandwidthFromNode(node *NodeInfo) float64 {
	// Try to extract bandwidth from node labels
	if bandwidthStr, exists := node.Labels["network.bandwidth.mbps"]; exists {
		if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
			return bandwidth
		}
	}

	// Try alternative label formats
	if bandwidthStr, exists := node.Labels["bandwidth"]; exists {
		if bandwidth, err := strconv.ParseFloat(bandwidthStr, 64); err == nil {
			return bandwidth
		}
	}

	return 0 // No dynamic data available
}

// extractLatencyFromNode extracts latency from node labels (dynamic data only)
func (kc *KubernetesClient) extractLatencyFromNode(node *NodeInfo) float64 {
	// Try to extract latency from node labels
	if latencyStr, exists := node.Labels["network.latency.ms"]; exists {
		if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
			return latency
		}
	}

	// Try alternative label formats
	if latencyStr, exists := node.Labels["latency"]; exists {
		if latency, err := strconv.ParseFloat(latencyStr, 64); err == nil {
			return latency
		}
	}

	return 0 // No dynamic data available
}

// extractThroughputFromNode extracts throughput from node labels (dynamic data only)
func (kc *KubernetesClient) extractThroughputFromNode(node *NodeInfo) float64 {
	// Try to extract throughput from node labels
	if throughputStr, exists := node.Labels["network.throughput.mbps"]; exists {
		if throughput, err := strconv.ParseFloat(throughputStr, 64); err == nil {
			return throughput
		}
	}

	// Try alternative label formats
	if throughputStr, exists := node.Labels["throughput"]; exists {
		if throughput, err := strconv.ParseFloat(throughputStr, 64); err == nil {
			return throughput
		}
	}

	return 0 // No dynamic data available
}

// extractPacketLossFromNode extracts packet loss from node labels (dynamic data only)
func (kc *KubernetesClient) extractPacketLossFromNode(node *NodeInfo) float64 {
	// Try to extract packet loss from node labels
	if packetLossStr, exists := node.Labels["network.packetloss.percent"]; exists {
		if packetLoss, err := strconv.ParseFloat(packetLossStr, 64); err == nil {
			return packetLoss
		}
	}

	// Try alternative label formats
	if packetLossStr, exists := node.Labels["packetloss"]; exists {
		if packetLoss, err := strconv.ParseFloat(packetLossStr, 64); err == nil {
			return packetLoss
		}
	}

	return -1 // No dynamic data available
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

func findLastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// extractLatencyFromZone tries to extract latency information from zone label
func (kc *KubernetesClient) extractLatencyFromZone(zone string) (string, bool) {
	// Try to extract latency from zone label
	// Expected formats: "region-zone-latency", "region-zone-5ms", etc.
	parts := strings.Split(zone, "-")
	if len(parts) >= 3 {
		// Check if last part is a number (latency)
		lastPart := parts[len(parts)-1]
		if _, err := strconv.ParseFloat(lastPart, 64); err == nil {
			return lastPart, true
		}
		// Check if last part contains "ms" or latency indicator
		if strings.Contains(strings.ToLower(lastPart), "ms") {
			latencyStr := strings.TrimSuffix(strings.ToLower(lastPart), "ms")
			if _, err := strconv.ParseFloat(latencyStr, 64); err == nil {
				return latencyStr, true
			}
		}
	}

	// Try to extract from zone label if it contains latency info
	if strings.Contains(strings.ToLower(zone), "latency") {
		// Look for latency pattern in the zone string
		// This is a simple implementation - could be enhanced with regex
		return "", false
	}

	return "", false
}
