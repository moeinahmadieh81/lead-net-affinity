package tests

import (
	"testing"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/models"
)

func TestAffinityRuleGeneration(t *testing.T) {
	graph := createTestGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	// Create a test path
	path := &models.Path{
		Services: []*models.ServiceNode{
			{ID: "fe", Name: "frontend"},
			{ID: "src", Name: "search"},
			{ID: "usr", Name: "user"},
		},
		Weight: 100,
	}

	// Generate affinity rules
	configs, err := affinityGen.GenerateAffinityRules(path, 100)
	if err != nil {
		t.Fatalf("Failed to generate affinity rules: %v", err)
	}

	// Verify that affinity rules are generated for even-indexed services only
	// (Algorithm 3: only for i % 2 == 0)
	expectedConfigs := 1 // Only for service at index 0 (fe -> src)
	if len(configs) != expectedConfigs {
		t.Errorf("Expected %d affinity configs, got %d", expectedConfigs, len(configs))
	}

	// Verify the generated configuration
	if len(configs) > 0 {
		config := configs[0]
		if config.ServiceID != "fe" {
			t.Errorf("Expected service ID 'fe', got '%s'", config.ServiceID)
		}

		if config.PodAffinity == nil {
			t.Error("Expected pod affinity to be set")
		}

		if config.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
			t.Error("Expected preferred pod affinity rules")
		}
	}
}

func TestAffinityRuleValidation(t *testing.T) {
	affinityGen := algorithms.NewAffinityRuleGenerator(nil)

	// Test valid configuration
	validConfig := &algorithms.KubernetesAffinityConfig{
		ServiceID: "test-service",
		PodAffinity: &algorithms.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []algorithms.WeightedPodAffinityTerm{
				{
					Weight: 50,
					PodAffinityTerm: algorithms.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}

	if err := affinityGen.ValidateAffinityConfig(validConfig); err != nil {
		t.Errorf("Valid config should not fail validation: %v", err)
	}

	// Test invalid configurations
	invalidConfigs := []struct {
		name   string
		config *algorithms.KubernetesAffinityConfig
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "empty service ID",
			config: &algorithms.KubernetesAffinityConfig{
				ServiceID: "",
			},
		},
		{
			name: "invalid weight",
			config: &algorithms.KubernetesAffinityConfig{
				ServiceID: "test",
				PodAffinity: &algorithms.PodAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []algorithms.WeightedPodAffinityTerm{
						{
							Weight: 150, // Invalid weight > 100
							PodAffinityTerm: algorithms.PodAffinityTerm{
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
		},
		{
			name: "missing topology key",
			config: &algorithms.KubernetesAffinityConfig{
				ServiceID: "test",
				PodAffinity: &algorithms.PodAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []algorithms.WeightedPodAffinityTerm{
						{
							Weight: 50,
							PodAffinityTerm: algorithms.PodAffinityTerm{
								TopologyKey: "", // Missing topology key
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range invalidConfigs {
		t.Run(tc.name, func(t *testing.T) {
			if err := affinityGen.ValidateAffinityConfig(tc.config); err == nil {
				t.Errorf("Expected validation to fail for %s", tc.name)
			}
		})
	}
}

func TestAffinityRuleMerging(t *testing.T) {
	graph := createTestGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	// Create two paths with overlapping services
	path1 := &models.Path{
		Services: []*models.ServiceNode{
			{ID: "fe", Name: "frontend"},
			{ID: "src", Name: "search"},
		},
		Weight: 100,
	}

	path2 := &models.Path{
		Services: []*models.ServiceNode{
			{ID: "fe", Name: "frontend"},
			{ID: "usr", Name: "user"},
		},
		Weight: 90,
	}

	// Generate affinity rules for both paths
	configs, err := affinityGen.GenerateAffinityRulesForPaths([]*models.Path{path1, path2})
	if err != nil {
		t.Fatalf("Failed to generate affinity rules for paths: %v", err)
	}

	// Verify that configurations are merged for the same service
	if config, exists := configs["fe"]; exists {
		if config.PodAffinity == nil {
			t.Error("Expected pod affinity to be set for service 'fe'")
		}

		// Should have multiple affinity terms due to merging
		terms := config.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution
		if len(terms) < 2 {
			t.Errorf("Expected multiple affinity terms after merging, got %d", len(terms))
		}
	} else {
		t.Error("Expected configuration for service 'fe'")
	}
}

func TestNetworkTopologyBasedAffinity(t *testing.T) {
	graph := createTestGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	// Create a path with services in different availability zones
	path := &models.Path{
		Services: []*models.ServiceNode{
			{
				ID:   "fe",
				Name: "frontend",
				NetworkTopology: &models.NetworkTopology{
					AvailabilityZone: "us-west-1a",
					Bandwidth:        1000,
					Hops:             0,
					GeoDistance:      0,
				},
			},
			{
				ID:   "src",
				Name: "search",
				NetworkTopology: &models.NetworkTopology{
					AvailabilityZone: "us-west-1a", // Same AZ
					Bandwidth:        800,
					Hops:             1,
					GeoDistance:      0,
				},
			},
		},
		Weight: 100,
	}

	// Generate affinity rules
	configs, err := affinityGen.GenerateAffinityRules(path, 100)
	if err != nil {
		t.Fatalf("Failed to generate affinity rules: %v", err)
	}

	// Verify that node affinity is generated based on network topology
	if len(configs) > 0 {
		config := configs[0]
		if config.NodeAffinity == nil {
			t.Error("Expected node affinity to be generated for services with network topology")
		}

		if config.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
			t.Error("Expected preferred node affinity rules")
		}

		// Verify availability zone preference
		terms := config.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
		foundAZPreference := false
		for _, term := range terms {
			for _, expr := range term.Preference.MatchExpressions {
				if expr.Key == "topology.kubernetes.io/zone" {
					foundAZPreference = true
					break
				}
			}
		}

		if !foundAZPreference {
			t.Error("Expected availability zone preference in node affinity")
		}
	}
}

func BenchmarkAffinityRuleGeneration(b *testing.B) {
	graph := createTestGraph()
	affinityGen := algorithms.NewAffinityRuleGenerator(graph)

	path := &models.Path{
		Services: []*models.ServiceNode{
			{ID: "fe", Name: "frontend"},
			{ID: "src", Name: "search"},
			{ID: "usr", Name: "user"},
			{ID: "rcm", Name: "recommendation"},
		},
		Weight: 100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := affinityGen.GenerateAffinityRules(path, 100)
		if err != nil {
			b.Fatalf("Affinity rule generation failed: %v", err)
		}
	}
}
