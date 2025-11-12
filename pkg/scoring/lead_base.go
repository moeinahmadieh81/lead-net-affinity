package scoring

import (
	"lead-net-affinity/pkg/graph"
)

type BaseInput struct {
	PathLength       int
	PodCount         int
	ServiceEdgeCount int
	RPS              float64
}

type Weights struct {
	PathLengthWeight   float64
	PodCountWeight     float64
	ServiceEdgesWeight float64
	RPSWeight          float64
}

func BaseScore(in BaseInput, w Weights) float64 {
	return w.PathLengthWeight*float64(in.PathLength) +
		w.PodCountWeight*float64(in.PodCount) +
		w.ServiceEdgesWeight*float64(in.ServiceEdgeCount) +
		w.RPSWeight*in.RPS
}

func Normalize(scores []float64) []float64 {
	if len(scores) == 0 {
		return scores
	}
	minV, maxV := scores[0], scores[0]
	for _, s := range scores {
		if s < minV {
			minV = s
		}
		if s > maxV {
			maxV = s
		}
	}
	out := make([]float64, len(scores))
	if maxV-minV == 0 {
		for i := range out {
			out[i] = 50
		}
		return out
	}
	for i, s := range scores {
		out[i] = (s - minV) / (maxV - minV) * 100.0
	}
	return out
}

func EstimatePodCount(p graph.Path) int {
	return len(p.Nodes)
}

func EstimateServiceEdges(p graph.Path) int {
	if len(p.Nodes) == 0 {
		return 0
	}
	return len(p.Nodes) - 1
}
