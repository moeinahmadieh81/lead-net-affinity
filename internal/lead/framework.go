package lead

import (
	"context"
	"fmt"
	"log"
	"time"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/discovery"
	"lead-framework/internal/kubernetes"
	"lead-framework/internal/models"
	"lead-framework/internal/monitoring"
)

// LEADFramework is the main orchestrator for the LEAD framework
type LEADFramework struct {
	graph                  *models.ServiceGraph
	scoringAlgorithm       *algorithms.ScoringAlgorithm
	monitoringAlgorithm    *algorithms.MonitoringAlgorithm
	affinityGenerator      *algorithms.AffinityRuleGenerator
	kubernetesGenerator    *kubernetes.KubernetesConfigGenerator
	prometheusMonitor      *monitoring.PrometheusMonitor
	enhancedNetworkMonitor *monitoring.EnhancedPrometheusNetworkMonitor
	yamlUpdater            *kubernetes.YAMLUpdater
	serviceDiscovery       *discovery.ServiceDiscovery
	k8sClient              *kubernetes.KubernetesClient
	config                 *FrameworkConfig
	ctx                    context.Context
	cancel                 context.CancelFunc
	isRunning              bool
}

// FrameworkConfig contains configuration for the LEAD framework
type FrameworkConfig struct {
	// Monitoring configuration
	MonitoringInterval time.Duration `json:"monitoring_interval"`
	ResourceThreshold  float64       `json:"resource_threshold"`
	LatencyThreshold   time.Duration `json:"latency_threshold"`

	// Prometheus configuration
	PrometheusURL string `json:"prometheus_url"`

	// Kubernetes configuration
	KubernetesNamespace string `json:"kubernetes_namespace"`
	OutputDirectory     string `json:"output_directory"`

	// Network topology weights
	BandwidthWeight        float64 `json:"bandwidth_weight"`
	HopsWeight             float64 `json:"hops_weight"`
	GeoDistanceWeight      float64 `json:"geo_distance_weight"`
	AvailabilityZoneWeight float64 `json:"availability_zone_weight"`

	// Network data source
	UseCiliumMetrics bool `json:"use_cilium_metrics"`
}

// DefaultFrameworkConfig returns default configuration for the LEAD framework
func DefaultFrameworkConfig() *FrameworkConfig {
	return &FrameworkConfig{
		MonitoringInterval:     30 * time.Second,
		ResourceThreshold:      80.0,
		LatencyThreshold:       100 * time.Millisecond,
		PrometheusURL:          "http://202.133.88.12:30090",
		KubernetesNamespace:    "default",
		OutputDirectory:        "./k8s-manifests",
		BandwidthWeight:        0.4,
		HopsWeight:             0.3,
		GeoDistanceWeight:      0.2,
		AvailabilityZoneWeight: 0.1,
		UseCiliumMetrics:       true,
	}
}

// NewLEADFramework creates a new LEAD framework instance
func NewLEADFramework() *LEADFramework {
	ctx, cancel := context.WithCancel(context.Background())

	return &LEADFramework{
		ctx:       ctx,
		cancel:    cancel,
		config:    DefaultFrameworkConfig(),
		isRunning: false,
	}
}

// NewLEADFrameworkWithConfig creates a new LEAD framework instance with custom configuration
func NewLEADFrameworkWithConfig(config *FrameworkConfig) *LEADFramework {
	ctx, cancel := context.WithCancel(context.Background())

	return &LEADFramework{
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
		isRunning: false,
	}
}

