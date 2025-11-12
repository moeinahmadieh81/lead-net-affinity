package tests

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"lead-net-affinity/pkg/config"
	"lead-net-affinity/pkg/controller"
	promc "lead-net-affinity/pkg/prometheus"
)

// ---- Fakes ----

type fakeKube struct {
	deploys []appsv1.Deployment
	pods    []corev1.Pod
	updated int
}

func (f *fakeKube) ListDeployments(_ context.Context, _ []string) ([]appsv1.Deployment, error) {
	return f.deploys, nil
}

func (f *fakeKube) UpdateDeployment(_ context.Context, d *appsv1.Deployment) error {
	// Just count; in real tests you could diff old/new.
	f.updated++
	_ = d
	return nil
}

func (f *fakeKube) ListPods(_ context.Context, _ string, selector string) ([]corev1.Pod, error) {
	// Very small selector matcher for "io.kompose.service=name"
	const key = "io.kompose.service="
	var name string
	if len(selector) > len(key) && selector[:len(key)] == key {
		name = selector[len(key):]
	}
	var out []corev1.Pod
	for _, p := range f.pods {
		if p.Labels["io.kompose.service"] == name {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeProm struct{}

func (f *fakeProm) FetchNetworkMatrix(_ context.Context, _, _, _ string) (*promc.NetworkMatrix, error) {
	// Return a tiny, neutral matrix: effectively zero penalties.
	return &promc.NetworkMatrix{Links: map[string]*promc.NodeLinkMetrics{}}, nil
}

// ---- Test ----

func TestController_ReconcileOnce_DryRun(t *testing.T) {
	// Minimal config: one simple path: a -> b
	cfg := &config.Config{
		NamespaceSelector: []string{"test-ns"},
		Graph: config.ServiceGraphConfig{
			Entry: "a",
			Services: []config.ServiceNode{
				{Name: "a", DependsOn: []string{"b"}},
				{Name: "b"},
			},
		},
		Prometheus: config.PrometheusConfig{},
		Scoring: config.ScoringWeights{
			PathLengthWeight:   1,
			PodCountWeight:     1,
			ServiceEdgesWeight: 1,
			RPSWeight:          0,
			NetLatencyWeight:   0,
			NetDropWeight:      0,
			NetBandwidthWeight: 0,
		},
		Affinity: config.AffinityConfig{
			TopPaths:          1,
			MinAffinityWeight: 50,
			MaxAffinityWeight: 100,
			BadLatencyMs:      5,
			BadDropRate:       0.01,
		},
	}

	fk := &fakeKube{
		deploys: []appsv1.Deployment{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "test-ns",
					Labels:    map[string]string{"io.kompose.service": "a"},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"io.kompose.service": "a"},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "b",
					Namespace: "test-ns",
					Labels:    map[string]string{"io.kompose.service": "b"},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"io.kompose.service": "b"},
						},
					},
				},
			},
		},
		pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a-pod",
					Namespace: "test-ns",
					Labels:    map[string]string{"io.kompose.service": "a"},
				},
				Spec: corev1.PodSpec{NodeName: "node1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "b-pod",
					Namespace: "test-ns",
					Labels:    map[string]string{"io.kompose.service": "b"},
				},
				Spec: corev1.PodSpec{NodeName: "node1"},
			},
		},
	}

	fp := &fakeProm{}

	// We want dry-run behavior: no real updates.
	// Instead of relying on env, just set the field directly for the test.
	ctrl := controller.New(cfg, fk, fp)
	ctrl.EnableDryRunForTest()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ctrl.ReconcileOnceForTest(ctx); err != nil {
		t.Fatalf("ReconcileOnceForTest returned error: %v", err)
	}

	// In dry-run mode, fakeKube.UpdateDeployment should not be called.
	if fk.updated != 0 {
		t.Fatalf("expected 0 real updates in dry-run, got %d", fk.updated)
	}
}

func TestController_DryRun_GeneratesAffinityInMemory(t *testing.T) {
	cfg := &config.Config{
		NamespaceSelector: []string{"test-ns"},
		Graph: config.ServiceGraphConfig{
			Entry: "a",
			Services: []config.ServiceNode{
				{Name: "a", DependsOn: []string{"b"}},
				{Name: "b"},
			},
		},
		Scoring:  config.ScoringWeights{PathLengthWeight: 1, PodCountWeight: 1, ServiceEdgesWeight: 1},
		Affinity: config.AffinityConfig{TopPaths: 1, MinAffinityWeight: 50, MaxAffinityWeight: 100},
	}
	fk := &fakeKube{
		deploys: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{
				Name: "a", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "a"},
			}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"io.kompose.service": "a"}},
			}}},
			{ObjectMeta: metav1.ObjectMeta{
				Name: "b", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "b"},
			}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"io.kompose.service": "b"}},
			}}},
		},
		pods: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "a-pod", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "a"}}, Spec: corev1.PodSpec{NodeName: "node1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "b-pod", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "b"}}, Spec: corev1.PodSpec{NodeName: "node1"}},
		},
	}
	fp := &fakeProm{}

	ctrl := controller.New(cfg, fk, fp)
	ctrl.EnableDryRunForTest()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ctrl.ReconcileOnceForTest(ctx); err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// svc-b (index 1) should now have PodAffinity terms (generated in-memory)
	aff := fk.deploys[1].Spec.Template.Spec.Affinity
	if aff == nil || aff.PodAffinity == nil || len(aff.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution) == 0 {
		t.Fatalf("expected generated podAffinity on svc-b in dry-run")
	}
	// still no real updates
	if fk.updated != 0 {
		t.Fatalf("dry-run: expected 0 updates, got %d", fk.updated)
	}
}

func TestController_NonDryRun_AppliesUpdates(t *testing.T) {
	cfg := &config.Config{
		NamespaceSelector: []string{"test-ns"},
		Graph: config.ServiceGraphConfig{
			Entry: "a",
			Services: []config.ServiceNode{
				{Name: "a", DependsOn: []string{"b"}},
				{Name: "b"},
			},
		},
		Scoring:  config.ScoringWeights{PathLengthWeight: 1, PodCountWeight: 1, ServiceEdgesWeight: 1},
		Affinity: config.AffinityConfig{TopPaths: 1, MinAffinityWeight: 50, MaxAffinityWeight: 100},
	}
	fk := &fakeKube{
		deploys: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{
				Name: "a", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "a"},
			}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"io.kompose.service": "a"}},
			}}},
			{ObjectMeta: metav1.ObjectMeta{
				Name: "b", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "b"},
			}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"io.kompose.service": "b"}},
			}}},
		},
		pods: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "a-pod", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "a"}}, Spec: corev1.PodSpec{NodeName: "node1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "b-pod", Namespace: "test-ns", Labels: map[string]string{"io.kompose.service": "b"}}, Spec: corev1.PodSpec{NodeName: "node1"}},
		},
	}
	fp := &fakeProm{}

	ctrl := controller.New(cfg, fk, fp) // not dry-run
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ctrl.ReconcileOnceForTest(ctx); err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// Controller updates every mapped deployment once at the end (2 here)
	if fk.updated == 0 {
		t.Fatalf("expected updates in non-dry-run, got %d", fk.updated)
	}
}
