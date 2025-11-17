package graph

import "log"

type NodeID string

type Node struct {
	ID            NodeID
	DependsOn     []NodeID
	LabelSelector map[string]string
}

type Graph struct {
	Nodes map[NodeID]*Node
	Entry NodeID
}

func NewGraph(entry string, services []struct {
	Name          string
	DependsOn     []string
	LabelSelector map[string]string
}) *Graph {
	log.Printf("[lead-net][graph] building graph for entry=%s with %d services", entry, len(services))

	g := &Graph{
		Nodes: map[NodeID]*Node{},
		Entry: NodeID(entry),
	}

	for _, s := range services {
		id := NodeID(s.Name)
		g.Nodes[id] = &Node{
			ID:            id,
			LabelSelector: s.LabelSelector,
		}
	}
	log.Printf("[lead-net][graph] created %d nodes", len(g.Nodes))

	for _, s := range services {
		n := g.Nodes[NodeID(s.Name)]
		for _, dep := range s.DependsOn {
			n.DependsOn = append(n.DependsOn, NodeID(dep))
		}
		log.Printf("[lead-net][graph] node=%s dependsOn=%v", s.Name, n.DependsOn)
	}

	log.Printf("[lead-net][graph] graph build complete; entry=%s nodes=%d", entry, len(g.Nodes))
	return g
}

type Path struct {
	Nodes          []NodeID
	BaseScore      float64
	NetworkPenalty float64
	FinalScore     float64
}

func (g *Graph) FindAllPaths() []Path {
	log.Printf("[lead-net][graph] FindAllPaths from entry=%s", g.Entry)

	var result []Path

	var dfs func(cur NodeID, current []NodeID)
	dfs = func(cur NodeID, current []NodeID) {
		current = append(current, cur)
		node := g.Nodes[cur]
		if len(node.DependsOn) == 0 {
			cp := make([]NodeID, len(current))
			copy(cp, current)
			result = append(result, Path{Nodes: cp})
			log.Printf("[lead-net][graph] discovered terminal path: %v", cp)
			return
		}
		for _, dep := range node.DependsOn {
			log.Printf("[lead-net][graph] traversing %s -> %s", cur, dep)
			dfs(dep, current)
		}
	}

	dfs(g.Entry, []NodeID{})
	log.Printf("[lead-net][graph] FindAllPaths complete; totalPaths=%d", len(result))
	return result
}
