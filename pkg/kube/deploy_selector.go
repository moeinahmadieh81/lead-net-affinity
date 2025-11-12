package kube

import (
	appsv1 "k8s.io/api/apps/v1"

	"lead-net-affinity/pkg/graph"
)

const svcLabel = "io.kompose.service"

func MapDeploymentsByService(deploys []appsv1.Deployment) map[graph.NodeID]*appsv1.Deployment {
	m := make(map[graph.NodeID]*appsv1.Deployment)
	for i := range deploys {
		d := &deploys[i]
		if d.Labels == nil {
			continue
		}
		if name, ok := d.Labels[svcLabel]; ok && name != "" {
			m[graph.NodeID(name)] = d
		}
	}
	return m
}
