package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"lead-framework/internal/lead"
)

// Server provides HTTP endpoints for the LEAD framework
type Server struct {
	framework *lead.LEADFramework
	server    *http.Server
}

// NewServer creates a new HTTP server for LEAD framework
func NewServer(framework *lead.LEADFramework, port int) *Server {
	mux := http.NewServeMux()

	server := &Server{
		framework: framework,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}

	// Register routes
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/ready", server.readyHandler)
	mux.HandleFunc("/status", server.statusHandler)
	mux.HandleFunc("/paths", server.pathsHandler)
	mux.HandleFunc("/health-summary", server.healthSummaryHandler)
	mux.HandleFunc("/network-topology", server.networkTopologyHandler)
	mux.HandleFunc("/reanalyze", server.reanalyzeHandler)

	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// healthHandler provides health check endpoint
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := "healthy"
	if !s.framework.IsRunning() {
		status = "unhealthy"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"service":   "lead-framework",
	})
}

// readyHandler provides readiness check endpoint
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := s.framework.IsRunning()
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now().UTC(),
	})
}

// statusHandler provides framework status
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.framework.GetFrameworkStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// pathsHandler provides critical paths information
func (s *Server) pathsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topN := 10 // Default to top 10 paths
	if n := r.URL.Query().Get("top"); n != "" {
		if parsed, err := fmt.Sscanf(n, "%d", &topN); err != nil || parsed != 1 {
			http.Error(w, "Invalid top parameter", http.StatusBadRequest)
			return
		}
	}

	paths, err := s.framework.GetCriticalPaths(topN)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get critical paths: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"paths":     paths,
		"count":     len(paths),
		"timestamp": time.Now().UTC(),
	})
}

// healthSummaryHandler provides cluster health summary
func (s *Server) healthSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary, err := s.framework.GetClusterHealth()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get cluster health: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// networkTopologyHandler provides network topology analysis
func (s *Server) networkTopologyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	analysis, err := s.framework.GetNetworkTopologyAnalysis()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get network topology analysis: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}

// reanalyzeHandler triggers re-analysis
func (s *Server) reanalyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.framework.Reanalyze()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to reanalyze: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Re-analysis completed successfully",
		"timestamp": time.Now().UTC(),
	})
}
