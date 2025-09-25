package algorithms

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lead-framework/internal/models"
)

// MonitoringAlgorithm implements Algorithm 2 from the LEAD framework
type MonitoringAlgorithm struct {
	graph             *models.ServiceGraph
	scoringAlgorithm  *ScoringAlgorithm
	threshold         float64
	checkInterval     time.Duration
	latencyThreshold  time.Duration
	resourceThreshold float64
	monitoringData    map[string]*ServiceMetrics
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	isRunning         bool
	scalingCallback   ScalingCallback
	metricsCallback   MetricsCallback
}

// ServiceMetrics contains real-time metrics for a service
type ServiceMetrics struct {
	ServiceID      string        `json:"service_id"`
	CPUUsage       float64       `json:"cpu_usage"`       // Percentage
	MemoryUsage    float64       `json:"memory_usage"`    // Percentage
	NetworkLatency time.Duration `json:"network_latency"` // Milliseconds
	RequestRate    float64       `json:"request_rate"`    // RPS
	ErrorRate      float64       `json:"error_rate"`      // Percentage
	ResponseTime   time.Duration `json:"response_time"`   // Milliseconds
	LastUpdated    time.Time     `json:"last_updated"`
	IsHealthy      bool          `json:"is_healthy"`
}

// ScalingCallback is called when a service needs to be scaled
type ScalingCallback func(serviceID string, currentReplicas int, targetReplicas int) error

// MetricsCallback is called to get current metrics for a service
type MetricsCallback func(serviceID string) (*ServiceMetrics, error)

// NewMonitoringAlgorithm creates a new monitoring algorithm instance
func NewMonitoringAlgorithm(
	graph *models.ServiceGraph,
	scoringAlgorithm *ScoringAlgorithm,
	threshold float64,
	checkInterval time.Duration,
) *MonitoringAlgorithm {
	ctx, cancel := context.WithCancel(context.Background())

	return &MonitoringAlgorithm{
		graph:             graph,
		scoringAlgorithm:  scoringAlgorithm,
		threshold:         threshold,
		checkInterval:     checkInterval,
		latencyThreshold:  100 * time.Millisecond, // Default 100ms
		resourceThreshold: 80.0,                   // Default 80% CPU/Memory
		monitoringData:    make(map[string]*ServiceMetrics),
		ctx:               ctx,
		cancel:            cancel,
		isRunning:         false,
	}
}

// SetScalingCallback sets the callback function for scaling operations
func (ma *MonitoringAlgorithm) SetScalingCallback(callback ScalingCallback) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.scalingCallback = callback
}

// SetMetricsCallback sets the callback function for metrics collection
func (ma *MonitoringAlgorithm) SetMetricsCallback(callback MetricsCallback) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.metricsCallback = callback
}

// SetThresholds sets monitoring thresholds
func (ma *MonitoringAlgorithm) SetThresholds(latency time.Duration, resource float64) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.latencyThreshold = latency
	ma.resourceThreshold = resource
}

// Start begins the real-time monitoring loop (Algorithm 2)
func (ma *MonitoringAlgorithm) Start() error {
	ma.mu.Lock()
	if ma.isRunning {
		ma.mu.Unlock()
		return fmt.Errorf("monitoring is already running")
	}
	ma.isRunning = true
	ma.mu.Unlock()

	log.Println("Starting real-time monitoring...")

	go ma.monitoringLoop()

	return nil
}

// Stop stops the monitoring loop
func (ma *MonitoringAlgorithm) Stop() {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	if !ma.isRunning {
		return
	}

	ma.cancel()
	ma.isRunning = false
	log.Println("Stopped real-time monitoring")
}

// monitoringLoop implements the main monitoring loop from Algorithm 2
func (ma *MonitoringAlgorithm) monitoringLoop() {
	ticker := time.NewTicker(ma.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ma.ctx.Done():
			return
		case <-ticker.C:
			ma.performMonitoringCycle()
		}
	}
}

