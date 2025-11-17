package prometheus

import (
	"context"
	"log"
	"strconv"
	"strings"
)

const (
	MasterNodeIP = "202.133.88.12"
)

// NodeMetrics holds per-node network signals derived from Prometheus.
type NodeMetrics struct {
	NodeID        string  // normalized node identifier (e.g. "91.228.186.28")
	AvgLatencyMs  float64 // p50 latency in ms
	DropRate      float64 // drop bytes rate (whatever unit your query returns)
	BandwidthRate float64 // flow rate (e.g. flows/sec)
}

// NetworkMatrix now holds *per-node* metrics instead of src/dst pairs.
type NetworkMatrix struct {
	Nodes map[string]*NodeMetrics
}

// GetNode returns metrics for a given node ID (or nil if missing).
func (nm *NetworkMatrix) GetNode(nodeID string) *NodeMetrics {
	if nm == nil {
		return nil
	}
	return nm.Nodes[nodeID]
}

// normalizeInstance("91.228.186.28:9962") -> "91.228.186.28".
func normalizeInstance(inst string) string {
	if inst == "" {
		return ""
	}
	if i := strings.IndexByte(inst, ':'); i != -1 {
		return inst[:i]
	}
	return inst
}

func isMasterInstance(inst string) bool {
	return normalizeInstance(inst) == MasterNodeIP
}

func (nm *NetworkMatrix) getOrCreate(nodeID string) *NodeMetrics {
	if nm.Nodes == nil {
		nm.Nodes = make(map[string]*NodeMetrics)
	}
	if m, ok := nm.Nodes[nodeID]; ok {
		return m
	}
	m := &NodeMetrics{NodeID: nodeID}
	nm.Nodes[nodeID] = m
	return m
}

// FetchNetworkMatrix queries Prometheus and builds a per-node view.
func (c *Client) FetchNetworkMatrix(
	ctx context.Context,
	latencyQuery, dropQuery, bwQuery string,
) (*NetworkMatrix, error) {

	nm := &NetworkMatrix{Nodes: make(map[string]*NodeMetrics)}

	// 1) Latency (seconds -> ms)
	if latencyQuery != "" {
		res, err := c.Query(ctx, latencyQuery)
		if err != nil {
			return nil, err
		}
		log.Printf("[lead-net][debug] latency query returned %d series", len(res.Data.Result))

		for _, r := range res.Data.Result {
			inst := r.Metric["instance"]
			if inst == "" || isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping latency sample for instance=%q (empty or master)", inst)
				continue
			}
			nodeID := normalizeInstance(inst)

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64) // seconds
			m := nm.getOrCreate(nodeID)
			m.AvgLatencyMs = v * 1000.0

			log.Printf(
				"[lead-net][debug] latency node=%s instance=%s raw_sec=%.6f latency_ms=%.6f",
				nodeID, inst, v, m.AvgLatencyMs,
			)
		}
	}

	// 2) Drop bytes rate
	if dropQuery != "" {
		res, err := c.Query(ctx, dropQuery)
		if err != nil {
			return nil, err
		}
		log.Printf("[lead-net][debug] drop query returned %d series", len(res.Data.Result))

		for _, r := range res.Data.Result {
			inst := r.Metric["instance"]
			if inst == "" || isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping drop sample for instance=%q (empty or master)", inst)
				continue
			}
			nodeID := normalizeInstance(inst)

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)
			m := nm.getOrCreate(nodeID)
			m.DropRate = v

			log.Printf(
				"[lead-net][debug] drop node=%s instance=%s drop_rate=%.6f",
				nodeID, inst, m.DropRate,
			)
		}
	}

	// 3) Flow rate (as a proxy for bandwidth / load)
	if bwQuery != "" {
		res, err := c.Query(ctx, bwQuery)
		if err != nil {
			return nil, err
		}
		log.Printf("[lead-net][debug] bandwidth/flow query returned %d series", len(res.Data.Result))

		for _, r := range res.Data.Result {
			inst := r.Metric["instance"]
			if inst == "" || isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping bandwidth sample for instance=%q (empty or master)", inst)
				continue
			}
			nodeID := normalizeInstance(inst)

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)
			m := nm.getOrCreate(nodeID)
			m.BandwidthRate = v

			log.Printf(
				"[lead-net][debug] bandwidth node=%s instance=%s flow_rate=%.6f",
				nodeID, inst, m.BandwidthRate,
			)
		}
	}

	log.Printf("[lead-net][debug] built NetworkMatrix with %d nodes", len(nm.Nodes))
	for id, n := range nm.Nodes {
		log.Printf(
			"[lead-net][debug] node summary id=%s latency_ms=%.6f drop=%.6f flow=%.6f",
			id, n.AvgLatencyMs, n.DropRate, n.BandwidthRate,
		)
	}

	return nm, nil
}
