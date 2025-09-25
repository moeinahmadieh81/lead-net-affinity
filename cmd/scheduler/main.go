package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lead-framework/internal/lead"
	"lead-framework/internal/scheduler"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	var (
		kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig file")
		configFile = flag.String("config", "", "Path to LEAD scheduler config file")
		_          = flag.Int("port", 10259, "Port for scheduler metrics")
	)
	flag.Parse()

	// Set up logging
	klog.InitFlags(nil)
	err := flag.Set("v", "2")
	if err != nil {
		return
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Kubernetes client
	kubeClient, err := createKubernetesClient(*kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create LEAD framework
	leadFramework := lead.NewLEADFrameworkWithConfig(config)

	// Create scheduler
	scheduler := scheduler.NewLEADScheduler(kubeClient, leadFramework, config)

	// Start the scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Start scheduler
	log.Println("Starting LEAD Kubernetes Scheduler...")
	if err := scheduler.Run(ctx); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}

	log.Println("LEAD Scheduler stopped")
}

func loadConfig(configFile string) (*lead.FrameworkConfig, error) {
	// Default configuration
	config := lead.DefaultFrameworkConfig()

	// TODO: Load from file if provided
	if configFile != "" {
		// Implement config file loading
		log.Printf("Loading config from file: %s", configFile)
	}

	return config, nil
}

func createKubernetesClient(kubeconfig string) (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
		}
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %v", err)
		}
	}

	return kubernetes.NewForConfig(config)
}
