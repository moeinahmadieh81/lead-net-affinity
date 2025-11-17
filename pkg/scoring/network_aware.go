package scoring

import (
	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
)

// NetWeights control how strongly each signal contributes to penalty.
type NetWeights struct {
	NetLatencyWeight   float64
	NetDropWeight      float64
	NetBandwidthWeight float64
	BadLatencyMs       float64
	BadDropRate        float64
}

// PodPlacement tells us which node a service currently runs on.
type PodPlacement interface {
	NodeNameForService(svc graph.NodeID) string
}

// nodeSeverityForMetrics converts raw node metrics into a scalar "badness".
func nodeSeverityForMetrics(m *promnet.NodeMetrics, w NetWeights) float64 {
	if m == nil {
		return 0
	}

	var sev float64

	// Latency: higher than BadLatencyMs is bad.
	if w.BadLatencyMs > 0 && m.AvgLatencyMs > w.BadLatencyMs {
		sev += w.NetLatencyWeight * (m.AvgLatencyMs / w.BadLatencyMs)
	}

	// Drop: higher than BadDropRate is bad.
	if w.BadDropRate > 0 && m.DropRate > w.BadDropRate {
		sev += w.NetDropWeight * (m.DropRate / w.BadDropRate)
	}

	// Flow / bandwidth:
	// For now we treat "more flow" as "more loaded", so higher = more penalty.
	// You can tune NetBandwidthWeight to control how strong this is.
	if w.NetBandwidthWeight > 0 && m.BandwidthRate > 0 {
		sev += w.NetBandwidthWeight * m.BandwidthRate
	}

	return sev
}

// ComputeNetworkPenalty now:
//   - walks the path's services,
//   - looks up which node each service is on,
//   - sums the severity of the UNIQUE nodes touched by that path.
//
// No cluster-wide averaging, no pair links.
func ComputeNetworkPenalty(
	path graph.Path,
	placements PodPlacement,
	matrix *promnet.NetworkMatrix,
	w NetWeights,
) float64 {
	if matrix == nil || len(matrix.Nodes) == 0 {
		return 0
	}
	if w.NetLatencyWeight == 0 && w.NetDropWeight == 0 && w.NetBandwidthWeight == 0 {
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
			continue // don't double-count same node on same path
		}
		seenNodes[nodeName] = struct{}{}

		m := matrix.GetNode(nodeName)
		if m == nil {
			// Node exists but we have no metrics: small default penalty.
			penalty += 1.0
			continue
		}

		penalty += nodeSeverityForMetrics(m, w)
	}

	return penalty
}

// CombineScores subtracts the penalty from the base LEAD score.
func CombineScores(base, penalty float64) float64 {
	return base - penalty
}
