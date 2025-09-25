package scheduler

import (
	"lead-framework/internal/models"
)

// ServiceInfo contains service information for scheduling decisions
type ServiceInfo struct {
	ServiceName     string
	ServiceType     string
	NetworkTopology *models.NetworkTopology
	Priority        int
}

// NetworkTopologyAnalysis represents network topology analysis results
type NetworkTopologyAnalysis struct {
	TotalPaths        int            `json:"total_paths"`
	AvgBandwidth      float64        `json:"avg_bandwidth"`
	AvgHops           float64        `json:"avg_hops"`
	AvgGeoDistance    float64        `json:"avg_geo_distance"`
	AvailabilityZones map[string]int `json:"availability_zones"`
}
