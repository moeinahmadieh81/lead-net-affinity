package rulegen

import (
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"lead-net-affinity/pkg/graph"
)

type AffinityConfig struct {
	MinAffinityWeight int
	MaxAffinityWeight int
}

// GenerateAffinityForPath adds preferred podAffinity between adjacent services on a path.
// It uses the normalized pathScore to choose the affinity weight.
func GenerateAffinityForPath(
	deploys map[graph.NodeID]*appsv1.Deployment,
	path graph.Path,
	pathScore float64,
	cfg AffinityConfig,
) {
	if len(path.Nodes) < 2 {
		log.Printf("[lead-net][affinity] path too short for affinity: %v", path.Nodes)
		return
	}

	log.Printf("[lead-net][affinity] generating affinity for path=%v score=%.2f cfg=%+v",
		path.Nodes, pathScore, cfg)

	// Scale normalized [0,100] to [MinAffinityWeight, MaxAffinityWeight]
	if cfg.MaxAffinityWeight <= 0 {
		cfg.MaxAffinityWeight = 100
	}
	if cfg.MinAffinityWeight < 0 {
		cfg.MinAffinityWeight = 0
	}
	w := cfg.MinAffinityWeight +
		int(pathScore/100.0*float64(cfg.MaxAffinityWeight-cfg.MinAffinityWeight))
	if w <= 0 {
		log.Printf("[lead-net][affinity] computed weight<=0 (%d) for path=%v; skipping", w, path.Nodes)
		return
	}

	log.Printf("[lead-net][affinity] computed affinity weight=%d for path=%v", w, path.Nodes)

	// Track which deployments we've already processed in this run to avoid clearing rules multiple times
	processedDeployments := make(map[graph.NodeID]bool)

	for i := 0; i < len(path.Nodes)-1; i++ {
		a := path.Nodes[i]
		b := path.Nodes[i+1]

		dA, okA := deploys[a]
		dB, okB := deploys[b]
		if !okA || !okB {
			log.Printf("[lead-net][affinity] missing deployments for edge %s -> %s (okA=%v okB=%v); skipping",
				a, b, okA, okB)
			continue
		}
		if dA.Spec.Template.Labels == nil || len(dA.Spec.Template.Labels) == 0 {
			log.Printf("[lead-net][affinity] deployment %s/%s has no template labels; cannot create selector for path edge %s -> %s",
				dA.Namespace, dA.Name, a, b)
			continue
		}

		selector := &metav1.LabelSelector{
			MatchLabels: dA.Spec.Template.Labels,
		}

		term := corev1.WeightedPodAffinityTerm{
			Weight: int32(w),
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey:   "kubernetes.io/hostname",
				LabelSelector: selector,
			},
		}

		// Ensure Affinity & PodAffinity objects exist
		if dB.Spec.Template.Spec.Affinity == nil {
			dB.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		if dB.Spec.Template.Spec.Affinity.PodAffinity == nil {
			dB.Spec.Template.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
		}

		// ⭐⭐ CRITICAL FIX: Clear existing rules only once per deployment per reconciliation
		if !processedDeployments[b] {
			log.Printf("[lead-net][affinity] clearing existing podAffinity rules for deployment %s/%s",
				dB.Namespace, dB.Name)
			dB.Spec.Template.Spec.Affinity.PodAffinity.
				PreferredDuringSchedulingIgnoredDuringExecution = nil
			processedDeployments[b] = true
		}

		// Log the changes being made to PodAffinity
		log.Printf("[lead-net][affinity] adding podAffinity: from service=%s (deployment=%s/%s) to service=%s (deployment=%s/%s) weight=%d",
			a, dA.Namespace, dA.Name, b, dB.Namespace, dB.Name, w)

		// Append the affinity rule
		dB.Spec.Template.Spec.Affinity.PodAffinity.
			PreferredDuringSchedulingIgnoredDuringExecution =
			append(
				dB.Spec.Template.Spec.Affinity.PodAffinity.
					PreferredDuringSchedulingIgnoredDuringExecution,
				term,
			)

		// Log the updated Affinity configuration for the target deployment
		log.Printf("[lead-net][affinity] updated PodAffinity for service=%s (deployment=%s/%s): %d rules",
			b, dB.Namespace, dB.Name, len(dB.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution))
	}
}

