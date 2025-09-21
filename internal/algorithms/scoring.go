package algorithms

import (
	"fmt"
	"math"

	"lead-framework/internal/models"
)

// ScoringAlgorithm implements Algorithm 1 from the LEAD framework
type ScoringAlgorithm struct {
	graph *models.ServiceGraph
}

// NewScoringAlgorithm creates a new scoring algorithm instance
func NewScoringAlgorithm(graph *models.ServiceGraph) *ScoringAlgorithm {
	return &ScoringAlgorithm{graph: graph}
}

// ScorePaths implements Algorithm 1: Scoring
// This algorithm evaluates critical paths through the service mesh based on:
// - Path length (number of vertices a request must traverse)
// - Number of pods per node (anticipated request load)
// - Number of service-to-service interactions (dependencies)
// - RPS (Requests Per Second)
// - Network topology factors (bandwidth, hops, geo distance, availability zone)
func (sa *ScoringAlgorithm) ScorePaths(gateway string) ([]*models.Path, error) {
	// Step 1: Find all paths from gateway
	pathFinder := models.NewPathFinder(sa.graph)
	paths := pathFinder.FindAllPaths(gateway)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths found from gateway '%s'", gateway)
	}

	// Step 2: Score each path
	for _, path := range paths {
		score := sa.calculateScore(path)
		path.Score = score
	}

	// Step 3: Normalize scores to 0-100 range
	sorter := models.NewPathSorter(paths)
	sorter.NormalizeScores()

	// Step 4: Sort paths by score (highest first)
	sortedPaths := sorter.SortByScore()

	// Step 5: Apply weights starting from 100 and decreasing by 1
	weight := 100
	for _, path := range sortedPaths {
		path.Weight = weight
		weight--
	}

	return sortedPaths, nil
}

// calculateScore calculates the score for a single path
func (sa *ScoringAlgorithm) calculateScore(path *models.Path) float64 {
	// Calculate individual components
	pathLengthScore := sa.calculatePathLengthScore(path.PathLength)
	podCountScore := sa.calculatePodCountScore(path.PodCount)
	edgeCountScore := sa.calculateEdgeCountScore(path.EdgeCount)
	rpsScore := sa.calculateRPSScore(path.Services)
	networkScore := sa.calculateNetworkTopologyScore(path.Services)

	// Weighted combination of all factors
	// Original factors (70% weight)
	originalScore := (pathLengthScore*0.3 + podCountScore*0.25 + edgeCountScore*0.15 + rpsScore*0.3)

	// Network topology factors (30% weight)
	networkContribution := networkScore * 0.3

	// Combine scores
	finalScore := originalScore*0.7 + networkContribution

	return finalScore
}

// calculatePathLengthScore calculates score based on path length
// Longer paths indicate potential bottlenecks and get lower scores
func (sa *ScoringAlgorithm) calculatePathLengthScore(pathLength int) float64 {
	// Inverse relationship: shorter paths get higher scores
	// Use exponential decay for more dramatic difference
	if pathLength <= 1 {
		return 1.0
	}
	return 1.0 / math.Pow(float64(pathLength), 0.5)
}

// calculatePodCountScore calculates score based on total pod count in path
// More pods indicate higher capacity and get higher scores
func (sa *ScoringAlgorithm) calculatePodCountScore(podCount int) float64 {
	// Logarithmic relationship: diminishing returns for very high pod counts
	if podCount <= 0 {
		return 0.0
	}
	return math.Log(float64(podCount)+1) / math.Log(10) // Normalize to 0-1 range
}

// calculateEdgeCountScore calculates score based on service-to-service interactions
// More interactions indicate higher complexity and dependencies
func (sa *ScoringAlgorithm) calculateEdgeCountScore(edgeCount int) float64 {
	// Inverse relationship: fewer edges get higher scores (less complexity)
	if edgeCount == 0 {
		return 1.0
	}
	return 1.0 / float64(edgeCount+1)
}

// calculateRPSScore calculates score based on total RPS in the path
// Higher RPS indicates more critical path
func (sa *ScoringAlgorithm) calculateRPSScore(services []*models.ServiceNode) float64 {
	var totalRPS float64
	for _, service := range services {
		totalRPS += service.RPS
	}

	// Normalize RPS score (assume max reasonable RPS is 10000)
	maxRPS := 10000.0
	if totalRPS > maxRPS {
		totalRPS = maxRPS
	}

	return totalRPS / maxRPS
}

