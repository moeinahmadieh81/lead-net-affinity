package models

import (
	"fmt"
	"math"
	"sort"
)

// InterNodeMetricsProvider provides inter-node network metrics
type InterNodeMetricsProvider interface {
	// GetAverageInterNodeMetrics returns average inter-node metrics across all node pairs
	// This is used when exact node placement is unknown
	GetAverageInterNodeMetrics() (latency float64, bandwidth float64, geoDistance float64)
	// GetInterNodeMetrics returns metrics between two specific nodes
	GetInterNodeMetrics(node1, node2 string) (latency float64, bandwidth float64, geoDistance float64, exists bool)
}

// PathFinder implements algorithms to find all paths in the service graph
type PathFinder struct {
	graph               *ServiceGraph
	networkMetrics      InterNodeMetricsProvider
	useInterNodeMetrics bool
}

// NewPathFinder creates a new path finder
func NewPathFinder(graph *ServiceGraph) *PathFinder {
	return &PathFinder{
		graph:               graph,
		useInterNodeMetrics: false,
	}
}

// SetNetworkMetricsProvider sets the network metrics provider for inter-node metrics
func (pf *PathFinder) SetNetworkMetricsProvider(provider InterNodeMetricsProvider) {
	pf.networkMetrics = provider
	pf.useInterNodeMetrics = provider != nil
}

// FindAllPaths finds all possible paths from the gateway
func (pf *PathFinder) FindAllPaths(gateway string) []*Path {
	var paths []*Path
	visited := make(map[string]bool)
	currentPath := make([]string, 0)

	pf.dfs(gateway, currentPath, visited, &paths)
	return paths
}

// dfs performs depth-first search to find all paths
func (pf *PathFinder) dfs(current string, currentPath []string, visited map[string]bool, paths *[]*Path) {
	// Add current node to path
	currentPath = append(currentPath, current)
	visited[current] = true

	// Get children of current node
	children := pf.graph.GetChildren(current)

	if len(children) == 0 {
		// Leaf node reached, create path
		path := pf.createPath(currentPath)
		if path != nil {
			*paths = append(*paths, path)
		}
	} else {
		// Continue to children
		for _, child := range children {
			if !visited[child] {
				pf.dfs(child, currentPath, visited, paths)
			}
		}
	}

	// Backtrack
	currentPath = currentPath[:len(currentPath)-1]
	visited[current] = false
}

// createPath creates a Path object from a sequence of service IDs
func (pf *PathFinder) createPath(serviceIDs []string) *Path {
	var services []*ServiceNode
	var totalRPS float64
	var totalPods int
	var totalEdges int
	var networkScore float64

	for i, serviceID := range serviceIDs {
		node, exists := pf.graph.GetNode(serviceID)
		if !exists {
			return nil
		}

		services = append(services, node)
		totalRPS += node.RPS
		totalPods += node.Replicas

		// Count edges (service-to-service interactions)
		if i > 0 {
			totalEdges++
		}
	}

	// Calculate network score based on inter-node metrics between services in the path
	if pf.useInterNodeMetrics && pf.networkMetrics != nil {
		networkScore = pf.calculatePathNetworkScoreFromInterNodeMetrics(services)
	} else {
		// Fallback: use individual service network topology (legacy behavior)
		for _, node := range services {
			if node.NetworkTopology != nil {
				networkScore += pf.calculateNetworkScore(node.NetworkTopology)
			}
		}
	}

	return &Path{
		Services:     services,
		PathLength:   len(services),
		PodCount:     totalPods,
		EdgeCount:    totalEdges,
		NetworkScore: networkScore,
		Weight:       0, // Will be set by scoring algorithm
	}
}

// calculatePathNetworkScoreFromInterNodeMetrics calculates network score based on inter-node metrics
// This considers the network cost between nodes where services might be placed
func (pf *PathFinder) calculatePathNetworkScoreFromInterNodeMetrics(services []*ServiceNode) float64 {
	if len(services) < 2 {
		// Single service: no inter-node communication needed
		return 1.0
	}

	var totalScore float64
	var edgeCount int

	// Get average inter-node metrics (used when exact node placement is unknown)
	avgLatency, avgBandwidth, avgGeoDistance := pf.networkMetrics.GetAverageInterNodeMetrics()

	// Calculate network score for each edge (service-to-service communication)
	for i := 0; i < len(services)-1; i++ {
		service1 := services[i]
		service2 := services[i+1]

		var latency, bandwidth, geoDistance float64
		var useMetrics bool

		// Try to get inter-node metrics if services have node information
		// For now, we use average inter-node metrics since we don't know exact node placement
		// In the future, we could use service labels or node selectors to determine likely nodes
		if service1.NetworkTopology != nil && service2.NetworkTopology != nil {
			// If services are in the same availability zone, use lower latency
			if service1.NetworkTopology.AvailabilityZone == service2.NetworkTopology.AvailabilityZone &&
				service1.NetworkTopology.AvailabilityZone != "" &&
				service1.NetworkTopology.AvailabilityZone != "unknown" {
				// Same AZ: use very low latency (e.g., 0.1ms for same node, 1ms for same AZ)
				latency = 1.0
				bandwidth = math.Max(service1.NetworkTopology.Bandwidth, service2.NetworkTopology.Bandwidth)
				geoDistance = 0.0
				useMetrics = true
			} else {
				// Different AZs: use average inter-node metrics
				latency = avgLatency
				bandwidth = avgBandwidth
				geoDistance = avgGeoDistance
				useMetrics = true
			}
		} else {
			// No topology info: use average inter-node metrics
			latency = avgLatency
			bandwidth = avgBandwidth
			geoDistance = avgGeoDistance
			useMetrics = true
		}

		if useMetrics {
			// Calculate edge score based on inter-node metrics
			edgeScore := pf.calculateInterNodeEdgeScore(latency, bandwidth, geoDistance)
			totalScore += edgeScore
			edgeCount++
		}
	}

	if edgeCount == 0 {
		return 0.0
	}

	// Return total score (sum of all edge scores)
	// This represents the cumulative network cost/quality of the path
	return totalScore
}

