package tests

import (
	"testing"

	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
	"lead-net-affinity/pkg/scoring"
)

// TestNetworkPenaltyAndCombine
// Basic sanity check that ComputeNetworkPenalty runs and that
// CombineScores(base, penalty) = base - penalty behaves as expected.
func TestNetworkPenaltyAndCombine(t *testing.T) {
	// Simple path with a couple of hops.
	path := graph.Path{Nodes: []graph.NodeID{"a", "b", "c"}}

	// Synthetic matrix entry. In the real cluster you're effectively using
	// cluster-level signals, but for unit tests we just need *some* numbers.
	m := &promnet.NetworkMatrix{
		Links: map[string]*promnet.NodeLinkMetrics{
			// Key is arbitrary here; tests don't rely on per-link keys,
			// they only care that the function can read something.
			"cluster||cluster": {
				SrcNode:       "cluster",
				DstNode:       "cluster",
				AvgLatencyMs:  50,  // "bad" latency
				DropRate:      0.1, // "bad" drop rate
				BandwidthMbps: 5,   // "low" bandwidth
			},
		},
	}

	w := scoring.NetWeights{
		NetLatencyWeight:   1.0,
		NetDropWeight:      1.0,
		NetBandwidthWeight: 1.0,
		BadLatencyMs:       10.0,
		BadDropRate:        0.01,
	}

	penalty := scoring.ComputeNetworkPenalty(path, m, w)

	// Penalty should at least not be negative.
	if penalty < 0 {
		t.Fatalf("expected non-negative penalty, got %.2f", penalty)
	}

	base := 100.0
	final := scoring.CombineScores(base, penalty)

	// CombineScores must not *increase* the score.
	if final > base {
		t.Fatalf("expected final score <= base; base=%.2f final=%.2f", base, final)
	}

	// And it must obey base - penalty exactly.
	expected := base - penalty
	if final != expected {
		t.Fatalf("expected final score %.2f = base - penalty, got %.2f", expected, final)
	}
}

// TestPenaltyAffectsRanking
// Checks that with heavier network weights we don't accidentally end up with a *smaller*
// penalty or a *higher* final score.
func TestPenaltyAffectsRanking(t *testing.T) {
	path := graph.Path{Nodes: []graph.NodeID{"a", "b", "c", "d"}}

	m := &promnet.NetworkMatrix{
		Links: map[string]*promnet.NodeLinkMetrics{
			"cluster||cluster": {
				SrcNode:       "cluster",
				DstNode:       "cluster",
				AvgLatencyMs:  50,
				DropRate:      0.1,
				BandwidthMbps: 5,
			},
		},
	}

	// "Light" vs "heavy" network weights.
	wLight := scoring.NetWeights{
		NetLatencyWeight:   0.5,
		NetDropWeight:      0.5,
		NetBandwidthWeight: 0.5,
		BadLatencyMs:       10.0,
		BadDropRate:        0.01,
	}
	wHeavy := scoring.NetWeights{
		NetLatencyWeight:   2.0,
		NetDropWeight:      2.0,
		NetBandwidthWeight: 2.0,
		BadLatencyMs:       10.0,
		BadDropRate:        0.01,
	}

	penLight := scoring.ComputeNetworkPenalty(path, m, wLight)
	penHeavy := scoring.ComputeNetworkPenalty(path, m, wHeavy)

	// Heavier weights should not produce a *smaller* penalty.
	if penHeavy < penLight {
		t.Fatalf("expected heavier weights to produce >= penalty; light=%.2f heavy=%.2f", penLight, penHeavy)
	}

	base := 100.0
	finalLight := scoring.CombineScores(base, penLight)
	finalHeavy := scoring.CombineScores(base, penHeavy)

	// With a higher penalty, the final score should be <= the light one.
	if finalHeavy > finalLight {
		t.Fatalf("expected heavier penalty to give <= score; light=%.2f heavy=%.2f", finalLight, finalHeavy)
	}
}
