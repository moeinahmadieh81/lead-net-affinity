package tests

import (
	"testing"

	"lead-net-affinity/pkg/graph"
	promnet "lead-net-affinity/pkg/prometheus"
	"lead-net-affinity/pkg/scoring"
)

type fakePlacement struct {
	m map[graph.NodeID]string
}

func (f *fakePlacement) NodeNameForService(s graph.NodeID) string {
	return f.m[s]
}

func TestNetworkPenaltyAndCombine(t *testing.T) {
	path := graph.Path{Nodes: []graph.NodeID{"a", "b"}}
	fp := &fakePlacement{m: map[graph.NodeID]string{
		"a": "node1",
		"b": "node2",
	}}

	nm := &promnet.NetworkMatrix{
		Links: map[string]*promnet.NodeLinkMetrics{
			"node1||node2": {
				SrcNode:       "node1",
				DstNode:       "node2",
				AvgLatencyMs:  10,
				DropRate:      0.05,
				BandwidthMbps: 5,
			},
		},
	}

	w := scoring.NetWeights{
		NetLatencyWeight:   2.0,
		NetDropWeight:      3.0,
		NetBandwidthWeight: 1.0,
		BadLatencyMs:       5.0,
		BadDropRate:        0.01,
	}

	pen := scoring.ComputeNetworkPenalty(path, fp, nm, w)
	if pen <= 0 {
		t.Fatalf("expected positive penalty, got %v", pen)
	}

	if scoring.CombineScores(100, pen) >= 100 {
		t.Fatalf("CombineScores should reduce score; got %v", scoring.CombineScores(100, pen))
	}
}

func TestPenaltyAffectsRanking(t *testing.T) {
	// Paths: a->b and c->d. Both base=equal. Only a->b crosses a bad link.
	aToB := graph.Path{Nodes: []graph.NodeID{"a", "b"}}
	cToD := graph.Path{Nodes: []graph.NodeID{"c", "d"}}

	fp := &fakePlacement{m: map[graph.NodeID]string{
		"a": "node1", "b": "node2",
		"c": "node3", "d": "node3", // same node (no penalty)
	}}

	nm := &promnet.NetworkMatrix{
		Links: map[string]*promnet.NodeLinkMetrics{
			"node1||node2": {SrcNode: "node1", DstNode: "node2", AvgLatencyMs: 20, DropRate: 0.05, BandwidthMbps: 5},
		},
	}

	w := scoring.NetWeights{
		NetLatencyWeight:   2.0,
		NetDropWeight:      3.0,
		NetBandwidthWeight: 1.0,
		BadLatencyMs:       5.0,
		BadDropRate:        0.01,
	}

	// Equal base for both (say 50).
	base := 50.0
	aPenalty := scoring.ComputeNetworkPenalty(aToB, fp, nm, w)
	cPenalty := scoring.ComputeNetworkPenalty(cToD, fp, nm, w)

	aFinal := scoring.CombineScores(base, aPenalty)
	cFinal := scoring.CombineScores(base, cPenalty)

	if !(aPenalty > 0 && cPenalty == 0) {
		t.Fatalf("expected penalty on a->b only; got a=%.2f c=%.2f", aPenalty, cPenalty)
	}
	if !(cFinal > aFinal) {
		t.Fatalf("expected c->d ranked above a->b; got aFinal=%.2f cFinal=%.2f", aFinal, cFinal)
	}
}
