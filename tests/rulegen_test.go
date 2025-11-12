package tests

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	"lead-net-affinity/pkg/graph"
	"lead-net-affinity/pkg/rulegen"
)

func TestGenerateAffinityAndAntiAffinity(t *testing.T) {
	path := graph.Path{Nodes: []graph.NodeID{"svc-a", "svc-b"}}

	dA := &appsv1.Deployment{}
	dA.Spec.Template.Labels = map[string]string{"io.kompose.service": "svc-a"}
	dB := &appsv1.Deployment{}
	dB.Spec.Template.Labels = map[string]string{"io.kompose.service": "svc-b"}

	deploys := map[graph.NodeID]*appsv1.Deployment{
		"svc-a": dA,
		"svc-b": dB,
	}

	cfg := rulegen.AffinityConfig{MinAffinityWeight: 50, MaxAffinityWeight: 100}
	rulegen.GenerateAffinityForPath(deploys, path, 100.0, cfg)

	if dB.Spec.Template.Spec.Affinity == nil ||
		dB.Spec.Template.Spec.Affinity.PodAffinity == nil {
		t.Fatalf("expected pod affinity to be added to svc-b")
	}

	// test anti-affinity
	rulegen.AddAntiAffinityForBadLink(dB, map[string]string{"bad": "node"}, 40)
	if dB.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
		t.Fatalf("expected anti-affinity section to exist")
	}
}
