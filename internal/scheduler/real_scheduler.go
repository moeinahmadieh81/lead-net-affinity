package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/lead"
	"lead-framework/internal/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// LEADScheduler implements a real Kubernetes scheduler using LEAD algorithms directly
type LEADScheduler struct {
	client        kubernetes.Interface
	leadFramework *lead.LEADFramework
	config        *lead.FrameworkConfig
	ctx           context.Context
	cancel        context.CancelFunc

	// LEAD algorithms
	scoringAlgorithm    *algorithms.ScoringAlgorithm
	affinityGenerator   *algorithms.AffinityRuleGenerator
	monitoringAlgorithm *algorithms.MonitoringAlgorithm

	// Current service graph
	serviceGraph *models.ServiceGraph

	// Node information
	nodes map[string]*corev1.Node
}

// NewLEADScheduler creates a new LEAD-based Kubernetes scheduler
func NewLEADScheduler(client kubernetes.Interface, leadFramework *lead.LEADFramework, config *lead.FrameworkConfig) *LEADScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &LEADScheduler{
		client:        client,
		leadFramework: leadFramework,
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		nodes:         make(map[string]*corev1.Node),
	}
}

// Run starts the LEAD scheduler
func (ls *LEADScheduler) Run(ctx context.Context) error {
	log.Println("Initializing LEAD Kubernetes Scheduler...")

	// Initialize LEAD framework
	if err := ls.leadFramework.Start(ctx, nil); err != nil {
		return fmt.Errorf("failed to start LEAD framework: %v", err)
	}

	// Get the service graph from LEAD framework
	// Note: The LEAD framework will discover services dynamically
	ls.serviceGraph = models.NewServiceGraph()

	// Initialize LEAD algorithms directly
	ls.initializeLEADAlgorithms()

	// Start HTTP server for metrics
	go ls.startHTTPServer()

	// Start informer factory
	informerFactory := informers.NewSharedInformerFactory(ls.client, time.Minute*10)

	// Create node informer
	nodeInformer := informerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ls.handleNodeAdd,
		UpdateFunc: ls.handleNodeUpdate,
		DeleteFunc: ls.handleNodeDelete,
	})

	// Create pod informer for pods using LEAD scheduler
	podInformer := informerFactory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ls.handlePodAdd,
		UpdateFunc: ls.handlePodUpdate,
		DeleteFunc: ls.handlePodDelete,
	})

	// Start informers
	informerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.HasSynced, podInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}

	log.Println("LEAD Kubernetes Scheduler started successfully")

	// Start scheduling analysis loop
	go ls.schedulingAnalysisLoop(ctx)

	// Start pod scheduling loop
	go ls.podSchedulingLoop(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	log.Println("LEAD Kubernetes Scheduler shutting down...")
	ls.leadFramework.Stop()

	return nil
}

// initializeLEADAlgorithms initializes LEAD algorithms directly
func (ls *LEADScheduler) initializeLEADAlgorithms() {
	log.Println("Initializing LEAD algorithms...")

	// Initialize scoring algorithm
	ls.scoringAlgorithm = algorithms.NewScoringAlgorithm(ls.serviceGraph)

	// Initialize affinity rule generator
	ls.affinityGenerator = algorithms.NewAffinityRuleGenerator(ls.serviceGraph)

	// Initialize monitoring algorithm
	ls.monitoringAlgorithm = algorithms.NewMonitoringAlgorithm(
		ls.serviceGraph,
		ls.scoringAlgorithm,
		ls.config.ResourceThreshold,
		ls.config.MonitoringInterval,
	)

	// Start monitoring algorithm
	if err := ls.monitoringAlgorithm.Start(); err != nil {
		log.Printf("Warning: Failed to start monitoring algorithm: %v", err)
	}

	log.Println("LEAD algorithms initialized successfully")
}