// Start initializes and starts the LEAD framework
func (lf *LEADFramework) Start(ctx context.Context, graph *models.ServiceGraph) error {
	if lf.isRunning {
		return fmt.Errorf("LEAD framework is already running")
	}

	lf.ctx = ctx

	// Try to initialize Kubernetes client (optional - can work with static graph)
	var err error
	lf.k8sClient, err = kubernetes.NewKubernetesClient(lf.config.KubernetesNamespace)
	if err != nil {
		log.Printf("Warning: Failed to create Kubernetes client: %v", err)
		log.Println("Continuing with static service graph (Kubernetes client not required for static graphs)")
		lf.k8sClient = nil
	}

	// If we have a static graph provided, use it directly
	if graph != nil && len(graph.GetAllNodes()) > 0 {
		log.Println("Using provided static service graph")
		lf.graph = graph
	} else if lf.k8sClient != nil {
		// Initialize service discovery only if we have Kubernetes client
		lf.serviceDiscovery = discovery.NewServiceDiscovery(lf.k8sClient)

		// Set up graph update callback
		lf.serviceDiscovery.SetUpdateCallback(func(updatedGraph *models.ServiceGraph) {
			lf.handleGraphUpdate(updatedGraph)
		})

		// Start service discovery
		if err := lf.serviceDiscovery.Start(); err != nil {
			log.Printf("Warning: Failed to start service discovery: %v", err)
			log.Println("Continuing with static graph if provided")
		} else {
			// Get the discovered service graph
			lf.graph = lf.serviceDiscovery.GetServiceGraph()
		}
	}

	// If still no graph, check if we can create a minimal static graph
	if lf.graph == nil || len(lf.graph.GetAllNodes()) == 0 {
		if graph != nil {
			log.Println("Using provided static graph as fallback")
			lf.graph = graph
		} else {
			// Create a minimal static graph with default services
			log.Println("Creating minimal static service graph with default services")
			lf.graph = lf.createDefaultStaticGraph()
		}
	}

	log.Println("Initializing LEAD framework components...")

	// Initialize algorithms with the graph we have
	lf.scoringAlgorithm = algorithms.NewScoringAlgorithm(lf.graph)
	lf.monitoringAlgorithm = algorithms.NewMonitoringAlgorithm(
		lf.graph,
		lf.scoringAlgorithm,
		lf.config.ResourceThreshold,
		lf.config.MonitoringInterval,
	)
	lf.affinityGenerator = algorithms.NewAffinityRuleGenerator(lf.graph)

	// Set network metrics provider for scoring algorithm (will be set later when network monitor is ready)

	// Initialize Kubernetes config generator
	lf.kubernetesGenerator = kubernetes.NewKubernetesConfigGenerator(
		lf.config.KubernetesNamespace,
		lf.config.OutputDirectory,
		lf.affinityGenerator,
	)

	// Initialize Prometheus monitor
	prometheusMonitor, err := monitoring.NewPrometheusMonitor(
		lf.config.PrometheusURL,
		lf.config.MonitoringInterval,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize Prometheus monitor: %v", err)
		log.Println("Continuing without Prometheus monitoring...")
	} else {
		lf.prometheusMonitor = prometheusMonitor

		// Validate Prometheus connection
		if err := lf.prometheusMonitor.ValidatePrometheusConnection(); err != nil {
			log.Printf("Warning: Prometheus connection validation failed: %v", err)
			log.Println("Continuing without Prometheus monitoring...")
			lf.prometheusMonitor = nil
		} else {
			log.Println("Prometheus monitoring initialized successfully")

			// Initialize enhanced Prometheus network monitor
			prometheusClient := monitoring.NewRealPrometheusClient(lf.config.PrometheusURL)
			enhancedNetworkMonitor := monitoring.NewEnhancedPrometheusNetworkMonitor(
				prometheusClient,
				lf.graph,
				lf.config.MonitoringInterval,
			)

			// If configured, use Cilium/Hubble-powered queries for inter-node and service metrics
			if lf.config.UseCiliumMetrics {
				enhancedNetworkMonitor.SetNetworkQueries(monitoring.CiliumNetworkQueries())
			}

			// Start enhanced network monitoring
			if err := enhancedNetworkMonitor.Start(); err != nil {
				log.Printf("Warning: Failed to start enhanced network monitoring: %v", err)
			} else {
				log.Println("Enhanced Prometheus network monitoring started successfully")
				lf.enhancedNetworkMonitor = enhancedNetworkMonitor

				// Set network monitor adapter for affinity generator to use Cilium metrics
				adapter := monitoring.NewNetworkMetricsAdapter(enhancedNetworkMonitor)
				lf.affinityGenerator.SetNetworkMonitor(adapter)

				// Set network metrics provider for scoring algorithm to use inter-node metrics
				// Create an adapter that implements models.InterNodeMetricsProvider
				networkMetricsAdapter := &NetworkMetricsProviderAdapter{monitor: enhancedNetworkMonitor}
				lf.scoringAlgorithm.SetNetworkMetricsProvider(networkMetricsAdapter)

				// Immediately populate network topology for all services (with defaults if needed)
				// This ensures services have NetworkTopology populated before analysis
				enhancedNetworkMonitor.UpdateServiceGraphNetworkTopology()

				// Wait a moment for Prometheus queries to complete if possible
				time.Sleep(1 * time.Second)
			}
		}
	}

	// Set up scaling callback for monitoring algorithm
	lf.monitoringAlgorithm.SetScalingCallback(lf.scaleService)

	// Initialize YAML updater to continuously update deployment YAML files
	// Use k8s directory as the base path for YAML files
	k8sDir := "./k8s"
	if lf.config.OutputDirectory != "" {
		k8sDir = lf.config.OutputDirectory
	}

	lf.yamlUpdater = kubernetes.NewYAMLUpdater(
		k8sDir,
		lf.affinityGenerator,
		lf.scoringAlgorithm,
		lf.graph,
		lf.config.MonitoringInterval,
	)

	// Start YAML updater
	if err := lf.yamlUpdater.Start(); err != nil {
		log.Printf("Warning: Failed to start YAML updater: %v", err)
	} else {
		log.Println("YAML updater started successfully - will update deployment YAML files with affinity rules")
	}

	// Perform initial analysis and generate configurations
	if err := lf.performInitialAnalysis(); err != nil {
		return fmt.Errorf("failed to perform initial analysis: %v", err)
	}

	// Start monitoring
	if lf.prometheusMonitor != nil {
		if err := lf.prometheusMonitor.StartMonitoring(lf.graph, lf.monitoringAlgorithm); err != nil {
			log.Printf("Warning: Failed to start Prometheus monitoring: %v", err)
		}
	}

	if err := lf.monitoringAlgorithm.Start(); err != nil {
		return fmt.Errorf("failed to start monitoring algorithm: %v", err)
	}

	lf.isRunning = true
	log.Println("LEAD framework started successfully")

	return nil
}

