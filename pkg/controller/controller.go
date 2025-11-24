package controller

import (
	"context"
	"fmt"
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
	DeletePod(ctx context.Context, namespace, name string) error // NEW: Added for rebalancing
}

type PromClient interface {
	FetchNetworkMatrix(ctx context.Context, latencyQuery, dropQuery, bwQuery string) (*promc.NetworkMatrix, error)
}

type Controller struct {
	cfg       *config.Config
	k8s       KubeClient
	prom      PromClient
	logLevel  LogLevel
	dryRun    bool
	dryDelete bool // NEW: Control pod deletion separately
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

	dryDelete := true // Default to safe mode
	if v := strings.ToLower(os.Getenv("LEAD_NET_DRY_DELETE")); v == "0" || v == "false" || v == "no" {
		dryDelete = false
	}

	c := &Controller{
		cfg:       cfg,
		k8s:       k8s,
		prom:      prom,
		logLevel:  level,
		dryRun:    dry,
		dryDelete: dryDelete, // NEW
	}

	c.infof("starting lead-net-affinity controller")
	c.infof("log level: %s", c.logLevelString())
	c.infof("dry-run: %v", c.dryRun)
	c.infof("dry-delete: %v", c.dryDelete) // NEW
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

// NEW: method for one-time execution
func (c *Controller) RunOnce(ctx context.Context) error {
	c.infof("=== LEAD-NET ONE-TIME RECONCILIATION ===")
	c.infof("This will run reconciliation once and then exit")
	c.infof("Dry-run mode: %v", c.dryRun)
	c.infof("Dry-delete mode: %v", c.dryDelete)

	// Directly call reconcileOnce instead of Run
	if err := c.reconcileOnce(ctx); err != nil {
		c.infof("one-time reconciliation failed: %v", err)
		return err
	}

	c.infof("=== ONE-TIME RECONCILIATION COMPLETED ===")
	return nil
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

// NEW: identifies nodes that should be avoided based on network metrics
func (c *Controller) IdentifyBadNodes(matrix *promc.NetworkMatrix) []string {
	if matrix == nil {
		return nil
	}

	var badNodes []string
	thresholdDropRate := c.cfg.Scoring.BadDropRate
	thresholdLatency := c.cfg.Scoring.BadLatencyMs

	c.debugf("identifying bad nodes with thresholds: dropRate=%.2f, latency=%.2fms",
		thresholdDropRate, thresholdLatency)

	for nodeID, metrics := range matrix.Nodes {
		isBad := false

		// Check drop rate
		if metrics.DropRate > thresholdDropRate {
			c.infof("node %s has high drop rate: %.2f > %.2f", nodeID, metrics.DropRate, thresholdDropRate)
			isBad = true
		}

		// Check latency
		if metrics.AvgLatencyMs > thresholdLatency {
			c.infof("node %s has high latency: %.2fms > %.2fms", nodeID, metrics.AvgLatencyMs, thresholdLatency)
			isBad = true
		}

		if isBad {
			// Convert IP to node name if needed
			nodeName := c.resolveNodeName(nodeID)
			if nodeName != "" {
				badNodes = append(badNodes, nodeName)
				c.infof("marked node %s (%s) as bad", nodeName, nodeID)
			} else {
				c.infof("could not resolve node name for %s", nodeID)
			}
		}
	}

	c.infof("identified %d bad nodes: %v", len(badNodes), badNodes)
	return badNodes
}

// NEW: Helper function to resolve node name from IP
func (c *Controller) resolveNodeName(nodeID string) string {
	// If it's already a node name, return as is
	if strings.HasPrefix(nodeID, "k8s-") {
		return nodeID
	}

	// For IP addresses, we need to map them to node names
	// This is a simplified implementation - in production you'd want to cache this
	ctx := context.Background()
	nodes, err := c.k8s.ListPods(ctx, "", "") // Empty namespace and selector to get all pods
	if err != nil {
		c.debugf("failed to list pods for node resolution: %v", err)
		return nodeID
	}

	// Look for any pod on this node to get the node name
	for _, pod := range nodes {
		if pod.Status.PodIP == nodeID || strings.HasPrefix(pod.Spec.NodeName, "k8s-") {
			// Try to get node info to verify
			node, err := c.k8s.GetNode(ctx, pod.Spec.NodeName)
			if err == nil {
				for _, addr := range node.Status.Addresses {
					if (addr.Type == corev1.NodeInternalIP || addr.Type == corev1.NodeExternalIP) && addr.Address == nodeID {
						return pod.Spec.NodeName
					}
				}
			}
		}
	}

	c.debugf("could not resolve node name for %s, using as-is", nodeID)
	return nodeID
}

// NEW: RebalancePods detects stuck pods on bad nodes and triggers rescheduling
func (c *Controller) RebalancePods(ctx context.Context, deployments []appsv1.Deployment, badNodes []string) error {
	if len(badNodes) == 0 {
		c.infof("no bad nodes identified for rebalancing")
		return nil
	}

	c.infof("checking for rebalancing opportunities, bad nodes: %v", badNodes)

	podsOnBadNodes := 0
	podsToRebalance := []corev1.Pod{}

	for _, d := range deployments {
		selector := fmt.Sprintf("io.kompose.service=%s", d.Labels["io.kompose.service"])
		pods, err := c.k8s.ListPods(ctx, d.Namespace, selector)
		if err != nil {
			c.infof("failed to list pods for %s: %v", d.Name, err)
			continue
		}

		for _, pod := range pods {
			if contains(badNodes, pod.Spec.NodeName) {
				podsOnBadNodes++
				podsToRebalance = append(podsToRebalance, pod)

				c.infof("pod %s/%s is on bad node %s", pod.Namespace, pod.Name, pod.Spec.NodeName)

				// Add node anti-affinity to prevent rescheduling on bad nodes
				deployCopy := d // Create a copy to avoid modifying the original
				c.addNodeAntiAffinity(&deployCopy, badNodes)

				// Update the deployment with anti-affinity
				if !c.dryRun {
					if err := c.k8s.UpdateDeployment(ctx, &deployCopy); err != nil {
						c.infof("failed to update deployment %s with anti-affinity: %v", d.Name, err)
					} else {
						c.infof("successfully added anti-affinity to deployment %s", d.Name)
					}
				}
			}
		}
	}

	c.infof("found %d pods on bad nodes that need rebalancing", podsOnBadNodes)
	if len(podsToRebalance) > 0 {
		c.infof("triggering rescheduling for %d pods", len(podsToRebalance))
		if err := c.triggerPodRescheduling(ctx, podsToRebalance); err != nil {
			return err
		}
	}

	return nil
}

// NEW: AddNodeAntiAffinity adds anti-affinity rules to avoid bad nodes
func (c *Controller) addNodeAntiAffinity(d *appsv1.Deployment, badNodes []string) {
	if d.Spec.Template.Spec.Affinity == nil {
		d.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}
	if d.Spec.Template.Spec.Affinity.NodeAffinity == nil {
		d.Spec.Template.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}

	requirement := corev1.NodeSelectorRequirement{
		Key:      "kubernetes.io/hostname",
		Operator: corev1.NodeSelectorOpNotIn,
		Values:   badNodes,
	}

	// Check if this anti-affinity already exists
	for _, term := range d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		for _, expr := range term.Preference.MatchExpressions {
			if expr.Key == "kubernetes.io/hostname" && expr.Operator == corev1.NodeSelectorOpNotIn {
				// Already exists, check if values need updating
				if equalSlices(expr.Values, badNodes) {
					return // Already configured
				}
			}
		}
	}

	// Add new anti-affinity rule
	d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
		d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
		corev1.PreferredSchedulingTerm{
			Weight: 100, // High weight to strongly avoid bad nodes
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{requirement},
			},
		},
	)

	c.infof("added node anti-affinity to deployment %s/%s to avoid nodes: %v",
		d.Namespace, d.Name, badNodes)
}

