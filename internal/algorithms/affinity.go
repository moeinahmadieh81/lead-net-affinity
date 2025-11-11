package algorithms

import (
	"fmt"
	"sort"

	"lead-framework/internal/models"
)

// AffinityRuleGenerator implements Algorithm 3 from the LEAD framework
type AffinityRuleGenerator struct {
	graph          *models.ServiceGraph
	networkMonitor NetworkMetricsProvider // Interface for getting network metrics
}

// NetworkMetricsProvider provides network metrics from Cilium/Prometheus
type NetworkMetricsProvider interface {
	GetInterNodeMetrics(node1, node2 string) (*InterNodeMetrics, bool)
	GetNodeNetworkInfo(nodeName string) (*NodeNetworkInfo, bool)
}

// InterNodeMetrics represents network metrics between two nodes
type InterNodeMetrics struct {
	Node1       string  `json:"node1"`
	Node2       string  `json:"node2"`
	Latency     float64 `json:"latency"`      // ms
	Bandwidth   float64 `json:"bandwidth"`    // Mbps
	Throughput  float64 `json:"throughput"`   // Mbps
	PacketLoss  float64 `json:"packet_loss"`  // percentage
	GeoDistance float64 `json:"geo_distance"` // km
}

// NodeNetworkInfo represents network information for a node
type NodeNetworkInfo struct {
	Bandwidth        float64 `json:"bandwidth"`   // Mbps
	Latency          float64 `json:"latency"`     // ms
	Throughput       float64 `json:"throughput"`  // Mbps
	PacketLoss       float64 `json:"packet_loss"` // percentage
	Region           string  `json:"region"`
	AvailabilityZone string  `json:"availability_zone"`
}

// AffinityRule represents a Kubernetes affinity rule
type AffinityRule struct {
	ServiceID     string `json:"service_id"`
	TargetService string `json:"target_service"`
	Weight        int    `json:"weight"`
	Type          string `json:"type"` // "preferred" or "required"
	Expression    string `json:"expression"`
}

// KubernetesAffinityConfig represents the complete affinity configuration for a service
type KubernetesAffinityConfig struct {
	ServiceID       string           `json:"service_id"`
	PodAffinity     *PodAffinity     `json:"pod_affinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"pod_anti_affinity,omitempty"`
	NodeAffinity    *NodeAffinity    `json:"node_affinity,omitempty"`
}

// PodAffinity represents pod affinity rules
type PodAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"required_during_scheduling_ignored_during_execution,omitempty"`
}

// PodAntiAffinity represents pod anti-affinity rules
type PodAntiAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"required_during_scheduling_ignored_during_execution,omitempty"`
}

// NodeAffinity represents node affinity rules
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `json:"required_during_scheduling_ignored_during_execution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
}

// WeightedPodAffinityTerm represents a weighted pod affinity term
type WeightedPodAffinityTerm struct {
	Weight          int             `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"pod_affinity_term"`
}

// PodAffinityTerm represents a pod affinity term
type PodAffinityTerm struct {
	LabelSelector *LabelSelector `json:"label_selector,omitempty"`
	Namespaces    []string       `json:"namespaces,omitempty"`
	TopologyKey   string         `json:"topology_key"`
}

// LabelSelector represents a label selector
type LabelSelector struct {
	MatchLabels      map[string]string          `json:"match_labels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"match_expressions,omitempty"`
}

// LabelSelectorRequirement represents a label selector requirement
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// NodeSelector represents a node selector
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"node_selector_terms"`
}

// NodeSelectorTerm represents a node selector term
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"match_expressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"match_fields,omitempty"`
}

// NodeSelectorRequirement represents a node selector requirement
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTerm represents a preferred scheduling term
type PreferredSchedulingTerm struct {
	Weight     int              `json:"weight"`
	Preference NodeSelectorTerm `json:"preference"`
}

