package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lead-net-affinity/pkg/config"
	"lead-net-affinity/pkg/controller"
	"lead-net-affinity/pkg/kube"
	promc "lead-net-affinity/pkg/prometheus"
)

func main() {
	cfgPath := os.Getenv("LEAD_NET_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/lead-net-affinity/config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	k8sClient, err := kube.NewInCluster()
	if err != nil {
		log.Fatalf("init k8s client: %v", err)
	}

	promClient, err := promc.NewClient(cfg.Prometheus.URL)
	if err != nil {
		log.Fatalf("init prometheus client: %v", err)
	}

	ctrl := controller.New(cfg, k8sClient, promClient)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := ctrl.Run(ctx); err != nil {
		log.Fatalf("controller error: %v", err)
	}
}
