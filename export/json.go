package export

import (
	"encoding/json"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// StatsJSON exports graph stats as JSON.
func StatsJSON(g *graph.Graph) ([]byte, error) {
	stats := g.GetStats()
	return json.MarshalIndent(stats, "", "  ")
}

// SubgraphJSON exports a subgraph (nodes + edges) as JSON.
func SubgraphJSON(g *graph.Graph, qualifiedNames []string) ([]byte, error) {
	nameSet := make(map[string]bool, len(qualifiedNames))
	for _, qn := range qualifiedNames {
		nameSet[qn] = true
	}

	type subgraph struct {
		Nodes []*graph.Node `json:"nodes"`
		Edges []*graph.Edge `json:"edges"`
	}

	var sg subgraph

	for _, n := range g.AllNodes() {
		if nameSet[n.QualifiedName] {
			sg.Nodes = append(sg.Nodes, n)
		}
	}

	for _, e := range g.AllEdges() {
		if nameSet[e.SourceQualified] || nameSet[e.TargetQualified] {
			sg.Edges = append(sg.Edges, e)
		}
	}

	return json.MarshalIndent(sg, "", "  ")
}