// NewAffinityRuleGenerator creates a new affinity rule generator
func NewAffinityRuleGenerator(graph *models.ServiceGraph) *AffinityRuleGenerator {
	return &AffinityRuleGenerator{
		graph:          graph,
		networkMonitor: nil, // Can be set later via SetNetworkMonitor
	}
}

// SetNetworkMonitor sets the network metrics provider
func (arg *AffinityRuleGenerator) SetNetworkMonitor(monitor NetworkMetricsProvider) {
	arg.networkMonitor = monitor
}

// GenerateAffinityRules implements Algorithm 3: Affinity rule generator
// This algorithm applies Kubernetes affinity rules in "Preferred" mode to co-locate interacting services
func (arg *AffinityRuleGenerator) GenerateAffinityRules(path *models.Path, weight int) ([]*KubernetesAffinityConfig, error) {
	if path == nil || len(path.Services) == 0 {
		return nil, fmt.Errorf("invalid path provided")
	}

	// Step 1: Sort services in ascending order to ensure child services are placed behind their parent services
	services := make([]*models.ServiceNode, len(path.Services))
	copy(services, path.Services)
	sort.Slice(services, func(i, j int) bool {
		return services[i].ID < services[j].ID // Simple lexicographic sort
	})

	var affinityConfigs []*KubernetesAffinityConfig

	// Step 2: Iterate over each service and generate affinity rules for adjacent pairs
	for i := 0; i < len(services)-1; i++ {
		// Step 3: Check if index is even (modulo operation)
		if i%2 == 0 {
			currentService := services[i]
			nextService := services[i+1]

			// Generate affinity rule for current service and its immediate successor
			affinityConfig, err := arg.generateAffinityRule(currentService, nextService, weight)
			if err != nil {
				return nil, fmt.Errorf("failed to generate affinity rule for services %s and %s: %v",
					currentService.ID, nextService.ID, err)
			}

			affinityConfigs = append(affinityConfigs, affinityConfig)
		}
	}

	return affinityConfigs, nil
}

// generateAffinityRule generates an affinity rule for two services
// Enhanced to use Cilium/Prometheus metrics for better placement decisions
func (arg *AffinityRuleGenerator) generateAffinityRule(service1, service2 *models.ServiceNode, weight int) (*KubernetesAffinityConfig, error) {
	config := &KubernetesAffinityConfig{
		ServiceID: service1.ID,
	}

	// Calculate affinity weight based on network metrics and service dependencies
	affinityWeight := arg.calculateAffinityWeight(service1, service2, weight)

	// Generate pod affinity rule to co-locate with service2
	// Use zone-level affinity if latency is low, node-level if very low
	topologyKey := arg.determineTopologyKey(service1, service2)

	podAffinity := &PodAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []WeightedPodAffinityTerm{
			{
				Weight: affinityWeight,
				PodAffinityTerm: PodAffinityTerm{
					LabelSelector: &LabelSelector{
						MatchLabels: map[string]string{
							"io.kompose.service": service2.ID,
							"app":                service2.Name,
						},
					},
					TopologyKey: topologyKey,
				},
			},
		},
	}

	config.PodAffinity = podAffinity

	// Generate node affinity based on network topology and Cilium metrics
	nodeAffinity := arg.generateNodeAffinity(service1, service2, affinityWeight)
	if nodeAffinity != nil {
		config.NodeAffinity = nodeAffinity
	}

	// Generate anti-affinity for services that should be distributed
	// (e.g., same service replicas should be on different nodes)
	antiAffinity := arg.generateAntiAffinityForService(service1, weight)
	if antiAffinity != nil {
		config.PodAntiAffinity = antiAffinity
	}

	return config, nil
}

