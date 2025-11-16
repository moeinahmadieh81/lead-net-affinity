package scoring

import (
	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
)

// NetWeights controls how strongly network signals influence the score.
type NetWeights struct {
	NetLatencyWeight   float64
	NetDropWeight      float64
	NetBandwidthWeight float64
	BadLatencyMs       float64
	BadDropRate        float64
}

// PodPlacement tells us which node each service is running on.
type PodPlacement interface {
	NodeNameForService(svc graph.NodeID) string
}

// ComputeNetworkPenalty sums per-node penalties for all nodes involved in
// this path. Each physical worker contributes once, no matter how many
// services from the path it hosts.
func ComputeNetworkPenalty(
	path graph.Path,
	placements PodPlacement,
	matrix *promnet.NetworkMatrix,
	w NetWeights,
) float64 {
	if matrix == nil || len(matrix.Nodes) == 0 {
		return 0
	}

	visited := make(map[string]bool)
	var penalty float64

	for _, svc := range path.Nodes {
		node := placements.NodeNameForService(svc)
		if node == "" || visited[node] {
			continue
		}
		visited[node] = true

		m := matrix.GetNode(node)
		if m == nil {
			// no telemetry for this node – treat as neutral
			continue
		}

		// Latency penalty
		if w.BadLatencyMs > 0 && m.AvgLatencyMs > w.BadLatencyMs {
			penalty += w.NetLatencyWeight * (m.AvgLatencyMs / w.BadLatencyMs)
		}

		// Drop penalty
		if w.BadDropRate > 0 && m.DropRate > w.BadDropRate {
			penalty += w.NetDropWeight * (m.DropRate / w.BadDropRate)
		}

		// Bandwidth / load penalty (low bandwidth or very high flow rate)
		if m.BandwidthRate > 0 {
			// Example heuristic: if flow rate is "too high", treat it as congested.
			const busyFactor = 1.0
			penalty += w.NetBandwidthWeight * (m.BandwidthRate * busyFactor)
		}
	}

	return penalty
}

// CombineScores merges the base LEAD score with the network penalty.
func CombineScores(base, penalty float64) float64 {
	return base - penalty
}