// handleGraphUpdate handles service graph updates from discovery
func (lf *LEADFramework) handleGraphUpdate(updatedGraph *models.ServiceGraph) {
	log.Println("Service graph updated, performing re-analysis...")

	// Update the current graph
	lf.graph = updatedGraph

	// Re-initialize algorithms with updated graph
	lf.scoringAlgorithm = algorithms.NewScoringAlgorithm(updatedGraph)
	lf.affinityGenerator = algorithms.NewAffinityRuleGenerator(updatedGraph)

	// Perform re-analysis
	if err := lf.performInitialAnalysis(); err != nil {
		log.Printf("Failed to perform re-analysis after graph update: %v", err)
	}
}

// Stop stops the LEAD framework
func (lf *LEADFramework) Stop() {
	if !lf.isRunning {
		return
	}

	log.Println("Stopping LEAD framework...")

	if lf.serviceDiscovery != nil {
		lf.serviceDiscovery.Stop()
	}

	if lf.k8sClient != nil {
		lf.k8sClient.Stop()
	}

	if lf.monitoringAlgorithm != nil {
		lf.monitoringAlgorithm.Stop()
	}

	if lf.prometheusMonitor != nil {
		lf.prometheusMonitor.StopMonitoring()
	}

	if lf.enhancedNetworkMonitor != nil {
		lf.enhancedNetworkMonitor.Stop()
	}

	if lf.yamlUpdater != nil {
		lf.yamlUpdater.Stop()
	}

	lf.cancel()
	lf.isRunning = false

	log.Println("LEAD framework stopped")
}

