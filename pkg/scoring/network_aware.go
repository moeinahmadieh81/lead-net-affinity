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
	placements PodPlacement,
	matrix *promnet.NetworkMatrix,
	w NetWeights,
) float64 {
	if matrix == nil {
		return 0
	}

	var penalty float64
	for i := 0; i < len(path.Nodes)-1; i++ {
		svcA := path.Nodes[i]
		svcB := path.Nodes[i+1]
		nodeA := placements.NodeNameForService(svcA)
		nodeB := placements.NodeNameForService(svcB)
		if nodeA == "" || nodeB == "" || nodeA == nodeB {
			continue
		}

		link := matrix.Get(nodeA, nodeB)
		if link == nil {
			penalty += 1.0
			continue
		}

		if w.BadLatencyMs > 0 && link.AvgLatencyMs > w.BadLatencyMs {
			penalty += w.NetLatencyWeight * (link.AvgLatencyMs / w.BadLatencyMs)
		}
		if w.BadDropRate > 0 && link.DropRate > w.BadDropRate {
			penalty += w.NetDropWeight * (link.DropRate / w.BadDropRate)
		}
		if link.BandwidthMbps > 0 && link.BandwidthMbps < 10 {
			penalty += w.NetBandwidthWeight * (10.0 / link.BandwidthMbps)
		}
	}

	return penalty
}

func CombineScores(base, penalty float64) float64 {
	return base - penalty
}