// startHTTPServer starts HTTP server for health checks and metrics
func (ls *LEADScheduler) startHTTPServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ls.leadFramework.IsRunning() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not Ready"))
		}
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// LEAD scheduler metrics
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# LEAD Scheduler Metrics\n"))
		w.Write([]byte("lead_scheduler_status 1\n"))

		// Add LEAD framework metrics
		if ls.serviceGraph != nil {
			fmt.Fprintf(w, "lead_services_total %d\n", len(ls.serviceGraph.GetAllNodes()))
			fmt.Fprintf(w, "lead_nodes_total %d\n", len(ls.nodes))
		}
	})

	// Add LEAD framework endpoints
	mux.HandleFunc("/lead/status", func(w http.ResponseWriter, r *http.Request) {
		status := ls.leadFramework.GetFrameworkStatus()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"%s","services":%d,"gateway":"%s","running":%t}`,
			"active", status.TotalServices, status.Gateway, status.IsRunning)
	})

	mux.HandleFunc("/lead/paths", func(w http.ResponseWriter, r *http.Request) {
		paths, err := ls.leadFramework.GetCriticalPaths(10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"paths":%d,"critical_paths":[`, len(paths))
		for i, path := range paths {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `{"score":%.2f,"weight":%d,"length":%d}`,
				path.Score, path.Weight, path.PathLength)
		}
		fmt.Fprintf(w, "]}")
	})

	server := &http.Server{
		Addr:    ":10259",
		Handler: mux,
	}

	log.Printf("Starting HTTP server on :10259")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP server error: %v", err)
	}
}

// handleNodeAdd handles node addition events
func (ls *LEADScheduler) handleNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	log.Printf("Node added: %s", node.Name)
	ls.nodes[node.Name] = node
	ls.updateNetworkTopology()
}

// handleNodeUpdate handles node update events
func (ls *LEADScheduler) handleNodeUpdate(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)

	// Only handle if node conditions changed significantly
	if ls.nodeConditionsChanged(oldNode, newNode) {
		log.Printf("Node updated: %s", newNode.Name)
		ls.nodes[newNode.Name] = newNode
		ls.updateNetworkTopology()
	}
}

// handleNodeDelete handles node deletion events
func (ls *LEADScheduler) handleNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	log.Printf("Node deleted: %s", node.Name)
	delete(ls.nodes, node.Name)
	ls.updateNetworkTopology()
}

// nodeConditionsChanged checks if node conditions changed significantly
func (ls *LEADScheduler) nodeConditionsChanged(oldNode, newNode *corev1.Node) bool {
	// Check if node became ready/unready
	oldReady := ls.isNodeReady(oldNode)
	newReady := ls.isNodeReady(newNode)

	return oldReady != newReady
}

// isNodeReady checks if a node is ready
func (ls *LEADScheduler) isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// handlePodAdd handles pod addition events for LEAD-scheduled pods
func (ls *LEADScheduler) handlePodAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)

	// Only handle pods that should be scheduled by LEAD scheduler
	if pod.Spec.SchedulerName != "lead-scheduler" {
		return
	}

	log.Printf("LEAD Pod added: %s/%s", pod.Namespace, pod.Name)
	ls.schedulePod(pod)
}

// handlePodUpdate handles pod update events
func (ls *LEADScheduler) handlePodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	// Only handle pods that should be scheduled by LEAD scheduler
	if newPod.Spec.SchedulerName != "lead-scheduler" {
		return
	}

	// Only handle if pod phase changed to Pending (needs scheduling)
	if oldPod.Status.Phase != newPod.Status.Phase && newPod.Status.Phase == corev1.PodPending {
		log.Printf("LEAD Pod needs scheduling: %s/%s", newPod.Namespace, newPod.Name)
		ls.schedulePod(newPod)
	}
}

// handlePodDelete handles pod deletion events
func (ls *LEADScheduler) handlePodDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)

	// Only handle pods that were scheduled by LEAD scheduler
	if pod.Spec.SchedulerName != "lead-scheduler" {
		return
	}

	log.Printf("LEAD Pod deleted: %s/%s", pod.Namespace, pod.Name)
}

