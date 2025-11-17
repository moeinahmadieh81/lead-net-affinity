package scoring

import (
	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
)

// NetWeights configures how much each network signal matters.
type NetWeights struct {
	NetLatencyWeight   float64
	NetDropWeight      float64
	NetBandwidthWeight float64

	// Thresholds beyond which we start treating the metric as "bad".
	BadLatencyMs float64
	BadDropRate  float64
}

// PodPlacement is implemented by kube.PlacementResolver.
type PodPlacement interface {
	// NodeNameForService returns the node name (or empty string) for a service.
	NodeNameForService(svc graph.NodeID) string
}

// NodeSeverityFromMetrics converts per-node metrics into a scalar penalty.
//
// Larger values mean "worse" nodes. If a metric is missing or thresholds are
// not configured, that signal simply contributes 0.
func NodeSeverityFromMetrics(m *promnet.NodeMetrics, w NetWeights) float64 {
	if m == nil {
		return 0
	}

	var penalty float64

	// Latency: penalize only when above BadLatencyMs.
	if w.NetLatencyWeight > 0 && w.BadLatencyMs > 0 && m.AvgLatencyMs > w.BadLatencyMs {
		// Example: if latency is 2x the "bad" threshold, factor ~1.0
		factor := (m.AvgLatencyMs / w.BadLatencyMs) - 1.0
		if factor < 0 {
			factor = 0
		}
		penalty += w.NetLatencyWeight * factor
	}

	// Drops: always bad once non-trivial. Scale by threshold.
	if w.NetDropWeight > 0 && w.BadDropRate > 0 && m.DropRate > 0 {
		factor := m.DropRate / w.BadDropRate
		if factor < 0 {
			factor = 0
		}
		penalty += w.NetDropWeight * factor
	}

	// Flow rate: you said "flow rate does matter". Here we treat *high* flow
	// as "pressure" on the node. You can tune NetBandwidthWeight to decide how
	// strongly this matters. If you later decide that high flow is *good*
	// (well-utilized), you can invert this logic instead.
	if w.NetBandwidthWeight > 0 && m.BandwidthRate > 0 {
		penalty += w.NetBandwidthWeight * m.BandwidthRate
	}

	return penalty
}

// ComputeNetworkPenalty computes a per-path penalty by:
//  1. Looking at which nodes the services in the path are actually running on.
//  2. Summing per-node severity for the *unique* nodes along that path.
//
// This is no longer a cluster-wide average; it's strictly path-topology dependent.
func ComputeNetworkPenalty(
	path graph.Path,
	placements PodPlacement,
	matrix *promnet.NetworkMatrix,
	w NetWeights,
) float64 {
	if matrix == nil || placements == nil {
		return 0
	}

	seenNodes := make(map[string]struct{})
	var penalty float64

	for _, svc := range path.Nodes {
		nodeName := placements.NodeNameForService(svc)
		if nodeName == "" {
			continue
		}
		if _, ok := seenNodes[nodeName]; ok {
			// Only penalize each node once per path.
			continue
		}
		seenNodes[nodeName] = struct{}{}

		metrics := matrix.GetNode(nodeName)
		penalty += NodeSeverityFromMetrics(metrics, w)
	}

	return penalty
}

// CombineScores merges base LEAD score and network penalty into a final score.
//
// Larger final scores are better, so we subtract the penalty.
func CombineScores(base, penalty float64) float64 {
	return base - penalty
}