// calculateInterNodeEdgeScore calculates network score for an edge based on inter-node metrics
// Lower latency, higher bandwidth, shorter distance = better score
func (pf *PathFinder) calculateInterNodeEdgeScore(latency, bandwidth, geoDistance float64) float64 {
	// Latency score: lower is better (normalize: 0-100ms -> 1.0-0.0)
	latencyScore := math.Max(0.0, 1.0-latency/100.0) // 0ms = 1.0, 100ms = 0.0

	// Bandwidth score: higher is better (normalize: 0-1000 Mbps -> 0.0-1.0)
	bandwidthScore := math.Min(bandwidth/1000.0, 1.0)

	// Geo distance score: shorter is better (normalize: 0-100km -> 1.0-0.0)
	distanceScore := math.Max(0.0, 1.0-geoDistance/100.0)

	// Weighted combination: latency is most important for path scoring
	edgeScore := latencyScore*0.5 + bandwidthScore*0.3 + distanceScore*0.2

	return edgeScore
}

// calculateNetworkScore calculates network score for a single node
func (pf *PathFinder) calculateNetworkScore(topology *NetworkTopology) float64 {
	// Higher bandwidth = better score
	bandwidthScore := topology.Bandwidth / 1000.0 // Normalize to 0-1

	// Fewer hops = better score
	hopScore := 1.0 / float64(topology.Hops+1) // +1 to avoid division by zero

	// Shorter distance = better score
	distanceScore := 1.0 / (topology.GeoDistance/100.0 + 1.0) // Normalize distance

	// Weighted combination of network factors
	networkScore := (bandwidthScore*0.4 + hopScore*0.3 + distanceScore*0.3)

	return networkScore
}

// PathSorter sorts paths by their scores
type PathSorter struct {
	paths []*Path
}

// NewPathSorter creates a new path sorter
func NewPathSorter(paths []*Path) *PathSorter {
	return &PathSorter{paths: paths}
}

// SortByScore sorts paths by score in descending order (highest first)
func (ps *PathSorter) SortByScore() []*Path {
	sort.Slice(ps.paths, func(i, j int) bool {
		return ps.paths[i].Score > ps.paths[j].Score
	})
	return ps.paths
}

// SortByScoreAscending sorts paths by score in ascending order (lowest first)
func (ps *PathSorter) SortByScoreAscending() []*Path {
	sort.Slice(ps.paths, func(i, j int) bool {
		return ps.paths[i].Score < ps.paths[j].Score
	})
	return ps.paths
}

// NormalizeScores normalizes all path scores to 0-100 range
func (ps *PathSorter) NormalizeScores() {
	if len(ps.paths) == 0 {
		return
	}

	// Find min and max scores
	minScore := ps.paths[0].Score
	maxScore := ps.paths[0].Score

	for _, path := range ps.paths {
		if path.Score < minScore {
			minScore = path.Score
		}
		if path.Score > maxScore {
			maxScore = path.Score
		}
	}

	// Normalize using min-max normalization formula
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		// All scores are the same
		for _, path := range ps.paths {
			path.Score = 50.0 // Set to middle value
		}
		return
	}

	for _, path := range ps.paths {
		normalizedScore := (path.Score - minScore) / scoreRange * 100.0
		path.Score = normalizedScore
	}
}

// String returns a string representation of a path
func (p *Path) String() string {
	var serviceNames []string
	for _, service := range p.Services {
		serviceNames = append(serviceNames, service.Name)
	}

	return fmt.Sprintf("Path: %v (Score: %.2f, Weight: %d, Length: %d, Pods: %d, Edges: %d, Network: %.2f)",
		serviceNames, p.Score, p.Weight, p.PathLength, p.PodCount, p.EdgeCount, p.NetworkScore)
}

// GetServiceNames returns the names of services in the path
func (p *Path) GetServiceNames() []string {
	var names []string
	for _, service := range p.Services {
		names = append(names, service.Name)
	}
	return names
}

// GetServiceIDs returns the IDs of services in the path
func (p *Path) GetServiceIDs() []string {
	var ids []string
	for _, service := range p.Services {
		ids = append(ids, service.ID)
	}
	return ids
}