// schedulePod schedules a pod using LEAD algorithms
func (ls *LEADScheduler) schedulePod(pod *corev1.Pod) {
	log.Printf("Scheduling pod %s/%s using LEAD algorithms", pod.Namespace, pod.Name)

	// Extract service information from pod
	serviceInfo := ls.extractServiceInfo(pod)
	if serviceInfo == nil {
		log.Printf("Could not extract service info for pod %s/%s, using default scheduling", pod.Namespace, pod.Name)
		ls.schedulePodToRandomNode(pod)
		return
	}

	// Get available nodes
	availableNodes := ls.getAvailableNodes()
	if len(availableNodes) == 0 {
		log.Printf("No available nodes for pod %s/%s", pod.Namespace, pod.Name)
		return
	}

	// Use LEAD scoring algorithm to score nodes
	nodeScores := ls.scoreNodesForPod(availableNodes, serviceInfo)
	if len(nodeScores) == 0 {
		log.Printf("No suitable nodes found for pod %s/%s", pod.Namespace, pod.Name)
		return
	}

	// Select the best node
	bestNode := ls.selectBestNode(nodeScores)

	// Schedule the pod to the selected node
	ls.bindPodToNode(pod, bestNode)

	log.Printf("Pod %s/%s scheduled to node %s (score: %.2f)",
		pod.Namespace, pod.Name, bestNode.Name, nodeScores[bestNode.Name])
}

// extractServiceInfo extracts service information from pod
func (ls *LEADScheduler) extractServiceInfo(pod *corev1.Pod) *ServiceInfo {
	serviceName := ""

	// Try to get service name from labels
	if name, exists := pod.Labels["io.kompose.service"]; exists {
		serviceName = name
	} else if name, exists := pod.Labels["app"]; exists {
		serviceName = name
	} else {
		// Extract from pod name
		serviceName = ls.extractServiceNameFromPodName(pod.Name)
	}

	if serviceName == "" {
		return nil
	}

	// Determine service type
	serviceType := ls.determineServiceType(pod, serviceName)

	// Get network topology from LEAD framework
	networkTopology := ls.getNetworkTopologyForService(serviceName)

	return &ServiceInfo{
		ServiceName:     serviceName,
		ServiceType:     serviceType,
		NetworkTopology: networkTopology,
		Priority:        ls.determineServicePriority(serviceName),
	}
}

// getNetworkTopologyForService gets network topology from LEAD framework
func (ls *LEADScheduler) getNetworkTopologyForService(serviceName string) *models.NetworkTopology {
	// Get service node from LEAD framework
	if node, exists := ls.serviceGraph.GetNode(serviceName); exists && node.NetworkTopology != nil {
		return node.NetworkTopology
	}

	// Return default network topology
	return &models.NetworkTopology{
		AvailabilityZone: "default",
		Bandwidth:        500,
		Hops:             1,
		GeoDistance:      0,
	}
}

// getAvailableNodes returns list of available nodes
func (ls *LEADScheduler) getAvailableNodes() []*corev1.Node {
	var availableNodes []*corev1.Node

	for _, node := range ls.nodes {
		if ls.isNodeReady(node) && ls.isNodeSchedulable(node) {
			availableNodes = append(availableNodes, node)
		}
	}

	return availableNodes
}

// isNodeSchedulable checks if a node is schedulable
func (ls *LEADScheduler) isNodeSchedulable(node *corev1.Node) bool {
	return !node.Spec.Unschedulable
}

// scoreNodesForPod scores nodes for a pod using LEAD algorithms
func (ls *LEADScheduler) scoreNodesForPod(nodes []*corev1.Node, serviceInfo *ServiceInfo) map[string]float64 {
	nodeScores := make(map[string]float64)

	for _, node := range nodes {
		score := ls.calculateNodeScore(node, serviceInfo)
		nodeScores[node.Name] = score
	}

	return nodeScores
}

