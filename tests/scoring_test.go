package tests

import (
	"math"
	"testing"

	"lead-net-affinity/pkg/graph"
	"lead-net-affinity/pkg/scoring"
)

func TestBaseScoreAndNormalize(t *testing.T) {
	in := scoring.BaseInput{
		PathLength:       3,
		PodCount:         5,
		ServiceEdgeCount: 2,
		RPS:              10,
	}
	w := scoring.Weights{
		PathLengthWeight:   1.0,
		PodCountWeight:     2.0,
		ServiceEdgesWeight: 3.0,
		RPSWeight:          0.5,
	}
	got := scoring.BaseScore(in, w)
	want := 24.0 // manual check
	if got != want {
		t.Fatalf("BaseScore = %v, want %v", got, want)
	}

	norm := scoring.Normalize([]float64{10, 20, 30})
	if norm[0] != 0 || norm[2] != 100 {
		t.Fatalf("Normalize incorrect: %v", norm)
	}
	flat := scoring.Normalize([]float64{5, 5, 5})
	for _, v := range flat {
		if math.Abs(v-50) > 1e-9 {
			t.Fatalf("flat normalize failed: %v", flat)
		}
	}
}

func TestEstimateHelpers(t *testing.T) {
	p := graph.Path{Nodes: []graph.NodeID{"a", "b", "c"}}
	if scoring.EstimatePodCount(p) != 3 {
		t.Fatal("EstimatePodCount wrong")
	}
	if scoring.EstimateServiceEdges(p) != 2 {
		t.Fatal("EstimateServiceEdges wrong")
	}
}