// NEW: TriggerPodRescheduling actually deletes pods to force rescheduling
func (c *Controller) triggerPodRescheduling(ctx context.Context, pods []corev1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	c.infof("triggering rescheduling for %d pods", len(pods))

	deletedCount := 0
	for _, pod := range pods {
		podInfo := fmt.Sprintf("%s/%s on node %s", pod.Namespace, pod.Name, pod.Spec.NodeName)

		if c.dryRun || c.dryDelete {
			c.infof("DRY-RUN: would delete pod %s to trigger rescheduling", podInfo)
			continue
		}

		// Check pod age - don't delete very young pods
		podAge := time.Since(pod.CreationTimestamp.Time)
		minPodAge := 30 * time.Second // Minimum 30 seconds old
		if podAge < minPodAge {
			c.infof("skipping pod %s - too young (age: %v)", podInfo, podAge)
			continue
		}

		c.infof("deleting pod %s to trigger rescheduling (age: %v)", podInfo, podAge)
		if err := c.k8s.DeletePod(ctx, pod.Namespace, pod.Name); err != nil {
			c.infof("failed to delete pod %s: %v", podInfo, err)
		} else {
			deletedCount++
			c.infof("successfully deleted pod %s", podInfo)
		}

		// Small delay to avoid overwhelming the API server
		time.Sleep(100 * time.Millisecond)
	}

	c.infof("triggered rescheduling for %d pods (%d actually deleted)", len(pods), deletedCount)
	return nil
}

// NEW: Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

		// ⭐⭐ NEW: Identify bad nodes and trigger rebalancing
		badNodes := c.IdentifyBadNodes(nm)
		if len(badNodes) > 0 {
			c.infof("detected %d bad nodes that need rebalancing: %v", len(badNodes), badNodes)
			if err := c.RebalancePods(ctx, deploysSlice, badNodes); err != nil {
				c.infof("rebalancing failed: %v", err)
			}
		}
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
		BadLatencyMs:       c.cfg.Scoring.BadLatencyMs,
		BadDropRate:        c.cfg.Scoring.BadDropRate,
		BadBandwidthRate:   c.cfg.Scoring.BadBandwidthRate,
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

	// ⭐⭐ CRITICAL FIX: Use the clean version to prevent rule accumulation
	for i := 0; i < top; i++ {
		p := paths[i]
		rulegen.GenerateCleanAffinityForPath(deploysBySvc, p, p.FinalScore, affCfg)
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