// performMonitoringCycle performs one iteration of the monitoring algorithm
func (ma *MonitoringAlgorithm) performMonitoringCycle() {
	// Step 1: Check for latency violations
	if !ma.detectLatencyViolation() {
		return // No latency violation detected
	}

	log.Println("Latency violation detected, analyzing bottlenecks...")

	// Step 2: Analyze and identify bottleneck services
	bottleneckServices := ma.analyzeAndIdentifyBottleneck()

	if len(bottleneckServices) == 0 {
		log.Println("No bottleneck services identified")
		return
	}

	log.Printf("Identified %d bottleneck services: %v", len(bottleneckServices), bottleneckServices)

	// Step 3: Call scoring algorithm (Algorithm 1)
	gateway := ma.graph.Gateway
	if gateway == "" {
		log.Println("No gateway set, skipping re-scoring")
		return
	}

	_, err := ma.scoringAlgorithm.ScorePaths(gateway)
	if err != nil {
		log.Printf("Error re-scoring paths: %v", err)
		return
	}

	// Step 4: Scale out bottleneck services
	ma.scaleOut(bottleneckServices)
}

// detectLatencyViolation checks if any service has latency violations
func (ma *MonitoringAlgorithm) detectLatencyViolation() bool {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	for _, metrics := range ma.monitoringData {
		if metrics.NetworkLatency > ma.latencyThreshold ||
			metrics.ResponseTime > ma.latencyThreshold {
			return true
		}
	}

	return false
}

// analyzeAndIdentifyBottleneck implements the bottleneck identification logic
func (ma *MonitoringAlgorithm) analyzeAndIdentifyBottleneck() []string {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	var bottleneckServices []string

	for serviceID, metrics := range ma.monitoringData {
		// Check if service exceeds resource threshold
		totalUsage := (metrics.CPUUsage + metrics.MemoryUsage) / 2.0

		if totalUsage > ma.resourceThreshold {
			bottleneckServices = append(bottleneckServices, serviceID)
			log.Printf("Service %s identified as bottleneck: CPU=%.2f%%, Memory=%.2f%%, Total=%.2f%%",
				serviceID, metrics.CPUUsage, metrics.MemoryUsage, totalUsage)
		}
	}

	return bottleneckServices
}

// scaleOut scales out the identified bottleneck services
func (ma *MonitoringAlgorithm) scaleOut(serviceIDs []string) {
	if ma.scalingCallback == nil {
		log.Println("No scaling callback set, cannot scale services")
		return
	}

	for _, serviceID := range serviceIDs {
		// Get current service information
		service, exists := ma.graph.GetNode(serviceID)
		if !exists {
			log.Printf("Service %s not found in graph", serviceID)
			continue
		}

		// Calculate target replicas (increase by 50% or minimum 1)
		currentReplicas := service.Replicas
		targetReplicas := int(float64(currentReplicas) * 1.5)
		if targetReplicas <= currentReplicas {
			targetReplicas = currentReplicas + 1
		}

		log.Printf("Scaling service %s from %d to %d replicas",
			serviceID, currentReplicas, targetReplicas)

		// Call scaling callback
		if err := ma.scalingCallback(serviceID, currentReplicas, targetReplicas); err != nil {
			log.Printf("Error scaling service %s: %v", serviceID, err)
		} else {
			// Update service replicas in graph
			service.Replicas = targetReplicas
			log.Printf("Successfully scaled service %s to %d replicas", serviceID, targetReplicas)
		}
	}
}

// UpdateMetrics updates metrics for a service
func (ma *MonitoringAlgorithm) UpdateMetrics(serviceID string, metrics *ServiceMetrics) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	metrics.ServiceID = serviceID
	metrics.LastUpdated = time.Now()
	ma.monitoringData[serviceID] = metrics

	// Update RPS in service graph as per LEAD paper
	// "RPS, though not available at the initial deployment, would be gathered by the monitoring system"
	ma.updateServiceRPS(serviceID, metrics.RequestRate)
}

