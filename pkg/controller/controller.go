package controller

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"lead-net-affinity/pkg/config"
	"lead-net-affinity/pkg/graph"
	"lead-net-affinity/pkg/kube"
	promc "lead-net-affinity/pkg/prometheus"
	"lead-net-affinity/pkg/rulegen"
	"lead-net-affinity/pkg/scoring"
)

type LogLevel int

const (
	LogLevelInfo LogLevel = iota
	LogLevelDebug
)

type KubeClient interface {
	ListDeployments(ctx context.Context, namespaces []string) ([]appsv1.Deployment, error)
	UpdateDeployment(ctx context.Context, d *appsv1.Deployment) error
	ListPods(ctx context.Context, namespace, selector string) ([]corev1.Pod, error)
	GetNode(ctx context.Context, name string) (*corev1.Node, error)
}

type PromClient interface {
	FetchNetworkMatrix(ctx context.Context, latencyQuery, dropQuery, bwQuery string) (*promc.NetworkMatrix, error)
}

type Controller struct {
	cfg      *config.Config
	k8s      KubeClient
	prom     PromClient
	logLevel LogLevel
	dryRun   bool
}

// nodeIPResolver implements scoring.NodeIPResolver by using the KubeClient to
// look up a node's InternalIP/ExternalIP and caching the result.
type nodeIPResolver struct {
	k8s   KubeClient
	cache map[string]string
}

// IPForNode returns the IP address for a given Kubernetes node name.
// It prefers InternalIP, then ExternalIP. If no address can be found, it
// returns the empty string and logs at info level.
func (r *nodeIPResolver) IPForNode(nodeName string) string {
	if nodeName == "" {
		return ""
	}
	if ip, ok := r.cache[nodeName]; ok {
		return ip
	}

	node, err := r.k8s.GetNode(context.Background(), nodeName)
	if err != nil {
		log.Printf("[lead-net][ip-resolver] GetNode(%q) failed: %v", nodeName, err)
		r.cache[nodeName] = ""
		return ""
	}

	var internalIP, externalIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP && internalIP == "" {
			internalIP = addr.Address
		}
		if addr.Type == corev1.NodeExternalIP && externalIP == "" {
			externalIP = addr.Address
		}
	}

	ip := internalIP
	if ip == "" {
		ip = externalIP
	}

	if ip == "" {
		log.Printf("[lead-net][ip-resolver] node %q has no InternalIP/ExternalIP addresses", nodeName)
		r.cache[nodeName] = ""
		return ""
	}

	r.cache[nodeName] = ip
	log.Printf("[lead-net][ip-resolver] mapped node %q -> ip %q", nodeName, ip)
	return ip
}

func New(cfg *config.Config, k8s KubeClient, prom PromClient) *Controller {
	level := LogLevelInfo
	if v := strings.ToLower(os.Getenv("LEAD_NET_LOG")); v == "debug" {
		level = LogLevelDebug
	}
	if os.Getenv("LEAD_NET_DEBUG") == "1" {
		level = LogLevelDebug
	}

	dry := false
	if v := strings.ToLower(os.Getenv("LEAD_NET_DRYRUN")); v == "1" || v == "true" || v == "yes" {
		dry = true
	}

	c := &Controller{
		cfg:      cfg,
		k8s:      k8s,
		prom:     prom,
		logLevel: level,
		dryRun:   dry,
	}

	c.infof("starting lead-net-affinity controller")
	c.infof("log level: %s", c.logLevelString())
	c.infof("dry-run: %v", c.dryRun)
	c.infof("namespaces: %v", cfg.NamespaceSelector)
	c.infof("graph entry: %s, services: %d", cfg.Graph.Entry, len(cfg.Graph.Services))
	return c
}