// calculateNodeScore calculates LEAD-based score for a node
// Enhanced with comprehensive network topology considerations
func (ls *LEADScheduler) calculateNodeScore(node *corev1.Node, serviceInfo *ServiceInfo) float64 {
	score := 50.0 // Base score

	// Enhanced network topology scoring
	if serviceInfo.NetworkTopology != nil {
		// Zone affinity scoring
		nodeZone := node.Labels["topology.kubernetes.io/zone"]
		if nodeZone == serviceInfo.NetworkTopology.AvailabilityZone {
			score += 25.0 // Increased bonus for same zone
		}

		// Bandwidth scoring (enhanced)
		bandwidthScore := serviceInfo.NetworkTopology.Bandwidth / 50.0 // Normalize bandwidth
		score += math.Min(bandwidthScore, 20.0)                        // Cap at 20 points

		// Throughput scoring (new)
		throughputScore := serviceInfo.NetworkTopology.Throughput / 50.0
		score += math.Min(throughputScore, 15.0) // Cap at 15 points

		// Latency scoring (new) - lower latency = higher score
		latencyScore := math.Max(0.0, 20.0-serviceInfo.NetworkTopology.Latency/5.0)
		score += latencyScore

		// Packet loss scoring (new) - lower packet loss = higher score
		packetLossScore := math.Max(0.0, 10.0-serviceInfo.NetworkTopology.PacketLoss*100.0)
		score += packetLossScore

		// Geo distance scoring (new) - shorter distance = higher score
		distanceScore := math.Max(0.0, 10.0-serviceInfo.NetworkTopology.GeoDistance/50.0)
		score += distanceScore
	}

	// Service priority scoring
	score += float64(serviceInfo.Priority) / 10.0

	// Resource availability scoring
	if node.Status.Allocatable != nil {
		// CPU availability
		if cpu := node.Status.Allocatable["cpu"]; !cpu.IsZero() {
			score += 10.0
		}
		// Memory availability
		if memory := node.Status.Allocatable["memory"]; !memory.IsZero() {
			score += 10.0
		}
	}

	// Node condition scoring
	if ls.isNodeReady(node) {
		score += 15.0
	}

	return score
}

// selectBestNode selects the best node from scored nodes
func (ls *LEADScheduler) selectBestNode(nodeScores map[string]float64) *corev1.Node {
	var bestNode *corev1.Node
	var bestScore float64

	for nodeName, score := range nodeScores {
		if score > bestScore {
			bestScore = score
			bestNode = ls.nodes[nodeName]
		}
	}

	return bestNode
}

// bindPodToNode binds a pod to a node
func (ls *LEADScheduler) bindPodToNode(pod *corev1.Pod, node *corev1.Node) {
	// Create binding
	binding := &corev1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Target: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       node.Name,
		},
	}

	// Bind the pod
	err := ls.client.CoreV1().Pods(pod.Namespace).Bind(ls.ctx, binding, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to bind pod %s/%s to node %s: %v",
			pod.Namespace, pod.Name, node.Name, err)
	} else {
		log.Printf("Successfully bound pod %s/%s to node %s",
			pod.Namespace, pod.Name, node.Name)
	}
}

// schedulePodToRandomNode schedules a pod to a random available node (fallback)
func (ls *LEADScheduler) schedulePodToRandomNode(pod *corev1.Pod) {
	availableNodes := ls.getAvailableNodes()
	if len(availableNodes) == 0 {
		log.Printf("No available nodes for fallback scheduling of pod %s/%s", pod.Namespace, pod.Name)
		return
	}

	// Select first available node
	node := availableNodes[0]
	ls.bindPodToNode(pod, node)
}

// schedulingAnalysisLoop runs periodic scheduling analysis
func (ls *LEADScheduler) schedulingAnalysisLoop(ctx context.Context) {
	ticker := time.NewTicker(ls.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ls.performSchedulingAnalysis()
		}
	}
}

// performSchedulingAnalysis performs LEAD-based scheduling analysis
func (ls *LEADScheduler) performSchedulingAnalysis() {
	// Get current critical paths from LEAD framework
	paths, err := ls.leadFramework.GetCriticalPaths(10)
	if err != nil {
		log.Printf("Failed to get critical paths: %v", err)
		return
	}

	// Get network topology analysis
	analysis, err := ls.leadFramework.GetNetworkTopologyAnalysis()
	if err != nil {
		log.Printf("Failed to get network topology analysis: %v", err)
		return
	}

	log.Printf("LEAD Scheduler Analysis:")
	log.Printf("  - Critical paths: %d", len(paths))
	log.Printf("  - Network analysis: %d total paths, avg bandwidth: %.2f Mbps",
		analysis.TotalPaths, analysis.AvgBandwidth)
	log.Printf("  - Available nodes: %d", len(ls.getAvailableNodes()))

	// Update scheduling decisions based on LEAD analysis
	ls.updateSchedulingDecisions(paths, analysis)
}

// podSchedulingLoop continuously processes pending pods
func (ls *LEADScheduler) podSchedulingLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ls.processPendingPods()
		}
	}
}