// GetMetrics returns current metrics for a service
func (ma *MonitoringAlgorithm) GetMetrics(serviceID string) (*ServiceMetrics, bool) {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	metrics, exists := ma.monitoringData[serviceID]
	return metrics, exists
}

// GetAllMetrics returns metrics for all services
func (ma *MonitoringAlgorithm) GetAllMetrics() map[string]*ServiceMetrics {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	result := make(map[string]*ServiceMetrics)
	for serviceID, metrics := range ma.monitoringData {
		result[serviceID] = metrics
	}
	return result
}

// IsHealthy checks if a service is healthy based on its metrics
func (ma *MonitoringAlgorithm) IsHealthy(serviceID string) bool {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	metrics, exists := ma.monitoringData[serviceID]
	if !exists {
		return false
	}

	return metrics.IsHealthy &&
		metrics.CPUUsage < ma.resourceThreshold &&
		metrics.MemoryUsage < ma.resourceThreshold &&
		metrics.NetworkLatency <= ma.latencyThreshold &&
		metrics.ResponseTime <= ma.latencyThreshold &&
		metrics.ErrorRate < 5.0 // Less than 5% error rate
}

// GetBottleneckServices returns currently identified bottleneck services
func (ma *MonitoringAlgorithm) GetBottleneckServices() []string {
	return ma.analyzeAndIdentifyBottleneck()
}

// GetHealthSummary returns a summary of service health
func (ma *MonitoringAlgorithm) GetHealthSummary() *HealthSummary {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	summary := &HealthSummary{
		TotalServices:      len(ma.monitoringData),
		HealthyServices:    0,
		BottleneckServices: 0,
		UnhealthyServices:  0,
		AvgCPUUsage:        0.0,
		AvgMemoryUsage:     0.0,
		AvgLatency:         0.0,
		LastUpdated:        time.Now(),
	}

	var totalCPU, totalMemory, totalLatency float64

	for _, metrics := range ma.monitoringData {
		totalCPU += metrics.CPUUsage
		totalMemory += metrics.MemoryUsage
		totalLatency += float64(metrics.NetworkLatency.Milliseconds())

		if ma.IsHealthy(metrics.ServiceID) {
			summary.HealthyServices++
		} else if metrics.CPUUsage > ma.resourceThreshold ||
			metrics.MemoryUsage > ma.resourceThreshold {
			summary.BottleneckServices++
		} else {
			summary.UnhealthyServices++
		}
	}

	if summary.TotalServices > 0 {
		summary.AvgCPUUsage = totalCPU / float64(summary.TotalServices)
		summary.AvgMemoryUsage = totalMemory / float64(summary.TotalServices)
		summary.AvgLatency = totalLatency / float64(summary.TotalServices)
	}

	return summary
}

// updateServiceRPS updates the RPS value in the service graph
// This implements the LEAD paper requirement: "RPS would be gathered by the monitoring system"
func (ma *MonitoringAlgorithm) updateServiceRPS(serviceID string, rps float64) {
	if service, exists := ma.graph.GetNode(serviceID); exists {
		service.RPS = rps
		log.Printf("Updated RPS for service %s to %.2f (gathered by monitoring system)", serviceID, rps)
	}
}

// HealthSummary contains a summary of service health
type HealthSummary struct {
	TotalServices      int       `json:"total_services"`
	HealthyServices    int       `json:"healthy_services"`
	BottleneckServices int       `json:"bottleneck_services"`
	UnhealthyServices  int       `json:"unhealthy_services"`
	AvgCPUUsage        float64   `json:"avg_cpu_usage"`
	AvgMemoryUsage     float64   `json:"avg_memory_usage"`
	AvgLatency         float64   `json:"avg_latency"`
	LastUpdated        time.Time `json:"last_updated"`
}
