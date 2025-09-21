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
	graph               *models.ServiceGraph
	scoringAlgorithm    *algorithms.ScoringAlgorithm
	monitoringAlgorithm *algorithms.MonitoringAlgorithm
	affinityGenerator   *algorithms.AffinityRuleGenerator
	kubernetesGenerator *kubernetes.KubernetesConfigGenerator
	prometheusMonitor   *monitoring.PrometheusMonitor
	serviceDiscovery    *discovery.ServiceDiscovery
	k8sClient           *kubernetes.KubernetesClient
	config              *FrameworkConfig
	ctx                 context.Context
	cancel              context.CancelFunc
	isRunning           bool
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
}

// DefaultFrameworkConfig returns default configuration for the LEAD framework
func DefaultFrameworkConfig() *FrameworkConfig {
	return &FrameworkConfig{
		MonitoringInterval:     30 * time.Second,
		ResourceThreshold:      80.0,
		LatencyThreshold:       100 * time.Millisecond,
		PrometheusURL:          "http://localhost:9090",
		KubernetesNamespace:    "default",
		OutputDirectory:        "./k8s-manifests",
		BandwidthWeight:        0.4,
		HopsWeight:             0.3,
		GeoDistanceWeight:      0.2,
		AvailabilityZoneWeight: 0.1,
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

	// Initialize Kubernetes client
	var err error
	lf.k8sClient, err = kubernetes.NewKubernetesClient(lf.config.KubernetesNamespace)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// Initialize service discovery
	lf.serviceDiscovery = discovery.NewServiceDiscovery(lf.k8sClient)

	// Set up graph update callback
	lf.serviceDiscovery.SetUpdateCallback(func(updatedGraph *models.ServiceGraph) {
		lf.handleGraphUpdate(updatedGraph)
	})

	// Start service discovery
	if err := lf.serviceDiscovery.Start(); err != nil {
		return fmt.Errorf("failed to start service discovery: %v", err)
	}

	// Get the discovered service graph
	lf.graph = lf.serviceDiscovery.GetServiceGraph()

	// If no services discovered yet, use the provided graph as fallback
	if lf.graph == nil || len(lf.graph.GetAllNodes()) == 0 {
		if graph != nil {
			log.Println("No services discovered, using provided graph as fallback")
			lf.graph = graph
		} else {
			return fmt.Errorf("no services discovered and no fallback graph provided")
		}
	}

	log.Println("Initializing LEAD framework components...")

	// Initialize algorithms
	lf.scoringAlgorithm = algorithms.NewScoringAlgorithm(graph)
	lf.monitoringAlgorithm = algorithms.NewMonitoringAlgorithm(
		graph,
		lf.scoringAlgorithm,
		lf.config.ResourceThreshold,
		lf.config.MonitoringInterval,
	)
	lf.affinityGenerator = algorithms.NewAffinityRuleGenerator(graph)

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
		}
	}

	// Set up scaling callback for monitoring algorithm
	lf.monitoringAlgorithm.SetScalingCallback(lf.scaleService)

	// Perform initial analysis and generate configurations
	if err := lf.performInitialAnalysis(); err != nil {
		return fmt.Errorf("failed to perform initial analysis: %v", err)
	}

	// Start monitoring
	if lf.prometheusMonitor != nil {
		if err := lf.prometheusMonitor.StartMonitoring(graph, lf.monitoringAlgorithm); err != nil {
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

	lf.cancel()
	lf.isRunning = false

	log.Println("LEAD framework stopped")
}

// performInitialAnalysis performs the initial analysis and generates Kubernetes configurations
func (lf *LEADFramework) performInitialAnalysis() error {
	log.Println("Performing initial service mesh analysis...")

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