// processPendingPods processes pods that are pending and need scheduling
func (ls *LEADScheduler) processPendingPods() {
	// Get all pods in all namespaces
	pods, err := ls.client.CoreV1().Pods("").List(ls.ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Pending",
	})
	if err != nil {
		log.Printf("Failed to list pending pods: %v", err)
		return
	}

	for _, pod := range pods.Items {
		// Only process pods that should be scheduled by LEAD scheduler
		if pod.Spec.SchedulerName == "lead-scheduler" && pod.Spec.NodeName == "" {
			log.Printf("Processing pending LEAD pod: %s/%s", pod.Namespace, pod.Name)
			ls.schedulePod(&pod)
		}
	}
}

// updateSchedulingDecisions updates scheduling decisions based on LEAD analysis
func (ls *LEADScheduler) updateSchedulingDecisions(paths []*models.Path, analysis *algorithms.NetworkTopologyAnalysis) {
	log.Printf("Updating scheduling decisions based on LEAD analysis...")
	if len(paths) > 0 {
		log.Printf("  - Top critical path: %v (Score: %.2f)",
			paths[0].GetServiceNames(), paths[0].Score)
	}
	log.Printf("  - Network topology: %d paths, avg bandwidth: %.2f Mbps",
		analysis.TotalPaths, analysis.AvgBandwidth)
}

// updateNetworkTopology updates network topology based on current nodes
func (ls *LEADScheduler) updateNetworkTopology() {
	log.Printf("Updating network topology for %d nodes", len(ls.nodes))

	// Update LEAD framework with current node information
	// This could trigger re-analysis of critical paths
	if ls.leadFramework != nil {
		ls.leadFramework.TriggerReanalysis()
	}
}

// Helper methods (same as before but using LEAD framework data)
func (ls *LEADScheduler) extractServiceNameFromPodName(podName string) string {
	// Remove common deployment suffixes
	suffixes := []string{"-deployment-", "-pod-", "-sts-"}
	for _, suffix := range suffixes {
		for i := len(podName) - 1; i >= 0; i-- {
			if i+len(suffix) <= len(podName) && podName[i:i+len(suffix)] == suffix {
				return podName[:i]
			}
		}
	}

	// Remove replica set suffixes (e.g., "-abc123")
	for i := len(podName) - 1; i >= 0; i-- {
		if podName[i] == '-' && i < len(podName)-1 && ls.isAlphanumeric(podName[i+1:]) {
			return podName[:i]
		}
	}

	return podName
}

func (ls *LEADScheduler) determineServiceType(pod *corev1.Pod, serviceName string) string {
	// Check for database services
	if ls.contains(serviceName, "mongodb") || ls.contains(serviceName, "mongo") {
		return "database"
	}
	if ls.contains(serviceName, "memcached") || ls.contains(serviceName, "memcache") {
		return "cache"
	}

	// Check for specific microservices
	switch serviceName {
	case "frontend", "fe":
		return "gateway"
	case "search", "src":
		return "search"
	case "user", "usr":
		return "user"
	case "recommendation", "rcm":
		return "recommendation"
	case "reservation", "rsv":
		return "reservation"
	case "profile", "prf":
		return "profile"
	case "geo", "geographic":
		return "geographic"
	case "rate", "rte":
		return "rate"
	default:
		return "microservice"
	}
}

func (ls *LEADScheduler) determineServicePriority(serviceName string) int {
	// Higher number = higher priority
	switch serviceName {
	case "frontend", "fe":
		return 100 // Gateway - highest priority
	case "search", "src":
		return 90
	case "user", "usr":
		return 80
	case "reservation", "rsv":
		return 85
	case "recommendation", "rcm":
		return 70
	case "profile", "prf":
		return 60
	case "geo", "geographic":
		return 50
	case "rate", "rte":
		return 55
	default:
		return 50 // Default priority
	}
}

func (ls *LEADScheduler) contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

func (ls *LEADScheduler) isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// GetLEADFramework returns the LEAD framework instance
func (ls *LEADScheduler) GetLEADFramework() *lead.LEADFramework {
	return ls.leadFramework
}

// Stop stops the LEAD scheduler
func (ls *LEADScheduler) Stop() {
	ls.cancel()
}
