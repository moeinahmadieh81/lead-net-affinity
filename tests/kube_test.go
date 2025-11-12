package tests

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"lead-net-affinity/pkg/kube"
)

func TestMapDeploymentsByService(t *testing.T) {
	deploys := []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"io.kompose.service": "frontend"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"io.kompose.service": "search"},
			},
		},
	}

	m := kube.MapDeploymentsByService(deploys)
	if len(m) != 2 {
		t.Fatalf("expected 2 mapped services, got %d", len(m))
	}
	if _, ok := m["frontend"]; !ok {
		t.Fatalf("missing frontend")
	}
}