// performInitialAnalysis performs the initial analysis and generates Kubernetes configurations
func (lf *LEADFramework) performInitialAnalysis() error {
	log.Println("Performing initial service mesh analysis...")

	// Ensure network topology is populated before path scoring
	// This ensures NetworkScore is calculated correctly
	if lf.enhancedNetworkMonitor != nil {
		lf.enhancedNetworkMonitor.UpdateServiceGraphNetworkTopology()
	}

	// Step 1: Score all paths using Algorithm 1
	gateway := lf.graph.Gateway
	paths, err := lf.scoringAlgorithm.ScorePaths(gateway)
	if err != nil {
		return fmt.Errorf("failed to score paths: %v", err)
	}

	log.Printf("Found and scored %d critical paths", len(paths))

	// Print top critical paths
	lf.printCriticalPaths(paths)

	// Step 2: Generate Kubernetes deployment configurations
	if err := lf.kubernetesGenerator.GenerateDeploymentConfigs(lf.graph, paths); err != nil {
		return fmt.Errorf("failed to generate deployment configurations: %v", err)
	}

	// Step 3: Generate Kubernetes service configurations
	if err := lf.kubernetesGenerator.GenerateServiceManifests(lf.graph); err != nil {
		return fmt.Errorf("failed to generate service manifests: %v", err)
	}

	// Step 4: Perform network topology analysis
	if analysis, err := lf.scoringAlgorithm.AnalyzeNetworkTopology(gateway); err == nil {
		lf.printNetworkTopologyAnalysis(analysis)
	}

	log.Println("Initial analysis completed successfully")
	return nil
}

// printCriticalPaths prints the top critical paths
func (lf *LEADFramework) printCriticalPaths(paths []*models.Path) {
	log.Println("\n=== Critical Paths Analysis ===")

	topN := 5
	if len(paths) < topN {
		topN = len(paths)
	}

	for i := 0; i < topN; i++ {
		path := paths[i]
		log.Printf("Path %d (Score: %.2f, Weight: %d): %v",
			i+1, path.Score, path.Weight, path.GetServiceNames())
		log.Printf("  Length: %d, Pods: %d, Edges: %d, Network Score: %.2f",
			path.PathLength, path.PodCount, path.EdgeCount, path.NetworkScore)
	}

	log.Println("===============================")
}

// printNetworkTopologyAnalysis prints network topology analysis
func (lf *LEADFramework) printNetworkTopologyAnalysis(analysis *algorithms.NetworkTopologyAnalysis) {
	log.Println("\n=== Network Topology Analysis ===")
	log.Printf("Total Paths: %d", analysis.TotalPaths)
	log.Printf("Average Bandwidth: %.2f Mbps", analysis.AvgBandwidth)
	log.Printf("Average Hops: %.2f", analysis.AvgHops)
	log.Printf("Average Geo Distance: %.2f km", analysis.AvgGeoDistance)
	log.Println("Availability Zones:")
	for az, count := range analysis.AvailabilityZones {
		log.Printf("  %s: %d services", az, count)
	}
	log.Println("================================")
}

// scaleService scales a service (callback for monitoring algorithm)
func (lf *LEADFramework) scaleService(serviceID string, currentReplicas, targetReplicas int) error {
	log.Printf("Scaling service %s from %d to %d replicas", serviceID, currentReplicas, targetReplicas)

	// Update the service graph
	if service, exists := lf.graph.GetNode(serviceID); exists {
		service.Replicas = targetReplicas

		// Re-generate Kubernetes configurations with updated replicas
		gateway := lf.graph.Gateway
		if paths, err := lf.scoringAlgorithm.ScorePaths(gateway); err == nil {
			if err := lf.kubernetesGenerator.GenerateDeploymentConfigs(lf.graph, paths); err != nil {
				log.Printf("Warning: Failed to regenerate deployment configs: %v", err)
			}
		}

		log.Printf("Successfully updated service %s replicas to %d", serviceID, targetReplicas)
		return nil
	}

	return fmt.Errorf("service %s not found in graph", serviceID)
}

// GetCriticalPaths returns the current critical paths
func (lf *LEADFramework) GetCriticalPaths(topN int) ([]*models.Path, error) {
	if lf.scoringAlgorithm == nil {
		return nil, fmt.Errorf("scoring algorithm not initialized")
	}

	return lf.scoringAlgorithm.GetCriticalPaths(lf.graph.Gateway, topN)
}

