package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"lead-net-affinity/pkg/graph"
)

type PlacementResolver struct {
	client     PodLister
	namespaces []string
}

type PodLister interface {
	ListPods(ctx context.Context, namespace, selector string) ([]corev1.Pod, error)
}

func NewPlacementResolver(client PodLister, namespaces []string) *PlacementResolver {
	return &PlacementResolver{
		client:     client,
		namespaces: namespaces,
	}
}

func (r *PlacementResolver) NodeNameForService(svc graph.NodeID) string {
	value := string(svc)

	for _, ns := range r.namespaces {
		pods, err := r.client.ListPods(context.TODO(), ns, "io.kompose.service="+value)
		if err != nil || len(pods) == 0 {
			continue
		}
		for _, p := range pods {
			if p.Spec.NodeName != "" {
				return p.Spec.NodeName
			}
		}
	}
	return ""
}
