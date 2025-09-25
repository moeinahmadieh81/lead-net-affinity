package models

import (
	"fmt"
	"sync"
	"time"
)

// NetworkTopology represents network characteristics of a node
type NetworkTopology struct {
	AvailabilityZone string  `json:"availability_zone"` // AWS AZ, GCP zone, etc.
	Bandwidth        float64 `json:"bandwidth"`         // Mbps
	Hops             int     `json:"hops"`              // Number of network hops from gateway
	GeoDistance      float64 `json:"geo_distance"`      // Distance in km from gateway
	Throughput       float64 `json:"throughput"`        // Actual throughput in Mbps
	Latency          float64 `json:"latency"`           // Network latency in ms
	PacketLoss       float64 `json:"packet_loss"`       // Packet loss percentage
}

// ServiceNode represents a microservice in the service mesh
type ServiceNode struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Replicas        int               `json:"replicas"`
	RPS             float64           `json:"rps"` // Requests per second
	NetworkTopology *NetworkTopology  `json:"network_topology"`
	Labels          map[string]string `json:"labels,omitempty"`
}

// PodInfo represents pod information for LEAD framework
type PodInfo struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	ServiceName      string            `json:"service_name"`
	ServiceType      string            `json:"service_type"` // microservice, mongodb, memcached
	NodeName         string            `json:"node_name"`
	PodIP            string            `json:"pod_ip"`
	HostIP           string            `json:"host_ip"`
	Status           string            `json:"status"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	ResourceRequests ResourceInfo      `json:"resource_requests"`
	ResourceLimits   ResourceInfo      `json:"resource_limits"`
	CreationTime     time.Time         `json:"creation_time"`
}

// ResourceInfo represents pod resource information
type ResourceInfo struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// Path represents a sequence of services from gateway to leaf nodes
type Path struct {
	Services     []*ServiceNode `json:"services"`
	Score        float64        `json:"score"`
	Weight       int            `json:"weight"`
	PathLength   int            `json:"path_length"`
	PodCount     int            `json:"pod_count"`
	EdgeCount    int            `json:"edge_count"`
	NetworkScore float64        `json:"network_score"` // Network topology score
}

// ServiceGraph represents the entire service mesh
type ServiceGraph struct {
	Nodes   map[string]*ServiceNode `json:"nodes"`
	Edges   map[string][]string     `json:"edges"`
	Gateway string                  `json:"gateway"`
	mu      sync.RWMutex
}

// NewServiceGraph creates a new service graph
func NewServiceGraph() *ServiceGraph {
	return &ServiceGraph{
		Nodes: make(map[string]*ServiceNode),
		Edges: make(map[string][]string),
	}
}

// AddNode adds a service node to the graph
func (g *ServiceGraph) AddNode(node *ServiceNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Nodes[node.ID] = node
}

// AddEdge adds a dependency edge between two services
func (g *ServiceGraph) AddEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Edges[from] == nil {
		g.Edges[from] = make([]string, 0)
	}
	g.Edges[from] = append(g.Edges[from], to)
}

// SetGateway sets the gateway service
func (g *ServiceGraph) SetGateway(gateway string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Gateway = gateway
}

// GetNode returns a service node by ID
func (g *ServiceGraph) GetNode(id string) (*ServiceNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, exists := g.Nodes[id]
	return node, exists
}

// GetChildren returns all child services of a given service
func (g *ServiceGraph) GetChildren(serviceID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Edges[serviceID]
}

// GetAllNodes returns all nodes in the graph
func (g *ServiceGraph) GetAllNodes() map[string]*ServiceNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make(map[string]*ServiceNode)
	for id, node := range g.Nodes {
		nodes[id] = node
	}
	return nodes
}

// Validate validates the graph structure
func (g *ServiceGraph) Validate() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.Gateway == "" {
		return fmt.Errorf("gateway not set")
	}

	if _, exists := g.Nodes[g.Gateway]; !exists {
		return fmt.Errorf("gateway node '%s' not found in graph", g.Gateway)
	}

	// Check if all edges reference existing nodes
	for from, children := range g.Edges {
		if _, exists := g.Nodes[from]; !exists {
			return fmt.Errorf("edge from non-existent node '%s'", from)
		}

		for _, child := range children {
			if _, exists := g.Nodes[child]; !exists {
				return fmt.Errorf("edge to non-existent node '%s'", child)
			}
		}
	}

	return nil
}

// String returns a string representation of the graph
func (g *ServiceGraph) String() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := fmt.Sprintf("ServiceGraph:\nGateway: %s\n", g.Gateway)
	result += "Nodes:\n"
	for id, node := range g.Nodes {
		result += fmt.Sprintf("  %s: %s (replicas: %d, RPS: %.2f)\n",
			id, node.Name, node.Replicas, node.RPS)
		if node.NetworkTopology != nil {
			result += fmt.Sprintf("    Network: AZ=%s, BW=%.2f Mbps, Hops=%d, Distance=%.2f km\n",
				node.NetworkTopology.AvailabilityZone,
				node.NetworkTopology.Bandwidth,
				node.NetworkTopology.Hops,
				node.NetworkTopology.GeoDistance)
		}
	}

	result += "Edges:\n"
	for from, children := range g.Edges {
		for _, to := range children {
			result += fmt.Sprintf("  %s -> %s\n", from, to)
		}
	}

	return result
}

// GetAdjacentNodes returns all nodes adjacent to the given node
func (g *ServiceGraph) GetAdjacentNodes(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if children, exists := g.Edges[nodeID]; exists {
		return children
	}
	return []string{}
}

// GetIncomingNodes returns all nodes that have edges to the given node
func (g *ServiceGraph) GetIncomingNodes(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	incoming := make([]string, 0)
	for source, targets := range g.Edges {
		for _, target := range targets {
			if target == nodeID {
				incoming = append(incoming, source)
				break
			}
		}
	}

	return incoming
}