// GetServiceHealth returns health status for a specific service
func (lf *LEADFramework) GetServiceHealth(serviceID string) (*algorithms.ServiceMetrics, error) {
	if lf.monitoringAlgorithm == nil {
		return nil, fmt.Errorf("monitoring algorithm not initialized")
	}

	metrics, exists := lf.monitoringAlgorithm.GetMetrics(serviceID)
	if !exists {
		return nil, fmt.Errorf("no metrics available for service %s", serviceID)
	}

	return metrics, nil
}

// GetClusterHealth returns overall cluster health
func (lf *LEADFramework) GetClusterHealth() (*algorithms.HealthSummary, error) {
	if lf.monitoringAlgorithm == nil {
		return nil, fmt.Errorf("monitoring algorithm not initialized")
	}

	return lf.monitoringAlgorithm.GetHealthSummary(), nil
}

// GetNetworkTopologyAnalysis returns network topology analysis
func (lf *LEADFramework) GetNetworkTopologyAnalysis() (*algorithms.NetworkTopologyAnalysis, error) {
	if lf.scoringAlgorithm == nil {
		return nil, fmt.Errorf("scoring algorithm not initialized")
	}

	return lf.scoringAlgorithm.AnalyzeNetworkTopology(lf.graph.Gateway)
}

// TriggerReanalysis triggers a re-analysis of the service mesh
func (lf *LEADFramework) TriggerReanalysis() {
	if !lf.isRunning {
		log.Println("Framework not running, cannot trigger re-analysis")
		return
	}

	log.Println("Triggering service mesh re-analysis...")

	// Perform re-analysis in a goroutine to avoid blocking
	go func() {
		if err := lf.performInitialAnalysis(); err != nil {
			log.Printf("Failed to perform re-analysis: %v", err)
		} else {
			log.Println("Re-analysis completed successfully")
		}
	}()
}

// Reanalyze triggers a re-analysis of the service mesh
func (lf *LEADFramework) Reanalyze() error {
	if !lf.isRunning {
		return fmt.Errorf("LEAD framework is not running")
	}

	log.Println("Triggering service mesh re-analysis...")
	return lf.performInitialAnalysis()
}

// UpdateConfiguration updates the framework configuration
func (lf *LEADFramework) UpdateConfiguration(config *FrameworkConfig) error {
	if lf.isRunning {
		return fmt.Errorf("cannot update configuration while framework is running")
	}

	lf.config = config
	return nil
}

// GetConfiguration returns the current framework configuration
func (lf *LEADFramework) GetConfiguration() *FrameworkConfig {
	return lf.config
}

// IsRunning returns whether the framework is currently running
func (lf *LEADFramework) IsRunning() bool {
	return lf.isRunning
}

// GetServiceGraph returns the current service graph
func (lf *LEADFramework) GetServiceGraph() *models.ServiceGraph {
	return lf.graph
}

// GetFrameworkStatus returns the current status of the framework
func (lf *LEADFramework) GetFrameworkStatus() *FrameworkStatus {
	status := &FrameworkStatus{
		IsRunning:         lf.isRunning,
		TotalServices:     len(lf.graph.GetAllNodes()),
		Gateway:           lf.graph.Gateway,
		PrometheusEnabled: lf.prometheusMonitor != nil,
		Configuration:     lf.config,
		LastAnalysis:      time.Now(),
	}

	// Get cluster health if monitoring is available
	if lf.monitoringAlgorithm != nil {
		status.ClusterHealth = lf.monitoringAlgorithm.GetHealthSummary()
	}

	return status
}