func (c *Controller) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		if err := c.reconcileOnce(ctx); err != nil {
			c.infof("reconcile error: %v", err)
		}
		select {
		case <-ctx.Done():
			c.infof("shutting down controller: %v", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func toServiceDefs(nodes []config.ServiceNode) []struct {
	Name          string
	DependsOn     []string
	LabelSelector map[string]string
} {
	out := make([]struct {
		Name          string
		DependsOn     []string
		LabelSelector map[string]string
	}, len(nodes))

	for i, n := range nodes {
		out[i].Name = n.Name
		out[i].DependsOn = n.DependsOn
		out[i].LabelSelector = n.LabelSelector
	}
	return out
}

func (c *Controller) reconcileOnce(ctx context.Context) error {
	start := time.Now()
	c.debugf("==== reconcile start ====")

	// 1) Graph & paths
	g := graph.NewGraph(c.cfg.Graph.Entry, toServiceDefs(c.cfg.Graph.Services))
	paths := g.FindAllPaths()
	if len(paths) == 0 {
		c.infof("no paths found from entry %q; nothing to do", c.cfg.Graph.Entry)
		c.debugf("==== reconcile end (no paths) ====")
		return nil
	}
	c.debugf("found %d paths from entry %q", len(paths), c.cfg.Graph.Entry)

	// 2) Deployments
	deploysSlice, err := c.k8s.ListDeployments(ctx, c.cfg.NamespaceSelector)
	if err != nil {
		c.infof("ListDeployments failed: %v", err)
		return err
	}
	deploysBySvc := kube.MapDeploymentsByService(deploysSlice)
	c.debugf("found %d deployments across namespaces, mapped %d services",
		len(deploysSlice), len(deploysBySvc))

	// 3) Placement resolver (nodeName lookup per service)
	placements := kube.NewPlacementResolver(c.k8s, c.cfg.NamespaceSelector)

	// ⭐ NEW: Node IP resolver (nodeName -> IP matching Prometheus instance)
	ipResolver := &nodeIPResolver{
		k8s:   c.k8s,
		cache: map[string]string{},
	}

	// 4) Fetch per-node network metrics
	nm, err := c.prom.FetchNetworkMatrix(
		ctx,
		c.cfg.Prometheus.NodeRTTQuery,
		c.cfg.Prometheus.NodeDropRateQuery,
		c.cfg.Prometheus.NodeBandwidthQuery,
	)
	if err != nil {
		c.infof("warning: failed to fetch network metrics; using base-only: %v", err)
	} else if nm == nil {
		c.infof("warning: network matrix is nil; fallback to base-only")
	} else {
		c.debugf("fetched network matrix with %d nodes", len(nm.Nodes))
	}

	// 5) Compute base scores for each path
	baseWeights := scoring.Weights{
		PathLengthWeight:   c.cfg.Scoring.PathLengthWeight,
		PodCountWeight:     c.cfg.Scoring.PodCountWeight,
		ServiceEdgesWeight: c.cfg.Scoring.ServiceEdgesWeight,
		RPSWeight:          c.cfg.Scoring.RPSWeight,
	}
	baseScores := make([]float64, len(paths))
	for i, p := range paths {
		in := scoring.BaseInput{
			PathLength:       len(p.Nodes),
			PodCount:         scoring.EstimatePodCount(p),
			ServiceEdgeCount: scoring.EstimateServiceEdges(p),
			RPS:              0,
		}
		baseScores[i] = scoring.BaseScore(in, baseWeights)
	}
	normBase := scoring.Normalize(baseScores)
	for i := range paths {
		paths[i].BaseScore = normBase[i]
	}

	// 6) Compute network penalties per path
	finalScores := make([]float64, len(paths))
	netWeights := scoring.NetWeights{
		NetLatencyWeight:   c.cfg.Scoring.NetLatencyWeight,
		NetDropWeight:      c.cfg.Scoring.NetDropWeight,
		NetBandwidthWeight: c.cfg.Scoring.NetBandwidthWeight,
		BadLatencyMs:       c.cfg.Affinity.BadLatencyMs,
		BadDropRate:        c.cfg.Affinity.BadDropRate,
	}
	for i := range paths {
		p := &paths[i]
		var pen float64
		if nm != nil {
			pen = scoring.ComputeNetworkPenalty(
				*p,
				placements,
				nm,
				ipResolver, // ⭐ FIXED: this was missing!
				netWeights,
			)
		}
		p.NetworkPenalty = pen
		p.FinalScore = scoring.CombineScores(p.BaseScore, pen)
		finalScores[i] = p.FinalScore
	}
	normFinal := scoring.Normalize(finalScores)
	for i := range paths {
		paths[i].FinalScore = normFinal[i]
	}

	// 7) Sort by final score
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].FinalScore > paths[j].FinalScore
	})

	// 8) Top-K affinity generation
	top := c.cfg.Affinity.TopPaths
	if top <= 0 || top > len(paths) {
		top = len(paths)
	}
	c.infof("evaluated %d paths; top %d:", len(paths), top)
	for i := 0; i < top; i++ {
		p := paths[i]
		c.infof("  path[%d]: %s | base=%.1f netPenalty=%.2f final=%.1f",
			i, formatPath(p), p.BaseScore, p.NetworkPenalty, p.FinalScore)
	}

	affCfg := rulegen.AffinityConfig{
		MinAffinityWeight: c.cfg.Affinity.MinAffinityWeight,
		MaxAffinityWeight: c.cfg.Affinity.MaxAffinityWeight,
	}

	for i := 0; i < top; i++ {
		p := paths[i]
		rulegen.GenerateAffinityForPath(deploysBySvc, p, p.FinalScore, affCfg)
	}

	// 9) Apply or dry-run
	updated := 0
	for _, d := range deploysBySvc {
		if c.dryRun {
			c.infof("dry-run: would update deployment %s/%s", d.Namespace, d.Name)
			continue
		}
		if err := c.k8s.UpdateDeployment(ctx, d); err != nil {
			c.infof("update failed: %s/%s: %v", d.Namespace, d.Name, err)
		} else {
			updated++
		}
	}

	c.infof("reconcile completed in %s; deployments updated: %d",
		time.Since(start).Round(time.Millisecond), updated)
	c.debugf("=`=== reconcile end ====")
	return nil
}

// ---- logging helpers ----

func (c *Controller) logLevelString() string {
	switch c.logLevel {
	case LogLevelDebug:
		return "debug"
	default:
		return "info"
	}
}

func (c *Controller) infof(format string, args ...interface{}) {
	log.Printf("[lead-net] "+format, args...)
}

func (c *Controller) debugf(format string, args ...interface{}) {
	if c.logLevel >= LogLevelDebug {
		log.Printf("[lead-net][debug] "+format, args...)
	}
}

func formatPath(p graph.Path) string {
	parts := make([]string, len(p.Nodes))
	for i, n := range p.Nodes {
		parts[i] = string(n)
	}
	return strings.Join(parts, " -> ")
}

func (c *Controller) ReconcileOnceForTest(ctx context.Context) error {
	return c.reconcileOnce(ctx)
}

func (c *Controller) EnableDryRunForTest() {
	c.dryRun = true
}
