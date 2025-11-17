package scoring

import (
	"log"

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
	score := w.PathLengthWeight*float64(in.PathLength) +
		w.PodCountWeight*float64(in.PodCount) +
		w.ServiceEdgesWeight*float64(in.ServiceEdgeCount) +
		w.RPSWeight*in.RPS

	log.Printf("[lead-net][score] BaseScore input=%+v weights=%+v score=%f", in, w, score)
	return score
}

func Normalize(scores []float64) []float64 {
	log.Printf("[lead-net][score] Normalize called with %d scores", len(scores))
	if len(scores) == 0 {
		log.Printf("[lead-net][score] Normalize: empty scores, returning as-is")
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
		log.Printf("[lead-net][score] Normalize: all scores equal (min=max=%f), returning %v", minV, out)
		return out
	}
	for i, s := range scores {
		out[i] = (s - minV) / (maxV - minV) * 100.0
	}
	log.Printf("[lead-net][score] Normalize: min=%f max=%f input=%v output=%v", minV, maxV, scores, out)
	return out
}

func EstimatePodCount(p graph.Path) int {
	count := len(p.Nodes)
	log.Printf("[lead-net][score] EstimatePodCount path=%v podCount=%d", p.Nodes, count)
	return count
}

func EstimateServiceEdges(p graph.Path) int {
	if len(p.Nodes) == 0 {
		log.Printf("[lead-net][score] EstimateServiceEdges path empty -> 0 edges")
		return 0
	}
	edges := len(p.Nodes) - 1
	log.Printf("[lead-net][score] EstimateServiceEdges path=%v edges=%d", p.Nodes, edges)
	return edges
}