// calculateAffinityWeight calculates the weight for affinity rules based on:
// - Network latency between services (lower latency = higher weight)
// - Service dependencies (frequent communication = higher weight)
// - Network bandwidth (higher bandwidth = higher weight)
func (arg *AffinityRuleGenerator) calculateAffinityWeight(service1, service2 *models.ServiceNode, baseWeight int) int {
	weight := baseWeight

	// If we have network metrics, adjust weight based on latency
	if arg.networkMonitor != nil {
		// Try to get inter-node metrics if services are on different nodes
		if service1.NetworkTopology != nil && service2.NetworkTopology != nil {
			// Use latency to adjust weight: lower latency = higher affinity weight
			avgLatency := (service1.NetworkTopology.Latency + service2.NetworkTopology.Latency) / 2

			// If latency is very low (< 10ms), increase weight significantly
			if avgLatency < 10 {
				weight = int(float64(weight) * 1.5)
			} else if avgLatency < 50 {
				// Moderate latency (10-50ms), increase weight moderately
				weight = int(float64(weight) * 1.2)
			} else if avgLatency > 100 {
				// High latency (> 100ms), reduce weight
				weight = int(float64(weight) * 0.8)
			}

			// Adjust based on bandwidth: higher bandwidth = higher weight
			avgBandwidth := (service1.NetworkTopology.Bandwidth + service2.NetworkTopology.Bandwidth) / 2
			if avgBandwidth > 800 {
				weight = int(float64(weight) * 1.1)
			}
		}
	}

	// Ensure weight is within valid range (1-100)
	if weight < 1 {
		weight = 1
	} else if weight > 100 {
		weight = 100
	}

	return weight
}

// determineTopologyKey determines the best topology key for affinity rules
// - "kubernetes.io/hostname" for node-level co-location (lowest latency)
// - "topology.kubernetes.io/zone" for zone-level co-location (moderate latency)
func (arg *AffinityRuleGenerator) determineTopologyKey(service1, service2 *models.ServiceNode) string {
	// If both services have network topology, use latency to decide
	if service1.NetworkTopology != nil && service2.NetworkTopology != nil {
		avgLatency := (service1.NetworkTopology.Latency + service2.NetworkTopology.Latency) / 2

		// Very low latency (< 5ms) suggests same node or very close nodes
		if avgLatency < 5 {
			return "kubernetes.io/hostname" // Prefer same node
		}

		// Low latency (< 20ms) suggests same zone
		if avgLatency < 20 {
			return "topology.kubernetes.io/zone" // Prefer same zone
		}
	}

	// Default to zone-level for better distribution
	return "topology.kubernetes.io/zone"
}

// generateAntiAffinityForService generates anti-affinity rules to distribute service replicas
func (arg *AffinityRuleGenerator) generateAntiAffinityForService(service *models.ServiceNode, weight int) *PodAntiAffinity {
	// Only generate anti-affinity if service has multiple replicas
	if service.Replicas <= 1 {
		return nil
	}

	// Use preferred anti-affinity to distribute replicas across nodes
	return &PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []WeightedPodAffinityTerm{
			{
				Weight: weight / 2, // Lower weight than affinity
				PodAffinityTerm: PodAffinityTerm{
					LabelSelector: &LabelSelector{
						MatchLabels: map[string]string{
							"io.kompose.service": service.ID,
							"app":                service.Name,
						},
					},
					TopologyKey: "kubernetes.io/hostname", // Distribute across nodes
				},
			},
		},
	}
}

