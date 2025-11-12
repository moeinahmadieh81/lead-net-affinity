package tests

import (
	"os"
	"path/filepath"
	"testing"

	"lead-net-affinity/pkg/config"
)

func TestConfigLoad(t *testing.T) {
	y := `
namespaceSelector: ["ns-a","ns-b"]
graph:
  entry: frontend
  services:
    - name: frontend
      dependsOn: ["search"]
prometheus:
  url: "http://prom:9090"
  nodeRTTQuery: "rtt_q"
  nodeDropRateQuery: "drop_q"
  nodeBandwidthQuery: "bw_q"
  sampleWindow: "5m"
scoring:
  pathLengthWeight: 1
  podCountWeight: 2
  serviceEdgesWeight: 3
  rpsWeight: 0.5
  netLatencyWeight: 2
  netDropWeight: 3
  netBandwidthWeight: 1
affinity:
  topPaths: 3
  minAffinityWeight: 50
  maxAffinityWeight: 100
  badLatencyMs: 5
  badDropRate: 0.01
`
	dir := t.TempDir()
	fp := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(fp, []byte(y), 0644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}

	cfg, err := config.Load(fp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Graph.Entry != "frontend" || len(cfg.Graph.Services) != 1 {
		t.Fatalf("graph not parsed: %+v", cfg.Graph)
	}
	if cfg.Affinity.TopPaths != 3 || cfg.Scoring.PodCountWeight != 2 {
		t.Fatalf("weights/affinity not parsed: %+v %+v", cfg.Scoring, cfg.Affinity)
	}
}
