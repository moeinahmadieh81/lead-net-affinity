package models

import (
	"fmt"
	"sort"
)

// PathFinder implements algorithms to find all paths in the service graph
type PathFinder struct {
	graph *ServiceGraph
}

// NewPathFinder creates a new path finder
func NewPathFinder(graph *ServiceGraph) *PathFinder {
	return &PathFinder{graph: graph}
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

		// Calculate network score contribution
		if node.NetworkTopology != nil {
			networkScore += pf.calculateNetworkScore(node.NetworkTopology)
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
