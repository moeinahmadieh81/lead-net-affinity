package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"lead-net-affinity/pkg/graph"
)

// PodLister is the small interface we need from the kube client.
// The real kube.Client already satisfies this.
type PodLister interface {
	ListPods(ctx context.Context, namespace, selector string) ([]corev1.Pod, error)
}

// PlacementResolver knows how to find which node a service
// (graph node) is currently running on.
type PlacementResolver struct {
	k8s        PodLister
	namespaces []string
}

// NewPlacementResolver wires in the kube client and the namespaces
// we care about (from config.yaml: namespaceSelector).
func NewPlacementResolver(k8s PodLister, namespaces []string) *PlacementResolver {
	return &PlacementResolver{
		k8s:        k8s,
		namespaces: namespaces,
	}
}

// NodeNameForService implements scoring.PodPlacement.
// It looks up a pod with label io.kompose.service=<service>
// in the configured namespaces and returns its node name.
func (p *PlacementResolver) NodeNameForService(svcID graph.NodeID) string {
	ctx := context.Background()
	selector := fmt.Sprintf("%s=%s", svcLabel, string(svcID))

	for _, ns := range p.namespaces {
		pods, err := p.k8s.ListPods(ctx, ns, selector)
		if err != nil || len(pods) == 0 {
			continue
		}
		// Any pod for that service is fine; in the benchmark
		// you usually have a small fixed replica count.
		return pods[0].Spec.NodeName
	}

	// Unknown placement
	return ""
}
