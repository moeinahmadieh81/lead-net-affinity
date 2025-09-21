package kubernetes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lead-framework/internal/algorithms"
	"lead-framework/internal/models"
)

// KubernetesConfigGenerator generates Kubernetes deployment configurations
type KubernetesConfigGenerator struct {
	namespace   string
	outputDir   string
	affinityGen *algorithms.AffinityRuleGenerator
}

// NewKubernetesConfigGenerator creates a new Kubernetes config generator
func NewKubernetesConfigGenerator(namespace, outputDir string, affinityGen *algorithms.AffinityRuleGenerator) *KubernetesConfigGenerator {
	return &KubernetesConfigGenerator{
		namespace:   namespace,
		outputDir:   outputDir,
		affinityGen: affinityGen,
	}
}

// DeploymentConfig represents a Kubernetes deployment configuration
type DeploymentConfig struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   Metadata       `json:"metadata"`
	Spec       DeploymentSpec `json:"spec"`
}

// Metadata represents Kubernetes metadata
type Metadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DeploymentSpec represents deployment specification
type DeploymentSpec struct {
	Replicas int32              `json:"replicas"`
	Selector LabelSelector      `json:"selector"`
	Template PodTemplateSpec    `json:"template"`
	Strategy DeploymentStrategy `json:"strategy"`
}

// LabelSelector represents a label selector
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

// PodTemplateSpec represents pod template specification
type PodTemplateSpec struct {
	Metadata PodMetadata `json:"metadata"`
	Spec     PodSpec     `json:"spec"`
}

// PodMetadata represents pod metadata
type PodMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodSpec represents pod specification
type PodSpec struct {
	Containers    []Container       `json:"containers"`
	Affinity      *PodAffinity      `json:"affinity,omitempty"`
	NodeSelector  map[string]string `json:"nodeSelector,omitempty"`
	Tolerations   []Toleration      `json:"tolerations,omitempty"`
	RestartPolicy string            `json:"restartPolicy"`
	SchedulerName string            `json:"schedulerName,omitempty"`
}

