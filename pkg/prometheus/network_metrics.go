package prometheus

import (
	"context"
	"strconv"
	"strings"
)

const MasterNodeIP = "202.133.88.12"

// helper to check if instance belongs to master
func isMasterInstance(instance string) bool {
	// instance usually looks like "IP:PORT"
	return strings.HasPrefix(instance, MasterNodeIP+":")
}

type NodeLinkMetrics struct {
	SrcNode      string
	DstNode      string
	AvgLatencyMs float64
	DropRate     float64
	// In our model BandwidthMbps is "flows per second" aggregated
	BandwidthMbps float64
}

type NetworkMatrix struct {
	Links map[string]*NodeLinkMetrics
}

func key(a, b string) string { return a + "||" + b }
func (nm *NetworkMatrix) Get(src, dst string) *NodeLinkMetrics {
	return nm.Links[key(src, dst)]
}

func (c *Client) FetchNetworkMatrix(
	ctx context.Context,
	latencyQuery, dropQuery, bwQuery string,
) (*NetworkMatrix, error) {

	m := &NetworkMatrix{Links: make(map[string]*NodeLinkMetrics)}

	clusterKey := key("cluster", "cluster")
	cluster := &NodeLinkMetrics{
		SrcNode: "cluster",
		DstNode: "cluster",
	}
	m.Links[clusterKey] = cluster

	type agg struct {
		sum   float64
		count int
	}

	filterAndAccumulate := func(query string, target *float64) error {
		if query == "" {
			return nil
		}

		res, err := c.Query(ctx, query)
		if err != nil {
			return err
		}

		var a agg
		for _, r := range res.Data.Result {
			instance := r.Metric["instance"]

			// ⛔ SKIP MASTER NODE
			if isMasterInstance(instance) {
				continue
			}

			valStr, _ := r.Value[1].(string)
			v, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			a.sum += v
			a.count++
		}

		if a.count > 0 {
			*target = a.sum / float64(a.count)
		}

		return nil
	}

	// latency (seconds → ms)
	if latencyQuery != "" {
		var avgSec float64
		if err := filterAndAccumulate(latencyQuery, &avgSec); err != nil {
			return nil, err
		}
		cluster.AvgLatencyMs = avgSec * 1000.0
	}

	// drop rate
	if dropQuery != "" {
		if err := filterAndAccumulate(dropQuery, &cluster.DropRate); err != nil {
			return nil, err
		}
	}

	// flows/s (our bandwidth proxy)
	if bwQuery != "" {
		if err := filterAndAccumulate(bwQuery, &cluster.BandwidthMbps); err != nil {
			return nil, err
		}
	}

	return m, nil
}
