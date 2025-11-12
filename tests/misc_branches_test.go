package tests

import (
	"testing"

	"lead-net-affinity/pkg/graph"
	"lead-net-affinity/pkg/scoring"
)

func TestNormalizeAndEdgeHelpers_NoHelpersNeeded(t *testing.T) {
	// empty normalize
	out := scoring.Normalize([]float64{})
	if len(out) != 0 {
		t.Fatalf("empty normalize should return empty slice")
	}

	// single-value normalize -> should be 50 per our impl
	one := scoring.Normalize([]float64{42})
	if len(one) != 1 || one[0] != 50 {
		t.Fatalf("single-value normalize must be [50], got %v", one)
	}

	// estimate edges edge-cases
	if e := scoring.EstimateServiceEdges(graph.Path{}); e != 0 {
		t.Fatalf("edges(nil) should be 0")
	}
	if e := scoring.EstimateServiceEdges(graph.Path{Nodes: []graph.NodeID{"x"}}); e != 0 {
		t.Fatalf("edges(one) should be 0")
	}
}