// Container represents a container specification
type Container struct {
	Name           string                `json:"name"`
	Image          string                `json:"image"`
	Ports          []ContainerPort       `json:"ports,omitempty"`
	Resources      *ResourceRequirements `json:"resources,omitempty"`
	Env            []EnvVar              `json:"env,omitempty"`
	LivenessProbe  *Probe                `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe                `json:"readinessProbe,omitempty"`
	VolumeMounts   []VolumeMount         `json:"volumeMounts,omitempty"`
}

// ContainerPort represents a container port
type ContainerPort struct {
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	Name          string `json:"name,omitempty"`
}

// ResourceRequirements represents resource requirements
type ResourceRequirements struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

// ResourceList represents a list of resources
type ResourceList map[string]string

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Probe represents a probe configuration
type Probe struct {
	HTTPGet             *HTTPGetAction `json:"httpGet,omitempty"`
	InitialDelaySeconds int32          `json:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int32          `json:"timeoutSeconds,omitempty"`
	PeriodSeconds       int32          `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32          `json:"successThreshold,omitempty"`
	FailureThreshold    int32          `json:"failureThreshold,omitempty"`
}

// HTTPGetAction represents an HTTP GET action
type HTTPGetAction struct {
	Path string `json:"path"`
	Port int32  `json:"port"`
}

// VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// Toleration represents a toleration
type Toleration struct {
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
}

// DeploymentStrategy represents deployment strategy
type DeploymentStrategy struct {
	Type string `json:"type"`
}

// PodAffinity represents pod affinity configuration
type PodAffinity struct {
	PodAffinity     *algorithms.PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *algorithms.PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	NodeAffinity    *algorithms.NodeAffinity    `json:"nodeAffinity,omitempty"`
}

// GenerateDeploymentConfigs generates Kubernetes deployment configurations for all services
func (kcg *KubernetesConfigGenerator) GenerateDeploymentConfigs(graph *models.ServiceGraph, paths []*models.Path) error {
	// Generate affinity rules for all paths
	affinityConfigs, err := kcg.affinityGen.GenerateAffinityRulesForPaths(paths)
	if err != nil {
		return fmt.Errorf("failed to generate affinity rules: %v", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(kcg.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Generate deployment config for each service
	nodes := graph.GetAllNodes()
	for serviceID, node := range nodes {
		deploymentConfig := kcg.createDeploymentConfig(node, affinityConfigs[serviceID])

		// Write deployment config to file
		filename := filepath.Join(kcg.outputDir, fmt.Sprintf("%s-deployment.json", serviceID))
		if err := kcg.writeConfigToFile(deploymentConfig, filename); err != nil {
			return fmt.Errorf("failed to write deployment config for %s: %v", serviceID, err)
		}

		fmt.Printf("Generated deployment config for service %s\n", serviceID)
	}

	// Generate a combined manifest file
	combinedManifest := kcg.generateCombinedManifest(nodes, affinityConfigs)
	combinedFilename := filepath.Join(kcg.outputDir, "all-deployments.json")
	if err := kcg.writeConfigToFile(combinedManifest, combinedFilename); err != nil {
		return fmt.Errorf("failed to write combined manifest: %v", err)
	}

	fmt.Printf("Generated combined manifest: %s\n", combinedFilename)
	return nil
}

// createDeploymentConfig creates a deployment configuration for a service
func (kcg *KubernetesConfigGenerator) createDeploymentConfig(node *models.ServiceNode, affinityConfig *algorithms.KubernetesAffinityConfig) *DeploymentConfig {
	labels := map[string]string{
		"app":     node.Name,
		"service": node.ID,
	}

	// Add network topology labels
	if node.NetworkTopology != nil {
		labels["availability-zone"] = node.NetworkTopology.AvailabilityZone
		labels["network-tier"] = kcg.getNetworkTier(node.NetworkTopology.Bandwidth)
	}

	config := &DeploymentConfig{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: Metadata{
			Name:      node.ID,
			Namespace: kcg.namespace,
			Labels:    labels,
		},
		Spec: DeploymentSpec{
			Replicas: int32(node.Replicas),
			Selector: LabelSelector{
				MatchLabels: map[string]string{
					"app": node.ID,
				},
			},
			Template: PodTemplateSpec{
				Metadata: PodMetadata{
					Labels: map[string]string{
						"app":     node.ID,
						"service": node.ID,
					},
				},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name:  node.ID,
							Image: fmt.Sprintf("%s:latest", node.Name),
							Ports: []ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
							},
							Resources: kcg.getResourceRequirements(node),
							Env: []EnvVar{
								{
									Name:  "SERVICE_NAME",
									Value: node.Name,
								},
								{
									Name:  "SERVICE_ID",
									Value: node.ID,
								},
							},
							LivenessProbe: &Probe{
								HTTPGet: &HTTPGetAction{
									Path: "/health",
									Port: 8080,
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &Probe{
								HTTPGet: &HTTPGetAction{
									Path: "/ready",
									Port: 8080,
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					RestartPolicy: "Always",
				},
			},
			Strategy: DeploymentStrategy{
				Type: "RollingUpdate",
			},
		},
	}

	// Add affinity configuration if available
	if affinityConfig != nil {
		config.Spec.Template.Spec.Affinity = &PodAffinity{
			PodAffinity:     affinityConfig.PodAffinity,
			PodAntiAffinity: affinityConfig.PodAntiAffinity,
			NodeAffinity:    affinityConfig.NodeAffinity,
		}
	}

	// Add node selector based on network topology
	if node.NetworkTopology != nil {
		config.Spec.Template.Spec.NodeSelector = map[string]string{
			"topology.kubernetes.io/zone": node.NetworkTopology.AvailabilityZone,
		}
	}

	return config
}

// getNetworkTier determines network tier based on bandwidth
func (kcg *KubernetesConfigGenerator) getNetworkTier(bandwidth float64) string {
	switch {
	case bandwidth >= 1000:
		return "premium"
	case bandwidth >= 500:
		return "standard"
	default:
		return "basic"
	}
}

// getResourceRequirements calculates resource requirements based on service characteristics
func (kcg *KubernetesConfigGenerator) getResourceRequirements(node *models.ServiceNode) *ResourceRequirements {
	// Base requirements
	cpuRequest := "100m"
	memoryRequest := "128Mi"
	cpuLimit := "500m"
	memoryLimit := "512Mi"

	// Adjust based on RPS and replicas
	if node.RPS > 1000 {
		cpuRequest = "200m"
		memoryRequest = "256Mi"
		cpuLimit = "1000m"
		memoryLimit = "1Gi"
	} else if node.RPS > 500 {
		cpuRequest = "150m"
		memoryRequest = "192Mi"
		cpuLimit = "750m"
		memoryLimit = "768Mi"
	}

	// Adjust based on network topology
	if node.NetworkTopology != nil && node.NetworkTopology.Bandwidth > 800 {
		// High bandwidth services might need more resources for network processing
		cpuRequest = "250m"
		memoryRequest = "320Mi"
	}

	return &ResourceRequirements{
		Requests: ResourceList{
			"cpu":    cpuRequest,
			"memory": memoryRequest,
		},
		Limits: ResourceList{
			"cpu":    cpuLimit,
			"memory": memoryLimit,
		},
	}
}

// writeConfigToFile writes a configuration to a file
func (kcg *KubernetesConfigGenerator) writeConfigToFile(config interface{}, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(config)
}

// generateCombinedManifest generates a combined manifest for all services
func (kcg *KubernetesConfigGenerator) generateCombinedManifest(nodes map[string]*models.ServiceNode, affinityConfigs map[string]*algorithms.KubernetesAffinityConfig) []*DeploymentConfig {
	var manifests []*DeploymentConfig

	for serviceID, node := range nodes {
		config := kcg.createDeploymentConfig(node, affinityConfigs[serviceID])
		manifests = append(manifests, config)
	}

	return manifests
}

// GenerateServiceManifests generates Kubernetes service manifests
func (kcg *KubernetesConfigGenerator) GenerateServiceManifests(graph *models.ServiceGraph) error {
	nodes := graph.GetAllNodes()

	for serviceID, node := range nodes {
		serviceConfig := kcg.createServiceConfig(node)

		filename := filepath.Join(kcg.outputDir, fmt.Sprintf("%s-service.json", serviceID))
		if err := kcg.writeConfigToFile(serviceConfig, filename); err != nil {
			return fmt.Errorf("failed to write service config for %s: %v", serviceID, err)
		}

		fmt.Printf("Generated service config for service %s\n", serviceID)
	}

	return nil
}

// ServiceConfig represents a Kubernetes service configuration
type ServiceConfig struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   Metadata    `json:"metadata"`
	Spec       ServiceSpec `json:"spec"`
}

// ServiceSpec represents service specification
type ServiceSpec struct {
	Selector map[string]string `json:"selector"`
	Ports    []ServicePort     `json:"ports"`
	Type     string            `json:"type"`
}

// ServicePort represents a service port
type ServicePort struct {
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol,omitempty"`
	Name       string `json:"name,omitempty"`
}

// createServiceConfig creates a service configuration for a service
func (kcg *KubernetesConfigGenerator) createServiceConfig(node *models.ServiceNode) *ServiceConfig {
	return &ServiceConfig{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: Metadata{
			Name:      node.ID,
			Namespace: kcg.namespace,
			Labels: map[string]string{
				"app":     node.Name,
				"service": node.ID,
			},
		},
		Spec: ServiceSpec{
			Selector: map[string]string{
				"app": node.ID,
			},
			Ports: []ServicePort{
				{
					Port:       80,
					TargetPort: 8080,
					Protocol:   "TCP",
					Name:       "http",
				},
			},
			Type: "ClusterIP",
		},
	}
}
