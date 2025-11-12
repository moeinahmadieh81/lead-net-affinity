package graph

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

	for _, s := range services {
		n := g.Nodes[NodeID(s.Name)]
		for _, dep := range s.DependsOn {
			n.DependsOn = append(n.DependsOn, NodeID(dep))
		}
	}

	return g
}

type Path struct {
	Nodes          []NodeID
	BaseScore      float64
	NetworkPenalty float64
	FinalScore     float64
}

func (g *Graph) FindAllPaths() []Path {
	var result []Path

	var dfs func(cur NodeID, current []NodeID)
	dfs = func(cur NodeID, current []NodeID) {
		current = append(current, cur)
		node := g.Nodes[cur]
		if len(node.DependsOn) == 0 {
			cp := make([]NodeID, len(current))
			copy(cp, current)
			result = append(result, Path{Nodes: cp})
			return
		}
		for _, dep := range node.DependsOn {
			dfs(dep, current)
		}
	}

	dfs(g.Entry, []NodeID{})
	return result
}
