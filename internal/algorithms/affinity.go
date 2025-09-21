package algorithms

import (
	"fmt"
	"sort"

	"lead-framework/internal/models"
)

// AffinityRuleGenerator implements Algorithm 3 from the LEAD framework
type AffinityRuleGenerator struct {
	graph *models.ServiceGraph
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
	return &AffinityRuleGenerator{graph: graph}
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
func (arg *AffinityRuleGenerator) generateAffinityRule(service1, service2 *models.ServiceNode, weight int) (*KubernetesAffinityConfig, error) {
	config := &KubernetesAffinityConfig{
		ServiceID: service1.ID,
	}

	// Generate pod affinity rule to co-locate with service2
	podAffinity := &PodAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []WeightedPodAffinityTerm{
			{
				Weight: weight,
				PodAffinityTerm: PodAffinityTerm{
					LabelSelector: &LabelSelector{
						MatchLabels: map[string]string{
							"app":     service2.Name,
							"service": service2.ID,
						},
					},
					TopologyKey: "kubernetes.io/hostname", // Co-locate on same node
				},
			},
		},
	}

	config.PodAffinity = podAffinity

	// Generate node affinity based on network topology if available
	if service1.NetworkTopology != nil || service2.NetworkTopology != nil {
		nodeAffinity := arg.generateNodeAffinity(service1, service2, weight)
		config.NodeAffinity = nodeAffinity
	}

	return config, nil
}

// generateNodeAffinity generates node affinity rules based on network topology
func (arg *AffinityRuleGenerator) generateNodeAffinity(service1, service2 *models.ServiceNode, weight int) *NodeAffinity {
	nodeAffinity := &NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []PreferredSchedulingTerm{},
	}

	// If both services have network topology information, try to co-locate in same AZ
	if service1.NetworkTopology != nil && service2.NetworkTopology != nil {
		if service1.NetworkTopology.AvailabilityZone == service2.NetworkTopology.AvailabilityZone {
			// Prefer same availability zone
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

	// Add bandwidth-based preferences
	if service1.NetworkTopology != nil && service1.NetworkTopology.Bandwidth > 500 {
		nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			PreferredSchedulingTerm{
				Weight: weight / 2, // Lower weight for bandwidth preference
				Preference: NodeSelectorTerm{
					MatchExpressions: []NodeSelectorRequirement{
						{
							Key:      "node.kubernetes.io/instance-type",
							Operator: "In",
							Values:   []string{"m5.large", "m5.xlarge", "c5.large", "c5.xlarge"}, // High bandwidth instance types
						},
					},
				},
			},
		)
	}

	return nodeAffinity
}

// GenerateAffinityRulesForPaths generates affinity rules for multiple paths
func (arg *AffinityRuleGenerator) GenerateAffinityRulesForPaths(paths []*models.Path) (map[string]*KubernetesAffinityConfig, error) {
	allConfigs := make(map[string]*KubernetesAffinityConfig)

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

	return allConfigs, nil
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