// generateNodeAffinity generates node affinity rules based on network topology and Cilium metrics
// Enhanced to use inter-node latency and bandwidth from Cilium/Prometheus
func (arg *AffinityRuleGenerator) generateNodeAffinity(service1, service2 *models.ServiceNode, weight int) *NodeAffinity {
	nodeAffinity := &NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []PreferredSchedulingTerm{},
	}

	// If both services have network topology information, use it for placement
	if service1.NetworkTopology != nil && service2.NetworkTopology != nil {
		// Prefer same availability zone if latency is low
		if service1.NetworkTopology.AvailabilityZone == service2.NetworkTopology.AvailabilityZone {
			avgLatency := (service1.NetworkTopology.Latency + service2.NetworkTopology.Latency) / 2

			// Only add zone preference if latency is acceptable
			if avgLatency < 50 {
				nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
					nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
					PreferredSchedulingTerm{
						Weight: weight,
						Preference: NodeSelectorTerm{
							MatchExpressions: []NodeSelectorRequirement{
								{
									Key:      "topology.kubernetes.io/zone",
									Operator: "In",
									Values:   []string{service1.NetworkTopology.AvailabilityZone},
								},
							},
						},
					},
				)
			}
		}

		// Add bandwidth-based preferences for high-bandwidth services
		avgBandwidth := (service1.NetworkTopology.Bandwidth + service2.NetworkTopology.Bandwidth) / 2
		if avgBandwidth > 500 {
			// Prefer nodes with high bandwidth capabilities
			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				PreferredSchedulingTerm{
					Weight: weight / 2, // Lower weight for bandwidth preference
					Preference: NodeSelectorTerm{
						MatchExpressions: []NodeSelectorRequirement{
							{
								Key:      "node.kubernetes.io/instance-type",
								Operator: "In",
								Values:   []string{"m5.large", "m5.xlarge", "c5.large", "c5.xlarge", "m5.2xlarge", "c5.2xlarge"},
							},
						},
					},
				},
			)
		}

		// Use Cilium metrics if available for more precise placement
		if arg.networkMonitor != nil {
			// Try to get inter-node metrics to make better decisions
			// This would require node names, which we might need to get from service topology
			// For now, we use the network topology data we have
		}
	}

	// If no preferences were added, return nil
	if len(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) == 0 {
		return nil
	}

	return nodeAffinity
}

// GenerateAffinityRulesForPaths generates affinity rules for multiple paths
// Also considers direct dependencies from the static graph for better co-location
func (arg *AffinityRuleGenerator) GenerateAffinityRulesForPaths(paths []*models.Path) (map[string]*KubernetesAffinityConfig, error) {
	allConfigs := make(map[string]*KubernetesAffinityConfig)

	// First, generate affinity rules from paths
	for _, path := range paths {
		configs, err := arg.GenerateAffinityRules(path, path.Weight)
		if err != nil {
			return nil, fmt.Errorf("failed to generate affinity rules for path: %v", err)
		}

		// Merge configurations for the same service
		for _, config := range configs {
			existingConfig, exists := allConfigs[config.ServiceID]
			if exists {
				// Merge with existing configuration
				mergedConfig := arg.mergeAffinityConfigs(existingConfig, config)
				allConfigs[config.ServiceID] = mergedConfig
			} else {
				allConfigs[config.ServiceID] = config
			}
		}
	}

	// Additionally, generate affinity rules based on direct dependencies in the static graph
	// This ensures services that directly depend on each other get co-located
	arg.generateAffinityFromDirectDependencies(allConfigs)

	return allConfigs, nil
}

