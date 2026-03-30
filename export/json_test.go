package export

import (
	"encoding/json"
	"testing"

	"github.com/JSLEEKR/codegraph-go/graph"
)

func TestStatsJSON(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::f", Kind: graph.KindFunction, Name: "f", FilePath: "a", Language: "go"})

	b, err := StatsJSON(g)
	if err != nil {
		t.Fatalf("StatsJSON: %v", err)
	}

	var stats graph.Stats
	if err := json.Unmarshal(b, &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stats.TotalNodes != 1 {
		t.Errorf("expected 1 node, got %d", stats.TotalNodes)
	}
}

func TestStatsJSONEmpty(t *testing.T) {
	g := graph.New()
	b, err := StatsJSON(g)
	if err != nil {
		t.Fatalf("StatsJSON: %v", err)
	}

	var stats graph.Stats
	json.Unmarshal(b, &stats)
	if stats.TotalNodes != 0 {
		t.Errorf("expected 0 nodes, got %d", stats.TotalNodes)
	}
}

func TestSubgraphJSON(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::f", Kind: graph.KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::g", Kind: graph.KindFunction, Name: "g", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::h", Kind: graph.KindFunction, Name: "h", FilePath: "a"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "a::g", TargetQualified: "a::h"})

	b, err := SubgraphJSON(g, []string{"a::f", "a::g"})
	if err != nil {
		t.Fatalf("SubgraphJSON: %v", err)
	}

	var result struct {
		Nodes []*graph.Node `json:"nodes"`
		Edges []*graph.Edge `json:"edges"`
	}
	json.Unmarshal(b, &result)

	if len(result.Nodes) != 2 {
		t.Errorf("expected 2 nodes in subgraph, got %d", len(result.Nodes))
	}
}

func TestSubgraphJSONEmpty(t *testing.T) {
	g := graph.New()
	b, err := SubgraphJSON(g, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("SubgraphJSON: %v", err)
	}

	var result struct {
		Nodes []*graph.Node `json:"nodes"`
		Edges []*graph.Edge `json:"edges"`
	}
	json.Unmarshal(b, &result)

	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(result.Nodes))
	}
}
