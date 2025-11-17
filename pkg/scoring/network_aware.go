package scoring

import (
	"log"

	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
)

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

// NodeIPResolver resolves a Kubernetes node name to an IP address that matches
// the Prometheus "instance" label (e.g. 91.228.186.28).
// Implemented on the controller side so scoring stays decoupled from kube.
type NodeIPResolver interface {
	// IPForNode returns the node's IP address (or "" if unknown).
	IPForNode(nodeName string) string
}

// NodeSeverityFromMetrics converts per-node metrics into a scalar penalty.
//
// Larger values mean "worse" nodes. If a metric is missing or thresholds are
// not configured, that signal simply contributes 0.
func NodeSeverityFromMetrics(m *promnet.NodeMetrics, w NetWeights) float64 {
	if m == nil {
		log.Printf("[lead-net][net-score] NodeSeverityFromMetrics: metrics=nil, penalty=0")
		return 0
	}

	log.Printf("[lead-net][net-score] NodeSeverityFromMetrics: node=%s metrics=%+v weights=%+v",
		m.NodeID, m, w)

	var penalty float64

	// Latency
	if w.NetLatencyWeight > 0 && w.BadLatencyMs > 0 && m.AvgLatencyMs > w.BadLatencyMs {
		factor := (m.AvgLatencyMs / w.BadLatencyMs) - 1.0
		if factor < 0 {
			factor = 0
		}
		penalty += w.NetLatencyWeight * factor
		log.Printf("[lead-net][net-score] node=%s latency contribution: factor=%f partialPenalty=%f",
			m.NodeID, factor, penalty)
	}

	// Drops
	if w.NetDropWeight > 0 && w.BadDropRate > 0 && m.DropRate > 0 {
		factor := m.DropRate / w.BadDropRate
		if factor < 0 {
			factor = 0
		}
		penalty += w.NetDropWeight * factor
		log.Printf("[lead-net][net-score] node=%s drop contribution: factor=%f partialPenalty=%f",
			m.NodeID, factor, penalty)
	}

	// Bandwidth
	if w.NetBandwidthWeight > 0 && m.BandwidthRate > 0 {
		penalty += w.NetBandwidthWeight * m.BandwidthRate
		log.Printf("[lead-net][net-score] node=%s bandwidth contribution: bw=%f partialPenalty=%f",
			m.NodeID, m.BandwidthRate, penalty)
	}

	log.Printf("[lead-net][net-score] NodeSeverityFromMetrics: node=%s finalPenalty=%f", m.NodeID, penalty)
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
	ipResolver NodeIPResolver,
	w NetWeights,
) float64 {
	if matrix == nil || placements == nil {
		log.Printf("[lead-net][net-score] ComputeNetworkPenalty: matrix or placements nil, penalty=0")
		return 0
	}

	log.Printf("[lead-net][net-score] ComputeNetworkPenalty start path=%v", path.Nodes)

	seenNodes := make(map[string]struct{})
	var penalty float64

	for _, svc := range path.Nodes {
		nodeName := placements.NodeNameForService(svc)
		if nodeName == "" {
			log.Printf("[lead-net][net-score] service=%s has no resolved node; skipping", svc)
			continue
		}
		if _, ok := seenNodes[nodeName]; ok {
			// Only penalize each node once per path.
			log.Printf("[lead-net][net-score] node=%s already accounted for; skipping duplicate", nodeName)
			continue
		}
		seenNodes[nodeName] = struct{}{}

		// Try metrics keyed by node name (if Prom ever uses node label).
		metrics := matrix.GetNode(nodeName)

		// If that fails, resolve nodeName -> IP and look up by IP.
		if metrics == nil && ipResolver != nil {
			ip := ipResolver.IPForNode(nodeName)
			if ip == "" {
				log.Printf("[lead-net][net-score] no IP mapping for node=%s; skipping metrics lookup", nodeName)
			} else {
				metrics = matrix.GetNode(ip)
				if metrics == nil {
					log.Printf("[lead-net][net-score] no metrics found for node=%s ip=%s", nodeName, ip)
				} else {
					log.Printf("[lead-net][net-score] resolved node=%s to ip=%s for metrics lookup", nodeName, ip)
				}
			}
		}

		nodePenalty := NodeSeverityFromMetrics(metrics, w)
		log.Printf("[lead-net][net-score] path node=%s contributes penalty=%f", nodeName, nodePenalty)
		penalty += nodePenalty
	}

	log.Printf("[lead-net][net-score] ComputeNetworkPenalty: path=%v totalPenalty=%f", path.Nodes, penalty)
	return penalty
}

// CombineScores merges base LEAD score and network penalty into a final score.
//
// Larger final scores are better, so we subtract the penalty.
func CombineScores(base, penalty float64) float64 {
	final := base - penalty
	log.Printf("[lead-net][net-score] CombineScores: base=%f penalty=%f final=%f", base, penalty, final)
	return final
}
