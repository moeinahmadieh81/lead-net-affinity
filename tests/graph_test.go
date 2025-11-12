package tests

import (
	"reflect"
	"testing"

	"lead-net-affinity/pkg/graph"
)

func TestGraph_FindAllPaths(t *testing.T) {
	services := []struct {
		Name          string
		DependsOn     []string
		LabelSelector map[string]string
	}{
		{Name: "frontend", DependsOn: []string{"search", "user"}},
		{Name: "search", DependsOn: []string{"profile"}},
		{Name: "user"},
		{Name: "profile"},
	}

	g := graph.NewGraph("frontend", services)
	paths := g.FindAllPaths()
	got := toStringPaths(paths)

	want := [][]string{
		{"frontend", "search", "profile"},
		{"frontend", "user"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected paths:\n got=%v\nwant=%v", got, want)
	}
}

func toStringPaths(paths []graph.Path) [][]string {
	out := make([][]string, len(paths))
	for i, p := range paths {
		row := make([]string, len(p.Nodes))
		for j, n := range p.Nodes {
			row[j] = string(n)
		}
		out[i] = row
	}
	return out
}
