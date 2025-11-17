package kube

import (
	"context"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	cs *kubernetes.Clientset
}

func NewInCluster() (*Client, error) {
	log.Printf("[lead-net][kube] creating in-cluster Kubernetes client")
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("[lead-net][kube] InClusterConfig failed: %v", err)
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("[lead-net][kube] NewForConfig failed: %v", err)
		return nil, err
	}
	log.Printf("[lead-net][kube] in-cluster client successfully created")
	return &Client{cs: cs}, nil
}

func (c *Client) ListDeployments(ctx context.Context, namespaces []string) ([]appsv1.Deployment, error) {
	log.Printf("[lead-net][kube] ListDeployments request for namespaces=%v", namespaces)

	var out []appsv1.Deployment
	for _, ns := range namespaces {
		list, err := c.cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("[lead-net][kube] ListDeployments failed for namespace=%s: %v", ns, err)
			return nil, err
		}
		log.Printf("[lead-net][kube] ListDeployments namespace=%s returned %d deployments", ns, len(list.Items))
		out = append(out, list.Items...)
	}
	log.Printf("[lead-net][kube] ListDeployments total deployments=%d across namespaces=%v", len(out), namespaces)
	return out, nil
}

func (c *Client) UpdateDeployment(ctx context.Context, d *appsv1.Deployment) error {
	log.Printf("[lead-net][kube] UpdateDeployment %s/%s starting", d.Namespace, d.Name)
	_, err := c.cs.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("[lead-net][kube] UpdateDeployment %s/%s failed: %v", d.Namespace, d.Name, err)
		return err
	}
	log.Printf("[lead-net][kube] UpdateDeployment %s/%s succeeded", d.Namespace, d.Name)
	return nil
}

func (c *Client) ListPods(ctx context.Context, namespace, selector string) ([]corev1.Pod, error) {
	log.Printf("[lead-net][kube] ListPods namespace=%s selector=%q", namespace, selector)
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		log.Printf("[lead-net][kube] ListPods namespace=%s selector=%q failed: %v", namespace, selector, err)
		return nil, err
	}
	log.Printf("[lead-net][kube] ListPods namespace=%s selector=%q returned %d pods", namespace, selector, len(pods.Items))
	return pods.Items, nil
}

func (c *Client) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	log.Printf("[lead-net][kube] GetNode %q", name)
	node, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Printf("[lead-net][kube] GetNode %q failed: %v", name, err)
		return nil, err
	}
	log.Printf("[lead-net][kube] GetNode %q succeeded", name)
	return node, nil
}