// calculateNetworkTopologyScore calculates score based on network characteristics
func (sa *ScoringAlgorithm) calculateNetworkTopologyScore(services []*models.ServiceNode) float64 {
	if len(services) == 0 {
		return 0.0
	}

	var totalNetworkScore float64
	var validServices int

	for _, service := range services {
		if service.NetworkTopology != nil {
			serviceScore := sa.calculateServiceNetworkScore(service.NetworkTopology)
			totalNetworkScore += serviceScore
			validServices++
		}
	}

	if validServices == 0 {
		return 0.0
	}

	return totalNetworkScore / float64(validServices)
}

// calculateServiceNetworkScore calculates network score for a single service
func (sa *ScoringAlgorithm) calculateServiceNetworkScore(topology *models.NetworkTopology) float64 {
	// Bandwidth score (higher is better, normalized to 0-1)
	bandwidthScore := math.Min(topology.Bandwidth/1000.0, 1.0) // Assume 1000 Mbps is excellent

	// Hop score (fewer hops is better)
	hopScore := 1.0 / math.Max(float64(topology.Hops)+1, 1.0)

	// Geo distance score (shorter distance is better)
	distanceScore := 1.0 / math.Max(topology.GeoDistance/100.0+1.0, 1.0) // Normalize by 100km

	// Availability zone diversity bonus (same AZ as gateway gets bonus)
	azScore := 1.0 // Default score

	// Weighted combination
	networkScore := bandwidthScore*0.4 + hopScore*0.3 + distanceScore*0.2 + azScore*0.1

	return networkScore
}

// GetCriticalPaths returns the top N critical paths
func (sa *ScoringAlgorithm) GetCriticalPaths(gateway string, topN int) ([]*models.Path, error) {
	allPaths, err := sa.ScorePaths(gateway)
	if err != nil {
		return nil, err
	}

	if topN <= 0 || topN > len(allPaths) {
		topN = len(allPaths)
	}

	return allPaths[:topN], nil
}

// GetPathByScore returns paths within a specific score range
func (sa *ScoringAlgorithm) GetPathByScore(gateway string, minScore, maxScore float64) ([]*models.Path, error) {
	allPaths, err := sa.ScorePaths(gateway)
	if err != nil {
		return nil, err
	}

	var filteredPaths []*models.Path
	for _, path := range allPaths {
		if path.Score >= minScore && path.Score <= maxScore {
			filteredPaths = append(filteredPaths, path)
		}
	}

	return filteredPaths, nil
}

// AnalyzeNetworkTopology provides detailed network analysis
func (sa *ScoringAlgorithm) AnalyzeNetworkTopology(gateway string) (*NetworkTopologyAnalysis, error) {
	paths, err := sa.ScorePaths(gateway)
	if err != nil {
		return nil, err
	}

	analysis := &NetworkTopologyAnalysis{
		TotalPaths:        len(paths),
		AvailabilityZones: make(map[string]int),
		AvgBandwidth:      0.0,
		AvgHops:           0.0,
		AvgGeoDistance:    0.0,
	}

	var totalBandwidth, totalHops, totalDistance float64
	var totalServices int

	for _, path := range paths {
		for _, service := range path.Services {
			if service.NetworkTopology != nil {
				// Count availability zones
				az := service.NetworkTopology.AvailabilityZone
				analysis.AvailabilityZones[az]++

				// Accumulate metrics
				totalBandwidth += service.NetworkTopology.Bandwidth
				totalHops += float64(service.NetworkTopology.Hops)
				totalDistance += service.NetworkTopology.GeoDistance
				totalServices++
			}
		}
	}

	if totalServices > 0 {
		analysis.AvgBandwidth = totalBandwidth / float64(totalServices)
		analysis.AvgHops = totalHops / float64(totalServices)
		analysis.AvgGeoDistance = totalDistance / float64(totalServices)
	}

	return analysis, nil
}

// NetworkTopologyAnalysis contains network topology analysis results
type NetworkTopologyAnalysis struct {
	TotalPaths        int            `json:"total_paths"`
	AvailabilityZones map[string]int `json:"availability_zones"`
	AvgBandwidth      float64        `json:"avg_bandwidth"`
	AvgHops           float64        `json:"avg_hops"`
	AvgGeoDistance    float64        `json:"avg_geo_distance"`
}