// GenerateCleanAffinityForPath is an alternative implementation that completely replaces
// all affinity rules for a deployment with a clean set based on the current path
func GenerateCleanAffinityForPath(
	deploys map[graph.NodeID]*appsv1.Deployment,
	path graph.Path,
	pathScore float64,
	cfg AffinityConfig,
) {
	if len(path.Nodes) < 2 {
		log.Printf("[lead-net][affinity] path too short for affinity: %v", path.Nodes)
		return
	}

	log.Printf("[lead-net][affinity] generating clean affinity for path=%v score=%.2f cfg=%+v",
		path.Nodes, pathScore, cfg)

	// Scale normalized [0,100] to [MinAffinityWeight, MaxAffinityWeight]
	if cfg.MaxAffinityWeight <= 0 {
		cfg.MaxAffinityWeight = 100
	}
	if cfg.MinAffinityWeight < 0 {
		cfg.MinAffinityWeight = 0
	}
	w := cfg.MinAffinityWeight +
		int(pathScore/100.0*float64(cfg.MaxAffinityWeight-cfg.MinAffinityWeight))
	if w <= 0 {
		log.Printf("[lead-net][affinity] computed weight<=0 (%d) for path=%v; skipping", w, path.Nodes)
		return
	}

	log.Printf("[lead-net][affinity] computed affinity weight=%d for path=%v", w, path.Nodes)

	// First, collect all affinity rules for this path
	type affinityRule struct {
		targetDeployment *appsv1.Deployment
		sourceService    graph.NodeID
		weight           int32
		selector         *metav1.LabelSelector
	}

	var rules []affinityRule

	for i := 0; i < len(path.Nodes)-1; i++ {
		a := path.Nodes[i]
		b := path.Nodes[i+1]

		dA, okA := deploys[a]
		dB, okB := deploys[b]
		if !okA || !okB {
			log.Printf("[lead-net][affinity] missing deployments for edge %s -> %s (okA=%v okB=%v); skipping",
				a, b, okA, okB)
			continue
		}
		if dA.Spec.Template.Labels == nil || len(dA.Spec.Template.Labels) == 0 {
			log.Printf("[lead-net][affinity] deployment %s/%s has no template labels; cannot create selector for path edge %s -> %s",
				dA.Namespace, dA.Name, a, b)
			continue
		}

		selector := &metav1.LabelSelector{
			MatchLabels: dA.Spec.Template.Labels,
		}

		rules = append(rules, affinityRule{
			targetDeployment: dB,
			sourceService:    a,
			weight:           int32(w),
			selector:         selector,
		})
	}

	// Now apply all rules to each target deployment
	targetDeployments := make(map[*appsv1.Deployment][]affinityRule)
	for _, rule := range rules {
		targetDeployments[rule.targetDeployment] = append(targetDeployments[rule.targetDeployment], rule)
	}

	for targetDeploy, deployRules := range targetDeployments {
		// Ensure Affinity & PodAffinity objects exist
		if targetDeploy.Spec.Template.Spec.Affinity == nil {
			targetDeploy.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		if targetDeploy.Spec.Template.Spec.Affinity.PodAffinity == nil {
			targetDeploy.Spec.Template.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
		}

		// Clear all existing rules
		log.Printf("[lead-net][affinity] clearing all existing podAffinity rules for deployment %s/%s",
			targetDeploy.Namespace, targetDeploy.Name)
		targetDeploy.Spec.Template.Spec.Affinity.PodAffinity.
			PreferredDuringSchedulingIgnoredDuringExecution = nil

		// Add all new rules for this deployment
		for _, rule := range deployRules {
			term := corev1.WeightedPodAffinityTerm{
				Weight: rule.weight,
				PodAffinityTerm: corev1.PodAffinityTerm{
					TopologyKey:   "kubernetes.io/hostname",
					LabelSelector: rule.selector,
				},
			}

			log.Printf("[lead-net][affinity] adding podAffinity: from service=%s to deployment=%s/%s weight=%d",
				rule.sourceService, targetDeploy.Namespace, targetDeploy.Name, rule.weight)

			targetDeploy.Spec.Template.Spec.Affinity.PodAffinity.
				PreferredDuringSchedulingIgnoredDuringExecution =
				append(
					targetDeploy.Spec.Template.Spec.Affinity.PodAffinity.
						PreferredDuringSchedulingIgnoredDuringExecution,
					term,
				)
		}

		log.Printf("[lead-net][affinity] deployment %s/%s now has %d podAffinity rules",
			targetDeploy.Namespace, targetDeploy.Name,
			len(targetDeploy.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution))
	}
}

// AddAntiAffinityForBadLink adds soft anti-affinity against pods with given labels.
// (Useful when we detect bad network links or overloaded nodes.)
func AddAntiAffinityForBadLink(
	d *appsv1.Deployment,
	badSelector map[string]string,
	weight int32,
) {
	if len(badSelector) == 0 || weight <= 0 {
		log.Printf("[lead-net][affinity] AddAntiAffinityForBadLink: no badSelector or non-positive weight (weight=%d); skipping", weight)
		return
	}

	if d.Spec.Template.Spec.Affinity == nil {
		d.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}
	if d.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
		d.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

	// Clear existing anti-affinity rules to prevent accumulation
	d.Spec.Template.Spec.Affinity.PodAntiAffinity.
		PreferredDuringSchedulingIgnoredDuringExecution = nil

	term := corev1.WeightedPodAffinityTerm{
		Weight: weight,
		PodAffinityTerm: corev1.PodAffinityTerm{
			TopologyKey: "kubernetes.io/hostname",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: badSelector,
			},
		},
	}

	d.Spec.Template.Spec.Affinity.PodAntiAffinity.
		PreferredDuringSchedulingIgnoredDuringExecution =
		append(
			d.Spec.Template.Spec.Affinity.PodAntiAffinity.
				PreferredDuringSchedulingIgnoredDuringExecution,
			term,
		)

	// Log the anti-affinity change
	log.Printf("[lead-net][affinity] added anti-affinity to deployment %s/%s weight=%d selector=%v",
		d.Namespace, d.Name, weight, badSelector)

	// Log the updated Anti-Affinity configuration
	log.Printf("[lead-net][affinity] updated PodAntiAffinity for deployment=%s/%s: %+v",
		d.Namespace, d.Name, d.Spec.Template.Spec.Affinity.PodAntiAffinity)
}

// ClearAllAffinityRules completely removes all affinity and anti-affinity rules from a deployment
func ClearAllAffinityRules(d *appsv1.Deployment) {
	if d.Spec.Template.Spec.Affinity == nil {
		return
	}

	if d.Spec.Template.Spec.Affinity.PodAffinity != nil {
		d.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = nil
	}
	if d.Spec.Template.Spec.Affinity.PodAntiAffinity != nil {
		d.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = nil
	}

	log.Printf("[lead-net][affinity] cleared all affinity rules from deployment %s/%s",
		d.Namespace, d.Name)
}
