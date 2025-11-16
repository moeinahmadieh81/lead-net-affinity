package scoring

import (
	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
)

type NetWeights struct {
	NetLatencyWeight   float64
	NetDropWeight      float64
	NetBandwidthWeight float64
	BadLatencyMs       float64
	BadDropRate        float64
}

type PodPlacement interface {
	NodeNameForService(svc graph.NodeID) string
}

func ComputeNetworkPenalty(
	path graph.Path,
	matrix *promnet.NetworkMatrix,
	w NetWeights,
) float64 {
	if matrix == nil || len(matrix.Links) == 0 {
		return 0
	}

	// Pick the single cluster-level link (the first and only entry).
	var link *promnet.NodeLinkMetrics
	for _, l := range matrix.Links {
		link = l
		break
	}
	if link == nil {
		return 0
	}

	hops := len(path.Nodes) - 1
	if hops <= 0 {
		return 0
	}

	var perHop float64

	// Latency penalty: higher AvgLatencyMs than BadLatencyMs is bad.
	if w.BadLatencyMs > 0 && link.AvgLatencyMs > 0 {
		perHop += w.NetLatencyWeight * (link.AvgLatencyMs / w.BadLatencyMs)
	}

	// Drop penalty: higher DropRate than BadDropRate is bad.
	if w.BadDropRate > 0 && link.DropRate > 0 {
		perHop += w.NetDropWeight * (link.DropRate / w.BadDropRate)
	}

	// "Bandwidth" penalty: we interpret BandwidthMbps as a load proxy
	// (e.g. flows/s). If you currently have NetBandwidthWeight=0 this will
	// not affect the score, but the code is here for future use.
	if w.NetBandwidthWeight > 0 && link.BandwidthMbps > 0 {
		// Define an arbitrary "healthy" baseline; you can tune this.
		const baseline = 1_000.0
		perHop += w.NetBandwidthWeight * (link.BandwidthMbps / baseline)
	}

	return perHop * float64(hops)
}

func CombineScores(base, penalty float64) float64 {
	return base - penalty
}
