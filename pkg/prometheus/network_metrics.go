package prometheus

import (
	"context"
	"strconv"
)

type NodeLinkMetrics struct {
	SrcNode       string
	DstNode       string
	AvgLatencyMs  float64
	DropRate      float64
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

	if latencyQuery != "" {
		res, err := c.Query(ctx, latencyQuery)
		if err != nil {
			return nil, err
		}
		for _, r := range res.Data.Result {
			src, dst := r.Metric["src_node"], r.Metric["dst_node"]
			if src == "" || dst == "" {
				continue
			}
			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)
			l := m.Links[key(src, dst)]
			if l == nil {
				l = &NodeLinkMetrics{SrcNode: src, DstNode: dst}
				m.Links[key(src, dst)] = l
			}
			l.AvgLatencyMs = v * 1000 // adjust if needed
		}
	}

	if dropQuery != "" {
		res, err := c.Query(ctx, dropQuery)
		if err != nil {
			return nil, err
		}
		for _, r := range res.Data.Result {
			src, dst := r.Metric["src_node"], r.Metric["dst_node"]
			if src == "" || dst == "" {
				continue
			}
			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)
			l := m.Links[key(src, dst)]
			if l == nil {
				l = &NodeLinkMetrics{SrcNode: src, DstNode: dst}
				m.Links[key(src, dst)] = l
			}
			l.DropRate = v
		}
	}

	if bwQuery != "" {
		res, err := c.Query(ctx, bwQuery)
		if err != nil {
			return nil, err
		}
		for _, r := range res.Data.Result {
			src, dst := r.Metric["src_node"], r.Metric["dst_node"]
			if src == "" || dst == "" {
				continue
			}
			valStr, _ := r.Value[1].(string)
			v, _ := strconv.ParseFloat(valStr, 64)
			l := m.Links[key(src, dst)]
			if l == nil {
				l = &NodeLinkMetrics{SrcNode: src, DstNode: dst}
				m.Links[key(src, dst)] = l
			}
			l.BandwidthMbps = v
		}
	}

	return m, nil
}