// createDefaultStaticGraph creates a default static service graph based on the architecture
// This matches the static dependency graph: fe->{src,usr,rcm,rsv}, src->{prf,geo,rte}, etc.
func (lf *LEADFramework) createDefaultStaticGraph() *models.ServiceGraph {
	graph := models.NewServiceGraph()

	// Create service nodes based on the static architecture
	services := []*models.ServiceNode{
		{ID: "frontend", Name: "frontend", Replicas: 1, RPS: 1000},
		{ID: "search", Name: "search", Replicas: 1, RPS: 500},
		{ID: "user", Name: "user", Replicas: 1, RPS: 300},
		{ID: "recommendation", Name: "recommendation", Replicas: 1, RPS: 200},
		{ID: "reservation", Name: "reservation", Replicas: 1, RPS: 400},
		{ID: "profile", Name: "profile", Replicas: 1, RPS: 250},
		{ID: "geo", Name: "geo", Replicas: 1, RPS: 150},
		{ID: "rate", Name: "rate", Replicas: 1, RPS: 350},
		{ID: "mongodb-profile", Name: "mongodb-profile", Replicas: 1, RPS: 0},
		{ID: "memcached-profile", Name: "memcached-profile", Replicas: 1, RPS: 0},
		{ID: "mongodb-rate", Name: "mongodb-rate", Replicas: 1, RPS: 0},
		{ID: "memcached-rate", Name: "memcached-rate", Replicas: 1, RPS: 0},
		{ID: "mongodb-user", Name: "mongodb-user", Replicas: 1, RPS: 0},
		{ID: "mongodb-geo", Name: "mongodb-geo", Replicas: 1, RPS: 0},
		{ID: "mongodb-recommendation", Name: "mongodb-recommendation", Replicas: 1, RPS: 0},
		{ID: "mongodb-reservation", Name: "mongodb-reservation", Replicas: 1, RPS: 0},
		{ID: "memcached-reservation", Name: "memcached-reservation", Replicas: 1, RPS: 0},
	}

	// Add all nodes to graph
	for _, service := range services {
		graph.AddNode(service)
	}

	// Set gateway
	graph.SetGateway("frontend")

	// Add edges based on static dependency graph
	// fe -> {src, usr, rcm, rsv}
	graph.AddEdge("frontend", "search")
	graph.AddEdge("frontend", "user")
	graph.AddEdge("frontend", "recommendation")
	graph.AddEdge("frontend", "reservation")

	// src -> {prf, geo, rte}
	graph.AddEdge("search", "profile")
	graph.AddEdge("search", "geo")
	graph.AddEdge("search", "rate")

	// prf -> {prf-mc, prf-db}
	graph.AddEdge("profile", "memcached-profile")
	graph.AddEdge("profile", "mongodb-profile")

	// geo -> geo-db
	graph.AddEdge("geo", "mongodb-geo")

	// usr -> usr-db
	graph.AddEdge("user", "mongodb-user")

	// rte -> {rte-mc, rte-db}
	graph.AddEdge("rate", "memcached-rate")
	graph.AddEdge("rate", "mongodb-rate")

	// rcm -> rcm-db
	graph.AddEdge("recommendation", "mongodb-recommendation")

	// rsv -> {rsv-mc, rsv-db}
	graph.AddEdge("reservation", "memcached-reservation")
	graph.AddEdge("reservation", "mongodb-reservation")

	return graph
}

// NetworkMetricsProviderAdapter adapts EnhancedPrometheusNetworkMonitor to models.InterNodeMetricsProvider
type NetworkMetricsProviderAdapter struct {
	monitor *monitoring.EnhancedPrometheusNetworkMonitor
}

// GetAverageInterNodeMetrics implements models.InterNodeMetricsProvider
func (a *NetworkMetricsProviderAdapter) GetAverageInterNodeMetrics() (latency float64, bandwidth float64, geoDistance float64) {
	return a.monitor.GetAverageInterNodeMetrics()
}

// GetInterNodeMetrics implements models.InterNodeMetricsProvider
func (a *NetworkMetricsProviderAdapter) GetInterNodeMetrics(node1, node2 string) (latency float64, bandwidth float64, geoDistance float64, exists bool) {
	return a.monitor.GetInterNodeMetricsForPathFinder(node1, node2)
}

// FrameworkStatus represents the current status of the LEAD framework
type FrameworkStatus struct {
	IsRunning         bool                      `json:"is_running"`
	TotalServices     int                       `json:"total_services"`
	Gateway           string                    `json:"gateway"`
	PrometheusEnabled bool                      `json:"prometheus_enabled"`
	Configuration     *FrameworkConfig          `json:"configuration"`
	ClusterHealth     *algorithms.HealthSummary `json:"cluster_health,omitempty"`
	LastAnalysis      time.Time                 `json:"last_analysis"`
}
