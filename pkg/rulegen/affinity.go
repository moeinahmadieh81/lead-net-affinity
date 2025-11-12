package rulegen

import (
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
		return
	}

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
		return
	}

	for i := 0; i < len(path.Nodes)-1; i++ {
		a := path.Nodes[i]
		b := path.Nodes[i+1]

		dA, okA := deploys[a]
		dB, okB := deploys[b]
		if !okA || !okB {
			continue
		}
		if dA.Spec.Template.Labels == nil || len(dA.Spec.Template.Labels) == 0 {
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

		dB.Spec.Template.Spec.Affinity.PodAffinity.
			PreferredDuringSchedulingIgnoredDuringExecution =
			append(
				dB.Spec.Template.Spec.Affinity.PodAffinity.
					PreferredDuringSchedulingIgnoredDuringExecution,
				term,
			)
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
		return
	}

	if d.Spec.Template.Spec.Affinity == nil {
		d.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}
	if d.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
		d.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

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
}