// generateAffinityFromDirectDependencies generates affinity rules based on direct dependencies
// in the static service graph. This ensures parent-child service pairs are co-located.
// Uses the static dependency graph: fe->{src,usr,rcm,rsv}, src->{prf,geo,rte}, etc.
// Note: Consul is excluded from affinity rules as its placement doesn't matter.
func (arg *AffinityRuleGenerator) generateAffinityFromDirectDependencies(allConfigs map[string]*KubernetesAffinityConfig) {
	if arg.graph == nil {
		return
	}

	// Get all nodes from the graph
	nodes := arg.graph.GetAllNodes()

	// Services to exclude from affinity rules (infrastructure services)
	excludedServices := map[string]bool{
		"consul": true, // Consul placement doesn't matter
	}

	// For each service, check its direct dependencies
	for serviceID, serviceNode := range nodes {
		// Skip excluded services
		if excludedServices[serviceID] {
			continue
		}

		// Get direct dependencies (children) from the graph
		// These are services that this service depends on (static dependency graph)
		dependencies := arg.graph.GetChildren(serviceID)

		// Generate affinity rules for each direct dependency
		for _, depID := range dependencies {
			// Skip excluded dependencies
			if excludedServices[depID] {
				continue
			}

			if depNode, exists := nodes[depID]; exists {
				// Generate affinity rule to co-locate with dependency
				// Use high weight (80) for direct dependencies as they communicate frequently
				// This ensures services like fe->src, src->prf, prf->prf-db get co-located
				affinityConfig, err := arg.generateAffinityRule(serviceNode, depNode, 80)
				if err != nil {
					continue
				}

				// Merge with existing config if present
				existingConfig, exists := allConfigs[serviceID]
				if exists {
					mergedConfig := arg.mergeAffinityConfigs(existingConfig, affinityConfig)
					allConfigs[serviceID] = mergedConfig
				} else {
					allConfigs[serviceID] = affinityConfig
				}
			}
		}
	}
}

// mergeAffinityConfigs merges two affinity configurations for the same service
func (arg *AffinityRuleGenerator) mergeAffinityConfigs(config1, config2 *KubernetesAffinityConfig) *KubernetesAffinityConfig {
	merged := &KubernetesAffinityConfig{
		ServiceID: config1.ServiceID,
	}

	// Merge pod affinity rules
	if config1.PodAffinity != nil || config2.PodAffinity != nil {
		merged.PodAffinity = &PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []WeightedPodAffinityTerm{},
		}

		if config1.PodAffinity != nil {
			merged.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				merged.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				config1.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution...,
			)
		}

		if config2.PodAffinity != nil {
			merged.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				merged.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				config2.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution...,
			)
		}
	}

	// Merge node affinity rules
	if config1.NodeAffinity != nil || config2.NodeAffinity != nil {
		merged.NodeAffinity = &NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []PreferredSchedulingTerm{},
		}

		if config1.NodeAffinity != nil {
			merged.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				merged.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				config1.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...,
			)
		}

		if config2.NodeAffinity != nil {
			merged.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				merged.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				config2.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...,
			)
		}
	}

	return merged
}

// GenerateAntiAffinityRules generates anti-affinity rules to distribute services across nodes
func (arg *AffinityRuleGenerator) GenerateAntiAffinityRules(serviceID string, weight int) *KubernetesAffinityConfig {
	config := &KubernetesAffinityConfig{
		ServiceID: serviceID,
		PodAntiAffinity: &PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []WeightedPodAffinityTerm{
				{
					Weight: weight,
					PodAffinityTerm: PodAffinityTerm{
						LabelSelector: &LabelSelector{
							MatchLabels: map[string]string{
								"app": serviceID,
							},
						},
						TopologyKey: "kubernetes.io/hostname", // Distribute across nodes
					},
				},
			},
		},
	}

	return config
}

// ValidateAffinityConfig validates an affinity configuration
func (arg *AffinityRuleGenerator) ValidateAffinityConfig(config *KubernetesAffinityConfig) error {
	if config == nil {
		return fmt.Errorf("affinity config is nil")
	}

	if config.ServiceID == "" {
		return fmt.Errorf("service ID is empty")
	}

	// Validate pod affinity
	if config.PodAffinity != nil {
		for _, term := range config.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if term.Weight <= 0 || term.Weight > 100 {
				return fmt.Errorf("invalid weight %d, must be between 1 and 100", term.Weight)
			}
			if term.PodAffinityTerm.TopologyKey == "" {
				return fmt.Errorf("topology key is required")
			}
		}
	}

	// Validate node affinity
	if config.NodeAffinity != nil {
		for _, term := range config.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if term.Weight <= 0 || term.Weight > 100 {
				return fmt.Errorf("invalid weight %d, must be between 1 and 100", term.Weight)
			}
		}
	}

	return nil
}
