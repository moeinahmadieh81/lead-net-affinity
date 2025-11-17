package prometheus

import (
	"context"
	"log"
	"strconv"
	"strings"
)

const (
	// This IP belongs to the master node; we don't want to use it for scoring.
	MasterNodeIP = "202.133.88.12"
)

// NodeMetrics holds per-node network signals derived from Prometheus.
type NodeMetrics struct {
	NodeID        string  // normalized node identifier (node name if possible)
	AvgLatencyMs  float64 // p50 latency in ms
	DropRate      float64 // drop bytes rate (unit depends on query)
	BandwidthRate float64 // flow rate (e.g. flows/sec)
}

// NetworkMatrix now holds *per-node* metrics.
type NetworkMatrix struct {
	Nodes map[string]*NodeMetrics
}

// GetNode returns metrics for a given node ID (or nil if missing).
func (nm *NetworkMatrix) GetNode(nodeID string) *NodeMetrics {
	if nm == nil || nm.Nodes == nil {
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
			nodeLabel := r.Metric["node"]

			// Ignore master
			if inst != "" && isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping latency sample for master instance=%q", inst)
				continue
			}

			// Prefer the Kubernetes node name if present.
			nodeID := nodeLabel
			if nodeID == "" {
				nodeID = normalizeInstance(inst)
			}
			if nodeID == "" {
				log.Printf("[lead-net][debug] skipping latency sample: no usable nodeID (instance=%q node=%q)", inst, nodeLabel)
				continue
			}

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64) // seconds
			latMs := v * 1000.0

			m := nm.getOrCreate(nodeID)
			m.AvgLatencyMs = latMs

			log.Printf("[lead-net][debug] latency node=%s instance=%s raw_sec=%s latency_ms=%f",
				nodeID, inst, valStr, latMs)
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
			nodeLabel := r.Metric["node"]

			if inst != "" && isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping drop sample for master instance=%q", inst)
				continue
			}

			nodeID := nodeLabel
			if nodeID == "" {
				nodeID = normalizeInstance(inst)
			}
			if nodeID == "" {
				log.Printf("[lead-net][debug] skipping drop sample: no usable nodeID (instance=%q node=%q)", inst, nodeLabel)
				continue
			}

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)

			m := nm.getOrCreate(nodeID)
			m.DropRate = v

			log.Printf("[lead-net][debug] drop node=%s instance=%s drop_rate=%f",
				nodeID, inst, v)
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
			nodeLabel := r.Metric["node"]

			if inst != "" && isMasterInstance(inst) {
				log.Printf("[lead-net][debug] skipping bandwidth sample for master instance=%q", inst)
				continue
			}

			nodeID := nodeLabel
			if nodeID == "" {
				nodeID = normalizeInstance(inst)
			}
			if nodeID == "" {
				log.Printf("[lead-net][debug] skipping bandwidth sample: no usable nodeID (instance=%q node=%q)", inst, nodeLabel)
				continue
			}

			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)

			m := nm.getOrCreate(nodeID)
			m.BandwidthRate = v

			log.Printf("[lead-net][debug] bandwidth node=%s instance=%s flow_rate=%f",
				nodeID, inst, v)
		}
	}

	// Final summary
	log.Printf("[lead-net][debug] built NetworkMatrix with %d nodes", len(nm.Nodes))
	for id, m := range nm.Nodes {
		log.Printf("[lead-net][debug] node summary id=%s latency_ms=%f drop=%f flow=%f",
			id, m.AvgLatencyMs, m.DropRate, m.BandwidthRate)
	}

	return nm, nil
}
